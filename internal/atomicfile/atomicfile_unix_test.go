//go:build unix

package atomicfile

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestWriteExactKeepsModeDespiteUmask(t *testing.T) {
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)
	p := filepath.Join(t.TempDir(), "out.json")
	if err := WriteExact(p, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0644", fi.Mode().Perm())
	}
}

func TestWriteNewAndReplace(t *testing.T) {
	old := syscall.Umask(0o022)
	defer syscall.Umask(old)
	dir := t.TempDir()
	p := filepath.Join(dir, "out.json")
	if err := Write(p, []byte("a"), 0o666); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o644 {
		t.Errorf("new mode = %v, want 0644", fi.Mode().Perm())
	}
	if err := os.Chmod(p, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Write(p, []byte("b"), 0o666); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 {
		t.Errorf("replaced mode = %v, want 0600", fi.Mode().Perm())
	}
	if b, _ := os.ReadFile(p); string(b) != "b" {
		t.Errorf("content = %q", b)
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) != 1 {
		t.Errorf("leftover files: %v", ents)
	}
}
