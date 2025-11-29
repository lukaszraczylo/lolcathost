package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lukaszraczylo/lolcathost/internal/protocol"
)

func TestListView_SetItems(t *testing.T) {
	lv := NewListView()

	entries := []protocol.HostEntry{
		{Domain: "a.com", IP: "127.0.0.1", Alias: "a", Enabled: true, Group: "dev"},
		{Domain: "b.com", IP: "127.0.0.1", Alias: "b", Enabled: false, Group: "dev"},
		{Domain: "c.com", IP: "192.168.1.1", Alias: "c", Enabled: true, Group: "staging"},
	}

	lv.SetItems(entries)

	assert.Equal(t, 3, lv.Len())
	assert.Len(t, lv.groups, 2)
	assert.Contains(t, lv.groupOrder, "dev")
	assert.Contains(t, lv.groupOrder, "staging")
}

func TestListView_Navigation(t *testing.T) {
	lv := NewListView()
	entries := []protocol.HostEntry{
		{Domain: "a.com", IP: "127.0.0.1", Alias: "a", Enabled: true, Group: "dev"},
		{Domain: "b.com", IP: "127.0.0.1", Alias: "b", Enabled: false, Group: "dev"},
		{Domain: "c.com", IP: "192.168.1.1", Alias: "c", Enabled: true, Group: "staging"},
	}
	lv.SetItems(entries)

	// Initial position
	assert.Equal(t, 0, lv.cursor)

	// Move down
	lv.MoveDown()
	assert.Equal(t, 1, lv.cursor)

	lv.MoveDown()
	assert.Equal(t, 2, lv.cursor)

	// Can't move past end
	lv.MoveDown()
	assert.Equal(t, 2, lv.cursor)

	// Move up
	lv.MoveUp()
	assert.Equal(t, 1, lv.cursor)

	lv.MoveUp()
	assert.Equal(t, 0, lv.cursor)

	// Can't move before start
	lv.MoveUp()
	assert.Equal(t, 0, lv.cursor)
}

func TestListView_Selected(t *testing.T) {
	lv := NewListView()

	t.Run("empty list", func(t *testing.T) {
		item := lv.Selected()
		assert.Nil(t, item)
	})

	t.Run("with items", func(t *testing.T) {
		entries := []protocol.HostEntry{
			{Domain: "a.com", IP: "127.0.0.1", Alias: "a", Enabled: true, Group: "dev"},
			{Domain: "b.com", IP: "127.0.0.1", Alias: "b", Enabled: false, Group: "dev"},
		}
		lv.SetItems(entries)

		item := lv.Selected()
		require.NotNil(t, item)
		assert.Equal(t, "a.com", item.Entry.Domain)

		lv.MoveDown()
		item = lv.Selected()
		require.NotNil(t, item)
		assert.Equal(t, "b.com", item.Entry.Domain)
	})
}

func TestListView_SelectedAlias(t *testing.T) {
	lv := NewListView()

	t.Run("empty list", func(t *testing.T) {
		alias := lv.SelectedAlias()
		assert.Empty(t, alias)
	})

	t.Run("with items", func(t *testing.T) {
		entries := []protocol.HostEntry{
			{Domain: "a.com", IP: "127.0.0.1", Alias: "my-alias", Enabled: true, Group: "dev"},
		}
		lv.SetItems(entries)

		alias := lv.SelectedAlias()
		assert.Equal(t, "my-alias", alias)
	})
}

func TestListView_SetPending(t *testing.T) {
	lv := NewListView()
	entries := []protocol.HostEntry{
		{Domain: "a.com", IP: "127.0.0.1", Alias: "a", Enabled: true, Group: "dev"},
	}
	lv.SetItems(entries)

	assert.False(t, lv.items[0].Pending)

	lv.SetPending("a", true)
	assert.True(t, lv.items[0].Pending)

	lv.SetPending("a", false)
	assert.False(t, lv.items[0].Pending)

	// Non-existent alias should not panic
	lv.SetPending("nonexistent", true)
}

func TestListView_SetError(t *testing.T) {
	lv := NewListView()
	entries := []protocol.HostEntry{
		{Domain: "a.com", IP: "127.0.0.1", Alias: "a", Enabled: true, Group: "dev"},
	}
	lv.SetItems(entries)

	assert.False(t, lv.items[0].HasError)

	lv.SetError("a", true)
	assert.True(t, lv.items[0].HasError)

	lv.SetError("a", false)
	assert.False(t, lv.items[0].HasError)
}

func TestListView_UpdateEntry(t *testing.T) {
	lv := NewListView()
	entries := []protocol.HostEntry{
		{Domain: "a.com", IP: "127.0.0.1", Alias: "a", Enabled: false, Group: "dev"},
	}
	lv.SetItems(entries)
	lv.items[0].Pending = true
	lv.items[0].HasError = true

	lv.UpdateEntry("a", true)

	assert.True(t, lv.items[0].Entry.Enabled)
	assert.False(t, lv.items[0].Pending)
	assert.False(t, lv.items[0].HasError)
}

