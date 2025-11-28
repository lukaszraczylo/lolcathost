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

func TestDefaultConfigDir(t *testing.T) {
	dir := DefaultConfigDir()
	assert.NotEmpty(t, dir)
	assert.Contains(t, dir, ".config/lolcathost")
}

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()
	assert.NotEmpty(t, path)
	assert.Contains(t, path, "config.yaml")
}

func TestConfig_GenerateAlias(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				Name: "dev",
				Hosts: []Host{
					{Domain: "existing.com", IP: "127.0.0.1", Alias: "existing-com", Enabled: true},
				},
			},
		},
	}

	t.Run("simple domain", func(t *testing.T) {
		alias := cfg.GenerateAlias("newdomain.com")
		assert.Equal(t, "newdomain-com", alias)
	})

	t.Run("domain with underscore", func(t *testing.T) {
		alias := cfg.GenerateAlias("my_app.test")
		assert.Equal(t, "my-app-test", alias)
	})

	t.Run("duplicate generates numbered alias", func(t *testing.T) {
		alias := cfg.GenerateAlias("existing.com")
		assert.Equal(t, "existing-com-2", alias)
	})
}

func TestConfig_AddHost(t *testing.T) {
	t.Run("add to existing group", func(t *testing.T) {
		cfg := &Config{
			Groups: []Group{
				{Name: "dev", Hosts: []Host{}},
			},
		}
		err := cfg.AddHost("test.local", "127.0.0.1", "test-local", "dev", true)
		require.NoError(t, err)
		assert.Len(t, cfg.Groups[0].Hosts, 1)
		assert.Equal(t, "test.local", cfg.Groups[0].Hosts[0].Domain)
	})

	t.Run("add to new group", func(t *testing.T) {
		cfg := &Config{Groups: []Group{}}
		err := cfg.AddHost("test.local", "127.0.0.1", "test-local", "newgroup", true)
		require.NoError(t, err)
		assert.Len(t, cfg.Groups, 1)
		assert.Equal(t, "newgroup", cfg.Groups[0].Name)
	})

	t.Run("auto-generate alias", func(t *testing.T) {
		cfg := &Config{Groups: []Group{}}
		err := cfg.AddHost("auto.test", "127.0.0.1", "", "dev", true)
		require.NoError(t, err)
		assert.Equal(t, "auto-test", cfg.Groups[0].Hosts[0].Alias)
	})

	t.Run("duplicate alias error", func(t *testing.T) {
		cfg := &Config{
			Groups: []Group{
				{Name: "dev", Hosts: []Host{{Domain: "a.com", IP: "127.0.0.1", Alias: "existing"}}},
			},
		}
		err := cfg.AddHost("b.com", "127.0.0.1", "existing", "dev", true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "alias already exists")
	})
}

func TestConfig_AddGroup(t *testing.T) {
	t.Run("add new group", func(t *testing.T) {
		cfg := &Config{Groups: []Group{}}
		err := cfg.AddGroup("newgroup")
		require.NoError(t, err)
		assert.Len(t, cfg.Groups, 1)
		assert.Equal(t, "newgroup", cfg.Groups[0].Name)
	})

	t.Run("duplicate group error", func(t *testing.T) {
		cfg := &Config{Groups: []Group{{Name: "existing"}}}
		err := cfg.AddGroup("existing")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "group already exists")
	})
}

func TestConfig_DeleteGroup(t *testing.T) {
	t.Run("delete existing group", func(t *testing.T) {
		cfg := &Config{Groups: []Group{{Name: "todelete"}, {Name: "keep"}}}
		err := cfg.DeleteGroup("todelete")
		require.NoError(t, err)
		assert.Len(t, cfg.Groups, 1)
		assert.Equal(t, "keep", cfg.Groups[0].Name)
	})

	t.Run("delete nonexistent group", func(t *testing.T) {
		cfg := &Config{Groups: []Group{}}
		err := cfg.DeleteGroup("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "group not found")
	})
}

func TestConfig_RenameGroup(t *testing.T) {
	t.Run("rename existing group", func(t *testing.T) {
		cfg := &Config{Groups: []Group{{Name: "oldname"}}}
		err := cfg.RenameGroup("oldname", "newname")
		require.NoError(t, err)
		assert.Equal(t, "newname", cfg.Groups[0].Name)
	})

	t.Run("rename to existing name error", func(t *testing.T) {
		cfg := &Config{Groups: []Group{{Name: "a"}, {Name: "b"}}}
		err := cfg.RenameGroup("a", "b")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "group already exists")
	})

	t.Run("rename nonexistent group", func(t *testing.T) {
		cfg := &Config{Groups: []Group{}}
		err := cfg.RenameGroup("nonexistent", "newname")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "group not found")
	})
}

func TestConfig_GetGroups(t *testing.T) {
	cfg := &Config{Groups: []Group{{Name: "a"}, {Name: "b"}, {Name: "c"}}}
	groups := cfg.GetGroups()
	assert.Equal(t, []string{"a", "b", "c"}, groups)
}

func TestConfig_DeleteHost(t *testing.T) {
	t.Run("delete existing host", func(t *testing.T) {
		cfg := &Config{
			Groups: []Group{
				{Name: "dev", Hosts: []Host{
					{Domain: "a.com", Alias: "a"},
					{Domain: "b.com", Alias: "b"},
				}},
			},
		}
		result := cfg.DeleteHost("a")
		assert.True(t, result)
		assert.Len(t, cfg.Groups[0].Hosts, 1)
		assert.Equal(t, "b", cfg.Groups[0].Hosts[0].Alias)
	})

	t.Run("delete nonexistent host", func(t *testing.T) {
		cfg := &Config{Groups: []Group{}}
		result := cfg.DeleteHost("nonexistent")
		assert.False(t, result)
	})
}

