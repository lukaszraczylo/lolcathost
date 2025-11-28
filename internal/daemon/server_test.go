package daemon

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lukaszraczylo/lolcathost/internal/config"
	"github.com/lukaszraczylo/lolcathost/internal/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T) (*Server, string, func()) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")
	configPath := filepath.Join(tmpDir, "config.yaml")
	hostsPath := filepath.Join(tmpDir, "hosts")
	backupDir := filepath.Join(tmpDir, "backups")

	// Create initial hosts file
	err := os.WriteFile(hostsPath, []byte("127.0.0.1\tlocalhost\n"), 0644)
	require.NoError(t, err)

	// Create config
	err = config.CreateDefault(configPath)
	require.NoError(t, err)

	cfgManager := config.NewManager(configPath)
	err = cfgManager.Load()
	require.NoError(t, err)

	server := &Server{
		socketPath:  socketPath,
		config:      cfgManager,
		hosts:       NewHostsManagerWithPaths(hostsPath, backupDir),
		flusher:     NewDNSFlusher(FlushMethodAuto),
		rateLimiter: NewRateLimiter(100, time.Minute),
		stopCh:      make(chan struct{}),
	}

	cleanup := func() {
		server.Stop()
	}

	return server, tmpDir, cleanup
}

func TestServer_HandlePing(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	resp := server.handlePing()
	assert.Equal(t, "ok", resp.Status)
}

func TestServer_HandleStatus(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	resp := server.handleStatus()
	assert.Equal(t, "ok", resp.Status)

	var data protocol.StatusData
	err := resp.ParseData(&data)
	require.NoError(t, err)

	assert.True(t, data.Running)
}

func TestServer_HandleList(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	resp := server.handleList()
	assert.Equal(t, "ok", resp.Status)

	var data protocol.ListData
	err := resp.ParseData(&data)
	require.NoError(t, err)

	assert.NotNil(t, data.Entries)
}

func TestServer_HandleSet(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// First add a host to set
	cfg := server.config.Get()
	cfg.AddHost("test.local", "127.0.0.1", "test-local", "default", false)
	server.config.Save()

	t.Run("enable host", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestSet, protocol.SetPayload{
			Alias:   "test-local",
			Enabled: true,
		})
		resp := server.handleSet(req)
		assert.Equal(t, "ok", resp.Status)
	})

	t.Run("disable host", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestSet, protocol.SetPayload{
			Alias:   "test-local",
			Enabled: false,
		})
		resp := server.handleSet(req)
		assert.Equal(t, "ok", resp.Status)
	})

	t.Run("nonexistent host", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestSet, protocol.SetPayload{
			Alias:   "nonexistent",
			Enabled: true,
		})
		resp := server.handleSet(req)
		assert.Equal(t, "error", resp.Status)
		assert.Equal(t, protocol.ErrCodeNotFound, resp.Code)
	})

	t.Run("invalid payload", func(t *testing.T) {
		req := &protocol.Request{
			Type:    protocol.RequestSet,
			Payload: json.RawMessage(`{invalid`),
		}
		resp := server.handleSet(req)
		assert.Equal(t, "error", resp.Status)
	})
}

