# skyquery Specification

**Status:** Draft
**Author:** Sky Team
**Created:** 2026-02-03

## Core Philosophy

**Starlark is the first-class citizen.**

Sky treats Starlark as a language in its own right. Bazel, Buck2, Tilt, and other
build systems are **dialects** — implementations that extend Starlark with their
own builtins and conventions.

`skyquery` queries **Starlark code structures**, not "Bazel targets." Dialect-specific
concepts (like Bazel's targets or Buck2's rules) are layered on top as optional
extensions.

## Overview

`skyquery` is a tool for querying and introspecting Starlark source files. It provides:

1. **Universal queries** — Work on any Starlark code (functions, loads, calls, etc.)
2. **Dialect extensions** — Optional layers for Bazel, Buck2, etc.
3. **No runtime required** — Pure static analysis of source files

### Use Cases

| Use Case                         | Example                                     |
| -------------------------------- | ------------------------------------------- |
| Find all function definitions    | `skyquery 'defs(//...)'`                    |
| Find all load statements         | `skyquery 'loads(//...)'`                   |
| What files load a module?        | `skyquery 'loadedby("//lib:utils.sky")'`    |
| Find all calls to a function     | `skyquery 'calls(http_archive, //...)'`     |
| Extract string literals          | `skyquery 'strings(//config.star)'`         |
| **[Bazel dialect]** List targets | `skyquery --dialect=bazel 'targets(//...)'` |
| **[Buck2 dialect]** List rules   | `skyquery --dialect=buck2 'rules(//...)'`   |

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         skyquery                             │
├─────────────────────────────────────────────────────────────┤
│  CLI (cmd/skyquery)                                          │
├─────────────────────────────────────────────────────────────┤
│  Dialect Extensions (optional)                               │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐            │
│  │   Bazel     │ │   Buck2     │ │   Tilt      │  ...       │
│  │  targets()  │ │   rules()   │ │ resources() │            │
│  └─────────────┘ └─────────────┘ └─────────────┘            │
├─────────────────────────────────────────────────────────────┤
│  Core Query Engine (internal/starlark/query)                 │
│  - defs(), loads(), calls(), strings(), etc.                │
│  - Set operations                                            │
│  - Pattern matching                                          │
├─────────────────────────────────────────────────────────────┤
│  Starlark Index (internal/starlark/query/index)              │
│  - File discovery                                            │
│  - AST extraction                                            │
│  - Symbol table                                              │
├─────────────────────────────────────────────────────────────┤
│  Parser (buildtools/build)                                   │
│  Classifier (internal/starlark/classifier)                   │
└─────────────────────────────────────────────────────────────┘
```

## Data Model

### File

A Starlark source file:

```go
type File struct {
    Path     string           // Relative path from workspace
    Kind     filekind.Kind    // BUILD, bzl, star, BUCK, etc.
    Dialect  dialect.Dialect  // bazel, buck2, starlark, etc.
    Defs     []Def            // Function definitions
    Loads    []Load           // Load statements
    Calls    []Call           // Top-level function calls
    Assigns  []Assign         // Top-level assignments
}
```

### Def (Function Definition)

```go
type Def struct {
    Name     string
    File     string
    Line     int
    Params   []Param
    Docstring string
}
```

### Load (Import Statement)

```go
type Load struct {
    Module   string            // "//lib:utils.bzl" or "@repo//pkg:file.star"
    Symbols  map[string]string // local_name -> exported_name
    File     string
    Line     int
}
```

### Call (Function Call)

```go
type Call struct {
    Function string            // Function name being called
    Args     []Arg             // Arguments (positional and keyword)
    File     string
    Line     int
}
```

### Assign (Assignment)

```go
type Assign struct {
    Name     string
    File     string
    Line     int
    Value    Expr              // The assigned expression
}
```

## Query Language

### Core Queries (All Starlark)

These work on **any** Starlark file regardless of dialect:

| Query                | Description                 | Example                        |
| -------------------- | --------------------------- | ------------------------------ |
| `files(pattern)`     | Find files matching pattern | `files(//...)`                 |
| `defs(expr)`         | Function definitions        | `defs(//lib/...)`              |
| `loads(expr)`        | Load statements             | `loads(//...)`                 |
| `loadedby(module)`   | Files that load a module    | `loadedby("//lib:utils.star")` |
| `calls(fn, expr)`    | Calls to a function         | `calls(print, //...)`          |
| `assigns(expr)`      | Top-level assignments       | `assigns(//config.star)`       |
| `strings(expr)`      | String literals             | `strings(//...)`               |
| `refs(symbol, expr)` | References to a symbol      | `refs(my_func, //...)`         |

### Pattern Expressions

| Pattern               | Description            | Example              |
| --------------------- | ---------------------- | -------------------- |
| `//path/to/file.star` | Specific file          | `//lib/utils.star`   |
| `//pkg:file.bzl`      | File with label syntax | `//tools:macros.bzl` |
| `//pkg/...`           | All files recursively  | `//internal/...`     |
| `*.star`              | Glob in current dir    | `*.star`             |
| `**/*.bzl`            | Recursive glob         | `**/*.bzl`           |

### Filter Functions

| Function              | Description             | Example                          |
| --------------------- | ----------------------- | -------------------------------- |
| `kind(pattern, expr)` | Filter by file kind     | `kind(bzl, //...)`               |
| `dialect(d, expr)`    | Filter by dialect       | `dialect(bazel, //...)`          |
| `filter(regex, expr)` | Filter by name/path     | `filter(".*_test", defs(//...))` |
| `hasarg(name, expr)`  | Calls with specific arg | `hasarg(deps, calls(*, //...))`  |

### Set Operations

| Operator | Description  |
| -------- | ------------ |
| `a + b`  | Union        |
| `a - b`  | Difference   |
| `a ^ b`  | Intersection |

## Dialect Extensions

Dialects add **domain-specific queries** that understand the semantics of that
build system. These are opt-in via `--dialect` flag.

### Bazel Dialect (`--dialect=bazel`)

Understands BUILD, WORKSPACE, MODULE.bazel, and .bzl files.

| Query                       | Description                        |
| --------------------------- | ---------------------------------- |
| `targets(expr)`             | Rule instantiations in BUILD files |
| `deps(target)`              | Dependencies (from deps attr)      |
| `rdeps(universe, target)`   | Reverse dependencies               |
| `attr(name, pattern, expr)` | Filter by attribute                |
| `labels(attr, expr)`        | Extract label-valued attributes    |
| `providers(expr)`           | Provider definitions in .bzl       |
| `rules(expr)`               | Rule definitions in .bzl           |
| `macros(expr)`              | Macro definitions in .bzl          |

### Buck2 Dialect (`--dialect=buck2`)

Understands BUCK files and .bxl files.

| Query                   | Description                       |
| ----------------------- | --------------------------------- |
| `rules(expr)`           | Rule instantiations in BUCK files |
| `deps(rule)`            | Dependencies                      |
| `rdeps(universe, rule)` | Reverse dependencies              |
| `bxl_main(expr)`        | BXL main functions                |

### Generic/Pure Starlark (`--dialect=starlark`)

Default. No build-system-specific queries. Just core Starlark analysis.

## CLI Interface

### Usage

```bash
# Core queries (any Starlark)
skyquery 'defs(//...)'                    # All function definitions
skyquery 'loads(//lib/...)'               # All loads in lib/
skyquery 'calls(load, //...)'             # All load() calls
skyquery 'loadedby("//lib:common.star")' # What loads common.star?

# With dialect for build-system-specific queries
skyquery --dialect=bazel 'targets(//cmd/...)'
skyquery --dialect=buck2 'rules(//...)'

# Output formats
skyquery --output=json 'defs(//...)'
skyquery --output=location 'calls(http_archive, //...)'

# Filtering
skyquery 'filter("^_", defs(//...))' # Private functions (start with _)
```

### Flags

| Flag           | Description                                   | Default           |
| -------------- | --------------------------------------------- | ----------------- |
| `--dialect`    | Dialect: `starlark`, `bazel`, `buck2`, `tilt` | auto-detect       |
| `--output`     | Format: `name`, `location`, `json`, `count`   | `name`            |
| `--workspace`  | Workspace root                                | Current directory |
| `--keep_going` | Continue on parse errors                      | `false`           |

### Output Formats

#### `name` (default)

```
my_function
other_function
_private_helper
```

#### `location`

```
//lib/utils.star:15: my_function
//lib/utils.star:42: other_function
//lib/internal.star:8: _private_helper
```

#### `json`

```json
{
  "results": [
    {
      "type": "def",
      "name": "my_function",
      "file": "lib/utils.star",
      "line": 15,
      "params": ["ctx", "deps"]
    }
  ]
}
```

#### `count`

```
3
```

## Implementation Phases

### Phase 1: Core Starlark Queries (MVP)

- [ ] File discovery and pattern matching (`//...`, globs)
- [ ] `files()` - enumerate files
- [ ] `defs()` - extract function definitions
- [ ] `loads()` - extract load statements
- [ ] `calls()` - extract function calls
- [ ] `filter()` - regex filtering
- [ ] Output formats: `name`, `location`, `json`

### Phase 2: Load Graph

- [ ] `loadedby()` - reverse load lookup
- [ ] Load graph construction
- [ ] Cycle detection
- [ ] `allloads()` - transitive loads

### Phase 3: Bazel Dialect

- [ ] `targets()` - BUILD file targets
- [ ] `deps()` / `rdeps()` - dependency queries
- [ ] `attr()` - attribute filtering
- [ ] `labels()` - label extraction

### Phase 4: Additional Dialects

- [ ] Buck2 dialect
- [ ] Tilt dialect
- [ ] Plugin system for custom dialects

## Package Structure

```
internal/starlark/query/
├── BUILD.bazel
├── query.go           # Public API
├── engine.go          # Query evaluation
├── parser.go          # Query language parser
├── ast.go             # Query AST
├── core/              # Core Starlark queries
│   ├── defs.go        # defs() implementation
│   ├── loads.go       # loads() implementation
│   ├── calls.go       # calls() implementation
│   └── ...
├── index/             # File indexing
│   ├── index.go
│   ├── file.go
│   └── extract.go
├── dialect/           # Dialect extensions
│   ├── dialect.go     # Interface
│   ├── bazel/         # Bazel-specific queries
│   ├── buck2/         # Buck2-specific queries
│   └── starlark/      # Pure Starlark (default)
└── output/            # Output formatting
    └── format.go
```

## Examples

### Find all public functions

```bash
skyquery 'filter("^[^_]", defs(//...))'
```

### What files load a utility module?

```bash
skyquery 'loadedby("//lib:strings.star")'
```

### Find all http_archive calls (Bazel)

```bash
skyquery 'calls(http_archive, //WORKSPACE)'
```

### List all rule definitions in .bzl files

```bash
skyquery --dialect=bazel 'rules(kind(bzl, //...))'
```

### Find functions with more than 5 parameters

```bash
skyquery --output=json 'defs(//...)' | jq '.results[] | select(.params | length > 5)'
```

### Count targets per package (Bazel)

```bash
for pkg in $(skyquery 'files(//...)' | xargs dirname | sort -u); do
  echo "$pkg: $(skyquery --output=count --dialect=bazel "targets(//$pkg:*)")"
done
```

## Design Decisions

### Why Starlark-first?

1. **Broader applicability** — Works with any Starlark codebase, not just Bazel
2. **Simpler core** — Build-system concepts are extensions, not fundamentals
3. **Future-proof** — New dialects can be added without changing core
4. **Educational** — Helps users understand Starlark vs dialect-specific concepts

### Why not just wrap `bazel query`?

1. **No Bazel required** — Works without installing/configuring Bazel
2. **Faster for simple queries** — No need to load entire build graph
3. **Works on incomplete projects** — Doesn't require valid WORKSPACE
4. **Cross-dialect** — Same tool for Bazel, Buck2, etc.

## References

- [Starlark Language Spec](https://github.com/bazelbuild/starlark/blob/master/spec.md)
- [Sky Dialect System](../internal/starlark/dialect/)
- [Sky File Classifier](../internal/starlark/classifier/)
