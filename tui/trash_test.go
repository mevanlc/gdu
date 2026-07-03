package tui

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
)

func analyzedTrashTestUI(t *testing.T) (*UI, func()) {
	t.Helper()

	fin := testdir.CreateTestDir()
	simScreen := testapp.CreateSimScreen()
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false)
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done
	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	return ui, func() {
		simScreen.Fini()
		fin()
	}
}

func runQueuedDraws(ui *UI) {
	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}
}

func TestTrashSelectedRunsCommandAndRemovesMissingPath(t *testing.T) {
	ui, cleanup := analyzedTrashTestUI(t)
	defer cleanup()

	ui.SetTrashCmd([]string{"trash", "--quiet"})
	ui.trashRunner = func(argv []string, selectedPath string) ([]byte, error) {
		assert.Equal(t, []string{"trash", "--quiet"}, argv)
		assert.Equal(t, "test_dir/nested", selectedPath)
		return nil, os.RemoveAll(selectedPath)
	}

	assert.Equal(t, 1, ui.table.GetRowCount())
	ui.table.Select(0, 0)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 't', 0))

	<-ui.done
	runQueuedDraws(ui)

	assert.NoDirExists(t, "test_dir/nested")
	assert.Equal(t, 0, ui.table.GetRowCount())
	assert.False(t, ui.pages.HasPage("error"))
}

func TestTrashSelectedKeepsExistingPath(t *testing.T) {
	ui, cleanup := analyzedTrashTestUI(t)
	defer cleanup()

	ui.SetTrashCmd([]string{"trash"})
	ui.trashRunner = func(argv []string, selectedPath string) ([]byte, error) {
		return nil, nil
	}

	ui.table.Select(0, 0)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 't', 0))

	<-ui.done
	runQueuedDraws(ui)

	assert.DirExists(t, "test_dir/nested")
	assert.Equal(t, 1, ui.table.GetRowCount())
	assert.False(t, ui.pages.HasPage("error"))
}

func TestTrashSelectedShowsCombinedOutputOnError(t *testing.T) {
	ui, cleanup := analyzedTrashTestUI(t)
	defer cleanup()

	ui.SetTrashCmd([]string{"trash"})
	ui.trashRunner = func(argv []string, selectedPath string) ([]byte, error) {
		return []byte("stdout\nstderr"), errors.New("exit status 1")
	}

	ui.table.Select(0, 0)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 't', 0))

	<-ui.done
	runQueuedDraws(ui)

	assert.DirExists(t, "test_dir/nested")
	assert.Equal(t, 1, ui.table.GetRowCount())
	assert.True(t, ui.pages.HasPage("error"))
}

func TestTrashHotkeyDisabledWithoutCommand(t *testing.T) {
	ui, cleanup := analyzedTrashTestUI(t)
	defer cleanup()

	called := false
	ui.trashRunner = func(argv []string, selectedPath string) ([]byte, error) {
		called = true
		return nil, nil
	}

	ui.table.Select(0, 0)
	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 't', 0))

	assert.False(t, called)
	assert.NotNil(t, key)
	assert.DirExists(t, "test_dir/nested")
}
