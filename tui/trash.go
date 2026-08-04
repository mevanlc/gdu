package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mevanlc/gdu/v5/pkg/fs"
	"github.com/rivo/tview"
)

const actingTrash = "trashing"

func runTrashCommand(argv []string, selectedPath string) ([]byte, error) {
	args := append([]string(nil), argv[1:]...)
	args = append(args, selectedPath)

	cmd := exec.Command(argv[0], args...)
	return cmd.CombinedOutput()
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !os.IsNotExist(err)
}

func (ui *UI) handleTrash() {
	if len(ui.trashCmd) == 0 || ui.currentDir == nil {
		return
	}
	if ui.noDelete {
		ui.showErr("Trash is disabled", nil)
		return
	}
	if ui.noDeleteWithFilter {
		ui.showErr(
			"Trash is disabled when a time filter is active.\n\n"+
				"To override, set GDU_ALLOW_DELETE_WITH_FILTER=1",
			nil,
		)
		return
	}
	if ui.collectorEnabled && ui.markedItemCount() > 0 {
		ui.trashMarked()
		return
	}

	row, column := ui.table.GetSelection()
	selectedItem, ok := ui.table.GetCell(row, column).GetReference().(fs.Item)
	if !ok || selectedItem == ui.currentDir.GetParent() {
		return
	}

	ui.trashSelected(row, selectedItem)
}

type trashResult struct {
	item   fs.Item
	output []byte
	err    error
	moved  bool
}

func (ui *UI) trashMarked() {
	items := compactCollectorActions(ui.resolvedCollectedItemsInOrder(), false)
	modal := tview.NewModal().SetText(fmt.Sprintf("Trashing %d collected items...", len(items)))
	ui.pages.AddPage(actingTrash, modal, true, true)

	go func() {
		results := make([]trashResult, 0, len(items))
		for _, item := range items {
			output, err := ui.trashRunner(ui.trashCmd, item.GetPath())
			result := trashResult{
				item:   item,
				output: output,
				err:    err,
				moved:  !pathExists(item.GetPath()),
			}
			results = append(results, result)
			if err != nil {
				break
			}
		}

		ui.app.QueueUpdateDraw(func() {
			ui.pages.RemovePage(actingTrash)
			movedItems := make([]fs.Item, 0, len(results))
			for _, result := range results {
				if result.moved {
					movedItems = append(movedItems, result.item)
					ui.removeTrashedItemFromModel(result.item)
					ui.removeCollectedAfterAction(result.item, false)
				}
			}
			ui.ensureCurrentDirAfterActions(movedItems, false)
			if ui.currentDir != nil {
				ui.showDir()
			}
			for _, result := range results {
				if result.err != nil {
					ui.showTrashErr(result.output, result.err)
					break
				}
			}
		})

		if ui.done != nil {
			ui.done <- struct{}{}
		}
	}()
}

func (ui *UI) trashSelected(row int, selectedItem fs.Item) {
	selectedPath := selectedItem.GetPath()
	modal := tview.NewModal().SetText(
		"Trashing " +
			tview.Escape(selectedItem.GetName()) +
			"...",
	)
	ui.pages.AddPage(actingTrash, modal, true, true)

	go func() {
		output, err := ui.trashRunner(ui.trashCmd, selectedPath)
		exists := pathExists(selectedPath)

		ui.app.QueueUpdateDraw(func() {
			ui.pages.RemovePage(actingTrash)
			if !exists {
				ui.removeTrashedItem(row, selectedItem)
			}
			if err != nil {
				ui.showTrashErr(output, err)
			}
		})

		if ui.done != nil {
			ui.done <- struct{}{}
		}
	}()
}

func (ui *UI) removeTrashedItem(row int, selectedItem fs.Item) {
	ui.removeTrashedItemFromModel(selectedItem)

	x, y := ui.table.GetOffset()
	ui.showDir()
	ui.table.Select(min(row, ui.table.GetRowCount()-1), 0)
	ui.table.SetOffset(min(x, ui.table.GetRowCount()-1), y)
}

func (ui *UI) removeTrashedItemFromModel(selectedItem fs.Item) {
	parent := selectedItem.GetParent()
	if parent == nil {
		parent = ui.currentDir
	}
	parent.RemoveFile(selectedItem)
}

func (ui *UI) showTrashErr(output []byte, err error) {
	errText := err.Error()
	outputText := strings.TrimSpace(string(output))
	if outputText != "" {
		errText = fmt.Sprintf("%s\n\n%s", errText, outputText)
	}
	ui.showErr("Trash command failed", errors.New(errText))
}
