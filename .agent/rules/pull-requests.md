---
description: Definition of done and PR submission standard for every change
---

# Pull Requests

Every change reaches `main` through a pull request. The `main-branch-protection`
ruleset requires one, so local squash-merging into `main` is not a workflow.
Open the PR from a `<type>/<short-kebab-description>` branch (see
`git-workflow.md`).

## Definition of done

A change is ready only when all of these hold. Do not open the PR before they do.

1. **Reproduce first.** For a bug fix, write the failing test before the fix and
   confirm it fails for the stated reason. A regression test that never failed
   proves nothing.
2. **Fix the cause, not the symptom.** If the same defect pattern exists
   elsewhere (a repeated type assertion, the same off-by-one in a sibling
   function), fix those too or list them in the PR body.
3. **Tests cover success and failure paths.** Concurrency changes need a
   `-race` test that exercises the shared state.
4. **Verification passes locally**, in this order:
   ```bash
   just format && just format-sh
   go vet ./...
   go test -race ./...
   just lint            # bazel build //... (nogo)
   just gazelle         # after adding/removing/renaming Go files or imports
   git diff --check
   ```
   CI also runs golangci-lint and `go mod tidy`. Run them if you touched
   dependencies or lint-sensitive code.
5. **BUILD files stay in sync.** New `_test.go` files and new imports need
   `just gazelle`. Never hand-edit `BUILD.bazel` without running Bazel.
6. **User-visible behavior is documented.** Update `docs/`, command help text
   and READMEs when flags, output, exit codes or config keys change.
7. **No unrelated changes.** No drive-by reformatting, no generated binaries,
   no local build output. Review `git diff --cached` before every commit.

## Scope

- One logical change per PR. Separate fix, refactor and dependency bump PRs.
- Large audits are split by area into separate PRs, each reviewable alone.
- Never mix a behavior change with mechanical churn in the same commit.
- Generated code (`internal/protocol/protocol.go`): change the generator or
  post-processor and regenerate. Do not hand-patch the output.

## Commits

- Conventional Commits: `<type>(<scope>): <description>`.
- Commits are small and each one builds and passes tests on its own.
- Do not add AI co-author trailers or attribution lines to commits or PR bodies.

## PR title and body

Title: the Conventional Commit line of the change.

Body (the template in `.github/pull_request_template.md` has these headings):

- **Summary**: what changed and why, in a few bullets.
- **Root cause** (fixes): the actual defect, with a failing input or scenario.
- **Verification**: the exact commands you ran and what you observed. State
  anything you could not run (for example Bazel without private-dependency
  auth) instead of implying it passed.
- **Risk**: public API, config, concurrency, build or protocol changes, and
  anything intentionally left out with the reason.

## Opening the PR

```bash
git branch --show-current          # never on main
git status                         # nothing unexpected
git diff --cached                  # review exactly what ships
git push -u origin <branch>        # never --force; --force-with-lease if needed
gh pr create --fill-first --web=false   # then edit title/body to the format above
gh pr checks <PR_NUMBER> --watch
```

Pushing and opening a PR are outward-facing. An agent does them only when the
user has asked for it in the current task.

## CI failures

- A failure caused by your change is yours to fix before asking for review.
- A failure caused by billing, quota or a platform outage is not a code
  problem. Say so in the PR and do not "fix" it by editing the workflow.
- Never merge over a failing technical check without the owner's explicit say-so.

## After review

Follow `.agent/workflows/review.md`: every thread gets a fix, a defer (with an
issue link) or a reasoned rejection, then a reply and resolution.
