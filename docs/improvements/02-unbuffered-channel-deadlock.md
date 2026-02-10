# 02 — Replace unbuffered channel with `sync.WaitGroup` in index test

## Category

Bug fix / Correctness

## Effort

~5 minutes

## Files

- `internal/starlark/query/index/index_test.go` — lines 341–354

## Problem

The concurrent access test spawns 10 goroutines that each send on an unbuffered
`chan bool`. If any goroutine panics before sending, the receiving loop
deadlocks and the test hangs forever:

```go
done := make(chan bool) // unbuffered
for i := 0; i < 10; i++ {
    go func() {
        _ = idx.Files()
        _ = idx.Count()
        _ = idx.Get("test0.bzl")
        _ = idx.MatchFiles("//...")
        done <- true
    }()
}
for i := 0; i < 10; i++ {
    <-done
}
```

## Fix

Replace with `sync.WaitGroup`:

```go
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        _ = idx.Files()
        _ = idx.Count()
        _ = idx.Get("test0.bzl")
        _ = idx.MatchFiles("//...")
    }()
}
wg.Wait()
```

Alternatively, buffer the channel: `make(chan bool, 10)`.

## Acceptance Criteria

- No unbuffered channel in the concurrent test
- Test still exercises concurrent access with 10 goroutines
- `go test ./internal/starlark/query/index/...` passes
