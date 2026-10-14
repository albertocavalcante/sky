package lsp

import (
	"os"
	"path/filepath"
	"testing"
)

// Linting uses the in-memory buffer, so offsets always match the document even
// when it differs from (or is missing on) disk.
func TestLintBuffer_LintsGivenContent(t *testing.T) {
	s := NewServer(nil)
	path := filepath.Join(t.TempDir(), "BUILD")
	if err := os.WriteFile(path, []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.lintBuffer(path, "x = 1\n"); err != nil {
		t.Errorf("clean buffer: %v", err)
	}
	if _, err := s.lintBuffer(path, "# edited\nx = 1\n"); err != nil {
		t.Errorf("dirty buffer: %v", err)
	}
}
