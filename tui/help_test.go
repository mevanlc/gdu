package tui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/mevanlc/gdu/v5/internal/testapp"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelpColumnAlignment(t *testing.T) {
	for _, mode := range []string{"color", "monochrome"} {
		t.Run(mode, func(t *testing.T) {
			app, screen := testapp.CreateTestAppWithSimScreen(100, 50)
			t.Cleanup(screen.Fini)
			ui := CreateUI(app, screen, &bytes.Buffer{}, mode == "color", true, false, false)
			ui.showHelp()
			ui.help.SetRect(0, 0, 100, 50)
			ui.help.Draw(screen)

			lines := make([]string, 50)
			for y := range lines {
				var line strings.Builder
				for x := range 100 {
					cell, _, _ := screen.Get(x, y)
					line.WriteString(cell)
				}
				lines[y] = line.String()
			}

			descriptionColumn, keyEndColumn := -1, -1
			for _, entry := range []struct{ keys, description string }{
				{"up/down, k/j", "Move cursor up/down"},
				{"enter, right, l", "Go to directory/device"},
				{"r", "Rescan current directory"},
				{"space", "Mark file or directory for deletion"},
				{"p", "Print marked items paths to stdout after quitting"},
				{"I", "Ignore file or directory"},
				{"esc", "Return focus to directory list"},
				{"M", "Sort by mtime (asc/desc)"},
			} {
				found := false
				for y, line := range lines {
					column := strings.Index(line, entry.description)
					if column < 0 {
						continue
					}
					found = true
					prefix := strings.TrimRight(line[:column], " ")
					assert.True(t, strings.HasSuffix(prefix, entry.keys), "key not rendered: %s", entry.keys)
					column = tview.TaggedStringWidth(line[:column])
					keyEnd := tview.TaggedStringWidth(prefix)
					if descriptionColumn < 0 {
						descriptionColumn, keyEndColumn = column, keyEnd
					}
					assert.Equal(t, descriptionColumn, column, entry.description)
					assert.Equal(t, keyEndColumn, keyEnd, entry.keys)
					_, style, _ := screen.Get(keyEnd-1, y)
					foreground, _, attributes := style.Decompose()
					if mode == "color" {
						assert.Equal(t, tcell.ColorRed, foreground)
					} else {
						assert.NotZero(t, attributes&tcell.AttrBold)
					}
					break
				}
				require.True(t, found, "description not rendered: %s", entry.description)
			}
		})
	}
}

func TestHelpDeleteAndTrashAvailability(t *testing.T) {
	for _, tt := range []struct {
		name           string
		noDelete       bool
		noDeleteFilter bool
		trashCmd       []string
		deletionSuffix string
		trashSuffix    string
	}{
		{name: "trash_unconfigured", trashSuffix: " (disabled)"},
		{name: "enabled", trashCmd: []string{"trash"}},
		{name: "no_delete", noDelete: true, deletionSuffix: " (disabled)", trashSuffix: " (disabled)"},
		{name: "filter", noDeleteFilter: true, deletionSuffix: " (disabled/filter)", trashSuffix: " (disabled/filter)"},
		{name: "no_delete_and_filter", noDelete: true, noDeleteFilter: true,
			deletionSuffix: " (disabled)", trashSuffix: " (disabled)"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			app, screen := testapp.CreateTestAppWithSimScreen(100, 50)
			t.Cleanup(screen.Fini)
			ui := CreateUI(app, screen, &bytes.Buffer{}, true, true, false, false)
			ui.noDelete, ui.noDeleteWithFilter, ui.trashCmd = tt.noDelete, tt.noDeleteFilter, tt.trashCmd

			text := ui.formatHelpTextFor()
			assert.Contains(t, text, "Delete file or directory"+tt.deletionSuffix+"\n")
			assert.Contains(t, text, "Empty file or directory"+tt.deletionSuffix+"\n")
			assert.Contains(t, text, "Trash file or directory"+tt.trashSuffix+"\n")
			assert.Contains(t, text, "Mark file or directory for deletion\n")
		})
	}
}