func TestListView_ActiveCount(t *testing.T) {
	lv := NewListView()
	entries := []protocol.HostEntry{
		{Domain: "a.com", IP: "127.0.0.1", Alias: "a", Enabled: true, Group: "dev"},
		{Domain: "b.com", IP: "127.0.0.1", Alias: "b", Enabled: false, Group: "dev"},
		{Domain: "c.com", IP: "192.168.1.1", Alias: "c", Enabled: true, Group: "staging"},
	}
	lv.SetItems(entries)

	assert.Equal(t, 2, lv.ActiveCount())
}

func TestListView_FindByAlias(t *testing.T) {
	lv := NewListView()
	entries := []protocol.HostEntry{
		{Domain: "a.com", IP: "127.0.0.1", Alias: "a", Enabled: true, Group: "dev"},
		{Domain: "b.com", IP: "127.0.0.1", Alias: "b", Enabled: false, Group: "dev"},
	}
	lv.SetItems(entries)

	t.Run("found", func(t *testing.T) {
		item := lv.FindByAlias("b")
		require.NotNil(t, item)
		assert.Equal(t, "b.com", item.Entry.Domain)
	})

	t.Run("not found", func(t *testing.T) {
		item := lv.FindByAlias("nonexistent")
		assert.Nil(t, item)
	})
}

func TestListView_Filter(t *testing.T) {
	lv := NewListView()
	entries := []protocol.HostEntry{
		{Domain: "myapp.com", IP: "127.0.0.1", Alias: "myapp", Enabled: true, Group: "dev"},
		{Domain: "api.myapp.com", IP: "127.0.0.1", Alias: "api", Enabled: false, Group: "dev"},
		{Domain: "other.com", IP: "192.168.1.1", Alias: "other", Enabled: true, Group: "staging"},
	}
	lv.SetItems(entries)

	t.Run("empty term", func(t *testing.T) {
		filtered := lv.Filter("")
		assert.Len(t, filtered, 3)
	})

	t.Run("by domain", func(t *testing.T) {
		filtered := lv.Filter("myapp")
		assert.Len(t, filtered, 2)
	})

	t.Run("by alias", func(t *testing.T) {
		filtered := lv.Filter("api")
		assert.Len(t, filtered, 1)
		assert.Equal(t, "api.myapp.com", filtered[0].Entry.Domain)
	})

	t.Run("by IP", func(t *testing.T) {
		filtered := lv.Filter("192.168")
		assert.Len(t, filtered, 1)
		assert.Equal(t, "other.com", filtered[0].Entry.Domain)
	})

	t.Run("case insensitive", func(t *testing.T) {
		filtered := lv.Filter("MYAPP")
		assert.Len(t, filtered, 2)
	})

	t.Run("no match", func(t *testing.T) {
		filtered := lv.Filter("nonexistent")
		assert.Empty(t, filtered)
	})
}

func TestListView_View(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		lv := NewListView()
		view := lv.View()
		assert.Contains(t, view, "No host entries")
	})

	t.Run("with items", func(t *testing.T) {
		lv := NewListView()
		entries := []protocol.HostEntry{
			{Domain: "example.com", IP: "127.0.0.1", Alias: "example", Enabled: true, Group: "dev"},
		}
		lv.SetItems(entries)

		view := lv.View()
		// Group header is shown as section title (uppercase)
		assert.Contains(t, view, "DEV")
		// Table headers
		assert.Contains(t, view, "DOMAIN")
		assert.Contains(t, view, "IP ADDRESS")
		assert.Contains(t, view, "STATUS")
		// Data is in the view
		assert.Contains(t, view, "example.com")
		assert.Contains(t, view, "127.0.0.1")
		assert.Contains(t, view, "Active")
	})
}

func TestListView_SetSize(t *testing.T) {
	lv := NewListView()
	lv.SetSize(80, 24)

	assert.Equal(t, 80, lv.width)
	assert.Equal(t, 24, lv.height)
}

