// Package tui provides the main Bubble Tea application.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lukaszraczylo/lolcathost/internal/client"
	"github.com/lukaszraczylo/lolcathost/internal/config"
	"github.com/lukaszraczylo/lolcathost/internal/protocol"
	"github.com/lukaszraczylo/lolcathost/internal/version"
)

// ViewMode represents the current view mode.
type ViewMode int

const (
	ViewList ViewMode = iota
	ViewForm
	ViewPresets
	ViewGroups
	ViewBackups
	ViewHelp
	ViewSearch
	ViewConfirmDelete
)

// Model is the main Bubble Tea model.
type Model struct {
	// Client
	client    *client.Client
	connected bool

	// Views
	mode         ViewMode
	list         *ListView
	form         *Form
	presetPicker *PresetPicker
	groupPicker  *GroupPicker
	backupPicker *BackupPicker
	searchInput  textinput.Model

	// State
	width              int
	height             int
	message            string
	messageStyle       string // "error" or "success"
	messageTime        time.Time
	searchTerm         string
	allGroups          []string // All groups including empty ones
	pendingDeleteAlias string   // Alias of host pending delete confirmation

	// Update notification
	updateAvailable bool
	updateVersion   string
	updateURL       string

	// Version info for update checking
	version     string
	githubOwner string
	githubRepo  string
}

// Message types
type (
	connectMsg struct{ err error }
	refreshMsg struct {
		entries []protocol.HostEntry
		err     error
	}
	toggleMsg struct {
		alias string
		err   error
	}
	presetMsg struct {
		name string
		err  error
	}
	addMsg struct {
		domain string
		err    error
	}
	deleteMsg struct {
		alias string
		err   error
	}
	addPresetMsg struct {
		name string
		err  error
	}
	deletePresetMsg struct {
		name string
		err  error
	}
	refreshPresetsMsg struct {
		presets []protocol.PresetInfo
		err     error
	}
	addGroupMsg struct {
		name string
		err  error
	}
	renameGroupMsg struct {
		name string
		err  error
	}
	deleteGroupMsg struct {
		name string
		err  error
	}
	refreshGroupsMsg struct {
		groups []string
		err    error
	}
	rollbackMsg struct {
		name string
		err  error
	}
	refreshBackupsMsg struct {
		backups []protocol.BackupInfo
		err     error
	}
	backupContentMsg struct {
		content string
		err     error
	}
	clearMsgMsg struct{}
	tickMsg     struct{}
	updateMsg   struct {
		version string
		url     string
	}
)

// NewModel creates a new TUI model.
func NewModel(socketPath string) *Model {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search..."
	searchInput.CharLimit = 100
	searchInput.Width = 50

	return &Model{
		client:       client.New(socketPath),
		list:         NewListView(),
		form:         NewForm(),
		presetPicker: NewPresetPicker(),
		groupPicker:  NewGroupPicker(),
		backupPicker: NewBackupPicker(),
		searchInput:  searchInput,
		mode:         ViewList,
	}
}

// Init initializes the model.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.connect(),
		tea.SetWindowTitle("lolcathost"),
		m.tick(),
		m.checkForUpdate(),
	)
}

func (m *Model) connect() tea.Cmd {
	return func() tea.Msg {
		if err := m.client.Connect(); err != nil {
			return connectMsg{err: err}
		}
		return connectMsg{err: nil}
	}
}

func (m *Model) refresh() tea.Cmd {
	return func() tea.Msg {
		entries, err := m.client.List()
		if err != nil {
			return refreshMsg{entries: nil, err: err}
		}
		return refreshMsg{entries: entries, err: nil}
	}
}

func (m *Model) toggle(alias string, enabled bool) tea.Cmd {
	return func() tea.Msg {
		_, err := m.client.Set(alias, enabled, false)
		return toggleMsg{alias: alias, err: err}
	}
}

func (m *Model) applyPreset(name string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.ApplyPreset(name)
		return presetMsg{name: name, err: err}
	}
}

func (m *Model) addHost(domain, ip, alias, group string) tea.Cmd {
	return func() tea.Msg {
		_, err := m.client.Add(domain, ip, alias, group, false)
		return addMsg{domain: domain, err: err}
	}
}

func (m *Model) deleteHost(alias string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.Delete(alias)
		return deleteMsg{alias: alias, err: err}
	}
}

