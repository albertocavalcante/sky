# Contributing

Prefer small patches with clear behavior.

## Development

```bash
just setup
just format
just lint
just test
```

Go-only iteration is also supported:

```bash
go test ./...
```

## Versioning

The project is not using release tags yet. Builds are identified by immutable
commit metadata:

```text
v0.0.0-YYYYMMDDHHMMSS-<commit12>
```

Use commit hashes for reproducible installs:

```bash
go install github.com/albertocavalcante/sky/cmd/sky@<commit-sha>
```

## Local Review Flow

- Keep changes focused.
- Add or update tests for behavior changes.
- Update docs when user-facing behavior changes.
- Do not commit generated binaries or local build outputs.

Use local branches and squash merges:

```bash
git switch main
git switch -c chore/example
# make focused changes
go test ./...
git diff --check
git diff
git add path/to/file.go path/to/test.go
git commit -m "chore(scope): describe change"
git switch main
git merge --squash chore/example
git commit -m "chore(scope): describe change"
```

Review the diff before staging. Stage files explicitly. Keep the final main
commit small.
