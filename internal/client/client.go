// Package client provides a client library for communicating with the lolcathost daemon.
package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/lukaszraczylo/lolcathost/internal/protocol"
)

// Client is a client for the lolcathost daemon.
type Client struct {
	socketPath string
	conn       net.Conn
	reader     *bufio.Reader
	timeout    time.Duration
	mu         sync.Mutex
}

// New creates a new client.
func New(socketPath string) *Client {
	return &Client{
		socketPath: socketPath,
		timeout:    5 * time.Second,
	}
}

// NewWithTimeout creates a new client with a custom timeout.
func NewWithTimeout(socketPath string, timeout time.Duration) *Client {
	return &Client{
		socketPath: socketPath,
		timeout:    timeout,
	}
}

// Connect establishes a connection to the daemon.
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close existing connection if any
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
		c.reader = nil
	}

	conn, err := net.DialTimeout("unix", c.socketPath, c.timeout)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	return nil
}

// Close closes the connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		c.reader = nil
		return err
	}
	return nil
}

// send sends a request and receives a response.
func (c *Client) send(req *protocol.Request) (*protocol.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Set deadline
	_ = c.conn.SetDeadline(time.Now().Add(c.timeout))

	// Send request
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	data = append(data, '\n')

	if _, err := c.conn.Write(data); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Read response
	line, err := c.reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var resp protocol.Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &resp, nil
}

// Ping checks if the daemon is responsive.
func (c *Client) Ping() error {
	req, _ := protocol.NewRequest(protocol.RequestPing, nil)
	resp, err := c.send(req)
	if err != nil {
		return err
	}
	if !resp.IsOK() {
		return fmt.Errorf("ping failed: %s", resp.Message)
	}
	return nil
}

// Status returns the daemon's status.
func (c *Client) Status() (*protocol.StatusData, error) {
	req, _ := protocol.NewRequest(protocol.RequestStatus, nil)
	resp, err := c.send(req)
	if err != nil {
		return nil, err
	}
	if !resp.IsOK() {
		return nil, fmt.Errorf("status failed: %s", resp.Message)
	}

	var data protocol.StatusData
	if err := resp.ParseData(&data); err != nil {
		return nil, err
	}
	return &data, nil
}

// List returns all host entries.
func (c *Client) List() ([]protocol.HostEntry, error) {
	req, _ := protocol.NewRequest(protocol.RequestList, nil)
	resp, err := c.send(req)
	if err != nil {
		return nil, err
	}
	if !resp.IsOK() {
		return nil, fmt.Errorf("list failed: %s", resp.Message)
	}

	var data protocol.ListData
	if err := resp.ParseData(&data); err != nil {
		return nil, err
	}
	return data.Entries, nil
}

// Set enables or disables a host entry by alias.
func (c *Client) Set(alias string, enabled bool, force bool) (*protocol.SetData, error) {
	req, _ := protocol.NewRequest(protocol.RequestSet, protocol.SetPayload{
		Alias:   alias,
		Enabled: enabled,
		Force:   force,
	})

	resp, err := c.send(req)
	if err != nil {
		return nil, err
	}
	if !resp.IsOK() {
		return nil, fmt.Errorf("%s: %s", resp.Code, resp.Message)
	}

	var data protocol.SetData
	if err := resp.ParseData(&data); err != nil {
		return nil, err
	}
	return &data, nil
}

// Enable enables a host entry by alias.
func (c *Client) Enable(alias string) (*protocol.SetData, error) {
	return c.Set(alias, true, false)
}

// Disable disables a host entry by alias.
func (c *Client) Disable(alias string) (*protocol.SetData, error) {
	return c.Set(alias, false, false)
}

// Add adds a new host entry.
func (c *Client) Add(domain, ip, alias, group string, enabled bool) (*protocol.SetData, error) {
	req, _ := protocol.NewRequest(protocol.RequestAdd, protocol.AddPayload{
		Domain:  domain,
		IP:      ip,
		Alias:   alias,
		Group:   group,
		Enabled: enabled,
	})

	resp, err := c.send(req)
	if err != nil {
		return nil, err
	}
	if !resp.IsOK() {
		return nil, fmt.Errorf("%s: %s", resp.Code, resp.Message)
	}

	var data protocol.SetData
	if err := resp.ParseData(&data); err != nil {
		return nil, err
	}
	return &data, nil
}

// Delete removes a host entry by alias.
func (c *Client) Delete(alias string) error {
	req, _ := protocol.NewRequest(protocol.RequestDelete, protocol.DeletePayload{
		Alias: alias,
	})

	resp, err := c.send(req)
	if err != nil {
		return err
	}
	if !resp.IsOK() {
		return fmt.Errorf("%s: %s", resp.Code, resp.Message)
	}
	return nil
}

// AddGroup adds a new group.
func (c *Client) AddGroup(name string) error {
	req, _ := protocol.NewRequest(protocol.RequestAddGroup, protocol.GroupPayload{
		Name: name,
	})

	resp, err := c.send(req)
	if err != nil {
		return err
	}
	if !resp.IsOK() {
		return fmt.Errorf("%s: %s", resp.Code, resp.Message)
	}
	return nil
}

