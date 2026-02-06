# sky

[![CI](https://github.com/albertocavalcante/sky/actions/workflows/ci.yml/badge.svg)](https://github.com/albertocavalcante/sky/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/albertocavalcante/sky)](https://goreportcard.com/report/github.com/albertocavalcante/sky)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/albertocavalcante/sky)](go.mod)

A comprehensive Go toolchain for Starlark, the configuration language used by
Bazel, Buck, and other build systems.

## Tools

| Tool       | Description                            | Status    |
| ---------- | -------------------------------------- | --------- |
| `sky`      | Plugin-first CLI and unified interface | ✅ Stable |
| `skyfmt`   | Code formatter (buildifier-based)      | ✅ Stable |
| `skylint`  | Linter with configurable rules         | ✅ Stable |
| `skycheck` | Static analyzer (type checking)        | ✅ Stable |
| `skyquery` | Query tool for Starlark sources        | ✅ Stable |
| `skydoc`   | Documentation generator                | ✅ Stable |
| `skytest`  | Test runner for Starlark tests         | ✅ Stable |
| `skyrepl`  | Interactive REPL                       | ✅ Stable |
| `skycov`   | Code coverage reporter                 | ✅ Stable |
| `skyls`    | Language Server Protocol (LSP)         | ✅ Stable |

## Installation

### From source (Go)

```bash
# Install individual tools
go install github.com/albertocavalcante/sky/cmd/sky@latest
go install github.com/albertocavalcante/sky/cmd/skylint@latest
go install github.com/albertocavalcante/sky/cmd/skyfmt@latest
# ... etc

# Or build the fat binary with all tools embedded
go install -tags=sky_full github.com/albertocavalcante/sky/cmd/sky@latest
```

### From releases

Download pre-built binaries from the [releases page](https://github.com/albertocavalcante/sky/releases).

## Usage

### Unified CLI (`sky`)

```bash
# Format files
sky fmt file.star

# Lint files
sky lint file.star

# Run static analysis
sky check file.star

# Generate documentation
sky doc file.star

# Start LSP server (for editor integration)
sky ls
```

### Standalone tools

Each tool can also be used directly:

```bash
skyfmt -w file.star      # Format in place
skylint file.star        # Lint
skycheck file.star       # Static analysis
skydoc -format json file.star  # Generate JSON docs
```

### Editor Integration (LSP)

`skyls` provides Language Server Protocol support for editors:

**Features:**

- Diagnostics (errors/warnings from skylint + skycheck)
- Hover documentation
- Go to definition
- Document symbols
- Code formatting

**VS Code:**

```json
{
  "starlark.lsp.path": "/path/to/skyls"
}
```

**Neovim (nvim-lspconfig):**

```lua
-- Custom LSP setup (skyls is not yet in lspconfig defaults)
vim.lsp.start({
  name = 'skyls',
  cmd = { 'skyls' },
  filetypes = { 'star', 'bzl', 'starlark' },
  root_dir = vim.fs.dirname(vim.fs.find({'WORKSPACE', 'MODULE.bazel', '.git'}, { upward = true })[1]),
})
```

## Plugin System

`sky` supports a plugin-first architecture. Unknown commands are resolved to
installed plugins, enabling extensibility without modifying the core.

```bash
# Install a plugin
sky plugin install my-plugin

# List installed plugins
sky plugin list

# Search marketplaces
sky plugin search formatter
```

See [docs/PLUGINS.md](docs/PLUGINS.md) for plugin development details.

## Project Layout

```
cmd/           CLI entrypoints
internal/      Shared packages (not importable)
  ├── cmd/     Tool implementations
  ├── lsp/     LSP server
  ├── starlark/  Starlark analysis packages
  └── plugins/ Plugin system
docs/          Design documents
examples/      Example plugins
```

## Development

### Prerequisites

- Go 1.25+
- Bazel 7+ (optional, for hermetic builds)
- [just](https://github.com/casey/just) (optional, for task runner)
- [lefthook](https://github.com/evilmartians/lefthook) (optional, for git hooks)

### Quick Start

```bash
# Using just (recommended)
just build      # Build all targets
just test       # Run all tests
just lint       # Run linter
just format     # Format code

# Using Go directly
go build ./...
go test ./...

# Using Bazel
bazel build //...
bazel test //...
```

### Build Variants

```bash
# Minimal sky binary (dispatches to external tools)
go build ./cmd/sky

# Fat binary with all tools embedded
go build -tags=sky_full ./cmd/sky

# Cross-compile for all platforms
just dist-all
```

### Git Hooks

Pre-commit hooks ensure code quality before commits:

```bash
# Install git hooks (one-time setup)
just hooks
# or: lefthook install

# Run pre-commit checks manually
just pre-commit
```

Hooks check: formatting, go.mod tidy, build, and tests.

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

Apache 2.0 - See [LICENSE](LICENSE) for details.
