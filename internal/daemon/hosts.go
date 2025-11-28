// Package daemon implements the privileged daemon that manages /etc/hosts.
package daemon

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	// HostsPath is the path to the system hosts file.
	HostsPath = "/etc/hosts"
	// BackupDir is the directory for hosts file backups.
	BackupDir = "/var/backups/lolcathost"
	// MaxBackups is the maximum number of backups to keep.
	MaxBackups = 10

	// Markers for the managed section.
	markerStart = "# ========== LOLCATHOST MANAGED - DO NOT EDIT =========="
	markerEnd   = "# ========== END LOLCATHOST =========="
)

// HostEntry represents a single entry in the hosts file.
type HostEntry struct {
	IP      string
	Domain  string
	Alias   string
	Enabled bool
}

// HostsManager handles reading and writing the hosts file.
type HostsManager struct {
	hostsPath string
	backupDir string
}

// NewHostsManager creates a new hosts manager.
func NewHostsManager() *HostsManager {
	return &HostsManager{
		hostsPath: HostsPath,
		backupDir: BackupDir,
	}
}

// NewHostsManagerWithPaths creates a hosts manager with custom paths (for testing).
func NewHostsManagerWithPaths(hostsPath, backupDir string) *HostsManager {
	return &HostsManager{
		hostsPath: hostsPath,
		backupDir: backupDir,
	}
}

// ReadManagedEntries reads the lolcathost-managed entries from the hosts file.
func (m *HostsManager) ReadManagedEntries() ([]HostEntry, error) {
	file, err := os.Open(m.hostsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open hosts file: %w", err)
	}
	defer file.Close()

	var entries []HostEntry
	inManagedSection := false
	scanner := bufio.NewScanner(file)
	entryRegex := regexp.MustCompile(`^(\S+)\s+(\S+)\s+#\s*lolcathost:(\S+)$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == markerStart {
			inManagedSection = true
			continue
		}
		if line == markerEnd {
			inManagedSection = false
			continue
		}

		if inManagedSection && !strings.HasPrefix(line, "#") && line != "" {
			matches := entryRegex.FindStringSubmatch(line)
			if len(matches) == 4 {
				entries = append(entries, HostEntry{
					IP:      matches[1],
					Domain:  matches[2],
					Alias:   matches[3],
					Enabled: true,
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read hosts file: %w", err)
	}

	return entries, nil
}

// WriteManagedEntries writes the managed entries to the hosts file.
func (m *HostsManager) WriteManagedEntries(entries []HostEntry) error {
	// Create backup first
	if err := m.CreateBackup(); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Read existing content
	content, err := os.ReadFile(m.hostsPath)
	if err != nil {
		return fmt.Errorf("failed to read hosts file: %w", err)
	}

	// Remove existing managed section
	newContent := m.removeManagedSection(string(content))

	// Build new managed section
	managedSection := m.buildManagedSection(entries)

	// Append managed section
	newContent = strings.TrimRight(newContent, "\n") + "\n\n" + managedSection

	// Write atomically
	if err := m.writeAtomic(newContent); err != nil {
		return fmt.Errorf("failed to write hosts file: %w", err)
	}

	return nil
}

func (m *HostsManager) removeManagedSection(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inManagedSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == markerStart {
			inManagedSection = true
			continue
		}
		if trimmed == markerEnd {
			inManagedSection = false
			continue
		}
		if !inManagedSection {
			result = append(result, line)
		}
	}

	// Remove trailing empty lines
	for len(result) > 0 && strings.TrimSpace(result[len(result)-1]) == "" {
		result = result[:len(result)-1]
	}

	return strings.Join(result, "\n")
}

func (m *HostsManager) buildManagedSection(entries []HostEntry) string {
	var sb strings.Builder
	sb.WriteString(markerStart)
	sb.WriteString("\n")

	for _, entry := range entries {
		if entry.Enabled {
			sb.WriteString(fmt.Sprintf("%s\t%s\t# lolcathost:%s\n", entry.IP, entry.Domain, entry.Alias))
		}
	}

	sb.WriteString(markerEnd)
	sb.WriteString("\n")

	return sb.String()
}

func (m *HostsManager) writeAtomic(content string) error {
	// Write to temp file first
	tmpFile := m.hostsPath + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		return err
	}

	// Rename atomically
	if err := os.Rename(tmpFile, m.hostsPath); err != nil {
		os.Remove(tmpFile)
		return err
	}

	return nil
}

// CreateBackup creates a backup of the current hosts file.
func (m *HostsManager) CreateBackup() error {
	if err := os.MkdirAll(m.backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	content, err := os.ReadFile(m.hostsPath)
	if err != nil {
		return fmt.Errorf("failed to read hosts file: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(m.backupDir, fmt.Sprintf("hosts.%s.bak", timestamp))

	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write backup: %w", err)
	}

	// Cleanup old backups
	if err := m.cleanupBackups(); err != nil {
		// Log but don't fail
		fmt.Fprintf(os.Stderr, "warning: failed to cleanup backups: %v\n", err)
	}

	return nil
}

func (m *HostsManager) cleanupBackups() error {
	entries, err := os.ReadDir(m.backupDir)
	if err != nil {
		return err
	}

	var backups []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "hosts.") && strings.HasSuffix(entry.Name(), ".bak") {
			backups = append(backups, entry)
		}
	}

	if len(backups) <= MaxBackups {
		return nil
	}

	// Sort by name (timestamp) descending
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Name() > backups[j].Name()
	})

	// Remove oldest backups
	for i := MaxBackups; i < len(backups); i++ {
		path := filepath.Join(m.backupDir, backups[i].Name())
		os.Remove(path)
	}

	return nil
}

// ListBackups returns a list of available backups.
func (m *HostsManager) ListBackups() ([]BackupInfo, error) {
	entries, err := os.ReadDir(m.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var backups []BackupInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "hosts.") || !strings.HasSuffix(entry.Name(), ".bak") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		backups = append(backups, BackupInfo{
			Name:      entry.Name(),
			Timestamp: info.ModTime().Unix(),
			Size:      info.Size(),
		})
	}

	// Sort by timestamp descending
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp > backups[j].Timestamp
	})

	return backups, nil
}

// BackupInfo holds information about a backup file.
type BackupInfo struct {
	Name      string
	Timestamp int64
	Size      int64
}

// RestoreBackup restores a backup by name.
func (m *HostsManager) RestoreBackup(name string) error {
	backupPath := filepath.Join(m.backupDir, name)

	// Validate backup name to prevent path traversal
	if filepath.Base(name) != name || !strings.HasPrefix(name, "hosts.") || !strings.HasSuffix(name, ".bak") {
		return fmt.Errorf("invalid backup name")
	}

	content, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	// Create a backup of current state before restoring
	if err := m.CreateBackup(); err != nil {
		return fmt.Errorf("failed to create backup before restore: %w", err)
	}

	if err := m.writeAtomic(string(content)); err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	return nil
}
