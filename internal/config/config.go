// Package config handles YAML configuration parsing and hot-reload.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// SystemConfigDir is the system-wide config directory for the daemon.
const SystemConfigDir = "/etc/lolcathost"

// SystemConfigPath is the system-wide config file path for the daemon.
const SystemConfigPath = "/etc/lolcathost/config.yaml"

// DefaultConfigDir returns the default config directory path for users.
func DefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "lolcathost")
}

// DefaultConfigPath returns the default config file path for users.
func DefaultConfigPath() string {
	return filepath.Join(DefaultConfigDir(), "config.yaml")
}

// FlushMethod defines DNS cache flush methods.
type FlushMethod string

const (
	FlushMethodAuto        FlushMethod = "auto"
	FlushMethodDscacheutil FlushMethod = "dscacheutil"
	FlushMethodKillall     FlushMethod = "killall"
	FlushMethodBoth        FlushMethod = "both"
)

// Settings holds global configuration settings.
type Settings struct {
	AutoApply   bool        `yaml:"autoApply"`
	FlushMethod FlushMethod `yaml:"flushMethod"`
}

// Host represents a single host entry in configuration.
type Host struct {
	Domain  string `yaml:"domain"`
	IP      string `yaml:"ip"`
	Alias   string `yaml:"alias"`
	Enabled bool   `yaml:"enabled"`
}

// Group represents a group of host entries.
type Group struct {
	Name  string `yaml:"name"`
	Hosts []Host `yaml:"hosts"`
}

// Preset defines a named preset that enables/disables specific aliases.
type Preset struct {
	Name    string   `yaml:"name"`
	Enable  []string `yaml:"enable,omitempty"`
	Disable []string `yaml:"disable,omitempty"`
}

// Config represents the complete configuration.
type Config struct {
	Settings Settings `yaml:"settings"`
	Groups   []Group  `yaml:"groups"`
	Presets  []Preset `yaml:"presets"`
}

// Manager handles configuration loading and watching.
type Manager struct {
	path     string
	config   *Config
	mu       sync.RWMutex
	watcher  *fsnotify.Watcher
	onChange func(*Config)
	stopCh   chan struct{}
}

// NewManager creates a new config manager.
func NewManager(path string) *Manager {
	return &Manager{
		path:   path,
		stopCh: make(chan struct{}),
	}
}

// Load reads and parses the configuration file.
func (m *Manager) Load() error {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := ValidateConfig(&cfg); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	m.mu.Lock()
	m.config = &cfg
	m.mu.Unlock()

	return nil
}

// Get returns the current configuration.
func (m *Manager) Get() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// Watch starts watching the config file for changes.
func (m *Manager) Watch(onChange func(*Config)) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	m.watcher = watcher
	m.onChange = onChange

	go m.watchLoop()

	if err := watcher.Add(m.path); err != nil {
		return fmt.Errorf("failed to watch config file: %w", err)
	}

	return nil
}

func (m *Manager) watchLoop() {
	for {
		select {
		case event, ok := <-m.watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				if err := m.Load(); err == nil && m.onChange != nil {
					m.onChange(m.Get())
				}
			}
		case <-m.watcher.Errors:
			// Ignore watcher errors
		case <-m.stopCh:
			return
		}
	}
}

// Stop stops watching the config file.
func (m *Manager) Stop() {
	close(m.stopCh)
	if m.watcher != nil {
		m.watcher.Close()
	}
}

// GetAllHosts returns all hosts from all groups.
func (c *Config) GetAllHosts() []Host {
	var hosts []Host
	for _, g := range c.Groups {
		hosts = append(hosts, g.Hosts...)
	}
	return hosts
}

// FindHostByAlias finds a host by its alias.
func (c *Config) FindHostByAlias(alias string) (*Host, *Group) {
	for i := range c.Groups {
		for j := range c.Groups[i].Hosts {
			if c.Groups[i].Hosts[j].Alias == alias {
				return &c.Groups[i].Hosts[j], &c.Groups[i]
			}
		}
	}
	return nil, nil
}

