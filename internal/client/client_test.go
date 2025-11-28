package client

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lukaszraczylo/lolcathost/internal/protocol"
)

// mockServer creates a mock Unix socket server for testing
type mockServer struct {
	listener net.Listener
	path     string
	handler  func(req *protocol.Request) *protocol.Response
}

func newMockServer(t *testing.T) *mockServer {
	// Use /tmp directly to avoid long paths (Unix socket paths have ~104 char limit on macOS)
	tmpDir, err := os.MkdirTemp("/tmp", "lolcat")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	socketPath := filepath.Join(tmpDir, "s.sock")

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	ms := &mockServer{
		listener: listener,
		path:     socketPath,
	}

	go ms.serve()

	return ms
}

func (ms *mockServer) serve() {
	for {
		conn, err := ms.listener.Accept()
		if err != nil {
			return
		}
		go ms.handleConn(conn)
	}
}

func (ms *mockServer) handleConn(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}

		var req protocol.Request
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}

		var resp *protocol.Response
		if ms.handler != nil {
			resp = ms.handler(&req)
		} else {
			resp, _ = protocol.NewOKResponse(nil)
		}

		data, _ := json.Marshal(resp)
		conn.Write(append(data, '\n'))
	}
}

func (ms *mockServer) close() {
	ms.listener.Close()
	os.Remove(ms.path)
}

func TestClient_Connect(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := newMockServer(t)
		defer server.close()

		client := New(server.path)
		err := client.Connect()
		require.NoError(t, err)
		defer client.Close()

		assert.NotNil(t, client.conn)
		assert.NotNil(t, client.reader)
	})

	t.Run("failure - socket not found", func(t *testing.T) {
		client := New("/nonexistent/socket.sock")
		err := client.Connect()
		assert.Error(t, err)
	})
}

func TestClient_Ping(t *testing.T) {
	server := newMockServer(t)
	defer server.close()

	server.handler = func(req *protocol.Request) *protocol.Response {
		if req.Type == protocol.RequestPing {
			resp, _ := protocol.NewOKResponse(map[string]string{"pong": "ok"})
			return resp
		}
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "unexpected request")
	}

	client := New(server.path)
	err := client.Connect()
	require.NoError(t, err)
	defer client.Close()

	err = client.Ping()
	assert.NoError(t, err)
}

func TestClient_Status(t *testing.T) {
	server := newMockServer(t)
	defer server.close()

	server.handler = func(req *protocol.Request) *protocol.Response {
		if req.Type == protocol.RequestStatus {
			resp, _ := protocol.NewOKResponse(protocol.StatusData{
				Running:      true,
				Version:      "1.0.0",
				Uptime:       3600,
				ActiveCount:  5,
				RequestCount: 100,
			})
			return resp
		}
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "unexpected")
	}

	client := New(server.path)
	err := client.Connect()
	require.NoError(t, err)
	defer client.Close()

	status, err := client.Status()
	require.NoError(t, err)

	assert.True(t, status.Running)
	assert.Equal(t, "1.0.0", status.Version)
	assert.Equal(t, int64(3600), status.Uptime)
	assert.Equal(t, 5, status.ActiveCount)
	assert.Equal(t, int64(100), status.RequestCount)
}

func TestClient_List(t *testing.T) {
	server := newMockServer(t)
	defer server.close()

	server.handler = func(req *protocol.Request) *protocol.Response {
		if req.Type == protocol.RequestList {
			resp, _ := protocol.NewOKResponse(protocol.ListData{
				Entries: []protocol.HostEntry{
					{Domain: "a.com", IP: "127.0.0.1", Alias: "a", Enabled: true, Group: "dev"},
					{Domain: "b.com", IP: "127.0.0.1", Alias: "b", Enabled: false, Group: "dev"},
				},
			})
			return resp
		}
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "unexpected")
	}

	client := New(server.path)
	err := client.Connect()
	require.NoError(t, err)
	defer client.Close()

	entries, err := client.List()
	require.NoError(t, err)

	assert.Len(t, entries, 2)
	assert.Equal(t, "a.com", entries[0].Domain)
	assert.True(t, entries[0].Enabled)
	assert.Equal(t, "b.com", entries[1].Domain)
	assert.False(t, entries[1].Enabled)
}

