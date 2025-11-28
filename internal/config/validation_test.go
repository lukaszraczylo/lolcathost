package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDomain(t *testing.T) {
	tests := []struct {
		domain string
		valid  bool
	}{
		{"example.com", true},
		{"sub.example.com", true},
		{"my-app.example.com", true},
		{"localhost", true},
		{"a.b.c.d.example.com", true},
		{"example123.com", true},

		{"", false},
		{"-example.com", false},
		{"example-.com", false},
		{"example.c", false}, // TLD too short
		{"example", false},   // No TLD
		{".example.com", false},
		{"example..com", false},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			result := ValidateDomain(tt.domain)
			assert.Equal(t, tt.valid, result, "domain: %s", tt.domain)
		})
	}
}

func TestValidateIP(t *testing.T) {
	tests := []struct {
		ip    string
		valid bool
	}{
		// Valid IPv4
		{"127.0.0.1", true},
		{"192.168.1.1", true},
		{"0.0.0.0", true},
		{"255.255.255.255", true},

		// Valid IPv6
		{"::1", true},
		{"2001:db8::1", true},
		{"fe80::1", true},
		{"::ffff:192.168.1.1", true},

		// Invalid
		{"", false},
		{"256.0.0.1", false},
		{"192.168.1", false},
		{"not-an-ip", false},
		{"192.168.1.1.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			result := ValidateIP(tt.ip)
			assert.Equal(t, tt.valid, result, "ip: %s", tt.ip)
		})
	}
}

func TestValidateAlias(t *testing.T) {
	tests := []struct {
		alias string
		valid bool
	}{
		{"my-alias", true},
		{"myalias", true},
		{"my_alias", true},
		{"alias123", true},
		{"a", true},
		{"a-b_c-d", true},

		{"", false},
		{"-startswithdash", false},
		{"_startswithunderscore", false},
		{"has spaces", false},
		{"has.dot", false},
	}

	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			result := ValidateAlias(tt.alias)
			assert.Equal(t, tt.valid, result, "alias: %s", tt.alias)
		})
	}
}

func TestIsBlockedDomain(t *testing.T) {
	tests := []struct {
		domain  string
		blocked bool
	}{
		// Blocked domains
		{"apple.com", true},
		{"icloud.com", true},
		{"sub.apple.com", true},
		{"deep.sub.icloud.com", true},
		{"APPLE.COM", true}, // Case insensitive

		// Allowed domains
		{"example.com", false},
		{"myapp.com", false},
		{"applestore.com", false}, // Not a subdomain
		{"notapple.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			result := IsBlockedDomain(tt.domain)
			assert.Equal(t, tt.blocked, result, "domain: %s", tt.domain)
		})
	}
}

func TestGetBlockedDomains(t *testing.T) {
	domains := GetBlockedDomains()
	assert.NotEmpty(t, domains)
	assert.Contains(t, domains, "apple.com")
	assert.Contains(t, domains, "icloud.com")
}

func TestValidateConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := &Config{
			Settings: Settings{
				AutoApply:   true,
				FlushMethod: FlushMethodAuto,
			},
			Groups: []Group{
				{
					Name: "development",
					Hosts: []Host{
						{Domain: "example.com", IP: "127.0.0.1", Alias: "example", Enabled: true},
					},
				},
			},
			Presets: []Preset{
				{Name: "local", Enable: []string{"example"}, Disable: []string{}},
			},
		}

		err := ValidateConfig(cfg)
		assert.NoError(t, err)
	})

	t.Run("nil config", func(t *testing.T) {
		err := ValidateConfig(nil)
		assert.Error(t, err)
	})

	t.Run("invalid flush method", func(t *testing.T) {
		cfg := &Config{
			Settings: Settings{FlushMethod: "invalid"},
		}
		err := ValidateConfig(cfg)
		assert.Error(t, err)
	})

	t.Run("empty group name", func(t *testing.T) {
		cfg := &Config{
			Groups: []Group{{Name: "", Hosts: []Host{}}},
		}
		err := ValidateConfig(cfg)
		assert.Error(t, err)
	})

	t.Run("invalid domain", func(t *testing.T) {
		cfg := &Config{
			Groups: []Group{
				{
					Name: "dev",
					Hosts: []Host{
						{Domain: "invalid", IP: "127.0.0.1", Alias: "test", Enabled: true},
					},
				},
			},
		}
		err := ValidateConfig(cfg)
		assert.Error(t, err)
	})

	t.Run("blocked domain", func(t *testing.T) {
		cfg := &Config{
			Groups: []Group{
				{
					Name: "dev",
					Hosts: []Host{
						{Domain: "apple.com", IP: "127.0.0.1", Alias: "test", Enabled: true},
					},
				},
			},
		}
		err := ValidateConfig(cfg)
		assert.Error(t, err)
	})

	t.Run("invalid IP", func(t *testing.T) {
		cfg := &Config{
			Groups: []Group{
				{
					Name: "dev",
					Hosts: []Host{
						{Domain: "example.com", IP: "invalid", Alias: "test", Enabled: true},
					},
				},
			},
		}
		err := ValidateConfig(cfg)
		assert.Error(t, err)
	})

	t.Run("invalid alias", func(t *testing.T) {
		cfg := &Config{
			Groups: []Group{
				{
					Name: "dev",
					Hosts: []Host{
						{Domain: "example.com", IP: "127.0.0.1", Alias: "-invalid", Enabled: true},
					},
				},
			},
		}
		err := ValidateConfig(cfg)
		assert.Error(t, err)
	})

	t.Run("duplicate alias", func(t *testing.T) {
		cfg := &Config{
			Groups: []Group{
				{
					Name: "dev",
					Hosts: []Host{
						{Domain: "a.com", IP: "127.0.0.1", Alias: "same", Enabled: true},
						{Domain: "b.com", IP: "127.0.0.1", Alias: "same", Enabled: true},
					},
				},
			},
		}
		err := ValidateConfig(cfg)
		assert.Error(t, err)
	})

	t.Run("empty preset name", func(t *testing.T) {
		cfg := &Config{
			Groups: []Group{
				{
					Name: "dev",
					Hosts: []Host{
						{Domain: "example.com", IP: "127.0.0.1", Alias: "test", Enabled: true},
					},
				},
			},
			Presets: []Preset{
				{Name: "", Enable: []string{}},
			},
		}
		err := ValidateConfig(cfg)
		assert.Error(t, err)
	})

	t.Run("preset with unknown alias is allowed", func(t *testing.T) {
		// Unknown aliases in presets are now allowed (they're simply skipped when applied)
		// This allows presets to survive when hosts are removed from the config
		cfg := &Config{
			Groups: []Group{
				{
					Name: "dev",
					Hosts: []Host{
						{Domain: "example.com", IP: "127.0.0.1", Alias: "test", Enabled: true},
					},
				},
			},
			Presets: []Preset{
				{Name: "local", Enable: []string{"unknown"}},
			},
		}
		err := ValidateConfig(cfg)
		assert.NoError(t, err)
	})
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{Field: "test.field", Message: "test message"}
	assert.Equal(t, "test.field: test message", err.Error())
}

