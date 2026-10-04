package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModel_View_UsesAltScreenAndTitle(t *testing.T) {
	m := NewModel("/nonexistent.sock")

	v := m.View()

	assert.True(t, v.AltScreen)
	assert.Equal(t, "lolcathost", v.WindowTitle)
	assert.Contains(t, v.Content, "lolcathost - Host Management")
}

func TestModel_SpaceKeyIsNamedSpace(t *testing.T) {
	msg := tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}

	require.Equal(t, "space", msg.String())
}
