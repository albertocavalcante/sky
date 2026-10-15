package atomicfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteIgnoresPredictableTmp(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "out.json")
	victim := filepath.Join(dir, "victim")
	if err := os.WriteFile(victim, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, p+".tmp"); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := Write(p, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(victim); !strings.HasPrefix(string(b), "keep") {
		t.Errorf("victim clobbered: %q", b)
	}
}

func TestWriteMaxLengthBasename(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, strings.Repeat("a", 255))
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Skipf("filesystem rejects a 255-byte name: %v", err)
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	for range 2 { // new file, then replace
		if err := Write(p, []byte("data"), 0o644); err != nil {
			t.Fatalf("Write with 255-byte basename: %v", err)
		}
	}
	if b, _ := os.ReadFile(p); string(b) != "data" {
		t.Errorf("content = %q", b)
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 1 {
		t.Errorf("leftover files: %v", ents)
	}
}
