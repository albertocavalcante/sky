# PLAN — new skylint rules powered by CST analysis

**Status:** Draft. ~2-4 sessions, incremental (per rule).
**Owner:** @albertocavalcante
**Prereq:** `PLAN-cst-deps-bump.md`.
**Tracks:** new lint rules `skylint` ships, powered by the analysis trio.

## Why this plan exists

`internal/starlark/linter/` already has a rule registry +
fix-applier infrastructure. Today's rules ship via the
`buildtools/` subdirectory — they're buildifier-style lints
applied via the buildtools AST. The CST stack analysis trio
(`FindIdentifierReferences/Declarations/Uses`) is exactly the
primitive that **structural / semantic** lint rules need —
the kinds buildifier doesn't ship.

Today's gap: sky's lint catalog is "buildifier-shaped". It's
strong on layout / convention rules. It's weak on the
identifier-level semantic checks that LSPs and IDEs are
expected to provide.

This plan documents a set of new lint rules to ship,
prioritized by user value, each implementable in ~50-150 LOC
on top of the CST analysis trio.

## Proposed rules

### `SKY-unused-load` — unused symbols in a load

**Diagnostic:** load imports symbol `foo` but `foo` is never
used in this file.

**Detection:** for each `load(...)` binding name, check that
`FindIdentifierUses(file, name)` is non-empty. If empty, the
load symbol is unused.

**Fix:** automated. Use `refactor.RemoveLoadSymbol(modulePath,
symbolName)` (already in starlark-refactor-go).

**Notes:** mirrors `buildifier --warnings=unused-load`. The
CST version benefits from `RemoveLoadSymbol`'s handling of
aliased imports (`alias = "src"`) — the alias gets removed
correctly, not blindly.

### `SKY-undefined-identifier` — use of unbound name

**Diagnostic:** identifier `x` is used but never defined in
this file (and not from a load, not a known builtin).

**Detection:** for each identifier USE
(`FindIdentifierUses(file, name)`), check that:

- A matching DECLARATION exists (`FindIdentifierDeclarations`),
  OR
- It's a known builtin (`builtins.Registry.Has(name)`),
  OR
- It's a global imported via a `load(...)` binding.

If none, it's undefined.

**Fix:** none automated. Quick-fix could suggest "add load
for `name`" via `refactor.AddLoadSymbolOrCreate` if the
identifier matches a known module's exports — but cross-file
knowledge is needed; punt for v1.

**Notes:** This is THE canonical static check that a Starlark
linter should ship. Buildifier doesn't have it (it's
syntactic-only).

### `SKY-shadowed-binding` — local shadows outer

**Diagnostic:** `def helper(x): ... x = 1` — `x` shadows the
parameter.

**Detection:** walk DefStatements; for each `ContextDefParam`
binding, check whether any `ContextAssignLHS` inside the def's
suite has the same name.

This needs basic scope-walking (which suite contains which
binding) — small extension over the existing helpers.

**Fix:** none automated (rename is a user choice).

### `SKY-redundant-load` — duplicate import of same symbol

**Diagnostic:** two distinct `load(...)` calls in the same
file import the same symbol from the same module.

**Detection:** walk top-level load statements; build a
`{(modulePath, symbolSource): []span}` map. Flag any key
with `len(spans) > 1`.

**Fix:** automated. Remove all but the first occurrence via
`refactor.RemoveLoadSymbol`.

### `SKY-unused-param` — function parameter never read

**Diagnostic:** `def helper(x, y): return x` — `y` is never
used.

**Detection:** for each def's ContextDefParam, check
whether USES of that name occur inside the def's suite
(needs scope walk again).

**Fix:** automated. Use `refactor.RenameRuleAttribute` is
the wrong tool; rename the param to `_` to silence, OR
remove (but removal changes the function signature — risky).
For v1, propose-only fix that renames `x` → `_x`.

**Notes:** common in Python/Starlark style.

### `SKY-load-symbol-order` — symbols inside a load not sorted

Already covered by `SortLoadSymbols` in the formatter
pipeline. If the formatter runs, this is enforced. The
LINT version surfaces it as a diagnostic when the
formatter ISN'T running (e.g., `skylint` without
`skyfmt --fix`).

**Fix:** delegate to `refactor.SortLoadSymbols`.

### `SKY-trailing-comma-multiline` — missing trailing comma

Same story as load-symbol-order — covered by formatter's
`add-trailing-comma`. Lint surfaces when formatter isn't
running.

**Fix:** delegate to `refactor.AddTrailingCommaInCollections`.

## Implementation pattern

Each rule is a single Go file under `internal/starlark/linter/cst/`:

```go
// rule_unused_load.go
package cst

import (
    "github.com/albertocavalcante/starlark-cst-go/analysis"
    "github.com/albertocavalcante/starlark-cst-go/ast"
    "github.com/albertocavalcante/starlark-cst-go/parser"
    refactor "github.com/albertocavalcante/starlark-refactor-go"

    "github.com/albertocavalcante/sky/internal/starlark/linter"
)

var UnusedLoad = linter.Rule{
    ID:       "SKY-unused-load",
    Severity: linter.SeverityWarning,
    Check:    checkUnusedLoad,
    Fix:      fixUnusedLoad,
}

func checkUnusedLoad(src []byte, ...) []linter.Diagnostic {
    tree := parser.ParseFile(src)
    file, _ := ast.AsFile(tree.Root())
    var out []linter.Diagnostic
    for _, load := range file.LoadStatements() {
        for _, binding := range load.Bindings() {
            uses := analysis.FindIdentifierUses(file, binding.Name)
            if len(uses) == 0 {
                out = append(out, linter.Diagnostic{
                    Rule:    "SKY-unused-load",
                    Span:    binding.TokenSpan(),
                    Message: "unused load symbol: " + binding.Name,
                })
            }
        }
    }
    return out
}
```

The fix function is similarly thin — call the corresponding
`refactor.*` Pass and return its edits.

## Phasing

### Phase 1 — `SKY-unused-load` (1 session)

The simplest and highest-impact rule. ~80 LOC including tests.
Validates the integration pattern.

### Phase 2 — `SKY-redundant-load` + `SKY-trailing-comma-multiline` (1 session)

Two more rules that map cleanly onto existing refactor passes.

### Phase 3 — `SKY-undefined-identifier` (1 session)

Higher-value rule. Needs the builtin-registry integration via
`bazel-cst-go/dialect.Builtins` (already shipped). Surface
quick-fix as "add load for `name`" when the name matches a
known module export.

### Phase 4 — `SKY-shadowed-binding` + `SKY-unused-param` (1 session)

Both need basic scope walking. Decide whether to:

- Extend `starlark-cst-go/analysis` with a tiny scope-walker
  primitive (reusable in LSP migration too — see
  `PLAN-lsp-cst-migration.md` Phase 3), OR
- Keep scope logic in sky-side (simpler for v1).

## Out of scope

- **Reporter integration** — these rules feed sky's existing
  reporter pipeline (text/json/github). No new reporter work.
- **Editor integration** — quick-fix is automatic via existing
  LSP `codeAction` plumbing once rules are registered.
- **Cross-file lints** — "this symbol is unused across the
  whole repo" requires a workspace index. Add to sky's
  existing `workspace.go` infrastructure when relevant.

## Acceptance for the arc

- 4-6 new lint rules registered, each with check + fix +
  golden tests.
- `skylint --rules=SKY-*` lists the new rules.
- Each new rule has at least one positive case + one
  negative case + (where applicable) a `--fix` test.
