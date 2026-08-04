package tui

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mevanlc/gdu/v5/internal/testapp"
	"github.com/mevanlc/gdu/v5/internal/testdir"
	"github.com/mevanlc/gdu/v5/pkg/fs"
)

func analyzedCollectorUI(t *testing.T, output *bytes.Buffer, split string) *UI {
	t.Helper()
	screen := testapp.CreateSimScreen()
	t.Cleanup(screen.Fini)

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, screen, output, false, true, false, false, func(ui *UI) {
		ui.SetCollector(split)
	})
	ui.done = make(chan struct{})
	require.NoError(t, ui.AnalyzePath("test_dir", nil))
	<-ui.done
	flushCollectorUpdates(ui, 0)
	return ui
}

func flushCollectorUpdates(ui *UI, from int) {
	updates := ui.app.(*testapp.MockedApp).GetUpdateDraws()
	for _, update := range updates[from:] {
		update()
	}
}

func tableRowForPath(t *testing.T, table *tview.Table, path string) int {
	t.Helper()
	for row := 0; row < table.GetRowCount(); row++ {
		cell := table.GetCell(row, 0)
		if cell == nil {
			continue
		}
		item, ok := cell.GetReference().(fs.Item)
		if ok && item.GetPath() == path {
			return row
		}
	}
	t.Fatalf("path %q not found in table", path)
	return -1
}

func collectorRowForPath(t *testing.T, ui *UI, path string) int {
	t.Helper()
	key := collectorKey(path)
	for row := 0; row < ui.collectorTable.GetRowCount(); row++ {
		cell := ui.collectorTable.GetCell(row, 0)
		if cell != nil && cell.GetReference() == key {
			return row
		}
	}
	t.Fatalf("path %q not found in collector", path)
	return -1
}

func TestCollectorRestoresMarksAcrossNavigation(t *testing.T) {
	cleanup := testdir.CreateTestDir()
	defer cleanup()

	ui := analyzedCollectorUI(t, &bytes.Buffer{}, CollectorSplitVertical)
	require.NotNil(t, ui.collectorTable)

	nestedRow := tableRowForPath(t, ui.table, "test_dir/nested")
	ui.fileItemMarked(nestedRow)
	assert.Len(t, ui.collectorItems, 1)

	ui.table.Select(nestedRow, 0)
	ui.handleRight()
	fileRow := tableRowForPath(t, ui.table, "test_dir/nested/file2")
	ui.fileItemMarked(fileRow)
	assert.Len(t, ui.collectorItems, 2)

	ui.handleLeft()
	nestedRow = tableRowForPath(t, ui.table, "test_dir/nested")
	assert.Contains(t, ui.markedRows, nestedRow)

	collectorRow := collectorRowForPath(t, ui, "test_dir/nested")
	collectedNested := ui.collectorItems[collectorKey("test_dir/nested")]
	require.True(t, ui.collectedItemAppliesToCurrentDir(collectedNested), "parent=%q current=%q", collectedNested.GetParent().GetPath(), ui.currentDir.GetPath())
	_, background, _ := ui.collectorTable.GetCell(collectorRow, 0).Style.Decompose()
	assert.Equal(t, tcell.ColorGray, background)

	ui.table.Select(nestedRow, 0)
	ui.handleRight()
	fileRow = tableRowForPath(t, ui.table, "test_dir/nested/file2")
	assert.Contains(t, ui.markedRows, fileRow)

	ui.setSorting(nameSortKey)
	fileRow = tableRowForPath(t, ui.table, "test_dir/nested/file2")
	assert.Contains(t, ui.markedRows, fileRow)
}

func TestCollectorFocusCanRemoveOneOrAllItems(t *testing.T) {
	cleanup := testdir.CreateTestDir()
	defer cleanup()

	ui := analyzedCollectorUI(t, &bytes.Buffer{}, CollectorSplitHorizontal)
	ui.fileItemMarked(tableRowForPath(t, ui.table, "test_dir/nested"))
	ui.table.Select(tableRowForPath(t, ui.table, "test_dir/nested"), 0)
	ui.handleRight()
	ui.fileItemMarked(tableRowForPath(t, ui.table, "test_dir/nested/file2"))

	assert.Nil(t, ui.keyPressed(tcell.NewEventKey(tcell.KeyTab, 0, 0)))
	assert.True(t, ui.collectorFocused)
	ui.collectorTable.Select(0, 0)
	assert.Nil(t, ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'd', 0)))
	assert.Len(t, ui.collectorItems, 1)
	assert.True(t, ui.collectorFocused)

	assert.Nil(t, ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'D', 0)))
	assert.Empty(t, ui.collectorItems)
	assert.Empty(t, ui.markedRows)

	assert.Nil(t, ui.keyPressed(tcell.NewEventKey(tcell.KeyTab, 0, 0)))
	assert.False(t, ui.collectorFocused)
}

