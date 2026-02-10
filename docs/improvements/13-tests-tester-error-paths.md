# 13 — Add error-path tests to `internal/starlark/tester`

## Category

Test coverage

## Effort

~2 hours

## Files

- `internal/starlark/tester/` — 3,543 LOC, 34 tests (mostly happy-path)

## Problem

The tester package is the largest in the codebase but has thin test coverage,
especially for error paths. Only ~11 references to "error" in 1,246 lines of
test code.

### Missing Error Path Coverage

1. **`loadPreludes()`** — What happens when a prelude file doesn't exist? When
   it has a syntax error? When it redefines a builtin?
2. **`resolveFixtureArgs()`** — Missing fixture files, type mismatches,
   circular references.
3. **Mock manager errors** — Mock setup failures, mock verification failures.
4. **Snapshot serialization** — Invalid snapshot format, snapshot file
   permissions, snapshot update mode.
5. **Discovery errors** — Invalid glob patterns, permission-denied on
   directories, symlink loops.
6. **Watcher errors** — File system notification failures, rapid file changes.

## Approach

For each error category, write 2–3 table-driven tests that exercise the failure
mode and verify the error message / behavior.

Focus on errors that would be confusing to users if they hit them in practice.

## Acceptance Criteria

- At least 2 tests per error category listed above
- Error messages verified (not just "it returns an error")
- `go test ./internal/starlark/tester/...` passes
- No test pollution (tests clean up temp files)
