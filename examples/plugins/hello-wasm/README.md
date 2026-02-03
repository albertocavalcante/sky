# hello-wasm

A minimal WASM Sky plugin example.

This plugin demonstrates how to build a plugin that runs as WebAssembly in a
sandboxed environment.

## What it Shows

- Building plugins for WASI (WebAssembly System Interface)
- TinyGo-compatible argument parsing (no `flag` package)
- Working within WASM sandbox limitations

## Build

### With Go (larger binary, full compatibility)

```bash
GOOS=wasip1 GOARCH=wasm go build -o plugin.wasm
```

### With TinyGo (smaller binary, some limitations)

```bash
tinygo build -o plugin.wasm -target=wasip1 .
```

## Install

```bash
sky plugin install --path ./plugin.wasm hello-wasm
```

## Usage

```bash
# Basic usage
sky hello-wasm

# Custom greeting
sky hello-wasm -name "Sky User"

# Show version
sky hello-wasm --version

# Show all plugin environment variables
sky hello-wasm --env

# Show help
sky hello-wasm --help
```

## Output

```
Hello from WASM, World!
Workspace: /path/to/your/workspace
(Running in WASM sandbox)
```

## WASI Limitations

WASM plugins run in a sandboxed environment with these restrictions:

### No Filesystem Access

WASM plugins cannot directly read or write files. All file paths come from
environment variables. If your plugin needs to process files, consider:

- Having the host (sky) read files and pass content via stdin
- Writing results to stdout for the host to capture
- Using a native plugin instead

### No Network Access

WASM plugins cannot make network requests. All I/O happens through:

- stdin/stdout/stderr streams
- Environment variables

### Limited Memory

Default memory is limited to ~16MB. For memory-intensive operations,
consider a native plugin.

### No External Processes

WASM plugins cannot spawn subprocesses. All processing must happen
within the plugin itself.

## When to Use WASM Plugins

WASM plugins are ideal for:

- Portable plugins that work across all platforms without recompilation
- Sandboxed execution (security-sensitive environments)
- Simple text transformation or analysis
- Plugins distributed via URL (downloaded and run safely)

For plugins that need:

- Filesystem access
- Network requests
- Large memory usage
- External process execution

Use a **native plugin** instead.

## TinyGo Compatibility

This example avoids the `flag` package for TinyGo compatibility.
TinyGo produces much smaller binaries but has some limitations:

- No `reflect` package (limits JSON encoding options)
- No `flag` package
- Some standard library features unavailable

The Go standard compiler (`go build`) produces larger but fully
compatible WASM binaries.

## Size Comparison

| Compiler | Approximate Size |
| -------- | ---------------- |
| Go 1.21  | ~2-3 MB          |
| TinyGo   | ~100-500 KB      |

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