func (m *Model) addPreset(name string, enable, disable []string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.AddPreset(name, enable, disable)
		return addPresetMsg{name: name, err: err}
	}
}

func (m *Model) deletePreset(name string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.DeletePreset(name)
		return deletePresetMsg{name: name, err: err}
	}
}

func (m *Model) refreshPresets() tea.Cmd {
	return func() tea.Msg {
		presets, err := m.client.ListPresets()
		return refreshPresetsMsg{presets: presets, err: err}
	}
}

func (m *Model) addGroup(name string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.AddGroup(name)
		return addGroupMsg{name: name, err: err}
	}
}

func (m *Model) renameGroup(oldName, newName string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.RenameGroup(oldName, newName)
		return renameGroupMsg{name: newName, err: err}
	}
}

func (m *Model) deleteGroup(name string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.DeleteGroup(name)
		return deleteGroupMsg{name: name, err: err}
	}
}

func (m *Model) refreshGroups() tea.Cmd {
	return func() tea.Msg {
		groups, err := m.client.ListGroups()
		return refreshGroupsMsg{groups: groups, err: err}
	}
}

func (m *Model) rollback(backupName string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.Rollback(backupName)
		return rollbackMsg{name: backupName, err: err}
	}
}

func (m *Model) refreshBackups() tea.Cmd {
	return func() tea.Msg {
		backups, err := m.client.ListBackups()
		return refreshBackupsMsg{backups: backups, err: err}
	}
}

func (m *Model) fetchBackupContent(backupName string) tea.Cmd {
	return func() tea.Msg {
		content, err := m.client.GetBackupContent(backupName)
		return backupContentMsg{content: content, err: err}
	}
}

