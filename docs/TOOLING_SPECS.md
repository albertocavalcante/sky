# Sky Tooling Specifications

**Status:** Living Document
**Created:** 2026-02-03

## Overview

This document provides detailed specifications for each tool in the Sky
toolchain, mapped to Python equivalents. It also identifies required
changes to `starlark-go` (our fork: `starlark-go-x`).

---

## Repository References

| Repo            | Purpose                                          |
| --------------- | ------------------------------------------------ |
| `sky`           | Main toolchain (skyfmt, skylint, skyquery, etc.) |
| `starlark-go-x` | Our fork of starlark-go with extensions          |

### starlark-go-x Branches

| Branch       | Status            | Purpose                        |
| ------------ | ----------------- | ------------------------------ |
| `main`       | Tracking upstream | Sync with go.starlark.net      |
| `type-hints` | WIP (+223 lines)  | Type annotation syntax support |

---

## Tool Specifications

### 1. skyfmt (Formatter)

**Python Equivalent:** black, ruff format

**Status:** ✅ Implemented

**Current Implementation:**

- Uses `github.com/bazelbuild/buildtools/build` for parsing/formatting
- Supports all file kinds (BUILD, bzl, WORKSPACE, MODULE, star)
- Dialect-aware formatting

**Gaps:**

- [ ] EditorConfig support
- [ ] Format-on-save daemon mode
- [ ] Parallel file processing for large repos

**starlark-go-x Changes Needed:** None

---

### 2. skylint (Linter)

**Python Equivalent:** ruff, pylint, flake8

**Status:** ✅ Implemented (MVP)

**Current Implementation:**

- Buildtools warnings integration
- Configurable rules (`.skylint.yaml`)
- Suppression comments
- Multiple output formats (text, JSON, GitHub Actions)

**Gaps:**

| Feature            | Python (ruff) | skylint | Priority |
| ------------------ | ------------- | ------- | -------- |
| Rule count         | 800+          | ~50     | Medium   |
| Auto-fix (`--fix`) | ✅            | ❌      | High     |
| Plugin system      | ✅            | ❌      | Medium   |
| Caching            | ✅            | ❌      | Low      |
| Watch mode         | ✅            | ❌      | Low      |

**Spec: Auto-fix**

```go
// Finding with optional fix
type Finding struct {
    // ... existing fields ...
    Fix *Fix // Optional auto-fix
}

type Fix struct {
    Description string
    Edits       []Edit
}

type Edit struct {
    Start, End Position
    NewText    string
}
```

CLI:

```bash
skylint --fix path/to/file.bzl      # Apply fixes
skylint --fix --diff path/...       # Show diff without applying
skylint --fix --unsafe path/...     # Apply unsafe fixes too
```

**starlark-go-x Changes Needed:** None

---

### 3. skycheck (Type Checker)

**Python Equivalent:** mypy, pyright, pylance, pyrefly, ty

**Status:** 🚧 Placeholder

**Challenge:** Starlark is dynamically typed. Python solved this with:

1. Type hints (PEP 484) - optional annotations
2. Type inference - deduce types from usage
3. Type stubs (.pyi files) - external type definitions

**Phased Approach:**

#### Phase 1: Basic Checks (No Types)

- Undefined names
- Unused variables/imports
- Unreachable code
- Incorrect arity (wrong number of args)

#### Phase 2: Type Inference

- Infer types from literals and operations
- Track types through assignments
- Report type mismatches

#### Phase 3: Type Annotations (Requires starlark-go-x)

Support Python-style type hints:

```python
def greet(name: str, times: int = 1) -> str:
    return name * times

# List, Dict, Optional
def process(items: list[str], config: dict[str, int] | None) -> bool:
    ...
```

#### Phase 4: Type Stubs (.skyi files)

External type definitions for builtins:

```python
# builtins.skyi
def len(obj: Sized) -> int: ...
def range(start: int, stop: int = ..., step: int = ...) -> list[int]: ...
def str(obj: Any) -> str: ...
```

