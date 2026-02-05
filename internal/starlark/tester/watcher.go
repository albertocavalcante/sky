// Package tester provides file watching support for test runs.
package tester

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"go.starlark.net/syntax"
)

// Watcher watches test files and their dependencies for changes.
type Watcher struct {
	mu sync.RWMutex

	// fsWatcher is the underlying file watcher.
	fsWatcher *fsnotify.Watcher

	// testFiles is the set of test files being watched.
	testFiles map[string]bool

	// dependencies maps a file to the files that depend on it (reverse deps).
	// If "helper.star" is loaded by "test_foo.star", then dependencies["helper.star"] contains "test_foo.star"
	dependencies map[string]map[string]bool

	// loads maps a test file to the files it loads (forward deps).
	loads map[string][]string

	// rootDir is the root directory for resolving relative paths.
	rootDir string

	// Events channel receives file change notifications.
	Events chan WatchEvent

	// Errors channel receives watcher errors.
	Errors chan error

	// done signals the watcher to stop.
	done chan struct{}
}

// WatchEvent represents a file change event.
type WatchEvent struct {
	// File is the file that changed.
	File string

	// Op is the operation (write, create, remove, etc.).
	Op fsnotify.Op

	// AffectedTests lists test files affected by this change.
	// If the changed file is a test file itself, it will be in this list.
	// If the changed file is a dependency, all test files that load it will be listed.
	AffectedTests []string
}

// NewWatcher creates a new file watcher.
func NewWatcher(rootDir string) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating watcher: %w", err)
	}

	w := &Watcher{
		fsWatcher:    fsWatcher,
		testFiles:    make(map[string]bool),
		dependencies: make(map[string]map[string]bool),
		loads:        make(map[string][]string),
		rootDir:      rootDir,
		Events:       make(chan WatchEvent, 100),
		Errors:       make(chan error, 10),
		done:         make(chan struct{}),
	}

	go w.run()

	return w, nil
}

// Add adds a test file to watch, along with its dependencies.
func (w *Watcher) Add(testFile string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	absPath, err := filepath.Abs(testFile)
	if err != nil {
		return fmt.Errorf("getting absolute path: %w", err)
	}

	// Already watching this file
	if w.testFiles[absPath] {
		return nil
	}

	// Watch the test file
	if err := w.fsWatcher.Add(absPath); err != nil {
		return fmt.Errorf("watching %s: %w", absPath, err)
	}
	w.testFiles[absPath] = true

	// Parse the file to find its load() dependencies
	deps, err := w.extractLoads(absPath)
	if err != nil {
		// Log but don't fail - the file might have syntax errors
		w.Errors <- fmt.Errorf("extracting loads from %s: %w", absPath, err)
		return nil
	}

	w.loads[absPath] = deps

	// Watch each dependency and track reverse dependencies
	for _, dep := range deps {
		// Resolve relative path
		depPath := w.resolveLoadPath(absPath, dep)
		if depPath == "" {
			continue
		}

		// Add to reverse deps
		if w.dependencies[depPath] == nil {
			w.dependencies[depPath] = make(map[string]bool)
		}
		w.dependencies[depPath][absPath] = true

		// Watch the dependency file
		if err := w.fsWatcher.Add(depPath); err != nil {
			// File might not exist yet, that's ok
			continue
		}

		// Recursively track dependencies of this file too
		w.trackTransitiveDeps(depPath, absPath)
	}

	return nil
}

// trackTransitiveDeps recursively tracks dependencies.
func (w *Watcher) trackTransitiveDeps(file, testFile string) {
	deps, err := w.extractLoads(file)
	if err != nil {
		return
	}

	for _, dep := range deps {
		depPath := w.resolveLoadPath(file, dep)
		if depPath == "" {
			continue
		}

		// Add testFile as depending on this transitive dep
		if w.dependencies[depPath] == nil {
			w.dependencies[depPath] = make(map[string]bool)
		}
		w.dependencies[depPath][testFile] = true

		// Watch the file
		if err := w.fsWatcher.Add(depPath); err != nil {
			continue
		}

		// Avoid infinite loops by checking if we've processed this path for this test
		alreadyTracked := false
		for _, existing := range w.loads[depPath] {
			if existing == dep {
				alreadyTracked = true
				break
			}
		}
		if !alreadyTracked {
			w.trackTransitiveDeps(depPath, testFile)
		}
	}
}

