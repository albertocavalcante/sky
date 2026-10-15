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

	// gone remembers test files that disappeared, so that an atomic save
	// (remove or rename, then create) re-adds the file instead of losing it.
	gone map[string]bool

	// rootDir is the root directory for resolving relative paths.
	rootDir string

	// Events channel receives file change notifications.
	Events chan WatchEvent

	// Errors channel receives watcher errors.
	Errors chan error

	// done signals the watcher to stop.
	done chan struct{}

	closeOnce sync.Once
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
		gone:         make(map[string]bool),
		rootDir:      rootDir,
		Events:       make(chan WatchEvent, 100),
		Errors:       make(chan error, 10),
		done:         make(chan struct{}),
	}

	go w.run()

	return w, nil
}

// sendErr delivers an error without ever blocking the caller.
func (w *Watcher) sendErr(err error) {
	select {
	case w.Errors <- err:
	default:
	}
}

// watchPath watches a file and its parent directory (so atomic saves that
// replace the file via rename are still observed).
func (w *Watcher) watchPath(p string) error {
	// The directory goes first: it is what reports the creation of a file
	// that does not exist yet.
	dirErr := w.fsWatcher.Add(filepath.Dir(p))
	if err := w.fsWatcher.Add(p); err != nil {
		if os.IsNotExist(err) && dirErr == nil {
			return nil
		}
		return err
	}
	return nil
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
	if err := w.watchPath(absPath); err != nil {
		return fmt.Errorf("watching %s: %w", absPath, err)
	}
	w.testFiles[absPath] = true
	delete(w.gone, absPath)

	if err := w.trackTest(absPath); err != nil {
		// Report but don't fail - the file might have syntax errors
		w.sendErr(fmt.Errorf("extracting loads from %s: %w", absPath, err))
	}
	return nil
}

// trackTest parses testFile and records its direct and transitive deps.
// Caller must hold w.mu.
func (w *Watcher) trackTest(testFile string) error {
	// conftest.star files are loaded implicitly by every test below them, so
	// edits (or creation) must rerun the test even without a load().
	for _, c := range w.conftestCandidates(testFile) {
		w.trackDep(c, testFile, map[string]bool{testFile: true})
	}

	deps, err := w.extractLoads(testFile)
	if err != nil {
		return err
	}
	w.loads[testFile] = deps

	visited := map[string]bool{testFile: true}
	for _, dep := range deps {
		depPath := w.resolveLoadPath(testFile, dep)
		if depPath == "" {
			continue
		}
		w.trackDep(depPath, testFile, visited)
	}
	return nil
}

// conftestCandidates lists the conftest.star paths (existing or not) that the
// runner may load for testFile: its directory and each ancestor up to rootDir.
// A test outside rootDir only considers its own directory.
func (w *Watcher) conftestCandidates(testFile string) []string {
	dir := filepath.Dir(testFile)
	root, err := filepath.Abs(w.rootDir)
	if err != nil {
		root = dir
	}
	if rel, err := filepath.Rel(root, dir); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		root = dir
	}

	var out []string
	for {
		out = append(out, filepath.Join(dir, "conftest.star"))
		parent := filepath.Dir(dir)
		if dir == root || parent == dir {
			return out
		}
		dir = parent
	}
}

// trackDep records depPath as a dependency of testFile and recurses into its
// loads. visited (keyed by absolute path) guards against load cycles.
func (w *Watcher) trackDep(depPath, testFile string, visited map[string]bool) {
	if w.dependencies[depPath] == nil {
		w.dependencies[depPath] = make(map[string]bool)
	}
	w.dependencies[depPath][testFile] = true
	_ = w.watchPath(depPath)

	if visited[depPath] {
		return
	}
	visited[depPath] = true

	deps, err := w.extractLoads(depPath)
	if err != nil {
		return
	}
	for _, dep := range deps {
		p := w.resolveLoadPath(depPath, dep)
		if p == "" {
			continue
		}
		w.trackDep(p, testFile, visited)
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

	// The file need not exist yet: it is still tracked so that creating it
	// later triggers a rerun.
	absPath, err := filepath.Abs(resolved)
	if err != nil {
		return ""
	}
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
	var err error
	w.closeOnce.Do(func() {
		close(w.done)
		err = w.fsWatcher.Close()
	})
	return err
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

			w.process(event)

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			w.sendErr(err)
		}
	}
}

// process applies one filesystem event to the watched set.
func (w *Watcher) process(event fsnotify.Event) {
	if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
		if _, err := os.Stat(event.Name); err == nil {
			// Atomic saves replace the inode; re-arm the watch.
			_ = w.fsWatcher.Add(event.Name)
		} else if abs, err := filepath.Abs(event.Name); err == nil {
			// Really gone. A removed dependency stays tracked (so that
			// recreating it is noticed) and reruns its dependents; a removed
			// test file leaves the watched set.
			w.forgetTest(abs)
			w.handleEvent(event)
			return
		}
	}
	if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
		return
	}
	if event.Op&fsnotify.Create != 0 {
		if abs, err := filepath.Abs(event.Name); err == nil && w.wasGone(abs) {
			_ = w.Add(abs)
		}
	}
	w.handleEvent(event)
}

// forgetTest drops a deleted test file from the watched set.
func (w *Watcher) forgetTest(abs string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.testFiles[abs] {
		return
	}
	delete(w.testFiles, abs)
	delete(w.loads, abs)
	for _, set := range w.dependencies {
		delete(set, abs)
	}
	w.gone[abs] = true
}

func (w *Watcher) wasGone(abs string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.gone[abs]
}

// handleEvent processes a file change event.
func (w *Watcher) handleEvent(event fsnotify.Event) {
	absPath, err := filepath.Abs(event.Name)
	if err != nil {
		w.sendErr(fmt.Errorf("resolve absolute path for event %s: %w", event.Name, err))
		return
	}

	// Collect under the lock, but send after releasing it: a consumer that
	// calls RefreshDependencies (write lock) must never wait on a producer
	// that is itself blocked sending to the consumer.
	affected := w.collectAffected(absPath)
	if len(affected) > 0 {
		select {
		case w.Events <- WatchEvent{File: absPath, Op: event.Op, AffectedTests: affected}:
		case <-w.done:
		}
	}
}

// collectAffected returns the test files affected by a change to absPath.
func (w *Watcher) collectAffected(absPath string) []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

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

	return affected
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

	absPath, err := filepath.Abs(file)
	if err != nil {
		w.sendErr(fmt.Errorf("resolve absolute path for %s: %w", file, err))
		return nil
	}

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

// RefreshDependencies re-parses a file and updates the dependency graph.
// The file may be a watched test file or a helper loaded by one; in the
// latter case every test depending on it is re-tracked.
func (w *Watcher) RefreshDependencies(file string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	absPath, err := filepath.Abs(file)
	if err != nil {
		return err
	}

	tests := map[string]bool{}
	if w.testFiles[absPath] {
		tests[absPath] = true
	}
	for t := range w.dependencies[absPath] {
		tests[t] = true
	}

	var firstErr error
	for t := range tests {
		for _, set := range w.dependencies {
			delete(set, t)
		}
		delete(w.loads, t)
		if err := w.trackTest(t); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