func TestCollectorFocusCyclesThroughActiveFilters(t *testing.T) {
	cleanup := testdir.CreateTestDir()
	defer cleanup()

	ui := analyzedCollectorUI(t, &bytes.Buffer{}, CollectorSplitVertical)
	ui.showFilterInput()
	nameHandler := ui.filteringInput.InputHandler()
	nameHandler(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(tview.Primitive) {})
	ui.showTypeFilterInput()
	typeHandler := ui.typeFilteringInput.InputHandler()
	typeHandler(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(tview.Primitive) {})
	require.Equal(t, directoryFocus, ui.currentContentFocus())

	assert.Nil(t, ui.keyPressed(tcell.NewEventKey(tcell.KeyTab, 0, 0)))
	assert.Equal(t, nameFilterFocus, ui.currentContentFocus())
	nameHandler(tcell.NewEventKey(tcell.KeyTab, 0, 0), func(tview.Primitive) {})
	assert.Equal(t, typeFilterFocus, ui.currentContentFocus())
	typeHandler(tcell.NewEventKey(tcell.KeyTab, 0, 0), func(tview.Primitive) {})
	assert.Equal(t, collectorFocus, ui.currentContentFocus())
	assert.Nil(t, ui.keyPressed(tcell.NewEventKey(tcell.KeyTab, 0, 0)))
	assert.Equal(t, directoryFocus, ui.currentContentFocus())

	assert.Nil(t, ui.keyPressed(tcell.NewEventKey(tcell.KeyBacktab, 0, 0)))
	assert.Equal(t, collectorFocus, ui.currentContentFocus())
	assert.Nil(t, ui.keyPressed(tcell.NewEventKey(tcell.KeyBacktab, 0, 0)))
	assert.Equal(t, typeFilterFocus, ui.currentContentFocus())
	typeHandler(tcell.NewEventKey(tcell.KeyBacktab, 0, 0), func(tview.Primitive) {})
	assert.Equal(t, nameFilterFocus, ui.currentContentFocus())
	nameHandler(tcell.NewEventKey(tcell.KeyBacktab, 0, 0), func(tview.Primitive) {})
	assert.Equal(t, directoryFocus, ui.currentContentFocus())

	ui.focusCollector()
	assert.Nil(t, ui.keyPressed(tcell.NewEventKey(tcell.KeyEscape, 0, 0)))
	assert.Equal(t, directoryFocus, ui.currentContentFocus())
}

func TestCollectorFocusedKeyAllowlist(t *testing.T) {
	cleanup := testdir.CreateTestDir()
	defer cleanup()

	ui := analyzedCollectorUI(t, &bytes.Buffer{}, CollectorSplitVertical)
	ui.focusCollector()

	allowedNavigation := []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyUp, 0, 0),
		tcell.NewEventKey(tcell.KeyDown, 0, 0),
		tcell.NewEventKey(tcell.KeyHome, 0, 0),
		tcell.NewEventKey(tcell.KeyPgDn, 0, 0),
		tcell.NewEventKey(tcell.KeyCtrlF, 0, 0),
		tcell.NewEventKey(tcell.KeyRune, 'g', 0),
		tcell.NewEventKey(tcell.KeyRune, 'G', 0),
		tcell.NewEventKey(tcell.KeyRune, 'j', 0),
		tcell.NewEventKey(tcell.KeyRune, 'k', 0),
	}
	for _, key := range allowedNavigation {
		assert.Same(t, key, ui.handleCollectorActions(key))
	}
	ctrlC := tcell.NewEventKey(tcell.KeyCtrlC, 0, 0)
	assert.Same(t, ctrlC, ui.handleCollectorActions(ctrlC))

	disabled := []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyEnter, 0, 0),
		tcell.NewEventKey(tcell.KeyRune, 'e', 0),
		tcell.NewEventKey(tcell.KeyRune, 't', 0),
		tcell.NewEventKey(tcell.KeyRune, 'v', 0),
		tcell.NewEventKey(tcell.KeyRune, 'o', 0),
		tcell.NewEventKey(tcell.KeyRune, 'i', 0),
		tcell.NewEventKey(tcell.KeyRune, 'I', 0),
		tcell.NewEventKey(tcell.KeyRune, '/', 0),
		tcell.NewEventKey(tcell.KeyRune, 'T', 0),
		tcell.NewEventKey(tcell.KeyRune, 'r', 0),
		tcell.NewEventKey(tcell.KeyRune, 'E', 0),
		tcell.NewEventKey(tcell.KeyRune, 'b', 0),
		tcell.NewEventKey(tcell.KeyRune, 'a', 0),
		tcell.NewEventKey(tcell.KeyRune, 'B', 0),
		tcell.NewEventKey(tcell.KeyRune, 'c', 0),
		tcell.NewEventKey(tcell.KeyRune, 'm', 0),
		tcell.NewEventKey(tcell.KeyRune, 'n', 0),
		tcell.NewEventKey(tcell.KeyRune, 's', 0),
		tcell.NewEventKey(tcell.KeyRune, 'C', 0),
		tcell.NewEventKey(tcell.KeyRune, 'M', 0),
		tcell.NewEventKey(tcell.KeyRune, 'x', 0),
	}
	for _, key := range disabled {
		assert.Nilf(t, ui.handleCollectorActions(key), "key %v should be consumed", key)
	}

	assert.False(t, ui.collectorPrintOnExit)
	assert.Nil(t, ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'p', 0)))
	assert.True(t, ui.collectorPrintOnExit)
}

