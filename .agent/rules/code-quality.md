---
description: Go code quality, tests, and style
---

# Code Quality

## Go Style

- Use `gofmt` (run `make format`).
- Keep package names short and lowercase.
- Prefer table-driven tests for data-heavy cases.
- Wrap errors with context using `%w`.

## Testing

- Add tests for new behavior and bug fixes.
- A regression test must fail without the fix. Confirm it, do not assume it.
- Cover both success and failure paths, and malformed or hostile input for
  anything that parses protocol, config or files.
- Run `go test -race ./...` and `just test` (Bazel) before commit. They are not
  equivalent: Go tests can pass while a missing Bazel fixture fails CI.
- Concurrency code needs a test that exercises the shared state.
- Do not leave dead helpers or unused test scaffolding behind.

## Correctness traps in this codebase

- LSP positions are UTF-16 code units, not bytes or runes. Use the helpers in
  `internal/lsp/position.go`.
- File URIs are percent-encoded; convert with the shared URI helpers, never by
  slicing `file://`.
- Never type-assert on the generated `Or_*` union `Value` without handling
  every variant.
- Filesystem walks must not skip the root directory because its name starts
  with `.`.
- Exit codes: non-zero when a check finds problems or input is unusable.
  Silent success on "nothing to check" hides CI failures.
- Anything that writes a user-supplied output path should validate first and
  write atomically (temp file plus rename).
- Values interpolated into GitHub workflow commands or shell scripts must be
  escaped or passed through environment variables.

## Linting

- Run `just lint` before commit (CI also runs golangci-lint with
  `.golangci.toml`).
- After adding Go files or imports, run `just gazelle` so `BUILD.bazel` matches.
