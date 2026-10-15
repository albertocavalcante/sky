//go:build unix

package atomicfile

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// longPathDir creates a directory under base whose path is exactly n bytes
// long, built from components short enough for NAME_MAX.
func longPathDir(t *testing.T, base string, n int) string {
	t.Helper()
	dir := base
	for len(dir) < n {
		room := n - len(dir) - 1 // the separator
		if room < 1 {
			t.Skipf("cannot build a %d-byte directory under %s", n, base)
		}
		comp := min(room, 200)
		dir = filepath.Join(dir, strings.Repeat("d", comp))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Skipf("filesystem cannot represent a %d-byte path: %v", n, err)
	}
	return dir
}

// A destination whose full path just fits the platform limit must still be
// replaceable: the temporary file lives in the same directory, so its name
// must not push the path over the limit. ".t" plus up to ten digits adds at
// most 11 bytes to a 1-byte destination name.
func TestWriteNearPathMax(t *testing.T) {
	var limit int // longest usable path, excluding the NUL
	switch runtime.GOOS {
	case "darwin":
		limit = 1023
	case "linux":
		limit = 4095
	default:
		t.Skipf("no known PATH_MAX for %s", runtime.GOOS)
	}
	// dest = dir + "/x" must be limit-11 bytes so ".t" + 10 digits still fits.
	// Resolve symlinks first (macOS /var is /private/var): the kernel counts the
	// resolved path against the limit.
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := longPathDir(t, base, limit-11-2)
	dest := filepath.Join(dir, "x")
	for _, existing := range []bool{true, false} {
		_ = os.Remove(dest)
		if existing {
			if err := os.WriteFile(dest, []byte("old"), 0o644); err != nil {
				t.Skipf("cannot create the destination: %v", err)
			}
		}
		for i := 0; i < 20; i++ { // the random suffix varies in length
			if err := Write(dest, []byte("new"), 0o644); err != nil {
				t.Fatalf("Write (existing=%v): %v", existing, err)
			}
			if err := WriteExact(dest, []byte("new"), 0o644); err != nil {
				t.Fatalf("WriteExact (existing=%v): %v", existing, err)
			}
		}
		if b, _ := os.ReadFile(dest); string(b) != "new" {
			t.Errorf("content = %q", b)
		}
	}
}
