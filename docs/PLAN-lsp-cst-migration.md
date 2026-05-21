# PLAN — migrate sky LSP from buildtools-AST to CST stack

**Status:** Draft. Multi-session arc. Sequenced by handler.
**Owner:** @albertocavalcante
**Prereq:** `PLAN-cst-deps-bump.md` (the CST deps need to be current).
**Tracks:** unifying sky's TWO Starlark backends.

## The architectural gap today

Sky has TWO Starlark backends running in parallel inside the same
process:

| Path                             | Backend                                                 | Used by                                                                                               |
| -------------------------------- | ------------------------------------------------------- | ----------------------------------------------------------------------------------------------------- |
| `internal/starlark/formatter/`   | Pluggable: **Buildtools (default)** OR **CST (opt-in)** | `skyfmt`, LSP `handle_formatting`                                                                     |
| `internal/starlark/query/index/` | **bazelbuild/buildtools** (`build.File` AST) only       | LSP `handle_definition`, `handle_symbols`, `handle_completion`, workspace index — **most of the LSP** |

So the CST stack work I poured ~60 commits into this week — error
recovery, incremental reparse, the analysis trio, diagnostic codes —
is invisible to sky's LSP. The LSP goes through a separate
buildtools-based extractor that re-implements (less expressively)
what `starlark-cst-go/analysis` does.

This plan migrates `query/index` to use the CST stack so the LSP
inherits everything the CST stack provides:

- **P5 recovery shapes** — the LSP keeps giving completions /
  symbols on broken input (today: parse fails → empty response).
- **P6 incremental reparse** — keystroke-grade latency for
  `documentDidChange` on large files (today: full reparse every
  change).
- **Diagnostic codes** — `textDocument/diagnostic` returns
  `SCST001`-style codes editors can dispatch on, plus the new
  Hint field surfaces as quick-fix candidates.
- **Token identity across reparses** — annotation caches on the
  server side (sky has hover, inlayHints, codeAction caches)
  stay valid through edits because text-bearing tokens
  pointer-share via `Arena.WithInterner`.

## Strategy

**Coexistence, not replacement.** Keep the buildtools path
working; add a CST-backed path behind a feature flag; migrate
handler-by-handler with comparison tests; flip the default per
handler once each is at parity. Same shape as the formatter's
`Default = Buildtools` story (PLAN-cst-library-versioning.md
Phase 3 ish), just now applied to the LSP's query layer.

The point is to never block sky's LSP on the migration. If a
handler doesn't have a CST path yet, it uses buildtools. If it
has one but feature-flagged off, it uses buildtools. Once the
CST path is proven, flip its default.

## Phasing

### Phase 0 — scaffolding (1 session)

- Add a `Backend` enum to `internal/starlark/query/index` similar
  to formatter's `Engine` interface:
  ```go
  type Backend interface {
      Name() string  // "buildtools" or "cst"
      Extract(src []byte, path string, kind filekind.Kind) *Index
  }
  ```
- Wrap the existing extract logic as `BuildtoolsBackend` (no
  behavior change).
- Stub a `CSTBackend` returning `ErrBackendDoesNotSupport` for
  every kind. Plumb the env-var / flag selector
  (`SKY_QUERY_BACKEND=cst` or similar).
- Comparison test harness: given a source + file kind, run both
  backends and assert structural equivalence on the bits the LSP
  actually uses (def names, def positions, assign LHS positions,
  load symbols, call sites).

**Exit gate:** sky's tests still pass on buildtools default;
running with `SKY_QUERY_BACKEND=cst` gives the
`ErrBackendDoesNotSupport` error path that handler-level fallback
catches.

### Phase 1 — `documentSymbol` on CST (1 session)

Simplest handler — just enumerates defs/assigns/loads with
positions. Maps directly to:

- `analysis.FindIdentifierDeclarations(file, name)` — but want ALL
  declarations, not by name. → add `analysis.AllDeclarations(file)
  []IdentifierReference` (new helper, ~30 LOC).
- `ast.LoadStatement` + `ast.DefStatement` already accessible.

Comparison test: for each file in the sky corpus, run both
backends and assert documentSymbol output is byte-identical (modulo
formatting). Where the CST path disagrees with buildtools, it's
either a real divergence (file the bug) or an enrichment (CST has
richer info — e.g., context tags). Decide per case.

**Flip:** once comparison shows zero structural disagreements on
the corpus, make CST the default for `handle_documentSymbol` (gated
behind config but env-var-default-on).

### Phase 2 — `definition` on CST (1 session)

Needs more than per-file analysis — sky's `handle_definition`
crosses files via workspace index. Plan:

