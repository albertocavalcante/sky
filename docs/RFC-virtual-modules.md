# RFC: Virtual Modules and External Repository Support

**Status:** Draft
**Date:** 2026-02-04

## Overview

Starlark/Bazel load statements can reference various types of modules that don't correspond to local `.bzl` files. The LSP needs special handling for these "virtual" modules to provide proper hover, completion, and go-to-definition support.

## Module Types

### 1. External Repositories (`@repo//...`)

```starlark
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@rules_python//python:defs.bzl", "py_binary")
load("@io_bazel_rules_go//go:def.bzl", "go_binary")
```

**Characteristics:**

- Prefixed with `@repo_name`
- Resolved via WORKSPACE or MODULE.bazel
- May be fetched from network or local path
- Cannot be resolved without workspace context

**LSP Handling:**

- Document links: Skip or mark as "external"
- Hover: Show repository info if known
- Completion: Provide known symbols from builtin data

### 2. Bazel Built-in Repositories

Special repositories that Bazel provides automatically:

| Repository            | Description                 |
| --------------------- | --------------------------- |
| `@bazel_tools`        | Core Bazel tools and rules  |
| `@local_config_cc`    | C++ toolchain configuration |
| `@local_config_xcode` | Xcode configuration (macOS) |
| `@platforms`          | Platform definitions        |
| `@rules_cc`           | C++ rules (embedded)        |
| `@rules_java`         | Java rules (embedded)       |
| `@rules_proto`        | Proto rules (embedded)      |

**LSP Handling:**

- Provide builtin documentation for common symbols
- Ship with embedded metadata for `@bazel_tools` symbols

### 3. Native Module (`native.*`)

Pre-Starlark built-in rules available via `native`:

```starlark
# In .bzl files
native.cc_library(name = "foo", srcs = ["foo.cc"])
native.glob(["**/*.java"])
native.package_name()
native.repository_name()

# In BUILD files (implicit)
cc_library(name = "foo", srcs = ["foo.cc"])
```

**Characteristics:**

- `native` is a special module, not loaded from a file
- Contains all native rules (cc__, java__, py_*, etc.)
- Also contains utility functions (glob, package_name, etc.)
- Only available in .bzl files (BUILD has them at top-level)

**LSP Handling:**

- Provide completion for `native.` prefix
- Hover shows native rule documentation
- Go-to-definition returns "native builtin" info

### 4. Dialect-Specific Virtual Modules

#### Buck2

```starlark
load("@prelude//rules.bzl", "cxx_binary")
load("@root//TARGETS", "some_target")
```

#### Tilt

```starlark
load("ext://restart_process", "docker_build_with_restart")
```

#### Pulumi

```starlark
# Special runtime bindings
from pulumi import Config, export
```

### 5. Language Integration Modules

Some build systems expose language-specific APIs:

```starlark
# Go rules - special providers
load("@io_bazel_rules_go//go:def.bzl", "go_library", "GoLibrary")

# Java rules - special providers
load("@rules_java//java:defs.bzl", "java_library", "JavaInfo")

# Proto rules
load("@rules_proto//proto:defs.bzl", "proto_library", "ProtoInfo")
```

**LSP Handling:**

- Ship builtin data for common rule sets
- Support loading additional rule metadata from config

## Implementation Strategy

### Phase 1: Classification

Add module classification to `resolveLoadPath`:

```go
type ModuleKind int

const (
    ModuleKindLocal      ModuleKind = iota // ./foo.bzl, //pkg:foo.bzl
    ModuleKindExternal                      // @repo//...
    ModuleKindBuiltin                       // @bazel_tools, @platforms
    ModuleKindNative                        // native.*
    ModuleKindExtension                     // ext://... (Tilt)
    ModuleKindUnknown
)

func classifyModule(module string, dialect string) ModuleKind
```

### Phase 2: Builtin Metadata

Extend `builtins.Provider` to support module-scoped builtins:

```go
type Provider interface {
    // Existing
    Builtins(dialect string, kind filekind.Kind) (Builtins, error)

    // New: Get builtins for a specific module
    ModuleBuiltins(dialect string, module string) (Builtins, error)
}
```

Ship embedded metadata for:

- `native` module (all native rules)
- `@bazel_tools//tools/build_defs/repo:*` (repository rules)
- Common rule sets (`@rules_go`, `@rules_python`, etc.)

### Phase 3: Workspace Resolution

For full external repo support:

1. Parse WORKSPACE/MODULE.bazel on init
2. Build repository -> path mapping
3. Resolve external loads to local paths when possible

This is complex and can be deferred - many repos are fetched on-demand.

## Data Format

Extend JSON builtin format for module-scoped definitions:

```json
{
  "dialect": "bazel",
  "modules": {
    "native": {
      "functions": [
        {"name": "glob", "doc": "...", "params": [...]},
        {"name": "package_name", "doc": "...", "params": []},
        {"name": "cc_library", "doc": "...", "params": [...]}
      ]
    },
    "@bazel_tools//tools/build_defs/repo:http.bzl": {
      "functions": [
        {"name": "http_archive", "doc": "...", "params": [...]}
      ]
    }
  }
}
```

## LSP Feature Integration

### Document Links

```go
func resolveLoadPath(module, fromPath, workspaceRoot string) (string, ModuleKind) {
    kind := classifyModule(module, "bazel")
    switch kind {
    case ModuleKindLocal:
        return resolveLocalPath(module, fromPath, workspaceRoot), kind
    case ModuleKindExternal, ModuleKindBuiltin:
        // Return empty path but indicate it's a known external
        return "", kind
    case ModuleKindNative:
        // native.* - no path
        return "", kind
    default:
        return "", ModuleKindUnknown
    }
}
```

### Completion

```go
// After "native." prefix
if strings.HasPrefix(prefix, "native.") {
    memberPrefix := strings.TrimPrefix(prefix, "native.")
    return s.getNativeCompletions(memberPrefix, uri)
}
```

### Hover

```go
// Check if hovering over "native" or "native.xyz"
if word == "native" || strings.HasPrefix(context, "native.") {
    return s.getNativeHover(word, uri)
}
```

## Test Cases

```starlark
# test_virtual_modules.bzl

# External repo - should not error, mark as external
load("@rules_go//go:def.bzl", "go_binary")

# Bazel builtin repo - provide docs
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# Native usage - provide completion/hover
def my_rule():
    native.cc_library(name = "foo")
    native.glob(["*.cc"])
```

## Migration Path

1. **Immediate:** Skip external repos in document links (don't break)
2. **Short-term:** Add native module support (high value)
3. **Medium-term:** Ship common rule set metadata
4. **Long-term:** Workspace parsing for full resolution

## Open Questions

1. Should we try to resolve `@local_*` repos that point to local paths?
2. How to handle bzlmod (`MODULE.bazel`) vs WORKSPACE?
3. Should we provide stubs for all native rules or just common ones?
4. How to handle version-specific API differences?

## References

- [Bazel Repository Rules](https://bazel.build/rules/lib/repo)
- [Native Module](https://bazel.build/rules/lib/toplevel/native)
- [Bzlmod Migration](https://bazel.build/external/migration)
