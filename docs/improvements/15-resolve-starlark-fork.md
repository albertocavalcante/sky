# 15 — Resolve the `go.starlark.net` experimental fork

## Category

Technical debt / Supply chain

## Effort

Variable (depends on upstream engagement)

## Files

- `go.mod` — line 40

## Problem

The entire codebase depends on a personal fork of `go.starlark.net`:

```go
replace go.starlark.net => github.com/albertocavalcante/starlark-go-x v0.0.0-20260203191202-da5a35fe16a6
```

This fork adds coverage hooks (`OnExec`, `OnBranch`, `OnFunctionEnter/Exit`,
`OnIteration`) needed by the `skycov` tool. The fork is pinned to a single
untagged commit.

### Risks

1. **Supply chain**: A personal fork is a single point of failure. If the repo
   is deleted or force-pushed, builds break.
2. **Upstream drift**: The fork will diverge from upstream `go.starlark.net`
   over time, making security patches harder to apply.
3. **Reproducibility**: No tagged release — only a commit hash.
4. **Contributor friction**: New contributors must understand the fork to work
   on coverage features.

## Options

### Option A: Upstream the hooks

Open a PR to `google/starlark-go` proposing the hook interface. This is the
best long-term solution but depends on upstream willingness.

### Option B: Vendor the fork

Copy the modified starlark-go source into `internal/starlark-go/` or a
`vendor/` directory. This removes the external dependency but increases
maintenance burden.

### Option C: Tag a release on the fork

At minimum, create a tagged version (`v0.1.0`) on the fork to improve
reproducibility and make the dependency explicit.

### Option D: Isolate the fork usage

Ensure only `skycov` and `internal/starlark/coverage/` depend on the fork's
hook API. Other packages should work with upstream starlark-go. This limits
blast radius.

## Recommendation

Short-term: Option C (tag the fork) + Option D (isolate usage).
Long-term: Option A (upstream the hooks).

## Acceptance Criteria

- Fork has at least one tagged release
- Coverage hook usage is isolated to coverage-related packages
- A tracking issue exists for upstreaming the hooks
- `go.mod` replace directive points to a tagged version
