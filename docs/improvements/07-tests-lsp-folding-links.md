# 07 — Add tests for LSP `folding.go` and `links.go`

## Category

Test coverage

## Effort

~1 hour

## Files

- `internal/lsp/folding.go` — ~110 LOC, 0 tests
- `internal/lsp/links.go` — ~109 LOC, 0 tests

## Problem

Both files implement LSP features with zero test coverage:

- **`folding.go`** — Handles `textDocument/foldingRange` requests. Silently
  returns empty on parse error (line ~27). Folding logic for functions, if/for
  blocks, and load groups is untested.

- **`links.go`** — Handles `textDocument/documentLink` requests. Contains
  `resolveLoadPath()` (~38 lines) which resolves `@`-prefixed external repo
  paths, relative paths, and `//`-prefixed workspace paths. This logic is
  fragile and path-handling bugs are common.

## What to Test

### `folding.go`

- Simple function definition → one folding range
- Nested blocks (if inside def) → multiple ranges
- File with parse errors → empty result (not a crash)
- Load statement groups → folded together

### `links.go`

- Relative load path → resolved to file URI
- `@repo//pkg:file.bzl` → handled correctly
- `//pkg:file.bzl` → workspace-relative resolution
- Missing/nonexistent target → no link (not a crash)
- `resolveLoadPath()` unit tests for each path format

## Acceptance Criteria

- At least 4 test cases per file
- Parse error edge cases covered
- `go test ./internal/lsp/...` passes
