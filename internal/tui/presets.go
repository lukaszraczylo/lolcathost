// Package tui provides the preset picker component.
package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lukaszraczylo/lolcathost/internal/protocol"
)

// PresetMode represents the preset view mode.
type PresetMode int

const (
	PresetModeSelect PresetMode = iota
	PresetModeAdd
	PresetModeEdit
	PresetModeConfirmDelete
)

// PresetFormField represents a form field index.
type PresetFormField int

const (
	PresetFieldName PresetFormField = iota
	PresetFieldEnable
	PresetFieldDisable
	PresetFieldCount
)

// PresetPicker handles the preset selection and management UI.
type PresetPicker struct {
	presets  []protocol.PresetInfo
	cursor   int
	width    int
	height   int
	mode     PresetMode
	fields   []textinput.Model
	focus    PresetFormField
	editName string // Original name when editing
}

// NewPresetPicker creates a new preset picker.
func NewPresetPicker() *PresetPicker {
	fields := make([]textinput.Model, PresetFieldCount)

	// Name field
	fields[PresetFieldName] = textinput.New()
	fields[PresetFieldName].Placeholder = "preset-name"
	fields[PresetFieldName].CharLimit = 63

	// Enable field
	fields[PresetFieldEnable] = textinput.New()
	fields[PresetFieldEnable].Placeholder = "alias1,alias2,alias3"
	fields[PresetFieldEnable].CharLimit = 500

	// Disable field
	fields[PresetFieldDisable] = textinput.New()
	fields[PresetFieldDisable].Placeholder = "alias1,alias2,alias3"
	fields[PresetFieldDisable].CharLimit = 500

	return &PresetPicker{
		fields: fields,
		mode:   PresetModeSelect,
	}
}

// SetPresets updates the available presets (legacy method for compatibility).
func (p *PresetPicker) SetPresets(presets []string) {
	p.presets = make([]protocol.PresetInfo, len(presets))
	for i, name := range presets {
		p.presets[i] = protocol.PresetInfo{Name: name}
	}
	if p.cursor >= len(presets) {
		p.cursor = max(0, len(presets)-1)
	}
}

// SetPresetsWithInfo updates the available presets with full info.
func (p *PresetPicker) SetPresetsWithInfo(presets []protocol.PresetInfo) {
	p.presets = presets
	if p.cursor >= len(presets) {
		p.cursor = max(0, len(presets)-1)
	}
}

// SetSize sets the picker dimensions.
func (p *PresetPicker) SetSize(width, height int) {
	p.width = width
	p.height = height

	inputWidth := min(60, width-10)
	for i := range p.fields {
		p.fields[i].Width = inputWidth
	}
}

// MoveUp moves the cursor up.
func (p *PresetPicker) MoveUp() {
	if p.cursor > 0 {
		p.cursor--
	}
}

// MoveDown moves the cursor down.
func (p *PresetPicker) MoveDown() {
	if p.cursor < len(p.presets)-1 {
		p.cursor++
	}
}

// Selected returns the currently selected preset name.
func (p *PresetPicker) Selected() string {
	if p.cursor >= 0 && p.cursor < len(p.presets) {
		return p.presets[p.cursor].Name
	}
	return ""
}

// SelectedInfo returns the currently selected preset info.
func (p *PresetPicker) SelectedInfo() *protocol.PresetInfo {
	if p.cursor >= 0 && p.cursor < len(p.presets) {
		return &p.presets[p.cursor]
	}
	return nil
}

// Len returns the number of presets.
func (p *PresetPicker) Len() int {
	return len(p.presets)
}

// Mode returns the current mode.
func (p *PresetPicker) Mode() PresetMode {
	return p.mode
}

// SetMode sets the mode.
func (p *PresetPicker) SetMode(mode PresetMode) {
	p.mode = mode
}

// InitAdd initializes the form for adding a new preset.
func (p *PresetPicker) InitAdd() {
	p.mode = PresetModeAdd
	p.editName = ""
	for i := range p.fields {
		p.fields[i].Reset()
	}
	p.focus = PresetFieldName
	p.fields[PresetFieldName].Focus()
}

// InitEdit initializes the form for editing an existing preset.
func (p *PresetPicker) InitEdit() {
	preset := p.SelectedInfo()
	if preset == nil {
		return
	}

	p.mode = PresetModeEdit
	p.editName = preset.Name

	p.fields[PresetFieldName].SetValue(preset.Name)
	p.fields[PresetFieldEnable].SetValue(strings.Join(preset.Enable, ","))
	p.fields[PresetFieldDisable].SetValue(strings.Join(preset.Disable, ","))

	p.focus = PresetFieldName
	p.fields[PresetFieldName].Focus()
}

// InitDelete starts delete confirmation.
func (p *PresetPicker) InitDelete() {
	if p.SelectedInfo() == nil {
		return
	}
	p.mode = PresetModeConfirmDelete
}