func TestConfig_UpdateHost(t *testing.T) {
	t.Run("update in same group", func(t *testing.T) {
		cfg := &Config{
			Groups: []Group{
				{Name: "dev", Hosts: []Host{{Domain: "old.com", IP: "127.0.0.1", Alias: "test"}}},
			},
		}
		err := cfg.UpdateHost("test", "new.com", "192.168.1.1", "test", "dev")
		require.NoError(t, err)
		assert.Equal(t, "new.com", cfg.Groups[0].Hosts[0].Domain)
		assert.Equal(t, "192.168.1.1", cfg.Groups[0].Hosts[0].IP)
	})

	t.Run("move to different group", func(t *testing.T) {
		cfg := &Config{
			Groups: []Group{
				{Name: "source", Hosts: []Host{{Domain: "a.com", IP: "127.0.0.1", Alias: "test"}}},
				{Name: "target", Hosts: []Host{}},
			},
		}
		err := cfg.UpdateHost("test", "a.com", "127.0.0.1", "test", "target")
		require.NoError(t, err)
		assert.Len(t, cfg.Groups[0].Hosts, 0)
		assert.Len(t, cfg.Groups[1].Hosts, 1)
	})

	t.Run("move to new group", func(t *testing.T) {
		cfg := &Config{
			Groups: []Group{
				{Name: "source", Hosts: []Host{{Domain: "a.com", IP: "127.0.0.1", Alias: "test"}}},
			},
		}
		err := cfg.UpdateHost("test", "a.com", "127.0.0.1", "test", "newgroup")
		require.NoError(t, err)
		assert.Len(t, cfg.Groups, 2)
		assert.Equal(t, "newgroup", cfg.Groups[1].Name)
	})

	t.Run("change alias", func(t *testing.T) {
		cfg := &Config{
			Groups: []Group{
				{Name: "dev", Hosts: []Host{{Domain: "a.com", IP: "127.0.0.1", Alias: "old"}}},
			},
		}
		err := cfg.UpdateHost("old", "a.com", "127.0.0.1", "new", "dev")
		require.NoError(t, err)
		assert.Equal(t, "new", cfg.Groups[0].Hosts[0].Alias)
	})

	t.Run("alias conflict error", func(t *testing.T) {
		cfg := &Config{
			Groups: []Group{
				{Name: "dev", Hosts: []Host{
					{Domain: "a.com", Alias: "a"},
					{Domain: "b.com", Alias: "b"},
				}},
			},
		}
		err := cfg.UpdateHost("a", "a.com", "127.0.0.1", "b", "dev")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "alias already exists")
	})

	t.Run("host not found error", func(t *testing.T) {
		cfg := &Config{Groups: []Group{}}
		err := cfg.UpdateHost("nonexistent", "a.com", "127.0.0.1", "new", "dev")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "alias not found")
	})
}

func TestConfig_AddPreset(t *testing.T) {
	t.Run("add new preset", func(t *testing.T) {
		cfg := &Config{Presets: []Preset{}}
		err := cfg.AddPreset("newpreset", []string{"a"}, []string{"b"})
		require.NoError(t, err)
		assert.Len(t, cfg.Presets, 1)
		assert.Equal(t, "newpreset", cfg.Presets[0].Name)
	})

	t.Run("duplicate preset error", func(t *testing.T) {
		cfg := &Config{Presets: []Preset{{Name: "existing"}}}
		err := cfg.AddPreset("existing", nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "preset already exists")
	})
}

func TestConfig_DeletePreset(t *testing.T) {
	t.Run("delete existing preset", func(t *testing.T) {
		cfg := &Config{Presets: []Preset{{Name: "todelete"}, {Name: "keep"}}}
		err := cfg.DeletePreset("todelete")
		require.NoError(t, err)
		assert.Len(t, cfg.Presets, 1)
		assert.Equal(t, "keep", cfg.Presets[0].Name)
	})

	t.Run("delete nonexistent preset", func(t *testing.T) {
		cfg := &Config{Presets: []Preset{}}
		err := cfg.DeletePreset("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "preset not found")
	})
}

func TestConfig_GetPresets(t *testing.T) {
	cfg := &Config{Presets: []Preset{{Name: "a"}, {Name: "b"}}}
	presets := cfg.GetPresets()
	assert.Len(t, presets, 2)
}

func TestConfig_EnsureDefaultGroup(t *testing.T) {
	t.Run("creates default when empty", func(t *testing.T) {
		cfg := &Config{Groups: []Group{}}
		cfg.EnsureDefaultGroup()
		assert.Len(t, cfg.Groups, 1)
		assert.Equal(t, "default", cfg.Groups[0].Name)
	})

	t.Run("does nothing when groups exist", func(t *testing.T) {
		cfg := &Config{Groups: []Group{{Name: "existing"}}}
		cfg.EnsureDefaultGroup()
		assert.Len(t, cfg.Groups, 1)
		assert.Equal(t, "existing", cfg.Groups[0].Name)
	})
}

func TestManager_Watch(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	err := CreateDefault(configPath)
	require.NoError(t, err)

	manager := NewManager(configPath)
	err = manager.Load()
	require.NoError(t, err)

	changeCh := make(chan *Config, 1)
	err = manager.Watch(func(cfg *Config) {
		changeCh <- cfg
	})
	require.NoError(t, err)

	// Stop the watcher
	manager.Stop()
}

func TestManager_Save_NoConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	manager := NewManager(configPath)
	// Don't load, so config is nil
	err := manager.Save()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no config loaded")
}
