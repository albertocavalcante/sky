# Tooling Benchmark: Python Ecosystem as Reference

**Status:** Living Document
**Created:** 2026-02-03

## Philosophy

Python has the most mature and sophisticated tooling ecosystem of any scripting
language. Our goal is to bring **the same level of sophistication to Starlark**.

Just as Python evolved from basic tools (pylint, autopep8) to modern, fast,
integrated tools (ruff, pyright, uv), we aim to build a **cohesive, fast,
developer-friendly toolchain** for Starlark.

## Benchmark Mapping

### Formatting

| Python          | Starlark (Sky) | Notes                                         |
| --------------- | -------------- | --------------------------------------------- |
| **black**       | **skyfmt** ✅  | Opinionated, deterministic formatting         |
| autopep8        | -              | Legacy, not needed                            |
| yapf            | -              | Google style, buildtools covers this          |
| **ruff format** | skyfmt         | Fast Rust-based (we use buildtools, Go-based) |

**Status:** `skyfmt` implemented using buildtools. Comparable to black.

**Gap:** Consider Rust rewrite for speed on massive monorepos (future).

---

### Linting

| Python   | Starlark (Sky) | Notes                           |
| -------- | -------------- | ------------------------------- |
| pylint   | skylint ✅     | Comprehensive linting           |
| flake8   | skylint        | Style + error checking          |
| pyflakes | skylint        | Unused imports, undefined names |
| **ruff** | skylint        | Fast, unified linter            |
| bandit   | skylint        | Security checks (future)        |

**Status:** `skylint` implemented with:

- Buildtools warnings integration
- Configurable rules (`.skylint.yaml`)
- Suppression comments (`# nolint`, `# skylint: disable`)
- Multiple output formats (text, JSON, GitHub Actions)

**Gap:**

- Rule coverage not as extensive as ruff's 800+ rules
- No auto-fix capability yet (ruff has `--fix`)
- Consider plugin system for custom rules

---

### Type Checking

| Python      | Starlark (Sky)    | Notes                         |
| ----------- | ----------------- | ----------------------------- |
| **mypy**    | skycheck 🚧       | Original, mature type checker |
| **pyright** | skycheck          | Fast, LSP-native (Microsoft)  |
| **pylance** | skycheck + skylsp | VS Code integration           |
| pyre        | -                 | Meta's type checker           |
| **pyrefly** | skycheck          | Meta's new Rust-based checker |
| **ty**      | skycheck          | New, fast type checker        |

**Status:** `skycheck` is placeholder. This is a **major gap**.

**Challenge:** Starlark is dynamically typed, but we can still check:

- Undefined names
- Unused variables
- Incorrect function signatures (arity)
- Provider field access (Bazel-specific)
- Type annotations in docstrings

**Opportunity:** Build a gradual type system like Python's journey:

