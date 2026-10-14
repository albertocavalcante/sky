package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func runFrames(t *testing.T, in string) []map[string]json.RawMessage {
	t.Helper()
	var out syncBuffer
	conn := NewConn(&mockConn{Reader: strings.NewReader(in), Writer: &out},
		HandlerFunc(func(context.Context, *Request) (any, error) { return nil, nil }))
	out.conn = conn
	if err := conn.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	return decodeResponses(t, out.String())
}

func TestRequestFieldValidation(t *testing.T) {
	for name, body := range map[string]string{
		"method null":     `{"jsonrpc":"2.0","id":1,"method":null}`,
		"method missing":  `{"jsonrpc":"2.0","id":1}`,
		"method empty":    `{"jsonrpc":"2.0","id":1,"method":""}`,
		"jsonrpc missing": `{"id":1,"method":"x"}`,
		"jsonrpc 1.0":     `{"jsonrpc":"1.0","id":1,"method":"x"}`,
		"id object":       `{"jsonrpc":"2.0","id":{},"method":"x"}`,
		"id bool":         `{"jsonrpc":"2.0","id":true,"method":"x"}`,
		"id null":         `{"jsonrpc":"2.0","id":null,"method":"x"}`,
	} {
		resps := runFrames(t, frame(body))
		if len(resps) != 1 {
			t.Errorf("%s: responses = %v", name, resps)
			continue
		}
		var e ResponseError
		_ = json.Unmarshal(resps[0]["error"], &e)
		if e.Code != CodeInvalidRequest {
			t.Errorf("%s: code = %d, want %d", name, e.Code, CodeInvalidRequest)
		}
	}
}

func TestClientResponsesAreIgnored(t *testing.T) {
	resps := runFrames(t, frame(`{"jsonrpc":"2.0","id":3,"result":null}`)+frame(`{"jsonrpc":"2.0","id":4,"method":"ping"}`))
	if len(resps) != 1 || string(resps[0]["id"]) != "4" {
		t.Errorf("responses = %v", resps)
	}
}

// A client that never reads must get its connection terminated once the
// outbound queue cap is exceeded, instead of growing memory without bound.
func TestOverloadedClientIsDisconnected(t *testing.T) {
	pr, pw := io.Pipe()
	defer func() { _ = pw.Close() }()
	w := &stallWriter{Reader: pr, release: make(chan struct{})}
	defer close(w.release)
	var conn *Conn
	conn = NewConn(w, HandlerFunc(func(context.Context, *Request) (any, error) {
		for i := 0; i < maxQueuedMessages+10; i++ {
			_ = conn.Notify(context.Background(), "x", nil)
		}
		return nil, nil
	}))
	go func() { _, _ = io.WriteString(pw, frame(`{"jsonrpc":"2.0","method":"spam"}`)) }()

	errc := make(chan error, 1)
	go func() { errc <- conn.Run(context.Background()) }()
	select {
	case err := <-errc:
		if !errors.Is(err, ErrOutput) {
			t.Errorf("Run err = %v, want ErrOutput", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("overloaded connection was not terminated")
	}
	conn.outMu.Lock()
	n := len(conn.outQueue)
	conn.outMu.Unlock()
	if n > maxQueuedMessages {
		t.Errorf("queue grew to %d", n)
	}
}

// EOF while output is blocked must not look like a clean shutdown.
func TestEOFWithBlockedOutputReportsError(t *testing.T) {
	w := &stallWriter{
		Reader:  strings.NewReader(frame(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)),
		release: make(chan struct{}),
	}
	defer close(w.release)
	conn := NewConn(w, HandlerFunc(func(context.Context, *Request) (any, error) { return nil, nil }))
	err := conn.Run(context.Background())
	if !errors.Is(err, ErrOutput) {
		t.Errorf("Run err = %v, want ErrOutput", err)
	}
}
