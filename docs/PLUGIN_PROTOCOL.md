# Plugin Protocol (v1)

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
