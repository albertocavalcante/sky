package lsp

import (
	"os"
	"path/filepath"
	"testing"
)

// The linter only reads from disk, so findings must never be reported for a
// buffer that differs from the file (offsets would not line up).
func TestLintBuffer_SkipsDirtyBuffers(t *testing.T) {
	s := NewServer(nil)
	path := filepath.Join(t.TempDir(), "BUILD")
	disk := "x = 1\n"
	if err := os.WriteFile(path, []byte(disk), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := s.lintBuffer(path, disk); err != nil {
		t.Errorf("clean buffer: %v", err)
	}

	findings, err := s.lintBuffer(path, "# edited\nx = 1\n")
	if err != nil || findings != nil {
		t.Errorf("dirty buffer: findings=%v err=%v, want nil,nil", findings, err)
	}

	if _, err := s.lintBuffer(filepath.Join(t.TempDir(), "missing"), "x"); err == nil {
		t.Error("missing file should report an error")
	}
}
