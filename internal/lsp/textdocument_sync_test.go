package lsp

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
)

func openDoc(s *Server, uri, content string) {
	s.mu.Lock()
	s.initialized = true
	s.documents[uri] = &Document{URI: uri, Version: 1, Content: content}
	s.mu.Unlock()
}

func docContent(s *Server, uri string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.documents[uri].Content
}

func sendDidChange(t *testing.T, s *Server, uri string, version int, changes string) {
	t.Helper()
	raw := `{"textDocument":{"uri":"` + uri + `","version":` + itoa(version) + `},"contentChanges":` + changes + `}`
	if _, err := s.Handle(context.Background(), &Request{Method: "textDocument/didChange", Params: json.RawMessage(raw)}); err != nil {
		t.Fatalf("didChange: %v", err)
	}
}

func itoa(i int) string { b, _ := json.Marshal(i); return string(b) }

func TestDidChange_TextOnlyReplacesDocument(t *testing.T) {
	s := NewServer(nil)
	uri := "file:///a.star"
	openDoc(s, uri, "x = 1\n")
	sendDidChange(t, s, uri, 2, `[{"text":"y = 2\n"}]`)
	if got := docContent(s, uri); got != "y = 2\n" {
		t.Errorf("content = %q", got)
	}
}

func TestDidChange_IncrementalUTF16(t *testing.T) {
	s := NewServer(nil)
	uri := "file:///a.star"
	// "😀" is 2 UTF-16 units (4 bytes); "é" is 1 unit (2 bytes).
	openDoc(s, uri, "a = \"😀é\"\nb = 2\n")

	// Replace "é" (UTF-16 cols 7..8 on line 0) with "e".
	sendDidChange(t, s, uri, 2,
		`[{"range":{"start":{"line":0,"character":7},"end":{"line":0,"character":8}},"text":"e"}]`)
	if got := docContent(s, uri); got != "a = \"😀e\"\nb = 2\n" {
		t.Errorf("after edit 1: %q", got)
	}

	// Insert at 0:0 with an explicit zero range must not replace the document.
	sendDidChange(t, s, uri, 3,
		`[{"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}},"text":"# hi\n"}]`)
	if got := docContent(s, uri); got != "# hi\na = \"😀e\"\nb = 2\n" {
		t.Errorf("after edit 2: %q", got)
	}

	// Multiple changes apply in order; multi-line range delete.
	sendDidChange(t, s, uri, 4, `[
		{"range":{"start":{"line":1,"character":0},"end":{"line":2,"character":0}},"text":""},
		{"text":"final\n"}
	]`)
	if got := docContent(s, uri); got != "final\n" {
		t.Errorf("after edit 3: %q", got)
	}
	s.mu.RLock()
	v := s.documents[uri].Version
	s.mu.RUnlock()
	if v != 4 {
		t.Errorf("version = %d", v)
	}
}

func TestDidChange_UnknownDocumentIgnored(t *testing.T) {
	s := NewServer(nil)
	s.mu.Lock()
	s.initialized = true
	s.mu.Unlock()
	sendDidChange(t, s, "file:///nope", 1, `[{"text":"x"}]`)
}

// Handlers read documents outside the lock while didChange replaces them;
// run under -race to verify snapshots are never mutated in place.
func TestDidChange_NoDataRaceWithReaders(t *testing.T) {
	s := NewServer(nil)
	uri := "file:///race.star"
	openDoc(s, uri, "def foo():\n    return 1\n")

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			params := json.RawMessage(`{"textDocument":{"uri":"` + uri + `"},"position":{"line":0,"character":5}}`)
			for {
				select {
				case <-stop:
					return
				default:
				}
				_, _ = s.Handle(context.Background(), &Request{Method: "textDocument/hover", Params: params})
				_, _ = s.Handle(context.Background(), &Request{Method: "textDocument/completion", Params: params})
			}
		}()
	}
	for i := 0; i < 200; i++ {
		sendDidChange(t, s, uri, i, `[{"range":{"start":{"line":0,"character":4},"end":{"line":0,"character":7}},"text":"foo"}]`)
	}
	close(stop)
	wg.Wait()
}

// publishRecorder returns a server wired to a conn that records notifications.
func publishRecorder(t *testing.T) (*Server, *syncBuffer) {
	t.Helper()
	var out syncBuffer
	s := NewServer(nil)
	s.SetConn(NewConn(&mockConn{Reader: strings.NewReader(""), Writer: &out}, s))
	return s, &out
}

func countPublishes(t *testing.T, out *syncBuffer, uri string) int {
	t.Helper()
	n := 0
	for _, m := range decodeResponses(t, out.String()) {
		if string(m["method"]) == `"textDocument/publishDiagnostics"` && strings.Contains(string(m["params"]), uri) {
			n++
		}
	}
	return n
}

func TestDidChange_RepublishesDiagnostics(t *testing.T) {
	s, out := publishRecorder(t)
	uri := "file:///tmp/does-not-exist-sky/a.star"
	openDoc(s, uri, "x = 1\n")
	sendDidChange(t, s, uri, 2, `[{"text":"y = undefined_name\n"}]`)
	if n := countPublishes(t, out, uri); n != 1 {
		t.Errorf("publishes after didChange = %d, want 1", n)
	}
}

func TestDidSave_EmptyContentStillPublishes(t *testing.T) {
	s, out := publishRecorder(t)
	uri := "file:///tmp/does-not-exist-sky/b.star"
	openDoc(s, uri, "")
	_, err := s.Handle(context.Background(), &Request{
		Method: "textDocument/didSave",
		Params: json.RawMessage(`{"textDocument":{"uri":"` + uri + `"}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := countPublishes(t, out, uri); n != 1 {
		t.Errorf("publishes after didSave of empty doc = %d, want 1", n)
	}
	// Published diagnostics must serialise as [] rather than null.
	if !strings.Contains(out.String(), `"diagnostics":[]`) {
		t.Errorf("expected empty diagnostics array, got %s", out.String())
	}
}

func TestSetConnConcurrentWithPublish(t *testing.T) {
	s := NewServer(nil)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			s.SetConn(NewConn(&mockConn{Reader: strings.NewReader(""), Writer: io.Discard}, s))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			s.publishDiagnostics(context.Background(), "file:///x.star", "x = 1\n")
		}
	}()
	wg.Wait()
}