func TestCollectorFocusRestoredAfterHelpAndError(t *testing.T) {
	cleanup := testdir.CreateTestDir()
	defer cleanup()

	ui := analyzedCollectorUI(t, &bytes.Buffer{}, CollectorSplitVertical)
	ui.focusCollector()
	assert.Nil(t, ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, '?', 0)))
	assert.True(t, ui.pages.HasPage("help"))
	assert.Equal(t, directoryFocus, ui.currentContentFocus())
	assert.Nil(t, ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, '?', 0)))
	assert.Equal(t, collectorFocus, ui.currentContentFocus())

	ui.showErr("Something went wrong", nil)
	assert.Equal(t, directoryFocus, ui.currentContentFocus())
	name, primitive := ui.pages.GetFrontPage()
	require.Equal(t, "error", name)
	modal, ok := primitive.(*tview.Modal)
	require.True(t, ok)
	modalApp := tview.NewApplication()
	modalApp.SetFocus(modal)
	modal.InputHandler()(
		tcell.NewEventKey(tcell.KeyEnter, 0, 0),
		func(p tview.Primitive) { modalApp.SetFocus(p) },
	)
	assert.False(t, ui.pages.HasPage("error"))
	assert.Equal(t, collectorFocus, ui.currentContentFocus())
}

func TestCollectorFocusRestoredWhenQuitIsCancelled(t *testing.T) {
	cleanup := testdir.CreateTestDir()
	defer cleanup()

	ui := analyzedCollectorUI(t, &bytes.Buffer{}, CollectorSplitVertical)
	ui.scanDuration = 10 * time.Second
	ui.focusCollector()
	assert.Nil(t, ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'q', 0)))
	assert.Equal(t, directoryFocus, ui.currentContentFocus())

	name, primitive := ui.pages.GetFrontPage()
	require.Equal(t, "confirm", name)
	modal, ok := primitive.(*tview.Modal)
	require.True(t, ok)
	modalApp := tview.NewApplication()
	modalApp.SetFocus(modal)
	modal.InputHandler()(
		tcell.NewEventKey(tcell.KeyRune, 'n', 0),
		func(p tview.Primitive) { modalApp.SetFocus(p) },
	)
	assert.False(t, ui.pages.HasPage("confirm"))
	assert.Equal(t, collectorFocus, ui.currentContentFocus())
}

func TestCollectorPrintUsesExitTimeContents(t *testing.T) {
	cleanup := testdir.CreateTestDir()
	defer cleanup()

	output := &bytes.Buffer{}
	ui := analyzedCollectorUI(t, output, CollectorSplitVertical)

	// Printing may be enabled before anything is collected.
	ui.printMarked()
	ui.fileItemMarked(tableRowForPath(t, ui.table, "test_dir/nested"))
	ui.table.Select(tableRowForPath(t, ui.table, "test_dir/nested"), 0)
	ui.handleRight()
	ui.fileItemMarked(tableRowForPath(t, ui.table, "test_dir/nested/file2"))

	ui.focusCollector()
	ui.collectorTable.Select(collectorRowForPath(t, ui, "test_dir/nested"), 0)
	ui.handleCollectorActions(tcell.NewEventKey(tcell.KeyRune, 'd', 0))

	ui.doQuit(false)
	assert.Equal(t, "test_dir/nested/file2\n", output.String())
}

