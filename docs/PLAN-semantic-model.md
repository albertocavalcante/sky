# PLAN — Starlark semantic model (HIR) on the CST stack

**Status:** Draft. Strategic / multi-month arc. Defines the major
architectural addition.
**Owner:** @albertocavalcante
**Prereq:** `PLAN-cst-deps-bump.md`, `PLAN-lsp-cst-migration.md`
foundations (the analysis trio + incremental driver).
**Tracks:** the largest remaining gap between our stack and
production-grade LSPs like starpls.

## Why this exists

A starpls survey (`~/dev/refs/starpls/crates/`) maps almost
1:1 onto our architecture, with **one major exception**:

| starpls crate | Our equivalent |
|---|---|
| `starpls_lexer` | `starlark-cst-go/lexer` |
| `starpls_parser` | `starlark-cst-go/parser` |
| `starpls_syntax` (rowan red/green) | `starlark-cst-go/syntax` |
| `starpls_intern` | `syntax.TokenInterner` / `TriviaInterner` |
| `starpls_bazel` | `bazel-cst-go` (builtins, labels, etc.) |
| `starpls_hir` (High-level IR) | **❌ MISSING** |
| `starpls_ide` (LSP features as library) | partial: sky's `internal/lsp` is server-coupled, not a library |
| `starpls` (LSP binary) | sky's `skyls` |

Without HIR, we have:

- Textual identifier matching (`FindIdentifierReferences`) — works
  but conflates same-named-different-scope occurrences.
- Buildtools-level AST inspection (sky's `query/index`) — limited to
  shallow extraction.
- No type inference, no scope resolution, no cross-file binding
  resolution beyond manual `workspace.go` plumbing.

**Production-grade LSP features that REQUIRE HIR:**

- Scope-aware "find references" / "go to definition" / "rename"
  (vs textual approximation)
- "Find implementations" for typed parameters
- Hover with inferred types (`def f(x): return x + 1` → hover on
  `x` shows `int` if usage constrains it)
- Code action: "extract variable" with correct scoping
- Cross-file unused-symbol detection
- The "true" LSP renameSymbol that respects scope

This plan documents what HIR is, why it's separate from the CST,
and a multi-phase build-out.

## What is HIR

A **High-level Intermediate Representation** is a typed, scoped,
desugared view of the program built on top of the syntactic CST.
Same source position info, richer semantic info:

```
Source (text bytes)
  ↓ lexer
Tokens (Token, Trivia)
  ↓ parser
CST / Green-tree (syntactically faithful, lossless)
  ↓ red layer
Red tree (positional view)
  ↓ HIR build pass    ← what we're adding
HIR (semantic items)
```

HIR items (rough sketch for Starlark, mirroring starpls):

- **Module** — top-level statements grouped by file
- **Function** — `def` declarations with typed parameters
- **Param** — function parameters, possibly with type annotations
- **Local** — bindings inside a function (assignments)
- **Global** — top-level assignments
- **LoadedSymbol** — imported via `load("...", name)`, with
  reference to the source module
- **Call** — call expressions resolved to a target Function
- **Reference** — uses of an identifier, resolved to the binding
  they refer to
- **Type** — Starlark's gradual type system (most expressions are
  `Any`; some are constrained: `[T]`, `dict[K, V]`, `int`, `str`)

HIR has SCOPES (lexical, function-scoped) and BINDINGS (each
name has a unique binding ID; references point to bindings).
That's the semantic muscle.

## Why HIR isn't in `starlark-cst-go`

The decoupling rule: starlark-cst-go is **pure Starlark syntax**.
HIR is **semantic** — it knows about scope semantics, type
semantics, and (for Bazel dialect) which identifiers resolve to
builtins.

HIR belongs in:

- **Option A:** a new sibling repo `starlark-hir-go` (mirrors the
  cst-go/refactor-go split — independent versioning, narrow
  surface). Recommended.
- **Option B:** inside `starlark-cst-go/hir` (single-repo, faster
  iteration). Acceptable if we never publish HIR-as-library.
- **Option C:** inside sky's `internal/starlark/hir/` (private,
  evolves with sky's needs only).

Recommend **Option A** — same architecture as the rest of the
stack. starpls keeps HIR in a separate crate; same reasoning
applies.

## Phasing

Six phases over multiple sessions. Each phase is independently
useful; the LSP becomes more capable at each step.

### Phase 1 — scopes (~2 sessions)

