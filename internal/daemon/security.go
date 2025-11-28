// Package daemon provides security functions including rate limiting and audit logging.
package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
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

// RateLimiter implements per-PID rate limiting.
type RateLimiter struct {
	mu       sync.Mutex
	requests map[int32][]time.Time
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[int32][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow checks if a request from the given PID should be allowed.
func (r *RateLimiter) Allow(pid int32) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	// Get existing requests for this PID
	reqs := r.requests[pid]

	// Filter out old requests
	var validReqs []time.Time
	for _, t := range reqs {
		if t.After(cutoff) {
			validReqs = append(validReqs, t)
		}
	}

	// Check if under limit
	if len(validReqs) >= r.limit {
		r.requests[pid] = validReqs
		return false
	}

	// Add new request
	validReqs = append(validReqs, now)
	r.requests[pid] = validReqs

	return true
}

// Cleanup removes old entries from the rate limiter.
func (r *RateLimiter) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	for pid, reqs := range r.requests {
		var validReqs []time.Time
		for _, t := range reqs {
			if t.After(cutoff) {
				validReqs = append(validReqs, t)
			}
		}
		if len(validReqs) == 0 {
			delete(r.requests, pid)
		} else {
			r.requests[pid] = validReqs
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
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

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
	targetGIDStr := fmt.Sprintf("%d", targetGID)
	for _, gid := range groupIDs {
		if gid == targetGIDStr {
			return true
		}
	}

	return false
}
