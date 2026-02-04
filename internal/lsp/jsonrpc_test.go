package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
)

func TestReadRequest(t *testing.T) {
	input := "Content-Length: 52\r\n\r\n{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"test\",\"params\":{}}"

	conn := NewConn(&mockConn{
		Reader: bytes.NewReader([]byte(input)),
		Writer: io.Discard,
	}, nil)

	req, err := conn.readRequest()
	if err != nil {
		t.Fatalf("readRequest failed: %v", err)
	}

	if req.Method != "test" {
		t.Errorf("Method = %q, want %q", req.Method, "test")
	}
	if req.ID == nil {
		t.Error("ID should not be nil")
	}
}

func TestWriteResponse(t *testing.T) {
	var buf bytes.Buffer
	conn := NewConn(&mockConn{
		Reader: bytes.NewReader(nil),
		Writer: &buf,
	}, nil)

	id := json.RawMessage(`1`)
	resp := &Response{
		JSONRPC: "2.0",
		ID:      &id,
		Result:  map[string]string{"status": "ok"},
	}

	if err := conn.writeResponse(resp); err != nil {
		t.Fatalf("writeResponse failed: %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("Content-Length:")) {
		t.Error("output should contain Content-Length header")
	}
	if !bytes.Contains([]byte(output), []byte(`"result"`)) {
		t.Error("output should contain result field")
	}
}

func TestResponseError(t *testing.T) {
	err := &ResponseError{
		Code:    CodeMethodNotFound,
		Message: "method not found",
	}

	if err.Error() != "jsonrpc error -32601: method not found" {
		t.Errorf("Error() = %q, want %q", err.Error(), "jsonrpc error -32601: method not found")
	}
}

func TestHandlerFunc(t *testing.T) {
	called := false
	h := HandlerFunc(func(ctx context.Context, req *Request) (any, error) {
		called = true
		return "ok", nil
	})

	result, err := h.Handle(context.Background(), &Request{Method: "test"})
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !called {
		t.Error("handler was not called")
	}
	if result != "ok" {
		t.Errorf("result = %v, want %q", result, "ok")
	}
}

type mockConn struct {
	io.Reader
	io.Writer
}

func (m *mockConn) Close() error {
	return nil
}