// CancelForm cancels the current form operation.
func (p *PresetPicker) CancelForm() {
	p.mode = PresetModeSelect
	p.editName = ""
	for i := range p.fields {
		p.fields[i].Reset()
		p.fields[i].Blur()
	}
}

// Update handles input events for form mode.
func (p *PresetPicker) Update(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "tab", "down":
		p.nextField()
		return nil
	case "shift+tab", "up":
		p.prevField()
		return nil
	}

	// Update the focused field
	var cmd tea.Cmd
	p.fields[p.focus], cmd = p.fields[p.focus].Update(msg)
	return cmd
}

func (p *PresetPicker) nextField() {
	p.fields[p.focus].Blur()
	p.focus = (p.focus + 1) % PresetFieldCount
	p.fields[p.focus].Focus()
}

func (p *PresetPicker) prevField() {
	p.fields[p.focus].Blur()
	p.focus = (p.focus - 1 + PresetFieldCount) % PresetFieldCount
	p.fields[p.focus].Focus()
}

// FormValues returns the form values (name, enable list, disable list).
func (p *PresetPicker) FormValues() (name string, enable, disable []string) {
	name = strings.TrimSpace(p.fields[PresetFieldName].Value())

	enableStr := strings.TrimSpace(p.fields[PresetFieldEnable].Value())
	if enableStr != "" {
		for _, s := range strings.Split(enableStr, ",") {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				enable = append(enable, trimmed)
			}
		}
	}

	disableStr := strings.TrimSpace(p.fields[PresetFieldDisable].Value())
	if disableStr != "" {
		for _, s := range strings.Split(disableStr, ",") {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				disable = append(disable, trimmed)
			}
		}
	}

	return name, enable, disable
}

// EditName returns the original name when editing.
func (p *PresetPicker) EditName() string {
	return p.editName
}

// IsEdit returns true if in edit mode.
func (p *PresetPicker) IsEdit() bool {
	return p.mode == PresetModeEdit
}

// ValidateForm validates the form values.
func (p *PresetPicker) ValidateForm() string {
	name, enable, disable := p.FormValues()

	if name == "" {
		return "Preset name is required"
	}
	if len(enable) == 0 && len(disable) == 0 {
		return "At least one alias to enable or disable is required"
	}

	return ""
}

// View renders the preset picker.
func (p *PresetPicker) View() string {
	switch p.mode {
	case PresetModeAdd, PresetModeEdit:
		return p.formView()
	case PresetModeConfirmDelete:
		return p.deleteView()
	default:
		return p.selectView()
	}
}

func (p *PresetPicker) selectView() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Presets"))
	sb.WriteString("\n\n")

	if len(p.presets) == 0 {
		sb.WriteString(helpDescStyle.Render("No presets configured."))
		sb.WriteString("\n\n")
		sb.WriteString(helpDescStyle.Render("Press 'n' to create one"))
	} else {
		for i, preset := range p.presets {
			if i == p.cursor {
				sb.WriteString(presetSelectedStyle.Render("▸ " + preset.Name))
			} else {
				sb.WriteString(presetItemStyle.Render("  " + preset.Name))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n\n")
	sb.WriteString(helpDescStyle.Render("↑↓ navigate • Enter apply • n new • e edit • d delete • Esc cancel"))

	return dialogStyle.Render(sb.String())
}

func (p *PresetPicker) formView() string {
	var sb strings.Builder

	title := "Add New Preset"
	if p.mode == PresetModeEdit {
		title = "Edit Preset"
	}

	sb.WriteString(titleStyle.Render(title))
	sb.WriteString("\n\n")

	labels := []string{"Name:", "Enable aliases (comma-separated):", "Disable aliases (comma-separated):"}

	for i, label := range labels {
		sb.WriteString(inputLabelStyle.Render(label))
		sb.WriteString("\n")

		style := inputStyle
		if PresetFormField(i) == p.focus {
			style = inputFocusStyle
		}

		sb.WriteString(style.Render(p.fields[i].View()))
		sb.WriteString("\n\n")
	}

	sb.WriteString("\n")
	sb.WriteString(helpDescStyle.Render("Tab/↓ next • Shift+Tab/↑ prev • Enter save • Esc cancel"))

	return dialogStyle.Render(sb.String())
}

func (p *PresetPicker) deleteView() string {
	var sb strings.Builder

	preset := p.SelectedInfo()
	presetName := ""
	if preset != nil {
		presetName = preset.Name
	}

	sb.WriteString(titleStyle.Render("Delete Preset"))
	sb.WriteString("\n\n")
	sb.WriteString(errorMsgStyle.Render("Are you sure you want to delete preset '" + presetName + "'?"))
	sb.WriteString("\n\n")
	sb.WriteString(helpDescStyle.Render("y confirm • n/Esc cancel"))

	return dialogStyle.Render(sb.String())
}
