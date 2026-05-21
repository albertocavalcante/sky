# RESEARCH — semantic-model architectures across ecosystems

**Date:** 2026-05-21
**Purpose:** Inform `PLAN-semantic-model.md`. Survey how other
language tooling solves the "semantic layer on top of CST"
problem so our Starlark design borrows from the best, not from
the first idea.

## TL;DR

There are **two distinct categories** of semantic-info systems
in language tooling, often confused:

1. **In-memory semantic models** (Roslyn, rust-analyzer, tsserver,
   starpls) — server-resident, incremental, lazy, used directly
   by the LSP / IDE.
2. **On-disk semantic indexes** (Scala SemanticDB, Sourcegraph
   SCIP, Microsoft LSIF) — serialized formats, produced by
   compilers/indexers, consumed by cross-tool readers (code
   browsers, search engines, IDE plugins outside the host
   compiler).

These are **complementary, not alternatives.** rust-analyzer
exports SCIP. Roslyn IDE features use the in-memory model AND
the same compilation can dump LSIF. The right design has BOTH
layers, with the in-memory model primary and the on-disk index
as an optional artifact.

For Starlark / sky's LSP: in-memory primary (in `starlark-hir-go`),
SCIP export later when cross-tool reuse becomes a need.

## Architectural choices in each ecosystem

### Roslyn (C# / .NET)

