package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHostsManager_ReadManagedEntries(t *testing.T) {
	tmpDir := t.TempDir()
	hostsPath := filepath.Join(tmpDir, "hosts")

	hostsContent := `127.0.0.1	localhost
255.255.255.255	broadcasthost
::1             localhost

# ========== LOLCATHOST MANAGED - DO NOT EDIT ==========
127.0.0.1	example.com	# lolcathost:example-local
192.168.1.1	api.example.com	# lolcathost:api-local
# ========== END LOLCATHOST ==========
`
	err := os.WriteFile(hostsPath, []byte(hostsContent), 0644)
	require.NoError(t, err)

	manager := NewHostsManagerWithPaths(hostsPath, filepath.Join(tmpDir, "backups"))
	entries, err := manager.ReadManagedEntries()
	require.NoError(t, err)

	assert.Len(t, entries, 2)
	assert.Equal(t, "127.0.0.1", entries[0].IP)
	assert.Equal(t, "example.com", entries[0].Domain)
	assert.Equal(t, "example-local", entries[0].Alias)
	assert.Equal(t, "192.168.1.1", entries[1].IP)
	assert.Equal(t, "api.example.com", entries[1].Domain)
	assert.Equal(t, "api-local", entries[1].Alias)
}

func TestHostsManager_ReadManagedEntries_NoSection(t *testing.T) {
	tmpDir := t.TempDir()
	hostsPath := filepath.Join(tmpDir, "hosts")

	hostsContent := `127.0.0.1	localhost
255.255.255.255	broadcasthost
`
	err := os.WriteFile(hostsPath, []byte(hostsContent), 0644)
	require.NoError(t, err)

	manager := NewHostsManagerWithPaths(hostsPath, filepath.Join(tmpDir, "backups"))
	entries, err := manager.ReadManagedEntries()
	require.NoError(t, err)

	assert.Empty(t, entries)
}

func TestHostsManager_WriteManagedEntries(t *testing.T) {
	tmpDir := t.TempDir()
	hostsPath := filepath.Join(tmpDir, "hosts")
	backupDir := filepath.Join(tmpDir, "backups")

	// Create initial hosts file
	initialContent := `127.0.0.1	localhost
255.255.255.255	broadcasthost
`
	err := os.WriteFile(hostsPath, []byte(initialContent), 0644)
	require.NoError(t, err)

	manager := NewHostsManagerWithPaths(hostsPath, backupDir)

	entries := []HostEntry{
		{IP: "127.0.0.1", Domain: "myapp.com", Alias: "myapp-local", Enabled: true},
		{IP: "127.0.0.1", Domain: "api.myapp.com", Alias: "api-local", Enabled: true},
		{IP: "192.168.1.1", Domain: "staging.myapp.com", Alias: "staging", Enabled: false},
	}

	err = manager.WriteManagedEntries(entries)
	require.NoError(t, err)

	// Read back
	content, err := os.ReadFile(hostsPath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "127.0.0.1\tlocalhost")
	assert.Contains(t, contentStr, "# ========== LOLCATHOST MANAGED - DO NOT EDIT ==========")
	assert.Contains(t, contentStr, "127.0.0.1\tmyapp.com\t# lolcathost:myapp-local")
	assert.Contains(t, contentStr, "127.0.0.1\tapi.myapp.com\t# lolcathost:api-local")
	assert.NotContains(t, contentStr, "staging.myapp.com") // disabled
	assert.Contains(t, contentStr, "# ========== END LOLCATHOST ==========")
}

func TestHostsManager_WriteManagedEntries_UpdatesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	hostsPath := filepath.Join(tmpDir, "hosts")
	backupDir := filepath.Join(tmpDir, "backups")

	// Create hosts file with existing managed section
	initialContent := `127.0.0.1	localhost

# ========== LOLCATHOST MANAGED - DO NOT EDIT ==========
127.0.0.1	old.com	# lolcathost:old
# ========== END LOLCATHOST ==========
`
	err := os.WriteFile(hostsPath, []byte(initialContent), 0644)
	require.NoError(t, err)

	manager := NewHostsManagerWithPaths(hostsPath, backupDir)

	entries := []HostEntry{
		{IP: "127.0.0.1", Domain: "new.com", Alias: "new", Enabled: true},
	}

	err = manager.WriteManagedEntries(entries)
	require.NoError(t, err)

	content, err := os.ReadFile(hostsPath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "127.0.0.1\tlocalhost")
	assert.Contains(t, contentStr, "new.com")
	assert.NotContains(t, contentStr, "old.com")
}

