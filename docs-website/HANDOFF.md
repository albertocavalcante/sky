# Handoff: Sky Documentation & Tooling

## Current State

All PRs merged. Main branch is up to date.

## Completed Work

### Documentation Website (PRs #19-28)

The Sky documentation website is live with:

- **Starlight framework** (Astro-based)
- **Tailwind CSS v4**
- **Starlark syntax highlighting** (custom TextMate grammar)
- **Link validator** (starlight-links-validator plugin)
- **i18n support** (English root locale)
- **Custom 404 page**

### Starlark Section (`/starlark/`)

| Page            | Path                         | Content                             |
| --------------- | ---------------------------- | ----------------------------------- |
| Overview        | `/starlark/overview/`        | What is Starlark, history, features |
| Basics          | `/starlark/basics/`          | Language tutorial                   |
| Types           | `/starlark/types/`           | Type annotations, records, enums    |
| Best Practices  | `/starlark/best-practices/`  | Performance tips from Buck2         |
| Resources       | `/starlark/resources/`       | Ecosystem links                     |
| Implementations | `/starlark/implementations/` | Go vs Rust vs Java comparison       |
| Use Cases       | `/starlark/use-cases/`       | Tilt, Kurtosis, Buck2 BXL           |
| Type Roadmap    | `/starlark/types-roadmap/`   | Type system evolution               |
| Tooling         | `/starlark/tooling/`         | LSP, DAP, IDE integrations          |

### Tools Section (`/tools/`)

| Tool     | Status                                                   |
| -------- | -------------------------------------------------------- |
| skylint  | ✅ Fully documented (40+ rules, categories, config, CI)  |
| skyfmt   | ✅ Fully documented (file types, extensions, CI)         |
| skytest  | ✅ Fully documented (assert module, reporters, coverage) |
| skycov   | ✅ Fully documented (formats, CI integration)            |
| skydoc   | ✅ Fully documented (docstring format, output types)     |
| skyrepl  | ✅ Fully documented (built-in modules, scripts)          |
| skyquery | ✅ Fully documented (query language, patterns)           |
| skycheck | ✅ Fully documented (undefined names, unused vars)       |

### Code Fixes (PRs #20, #28)

- `cmd/skycov`: Use `strconv.Atoi` instead of `fmt.Sscanf`
- `cmd/skytest`: Explicit JSON structs with snake_case keys

## starlark-go-x Positioning

**Important**: starlark-go-x is positioned as:

- Experimental fork exploring additional features
- Goal is to upstream improvements, NOT fragment ecosystem
- Always reference official starlark-go first

## Build Commands

```bash
cd docs-website
bun install
bun run dev    # Development server at localhost:4321
bun run build  # Production build to ./dist/
```

## Deployment

- **Workflow**: `.github/workflows/docs.yml`
- **Base URL**: `/sky/`
- Enable GitHub Pages in repo settings after first workflow run

## Completed Recently

- **Plugin Architecture** - Phase 1 & 2 complete (PR #29)
  - Embedded tool support with build tags
  - `go build -tags=sky_full ./cmd/sky` for bundled binary
- **Build Targets** - Cross-compilation support (PR #30)
  - Bazel targets: `//cmd/sky:sky`, `//cmd/sky:sky_full`
  - `just build-all` for linux/darwin/windows
- **Tool Documentation** - All 8 tools fully documented

## Pending Work

### High Priority

1. **Plugin CLI Polish** (RFC Phase 3)
   - Better error: "lint not found, install with: sky plugin install lint"
   - `sky plugin init` scaffolding command
   - Plugin update notifications

2. **starlark-go-x section** - Pages exist but need expansion:
   - `/starlark-go-x/overview/`
   - `/starlark-go-x/type-annotations/`
   - `/starlark-go-x/onexec-hook/`

### Medium Priority

3. **Getting Started guide** - Quick-start tutorial for new users

4. **API reference** - Auto-generate from Go source if possible

5. **Examples section** - Real-world Starlark examples

### Low Priority

6. **Search improvements** - Pagefind is configured but could use tuning

7. **Dark mode** - Starlight supports it, just needs enabling

## Technical Notes

### Starlark Syntax Highlighting

- Grammar: `docs-website/starlark.tmLanguage.json`
- Aliases: `bzl`, `bazel`, `build`, `star`
- All code blocks use `` ```starlark `` not `` ```python ``

### MDX Gotchas

- Curly braces in tables get parsed as JSX - escape or reword
- Example: `ctx.{outputs,files}` → `ctx.outputs and ctx.files`

### VS Code LSP Setup

The vscode-bazel extension supports both starpls and bazel-lsp:

```json
{
  "bazel.lsp.command": "starpls"
}
```

There is NO separate starpls VS Code extension.

### skylint Suppression

Supported directives:

- `# skylint: disable=rule`
- `# skylint: disable-next-line=rule`
- Inline: `code  # skylint: disable=rule`

**NOT supported**: `# skylint: enable=...` (no block suppression)

## Reference Materials

Local references available:

- `/Users/adsc/dev/refs/buck2/` - Buck2 source with starlark-rust
- `/Users/adsc/dev/refs/bazel/` - Bazel source with Java implementation
- `/Users/adsc/dev/refs/starlark-lang.org/` - Community site source

## Goal

Make Sky docs the **definitive Starlark resource** - comprehensive, accurate, and useful for developers across all implementations and use cases.
