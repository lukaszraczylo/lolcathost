// Package daemon provides the Unix socket server for the daemon.
package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/lukaszraczylo/lolcathost/internal/config"
	"github.com/lukaszraczylo/lolcathost/internal/protocol"
)

// Version is set by the main package at startup
var Version = "dev"

// Server is the daemon's Unix socket server.
type Server struct {
	socketPath   string
	listener     net.Listener
	config       *config.Manager
	hosts        *HostsManager
	flusher      *DNSFlusher
	rateLimiter  *RateLimiter
	auditLogger  *AuditLogger
	mu           sync.RWMutex
	running      bool
	stopCh       chan struct{}
	requestCount int64
	startTime    int64
}

// NewServer creates a new daemon server.
func NewServer(socketPath string, cfgManager *config.Manager) *Server {
	return &Server{
		socketPath:  socketPath,
		config:      cfgManager,
		hosts:       NewHostsManager(),
		flusher:     NewDNSFlusher(FlushMethodAuto),
		rateLimiter: NewRateLimiter(RateLimit, RateLimitWindow),
		stopCh:      make(chan struct{}),
	}
}

// Start starts the server.
func (s *Server) Start() error {
	// Remove existing socket
	os.Remove(s.socketPath)

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket: %w", err)
	}

	// Set socket permissions: 0660 root:lolcathost
	if err := os.Chmod(s.socketPath, 0660); err != nil {
		listener.Close()
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	// Set socket group to lolcathost (GID 850)
	if err := os.Chown(s.socketPath, 0, 850); err != nil {
		listener.Close()
		return fmt.Errorf("failed to set socket ownership: %w", err)
	}

	s.listener = listener
	s.running = true
	s.startTime = currentTimeUnix()

	// Try to create audit logger, but don't fail if it doesn't work
	if logger, err := NewAuditLogger(AuditLogPath); err == nil {
		s.auditLogger = logger
	}

	go s.acceptLoop()

	return nil
}

func currentTimeUnix() int64 {
	return time.Now().Unix()
}

// Stop stops the server.
func (s *Server) Stop() error {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	close(s.stopCh)

	if s.listener != nil {
		s.listener.Close()
	}

	os.Remove(s.socketPath)

	if s.auditLogger != nil {
		s.auditLogger.Close()
	}

	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				continue
			}
		}

		go s.handleConnection(conn)
	}
}

// LolcathostGID is the group ID for the lolcathost group.
const LolcathostGID = 850

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Get peer credentials
	creds := s.getPeerCredentials(conn)

	// Authorization check: verify peer is authorized
	if !s.isAuthorized(creds) {
		s.writeResponse(conn, protocol.NewErrorResponse(protocol.ErrCodeUnauthorized, "unauthorized: user not in lolcathost group"))
		if s.auditLogger != nil {
			var uid uint32
			var pid int32
			if creds != nil {
				uid = creds.UID
				pid = creds.PID
			}
			s.auditLogger.Log(uid, pid, "connect", nil, false, "unauthorized access attempt")
		}
		return
	}

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}

		var req protocol.Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeResponse(conn, protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "invalid JSON"))
			continue
		}

		// Rate limiting
		if creds != nil && !s.rateLimiter.Allow(creds.PID) {
			s.writeResponse(conn, protocol.NewErrorResponse(protocol.ErrCodeRateLimited, "rate limit exceeded"))
			continue
		}

		s.mu.Lock()
		s.requestCount++
		s.mu.Unlock()

		resp := s.handleRequest(&req, creds)
		s.writeResponse(conn, resp)
	}
}

// isAuthorized checks if the peer is authorized to access the daemon.
// Authorized users are: root (UID 0) or members of the lolcathost group (GID 850).
func (s *Server) isAuthorized(creds *PeerCredentials) bool {
	if creds == nil {
		// Can't verify credentials - deny by default
		return false
	}

	// Root is always authorized
	if creds.UID == 0 {
		return true
	}

	// Check if user's primary GID is lolcathost
	if creds.GID == LolcathostGID {
		return true
	}

	// Check supplementary groups (user might be in lolcathost as secondary group)
	// This requires looking up the user's groups from the system
	return isUserInGroup(creds.UID, LolcathostGID)
}

