//go:build unix

package skycov

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestWriteFileAtomicPreservesExistingMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %v, want 0600", got)
	}
	if b, _ := os.ReadFile(path); string(b) != "new" {
		t.Errorf("content = %q", b)
	}
}

func TestWriteFileAtomicNewFileHonorsUmask(t *testing.T) {
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)
	path := filepath.Join(t.TempDir(), "out")
	if err := writeFileAtomic(path, []byte("x")); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %v, want 0600 under umask 077", got)
	}
}
