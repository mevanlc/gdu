package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/dundee/gdu/v5/pkg/fs"
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

	row, column := ui.table.GetSelection()
	selectedItem, ok := ui.table.GetCell(row, column).GetReference().(fs.Item)
	if !ok || selectedItem == ui.currentDir.GetParent() {
		return
	}

	ui.trashSelected(row, selectedItem)
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
	parent := selectedItem.GetParent()
	if parent == nil {
		parent = ui.currentDir
	}
	parent.RemoveFile(selectedItem)

	x, y := ui.table.GetOffset()
	ui.showDir()
	ui.table.Select(min(row, ui.table.GetRowCount()-1), 0)
	ui.table.SetOffset(min(x, ui.table.GetRowCount()-1), y)
}

func (ui *UI) showTrashErr(output []byte, err error) {
	errText := err.Error()
	outputText := strings.TrimSpace(string(output))
	if outputText != "" {
		errText = fmt.Sprintf("%s\n\n%s", errText, outputText)
	}
	ui.showErr("Trash command failed", errors.New(errText))
}
