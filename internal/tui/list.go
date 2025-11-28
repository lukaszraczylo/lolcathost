// Package tui provides the list view component.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/lukaszraczylo/lolcathost/internal/protocol"
)

// EntryItem represents a displayable host entry.
type EntryItem struct {
	Entry    protocol.HostEntry
	Pending  bool
	HasError bool
}

// ListView handles the list of host entries.
type ListView struct {
	items      []EntryItem
	groups     map[string][]int // group name -> indices in items
	groupOrder []string         // ordered group names
	cursor     int
	width      int
	height     int
}

// NewListView creates a new list view.
func NewListView() *ListView {
	return &ListView{
		groups: make(map[string][]int),
	}
}

// SetItems updates the list items.
func (l *ListView) SetItems(entries []protocol.HostEntry) {
	l.items = make([]EntryItem, len(entries))
	l.groups = make(map[string][]int)
	l.groupOrder = nil

	groupSeen := make(map[string]bool)

	for i, e := range entries {
		l.items[i] = EntryItem{Entry: e}

		if !groupSeen[e.Group] {
			groupSeen[e.Group] = true
			l.groupOrder = append(l.groupOrder, e.Group)
		}

		l.groups[e.Group] = append(l.groups[e.Group], i)
	}

	// Reset cursor if out of bounds
	if l.cursor >= len(l.items) {
		l.cursor = max(0, len(l.items)-1)
	}
}

// SetSize sets the view dimensions.
func (l *ListView) SetSize(width, height int) {
	l.width = width
	l.height = height
}

// MoveUp moves the cursor up.
func (l *ListView) MoveUp() {
	if l.cursor > 0 {
		l.cursor--
	}
}

// MoveDown moves the cursor down.
func (l *ListView) MoveDown() {
	if l.cursor < len(l.items)-1 {
		l.cursor++
	}
}

// Selected returns the currently selected item.
func (l *ListView) Selected() *EntryItem {
	if l.cursor >= 0 && l.cursor < len(l.items) {
		return &l.items[l.cursor]
	}
	return nil
}

// SelectedAlias returns the alias of the selected item.
func (l *ListView) SelectedAlias() string {
	if item := l.Selected(); item != nil {
		return item.Entry.Alias
	}
	return ""
}

// SetPending marks an item as pending.
func (l *ListView) SetPending(alias string, pending bool) {
	for i := range l.items {
		if l.items[i].Entry.Alias == alias {
			l.items[i].Pending = pending
			break
		}
	}
}

// SetError marks an item as having an error.
func (l *ListView) SetError(alias string, hasError bool) {
	for i := range l.items {
		if l.items[i].Entry.Alias == alias {
			l.items[i].HasError = hasError
			break
		}
	}
}

// UpdateEntry updates an entry's enabled state.
func (l *ListView) UpdateEntry(alias string, enabled bool) {
	for i := range l.items {
		if l.items[i].Entry.Alias == alias {
			l.items[i].Entry.Enabled = enabled
			l.items[i].Pending = false
			l.items[i].HasError = false
			break
		}
	}
}

// Len returns the number of items.
func (l *ListView) Len() int {
	return len(l.items)
}

// ActiveCount returns the number of enabled entries.
func (l *ListView) ActiveCount() int {
	count := 0
	for _, item := range l.items {
		if item.Entry.Enabled {
			count++
		}
	}
	return count
}

// FindByAlias finds an item by alias.
func (l *ListView) FindByAlias(alias string) *EntryItem {
	for i := range l.items {
		if l.items[i].Entry.Alias == alias {
			return &l.items[i]
		}
	}
	return nil
}