The simplest semantic primitive: **what's in scope at a position**.

API sketch:

```go
package hir

type Scope struct {
    Parent *Scope         // outer scope, or nil for module-level
    Kind   ScopeKind      // Module / Function / Comprehension / If
    Span   syntax.Span
    Names  map[string]Binding  // bindings introduced in this scope
}

type Binding struct {
    Name     string
    Decl     IdentifierReference  // where it's bound
    Scope    *Scope               // owning scope
}

// BuildScopes walks the AST and returns the scope tree.
func BuildScopes(file ast.File) *Scope { ... }

// ScopeAt returns the innermost scope containing position offset.
func (s *Scope) ScopeAt(offset int) *Scope { ... }

// Lookup walks up the scope chain looking for name.
func (s *Scope) Lookup(name string) (Binding, bool) { ... }
```

**Consumes:** `FindIdentifierDeclarations` (which spans introduce
new names) and AST node kinds (DefStmt, AssignmentExpr,
LoadStmt, comprehension clauses).

**Unlocks:**

- Scope-aware textual rename (sky's RenameSymbol gains a
  `RenameInScope(start, end, name)` variant).
- The LSP's `definition` handler can answer "which decl does this
  use refer to?" — walk up the scope chain.
- The lint rule `SKY-shadowed-binding` (from
  `PLAN-skylint-cst-rules.md`) gains a true implementation.

**Out of scope for Phase 1:** type inference, cross-file
resolution, comprehension-clause subtleties (defer to Phase 3).

### Phase 2 — symbol table (~1 session)

Build on Phase 1 to give every binding a stable identity.

```go
type SymbolID uint32
type SymbolTable struct {
    bindings []Binding   // index = SymbolID
    byScope  map[*Scope][]SymbolID
}

func BuildSymbolTable(file ast.File, scopes *Scope) *SymbolTable
```

Each binding gets a stable `SymbolID`. References become
`(span, SymbolID)` pairs.

**Unlocks:** LSP can present "all references to THIS symbol" by
matching SymbolID, not name. Renames are now structurally safe
within a file.

### Phase 3 — resolved references (~2 sessions)

Walk every identifier use and resolve to its binding:

```go
type ResolvedReference struct {
    Use     IdentifierReference  // the use site
    Target  SymbolID             // which binding it refers to
}

func ResolveReferences(file ast.File, scopes *Scope, syms *SymbolTable) []ResolvedReference
```

For unresolved references (use of unbound name, no matching scope
walk), produce a diagnostic via `SKY-undefined-identifier`.

Edge cases to design through:

- Comprehension scopes: `[x for x in xs]` — `x` is scoped to the
  comprehension, not the enclosing function.
- `load("...", x)` — `x` is module-scoped, resolves to a
  `LoadedSymbol` (cross-file pointer, see Phase 5).
- Builtins: `print`, `len`, `dict`, etc. — provided by the
  dialect's `dialect.Builtins` registry. References to builtins
  resolve to a sentinel `SymbolID = Builtin(name)`.

### Phase 4 — minimal types (~3 sessions)

The hardest phase. Starlark has a gradual type system: most
expressions are `Any`; some have inferable types from context.

Minimal type inference for v1:

- **Literals:** `1` → `int`, `"x"` → `str`, `[1]` → `list[int]`,
  `{}` → `dict[Any, Any]`.
- **Builtins:** `len(x)` → `int`, `str(x)` → `str` (from registry
  signatures).
- **Function returns** when the body is `return literal` or
  `return name`.
- **Parameter constraints** from usage (`x + 1` constrains `x` to
  numeric; not strict, just a hint).

**Out of scope for Phase 4:** full bidirectional type inference,
union types, generics. starpls does partial type inference; we
mirror its scope, no more.

### Phase 5 — cross-file resolution (~3 sessions)

`load("@x//foo.bzl", helper)` — Phase 3 leaves `helper` resolved
to a `LoadedSymbol(module="@x//foo.bzl", source="helper")` but
not to the actual `def helper` in that file.

Cross-file resolution needs a **workspace index** (sky already
has primitives via `workspace.go` for the buildtools path —
parallel infrastructure on HIR).

```go
type Workspace struct {
    files     map[string]*FileHIR    // path → its HIR
    loadGraph map[string][]string    // path → paths it loads from
}

func (w *Workspace) Resolve(load LoadedSymbol, fromPath string) (*Binding, bool)
```