func TestHostsManager_CreateBackup(t *testing.T) {
	tmpDir := t.TempDir()
	hostsPath := filepath.Join(tmpDir, "hosts")
	backupDir := filepath.Join(tmpDir, "backups")

	hostsContent := "127.0.0.1\tlocalhost\n"
	err := os.WriteFile(hostsPath, []byte(hostsContent), 0644)
	require.NoError(t, err)

	manager := NewHostsManagerWithPaths(hostsPath, backupDir)

	err = manager.CreateBackup()
	require.NoError(t, err)

	// Verify backup exists
	entries, err := os.ReadDir(backupDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.True(t, strings.HasPrefix(entries[0].Name(), "hosts."))
	assert.True(t, strings.HasSuffix(entries[0].Name(), ".bak"))

	// Verify backup content
	backupContent, err := os.ReadFile(filepath.Join(backupDir, entries[0].Name()))
	require.NoError(t, err)
	assert.Equal(t, hostsContent, string(backupContent))
}

func TestHostsManager_ListBackups(t *testing.T) {
	tmpDir := t.TempDir()
	hostsPath := filepath.Join(tmpDir, "hosts")
	backupDir := filepath.Join(tmpDir, "backups")

	// Create hosts file
	err := os.WriteFile(hostsPath, []byte("localhost"), 0644)
	require.NoError(t, err)

	// Manually create backup files with different timestamps
	err = os.MkdirAll(backupDir, 0755)
	require.NoError(t, err)

	backupNames := []string{
		"hosts.20231201-120000.bak",
		"hosts.20231201-120001.bak",
		"hosts.20231201-120002.bak",
	}
	for _, name := range backupNames {
		err = os.WriteFile(filepath.Join(backupDir, name), []byte("backup"), 0644)
		require.NoError(t, err)
	}

	manager := NewHostsManagerWithPaths(hostsPath, backupDir)

	backups, err := manager.ListBackups()
	require.NoError(t, err)
	assert.Len(t, backups, 3)
}

func TestHostsManager_ListBackups_NoBackupDir(t *testing.T) {
	tmpDir := t.TempDir()
	hostsPath := filepath.Join(tmpDir, "hosts")
	backupDir := filepath.Join(tmpDir, "nonexistent")

	manager := NewHostsManagerWithPaths(hostsPath, backupDir)

	backups, err := manager.ListBackups()
	require.NoError(t, err)
	assert.Empty(t, backups)
}

func TestHostsManager_RestoreBackup(t *testing.T) {
	tmpDir := t.TempDir()
	hostsPath := filepath.Join(tmpDir, "hosts")
	backupDir := filepath.Join(tmpDir, "backups")

	// Create initial hosts file
	initialContent := "initial content"
	err := os.WriteFile(hostsPath, []byte(initialContent), 0644)
	require.NoError(t, err)

	manager := NewHostsManagerWithPaths(hostsPath, backupDir)

	// Create backup
	err = manager.CreateBackup()
	require.NoError(t, err)

	// Modify hosts file
	err = os.WriteFile(hostsPath, []byte("modified content"), 0644)
	require.NoError(t, err)

	// Get backup name
	backups, err := manager.ListBackups()
	require.NoError(t, err)
	require.Len(t, backups, 1)

	// Restore
	err = manager.RestoreBackup(backups[0].Name)
	require.NoError(t, err)

	// Verify content restored
	content, err := os.ReadFile(hostsPath)
	require.NoError(t, err)
	assert.Equal(t, initialContent, string(content))
}

func TestHostsManager_RestoreBackup_InvalidName(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewHostsManagerWithPaths(
		filepath.Join(tmpDir, "hosts"),
		filepath.Join(tmpDir, "backups"),
	)

	tests := []string{
		"../../../etc/passwd",
		"hosts.bak",        // Missing timestamp
		"notahosts.backup", // Wrong format
		"",
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			err := manager.RestoreBackup(name)
			assert.Error(t, err)
		})
	}
}

func TestHostsManager_CleanupBackups(t *testing.T) {
	tmpDir := t.TempDir()
	hostsPath := filepath.Join(tmpDir, "hosts")
	backupDir := filepath.Join(tmpDir, "backups")

	err := os.WriteFile(hostsPath, []byte("localhost"), 0644)
	require.NoError(t, err)

	manager := NewHostsManagerWithPaths(hostsPath, backupDir)

	// Create more than MaxBackups
	for i := 0; i < MaxBackups+5; i++ {
		err = manager.CreateBackup()
		require.NoError(t, err)
	}

	// Verify only MaxBackups remain
	backups, err := manager.ListBackups()
	require.NoError(t, err)
	assert.LessOrEqual(t, len(backups), MaxBackups)
}