// Filter filters items by search term.
func (l *ListView) Filter(term string) []EntryItem {
	if term == "" {
		return l.items
	}

	term = strings.ToLower(term)
	var filtered []EntryItem
	for _, item := range l.items {
		if strings.Contains(strings.ToLower(item.Entry.Domain), term) ||
			strings.Contains(strings.ToLower(item.Entry.Alias), term) ||
			strings.Contains(strings.ToLower(item.Entry.IP), term) ||
			strings.Contains(strings.ToLower(item.Entry.Group), term) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// ViewFiltered renders the list filtered by search term.
func (l *ListView) ViewFiltered(searchTerm string) string {
	if searchTerm == "" {
		return l.View()
	}

	filtered := l.Filter(searchTerm)
	if len(filtered) == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(colorMuted)
		return "\n" + emptyStyle.Render(fmt.Sprintf("  No results for '%s'. Press Esc to clear search.", searchTerm)) + "\n"
	}

	var sb strings.Builder

	// Show search indicator
	searchIndicator := lipgloss.NewStyle().
		Foreground(colorWarning).
		Bold(true).
		Render(fmt.Sprintf("  Search: %s (%d results)", searchTerm, len(filtered)))
	sb.WriteString(searchIndicator)
	sb.WriteString("\n")

	// Group header style - bright colors for dark terminals
	groupHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorGroupHeader).
		Background(lipgloss.Color("238")).
		Padding(0, 1).
		MarginTop(1)

	// Organize filtered items by group
	groupItems := make(map[string][]EntryItem)
	var groupOrder []string
	groupSeen := make(map[string]bool)

	for _, item := range filtered {
		group := item.Entry.Group
		if !groupSeen[group] {
			groupSeen[group] = true
			groupOrder = append(groupOrder, group)
		}
		groupItems[group] = append(groupItems[group], item)
	}

	for _, groupName := range groupOrder {
		items := groupItems[groupName]
		if len(items) == 0 {
			continue
		}

		// Group header
		headerText := fmt.Sprintf(" %s (%d)", strings.ToUpper(groupName), len(items))
		sb.WriteString(groupHeaderStyle.Render(headerText))
		sb.WriteString("\n")

		// Build rows for this group's table
		var rows [][]string
		for _, item := range items {
			status := l.getStatusString(item)
			rows = append(rows, []string{
				truncate(item.Entry.Domain, 30),
				truncate(item.Entry.IP, 15),
				status,
			})
		}

		// Create table for this group
		t := table.New().
			Border(lipgloss.HiddenBorder()).
			Headers("DOMAIN", "IP ADDRESS", "STATUS").
			Rows(rows...).
			StyleFunc(func(row, col int) lipgloss.Style {
				// Header row
				if row == table.HeaderRow {
					return lipgloss.NewStyle().
						Bold(true).
						Foreground(colorHeader).
						Padding(0, 1)
				}

				baseStyle := lipgloss.NewStyle().Padding(0, 1)

				if row >= 0 && row < len(items) {
					item := items[row]

					// Disabled rows are muted
					if !item.Entry.Enabled && !item.Pending && !item.HasError {
						return baseStyle.Foreground(colorMuted)
					}

					// Status column gets colored based on status
					if col == 2 { // STATUS column
						if item.HasError {
							return baseStyle.Foreground(colorError)
						}
						if item.Pending {
							return baseStyle.Foreground(colorWarning)
						}
						if item.Entry.Enabled {
							return baseStyle.Foreground(colorSuccess)
						}
					}
				}

				return baseStyle
			})

		sb.WriteString(t.Render())
		sb.WriteString("\n")
	}

	return sb.String()
}

// GroupCount returns the number of groups.
func (l *ListView) GroupCount() int {
	return len(l.groupOrder)
}

// GetGroups returns all group names.
func (l *ListView) GetGroups() []string {
	return l.groupOrder
}

// View renders the list with groups as headers.
func (l *ListView) View() string {
	if len(l.items) == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(colorMuted)
		return "\n" + emptyStyle.Render("  No host entries configured. Press 'n' to add a new entry.") + "\n"
	}

	var sb strings.Builder

	// Group header style - bright colors for dark terminals
	groupHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorGroupHeader).
		Background(lipgloss.Color("238")).
		Padding(0, 1).
		MarginTop(1)

	for _, groupName := range l.groupOrder {
		indices := l.groups[groupName]
		if len(indices) == 0 {
			continue
		}

		// Group header
		headerText := fmt.Sprintf(" %s (%d)", strings.ToUpper(groupName), len(indices))
		sb.WriteString(groupHeaderStyle.Render(headerText))
		sb.WriteString("\n")

		// Build rows for this group's table
		var rows [][]string
		// Store actual item indices for cursor matching
		itemIndices := make([]int, len(indices))
		copy(itemIndices, indices)

		for _, idx := range indices {
			item := l.items[idx]
			status := l.getStatusString(item)
			rows = append(rows, []string{
				truncate(item.Entry.Domain, 30),
				truncate(item.Entry.IP, 15),
				status,
			})
		}

		// Create table for this group
		t := table.New().
			Border(lipgloss.HiddenBorder()).
			Headers("DOMAIN", "IP ADDRESS", "STATUS").
			Rows(rows...).
			StyleFunc(func(row, col int) lipgloss.Style {
				// Header row
				if row == table.HeaderRow {
					return lipgloss.NewStyle().
						Bold(true).
						Foreground(colorHeader).
						Padding(0, 1)
				}

				baseStyle := lipgloss.NewStyle().Padding(0, 1)

				// Check if this row is selected
				if row >= 0 && row < len(itemIndices) {
					actualItemIdx := itemIndices[row]
					isSelected := actualItemIdx == l.cursor
					item := l.items[actualItemIdx]

					// Selected row gets background highlight
					if isSelected {
						return baseStyle.
							Background(colorSelectedBg).
							Foreground(colorSelectedFg)
					}

					// Disabled rows are muted
					if !item.Entry.Enabled && !item.Pending && !item.HasError {
						return baseStyle.Foreground(colorMuted)
					}

					// Status column gets colored based on status
					if col == 2 { // STATUS column
						if item.HasError {
							return baseStyle.Foreground(colorError)
						}
						if item.Pending {
							return baseStyle.Foreground(colorWarning)
						}
						if item.Entry.Enabled {
							return baseStyle.Foreground(colorSuccess)
						}
					}
				}

				return baseStyle
			})

		sb.WriteString(t.Render())
		sb.WriteString("\n")
	}

	return sb.String()
}

func (l *ListView) getStatusString(item EntryItem) string {
	if item.HasError {
		return "✗ Error"
	}
	if item.Pending {
		return "◐ Pending"
	}
	if item.Entry.Enabled {
		return "● Active"
	}
	return "○ Disabled"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
