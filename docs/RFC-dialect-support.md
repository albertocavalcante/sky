# RFC: Starlark Dialect Support in skyls

**Date:** 2026-02-04
**Status:** Draft
**Author:** Generated from research
**Supersedes:** RFC-custom-dialects.md, RFC-dialect-file-structure.md

## Abstract

This RFC proposes a unified system for configuring custom Starlark dialect support in the Sky language server (skyls). It consolidates the dialect configuration and builtin file structure designs, adds collision handling semantics, and provides migration guidance from other tools.

## Problem Statement

Starlark is used by many build systems and tools, each extending it with custom builtins:

| Tool     | File Types                     | Custom Builtins                             |
| -------- | ------------------------------ | ------------------------------------------- |
| Bazel    | BUILD, .bzl, WORKSPACE, MODULE | cc_library, java_binary, providers, depsets |
| Buck2    | BUCK, .bxl                     | rule, attrs, ctx, bxl functions             |
| Copybara | copy.bara.sky                  | core.workflow, git.origin, transformations  |
| Tilt     | Tiltfile                       | docker_build, k8s_yaml, local_resource      |
| Custom   | .star, .sky                    | User-defined DSL functions                  |

Without dialect awareness, skyls reports false positives:

- `undefined: cc_library` in BUILD files
- `undefined: docker_build` in Tiltfiles
- No completions for dialect-specific functions
- No hover documentation for builtins

## Goals

1. **Zero-config for common dialects**: Auto-detect Bazel/Buck2/Tilt from workspace markers
2. **Configurable for custom dialects**: Support user-provided builtin definitions
3. **Composable**: Chain multiple builtin sources (core + dialect + workspace)
4. **Multi-format**: Support JSON, textproto, and Python stubs
5. **Cross-tool compatible**: Align with starpls, Hirschgarten where possible
6. **Performant**: Load builtins once, cache aggressively

## Design Overview

```
.starlark/
├── config.json              # Which files use which dialect
└── builtins/
    ├── tilt.builtins.json   # JSON format
    ├── copybara.builtins.textproto  # Textproto format
    └── custom.builtins.pyi  # Python stub format
```

## Configuration File

### Location and Discovery

Search order:

1. CLI flag: `--config path/to/config.json`
2. Environment: `STARLARK_CONFIG=/path/to/config.json`
3. `.starlark/config.json` (walking up from file)
4. `starlark.config.json` (root level)
5. `$XDG_CONFIG_HOME/starlark/config.json` (user default)
6. Built-in defaults (auto-detect from workspace markers)

### Schema

```json
{
  "$schema": "https://sky.dev/schemas/starlark-config.json",
  "version": 1,

  "rules": [
    {
      "files": ["Tiltfile", "tilt_modules/**/*.star"],
      "dialect": "tilt"
    },
    {
      "files": ["*.bara.sky"],
      "dialect": "copybara"
    },
    {
      "files": ["**/*.bzl"],
      "dialect": "bazel-bzl"
    },
    {
      "files": ["BUILD", "BUILD.bazel", "**/BUILD", "**/BUILD.bazel"],
      "dialect": "bazel-build"
    }
  ],

  "dialects": {
    "tilt": {
      "builtins": [".starlark/builtins/tilt.builtins.json"],
      "extends": "starlark"
    },
    "copybara": {
      "builtins": [".starlark/builtins/copybara.builtins.json"],
      "extends": "starlark"
    }
  },

  "settings": {
    "reportUndefinedNames": true,
    "reportUnusedBindings": true,
    "checkLoadStatements": false
  }
}
```

### Field Definitions

#### Top-Level

| Field      | Type   | Required | Description             |
| ---------- | ------ | -------- | ----------------------- |
| `$schema`  | string | No       | JSON Schema URL         |
| `version`  | number | Yes      | Schema version (1)      |
| `dialect`  | string | No       | Default dialect         |
| `rules`    | array  | No       | File-to-dialect mapping |
| `dialects` | object | No       | Dialect definitions     |
| `settings` | object | No       | Analysis settings       |

#### Rules

```json
{
  "files": ["Tiltfile", "**/*.tilt.star"],
  "dialect": "tilt"
}
```

- `files`: Array of glob patterns
- `dialect`: Dialect identifier

First matching rule wins. Order matters.

#### Dialect Definition

