# Sky Plugin Examples

This directory contains example plugins demonstrating how to build plugins for Sky.

## Examples

| Directory                       | Description              | Dependencies       |
| ------------------------------- | ------------------------ | ------------------ |
| [hello-native](./hello-native/) | Minimal native Go plugin | None (stdlib only) |
| [hello-wasm](./hello-wasm/)     | Minimal WASM plugin      | None (stdlib only) |
| [star-counter](./star-counter/) | Starlark file analyzer   | `buildtools`       |
| [custom-lint](./custom-lint/)   | Custom lint rules        | `buildtools`       |

## Quick Start

### Build and Install hello-native

```bash
cd hello-native
go build -o plugin
sky plugin install --path ./plugin hello-native
sky hello-native
```

### Build and Install hello-wasm

```bash
cd hello-wasm
GOOS=wasip1 GOARCH=wasm go build -o plugin.wasm
sky plugin install --path ./plugin.wasm hello-wasm
sky hello-wasm
```

## Plugin Protocol

All plugins communicate with Sky via environment variables and JSON:

**Environment Variables (set by Sky):**

- `SKY_PLUGIN=1` - Indicates running as a plugin
- `SKY_PLUGIN_MODE=exec|metadata` - The execution mode
- `SKY_PLUGIN_NAME=<name>` - The plugin's registered name
- `SKY_WORKSPACE_ROOT=<path>` - Workspace root directory
- `SKY_CONFIG_DIR=<path>` - Sky configuration directory
- `SKY_OUTPUT_FORMAT=text|json` - Preferred output format
- `SKY_NO_COLOR=1` - Disable color output
- `SKY_VERBOSE=0-3` - Verbosity level

**Metadata Mode:**
When `SKY_PLUGIN_MODE=metadata`, the plugin must print JSON to stdout and exit:

```json
{
  "api_version": 1,
  "name": "my-plugin",
  "version": "1.0.0",
  "summary": "Plugin description",
  "commands": [
    {"name": "my-plugin", "summary": "Main command"}
  ]
}
```

## Using the SDK

The `pkg/skyplugin` package provides helpers that eliminate boilerplate:

```go
package main

import (
    "context"
    "fmt"

    "github.com/albertocavalcante/sky/pkg/skyplugin"
)

func main() {
    skyplugin.Serve(skyplugin.Plugin{
        Metadata: skyplugin.Metadata{
            APIVersion: 1,
            Name:       "my-plugin",
            Version:    "1.0.0",
            Summary:    "Does something useful",
        },
        Run: func(ctx context.Context, args []string) error {
            fmt.Println("Hello from my plugin!")
            return nil
        },
    })
}
```

## Testing

Each example includes tests. Run them with:

```bash
cd <example-dir>
go test ./...
```

## Resources

- [Plugin Protocol](../../docs/PLUGIN_PROTOCOL.md)
- [Plugins Documentation](../../docs/PLUGINS.md)
- [SDK Package](../../pkg/skyplugin/)