func TestListView_CursorBounds(t *testing.T) {
	lv := NewListView()

	// Set items
	entries := []protocol.HostEntry{
		{Domain: "a.com", IP: "127.0.0.1", Alias: "a", Enabled: true, Group: "dev"},
		{Domain: "b.com", IP: "127.0.0.1", Alias: "b", Enabled: true, Group: "dev"},
	}
	lv.SetItems(entries)
	lv.cursor = 1

	// Set fewer items - cursor should be adjusted
	entries = []protocol.HostEntry{
		{Domain: "a.com", IP: "127.0.0.1", Alias: "a", Enabled: true, Group: "dev"},
	}
	lv.SetItems(entries)

	assert.Equal(t, 0, lv.cursor)
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is too long", 10, "this is..."},
		{"", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMax(t *testing.T) {
	assert.Equal(t, 5, max(3, 5))
	assert.Equal(t, 5, max(5, 3))
	assert.Equal(t, 5, max(5, 5))
	assert.Equal(t, 0, max(0, -1))
}

// Matrix test for navigation
func TestListView_Navigation_Matrix(t *testing.T) {
	sizes := []int{1, 5, 10, 100}

	for _, size := range sizes {
		t.Run("size="+string(rune('0'+size)), func(t *testing.T) {
			lv := NewListView()

			entries := make([]protocol.HostEntry, size)
			for i := range entries {
				entries[i] = protocol.HostEntry{
					Domain:  "domain" + string(rune('a'+i%26)) + ".com",
					IP:      "127.0.0.1",
					Alias:   "alias" + string(rune('a'+i%26)),
					Enabled: true,
					Group:   "dev",
				}
			}
			lv.SetItems(entries)

			// Move to end
			for i := 0; i < size*2; i++ {
				lv.MoveDown()
			}
			assert.Equal(t, size-1, lv.cursor)

			// Move to start
			for i := 0; i < size*2; i++ {
				lv.MoveUp()
			}
			assert.Equal(t, 0, lv.cursor)
		})
	}
}

func BenchmarkListView_SetItems(b *testing.B) {
	entries := make([]protocol.HostEntry, 100)
	for i := range entries {
		entries[i] = protocol.HostEntry{
			Domain:  "domain.com",
			IP:      "127.0.0.1",
			Alias:   "alias",
			Enabled: true,
			Group:   "dev",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lv := NewListView()
		lv.SetItems(entries)
	}
}

func BenchmarkListView_Filter(b *testing.B) {
	lv := NewListView()
	entries := make([]protocol.HostEntry, 100)
	for i := range entries {
		entries[i] = protocol.HostEntry{
			Domain:  "domain" + string(rune('a'+i%26)) + ".com",
			IP:      "127.0.0.1",
			Alias:   "alias" + string(rune('a'+i%26)),
			Enabled: true,
			Group:   "dev",
		}
	}
	lv.SetItems(entries)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lv.Filter("domain")
	}
}

func BenchmarkListView_View(b *testing.B) {
	lv := NewListView()
	entries := make([]protocol.HostEntry, 50)
	for i := range entries {
		entries[i] = protocol.HostEntry{
			Domain:  "domain.com",
			IP:      "127.0.0.1",
			Alias:   "alias",
			Enabled: i%2 == 0,
			Group:   "group" + string(rune('a'+i%5)),
		}
	}
	lv.SetItems(entries)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lv.View()
	}
}

func TestListView_DeleteSimulation(t *testing.T) {
	lv := NewListView()

	// Initial entries
	entries := []protocol.HostEntry{
		{Domain: "a.com", IP: "127.0.0.1", Alias: "a", Enabled: true, Group: "dev"},
		{Domain: "b.com", IP: "127.0.0.1", Alias: "b", Enabled: false, Group: "dev"},
		{Domain: "c.com", IP: "192.168.1.1", Alias: "c", Enabled: true, Group: "staging"},
	}
	lv.SetItems(entries)
	require.Equal(t, 3, lv.Len())

	// Select the second item
	lv.MoveDown()
	selected := lv.Selected()
	require.NotNil(t, selected)
	require.Equal(t, "b", selected.Entry.Alias)

	// Simulate delete: set new items without the deleted entry
	entriesAfterDelete := []protocol.HostEntry{
		{Domain: "a.com", IP: "127.0.0.1", Alias: "a", Enabled: true, Group: "dev"},
		{Domain: "c.com", IP: "192.168.1.1", Alias: "c", Enabled: true, Group: "staging"},
	}
	lv.SetItems(entriesAfterDelete)

	// Verify list has only 2 items now
	assert.Equal(t, 2, lv.Len())

	// Verify "b" is no longer in the list
	for i := 0; i < lv.Len(); i++ {
		assert.NotEqual(t, "b", lv.items[i].Entry.Alias, "deleted entry should not be in list")
	}

	// Cursor should be adjusted if needed
	assert.LessOrEqual(t, lv.cursor, lv.Len()-1)
}

func TestListView_SetItemsWithNil(t *testing.T) {
	lv := NewListView()

	// Initial entries
	entries := []protocol.HostEntry{
		{Domain: "a.com", IP: "127.0.0.1", Alias: "a", Enabled: true, Group: "dev"},
	}
	lv.SetItems(entries)
	require.Equal(t, 1, lv.Len())

	// Set nil entries (simulating empty list from server)
	lv.SetItems(nil)
	assert.Equal(t, 0, lv.Len())
	assert.Equal(t, 0, lv.cursor)
}

func TestListView_SetItemsWithEmptySlice(t *testing.T) {
	lv := NewListView()

	// Initial entries
	entries := []protocol.HostEntry{
		{Domain: "a.com", IP: "127.0.0.1", Alias: "a", Enabled: true, Group: "dev"},
	}
	lv.SetItems(entries)
	require.Equal(t, 1, lv.Len())

	// Set empty slice
	lv.SetItems([]protocol.HostEntry{})
	assert.Equal(t, 0, lv.Len())
	assert.Equal(t, 0, lv.cursor)
}
