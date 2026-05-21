# PLAN — bump CST stack deps to the 2026-05-21 state

**Status:** Ready to ship. ~30 min of work + test.
**Owner:** @albertocavalcante
**Tracks:** sky's `go.mod` dependencies on the CST library stack.
**Prereq for:** PLAN-lsp-cst-migration (the LSP migration arc).

## Context

The CST stack landed a major arc on 2026-05-20 — 2026-05-21 spanning ~60
commits across four sibling repos. Sky's `go.mod` is pinned to
**pre-arc** versions:

```
github.com/albertocavalcante/bazel-cst-go         v0.0.0-20260518153803-...
github.com/albertocavalcante/starlark-cst-go      v0.0.0-20260517131122-...
github.com/albertocavalcante/starlark-format-go   v0.0.0-20260517140007-...
```

Missing from sky:

- **P5 error recovery** — diagnostic codes (`SCST001`…`SCST009`),
  Severity / Hint / Range fields, `KindMissing` structural primitive,
  if/def/for/load statement structural recovery, Pratt expression
  anchor-set recovery. Sky's LSP gains diagnostic codes to dispatch
  on (was Message-only) and richer partial-trees on broken input.
- **P6 incremental reparse** — `incremental.Driver` with three
  strategies (full-parse-share, edited-statement-reparse,
  nested-statement-reparse) plus end-to-end token interning. On a
  1000-statement file, a single-char edit reparse drops from 682µs
  full-parse → 20µs level-1 (~34x). Token identity is now shared
  across reparses via `Arena.WithInterner` — annotation caches
  keyed by `*GreenNode` get cache hits.
- **Analysis trio** — `FindIdentifierReferences` /
  `FindIdentifierDeclarations` / `FindIdentifierUses` in
  `starlark-cst-go/analysis`. Direct foundation for LSP
  documentSymbol, definition, references, renameSymbol, highlight.
  Spans computed at red layer (survive token interning).
- **starlark-refactor-go** (new dep) — 20+ dialect-agnostic refactor
  passes including `RenameSymbol`, `MoveTarget`,
  `AddLoadSymbolOrCreate`, `RewriteLoadPathRegex`,
  `RewriteRuleAttribute`. Plus combinators: `Chain`, `ChainNamed`,
  `Repeat`, `When`, `First`.
- **bazel-cst-go/format/buildifier** — the formatter pipeline now
  composes via `ChainNamed` (structured per-step error wrapping)
  and includes the full set of MODULE.bazel-flavored passes.
- **TriviaInterner + TokenInterner + Arena.WithInterner** —
  opt-in for long-lived consumers (LSP sessions reparsing one
  file many times). Reduces cross-edit memory churn.

The CST repos are now hosted at `git.alberto.engineer/adsc/*` as
primary with GitHub dual-mirror, but the Go module paths remain
`github.com/albertocavalcante/...` — no consumer-side change needed.

## Plan

### Step 1 — bump versions in `go.mod`

Run from sky's repo root in a worktree:

```bash
go get \
  github.com/albertocavalcante/starlark-cst-go@latest \
  github.com/albertocavalcante/bazel-cst-go@latest \
  github.com/albertocavalcante/starlark-format-go@latest \
  github.com/albertocavalcante/starlark-refactor-go@latest \
  github.com/albertocavalcante/buck2-cst-go@latest

go mod tidy
```

`starlark-refactor-go` and `buck2-cst-go` will be added (transitively
required by bazel-cst-go now that it depends on refactor-go for the
ChainNamed-based pipeline).

### Step 2 — run sky's tests

```bash
bazel test //...        # or `just test` if a justfile target exists
go test ./...
```

Expected: all pass. If `internal/starlark/formatter/engine_test.go`
or `engine_cst.go` reference removed/renamed symbols, fix those.

### Step 3 — refresh stale documentation

The `Default` engine docstring in
`internal/starlark/formatter/engine.go` claims CST is missing
"Trailing comma insertion" and "Quote-style normalization" — both
of those ARE now in the buildifier pipeline (have been for weeks,
but the doc never updated). Refresh that comment so the
remaining-gap list is accurate. The real remaining gaps are:

- **Line reflow toward compactness** (collapse short multi-lines).
  `expand-long-calls` does the opposite direction (expand long
  single-lines); collapse-short-multilines isn't shipped.
- Some inter-token whitespace normalisation (parts handled by
  neutral, parts not).

That doc refresh decouples the actual CST gap from the dated
gap-list it currently presents.

### Step 4 — commit + push

Single chore commit:

```
chore: bump CST stack deps to 2026-05-21

- P5 error recovery (diagnostic codes, KindMissing, structural recovery)
- P6 incremental reparse (3 strategies, end-to-end token interning)
- analysis trio (FindIdentifierReferences/Declarations/Uses)
- starlark-refactor-go added as direct dep (20+ passes + combinators)
- buildifier pipeline now composes via ChainNamed
- Refresh engine.go Default-flip gap doc
```

## Risks

- **None known.** The CST stack tests pass on the new deps; the bench
  numbers improved (parser allocations down ~40%, FormatBytes
  pipeline allocations down ~20%); the equivalence fuzz harness has
  ~10M iterations clean.
- **One subtle case to verify:** sky imports `starlark-cst-go/parser`
  and may construct `parser.Tree` values directly. The new
  `parser.NewTree(root, diagnostics)` and
  `parser.ParseFileWithInterner(src, interner)` constructors are
  additive — existing call sites unchanged.
- **`go.work` for local dev** — per
  `docs/PLAN-cst-library-versioning.md` (Phase 2), local
  development uses a gitignored `go.work` to override pseudo-
  versions with sibling-clone paths. After this bump, sibling
  clones need `git pull` to match.

## Why now

Three reasons:

1. **The CST stack has hit a real correctness milestone** — the
   incremental driver has a formal equivalence invariant under fuzz,
   structural recovery captures every minimum-bar case from
   proposal-02. Bumping NOW gets sky those guarantees.
2. **Prereq for LSP migration** — `PLAN-lsp-cst-migration.md`
   wants the analysis trio, the incremental driver, and the
   diagnostic codes. None of those exist on the old pseudo-version.
3. **The buildifier pipeline got ChainNamed structured errors** —
   sky's user-facing formatter errors become `chain[sort-loads]: ...`
   instead of `buildifier: pass 3: ...`. Better debuggability for
   free.
