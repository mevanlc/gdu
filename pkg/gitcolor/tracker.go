// Package gitcolor identifies files and directories represented by a Git index.
package gitcolor

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	git "github.com/go-git/go-git/v5"
)

// Tracker caches Git indexes and answers whether paths are tracked.
type Tracker struct {
	mu           sync.Mutex
	repositories map[string]*repositoryIndex
	lookups      map[string]*repositoryIndex
}

type repositoryIndex struct {
	root        string
	tracked     map[string]struct{}
	directories map[string]struct{}
}

// NewTracker creates an empty, lazily populated path tracker.
func NewTracker() *Tracker {
	return &Tracker{
		repositories: make(map[string]*repositoryIndex),
		lookups:      make(map[string]*repositoryIndex),
	}
}

// IsTracked reports whether path is present in the nearest Git worktree's
// index. A directory is tracked when it is itself an index entry (for example,
// a submodule) or contains at least one tracked entry.
func (t *Tracker) IsTracked(itemPath string, isDir bool) bool {
	absPath, err := filepath.Abs(itemPath)
	if err != nil {
		return false
	}
	absPath = filepath.Clean(absPath)

	lookupDir := absPath
	if !isDir {
		lookupDir = filepath.Dir(absPath)
	}

	repo := t.repositoryForDir(lookupDir)
	if repo == nil {
		return false
	}

	rel, err := filepath.Rel(repo.root, absPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." {
		return len(repo.tracked) > 0
	}

	if _, ok := repo.tracked[rel]; ok {
		return true
	}
	if !isDir {
		return false
	}

	_, ok := repo.directories[rel]
	return ok
}

func (t *Tracker) repositoryForDir(dir string) *repositoryIndex {
	t.mu.Lock()
	defer t.mu.Unlock()

	var visited []string
	current := filepath.Clean(dir)
	for {
		if repo, ok := t.repositories[current]; ok {
			t.cacheLookups(visited, repo)
			return repo
		}
		if repo, ok := t.lookups[current]; ok {
			t.cacheLookups(visited, repo)
			return repo
		}

		visited = append(visited, current)
		if _, err := os.Lstat(filepath.Join(current, ".git")); err == nil {
			repo := loadRepositoryIndex(current)
			if repo != nil {
				t.repositories[repo.root] = repo
			}
			t.cacheLookups(visited, repo)
			return repo
		}

		parent := filepath.Dir(current)
		if parent == current {
			t.cacheLookups(visited, nil)
			return nil
		}
		current = parent
	}
}

func (t *Tracker) cacheLookups(paths []string, repo *repositoryIndex) {
	for _, lookupPath := range paths {
		t.lookups[lookupPath] = repo
	}
}

func loadRepositoryIndex(root string) *repositoryIndex {
	repo, err := git.PlainOpen(root)
	if err != nil {
		return nil
	}

	index, err := repo.Storer.Index()
	if err != nil {
		return nil
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil
	}

	result := &repositoryIndex{
		root:        filepath.Clean(absRoot),
		tracked:     make(map[string]struct{}, len(index.Entries)),
		directories: make(map[string]struct{}),
	}
	for _, entry := range index.Entries {
		name := path.Clean(entry.Name)
		if name == "." || name == ".." || strings.HasPrefix(name, "../") || strings.HasPrefix(name, "/") {
			continue
		}
		result.tracked[name] = struct{}{}
		for parent := path.Dir(name); parent != "." && parent != "/"; parent = path.Dir(parent) {
			result.directories[parent] = struct{}{}
		}
	}

	return result
}
