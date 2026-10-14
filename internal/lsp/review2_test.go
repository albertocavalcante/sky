package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/albertocavalcante/sky/internal/protocol"
	"github.com/albertocavalcante/sky/internal/starlark/checker"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

func TestPathToURI_LiteralBackslashOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix paths only")
	}
	p := `/tmp/a\b.bzl`
	got := pathToURI(p)
	if got != "file:///tmp/a%5Cb.bzl" {
		t.Errorf("pathToURI = %q", got)
	}
	if back := uriToPath(got); back != p {
		t.Errorf("round trip = %q", back)
	}
}

func TestCheckerDiagnostic_RuneColumns(t *testing.T) {
	// "s = \"😀\"; missing": 'missing' starts at rune col 10 (1-based), which
	// is UTF-16 col 10 (the emoji takes 2 units): 0-based rune 9 -> 10.
	content := "s = \"😀\"; missing\n"
	d := checker.Diagnostic{Pos: posAt(1, 10), End: posAt(1, 17), Severity: checker.SeverityError}
	got := checkerDiagnosticToLSPIn(content, d)
	if got.Range.Start.Character != 10 || got.Range.End.Character != 17 {
		t.Errorf("range = %+v, want 10..17", got.Range)
	}
}

func TestSemanticTokens_CommentLengthUTF16(t *testing.T) {
	toks := tokenizeContent("x = 1 # 😀\n")
	for _, tk := range toks {
		if tk.Type == TokenComment && tk.Length != 4 {
			t.Errorf("comment length = %d, want 4 UTF-16 units", tk.Length)
		}
	}
}

