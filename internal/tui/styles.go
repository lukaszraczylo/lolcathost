// Package tui provides the terminal user interface.
package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Colors - matching kportal style, optimized for dark terminals
var (
	colorPrimary     = lipgloss.Color("205") // Pink/Magenta
	colorSuccess     = lipgloss.Color("42")  // Green
	colorWarning     = lipgloss.Color("220") // Yellow
	colorError       = lipgloss.Color("196") // Red
	colorMuted       = lipgloss.Color("245") // Gray (brighter for dark terminals)
	colorAccent      = lipgloss.Color("141") // Light purple (brighter for dark terminals)
	colorHeader      = lipgloss.Color("220") // Yellow for headers
	colorSelectedBg  = lipgloss.Color("236") // Gray background for selection
	colorSelectedFg  = lipgloss.Color("255") // White foreground for selection
	colorGroupHeader = lipgloss.Color("213") // Light pink for group headers
)

// Title and header styles
var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorHeader).
		Padding(0, 1)
)

// Status indicators
var (
	enabledStyle = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)

	disabledStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	pendingStyle = lipgloss.NewStyle().
			Foreground(colorWarning)

	errorIndicatorStyle = lipgloss.NewStyle().
				Foreground(colorError)
)

// Status bar and help
var (
	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	connectedStyle = lipgloss.NewStyle().
			Foreground(colorSuccess).
			SetString("Connected")

	disconnectedStyle = lipgloss.NewStyle().
				Foreground(colorError).
				SetString("Disconnected")

	helpBarStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorHeader).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(colorMuted)
)

// Message styles
var (
	errorMsgStyle = lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true).
			MarginTop(1)

	successMsgStyle = lipgloss.NewStyle().
			Foreground(colorSuccess).
			MarginTop(1)

	updateStyle = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)
)

// Form styles
var (
	inputLabelStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMuted).
			Padding(0, 1)

	inputFocusStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1)
)

// Dialog/modal styles
var (
	dialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(1, 2)

	presetItemStyle = lipgloss.NewStyle().
			Padding(0, 1)

	presetSelectedStyle = lipgloss.NewStyle().
				Background(colorSelectedBg).
				Foreground(colorSelectedFg).
				Padding(0, 1)
)

// Indicator returns the appropriate status indicator string.
func Indicator(enabled bool, pending bool, hasError bool) string {
	if hasError {
		return errorIndicatorStyle.Render("✗")
	}
	if pending {
		return pendingStyle.Render("◐")
	}
	if enabled {
		return enabledStyle.Render("●")
	}
	return disabledStyle.Render("○")
}

// StatusText returns the status text with appropriate styling
func StatusText(enabled bool, pending bool, hasError bool) string {
	if hasError {
		return errorIndicatorStyle.Render("✗ Error")
	}
	if pending {
		return pendingStyle.Render("◐ Pending")
	}
	if enabled {
		return enabledStyle.Render("● Active")
	}
	return disabledStyle.Render("○ Disabled")
}

// HelpItem formats a help item.
func HelpItem(key, desc string) string {
	return helpKeyStyle.Render(key) + " " + helpDescStyle.Render(desc)
}