// DeleteGroup removes a group and all its hosts.
func (c *Client) DeleteGroup(name string) error {
	req, _ := protocol.NewRequest(protocol.RequestDeleteGroup, protocol.GroupPayload{
		Name: name,
	})

	resp, err := c.send(req)
	if err != nil {
		return err
	}
	if !resp.IsOK() {
		return fmt.Errorf("%s: %s", resp.Code, resp.Message)
	}
	return nil
}

// ListGroups returns all group names.
func (c *Client) ListGroups() ([]string, error) {
	req, _ := protocol.NewRequest(protocol.RequestListGroups, nil)
	resp, err := c.send(req)
	if err != nil {
		return nil, err
	}
	if !resp.IsOK() {
		return nil, fmt.Errorf("%s: %s", resp.Code, resp.Message)
	}

	var data protocol.GroupsData
	if err := resp.ParseData(&data); err != nil {
		return nil, err
	}
	return data.Groups, nil
}

// Sync synchronizes the config to the hosts file.
func (c *Client) Sync() error {
	req, _ := protocol.NewRequest(protocol.RequestSync, nil)
	resp, err := c.send(req)
	if err != nil {
		return err
	}
	if !resp.IsOK() {
		return fmt.Errorf("sync failed: %s", resp.Message)
	}
	return nil
}

// ApplyPreset applies a named preset.
func (c *Client) ApplyPreset(name string) error {
	req, _ := protocol.NewRequest(protocol.RequestPreset, protocol.PresetPayload{
		Name: name,
	})

	resp, err := c.send(req)
	if err != nil {
		return err
	}
	if !resp.IsOK() {
		return fmt.Errorf("preset failed: %s", resp.Message)
	}
	return nil
}

// Rollback restores a backup by name.
func (c *Client) Rollback(backupName string) error {
	req, _ := protocol.NewRequest(protocol.RequestRollback, protocol.RollbackPayload{
		BackupName: backupName,
	})

	resp, err := c.send(req)
	if err != nil {
		return err
	}
	if !resp.IsOK() {
		return fmt.Errorf("rollback failed: %s", resp.Message)
	}
	return nil
}

// ListBackups returns available backups.
func (c *Client) ListBackups() ([]protocol.BackupInfo, error) {
	req, _ := protocol.NewRequest(protocol.RequestBackups, nil)
	resp, err := c.send(req)
	if err != nil {
		return nil, err
	}
	if !resp.IsOK() {
		return nil, fmt.Errorf("backups failed: %s", resp.Message)
	}

	var data protocol.BackupsData
	if err := resp.ParseData(&data); err != nil {
		return nil, err
	}
	return data.Backups, nil
}

// GetBackupContent returns the content of a backup file.
func (c *Client) GetBackupContent(backupName string) (string, error) {
	req, _ := protocol.NewRequest(protocol.RequestBackupContent, protocol.BackupContentPayload{
		BackupName: backupName,
	})

	resp, err := c.send(req)
	if err != nil {
		return "", err
	}
	if !resp.IsOK() {
		return "", fmt.Errorf("backup content failed: %s", resp.Message)
	}

	var data protocol.BackupContentData
	if err := resp.ParseData(&data); err != nil {
		return "", err
	}
	return data.Content, nil
}

// RenameGroup renames a group.
func (c *Client) RenameGroup(oldName, newName string) error {
	req, _ := protocol.NewRequest(protocol.RequestRenameGroup, protocol.RenameGroupPayload{
		OldName: oldName,
		NewName: newName,
	})

	resp, err := c.send(req)
	if err != nil {
		return err
	}
	if !resp.IsOK() {
		return fmt.Errorf("%s: %s", resp.Code, resp.Message)
	}
	return nil
}

// AddPreset adds a new preset.
func (c *Client) AddPreset(name string, enable, disable []string) error {
	req, _ := protocol.NewRequest(protocol.RequestAddPreset, protocol.AddPresetPayload{
		Name:    name,
		Enable:  enable,
		Disable: disable,
	})

	resp, err := c.send(req)
	if err != nil {
		return err
	}
	if !resp.IsOK() {
		return fmt.Errorf("%s: %s", resp.Code, resp.Message)
	}
	return nil
}

// DeletePreset removes a preset by name.
func (c *Client) DeletePreset(name string) error {
	req, _ := protocol.NewRequest(protocol.RequestDeletePreset, protocol.PresetPayload{
		Name: name,
	})

	resp, err := c.send(req)
	if err != nil {
		return err
	}
	if !resp.IsOK() {
		return fmt.Errorf("%s: %s", resp.Code, resp.Message)
	}
	return nil
}

// ListPresets returns all presets.
func (c *Client) ListPresets() ([]protocol.PresetInfo, error) {
	req, _ := protocol.NewRequest(protocol.RequestListPresets, nil)
	resp, err := c.send(req)
	if err != nil {
		return nil, err
	}
	if !resp.IsOK() {
		return nil, fmt.Errorf("%s: %s", resp.Code, resp.Message)
	}

	var data protocol.PresetsData
	if err := resp.ParseData(&data); err != nil {
		return nil, err
	}
	return data.Presets, nil
}

// IsConnected checks if the daemon is reachable.
func IsConnected(socketPath string) bool {
	client := New(socketPath)
	if err := client.Connect(); err != nil {
		return false
	}
	defer client.Close()

	return client.Ping() == nil
}