func (s *Server) writeResponse(conn net.Conn, resp *protocol.Response) {
	data, _ := json.Marshal(resp)
	data = append(data, '\n')
	conn.Write(data)
}

func (s *Server) handleRequest(req *protocol.Request, creds *PeerCredentials) *protocol.Response {
	var uid uint32
	var pid int32
	if creds != nil {
		uid = creds.UID
		pid = creds.PID
	}

	switch req.Type {
	case protocol.RequestPing:
		return s.handlePing()

	case protocol.RequestStatus:
		return s.handleStatus()

	case protocol.RequestList:
		return s.handleList()

	case protocol.RequestSet:
		resp := s.handleSet(req)
		if s.auditLogger != nil {
			var payload protocol.SetPayload
			_ = req.ParsePayload(&payload)
			s.auditLogger.Log(uid, pid, "set", payload, resp.IsOK(), resp.Message)
		}
		return resp

	case protocol.RequestSync:
		resp := s.handleSync()
		if s.auditLogger != nil {
			s.auditLogger.Log(uid, pid, "sync", nil, resp.IsOK(), resp.Message)
		}
		return resp

	case protocol.RequestPreset:
		resp := s.handlePreset(req)
		if s.auditLogger != nil {
			var payload protocol.PresetPayload
			_ = req.ParsePayload(&payload)
			s.auditLogger.Log(uid, pid, "preset", payload, resp.IsOK(), resp.Message)
		}
		return resp

	case protocol.RequestRollback:
		resp := s.handleRollback(req)
		if s.auditLogger != nil {
			var payload protocol.RollbackPayload
			_ = req.ParsePayload(&payload)
			s.auditLogger.Log(uid, pid, "rollback", payload, resp.IsOK(), resp.Message)
		}
		return resp

	case protocol.RequestBackups:
		return s.handleBackups()

	case protocol.RequestAdd:
		resp := s.handleAdd(req)
		if s.auditLogger != nil {
			var payload protocol.AddPayload
			_ = req.ParsePayload(&payload)
			s.auditLogger.Log(uid, pid, "add", payload, resp.IsOK(), resp.Message)
		}
		return resp

	case protocol.RequestDelete:
		resp := s.handleDelete(req)
		if s.auditLogger != nil {
			var payload protocol.DeletePayload
			_ = req.ParsePayload(&payload)
			s.auditLogger.Log(uid, pid, "delete", payload, resp.IsOK(), resp.Message)
		}
		return resp

	case protocol.RequestAddGroup:
		resp := s.handleAddGroup(req)
		if s.auditLogger != nil {
			var payload protocol.GroupPayload
			_ = req.ParsePayload(&payload)
			s.auditLogger.Log(uid, pid, "add_group", payload, resp.IsOK(), resp.Message)
		}
		return resp

	case protocol.RequestDeleteGroup:
		resp := s.handleDeleteGroup(req)
		if s.auditLogger != nil {
			var payload protocol.GroupPayload
			_ = req.ParsePayload(&payload)
			s.auditLogger.Log(uid, pid, "delete_group", payload, resp.IsOK(), resp.Message)
		}
		return resp

	case protocol.RequestListGroups:
		return s.handleListGroups()

	case protocol.RequestRenameGroup:
		resp := s.handleRenameGroup(req)
		if s.auditLogger != nil {
			var payload protocol.RenameGroupPayload
			_ = req.ParsePayload(&payload)
			s.auditLogger.Log(uid, pid, "rename_group", payload, resp.IsOK(), resp.Message)
		}
		return resp

	case protocol.RequestAddPreset:
		resp := s.handleAddPreset(req)
		if s.auditLogger != nil {
			var payload protocol.AddPresetPayload
			_ = req.ParsePayload(&payload)
			s.auditLogger.Log(uid, pid, "add_preset", payload, resp.IsOK(), resp.Message)
		}
		return resp

	case protocol.RequestDeletePreset:
		resp := s.handleDeletePreset(req)
		if s.auditLogger != nil {
			var payload protocol.PresetPayload
			_ = req.ParsePayload(&payload)
			s.auditLogger.Log(uid, pid, "delete_preset", payload, resp.IsOK(), resp.Message)
		}
		return resp

	case protocol.RequestListPresets:
		return s.handleListPresets()

	default:
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, fmt.Sprintf("unknown request type: %s", req.Type))
	}
}

