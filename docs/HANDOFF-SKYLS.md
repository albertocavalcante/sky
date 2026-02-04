# skyls LSP Server - Handoff Document

**Date:** 2026-02-04
**Status:** In Progress - Formatting Complete

## Overview

`skyls` is the Language Server Protocol (LSP) implementation for the Sky Starlark toolchain. It provides IDE integration by exposing existing Sky tools via the LSP protocol.

## Architecture

```
cmd/skyls/main.go                 # Entry point
internal/cmd/skyls/run.go         # CLI handling, stdio server setup
internal/lsp/
  ├── jsonrpc.go                  # Custom JSON-RPC 2.0 implementation (~200 lines)
  ├── jsonrpc_test.go             # JSON-RPC tests
  ├── server.go                   # LSP method routing & handlers
  └── server_test.go              # Server tests (lifecycle, sync, formatting)
```

## Design Decisions

1. **Custom JSON-RPC instead of go.lsp.dev/jsonrpc2**
   - Keeps dependencies minimal
   - ~200 lines, handles stdio framing (Content-Length)
   - Simple method routing via switch statement (no FallbackServer boilerplate)

2. **Protocol types from go.lsp.dev/protocol**
   - 100+ LSP types - not practical to write by hand
   - Well-maintained, generated from official LSP spec

3. **Reference implementation studied:** `/Users/adsc/dev/refs/starlark-lsp` (tilt-dev)

## Current State

### Completed ✅

| Feature                                | Handler | Integration                   |
| -------------------------------------- | ------- | ----------------------------- |
| Initialize/Shutdown/Exit               | ✅      | -                             |
| Document sync (open/change/close/save) | ✅      | -                             |
| **Formatting**                         | ✅      | `internal/starlark/formatter` |

### Stubbed (TODO) 🚧

| Feature          | Handler                | Integration Needed                     |
| ---------------- | ---------------------- | -------------------------------------- |
| Hover            | `handleHover`          | `internal/starlark/docgen` (skydoc)    |
| Go to Definition | `handleDefinition`     | `internal/starlark/query` (skyquery)   |
| Document Symbols | `handleDocumentSymbol` | `internal/starlark/query` (skyquery)   |
| Completion       | `handleCompletion`     | builtins + loaded symbols              |
| Diagnostics      | on didSave             | `internal/starlark/linter` + `checker` |

### Not Started ❌

- Code Actions (skylint --fix)
- Find References
- Rename
- Signature Help

## Key Files to Understand

1. **internal/lsp/server.go:Handle()** - Main request router (line ~49)
2. **internal/lsp/server.go:handleFormatting()** - Example of tool integration (line ~310)
3. **internal/starlark/formatter/formatter.go:Format()** - Formatter API
4. **internal/starlark/query/** - Query API for definition/symbols
5. **internal/starlark/docgen/** - Documentation API for hover

## Next Steps (Priority Order)

1. **Diagnostics on save** - Integrate skylint + skycheck
   - Call linter on `didSave`
   - Use `conn.Notify()` to publish diagnostics
   - See `protocol.PublishDiagnosticsParams`

2. **Document Symbols** - Integrate skyquery
   - Look at `internal/starlark/query/` for symbol extraction
   - Return `[]protocol.DocumentSymbol`

3. **Hover** - Integrate skydoc
   - Look at `internal/starlark/docgen/`
   - Return `protocol.Hover` with markdown content

4. **Go to Definition** - Integrate skyquery
   - Need to resolve symbol at position to its definition location
   - Return `[]protocol.Location`

## Testing

```bash
# Run all LSP tests
go test ./internal/lsp/... -v

# Build the server
go build ./cmd/skyls

# Test manually (will wait for JSON-RPC input on stdin)
./skyls -v
```

## Editor Testing

To test with an editor:

1. Build: `go build -o skyls ./cmd/skyls`
2. Configure editor to use `./skyls` as LSP server for `.star`, `.bzl` files
3. Open a Starlark file and try formatting (should work)

### VS Code

```json
// settings.json
{
  "starlark.lsp.path": "/path/to/skyls"
}
```

### Neovim (nvim-lspconfig)

```lua
require('lspconfig').starlark_rust.setup{
  cmd = { '/path/to/skyls' },
  filetypes = { 'star', 'bzl', 'bazel' },
}
```

## Code Quality Notes

- Uses `strings.CutPrefix` (Go 1.20+)
- Uses `errors.As` for error type checking
- Proper mutex usage for document map
- Graceful error handling (returns empty results instead of errors to editor)

## Dependencies Added

```
go.lsp.dev/protocol v0.12.0  # LSP types
go.lsp.dev/uri v0.3.0        # URI handling (transitive)
```

## Git Log (Recent)

```
eca8b33 feat(skyls): integrate skyfmt for document formatting
a59f342 refactor(skyls): improve code quality and add server tests
f4ca546 feat(skyls): scaffold LSP server with custom JSON-RPC
8cb6abe refactor(skycov): use html/template for HTML reporter
6a0e1c6 feat(skycov): add HTML coverage reporter
d88ac34 fix: update golangci-lint config for v2 schema
```
