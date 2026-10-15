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
	"time"
)

// JSON-RPC 2.0 message types

// Request is a JSON-RPC request or notification.
type Request struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"` // nil for notifications
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`

	// snapshot is set by a requestPreparer on the read loop, before the
	// request is dispatched, and is private to the handler.
	snapshot any
}

// requestPreparer is implemented by handlers that need to capture state at
// the moment a request is received. PrepareRequest runs on the read loop,
// before the request is dispatched to its goroutine, so it observes every
// earlier notification (didOpen, didChange, ...) and no later one.
type requestPreparer interface {
	PrepareRequest(req *Request)
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
	rwc    io.ReadWriteCloser
	reader *bufio.Reader

	// Outbound messages are queued and written by a single drain goroutine
	// so handlers (which run on the read loop for ordered messages) never
	// block on a client that is not reading its end of the transport.
	outMu    sync.Mutex
	outCond  *sync.Cond
	outQueue [][]byte
	outBytes int   // total size of outQueue
	writing  bool  // a drain goroutine is running
	outFail  error // first fatal output error; later output is discarded
	cancelFn context.CancelFunc

	closeOnce sync.Once
	closeErr  error

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
	c := &Conn{
		rwc:     rwc,
		reader:  bufio.NewReader(rwc),
		handler: handler,
	}
	c.outCond = sync.NewCond(&c.outMu)
	return c
}

// flushGrace bounds how long Run waits, on return, for queued output to reach
// a client that may have stopped reading.
const flushGrace = 500 * time.Millisecond

// readerJoinGrace bounds how long Run waits for the reader goroutine after
// closing the transport.
const readerJoinGrace = 200 * time.Millisecond

// Outbound queue limits. A client that stops reading while still sending
// requests would otherwise grow the queue without bound; exceeding either
// cap terminates the connection instead.
const (
	maxQueuedMessages = 10000
	maxQueuedBytes    = 64 << 20 // 64 MiB
)

// ErrOutput is wrapped by errors reporting that outbound messages could not
// be delivered (client overloaded, not reading, or transport write failure).
var ErrOutput = errors.New("lsp output failed")

// maxMessageSize caps the Content-Length we are willing to allocate.
const maxMessageSize = 64 << 20 // 64 MiB

// Header limits. Legitimate headers are a few dozen bytes; anything larger
// is hostile or corrupt, and the stream cannot be resynchronised.
const (
	maxHeaderLine  = 8 << 10  // one header line, including its terminator
	maxHeaderBytes = 64 << 10 // all header lines of one message
)

// errHeaderTooLarge is returned when a header line or block exceeds its cap.
var errHeaderTooLarge = errors.New("header exceeds size limit")

// readHeaderLine reads one line, failing with errHeaderTooLarge as soon as
// it holds more than limit bytes rather than buffering it whole.
func (c *Conn) readHeaderLine(limit int) (string, error) {
	var line []byte
	for {
		chunk, err := c.reader.ReadSlice('\n')
		line = append(line, chunk...)
		if len(line) > limit {
			return "", fmt.Errorf("reading header: %w (limit %d bytes)", errHeaderTooLarge, limit)
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		return string(line), err
	}
}

// errBadBody marks a message whose framing was fine but whose body was not
// valid JSON-RPC. The stream is still in sync, so the server can continue.
type errBadBody struct {
	err  error
	code int // CodeParseError or CodeInvalidRequest
}

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
// other requests run concurrently. On return Run cancels the context passed
// to handlers, then waits for in-flight handlers.
func (c *Conn) Run(parent context.Context) (err error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	c.outMu.Lock()
	c.cancelFn = cancel
	c.outMu.Unlock()

	// Read in a separate goroutine so cancellation (e.g. "exit") returns
	// promptly even while blocked on the underlying reader.
	results := make(chan readResult)
	stop := make(chan struct{})
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
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

	defer func() {
		close(stop)
		// Cancel before waiting: a handler blocked on ctx.Done() would
		// otherwise keep Run (and so EOF handling) from ever returning.
		cancel()
		c.wg.Wait()
		if !c.flush(flushGrace) {
			// Pending output could not be delivered: interrupt the blocked
			// write where the transport allows it, and report the failure.
			_ = c.closeTransport()
		}
		if ferr := c.outputErr(); ferr != nil {
			err = ferr
		}
		select {
		case <-readerDone:
		default:
			// Reader is blocked in the transport: interrupt it by closing
			// the transport, then join it (bounded).
			_ = c.closeTransport()
			select {
			case <-readerDone:
			case <-time.After(readerJoinGrace):
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
						Error:   &ResponseError{Code: bad.code, Message: res.err.Error()},
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
			if p, ok := c.handler.(requestPreparer); ok {
				p.PrepareRequest(req)
			}
			c.wg.Add(1)
			go func() {
				defer c.wg.Done()
				c.handleRequest(ctx, req)
			}()
		}
	}
}

// readRequest returns the next request or notification. Messages that are
// well-formed responses from the client are ignored (the server never sends
// requests, so there is nothing to correlate them with).
func (c *Conn) readRequest() (*Request, error) {
	for {
		req, err := c.readMessage()
		if err != nil || req != nil {
			return req, err
		}
	}
}

// readMessage reads one frame. It returns (nil, nil) for a client response.
func (c *Conn) readMessage() (*Request, error) {
	// Read headers
	contentLength := -1
	sawHeader := false
	headerBytes := 0
	for {
		line, err := c.readHeaderLine(min(maxHeaderLine, maxHeaderBytes-headerBytes))
		headerBytes += len(line)
		if err != nil {
			if errors.Is(err, io.EOF) && (sawHeader || line != "") {
				// EOF inside a header block is a truncated frame, not a
				// clean end of stream.
				return nil, fmt.Errorf("reading header: %w", io.ErrUnexpectedEOF)
			}
			return nil, err
		}
		sawHeader = true

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
		if errors.Is(err, io.EOF) {
			err = io.ErrUnexpectedEOF // EOF inside a declared Content-Length
		}
		return nil, fmt.Errorf("reading body: %w", err)
	}

	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		code := CodeParseError
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			// Well-formed JSON that is not a valid Request object.
			code = CodeInvalidRequest
		}
		return nil, &errBadBody{err: err, code: code}
	}

	// Validate required members. Raw members are used so that an explicit
	// null (which decodes to the zero value) is distinguishable from absence.
	var probe struct {
		ID     json.RawMessage `json:"id"`
		Method json.RawMessage `json:"method"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	_ = json.Unmarshal(body, &probe)
	invalid := func(format string, args ...any) (*Request, error) {
		return nil, &errBadBody{err: fmt.Errorf(format, args...), code: CodeInvalidRequest}
	}
	if len(probe.Method) == 0 && (len(probe.Result) > 0 || len(probe.Error) > 0) {
		return nil, nil // a response, not a request
	}
	if req.JSONRPC != "2.0" {
		return invalid("invalid request: jsonrpc must be \"2.0\"")
	}
	if len(probe.Method) == 0 || probe.Method[0] != '"' || req.Method == "" {
		return invalid("invalid request: method must be a non-empty string")
	}
	if len(probe.ID) > 0 {
		if ch := probe.ID[0]; ch != '"' && ch != '-' && (ch < '0' || ch > '9') {
			return invalid("invalid request: id must be a string or number")
		}
		id := probe.ID
		req.ID = &id
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
	c.enqueue(body)
	return nil
}