func (s *Server) handlePing() *protocol.Response {
	resp, _ := protocol.NewOKResponse(map[string]string{"pong": "ok"})
	return resp
}

func (s *Server) handleStatus() *protocol.Response {
	s.mu.RLock()
	reqCount := s.requestCount
	startTime := s.startTime
	s.mu.RUnlock()

	cfg := s.config.Get()
	var activeCount int
	if cfg != nil {
		for _, h := range cfg.GetAllHosts() {
			if h.Enabled {
				activeCount++
			}
		}
	}

	data := protocol.StatusData{
		Running:      true,
		Version:      Version,
		Uptime:       nowUnix() - startTime,
		ActiveCount:  activeCount,
		RequestCount: reqCount,
	}

	resp, _ := protocol.NewOKResponse(data)
	return resp
}

func nowUnix() int64 {
	return time.Now().Unix()
}

func (s *Server) handleList() *protocol.Response {
	cfg := s.config.Get()
	if cfg == nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, "no configuration loaded")
	}

	var entries []protocol.HostEntry
	for _, g := range cfg.Groups {
		for _, h := range g.Hosts {
			entries = append(entries, protocol.HostEntry{
				Domain:  h.Domain,
				IP:      h.IP,
				Alias:   h.Alias,
				Enabled: h.Enabled,
				Group:   g.Name,
			})
		}
	}

	resp, _ := protocol.NewOKResponse(protocol.ListData{Entries: entries})
	return resp
}

func (s *Server) handleSet(req *protocol.Request) *protocol.Response {
	var payload protocol.SetPayload
	if err := req.ParsePayload(&payload); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "invalid payload")
	}

	cfg := s.config.Get()
	if cfg == nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, "no configuration loaded")
	}

	host, _ := cfg.FindHostByAlias(payload.Alias)
	if host == nil {
		return protocol.NewErrorResponse(protocol.ErrCodeNotFound, fmt.Sprintf("alias not found: %s", payload.Alias))
	}

	// Check for conflicts if enabling
	if payload.Enabled && !payload.Force {
		for _, g := range cfg.Groups {
			for _, h := range g.Hosts {
				if h.Alias != payload.Alias && h.Domain == host.Domain && h.Enabled {
					return protocol.NewErrorResponse(protocol.ErrCodeConflict,
						fmt.Sprintf("domain %s already mapped by alias %s (use force to override)", host.Domain, h.Alias))
				}
			}
		}
	}

	// Update config
	cfg.SetHostEnabled(payload.Alias, payload.Enabled)

	// Save config
	if err := s.config.Save(); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, fmt.Sprintf("failed to save config: %v", err))
	}

	// Sync to hosts file
	if err := s.syncHostsFile(); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, fmt.Sprintf("failed to sync hosts: %v", err))
	}

	resp, _ := protocol.NewOKResponse(protocol.SetData{
		Domain:  host.Domain,
		Applied: true,
	})
	return resp
}

func (s *Server) handleSync() *protocol.Response {
	if err := s.syncHostsFile(); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, fmt.Sprintf("failed to sync: %v", err))
	}

	resp, _ := protocol.NewOKResponse(map[string]bool{"synced": true})
	return resp
}

func (s *Server) handlePreset(req *protocol.Request) *protocol.Response {
	var payload protocol.PresetPayload
	if err := req.ParsePayload(&payload); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "invalid payload")
	}

	cfg := s.config.Get()
	if cfg == nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, "no configuration loaded")
	}

	if err := cfg.ApplyPreset(payload.Name); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeNotFound, err.Error())
	}

	// Save config
	if err := s.config.Save(); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, fmt.Sprintf("failed to save config: %v", err))
	}

	// Sync to hosts file
	if err := s.syncHostsFile(); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, fmt.Sprintf("failed to sync hosts: %v", err))
	}

	resp, _ := protocol.NewOKResponse(map[string]string{"preset": payload.Name, "applied": "true"})
	return resp
}

