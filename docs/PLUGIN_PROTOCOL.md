# Plugin Protocol (v1.1)

Sky is plugin-first. Plugins can be native executables or WASI-compatible
WebAssembly modules. Both are invoked the same way: via args + environment,
with JSON metadata exchanged over stdout.

## Modes

Sky sets the following environment variables on every plugin invocation:

- `SKY_PLUGIN=1`
- `SKY_PLUGIN_NAME=<plugin name>`
- `SKY_PLUGIN_MODE=exec | metadata`

### exec

When `SKY_PLUGIN_MODE=exec`, the plugin receives the original CLI arguments
(verbatim) and should perform its command. Standard output and error are
streamed to the user. The plugin exit code becomes the `sky` exit code.

### metadata

When `SKY_PLUGIN_MODE=metadata`, the plugin must print a single JSON object to
stdout and exit with status 0. The JSON must not include extra log output.

## Environment Variables

Sky sets these environment variables when running plugins:

| Variable             | Version | Description                                |
| -------------------- | ------- | ------------------------------------------ |
| `SKY_PLUGIN`         | v1.0    | Always "1" when running as a plugin        |
| `SKY_PLUGIN_MODE`    | v1.0    | "exec" or "metadata"                       |
| `SKY_PLUGIN_NAME`    | v1.0    | The plugin's registered name               |
| `SKY_WORKSPACE_ROOT` | v1.1    | Workspace root directory (see below)       |
| `SKY_CONFIG_DIR`     | v1.1    | Sky configuration directory                |
| `SKY_OUTPUT_FORMAT`  | v1.1    | Preferred output format ("text" or "json") |
| `SKY_NO_COLOR`       | v1.1    | "1" if color output should be disabled     |
| `SKY_VERBOSE`        | v1.1    | Verbosity level (0-3)                      |

### Workspace Root Detection

`SKY_WORKSPACE_ROOT` is determined by searching upward from the current
directory for these markers (in order):

1. `.sky.yaml` or `.sky.yml` - Sky configuration file
2. `.git` directory - Version control root

If no markers are found, it defaults to the current working directory.

### Handling Optional Variables

Variables added in v1.1 may not be set by older versions of Sky. Always
provide fallbacks:

```go
func workspaceRoot() string {
    if root := os.Getenv("SKY_WORKSPACE_ROOT"); root != "" {
        return root
    }
    // Fallback to current directory
    cwd, _ := os.Getwd()
    return cwd
}

func outputFormat() string {
    if format := os.Getenv("SKY_OUTPUT_FORMAT"); format != "" {
        return format
    }
    return "text"  // Default to text
}
```

## Metadata JSON

```json
{
  "api_version": 1,
  "name": "skyfmt",
  "version": "0.1.0",
  "summary": "Starlark formatter",
  "commands": [
    {
      "name": "format",
      "summary": "Format Starlark sources"
    }
  ]
}
```

- `api_version` is required and must be `1`.
- `name` should match the installed plugin name.
- `summary` is used as the human description.

## Plugin Types

- `exe`: Native executable (default).
- `wasm`: WASI-compatible WebAssembly module. The module must expose `_start`.

## WASM Requirements

WASM plugins must be built for WASI and write to stdout/stderr as usual. Sky
runs them with the same args + env as native plugins.

### WASM Limitations

WASM plugins run in a sandboxed environment:

- **No filesystem access** - Use environment variables for paths
- **No network access** - All I/O through stdin/stdout
- **Limited memory** - Default ~16MB

For plugins requiring filesystem or network access, use a native executable.

## SDK Package

The `pkg/skyplugin` package provides helpers that eliminate boilerplate:

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
            // Plugin logic here
            return nil
        },
    })
}
```

See `pkg/skyplugin/` for full documentation.

## Backward Compatibility

- v1.0 plugins work unchanged with v1.1 Sky
- v1.1 environment variables are optional; plugins should provide fallbacks
- `api_version` in metadata remains `1` for both v1.0 and v1.1
