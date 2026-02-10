# 09 — Unify Reporter interfaces across packages

## Category

Idiomatic Go / API consistency

## Effort

~1–2 hours

## Files

- `internal/starlark/linter/reporter.go` — `Report(w io.Writer, result *Result) error`
- `internal/starlark/tester/reporter.go` — `ReportFile(w io.Writer, result *FileResult)`
- `internal/starlark/coverage/reporter.go` — `Write(w io.Writer, report *Report) error`

## Problem

Three packages define nearly-identical reporter interfaces with inconsistent
method names and signatures:

| Package    | Method         | Returns error? |
| ---------- | -------------- | -------------- |
| `linter`   | `Report()`     | Yes            |
| `tester`   | `ReportFile()` | No             |
| `coverage` | `Write()`      | Yes            |

This makes the codebase harder to navigate and prevents writing generic
reporting utilities.

## Options

### Option A: Align method names (minimal change)

Rename all to `Report(w io.Writer, result T) error` where T is the
package-specific result type. Add error return to `tester.Reporter`.

### Option B: Generic shared interface

```go
// internal/report/reporter.go
type Reporter[T any] interface {
    Report(w io.Writer, result T) error
}
```

Each package type-aliases or embeds this interface.

### Option C: Keep separate but align naming

Just rename `Write` → `Report` and `ReportFile` → `Report`, keeping interfaces
in their own packages but with consistent naming.

## Recommendation

Option A or C — minimal disruption, maximum consistency. Option B adds a
generic dependency that may not be worth it.

## Acceptance Criteria

- All three reporter interfaces use the same method name
- All return `error`
- Existing callers updated
- Tests pass