1. Basic checks (undefined, unused) — Phase 1
2. Type inference from usage patterns — Phase 2
3. Optional type annotations (like Python's `typing`) — Phase 3
4. Full type checking with stubs (like `.pyi` files) — Phase 4

---

### Language Server (LSP)

| Python               | Starlark (Sky) | Notes                  |
| -------------------- | -------------- | ---------------------- |
| **pylance**          | skylsp 📋      | VS Code, best-in-class |
| pyright              | skylsp         | CLI + LSP              |
| jedi-language-server | -              | Older, still useful    |
| python-lsp-server    | -              | Plugin-based           |

**Status:** No LSP yet. **Critical gap for IDE adoption.**

**Required features:**

- Go to definition
- Find references
- Hover documentation
- Code completion
- Diagnostics (lint + type errors)
- Code actions (quick fixes)
- Rename refactoring

**Reference:** [starpls](https://github.com/withered-magic/starpls) exists but is Bazel-focused.

---

### REPL / Interactive

| Python      | Starlark (Sky) | Notes                  |
| ----------- | -------------- | ---------------------- |
| **ipython** | skyrepl 🚧     | Rich interactive shell |
| ptpython    | skyrepl        | Alternative REPL       |
| jupyter     | -              | Notebooks (future?)    |
| bpython     | -              | Autocomplete REPL      |

**Status:** `skyrepl` is placeholder.

**Features to implement:**

- Dialect-aware evaluation (load builtins for bazel/buck2/etc)
- Tab completion
- Syntax highlighting
- History
- Magic commands (`%load`, `%ast`, `%type`)

---

### Code Query / Analysis

| Python        | Starlark (Sky) | Notes                 |
| ------------- | -------------- | --------------------- |
| `ast` module  | skyquery ✅    | Parse and inspect AST |
| jedi          | skyquery       | Code intelligence     |
| rope          | skyquery       | Refactoring           |
| vulture       | skyquery       | Dead code detection   |
| **importlab** | skyquery       | Import graph analysis |

**Status:** `skyquery` spec written, implementation starting.

**Core queries (Starlark-first):**

- `defs()` — function definitions
- `loads()` — import statements
- `calls()` — function invocations
- `loadedby()` — reverse import graph

---

### Package Management

| Python | Starlark (Sky) | Notes                  |
| ------ | -------------- | ---------------------- |
| pip    | sky plugins    | Basic package install  |
| poetry | sky plugins    | Dependency management  |
| **uv** | sky plugins    | Fast, Rust-based       |
| pipx   | sky plugins    | Isolated tool installs |

**Status:** Plugin system exists with marketplace concept.

**Gap:** Not as mature as uv. Consider:

- Lockfile format
- Dependency resolution
- Virtual environments equivalent?

---

### Documentation

| Python | Starlark (Sky) | Notes                         |
| ------ | -------------- | ----------------------------- |
| sphinx | skydoc 📋      | Documentation generator       |
| mkdocs | skydoc         | Markdown-based docs           |
| pdoc   | skydoc         | Auto-generate from docstrings |
| pydoc  | skydoc         | Built-in doc viewer           |

**Status:** No documentation generator yet.

**Opportunity:** Generate docs from:

- Docstrings in `.bzl` / `.star` files
- Provider definitions
- Rule definitions
- Macro signatures

---

### Testing

| Python      | Starlark (Sky) | Notes                  |
| ----------- | -------------- | ---------------------- |
| pytest      | skytest 📋     | Test runner            |
| unittest    | -              | Built-in               |
| hypothesis  | -              | Property-based testing |
| coverage.py | skycov 📋      | Code coverage          |

**Status:** No dedicated test runner or coverage tool.

**Note:** Starlark files are typically tested through the build system
(bazel test, buck2 test). A standalone test runner could be useful for
pure Starlark libraries.

---

## Tool Maturity Comparison

```
Python Ecosystem (2024)          Starlark/Sky (2026)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━   ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Formatting
  black ████████████████████ 100%   skyfmt ████████████████░░░░ 80%
  ruff  ████████████████████ 100%

Linting
  ruff  ████████████████████ 100%   skylint ██████████████░░░░░░ 70%

Type Checking
  mypy  ████████████████████ 100%   skycheck ████░░░░░░░░░░░░░░░░ 20%
  pyright ██████████████████ 95%

Language Server
  pylance █████████████████ 90%     skylsp ░░░░░░░░░░░░░░░░░░░░ 0%

REPL
  ipython ████████████████ 85%      skyrepl ████░░░░░░░░░░░░░░░░ 20%

Query/Analysis
  ast+jedi ██████████████ 75%       skyquery ████████░░░░░░░░░░░░ 40%

Package Management
  uv    ████████████████████ 100%   plugins ████████░░░░░░░░░░░░ 40%
```

## Priority Roadmap

Based on Python ecosystem maturity and developer impact:

### Tier 1: Foundation (Now)

1. **skyquery** — Code intelligence foundation
2. **skylint** — Already good, expand rules

### Tier 2: IDE Experience (Next)

3. **skylsp** — Language server for VS Code/Neovim
4. **skycheck** — Basic type/error checking

### Tier 3: Developer Experience

5. **skyrepl** — Interactive development
6. **skydoc** — Documentation generation

### Tier 4: Ecosystem

7. **skytest** — Standalone test runner
8. **skycov** — Coverage analysis

## Key Learnings from Python

### 1. Speed Matters

Python's shift to Rust-based tools (ruff, uv, pyrefly) shows that **speed is
not optional**. Our Go-based tools are fast, but we should profile and
optimize for large monorepos (100k+ files).

### 2. Unified Tools Win

ruff replaced pylint + flake8 + isort + pyupgrade. Users prefer **one tool
that does everything well**. Consider:

- `sky lint` (skylint)
- `sky fmt` (skyfmt)
- `sky check` (skycheck)
- `sky query` (skyquery)

All under a single `sky` CLI.

### 3. IDE Integration is Critical

pylance's success shows that **LSP is table stakes**. Without good IDE
support, adoption suffers. `skylsp` should be high priority.

### 4. Gradual Typing Works

Python's journey from untyped → type hints → strict typing shows that
**gradual adoption** is key. We should:

- Start with basic checks (undefined names)
- Add optional type annotations later
- Never require types for existing code to work

### 5. Configuration Should Be Optional

ruff works great with zero config. Tools should have **sensible defaults**
that work for 80% of users. Configuration is for power users.

## Competitive Landscape

### Existing Starlark Tools

| Tool          | Focus             | Limitation     |
| ------------- | ----------------- | -------------- |
| buildtools    | Bazel BUILD files | Bazel-specific |
| buildifier    | Formatting        | Bazel-specific |
| starpls       | LSP               | Bazel-focused  |
| starlark-rust | Runtime           | Not tooling    |
| go-starlark   | Runtime           | Not tooling    |

### Sky's Differentiation

1. **Starlark-first** — Not Bazel-first
2. **Dialect-aware** — Bazel, Buck2, Tilt as plugins
3. **Unified** — One toolchain, not scattered tools
4. **Modern** — Learning from Python's evolution

## References

- [ruff](https://github.com/astral-sh/ruff) — Fast Python linter
- [uv](https://github.com/astral-sh/uv) — Fast Python package manager
- [pyright](https://github.com/microsoft/pyright) — Python type checker
- [pylance](https://marketplace.visualstudio.com/items?itemName=ms-python.vscode-pylance) — VS Code Python
- [pyrefly](https://github.com/pyre-check/pyre-check) — Meta's type checker
- [ty](https://github.com/astral-sh/ty) — New type checker from Astral
