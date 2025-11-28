package protocol

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequest(t *testing.T) {
	tests := []struct {
		name    string
		reqType RequestType
		payload interface{}
		wantErr bool
	}{
		{
			name:    "ping request without payload",
			reqType: RequestPing,
			payload: nil,
			wantErr: false,
		},
		{
			name:    "set request with payload",
			reqType: RequestSet,
			payload: SetPayload{Alias: "test", Enabled: true},
			wantErr: false,
		},
		{
			name:    "preset request with payload",
			reqType: RequestPreset,
			payload: PresetPayload{Name: "local"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := NewRequest(tt.reqType, tt.payload)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.reqType, req.Type)
			if tt.payload != nil {
				assert.NotNil(t, req.Payload)
			}
		})
	}
}

func TestRequest_ParsePayload(t *testing.T) {
	t.Run("valid payload", func(t *testing.T) {
		payload := SetPayload{Alias: "test-alias", Enabled: true, Force: false}
		req, err := NewRequest(RequestSet, payload)
		require.NoError(t, err)

		var parsed SetPayload
		err = req.ParsePayload(&parsed)
		require.NoError(t, err)
		assert.Equal(t, "test-alias", parsed.Alias)
		assert.True(t, parsed.Enabled)
		assert.False(t, parsed.Force)
	})

	t.Run("nil payload", func(t *testing.T) {
		req := &Request{Type: RequestPing}
		var parsed SetPayload
		err := req.ParsePayload(&parsed)
		assert.Error(t, err)
	})
}

func TestNewOKResponse(t *testing.T) {
	t.Run("with data", func(t *testing.T) {
		data := StatusData{
			Running:      true,
			Version:      "1.0.0",
			Uptime:       3600,
			ActiveCount:  5,
			RequestCount: 100,
		}

		resp, err := NewOKResponse(data)
		require.NoError(t, err)
		assert.Equal(t, "ok", resp.Status)
		assert.NotNil(t, resp.Data)
		assert.True(t, resp.IsOK())
	})

	t.Run("without data", func(t *testing.T) {
		resp, err := NewOKResponse(nil)
		require.NoError(t, err)
		assert.Equal(t, "ok", resp.Status)
		assert.Nil(t, resp.Data)
	})
}

func TestNewErrorResponse(t *testing.T) {
	resp := NewErrorResponse(ErrCodeBlockedDomain, "domain is blocked")

	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, ErrCodeBlockedDomain, resp.Code)
	assert.Equal(t, "domain is blocked", resp.Message)
	assert.False(t, resp.IsOK())
}

func TestResponse_ParseData(t *testing.T) {
	t.Run("valid data", func(t *testing.T) {
		data := ListData{
			Entries: []HostEntry{
				{Domain: "example.com", IP: "127.0.0.1", Alias: "example", Enabled: true, Group: "dev"},
			},
		}
		resp, err := NewOKResponse(data)
		require.NoError(t, err)

		var parsed ListData
		err = resp.ParseData(&parsed)
		require.NoError(t, err)
		assert.Len(t, parsed.Entries, 1)
		assert.Equal(t, "example.com", parsed.Entries[0].Domain)
	})

	t.Run("nil data", func(t *testing.T) {
		resp := &Response{Status: "ok"}
		var parsed ListData
		err := resp.ParseData(&parsed)
		assert.Error(t, err)
	})
}

func TestRequestTypes(t *testing.T) {
	types := []RequestType{
		RequestPing,
		RequestStatus,
		RequestList,
		RequestSet,
		RequestSync,
		RequestPreset,
		RequestRollback,
		RequestBackups,
	}

	for _, rt := range types {
		t.Run(string(rt), func(t *testing.T) {
			req, err := NewRequest(rt, nil)
			require.NoError(t, err)
			assert.Equal(t, rt, req.Type)

			// Verify JSON marshaling works
			data, err := json.Marshal(req)
			require.NoError(t, err)
			assert.Contains(t, string(data), string(rt))
		})
	}
}

func TestErrorCodes(t *testing.T) {
	codes := []ErrorCode{
		ErrCodeInvalidRequest,
		ErrCodeInvalidDomain,
		ErrCodeInvalidIP,
		ErrCodeBlockedDomain,
		ErrCodeRateLimited,
		ErrCodeNotFound,
		ErrCodeConflict,
		ErrCodeInternalError,
		ErrCodePermissionError,
	}

	for _, code := range codes {
		t.Run(string(code), func(t *testing.T) {
			resp := NewErrorResponse(code, "test error")
			assert.Equal(t, code, resp.Code)

			// Verify JSON marshaling works
			data, err := json.Marshal(resp)
			require.NoError(t, err)
			assert.Contains(t, string(data), string(code))
		})
	}
}

func TestHostEntry(t *testing.T) {
	entry := HostEntry{
		Domain:  "example.com",
		IP:      "127.0.0.1",
		Alias:   "example-local",
		Enabled: true,
		Group:   "development",
	}

	data, err := json.Marshal(entry)
	require.NoError(t, err)

	var parsed HostEntry
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, entry.Domain, parsed.Domain)
	assert.Equal(t, entry.IP, parsed.IP)
	assert.Equal(t, entry.Alias, parsed.Alias)
	assert.Equal(t, entry.Enabled, parsed.Enabled)
	assert.Equal(t, entry.Group, parsed.Group)
}

func TestBackupInfo(t *testing.T) {
	info := BackupInfo{
		Name:      "hosts.20231201-120000.bak",
		Timestamp: 1701432000,
		Size:      1024,
	}

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var parsed BackupInfo
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, info.Name, parsed.Name)
	assert.Equal(t, info.Timestamp, parsed.Timestamp)
	assert.Equal(t, info.Size, parsed.Size)
}
