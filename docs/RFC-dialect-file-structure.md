# RFC: Starlark Dialect File Structure and Naming

**Date:** 2026-02-04
**Status:** Draft
**Supersedes:** Naming section of RFC-custom-dialects.md

## Problem

We need a standard way to:

1. Configure which Starlark dialect applies to which files
2. Define builtin functions/types for custom dialects
3. Share dialect definitions across projects and tools

Current approaches are fragmented:

- Hirschgarten: `.starlark-dialects.json`
- starpls: `--ext-paths` CLI flag
- Tilt: `--builtin-paths` with `.py` stubs
- No industry standard exists

## Design Principles

Based on research of successful conventions:

| Principle                        | Example                        | Rationale                          |
| -------------------------------- | ------------------------------ | ---------------------------------- |
| **Extension as semantic marker** | `.pyi`, `.d.ts`                | Immediately clear what file is for |
| **Separate config from data**    | `tsconfig.json` + `lib.*.d.ts` | Config is small, data can be large |
| **Directory for collections**    | `syntaxes/`, `stubs/`          | Scales without root clutter        |
| **Single file for simple cases** | `.editorconfig`                | Don't over-engineer small projects |

## Proposed Structure

### Option A: `.starlark/` Directory (Recommended)

```
project/
├── .starlark/
│   ├── config.json              # Which files use which dialect
│   └── builtins/
│       ├── tilt.json            # Tilt builtin definitions
│       ├── copybara.json        # Copybara builtin definitions
│       └── internal.json        # Project-specific additions
├── BUILD
├── Tiltfile
└── copy.bara.sky
```

**Rationale:**

- `.starlark/` is clearly Starlark-specific (like `.github/`, `.vscode/`)
- Separates config (small) from builtins (large)
- Scales to many dialects without root pollution
- Easy to `.gitignore` generated files

### Option B: Root-Level Files (Simple Projects)

```
project/
├── starlark.config.json         # Config + optional inline builtins
├── BUILD
└── Tiltfile
```

**Rationale:**

- Single file for simple projects
- Follows `.prettierrc`, `.eslintrc` pattern
- No directory overhead

### Option C: Hybrid (Auto-Detected)

Tools should support both:

1. Check for `.starlark/config.json` first
2. Fall back to `starlark.config.json` in root
3. Allow CLI override: `--config path/to/config.json`

## File Naming Conventions

### Config File

| Name                    | Format | Notes                  |
| ----------------------- | ------ | ---------------------- |
| `.starlark/config.json` | JSON   | Primary location       |
| `starlark.config.json`  | JSON   | Root-level alternative |
| `starlark.config.yaml`  | YAML   | If YAML support added  |
| `starlark.config.toml`  | TOML   | If TOML support added  |

**Not recommended:**

- `.starlark-dialects.json` - too specific, hard to extend
- `.starlarkrc` - unclear what it configures
- `sky.json` - tool-specific, not ecosystem-wide

### Builtin Definition Files

Use a **compound extension** pattern to indicate purpose:

| Pattern                        | Example                    | Format        |
| ------------------------------ | -------------------------- | ------------- |
| `{dialect}.builtins.json`      | `tilt.builtins.json`       | JSON          |
| `{dialect}.builtins.textproto` | `bazel.builtins.textproto` | Protobuf text |
| `{dialect}.builtins.pyi`       | `tilt.builtins.pyi`        | Python stub   |

**Why compound extensions?**

- `tilt.json` could be anything; `tilt.builtins.json` is unambiguous
- Allows multiple formats: `tilt.builtins.json` + `tilt.builtins.pyi`
- Follows `*.d.ts`, `*.test.js`, `*.stories.tsx` patterns

**Alternative considered:**

- `.sbi` (Starlark Builtins Interface) - new extension, no tooling support
- `.starlark` - conflicts with source files

## Config File Schema

### Minimal Config

```json
{
  "$schema": "https://sky.dev/schemas/starlark-config.json",
  "version": 1,
  "dialect": "bazel"
}
```

### Full Config

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
      "files": ["**/*.bzl", "BUILD", "BUILD.bazel"],
      "dialect": "bazel"
    }
  ],

  "dialects": {
    "tilt": {
      "builtins": [
        ".starlark/builtins/tilt.builtins.json",
        ".starlark/builtins/tilt-extensions.builtins.json"
      ],
      "extends": "starlark"
    },
    "copybara": {
      "builtins": ["https://copybara.dev/builtins/v1.builtins.json"],
      "extends": "starlark"
    },
    "internal": {
      "builtins": [".starlark/builtins/internal.builtins.json"],
      "extends": "bazel"
    }
  },

  "settings": {
    "reportUndefinedNames": true,
    "reportUnusedBindings": true
  }
}
```

### Key Design Choices

1. **`rules` array** - Ordered, first match wins (like `.gitignore`)
2. **`files` glob patterns** - Standard glob syntax, relative to config
3. **`extends`** - Dialects can inherit from others
4. **Remote URLs** - Allow fetching builtins from URLs
5. **Multiple builtins** - Array allows composing definitions

## Builtin File Schema

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
      "doc": "Build a Docker image from a Dockerfile",
      "params": [
        {
          "name": "ref",
          "type": "string",
          "doc": "Image reference (e.g., 'myimage:latest')",
          "required": true
        },
        {
          "name": "context",
          "type": "string",
          "doc": "Build context path",
          "default": "'.'",
          "required": false,
          "positional": true,
          "named": true
        },
        {
          "name": "kwargs",
          "type": "dict",
          "variadic": "kwargs"
        }
      ],
      "returns": "Image"
    }
  ],

  "types": [
    {
      "name": "Image",
      "doc": "Represents a Docker image",
      "fields": [
        {"name": "ref", "type": "string", "doc": "Image reference"}
      ],
      "methods": [
        {
          "name": "tag",
          "doc": "Add a tag to this image",
          "params": [{"name": "tag", "type": "string", "required": true}],
          "returns": "Image"
        }
      ]
    }
  ],

  "globals": [
    {
      "name": "TILT_VERSION",
      "type": "string",
      "doc": "Current Tilt version"
    }
  ],

  "modules": {
    "os": {
      "functions": [
        {"name": "getcwd", "returns": "string", "doc": "Get current directory"}
      ]
    }
  }
}
```

