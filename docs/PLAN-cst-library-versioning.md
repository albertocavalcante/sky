# PLAN — CST library versioning migration

**Status:** Phase 2 shipped — Go pseudo-versions, octo-sts auth. Tagged releases (Phase 3) intentionally deferred.
**Owner:** @albertocavalcante
**Tracks:** how sky depends on the three private CST library repos.

## Context

`formatter.CST` is layered on three sibling Go module repos:

- `github.com/albertocavalcante/starlark-cst-go` — Roslyn-style green/red CST + parser
- `github.com/albertocavalcante/bazel-cst-go` — Bazel dialect (BUILD/.bzl/MODULE)
- `github.com/albertocavalcante/starlark-format-go` — neutral spec-only formatter

All three are **private** GitHub repos and intentionally **pre-1.0** — APIs are still evolving, so semver-tagged releases would be premature. Pinning happens via Go pseudo-versions (`v0.0.0-YYYYMMDDhhmmss-{12-char-hash}`), which give exact-commit reproducibility without making a stability promise.

## Migration phases

### Phase 1 — local `replace` + multi-checkout CI ✅ (PR #53, superseded)

`go.mod` had `replace github.com/.../foo => ../foo` for each lib; CI checked out sky and the 3 libs as siblings so the relative paths resolved.

**Why we moved off it:** the relative paths only worked in one specific filesystem layout. Any consumer outside that layout (a fresh `go get`, a sky worktree at a different depth, an external developer) saw `directory does not exist`.

### Phase 2 — Go pseudo-versions + octo-sts module auth ✅ (current)

- `go.mod` requires real pseudo-versions for each lib (`v0.0.0-{ts}-{hash}`)
- `replace` directives gone from `go.mod`
- `.github/actions/setup-cst-private-auth` composite mints 3 short-lived (1h) octo-sts tokens via OIDC and configures `GOPRIVATE` + per-repo `git insteadOf` so `go mod download` transparently authenticates
- Single-checkout CI — no more `path: sky` shuffle
- Local dev: gitignored `go.work` overrides the pseudo-versions with absolute paths to sibling clones (no `replace` in `go.mod`)
- Each lib's `go.mod` likewise pins its sibling deps via pseudo-versions, with `replace` directives kept commented out for opt-in local iteration

**Bumping a lib:** push to its `main`, then in sky run `go get github.com/.../foo@<new-hash>` (auth set up locally via `gh auth token` + `git config url.insteadOf`). Commit the `go.mod`/`go.sum` change.

**What changes per lib bump:** if a lib has sibling deps (bazel-cst-go → starlark-cst-go + starlark-format-go), bump them in the lib's `go.mod` first, push, then bump sky to the new lib pseudo-version. Two pushes per lib, predictable.

### Phase 3 — tagged releases (deferred, on hold)

Will happen when:

- Each lib's surface stabilises enough to commit to semver
- We want sky installable without GitHub auth (public modules + `go get` from proxy.golang.org)
- External contributors arrive

Migration to Phase 3 is a one-line `go.mod` change per lib (`v0.0.0-{ts}-{hash}` → `v0.1.0`) plus a `gh release create` per lib. The auth machinery from Phase 2 keeps working unchanged.

### Phase 4 — libraries go public (optional, orthogonal)

If/when the libs become public:

- octo-sts setup becomes optional (only needed for write actions)
- `GOPRIVATE` becomes unnecessary
- The `setup-cst-private-auth` action can be deleted, sky CI drops to plain `actions/checkout` + `setup-go` + `go test`

Independent of Phase 3 — can happen before or after.

## Local dev workflow

A `go.work` file at sky's root (gitignored) overrides the pseudo-versions with absolute paths to sibling clones. Mine:

```
go 1.26

use .

replace github.com/albertocavalcante/starlark-cst-go    => /Volumes/T9/dev/ws/starlark-cst-go
replace github.com/albertocavalcante/bazel-cst-go       => /Volumes/T9/dev/ws/bazel-cst-go
replace github.com/albertocavalcante/starlark-format-go => /Volumes/T9/dev/ws/starlark-format-go
```

Without `go.work`, sky builds against the pinned pseudo-versions (CI behaviour).

For local `go get` to pull updated pseudo-versions (when not using go.work overrides):

```
export GOPRIVATE=github.com/albertocavalcante/*
git config --global url."https://x-access-token:$(gh auth token)@github.com/".insteadOf "https://github.com/"
go get github.com/albertocavalcante/<lib>@main
```

(The `git config --global` change persists across shells. Unset with `git config --global --unset url.…@github.com/.insteadOf` when done.)

## Why no tags yet

- Each lib is pre-1.0; tagging implies API stability we don't yet have
- Pseudo-versions give the same reproducibility guarantee (exact commit pin in `go.sum`)
- `go get …@<hash>` is just as ergonomic as `go get …@v0.1.0` for bumping
- Avoids the bookkeeping of changelogs, release notes, and version negotiation across the 3 interdependent libs during rapid iteration

When stability lands, flip to tagged releases (Phase 3) without changing the auth or CI shape.

## Out of scope

- Bazel-side integration — tracked in #54
- Buck2 dialect (`buck2-cst-go`) — deferred until there's Buck2 demand
