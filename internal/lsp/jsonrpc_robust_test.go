package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func frame(body string) string {
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
}

// syncBuffer is a goroutine-safe bytes.Buffer.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// decodeResponses splits framed output into decoded JSON objects.
func decodeResponses(t *testing.T, out string) []map[string]json.RawMessage {
	t.Helper()
	var res []map[string]json.RawMessage
	for len(out) > 0 {
		hdr, rest, ok := strings.Cut(out, "\r\n\r\n")
		if !ok {
			t.Fatalf("bad framing: %q", out)
		}
		var n int
		if _, err := fmt.Sscanf(hdr, "Content-Length: %d", &n); err != nil {
			t.Fatalf("bad header %q: %v", hdr, err)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal([]byte(rest[:n]), &m); err != nil {
			t.Fatalf("bad body %q: %v", rest[:n], err)
		}
		res = append(res, m)
		out = rest[n:]
	}
	return res
}

func TestResponseAlwaysHasResultOnSuccess(t *testing.T) {
	id := json.RawMessage(`7`)
	b, err := json.Marshal(Response{JSONRPC: "2.0", ID: &id})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"jsonrpc":"2.0","id":7,"result":null}` {
		t.Errorf("got %s", b)
	}

	b, _ = json.Marshal(Response{JSONRPC: "2.0", ID: &id, Error: &ResponseError{Code: 1, Message: "x"}})
	if strings.Contains(string(b), `"result"`) || !strings.Contains(string(b), `"error"`) {
		t.Errorf("error response wrong: %s", b)
	}
}

func TestShutdownResponseHasNullResult(t *testing.T) {
	var out syncBuffer
	srv := NewServer(nil)
	in := frame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`) +
		frame(`{"jsonrpc":"2.0","method":"initialized","params":{}}`) +
		frame(`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`)
	conn := NewConn(&mockConn{Reader: strings.NewReader(in), Writer: &out}, srv)
	srv.SetConn(conn)
	if err := conn.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	resps := decodeResponses(t, out.String())
	last := resps[len(resps)-1]
	if res, ok := last["result"]; !ok || string(res) != "null" {
		t.Errorf("shutdown response = %v, want result:null", last)
	}
}

func TestHandlerPanicBecomesInternalError(t *testing.T) {
	var out syncBuffer
	h := HandlerFunc(func(context.Context, *Request) (any, error) { panic("boom") })
	in := frame(`{"jsonrpc":"2.0","id":1,"method":"x"}`) + frame(`{"jsonrpc":"2.0","method":"note"}`)
	conn := NewConn(&mockConn{Reader: strings.NewReader(in), Writer: &out}, h)
	if err := conn.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	resps := decodeResponses(t, out.String())
	if len(resps) != 1 {
		t.Fatalf("got %d responses, want 1 (notification gets none)", len(resps))
	}
	var e ResponseError
	if err := json.Unmarshal(resps[0]["error"], &e); err != nil || e.Code != CodeInternalError {
		t.Errorf("error = %s", resps[0]["error"])
	}
}

func TestReadRequestValidation(t *testing.T) {
	read := func(in string) (*Request, error) {
		c := NewConn(&mockConn{Reader: strings.NewReader(in), Writer: io.Discard}, nil)
		return c.readRequest()
	}

	if _, err := read("Content-Length: -5\r\n\r\n"); err == nil {
		t.Error("negative length accepted")
	}
	if _, err := read("Content-Length: 999999999999\r\n\r\n"); err == nil {
		t.Error("huge length accepted")
	}
	if _, err := read(fmt.Sprintf("Content-Length: %d\r\n\r\n", maxMessageSize+1)); err == nil {
		t.Error("over-cap length accepted")
	}

	body := `{"jsonrpc":"2.0","id":1,"method":"m"}`
	for _, hdr := range []string{
		fmt.Sprintf("content-length:%d", len(body)),
		fmt.Sprintf("CONTENT-LENGTH:   %d  ", len(body)),
		fmt.Sprintf("Content-Type: x\r\nContent-Length: %d", len(body)),
	} {
		req, err := read(hdr + "\r\n\r\n" + body)
		if err != nil || req.Method != "m" {
			t.Errorf("header %q: req=%v err=%v", hdr, req, err)
		}
	}
}

func TestBadJSONBodyRepliesParseErrorAndContinues(t *testing.T) {
	var out syncBuffer
	h := HandlerFunc(func(context.Context, *Request) (any, error) { return "ok", nil })
	in := frame(`{not json`) + frame(`{"jsonrpc":"2.0","id":5,"method":"m"}`)
	conn := NewConn(&mockConn{Reader: strings.NewReader(in), Writer: &out}, h)
	if err := conn.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	resps := decodeResponses(t, out.String())
	if len(resps) != 2 {
		t.Fatalf("got %d responses: %v", len(resps), resps)
	}
	var e ResponseError
	_ = json.Unmarshal(resps[0]["error"], &e)
	if e.Code != CodeParseError || string(resps[0]["id"]) != "null" {
		t.Errorf("first response = %v", resps[0])
	}
	if string(resps[1]["result"]) != `"ok"` {
		t.Errorf("second response = %v", resps[1])
	}
}

// Notifications are handled in order on the read loop, so a request that
// follows didOpen always sees the opened document.
func TestNotificationsOrderedBeforeRequests(t *testing.T) {
	for i := 0; i < 50; i++ {
		var out syncBuffer
		srv := NewServer(nil)
		uri := "file:///order.star"
		in := frame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`) +
			frame(`{"jsonrpc":"2.0","method":"initialized","params":{}}`) +
			frame(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+uri+`","languageId":"starlark","version":1,"text":"def foo():\n    pass\n"}}}`) +
			frame(`{"jsonrpc":"2.0","id":2,"method":"textDocument/documentSymbol","params":{"textDocument":{"uri":"`+uri+`"}}}`)
		conn := NewConn(&mockConn{Reader: strings.NewReader(in), Writer: &out}, srv)
		srv.SetConn(conn)
		if err := conn.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		var found bool
		for _, r := range decodeResponses(t, out.String()) {
			if string(r["id"]) == "2" {
				found = strings.Contains(string(r["result"]), "foo")
			}
		}
		if !found {
			t.Fatalf("iteration %d: documentSymbol did not see didOpen document", i)
		}
	}
}

// blockingReader blocks Read until released, like stdin with no data.
type blockingReader struct{ release chan struct{} }

func (r *blockingReader) Read([]byte) (int, error) {
	<-r.release
	return 0, io.EOF
}

func TestRunReturnsPromptlyOnCancelWhileBlockedInRead(t *testing.T) {
	br := &blockingReader{release: make(chan struct{})}
	defer close(br.release)
	conn := NewConn(&mockConn{Reader: br, Writer: io.Discard}, HandlerFunc(
		func(context.Context, *Request) (any, error) { return nil, nil }))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- conn.Run(ctx) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}
}

func TestRunWaitsForInFlightHandlersOnEOF(t *testing.T) {
	var out syncBuffer
	var finished bool
	var mu sync.Mutex
	h := HandlerFunc(func(context.Context, *Request) (any, error) {
		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		finished = true
		mu.Unlock()
		return "ok", nil
	})
	conn := NewConn(&mockConn{
		Reader: strings.NewReader(frame(`{"jsonrpc":"2.0","id":1,"method":"slow"}`)),
		Writer: &out,
	}, h)
	if err := conn.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !finished {
		t.Error("Run returned before in-flight handler finished")
	}
	if !strings.Contains(out.String(), `"ok"`) {
		t.Error("response not written before Run returned")
	}
}