**starlark-go-x Changes Needed:**

| Change                 | Branch       | Status           |
| ---------------------- | ------------ | ---------------- |
| Parse type annotations | `type-hints` | WIP (+223 lines) |
| TypeExpr AST node      | `type-hints` | ✅ Done          |
| TypedParam AST node    | `type-hints` | ✅ Done          |
| Return type on DefStmt | `type-hints` | ✅ Done          |
| Type checking runtime  | TBD          | Not started      |

**Current type-hints Branch Progress:**

```go
// New AST nodes in syntax/syntax.go
type TypeExpr struct {
    Expr Expr // the underlying type expression
}

type TypedParam struct {
    Name    *Ident
    Colon   Position
    Type    *TypeExpr
    Default Expr // optional default value
}

type DefStmt struct {
    // ... existing fields ...
    Arrow      Position  // position of '->' if present
    ReturnType *TypeExpr // return type annotation
}
```

---

### 4. skylsp (Language Server)

**Python Equivalent:** pylance, pyright, jedi-language-server

**Status:** 📋 Not Started

**Existing Tools:**

- [starpls](https://github.com/withered-magic/starpls) - Bazel-focused, Rust

**Features Required:**

| Feature          | Priority | Depends On         |
| ---------------- | -------- | ------------------ |
| Go to definition | P0       | skyquery           |
| Find references  | P0       | skyquery           |
| Hover (docs)     | P0       | skyquery           |
| Completion       | P0       | builtins, skyquery |
| Diagnostics      | P1       | skylint, skycheck  |
| Code actions     | P1       | skylint --fix      |
| Rename           | P2       | skyquery           |
| Formatting       | P2       | skyfmt             |

**Architecture:**

```
┌─────────────────────────────────────────────┐
│                  skylsp                      │
├─────────────────────────────────────────────┤
│  LSP Protocol Handler (jsonrpc)             │
├─────────────────────────────────────────────┤
│  Document Manager (open files, sync)        │
├─────────────────────────────────────────────┤
│  ┌─────────┐ ┌─────────┐ ┌─────────┐       │
│  │skyquery │ │skylint  │ │skyfmt   │       │
│  │(index)  │ │(diag)   │ │(format) │       │
│  └─────────┘ └─────────┘ └─────────┘       │
├─────────────────────────────────────────────┤
│  Dialect Builtins (bazel, buck2, etc.)      │
└─────────────────────────────────────────────┘
```

**starlark-go-x Changes Needed:** None directly, but benefits from type-hints

---

### 5. skyquery (Code Query)

**Python Equivalent:** ast module, jedi, rope, importlab

**Status:** 📋 Spec Complete

See [SKYQUERY_SPEC.md](./SKYQUERY_SPEC.md)

**Core Queries (Starlark-first):**

- `defs()` - function definitions
- `loads()` - import statements
- `calls()` - function invocations
- `loadedby()` - reverse import graph

**starlark-go-x Changes Needed:** None

---

### 6. skyrepl (Interactive REPL)

**Python Equivalent:** ipython, ptpython, bpython

**Status:** 🚧 Placeholder

**Existing:**

- starlark-go has basic REPL in `cmd/starlark`
- Limited features (no completion, no syntax highlighting)

**Features Required:**

| Feature              | ipython | starlark-go | skyrepl |
| -------------------- | ------- | ----------- | ------- |
| Tab completion       | ✅      | ❌          | ✅      |
| Syntax highlighting  | ✅      | ❌          | ✅      |
| History (persistent) | ✅      | Partial     | ✅      |
| Magic commands       | ✅      | ❌          | ✅      |
| Multi-line editing   | ✅      | Basic       | ✅      |
| Object introspection | ✅      | ❌          | ✅      |
| Debugger integration | ✅      | ❌          | Future  |

**Magic Commands:**

```
%load file.star     # Load and execute file
%ast expr           # Show AST of expression
%type expr          # Show inferred type
%dialect bazel      # Switch dialect
%builtins           # List available builtins
%doc symbol         # Show documentation
```

**starlark-go-x Changes Needed:**

| Change                     | Purpose                            |
| -------------------------- | ---------------------------------- |
| Expose completion API      | Tab completion                     |
| Add introspection builtins | `dir()`, `type()`, `help()`        |
| REPL hooks                 | Custom prompts, pre/post execution |

---

### 7. skytest (Test Runner)

**Python Equivalent:** pytest, unittest

**Status:** 📋 Not Started

**Existing:**

- starlark-go has `starlarktest` package (basic assert module)
- Used internally, not as standalone test runner

**Current starlarktest Features:**

```python
load("assert.star", "assert")

assert.eq(x, y)        # equality
assert.ne(x, y)        # not equal
assert.true(cond)      # truthy
assert.lt(x, y)        # less than
assert.contains(x, y)  # y in x
assert.fails(f, pat)   # f() raises matching error
```

**Gap Analysis vs pytest:**

| Feature            | pytest      | starlarktest | skytest |
| ------------------ | ----------- | ------------ | ------- |
| Test discovery     | ✅          | ❌           | ✅      |
| Fixtures           | ✅          | ❌           | ✅      |
| Parametrization    | ✅          | ❌           | ✅      |
| Markers/tags       | ✅          | ❌           | ✅      |
| Coverage           | ✅          | ❌           | ✅      |
| Parallel execution | ✅          | ❌           | ✅      |
| Watch mode         | ✅ (plugin) | ❌           | ✅      |
| Rich output        | ✅          | ❌           | ✅      |
| JUnit XML          | ✅          | ❌           | ✅      |

**Spec:**

Test file convention: `*_test.star` or `test_*.star`

```python
# math_test.star
load("//lib:math.star", "add", "multiply")

def test_add():
    assert.eq(add(1, 2), 3)
    assert.eq(add(-1, 1), 0)

def test_multiply():
    assert.eq(multiply(2, 3), 6)

# Parametrized test
@parametrize("a,b,expected", [
    (1, 2, 3),
    (0, 0, 0),
    (-1, 1, 0),
])
def test_add_params(a, b, expected):
    assert.eq(add(a, b), expected)

# Fixtures
@fixture
def config():
    return {"debug": True}

def test_with_fixture(config):
    assert.true(config["debug"])
```

CLI:

```bash
skytest path/to/tests/           # Run all tests
skytest -k "add"                 # Filter by name
skytest --coverage               # With coverage
skytest --parallel=4             # Parallel execution
skytest --watch                  # Watch mode
skytest --output=junit           # JUnit XML output
```

**starlark-go-x Changes Needed:**

| Change                    | Purpose                | Priority |
| ------------------------- | ---------------------- | -------- |
| Coverage instrumentation  | Track executed lines   | P0       |
| Decorator support         | @fixture, @parametrize | P1       |
| Thread-local test context | Fixture injection      | P1       |
| Source mapping            | Coverage reports       | P0       |

---

### 8. skycov (Coverage)

**Python Equivalent:** coverage.py

**Status:** 📋 Not Started

**Challenge:** starlark-go doesn't support coverage instrumentation.

**Required starlark-go-x Changes:**

```go
// New coverage API
type CoverageData struct {
    Files map[string]*FileCoverage
}

type FileCoverage struct {
    Lines    map[int]int  // line -> hit count
    Branches map[int]bool // branch -> taken
}

// Enable coverage on thread
func (t *Thread) EnableCoverage()
func (t *Thread) GetCoverage() *CoverageData
```

**Output Formats:**

- Text summary
- HTML report
- Cobertura XML (CI integration)
- LCOV (IDE integration)

---

### 9. skydoc (Documentation Generator)

**Python Equivalent:** sphinx, pdoc, mkdocs

**Status:** 📋 Not Started

**Existing Tools:**

- [stardoc](https://github.com/bazelbuild/stardoc) - **Tightly coupled to Bazel**
  - Uses `native.starlark_doc_extract` (Bazel-only)
  - Cannot be used standalone
  - Only for Bazel rule documentation

**We need a standalone solution.**

**Docstring Convention (Python-style):**

```python
def create_user(name, email, admin = False):
    """Create a new user account.

    Creates a user with the specified name and email address.
    Optionally grants admin privileges.

    Args:
        name: The user's display name.
        email: The user's email address (must be unique).
        admin: If True, grants admin privileges. Defaults to False.

    Returns:
        A User struct with id, name, email, and is_admin fields.

    Raises:
        ValueError: If email is already registered.

    Example:
        user = create_user("Alice", "alice@example.com")
        admin = create_user("Bob", "bob@example.com", admin=True)
    """
    ...
```

**Output Formats:**

- Markdown (for GitHub/GitLab)
- HTML (standalone site)
- JSON (for tooling)

**Features:**

| Feature               | stardoc | skydoc       |
| --------------------- | ------- | ------------ |
| Standalone (no Bazel) | ❌      | ✅           |
| Any Starlark file     | ❌      | ✅           |
| Function docs         | ✅      | ✅           |
| Rule docs (Bazel)     | ✅      | ✅ (dialect) |
| Provider docs         | ✅      | ✅ (dialect) |
| Cross-references      | ✅      | ✅           |
| Search                | ❌      | ✅           |
| Type annotations      | ❌      | ✅           |

**starlark-go-x Changes Needed:**

| Change                   | Purpose                 |
| ------------------------ | ----------------------- |
| Docstring extraction API | Get docstrings from AST |
| Preserve comments in AST | Comment association     |

---

## starlark-go-x Change Summary

### Current Branches

| Branch       | Changes                | Status |
| ------------ | ---------------------- | ------ |
| `type-hints` | Type annotation syntax | WIP    |

### Required Changes (Prioritized)

#### P0: Critical for Tooling

| Change                    | Tools Affected  | Effort |
| ------------------------- | --------------- | ------ |
| Coverage instrumentation  | skytest, skycov | Large  |
| Finish type-hints parsing | skycheck        | Medium |

#### P1: Important

| Change                   | Tools Affected  | Effort |
| ------------------------ | --------------- | ------ |
| Docstring extraction API | skydoc          | Small  |
| Introspection builtins   | skyrepl         | Small  |
| Completion API           | skyrepl, skylsp | Medium |

#### P2: Nice to Have

| Change            | Tools Affected | Effort |
| ----------------- | -------------- | ------ |
| Decorator support | skytest        | Medium |
| Debug hooks       | skyrepl        | Medium |
| Source mapping    | skycov         | Small  |

---

## Implementation Order

Based on dependencies and user impact:

```
Phase 1: Foundation
├── skyquery (core) ────────────────────────► skylsp (goto def, refs)
└── skylint (expand rules, add --fix)

Phase 2: IDE Experience
├── skylsp (using skyquery + skylint)
└── skycheck Phase 1 (basic checks)

Phase 3: Testing
├── starlark-go-x: coverage instrumentation
├── skytest (test runner)
└── skycov (coverage reports)

Phase 4: Advanced
├── starlark-go-x: finish type-hints
├── skycheck Phase 2-3 (type inference, annotations)
├── skyrepl (rich REPL)
└── skydoc (documentation)
```

---

## References

- [starlark-go](https://github.com/google/starlark-go) - Original Go implementation
- [starlark-go-x](https://github.com/albertocavalcante/starlark-go-x) - Our fork
- [stardoc](https://github.com/bazelbuild/stardoc) - Bazel doc generator (reference only)
- [starpls](https://github.com/withered-magic/starpls) - Existing Starlark LSP
- [ruff](https://github.com/astral-sh/ruff) - Python linter (benchmark)
- [pytest](https://github.com/pytest-dev/pytest) - Python test framework (benchmark)
