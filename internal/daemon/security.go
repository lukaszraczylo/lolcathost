// Package daemon provides security functions including rate limiting and audit logging.
package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"sync"
	"time"
)

const (
	// AuditLogPath is the path to the audit log file.
	AuditLogPath = "/var/log/lolcathost/audit.log"
	// RateLimit is the maximum requests per minute per PID.
	RateLimit = 100
	// RateLimitWindow is the time window for rate limiting.
	RateLimitWindow = time.Minute
)

// pidRateBucket holds rate limiting data for a single PID using a ring buffer.
type pidRateBucket struct {
	timestamps []time.Time // Ring buffer of request timestamps
	head       int         // Next write position
	count      int         // Number of valid entries
}

// RateLimiter implements per-PID rate limiting with efficient memory usage.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[int32]*pidRateBucket
	limit   int
	window  time.Duration
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		buckets: make(map[int32]*pidRateBucket),
		limit:   limit,
		window:  window,
	}
}

// Allow checks if a request from the given PID should be allowed.
func (r *RateLimiter) Allow(pid int32) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	bucket, exists := r.buckets[pid]
	if !exists {
		// Create new bucket with fixed capacity
		bucket = &pidRateBucket{
			timestamps: make([]time.Time, r.limit),
			head:       0,
			count:      0,
		}
		r.buckets[pid] = bucket
	}

	// Count valid (non-expired) requests in the ring buffer
	validCount := 0
	for i := 0; i < bucket.count; i++ {
		idx := (bucket.head - bucket.count + i + r.limit) % r.limit
		if bucket.timestamps[idx].After(cutoff) {
			validCount++
		}
	}

	// Check if under limit
	if validCount >= r.limit {
		return false
	}

	// Add new request to ring buffer (overwrites oldest if full)
	bucket.timestamps[bucket.head] = now
	bucket.head = (bucket.head + 1) % r.limit
	if bucket.count < r.limit {
		bucket.count++
	}

	return true
}

// Cleanup removes stale PID entries from the rate limiter.
func (r *RateLimiter) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	for pid, bucket := range r.buckets {
		// Check if all timestamps are expired
		hasValid := false
		for i := 0; i < bucket.count; i++ {
			idx := (bucket.head - bucket.count + i + r.limit) % r.limit
			if bucket.timestamps[idx].After(cutoff) {
				hasValid = true
				break
			}
		}
		if !hasValid {
			delete(r.buckets, pid)
		}
	}
}

// AuditLogger handles audit logging.
type AuditLogger struct {
	mu      sync.Mutex
	file    *os.File
	path    string
	encoder *json.Encoder
}

// AuditEntry represents a single audit log entry.
type AuditEntry struct {
	Timestamp string `json:"timestamp"`
	UID       uint32 `json:"uid"`
	PID       int32  `json:"pid"`
	Action    string `json:"action"`
	Details   any    `json:"details,omitempty"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// NewAuditLogger creates a new audit logger.
func NewAuditLogger(path string) (*AuditLogger, error) {
	// Ensure directory exists
	dir := path[:len(path)-len("/audit.log")]
	// #nosec G301 - Log directory permissions are intentionally 0755
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// #nosec G302,G304,G306 - Path is constant, permissions are intentional for audit log
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log: %w", err)
	}

	return &AuditLogger{
		file:    file,
		path:    path,
		encoder: json.NewEncoder(file),
	}, nil
}

// Log writes an audit entry.
func (a *AuditLogger) Log(uid uint32, pid int32, action string, details any, success bool, errMsg string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	entry := AuditEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		UID:       uid,
		PID:       pid,
		Action:    action,
		Details:   details,
		Success:   success,
		Error:     errMsg,
	}

	// Ignore encoding errors - audit logging should not fail the operation
	_ = a.encoder.Encode(entry)
}

// Close closes the audit logger.
func (a *AuditLogger) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.file != nil {
		err := a.file.Close()
		a.file = nil // Prevent double close
		return err
	}
	return nil
}

// PeerCredentials holds the credentials of a connected peer.
type PeerCredentials struct {
	UID uint32
	GID uint32
	PID int32
}

// isUserInGroup checks if a user (by UID) is a member of a group (by GID).
// This checks supplementary groups, not just the primary GID.
func isUserInGroup(uid uint32, targetGID uint32) bool {
	// Look up user by UID
	u, err := user.LookupId(fmt.Sprintf("%d", uid))
	if err != nil {
		return false
	}

	// Get user's group IDs
	groupIDs, err := u.GroupIds()
	if err != nil {
		return false
	}

	// Check if target GID is in the list
	for _, gidStr := range groupIDs {
		gid, err := strconv.ParseUint(gidStr, 10, 32)
		if err != nil {
			continue
		}
		if uint32(gid) == targetGID {
			return true
		}
	}

	return false
}

// lookupGroupGID looks up a group by name and returns its GID.
func lookupGroupGID(name string) (int, error) {
	group, err := user.LookupGroup(name)
	if err != nil {
		return 0, fmt.Errorf("group not found: %s", name)
	}
	gid, err := strconv.Atoi(group.Gid)
	if err != nil {
		return 0, fmt.Errorf("invalid GID for group %s: %s", name, group.Gid)
	}
	return gid, nil
}
