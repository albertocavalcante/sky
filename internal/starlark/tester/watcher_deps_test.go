package tester

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func newDepsWatcher(t *testing.T, dir string, files map[string]string) *Watcher {
	t.Helper()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w, err := NewWatcher(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return w
}

func recvEvent(t *testing.T, w *Watcher) WatchEvent {
	t.Helper()
	select {
	case ev := <-w.Events:
		return ev
	case <-time.After(10 * time.Second): // hang guard only
		t.Fatal("no event")
		return WatchEvent{}
	}
}

func TestWatcher_ConftestEditAffectsTest(t *testing.T) {
	dir := t.TempDir()
	w := newDepsWatcher(t, dir, map[string]string{"conftest.star": "x = 1\n", "test_a.star": "def test_a():\n  pass\n"})
	tf := filepath.Join(dir, "test_a.star")
	if err := w.Add(tf); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(dir, "conftest.star")
	if got := w.AffectedTestFiles(conf); len(got) != 1 || got[0] != tf {
		t.Fatalf("AffectedTestFiles(conftest) = %v, want [%s]", got, tf)
	}
	w.process(fsnotify.Event{Name: conf, Op: fsnotify.Write})
	if ev := recvEvent(t, w); len(ev.AffectedTests) != 1 || ev.AffectedTests[0] != tf {
		t.Errorf("event = %+v", ev)
	}
}

func TestWatcher_ConftestCreatedLater(t *testing.T) {
	dir := t.TempDir()
	w := newDepsWatcher(t, dir, map[string]string{"test_a.star": "x = 1\n"})
	tf := filepath.Join(dir, "test_a.star")
	if err := w.Add(tf); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(dir, "conftest.star")
	if err := os.WriteFile(conf, []byte("y = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w.process(fsnotify.Event{Name: conf, Op: fsnotify.Create})
	if ev := recvEvent(t, w); len(ev.AffectedTests) != 1 || ev.AffectedTests[0] != tf {
		t.Errorf("event = %+v", ev)
	}
}

func TestWatcher_MissingLoadCreatedLater(t *testing.T) {
	dir := t.TempDir()
	w := newDepsWatcher(t, dir, map[string]string{"test_a.star": "load(\"later.star\", \"v\")\n"})
	tf := filepath.Join(dir, "test_a.star")
	if err := w.Add(tf); err != nil {
		t.Fatal(err)
	}
	later := filepath.Join(dir, "later.star")
	if got := w.AffectedTestFiles(later); len(got) != 1 || got[0] != tf {
		t.Fatalf("AffectedTestFiles(later) = %v, want [%s]", got, tf)
	}
	// The new file's own loads become tracked after the refresh.
	if err := os.WriteFile(later, []byte("load(\"deeper.star\", \"d\")\nv = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w.process(fsnotify.Event{Name: later, Op: fsnotify.Create})
	recvEvent(t, w)
	if err := w.RefreshDependencies(later); err != nil {
		t.Fatal(err)
	}
	if got := w.AffectedTestFiles(filepath.Join(dir, "deeper.star")); len(got) != 1 || got[0] != tf {
		t.Errorf("deeper not tracked: %v", got)
	}
}

func TestWatcher_DeletedTestFileLeavesWatchedSet(t *testing.T) {
	dir := t.TempDir()
	w := newDepsWatcher(t, dir, map[string]string{"test_a.star": "x = 1\n", "test_b.star": "x = 1\n"})
	a, b := filepath.Join(dir, "test_a.star"), filepath.Join(dir, "test_b.star")
	for _, f := range []string{a, b} {
		if err := w.Add(f); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(a); err != nil {
		t.Fatal(err)
	}
	w.process(fsnotify.Event{Name: a, Op: fsnotify.Remove})
	if got := w.WatchedFiles(); len(got) != 1 || got[0] != b {
		t.Fatalf("WatchedFiles = %v, want [%s]", got, b)
	}
	if len(w.Events) != 0 {
		t.Error("unexpected event for a deleted test file")
	}
	// Re-created (e.g. the tail of an atomic save): watched again.
	if err := os.WriteFile(a, []byte("x = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w.process(fsnotify.Event{Name: a, Op: fsnotify.Create})
	if ev := recvEvent(t, w); len(ev.AffectedTests) != 1 || ev.AffectedTests[0] != a {
		t.Errorf("event = %+v", ev)
	}
	if len(w.WatchedFiles()) != 2 {
		t.Errorf("WatchedFiles = %v, want both", w.WatchedFiles())
	}
}

func TestWatcher_DeletedDependencyRerunsAndIsRecreatable(t *testing.T) {
	dir := t.TempDir()
	w := newDepsWatcher(t, dir, map[string]string{"h.star": "x = 1\n", "test_a.star": "load(\"h.star\", \"x\")\n"})
	tf, h := filepath.Join(dir, "test_a.star"), filepath.Join(dir, "h.star")
	if err := w.Add(tf); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(h); err != nil {
		t.Fatal(err)
	}
	w.process(fsnotify.Event{Name: h, Op: fsnotify.Rename})
	if ev := recvEvent(t, w); len(ev.AffectedTests) != 1 || ev.AffectedTests[0] != tf {
		t.Errorf("event = %+v", ev)
	}
	if err := w.RefreshDependencies(h); err != nil {
		t.Fatal(err)
	}
	if got := w.AffectedTestFiles(h); len(got) != 1 {
		t.Errorf("deleted dep no longer tracked: %v", got)
	}
}
