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

Work on a branch and open a pull request. `main` is protected and changes land
through PRs. See `.agent/rules/pull-requests.md` for the definition of done and
`.agent/rules/git-workflow.md` for branch and commit rules.

```bash
git switch main
git pull origin main
git switch -c fix/example
# make focused changes, with a regression test that fails first
go test -race ./...
git diff --check
git diff
git add path/to/file.go path/to/test.go
git commit -m "fix(scope): describe change"
```

Review the diff before staging. Stage files explicitly. Keep PRs small and
focused; they are squash-merged.

## Snapshot builds

Run the Snapshot workflow from protected `main`. Leave `ref` as `main` to build
the commit captured by that workflow event, or enter a full lowercase commit SHA
already in that commit's history. Branch names, tags, SHA expressions and commits
outside that history are refused before authentication or builds.

Every platform build and its summary use the same validated commit and version.
Snapshot builds fetch private modules with short-lived authentication into a
fresh module cache; they do not depend on a previous runner's cached sources.
Artifacts include per-platform checksums. Cross-compilation does not establish
native test coverage for each target platform.