func TestServer_HandleAdd(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	t.Run("valid host", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestAdd, protocol.AddPayload{
			Domain: "newhost.local",
			IP:     "127.0.0.1",
			Group:  "default",
		})
		resp := server.handleAdd(req)
		assert.Equal(t, "ok", resp.Status)
	})

	t.Run("duplicate alias", func(t *testing.T) {
		// When alias is explicitly provided, duplicates are rejected
		req, _ := protocol.NewRequest(protocol.RequestAdd, protocol.AddPayload{
			Domain: "another.local",
			IP:     "127.0.0.1",
			Alias:  "newhost-local", // Same alias as auto-generated for newhost.local
			Group:  "default",
		})
		resp := server.handleAdd(req)
		assert.Equal(t, "error", resp.Status)
		assert.Equal(t, protocol.ErrCodeConflict, resp.Code)
	})

	t.Run("blocked domain", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestAdd, protocol.AddPayload{
			Domain: "apple.com",
			IP:     "127.0.0.1",
			Group:  "default",
		})
		resp := server.handleAdd(req)
		assert.Equal(t, "error", resp.Status)
		assert.Equal(t, protocol.ErrCodeBlockedDomain, resp.Code)
	})

	t.Run("invalid domain", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestAdd, protocol.AddPayload{
			Domain: "",
			IP:     "127.0.0.1",
			Group:  "default",
		})
		resp := server.handleAdd(req)
		assert.Equal(t, "error", resp.Status)
	})

	t.Run("empty IP", func(t *testing.T) {
		// Only empty IP is rejected, format is not validated
		req, _ := protocol.NewRequest(protocol.RequestAdd, protocol.AddPayload{
			Domain: "valid.local",
			IP:     "",
			Group:  "default",
		})
		resp := server.handleAdd(req)
		assert.Equal(t, "error", resp.Status)
		assert.Equal(t, protocol.ErrCodeInvalidIP, resp.Code)
	})

	t.Run("invalid payload", func(t *testing.T) {
		req := &protocol.Request{
			Type:    protocol.RequestAdd,
			Payload: json.RawMessage(`{invalid`),
		}
		resp := server.handleAdd(req)
		assert.Equal(t, "error", resp.Status)
	})
}

func TestServer_HandleDelete(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Add a host first
	cfg := server.config.Get()
	cfg.AddHost("todelete.local", "127.0.0.1", "todelete", "default", false)
	server.config.Save()

	t.Run("delete existing", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestDelete, protocol.DeletePayload{
			Alias: "todelete",
		})
		resp := server.handleDelete(req)
		assert.Equal(t, "ok", resp.Status)
	})

	t.Run("delete nonexistent", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestDelete, protocol.DeletePayload{
			Alias: "nonexistent",
		})
		resp := server.handleDelete(req)
		assert.Equal(t, "error", resp.Status)
		assert.Equal(t, protocol.ErrCodeNotFound, resp.Code)
	})

	t.Run("invalid payload", func(t *testing.T) {
		req := &protocol.Request{
			Type:    protocol.RequestDelete,
			Payload: json.RawMessage(`{invalid`),
		}
		resp := server.handleDelete(req)
		assert.Equal(t, "error", resp.Status)
	})
}

func TestServer_HandleSync(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	resp := server.handleSync()
	assert.Equal(t, "ok", resp.Status)
}

func TestServer_HandleBackups(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a backup first
	server.hosts.CreateBackup()

	resp := server.handleBackups()
	assert.Equal(t, "ok", resp.Status)

	var data protocol.BackupsData
	err := resp.ParseData(&data)
	require.NoError(t, err)
	assert.NotNil(t, data.Backups)
}

func TestServer_HandleAddGroup(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	t.Run("add new group", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestAddGroup, protocol.GroupPayload{
			Name: "newgroup",
		})
		resp := server.handleAddGroup(req)
		assert.Equal(t, "ok", resp.Status)
	})

	t.Run("add duplicate group", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestAddGroup, protocol.GroupPayload{
			Name: "newgroup",
		})
		resp := server.handleAddGroup(req)
		assert.Equal(t, "error", resp.Status)
		assert.Equal(t, protocol.ErrCodeConflict, resp.Code)
	})

	t.Run("empty name", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestAddGroup, protocol.GroupPayload{
			Name: "",
		})
		resp := server.handleAddGroup(req)
		assert.Equal(t, "error", resp.Status)
	})

	t.Run("invalid payload", func(t *testing.T) {
		req := &protocol.Request{
			Type:    protocol.RequestAddGroup,
			Payload: json.RawMessage(`{invalid`),
		}
		resp := server.handleAddGroup(req)
		assert.Equal(t, "error", resp.Status)
	})
}

