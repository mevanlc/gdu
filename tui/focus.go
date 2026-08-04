package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type contentFocus uint8

const (
	directoryFocus contentFocus = iota
	nameFilterFocus
	typeFilterFocus
	collectorFocus
)

func (ui *UI) currentContentFocus() contentFocus {
	switch {
	case ui.filtering:
		return nameFilterFocus
	case ui.typeFiltering:
		return typeFilterFocus
	case ui.collectorFocused:
		return collectorFocus
	default:
		return directoryFocus
	}
}

func (ui *UI) setContentFocusState(target contentFocus) {
	ui.filtering = target == nameFilterFocus
	ui.typeFiltering = target == typeFilterFocus
	ui.collectorFocused = target == collectorFocus
}

func (ui *UI) focusContent(target contentFocus) bool {
	var primitive tview.Primitive
	switch target {
	case directoryFocus:
		primitive = ui.table
	case nameFilterFocus:
		primitive = ui.filteringInput
	case typeFilterFocus:
		primitive = ui.typeFilteringInput
	case collectorFocus:
		primitive = ui.collectorTable
	}
	if primitive == nil {
		return false
	}
	ui.setContentFocusState(target)
	ui.app.SetFocus(primitive)
	return true
}

func (ui *UI) focusTable() {
	ui.focusContent(directoryFocus)
}

func (ui *UI) focusNameFilter() {
	ui.focusContent(nameFilterFocus)
}

func (ui *UI) focusTypeFilter() {
	ui.focusContent(typeFilterFocus)
}

func (ui *UI) focusCollector() {
	ui.focusContent(collectorFocus)
}

func (ui *UI) contentFocusOrder() []contentFocus {
	order := []contentFocus{directoryFocus}
	if ui.filteringInput != nil {
		order = append(order, nameFilterFocus)
	}
	if ui.typeFilteringInput != nil {
		order = append(order, typeFilterFocus)
	}
	if ui.collectorTable != nil {
		order = append(order, collectorFocus)
	}
	return order
}

func (ui *UI) cycleContentFocus(reverse bool) bool {
	order := ui.contentFocusOrder()
	if len(order) < 2 {
		return false
	}

	current := ui.currentContentFocus()
	index := 0
	for candidateIndex, candidate := range order {
		if candidate == current {
			index = candidateIndex
			break
		}
	}
	if reverse {
		index = (index - 1 + len(order)) % len(order)
	} else {
		index = (index + 1) % len(order)
	}
	return ui.focusContent(order[index])
}

func (ui *UI) handleFocusTraversal(key *tcell.EventKey) *tcell.EventKey {
	var reverse bool
	//nolint:exhaustive // This handler only owns forward and reverse traversal.
	switch key.Key() {
	case tcell.KeyTab:
	case tcell.KeyBacktab:
		reverse = true
	default:
		return key
	}
	if ui.cycleContentFocus(reverse) {
		return nil
	}
	return key
}

func (ui *UI) focusOverlay(primitive tview.Primitive) {
	if !ui.overlayFocusSaved {
		ui.overlayReturnFocus = ui.currentContentFocus()
		ui.overlayFocusSaved = true
	}
	ui.setContentFocusState(directoryFocus)
	ui.app.SetFocus(primitive)
}

func (ui *UI) restoreOverlayFocus() {
	target := directoryFocus
	if ui.overlayFocusSaved {
		target = ui.overlayReturnFocus
	}
	ui.overlayFocusSaved = false
	if !ui.focusContent(target) {
		ui.focusTable()
	}
}