func (m *Model) tick() tea.Cmd {
	return tea.Tick(time.Second*3, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m *Model) clearMsg() tea.Cmd {
	return tea.Tick(time.Second*3, func(t time.Time) tea.Msg {
		return clearMsgMsg{}
	})
}

func (m *Model) checkForUpdate() tea.Cmd {
	if m.githubOwner == "" || m.githubRepo == "" {
		return nil
	}
	return func() tea.Msg {
		checker := version.NewChecker(m.githubOwner, m.githubRepo, m.version)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if update := checker.CheckForUpdate(ctx); update != nil {
			return updateMsg{version: update.LatestVersion, url: update.ReleaseURL}
		}
		return nil
	}
}

// Update handles messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-10)
		m.form.SetSize(msg.Width, msg.Height)
		m.presetPicker.SetSize(msg.Width, msg.Height)
		m.groupPicker.SetSize(msg.Width, msg.Height)
		m.backupPicker.SetSize(msg.Width, msg.Height)
		// Set search input width
		searchWidth := msg.Width - 20
		if searchWidth > 60 {
			searchWidth = 60
		}
		m.searchInput.Width = searchWidth

	case tea.KeyMsg:
		cmd := m.handleKey(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case connectMsg:
		if msg.err != nil {
			m.connected = false
			m.setError(fmt.Sprintf("Failed to connect: %v", msg.err))
		} else {
			m.connected = true
			cmds = append(cmds, m.refresh())
			cmds = append(cmds, m.refreshPresets())
			cmds = append(cmds, m.refreshGroups())
		}

	case refreshMsg:
		if msg.err != nil {
			m.setError(fmt.Sprintf("Refresh failed: %v", msg.err))
			// Mark as disconnected to trigger reconnect
			m.connected = false
			m.client.Close()
		} else {
			// Always update the list, even if entries is nil/empty
			m.list.SetItems(msg.entries)
		}

	case toggleMsg:
		if msg.err != nil {
			m.list.SetError(msg.alias, true)
			m.setError(fmt.Sprintf("Toggle failed: %v", msg.err))
		} else {
			m.list.SetPending(msg.alias, false)
			cmds = append(cmds, m.refresh())
			m.setSuccess("Entry toggled")
		}

	case presetMsg:
		if msg.err != nil {
			m.setError(fmt.Sprintf("Preset failed: %v", msg.err))
		} else {
			cmds = append(cmds, m.refresh())
			m.setSuccess(fmt.Sprintf("Applied preset: %s", msg.name))
		}
		m.mode = ViewList

	case addMsg:
		if msg.err != nil {
			m.setError(fmt.Sprintf("Add failed: %v", msg.err))
		} else {
			cmds = append(cmds, m.refresh())
			m.setSuccess(fmt.Sprintf("Added host: %s", msg.domain))
		}
		m.mode = ViewList

	case deleteMsg:
		// Clear pending state regardless of success/failure
		m.list.SetPending(msg.alias, false)
		if msg.err != nil {
			m.list.SetError(msg.alias, true)
			m.setError(fmt.Sprintf("Delete failed: %v", msg.err))
		} else {
			cmds = append(cmds, m.refresh())
			m.setSuccess(fmt.Sprintf("Deleted: %s", msg.alias))
		}

	case addPresetMsg:
		if msg.err != nil {
			m.setError(fmt.Sprintf("Add preset failed: %v", msg.err))
		} else {
			cmds = append(cmds, m.refreshPresets())
			m.setSuccess(fmt.Sprintf("Added preset: %s", msg.name))
		}
		m.presetPicker.CancelForm()

	case deletePresetMsg:
		if msg.err != nil {
			m.setError(fmt.Sprintf("Delete preset failed: %v", msg.err))
		} else {
			cmds = append(cmds, m.refreshPresets())
			m.setSuccess(fmt.Sprintf("Deleted preset: %s", msg.name))
		}
		m.presetPicker.CancelForm()

	case refreshPresetsMsg:
		if msg.err == nil && msg.presets != nil {
			m.presetPicker.SetPresetsWithInfo(msg.presets)
		}

	case addGroupMsg:
		if msg.err != nil {
			m.setError(fmt.Sprintf("Add group failed: %v", msg.err))
		} else {
			cmds = append(cmds, m.refreshGroups())
			cmds = append(cmds, m.refresh()) // Refresh list to show new group
			m.setSuccess(fmt.Sprintf("Added group: %s", msg.name))
		}
		m.groupPicker.CancelForm()

	case renameGroupMsg:
		if msg.err != nil {
			m.setError(fmt.Sprintf("Rename group failed: %v", msg.err))
		} else {
			cmds = append(cmds, m.refreshGroups())
			cmds = append(cmds, m.refresh())
			m.setSuccess(fmt.Sprintf("Renamed group to: %s", msg.name))
		}
		m.groupPicker.CancelForm()

	case deleteGroupMsg:
		if msg.err != nil {
			m.setError(fmt.Sprintf("Delete group failed: %v", msg.err))
		} else {
			cmds = append(cmds, m.refreshGroups())
			cmds = append(cmds, m.refresh())
			m.setSuccess(fmt.Sprintf("Deleted group: %s", msg.name))
		}
		m.groupPicker.CancelForm()

	case refreshGroupsMsg:
		if msg.err == nil && msg.groups != nil {
			m.allGroups = msg.groups
			m.groupPicker.SetGroups(msg.groups)
		}

	case rollbackMsg:
		if msg.err != nil {
			m.setError(fmt.Sprintf("Rollback failed: %v", msg.err))
		} else {
			cmds = append(cmds, m.refresh())
			m.setSuccess("Restored from backup")
		}
		m.backupPicker.Cancel()
		m.mode = ViewList

	case refreshBackupsMsg:
		if msg.err == nil && msg.backups != nil {
			m.backupPicker.SetBackups(msg.backups)
			// Fetch content for the first backup
			if len(msg.backups) > 0 {
				cmds = append(cmds, m.fetchBackupContent(msg.backups[0].Name))
			}
		}

	case backupContentMsg:
		if msg.err == nil {
			m.backupPicker.SetPreviewContent(msg.content)
		}

	case clearMsgMsg:
		if time.Since(m.messageTime) >= time.Second*3 {
			m.message = ""
		}

	case tickMsg:
		// Reconnect if disconnected
		if !m.connected {
			cmds = append(cmds, m.connect())
		}
		cmds = append(cmds, m.tick())

	case updateMsg:
		if msg.version != "" {
			m.updateAvailable = true
			m.updateVersion = msg.version
			m.updateURL = msg.url
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	// Global keys
	switch msg.String() {
	case "ctrl+c":
		return tea.Quit
	}

	// Mode-specific keys
	switch m.mode {
	case ViewList:
		return m.handleListKey(msg)
	case ViewForm:
		return m.handleFormKey(msg)
	case ViewPresets:
		return m.handlePresetKey(msg)
	case ViewGroups:
		return m.handleGroupKey(msg)
	case ViewBackups:
		return m.handleBackupKey(msg)
	case ViewHelp:
		return m.handleHelpKey(msg)
	case ViewSearch:
		return m.handleSearchKey(msg)
	case ViewConfirmDelete:
		return m.handleConfirmDeleteKey(msg)
	}

	return nil
}

func (m *Model) handleListKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "q":
		return tea.Quit
	case "esc":
		// Clear search if active
		if m.searchTerm != "" {
			m.searchTerm = ""
			m.searchInput.Reset()
		}
	case "up", "k":
		m.list.MoveUp()
	case "down", "j":
		m.list.MoveDown()
	case " ", "enter":
		return m.toggleSelected()
	case "n":
		m.mode = ViewForm
		m.form.SetGroups(m.allGroups)
		m.form.Init()
	case "e":
		if item := m.list.Selected(); item != nil {
			m.mode = ViewForm
			m.form.SetGroups(m.allGroups)
			m.form.InitEdit(item.Entry.Domain, item.Entry.IP, item.Entry.Alias, item.Entry.Group)
		}
	case "d":
		if item := m.list.Selected(); item != nil {
			m.pendingDeleteAlias = item.Entry.Alias
			m.mode = ViewConfirmDelete
		}
	case "p":
		m.mode = ViewPresets
		// Pass available aliases to preset picker
		m.presetPicker.SetAvailableAliases(m.list.GetAliases())
	case "g":
		m.mode = ViewGroups
		return m.refreshGroups()
	case "b":
		m.mode = ViewBackups
		return m.refreshBackups()
	case "/":
		m.mode = ViewSearch
		m.searchInput.Focus()
	case "?":
		m.mode = ViewHelp
	case "r":
		return m.refresh()
	}
	return nil
}

