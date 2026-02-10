# 01 — Handle `filepath.Abs()` errors in watcher

## Category

Bug fix / Correctness

## Effort

~5 minutes

## Files

- `internal/starlark/tester/watcher.go` — lines 219, 284, 336

## Problem

Three call sites silently discard the error from `filepath.Abs()` using the
blank identifier:

```go
absPath, _ := filepath.Abs(resolved)   // line 219
absPath, _ := filepath.Abs(event.Name) // line 284
absPath, _ := filepath.Abs(file)       // line 336
```

`filepath.Abs()` can fail (e.g. `os.Getwd()` failure inside it). When it does,
`absPath` is empty, and that empty string propagates downstream — potentially
matching nothing or, worse, matching the wrong thing.

## Fix

Check and propagate the error at each call site. For example:

```go
absPath, err := filepath.Abs(resolved)
if err != nil {
    return "", fmt.Errorf("resolve absolute path: %w", err)
}
```

Adapt the return / error handling to the surrounding function signature at each
of the three locations.

## Acceptance Criteria

- All three `filepath.Abs` calls check and handle errors
- No `_ :=` pattern remains for `filepath.Abs` in the file
- Existing tests still pass