func TestClient_Set(t *testing.T) {
	server := newMockServer(t)
	defer server.close()

	server.handler = func(req *protocol.Request) *protocol.Response {
		if req.Type == protocol.RequestSet {
			var payload protocol.SetPayload
			req.ParsePayload(&payload)

			resp, _ := protocol.NewOKResponse(protocol.SetData{
				Domain:  "example.com",
				Applied: true,
			})
			return resp
		}
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "unexpected")
	}

	client := New(server.path)
	err := client.Connect()
	require.NoError(t, err)
	defer client.Close()

	data, err := client.Set("test", true, false)
	require.NoError(t, err)

	assert.Equal(t, "example.com", data.Domain)
	assert.True(t, data.Applied)
}

func TestClient_Enable(t *testing.T) {
	server := newMockServer(t)
	defer server.close()

	server.handler = func(req *protocol.Request) *protocol.Response {
		if req.Type == protocol.RequestSet {
			var payload protocol.SetPayload
			req.ParsePayload(&payload)
			assert.True(t, payload.Enabled)

			resp, _ := protocol.NewOKResponse(protocol.SetData{Domain: "test.com", Applied: true})
			return resp
		}
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "unexpected")
	}

	client := New(server.path)
	err := client.Connect()
	require.NoError(t, err)
	defer client.Close()

	_, err = client.Enable("test")
	assert.NoError(t, err)
}

func TestClient_Disable(t *testing.T) {
	server := newMockServer(t)
	defer server.close()

	server.handler = func(req *protocol.Request) *protocol.Response {
		if req.Type == protocol.RequestSet {
			var payload protocol.SetPayload
			req.ParsePayload(&payload)
			assert.False(t, payload.Enabled)

			resp, _ := protocol.NewOKResponse(protocol.SetData{Domain: "test.com", Applied: true})
			return resp
		}
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "unexpected")
	}

	client := New(server.path)
	err := client.Connect()
	require.NoError(t, err)
	defer client.Close()

	_, err = client.Disable("test")
	assert.NoError(t, err)
}

func TestClient_Sync(t *testing.T) {
	server := newMockServer(t)
	defer server.close()

	server.handler = func(req *protocol.Request) *protocol.Response {
		if req.Type == protocol.RequestSync {
			resp, _ := protocol.NewOKResponse(map[string]bool{"synced": true})
			return resp
		}
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "unexpected")
	}

	client := New(server.path)
	err := client.Connect()
	require.NoError(t, err)
	defer client.Close()

	err = client.Sync()
	assert.NoError(t, err)
}

func TestClient_ApplyPreset(t *testing.T) {
	server := newMockServer(t)
	defer server.close()

	server.handler = func(req *protocol.Request) *protocol.Response {
		if req.Type == protocol.RequestPreset {
			var payload protocol.PresetPayload
			req.ParsePayload(&payload)
			assert.Equal(t, "local", payload.Name)

			resp, _ := protocol.NewOKResponse(map[string]string{"preset": "local"})
			return resp
		}
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "unexpected")
	}

	client := New(server.path)
	err := client.Connect()
	require.NoError(t, err)
	defer client.Close()

	err = client.ApplyPreset("local")
	assert.NoError(t, err)
}

func TestClient_Rollback(t *testing.T) {
	server := newMockServer(t)
	defer server.close()

	server.handler = func(req *protocol.Request) *protocol.Response {
		if req.Type == protocol.RequestRollback {
			var payload protocol.RollbackPayload
			req.ParsePayload(&payload)
			assert.Equal(t, "hosts.backup.bak", payload.BackupName)

			resp, _ := protocol.NewOKResponse(map[string]string{"restored": payload.BackupName})
			return resp
		}
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "unexpected")
	}

	client := New(server.path)
	err := client.Connect()
	require.NoError(t, err)
	defer client.Close()

	err = client.Rollback("hosts.backup.bak")
	assert.NoError(t, err)
}

