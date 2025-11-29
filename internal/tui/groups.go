// Package tui provides the group management component.
package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// GroupMode represents the group view mode.
type GroupMode int

const (
	GroupModeSelect GroupMode = iota
	GroupModeAdd
	GroupModeRename
	GroupModeConfirmDelete
)

// GroupPicker handles the group selection and management UI.
type GroupPicker struct {
	groups   []string
	cursor   int
	width    int
	height   int
	mode     GroupMode
	input    textinput.Model
	editName string // Original name when renaming
}

// NewGroupPicker creates a new group picker.
func NewGroupPicker() *GroupPicker {
	input := textinput.New()
	input.Placeholder = "group-name"
	input.CharLimit = 63

	return &GroupPicker{
		input: input,
		mode:  GroupModeSelect,
	}
}

// SetGroups updates the available groups.
func (g *GroupPicker) SetGroups(groups []string) {
	g.groups = groups
	if g.cursor >= len(groups) {
		g.cursor = max(0, len(groups)-1)
	}
}

// SetSize sets the picker dimensions.
func (g *GroupPicker) SetSize(width, height int) {
	g.width = width
	g.height = height
	g.input.Width = min(50, width-10)
}

// MoveUp moves the cursor up.
func (g *GroupPicker) MoveUp() {
	if g.cursor > 0 {
		g.cursor--
	}
}

// MoveDown moves the cursor down.
func (g *GroupPicker) MoveDown() {
	if g.cursor < len(g.groups)-1 {
		g.cursor++
	}
}

// Selected returns the currently selected group.
func (g *GroupPicker) Selected() string {
	if g.cursor >= 0 && g.cursor < len(g.groups) {
		return g.groups[g.cursor]
	}
	return ""
}

// Len returns the number of groups.
func (g *GroupPicker) Len() int {
	return len(g.groups)
}

// Mode returns the current mode.
func (g *GroupPicker) Mode() GroupMode {
	return g.mode
}

// InitAdd initializes the form for adding a new group.
func (g *GroupPicker) InitAdd() {
	g.mode = GroupModeAdd
	g.editName = ""
	g.input.Reset()
	g.input.Focus()
}

// InitRename initializes the form for renaming an existing group.
func (g *GroupPicker) InitRename() {
	selected := g.Selected()
	if selected == "" {
		return
	}

	g.mode = GroupModeRename
	g.editName = selected
	g.input.SetValue(selected)
	g.input.Focus()
}

// InitDelete starts delete confirmation.
func (g *GroupPicker) InitDelete() {
	if g.Selected() == "" {
		return
	}
	g.mode = GroupModeConfirmDelete
}

// CancelForm cancels the current form operation.
func (g *GroupPicker) CancelForm() {
	g.mode = GroupModeSelect
	g.editName = ""
	g.input.Reset()
	g.input.Blur()
}

// Update handles input events for form mode.
func (g *GroupPicker) Update(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	g.input, cmd = g.input.Update(msg)
	return cmd
}

// FormValue returns the form input value.
func (g *GroupPicker) FormValue() string {
	return strings.TrimSpace(g.input.Value())
}

// EditName returns the original name when renaming.
func (g *GroupPicker) EditName() string {
	return g.editName
}

// IsRename returns true if in rename mode.
func (g *GroupPicker) IsRename() bool {
	return g.mode == GroupModeRename
}

// ValidateForm validates the form value.
func (g *GroupPicker) ValidateForm() string {
	value := g.FormValue()
	if value == "" {
		return "Group name is required"
	}
	return ""
}

// View renders the group picker.
func (g *GroupPicker) View() string {
	switch g.mode {
	case GroupModeAdd, GroupModeRename:
		return g.formView()
	case GroupModeConfirmDelete:
		return g.deleteView()
	default:
		return g.selectView()
	}
}

func (g *GroupPicker) selectView() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Groups"))
	sb.WriteString("\n\n")

	if len(g.groups) == 0 {
		sb.WriteString(helpDescStyle.Render("No groups configured."))
		sb.WriteString("\n\n")
		sb.WriteString(helpDescStyle.Render("Press 'n' to create one"))
	} else {
		for i, group := range g.groups {
			if i == g.cursor {
				sb.WriteString(presetSelectedStyle.Render("▸ " + group))
			} else {
				sb.WriteString(presetItemStyle.Render("  " + group))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n\n")
	sb.WriteString(WrapHelpText("↑↓ navigate • n new • r rename • d delete • Esc back", g.width-6))

	return dialogStyle.Render(sb.String())
}

func (g *GroupPicker) formView() string {
	var sb strings.Builder

	title := "Add New Group"
	if g.mode == GroupModeRename {
		title = "Rename Group"
	}

	sb.WriteString(titleStyle.Render(title))
	sb.WriteString("\n\n")

	sb.WriteString(inputLabelStyle.Render("Name:"))
	sb.WriteString("\n")
	sb.WriteString(inputFocusStyle.Render(g.input.View()))
	sb.WriteString("\n\n")
	sb.WriteString(helpDescStyle.Render("Enter save • Esc cancel"))

	return dialogStyle.Render(sb.String())
}

func (g *GroupPicker) deleteView() string {
	var sb strings.Builder

	groupName := g.Selected()

	sb.WriteString(titleStyle.Render("Delete Group"))
	sb.WriteString("\n\n")
	sb.WriteString(errorMsgStyle.Render("Are you sure you want to delete group '" + groupName + "'?"))
	sb.WriteString("\n")
	sb.WriteString(helpDescStyle.Render("This will remove all hosts in this group!"))
	sb.WriteString("\n\n")
	sb.WriteString(helpDescStyle.Render("y confirm • n/Esc cancel"))

	return dialogStyle.Render(sb.String())
}