func TestIsValidIdentifier_StarlarkRules(t *testing.T) {
	for _, ok := range []string{"x", "_x", "π", "foo1", "été"} {
		if !isValidIdentifier(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "1x", "a-b", "a b", "is", "class", "async", "yield", "import", "def"} {
		if isValidIdentifier(bad) {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestReferences_LoadExcludedWithoutDeclarations(t *testing.T) {
	code := "load(\":lib.bzl\", \"foo\")\nfoo()\n"
	locs := doRefs(t, code, 1, 1, false)
	if len(locs) != 1 || locs[0].Range.Start.Line != 1 {
		t.Errorf("got %+v, want only the use on line 1", locs)
	}
}

func TestRename_ParameterRenamesKeywordArgs(t *testing.T) {
	code := "def f(name):\n    return name\n\nf(name = 1)\nf(name=2)\ng(name=3)\n"
	s := NewServer(nil)
	uri := "file:///kw.star"
	openDoc(s, uri, code)
	params, _ := json.Marshal(protocol.RenameParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
		Position:     protocol.Position{Line: 0, Character: 7},
		NewName:      "label",
	})
	res, err := s.Handle(context.Background(), &Request{Method: "textDocument/rename", Params: params})
	if err != nil {
		t.Fatal(err)
	}
	we := res.(*protocol.WorkspaceEdit)
	lines := map[uint32]bool{}
	for _, e := range we.Changes[uri] {
		lines[e.Range.Start.Line] = true
	}
	// def line, return line, and both f(...) calls; g(name=3) untouched.
	for _, l := range []uint32{0, 1, 3, 4} {
		if !lines[l] {
			t.Errorf("line %d not edited: %+v", l, we.Changes[uri])
		}
	}
	if lines[5] {
		t.Error("g(name=3) must not be edited")
	}
}

func TestCollectIdentRefs_Linear(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 20000; i++ {
		b.WriteString("x = x + 1\n")
	}
	content := b.String()
	file, err := parseStarlarkFile([]byte(content), "a.star", filekind.KindStarlark)
	if err != nil {
		t.Skip(err)
	}
	start := time.Now()
	refs := collectIdentRefs(content, file, "x")
	if len(refs) != 40000 {
		t.Fatalf("refs = %d", len(refs))
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("collectIdentRefs took %v (quadratic)", d)
	}
}

func TestLintContent_UsesGivenBytes(t *testing.T) {
	s := NewServer(nil)
	path := filepath.Join(t.TempDir(), "BUILD")
	if err := os.WriteFile(path, []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Buffer differs from disk; lint must not fail or use disk bytes.
	if _, err := s.lintBuffer(path, "y = 2\n"); err != nil {
		t.Fatal(err)
	}
	// Missing file on disk is fine: the buffer is what is linted.
	if _, err := s.lintBuffer(filepath.Join(t.TempDir(), "nope.bzl"), "y = 2\n"); err != nil {
		t.Errorf("in-memory lint should not need the file: %v", err)
	}
}

func TestTruncatedFrameIsError(t *testing.T) {
	conn := NewConn(&mockConn{
		Reader: strings.NewReader("Content-Length: 5\r\n\r\n"),
		Writer: io.Discard,
	}, HandlerFunc(func(context.Context, *Request) (any, error) { return nil, nil }))
	err := conn.Run(context.Background())
	if err == nil || errors.Is(err, io.EOF) {
		t.Errorf("Run err = %v, want non-EOF error", err)
	}
	// Truncated headers too.
	conn = NewConn(&mockConn{Reader: strings.NewReader("Content-Len"), Writer: io.Discard}, nil)
	if err := conn.Run(context.Background()); err == nil || errors.Is(err, io.EOF) {
		t.Errorf("truncated header err = %v", err)
	}
	// Clean EOF between messages is fine.
	conn = NewConn(&mockConn{Reader: strings.NewReader(""), Writer: io.Discard}, nil)
	if err := conn.Run(context.Background()); err != nil {
		t.Errorf("clean EOF err = %v", err)
	}
}

func TestInvalidRequestStructureIsInvalidRequest(t *testing.T) {
	var out syncBuffer
	in := frame(`{"jsonrpc":"2.0","id":1,"method":17}`)
	conn := NewConn(&mockConn{Reader: strings.NewReader(in), Writer: &out},
		HandlerFunc(func(context.Context, *Request) (any, error) { return nil, nil }))
	if err := conn.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	resps := decodeResponses(t, out.String())
	if len(resps) != 1 {
		t.Fatalf("resps = %v", resps)
	}
	var e ResponseError
	_ = json.Unmarshal(resps[0]["error"], &e)
	if e.Code != CodeInvalidRequest {
		t.Errorf("code = %d, want %d", e.Code, CodeInvalidRequest)
	}
}

// closableReader unblocks Read when closed, like a pipe.
type closableReader struct {
	once sync.Once
	ch   chan struct{}
	done chan struct{}
}

func (r *closableReader) Read([]byte) (int, error) {
	defer close(r.done)
	<-r.ch
	return 0, io.ErrClosedPipe
}
func (r *closableReader) Write(p []byte) (int, error) { return len(p), nil }
func (r *closableReader) Close() error                { r.once.Do(func() { close(r.ch) }); return nil }

func TestRunCancelJoinsReaderGoroutine(t *testing.T) {
	r := &closableReader{ch: make(chan struct{}), done: make(chan struct{})}
	conn := NewConn(r, HandlerFunc(func(context.Context, *Request) (any, error) { return nil, nil }))
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- conn.Run(ctx) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-errc:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return")
	}
	select {
	case <-r.done:
	case <-time.After(time.Second):
		t.Fatal("reader goroutine leaked after cancel")
	}
}

// stallWriter never completes a write until released, like a client that
// is not reading its stdin.
type stallWriter struct {
	io.Reader
	release chan struct{}
}

func (w *stallWriter) Write(p []byte) (int, error) { <-w.release; return len(p), nil }
func (w *stallWriter) Close() error                { return nil }

func TestExitNotBlockedByStalledClient(t *testing.T) {
	uri := "file:///stall.star"
	var in bytes.Buffer
	in.WriteString(frame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	in.WriteString(frame(`{"jsonrpc":"2.0","method":"initialized","params":{}}`))
	in.WriteString(frame(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"` + uri + `","languageId":"starlark","version":1,"text":"x = 1\n"}}}`))
	in.WriteString(frame(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":"` + uri + `","version":2},"contentChanges":[{"text":"x = 2\n"}]}}`))
	in.WriteString(frame(`{"jsonrpc":"2.0","method":"exit"}`))

	w := &stallWriter{Reader: &in, release: make(chan struct{})}
	defer close(w.release)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := NewServer(cancel)
	conn := NewConn(w, srv)
	srv.SetConn(conn)

	done := make(chan struct{})
	go func() { _ = conn.Run(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run deadlocked with a client that never reads")
	}
	if _, exited := srv.ExitStatus(); !exited {
		t.Error("exit was never processed")
	}
}
