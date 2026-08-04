package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/mevanlc/gdu/v5/pkg/fs"
)

func (ui *UI) createCollectorTable() {
	ui.collectorTable = tview.NewTable().SetSelectable(true, false)
	ui.collectorTable.SetBackgroundColor(tcell.ColorDefault)
	ui.collectorTable.SetBorder(true).SetBorderColor(tcell.ColorDefault)
	ui.collectorTable.SetSelectedStyle(tcell.Style{}.
		Foreground(ui.selectedTextColor).
		Background(ui.selectedBackgroundColor))
	ui.collectorTable.SetFocusFunc(func() {
		ui.setContentFocusState(collectorFocus)
	})
	ui.collectorTable.SetBlurFunc(func() {
		ui.collectorFocused = false
	})
	ui.showCollector()
}

func collectorKey(path string) string {
	return filepath.Clean(path)
}

func (ui *UI) isCollected(item fs.Item) bool {
	if !ui.collectorEnabled || item == nil {
		return false
	}
	_, ok := ui.collectorItems[collectorKey(item.GetPath())]
	return ok
}

func (ui *UI) toggleCollected(item fs.Item) {
	key := collectorKey(item.GetPath())
	if _, ok := ui.collectorItems[key]; ok {
		ui.removeCollectedKey(key)
		return
	}
	ui.collectorItems[key] = item
	ui.collectorOrder = append(ui.collectorOrder, key)
}

func (ui *UI) removeCollectedKey(key string) {
	if _, ok := ui.collectorItems[key]; !ok {
		return
	}
	delete(ui.collectorItems, key)
	for index, candidate := range ui.collectorOrder {
		if candidate == key {
			ui.collectorOrder = append(ui.collectorOrder[:index], ui.collectorOrder[index+1:]...)
			break
		}
	}
}

func (ui *UI) clearCollector() {
	ui.collectorItems = make(map[string]fs.Item)
	ui.collectorOrder = nil
}

func (ui *UI) collectedItemsInOrder() []fs.Item {
	items := make([]fs.Item, 0, len(ui.collectorOrder))
	for _, key := range ui.collectorOrder {
		if item, ok := ui.collectorItems[key]; ok {
			items = append(items, item)
		}
	}
	return items
}

func (ui *UI) resolvedCollectedItemsInOrder() []fs.Item {
	items := ui.collectedItemsInOrder()
	for index, item := range items {
		resolved := findItemInTree(ui.topDir, item.GetPath())
		if resolved == nil {
			continue
		}
		key := collectorKey(item.GetPath())
		ui.collectorItems[key] = resolved
		items[index] = resolved
	}
	return items
}

func findItemInTree(root fs.Item, path string) fs.Item {
	if root == nil {
		return nil
	}
	target := collectorKey(path)
	current := root
	for current != nil {
		currentPath := collectorKey(current.GetPath())
		if currentPath == target {
			return current
		}
		if !current.IsDir() || !isDescendantPath(target, currentPath) {
			return nil
		}

		var next fs.Item
		for child := range current.GetFiles(fs.SortByName, fs.SortAsc) {
			childPath := collectorKey(child.GetPath())
			if childPath == target || isDescendantPath(target, childPath) {
				next = child
				break
			}
		}
		current = next
	}
	return nil
}

func (ui *UI) markedItems() []fs.Item {
	if ui.collectorEnabled {
		return ui.resolvedCollectedItemsInOrder()
	}

	items := make([]fs.Item, 0, len(ui.markedRows))
	for row := range ui.markedRows {
		cell := ui.table.GetCell(row, 0)
		if cell == nil {
			continue
		}
		if item, ok := cell.GetReference().(fs.Item); ok {
			items = append(items, item)
		}
	}
	return items
}

func (ui *UI) markedItemCount() int {
	if ui.collectorEnabled {
		return len(ui.collectorItems)
	}
	return len(ui.markedRows)
}

func (ui *UI) itemMarkedAtRow(row int, item fs.Item) bool {
	if ui.collectorEnabled {
		return ui.isCollected(item)
	}
	_, marked := ui.markedRows[row]
	return marked
}

func (ui *UI) showCollector() {
	if ui.collectorTable == nil {
		return
	}

	ui.collectorTable.Clear()
	title := fmt.Sprintf(" Collector (%d) — Tab/S-Tab focus, Esc back, d remove, D clear ", len(ui.collectorItems))
	if ui.collectorPrintOnExit {
		title = fmt.Sprintf(" Collector (%d, print on) — Tab/S-Tab focus, Esc back, d remove, D clear ", len(ui.collectorItems))
	}
	ui.collectorTable.SetTitle(title)

	row := 0
	for _, key := range ui.collectorOrder {
		item, ok := ui.collectorItems[key]
		if !ok {
			continue
		}
		cell := tview.NewTableCell(tview.Escape(item.GetPath())).SetReference(key)
		if ui.collectedItemAppliesToCurrentDir(item) {
			background := tcell.ColorDarkSlateGray
			if !ui.UseColors {
				background = tcell.ColorGray
			}
			cell.SetBackgroundColor(background)
		}
		ui.collectorTable.SetCell(row, 0, cell)
		row++
	}

	if row == 0 {
		ui.collectorTable.SetCell(0, 0, tview.NewTableCell(" No collected items ").SetSelectable(false))
	}
}