func (s *Server) handleRollback(req *protocol.Request) *protocol.Response {
	var payload protocol.RollbackPayload
	if err := req.ParsePayload(&payload); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "invalid payload")
	}

	if err := s.hosts.RestoreBackup(payload.BackupName); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, fmt.Sprintf("failed to restore backup: %v", err))
	}

	// Flush DNS after restore
	s.flusher.Flush()

	resp, _ := protocol.NewOKResponse(map[string]string{"restored": payload.BackupName})
	return resp
}

func (s *Server) handleBackups() *protocol.Response {
	backups, err := s.hosts.ListBackups()
	if err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, fmt.Sprintf("failed to list backups: %v", err))
	}

	var infos []protocol.BackupInfo
	for _, b := range backups {
		infos = append(infos, protocol.BackupInfo{
			Name:      b.Name,
			Timestamp: b.Timestamp,
			Size:      b.Size,
		})
	}

	resp, _ := protocol.NewOKResponse(protocol.BackupsData{Backups: infos})
	return resp
}

func (s *Server) handleAdd(req *protocol.Request) *protocol.Response {
	var payload protocol.AddPayload
	if err := req.ParsePayload(&payload); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "invalid payload")
	}

	// Validate domain
	if payload.Domain == "" {
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidDomain, "domain is required")
	}

	// Validate IP
	if payload.IP == "" {
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidIP, "IP address is required")
	}

	// Validate group
	if payload.Group == "" {
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "group is required")
	}

	// Check blocked domains
	if config.IsBlockedDomain(payload.Domain) {
		return protocol.NewErrorResponse(protocol.ErrCodeBlockedDomain, fmt.Sprintf("domain %s is blocked", payload.Domain))
	}

	cfg := s.config.Get()
	if cfg == nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, "no configuration loaded")
	}

	// Add to config (alias will be auto-generated if empty)
	if err := cfg.AddHost(payload.Domain, payload.IP, payload.Alias, payload.Group, payload.Enabled); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeConflict, err.Error())
	}

	// Save config
	if err := s.config.Save(); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, fmt.Sprintf("failed to save config: %v", err))
	}

	// Sync to hosts file
	if err := s.syncHostsFile(); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, fmt.Sprintf("failed to sync hosts: %v", err))
	}

	resp, _ := protocol.NewOKResponse(protocol.SetData{
		Domain:  payload.Domain,
		Applied: true,
	})
	return resp
}

func (s *Server) handleDelete(req *protocol.Request) *protocol.Response {
	var payload protocol.DeletePayload
	if err := req.ParsePayload(&payload); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "invalid payload")
	}

	if payload.Alias == "" {
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "alias is required")
	}

	cfg := s.config.Get()
	if cfg == nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, "no configuration loaded")
	}

	// Delete from config
	if !cfg.DeleteHost(payload.Alias) {
		return protocol.NewErrorResponse(protocol.ErrCodeNotFound, fmt.Sprintf("alias not found: %s", payload.Alias))
	}

	// Save config
	if err := s.config.Save(); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, fmt.Sprintf("failed to save config: %v", err))
	}

	// Sync to hosts file
	if err := s.syncHostsFile(); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, fmt.Sprintf("failed to sync hosts: %v", err))
	}

	resp, _ := protocol.NewOKResponse(map[string]string{"deleted": payload.Alias})
	return resp
}

func (s *Server) handleAddGroup(req *protocol.Request) *protocol.Response {
	var payload protocol.GroupPayload
	if err := req.ParsePayload(&payload); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "invalid payload")
	}

	if payload.Name == "" {
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "group name is required")
	}

	cfg := s.config.Get()
	if cfg == nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, "no configuration loaded")
	}

	if err := cfg.AddGroup(payload.Name); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeConflict, err.Error())
	}

	// Save config
	if err := s.config.Save(); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, fmt.Sprintf("failed to save config: %v", err))
	}

	resp, _ := protocol.NewOKResponse(map[string]string{"added": payload.Name})
	return resp
}