func TestServer_HandleDeleteGroup(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Add a group first
	cfg := server.config.Get()
	cfg.AddGroup("todeletegroup")
	server.config.Save()

	t.Run("delete existing group", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestDeleteGroup, protocol.GroupPayload{
			Name: "todeletegroup",
		})
		resp := server.handleDeleteGroup(req)
		assert.Equal(t, "ok", resp.Status)
	})

	t.Run("delete nonexistent group", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestDeleteGroup, protocol.GroupPayload{
			Name: "nonexistent",
		})
		resp := server.handleDeleteGroup(req)
		assert.Equal(t, "error", resp.Status)
		assert.Equal(t, protocol.ErrCodeNotFound, resp.Code)
	})

	t.Run("invalid payload", func(t *testing.T) {
		req := &protocol.Request{
			Type:    protocol.RequestDeleteGroup,
			Payload: json.RawMessage(`{invalid`),
		}
		resp := server.handleDeleteGroup(req)
		assert.Equal(t, "error", resp.Status)
	})
}

func TestServer_HandleListGroups(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	resp := server.handleListGroups()
	assert.Equal(t, "ok", resp.Status)

	var data protocol.GroupsData
	err := resp.ParseData(&data)
	require.NoError(t, err)
	assert.NotNil(t, data.Groups)
}

func TestServer_HandleRenameGroup(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Add a group to rename
	cfg := server.config.Get()
	cfg.AddGroup("oldname")
	server.config.Save()

	t.Run("rename existing group", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestRenameGroup, protocol.RenameGroupPayload{
			OldName: "oldname",
			NewName: "newname",
		})
		resp := server.handleRenameGroup(req)
		assert.Equal(t, "ok", resp.Status)
	})

	t.Run("rename nonexistent group", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestRenameGroup, protocol.RenameGroupPayload{
			OldName: "nonexistent",
			NewName: "newname2",
		})
		resp := server.handleRenameGroup(req)
		assert.Equal(t, "error", resp.Status)
	})

	t.Run("invalid payload", func(t *testing.T) {
		req := &protocol.Request{
			Type:    protocol.RequestRenameGroup,
			Payload: json.RawMessage(`{invalid`),
		}
		resp := server.handleRenameGroup(req)
		assert.Equal(t, "error", resp.Status)
	})
}

func TestServer_HandleAddPreset(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	t.Run("add new preset", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestAddPreset, protocol.AddPresetPayload{
			Name:    "newpreset",
			Enable:  []string{"alias1"},
			Disable: []string{"alias2"},
		})
		resp := server.handleAddPreset(req)
		assert.Equal(t, "ok", resp.Status)
	})

	t.Run("add duplicate preset", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestAddPreset, protocol.AddPresetPayload{
			Name:    "newpreset",
			Enable:  []string{"alias1"},
			Disable: []string{"alias2"},
		})
		resp := server.handleAddPreset(req)
		assert.Equal(t, "error", resp.Status)
		assert.Equal(t, protocol.ErrCodeConflict, resp.Code)
	})

	t.Run("empty name", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestAddPreset, protocol.AddPresetPayload{
			Name: "",
		})
		resp := server.handleAddPreset(req)
		assert.Equal(t, "error", resp.Status)
	})

	t.Run("invalid payload", func(t *testing.T) {
		req := &protocol.Request{
			Type:    protocol.RequestAddPreset,
			Payload: json.RawMessage(`{invalid`),
		}
		resp := server.handleAddPreset(req)
		assert.Equal(t, "error", resp.Status)
	})
}

func TestServer_HandleDeletePreset(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Add a preset first
	cfg := server.config.Get()
	cfg.AddPreset("todeletepreset", []string{"a"}, []string{"b"})
	server.config.Save()

	t.Run("delete existing preset", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestDeletePreset, protocol.PresetPayload{
			Name: "todeletepreset",
		})
		resp := server.handleDeletePreset(req)
		assert.Equal(t, "ok", resp.Status)
	})

	t.Run("delete nonexistent preset", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestDeletePreset, protocol.PresetPayload{
			Name: "nonexistent",
		})
		resp := server.handleDeletePreset(req)
		assert.Equal(t, "error", resp.Status)
		assert.Equal(t, protocol.ErrCodeNotFound, resp.Code)
	})

	t.Run("invalid payload", func(t *testing.T) {
		req := &protocol.Request{
			Type:    protocol.RequestDeletePreset,
			Payload: json.RawMessage(`{invalid`),
		}
		resp := server.handleDeletePreset(req)
		assert.Equal(t, "error", resp.Status)
	})
}

