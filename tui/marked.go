package tui

import (
	"strconv"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/gdamore/tcell/v2"
	"github.com/mevanlc/gdu/v5/pkg/fs"
	"github.com/rivo/tview"
)

func (ui *UI) fileItemMarked(row int) {
	if ui.collectorEnabled {
		cell := ui.table.GetCell(row, 0)
		if cell == nil {
			return
		}
		item, ok := cell.GetReference().(fs.Item)
		if !ok {
			return
		}
		ui.toggleCollected(item)
		ui.showDir()
		ui.table.Select(min(row+1, ui.table.GetRowCount()-1), 0)
		return
	}

	if _, ok := ui.markedRows[row]; ok {
		delete(ui.markedRows, row)
	} else {
		ui.markedRows[row] = struct{}{}
	}
	ui.showDir()
	// select next row if possible
	ui.table.Select(min(row+1, ui.table.GetRowCount()-1), 0)
}

func (ui *UI) deleteMarked(shouldEmpty bool) {
	var action, acting string
	if shouldEmpty {
		action = actionEmpty
		acting = actingEmpty
	} else {
		action = actionDelete
		acting = actingDelete
	}

	markedItems := ui.markedItems()

	if ui.deleteInBackground {
		ui.queueForDeletion(markedItems, shouldEmpty)
		return
	}

	modal := tview.NewModal()
	ui.pages.AddPage(acting, modal, true, true)

	currentRow, _ := ui.table.GetSelection()

	go func() {
		var completed []fs.Item
		for _, one := range markedItems {
			if ui.collectorEnabled && coveredByCompletedAction(one, completed, shouldEmpty) {
				continue
			}
			ui.app.QueueUpdateDraw(func() {
				modal.SetText(
					cases.Title(language.English).String(acting) +
						" " +
						tview.Escape(one.GetName()) +
						"...",
				)
			})
			if err := ui.deleteOne(one, shouldEmpty, false); err != nil {
				msg := "Can't " + action + " " + tview.Escape(one.GetName())
				ui.app.QueueUpdateDraw(func() {
					ui.ensureCurrentDirAfterActions(completed, shouldEmpty)
					ui.applyCompletedCollectorActions(completed, shouldEmpty)
					ui.pages.RemovePage(acting)
					if ui.currentDir != nil {
						ui.showDir()
					}
					ui.showErr(msg, err)
				})
				if ui.done != nil {
					ui.done <- struct{}{}
				}
				return
			}
			completed = append(completed, one)
		}

		ui.app.QueueUpdateDraw(func() {
			ui.pages.RemovePage(acting)
			ui.ensureCurrentDirAfterActions(completed, shouldEmpty)
			ui.applyCompletedCollectorActions(completed, shouldEmpty)
			if !ui.collectorEnabled {
				ui.markedRows = make(map[int]struct{})
			}
			x, y := ui.table.GetOffset()
			if ui.currentDir != nil {
				ui.showDir()
				ui.table.Select(min(currentRow, ui.table.GetRowCount()-1), 0)
				ui.table.SetOffset(min(x, ui.table.GetRowCount()-1), y)
			}
		})

		if ui.done != nil {
			ui.done <- struct{}{}
		}
	}()
}

func (ui *UI) deleteOne(item fs.Item, shouldEmpty, locked bool) error {
	deleteFun := ui.remover
	if shouldEmpty && !item.IsDir() {
		deleteFun = ui.emptier
	}

	parent := item.GetParent()
	deleteItems := []fs.Item{item}
	if shouldEmpty && item.IsDir() {
		parent = item
		deleteItems = nil
		files := item.GetFiles(fs.SortBySize, fs.SortDesc)
		if locked {
			files = item.GetFilesLocked(fs.SortBySize, fs.SortDesc)
		}
		for child := range files {
			deleteItems = append(deleteItems, child)
		}
	}
	if parent == nil {
		parent = ui.currentDir
	}

	for _, toDelete := range deleteItems {
		if err := deleteFun(parent, toDelete); err != nil {
			return err
		}
	}
	return nil
}

func coveredByCompletedAction(item fs.Item, completed []fs.Item, shouldEmpty bool) bool {
	path := collectorKey(item.GetPath())
	for _, actedOn := range completed {
		if pathRemovedByAction(path, actedOn, shouldEmpty) {
			return true
		}
	}
	return false
}

func pathRemovedByAction(path string, item fs.Item, shouldEmpty bool) bool {
	root := collectorKey(item.GetPath())
	return (!shouldEmpty && (path == root || isDescendantPath(path, root))) ||
		(shouldEmpty && item.IsDir() && isDescendantPath(path, root))
}

func (ui *UI) ensureCurrentDirAfterActions(items []fs.Item, shouldEmpty bool) {
	for ui.currentDir != nil {
		path := collectorKey(ui.currentDir.GetPath())
		removed := false
		for _, item := range items {
			if pathRemovedByAction(path, item, shouldEmpty) {
				removed = true
				break
			}
		}
		if !removed {
			return
		}
		ui.currentDir = ui.currentDir.GetParent()
	}
}

func (ui *UI) applyCompletedCollectorActions(items []fs.Item, shouldEmpty bool) {
	if !ui.collectorEnabled {
		return
	}
	for _, item := range items {
		ui.removeCollectedAfterAction(item, shouldEmpty)
	}
	ui.showCollector()
}

func (ui *UI) confirmDeletionMarked(shouldEmpty bool) {
	var action string
	if shouldEmpty {
		action = actionEmpty
	} else {
		action = actionDelete
	}

	modal := tview.NewModal().
		SetText(
			"Are you sure you want to " +
				action + " [::b]" +
				strconv.Itoa(ui.markedItemCount()) +
				"[::-] items?",
		).
		AddButtons([]string{yesButton, noButton, dontAskAgainButton}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			switch buttonIndex {
			case 2:
				ui.askBeforeDelete = false
				fallthrough
			case 0:
				ui.deleteMarked(shouldEmpty)
			}
			ui.pages.RemovePage("confirm")
		})
	setYesNoKeys(modal, 0, 1)

	if !ui.UseColors {
		modal.SetBackgroundColor(tcell.ColorGray)
	} else {
		modal.SetBackgroundColor(tcell.ColorBlack)
	}
	modal.SetBorderColor(tcell.ColorDefault)

	ui.pages.AddPage("confirm", modal, true, true)
}

func (ui *UI) printMarked() {
	if ui.collectorEnabled {
		ui.collectorPrintOnExit = true
		ui.showCollector()
		return
	}

	if len(ui.markedRows) == 0 {
		return
	}
	for row := range ui.markedRows {
		item := ui.table.GetCell(row, 0).GetReference().(fs.Item)
		ui.markedPaths = append(ui.markedPaths, item.GetPath())
	}
	ui.markedRows = make(map[int]struct{})
	selectRow, _ := ui.table.GetSelection()
	ui.showDir()
	ui.table.Select(selectRow, 0)
}
