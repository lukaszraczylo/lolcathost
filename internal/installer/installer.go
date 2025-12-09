// Package installer handles installation and uninstallation of the lolcathost daemon.
package installer

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/lukaszraczylo/lolcathost/internal/config"
)

const (
	// GroupName is the name of the lolcathost group.
	GroupName = "lolcathost"
	// GroupGID is the GID for the lolcathost group (macOS).
	GroupGID = 850

	// Paths
	LogDir          = "/var/log/lolcathost"
	BackupDir       = "/var/backups/lolcathost"
	SocketPath      = "/var/run/lolcathost.sock"
	LaunchDaemonDir = "/Library/LaunchDaemons"
	SystemdDir      = "/etc/systemd/system"
)

// LaunchDaemonPlist is the macOS LaunchDaemon plist template.
const LaunchDaemonPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.lolcathost.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>--daemon</string>
        <string>--config</string>
        <string>/etc/lolcathost/config.yaml</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/var/log/lolcathost/daemon.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/lolcathost/daemon.err</string>
</dict>
</plist>
`

// SystemdUnit is the Linux systemd unit template.
const SystemdUnit = `[Unit]
Description=lolcathost - Dynamic Host Management Daemon
After=network.target

[Service]
Type=simple
ExecStart=%s --daemon --config /etc/lolcathost/config.yaml
Restart=always
RestartSec=5
User=root
Group=root

