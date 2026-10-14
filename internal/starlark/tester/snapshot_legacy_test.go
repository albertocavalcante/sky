package tester

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.starlark.net/starlark"
)

// legacySnapshotFile is where releases before snapshot name encoding and
// bounding stored a baseline: the raw test name, separators and Windows-invalid
// characters in the snapshot name replaced by underscores. It is written out
// here, not derived from the implementation, so the on-disk contract is pinned.
func legacySnapshotFile(sm *SnapshotManager, snapName string) string {
	safe := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(snapName)
	return filepath.Join(sm.snapshotDir(), sm.testName+"__"+safe+".snap")
}

func writeBaseline(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func countFiles(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	_ = filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return nil
	})
	return n
}

// A baseline stored under a legacy name must still be compared against, never
// silently replaced by a fresh baseline at a new path (which would make a
// changed value pass without update mode).
func TestLegacyBaselineIsStillComparedAgainst(t *testing.T) {
	cases := []struct {
		name      string
		testName  string
		snapName  string
		noWindows bool // the legacy file name contains characters Windows forbids
	}{
		{"name longer than the bound", "test_a", strings.Repeat("n", 205), false},
		{"colon in test name", "test_p[a:b]", "snap", true},
		{"percent in test name", "test_p[100%]", "snap", false},
		{"star in test name", "test_p[*]", "snap", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.noWindows && runtime.GOOS == "windows" {
				t.Skip("legacy file name is not valid on Windows")
			}
			dir := t.TempDir()
			sm := NewSnapshotManager(false)
			sm.SetContext(filepath.Join(dir, "t.star"), c.testName)
			legacy := legacySnapshotFile(sm, c.snapName)
			if len(filepath.Base(legacy)) > 255 {
				t.Skip("legacy file name not representable on this filesystem")
			}
			writeBaseline(t, legacy, `"OLD"`+"\n")

			err := sm.Compare(starlark.String("REGRESSION"), c.snapName)
			if err == nil {
				t.Fatalf("changed value passed without update mode; baseline at %s was ignored", legacy)
			}
			if n := countFiles(t, sm.snapshotDir()); n != 1 {
				t.Errorf("found %d snapshot files, want only the legacy baseline", n)
			}
		})
	}
}

func TestUpdateModeRewritesTheLegacyBaselineInPlace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("legacy file name is not valid on Windows")
	}
	dir := t.TempDir()
	sm := NewSnapshotManager(true)
	sm.SetContext(filepath.Join(dir, "t.star"), "test_p[a:b]")
	legacy := legacySnapshotFile(sm, "snap")
	writeBaseline(t, legacy, `"OLD"`+"\n")

	if err := sm.Compare(starlark.String("NEW"), "snap"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(legacy)
	if err != nil || !strings.Contains(string(got), "NEW") {
		t.Fatalf("legacy baseline not updated in place: %q, %v", got, err)
	}
	if n := countFiles(t, sm.snapshotDir()); n != 1 {
		t.Errorf("found %d snapshot files, want 1", n)
	}
}

// The encoding exists so a hostile test name cannot reach outside the snapshot
// directory. A file that happens to exist at the legacy location outside it must
// not be adopted, read, or overwritten.
func TestLegacyFallbackNeverLeavesTheSnapshotDirectory(t *testing.T) {
	dir := t.TempDir()
	sm := NewSnapshotManager(true)
	sm.SetContext(filepath.Join(dir, "t.star"), "test_p[../../../outside]")
	legacy := legacySnapshotFile(sm, "snap")
	rel, err := filepath.Rel(sm.snapshotDir(), legacy)
	if err != nil || !strings.HasPrefix(rel, "..") {
		t.Fatalf("test setup: legacy path %s should escape %s", legacy, sm.snapshotDir())
	}
	writeBaseline(t, legacy, "SENTINEL")

	if err := sm.Compare(starlark.String("NEW"), "snap"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(legacy); string(got) != "SENTINEL" {
		t.Errorf("file outside the snapshot directory was modified: %q", got)
	}
	if n := countFiles(t, sm.snapshotDir()); n != 1 {
		t.Errorf("expected the new baseline inside the snapshot directory, found %d files", n)
	}
}

func TestNoLegacyFileMeansTheEncodedPath(t *testing.T) {
	dir := t.TempDir()
	sm := NewSnapshotManager(false)
	sm.SetContext(filepath.Join(dir, "t.star"), "test_p[a:b]")
	want := filepath.Join(sm.snapshotDir(), "test_p[a%3Ab]__snap.snap")
	if got := sm.snapshotPath("snap"); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}