// FindPreset finds a preset by name.
func (c *Config) FindPreset(name string) *Preset {
	for i := range c.Presets {
		if c.Presets[i].Name == name {
			return &c.Presets[i]
		}
	}
	return nil
}

// SetHostEnabled sets the enabled state of a host by alias.
func (c *Config) SetHostEnabled(alias string, enabled bool) bool {
	for i := range c.Groups {
		for j := range c.Groups[i].Hosts {
			if c.Groups[i].Hosts[j].Alias == alias {
				c.Groups[i].Hosts[j].Enabled = enabled
				return true
			}
		}
	}
	return false
}

// GenerateAlias creates a unique alias from a domain name.
func (c *Config) GenerateAlias(domain string) string {
	// Convert domain to alias format: example.com -> example-com
	alias := strings.ReplaceAll(domain, ".", "-")
	alias = strings.ReplaceAll(alias, "_", "-")
	alias = strings.ToLower(alias)

	// Check if alias exists, if so append a number
	baseAlias := alias
	counter := 1
	for {
		if existing, _ := c.FindHostByAlias(alias); existing == nil {
			break
		}
		counter++
		alias = fmt.Sprintf("%s-%d", baseAlias, counter)
	}

	return alias
}

// AddHost adds a new host to the configuration.
func (c *Config) AddHost(domain, ip, alias, groupName string, enabled bool) error {
	// Auto-generate alias if empty
	if alias == "" {
		alias = c.GenerateAlias(domain)
	} else {
		// Check for duplicate alias
		if existing, _ := c.FindHostByAlias(alias); existing != nil {
			return fmt.Errorf("alias already exists: %s", alias)
		}
	}

	host := Host{
		Domain:  domain,
		IP:      ip,
		Alias:   alias,
		Enabled: enabled,
	}

	// Find or create group
	for i := range c.Groups {
		if c.Groups[i].Name == groupName {
			c.Groups[i].Hosts = append(c.Groups[i].Hosts, host)
			return nil
		}
	}

	// Create new group
	c.Groups = append(c.Groups, Group{
		Name:  groupName,
		Hosts: []Host{host},
	})
	return nil
}

// AddGroup adds a new empty group.
func (c *Config) AddGroup(name string) error {
	// Check if group already exists
	for _, g := range c.Groups {
		if g.Name == name {
			return fmt.Errorf("group already exists: %s", name)
		}
	}

	c.Groups = append(c.Groups, Group{
		Name:  name,
		Hosts: []Host{},
	})
	return nil
}

