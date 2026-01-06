# JSON Builtins Loader

The JSON loader provides a simple, human-readable format for defining Starlark builtins. It complements the proto loader by offering an easier authoring experience for custom dialects and extensions.

## Overview

The JSON loader (`JSONProvider`) is part of the dual-format builtins loading system:

- **Proto Loader**: Binary format, efficient, used for standard dialects (Bazel, Buck2)
- **JSON Loader**: Text format, easy to edit, used for custom dialects and extensions

Both loaders implement the same `Provider` interface and can be chained together using `ChainProvider`.

## JSON Schema

The JSON schema maps directly to the `builtins.Builtins` Go struct, requiring **zero conversion** - just direct unmarshaling.

### Top-Level Structure

```json
{
  "functions": [...],
  "types": [...],
  "globals": [...]
}
```

### Functions

Functions are callables available in the global scope:

```json
{
  "functions": [
    {
      "name": "function_name",
      "doc": "Function documentation",
      "params": [
        {
          "name": "param_name",
          "type": "param_type",
          "default": "default_value",
          "required": true,
          "variadic": false,
          "kwargs": false
        }
      ],
      "return_type": "return_type"
    }
  ]
}
```

**Fields:**

- `name` (string, required): Function name
- `doc` (string, optional): Documentation string
- `params` (array, optional): List of parameters
- `return_type` (string, optional): Return type

**Parameter Fields:**

- `name` (string, required): Parameter name
- `type` (string, optional): Type annotation
- `default` (string, optional): Default value as string
- `required` (boolean, optional): Whether parameter is mandatory
- `variadic` (boolean, optional): Whether this is `*args`
- `kwargs` (boolean, optional): Whether this is `**kwargs`

### Types

Type definitions (classes, structs, providers):

```json
{
  "types": [
    {
      "name": "TypeName",
      "doc": "Type documentation",
      "fields": [
        {
          "name": "field_name",
          "type": "field_type",
          "doc": "Field documentation"
        }
      ],
      "methods": [
        {
          "name": "method_name",
          "doc": "Method documentation",
          "params": [...],
          "return_type": "return_type"
        }
      ]
    }
  ]
}
```

**Fields:**

- `name` (string, required): Type name
- `doc` (string, optional): Documentation string
- `fields` (array, optional): List of fields
- `methods` (array, optional): List of methods (same format as functions)

**Field Fields:**

- `name` (string, required): Field name
- `type` (string, optional): Type annotation
- `doc` (string, optional): Documentation string

### Globals

Global variables and constants:

```json
{
  "globals": [
    {
      "name": "GLOBAL_NAME",
      "type": "global_type",
      "doc": "Global documentation"
    }
  ]
}
```

**Fields:**

- `name` (string, required): Global name
- `type` (string, optional): Type annotation
- `doc` (string, optional): Documentation string

## Complete Example

```json
{
  "functions": [
    {
      "name": "cc_library",
      "doc": "Creates a C++ library target",
      "params": [
        {
          "name": "name",
          "type": "string",
          "required": true
        },
        {
          "name": "srcs",
          "type": "list[str]",
          "default": "[]",
          "required": false
        },
        {
          "name": "deps",
          "type": "list[Label]",
          "default": "[]",
          "required": false
        },
        {
          "name": "kwargs",
          "type": "any",
          "kwargs": true
        }
      ],
      "return_type": "None"
    },
    {
      "name": "glob",
      "doc": "Returns files matching patterns",
      "params": [
        {
          "name": "include",
          "type": "list[str]",
          "required": true
        },
        {
          "name": "exclude",
          "type": "list[str]",
          "default": "[]",
          "required": false
        }
      ],
      "return_type": "list[str]"
    }
  ],
  "types": [
    {
      "name": "Label",
      "doc": "Represents a build target",
      "fields": [
        {
          "name": "name",
          "type": "string",
          "doc": "Target name"
        },
        {
          "name": "package",
          "type": "string",
          "doc": "Package path"
        }
      ],
      "methods": [
        {
          "name": "relative",
          "doc": "Returns a label relative to this label",
          "params": [
            {
              "name": "target",
              "type": "string",
              "required": true
            }
          ],
          "return_type": "Label"
        }
      ]
    }
  ],
  "globals": [
    {
      "name": "PACKAGE_NAME",
      "type": "string",
      "doc": "The name of the current package"
    },
    {
      "name": "True",
      "type": "bool",
      "doc": "Boolean true constant"
    }
  ]
}
```

## Usage

### Basic Usage

```go
import (
    "github.com/albertocavalcante/sky/internal/starlark/builtins/loader"
    "github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// Create JSON provider
provider := loader.NewJSONProvider()

// Load builtins for a dialect and file kind
builtins, err := provider.Builtins("starlark", filekind.KindStarlark)
if err != nil {
    // Handle error
}

// Use builtins...
for _, fn := range builtins.Functions {
    fmt.Printf("Function: %s\n", fn.Name)
}
```

### Chaining with Proto Provider

