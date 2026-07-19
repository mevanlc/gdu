package gitcolor

import (
	"os"
	"path/filepath"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrackerFindsIndexedPaths(t *testing.T) {
	root := t.TempDir()
	repo, err := git.PlainInit(root, false)
	require.NoError(t, err)

	trackedFile := filepath.Join(root, "tracked", "file.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(trackedFile), 0o755))
	require.NoError(t, os.WriteFile(trackedFile, []byte("tracked"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("untracked"), 0o644))

	worktree, err := repo.Worktree()
	require.NoError(t, err)
	_, err = worktree.Add(filepath.ToSlash(filepath.Join("tracked", "file.txt")))
	require.NoError(t, err)

	tracker := NewTracker()
	assert.True(t, tracker.IsTracked(trackedFile, false))
	assert.True(t, tracker.IsTracked(filepath.Dir(trackedFile), true))
	assert.True(t, tracker.IsTracked(root, true))
	assert.False(t, tracker.IsTracked(filepath.Join(root, "untracked.txt"), false))
	assert.False(t, tracker.IsTracked(filepath.Join(root, ".git"), true))
}

func TestTrackerUsesNearestRepository(t *testing.T) {
	root := t.TempDir()
	outer, err := git.PlainInit(root, false)
	require.NoError(t, err)

	nested := filepath.Join(root, "nested")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	outerOnly := filepath.Join(nested, "outer-only.txt")
	require.NoError(t, os.WriteFile(outerOnly, []byte("outer"), 0o644))
	outerWorktree, err := outer.Worktree()
	require.NoError(t, err)
	_, err = outerWorktree.Add(filepath.ToSlash(filepath.Join("nested", "outer-only.txt")))
	require.NoError(t, err)

	inner, err := git.PlainInit(nested, false)
	require.NoError(t, err)
	innerFile := filepath.Join(nested, "inner.txt")
	require.NoError(t, os.WriteFile(innerFile, []byte("inner"), 0o644))
	innerWorktree, err := inner.Worktree()
	require.NoError(t, err)
	_, err = innerWorktree.Add("inner.txt")
	require.NoError(t, err)

	tracker := NewTracker()
	assert.True(t, tracker.IsTracked(innerFile, false))
	assert.False(t, tracker.IsTracked(outerOnly, false))
}

func TestTrackerReturnsFalseOutsideRepository(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "plain.txt")
	require.NoError(t, os.WriteFile(file, []byte("plain"), 0o644))

	assert.False(t, NewTracker().IsTracked(file, false))
}
