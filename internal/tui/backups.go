// Package tui provides the backup picker component.
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/lukaszraczylo/lolcathost/internal/protocol"
)

// BackupMode represents the backup view mode.
type BackupMode int

const (
	BackupModeSelect BackupMode = iota
	BackupModeConfirmRestore
)

// BackupPicker handles the backup selection and restore UI.
type BackupPicker struct {
	backups        []protocol.BackupInfo
	cursor         int
	width          int
	height         int
	mode           BackupMode
	previewContent string
	previewScroll  int
}

// NewBackupPicker creates a new backup picker.
func NewBackupPicker() *BackupPicker {
	return &BackupPicker{
		mode: BackupModeSelect,
	}
}

// SetBackups updates the available backups.
func (b *BackupPicker) SetBackups(backups []protocol.BackupInfo) {
	b.backups = backups
	if b.cursor >= len(backups) {
		b.cursor = max(0, len(backups)-1)
	}
}

// SetSize sets the picker dimensions.
func (b *BackupPicker) SetSize(width, height int) {
	b.width = width
	b.height = height
}

// MoveUp moves the cursor up.
func (b *BackupPicker) MoveUp() {
	if b.cursor > 0 {
		b.cursor--
		b.previewContent = "" // Clear preview to trigger reload
		b.previewScroll = 0
	}
}

// MoveDown moves the cursor down.
func (b *BackupPicker) MoveDown() {
	if b.cursor < len(b.backups)-1 {
		b.cursor++
		b.previewContent = "" // Clear preview to trigger reload
		b.previewScroll = 0
	}
}

// SetPreviewContent sets the preview content for the current backup.
func (b *BackupPicker) SetPreviewContent(content string) {
	b.previewContent = content
	b.previewScroll = 0
}

// PreviewContent returns the current preview content.
func (b *BackupPicker) PreviewContent() string {
	return b.previewContent
}

// ScrollPreviewUp scrolls the preview up.
func (b *BackupPicker) ScrollPreviewUp() {
	if b.previewScroll > 0 {
		b.previewScroll--
	}
}

// ScrollPreviewDown scrolls the preview down.
func (b *BackupPicker) ScrollPreviewDown() {
	b.previewScroll++
}

// Selected returns the currently selected backup name.
func (b *BackupPicker) Selected() string {
	if b.cursor >= 0 && b.cursor < len(b.backups) {
		return b.backups[b.cursor].Name
	}
	return ""
}

// SelectedInfo returns the currently selected backup info.
func (b *BackupPicker) SelectedInfo() *protocol.BackupInfo {
	if b.cursor >= 0 && b.cursor < len(b.backups) {
		return &b.backups[b.cursor]
	}
	return nil
}

// Len returns the number of backups.
func (b *BackupPicker) Len() int {
	return len(b.backups)
}

// Mode returns the current mode.
func (b *BackupPicker) Mode() BackupMode {
	return b.mode
}

// InitRestore starts restore confirmation.
func (b *BackupPicker) InitRestore() {
	if b.SelectedInfo() == nil {
		return
	}
	b.mode = BackupModeConfirmRestore
}

// Cancel cancels the current operation.
func (b *BackupPicker) Cancel() {
	b.mode = BackupModeSelect
}

// View renders the backup picker.
func (b *BackupPicker) View() string {
	switch b.mode {
	case BackupModeConfirmRestore:
		return b.restoreView()
	default:
		return b.selectView()
	}
}