```go
import (
    "github.com/albertocavalcante/sky/internal/starlark/builtins"
    "github.com/albertocavalcante/sky/internal/starlark/builtins/loader"
)

// Create both providers
protoProvider := loader.NewProtoProvider()
jsonProvider := loader.NewJSONProvider()

// Chain them: proto first (standard), JSON second (custom)
chain := builtins.NewChainProvider(protoProvider, jsonProvider)

// Load builtins - merges results from both
builtins, err := chain.Builtins("bazel", filekind.KindBUILD)
```

## File Organization

JSON files are organized by dialect and file kind:

```
internal/starlark/builtins/loader/data/json/
├── bazel-build.json      # Bazel BUILD files
├── bazel-bzl.json        # Bazel .bzl files
├── bazel-workspace.json  # Bazel WORKSPACE files
├── bazel-module.json     # Bazel MODULE.bazel files
├── buck2-buck.json       # Buck2 BUCK files
├── buck2-bzl.json        # Buck2 .bzl files
├── starlark-core.json    # Core Starlark builtins
└── ...
```

## Conversion from starpls

Use the `convert-starpls-json` tool to convert starpls JSON format to our format:

```bash
# Build the tool
cd tools/convert-starpls-json
go build

# Run conversion
./convert-starpls-json \
  -input=/path/to/starpls/data \
  -output=../../internal/starlark/builtins/loader/data/json
```

The tool converts:

- `build.builtins.json` → `bazel-build.json`
- `bzl.builtins.json` → `bazel-bzl.json`
- `workspace.builtins.json` → `bazel-workspace.json`
- `module-bazel.builtins.json` → `bazel-module.json`

## Implementation Details

### Caching

The JSON provider includes built-in caching:

1. First load: Reads file, parses JSON, caches result
2. Subsequent loads: Returns cached result (no I/O or parsing)
3. Thread-safe: Uses `sync.RWMutex` for concurrent access

### Performance

Benchmarks (approximate):

- **First load**: ~1-2ms (file I/O + JSON parsing)
- **Cached load**: ~50-100µs (memory lookup only)
- **JSON unmarshal**: ~500µs-1ms (for typical builtin files)

JSON is slightly faster than proto for small files, but proto scales better for large datasets.

### Error Handling

The loader handles errors gracefully:

- Missing file: Returns error with clear message
- Malformed JSON: Returns JSON parse error
- Unsupported dialect/kind: Returns error indicating unsupported combination

## Supported Dialects and File Kinds

### Bazel

- `KindBUILD`: BUILD files (e.g., `BUILD`, `BUILD.bazel`)
- `KindBzl`: .bzl files (Starlark extensions)
- `KindWORKSPACE`: WORKSPACE files
- `KindMODULE`: MODULE.bazel files
- `KindBzlmod`: Bzlmod files

### Buck2

- `KindBUCK`: BUCK files
- `KindBzlBuck`: .bzl files (Buck2 extensions)
- `KindBuckconfig`: .buckconfig files

### Starlark

- `KindStarlark`: Generic Starlark files (`.star`, `.sky`)
- `KindSkyI`: Sky interface files (`.skyi`)

## Comparison with Proto Loader

| Feature           | JSON Loader                 | Proto Loader                      |
| ----------------- | --------------------------- | --------------------------------- |
| Format            | Text (JSON)                 | Binary (protobuf)                 |
| Human-readable    | Yes                         | No (binary), Yes (pbtxt)          |
| Easy to edit      | Yes                         | Requires proto tools              |
| Performance       | Fast                        | Faster for large files            |
| Size on disk      | Larger                      | Smaller (binary)                  |
| Conversion needed | No                          | Yes (from proto schema)           |
| Best for          | Custom dialects, extensions | Standard dialects, large datasets |

## Testing

The JSON loader includes comprehensive tests:

- **Unit tests**: `json_loader_test.go`
  - Filename mapping
  - JSON parsing
  - Caching behavior
  - Concurrent access
  - Schema mapping

- **Integration tests**: `chain_integration_test.go`
  - ChainProvider with proto + JSON
  - Fallback behavior
  - Real-world scenarios

- **Benchmarks**:
  - First load performance
  - Cached load performance
  - JSON unmarshal performance

Run tests:

```bash
cd internal/starlark/builtins/loader
go test -v
go test -bench=.
```

## Future Enhancements

Potential improvements:

1. **Schema validation**: JSON schema for validation
2. **Compression**: Gzip compression for embedded JSON
3. **Lazy loading**: Load only requested sections
4. **Hot reload**: Watch for file changes and reload
5. **Remote loading**: Load from URLs or registries

## Contributing

When adding new JSON builtin files:

1. Follow the schema exactly (validates via tests)
2. Include comprehensive documentation strings
3. Use consistent type annotations
4. Add tests for new dialects/file kinds
5. Update this documentation

## References

- [builtins.Builtins struct](../provider.go) - Go struct definition
- [Proto loader](./proto_loader.go) - Alternative loader
- [ChainProvider](../provider.go) - Provider chaining
- [starpls](https://github.com/withered-magic/starpls) - Original data source