func TestHostsManager_RemoveManagedSection(t *testing.T) {
	manager := &HostsManager{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "with managed section",
			input: `127.0.0.1	localhost

# ========== LOLCATHOST MANAGED - DO NOT EDIT ==========
127.0.0.1	example.com	# lolcathost:test
# ========== END LOLCATHOST ==========
`,
			expected: "127.0.0.1\tlocalhost",
		},
		{
			name:     "without managed section",
			input:    "127.0.0.1\tlocalhost\n",
			expected: "127.0.0.1\tlocalhost",
		},
		{
			name: "multiple managed sections",
			input: `127.0.0.1	localhost
# ========== LOLCATHOST MANAGED - DO NOT EDIT ==========
entry1
# ========== END LOLCATHOST ==========
more content
# ========== LOLCATHOST MANAGED - DO NOT EDIT ==========
entry2
# ========== END LOLCATHOST ==========
`,
			expected: "127.0.0.1\tlocalhost\nmore content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.removeManagedSection(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHostsManager_BuildManagedSection(t *testing.T) {
	manager := &HostsManager{}

	entries := []HostEntry{
		{IP: "127.0.0.1", Domain: "a.com", Alias: "a", Enabled: true},
		{IP: "192.168.1.1", Domain: "b.com", Alias: "b", Enabled: true},
		{IP: "10.0.0.1", Domain: "c.com", Alias: "c", Enabled: false},
	}

	result := manager.buildManagedSection(entries)

	assert.Contains(t, result, "# ========== LOLCATHOST MANAGED - DO NOT EDIT ==========")
	assert.Contains(t, result, "127.0.0.1\ta.com\t# lolcathost:a")
	assert.Contains(t, result, "192.168.1.1\tb.com\t# lolcathost:b")
	assert.NotContains(t, result, "c.com") // disabled
	assert.Contains(t, result, "# ========== END LOLCATHOST ==========")
}

// Matrix tests for hosts file parsing
func TestHostsManager_ReadManagedEntries_Matrix(t *testing.T) {
	ips := []string{"127.0.0.1", "192.168.1.1", "::1"}
	domains := []string{"example.com", "sub.example.com", "my-app.test"}
	aliases := []string{"test", "my-alias", "app-1"}

	for _, ip := range ips {
		for _, domain := range domains {
			for _, alias := range aliases {
				t.Run(ip+"/"+domain+"/"+alias, func(t *testing.T) {
					tmpDir := t.TempDir()
					hostsPath := filepath.Join(tmpDir, "hosts")

					content := "# ========== LOLCATHOST MANAGED - DO NOT EDIT ==========\n"
					content += ip + "\t" + domain + "\t# lolcathost:" + alias + "\n"
					content += "# ========== END LOLCATHOST ==========\n"

					err := os.WriteFile(hostsPath, []byte(content), 0644)
					require.NoError(t, err)

					manager := NewHostsManagerWithPaths(hostsPath, filepath.Join(tmpDir, "backups"))
					entries, err := manager.ReadManagedEntries()
					require.NoError(t, err)
					require.Len(t, entries, 1)

					assert.Equal(t, ip, entries[0].IP)
					assert.Equal(t, domain, entries[0].Domain)
					assert.Equal(t, alias, entries[0].Alias)
				})
			}
		}
	}
}

func BenchmarkHostsManager_ReadManagedEntries(b *testing.B) {
	tmpDir := b.TempDir()
	hostsPath := filepath.Join(tmpDir, "hosts")

	// Create a hosts file with many entries
	var content strings.Builder
	content.WriteString("127.0.0.1\tlocalhost\n")
	content.WriteString("# ========== LOLCATHOST MANAGED - DO NOT EDIT ==========\n")
	for i := 0; i < 100; i++ {
		content.WriteString("127.0.0.1\texample" + string(rune('a'+i%26)) + ".com\t# lolcathost:alias" + string(rune('a'+i%26)) + "\n")
	}
	content.WriteString("# ========== END LOLCATHOST ==========\n")

	err := os.WriteFile(hostsPath, []byte(content.String()), 0644)
	require.NoError(b, err)

	manager := NewHostsManagerWithPaths(hostsPath, filepath.Join(tmpDir, "backups"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.ReadManagedEntries()
	}
}

func BenchmarkHostsManager_WriteManagedEntries(b *testing.B) {
	tmpDir := b.TempDir()
	hostsPath := filepath.Join(tmpDir, "hosts")
	backupDir := filepath.Join(tmpDir, "backups")

	err := os.WriteFile(hostsPath, []byte("127.0.0.1\tlocalhost\n"), 0644)
	require.NoError(b, err)

	manager := NewHostsManagerWithPaths(hostsPath, backupDir)

	entries := make([]HostEntry, 50)
	for i := range entries {
		entries[i] = HostEntry{
			IP:      "127.0.0.1",
			Domain:  "example" + string(rune('a'+i%26)) + ".com",
			Alias:   "alias" + string(rune('a'+i%26)),
			Enabled: i%2 == 0,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.WriteManagedEntries(entries)
	}
}
