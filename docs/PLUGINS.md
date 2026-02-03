# Plugins

Sky is plugin-first. Core commands stay minimal; most features ship as plugins.
If a command is not built into `sky`, it attempts to resolve and run an
installed plugin with the same name.

The plugin protocol is defined in `docs/PLUGIN_PROTOCOL.md`.

## Quick Start

Create a new plugin:

```bash
# Create a native Go plugin
sky plugin init my-plugin

# Or create a WASM plugin
sky plugin init my-wasm-plugin --wasm
```

Build and install:

```bash
cd my-plugin
go build -o plugin
sky plugin install --path ./plugin my-plugin
sky my-plugin
```

## Plugin Discovery

Installed plugins are stored in the user config directory:

- `~/.config/sky/plugins/` (binaries)
- `~/.config/sky/plugins.json` (metadata)
- `~/.config/sky/marketplaces.json` (marketplace list)

Override the config directory with `SKY_CONFIG_DIR` for local testing.
Plugin names must be lowercase alphanumerics with optional dashes.

## CLI

```bash
# Create plugins
sky plugin init <name>           # Create native plugin project
sky plugin init <name> --wasm    # Create WASM plugin project

# Manage plugins
sky plugin list                  # List installed plugins
sky plugin inspect <name>        # Show plugin metadata
sky plugin install <name> --path ./plugin   # Install from local file
sky plugin install <name> --url https://...  # Install from URL
sky plugin install <name>        # Install from marketplaces
sky plugin remove <name>         # Remove a plugin
sky plugin search <query>        # Search marketplaces

# Manage marketplaces
sky plugin marketplace list
sky plugin marketplace add <name> <url>
sky plugin marketplace remove <name>
```

## SDK Package

The `pkg/skyplugin` package eliminates boilerplate for plugin development:

```go
import "github.com/albertocavalcante/sky/pkg/skyplugin"

func main() {
    skyplugin.Serve(skyplugin.Plugin{
        Metadata: skyplugin.Metadata{
            APIVersion: 1,
            Name:       "my-plugin",
            Version:    "1.0.0",
            Summary:    "Does something useful",
        },
        Run: func(ctx context.Context, args []string) error {
            // Access workspace root
            root := skyplugin.WorkspaceRoot()

            // Check output format preference
            if skyplugin.IsJSONOutput() {
                // Output JSON
            }

            return nil
        },
    })
}
```

### SDK Features

- **Environment Helpers**: `WorkspaceRoot()`, `ConfigDir()`, `OutputFormat()`, etc.
- **Metadata Handling**: Automatic metadata mode detection and response
- **Output Formatting**: `WriteResult()` handles JSON vs text output
- **Testing Utilities**: `pkg/skyplugin/testing` for unit testing plugins

See `pkg/skyplugin/doc.go` for full documentation.

## Example Plugins

The `examples/plugins/` directory contains example plugins:

| Example        | Description                             |
| -------------- | --------------------------------------- |
| `hello-native` | Minimal native Go plugin                |
| `hello-wasm`   | Minimal WASM plugin                     |
| `star-counter` | Starlark file analyzer using buildtools |
| `custom-lint`  | Custom lint rules for Starlark          |

## Marketplace Index Format

A marketplace exposes a JSON index:

```json
{
  "name": "example",
  "updated_at": "2025-01-01T00:00:00Z",
  "plugins": [
    {
      "name": "skyfmt",
      "version": "0.1.0",
      "description": "Starlark formatter",
      "url": "https://example.com/skyfmt",
      "sha256": "<sha256 hex>",
      "type": "exe"
    }
  ]
}
```

The `url` should point to a standalone executable. Archive support can be added
later if needed. Set `"type": "wasm"` for WASI-compatible WebAssembly modules.

## Plugin Types

### Native Plugins (`exe`)

Native plugins are compiled executables. They have full system access:

- Filesystem operations
- Network requests
- External process execution
- Unlimited memory

Build for the target platform:

```bash
go build -o plugin
```

### WASM Plugins (`wasm`)

WASM plugins are WebAssembly modules that run in a sandbox:

- Portable across platforms
- Secure (no filesystem/network access)
- Limited to ~16MB memory by default

Build with Go:

```bash
GOOS=wasip1 GOARCH=wasm go build -o plugin.wasm
```

Or with TinyGo for smaller binaries:

```bash
tinygo build -o plugin.wasm -target=wasip1 .
```
