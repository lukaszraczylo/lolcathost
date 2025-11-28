// Package main provides the entry point for the lolcathost application.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/lukaszraczylo/lolcathost/internal/client"
	"github.com/lukaszraczylo/lolcathost/internal/config"
	"github.com/lukaszraczylo/lolcathost/internal/daemon"
	"github.com/lukaszraczylo/lolcathost/internal/installer"
	"github.com/lukaszraczylo/lolcathost/internal/protocol"
	"github.com/lukaszraczylo/lolcathost/internal/tui"
	"github.com/lukaszraczylo/lolcathost/internal/version"
)

// version is set at compile time via ldflags
var appVersion = "dev"

const (
	githubOwner = "lukaszraczylo"
	githubRepo  = "lolcathost"
)

func main() {
	// Flags
	daemonMode := flag.Bool("daemon", false, "Run as daemon (called by LaunchDaemon/systemd)")
	installFlag := flag.Bool("install", false, "Install the daemon service (requires sudo)")
	uninstallFlag := flag.Bool("uninstall", false, "Uninstall the daemon service (requires sudo)")
	versionFlag := flag.Bool("version", false, "Show version")
	updateFlag := flag.Bool("update", false, "Check for updates")
	configPath := flag.String("config", config.DefaultConfigPath(), "Path to config file")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "lolcathost - Dynamic Host Management\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  lolcathost                  Launch TUI\n")
		fmt.Fprintf(os.Stderr, "  lolcathost list             List all entries\n")
		fmt.Fprintf(os.Stderr, "  lolcathost on <alias>       Enable entry\n")
		fmt.Fprintf(os.Stderr, "  lolcathost off <alias>      Disable entry\n")
		fmt.Fprintf(os.Stderr, "  lolcathost preset <name>    Apply preset\n")
		fmt.Fprintf(os.Stderr, "  lolcathost status           Show daemon status\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Installation:\n")
		fmt.Fprintf(os.Stderr, "  sudo lolcathost --install   Install daemon\n")
		fmt.Fprintf(os.Stderr, "  sudo lolcathost --uninstall Uninstall daemon\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Version
	if *versionFlag {
		fmt.Printf("lolcathost version %s\n", appVersion)
		os.Exit(0)
	}

	// Update check
	if *updateFlag {
		checkForUpdates()
		os.Exit(0)
	}

	// Install/Uninstall
	if *installFlag {
		runInstall()
		return
	}

	if *uninstallFlag {
		runUninstall()
		return
	}

	// Daemon mode
	if *daemonMode {
		runDaemon(*configPath)
		return
	}

	// Parse subcommand
	args := flag.Args()
	if len(args) == 0 {
		// No subcommand - launch TUI
		runTUI(*configPath)
		return
	}

	// Handle subcommands
	switch args[0] {
	case "list":
		runList()
	case "on":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: lolcathost on <alias>")
			os.Exit(1)
		}
		runOn(args[1])
	case "off":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: lolcathost off <alias>")
			os.Exit(1)
		}
		runOff(args[1])
	case "preset":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: lolcathost preset <name>")
			os.Exit(1)
		}
		runPreset(args[1])
	case "status":
		runStatus()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		flag.Usage()
		os.Exit(1)
	}
}

func runInstall() {
	inst, err := installer.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := inst.Install(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runUninstall() {
	inst, err := installer.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := inst.Uninstall(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runDaemon(configPath string) {
	daemon.Version = appVersion
	d, err := daemon.New(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create daemon: %v\n", err)
		os.Exit(1)
	}

	if err := d.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Daemon error: %v\n", err)
		os.Exit(1)
	}
}

func runTUI(configPath string) {
	// Check installation
	if err := installer.CheckInstallation(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintln(os.Stderr, "\nTo install, run: sudo lolcathost --install")
		os.Exit(1)
	}

	if err := tui.RunWithVersion(protocol.SocketPath, configPath, appVersion, githubOwner, githubRepo); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runList() {
	c := connectClient()
	defer c.Close()

	entries, err := c.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(entries) == 0 {
		fmt.Println("No entries configured.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tDOMAIN\tIP\tALIAS\tGROUP")
	fmt.Fprintln(w, "------\t------\t--\t-----\t-----")

	for _, e := range entries {
		status := "○"
		if e.Enabled {
			status = "●"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", status, e.Domain, e.IP, e.Alias, e.Group)
	}

	w.Flush()
}

func runOn(alias string) {
	c := connectClient()
	defer c.Close()

	data, err := c.Enable(alias)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Enabled: %s → %s\n", alias, data.Domain)
}

func runOff(alias string) {
	c := connectClient()
	defer c.Close()

	data, err := c.Disable(alias)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Disabled: %s → %s\n", alias, data.Domain)
}

func runPreset(name string) {
	c := connectClient()
	defer c.Close()

	if err := c.ApplyPreset(name); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Applied preset: %s\n", name)
}

func runStatus() {
	c := connectClient()
	defer c.Close()

	status, err := c.Status()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Status: %s\n", greenIf("running", status.Running))
	fmt.Printf("Version: %s\n", status.Version)
	fmt.Printf("Uptime: %d seconds\n", status.Uptime)
	fmt.Printf("Active entries: %d\n", status.ActiveCount)
	fmt.Printf("Total requests: %d\n", status.RequestCount)
}

func connectClient() *client.Client {
	// Check installation first
	if err := installer.CheckInstallation(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintln(os.Stderr, "\nTo install, run: sudo lolcathost --install")
		os.Exit(1)
	}

	c := client.New(protocol.SocketPath)
	if err := c.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to daemon: %v\n", err)
		os.Exit(1)
	}

	return c
}

func greenIf(s string, condition bool) string {
	if condition {
		return "\033[32m" + s + "\033[0m"
	}
	return "\033[31mnot " + s + "\033[0m"
}

func checkForUpdates() {
	fmt.Printf("lolcathost version %s\n", appVersion)
	fmt.Println("Checking for updates...")

	checker := version.NewChecker(githubOwner, githubRepo, appVersion)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	update := checker.CheckForUpdate(ctx)
	if update == nil {
		fmt.Println("You are running the latest version.")
		return
	}

	fmt.Printf("\n\033[32mUpdate available: v%s\033[0m\n", update.LatestVersion)
	fmt.Printf("Download: %s\n", update.ReleaseURL)
	fmt.Println("\nTo update, download the latest release from the URL above")
	fmt.Println("or use your package manager (e.g., 'brew upgrade lolcathost').")
}