func (m *Model) handleFormKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.mode = ViewList
		return nil
	case "enter":
		if errMsg := m.form.Validate(); errMsg != "" {
			m.setError(errMsg)
			return m.clearMsg()
		}
		domain, ip, group := m.form.Values()
		if m.form.IsEdit() {
			// For edit, delete old and add new (simple approach)
			oldAlias := m.form.EditAlias()
			return tea.Sequence(
				func() tea.Msg {
					m.client.Delete(oldAlias)
					return nil
				},
				m.addHost(domain, ip, "", group), // Empty alias = auto-generate
			)
		}
		return m.addHost(domain, ip, "", group) // Empty alias = auto-generate
	}

	return m.form.Update(msg)
}

func (m *Model) handlePresetKey(msg tea.KeyMsg) tea.Cmd {
	// Handle based on preset picker mode
	switch m.presetPicker.Mode() {
	case PresetModeSelect:
		return m.handlePresetSelectKey(msg)
	case PresetModeAdd, PresetModeEdit:
		return m.handlePresetFormKey(msg)
	case PresetModePickEnable, PresetModePickDisable:
		return m.handlePresetPickerKey(msg)
	case PresetModeConfirmDelete:
		return m.handlePresetDeleteKey(msg)
	}
	return nil
}

func (m *Model) handlePresetSelectKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "q":
		m.mode = ViewList
	case "up", "k":
		m.presetPicker.MoveUp()
	case "down", "j":
		m.presetPicker.MoveDown()
	case "enter":
		if preset := m.presetPicker.Selected(); preset != "" {
			return m.applyPreset(preset)
		}
	case "n":
		m.presetPicker.InitAdd()
	case "e":
		m.presetPicker.InitEdit()
	case "d":
		m.presetPicker.InitDelete()
	}
	return nil
}

func (m *Model) handlePresetFormKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.presetPicker.CancelForm()
		return nil
	case "enter":
		// Check which field is focused
		switch m.presetPicker.Focus() {
		case PresetFieldEnable:
			m.presetPicker.OpenEnablePicker()
			return nil
		case PresetFieldDisable:
			m.presetPicker.OpenDisablePicker()
			return nil
		case PresetFieldSave:
			// Save the preset
			if errMsg := m.presetPicker.ValidateForm(); errMsg != "" {
				m.setError(errMsg)
				return m.clearMsg()
			}
			name, enable, disable := m.presetPicker.FormValues()
			if m.presetPicker.IsEdit() {
				// For edit, delete old and add new
				oldName := m.presetPicker.EditName()
				return tea.Sequence(
					func() tea.Msg {
						m.client.DeletePreset(oldName)
						return nil
					},
					m.addPreset(name, enable, disable),
				)
			}
			return m.addPreset(name, enable, disable)
		}
	}
	return m.presetPicker.Update(msg)
}

