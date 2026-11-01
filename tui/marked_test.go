package tui

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/mevanlc/gdu/v5/internal/testapp"
	"github.com/mevanlc/gdu/v5/internal/testdir"
	"github.com/mevanlc/gdu/v5/pkg/analyze"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestItemMarked(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.done = make(chan struct{})

	ui.fileItemMarked(1)
	assert.Equal(t, ui.markedRows, map[int]struct{}{1: {}})

	ui.fileItemMarked(1)
	assert.Equal(t, ui.markedRows, map[int]struct{}{})
}

func TestMarkPreservesScrollOffset(t *testing.T) {
	for _, mode := range []string{"normal", "collector"} {
		for _, tt := range []struct {
			name           string
			row            int
			offset         int
			expectedOffset int
		}{
			{name: "near_bottom", row: 19, offset: 12, expectedOffset: 12},
			{name: "bottom_advances", row: 21, offset: 12, expectedOffset: 13},
			{name: "last_item", row: 29, offset: 20, expectedOffset: 20},
		} {
			t.Run(mode+"/"+tt.name, func(t *testing.T) {
				app, screen := testapp.CreateTestAppWithSimScreen(100, 10)
				t.Cleanup(screen.Fini)
				ui := CreateUI(app, screen, &bytes.Buffer{}, false, true, false, false, func(ui *UI) {
					if mode == "collector" {
						ui.SetCollector(CollectorSplitVertical)
					}
				})
				dir := &analyze.Dir{File: &analyze.File{Name: "items"}, BasePath: "."}
				for row := range 30 {
					dir.Files = append(dir.Files, &analyze.File{
						Name: fmt.Sprintf("file%02d", row), Parent: dir, Size: 1, Usage: 1,
					})
				}
				ui.currentDir = dir
				ui.topDirPath = dir.GetPath()
				ui.showDir()
				ui.table.SetRect(0, 0, 100, 10)

				for _, marked := range []bool{true, false} {
					ui.table.Select(tt.row, 0).SetOffset(tt.offset, 0)
					ui.table.Draw(screen)
					rowOffset, _ := ui.table.GetOffset()
					require.Equal(t, tt.offset, rowOffset)

					ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
					ui.table.Draw(screen)

					assert.Equal(t, marked, ui.itemMarkedAtRow(tt.row, ui.table.GetCell(tt.row, 0).GetReference().(*analyze.File)))
					row, _ := ui.table.GetSelection()
					assert.Equal(t, min(tt.row+1, 29), row)
					rowOffset, columnOffset := ui.table.GetOffset()
					assert.Equal(t, tt.expectedOffset, rowOffset, "marked=%t", marked)
					assert.Zero(t, columnOffset)
				}
			})
		}
	}
}
