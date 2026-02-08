# Sky - Starlark

Starlark language support with formatting, linting, and analysis

## Features

- Syntax highlighting for starlark files
- Language server integration via `skyls`
- Auto-completion, diagnostics, and more

## Installation

### From VS Code Marketplace

Search for "Sky - Starlark" in the Extensions view (`Ctrl+Shift+X`).

### From VSIX

```bash
code --install-extension sky-starlark-*.vsix
```

## Configuration

| Setting                     | Description             | Default          |
| --------------------------- | ----------------------- | ---------------- |
| `sky-starlark.server.path`  | Path to `skyls` binary  | `""` (uses PATH) |
| `sky-starlark.trace.server` | Trace LSP communication | `"off"`          |

## Requirements

- VS Code 1.85.0 or later
- `skyls` language server in PATH (or configure path in settings)

## Commands

| Command                                   | Description                 |
| ----------------------------------------- | --------------------------- |
| `Sky - Starlark: Restart Language Server` | Restart the language server |

## Development

```bash
# Install dependencies
pnpm install

# Build
pnpm run build

# Watch mode
pnpm run watch

# Launch Extension Development Host
# Press F5 in VS Code
```

## License

MIT
