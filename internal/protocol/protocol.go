// Package protocol defines shared message types for client-daemon communication.
package protocol

import (
	"encoding/json"
	"fmt"
)

// SocketPath is the Unix socket path for daemon communication.
const SocketPath = "/var/run/lolcathost.sock"

// RequestType defines the type of request.
type RequestType string

const (
	RequestPing         RequestType = "ping"
	RequestStatus       RequestType = "status"
	RequestList         RequestType = "list"
	RequestSet          RequestType = "set"
	RequestAdd          RequestType = "add"
	RequestDelete       RequestType = "delete"
	RequestSync         RequestType = "sync"
	RequestPreset       RequestType = "preset"
	RequestRollback     RequestType = "rollback"
	RequestBackups      RequestType = "backups"
	RequestAddGroup     RequestType = "add_group"
	RequestDeleteGroup  RequestType = "delete_group"
	RequestRenameGroup  RequestType = "rename_group"
	RequestListGroups   RequestType = "list_groups"
	RequestAddPreset    RequestType = "add_preset"
	RequestDeletePreset RequestType = "delete_preset"
	RequestListPresets  RequestType = "list_presets"
)

// ErrorCode defines standard error codes.
type ErrorCode string

const (
	ErrCodeInvalidRequest  ErrorCode = "INVALID_REQUEST"
	ErrCodeInvalidDomain   ErrorCode = "INVALID_DOMAIN"
	ErrCodeInvalidIP       ErrorCode = "INVALID_IP"
	ErrCodeBlockedDomain   ErrorCode = "BLOCKED_DOMAIN"
	ErrCodeRateLimited     ErrorCode = "RATE_LIMITED"
	ErrCodeUnauthorized    ErrorCode = "UNAUTHORIZED"
	ErrCodeNotFound        ErrorCode = "NOT_FOUND"
	ErrCodeConflict        ErrorCode = "CONFLICT"
	ErrCodeInternalError   ErrorCode = "INTERNAL_ERROR"
	ErrCodePermissionError ErrorCode = "PERMISSION_ERROR"
)

// Request represents a client request to the daemon.
type Request struct {
	Type    RequestType     `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// SetPayload is the payload for set requests.
type SetPayload struct {
	Alias   string `json:"alias"`
	Enabled bool   `json:"enabled"`
	Force   bool   `json:"force,omitempty"`
}

// PresetPayload is the payload for preset requests.
type PresetPayload struct {
	Name string `json:"name"`
}

// RollbackPayload is the payload for rollback requests.
type RollbackPayload struct {
	BackupName string `json:"backup_name"`
}

// AddPayload is the payload for add requests.
type AddPayload struct {
	Domain  string `json:"domain"`
	IP      string `json:"ip"`
	Alias   string `json:"alias"`
	Group   string `json:"group"`
	Enabled bool   `json:"enabled"`
}

// DeletePayload is the payload for delete requests.
type DeletePayload struct {
	Alias string `json:"alias"`
}

// GroupPayload is the payload for group add/delete requests.
type GroupPayload struct {
	Name string `json:"name"`
}

// RenameGroupPayload is the payload for rename_group requests.
type RenameGroupPayload struct {
	OldName string `json:"old_name"`
	NewName string `json:"new_name"`
}

// GroupsData is the data for list_groups responses.
type GroupsData struct {
	Groups []string `json:"groups"`
}

// AddPresetPayload is the payload for add_preset requests.
type AddPresetPayload struct {
	Name    string   `json:"name"`
	Enable  []string `json:"enable"`
	Disable []string `json:"disable"`
}

// PresetInfo represents a preset with its configuration.
type PresetInfo struct {
	Name    string   `json:"name"`
	Enable  []string `json:"enable"`
	Disable []string `json:"disable"`
}

// PresetsData is the data for list_presets responses.
type PresetsData struct {
	Presets []PresetInfo `json:"presets"`
}

// Response represents a daemon response.
type Response struct {
	Status  string          `json:"status"`
	Data    json.RawMessage `json:"data,omitempty"`
	Message string          `json:"message,omitempty"`
	Code    ErrorCode       `json:"code,omitempty"`
}

// StatusData is the data for status responses.
type StatusData struct {
	Running      bool   `json:"running"`
	Version      string `json:"version"`
	Uptime       int64  `json:"uptime_seconds"`
	ActiveCount  int    `json:"active_count"`
	RequestCount int64  `json:"request_count"`
}

// HostEntry represents a single host entry.
type HostEntry struct {
	Domain  string `json:"domain"`
	IP      string `json:"ip"`
	Alias   string `json:"alias"`
	Enabled bool   `json:"enabled"`
	Group   string `json:"group"`
}

// ListData is the data for list responses.
type ListData struct {
	Entries []HostEntry `json:"entries"`
}

// SetData is the data for set responses.
type SetData struct {
	Domain  string `json:"domain"`
	Applied bool   `json:"applied"`
}

// BackupsData is the data for backups responses.
type BackupsData struct {
	Backups []BackupInfo `json:"backups"`
}

// BackupInfo represents a backup file.
type BackupInfo struct {
	Name      string `json:"name"`
	Timestamp int64  `json:"timestamp"`
	Size      int64  `json:"size"`
}

// NewRequest creates a new request with the given type and payload.
func NewRequest(reqType RequestType, payload interface{}) (*Request, error) {
	req := &Request{Type: reqType}
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
		req.Payload = data
	}
	return req, nil
}

// NewOKResponse creates a success response with optional data.
func NewOKResponse(data interface{}) (*Response, error) {
	resp := &Response{Status: "ok"}
	if data != nil {
		dataBytes, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal data: %w", err)
		}
		resp.Data = dataBytes
	}
	return resp, nil
}

// NewErrorResponse creates an error response.
func NewErrorResponse(code ErrorCode, message string) *Response {
	return &Response{
		Status:  "error",
		Code:    code,
		Message: message,
	}
}

// ParsePayload unmarshals the request payload into the given target.
func (r *Request) ParsePayload(target interface{}) error {
	if r.Payload == nil {
		return fmt.Errorf("no payload in request")
	}
	return json.Unmarshal(r.Payload, target)
}

// ParseData unmarshals the response data into the given target.
func (r *Response) ParseData(target interface{}) error {
	if r.Data == nil {
		return fmt.Errorf("no data in response")
	}
	return json.Unmarshal(r.Data, target)
}

// IsOK returns true if the response indicates success.
func (r *Response) IsOK() bool {
	return r.Status == "ok"
}
