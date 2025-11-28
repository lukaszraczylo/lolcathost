// Package tui provides the form component for adding/editing entries.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// FormMode represents the form mode.
type FormMode int

const (
	FormModeAdd FormMode = iota
	FormModeEdit
)

// FormField represents a form field index.
type FormField int

const (
	FieldDomain FormField = iota
	FieldIP
	FieldGroup
	FieldCount
)

// Form handles the add/edit entry form.
type Form struct {
	mode      FormMode
	fields    []textinput.Model
	focus     FormField
	width     int
	height    int
	editAlias string // Original alias when editing

	// Group dropdown
	groups       []string
	groupCursor  int
	groupFocused bool
}

// NewForm creates a new form.
func NewForm() *Form {
	fields := make([]textinput.Model, FieldCount)

	// Domain field
	fields[FieldDomain] = textinput.New()
	fields[FieldDomain].Placeholder = "example.com"
	fields[FieldDomain].CharLimit = 253

	// IP field
	fields[FieldIP] = textinput.New()
	fields[FieldIP].Placeholder = "127.0.0.1"
	fields[FieldIP].CharLimit = 45 // IPv6 max

	// Group field (not used as text input, but kept for compatibility)
	fields[FieldGroup] = textinput.New()
	fields[FieldGroup].Placeholder = "development"
	fields[FieldGroup].CharLimit = 63

	return &Form{
		fields: fields,
		focus:  FieldDomain,
		groups: []string{"default"},
	}
}

// SetGroups sets the available groups for the dropdown.
func (f *Form) SetGroups(groups []string) {
	if len(groups) == 0 {
		f.groups = []string{"default"}
	} else {
		f.groups = groups
	}
	// Reset cursor if out of bounds
	if f.groupCursor >= len(f.groups) {
		f.groupCursor = 0
	}
}

// Init initializes the form for adding a new entry.
func (f *Form) Init() {
	f.mode = FormModeAdd
	f.editAlias = ""

	for i := range f.fields {
		f.fields[i].Reset()
	}

	f.fields[FieldIP].SetValue("127.0.0.1")
	f.groupCursor = 0
	f.groupFocused = false
	f.focus = FieldDomain
	f.fields[FieldDomain].Focus()
}

// InitEdit initializes the form for editing an existing entry.
func (f *Form) InitEdit(domain, ip, alias, group string) {
	f.mode = FormModeEdit
	f.editAlias = alias

	f.fields[FieldDomain].SetValue(domain)
	f.fields[FieldIP].SetValue(ip)

	// Find the group in the list
	f.groupCursor = 0
	for i, g := range f.groups {
		if g == group {
			f.groupCursor = i
			break
		}
	}

	f.groupFocused = false
	f.focus = FieldDomain
	f.fields[FieldDomain].Focus()
}

// SetSize sets the form dimensions.
func (f *Form) SetSize(width, height int) {
	f.width = width
	f.height = height

	inputWidth := min(50, width-10)
	for i := range f.fields {
		f.fields[i].Width = inputWidth
	}
}

// Update handles input events.
func (f *Form) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle group dropdown navigation
		if f.focus == FieldGroup {
			switch msg.String() {
			case "tab":
				f.nextField()
				return nil
			case "shift+tab":
				f.prevField()
				return nil
			case "up", "k":
				if f.groupCursor > 0 {
					f.groupCursor--
				}
				return nil
			case "down", "j":
				if f.groupCursor < len(f.groups)-1 {
					f.groupCursor++
				}
				return nil
			case "left":
				if f.groupCursor > 0 {
					f.groupCursor--
				}
				return nil
			case "right":
				if f.groupCursor < len(f.groups)-1 {
					f.groupCursor++
				}
				return nil
			}
			return nil
		}

		// Handle text input fields
		switch msg.String() {
		case "tab", "down":
			f.nextField()
			return nil
		case "shift+tab", "up":
			f.prevField()
			return nil
		}
	}

	// Update the focused text field (only for Domain and IP)
	if f.focus != FieldGroup {
		var cmd tea.Cmd
		f.fields[f.focus], cmd = f.fields[f.focus].Update(msg)
		return cmd
	}

	return nil
}