func TestValidateSettings(t *testing.T) {
	tests := []struct {
		name    string
		method  FlushMethod
		wantErr bool
	}{
		{"auto", FlushMethodAuto, false},
		{"dscacheutil", FlushMethodDscacheutil, false},
		{"killall", FlushMethodKillall, false},
		{"both", FlushMethodBoth, false},
		{"empty", "", false},
		{"invalid", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := &Settings{FlushMethod: tt.method}
			err := validateSettings(settings)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Matrix testing for domain validation
func TestValidateDomain_Matrix(t *testing.T) {
	prefixes := []string{"", "sub.", "a.b."}
	domains := []string{"example", "my-app", "test123"}
	tlds := []string{".com", ".io", ".co.uk", ".dev"}

	for _, prefix := range prefixes {
		for _, domain := range domains {
			for _, tld := range tlds {
				fullDomain := prefix + domain + tld
				t.Run(fullDomain, func(t *testing.T) {
					result := ValidateDomain(fullDomain)
					assert.True(t, result, "expected %s to be valid", fullDomain)
				})
			}
		}
	}
}

// Matrix testing for IP validation
func TestValidateIP_Matrix(t *testing.T) {
	octets := []string{"0", "127", "192", "255"}

	for _, o1 := range octets {
		for _, o2 := range octets {
			for _, o3 := range octets {
				for _, o4 := range octets {
					ip := o1 + "." + o2 + "." + o3 + "." + o4
					t.Run(ip, func(t *testing.T) {
						result := ValidateIP(ip)
						assert.True(t, result, "expected %s to be valid", ip)
					})
				}
			}
		}
	}
}

// Benchmark tests
func BenchmarkValidateDomain(b *testing.B) {
	domains := []string{
		"example.com",
		"sub.example.com",
		"very.long.subdomain.chain.example.com",
	}

	for _, domain := range domains {
		b.Run(domain, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ValidateDomain(domain)
			}
		})
	}
}

func BenchmarkValidateIP(b *testing.B) {
	ips := []string{
		"127.0.0.1",
		"192.168.1.1",
		"::1",
		"2001:db8::1",
	}

	for _, ip := range ips {
		b.Run(ip, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ValidateIP(ip)
			}
		})
	}
}

func BenchmarkIsBlockedDomain(b *testing.B) {
	domains := []string{
		"example.com",    // not blocked
		"apple.com",      // blocked
		"sub.icloud.com", // blocked subdomain
	}

	for _, domain := range domains {
		b.Run(domain, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				IsBlockedDomain(domain)
			}
		})
	}
}

func BenchmarkValidateConfig(b *testing.B) {
	cfg := &Config{
		Settings: Settings{AutoApply: true, FlushMethod: FlushMethodAuto},
		Groups: []Group{
			{
				Name: "development",
				Hosts: []Host{
					{Domain: "a.example.com", IP: "127.0.0.1", Alias: "a", Enabled: true},
					{Domain: "b.example.com", IP: "127.0.0.1", Alias: "b", Enabled: true},
					{Domain: "c.example.com", IP: "127.0.0.1", Alias: "c", Enabled: false},
				},
			},
		},
		Presets: []Preset{
			{Name: "local", Enable: []string{"a", "b"}, Disable: []string{"c"}},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := ValidateConfig(cfg)
		require.NoError(b, err)
	}
}