```json
{
  "builtins": [
    ".starlark/builtins/core.builtins.json",
    ".starlark/builtins/extensions.builtins.json"
  ],
  "extends": "starlark"
}
```

- `builtins`: Array of paths to builtin files
- `extends`: Parent dialect for inheritance

## Builtin File Formats

### File Naming Convention

Use compound extensions to indicate purpose:

| Pattern                     | Example                    | Format      |
| --------------------------- | -------------------------- | ----------- |
| `{name}.builtins.json`      | `tilt.builtins.json`       | JSON        |
| `{name}.builtins.textproto` | `bazel.builtins.textproto` | Textproto   |
| `{name}.builtins.pyi`       | `tilt.builtins.pyi`        | Python stub |

### JSON Format (Primary)

```json
{
  "$schema": "https://sky.dev/schemas/starlark-builtins.json",
  "version": 1,
  "name": "tilt",
  "description": "Tilt Starlark builtins",

  "functions": [
    {
      "name": "docker_build",
      "doc": "Build a Docker image from a Dockerfile.",
      "params": [
        {"name": "ref", "type": "string", "required": true, "doc": "Image reference"},
        {"name": "context", "type": "string", "default": "'.'", "doc": "Build context"},
        {"name": "kwargs", "kwargs": true}
      ],
      "return_type": "None"
    }
  ],

  "types": [
    {
      "name": "Blob",
      "doc": "Binary large object",
      "fields": [
        {"name": "size", "type": "int", "doc": "Size in bytes"}
      ],
      "methods": [
        {"name": "__str__", "doc": "Convert to string", "return_type": "string"}
      ]
    }
  ],

  "globals": [
    {"name": "os", "type": "OsModule", "doc": "OS utilities"}
  ],

  "modules": {
    "os": {
      "functions": [
        {"name": "getcwd", "return_type": "string", "doc": "Get current directory"}
      ]
    }
  }
}
```

### Textproto Format

```textproto
values {
  name: "docker_build"
  doc: "Build a Docker image from a Dockerfile."
  callable {
    params {
      name: "ref"
      type: "string"
      is_mandatory: true
    }
    params {
      name: "context"
      type: "string"
      default_value: "'.'"
    }
    return_type: "None"
  }
}

types {
  name: "Blob"
  doc: "Binary large object"
  fields {
    name: "size"
    type: "int"
  }
}
```

### Python Stub Format

```python
# tilt.builtins.pyi
"""Tilt Starlark builtins."""

class Blob:
    """Binary large object."""
    size: int

    def __str__(self) -> str:
        """Convert to string."""
        ...

def docker_build(
    ref: str,
    context: str = ".",
    **kwargs,
) -> None:
    """
    Build a Docker image from a Dockerfile.

    Args:
        ref: Image reference
        context: Build context
        **kwargs: Additional arguments
    """
    ...

os: OsModule
"""OS utilities."""
```

## Dialect Inheritance

Dialects form an inheritance tree:

```
starlark (core builtins)
    |
    +-- bazel (Bazel common)
    |       |
    |       +-- bazel-build (BUILD files)
    |       |
    |       +-- bazel-bzl (.bzl files)
    |       |
    |       +-- bazel-workspace (WORKSPACE)
    |       |
    |       +-- bazel-module (MODULE.bazel)
    |
    +-- buck2 (Buck2 common)
    |       |
    |       +-- buck2-buck (BUCK files)
    |       |
    |       +-- buck2-bzl (.bzl files)
    |
    +-- tilt (Tilt)
    |
    +-- copybara (Copybara)
```

Resolution order:

1. Child dialect's builtins (in order)
2. Parent dialect's builtins (recursively)
3. Core starlark builtins

## Collision Handling

When multiple sources define the same symbol:

### Priority Order

1. **Last builtin file** in dialect's `builtins` array
2. **Child dialect** over parent dialect
3. **User-defined** over built-in

### Example

```json
{
  "dialects": {
    "my-bazel": {
      "builtins": [
        "base.builtins.json",      // Loaded first
        "overrides.builtins.json"  // Wins on conflict
      ],
      "extends": "bazel-build"     // User overrides bazel
    }
  }
}
```

If both `overrides.builtins.json` and `bazel-build` define `glob`:

- The version from `overrides.builtins.json` wins
- Documentation and parameters are NOT merged
- Complete replacement semantics

### Collision Detection

