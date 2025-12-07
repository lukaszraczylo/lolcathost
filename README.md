<p align="center">
  <img src="docs/lolcathost.png" alt="lolcathost logo" width="200">
</p>

<p align="center">
  <span style="font-size: 72px; font-weight: bold; background: linear-gradient(135deg, #f472b6 0%, #c084fc 100%); -webkit-background-clip: text; -webkit-text-fill-color: transparent;">lolcathost</span>
</p>

<p align="center">
  <a href="https://github.com/lukaszraczylo/lolcathost/releases"><img src="https://img.shields.io/github/v/release/lukaszraczylo/lolcathost" alt="Release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/lukaszraczylo/lolcathost" alt="License"></a>
  <a href="https://goreportcard.com/report/github.com/lukaszraczylo/lolcathost"><img src="https://goreportcard.com/badge/github.com/lukaszraczylo/lolcathost" alt="Go Report Card"></a>
</p>

<p align="center">
  <strong>Dynamic hosts file manager with interactive terminal UI</strong>
</p>

lolcathost manages your `/etc/hosts` file with an interactive terminal interface. It provides real-time management, automatic backups, group organization, presets, and a secure daemon-based architecture.

## Features

- **Interactive TUI** - Terminal interface with keyboard navigation
- **Live management** - Add, edit, and delete host entries without restarting
- **Groups** - Organize hosts into logical groups
- **Presets** - Save and apply preset configurations with a single command
- **Auto-backup** - Automatic backups before every change with rollback support
- **Secure daemon** - Privileged daemon handles file access via Unix socket IPC
- **Domain blocking** - Configurable blocklist to prevent dangerous entries
- **Cross-platform** - Works on macOS (LaunchDaemon) and Linux (systemd)
- **CLI & TUI** - Both command-line and interactive modes for flexibility
- **Auto-update check** - Notifies you when a new version is available

## Comparison with Other Tools

