# 11 — Split `internal/lsp/server.go` into feature files

## Category

Maintainability

## Effort

~2 hours

## Files

- `internal/lsp/server.go` — 1,458 lines

## Problem

`server.go` is a monolithic file containing all LSP handler methods:

- Document lifecycle (didOpen, didChange, didClose)
- Hover, completion, definition, references
- Rename, formatting, document symbols
- Folding, semantic tokens, inlay hints
- Diagnostics, code actions

This makes the file hard to navigate and increases merge conflict likelihood.

## Fix

Split into files that mirror the LSP specification method grouping:

```
internal/lsp/
  server.go              — Server struct, NewServer, lifecycle, dispatch
  handle_textdocument.go — didOpen, didChange, didClose, didSave
  handle_hover.go        — handleHover
  handle_completion.go   — handleCompletion
  handle_definition.go   — handleDefinition, handleReferences
  handle_rename.go       — handleRename, handlePrepareRename
  handle_formatting.go   — handleFormatting
  handle_symbols.go      — handleDocumentSymbol
  handle_diagnostics.go  — publishDiagnostics, runDiagnostics
  handle_codelens.go     — handleCodeAction (if present)
```

All methods stay on the `*Server` receiver — this is purely a file
reorganization, not an API change.

## Acceptance Criteria

- `server.go` reduced to <300 lines (struct + lifecycle + dispatch)
- Each handler in its own file or grouped with closely related handlers
- Zero API changes — all methods remain on `*Server`
- All existing tests pass unchanged
- `go build ./internal/lsp/...` succeeds
