## Summary

<!-- What changed and why, in a few bullets. -->

## Root cause

<!-- Fixes only: the actual defect and a failing input/scenario. Delete for features. -->

## Verification

<!-- Exact commands run and what you observed. Say what you could NOT run. -->

- [ ] Regression test fails without the fix and passes with it
- [ ] `just gazelle`, `go vet ./...`, `go test -race ./...`, `just test`
- [ ] `just format`, `just format-sh`, `just lint`, `just lint-sh`
- [ ] Reviewed `git diff main...HEAD`, not just the last commit
- [ ] Docs/help text updated for user-visible changes

## Risk

<!-- Public API, config, concurrency, build or protocol changes. Anything intentionally left out, and why. -->