func TestServer_HandleListPresets(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	resp := server.handleListPresets()
	assert.Equal(t, "ok", resp.Status)

	var data protocol.PresetsData
	err := resp.ParseData(&data)
	require.NoError(t, err)
	assert.NotNil(t, data.Presets)
}

func TestServer_HandlePreset(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Add hosts and preset
	cfg := server.config.Get()
	cfg.AddHost("host1.local", "127.0.0.1", "host1", "default", false)
	cfg.AddHost("host2.local", "127.0.0.1", "host2", "default", false)
	cfg.AddPreset("testpreset", []string{"host1"}, []string{"host2"})
	server.config.Save()

	t.Run("apply existing preset", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestPreset, protocol.PresetPayload{
			Name: "testpreset",
		})
		resp := server.handlePreset(req)
		assert.Equal(t, "ok", resp.Status)
	})

	t.Run("apply nonexistent preset", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestPreset, protocol.PresetPayload{
			Name: "nonexistent",
		})
		resp := server.handlePreset(req)
		assert.Equal(t, "error", resp.Status)
		assert.Equal(t, protocol.ErrCodeNotFound, resp.Code)
	})

	t.Run("invalid payload", func(t *testing.T) {
		req := &protocol.Request{
			Type:    protocol.RequestPreset,
			Payload: json.RawMessage(`{invalid`),
		}
		resp := server.handlePreset(req)
		assert.Equal(t, "error", resp.Status)
	})
}

func TestServer_HandleRollback(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a backup first
	server.hosts.CreateBackup()
	backups, _ := server.hosts.ListBackups()
	require.NotEmpty(t, backups)

	t.Run("rollback to existing backup", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestRollback, protocol.RollbackPayload{
			BackupName: backups[0].Name,
		})
		resp := server.handleRollback(req)
		assert.Equal(t, "ok", resp.Status)
	})

	t.Run("rollback to nonexistent backup", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestRollback, protocol.RollbackPayload{
			BackupName: "nonexistent.bak",
		})
		resp := server.handleRollback(req)
		assert.Equal(t, "error", resp.Status)
	})

	t.Run("invalid payload", func(t *testing.T) {
		req := &protocol.Request{
			Type:    protocol.RequestRollback,
			Payload: json.RawMessage(`{invalid`),
		}
		resp := server.handleRollback(req)
		assert.Equal(t, "error", resp.Status)
	})
}

func TestServer_HandleBackupContent(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a backup first
	server.hosts.CreateBackup()
	backups, _ := server.hosts.ListBackups()
	require.NotEmpty(t, backups)

	t.Run("get existing backup content", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestBackupContent, protocol.BackupContentPayload{
			BackupName: backups[0].Name,
		})
		resp := server.handleBackupContent(req)
		assert.Equal(t, "ok", resp.Status)

		var data protocol.BackupContentData
		err := resp.ParseData(&data)
		require.NoError(t, err)
		assert.NotEmpty(t, data.Content)
	})

	t.Run("get nonexistent backup content", func(t *testing.T) {
		req, _ := protocol.NewRequest(protocol.RequestBackupContent, protocol.BackupContentPayload{
			BackupName: "nonexistent.bak",
		})
		resp := server.handleBackupContent(req)
		assert.Equal(t, "error", resp.Status)
	})

	t.Run("invalid payload", func(t *testing.T) {
		req := &protocol.Request{
			Type:    protocol.RequestBackupContent,
			Payload: json.RawMessage(`{invalid`),
		}
		resp := server.handleBackupContent(req)
		assert.Equal(t, "error", resp.Status)
	})
}