Path resolution rules are Bazel-flavored
(`@repo//pkg:label.bzl` → filesystem path). Lives in
`bazel-cst-go/workspace` or similar (dialect-specific).

### Phase 6 — incremental HIR (~2 sessions)

The big perf concern: HIR build cost. For a 5000-line file,
re-building HIR on every keystroke is wasteful.

Mirror the incremental driver pattern from Phase L1/L2:

- HIR scopes per top-level Statement are independent (a `def`'s
  scope tree depends only on that def's body).
- Edit affects one Statement → rebuild HIR for that Statement
  only; reuse the rest by pointer.
- Same prefix/suffix matching as `incremental.Driver`.

Implementation: HIR `Driver` analogous to `incremental.Driver`,
takes the new CST tree + old HIR + edit info, returns new HIR
with maximum sharing.

## Sequencing

Strict prereq chain: Phase 1 → 2 → 3 → 4 → 5 → 6.

Each phase ships a public API that sky's LSP can incrementally
adopt. The migration mirrors `PLAN-lsp-cst-migration.md`:

1. Phase 1 ships → LSP gains scope-aware rename (PLAN-skylint
   `SKY-shadowed-binding` becomes implementable).
2. Phase 2 ships → SymbolID-keyed caches; LSP highlight gains
   scope-correctness.
3. Phase 3 ships → `SKY-undefined-identifier` becomes
   implementable; LSP `definition` accuracy improves.
4. Phase 4 ships → LSP hover starts showing types; lint rules
   gain type-aware checks.
5. Phase 5 ships → cross-file `definition` works without
   buildtools fallback.
6. Phase 6 ships → keystroke-grade HIR rebuild for large files.

## Reference: starpls's HIR layout

`~/dev/refs/starpls/crates/starpls_hir/src/`:

- `db.rs` — salsa-based incremental query engine (Rust-specific,
  but the concept maps to our HIR driver)
- `def_map.rs` — module-level binding map
- `def/` — definitions (Function, Param, Local, etc.)
- `lower.rs` — CST → HIR lowering pass
- `name_resolution.rs` — reference resolution
- `type_inference.rs` — gradual type inference

Read order for the implementer: `def_map.rs` → `lower.rs` →
`name_resolution.rs` → `type_inference.rs`. The salsa machinery
is Rust-idiomatic; our Go equivalent uses straight function
calls + the incremental driver pattern we already have.

## Out of scope for this plan

- **Bidirectional type inference.** Starlark's type system is
  mostly Any; bidirectional inference would be overengineering.
- **Effect tracking** (which functions read globals, which mutate
  parameters). Not needed for LSP features at our level.
- **Macro expansion as semantic step.** Macros in Starlark are
  ordinary functions returning AST-shaped data; our HIR doesn't
  need to "see through" them for LSP purposes (the macro body
  itself gets its own HIR).
- **`copybara` and `tilt` dialect-specific semantics.** Future
  dialect-cst-go repos when the need surfaces.

## Risks

1. **API stability.** HIR consumers (sky, future LSPs) will
   couple tightly to its types. Once published, breakage hurts.
   Mitigation: ship in `starlark-hir-go` as v0.x with explicit
   "API evolving" disclaimer, sky pinned via pseudo-versions
   (same model as the rest of the stack).
2. **Incremental complexity.** Phase 6 is the hardest by a wide
   margin. May want to defer until production usage proves the
   non-incremental cost is unacceptable.
3. **Bazel-specific name resolution.** Things like
   `load("@bazel_tools//tools/build_defs/...", _name)` have
   workspace-resolution rules that bleed into HIR's cross-file
   layer. Plan: keep dialect-aware resolution in
   `bazel-cst-go/workspace`, not in `starlark-hir-go`.

## Acceptance criteria for the arc (full ship)

- `starlark-hir-go` repo published, mirroring the cst-go stack
  layout (Forgejo primary + GitHub mirror).
- HIR Phases 1-5 shipped (Phase 6 optional, ship when perf
  motivates).
- sky's LSP `definition`, `references`, `rename`, `hover`
  handlers use HIR instead of buildtools-AST.
- A `compare.go`-style equivalence harness validates HIR-based
  handler output against starpls's output on a shared corpus
  where possible. Differences documented as either improvements,
  parity, or known gaps.
- The "buildtools-AST is the LSP's primary backend" story is
  finally retired.