skyls logs warnings for collisions at debug level:

```
DEBUG: Builtin 'glob' from 'overrides.builtins.json' shadows 'bazel-build'
```

## Auto-Detection

When no config file exists:

| Workspace Marker                 | Detected Dialect |
| -------------------------------- | ---------------- |
| `WORKSPACE` or `WORKSPACE.bazel` | `bazel`          |
| `MODULE.bazel`                   | `bazel`          |
| `.buckconfig`                    | `buck2`          |
| `Tiltfile`                       | `tilt`           |
| (none)                           | `starlark`       |

File type detection within dialect:

| Pattern                        | File Kind |
| ------------------------------ | --------- |
| `BUILD`, `BUILD.bazel`         | build     |
| `*.bzl`                        | bzl       |
| `WORKSPACE`, `WORKSPACE.bazel` | workspace |
| `MODULE.bazel`                 | module    |
| `BUCK`                         | buck      |
| `*.star`, `*.sky`              | generic   |

## Implementation Architecture

### Provider Interface

```go
// Provider supplies builtin definitions.
type Provider interface {
    Builtins(dialect string, kind filekind.Kind) (Builtins, error)
    SupportedDialects() []string
}

// ChainProvider merges multiple providers.
type ChainProvider struct {
    providers []Provider
}

func (c *ChainProvider) Builtins(dialect string, kind filekind.Kind) (Builtins, error) {
    var result Builtins
    for _, p := range c.providers {
        b, err := p.Builtins(dialect, kind)
        if err != nil {
            continue
        }
        result.Merge(b)  // Later providers win on collision
    }
    return result, nil
}
```

### Multi-Format Loader

```go
type MultiFormatLoader struct {
    jsonLoader      *JSONProvider
    textprotoLoader *TextprotoProvider
    pythonLoader    *PythonStubProvider
}

func (m *MultiFormatLoader) LoadFromPath(path string) (*Builtins, error) {
    ext := filepath.Ext(path)
    switch ext {
    case ".json":
        return m.jsonLoader.LoadFile(path)
    case ".textproto", ".pbtxt":
        return m.textprotoLoader.LoadFile(path)
    case ".py", ".pyi":
        return m.pythonLoader.LoadFile(path)
    default:
        return nil, fmt.Errorf("unsupported format: %s", ext)
    }
}
```

### LSP Integration

```go
func (s *Server) handleInitialize(ctx context.Context, params json.RawMessage) (any, error) {
    // ... parse params ...

    // Load config
    config, err := s.loadConfig(s.rootURI)
    if err != nil {
        // Use auto-detection
        config = s.autoDetectConfig(s.rootURI)
    }

    // Initialize builtin providers
    s.builtins = s.buildProviderChain(config)

    // ... return capabilities ...
}

func (s *Server) handleCompletion(ctx context.Context, params json.RawMessage) (any, error) {
    // ... parse params ...

    // Get dialect for this file
    dialect := s.dialectForFile(doc.URI)
    filekind := s.classifier.Classify(string(doc.URI))

    // Get builtins
    builtins, _ := s.builtins.Builtins(dialect, filekind)

    // Include in completions
    for _, fn := range builtins.Functions {
        items = append(items, CompletionItem{
            Label:         fn.Name,
            Kind:          CompletionItemKindFunction,
            Documentation: fn.Doc,
        })
    }

    // ... return items ...
}
```

## Migration from Other Tools

### From starpls `--ext-paths`

**Before (CLI):**

```bash
starpls --ext-paths=.starlark/builtins
```

**After (config file):**

```json
{
  "version": 1,
  "dialects": {
    "custom": {
      "builtins": [".starlark/builtins/custom.builtins.json"]
    }
  },
  "rules": [
    {"files": ["**/*.star"], "dialect": "custom"}
  ]
}
```

### From Hirschgarten `.starlark-dialects.json`

**Before:**

```json
{
  "version": 1,
  "rules": [
    {"glob": "Tiltfile", "dialectId": "tilt", "priority": 100}
  ],
  "builtinFilesByDialect": {
    "tilt": ["starlark/tilt.builtins.json"]
  }
}
```

**After:**

```json
{
  "version": 1,
  "rules": [
    {"files": ["Tiltfile"], "dialect": "tilt"}
  ],
  "dialects": {
    "tilt": {
      "builtins": ["starlark/tilt.builtins.json"],
      "extends": "starlark"
    }
  }
}
```

