# Nogo Analyzer Spec

This document defines the analyzers we enable for Sky. The list is curated for
high signal and low noise. We favor analyzers that catch real bugs or enforce
team conventions with minimal false positives.

## Built-in (vet = true)

Enabled automatically via `vet = True` in `tools/nogo:sky_nogo`. This is the
standard `go vet` analyzer set (see `go tool vet help` for the full list).

## Additional analyzers

These are added explicitly on top of `vet = True`:

- `nilness` — nil dereference paths.
- `unusedwrite` — writes to variables that are never read.
- `bodyclose` — ensures HTTP response bodies are closed.
- `errcheck` — unchecked error returns.

## Dependency tracking

We keep analyzer module dependencies pinned in `internal/tooldeps` so
`go mod tidy` does not drop them while Bazel still depends on them.
