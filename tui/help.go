package tui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type helpAction uint8

const (
	helpActionNone helpAction = iota
	helpActionDelete
	helpActionTrash
	helpActionSpawnShell
	helpActionViewFile
)

type helpEntry struct {
	keys        string
	description string
	action      helpAction
}

type helpSection struct {
	title   string
	entries []helpEntry
}

var helpSections = []helpSection{
	{entries: []helpEntry{
		{keys: "up/down, k/j", description: "Move cursor up/down"},
		{keys: "pgup/pgdn, g/G", description: "Move cursor top/bottom"},
		{keys: "enter, right, l", description: "Go to directory/device"},
		{keys: "left, h", description: "Go to parent directory"},
		{keys: "tab/shift-tab", description: "Cycle directory, filters, and collector focus"},
		{keys: "ctrl-l", description: "Redraw screen"},
	}},
	{entries: []helpEntry{
		{keys: "r", description: "Rescan current directory"},
		{keys: "E", description: "Export analysis data to file as JSON"},
		{keys: "/", description: "Search items by name"},
		{keys: "T", description: "Filter items by file type (extension)"},
		{keys: "a", description: "Toggle between showing disk usage and apparent size"},
		{keys: "B", description: "Toggle bar alignment to biggest file or directory"},
		{keys: "c", description: "Show/hide file count"},
		{keys: "m", description: "Show/hide latest mtime"},
		{keys: "b", description: "Spawn shell in current directory", action: helpActionSpawnShell},
		{keys: "q", description: "Quit gdu (asks to confirm after a long scan)"},
		{keys: "Q", description: "Quit gdu and print current directory path"},
	}},
	{title: "Item under cursor:", entries: []helpEntry{
		{keys: "d", description: "Delete file or directory", action: helpActionDelete},
		{keys: "e", description: "Empty file or directory", action: helpActionDelete},
		{keys: "t", description: "Trash file or directory", action: helpActionTrash},
		{keys: "space", description: "Mark file or directory for deletion"},
		{keys: "p", description: "Print marked items paths to stdout after quitting"},
		{keys: "I", description: "Ignore file or directory"},
		{keys: "v", description: "Show content of file", action: helpActionViewFile},
		{keys: "o", description: "Open file or directory in external program", action: helpActionSpawnShell},
		{keys: "i", description: "Show info about item"},
	}},
	{title: "Collector (when focused):", entries: []helpEntry{
		{keys: "arrows, hjkl", description: "Move through collected items"},
		{keys: "esc", description: "Return focus to directory list"},
		{keys: "space, d", description: "Remove item from collector"},
		{keys: "D", description: "Remove all items from collector"},
	}},
	{title: "Sort by (twice toggles asc/desc):", entries: []helpEntry{
		{keys: "n", description: "Sort by name (asc/desc)"},
		{keys: "s", description: "Sort by size (asc/desc)"},
		{keys: "C", description: "Sort by file count (asc/desc)"},
		{keys: "M", description: "Sort by mtime (asc/desc)"},
	}},
}

func (ui *UI) showHelp() {
	text := tview.NewTextView().SetDynamicColors(true)
	text.SetBorder(true).SetBorderPadding(2, 2, 2, 2)
	text.SetBorderColor(tcell.ColorDefault)
	text.SetTitle(" gdu help ")
	text.SetScrollable(true)

	formattedHelpText := ui.formatHelpTextFor()
	text.SetText(formattedHelpText)

	maxHeight := strings.Count(formattedHelpText, "\n") + 7
	_, height := ui.screen.Size()
	if height > maxHeight {
		height = maxHeight
	}

	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(text, height, 1, false).
			AddItem(nil, 0, 1, false), 80, 1, false).
		AddItem(nil, 0, 1, false)

	ui.help = flex
	ui.pages.AddPage("help", flex, true, true)
	ui.focusOverlay(text)
}

func (ui *UI) formatHelpTextFor() string {
	keyStyle, descriptionStyle := defaultColorBold, whiteOnBlack
	if ui.UseColors {
		keyStyle, descriptionStyle = "[red]", "[white]"
	}

	// Use one key column for every section, measured in terminal cells.
	keyWidth := 0
	for _, section := range helpSections {
		for _, entry := range section.entries {
			keyWidth = max(keyWidth, tview.TaggedStringWidth(tview.Escape(entry.keys)))
		}
	}

	var text strings.Builder
	for i, section := range helpSections {
		if i > 0 {
			text.WriteString("\n\n")
		}
		if section.title != "" {
			fmt.Fprintf(&text, "%s%s\n", descriptionStyle, tview.Escape(section.title))
		}
		for j, entry := range section.entries {
			if j > 0 {
				text.WriteByte('\n')
			}
			keys := tview.Escape(entry.keys)
			padding := strings.Repeat(" ", keyWidth-tview.TaggedStringWidth(keys)+1)
			fmt.Fprintf(&text, "%s%s%s     %s%s%s", padding, keyStyle, keys,
				descriptionStyle, tview.Escape(entry.description), ui.helpActionSuffix(entry.action))
		}
	}
	return text.String()
}

func (ui *UI) helpActionSuffix(action helpAction) string {
	const disabled = " (disabled)"
	switch action {
	case helpActionNone:
	case helpActionDelete, helpActionTrash:
		switch {
		case ui.noDelete:
			return disabled
		case ui.noDeleteWithFilter:
			return " (disabled/filter)"
		case action == helpActionTrash && len(ui.trashCmd) == 0:
			return disabled
		}
	case helpActionSpawnShell:
		if ui.noSpawnShell {
			return disabled
		}
	case helpActionViewFile:
		if ui.noViewFile {
			return disabled
		}
	}
	return ""
}
