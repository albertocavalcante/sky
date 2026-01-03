# Plugins

Sky is plugin-first. Core commands stay minimal; most features ship as plugins.
If a command is not built into `sky`, it attempts to resolve and run an
installed plugin with the same name.

The plugin protocol is defined in `docs/PLUGIN_PROTOCOL.md`.

## Plugin Discovery

Installed plugins are stored in the user config directory:

- `~/.config/sky/plugins/` (binaries)
- `~/.config/sky/plugins.json` (metadata)
- `~/.config/sky/marketplaces.json` (marketplace list)

Override the config directory with `SKY_CONFIG_DIR` for local testing.
Plugin names must be lowercase alphanumerics with optional dashes.

## CLI

```bash
sky plugin list
sky plugin inspect <name>
sky plugin install <name> --path /path/to/plugin
sky plugin install <name> --url https://example.com/skyfmt
sky plugin install <name>            # uses marketplaces
sky plugin search <query>
sky plugin marketplace add <name> <url>
sky plugin marketplace list
```

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
