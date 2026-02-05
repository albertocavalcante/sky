# AGENTS.md

Sky - Go monorepo for Starlark tooling.

## Setup (Run Once After Clone)

```bash
just setup     # Install git hooks (lefthook) and verify tools
```

## Commands

Prefer `just` (modern command runner). `make` also works for compatibility.

```bash
# Using just (preferred)
just setup     # ONE-TIME: Install git hooks
just build     # Build all CLI tools
just test      # Run all tests
just lint      # Run linter (nogo via bazel)
just format    # Format all Go files
just gazelle   # Update BUILD.bazel files
just tidy      # Tidy go modules
just check     # Run format + lint + test (CI)
just tool X    # Build specific tool (e.g., just tool skylint)
just run X     # Run specific tool (e.g., just run skylint -- --help)
just           # Show all available recipes

# Using make (legacy, still works)
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