### Python Stub Format

```python
# tilt.builtins.pyi
"""Tilt Starlark builtins."""

class Image:
    """Represents a Docker image."""
    ref: str

    def tag(self, tag: str) -> Image:
        """Add a tag to this image."""
        ...

def docker_build(
    ref: str,
    context: str = ".",
    **kwargs,
) -> Image:
    """
    Build a Docker image from a Dockerfile.

    Args:
        ref: Image reference (e.g., 'myimage:latest')
        context: Build context path
        **kwargs: Additional build arguments
    """
    ...

TILT_VERSION: str
"""Current Tilt version."""
```

### Textproto Format

```textproto
# tilt.builtins.textproto
# Compatible with Bazel's builtin proto schema

values {
  name: "docker_build"
  doc: "Build a Docker image from a Dockerfile"
  callable {
    params {
      name: "ref"
      type: "string"
      doc: "Image reference"
      is_mandatory: true
    }
    params {
      name: "context"
      type: "string"
      default_value: "'.'"
    }
    return_type: "Image"
  }
}

types {
  name: "Image"
  doc: "Represents a Docker image"
  fields {
    name: "ref"
    type: "string"
  }
}
```

## Directory Layout Examples

### Large Project (Monorepo)

```
monorepo/
├── .starlark/
│   ├── config.json
│   └── builtins/
│       ├── bazel.builtins.json      # Bazel overrides/extensions
│       ├── internal.builtins.json   # Company-specific rules
│       └── generated/               # Auto-generated from protos
│           └── api.builtins.json
├── projects/
│   ├── frontend/
│   │   └── .starlark/
│   │       └── config.json          # Frontend-specific overrides
│   └── backend/
└── tools/
```

### Simple Project

```
myproject/
├── starlark.config.json    # Just "dialect": "tilt"
├── Tiltfile
└── src/
```

### Shared Dialect Package

```
# Published to npm/PyPI/GitHub releases
tilt-starlark-builtins/
├── package.json            # Or setup.py, go.mod
├── tilt.builtins.json
├── tilt.builtins.pyi       # Alternative format
└── README.md
```

## Discovery Order

Tools should search for config in this order:

1. CLI flag: `--config path/to/config.json`
2. Environment: `STARLARK_CONFIG=/path/to/config.json`
3. `.starlark/config.json` (walking up from file)
4. `starlark.config.json` (walking up from file)
5. `$XDG_CONFIG_HOME/starlark/config.json` (user default)
6. Built-in defaults (auto-detect Bazel/Buck2)

## Cross-Tool Compatibility

### Compatibility Matrix

| Feature                 | Sky      | starpls  | Hirschgarten | Tilt LSP |
| ----------------------- | -------- | -------- | ------------ | -------- |
| `.starlark/config.json` | ✅       | Proposed | Proposed     | N/A      |
| `starlark.config.json`  | ✅       | Proposed | N/A          | N/A      |
| `.builtins.json`        | ✅       | ✅       | ✅           | N/A      |
| `.builtins.pyi`         | Proposed | N/A      | N/A          | ✅       |
| `.builtins.textproto`   | Proposed | ✅       | N/A          | N/A      |
| Glob rules              | ✅       | ✅       | ✅           | N/A      |
| Remote URLs             | Proposed | N/A      | ✅           | N/A      |

### Migration from Existing Formats

| From                                     | To                                   | Notes                                |
| ---------------------------------------- | ------------------------------------ | ------------------------------------ |
| `.starlark-dialects.json` (Hirschgarten) | `.starlark/config.json`              | Rename, move builtins to `builtins/` |
| `--ext-paths` (starpls)                  | `.starlark/config.json` + `dialects` | Add config file                      |
| `--builtin-paths` (Tilt)                 | `.starlark/builtins/*.pyi`           | Move stubs to standard location      |

## Open Questions

1. **JSON Schema hosting**: Should `$schema` point to `sky.dev`, `json-schema.org`, or tool-specific?
2. **Format priority**: When both `.json` and `.pyi` exist, which takes precedence?
3. **Workspace vs User config**: Should user-level config extend or override project config?
4. **Caching**: How to handle remote URL caching and invalidation?

## References

- [PEP 561 - Python Type Stubs](https://peps.python.org/pep-0561/)
- [TypeScript lib.d.ts](https://www.typescriptlang.org/docs/handbook/2/type-declarations.html)
- [VS Code TextMate Grammars](https://code.visualstudio.com/api/language-extensions/syntax-highlight-guide)
- [EditorConfig](https://editorconfig.org/)
- [Databricks CLI **builtins**.pyi](https://github.com/databricks/cli/blob/main/.vscode/__builtins__.pyi)
- [RFC-custom-dialects.md](./RFC-custom-dialects.md)
