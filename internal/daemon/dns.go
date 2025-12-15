// Package daemon provides DNS cache flushing functionality.
package daemon

import (
	"fmt"
	"os/exec"
	"runtime"
)

// DNSFlusher handles DNS cache flushing.
type DNSFlusher struct {
	method FlushMethod
}

// FlushMethod defines the DNS flush method to use.
type FlushMethod string

const (
	FlushMethodAuto        FlushMethod = "auto"
	FlushMethodDscacheutil FlushMethod = "dscacheutil"
	FlushMethodKillall     FlushMethod = "killall"
	FlushMethodBoth        FlushMethod = "both"
	FlushMethodSystemd     FlushMethod = "systemd"
	FlushMethodNscd        FlushMethod = "nscd"
)

// NewDNSFlusher creates a new DNS flusher.
func NewDNSFlusher(method FlushMethod) *DNSFlusher {
	return &DNSFlusher{method: method}
}

// Flush flushes the DNS cache using the configured method.
func (f *DNSFlusher) Flush() error {
	method := f.method
	if method == FlushMethodAuto || method == "" {
		method = f.detectMethod()
	}

	switch runtime.GOOS {
	case "darwin":
		return f.flushDarwin(method)
	case "linux":
		return f.flushLinux(method)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func (f *DNSFlusher) detectMethod() FlushMethod {
	switch runtime.GOOS {
	case "darwin":
		return FlushMethodBoth
	case "linux":
		// Check for systemd-resolve first
		if _, err := exec.LookPath("systemd-resolve"); err == nil {
			return FlushMethodSystemd
		}
		if _, err := exec.LookPath("resolvectl"); err == nil {
			return FlushMethodSystemd
		}
		// Fall back to nscd
		if _, err := exec.LookPath("nscd"); err == nil {
			return FlushMethodNscd
		}
		return FlushMethodAuto
	default:
		return FlushMethodAuto
	}
}

func (f *DNSFlusher) flushDarwin(method FlushMethod) error {
	var errs []error

	switch method {
	case FlushMethodDscacheutil:
		if err := runCommand("dscacheutil", "-flushcache"); err != nil {
			return fmt.Errorf("dscacheutil failed: %w", err)
		}
	case FlushMethodKillall:
		if err := runCommand("killall", "-HUP", "mDNSResponder"); err != nil {
			return fmt.Errorf("killall mDNSResponder failed: %w", err)
		}
	case FlushMethodBoth:
		if err := runCommand("dscacheutil", "-flushcache"); err != nil {
			errs = append(errs, fmt.Errorf("dscacheutil failed: %w", err))
		}
		if err := runCommand("killall", "-HUP", "mDNSResponder"); err != nil {
			errs = append(errs, fmt.Errorf("killall mDNSResponder failed: %w", err))
		}
		if len(errs) == 2 {
			return fmt.Errorf("all DNS flush methods failed: %v, %v", errs[0], errs[1])
		}
	default:
		// Auto - try both
		_ = runCommand("dscacheutil", "-flushcache")
		_ = runCommand("killall", "-HUP", "mDNSResponder")
	}

	return nil
}

func (f *DNSFlusher) flushLinux(method FlushMethod) error {
	switch method {
	case FlushMethodSystemd:
		// Try resolvectl first (newer), then systemd-resolve (older)
		if err := runCommand("resolvectl", "flush-caches"); err != nil {
			if err := runCommand("systemd-resolve", "--flush-caches"); err != nil {
				return fmt.Errorf("systemd DNS flush failed: %w", err)
			}
		}
	case FlushMethodNscd:
		// Try to restart nscd
		if err := runCommand("nscd", "-i", "hosts"); err != nil {
			// Try service restart as fallback
			if err := runCommand("service", "nscd", "restart"); err != nil {
				return fmt.Errorf("nscd flush failed: %w", err)
			}
		}
	default:
		// Auto - try all methods
		// Try systemd first
		if err := runCommand("resolvectl", "flush-caches"); err == nil {
			return nil
		}
		if err := runCommand("systemd-resolve", "--flush-caches"); err == nil {
			return nil
		}
		// Try nscd
		if err := runCommand("nscd", "-i", "hosts"); err == nil {
			return nil
		}
		// On many Linux systems, no explicit flush is needed as /etc/hosts is read directly
		// So we return nil here
	}

	return nil
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...) // #nosec G204 - Commands are hardcoded DNS flush utilities, not user input
	return cmd.Run()
}