[Install]
WantedBy=multi-user.target
`

// Installer handles installation and uninstallation.
type Installer struct {
	binaryPath string
	verbose    bool
}

// New creates a new installer.
func New() (*Installer, error) {
	binaryPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks
	binaryPath, err = filepath.EvalSymlinks(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve executable path: %w", err)
	}

	return &Installer{
		binaryPath: binaryPath,
		verbose:    true,
	}, nil
}

// Install performs the full installation.
func (i *Installer) Install() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("--install requires sudo")
	}

	i.log("Installing lolcathost...")

	// Create group
	if err := i.createGroup(); err != nil {
		return fmt.Errorf("failed to create group: %w", err)
	}

	// Add current user to group
	if err := i.addCurrentUserToGroup(); err != nil {
		return fmt.Errorf("failed to add user to group: %w", err)
	}

	// Create directories
	if err := i.createDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Create system config for daemon
	if err := i.createSystemConfig(); err != nil {
		return fmt.Errorf("failed to create system config: %w", err)
	}

	// Install service
	if runtime.GOOS == "darwin" {
		if err := i.installLaunchDaemon(); err != nil {
			return fmt.Errorf("failed to install LaunchDaemon: %w", err)
		}
	} else if runtime.GOOS == "linux" {
		if err := i.installSystemdService(); err != nil {
			return fmt.Errorf("failed to install systemd service: %w", err)
		}
	}

	// Create default config for the invoking user
	if err := i.createDefaultConfig(); err != nil {
		i.log("Warning: failed to create default config: %v", err)
	}

	i.log("")
	i.log("✓ Installed successfully!")
	i.log("")
	i.log("Next steps:")
	i.log("  1. Open a NEW terminal (for group membership to take effect)")
	i.log("  2. Run 'lolcathost' to start the TUI")
	i.log("")

	return nil
}

// Uninstall removes the installation.
func (i *Installer) Uninstall() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("--uninstall requires sudo")
	}

	i.log("Uninstalling lolcathost...")

	// Stop and remove service
	if runtime.GOOS == "darwin" {
		i.uninstallLaunchDaemon()
	} else if runtime.GOOS == "linux" {
		i.uninstallSystemdService()
	}

	// Remove socket
	_ = os.Remove(SocketPath)

	// Note: We don't remove the group, logs, or backups
	// The user may want to keep these

	i.log("")
	i.log("✓ Uninstalled successfully!")
	i.log("")
	i.log("Note: Log files, backups, and the group were preserved.")
	i.log("To fully remove, manually delete:")
	i.log("  - /var/log/lolcathost/")
	i.log("  - /var/backups/lolcathost/")
	i.log("  - ~/.config/lolcathost/")
	if runtime.GOOS == "darwin" {
		i.log("  - Remove group: sudo dscl . -delete /Groups/%s", GroupName)
	} else {
		i.log("  - Remove group: sudo groupdel %s", GroupName)
	}
	i.log("")

	return nil
}

func (i *Installer) log(format string, args ...any) {
	if i.verbose {
		fmt.Printf(format+"\n", args...)
	}
}

func (i *Installer) createGroup() error {
	switch runtime.GOOS {
	case "darwin":
		return i.createGroupDarwin()
	case "linux":
		return i.createGroupLinux()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func (i *Installer) createGroupDarwin() error {
	// Check if group exists
	if _, err := exec.Command("dscl", ".", "-read", "/Groups/"+GroupName).Output(); err == nil {
		i.log("  Group '%s' already exists", GroupName)
		return nil
	}

	i.log("  Creating group '%s' (GID %d)...", GroupName, GroupGID)

	// Create group
	cmds := [][]string{
		{"dscl", ".", "-create", "/Groups/" + GroupName},
		{"dscl", ".", "-create", "/Groups/" + GroupName, "PrimaryGroupID", strconv.Itoa(GroupGID)},
		{"dscl", ".", "-create", "/Groups/" + GroupName, "RealName", "lolcathost users"},
	}

	for _, args := range cmds {
		// #nosec G204 -- args are hardcoded dscl commands with the constant GroupName
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			return fmt.Errorf("command %v failed: %w", args, err)
		}
	}

	return nil
}

func (i *Installer) createGroupLinux() error {
	// Check if group exists
	if _, err := exec.Command("getent", "group", GroupName).Output(); err == nil {
		i.log("  Group '%s' already exists", GroupName)
		return nil
	}

	i.log("  Creating group '%s'...", GroupName)

	if err := exec.Command("groupadd", "-r", GroupName).Run(); err != nil {
		return fmt.Errorf("groupadd failed: %w", err)
	}

	return nil
}

func (i *Installer) addCurrentUserToGroup() error {
	// Get the real user (not root)
	username := os.Getenv("SUDO_USER")
	if username == "" {
		// Fall back to current user
		u, err := user.Current()
		if err != nil {
			return fmt.Errorf("failed to get current user: %w", err)
		}
		username = u.Username
	}

	if username == "root" {
		i.log("  Skipping adding root to group")
		return nil
	}

	switch runtime.GOOS {
	case "darwin":
		return i.addUserToGroupDarwin(username)
	case "linux":
		return i.addUserToGroupLinux(username)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func (i *Installer) addUserToGroupDarwin(username string) error {
	// Check if user is already in group
	output, err := exec.Command("dscl", ".", "-read", "/Groups/"+GroupName, "GroupMembership").Output()
	if err == nil && strings.Contains(string(output), username) {
		i.log("  User '%s' already in group '%s'", username, GroupName)
		return nil
	}

	i.log("  Adding user '%s' to group '%s'...", username, GroupName)

	if err := exec.Command("dscl", ".", "-append", "/Groups/"+GroupName, "GroupMembership", username).Run(); err != nil {
		return fmt.Errorf("failed to add user to group: %w", err)
	}

	return nil
}

func (i *Installer) addUserToGroupLinux(username string) error {
	// Check if user is already in group
	output, err := exec.Command("id", "-nG", username).Output()
	if err == nil && strings.Contains(string(output), GroupName) {
		i.log("  User '%s' already in group '%s'", username, GroupName)
		return nil
	}

	i.log("  Adding user '%s' to group '%s'...", username, GroupName)

	if err := exec.Command("usermod", "-aG", GroupName, username).Run(); err != nil {
		return fmt.Errorf("failed to add user to group: %w", err)
	}

	return nil
}

func (i *Installer) createDirectories() error {
	dirs := []string{LogDir, BackupDir, config.SystemConfigDir}

	for _, dir := range dirs {
		i.log("  Creating directory '%s'...", dir)
		// #nosec G301 -- system directories should be world-readable
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	return nil
}

func (i *Installer) createSystemConfig() error {
	// Check if system config already exists
	if _, err := os.Stat(config.SystemConfigPath); err == nil {
		i.log("  System config already exists at %s", config.SystemConfigPath)
		return nil
	}

	i.log("  Creating system config at %s...", config.SystemConfigPath)
	return config.CreateDefault(config.SystemConfigPath)
}

func (i *Installer) installLaunchDaemon() error {
	plistPath := filepath.Join(LaunchDaemonDir, "com.lolcathost.daemon.plist")
	plistContent := fmt.Sprintf(LaunchDaemonPlist, i.binaryPath)

	// Unload if already loaded (do this before writing plist)
	i.log("  Stopping existing daemon if running...")
	_ = exec.Command("launchctl", "bootout", "system/com.lolcathost.daemon").Run()

	// Give launchd time to fully unload the service
	time.Sleep(500 * time.Millisecond)

	// Remove old plist to ensure clean state
	_ = os.Remove(plistPath)

	i.log("  Writing LaunchDaemon plist...")
	// #nosec G306 -- plist files are world-readable by convention
	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	// Bootstrap the daemon
	i.log("  Starting daemon...")
	// #nosec G204 -- plistPath is constructed from constant LaunchDaemonDir
	cmd := exec.Command("launchctl", "bootstrap", "system", plistPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Exit code 5 means "service already loaded" - try kickstart instead
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 5 {
			i.log("  Service already registered, restarting...")
			if err := exec.Command("launchctl", "kickstart", "-k", "system/com.lolcathost.daemon").Run(); err != nil {
				return fmt.Errorf("failed to restart daemon: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to bootstrap daemon: %w (output: %s)", err, string(output))
	}

	return nil
}

func (i *Installer) uninstallLaunchDaemon() {
	plistPath := filepath.Join(LaunchDaemonDir, "com.lolcathost.daemon.plist")

	i.log("  Stopping daemon...")
	_ = exec.Command("launchctl", "bootout", "system/com.lolcathost.daemon").Run()

	i.log("  Removing LaunchDaemon plist...")
	_ = os.Remove(plistPath)
}

func (i *Installer) installSystemdService() error {
	unitPath := filepath.Join(SystemdDir, "lolcathost.service")
	unitContent := fmt.Sprintf(SystemdUnit, i.binaryPath)

	i.log("  Writing systemd unit...")
	// #nosec G306 -- systemd unit files are world-readable by convention
	if err := os.WriteFile(unitPath, []byte(unitContent), 0644); err != nil {
		return fmt.Errorf("failed to write unit file: %w", err)
	}

	// Reload systemd
	i.log("  Reloading systemd...")
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	// Enable and start the service
	i.log("  Enabling and starting service...")
	if err := exec.Command("systemctl", "enable", "--now", "lolcathost.service").Run(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	return nil
}

func (i *Installer) uninstallSystemdService() {
	i.log("  Stopping and disabling service...")
	_ = exec.Command("systemctl", "disable", "--now", "lolcathost.service").Run()

	i.log("  Removing systemd unit...")
	_ = os.Remove(filepath.Join(SystemdDir, "lolcathost.service"))

	_ = exec.Command("systemctl", "daemon-reload").Run()
}

func (i *Installer) createDefaultConfig() error {
	// Config is stored at /etc/lolcathost/config.yaml and managed by the daemon.
	// The daemon creates a default config if none exists when it starts.
	// No user-level config is created to avoid confusion with two config files.
	configPath := "/etc/lolcathost/config.yaml"

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		i.log("  Config already exists at %s", configPath)
		return nil
	}

	// Create config directory
	configDir := filepath.Dir(configPath)
	// #nosec G301 -- config directory should be world-readable
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	i.log("  Creating default config at %s...", configPath)

	if err := config.CreateDefault(configPath); err != nil {
		return err
	}

	return nil
}

// CheckInstallation checks if the daemon is properly installed.
func CheckInstallation() error {
	// Check if socket exists
	if _, err := os.Stat(SocketPath); os.IsNotExist(err) {
		return fmt.Errorf("daemon not running (socket not found)")
	}

	// Check if user is in group
	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	groups, err := u.GroupIds()
	if err != nil {
		return fmt.Errorf("failed to get user groups: %w", err)
	}

	inGroup := false
	for _, gid := range groups {
		g, err := user.LookupGroupId(gid)
		if err != nil {
			continue
		}
		if g.Name == GroupName {
			inGroup = true
			break
		}
	}

	if !inGroup {
		return fmt.Errorf("user '%s' is not in group '%s'. Run 'sudo lolcathost --install' and open a new terminal", u.Username, GroupName)
	}

	return nil
}
