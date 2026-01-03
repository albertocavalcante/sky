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