func TestClient_ListBackups(t *testing.T) {
	server := newMockServer(t)
	defer server.close()

	server.handler = func(req *protocol.Request) *protocol.Response {
		if req.Type == protocol.RequestBackups {
			resp, _ := protocol.NewOKResponse(protocol.BackupsData{
				Backups: []protocol.BackupInfo{
					{Name: "hosts.20231201.bak", Timestamp: 1701432000, Size: 1024},
					{Name: "hosts.20231130.bak", Timestamp: 1701345600, Size: 1000},
				},
			})
			return resp
		}
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "unexpected")
	}

	client := New(server.path)
	err := client.Connect()
	require.NoError(t, err)
	defer client.Close()

	backups, err := client.ListBackups()
	require.NoError(t, err)

	assert.Len(t, backups, 2)
	assert.Equal(t, "hosts.20231201.bak", backups[0].Name)
}

func TestClient_ErrorResponse(t *testing.T) {
	server := newMockServer(t)
	defer server.close()

	server.handler = func(req *protocol.Request) *protocol.Response {
		return protocol.NewErrorResponse(protocol.ErrCodeBlockedDomain, "domain is blocked")
	}

	client := New(server.path)
	err := client.Connect()
	require.NoError(t, err)
	defer client.Close()

	_, err = client.Set("test", true, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "domain is blocked")
}

func TestClient_NotConnected(t *testing.T) {
	client := New("/nonexistent/socket.sock")

	_, err := client.Status()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestClient_Timeout(t *testing.T) {
	client := NewWithTimeout("/nonexistent.sock", 100*time.Millisecond)
	assert.Equal(t, 100*time.Millisecond, client.timeout)
}

func TestIsConnected(t *testing.T) {
	t.Run("connected", func(t *testing.T) {
		server := newMockServer(t)
		defer server.close()

		server.handler = func(req *protocol.Request) *protocol.Response {
			resp, _ := protocol.NewOKResponse(nil)
			return resp
		}

		connected := IsConnected(server.path)
		assert.True(t, connected)
	})

	t.Run("not connected", func(t *testing.T) {
		connected := IsConnected("/nonexistent/socket.sock")
		assert.False(t, connected)
	})
}

// Matrix test for request types
func TestClient_RequestTypes_Matrix(t *testing.T) {
	types := []struct {
		name    string
		reqType protocol.RequestType
		call    func(*Client) error
	}{
		{"ping", protocol.RequestPing, func(c *Client) error { return c.Ping() }},
		{"status", protocol.RequestStatus, func(c *Client) error { _, err := c.Status(); return err }},
		{"list", protocol.RequestList, func(c *Client) error { _, err := c.List(); return err }},
		{"sync", protocol.RequestSync, func(c *Client) error { return c.Sync() }},
		{"preset", protocol.RequestPreset, func(c *Client) error { return c.ApplyPreset("test") }},
		{"backups", protocol.RequestBackups, func(c *Client) error { _, err := c.ListBackups(); return err }},
	}

	for _, tt := range types {
		t.Run(tt.name, func(t *testing.T) {
			server := newMockServer(t)
			defer server.close()

			receivedType := protocol.RequestType("")
			server.handler = func(req *protocol.Request) *protocol.Response {
				receivedType = req.Type

				switch req.Type {
				case protocol.RequestStatus:
					resp, _ := protocol.NewOKResponse(protocol.StatusData{})
					return resp
				case protocol.RequestList:
					resp, _ := protocol.NewOKResponse(protocol.ListData{})
					return resp
				case protocol.RequestBackups:
					resp, _ := protocol.NewOKResponse(protocol.BackupsData{})
					return resp
				default:
					resp, _ := protocol.NewOKResponse(nil)
					return resp
				}
			}

			client := New(server.path)
			err := client.Connect()
			require.NoError(t, err)
			defer client.Close()

			_ = tt.call(client)
			assert.Equal(t, tt.reqType, receivedType)
		})
	}
}

func TestClient_Add(t *testing.T) {
	server := newMockServer(t)
	defer server.close()

	server.handler = func(req *protocol.Request) *protocol.Response {
		if req.Type == protocol.RequestAdd {
			var payload protocol.AddPayload
			req.ParsePayload(&payload)
			assert.Equal(t, "test.local", payload.Domain)
			assert.Equal(t, "127.0.0.1", payload.IP)
			assert.Equal(t, "test-local", payload.Alias)
			assert.Equal(t, "dev", payload.Group)
			assert.True(t, payload.Enabled)

			resp, _ := protocol.NewOKResponse(protocol.SetData{Domain: payload.Domain, Applied: true})
			return resp
		}
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "unexpected")
	}

	client := New(server.path)
	err := client.Connect()
	require.NoError(t, err)
	defer client.Close()

	data, err := client.Add("test.local", "127.0.0.1", "test-local", "dev", true)
	assert.NoError(t, err)
	assert.Equal(t, "test.local", data.Domain)
	assert.True(t, data.Applied)
}

