// Package lsp implements a Language Server Protocol server for Starlark.
package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"sync"
)

// JSON-RPC 2.0 message types

// Request is a JSON-RPC request or notification.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"` // nil for notifications
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *ResponseError  `json:"error,omitempty"`
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
	CodeRequestCancelled = -32800
	CodeContentModified  = -32801
)

// Conn handles JSON-RPC communication over an io.ReadWriteCloser.
type Conn struct {
	rwc     io.ReadWriteCloser
	reader  *bufio.Reader
	writeMu sync.Mutex

	handler Handler
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

// Run reads and handles messages until EOF or error.
func (c *Conn) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := c.readRequest()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("reading request: %w", err)
		}

		// Handle in goroutine to allow concurrent requests
		go c.handleRequest(ctx, req)
	}
}

func (c *Conn) readRequest() (*Request, error) {
	// Read headers
	var contentLength int
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = line[:len(line)-1] // Remove \n
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1] // Remove \r
		}

		if line == "" {
			break // End of headers
		}

		// Parse Content-Length header
		if len(line) > 16 && line[:16] == "Content-Length: " {
			n, err := strconv.Atoi(line[16:])
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %w", err)
			}
			contentLength = n
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	// Read body
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(c.reader, body); err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}

	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("parsing request: %w", err)
	}

	return &req, nil
}

func (c *Conn) handleRequest(ctx context.Context, req *Request) {
	result, err := c.handler.Handle(ctx, req)

	// Notifications don't get responses
	if req.ID == nil {
		return
	}

	resp := Response{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	if err != nil {
		resp.Error = &ResponseError{
			Code:    CodeInternalError,
			Message: err.Error(),
		}
		// Check for specific error types
		if rpcErr, ok := err.(*ResponseError); ok {
			resp.Error = rpcErr
		}
	} else {
		resp.Result = result
	}

	c.writeResponse(&resp)
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

// Close closes the underlying connection.
func (c *Conn) Close() error {
	return c.rwc.Close()
}

// ErrMethodNotFound is returned when a method is not implemented.
var ErrMethodNotFound = &ResponseError{
	Code:    CodeMethodNotFound,
	Message: "method not found",
}