Key changes:

- `glob` → `files` (array)
- `dialectId` → `dialect`
- `priority` removed (order-based)
- `builtinFilesByDialect` → `dialects` with `builtins`

### From Tilt `--builtin-paths`

**Before (CLI):**

```bash
starlark-lsp --builtin-paths=stubs/
```

**After:**

1. Move Python stubs to `.starlark/builtins/`
2. Create config:

```json
{
  "version": 1,
  "rules": [
    {"files": ["Tiltfile", "**/*.star"], "dialect": "tilt"}
  ],
  "dialects": {
    "tilt": {
      "builtins": [".starlark/builtins/tilt.builtins.pyi"],
      "extends": "starlark"
    }
  }
}
```

## Cross-Tool Alignment

### Compatibility Matrix

| Feature                 | Sky     | starpls  | Hirschgarten | Tilt LSP |
| ----------------------- | ------- | -------- | ------------ | -------- |
| `.starlark/config.json` | Yes     | Proposed | Proposed     | N/A      |
| `.builtins.json`        | Yes     | Yes      | Yes          | N/A      |
| `.builtins.pyi`         | Planned | N/A      | N/A          | Yes      |
| `.builtins.textproto`   | Planned | Yes      | N/A          | N/A      |
| Glob rules              | Yes     | Yes      | Yes          | N/A      |
| Dialect inheritance     | Yes     | N/A      | N/A          | N/A      |
| Remote URLs             | Planned | N/A      | Yes          | N/A      |

### JSON Builtin Schema Alignment

We align with Hirschgarten's JSON schema with extensions:

| Field                 | Hirschgarten   | Sky       | Notes         |
| --------------------- | -------------- | --------- | ------------- |
| `name`                | Yes            | Yes       | Aligned       |
| `doc`                 | Yes            | Yes       | Aligned       |
| `params[].name`       | Yes            | Yes       | Aligned       |
| `params[].required`   | Yes            | Yes       | Aligned       |
| `params[].positional` | Yes            | Yes       | Aligned       |
| `params[].named`      | Yes            | Yes       | Aligned       |
| `params[].type`       | No             | Yes       | Sky extension |
| `params[].default`    | `defaultValue` | `default` | Alias both    |
| `return_type`         | No             | Yes       | Sky extension |
| `types`               | No             | Yes       | Sky extension |
| `globals`             | No             | Yes       | Sky extension |
| `modules`             | No             | Yes       | Sky extension |

## Implementation Plan

### Phase 1: Config Loading (1-2 days)

1. Implement config file parser
2. Support `.starlark/config.json` discovery
3. Add glob pattern matching for rules
4. Wire up to LSP initialization

### Phase 2: JSON Builtins (2-3 days)

1. Enhance JSONProvider for config-based loading
2. Implement dialect inheritance chain
3. Add collision handling with logging
4. Connect to completion, hover, diagnostics

### Phase 3: Additional Formats (2-3 days)

1. Textproto loader (starpls compatibility)
2. Python stub parser (Tilt compatibility)
3. Format auto-detection by extension

### Phase 4: Polish (1-2 days)

1. Auto-detection for zero-config experience
2. Config validation with helpful errors
3. Documentation and examples
4. CLI tools: `sky builtins validate`, `sky builtins convert`

## Open Questions

1. **Remote URLs**: Should we support loading builtins from URLs? Security implications?
2. **Caching**: How to handle cache invalidation for remote URLs?
3. **LSP initialization options**: Should `initializationOptions` override config file?
4. **Module resolution**: Should we resolve relative builtin paths from config location or workspace root?

## References

- [JSON Loader Documentation](../internal/starlark/builtins/loader/JSON_LOADER.md)
- [Hirschgarten BYO Dialects PR](https://github.com/albertocavalcante/fork-jetbrains-hirschgarten/pull/1)
- [starlark-lsp (Tilt)](https://github.com/tilt-dev/starlark-lsp)
- [starpls](https://github.com/withered-magic/starpls)
- [starpls#379: Custom stubs discussion](https://github.com/withered-magic/starpls/issues/379)
- [Pyright Configuration](https://github.com/microsoft/pyright/blob/main/docs/configuration.md)
- [LSP Specification](https://microsoft.github.io/language-server-protocol/)
