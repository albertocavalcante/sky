# 03 — Pre-allocate suggestions slice in `findSimilarCommands()`

## Category

Idiomatic Go / Minor performance

## Effort

~5 minutes

## Files

- `cmd/sky/main.go` — lines 532–560

## Problem

`findSimilarCommands()` builds a `[]commandSuggestion` via unbounded `append`.
There are only ~8 core commands, so the slice can never exceed that size, but Go
will allocate and copy on each growth step:

```go
var suggestions []commandSuggestion
// ...
suggestions = append(suggestions, commandSuggestion{...})
```

## Fix

Pre-allocate with the known upper bound:

```go
suggestions := make([]commandSuggestion, 0, len(coreCommands))
```

This is a single-line change that avoids intermediate allocations on the error
path.

## Acceptance Criteria

- Slice is pre-allocated with capacity equal to `len(coreCommands)`
- No functional change — same output for all inputs
- Existing tests pass
