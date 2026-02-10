# 05 — Remove dead `AssignItem` interface

## Category

Cleanup / Idiomatic Go

## Effort

~5 minutes

## Files

- `internal/starlark/query/output/format.go`

## Problem

`AssignItem` embeds `Item` but adds zero new methods:

```go
type AssignItem interface {
    Item
}
```

This is a no-op abstraction. Any `Item` already satisfies `AssignItem`, so the
interface adds no type safety or semantic value. It increases the API surface
without benefit.

## Fix

1. Remove the `AssignItem` interface definition.
2. Replace any usage of `AssignItem` with `Item` in type assertions, switch
   cases, and function signatures.
3. Verify no external consumers depend on it (check `pkg/` and `examples/`).

## Acceptance Criteria

- `AssignItem` type is removed
- All references updated to use `Item`
- Existing tests pass
- No compilation errors
