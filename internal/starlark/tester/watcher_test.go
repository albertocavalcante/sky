package tester

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcher_Basic(t *testing.T) {
	dir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(dir, "test_basic.star")
	content := `def test_basic():
    assert.eq(1, 1)
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create watcher
	watcher, err := NewWatcher(dir)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Close()

	// Add test file
	if err := watcher.Add(testFile); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Verify file is being watched
	watched := watcher.WatchedFiles()
	if len(watched) != 1 {
		t.Errorf("expected 1 watched file, got %d", len(watched))
	}
}

func TestWatcher_WithLoads(t *testing.T) {
	dir := t.TempDir()

	// Create a helper file
	helperFile := filepath.Join(dir, "helpers.star")
	helperContent := `def helper():
    return 42
`
	if err := os.WriteFile(helperFile, []byte(helperContent), 0644); err != nil {
		t.Fatalf("failed to write helper file: %v", err)
	}

	// Create a test file that loads the helper
	testFile := filepath.Join(dir, "test_with_load.star")
	testContent := `load("helpers.star", "helper")

def test_with_helper():
    assert.eq(helper(), 42)
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create watcher
	watcher, err := NewWatcher(dir)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Close()

	// Add test file
	if err := watcher.Add(testFile); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Helper file change should affect the test file
	affected := watcher.AffectedTestFiles(helperFile)
	if len(affected) != 1 {
		t.Errorf("expected 1 affected test, got %d", len(affected))
	}

	absTestFile, _ := filepath.Abs(testFile)
	if len(affected) > 0 && affected[0] != absTestFile {
		t.Errorf("expected affected test %q, got %q", absTestFile, affected[0])
	}
}

func TestWatcher_TransitiveDeps(t *testing.T) {
	dir := t.TempDir()

	// Create base helper
	baseFile := filepath.Join(dir, "base.star")
	baseContent := `BASE_VALUE = 10
`
	if err := os.WriteFile(baseFile, []byte(baseContent), 0644); err != nil {
		t.Fatalf("failed to write base file: %v", err)
	}

	// Create helper that loads base
	helperFile := filepath.Join(dir, "helper.star")
	helperContent := `load("base.star", "BASE_VALUE")

def get_value():
    return BASE_VALUE * 2
`
	if err := os.WriteFile(helperFile, []byte(helperContent), 0644); err != nil {
		t.Fatalf("failed to write helper file: %v", err)
	}

	// Create test file that loads helper
	testFile := filepath.Join(dir, "test_transitive.star")
	testContent := `load("helper.star", "get_value")

def test_value():
    assert.eq(get_value(), 20)
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create watcher
	watcher, err := NewWatcher(dir)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Close()

	// Add test file
	if err := watcher.Add(testFile); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Base file change should affect the test file (transitive dependency)
	affected := watcher.AffectedTestFiles(baseFile)
	if len(affected) != 1 {
		t.Errorf("expected 1 affected test for base file, got %d", len(affected))
	}

	// Helper file change should also affect the test file
	affected = watcher.AffectedTestFiles(helperFile)
	if len(affected) != 1 {
		t.Errorf("expected 1 affected test for helper file, got %d", len(affected))
	}
}

func TestWatcher_FileChange(t *testing.T) {
	dir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(dir, "test_change.star")
	content := `def test_change():
    assert.eq(1, 1)
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create watcher
	watcher, err := NewWatcher(dir)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Close()

	// Add test file
	if err := watcher.Add(testFile); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Modify the file
	newContent := `def test_change():
    assert.eq(2, 2)
`
	// Small delay to ensure watcher is set up
	time.Sleep(50 * time.Millisecond)

	if err := os.WriteFile(testFile, []byte(newContent), 0644); err != nil {
		t.Fatalf("failed to modify test file: %v", err)
	}

	// Wait for event with timeout
	select {
	case event := <-watcher.Events:
		absTestFile, _ := filepath.Abs(testFile)
		if event.File != absTestFile {
			t.Errorf("expected file %q, got %q", absTestFile, event.File)
		}
		if len(event.AffectedTests) != 1 {
			t.Errorf("expected 1 affected test, got %d", len(event.AffectedTests))
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for file change event")
	}
}

func TestWatcher_Remove(t *testing.T) {
	dir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(dir, "test_remove.star")
	content := `def test_remove():
    assert.eq(1, 1)
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create watcher
	watcher, err := NewWatcher(dir)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Close()

	// Add test file
	if err := watcher.Add(testFile); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Remove test file from watching
	if err := watcher.Remove(testFile); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify file is no longer being watched
	watched := watcher.WatchedFiles()
	if len(watched) != 0 {
		t.Errorf("expected 0 watched files after remove, got %d", len(watched))
	}
}

func TestWatcher_IgnoreBazelLabels(t *testing.T) {
	dir := t.TempDir()

	// Create a test file that uses Bazel-style labels (should be ignored)
	testFile := filepath.Join(dir, "test_bazel.star")
	testContent := `load("//pkg:helper.bzl", "helper")
load("@external//lib:utils.bzl", "util")

def test_bazel():
    pass
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create watcher
	watcher, err := NewWatcher(dir)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Close()

	// Add test file - should not fail even though load targets don't exist
	if err := watcher.Add(testFile); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Bazel-style loads should not be tracked as dependencies
	watched := watcher.WatchedFiles()
	if len(watched) != 1 {
		t.Errorf("expected only 1 watched file (the test file), got %d", len(watched))
	}
}

func TestWatcher_RefreshDependencies(t *testing.T) {
	dir := t.TempDir()

	// Create helper file
	helperFile := filepath.Join(dir, "helper.star")
	if err := os.WriteFile(helperFile, []byte(`HELPER_VALUE = 1`), 0644); err != nil {
		t.Fatalf("failed to write helper file: %v", err)
	}

	// Create another helper
	helper2File := filepath.Join(dir, "helper2.star")
	if err := os.WriteFile(helper2File, []byte(`HELPER2_VALUE = 2`), 0644); err != nil {
		t.Fatalf("failed to write helper2 file: %v", err)
	}

	// Create test file that loads only helper.star
	testFile := filepath.Join(dir, "test_refresh.star")
	content1 := `load("helper.star", "HELPER_VALUE")

def test_value():
    assert.eq(HELPER_VALUE, 1)
`
	if err := os.WriteFile(testFile, []byte(content1), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create watcher
	watcher, err := NewWatcher(dir)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Close()

	// Add test file
	if err := watcher.Add(testFile); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Initially, only helper.star should affect the test
	affected := watcher.AffectedTestFiles(helperFile)
	if len(affected) != 1 {
		t.Errorf("expected 1 affected test for helper.star, got %d", len(affected))
	}

	affected = watcher.AffectedTestFiles(helper2File)
	if len(affected) != 0 {
		t.Errorf("expected 0 affected tests for helper2.star, got %d", len(affected))
	}

	// Update test file to load helper2.star instead
	content2 := `load("helper2.star", "HELPER2_VALUE")

def test_value():
    assert.eq(HELPER2_VALUE, 2)
`
	if err := os.WriteFile(testFile, []byte(content2), 0644); err != nil {
		t.Fatalf("failed to modify test file: %v", err)
	}

	// Refresh dependencies
	if err := watcher.RefreshDependencies(testFile); err != nil {
		t.Fatalf("RefreshDependencies failed: %v", err)
	}

	// Now helper2.star should affect the test
	affected = watcher.AffectedTestFiles(helper2File)
	if len(affected) != 1 {
		t.Errorf("expected 1 affected test for helper2.star after refresh, got %d", len(affected))
	}
}