func TestClient_Delete(t *testing.T) {
	server := newMockServer(t)
	defer server.close()

	server.handler = func(req *protocol.Request) *protocol.Response {
		if req.Type == protocol.RequestDelete {
			var payload protocol.DeletePayload
			req.ParsePayload(&payload)
			assert.Equal(t, "test-alias", payload.Alias)

			resp, _ := protocol.NewOKResponse(map[string]string{"deleted": payload.Alias})
			return resp
		}
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "unexpected")
	}

	client := New(server.path)
	err := client.Connect()
	require.NoError(t, err)
	defer client.Close()

	err = client.Delete("test-alias")
	assert.NoError(t, err)
}

func TestClient_AddGroup(t *testing.T) {
	server := newMockServer(t)
	defer server.close()

	server.handler = func(req *protocol.Request) *protocol.Response {
		if req.Type == protocol.RequestAddGroup {
			var payload protocol.GroupPayload
			req.ParsePayload(&payload)
			assert.Equal(t, "newgroup", payload.Name)

			resp, _ := protocol.NewOKResponse(map[string]string{"added": payload.Name})
			return resp
		}
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "unexpected")
	}

	client := New(server.path)
	err := client.Connect()
	require.NoError(t, err)
	defer client.Close()

	err = client.AddGroup("newgroup")
	assert.NoError(t, err)
}

func TestClient_DeleteGroup(t *testing.T) {
	server := newMockServer(t)
	defer server.close()

	server.handler = func(req *protocol.Request) *protocol.Response {
		if req.Type == protocol.RequestDeleteGroup {
			var payload protocol.GroupPayload
			req.ParsePayload(&payload)
			assert.Equal(t, "todelete", payload.Name)

			resp, _ := protocol.NewOKResponse(map[string]string{"deleted": payload.Name})
			return resp
		}
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "unexpected")
	}

	client := New(server.path)
	err := client.Connect()
	require.NoError(t, err)
	defer client.Close()

	err = client.DeleteGroup("todelete")
	assert.NoError(t, err)
}

func TestClient_RenameGroup(t *testing.T) {
	server := newMockServer(t)
	defer server.close()

	server.handler = func(req *protocol.Request) *protocol.Response {
		if req.Type == protocol.RequestRenameGroup {
			var payload protocol.RenameGroupPayload
			req.ParsePayload(&payload)
			assert.Equal(t, "oldname", payload.OldName)
			assert.Equal(t, "newname", payload.NewName)

			resp, _ := protocol.NewOKResponse(map[string]string{"renamed": payload.NewName})
			return resp
		}
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "unexpected")
	}

	client := New(server.path)
	err := client.Connect()
	require.NoError(t, err)
	defer client.Close()

	err = client.RenameGroup("oldname", "newname")
	assert.NoError(t, err)
}

func TestClient_ListGroups(t *testing.T) {
	server := newMockServer(t)
	defer server.close()

	server.handler = func(req *protocol.Request) *protocol.Response {
		if req.Type == protocol.RequestListGroups {
			resp, _ := protocol.NewOKResponse(protocol.GroupsData{
				Groups: []string{"dev", "staging", "prod"},
			})
			return resp
		}
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "unexpected")
	}

	client := New(server.path)
	err := client.Connect()
	require.NoError(t, err)
	defer client.Close()

	groups, err := client.ListGroups()
	require.NoError(t, err)

	assert.Equal(t, []string{"dev", "staging", "prod"}, groups)
}

