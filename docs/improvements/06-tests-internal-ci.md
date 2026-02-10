# 06 — Add tests for `internal/ci` package

## Category

Test coverage

## Effort

~1–2 hours

## Files

- `internal/ci/` — 442 LOC across 4 Go files, 0 test files

## Problem

The `internal/ci` package handles CI system detection (GitHub Actions, GitLab CI,
CircleCI, Azure Pipelines, Jenkins) and test result reporting. It has zero test
coverage.

This code formats output for external systems — subtle formatting bugs can break
CI integrations silently.

## What to Test

1. **CI system detection** — `detectSystem()` should correctly identify each CI
   based on environment variables. Test with `t.Setenv()`.
2. **Result reading** — `readResults()` should handle valid results, empty
   results, and malformed input.
3. **Handler dispatch** — each CI handler should produce correctly formatted
   output for its platform.
4. **`Run()` orchestration** — integration-style test for the top-level
   function.

## Approach

Use table-driven tests grouped by CI system. Mock environment variables using
`t.Setenv()`. Use golden files or inline expected output for format validation.

## Acceptance Criteria

- At least one test per CI system handler
- Detection logic tested for all supported CI systems
- Error paths tested (unknown CI, malformed results)
- `go test ./internal/ci/...` passes
