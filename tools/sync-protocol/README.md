# sync-protocol

Extracts LSP protocol types from [gopls](https://github.com/golang/tools/tree/master/gopls).

## Why?

The `go.lsp.dev/protocol` package only supports LSP 3.15.3, but we need LSP 3.17+ types like `InlayHint`.

gopls (the official Go language server) generates its protocol types from the [LSP metaModel.json specification](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/). Since gopls uses an `internal` package, we can't import it directly.

This tool extracts the specific types we need.

## Usage

```bash
# Extract types (clones gopls to temp dir)
go run ./tools/sync-protocol

# Use existing golang/tools checkout
go run ./tools/sync-protocol -gopls-dir=/path/to/golang/tools

# Preview without writing
go run ./tools/sync-protocol -dry-run

# Extract specific types
go run ./tools/sync-protocol -types=InlayHint,InlayHintKind,InlayHintParams

# Verbose output
go run ./tools/sync-protocol -verbose
```

## Options

| Flag         | Default                          | Description                     |
| ------------ | -------------------------------- | ------------------------------- |
| `-gopls-dir` | (temp clone)                     | Path to golang/tools repo       |
| `-output`    | `internal/lsp/protocol_types.go` | Output file                     |
| `-types`     | `InlayHint,InlayHintKind,...`    | Types to extract                |
| `-dry-run`   | false                            | Print output instead of writing |
| `-verbose`   | false                            | Verbose logging                 |

## Default Types Extracted

- `InlayHint` - Inlay hint structure
- `InlayHintKind` - Type or Parameter hint
- `InlayHintParams` - Request parameters
- `InlayHintLabelPart` - Label parts for complex hints
- `InlayHintOptions` - Server capability options

## Adding New Types

To extract additional LSP 3.17+ types:

```bash
go run ./tools/sync-protocol -types=InlayHint,InlayHintKind,TypeHierarchyItem
```

## How It Works

1. Clones golang/tools repo (sparse checkout of just `gopls/internal/protocol`)
2. Parses Go source files using `go/ast`
3. Extracts requested type declarations and related constants
4. Generates a standalone Go file with proper imports
5. Post-processes to adapt types for our codebase

## Updating Protocol Types

When a new LSP version is released:

1. Run `go run ./tools/sync-protocol -verbose` to update
2. Review generated `internal/lsp/protocol_types.go`
3. Test: `go test ./internal/lsp/...`
4. Commit the changes

## Future: Removing go.lsp.dev/protocol

Once we extract all needed types, we could potentially remove the `go.lsp.dev/protocol` dependency entirely and use only extracted types. This would:

- Give us full control over LSP version support
- Reduce dependency on unmaintained external packages
- Match gopls's approach

To evaluate this, run:

```bash
grep -r "go.lsp.dev/protocol" internal/lsp/*.go | wc -l
```
