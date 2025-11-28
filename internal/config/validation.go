// Package config provides validation functions for configuration.
package config

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

// domainRegex validates domain names.
var domainRegex = regexp.MustCompile(`^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$|^localhost$`)

// aliasRegex validates alias names.
var aliasRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$`)

// blockedDomains contains domains that cannot be modified.
var blockedDomains = map[string]bool{
	"apple.com":          true,
	"icloud.com":         true,
	"icloud-content.com": true,
	"apple-dns.cn":       true,
	"apple-dns.net":      true,
	"mzstatic.com":       true,
	"itunes.apple.com":   true,
	"updates.apple.com":  true,
}

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateConfig validates the entire configuration.
func ValidateConfig(cfg *Config) error {
	if cfg == nil {
		return &ValidationError{Field: "config", Message: "config is nil"}
	}

	if err := validateSettings(&cfg.Settings); err != nil {
		return err
	}

	// Track aliases for uniqueness
	aliases := make(map[string]bool)

	for i, g := range cfg.Groups {
		if err := validateGroup(&g, i, aliases); err != nil {
			return err
		}
	}

	for i, p := range cfg.Presets {
		if err := validatePreset(&p, i, aliases); err != nil {
			return err
		}
	}

	return nil
}

func validateSettings(s *Settings) error {
	switch s.FlushMethod {
	case FlushMethodAuto, FlushMethodDscacheutil, FlushMethodKillall, FlushMethodBoth, "":
		// Valid
	default:
		return &ValidationError{
			Field:   "settings.flushMethod",
			Message: fmt.Sprintf("invalid flush method: %s", s.FlushMethod),
		}
	}
	return nil
}

func validateGroup(g *Group, index int, aliases map[string]bool) error {
	if strings.TrimSpace(g.Name) == "" {
		return &ValidationError{
			Field:   fmt.Sprintf("groups[%d].name", index),
			Message: "group name is required",
		}
	}

	for i, h := range g.Hosts {
		if err := validateHost(&h, index, i, aliases); err != nil {
			return err
		}
	}

	return nil
}

func validateHost(h *Host, groupIndex, hostIndex int, aliases map[string]bool) error {
	fieldPrefix := fmt.Sprintf("groups[%d].hosts[%d]", groupIndex, hostIndex)

	// Validate domain
	if !ValidateDomain(h.Domain) {
		return &ValidationError{
			Field:   fieldPrefix + ".domain",
			Message: fmt.Sprintf("invalid domain: %s", h.Domain),
		}
	}

	// Check blocked domains
	if IsBlockedDomain(h.Domain) {
		return &ValidationError{
			Field:   fieldPrefix + ".domain",
			Message: fmt.Sprintf("domain is blocked: %s", h.Domain),
		}
	}

	// Validate IP
	if !ValidateIP(h.IP) {
		return &ValidationError{
			Field:   fieldPrefix + ".ip",
			Message: fmt.Sprintf("invalid IP address: %s", h.IP),
		}
	}

	// Validate alias
	if !ValidateAlias(h.Alias) {
		return &ValidationError{
			Field:   fieldPrefix + ".alias",
			Message: fmt.Sprintf("invalid alias: %s", h.Alias),
		}
	}

	// Check alias uniqueness
	if aliases[h.Alias] {
		return &ValidationError{
			Field:   fieldPrefix + ".alias",
			Message: fmt.Sprintf("duplicate alias: %s", h.Alias),
		}
	}
	aliases[h.Alias] = true

	return nil
}

func validatePreset(p *Preset, index int, aliases map[string]bool) error {
	fieldPrefix := fmt.Sprintf("presets[%d]", index)

	if strings.TrimSpace(p.Name) == "" {
		return &ValidationError{
			Field:   fieldPrefix + ".name",
			Message: "preset name is required",
		}
	}

	// Note: We don't validate preset aliases strictly anymore.
	// Unknown aliases in presets will simply be skipped when applying the preset.
	// This allows presets to survive when hosts are removed from the config.

	return nil
}

// ValidateDomain checks if a domain name is valid.
func ValidateDomain(domain string) bool {
	if domain == "" {
		return false
	}
	return domainRegex.MatchString(domain)
}

// ValidateIP checks if an IP address is valid (IPv4 or IPv6).
func ValidateIP(ip string) bool {
	if ip == "" {
		return false
	}
	return net.ParseIP(ip) != nil
}

// ValidateAlias checks if an alias is valid.
func ValidateAlias(alias string) bool {
	if alias == "" {
		return false
	}
	return aliasRegex.MatchString(alias)
}

// IsBlockedDomain checks if a domain is in the blocklist.
func IsBlockedDomain(domain string) bool {
	domain = strings.ToLower(domain)

	// Check exact match
	if blockedDomains[domain] {
		return true
	}

	// Check if it's a subdomain of a blocked domain
	for blocked := range blockedDomains {
		if strings.HasSuffix(domain, "."+blocked) {
			return true
		}
	}

	return false
}

// GetBlockedDomains returns a copy of the blocked domains list.
func GetBlockedDomains() []string {
	domains := make([]string, 0, len(blockedDomains))
	for d := range blockedDomains {
		domains = append(domains, d)
	}
	return domains
}
