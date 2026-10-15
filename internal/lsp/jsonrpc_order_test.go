package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// gatedServer holds textDocument/rename until the didChange that follows it
// on the wire has been handled, forcing the interleaving that used to let the
// handler read the replaced document.
type gatedServer struct {
	*Server
	changed chan struct{}
}

func (g *gatedServer) Handle(ctx context.Context, req *Request) (any, error) {
	switch req.Method {
	case "textDocument/rename":
		<-g.changed
	case "textDocument/didChange":
		defer close(g.changed)
	}
	return g.Server.Handle(ctx, req)
}

func TestRequestSeesDocumentAsReceived(t *testing.T) {
	var out syncBuffer
	srv := &gatedServer{Server: NewServer(nil), changed: make(chan struct{})}
	uri := "file:///swap.star"
	in := frame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`) +
		frame(`{"jsonrpc":"2.0","method":"initialized","params":{}}`) +
		frame(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+uri+`","languageId":"starlark","version":1,"text":"a = 1\nb = 2\nprint(a)\n"}}}`) +
		frame(`{"jsonrpc":"2.0","id":2,"method":"textDocument/rename","params":{"textDocument":{"uri":"`+uri+`"},"position":{"line":0,"character":0},"newName":"c"}}`) +
		// The declarations swap: line 0 is now b, not a.
		frame(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":"`+uri+`","version":2},"contentChanges":[{"text":"b = 2\na = 1\nprint(a)\n"}]}}`)
	conn := NewConn(&mockConn{Reader: strings.NewReader(in), Writer: &out}, srv)
	srv.SetConn(conn)
	if err := conn.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, r := range decodeResponses(t, out.String()) {
		if string(r["id"]) != "2" {
			continue
		}
		var e ResponseError
		if err := json.Unmarshal(r["error"], &e); err != nil || e.Code != CodeContentModified {
			t.Fatalf("rename response = %s, want ContentModified error", out.String())
		}
		return
	}
	t.Fatal("no rename response")
}

func TestPreparedRequestUnchangedDocumentIsAnswered(t *testing.T) {
	s := NewServer(nil)
	openDoc(s, "file:///ok.star", "a = 1\nprint(a)\n")
	req := &Request{
		Method: "textDocument/references",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///ok.star"},"position":{"line":0,"character":0},"context":{"includeDeclaration":true}}`),
	}
	s.PrepareRequest(req)
	if req.snapshot == nil {
		t.Fatal("no snapshot taken")
	}
	if _, err := s.Handle(context.Background(), req); err != nil {
		t.Fatalf("unchanged document must be answered: %v", err)
	}
	openDoc(s, "file:///ok.star", "a = 1\nprint(a)\n") // same text, replaced document
	_, err := s.Handle(context.Background(), req)
	var re *ResponseError
	if !errors.As(err, &re) || re.Code != CodeContentModified {
		t.Fatalf("err = %v, want ContentModified", err)
	}
}

func TestHeaderLimits(t *testing.T) {
	run := func(in string) error {
		conn := NewConn(&mockConn{Reader: strings.NewReader(in), Writer: &syncBuffer{}},
			HandlerFunc(func(context.Context, *Request) (any, error) { return nil, nil }))
		return conn.Run(context.Background())
	}
	body := `{"jsonrpc":"2.0","method":"x"}`
	cl := "Content-Length: 30\r\n"
	if len(body) != 30 {
		t.Fatalf("test body length = %d", len(body))
	}

	t.Run("huge line", func(t *testing.T) {
		in := "X-Fill: " + strings.Repeat("a", 1<<20) + "\r\n" + cl + "\r\n" + body
		if err := run(in); !errors.Is(err, errHeaderTooLarge) {
			t.Fatalf("err = %v, want errHeaderTooLarge", err)
		}
	})
	// The next two cases stay far below the block cap, so only the per-line
	// cap can tell them apart.
	t.Run("line just over the cap", func(t *testing.T) {
		in := "X-Fill: " + strings.Repeat("a", maxHeaderLine-len("X-Fill: \r\n")+1) + "\r\n" + cl + "\r\n" + body
		if err := run(in); !errors.Is(err, errHeaderTooLarge) {
			t.Fatalf("err = %v, want errHeaderTooLarge", err)
		}
	})
	t.Run("line just under the cap", func(t *testing.T) {
		in := "X-Fill: " + strings.Repeat("a", maxHeaderLine-len("X-Fill: \r\n")-1) + "\r\n" + cl + "\r\n" + body
		if err := run(in); err != nil {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("oversized header block", func(t *testing.T) {
		// Every line is under the per-line cap; only the total is too big.
		line := "X-Fill: " + strings.Repeat("a", maxHeaderLine-len("X-Fill: \r\n")-1) + "\r\n"
		in := strings.Repeat(line, maxHeaderBytes/len(line)+2) + cl + "\r\n" + body
		if err := run(in); !errors.Is(err, errHeaderTooLarge) {
			t.Fatalf("err = %v, want errHeaderTooLarge", err)
		}
	})
	t.Run("large but allowed", func(t *testing.T) {
		line := "X-Fill: " + strings.Repeat("a", 1000) + "\r\n"
		in := strings.Repeat(line, 20) + cl + "\r\n" + body
		if err := run(in); err != nil {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestRunCancelsHandlersOnEOF(t *testing.T) {
	started := make(chan struct{})
	h := HandlerFunc(func(ctx context.Context, _ *Request) (any, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	conn := NewConn(&mockConn{
		Reader: strings.NewReader(frame(`{"jsonrpc":"2.0","id":1,"method":"block"}`)),
		Writer: &syncBuffer{},
	}, h)
	done := make(chan error, 1)
	go func() { done <- conn.Run(context.Background()) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run hangs on EOF while a handler waits for ctx.Done()")
	}
	select {
	case <-started:
	default:
		t.Fatal("handler never ran")
	}
}
