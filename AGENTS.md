# AGENTS.md

Sky - Go monorepo for Starlark tooling.

## Commands

```bash
make build   # Build all tools
make test    # Run tests
make lint    # Run Bazel build (nogo)
make format  # Run gofmt
make gazelle # Generate/update BUILD.bazel files
```

## Rules (ALWAYS APPLY)

1. Git: `git branch --show-current` before commit. `git add <file>` explicitly, never `git add .`.
2. Formatting: run `make format` before commit.
3. Lint: run `make lint` before commit.

## READ BEFORE ACTING

| IF task involves...            | THEN read                      |
| ------------------------------ | ------------------------------ |
| Creating branch, committing PR | `.agent/rules/git-workflow.md` |
| Writing tests, naming, style   | `.agent/rules/code-quality.md` |

## Repository Layout

| Path        | Purpose                        |
| ----------- | ------------------------------ |
| `cmd/`      | CLI entrypoints for tools      |
| `internal/` | Shared, non-public Go packages |
| `docs/`     | Design notes and roadmap       |

## Helper Files

| Path            | Contains        |
| --------------- | --------------- |
| `.agent/rules/` | Permanent rules |