func TestServer_HandleRequest_UnknownType(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := &protocol.Request{
		Type: "unknown_type",
	}
	creds := &PeerCredentials{UID: 0, GID: 0, PID: 1}
	resp := server.handleRequest(req, creds)
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, protocol.ErrCodeInvalidRequest, resp.Code)
}

func TestServer_IsAuthorized(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	t.Run("root user", func(t *testing.T) {
		creds := &PeerCredentials{UID: 0, GID: 0, PID: 1}
		assert.True(t, server.isAuthorized(creds))
	})

	t.Run("nil credentials", func(t *testing.T) {
		assert.False(t, server.isAuthorized(nil))
	})
}

func TestServer_StartStop(t *testing.T) {
	// Skip test if not running as root (server.Start requires root to chown socket)
	if os.Getuid() != 0 {
		t.Skip("Test requires root privileges to create socket with proper ownership")
	}

	server, _, _ := setupTestServer(t)
	// Don't use cleanup since we manually call Stop

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start()
	}()

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Verify socket exists
	_, err := os.Stat(server.socketPath)
	assert.NoError(t, err)

	// Stop server
	err = server.Stop()
	assert.NoError(t, err)

	// Verify socket is removed
	_, err = os.Stat(server.socketPath)
	assert.True(t, os.IsNotExist(err))
}

func TestServer_AcceptConnection(t *testing.T) {
	// Skip test if not running as root (server.Start requires root to chown socket)
	if os.Getuid() != 0 {
		t.Skip("Test requires root privileges to create socket with proper ownership")
	}

	server, _, _ := setupTestServer(t)
	// Don't use cleanup - manually stop

	// Start server
	go server.Start()
	time.Sleep(100 * time.Millisecond)
	defer server.Stop()

	// Connect to server
	conn, err := net.Dial("unix", server.socketPath)
	require.NoError(t, err)
	defer conn.Close()

	// Send ping request
	req, _ := protocol.NewRequest(protocol.RequestPing, nil)
	encoder := json.NewEncoder(conn)
	err = encoder.Encode(req)
	require.NoError(t, err)

	// Read response
	decoder := json.NewDecoder(conn)
	var resp protocol.Response
	err = decoder.Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "ok", resp.Status)
}

// Benchmarks

func BenchmarkServer_HandlePing(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	config.CreateDefault(configPath)
	cfgManager := config.NewManager(configPath)
	cfgManager.Load()

	server := &Server{
		config:      cfgManager,
		rateLimiter: NewRateLimiter(100000, time.Minute),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server.handlePing()
	}
}

func BenchmarkServer_HandleList(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	config.CreateDefault(configPath)
	cfgManager := config.NewManager(configPath)
	cfgManager.Load()

	server := &Server{
		config:      cfgManager,
		rateLimiter: NewRateLimiter(100000, time.Minute),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server.handleList()
	}
}

func BenchmarkServer_HandleSet(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	hostsPath := filepath.Join(tmpDir, "hosts")
	backupDir := filepath.Join(tmpDir, "backups")

	os.WriteFile(hostsPath, []byte("127.0.0.1\tlocalhost\n"), 0644)
	config.CreateDefault(configPath)
	cfgManager := config.NewManager(configPath)
	cfgManager.Load()

	// Add a test host
	cfg := cfgManager.Get()
	cfg.AddHost("bench.local", "127.0.0.1", "bench-local", "default", false)

	server := &Server{
		config:      cfgManager,
		hosts:       NewHostsManagerWithPaths(hostsPath, backupDir),
		flusher:     NewDNSFlusher(FlushMethodAuto),
		rateLimiter: NewRateLimiter(100000, time.Minute),
	}

	req, _ := protocol.NewRequest(protocol.RequestSet, protocol.SetPayload{
		Alias:   "bench-local",
		Enabled: true,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server.handleSet(req)
	}
}