| Feature | lolcathost | [HostsMan](https://hostsfileman.github.io/) | [Gas Mask](https://github.com/2ndalpha/gasmask) | Manual editing |
|---------|------------|---------------------------------------------|------------------------------------------------|----------------|
| **Platform** | macOS/Linux | Windows | macOS only | All |
| **Interface** | Terminal TUI | Desktop GUI | Desktop GUI | Text editor |
| **Daemon architecture** | Yes (secure) | No | No | N/A |
| **Real-time sync** | Yes | No | Manual | Manual |
| **Groups** | Yes | Yes | Yes | Manual |
| **Presets** | Yes | Yes | Yes | No |
| **Auto-backup** | 10 rolling | Manual | Manual | No |
| **Rollback** | Yes | No | No | No |
| **CLI automation** | Yes | Limited | No | Yes |
| **Rate limiting** | Yes | No | No | N/A |
| **Domain blocking** | Yes | No | No | No |
| **Auto-update check** | Yes | No | No | N/A |

## Installation

### Homebrew (macOS)

```bash
brew install --cask lukaszraczylo/taps/lolcathost
```

> **Note**: If you previously installed via `brew install lukaszraczylo/taps/lolcathost` (formula), uninstall first:
> ```bash
> brew uninstall lolcathost
> ```

After Homebrew installation, run:

```bash
sudo lolcathost --install
```

### Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/lukaszraczylo/lolcathost/main/install.sh | bash
```

### Manual Download

Download binaries from the [releases page](https://github.com/lukaszraczylo/lolcathost/releases).

### Build from Source

```bash
git clone https://github.com/lukaszraczylo/lolcathost.git
cd lolcathost
make build
sudo ./build/lolcathost --install
```

### Post-Installation

The installer will:
- Install the binary to `/usr/local/bin/lolcathost`
- Create a LaunchDaemon (macOS) or systemd service (Linux)
- Start the daemon automatically
- Create the default config at `/etc/lolcathost/config.yaml`

## Quick Start

After installation, open a **new terminal** and run:

```bash
lolcathost
```

### Keyboard Controls

| Key | Action |
|-----|--------|
| `↑↓` / `j/k` | Navigate entries |
| `Space` / `Enter` | Toggle entry enabled/disabled |
| `n` | Add new host entry |
| `e` | Edit selected entry |
| `d` | Delete selected entry |
| `p` | Open preset picker |
| `g` | Open group manager |
| `/` | Search |
| `r` | Refresh list |
| `?` | Show help |
| `q` | Quit |

## Configuration

### Config File Location

The configuration is stored at `/etc/lolcathost/config.yaml` and managed by the daemon.

- **TUI/CLI changes**: All changes made through the TUI or CLI are automatically saved to this file
- **Manual editing**: To edit manually, use `sudo nano /etc/lolcathost/config.yaml` (changes are picked up automatically via hot-reload)

### Example Configuration

```yaml
# Groups for organizing host entries
groups:
  - name: development
    hosts:
      - domain: myapp.local
        ip: 127.0.0.1
        enabled: true
      - domain: api.myapp.local
        ip: 127.0.0.1
        enabled: true

  - name: staging
    hosts:
      - domain: staging.example.com
        ip: 192.168.1.100
        enabled: false

# Presets for quick configuration switching
presets:
  - name: work
    enable:
      - myapp-local
      - api-myapp-local
    disable:
      - staging-example-com

  - name: testing
    enable:
      - staging-example-com
    disable:
      - myapp-local

# Domain blocklist (prevent adding these domains)
blocklist:
  - google.com
  - facebook.com
  - github.com
```

### Host Entry Fields

| Field | Required | Description |
|-------|----------|-------------|
| `domain` | Yes | The hostname (e.g., myapp.local) |
| `ip` | Yes | IP address to resolve to |
| `enabled` | No | Whether entry is active (default: false) |

Note: Aliases are auto-generated from domain names (e.g., `myapp.local` becomes `myapp-local`).

## CLI Commands

```bash
lolcathost                  # Launch TUI
lolcathost list             # List all entries
lolcathost on <alias>       # Enable entry
lolcathost off <alias>      # Disable entry
lolcathost preset <name>    # Apply preset
lolcathost status           # Show daemon status
```

### Version & Updates

```bash
lolcathost --version        # Show current version
lolcathost --update         # Check for updates
```

### Installation Commands

```bash
sudo lolcathost --install   # Install daemon
sudo lolcathost --uninstall # Uninstall daemon
```

## Status Indicators

| Indicator | Description |
|-----------|-------------|
| `● Active` | Entry is enabled and in /etc/hosts |
| `○ Disabled` | Entry is disabled |
| `◐ Pending` | Operation in progress |
| `✗ Error` | Operation failed |

## Architecture

lolcathost uses a daemon-based architecture for security:

```
┌─────────────────┐         ┌─────────────────────┐
│   lolcathost    │  JSON   │      Daemon         │
│   CLI / TUI     │◄───────►│    (runs as root)   │
│  (runs as user) │  Unix   │                     │
└─────────────────┘  Socket └──────────┬──────────┘
                                       │
                              ┌────────▼────────┐
                              │   /etc/hosts    │
                              └─────────────────┘
```

**Daemon** (runs as root):
- Handles `/etc/hosts` modifications
- Creates automatic backups (10 rolling)
- Validates inputs (domain, IP)
- Rate limiting protection (100 req/min per PID)
- Flushes DNS cache automatically

**Client** (CLI/TUI, runs as user):
- Connects via Unix socket
- JSON protocol for commands
- No sudo required for operations
- Real-time status updates

Socket: `/var/run/lolcathost.sock`
Backups: `/var/backups/lolcathost/`

## Troubleshooting

### "daemon not running (socket not found)"

The daemon isn't running. Install or reinstall:

```bash
sudo lolcathost --uninstall
sudo lolcathost --install
```

Then open a **new terminal** for group membership to take effect.

### Check Daemon Status

```bash
# macOS
sudo launchctl list | grep lolcathost

# Linux
sudo systemctl status lolcathost
```

### View Daemon Logs

```bash
# macOS/Linux
cat /var/log/lolcathost/daemon.log
cat /var/log/lolcathost/daemon.err
```

### DNS Cache Not Flushing

lolcathost automatically flushes the DNS cache after changes:

- **macOS**: Uses `dscacheutil -flushcache` and `killall -HUP mDNSResponder`
- **Linux**: Uses `systemd-resolve --flush-caches` or `nscd -i hosts`

If changes don't take effect, manually flush:

```bash
# macOS
sudo dscacheutil -flushcache && sudo killall -HUP mDNSResponder

# Linux (systemd)
sudo systemd-resolve --flush-caches
```

## Development

### Prerequisites

- Go 1.24+
- macOS or Linux

### Build

```bash
make build          # Build binary
make test           # Run tests
make test-coverage  # Tests with coverage
make lint           # Run linters
make dev            # Format, lint, test, build
```

### Project Structure

```
cmd/lolcathost/    - Entry point, CLI commands
internal/
  protocol/        - JSON message types (Unix socket)
  config/          - YAML config parsing, hot-reload
  daemon/          - Socket server, /etc/hosts management
  client/          - Socket client library
  installer/       - --install/--uninstall logic
  tui/             - Bubble Tea TUI
  version/         - Update checker
```

## License

MIT License - see [LICENSE](LICENSE).

## Acknowledgments

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Terminal UI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling

## Links

- [Website](https://lukaszraczylo.github.io/lolcathost)
- [Issues](https://github.com/lukaszraczylo/lolcathost/issues)
- [Releases](https://github.com/lukaszraczylo/lolcathost/releases)