func (m *Model) handlePresetPickerKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.presetPicker.ClosePicker()
	case "enter":
		m.presetPicker.ClosePicker()
	case "up", "k":
		m.presetPicker.PickerMoveUp()
	case "down", "j":
		m.presetPicker.PickerMoveDown()
	case " ":
		m.presetPicker.TogglePickerSelection()
	}
	return nil
}

func (m *Model) handlePresetDeleteKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "y", "Y":
		if preset := m.presetPicker.Selected(); preset != "" {
			return m.deletePreset(preset)
		}
		m.presetPicker.CancelForm()
	case "n", "N", "esc":
		m.presetPicker.CancelForm()
	}
	return nil
}

func (m *Model) handleGroupKey(msg tea.KeyMsg) tea.Cmd {
	// Handle based on group picker mode
	switch m.groupPicker.Mode() {
	case GroupModeSelect:
		return m.handleGroupSelectKey(msg)
	case GroupModeAdd, GroupModeRename:
		return m.handleGroupFormKey(msg)
	case GroupModeConfirmDelete:
		return m.handleGroupDeleteKey(msg)
	}
	return nil
}

func (m *Model) handleGroupSelectKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "q":
		m.mode = ViewList
	case "up", "k":
		m.groupPicker.MoveUp()
	case "down", "j":
		m.groupPicker.MoveDown()
	case "n":
		m.groupPicker.InitAdd()
	case "r":
		m.groupPicker.InitRename()
	case "d":
		m.groupPicker.InitDelete()
	}
	return nil
}

func (m *Model) handleGroupFormKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.groupPicker.CancelForm()
		return nil
	case "enter":
		if errMsg := m.groupPicker.ValidateForm(); errMsg != "" {
			m.setError(errMsg)
			return m.clearMsg()
		}
		name := m.groupPicker.FormValue()
		if m.groupPicker.IsRename() {
			oldName := m.groupPicker.EditName()
			return m.renameGroup(oldName, name)
		}
		return m.addGroup(name)
	}
	return m.groupPicker.Update(msg)
}

func (m *Model) handleGroupDeleteKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "y", "Y":
		if group := m.groupPicker.Selected(); group != "" {
			return m.deleteGroup(group)
		}
		m.groupPicker.CancelForm()
	case "n", "N", "esc":
		m.groupPicker.CancelForm()
	}
	return nil
}

func (m *Model) handleBackupKey(msg tea.KeyMsg) tea.Cmd {
	switch m.backupPicker.Mode() {
	case BackupModeSelect:
		return m.handleBackupSelectKey(msg)
	case BackupModeConfirmRestore:
		return m.handleBackupRestoreKey(msg)
	}
	return nil
}

func (m *Model) handleBackupSelectKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "q":
		m.mode = ViewList
	case "up", "k":
		m.backupPicker.MoveUp()
		// Fetch content for newly selected backup
		if backup := m.backupPicker.Selected(); backup != "" && m.backupPicker.PreviewContent() == "" {
			return m.fetchBackupContent(backup)
		}
	case "down", "j":
		m.backupPicker.MoveDown()
		// Fetch content for newly selected backup
		if backup := m.backupPicker.Selected(); backup != "" && m.backupPicker.PreviewContent() == "" {
			return m.fetchBackupContent(backup)
		}
	case "shift+up", "K":
		m.backupPicker.ScrollPreviewUp()
	case "shift+down", "J":
		m.backupPicker.ScrollPreviewDown()
	case "enter":
		m.backupPicker.InitRestore()
	case "r":
		return m.refreshBackups()
	}
	return nil
}

func (m *Model) handleBackupRestoreKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "y", "Y":
		if backup := m.backupPicker.Selected(); backup != "" {
			return m.rollback(backup)
		}
		m.backupPicker.Cancel()
	case "n", "N", "esc":
		m.backupPicker.Cancel()
	}
	return nil
}

func (m *Model) handleHelpKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "q", "?":
		m.mode = ViewList
	}
	return nil
}

