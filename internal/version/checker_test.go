package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"v1.0.0", "1.0.0"},
		{"1.0.0", "1.0.0"},
		{"  v2.1.3  ", "2.1.3"},
		{"V1.0.0", "1.0.0"},
		{"v0.1.0", "0.1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeVersion(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected []int
	}{
		{"1.0.0", []int{1, 0, 0}},
		{"2.1.3", []int{2, 1, 3}},
		{"1.0", []int{1, 0}},
		{"10.20.30", []int{10, 20, 30}},
		{"1.0.0-beta", []int{1, 0, 0}},
		{"1.0.0-rc1", []int{1, 0, 0}},
		{"1.0.0+build123", []int{1, 0, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseVersion(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		name     string
		latest   string
		current  string
		expected bool
	}{
		{"major version bump", "2.0.0", "1.0.0", true},
		{"minor version bump", "1.1.0", "1.0.0", true},
		{"patch version bump", "1.0.1", "1.0.0", true},
		{"same version", "1.0.0", "1.0.0", false},
		{"current is newer major", "1.0.0", "2.0.0", false},
		{"current is newer minor", "1.0.0", "1.1.0", false},
		{"current is newer patch", "1.0.0", "1.0.1", false},
		{"longer version is newer", "1.0.1", "1.0", true},
		{"shorter version is older", "1.0", "1.0.1", false},
		{"double digit versions", "10.0.0", "9.0.0", true},
		{"with prerelease suffix", "1.1.0", "1.0.0-beta", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNewerVersion(tt.latest, tt.current)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUpdateInfo_FormatUpdateMessage(t *testing.T) {
	info := &UpdateInfo{
		CurrentVersion: "1.0.0",
		LatestVersion:  "1.1.0",
		ReleaseURL:     "https://github.com/lukaszraczylo/lolcathost/releases/tag/v1.1.0",
	}

	msg := info.FormatUpdateMessage()
	assert.Contains(t, msg, "1.0.0")
	assert.Contains(t, msg, "1.1.0")
	assert.Contains(t, msg, "https://github.com")
}

func TestNewChecker(t *testing.T) {
	checker := NewChecker("lukaszraczylo", "lolcathost", "v1.0.0")

	assert.Equal(t, "lukaszraczylo", checker.owner)
	assert.Equal(t, "lolcathost", checker.repo)
	assert.Equal(t, "1.0.0", checker.current) // Should be normalized
	assert.NotNil(t, checker.client)
}