func (s *Server) handleDeleteGroup(req *protocol.Request) *protocol.Response {
	var payload protocol.GroupPayload
	if err := req.ParsePayload(&payload); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "invalid payload")
	}

	if payload.Name == "" {
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "group name is required")
	}

	cfg := s.config.Get()
	if cfg == nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, "no configuration loaded")
	}

	if err := cfg.DeleteGroup(payload.Name); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeNotFound, err.Error())
	}

	// Save config
	if err := s.config.Save(); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, fmt.Sprintf("failed to save config: %v", err))
	}

	// Sync to hosts file
	if err := s.syncHostsFile(); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, fmt.Sprintf("failed to sync hosts: %v", err))
	}

	resp, _ := protocol.NewOKResponse(map[string]string{"deleted": payload.Name})
	return resp
}

func (s *Server) handleListGroups() *protocol.Response {
	cfg := s.config.Get()
	if cfg == nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, "no configuration loaded")
	}

	resp, _ := protocol.NewOKResponse(protocol.GroupsData{Groups: cfg.GetGroups()})
	return resp
}

func (s *Server) handleRenameGroup(req *protocol.Request) *protocol.Response {
	var payload protocol.RenameGroupPayload
	if err := req.ParsePayload(&payload); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "invalid payload")
	}

	if payload.OldName == "" || payload.NewName == "" {
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "old_name and new_name are required")
	}

	cfg := s.config.Get()
	if cfg == nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, "no configuration loaded")
	}

	if err := cfg.RenameGroup(payload.OldName, payload.NewName); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeNotFound, err.Error())
	}

	// Save config
	if err := s.config.Save(); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, fmt.Sprintf("failed to save config: %v", err))
	}

	resp, _ := protocol.NewOKResponse(map[string]string{"renamed": payload.NewName})
	return resp
}

func (s *Server) handleAddPreset(req *protocol.Request) *protocol.Response {
	var payload protocol.AddPresetPayload
	if err := req.ParsePayload(&payload); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "invalid payload")
	}

	if payload.Name == "" {
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "preset name is required")
	}

	cfg := s.config.Get()
	if cfg == nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, "no configuration loaded")
	}

	if err := cfg.AddPreset(payload.Name, payload.Enable, payload.Disable); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeConflict, err.Error())
	}

	// Save config
	if err := s.config.Save(); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, fmt.Sprintf("failed to save config: %v", err))
	}

	resp, _ := protocol.NewOKResponse(map[string]string{"added": payload.Name})
	return resp
}

func (s *Server) handleDeletePreset(req *protocol.Request) *protocol.Response {
	var payload protocol.PresetPayload
	if err := req.ParsePayload(&payload); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "invalid payload")
	}

	if payload.Name == "" {
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "preset name is required")
	}

	cfg := s.config.Get()
	if cfg == nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, "no configuration loaded")
	}

	if err := cfg.DeletePreset(payload.Name); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeNotFound, err.Error())
	}

	// Save config
	if err := s.config.Save(); err != nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, fmt.Sprintf("failed to save config: %v", err))
	}

	resp, _ := protocol.NewOKResponse(map[string]string{"deleted": payload.Name})
	return resp
}

func (s *Server) handleListPresets() *protocol.Response {
	cfg := s.config.Get()
	if cfg == nil {
		return protocol.NewErrorResponse(protocol.ErrCodeInternalError, "no configuration loaded")
	}

	presets := cfg.GetPresets()
	infos := make([]protocol.PresetInfo, len(presets))
	for i, p := range presets {
		infos[i] = protocol.PresetInfo{
			Name:    p.Name,
			Enable:  p.Enable,
			Disable: p.Disable,
		}
	}

	resp, _ := protocol.NewOKResponse(protocol.PresetsData{Presets: infos})
	return resp
}

func (s *Server) syncHostsFile() error {
	cfg := s.config.Get()
	if cfg == nil {
		return fmt.Errorf("no configuration loaded")
	}

	var entries []HostEntry
	for _, g := range cfg.Groups {
		for _, h := range g.Hosts {
			entries = append(entries, HostEntry{
				IP:      h.IP,
				Domain:  h.Domain,
				Alias:   h.Alias,
				Enabled: h.Enabled,
			})
		}
	}

	if err := s.hosts.WriteManagedEntries(entries); err != nil {
		return err
	}

	// Flush DNS cache
	return s.flusher.Flush()
}
