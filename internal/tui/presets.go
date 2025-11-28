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
	PresetModePickEnable  // Multi-select picker for enable aliases
	PresetModePickDisable // Multi-select picker for disable aliases
)

// PresetFormField represents a form field index.
type PresetFormField int

const (
	PresetFieldName PresetFormField = iota
	PresetFieldEnable
	PresetFieldDisable
	PresetFieldSave
	PresetFieldCount
)

// PresetPicker handles the preset selection and management UI.
type PresetPicker struct {
	presets          []protocol.PresetInfo
	cursor           int
	width            int
	height           int
	mode             PresetMode
	fields           []textinput.Model
	focus            PresetFormField
	editName         string   // Original name when editing
	availableAliases []string // Available host aliases for reference

	// Multi-select picker state
	pickerCursor    int
	selectedEnable  map[string]bool
	selectedDisable map[string]bool
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
		fields:          fields,
		mode:            PresetModeSelect,
		selectedEnable:  make(map[string]bool),
		selectedDisable: make(map[string]bool),
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

// SetAvailableAliases sets the list of available host aliases for reference.
func (p *PresetPicker) SetAvailableAliases(aliases []string) {
	p.availableAliases = aliases
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

// Focus returns the currently focused form field.
func (p *PresetPicker) Focus() PresetFormField {
	return p.focus
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
	// Clear selections
	p.selectedEnable = make(map[string]bool)
	p.selectedDisable = make(map[string]bool)
	p.pickerCursor = 0
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

	// Initialize selections from preset (only include aliases that exist)
	p.selectedEnable = make(map[string]bool)
	p.selectedDisable = make(map[string]bool)
	for _, alias := range p.filterExistingAliases(preset.Enable) {
		p.selectedEnable[alias] = true
	}
	for _, alias := range p.filterExistingAliases(preset.Disable) {
		p.selectedDisable[alias] = true
	}
	p.pickerCursor = 0

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

	// Only update text field if focused on name
	if p.focus == PresetFieldName {
		var cmd tea.Cmd
		p.fields[PresetFieldName], cmd = p.fields[PresetFieldName].Update(msg)
		return cmd
	}
	return nil
}

func (p *PresetPicker) nextField() {
	// Only blur/focus the name field (it's the only text input)
	if p.focus == PresetFieldName {
		p.fields[PresetFieldName].Blur()
	}
	p.focus = (p.focus + 1) % PresetFieldCount
	if p.focus == PresetFieldName {
		p.fields[PresetFieldName].Focus()
	}
}

func (p *PresetPicker) prevField() {
	// Only blur/focus the name field (it's the only text input)
	if p.focus == PresetFieldName {
		p.fields[PresetFieldName].Blur()
	}
	p.focus = (p.focus - 1 + PresetFieldCount) % PresetFieldCount
	if p.focus == PresetFieldName {
		p.fields[PresetFieldName].Focus()
	}
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
	name := strings.TrimSpace(p.fields[PresetFieldName].Value())

	if name == "" {
		return "Preset name is required"
	}
	if len(p.selectedEnable) == 0 && len(p.selectedDisable) == 0 {
		return "Select at least one alias to enable or disable"
	}

	return ""
}

// FormValues returns the current form values using the selection maps.
func (p *PresetPicker) FormValues() (name string, enable, disable []string) {
	name = strings.TrimSpace(p.fields[PresetFieldName].Value())

	for alias := range p.selectedEnable {
		enable = append(enable, alias)
	}
	for alias := range p.selectedDisable {
		disable = append(disable, alias)
	}

	return name, enable, disable
}

// OpenEnablePicker opens the alias picker for enable selection.
func (p *PresetPicker) OpenEnablePicker() {
	p.mode = PresetModePickEnable
	p.pickerCursor = 0
}

// OpenDisablePicker opens the alias picker for disable selection.
func (p *PresetPicker) OpenDisablePicker() {
	p.mode = PresetModePickDisable
	p.pickerCursor = 0
}

// ClosePicker closes the alias picker and returns to form.
func (p *PresetPicker) ClosePicker() {
	if p.editName != "" {
		p.mode = PresetModeEdit
	} else {
		p.mode = PresetModeAdd
	}
}

// TogglePickerSelection toggles the currently highlighted alias.
func (p *PresetPicker) TogglePickerSelection() {
	filtered := p.getFilteredAliases()
	if p.pickerCursor >= len(filtered) {
		return
	}
	alias := filtered[p.pickerCursor]

	if p.mode == PresetModePickEnable {
		if p.selectedEnable[alias] {
			delete(p.selectedEnable, alias)
		} else {
			p.selectedEnable[alias] = true
		}
	} else if p.mode == PresetModePickDisable {
		if p.selectedDisable[alias] {
			delete(p.selectedDisable, alias)
		} else {
			p.selectedDisable[alias] = true
		}
	}
}

// PickerMoveUp moves picker cursor up.
func (p *PresetPicker) PickerMoveUp() {
	if p.pickerCursor > 0 {
		p.pickerCursor--
	}
}

// PickerMoveDown moves picker cursor down.
func (p *PresetPicker) PickerMoveDown() {
	filtered := p.getFilteredAliases()
	if p.pickerCursor < len(filtered)-1 {
		p.pickerCursor++
	}
}

// View renders the preset picker.
func (p *PresetPicker) View() string {
	switch p.mode {
	case PresetModeAdd, PresetModeEdit:
		return p.formView()
	case PresetModePickEnable, PresetModePickDisable:
		return p.pickerView()
	case PresetModeConfirmDelete:
		return p.deleteView()
	default:
		return p.selectView()
	}
}

func (p *PresetPicker) selectView() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Presets"))
	sb.WriteString("\n")
	sb.WriteString(helpDescStyle.Render("Quickly enable/disable multiple hosts at once"))
	sb.WriteString("\n\n")

	if len(p.presets) == 0 {
		sb.WriteString(helpDescStyle.Render("No presets configured."))
		sb.WriteString("\n\n")
		sb.WriteString(helpDescStyle.Render("Press 'n' to create one"))
	} else {
		for i, preset := range p.presets {
			if i == p.cursor {
				sb.WriteString(presetSelectedStyle.Render("▸ " + preset.Name))
				sb.WriteString("\n")
				// Show details for selected preset (only aliases that exist)
				enableList := p.filterExistingAliases(preset.Enable)
				disableList := p.filterExistingAliases(preset.Disable)
				if len(enableList) > 0 {
					sb.WriteString(enabledStyle.Render("    ● Enable: " + strings.Join(enableList, ", ")))
					sb.WriteString("\n")
				}
				if len(disableList) > 0 {
					sb.WriteString(disabledStyle.Render("    ○ Disable: " + strings.Join(disableList, ", ")))
					sb.WriteString("\n")
				}
			} else {
				sb.WriteString(presetItemStyle.Render("  " + preset.Name))
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString("\n")
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
	sb.WriteString("\n")
	sb.WriteString(helpDescStyle.Render("A preset lets you toggle multiple hosts with one action"))
	sb.WriteString("\n\n")

	// Name field
	sb.WriteString(inputLabelStyle.Render("Name:"))
	sb.WriteString("\n")
	style := inputStyle
	if p.focus == PresetFieldName {
		style = inputFocusStyle
	}
	sb.WriteString(style.Render(p.fields[PresetFieldName].View()))
	sb.WriteString("\n\n")

	// Enable selection (button-style)
	enableLabel := "Enable hosts:"
	if p.focus == PresetFieldEnable {
		enableLabel = "▸ Enable hosts: (press Enter to select)"
	}
	sb.WriteString(inputLabelStyle.Render(enableLabel))
	sb.WriteString("\n")
	if len(p.selectedEnable) > 0 {
		var enableList []string
		for alias := range p.selectedEnable {
			enableList = append(enableList, alias)
		}
		sb.WriteString(enabledStyle.Render("  ● " + strings.Join(enableList, ", ")))
	} else {
		sb.WriteString(helpDescStyle.Render("  (none selected)"))
	}
	sb.WriteString("\n\n")

	// Disable selection (button-style)
	disableLabel := "Disable hosts:"
	if p.focus == PresetFieldDisable {
		disableLabel = "▸ Disable hosts: (press Enter to select)"
	}
	sb.WriteString(inputLabelStyle.Render(disableLabel))
	sb.WriteString("\n")
	if len(p.selectedDisable) > 0 {
		var disableList []string
		for alias := range p.selectedDisable {
			disableList = append(disableList, alias)
		}
		sb.WriteString(disabledStyle.Render("  ○ " + strings.Join(disableList, ", ")))
	} else {
		sb.WriteString(helpDescStyle.Render("  (none selected)"))
	}
	sb.WriteString("\n\n")

	// Save button
	if p.focus == PresetFieldSave {
		sb.WriteString(presetSelectedStyle.Render("▸ [ Save Preset ]"))
	} else {
		sb.WriteString(presetItemStyle.Render("  [ Save Preset ]"))
	}
	sb.WriteString("\n\n")

	sb.WriteString(helpDescStyle.Render("Tab/↓ next • Enter select/save • Esc cancel"))

	return dialogStyle.Render(sb.String())
}

// getFilteredAliases returns aliases filtered for the current picker mode.
// Enable picker hides items already in disable list, and vice versa.
func (p *PresetPicker) getFilteredAliases() []string {
	var filtered []string
	for _, alias := range p.availableAliases {
		if p.mode == PresetModePickEnable {
			// Don't show items already in disable list (unless also in enable)
			if !p.selectedDisable[alias] || p.selectedEnable[alias] {
				filtered = append(filtered, alias)
			}
		} else {
			// Don't show items already in enable list (unless also in disable)
			if !p.selectedEnable[alias] || p.selectedDisable[alias] {
				filtered = append(filtered, alias)
			}
		}
	}
	return filtered
}

// filterExistingAliases filters a list of aliases to only include those that exist.
func (p *PresetPicker) filterExistingAliases(aliases []string) []string {
	if len(p.availableAliases) == 0 {
		return aliases
	}
	existsMap := make(map[string]bool)
	for _, alias := range p.availableAliases {
		existsMap[alias] = true
	}
	var filtered []string
	for _, alias := range aliases {
		if existsMap[alias] {
			filtered = append(filtered, alias)
		}
	}
	return filtered
}

func (p *PresetPicker) pickerView() string {
	var sb strings.Builder

	title := "Select hosts to ENABLE"
	if p.mode == PresetModePickDisable {
		title = "Select hosts to DISABLE"
	}

	sb.WriteString(titleStyle.Render(title))
	sb.WriteString("\n")
	sb.WriteString(helpDescStyle.Render("Space to toggle • Enter to confirm • Esc to cancel"))
	sb.WriteString("\n\n")

	filtered := p.getFilteredAliases()

	if len(filtered) == 0 {
		if len(p.availableAliases) == 0 {
			sb.WriteString(helpDescStyle.Render("No hosts available. Add some hosts first."))
		} else {
			sb.WriteString(helpDescStyle.Render("All hosts are already in the other list."))
		}
	} else {
		// Clamp cursor to filtered list
		if p.pickerCursor >= len(filtered) {
			p.pickerCursor = len(filtered) - 1
		}

		for i, alias := range filtered {
			var indicator string
			if p.mode == PresetModePickEnable {
				if p.selectedEnable[alias] {
					indicator = enabledStyle.Render("[●]")
				} else {
					indicator = helpDescStyle.Render("[ ]")
				}
			} else {
				if p.selectedDisable[alias] {
					indicator = disabledStyle.Render("[○]")
				} else {
					indicator = helpDescStyle.Render("[ ]")
				}
			}

			line := indicator + " " + alias

			if i == p.pickerCursor {
				sb.WriteString(presetSelectedStyle.Render("▸ " + line))
			} else {
				sb.WriteString(presetItemStyle.Render("  " + line))
			}
			sb.WriteString("\n")
		}
	}

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
