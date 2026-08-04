package tui

import (
	"github.com/mevanlc/gdu/v5/pkg/fs"
	"github.com/rivo/tview"
)

func (ui *UI) queueForDeletion(items []fs.Item, shouldEmpty bool) {
	if ui.collectorEnabled {
		items = compactCollectorActions(items, shouldEmpty)
	}
	go func() {
		for _, item := range items {
			ui.deleteQueue <- deleteQueueItem{item: item, shouldEmpty: shouldEmpty}
		}
	}()

	if !ui.collectorEnabled {
		ui.markedRows = make(map[int]struct{})
	}
}

func (ui *UI) deleteWorker() {
	defer func() {
		if r := recover(); r != nil {
			ui.app.Stop()
			panic(r)
		}
	}()

	for item := range ui.deleteQueue {
		ui.deleteItem(item.item, item.shouldEmpty)
	}
}

func (ui *UI) deleteItem(item fs.Item, shouldEmpty bool) {
	ui.increaseActiveWorkers()
	defer ui.decreaseActiveWorkers()

	var action string
	if shouldEmpty {
		action = actionEmpty
	} else {
		action = actionDelete
	}

	if err := ui.deleteOne(item, shouldEmpty, true); err != nil {
		msg := "Can't " + action + " " + tview.Escape(item.GetName())
		ui.app.QueueUpdateDraw(func() {
			ui.showErr(msg, err)
		})
		if ui.done != nil {
			ui.done <- struct{}{}
		}
		return
	}

	ui.app.QueueUpdateDraw(func() {
		ui.ensureCurrentDirAfterActions([]fs.Item{item}, shouldEmpty)
		ui.removeCollectedAfterAction(item, shouldEmpty)
		ui.showCollector()
		if ui.currentDir != nil {
			row, _ := ui.table.GetSelection()
			x, y := ui.table.GetOffset()
			ui.showDir()
			ui.table.Select(min(row, ui.table.GetRowCount()-1), 0)
			ui.table.SetOffset(min(x, ui.table.GetRowCount()-1), y)
		}
	})
	if ui.done != nil {
		ui.done <- struct{}{}
	}
}

func (ui *UI) increaseActiveWorkers() {
	ui.workersMut.Lock()
	defer ui.workersMut.Unlock()
	ui.activeWorkers++
}

func (ui *UI) decreaseActiveWorkers() {
	ui.workersMut.Lock()
	defer ui.workersMut.Unlock()
	ui.activeWorkers--
}
