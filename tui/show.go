package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	log "github.com/sirupsen/logrus"

	"github.com/mevanlc/gdu/v5/build"
	"github.com/mevanlc/gdu/v5/pkg/fs"
)

// currentDirLabelText builds the breadcrumb label shown above the table,
// annotated when a mid-scan preview is being displayed.
func (ui *UI) currentDirLabelText() string {
	label := "[::b] --- " +
		tview.Escape(
			strings.TrimPrefix(ui.currentDirPath, build.RootPathPrefix),
		) +
		" ---"
	if ui.previewing {
		label += "  [::b][yellow]scanning… (preview, Tab to resume)[-]"
	}
	return label
}

//nolint:funlen,gocyclo // Directory rendering combines layout, totals, filtering, and row styling.
func (ui *UI) showDir() {
	keepCollectorFocus := ui.collectorFocused
	var (
		totalUsage int64
		totalSize  int64
		maxUsage   int64
		maxSize    int64
		itemCount  int64
	)

	ui.currentDirPath = ui.currentDir.GetPath()
	if ui.collectorEnabled {
		ui.markedRows = make(map[int]struct{})
	}

	if ui.changeCwdFn != nil {
		err := ui.changeCwdFn(ui.currentDirPath)
		if err != nil {
			log.Printf("error setting cwd: %s", err.Error())
		}
		log.Printf("changing cwd to %s", ui.currentDirPath)
	}

	ui.currentDirLabel.SetText(ui.currentDirLabelText()).SetDynamicColors(true)

	ui.table.Clear()

	rowIndex := 0
	if ui.currentDirPath != ui.topDirPath {
		prefix := "                         "
		if ui.markedItemCount() > 0 {
			prefix += "  "
		}

		cell := tview.NewTableCell(prefix + "[::b]/..")

		// Use the collapsed parent logic to handle navigation back through collapsed paths
		var collapsedParent fs.Item
		if ui.collapsePath {
			collapsedParent = findCollapsedParent(ui.currentDir)
		} else {
			collapsedParent = ui.currentDir.GetParent()
		}
		cell.SetReference(collapsedParent)
		cell.SetStyle(tcell.Style{}.Foreground(tcell.ColorDefault))
		ui.table.SetCell(0, 0, cell)
		rowIndex++
	}

	sortBy, sortOrder := ui.getSortParams()

	unlock := ui.currentDir.RLock()
	defer unlock()

	i := rowIndex
	maxUsage = 0
	maxSize = 0
	for item := range ui.currentDir.GetFiles(sortBy, sortOrder) {
		if _, ignored := ui.ignoredRows[i]; ignored {
			i++
			continue
		}

		if ui.ShowRelativeSize {
			if item.GetUsage() > maxUsage {
				maxUsage = item.GetUsage()
			}
			if item.GetSize() > maxSize {
				maxSize = item.GetSize()
			}
		} else {
			maxSize += item.GetSize()
			maxUsage += item.GetUsage()
		}
		i++
	}

	for item := range ui.currentDir.GetFiles(sortBy, sortOrder) {
		if ui.filterValue != "" && !strings.Contains(
			strings.ToLower(item.GetName()),
			strings.ToLower(ui.filterValue),
		) {
			continue
		}

		if !ui.matchesTypeFilter(item.GetName(), item.IsDir()) {
			continue
		}

		_, ignored := ui.ignoredRows[rowIndex]

		if !ignored {
			totalUsage += item.GetUsage()
			totalSize += item.GetSize()
			itemCount += item.GetItemCount()
		}

		var cell *tview.TableCell
		reference := item
		var collapsedPath *CollapsedPath

		// Check if this directory can be collapsed
		if item.IsDir() && ui.collapsePath {
			collapsedPath = findCollapsiblePath(item)
		}
		if collapsedPath != nil {
			// Reference should point to the deepest directory for navigation.
			reference = collapsedPath.DeepestDir
		}

		marked := ui.itemMarkedAtRow(rowIndex, reference)
		if collapsedPath != nil {
			cell = tview.NewTableCell(ui.formatCollapsedRow(collapsedPath, maxUsage, maxSize, marked, ignored))
		} else {
			cell = tview.NewTableCell(ui.formatFileRow(item, maxUsage, maxSize, marked, ignored))
		}

		cell.SetReference(reference)
		if ui.collectorEnabled && marked {
			ui.collectorItems[collectorKey(reference.GetPath())] = reference
			ui.markedRows[rowIndex] = struct{}{}
		}

		switch {
		case ignored:
			cell.SetStyle(tcell.Style{}.Foreground(tview.Styles.SecondaryTextColor))
		case marked:
			cell.SetStyle(tcell.Style{}.Foreground(ui.markedTextColor))
			cell.SetBackgroundColor(ui.markedBackgroundColor)
		default:
			cell.SetStyle(tcell.Style{}.Foreground(tcell.ColorDefault))
		}

		ui.table.SetCell(rowIndex, 0, cell)
		rowIndex++
	}

	var footerNumberColor, footerTextColor string
	if ui.UseColors {
		footerNumberColor = fmt.Sprintf(
			"[%s:%s:b]",
			ui.footerNumberColor,
			ui.footerBackgroundColor,
		)
		footerTextColor = fmt.Sprintf(
			"[%s:%s:-]",
			ui.footerTextColor,
			ui.footerBackgroundColor,
		)
	} else {
		footerNumberColor = "[black:white:b]"
		footerTextColor = blackOnWhite
	}

	selected := ""
	if ui.markedItemCount() > 0 {
		label := " Selected items: "
		if ui.collectorEnabled {
			label = " Collected items: "
		}
		selected = label + footerNumberColor +
			strconv.Itoa(ui.markedItemCount()) + footerTextColor
	}

	timeFilterText := ui.formatTimeFilterInfo()

	typeFilterText := ui.formatTypeFilterInfo(footerNumberColor, footerTextColor)

	ui.footerLabel.SetText(
		selected + footerTextColor +
			" Total disk usage: " +
			footerNumberColor +
			ui.formatSize(totalUsage, true, false) +
			" Apparent size: " +
			footerNumberColor +
			ui.formatSize(totalSize, true, false) +
			" Items: " + footerNumberColor + fmt.Sprintf("%d", itemCount) +
			footerTextColor +
			" Sorting by: " + ui.sortBy + " " + ui.sortOrder +
			typeFilterText +
			timeFilterText)

	ui.table.Select(0, 0)
	ui.table.ScrollToBeginning()
	ui.showCollector()

	if !ui.filtering && !ui.typeFiltering {
		if keepCollectorFocus && ui.collectorVisible() {
			ui.focusCollector()
		} else {
			ui.focusTable()
		}
	}
}