// extractLoads parses a Starlark file and returns its load() modules.
func (w *Watcher) extractLoads(file string) ([]string, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	f, err := syntax.Parse(file, src, 0)
	if err != nil {
		return nil, err
	}

	var loads []string
	for _, stmt := range f.Stmts {
		if load, ok := stmt.(*syntax.LoadStmt); ok {
			// Get the module path (first argument to load())
			if module, ok := load.Module.Value.(string); ok {
				loads = append(loads, module)
			}
		}
	}

	return loads, nil
}

// resolveLoadPath resolves a load path relative to the loading file.
func (w *Watcher) resolveLoadPath(fromFile, loadPath string) string {
	// Skip Bazel-style absolute labels (//pkg:file or @repo//pkg:file)
	if strings.HasPrefix(loadPath, "//") || strings.HasPrefix(loadPath, "@") {
		return ""
	}

	// Resolve relative to the loading file's directory
	dir := filepath.Dir(fromFile)
	resolved := filepath.Join(dir, loadPath)

	// Check if file exists
	if _, err := os.Stat(resolved); err != nil {
		return ""
	}

	absPath, _ := filepath.Abs(resolved)
	return absPath
}

// Remove removes a test file from watching.
func (w *Watcher) Remove(testFile string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	absPath, err := filepath.Abs(testFile)
	if err != nil {
		return err
	}

	delete(w.testFiles, absPath)

	// Remove from deps tracking
	for dep := range w.dependencies {
		delete(w.dependencies[dep], absPath)
	}

	delete(w.loads, absPath)

	return w.fsWatcher.Remove(absPath)
}

// Close stops the watcher and releases resources.
func (w *Watcher) Close() error {
	close(w.done)
	return w.fsWatcher.Close()
}

// run processes filesystem events.
func (w *Watcher) run() {
	for {
		select {
		case <-w.done:
			return

		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}

			// Ignore non-modify events for now (could expand later)
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			w.handleEvent(event)

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			w.Errors <- err
		}
	}
}

// handleEvent processes a file change event.
func (w *Watcher) handleEvent(event fsnotify.Event) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	absPath, _ := filepath.Abs(event.Name)

	var affected []string

	// If it's a test file itself, it's affected
	if w.testFiles[absPath] {
		affected = append(affected, absPath)
	}

	// Find all test files that depend on this file
	if deps, ok := w.dependencies[absPath]; ok {
		for testFile := range deps {
			// Avoid duplicates
			found := false
			for _, a := range affected {
				if a == testFile {
					found = true
					break
				}
			}
			if !found {
				affected = append(affected, testFile)
			}
		}
	}

	if len(affected) > 0 {
		w.Events <- WatchEvent{
			File:          absPath,
			Op:            event.Op,
			AffectedTests: affected,
		}
	}
}

// WatchedFiles returns the list of files being watched.
func (w *Watcher) WatchedFiles() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var files []string
	for f := range w.testFiles {
		files = append(files, f)
	}
	return files
}

// AffectedTestFiles returns all test files affected by changes to the given file.
func (w *Watcher) AffectedTestFiles(file string) []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	absPath, _ := filepath.Abs(file)

	var affected []string

	// If it's a test file itself
	if w.testFiles[absPath] {
		affected = append(affected, absPath)
	}

	// All test files that depend on this file
	if deps, ok := w.dependencies[absPath]; ok {
		for testFile := range deps {
			found := false
			for _, a := range affected {
				if a == testFile {
					found = true
					break
				}
			}
			if !found {
				affected = append(affected, testFile)
			}
		}
	}

	return affected
}

// RefreshDependencies re-parses a file and updates its dependencies.
// Call this after a file is modified to update the dependency graph.
func (w *Watcher) RefreshDependencies(file string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	absPath, err := filepath.Abs(file)
	if err != nil {
		return err
	}

	// Only refresh if it's a watched test file
	if !w.testFiles[absPath] {
		return nil
	}

	// Remove old dependency tracking
	oldLoads := w.loads[absPath]
	for _, dep := range oldLoads {
		depPath := w.resolveLoadPath(absPath, dep)
		if depPath != "" && w.dependencies[depPath] != nil {
			delete(w.dependencies[depPath], absPath)
		}
	}

	// Re-extract and track new dependencies
	deps, err := w.extractLoads(absPath)
	if err != nil {
		return err
	}

	w.loads[absPath] = deps

	for _, dep := range deps {
		depPath := w.resolveLoadPath(absPath, dep)
		if depPath == "" {
			continue
		}

		if w.dependencies[depPath] == nil {
			w.dependencies[depPath] = make(map[string]bool)
		}
		w.dependencies[depPath][absPath] = true

		_ = w.fsWatcher.Add(depPath)
	}

	return nil
}