func TestClient_GetBackupContent(t *testing.T) {
	server := newMockServer(t)
	defer server.close()

	expectedContent := "127.0.0.1\tlocalhost\n"

	server.handler = func(req *protocol.Request) *protocol.Response {
		if req.Type == protocol.RequestBackupContent {
			var payload protocol.BackupContentPayload
			req.ParsePayload(&payload)
			assert.Equal(t, "hosts.backup.bak", payload.BackupName)

			resp, _ := protocol.NewOKResponse(protocol.BackupContentData{
				Content: expectedContent,
			})
			return resp
		}
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "unexpected")
	}

	client := New(server.path)
	err := client.Connect()
	require.NoError(t, err)
	defer client.Close()

	content, err := client.GetBackupContent("hosts.backup.bak")
	require.NoError(t, err)

	assert.Equal(t, expectedContent, content)
}

func TestClient_AddPreset(t *testing.T) {
	server := newMockServer(t)
	defer server.close()

	server.handler = func(req *protocol.Request) *protocol.Response {
		if req.Type == protocol.RequestAddPreset {
			var payload protocol.AddPresetPayload
			req.ParsePayload(&payload)
			assert.Equal(t, "newpreset", payload.Name)
			assert.Equal(t, []string{"a", "b"}, payload.Enable)
			assert.Equal(t, []string{"c"}, payload.Disable)

			resp, _ := protocol.NewOKResponse(map[string]string{"added": payload.Name})
			return resp
		}
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "unexpected")
	}

	client := New(server.path)
	err := client.Connect()
	require.NoError(t, err)
	defer client.Close()

	err = client.AddPreset("newpreset", []string{"a", "b"}, []string{"c"})
	assert.NoError(t, err)
}

func TestClient_DeletePreset(t *testing.T) {
	server := newMockServer(t)
	defer server.close()

	server.handler = func(req *protocol.Request) *protocol.Response {
		if req.Type == protocol.RequestDeletePreset {
			var payload protocol.PresetPayload
			req.ParsePayload(&payload)
			assert.Equal(t, "todelete", payload.Name)

			resp, _ := protocol.NewOKResponse(map[string]string{"deleted": payload.Name})
			return resp
		}
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "unexpected")
	}

	client := New(server.path)
	err := client.Connect()
	require.NoError(t, err)
	defer client.Close()

	err = client.DeletePreset("todelete")
	assert.NoError(t, err)
}

func TestClient_ListPresets(t *testing.T) {
	server := newMockServer(t)
	defer server.close()

	server.handler = func(req *protocol.Request) *protocol.Response {
		if req.Type == protocol.RequestListPresets {
			resp, _ := protocol.NewOKResponse(protocol.PresetsData{
				Presets: []protocol.PresetInfo{
					{Name: "local", Enable: []string{"a"}, Disable: []string{"b"}},
					{Name: "staging", Enable: []string{"b"}, Disable: []string{"a"}},
				},
			})
			return resp
		}
		return protocol.NewErrorResponse(protocol.ErrCodeInvalidRequest, "unexpected")
	}

	client := New(server.path)
	err := client.Connect()
	require.NoError(t, err)
	defer client.Close()

	presets, err := client.ListPresets()
	require.NoError(t, err)

	assert.Len(t, presets, 2)
	assert.Equal(t, "local", presets[0].Name)
	assert.Equal(t, "staging", presets[1].Name)
}

func BenchmarkClient_Ping(b *testing.B) {
	tmpDir := b.TempDir()
	socketPath := filepath.Join(tmpDir, "bench.sock")

	listener, err := net.Listen("unix", socketPath)
	require.NoError(b, err)
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				reader := bufio.NewReader(c)
				for {
					_, err := reader.ReadBytes('\n')
					if err != nil {
						return
					}
					resp, _ := protocol.NewOKResponse(nil)
					data, _ := json.Marshal(resp)
					c.Write(append(data, '\n'))
				}
			}(conn)
		}
	}()

	client := New(socketPath)
	err = client.Connect()
	require.NoError(b, err)
	defer client.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.Ping()
	}
}
