---
description: Git workflow, branching, and commit conventions
---

# Git Workflow

These rules are non-negotiable.

## Safety Rules

| Rule                   | Command                                   | Rationale                   |
| ---------------------- | ----------------------------------------- | --------------------------- |
| Never commit on main   | `git branch --show-current` before commit | Protects main branch        |
| Stage files explicitly | `git add file1.go file2.go`               | Prevents accidental commits |
| Verify before push     | `git status` + `git diff --cached`        | Catches mistakes early      |

Forbidden:

- `git add .` or `git add -A`
- `git commit` without verifying branch
- `git push --force` (use `--force-with-lease` if necessary)

## Branching Strategy

```bash
# Ensure main is current
git checkout main
git pull origin main

# Create a feature branch
git checkout -b <type>/<short-description>
```

Branch naming:

```
<type>/<short-kebab-description>
```

Types: `feat` `fix` `refactor` `test` `docs` `ci` `chore`

## Commit Format

Use Conventional Commits:

```
<type>(<scope>): <description>
```

Examples:

- `feat(parser): add starlark module loader`
- `fix(lint): handle empty files`

## Branch Hygiene

Stale branches and worktrees are debt. Cleaning up is part of finishing a PR, not a later chore.

After a PR merges:

```bash
git switch main && git pull --ff-only && git fetch --prune
git branch -D <branch>          # squash merges fail -d; the merged PR is the proof
git worktree remove <path>      # every worktree you created
```

- Merge with `gh pr merge --squash --delete-branch`. The repo also deletes head branches on merge.
- Agents and review tooling use disposable worktrees outside the repo tree and remove them when done. Never leave one registered.
- A branch is safe to delete only when its tip is an ancestor of `main` or exactly the head of a merged PR. Keep anything with an open PR, commits added after its PR merged, or no PR, and ask its owner.
- Before force-removing a dirty worktree, look at what is dirty. Archive anything worth keeping first.