// DeleteGroup removes a group and all its hosts.
func (c *Config) DeleteGroup(name string) error {
	for i, g := range c.Groups {
		if g.Name == name {
			c.Groups = append(c.Groups[:i], c.Groups[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("group not found: %s", name)
}

// RenameGroup renames an existing group.
func (c *Config) RenameGroup(oldName, newName string) error {
	// Check if new name already exists
	for _, g := range c.Groups {
		if g.Name == newName {
			return fmt.Errorf("group already exists: %s", newName)
		}
	}

	for i := range c.Groups {
		if c.Groups[i].Name == oldName {
			c.Groups[i].Name = newName
			return nil
		}
	}
	return fmt.Errorf("group not found: %s", oldName)
}

// GetGroups returns all group names.
func (c *Config) GetGroups() []string {
	names := make([]string, len(c.Groups))
	for i, g := range c.Groups {
		names[i] = g.Name
	}
	return names
}

// DeleteHost removes a host by alias.
func (c *Config) DeleteHost(alias string) bool {
	for i := range c.Groups {
		for j := range c.Groups[i].Hosts {
			if c.Groups[i].Hosts[j].Alias == alias {
				c.Groups[i].Hosts = append(c.Groups[i].Hosts[:j], c.Groups[i].Hosts[j+1:]...)
				return true
			}
		}
	}
	return false
}

// UpdateHost updates an existing host by alias.
func (c *Config) UpdateHost(oldAlias, domain, ip, newAlias, groupName string) error {
	// Find the host
	var foundGroup int = -1
	var foundHost int = -1
	for i := range c.Groups {
		for j := range c.Groups[i].Hosts {
			if c.Groups[i].Hosts[j].Alias == oldAlias {
				foundGroup = i
				foundHost = j
				break
			}
		}
		if foundHost >= 0 {
			break
		}
	}

	if foundHost < 0 {
		return fmt.Errorf("alias not found: %s", oldAlias)
	}

	// Check for duplicate alias if alias is changing
	if oldAlias != newAlias {
		if existing, _ := c.FindHostByAlias(newAlias); existing != nil {
			return fmt.Errorf("alias already exists: %s", newAlias)
		}
	}

	// Get current enabled state
	enabled := c.Groups[foundGroup].Hosts[foundHost].Enabled

	// If group is changing, move to new group
	if c.Groups[foundGroup].Name != groupName {
		// Remove from old group
		c.Groups[foundGroup].Hosts = append(c.Groups[foundGroup].Hosts[:foundHost], c.Groups[foundGroup].Hosts[foundHost+1:]...)

		// Add to new group
		host := Host{
			Domain:  domain,
			IP:      ip,
			Alias:   newAlias,
			Enabled: enabled,
		}

		// Find or create target group
		found := false
		for i := range c.Groups {
			if c.Groups[i].Name == groupName {
				c.Groups[i].Hosts = append(c.Groups[i].Hosts, host)
				found = true
				break
			}
		}
		if !found {
			c.Groups = append(c.Groups, Group{
				Name:  groupName,
				Hosts: []Host{host},
			})
		}
	} else {
		// Update in place
		c.Groups[foundGroup].Hosts[foundHost].Domain = domain
		c.Groups[foundGroup].Hosts[foundHost].IP = ip
		c.Groups[foundGroup].Hosts[foundHost].Alias = newAlias
	}

	return nil
}

// ApplyPreset applies a preset to the configuration.
func (c *Config) ApplyPreset(name string) error {
	preset := c.FindPreset(name)
	if preset == nil {
		return fmt.Errorf("preset not found: %s", name)
	}

	for _, alias := range preset.Enable {
		c.SetHostEnabled(alias, true)
	}
	for _, alias := range preset.Disable {
		c.SetHostEnabled(alias, false)
	}
	return nil
}

// AddPreset adds a new preset.
func (c *Config) AddPreset(name string, enable, disable []string) error {
	// Check if preset already exists
	for _, p := range c.Presets {
		if p.Name == name {
			return fmt.Errorf("preset already exists: %s", name)
		}
	}

	c.Presets = append(c.Presets, Preset{
		Name:    name,
		Enable:  enable,
		Disable: disable,
	})
	return nil
}

// DeletePreset removes a preset by name.
func (c *Config) DeletePreset(name string) error {
	for i, p := range c.Presets {
		if p.Name == name {
			c.Presets = append(c.Presets[:i], c.Presets[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("preset not found: %s", name)
}

// GetPresets returns all presets.
func (c *Config) GetPresets() []Preset {
	return c.Presets
}

// EnsureDefaultGroup ensures at least one group exists, creating "default" if needed.
func (c *Config) EnsureDefaultGroup() {
	if len(c.Groups) == 0 {
		c.Groups = append(c.Groups, Group{
			Name:  "default",
			Hosts: []Host{},
		})
	}
}

// Save writes the configuration to the file.
func (m *Manager) Save() error {
	m.mu.RLock()
	cfg := m.config
	m.mu.RUnlock()

	if cfg == nil {
		return fmt.Errorf("no config loaded")
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(m.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// CreateDefault creates a default configuration file.
func CreateDefault(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	cfg := &Config{
		Settings: Settings{
			AutoApply:   true,
			FlushMethod: FlushMethodAuto,
		},
		Groups: []Group{
			{
				Name: "development",
				Hosts: []Host{
					{
						Domain:  "example.local",
						IP:      "127.0.0.1",
						Alias:   "example-local",
						Enabled: false,
					},
				},
			},
		},
		Presets: []Preset{
			{
				Name:    "local",
				Enable:  []string{"example-local"},
				Disable: []string{},
			},
			{
				Name:    "clear",
				Enable:  []string{},
				Disable: []string{"example-local"},
			},
		},
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write default config: %w", err)
	}

	return nil
}
