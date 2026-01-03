---
description: Go code quality, tests, and style
---

# Code Quality

## Go Style

- Use `gofmt` (run `make format`).
- Keep package names short and lowercase.
- Prefer table-driven tests for data-heavy cases.
- Wrap errors with context using `%w`.

## Testing

- Add tests for new behavior and bug fixes.
- Cover both success and failure paths.
- Run `make test` before commit.

## Linting

- Run `make lint` before commit.