func (m *Model) handleSearchKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.mode = ViewList
		m.searchTerm = ""
		m.searchInput.Reset()
		return nil
	case "enter":
		m.searchTerm = m.searchInput.Value()
		m.mode = ViewList
		return nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return cmd
}

func (m *Model) handleConfirmDeleteKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "y", "Y":
		alias := m.pendingDeleteAlias
		m.pendingDeleteAlias = ""
		m.mode = ViewList
		// Set pending state for visual feedback
		m.list.SetPending(alias, true)
		return m.deleteHost(alias)
	case "n", "N", "esc":
		m.pendingDeleteAlias = ""
		m.mode = ViewList
		return nil
	}
	return nil
}

func (m *Model) toggleSelected() tea.Cmd {
	item := m.list.Selected()
	if item == nil {
		return nil
	}

	m.list.SetPending(item.Entry.Alias, true)
	return m.toggle(item.Entry.Alias, !item.Entry.Enabled)
}

func (m *Model) setError(msg string) {
	m.message = msg
	m.messageStyle = "error"
	m.messageTime = time.Now()
}

func (m *Model) setSuccess(msg string) {
	m.message = msg
	m.messageStyle = "success"
	m.messageTime = time.Now()
}

// View renders the UI.
func (m *Model) View() string {
	var sb strings.Builder

	// Title with version
	title := titleStyle.Render("lolcathost - Host Management")
	sb.WriteString(title)

	// Update notification
	if m.updateAvailable {
		sb.WriteString("  ")
		sb.WriteString(updateStyle.Render(fmt.Sprintf("Update available: v%s", m.updateVersion)))
	}

	sb.WriteString("\n\n")

	// Main content based on mode
	switch m.mode {
	case ViewList:
		sb.WriteString(m.list.ViewFiltered(m.searchTerm))
	case ViewForm:
		sb.WriteString(m.form.View())
	case ViewPresets:
		sb.WriteString(m.presetPicker.View())
	case ViewGroups:
		sb.WriteString(m.groupPicker.View())
	case ViewBackups:
		sb.WriteString(m.backupPicker.View())
	case ViewHelp:
		sb.WriteString(m.helpView())
	case ViewSearch:
		sb.WriteString(m.searchView())
	case ViewConfirmDelete:
		sb.WriteString(m.confirmDeleteView())
	}

	// Message
	if m.message != "" {
		sb.WriteString("\n")
		if m.messageStyle == "error" {
			sb.WriteString(errorMsgStyle.Render(m.message))
		} else {
			sb.WriteString(successMsgStyle.Render(m.message))
		}
	}

	// Calculate remaining space for footer positioning
	currentContent := sb.String()
	currentLines := strings.Count(currentContent, "\n") + 1

	// Calculate footer height dynamically (help bar lines + status bar + spacing)
	footerHeight := 2 // status bar + newline before it
	var helpBarContent string
	if m.mode == ViewList {
		helpBarContent = m.helpBar()
		helpBarLines := strings.Count(helpBarContent, "\n") + 1
		footerHeight += helpBarLines + 1 // +1 for newline before help bar
	}

	remainingLines := m.height - currentLines - footerHeight
	if remainingLines > 0 {
		sb.WriteString(strings.Repeat("\n", remainingLines))
	}

	// Footer (help bar + status bar)
	if m.mode == ViewList {
		sb.WriteString("\n")
		sb.WriteString(helpBarContent)
	}
	sb.WriteString("\n")
	sb.WriteString(m.statusBar())

	return sb.String()
}

func (m *Model) helpBar() string {
	// Define help items with their display widths (without ANSI codes)
	type helpItem struct {
		key      string
		desc     string
		rawWidth int // width without ANSI escape codes
	}

	items := []helpItem{
		{"↑↓/jk", "Navigate", 13},
		{"Space", "Toggle", 13},
		{"n", "New", 6},
		{"e", "Edit", 7},
		{"d", "Delete", 9},
		{"p", "Presets", 10},
		{"g", "Groups", 9},
		{"b", "Backups", 10},
		{"/", "Search", 9},
		{"?", "Help", 7},
		{"q", "Quit", 7},
	}

	separator := "  "
	sepWidth := 2

	var lines []string
	var currentLine string
	var currentWidth int

	for i, item := range items {
		rendered := helpKeyStyle.Render(item.key) + ": " + item.desc

		// Check if adding this item would exceed width
		newWidth := currentWidth + item.rawWidth
		if currentWidth > 0 {
			newWidth += sepWidth
		}

		if m.width > 0 && newWidth > m.width && currentWidth > 0 {
			// Start a new line
			lines = append(lines, currentLine)
			currentLine = rendered
			currentWidth = item.rawWidth
		} else {
			// Add to current line
			if currentWidth > 0 {
				currentLine += separator
			}
			currentLine += rendered
			currentWidth = newWidth
			if i == 0 {
				currentWidth = item.rawWidth
			}
		}
	}

	// Add the last line
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return helpBarStyle.Render(strings.Join(lines, "\n"))
}