func TestCollectorDeleteActsAcrossDirectories(t *testing.T) {
	cleanup := testdir.CreateTestDir()
	defer cleanup()

	ui := analyzedCollectorUI(t, &bytes.Buffer{}, CollectorSplitVertical)
	ui.askBeforeDelete = false
	ui.table.Select(tableRowForPath(t, ui.table, "test_dir/nested"), 0)
	ui.handleRight()
	ui.fileItemMarked(tableRowForPath(t, ui.table, "test_dir/nested/file2"))

	ui.table.Select(tableRowForPath(t, ui.table, "test_dir/nested/subnested"), 0)
	ui.handleRight()
	ui.fileItemMarked(tableRowForPath(t, ui.table, "test_dir/nested/subnested/file"))
	ui.handleLeft()
	ui.handleLeft()

	updateCount := len(ui.app.(*testapp.MockedApp).GetUpdateDraws())
	ui.handleDelete(false)
	<-ui.done
	flushCollectorUpdates(ui, updateCount)

	assert.NoFileExists(t, "test_dir/nested/file2")
	assert.NoFileExists(t, "test_dir/nested/subnested/file")
	assert.Empty(t, ui.collectorItems)
}

func TestCollectorEmptyKeepsSurvivingCollectedItem(t *testing.T) {
	cleanup := testdir.CreateTestDir()
	defer cleanup()

	ui := analyzedCollectorUI(t, &bytes.Buffer{}, CollectorSplitVertical)
	ui.askBeforeDelete = false
	ui.fileItemMarked(tableRowForPath(t, ui.table, "test_dir/nested"))

	updateCount := len(ui.app.(*testapp.MockedApp).GetUpdateDraws())
	ui.handleDelete(true)
	<-ui.done
	flushCollectorUpdates(ui, updateCount)

	assert.DirExists(t, "test_dir/nested")
	assert.NoDirExists(t, "test_dir/nested/subnested")
	assert.Contains(t, ui.collectorItems, collectorKey("test_dir/nested"))
}

func TestCollectorDeleteMovesOutOfDeletedCurrentDirectory(t *testing.T) {
	cleanup := testdir.CreateTestDir()
	defer cleanup()

	ui := analyzedCollectorUI(t, &bytes.Buffer{}, CollectorSplitVertical)
	ui.askBeforeDelete = false
	nestedRow := tableRowForPath(t, ui.table, "test_dir/nested")
	ui.fileItemMarked(nestedRow)
	ui.table.Select(nestedRow, 0)
	ui.handleRight()

	updateCount := len(ui.app.(*testapp.MockedApp).GetUpdateDraws())
	ui.handleDelete(false)
	<-ui.done
	flushCollectorUpdates(ui, updateCount)

	assert.NoDirExists(t, "test_dir/nested")
	require.NotNil(t, ui.currentDir)
	assert.Equal(t, "test_dir", ui.currentDir.GetPath())
}

func TestCollectorTrashActsOnAllItems(t *testing.T) {
	cleanup := testdir.CreateTestDir()
	defer cleanup()

	ui := analyzedCollectorUI(t, &bytes.Buffer{}, CollectorSplitVertical)
	ui.SetTrashCmd([]string{"trash"})
	ui.trashRunner = func(_ []string, path string) ([]byte, error) {
		return nil, os.RemoveAll(path)
	}

	ui.table.Select(tableRowForPath(t, ui.table, "test_dir/nested"), 0)
	ui.handleRight()
	ui.fileItemMarked(tableRowForPath(t, ui.table, "test_dir/nested/file2"))
	ui.table.Select(tableRowForPath(t, ui.table, "test_dir/nested/subnested"), 0)
	ui.handleRight()
	ui.fileItemMarked(tableRowForPath(t, ui.table, "test_dir/nested/subnested/file"))
	ui.handleLeft()
	ui.handleLeft()

	updateCount := len(ui.app.(*testapp.MockedApp).GetUpdateDraws())
	ui.handleTrash()
	<-ui.done
	flushCollectorUpdates(ui, updateCount)

	assert.NoFileExists(t, "test_dir/nested/file2")
	assert.NoFileExists(t, "test_dir/nested/subnested/file")
	assert.Empty(t, ui.collectorItems)
}
