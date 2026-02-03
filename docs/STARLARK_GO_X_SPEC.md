# starlark-go-x Extension Specification

> **Source of Truth**: This document specifies all extensions to `starlark-go` for the
> Sky toolchain. Implementation lives in `github.com/albertocavalcante/starlark-go-x`,
> but design and specs are maintained here.

## Overview

`starlark-go-x` is a fork of `go.starlark.net` with extensions required by Sky tools:

| Extension        | Purpose                       | Required By      | Status         |
| ---------------- | ----------------------------- | ---------------- | -------------- |
| Type Annotations | Python-style type hints       | skycheck, skylsp | 🟡 In Progress |
| Coverage API     | Line/branch coverage tracking | skytest, skycov  | 🔴 Not Started |
| Introspection    | `dir()`, `help()`, completion | skyrepl, skylsp  | 🔴 Not Started |
| Docstring API    | Extract docstrings from AST   | skydoc           | 🔴 Not Started |

---

## 1. Type Annotations (In Progress)

### 1.1 Syntax

Python 3.10+ style type annotations:

```python
# Function with typed parameters and return type
def greet(name: str, times: int = 1) -> str:
    return name * times

# Union types (Python 3.10+ syntax)
def process(value: int | str) -> None:
    pass

# Generic types
def first(items: list[T]) -> T:
    return items[0]

# Optional (None union)
def maybe(x: int | None) -> int | None:
    return x

# *args and **kwargs with types
def variadic(*args: int, **kwargs: str) -> None:
    pass
```

### 1.2 AST Nodes

**New nodes in `syntax/syntax.go`:**

```go
// TypeExpr wraps a type annotation expression.
// Valid forms: Ident, IndexExpr (generics), BinaryExpr with PIPE (unions).
type TypeExpr struct {
    commentsRef
    Expr Expr // the underlying type expression
}

// TypedParam represents a parameter with type annotation.
// Forms: name: type, name: type = default
type TypedParam struct {
    commentsRef
    Name    *Ident
    Colon   Position
    Type    *TypeExpr
    Default Expr      // optional default value
}
```

**Extended `DefStmt`:**

```go
type DefStmt struct {
    commentsRef
    Def        Position
    Name       *Ident
    Lparen     Position
    Params     []Expr    // includes TypedParam for typed parameters
    Rparen     Position
    Arrow      Position  // position of '->' if present
    ReturnType *TypeExpr // return type annotation, or nil
    Body       []Stmt
    Function   interface{}
}
```

### 1.3 Scanner Token

**New token in `syntax/scan.go`:**

```go
const (
    // ...existing tokens...
    ARROW // ->
)
```

### 1.4 Parser Options

**New option in `syntax/options.go`:**

```go
type TypeMode int

const (
    // TypesDisabled rejects type annotation syntax (default, backward compatible)
    TypesDisabled TypeMode = iota

    // TypesParseOnly parses type annotations but ignores them at runtime.
    // Use for documentation, IDE support, and external type checkers.
    TypesParseOnly

    // TypesEnabled enables runtime type checking (future).
    TypesEnabled
)

type FileOptions struct {
    // ...existing fields...
    Types TypeMode
}
```

### 1.5 Implementation Status

| File                   | Changes                    | Status  |
| ---------------------- | -------------------------- | ------- |
| `syntax/syntax.go`     | TypeExpr, TypedParam nodes | ✅ Done |
| `syntax/scan.go`       | ARROW token                | ✅ Done |
| `syntax/parse.go`      | Parse type annotations     | ✅ Done |
| `syntax/walk.go`       | Walk TypeExpr, TypedParam  | ✅ Done |
| `syntax/options.go`    | TypeMode option            | ✅ Done |
| `syntax/parse_test.go` | Test cases                 | ✅ Done |
| `syntax/scan_test.go`  | ARROW token test           | ✅ Done |

**TODO:**

- [ ] Commit changes to `type-hints` branch
- [ ] Add comprehensive test suite
- [ ] Document in upstream-compatible way

---

## 2. Coverage Instrumentation API (Not Started)

### 2.1 Purpose

Enable line and branch coverage tracking for `skytest` and `skycov`.

### 2.2 Proposed API

**New types in `starlark/coverage.go`:**

```go
// CoverageData holds coverage information collected during execution.
type CoverageData struct {
    Files map[string]*FileCoverage
}

// FileCoverage holds coverage data for a single file.
type FileCoverage struct {
    // Lines maps line number to execution count.
    // Line numbers are 1-based.
    Lines map[int]int

    // Branches maps branch ID to taken/not-taken.
    // Branch IDs are assigned during instrumentation.
    Branches map[int]*BranchCoverage
}

// BranchCoverage tracks a single branch point.
type BranchCoverage struct {
    Line      int  // source line
    TrueHits  int  // times true branch taken
    FalseHits int  // times false branch taken
}
```

**Thread methods:**

```go
// EnableCoverage starts collecting coverage data.
// Must be called before execution.
func (t *Thread) EnableCoverage()

// DisableCoverage stops collecting coverage data.
func (t *Thread) DisableCoverage()

// Coverage returns the collected coverage data.
// Returns nil if coverage was never enabled.
func (t *Thread) Coverage() *CoverageData

// ResetCoverage clears all collected coverage data.
func (t *Thread) ResetCoverage()

// MergeCoverage combines coverage data from another thread.
// Useful for aggregating results from parallel test execution.
func (t *Thread) MergeCoverage(other *CoverageData)
```

### 2.3 Implementation Strategy

