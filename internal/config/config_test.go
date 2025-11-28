package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_GetAllHosts(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				Name: "dev",
				Hosts: []Host{
					{Domain: "a.com", IP: "127.0.0.1", Alias: "a", Enabled: true},
					{Domain: "b.com", IP: "127.0.0.1", Alias: "b", Enabled: false},
				},
			},
			{
				Name: "staging",
				Hosts: []Host{
					{Domain: "c.com", IP: "192.168.1.1", Alias: "c", Enabled: true},
				},
			},
		},
	}

	hosts := cfg.GetAllHosts()
	assert.Len(t, hosts, 3)
	assert.Equal(t, "a.com", hosts[0].Domain)
	assert.Equal(t, "b.com", hosts[1].Domain)
	assert.Equal(t, "c.com", hosts[2].Domain)
}

func TestConfig_FindHostByAlias(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				Name: "dev",
				Hosts: []Host{
					{Domain: "example.com", IP: "127.0.0.1", Alias: "example", Enabled: true},
				},
			},
		},
	}

	t.Run("found", func(t *testing.T) {
		host, group := cfg.FindHostByAlias("example")
		require.NotNil(t, host)
		require.NotNil(t, group)
		assert.Equal(t, "example.com", host.Domain)
		assert.Equal(t, "dev", group.Name)
	})

	t.Run("not found", func(t *testing.T) {
		host, group := cfg.FindHostByAlias("nonexistent")
		assert.Nil(t, host)
		assert.Nil(t, group)
	})
}

func TestConfig_FindPreset(t *testing.T) {
	cfg := &Config{
		Presets: []Preset{
			{Name: "local", Enable: []string{"a"}, Disable: []string{"b"}},
			{Name: "staging", Enable: []string{"b"}, Disable: []string{"a"}},
		},
	}

	t.Run("found", func(t *testing.T) {
		preset := cfg.FindPreset("local")
		require.NotNil(t, preset)
		assert.Equal(t, "local", preset.Name)
		assert.Equal(t, []string{"a"}, preset.Enable)
	})

	t.Run("not found", func(t *testing.T) {
		preset := cfg.FindPreset("nonexistent")
		assert.Nil(t, preset)
	})
}

func TestConfig_SetHostEnabled(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				Name: "dev",
				Hosts: []Host{
					{Domain: "example.com", IP: "127.0.0.1", Alias: "example", Enabled: false},
				},
			},
		},
	}

	t.Run("enable existing", func(t *testing.T) {
		result := cfg.SetHostEnabled("example", true)
		assert.True(t, result)
		assert.True(t, cfg.Groups[0].Hosts[0].Enabled)
	})

	t.Run("disable existing", func(t *testing.T) {
		result := cfg.SetHostEnabled("example", false)
		assert.True(t, result)
		assert.False(t, cfg.Groups[0].Hosts[0].Enabled)
	})

	t.Run("nonexistent alias", func(t *testing.T) {
		result := cfg.SetHostEnabled("nonexistent", true)
		assert.False(t, result)
	})
}

func TestConfig_ApplyPreset(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				Name: "dev",
				Hosts: []Host{
					{Domain: "a.com", IP: "127.0.0.1", Alias: "a", Enabled: false},
					{Domain: "b.com", IP: "127.0.0.1", Alias: "b", Enabled: true},
				},
			},
		},
		Presets: []Preset{
			{Name: "swap", Enable: []string{"a"}, Disable: []string{"b"}},
		},
	}

	t.Run("valid preset", func(t *testing.T) {
		err := cfg.ApplyPreset("swap")
		require.NoError(t, err)
		assert.True(t, cfg.Groups[0].Hosts[0].Enabled)
		assert.False(t, cfg.Groups[0].Hosts[1].Enabled)
	})

	t.Run("nonexistent preset", func(t *testing.T) {
		err := cfg.ApplyPreset("nonexistent")
		assert.Error(t, err)
	})
}

func TestManager_LoadAndGet(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
settings:
  autoApply: true
  flushMethod: auto
groups:
  - name: development
    hosts:
      - domain: example.com
        ip: 127.0.0.1
        alias: example-local
        enabled: true
presets:
  - name: local
    enable: [example-local]
    disable: []
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	manager := NewManager(configPath)
	err = manager.Load()
	require.NoError(t, err)

	cfg := manager.Get()
	require.NotNil(t, cfg)

	assert.True(t, cfg.Settings.AutoApply)
	assert.Equal(t, FlushMethodAuto, cfg.Settings.FlushMethod)
	assert.Len(t, cfg.Groups, 1)
	assert.Equal(t, "development", cfg.Groups[0].Name)
	assert.Len(t, cfg.Groups[0].Hosts, 1)
	assert.Equal(t, "example.com", cfg.Groups[0].Hosts[0].Domain)
}

func TestManager_Save(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create initial config
	err := CreateDefault(configPath)
	require.NoError(t, err)

	// Load and modify
	manager := NewManager(configPath)
	err = manager.Load()
	require.NoError(t, err)

	cfg := manager.Get()
	cfg.Groups[0].Hosts[0].Enabled = true

	// Save
	err = manager.Save()
	require.NoError(t, err)

	// Reload and verify
	manager2 := NewManager(configPath)
	err = manager2.Load()
	require.NoError(t, err)

	cfg2 := manager2.Get()
	assert.True(t, cfg2.Groups[0].Hosts[0].Enabled)
}

func TestCreateDefault(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "config.yaml")

	err := CreateDefault(configPath)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Verify content is valid
	manager := NewManager(configPath)
	err = manager.Load()
	require.NoError(t, err)

	cfg := manager.Get()
	require.NotNil(t, cfg)
	assert.True(t, cfg.Settings.AutoApply)
	assert.Len(t, cfg.Groups, 1)
	assert.Len(t, cfg.Presets, 2)
}

func TestManager_Load_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configPath, []byte("invalid: yaml: content:"), 0644)
	require.NoError(t, err)

	manager := NewManager(configPath)
	err = manager.Load()
	assert.Error(t, err)
}

func TestManager_Load_FileNotFound(t *testing.T) {
	manager := NewManager("/nonexistent/path/config.yaml")
	err := manager.Load()
	assert.Error(t, err)
}

func TestFlushMethod(t *testing.T) {
	methods := []FlushMethod{
		FlushMethodAuto,
		FlushMethodDscacheutil,
		FlushMethodKillall,
		FlushMethodBoth,
	}

	for _, m := range methods {
		t.Run(string(m), func(t *testing.T) {
			assert.NotEmpty(t, string(m))
		})
	}
}
