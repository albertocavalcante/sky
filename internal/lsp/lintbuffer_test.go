package lsp

import (
	"os"
	"path/filepath"
	"testing"
)

// Linting uses the in-memory buffer: findings and replacement offsets must
// describe the buffer even when the file on disk is different (here the disk
// content has no findings at all, and a different length).
func TestLintBuffer_LintsGivenContent(t *testing.T) {
	s := NewServer(nil)
	path := filepath.Join(t.TempDir(), "lib.bzl")
	if err := os.WriteFile(path, []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf := "# header comment line\nload(\"//a:b.bzl\", \"unused\")\n\ndef f(a):\n    return a\n"
	findings, err := s.lintBuffer(path, buf)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range findings {
		if f.Rule != "load" || f.Line != 2 {
			continue
		}
		found = true
		if f.Replacement == nil || f.Replacement.Start != 22 || f.Replacement.End != 49 {
			t.Errorf("replacement = %+v, want offsets 22..49 of the buffer", f.Replacement)
		}
	}
	if !found {
		t.Errorf("no unused-load finding for the buffer (disk has none): %+v", findings)
	}
}

func TestLintBuffer_MissingFileOnDisk(t *testing.T) {
	s := NewServer(nil)
	path := filepath.Join(t.TempDir(), "absent.bzl")
	findings, err := s.lintBuffer(path, "load(\"//a:b.bzl\", \"unused\")\n")
	if err != nil {
		t.Fatalf("in-memory lint should not need the file: %v", err)
	}
	if len(findings) == 0 {
		t.Error("expected findings for the buffer")
	}
}