func (ui *UI) showDevices() {
	var totalUsage int64

	ui.table.Clear()
	ui.table.SetCell(0, 0, tview.NewTableCell("Device name").SetSelectable(false))
	ui.table.SetCell(0, 1, tview.NewTableCell("Size").SetSelectable(false))
	ui.table.SetCell(0, 2, tview.NewTableCell("Used").SetSelectable(false))
	ui.table.SetCell(0, 3, tview.NewTableCell("Used part").SetSelectable(false))
	ui.table.SetCell(0, 4, tview.NewTableCell("Free").SetSelectable(false))
	ui.table.SetCell(0, 5, tview.NewTableCell("Mount point").SetSelectable(false))

	var textColor, sizeColor string
	if ui.UseColors {
		textColor = "[#3498db:-:b]"
		sizeColor = "[#edb20a:-:b]"
	} else {
		textColor = "[white:-:b]"
		sizeColor = "[white:-:b]"
	}

	ui.sortDevices()

	for i, device := range ui.devices {
		totalUsage += device.GetUsage()
		ui.table.SetCell(i+1, 0, tview.NewTableCell(textColor+device.Name).SetReference(ui.devices[i]))
		ui.table.SetCell(i+1, 1, tview.NewTableCell(ui.formatSize(device.Size, false, true)))
		ui.table.SetCell(i+1, 2, tview.NewTableCell(sizeColor+ui.formatSize(device.Size-device.Free, false, true)))
		ui.table.SetCell(i+1, 3, tview.NewTableCell(getDeviceUsagePart(device, ui.useOldSizeBar)))
		ui.table.SetCell(i+1, 4, tview.NewTableCell(ui.formatSize(device.Free, false, true)))
		ui.table.SetCell(i+1, 5, tview.NewTableCell(textColor+device.MountPoint).SetReference(ui.devices[i]))
	}

	var footerNumberColor, footerTextColor string
	if ui.UseColors {
		footerNumberColor = fmt.Sprintf(
			"[%s:%s:b]",
			ui.footerNumberColor,
			ui.footerBackgroundColor,
		)
		footerTextColor = fmt.Sprintf(
			"[%s:%s:-]",
			ui.footerTextColor,
			ui.footerBackgroundColor,
		)
	} else {
		footerNumberColor = "[black:white:b]"
		footerTextColor = blackOnWhite
	}

	ui.footerLabel.SetText(
		" Total usage: " +
			footerNumberColor +
			ui.formatSize(totalUsage, true, false) +
			footerTextColor +
			" Sorting by: " + ui.sortBy + " " + ui.sortOrder)

	ui.table.Select(1, 0)
	ui.table.SetSelectedFunc(ui.deviceItemSelected)

	if ui.topDirPath != "" {
		for i, device := range ui.devices {
			if device.MountPoint == ui.topDirPath {
				ui.table.Select(i+1, 0)
				break
			}
		}
	}
	ui.showCollector()
}

func (ui *UI) showErr(msg string, err error) {
	text := msg
	if err != nil {
		text += ": " + err.Error()
	}

	modal := tview.NewModal().
		SetText(text).
		AddButtons([]string{"ok"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			ui.pages.RemovePage("error")
			ui.restoreOverlayFocus()
		})

	if !ui.UseColors {
		modal.SetBackgroundColor(tcell.ColorGray)
	}

	ui.pages.AddPage("error", modal, true, true)
	ui.focusOverlay(modal)
}

func (ui *UI) showErrFromGo(msg string, err error) {
	ui.app.QueueUpdateDraw(func() {
		ui.showErr(msg, err)
	})
}

func (ui *UI) formatTypeFilterInfo(numberColor, textColor string) string {
	if ui.typeFilterValue == "" {
		return ""
	}
	return " Type filter: " + numberColor + ui.typeFilterValue + textColor
}
