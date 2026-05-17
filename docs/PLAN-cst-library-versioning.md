# PLAN — CST library versioning migration

**Status:** in-progress — phase 1 (local replace + multi-checkout) shipped in PR #53.
**Owner:** @albertocavalcante
**Tracks:** the migration from `replace ../foo` to versioned `require` for the
three private library repos that back the `formatter.CST` engine.

## Context

`formatter.CST` is layered on three sibling Go module repos:

- `github.com/albertocavalcante/starlark-cst-go` — Roslyn-style green/red CST + parser
- `github.com/albertocavalcante/bazel-cst-go` — Bazel dialect (BUILD/.bzl/MODULE)
- `github.com/albertocavalcante/starlark-format-go` — neutral spec-only formatter

All three are **private** GitHub repos. Sky pulls them in via `replace`
directives in `go.mod`, and CI multi-checkouts them as siblings:

```
<workspace>/
  sky/                  ← this repo
  starlark-cst-go/      ← sibling (LIBRARIES_PAT-checkout in CI)
  bazel-cst-go/         ← sibling
  starlark-format-go/   ← sibling
```

This is the **simplest working setup** that lets:

- local devs iterate on libs without publishing
- CI build sky end-to-end against private libs (PAT secret)
- the divergence-guard workflow (`format-compare.yml`) reproduce real numbers

It is **not** the long-term steady state.

## Why we want to move off it

| Pain                  | Today                                       | Wanted                             |
| --------------------- | ------------------------------------------- | ---------------------------------- |
| Reproducibility       | every consumer must clone 3 siblings        | `go get` resolves a pinned version |
| Cross-repo CI auth    | LIBRARIES_PAT secret in every consumer repo | GOPRIVATE + token works once       |
| Version pinning       | implicit (whatever's on `main`)             | explicit (semver tag)              |
| `go mod tidy` UX      | needs siblings on disk                      | works with module cache            |
| External contributors | impossible (need access to 3 repos)         | possible once libs go public       |

## Migration phases

### Phase 1 — local `replace` + multi-checkout CI ✅ (shipped)

- `go.mod` has 3 relative `replace` directives (`../starlark-cst-go`, etc.)
- CI workflows checkout sky into `./sky/` and siblings at workspace root
- Uses `LIBRARIES_PAT` secret (PAT with `repo` scope on the 3 private repos);
  falls back to `secrets.GITHUB_TOKEN` for the day the libs go public
- See `.github/workflows/ci.yml` and `.github/workflows/format-compare.yml`

**Limitation:** the relative paths only work in the canonical layout.
Anyone trying to use sky as a Go module gets `directory does not exist`.

### Phase 2 — tag library releases

Prerequisite: a stable surface in each library.

- `starlark-cst-go v0.1.0` — parser + dialect interface API frozen
- `bazel-cst-go v0.1.0` — buildifier pipeline at current 99.76% parity
- `starlark-format-go v0.1.0` — Neutral.Format signature frozen

Each release goes out via `gh release create` with a changelog entry and the
go.sum hash recorded in the consumer's `go.sum`.

### Phase 3 — switch sky to versioned `require`

In sky:

```
require (
  github.com/albertocavalcante/starlark-cst-go v0.1.0
  github.com/albertocavalcante/bazel-cst-go v0.1.0
  github.com/albertocavalcante/starlark-format-go v0.1.0
)
// no more replace directives
```

Sky's CI workflows drop the 3 sibling checkouts. The only auth surface
becomes `GOPRIVATE` + `~/.netrc` (or GitHub App token) so `go mod download`
can fetch private modules. One env var instead of 9 lines of checkout YAML.

Local devs who want to iterate on a library against sky use a **gitignored**
`go.work`:

```
go 1.26
use (
  ./sky
  ./starlark-cst-go     // optional, only if iterating
  ./bazel-cst-go        // optional
  ./starlark-format-go  // optional
)
```

`go.work` is already in `.gitignore`. No `replace` directives needed.

### Phase 4 — libraries go public (optional)

If/when the libraries become public:

- `LIBRARIES_PAT` secret is removed (the `secrets.GITHUB_TOKEN` fallback
  starts working)
- `GOPRIVATE` is unset
- External contributors can build sky end-to-end

This is reversible and orthogonal to the versioning move.

## Open questions

- **Replace-directive sync.** During phase 2 (between cutting library
  releases and switching sky's require), we'll briefly have BOTH versioned
  requires and replace directives. The replace wins. Plan: do the cut
  atomically in one PR per library.
- **Pre-release iteration.** If we need to ship a feature in sky that
  requires a library change, the options are: (a) tag a `-rc.N` release of
  the library, (b) temporarily add a `replace` to a commit SHA, (c) hold
  the sky change. Default to (a) for shippable features, (b) for plumbing.
- **Cross-library version compat.** `bazel-cst-go` depends on
  `starlark-cst-go`. We need a release order: cut `starlark-cst-go` first,
  then `bazel-cst-go` bumps its require, then we cut `bazel-cst-go`.

## Out of scope here

- Buck2 dialect (`buck2-cst-go`) — same pattern, deferred until there's
  Buck2 demand
- Bazel-side integration (`MODULE.bazel` `bazel_dep` for the 3 libs) —
  tracked in PR #53's "Caveats" section