1. Add `coverage *CoverageData` field to `Thread` struct
2. Instrument `eval.go` at statement boundaries to track line hits
3. Instrument conditional expressions (`if`, `and`, `or`, ternary) for branch coverage
4. Track function entry/exit for function coverage
5. Expose via Thread methods

### 2.4 Output Formats (handled by skycov)

The coverage API provides raw data. Output formatting is handled by `skycov`:

- Text summary
- HTML report
- Cobertura XML (CI integration)
- LCOV (IDE integration)

---

## 3. Introspection API (Not Started)

### 3.1 Purpose

Enable rich REPL experience and IDE completion.

### 3.2 Proposed Builtins

```python
# Return list of names in scope or object attributes
dir()          # names in current scope
dir(obj)       # attributes of obj

# Return documentation string
help(obj)      # print docstring for obj

# Type information
type(obj)      # return type name (already exists)
isinstance(obj, type)  # type checking (new)
```

### 3.3 Completion API

**New in `starlark/starlark.go`:**

```go
// CompletionItem represents a completion suggestion.
type CompletionItem struct {
    Name       string
    Kind       CompletionKind // Variable, Function, Method, Field, Keyword
    Type       string         // type annotation if known
    Doc        string         // documentation snippet
    InsertText string         // text to insert (may include template)
}

type CompletionKind int

const (
    CompletionVariable CompletionKind = iota
    CompletionFunction
    CompletionMethod
    CompletionField
    CompletionKeyword
    CompletionModule
)

// Complete returns completion suggestions for the given position.
// prefix is the partial identifier being typed.
func (t *Thread) Complete(prefix string) []CompletionItem
```

---

## 4. Docstring Extraction API (Not Started)

### 4.1 Purpose

Enable `skydoc` to generate documentation from source files.

### 4.2 Proposed API

**New in `syntax/syntax.go`:**

```go
// Docstring returns the docstring for a DefStmt, if present.
// A docstring is a string literal as the first statement in the body.
func (d *DefStmt) Docstring() string

// ModuleDocstring returns the module docstring from a File.
// A module docstring is a string literal as the first statement.
func (f *File) ModuleDocstring() string
```

**Parsed docstring structure:**

```go
// ParsedDocstring represents a structured docstring.
type ParsedDocstring struct {
    Summary     string              // first paragraph
    Description string              // full description
    Args        []DocstringArg      // documented arguments
    Returns     string              // return value description
    Raises      []DocstringRaises   // documented exceptions
    Examples    []DocstringExample  // usage examples
}

type DocstringArg struct {
    Name        string
    Type        string // from docstring, not annotation
    Description string
}

// ParseDocstring parses a docstring into structured form.
// Supports Google, NumPy, and Sphinx docstring styles.
func ParseDocstring(s string) *ParsedDocstring
```

---

## 5. Fork Maintenance Strategy

### 5.1 Principles

1. **Minimal divergence**: Only add what's necessary for Sky tools
2. **Backward compatible**: All extensions are opt-in via options
3. **Upstream-friendly**: Changes should be mergeable upstream if accepted
4. **Well-tested**: Comprehensive tests for all extensions

### 5.2 Branch Strategy

| Branch       | Purpose                           |
| ------------ | --------------------------------- |
| `main`       | Tracks upstream `go.starlark.net` |
| `type-hints` | Type annotation implementation    |
| `coverage`   | Coverage instrumentation (future) |
| `sky`        | Merged extensions for Sky release |

### 5.3 Syncing with Upstream

```bash
# Add upstream remote (one-time)
git remote add upstream https://github.com/google/starlark-go.git

# Sync main with upstream
git checkout main
git fetch upstream
git merge upstream/master

# Rebase feature branches
git checkout type-hints
git rebase main
```

### 5.4 Release Process

1. Merge completed features into `sky` branch
2. Tag release (e.g., `v0.1.0-sky`)
3. Update Sky's go.mod to reference tag
4. Test full Sky toolchain

---

## 6. Integration with Sky

### 6.1 go.mod Reference

```go
require (
    github.com/albertocavalcante/starlark-go-x v0.1.0-sky
)

replace go.starlark.net => github.com/albertocavalcante/starlark-go-x v0.1.0-sky
```

### 6.2 Using Extensions in Sky Tools

**skycheck (type checking):**

```go
opts := syntax.FileOptions{Types: syntax.TypesParseOnly}
f, err := syntax.Parse(filename, src, 0, opts)
// Extract type annotations from AST for checking
```

**skytest (coverage):**

```go
thread := &starlark.Thread{Name: "test"}
thread.EnableCoverage()
// ... run tests ...
cov := thread.Coverage()
// Generate reports
```

**skylsp (completion):**

```go
items := thread.Complete(prefix)
// Convert to LSP CompletionItem
```

---

## Appendix A: Current type-hints Diff Summary

```
syntax/options.go    | 16 ++++++++++
syntax/parse.go      | 84 +++++++++++++++++++++++++++++++++++++++++----
syntax/parse_test.go | 64 ++++++++++++++++++++++++++++++++++++
syntax/scan.go       |  8 ++++-
syntax/scan_test.go  |  9 ++++++
syntax/syntax.go     | 46 +++++++++++++++++++++----
syntax/walk.go       | 13 ++++++++
7 files changed, 223 insertions(+), 17 deletions(-)
```

## Appendix B: References

- [Starlark Language Spec](https://github.com/bazelbuild/starlark/blob/master/spec.md)
- [Python Type Hints PEP 484](https://peps.python.org/pep-0484/)
- [Python Union Syntax PEP 604](https://peps.python.org/pep-0604/)
- [coverage.py](https://coverage.readthedocs.io/) - Python coverage inspiration