func (ui *UI) collectedItemAppliesToCurrentDir(item fs.Item) bool {
	if ui.currentDir == nil || item == nil {
		return false
	}
	itemKey := collectorKey(item.GetPath())
	for row := 0; row < ui.table.GetRowCount(); row++ {
		cell := ui.table.GetCell(row, 0)
		if cell == nil {
			continue
		}
		if displayed, ok := cell.GetReference().(fs.Item); ok &&
			collectorKey(displayed.GetPath()) == itemKey {
			return true
		}
	}
	if parent := item.GetParent(); parent != nil {
		return collectorKey(parent.GetPath()) == collectorKey(ui.currentDir.GetPath())
	}
	return collectorKey(filepath.Dir(item.GetPath())) == collectorKey(ui.currentDir.GetPath())
}

func (ui *UI) handleCollectorActions(key *tcell.EventKey) *tcell.EventKey {
	//nolint:exhaustive // Collector keys are an intentional allowlist.
	switch key.Key() {
	case tcell.KeyTab:
		ui.cycleContentFocus(false)
		return nil
	case tcell.KeyBacktab:
		ui.cycleContentFocus(true)
		return nil
	case tcell.KeyEscape:
		ui.focusTable()
		return nil
	case tcell.KeyCtrlC:
		return key
	}

	switch {
	case key.Rune() == 'p':
		ui.printMarked()
		return nil
	case key.Rune() == 'D':
		ui.clearCollector()
		ui.redrawCollectorSelection(0)
		return nil
	case key.Rune() == 'd', key.Rune() == ' ', key.Key() == tcell.KeyDelete,
		key.Key() == tcell.KeyBackspace, key.Key() == tcell.KeyBackspace2:
		row, _ := ui.collectorTable.GetSelection()
		cell := ui.collectorTable.GetCell(row, 0)
		if cell == nil {
			return nil
		}
		collectedKey, ok := cell.GetReference().(string)
		if !ok {
			return nil
		}
		ui.removeCollectedKey(collectedKey)
		ui.redrawCollectorSelection(row)
		return nil
	}

	if isCollectorNavigationKey(key) {
		return key
	}

	// Collector actions are deliberately allowlisted so directory commands do
	// not accidentally act on an item in the unfocused directory pane.
	return nil
}

func isCollectorNavigationKey(key *tcell.EventKey) bool {
	//nolint:exhaustive // Only navigation understood by tview.Table is forwarded.
	switch key.Key() {
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyLeft, tcell.KeyRight,
		tcell.KeyHome, tcell.KeyEnd, tcell.KeyPgUp, tcell.KeyPgDn,
		tcell.KeyCtrlF, tcell.KeyCtrlB:
		return true
	case tcell.KeyRune:
		switch key.Rune() {
		case 'g', 'G', 'j', 'k', 'h', 'l':
			return true
		}
	}
	return false
}

func (ui *UI) redrawCollectorSelection(row int) {
	if ui.currentDir != nil {
		ui.showDir()
	} else {
		ui.showCollector()
	}
	ui.focusCollector()
	if len(ui.collectorItems) > 0 {
		ui.collectorTable.Select(min(row, len(ui.collectorItems)-1), 0)
	}
}

func (ui *UI) removeCollectedAfterAction(item fs.Item, shouldEmpty bool) {
	if !ui.collectorEnabled || item == nil {
		return
	}

	root := collectorKey(item.GetPath())
	if !shouldEmpty {
		ui.removeCollectedTree(root, true)
		return
	}
	if item.IsDir() {
		ui.removeCollectedTree(root, false)
		return
	}

	// Emptying a file replaces its in-memory item. Keep it collected, but point
	// at the replacement so a later action does not operate on a stale object.
	if parent := item.GetParent(); parent != nil {
		for candidate := range parent.GetFiles(fs.SortByName, fs.SortAsc) {
			if collectorKey(candidate.GetPath()) == root {
				ui.collectorItems[root] = candidate
				break
			}
		}
	}
}

func (ui *UI) removeCollectedTree(root string, includeRoot bool) {
	for _, key := range append([]string(nil), ui.collectorOrder...) {
		if (includeRoot && key == root) || isDescendantPath(key, root) {
			ui.removeCollectedKey(key)
		}
	}
}

func isDescendantPath(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == "." || relative == ".." {
		return false
	}
	return !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func compactCollectorActions(items []fs.Item, shouldEmpty bool) []fs.Item {
	result := make([]fs.Item, 0, len(items))
	for _, item := range items {
		covered := false
		path := collectorKey(item.GetPath())
		for _, candidate := range items {
			if candidate == item || (shouldEmpty && !candidate.IsDir()) {
				continue
			}
			if isDescendantPath(path, collectorKey(candidate.GetPath())) {
				covered = true
				break
			}
		}
		if !covered {
			result = append(result, item)
		}
	}
	return result
}