func (f *Form) nextField() {
	if f.focus != FieldGroup {
		f.fields[f.focus].Blur()
	}
	f.focus = (f.focus + 1) % FieldCount
	if f.focus != FieldGroup {
		f.fields[f.focus].Focus()
	}
}

func (f *Form) prevField() {
	if f.focus != FieldGroup {
		f.fields[f.focus].Blur()
	}
	f.focus = (f.focus - 1 + FieldCount) % FieldCount
	if f.focus != FieldGroup {
		f.fields[f.focus].Focus()
	}
}

// Values returns the form values (domain, ip, group).
func (f *Form) Values() (domain, ip, group string) {
	group = ""
	if f.groupCursor < len(f.groups) {
		group = f.groups[f.groupCursor]
	}
	return strings.TrimSpace(f.fields[FieldDomain].Value()),
		strings.TrimSpace(f.fields[FieldIP].Value()),
		group
}

// EditAlias returns the original alias when editing.
func (f *Form) EditAlias() string {
	return f.editAlias
}

// IsEdit returns true if in edit mode.
func (f *Form) IsEdit() bool {
	return f.mode == FormModeEdit
}

// Validate validates the form values.
func (f *Form) Validate() string {
	domain, ip, group := f.Values()

	if domain == "" {
		return "Domain is required"
	}
	if ip == "" {
		return "IP address is required"
	}
	if group == "" {
		return "Group is required"
	}

	return ""
}

// View renders the form.
func (f *Form) View() string {
	var sb strings.Builder

	title := "Add New Entry"
	if f.mode == FormModeEdit {
		title = "Edit Entry"
	}

	sb.WriteString(titleStyle.Render(title))
	sb.WriteString("\n\n")

	// Domain field
	sb.WriteString(inputLabelStyle.Render("Domain:"))
	sb.WriteString("\n")
	style := inputStyle
	if f.focus == FieldDomain {
		style = inputFocusStyle
	}
	sb.WriteString(style.Render(f.fields[FieldDomain].View()))
	sb.WriteString("\n\n")

	// IP field
	sb.WriteString(inputLabelStyle.Render("IP Address:"))
	sb.WriteString("\n")
	style = inputStyle
	if f.focus == FieldIP {
		style = inputFocusStyle
	}
	sb.WriteString(style.Render(f.fields[FieldIP].View()))
	sb.WriteString("\n\n")

	// Group dropdown
	sb.WriteString(inputLabelStyle.Render("Group:"))
	sb.WriteString("\n")
	sb.WriteString(f.renderGroupDropdown())
	sb.WriteString("\n\n")

	sb.WriteString("\n")
	sb.WriteString(helpDescStyle.Render("Tab/↓ next • Shift+Tab/↑ prev • ←→ select group • Enter save • Esc cancel"))

	return dialogStyle.Render(sb.String())
}

func (f *Form) renderGroupDropdown() string {
	isFocused := f.focus == FieldGroup

	// Get current group name
	currentGroup := "default"
	if f.groupCursor < len(f.groups) {
		currentGroup = f.groups[f.groupCursor]
	}

	// Build the selector content: ◀ group_name ▶
	var content string
	if isFocused {
		// Show arrows when focused
		leftArrow := "◀"
		rightArrow := "▶"
		if f.groupCursor == 0 {
			leftArrow = " " // dim or hide left arrow at start
		}
		if f.groupCursor >= len(f.groups)-1 {
			rightArrow = " " // dim or hide right arrow at end
		}
		content = leftArrow + "  " + currentGroup + "  " + rightArrow
	} else {
		content = "   " + currentGroup + "   "
	}

	// Show position indicator if multiple groups
	if len(f.groups) > 1 {
		content += fmt.Sprintf("  (%d/%d)", f.groupCursor+1, len(f.groups))
	}

	// Apply border style
	if isFocused {
		return inputFocusStyle.Render(content)
	}
	return inputStyle.Render(content)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
