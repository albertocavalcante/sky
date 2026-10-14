package tester

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestMockFileScopedFixtureSurvivesReset(t *testing.T) {
	res := runSrc(t, "m.star", `
__fixture_config__ = {"wrapped": "file"}

def fixture_wrapped(mock):
    return mock.wrap(len)

def test_a(wrapped, mock):
    wrapped([1])
    assert.eq(mock.call_count(wrapped), 1)

def test_b(wrapped, mock):
    wrapped([1])
    assert.eq(mock.call_count(wrapped), 1)
`, DefaultOptions())
	for _, tr := range res.Tests {
		if !tr.Passed {
			t.Errorf("%s failed: %v", tr.Name, tr.Error)
		}
	}
}

func TestSnapshotParametrizedNameCannotEscape(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "a", "b", "c")
	sm := NewSnapshotManager(true)
	sm.SetContext(filepath.Join(dir, "t.star"), "test_p[../../../../outside]")
	p := sm.snapshotPath("snap")
	rel, err := filepath.Rel(sm.snapshotDir(), p)
	if err != nil || strings.HasPrefix(rel, "..") || strings.ContainsAny(rel, `/\`) {
		t.Fatalf("snapshot path %q escapes %q (rel %q)", p, sm.snapshotDir(), rel)
	}
	sm.SetContext(filepath.Join(dir, "t.star"), "test_p[.._.._x]")
	if q := sm.snapshotPath("snap"); q == p {
		t.Errorf("distinct case names collide: %s", q)
	}
	sm.SetContext(filepath.Join(dir, "t.star"), "test_p[../..]")
	p1 := sm.snapshotPath("snap")
	sm.SetContext(filepath.Join(dir, "t.star"), "test_p[__..]")
	if p1 == sm.snapshotPath("snap") {
		t.Errorf("collision")
	}
}

func TestWatcherHandleEventNoDeadlockWithRefresh(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.star")
	if err := os.WriteFile(f, []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w, err := NewWatcher(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	if err := w.Add(f); err != nil {
		t.Fatal(err)
	}
	// Producer: more events than the buffer holds.
	go func() {
		for i := 0; i < cap(w.Events)*3; i++ {
			w.handleEvent(fsnotify.Event{Name: f, Op: fsnotify.Write})
		}
	}()
	// Consumer: needs the write lock for each event it receives.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < cap(w.Events)+5; i++ {
			select {
			case ev := <-w.Events:
				_ = w.RefreshDependencies(ev.File)
			case <-time.After(3 * time.Second):
				return
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock: consumer stuck")
	}
	// Let the producer refill the buffer and block on a further send.
	for i := 0; i < 200 && len(w.Events) < cap(w.Events); i++ {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	// Prove no stuck lock: write lock must be acquirable.
	ok := make(chan struct{})
	go func() { w.mu.Lock(); w.mu.Unlock(); close(ok) }()
	select {
	case <-ok:
	case <-time.After(2 * time.Second):
		t.Fatal("deadlock: write lock unobtainable (handleEvent blocked holding RLock)")
	}
}

func TestSnapshotPathKeepsLegacyNames(t *testing.T) {
	dir := t.TempDir()
	sm := NewSnapshotManager(false)
	sm.SetContext(filepath.Join(dir, "t.star"), "test_p[hello world]")
	want := filepath.Join(sm.snapshotDir(), "test_p[hello world]__snap.snap")
	if got := sm.snapshotPath("snap"); got != want {
		t.Errorf("existing-style name changed:\n got %s\nwant %s", got, want)
	}
}

func TestSnapshotPathBoundedLength(t *testing.T) {
	dir := t.TempDir()
	sm := NewSnapshotManager(false)
	long := "test_p[" + strings.Repeat("a b ", 100) + "]"
	sm.SetContext(filepath.Join(dir, "t.star"), long)
	p := sm.snapshotPath("snap")
	if n := len(filepath.Base(p)); n > 255 {
		t.Errorf("filename %d bytes > 255", n)
	}
	sm.SetContext(filepath.Join(dir, "t.star"), long+"x")
	if sm.snapshotPath("snap") == p {
		t.Error("distinct long names collide")
	}
}
