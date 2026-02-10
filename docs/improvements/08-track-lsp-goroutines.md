# 08 — Track LSP request goroutines with `sync.WaitGroup`

## Category

Correctness / Resource management

## Effort

~30 minutes

## Files

- `internal/lsp/jsonrpc.go` — line 108

## Problem

The JSON-RPC `Run()` method spawns a goroutine per incoming request without
tracking them:

```go
go c.handleRequest(ctx, req)
```

When the connection closes or the server shuts down, in-flight handlers are
abandoned mid-execution. This can cause:

- Partial writes to shared state
- Resource leaks (open files, pending I/O)
- Race conditions during shutdown

## Fix

Add a `sync.WaitGroup` to the connection struct:

```go
type Conn struct {
    // ...
    wg sync.WaitGroup
}

// In Run():
c.wg.Add(1)
go func() {
    defer c.wg.Done()
    c.handleRequest(ctx, req)
}()

// In shutdown/close:
c.wg.Wait()
```

This ensures all handlers complete before the connection is torn down.

## Acceptance Criteria

- All request goroutines are tracked
- Shutdown waits for in-flight handlers to complete
- No goroutine leaks (verify with `goleak` or manual inspection)
- Existing LSP tests pass
