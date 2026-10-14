## Summary

<!-- What changed and why, in a few bullets. -->

## Root cause

<!-- Fixes only: the actual defect and a failing input/scenario. Delete for features. -->

## Verification

<!-- Exact commands run and what you observed. Say what you could NOT run. -->

- [ ] Regression test fails without the fix and passes with it
- [ ] `go vet ./...` and `go test -race ./...`
- [ ] `just format`, `just lint`, and `just gazelle` (if Go files or imports changed)
- [ ] Docs/help text updated for user-visible changes

## Risk

<!-- Public API, config, concurrency, build or protocol changes. Anything intentionally left out, and why. -->
