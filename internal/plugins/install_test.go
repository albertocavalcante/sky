package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

// TestInstall_CrossFilesystem tests that plugin installation works across
// filesystem boundaries. This is a regression test for the os.Rename issue
// where moving files across different filesystems fails.
//
// Currently SKIPPED because the code uses os.Rename which doesn't work
// across filesystem boundaries. Should use copy+delete instead.
func TestInstall_CrossFilesystem(t *testing.T) {
	// This test would ideally use different filesystems, but that's hard
	// to do portably. Instead, we test the portable copy behavior.
	t.Skip("TODO: Implement cross-filesystem copy for plugin installation")
}

// TestCopyFile tests the copyFile helper function.
func TestCopyFile_Basic(t *testing.T) {
	dir := t.TempDir()

	srcFile := filepath.Join(dir, "src")
	dstFile := filepath.Join(dir, "dst")

	content := []byte("test content")
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	err := copyFile(srcFile, dstFile, 0644)
	if err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}

	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read dst file: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("copied content = %q, want %q", string(got), string(content))
	}
}

func TestCopyFile_PreservesMode(t *testing.T) {
	dir := t.TempDir()

	srcFile := filepath.Join(dir, "src")
	dstFile := filepath.Join(dir, "dst")

	content := []byte("#!/bin/sh\necho hello")
	if err := os.WriteFile(srcFile, content, 0755); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// Copy with executable mode
	err := copyFile(srcFile, dstFile, 0755)
	if err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}

	info, err := os.Stat(dstFile)
	if err != nil {
		t.Fatalf("failed to stat dst file: %v", err)
	}
	if info.Mode().Perm()&0100 == 0 {
		t.Error("copied file is not executable")
	}
}

func TestCopyFile_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()

	srcFile := filepath.Join(dir, "src")
	dstFile := filepath.Join(dir, "dst")

	// Create existing destination
	if err := os.WriteFile(dstFile, []byte("old content"), 0644); err != nil {
		t.Fatalf("failed to write dst file: %v", err)
	}

	// Create source with new content
	newContent := []byte("new content")
	if err := os.WriteFile(srcFile, newContent, 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	err := copyFile(srcFile, dstFile, 0644)
	if err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}

	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read dst file: %v", err)
	}
	if string(got) != string(newContent) {
		t.Errorf("copied content = %q, want %q", string(got), string(newContent))
	}
}

func TestCopyFile_NonexistentSource(t *testing.T) {
	dir := t.TempDir()

	srcFile := filepath.Join(dir, "nonexistent")
	dstFile := filepath.Join(dir, "dst")

	err := copyFile(srcFile, dstFile, 0644)
	if err == nil {
		t.Error("copyFile() with nonexistent source should return error")
	}
}

func TestCopyFile_InvalidDestDir(t *testing.T) {
	dir := t.TempDir()

	srcFile := filepath.Join(dir, "src")
	dstFile := filepath.Join(dir, "nonexistent", "dst")

	if err := os.WriteFile(srcFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	err := copyFile(srcFile, dstFile, 0644)
	if err == nil {
		t.Error("copyFile() with nonexistent dest dir should return error")
	}
}

// TestInstallPlugin_CreatesParentDirs tests that installation creates
// parent directories as needed.
func TestInstallPlugin_CreatesParentDirs(t *testing.T) {
	t.Skip("TODO: Test that Install creates parent directories")
}

// TestInstallPlugin_AtomicWrite tests that installation is atomic
// (uses temp file + rename pattern).
func TestInstallPlugin_AtomicWrite(t *testing.T) {
	t.Skip("TODO: Test atomic write behavior")
}
