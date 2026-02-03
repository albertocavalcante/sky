# hello-native

A minimal native Sky plugin example.

This plugin demonstrates the Sky plugin protocol without any external
dependencies. Use this as a starting point for your own plugins.

## What it Shows

- Handling `SKY_PLUGIN_MODE=metadata` for plugin discovery
- Reading plugin environment variables
- Processing command-line arguments
- Using the workspace root

## Build

```bash
go build -o plugin
```

## Install

```bash
sky plugin install --path ./plugin hello-native
```

## Usage

```bash
# Basic usage
sky hello-native

# Custom greeting
sky hello-native -name "Sky User"

# Show version
sky hello-native -version

# Show all plugin environment variables
sky hello-native -env
```

## Output

```
Hello, World!
Workspace: /path/to/your/workspace
```

## Code Structure

```go
func main() {
    // 1. Verify running as a plugin
    if os.Getenv("SKY_PLUGIN") != "1" {
        fmt.Fprintln(os.Stderr, "Run via: sky hello-native")
        os.Exit(1)
    }

    // 2. Handle metadata mode
    if os.Getenv("SKY_PLUGIN_MODE") == "metadata" {
        outputMetadata()
        return
    }

    // 3. Run the plugin logic
    os.Exit(run(os.Args[1:]))
}
```

## Environment Variables

The plugin reads these Sky environment variables:

| Variable             | Description                         |
| -------------------- | ----------------------------------- |
| `SKY_PLUGIN`         | Always "1" when running as a plugin |
| `SKY_PLUGIN_MODE`    | "exec" or "metadata"                |
| `SKY_PLUGIN_NAME`    | The registered plugin name          |
| `SKY_WORKSPACE_ROOT` | The workspace root directory        |
| `SKY_CONFIG_DIR`     | Sky configuration directory         |
| `SKY_OUTPUT_FORMAT`  | "text" or "json"                    |
| `SKY_NO_COLOR`       | "1" if color should be disabled     |
| `SKY_VERBOSE`        | Verbosity level (0-3)               |

## Testing

```bash
go test ./...
```
