// Package lsp implements a Language Server Protocol server for Starlark.
package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
)

// JSON-RPC 2.0 message types

// Request is a JSON-RPC request or notification.
type Request struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"` // nil for notifications
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

// Response is a JSON-RPC response. On success the "result" member is always
// emitted (as null when Result is nil); on failure only "error" is emitted.
type Response struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Result  any              `json:"result,omitempty"`
	Error   *ResponseError   `json:"error,omitempty"`
}

// MarshalJSON implements json.Marshaler.
func (r Response) MarshalJSON() ([]byte, error) {
	id := json.RawMessage("null")
	if r.ID != nil && len(*r.ID) > 0 {
		id = *r.ID
	}
	if r.Error != nil {
		return json.Marshal(struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Error   *ResponseError  `json:"error"`
		}{r.JSONRPC, id, r.Error})
	}
	return json.Marshal(struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  any             `json:"result"`
	}{r.JSONRPC, id, r.Result})
}

// ResponseError is a JSON-RPC error.
type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error implements the error interface.
func (e *ResponseError) Error() string {
	return fmt.Sprintf("jsonrpc error %d: %s", e.Code, e.Message)
}

// Standard JSON-RPC error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603

	// LSP-specific error codes
	CodeRequestFailed    = -32803
	CodeRequestCancelled = -32800
	CodeContentModified  = -32801
)

// Conn handles JSON-RPC communication over an io.ReadWriteCloser.
type Conn struct {
	rwc     io.ReadWriteCloser
	reader  *bufio.Reader
	writeMu sync.Mutex

	handler Handler

	wg sync.WaitGroup // tracks in-flight request goroutines
}

// Handler processes incoming requests.
type Handler interface {
	Handle(ctx context.Context, req *Request) (result any, err error)
}

// HandlerFunc is an adapter to use functions as Handler.
type HandlerFunc func(ctx context.Context, req *Request) (any, error)

func (f HandlerFunc) Handle(ctx context.Context, req *Request) (any, error) {
	return f(ctx, req)
}

// NewConn creates a new JSON-RPC connection.
func NewConn(rwc io.ReadWriteCloser, handler Handler) *Conn {
	return &Conn{
		rwc:     rwc,
		reader:  bufio.NewReader(rwc),
		handler: handler,
	}
}

// maxMessageSize caps the Content-Length we are willing to allocate.
const maxMessageSize = 64 << 20 // 64 MiB

// errBadBody marks a message whose framing was fine but whose body was not
// valid JSON-RPC. The stream is still in sync, so the server can continue.
type errBadBody struct{ err error }

func (e *errBadBody) Error() string { return "parsing request: " + e.err.Error() }
func (e *errBadBody) Unwrap() error { return e.err }

type readResult struct {
	req *Request
	err error
}

// isOrdered reports whether a message must be processed synchronously in the
// read loop to preserve client ordering: all notifications (didOpen,
// didChange, ...) and lifecycle requests.
func isOrdered(req *Request) bool {
	if req.ID == nil {
		return true
	}
	switch req.Method {
	case "initialize", "shutdown", "exit":
		return true
	}
	return false
}

// Run reads and handles messages until EOF, error or ctx cancellation.
// Notifications and lifecycle messages are handled in order on the read loop;
// other requests run concurrently. Run waits for in-flight handlers before
// returning.
func (c *Conn) Run(ctx context.Context) error {
	defer c.wg.Wait()

	// Read in a separate goroutine so cancellation (e.g. "exit") returns
	// promptly even while blocked on the underlying reader.
	results := make(chan readResult)
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			req, err := c.readRequest()
			select {
			case results <- readResult{req, err}:
			case <-stop:
				return
			}
			var bad *errBadBody
			if err != nil && !errors.As(err, &bad) {
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case res := <-results:
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if res.err != nil {
				var bad *errBadBody
				if errors.As(res.err, &bad) {
					null := json.RawMessage("null")
					_ = c.writeResponse(&Response{
						JSONRPC: "2.0",
						ID:      &null,
						Error:   &ResponseError{Code: CodeParseError, Message: res.err.Error()},
					})
					continue
				}
				if errors.Is(res.err, io.EOF) {
					return nil
				}
				return fmt.Errorf("reading request: %w", res.err)
			}
			req := res.req
			if isOrdered(req) {
				c.handleRequest(ctx, req)
				continue
			}
			c.wg.Add(1)
			go func() {
				defer c.wg.Done()
				c.handleRequest(ctx, req)
			}()
		}
	}
}

func (c *Conn) readRequest() (*Request, error) {
	// Read headers
	contentLength := -1
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break // End of headers
		}

		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %w", err)
			}
			if n < 0 {
				return nil, fmt.Errorf("invalid Content-Length: %d is negative", n)
			}
			if n > maxMessageSize {
				return nil, fmt.Errorf("invalid Content-Length: %d exceeds limit of %d", n, maxMessageSize)
			}
			contentLength = n
		}
	}

	if contentLength <= 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	// Read body
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(c.reader, body); err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}

	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, &errBadBody{err}
	}

	return &req, nil
}

// safeHandle invokes the handler, converting a panic into an internal error.
func (c *Conn) safeHandle(ctx context.Context, req *Request) (result any, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic handling %s: %v\n%s", req.Method, r, debug.Stack())
			result = nil
			err = &ResponseError{Code: CodeInternalError, Message: fmt.Sprintf("internal error: %v", r)}
		}
	}()
	return c.handler.Handle(ctx, req)
}

func (c *Conn) handleRequest(ctx context.Context, req *Request) {
	result, err := c.safeHandle(ctx, req)

	// Notifications don't get responses
	if req.ID == nil {
		return
	}

	resp := Response{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	if err != nil {
		// Check for ResponseError first (preserves code)
		var rpcErr *ResponseError
		if errors.As(err, &rpcErr) {
			resp.Error = rpcErr
		} else {
			resp.Error = &ResponseError{
				Code:    CodeInternalError,
				Message: err.Error(),
			}
		}
	} else {
		resp.Result = result
	}

	if writeErr := c.writeResponse(&resp); writeErr != nil {
		log.Printf("failed to write response: %v", writeErr)
	}
}

func (c *Conn) writeResponse(resp *Response) error {
	body, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshaling response: %w", err)
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := c.rwc.Write([]byte(header)); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}
	if _, err := c.rwc.Write(body); err != nil {
		return fmt.Errorf("writing body: %w", err)
	}

	return nil
}

// Notify sends a notification to the client (no response expected).
func (c *Conn) Notify(ctx context.Context, method string, params any) error {
	req := Request{
		JSONRPC: "2.0",
		Method:  method,
	}

	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshaling params: %w", err)
		}
		req.Params = data
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling notification: %w", err)
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := c.rwc.Write([]byte(header)); err != nil {
		return err
	}
	if _, err := c.rwc.Write(body); err != nil {
		return err
	}

	return nil
}

// Close waits for in-flight request handlers to finish, then closes the
// underlying connection.
func (c *Conn) Close() error {
	c.wg.Wait()
	return c.rwc.Close()
}

// ErrMethodNotFound is returned when a method is not implemented.
var ErrMethodNotFound = &ResponseError{
	Code:    CodeMethodNotFound,
	Message: "method not found",
}