func (m *Model) statusBar() string {
	var status string
	if m.connected {
		status = connectedStyle.String()
	} else {
		status = disconnectedStyle.String()
	}

	active := fmt.Sprintf("%d active", m.list.ActiveCount())
	total := fmt.Sprintf("%d total", m.list.Len())

	return statusBarStyle.Render(fmt.Sprintf("%s  |  %s  |  %s", status, active, total))
}

func (m *Model) helpView() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Help"))
	sb.WriteString("\n\n")

	help := []struct{ key, desc string }{
		{"↑/↓ or j/k", "Navigate up/down"},
		{"Space/Enter", "Toggle entry on/off"},
		{"n", "Add new entry"},
		{"e", "Edit selected entry"},
		{"d", "Delete selected entry"},
		{"p", "Open preset manager"},
		{"g", "Open group manager"},
		{"b", "Open backup manager"},
		{"/", "Search"},
		{"r", "Refresh list"},
		{"?", "Toggle this help"},
		{"q", "Quit"},
	}

	for _, h := range help {
		sb.WriteString(fmt.Sprintf("  %s  %s\n",
			helpKeyStyle.Width(15).Render(h.key),
			helpDescStyle.Render(h.desc)))
	}

	// Show blocked domains
	sb.WriteString("\n")
	sb.WriteString(inputLabelStyle.Render("Blocked Domains:"))
	sb.WriteString("\n")
	blockedDomains := config.GetBlockedDomains()
	sb.WriteString(helpDescStyle.Render("  " + strings.Join(blockedDomains, ", ")))
	sb.WriteString("\n")

	sb.WriteString("\n")
	sb.WriteString(helpDescStyle.Render("Press ? or Esc to close"))

	return dialogStyle.Render(sb.String())
}

func (m *Model) searchView() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Search"))
	sb.WriteString("\n\n")

	sb.WriteString(inputFocusStyle.Render(m.searchInput.View()))
	sb.WriteString("\n\n")
	sb.WriteString(helpDescStyle.Render("Enter to search • Esc to cancel"))

	return dialogStyle.Render(sb.String())
}

func (m *Model) confirmDeleteView() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Confirm Delete"))
	sb.WriteString("\n\n")

	// Find the entry details for the pending delete
	var domain, ip string
	for _, item := range m.list.items {
		if item.Entry.Alias == m.pendingDeleteAlias {
			domain = item.Entry.Domain
			ip = item.Entry.IP
			break
		}
	}

	warningStyle := lipgloss.NewStyle().Foreground(colorWarning).Bold(true)
	sb.WriteString(warningStyle.Render("Are you sure you want to delete this host?"))
	sb.WriteString("\n\n")

	sb.WriteString(fmt.Sprintf("  Alias:  %s\n", helpKeyStyle.Render(m.pendingDeleteAlias)))
	sb.WriteString(fmt.Sprintf("  Domain: %s\n", helpDescStyle.Render(domain)))
	sb.WriteString(fmt.Sprintf("  IP:     %s\n", helpDescStyle.Render(ip)))

	sb.WriteString("\n")
	sb.WriteString(helpDescStyle.Render("y confirm • n/Esc cancel"))

	return dialogStyle.Render(sb.String())
}

// Run starts the TUI application.
func Run(socketPath string) error {
	return RunWithVersion(socketPath, "dev", "", "")
}

// RunWithVersion starts the TUI application with version info for update checking.
func RunWithVersion(socketPath, version, githubOwner, githubRepo string) error {
	m := NewModel(socketPath)
	m.version = version
	m.githubOwner = githubOwner
	m.githubRepo = githubRepo
	p := tea.NewProgram(m, tea.WithAltScreen())

	_, err := p.Run()
	return err
}
