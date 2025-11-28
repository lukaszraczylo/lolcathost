package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter_Allow(t *testing.T) {
	t.Run("under limit", func(t *testing.T) {
		rl := NewRateLimiter(5, time.Minute)

		for i := 0; i < 5; i++ {
			assert.True(t, rl.Allow(123), "request %d should be allowed", i)
		}
	})

	t.Run("over limit", func(t *testing.T) {
		rl := NewRateLimiter(3, time.Minute)

		for i := 0; i < 3; i++ {
			assert.True(t, rl.Allow(123))
		}

		// 4th request should be blocked
		assert.False(t, rl.Allow(123))
	})

	t.Run("different PIDs", func(t *testing.T) {
		rl := NewRateLimiter(2, time.Minute)

		// PID 1
		assert.True(t, rl.Allow(1))
		assert.True(t, rl.Allow(1))
		assert.False(t, rl.Allow(1))

		// PID 2 should have its own limit
		assert.True(t, rl.Allow(2))
		assert.True(t, rl.Allow(2))
		assert.False(t, rl.Allow(2))
	})

	t.Run("window expiration", func(t *testing.T) {
		rl := NewRateLimiter(2, 10*time.Millisecond)

		assert.True(t, rl.Allow(123))
		assert.True(t, rl.Allow(123))
		assert.False(t, rl.Allow(123))

		// Wait for window to expire
		time.Sleep(15 * time.Millisecond)

		// Should be allowed again
		assert.True(t, rl.Allow(123))
	})
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(10, 10*time.Millisecond)

	// Add requests from multiple PIDs
	for pid := int32(1); pid <= 5; pid++ {
		rl.Allow(pid)
	}

	assert.Len(t, rl.requests, 5)

	// Wait for expiration
	time.Sleep(15 * time.Millisecond)

	// Cleanup
	rl.Cleanup()

	assert.Empty(t, rl.requests)
}

func TestAuditLogger_Log(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(logPath)
	require.NoError(t, err)
	defer logger.Close()

	logger.Log(1000, 12345, "set", map[string]string{"alias": "test"}, true, "")
	logger.Log(1000, 12345, "sync", nil, false, "sync failed")

	// Read log file
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, `"action":"set"`)
	assert.Contains(t, contentStr, `"uid":1000`)
	assert.Contains(t, contentStr, `"pid":12345`)
	assert.Contains(t, contentStr, `"success":true`)
	assert.Contains(t, contentStr, `"action":"sync"`)
	assert.Contains(t, contentStr, `"success":false`)
	assert.Contains(t, contentStr, `"error":"sync failed"`)
}

func TestAuditLogger_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "subdir", "audit.log")

	logger, err := NewAuditLogger(logPath)
	require.NoError(t, err)
	defer logger.Close()

	// Verify directory was created
	_, err = os.Stat(filepath.Dir(logPath))
	assert.NoError(t, err)
}

func TestAuditLogger_Close(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(logPath)
	require.NoError(t, err)

	err = logger.Close()
	assert.NoError(t, err)

	// Closing again should not error
	err = logger.Close()
	assert.NoError(t, err)
}

func TestPeerCredentials(t *testing.T) {
	creds := &PeerCredentials{
		UID: 501,
		GID: 20,
		PID: 12345,
	}

	assert.Equal(t, uint32(501), creds.UID)
	assert.Equal(t, uint32(20), creds.GID)
	assert.Equal(t, int32(12345), creds.PID)
}

// Matrix test for rate limiting
func TestRateLimiter_Matrix(t *testing.T) {
	limits := []int{1, 5, 10, 100}
	windows := []time.Duration{10 * time.Millisecond, 100 * time.Millisecond, time.Second}

	for _, limit := range limits {
		for _, window := range windows {
			t.Run(
				"limit="+string(rune('0'+limit))+"_window="+window.String(),
				func(t *testing.T) {
					rl := NewRateLimiter(limit, window)

					// Should allow exactly 'limit' requests
					for i := 0; i < limit; i++ {
						assert.True(t, rl.Allow(1))
					}

					// Next should be blocked
					assert.False(t, rl.Allow(1))
				},
			)
		}
	}
}

func BenchmarkRateLimiter_Allow(b *testing.B) {
	rl := NewRateLimiter(RateLimit, RateLimitWindow)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Allow(int32(i % 100))
	}
}

func BenchmarkRateLimiter_Cleanup(b *testing.B) {
	rl := NewRateLimiter(RateLimit, RateLimitWindow)

	// Pre-populate with requests
	for i := 0; i < 1000; i++ {
		rl.Allow(int32(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Cleanup()
	}
}

func BenchmarkAuditLogger_Log(b *testing.B) {
	tmpDir := b.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(logPath)
	require.NoError(b, err)
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Log(1000, 12345, "set", map[string]string{"alias": "test"}, true, "")
	}
}
