# 16 — Create `internal/testutil` package

## Category

Developer experience

## Effort

~2–3 hours

## Files

- New: `internal/testutil/testutil.go`
- Consumers: ~25 test files across the codebase

## Problem

No shared test utilities exist. Each package re-invents:

- Temporary directory creation and cleanup
- Golden file comparison
- Starlark source parsing for test input
- Assertion helpers for common patterns
- Fixture loading

This leads to duplicated setup code and inconsistent test patterns.

## Proposed API

```go
package testutil

// TempDir creates a temp directory populated with the given files.
// Files are specified as path → content pairs. Cleanup is automatic.
func TempDir(t testing.TB, files map[string]string) string

// Golden compares got against a golden file, updating it if -update flag is set.
func Golden(t testing.TB, name string, got []byte)

// ParseStarlark parses a Starlark source string and fails the test on error.
func ParseStarlark(t testing.TB, filename, source string) *syntax.File

// RequireError asserts that err is non-nil and contains substr.
func RequireError(t testing.TB, err error, substr string)

// RequireNoError asserts that err is nil, failing with the error message.
func RequireNoError(t testing.TB, err error)
```

## Approach

1. Start small — only add helpers that are already duplicated in 3+ test files.
2. Use `testing.TB` (not `*testing.T`) so helpers work in benchmarks too.
3. Call `t.Helper()` in every function for correct line reporting.
4. No external dependencies (no testify, no gomega).

## Acceptance Criteria

- Package created at `internal/testutil/`
- At least 3 existing test files refactored to use the helpers
- All helpers call `t.Helper()`
- `go test ./internal/testutil/...` passes
- Helpers are themselves tested