[Source: roslyn/docs/wiki/Roslyn-Overview.md](https://github.com/dotnet/roslyn/blob/main/docs/wiki/Roslyn-Overview.md),
[SemanticModel.cs](https://github.com/dotnet/roslyn/blob/main/src/Compilers/Core/Portable/Compilation/SemanticModel.cs)

**Compiler-as-platform.** Roslyn exposes its internal compiler
data structures as a queryable API. The three phases:

```
Parse  → SyntaxTree (immutable, lossless)
Bind   → Symbol table + SemanticModel
Emit   → IL bytecode
```

**Symbol-centric core.** Every API verb is symbol-based:
`GetSymbolInfo(node)`, `GetDeclaredSymbol(node)`,
`GetSpeculativeSymbolInfo(position, expression)`.

**Key concepts the API exposes:**

- `Compilation` — the unit of analysis (set of SyntaxTrees + references)
- `SyntaxTree` — per-file CST (immutable)
- `SemanticModel` — per-syntax-tree semantic facade (cached)
- `ISymbol` — interface for ANYTHING semantic (function, variable,
  type, namespace, parameter); discriminated by `Kind`
- **Position-based binding** — every query takes a character
  position; "what's in scope here?" "what symbol is at this
  position?"

**Speculative binding** is a notable feature: ask "if I had this
hypothetical expression at this position, what would it bind
to?" Used heavily for completion ranking and refactor preview.
`GetSpeculativeSymbolInfo(position, expression)`.

**Caching:** `SemanticModel.GetDiagnostics()` etc. internally
caches local symbols and analysis. The doc explicitly says
"reuse a single SemanticModel instance to amortize cost across
multiple queries on the same file."

### rust-analyzer

[Source: rust-analyzer architecture doc](https://rust-analyzer.github.io/book/contributing/architecture.html),
[Semantic Analysis deep-wiki](https://deepwiki.com/rust-lang/rust-analyzer/5-semantic-analysis)

**HIR (High-level Intermediate Representation)** layered above
the CST/AST. The major innovation is the **layered IR split**:

- `ItemTree` — per-file summary of top-level items (function
  signatures, type defs, module structure). **Stable across edits
  to function bodies.**
- `DefMap` — module tree, name resolution at module scope.
- `Body` — expression-level IR per function body.

The split enables the **core incremental invariant:**

> "typing inside a function's body never invalidates global
> derived data — if you change the body of `foo`, all facts
> about `bar` should remain intact"

**Salsa** is the engine that makes this work. Every analysis
query is a Salsa query that tracks its dependencies and
auto-invalidates when inputs change. The result: editing one
char in `foo`'s body recomputes only `foo`'s body-level
queries, not all of `bar`'s.

**HIR is NOT an API boundary.** It's INTERNAL. A facade `hir`
crate is the stable surface that IDE features import. The HIR
internals (`hir-def`, `hir-expand`, `hir-ty`) can refactor
freely.

**Lazy evaluation:** semantic info is computed only when the
IDE asks for it. Hover on an identifier? Resolve THAT
identifier. Don't pre-compute the whole crate.

[Matklad's retrospective (issue #8713)](https://github.com/rust-lang/rust-analyzer/issues/8713)
admits the syntax/semantics split has friction (going between
layers costs effort), but the incrementality wins justified
the choice.

### TypeScript (tsserver / tsgo)

[Source: TS LanguageService API wiki](https://github.com/microsoft/typescript/wiki/using-the-language-service-api),
[tsgo internals article](https://zenn.dev/mizchi/articles/tsgo-try-and-internal?locale=en)

**LanguageService is the persistent server.** A single
long-lived `Program` object holds the parsed files + dependency
graph. Each query is dispatched against this Program.

**Core principle: do the absolute minimum work.** Quoting the
docs:

> "All language service interfaces only compute the necessary
> level of information needed to answer a query."

Example: `getSyntacticDiagnostics(file)` parses but doesn't
bind. `getCompletionsAtPosition(file, pos)` binds only the
declarations contributing to the type in question.

**Host abstraction.** The LanguageService doesn't read files
directly. It delegates filesystem + module resolution to a
"host" interface the editor implements. This lets editors
manage document state (unsaved buffers, virtual files) without
the language service caring.

**tsgo** (the Go rewrite, in progress): mirrors the same
architecture in Go. Key APIs: `GetSymbolAtPosition(node)`,
`GetTypeOfSymbol(symbol)`. The internal layering is:
LSP → LanguageService → Program → AST + TypeChecker.

### Scala Metals + SemanticDB

[Source: Scalameta SemanticDB spec](https://scalameta.org/docs/semanticdb/specification.html),
[Metals architecture](https://github.com/scalameta/metals/blob/main/architecture.md)

**Decouple production from consumption.** The Scala compiler
emits `.semanticdb` files at compile time (under
`META-INF/semanticdb/`). Tools read these files independently.

Data model is simple:

- `TextDocument` — per-file: language, MD5 fingerprint,
  language-specific sections
- `Symbol` — a stable string ID (e.g., `_empty_/Main.`,
  `scala/Predef.println(+1).`)
- `Occurrence` — `(source-range, Symbol)` — maps a position
  to its symbol

Stored as Protobuf. Schema in [semanticdb.proto](https://github.com/scalameta/scalameta/blob/main/semanticdb/semanticdb.proto).

**Why this matters:** Metals uses SemanticDB so it doesn't
re-implement the Scala compiler. Scalafix (a refactor tool)
uses the SAME SemanticDB files. Any consumer can read them
without compiler internals.

**Metals fallback:** when build-server-emitted SemanticDB isn't
available (3rd-party deps, partial compiles), Metals' `mtags`
generates approximate SemanticDB from syntax alone. The
data model is the same; the producer differs.

### Sourcegraph SCIP

[Source: SCIP announcement](https://sourcegraph.com/blog/announcing-scip),
[scip-code/scip](https://github.com/sourcegraph/scip/)

**SCIP is "LSIF done right."** Inspired by SemanticDB; designed
to be easier to produce, debug, and consume than LSIF.

Key design points:

- **Protobuf, not JSON** (10-20% smaller; schema-typed)
- **Human-readable symbol string IDs** (debuggable)
- **Cross-language** (one consumer reads any language's index)
- **LSIF-convertible** for backward compat
- **Indexers exist for many languages:** scip-java, scip-typescript,
  scip-clang, scip-python, scip-ruby, scip-dotnet, etc.

**SCIP is THE on-disk format winning the long game.** Sourcegraph
deprecated LSIF for SCIP. rust-analyzer can export SCIP. GitLab
is moving toward SCIP support.

### Microsoft LSIF

The OG cross-tool format. Designed as "LSP serialized to a
graph." Useful conceptually (the graph model maps to LSP
queries) but practically harder to produce than SCIP, with
inconvenient JSON encoding and graph-walking semantics. GitLab
still uses LSIF; GitHub never adopted it (uses tree-sitter
instead).

For new tooling, SCIP > LSIF. LSIF stays relevant for legacy
consumers.

## Lessons distilled

Six design principles emerge across all five ecosystems:

### 1. Symbol-centric API

Every successful semantic model has **Symbol as the core noun.**
Roslyn's `ISymbol`, rust-analyzer's `hir::Symbol`, TS's `Symbol`,
SemanticDB's string IDs, SCIP's string IDs.

Every operation is:

- `SymbolAt(position) → Symbol`
- `Definition(symbol) → Position`
- `References(symbol) → []Position`
- `TypeOf(symbol) → Type`

Don't design around references or positions. Design around
symbols.

### 2. Position-based queries

The LSP world is position-driven (`textDocument/definition`,
`textDocument/hover` all take a position). The semantic model's
public API must accept positions and resolve to symbols. This
isn't optional.

### 3. Lazy evaluation

Roslyn caches per-question. TypeScript explicitly only computes
"the absolute minimum." rust-analyzer's salsa queries are
lazy-by-default. The pattern is universal: don't compute types
unless asked.

### 4. Layered IRs with locality

rust-analyzer's biggest insight: split the IR so editing one
region doesn't invalidate everything. ItemTree (signatures)
stays valid when Body (implementations) changes.

For Starlark, the analog is:

- **Module IR** — top-level statement signatures, load
  resolution, global bindings
- **Body IR** — per-function-body details

Editing a function body shouldn't invalidate module-level
analysis. This is the same locality principle.

### 5. In-memory primary, on-disk optional

The split between Categories 1 and 2 above. Pick **in-memory
primary** for an LSP (low latency, incremental); add **on-disk
export** (SCIP) when cross-tool reuse becomes a need.

### 6. Dialect / host pluggability

TypeScript's `LanguageServiceHost` abstracts filesystem.
Roslyn's `Compilation` abstracts references. starpls's
`starpls_bazel` provides Bazel-specific builtins.

For Starlark, the analog: dialect plug-in for builtins +
known-callables (Bazel knows `cc_library`; generic Starlark
doesn't). The dialect interface already exists in
`starlark-cst-go/dialect`; HIR just consumes it.

## What this means for `starlark-hir-go`

The semantic model plan should:

- Be **Symbol-centric** in API shape (Roslyn-inspired).
- Use **position-based queries** as the public entry point.
- Be **lazy** — don't compute types unless asked.
- Layer the IR (Module IR + Body IR) for locality.
- Stay in-memory primary; defer SCIP export until needed.
- Plug in dialect-provided builtins from
  `starlark-cst-go/dialect.Builtins`.
- Mirror our existing `incremental.Driver` pattern for
  incremental rebuilds.
- Treat the HIR internals as NOT-an-API-boundary (per
  rust-analyzer); the public surface is a thin facade.

## Open questions for the plan

1. **Symbol ID scheme.** rust-analyzer uses path-like IDs
   (`crate::module::function::local`). SemanticDB uses similar
   strings. SCIP requires string IDs. We need a stable scheme
   that survives reparses. Probably `file://path/<scope-path>/<name>`.
   Design before implementation.
2. **Salsa-equivalent in Go.** Go doesn't have salsa. The closest
   patterns are: hand-rolled memoization, the existing
   `incremental.Driver` pattern, or building a minimal
   query-DAG framework. Probably start with hand-rolled
   memoization keyed on `*GreenNode` identity (which we now
   share across reparses via Arena.WithInterner — token
   identity persists, which makes memoization keys stable).
3. **Module IR / Body IR boundary.** Where exactly does Module
   IR end and Body IR begin? Probably: Module IR covers
   top-level Statement-level info (def signatures without
   bodies, top-level assignments, loads). Body IR covers what's
   inside a suite. Needs design.
4. **SCIP export — when?** Sourcegraph integration for sky's
   Starlark files would be a real product win. But it's a
   v2+ artifact. Decide in 6 months when there's user demand.
5. **Cross-file resolution boundary.** Per-file HIR vs workspace
   layer. starpls keeps workspace work in a separate crate.
   We should too — `starlark-hir-go` is per-file;
   `bazel-cst-go/workspace` or a new `starlark-workspace-go`
   does cross-file.

## References

- [Roslyn Overview](https://github.com/dotnet/roslyn/blob/main/docs/wiki/Roslyn-Overview.md) — compiler-as-platform philosophy
- [rust-analyzer Architecture](https://rust-analyzer.github.io/book/contributing/architecture.html) — layered IR with salsa
- [Rust-analyzer semantic analysis (DeepWiki)](https://deepwiki.com/rust-lang/rust-analyzer/5-semantic-analysis) — incremental query design
- [SemanticDB Specification](https://scalameta.org/docs/semanticdb/specification.html) — on-disk format pioneered by Scala
- [Metals Architecture](https://github.com/scalameta/metals/blob/main/architecture.md) — production LSP built on SemanticDB
- [SCIP announcement](https://sourcegraph.com/blog/announcing-scip) — modern cross-tool index format
- [SCIP repo](https://github.com/sourcegraph/scip/) — protobuf schema + tooling
- [TypeScript LanguageService wiki](https://github.com/microsoft/typescript/wiki/using-the-language-service-api) — minimal-work principle
- [tsgo internals](https://zenn.dev/mizchi/articles/tsgo-try-and-internal?locale=en) — Go-port of TS LanguageService design
- [starpls](https://github.com/withered-magic/starpls) at `~/dev/refs/starpls` — direct reference for Starlark-specific design
