package index

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// addPatternWithin runs AddPattern but fails the test instead of hanging when
// indexing blocks (for example reading a FIFO).
func addPatternWithin(t *testing.T, root, pattern string) (int, []error) {
	t.Helper()
	type result struct {
		n    int
		errs []error
	}
	done := make(chan result, 1)
	go func() {
		n, errs := New(root).AddPattern(pattern)
		done <- result{n, errs}
	}()
	select {
	case r := <-done:
		return r.n, r.errs
	case <-time.After(5 * time.Second):
		t.Fatalf("AddPattern(%q) blocked: a non-regular file was opened", pattern)
		return 0, nil
	}
}

func mkfifo(t *testing.T, path string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("mkfifo is not available on Windows")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("mkfifo", path).Run(); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
}

func TestDiscoverSkipsNonRegularFiles(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "ok.bzl", "lib/ok.bzl")
	mkfifo(t, filepath.Join(root, "blocked.bzl"))
	mkfifo(t, filepath.Join(root, "lib", "pipe.bzl"))
	// A symlink to a FIFO is not a regular file either.
	if err := os.Symlink(filepath.Join(root, "blocked.bzl"), filepath.Join(root, "alias.bzl")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	for pattern, want := range map[string]string{
		"//...":         "lib/ok.bzl,ok.bzl",
		"**/*.bzl":      "lib/ok.bzl,ok.bzl",
		"*.bzl":         "ok.bzl",
		"//lib":         "lib/ok.bzl",
		"//lib/...":     "lib/ok.bzl",
		"//blocked.bzl": "",
		"//alias.bzl":   "",
	} {
		got, err := Discover(pattern, root)
		if err != nil {
			t.Fatalf("Discover(%q): %v", pattern, err)
		}
		if rel := strings.Join(relPaths(root, got), ","); rel != want {
			t.Errorf("Discover(%q) = %q, want %q", pattern, rel, want)
		}
	}

	n, errs := addPatternWithin(t, root, "//...")
	if n != 2 || len(errs) != 0 {
		t.Errorf("AddPattern(//...) = %d, %v; want 2 files and no errors", n, errs)
	}
}

// TestDiscoverReportsUnreadableSymlinkTarget: a symlink whose target cannot be
// stat'ed for a reason other than "does not exist" must still be a candidate,
// so indexing reports the error (the query warns and exits non-zero). A
// dangling link is skipped silently.
func TestDiscoverReportsUnreadableSymlinkTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 000 does not restrict access on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	base := t.TempDir()
	root := filepath.Join(base, "ws")
	// The private directory sits outside the workspace so the walk itself succeeds.
	writeTree(t, base, "ws/ok.bzl", "priv/secret.bzl")
	if err := os.Symlink(filepath.Join(base, "priv", "secret.bzl"), filepath.Join(root, "hidden.bzl")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(filepath.Join(root, "missing.bzl"), filepath.Join(root, "dangling.bzl")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	priv := filepath.Join(base, "priv")
	if err := os.Chmod(priv, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(priv, 0o755) })

	for _, pattern := range []string{"//...", "**/*.bzl", "*.bzl"} {
		n, errs := addPatternWithin(t, root, pattern)
		if n != 1 {
			t.Errorf("AddPattern(%q) indexed %d files, want 1 (ok.bzl)", pattern, n)
		}
		if len(errs) != 1 || !strings.Contains(errs[0].Error(), "hidden.bzl") || !errors.Is(errs[0], os.ErrPermission) {
			t.Errorf("AddPattern(%q) errors = %v, want one permission error for hidden.bzl", pattern, errs)
		}
	}
}
