package tester

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcher_LoadCycles(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	a := write("a.star", "load(\"b.star\", \"y\")\nx = 1\n")
	b := write("b.star", "load(\"a.star\", \"x\")\ny = 1\n")
	self := write("self.star", "load(\"self.star\", \"z\")\nz = 1\n")

	w, err := NewWatcher(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	for _, f := range []string{a, self} {
		if err := w.Add(f); err != nil {
			t.Fatal(err)
		}
	}
	if got := w.AffectedTestFiles(b); len(got) != 1 || got[0] != a {
		t.Errorf("AffectedTestFiles(b) = %v, want [%s]", got, a)
	}
}

func TestWatcher_CloseTwice(t *testing.T) {
	w, err := NewWatcher(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	_ = w.Close()
}

func TestWatcher_AddManyBadFilesDoesNotBlock(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWatcher(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 30; i++ {
			p := filepath.Join(dir, fmt.Sprintf("bad%d.star", i))
			_ = os.WriteFile(p, []byte("def ("), 0o644)
			_ = w.Add(p)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Add blocked on full Errors channel")
	}
}

func TestWatcher_RefreshDependencies_Helper(t *testing.T) {
	dir := t.TempDir()
	h := filepath.Join(dir, "h.star")
	h2 := filepath.Join(dir, "h2.star")
	tf := filepath.Join(dir, "test_x.star")
	for p, b := range map[string]string{h: "x = 1\n", h2: "y = 1\n", tf: "load(\"h.star\", \"x\")\n"} {
		if err := os.WriteFile(p, []byte(b), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w, err := NewWatcher(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	if err := w.Add(tf); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h, []byte("load(\"h2.star\", \"y\")\nx = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := w.RefreshDependencies(h); err != nil {
		t.Fatal(err)
	}
	if got := w.AffectedTestFiles(h2); len(got) != 1 || got[0] != tf {
		t.Errorf("AffectedTestFiles(h2) = %v, want [%s]", got, tf)
	}
}

func TestWatcher_AtomicSave(t *testing.T) {
	dir := t.TempDir()
	tf := filepath.Join(dir, "test_x.star")
	if err := os.WriteFile(tf, []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w, err := NewWatcher(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	if err := w.Add(tf); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		tmp := filepath.Join(dir, ".tmp.swp")
		if err := os.WriteFile(tmp, []byte("x = 2\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(tmp, tf); err != nil {
			t.Fatal(err)
		}
		select {
		case <-w.Events:
		case <-time.After(3 * time.Second):
			t.Fatalf("save %d: no event after atomic rename", i)
		}
		// drain any trailing duplicates
		time.Sleep(100 * time.Millisecond)
		for len(w.Events) > 0 {
			<-w.Events
		}
	}
}
