package skyfmt

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSymlinkedDirectoryRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	if err := os.MkdirAll(filepath.Join(real, ".hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "a.star"), []byte("x=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, ".hidden", "h.star"), []byte("x=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	for _, arg := range []string{link, link + string(os.PathSeparator)} {
		got, err := expandPath(arg)
		if err != nil || len(got) != 1 || filepath.Base(got[0]) != "a.star" {
			t.Errorf("expandPath(%q) = %v, %v; want only a.star", arg, got, err)
		}
		var stdout, stderr bytes.Buffer
		if code := RunWithIO(context.Background(), []string{"-check", arg}, nil, &stdout, &stderr); code != exitNeedsFormat {
			t.Errorf("RunWithIO(-check %q) = %d, want %d; stderr=%s", arg, code, exitNeedsFormat, stderr.String())
		}
	}
}
