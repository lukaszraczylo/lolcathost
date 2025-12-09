// Package daemon provides the main daemon loop and lifecycle management.
package daemon

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lukaszraczylo/lolcathost/internal/config"
	"github.com/lukaszraczylo/lolcathost/internal/protocol"
)

// Daemon represents the lolcathost daemon.
type Daemon struct {
	server    *Server
	config    *config.Manager
	stopCh    chan struct{}
	cleanupCh chan struct{}
}

// New creates a new daemon instance.
func New(configPath string) (*Daemon, error) {
	cfgManager := config.NewManager(configPath)

	// Try to load config, create default if it doesn't exist
	if err := cfgManager.Load(); err != nil {
		if os.IsNotExist(err) {
			if err := config.CreateDefault(configPath); err != nil {
				return nil, fmt.Errorf("failed to create default config: %w", err)
			}
			if err := cfgManager.Load(); err != nil {
				return nil, fmt.Errorf("failed to load default config: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
	}

	// Ensure at least one group exists
	cfg := cfgManager.Get()
	if cfg != nil {
		cfg.EnsureDefaultGroup()
		// Save if we added a default group
		if len(cfg.Groups) == 1 && cfg.Groups[0].Name == "default" && len(cfg.Groups[0].Hosts) == 0 {
			_ = cfgManager.Save()
		}
	}

	server := NewServer(protocol.SocketPath, cfgManager)

	return &Daemon{
		server:    server,
		config:    cfgManager,
		stopCh:    make(chan struct{}),
		cleanupCh: make(chan struct{}),
	}, nil
}

// Run starts the daemon and blocks until stopped.
func (d *Daemon) Run() error {
	// Verify we're running as root
	if os.Geteuid() != 0 {
		return fmt.Errorf("daemon must run as root")
	}

	// Start the server
	if err := d.server.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Watch config for changes
	if err := d.config.Watch(d.onConfigChange); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to watch config: %v\n", err)
	}

	// Start cleanup goroutine
	go d.cleanupLoop()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		fmt.Println("Received shutdown signal")
	case <-d.stopCh:
		fmt.Println("Shutdown requested")
	}

	return d.shutdown()
}

// Stop signals the daemon to stop.
func (d *Daemon) Stop() {
	close(d.stopCh)
}

func (d *Daemon) shutdown() error {
	close(d.cleanupCh)
	d.config.Stop()

	if err := d.server.Stop(); err != nil {
		return fmt.Errorf("failed to stop server: %w", err)
	}

	return nil
}

func (d *Daemon) onConfigChange(cfg *config.Config) {
	fmt.Println("Config changed, syncing hosts file...")
	// The server will use the updated config on next request
	// We could trigger a sync here if autoApply is enabled
	if cfg != nil && cfg.Settings.AutoApply {
		// Sync hosts file with new config
		// This is handled by the server internally
	}
}

func (d *Daemon) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.server.rateLimiter.Cleanup()
		case <-d.cleanupCh:
			return
		}
	}
}