- Per-file: use `analysis.FindIdentifierDeclarations(file, name)`
  to find local bindings.
- Cross-file: keep sky's existing `workspace.FindDefinitionInFile`
  which already accepts a path + name. The CST-backed extractor
  just needs to feed it the same shape.
- Load-binding resolution: `ast.LoadStatement.Bindings()` gives
  `(name, source, span)` per binding — drop-in for sky's existing
  load-graph code.

**Flip:** same comparison-corpus gate.

### Phase 3 — `completion` on CST (1-2 sessions)

Most subtle. Sky's `handle_completion` does scope-aware completion
(within-function locals, builtins, loaded symbols). The CST
analysis trio is name-textual; **scope analysis is not in CST stack
today**. Two options:

- **Option A — keep scope logic in sky, use CST for extraction.**
  Sky layers its scope walk on top of `analysis.FindIdentifier*`
  results. Self-contained: no new CST capability needed.
- **Option B — add `analysis.Scopes(file)` to CST stack.** A
  primitive that returns scope-trees; consumers query for "what's
  in scope at this position". More work, more reuse downstream.

Recommend Option A for v1 (faster) and reconsider after sky's
scope code is exercised.

### Phase 4 — incremental reparse integration (1 session)

This is the LSP latency win. Sky's `internal/lsp/handle_textdocument.go`
should hold a per-document `*incremental.Driver` (with an
`*Interner` so token identity persists across reparses), and on
`textDocument/didChange` use `Driver.Reparse(edit)` instead of a
fresh `parser.ParseFile`.

The `Reparse` mode field tells sky whether the reparse was a
full-parse, edited-range, or nested-statement reparse — useful
for instrumentation but invisible to LSP clients.

**Flip:** behind an env var (`SKY_LSP_INCREMENTAL=1`) initially,
then default-on once the cache invalidation story is sorted
(annotation caches keyed by `*GreenNode` need to be aware that
old pointers may persist across reparses via interner — that's
the SHARING win, not a bug).

### Phase 5 — diagnostic codes (small, can interleave)

Sky's `handle_diagnostics` currently surfaces parse errors with
generic messages. Once on CST:

- Each diagnostic carries `Code` (`SCST001` etc.), `Severity`,
  `Hint`, and `Range`.
- Wire those to LSP `Diagnostic.code`, `Diagnostic.codeDescription`,
  and `Diagnostic.relatedInformation` per LSP spec.
- Editors gain quick-fix surface (the `Hint` field) at zero
  additional cost on sky's side.

This is a small interleavable enhancement, not its own phase.

### Phase 6 — RenameSymbol-backed `textDocument/rename` (1 session)

Once the analysis trio is in the LSP path, wiring
`refactor.RenameSymbol` to the rename handler is ~50 LOC:

- `prepareRename` returns the identifier span at cursor.
- `rename` runs `RenameSymbol(oldName, newName)` and translates
  the byte-level edits to `WorkspaceEdit` LSP format.

Scope-unawareness is a known limitation (documented in
`RenameSymbol`'s godoc) — caller layers scope analysis on top.
For v1 do textual rename file-local; users get a
warning when the rename might cross scopes.

## Out of scope for this plan

- **LSP runtime / WASM distribution** — see
  `PLAN-lsp-wasm-distribution.md`. Separate concern.
- **Workspace-level semantic model** — multi-file type / scope
  resolution. Big arc; rules itself out for the per-handler
  migration approach above.
- **Cross-file FindReferences** — works on the per-file primitive
  if the workspace index is augmented. Add when a consumer asks.

## Sequencing

A → 1 → 2 → 5 (interleave) → 3 → 6 → 4

Phase 4 (incremental) is last because it touches state management
across the server — needs the per-handler paths to be CST-native
first. The other phases compound: each makes the next easier
because the extraction layer is shared.

Total estimated effort: 6-8 focused sessions.

## Open questions

1. Should the `Backend` selector be a flag, env var, config option,
   or `--engine=cst` like the formatter's flag? Lean toward env
   var initially (no config-file churn), config option after the
   default flips.
2. The comparison-test corpus — sky already has corpus files
   somewhere (`internal/starlark/formatter/testdata`?). Reuse or
   create a per-handler one?
3. What's the right error message when the CST backend hits an
   unsupported file kind? Today's `ErrEngineDoesNotSupport`
   pattern from formatter is the right template.

## Acceptance criteria for the arc

- All six handlers tested via comparison harness against
  buildtools baseline.
- `SKY_QUERY_BACKEND=cst` default-on in CI for at least 2 weeks
  before becoming user-visible default.
- Incremental reparse measurably reduces P95 didChange→response
  latency on a 1000-line MODULE.bazel test fixture.