func (b *BackupPicker) selectView() string {
	if len(b.backups) == 0 {
		var sb strings.Builder
		sb.WriteString(titleStyle.Render("Backups"))
		sb.WriteString("\n\n")
		sb.WriteString(helpDescStyle.Render("No backups available."))
		sb.WriteString("\n\n")
		sb.WriteString(helpDescStyle.Render("Backups are created automatically when hosts are modified."))
		sb.WriteString("\n\n")
		sb.WriteString(helpDescStyle.Render("Esc cancel"))
		return dialogStyle.Render(sb.String())
	}

	// Build left panel (backup list)
	var leftSb strings.Builder
	leftSb.WriteString(titleStyle.Render("Backups"))
	leftSb.WriteString("\n\n")
	leftSb.WriteString(helpDescStyle.Render(fmt.Sprintf("%d backup(s)", len(b.backups))))
	leftSb.WriteString("\n\n")

	for i, backup := range b.backups {
		timestamp := time.Unix(backup.Timestamp, 0).Format("2006-01-02 15:04:05")
		sizeStr := formatSize(backup.Size)
		line := fmt.Sprintf("%s  (%s)", timestamp, sizeStr)

		if i == b.cursor {
			leftSb.WriteString(presetSelectedStyle.Render("▸ " + line))
		} else {
			leftSb.WriteString(presetItemStyle.Render("  " + line))
		}
		leftSb.WriteString("\n")
	}

	leftSb.WriteString("\n")
	leftSb.WriteString(helpDescStyle.Render("↑↓ navigate • Enter restore • Esc cancel"))

	// Build right panel (preview)
	var rightSb strings.Builder
	rightSb.WriteString(titleStyle.Render("Preview"))
	rightSb.WriteString("\n\n")

	if b.previewContent == "" {
		rightSb.WriteString(helpDescStyle.Render("Loading..."))
	} else {
		// Show content with scroll support
		lines := strings.Split(b.previewContent, "\n")
		previewHeight := b.height - 12 // Reserve space for title, borders, help
		if previewHeight < 5 {
			previewHeight = 5
		}

		// Clamp scroll position
		maxScroll := len(lines) - previewHeight
		if maxScroll < 0 {
			maxScroll = 0
		}
		if b.previewScroll > maxScroll {
			b.previewScroll = maxScroll
		}

		// Get visible lines
		endLine := b.previewScroll + previewHeight
		if endLine > len(lines) {
			endLine = len(lines)
		}

		visibleLines := lines[b.previewScroll:endLine]
		for _, line := range visibleLines {
			// Truncate long lines
			if len(line) > 50 {
				line = line[:47] + "..."
			}
			rightSb.WriteString(helpDescStyle.Render(line))
			rightSb.WriteString("\n")
		}

		// Show scroll indicator
		if len(lines) > previewHeight {
			rightSb.WriteString("\n")
			rightSb.WriteString(helpDescStyle.Render(fmt.Sprintf("Lines %d-%d of %d (Shift+↑↓ scroll)", b.previewScroll+1, endLine, len(lines))))
		}
	}

	// Style the panels
	leftWidth := 45
	rightWidth := b.width - leftWidth - 10
	if rightWidth < 30 {
		rightWidth = 30
	}

	leftPanel := lipgloss.NewStyle().
		Width(leftWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(1, 2).
		Render(leftSb.String())

	rightPanel := lipgloss.NewStyle().
		Width(rightWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorMuted).
		Padding(1, 2).
		Render(rightSb.String())

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, " ", rightPanel)
}

func (b *BackupPicker) restoreView() string {
	var sb strings.Builder

	backup := b.SelectedInfo()
	timestamp := ""
	if backup != nil {
		timestamp = time.Unix(backup.Timestamp, 0).Format("2006-01-02 15:04:05")
	}

	sb.WriteString(titleStyle.Render("Restore Backup"))
	sb.WriteString("\n\n")
	sb.WriteString(errorMsgStyle.Render(fmt.Sprintf("Restore /etc/hosts from backup '%s'?", timestamp)))
	sb.WriteString("\n\n")
	sb.WriteString(helpDescStyle.Render("This will replace your current hosts file."))
	sb.WriteString("\n\n")
	sb.WriteString(helpDescStyle.Render("y confirm • n/Esc cancel"))

	return dialogStyle.Render(sb.String())
}

// formatSize formats bytes to human readable format.
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
	)

	switch {
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
