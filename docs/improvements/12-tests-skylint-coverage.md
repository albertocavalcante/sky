# 12 — Expand test coverage for `internal/cmd/skylint`

## Category

Test coverage

## Effort

~2 hours

## Files

- `internal/cmd/skylint/` — 14 exported functions, only 5 test functions

## Problem

The linter driver package exposes `LoadConfig`, `NewDriver`, `ApplyFixes`,
`FixFiles`, `WriteFixResults`, `NewRegistry`, 3 reporter constructors,
`SuppressionParser`, and `FilterSuppressed` — but has only 5 test functions.

Key untested areas:

- `ApplyFixes` / `FixFiles` / `WriteFixResults` — the auto-fix pipeline
- `LoadConfig` error paths — missing file, invalid JSON, unknown fields
- Reporter output formatting
- Suppression filtering with edge cases

## What to Test

1. **Fix pipeline**: Apply fixes to a sample file, verify output. Test
   overlapping fixes, empty fixes, fixes that expand/shrink content.
2. **Config loading**: Valid config, missing file (should use defaults),
   malformed JSON, unknown fields.
3. **Reporters**: Text, GitHub, JSON reporters produce expected output for a
   known result set.
4. **Suppression**: Filter findings with inline suppressions, regex
   suppressions, nested suppressions.

## Approach

Table-driven tests for each function. Use `testdata/` directory with sample
Starlark files for fix testing.

## Acceptance Criteria

- At least one test per exported function
- Fix pipeline tested end-to-end
- Error paths for config loading tested
- `go test ./internal/cmd/skylint/...` passes
