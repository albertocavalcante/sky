# PLAN — refresh CST gap analysis + plan per-filekind default-flip

**Status:** Draft. Decision-heavy, not code-heavy. 1-2 hours of audit + writeup.
**Owner:** @albertocavalcante
**Prereq:** `PLAN-cst-deps-bump.md` so the gap analysis runs against current CST.
**Tracks:** when `formatter.Default = CST` becomes safe.

## Context

`internal/starlark/formatter/engine.go` documents `Default =
Buildtools` with a comment explaining why CST isn't the default:

> Currently Buildtools (upstream-stable). Will flip to CST —
> partially or per-kind — once CST gains the canonicalization
> passes that build.Format performs on NON-canonical input.

The comment then lists four specific gaps:

1. Line reflow (single-line → multi-line when long; collapse short multi-lines)
2. Trailing comma insertion on multi-line collections
3. Quote-style normalization (`'` → `"`)
4. Some inter-token whitespace normalization

**Three of the four are out-of-date.** Today's CST stack
buildifier pipeline shipped:

- `add-trailing-comma` (trailing commas on multi-line collections) ✅
- `normalize-quotes` (`'` → `"` with safe-text gating) ✅
- `expand-long-calls` (long single-lines → multi-line) ✅ half of (1)

The actual remaining gap is **only the inverse half of (1)**:
_collapse_ short multi-line calls to single-line. Plus possibly
some inter-token whitespace nuances (4).

Sky's `Default = Buildtools` is therefore more conservative than
the actual capability gap requires. This plan audits the _real_
gap and proposes a per-filekind flip strategy.

## Audit method

Three artifacts to compare:

1. **CST output** — `bazel-cst-go/format/buildifier.FormatBytes(src)`
   on each fixture
2. **Buildtools output** — `build.Format(file)` on the same
3. **Diff** — what does buildtools do that CST doesn't?

Sources for fixtures:

- `internal/starlark/formatter/testdata/` (existing sky corpus)
- `internal/starlark/formatter/engine_test.go` (golden tests)
- A new "non-canonical input" corpus: take real BUILD/MODULE files,
  _un_-format them deliberately (collapse multi-lines, swap quote
  style, strip trailing commas), then compare both engines.

The non-canonical corpus is where the gap surfaces. Canonical input
already produces near-identical output (sky's earlier validation
measured 98.7% byte-equivalence on already-buildifier'd files —
that doesn't tell us about un-formatted input).

## Per-filekind flip strategy

Different file kinds have different risk profiles for a default
flip:

| Kind                           | Gap exposure                                               | Flip recommendation               |
| ------------------------------ | ---------------------------------------------------------- | --------------------------------- |
| `KindMODULE`                   | Low — MODULE files are typically well-formatted by tooling | Safe to flip first                |
| `KindBUILD`                    | Medium — many hand-written BUILD files                     | Audit per-rule; flip after MODULE |
| `KindBzl`                      | Higher — generic Starlark + macros, more shape variance    | Flip last                         |
| `KindWORKSPACE`                | Low — small files, less reformatting needed                | Safe to flip with MODULE          |
| `KindWORKSPACEBzlmod`          | Very low — usually empty or tiny                           | Safe                              |
| `KindStarlark` (generic .star) | Variable — sky's neutral mode probably already fine        | Keep buildtools or neutral        |
| `KindBUCK`                     | N/A — Buck2 routes to Neutral (no buildifier-equivalent)   | Already CST-routed                |

### Phase 1 — MODULE-only flip (1 day, low risk)

Steps:

- Audit the gap on MODULE.bazel-style files specifically:
  bench `FormatBytes(unformatted_module)` vs
  `build.Format(unformatted_module)` on a corpus of 50+ MODULE
  files from real Bazel repos.
- If gap is zero / negligible, change the dispatch in
  `formatter.Default`'s implementation (or add a `DefaultForKind(k)`
  helper):

  ```go
  func DefaultForKind(k filekind.Kind) Engine {
      switch k {
      case filekind.KindMODULE, filekind.KindWORKSPACEBzlmod:
          return CST
      }
      return Buildtools
  }
  ```

- Update the doc comment with the audit findings.

### Phase 2 — BUILD flip (2-3 days, medium risk)

Same audit on BUILD files. The medium risk is rule-attribute
sorting differences — buildifier has a long list of
rule-specific arg orders; CST stack ships the buildifier
NamePriority table verbatim (per the `buildifier_tables.go`
port in `starlark-refactor-go`), so divergences should be
narrow.

Expected divergences to investigate:

- Inter-call whitespace (extra blank between consecutive
  `cc_library` calls?)
- Multi-line call expansion thresholds
- Attribute ordering in unusual rules

If audit comes back clean (or with documented small
divergences), add BUILD to the flip list:

```go
case filekind.KindMODULE, filekind.KindWORKSPACEBzlmod, filekind.KindBUILD:
    return CST
```

### Phase 3 — .bzl files (1 week, higher uncertainty)

.bzl macros have the widest shape variance. Audit needs a
larger corpus (rules_go, rules_python, rules_cc, plus
hand-written macros from real projects). Document any
systematic divergences before flipping.

The CST-stack-specific concern: macros with `def helper(x):
suite` benefit greatly from P5 structural recovery (the LSP
gets a partial tree on broken input). The buildifier engine's
behavior on broken input is to refuse to format. Sky users
get better-on-broken behavior under CST — that's a UX win even
if formatted output is identical.

### Phase 4 — generic Starlark (no flip planned)

Generic `.star` / `.sky` files outside the Bazel ecosystem
don't need buildifier-style opinions. Stay on neutral mode
(already shipped) or buildtools. No flip; document why.

## What the flip is NOT

- **Not a hard break.** Existing `--engine=buildtools` continues
  working forever. The flip changes only `Default`.
- **Not a one-way door.** Per-filekind flips can be reverted
  trivially if a regression surfaces.
- **Not a perf claim.** Some passes may be slower under CST
  (the formatter pipeline allocates more). The flip is about
  semantics, not perf.

## Out of scope for this plan

- **The opposite-direction line-reflow pass** (collapse short
  multi-lines). That's a new refactor pass to write — would
  live in `starlark-refactor-go` (dialect-agnostic) or
  `bazel-cst-go/refactor` (if Bazel-flavored heuristics
  matter). Separate arc.
- **Cross-formatter migration tools.** If users have
  `.buildifier-config` files or `.bcr.format` settings, those
  belong in their own migration plan when relevant.

## Acceptance criteria

- Refreshed `engine.go` `Default`'s docstring with current gap.
- A `formatter.DefaultForKind(k) Engine` helper that routes
  MODULE and WORKSPACEBzlmod to CST by default.
- A docs/cst-flip-audit-RESULTS.md (or similar) capturing the
  comparison-corpus findings.
- Sky test suite passes; no user-visible formatter changes for
  any file kind NOT in the flip list.

## Why this matters

The Default = Buildtools comment is stale and discourages
adoption. Refreshing it surfaces the real (much smaller) gap
and unblocks per-filekind flips that are already safe to do.
Each flip increases CST stack usage in production, which
surfaces real bugs faster.

The honest end-state isn't "CST replaces buildtools everywhere".
It's "CST is the default where it's at parity; buildtools stays
the escape hatch for the remaining edge cases". That's a
healthy multi-engine architecture.
