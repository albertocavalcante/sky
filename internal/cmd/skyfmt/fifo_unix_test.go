//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package skyfmt

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A FIFO named like a source file must be skipped by directory expansion:
// reading it would block forever.
func TestDirectoryExpansionSkipsFIFO(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.star"), []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "wait.star")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	t.Cleanup(func() {
		// Release a blocked reader so a failing run does not leak a stuck goroutine.
		if f, err := os.OpenFile(fifo, os.O_RDWR, 0); err == nil {
			_ = f.Close()
		}
	})

	done := make(chan int, 1)
	go func() {
		var stdout, stderr bytes.Buffer
		done <- RunWithIO(context.Background(), []string{"-check", dir}, nil, &stdout, &stderr)
	}()
	select {
	case code := <-done:
		if code != 0 {
			t.Errorf("exit code %d, want 0", code)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("blocked reading a FIFO selected by filename")
	}

	got, err := expandPath(dir)
	if err != nil || len(got) != 1 || filepath.Base(got[0]) != "ok.star" {
		t.Errorf("expandPath = %v, %v; want only ok.star", got, err)
	}
}

// A glob or explicit path naming a FIFO must fail fast instead of blocking.
func TestExplicitFIFOIsRejected(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "wait.star")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	t.Cleanup(func() {
		if f, err := os.OpenFile(fifo, os.O_RDWR, 0); err == nil {
			_ = f.Close()
		}
	})

	for _, pat := range []string{fifo} {
		done := make(chan int, 1)
		var stdout, stderr bytes.Buffer
		go func() {
			done <- RunWithIO(context.Background(), []string{"-check", pat}, nil, &stdout, &stderr)
		}()
		select {
		case code := <-done:
			if code == 0 {
				t.Errorf("%s: exit code 0, want non-zero", pat)
			}
			if !strings.Contains(stderr.String(), "not a regular file") {
				t.Errorf("%s: stderr = %q, want a not-a-regular-file error", pat, stderr.String())
			}
		case <-time.After(30 * time.Second):
			t.Fatalf("%s: blocked reading a FIFO", pat)
		}
	}
}
