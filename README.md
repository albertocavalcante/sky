# sky

Go monorepo for Starlark tooling. The initial focus is on CLI tools that work
with Starlark sources in Bazel and related ecosystems.

## Tools

- `sky`: Plugin-first CLI and plugin manager.
- `skyfmt`: Starlark formatter (planned).
- `skylint`: Starlark linter (planned).
- `skyquery`: Starlark query helper (planned).

## Plugin-first CLI

`sky` resolves unknown commands to installed plugins, so feature development
leans toward plugins first and core commands second. See `docs/PLUGINS.md` for
details and marketplace format. The protocol is in `docs/PLUGIN_PROTOCOL.md`.

## Layout

- `cmd/`: CLI entrypoints.
- `internal/`: Shared, non-public packages.
- `docs/`: Design notes and roadmap.

## Development

```bash
make gazelle
make build
make test
make lint
make format
```

Build artifacts are written to `bazel-bin/`.
