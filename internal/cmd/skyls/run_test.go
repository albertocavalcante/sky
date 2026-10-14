package skyls

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func frame(body string) string {
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
}

const (
	initMsg     = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	initdMsg    = `{"jsonrpc":"2.0","method":"initialized","params":{}}`
	shutdownMsg = `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`
	exitMsg     = `{"jsonrpc":"2.0","method":"exit"}`
)

func runWith(t *testing.T, stdin io.Reader) (int, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	done := make(chan int, 1)
	go func() { done <- RunWithIO(context.Background(), nil, stdin, &out, &errOut) }()
	select {
	case code := <-done:
		return code, out.String()
	case <-time.After(5 * time.Second):
		t.Fatal("RunWithIO did not return")
		return 0, ""
	}
}

func TestExitAfterShutdownIsZero(t *testing.T) {
	in := frame(initMsg) + frame(initdMsg) + frame(shutdownMsg) + frame(exitMsg)
	code, out := runWith(t, strings.NewReader(in))
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, `"id":2,"result":null`) {
		t.Errorf("shutdown response missing result:null: %s", out)
	}
}

func TestExitWithoutShutdownIsOne(t *testing.T) {
	in := frame(initMsg) + frame(initdMsg) + frame(exitMsg)
	if code, _ := runWith(t, strings.NewReader(in)); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

// exit must terminate Run even though stdin stays open (blocked in read).
func TestExitReturnsWhileStdinStaysOpen(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()
	go func() {
		_, _ = io.WriteString(pw, frame(initMsg)+frame(shutdownMsg)+frame(exitMsg))
	}()
	if code, _ := runWith(t, pr); code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestEOFWithoutExitIsClean(t *testing.T) {
	if code, _ := runWith(t, strings.NewReader(frame(initMsg))); code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

// EOF inside a declared Content-Length is a protocol error, not a clean stop.
// The zero-body case (header block, then immediate EOF) is the one that used
// to be mistaken for a clean end of stream.
func TestTruncatedFrameIsError(t *testing.T) {
	for name, tail := range map[string]string{
		"no body bytes":      "Content-Length: 50\r\n\r\n",
		"partial body bytes": "Content-Length: 50\r\n\r\n{\"jsonrpc\"",
	} {
		in := frame(initMsg) + tail
		if code, _ := runWith(t, strings.NewReader(in)); code != 1 {
			t.Errorf("%s: exit code = %d, want 1", name, code)
		}
	}
}

// blockedWriter never accepts output, like a client that stopped reading.
type blockedWriter struct{ release chan struct{} }

func (w blockedWriter) Write(p []byte) (int, error) { <-w.release; return len(p), nil }

// EOF while responses are still unflushed must not exit 0.
func TestEOFWithUnflushedOutputIsError(t *testing.T) {
	w := blockedWriter{release: make(chan struct{})}
	defer close(w.release)
	var errOut bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- RunWithIO(context.Background(), nil, strings.NewReader(frame(initMsg)), w, &errOut)
	}()
	select {
	case code := <-done:
		if code != 1 {
			t.Errorf("exit code = %d, want 1 (stderr: %s)", code, errOut.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunWithIO did not return")
	}
}