// Notify sends a notification to the client (no response expected). It queues
// the message and never blocks on the client reading it.
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
	c.enqueue(body)
	return nil
}

// enqueue adds a message body to the outbound queue, starting the drain
// goroutine if none is running. Messages are written in enqueue order. It
// never blocks: when the queue exceeds its caps the connection is terminated
// and the message dropped.
func (c *Conn) enqueue(body []byte) {
	c.outMu.Lock()
	defer c.outMu.Unlock()
	if c.outFail != nil {
		return
	}
	if len(c.outQueue) >= maxQueuedMessages || c.outBytes+len(body) > maxQueuedBytes {
		c.failLocked(fmt.Errorf("%w: client is not reading (%d messages, %d bytes queued)", ErrOutput, len(c.outQueue), c.outBytes))
		return
	}
	c.outQueue = append(c.outQueue, body)
	c.outBytes += len(body)
	if !c.writing {
		c.writing = true
		go c.drain()
	}
}

// failLocked records a fatal output error, drops queued output, cancels the
// Run context and closes the transport (from another goroutine, since Close
// may be what unblocks a stuck write). Callers hold outMu.
func (c *Conn) failLocked(err error) {
	if c.outFail != nil {
		return
	}
	c.outFail = err
	c.outQueue = nil
	c.outBytes = 0
	cancel := c.cancelFn
	log.Printf("terminating connection: %v", err)
	go func() {
		if cancel != nil {
			cancel()
		}
		_ = c.closeTransport()
	}()
	c.outCond.Broadcast()
}

// outputErr returns the fatal output error, if any.
func (c *Conn) outputErr() error {
	c.outMu.Lock()
	defer c.outMu.Unlock()
	return c.outFail
}

func (c *Conn) drain() {
	for {
		c.outMu.Lock()
		if len(c.outQueue) == 0 {
			c.writing = false
			c.outCond.Broadcast()
			c.outMu.Unlock()
			return
		}
		body := c.outQueue[0]
		c.outQueue[0] = nil
		c.outQueue = c.outQueue[1:]
		c.outBytes -= len(body)
		c.outMu.Unlock()

		header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
		if _, err := c.rwc.Write(append([]byte(header), body...)); err != nil {
			c.outMu.Lock()
			c.failLocked(fmt.Errorf("%w: writing message: %v", ErrOutput, err))
			c.outMu.Unlock()
		}
	}
}

// flush waits until all queued output has been written or grace elapses. It
// reports whether everything was delivered; on timeout the connection is
// marked failed.
func (c *Conn) flush(grace time.Duration) bool {
	timedOut := false
	t := time.AfterFunc(grace, func() {
		c.outMu.Lock()
		timedOut = true
		c.outCond.Broadcast()
		c.outMu.Unlock()
	})
	defer t.Stop()
	c.outMu.Lock()
	defer c.outMu.Unlock()
	for c.writing && c.outFail == nil && !timedOut {
		c.outCond.Wait()
	}
	if c.outFail == nil && c.writing {
		c.failLocked(fmt.Errorf("%w: pending output was not flushed within %v", ErrOutput, grace))
	}
	return c.outFail == nil
}

func (c *Conn) closeTransport() error {
	c.closeOnce.Do(func() { c.closeErr = c.rwc.Close() })
	return c.closeErr
}

// Close waits for in-flight request handlers to finish, then closes the
// underlying connection.
func (c *Conn) Close() error {
	c.wg.Wait()
	flushed := c.flush(flushGrace)
	err := c.closeTransport()
	if !flushed {
		return errors.Join(c.outputErr(), err)
	}
	return err
}

// ErrMethodNotFound is returned when a method is not implemented.
var ErrMethodNotFound = &ResponseError{
	Code:    CodeMethodNotFound,
	Message: "method not found",
}
