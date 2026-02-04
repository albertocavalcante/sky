# RFC: Custom Dialect Support in skyls

**Date:** 2026-02-04
**Status:** Draft
**Author:** Generated from research

## Problem Statement

Starlark is a configuration language used by many build systems and tools, each extending it with their own builtins:

| Tool         | File Types                     | Custom Builtins                                |
| ------------ | ------------------------------ | ---------------------------------------------- |
| **Bazel**    | BUILD, .bzl, WORKSPACE, MODULE | cc_library, java_binary, providers, depsets... |
| **Buck2**    | BUCK, .bxl                     | rule, attrs, ctx, bxl functions...             |
| **Copybara** | copy.bara.sky                  | core.workflow, git.origin, transformations...  |
| **Tilt**     | Tiltfile                       | docker_build, k8s_yaml, local_resource...      |
| **Custom**   | .star, .sky                    | User-defined DSL functions                     |

Currently, `skyls` provides basic LSP functionality but lacks dialect awareness:

- **Completion** returns nothing (TODO stub)
- **Diagnostics** report false positives for dialect-specific builtins (`undefined: cc_library`)
- **Hover** shows no documentation for builtin functions
- **No configuration** for workspace-specific dialects

## Goals

1. **Zero-config for common dialects**: Auto-detect Bazel/Buck2 from file patterns
2. **Configurable for custom dialects**: Support user-provided builtin definitions
3. **Composable**: Chain multiple builtin sources (core + dialect + workspace)
4. **Performant**: Load builtins once, cache aggressively

## Research Summary

### How Others Solve This

| Tool                    | Approach                       | Pros                 | Cons                    |
| ----------------------- | ------------------------------ | -------------------- | ----------------------- |
| **starlark-lsp** (Tilt) | `--builtin-paths` flag + stubs | Simple, discoverable | Manual maintenance      |
| **starpls**             | Proto schemas + embedded data  | Strongly typed       | Requires rebuilds       |
| **Pyright**             | `stubPath` + `typeshedPath`    | Industry standard    | Complex stub authoring  |
| **pylsp**               | Plugin architecture            | Extensible           | Plugin isolation issues |

### Key Insights

1. **Stub files are standard**: `.pyi` (Python) or JSON/proto schemas
2. **Configuration over code**: Init files (`pyrightconfig.json`) > CLI flags > hardcoding
3. **Load at init, not per-request**: Parse builtins during LSP initialization
4. **Multi-path composition**: Support layering (core < dialect < workspace)

## Current Architecture

Sky already has infrastructure for this:

```
internal/starlark/
├── builtins/
│   ├── provider.go           # Provider interface + ChainProvider
│   ├── proto/builtins.proto  # Proto schema definition
│   └── loader/
│       ├── proto_loader.go   # Binary proto format (Bazel, Buck2)
│       ├── json_loader.go    # JSON format (custom dialects)
│       └── data/
│           ├── proto/        # Embedded .pb files
│           └── json/         # JSON builtin definitions
├── classifier/
│   └── classifier.go         # Detects dialect from file path/name
├── filekind/
│   └── filekind.go          # KindBUILD, KindBzl, KindBUCK, etc.
└── dialect/
    └── dialect.go           # Dialect configurations
```

**Gap**: The LSP server (`internal/lsp/server.go`) doesn't use any of this.

## Multiple Format Support

Based on research from [starpls#379](https://github.com/withered-magic/starpls/issues/379) and [tilt-dev/starlark-lsp](https://github.com/tilt-dev/starlark-lsp), we should support multiple builtin definition formats.

### Supported Formats

| Format           | Extension              | Use Case                             | Pros                            | Cons                     |
| ---------------- | ---------------------- | ------------------------------------ | ------------------------------- | ------------------------ |
| **Binary Proto** | `.pb`                  | Embedded standard dialects           | Fast, compact                   | Not human-editable       |
| **JSON**         | `.json`                | Custom dialects, easy authoring      | Human-readable, no tools needed | Verbose                  |
| **Textproto**    | `.textproto`, `.pbtxt` | Schema-validated, starpls-compatible | Readable + validated            | Requires proto knowledge |
| **Python Stubs** | `.py`, `.pyi`          | Tilt-compatible, rich ecosystem      | Familiar syntax, docstrings     | Requires parser          |

### Format Examples

#### 1. JSON (Current)

```json
{
  "functions": [
    {
      "name": "cc_library",
      "doc": "Creates a C++ library target",
      "params": [
        { "name": "name", "type": "string", "required": true },
        { "name": "srcs", "type": "list[str]", "default": "[]" }
      ],
      "return_type": "None"
    }
  ]
}
```

#### 2. Textproto (Proposed)

Compatible with starpls proto schema and Bazel's builtins.proto format:

```textproto
# copybara-builtins.textproto
types {
  name: "origin"
  doc: "Source repository for a workflow"
  fields {
    name: "url"
    type: "string"
    doc: "Repository URL"
  }
}

values {
  name: "core.workflow"
  doc: "Defines a migration workflow"
  callable {
    params {
      name: "name"
      type: "string"
      is_mandatory: true
    }
    params {
      name: "origin"
      type: "origin"
      is_mandatory: true
    }
    params {
      name: "transformations"
      type: "list[transformation]"
      default_value: "[]"
    }
    return_type: "None"
  }
}
```

#### 3. Python Stubs (Proposed)

Compatible with tilt-dev/starlark-lsp format:

```python
# copybara_builtins.py
"""Copybara Starlark builtins."""

class origin:
    """Source repository for a workflow."""
    url: str
    ref: str

class destination:
    """Target repository for a workflow."""
    pass

def workflow(
    name: str,
    origin: origin,
    destination: destination,
    authoring: authoring,
    transformations: list = [],
) -> None:
    """
    Defines a migration workflow from origin to destination.

    Args:
        name: Workflow identifier
        origin: Source repository
        destination: Target repository
        authoring: Author mapping configuration
        transformations: List of transformations to apply
    """
    pass

def git_origin(url: str, ref: str = "master") -> origin:
    """
    Defines a Git repository as the source of truth.

    Args:
        url: Git repository URL
        ref: Branch, tag, or commit to use
    """
    pass
```

### Unified Loader Architecture

```go
// loader/multi_loader.go
type MultiFormatLoader struct {
    jsonLoader     *JSONProvider
    protoLoader    *ProtoProvider
    textprotoLoader *TextprotoProvider
    pythonLoader   *PythonStubProvider
}

func (m *MultiFormatLoader) LoadFromPath(path string) (*Builtins, error) {
    ext := filepath.Ext(path)
    switch ext {
    case ".json":
        return m.jsonLoader.LoadFile(path)
    case ".pb":
        return m.protoLoader.LoadFile(path)
    case ".textproto", ".pbtxt":
        return m.textprotoLoader.LoadFile(path)
    case ".py", ".pyi":
        return m.pythonLoader.LoadFile(path)
    default:
        return nil, fmt.Errorf("unsupported format: %s", ext)
    }
}

// Directory loading (like tilt-dev/starlark-lsp)
func (m *MultiFormatLoader) LoadFromDir(dir string) (*Builtins, error) {
    result := &Builtins{}
    err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
        if d.IsDir() {
            return nil
        }
        builtins, err := m.LoadFromPath(path)
        if err != nil {
            return err
        }
        result.Merge(builtins)
        return nil
    })
    return result, err
}
```

### Python Stub Parser

For Python stub support, use a lightweight parser (not full Python):

```go
// loader/python_stub_loader.go
type PythonStubProvider struct {
    // Uses tree-sitter-python or a simplified parser
}

func (p *PythonStubProvider) LoadFile(path string) (*Builtins, error) {
    content, _ := os.ReadFile(path)

    // Parse with tree-sitter (like tilt-dev/starlark-lsp)
    // Or use a Go Python parser like github.com/go-python/gpython

    // Extract:
    // - Function definitions (def foo(...) -> Type:)
    // - Class definitions (class Foo:)
    // - Docstrings for documentation
    // - Type hints for parameter/return types

    return builtins, nil
}
```

### Configuration with Multiple Formats

```json
{
  "dialect": "custom",
  "builtins": {
    "paths": [
      ".sky/builtins.json",
      ".sky/types.textproto",
      ".sky/stubs/api.py",
      "/shared/team-builtins/"
    ]
  }
}
```

The loader automatically detects format by extension and merges all definitions.

## Proposed Design

### 1. Configuration File: `.sky/config.json`

```json
{
  "dialect": "bazel",
  "builtins": {
    "paths": [
      ".sky/builtins/custom.json",
      "/path/to/team-shared-builtins.json"
    ],
    "inline": {
      "functions": [
        {
          "name": "my_custom_rule",
          "doc": "Project-specific rule wrapper",
          "params": [
            { "name": "name", "type": "string", "required": true },
            { "name": "deps", "type": "list[Label]", "default": "[]" }
          ],
          "return_type": "None"
        }
      ]
    }
  },
  "features": {
    "reportUnusedBindings": true,
    "reportUndefinedNames": true
  }
}
```

### 2. Auto-Detection (Zero Config)

When no config file exists, detect dialect from workspace structure:

```go
func detectDialect(rootURI string) string {
    patterns := map[string]string{
        "WORKSPACE":       "bazel",
        "WORKSPACE.bazel": "bazel",
        "MODULE.bazel":    "bazel",
        ".buckconfig":     "buck2",
        "Tiltfile":        "tilt",
    }
    for pattern, dialect := range patterns {
        if fileExists(rootURI, pattern) {
            return dialect
        }
    }
    return "starlark" // Default
}
```

### 3. Builtin Provider Chain

```go
// During LSP initialization
func (s *Server) initializeBuiltins(rootURI string, config *Config) {
    providers := []builtins.Provider{
        // 1. Core Starlark builtins (always)
        loader.NewCoreProvider(),

        // 2. Dialect-specific builtins (auto-detected or configured)
        loader.NewProtoProvider(),  // Bazel, Buck2
        loader.NewJSONProvider(),   // Custom JSON files

        // 3. Workspace-local builtins (from config)
        loader.NewPathProvider(config.Builtins.Paths...),

        // 4. Inline builtins (from config)
        loader.NewInlineProvider(config.Builtins.Inline),
    }

    s.builtins = builtins.NewChainProvider(providers...)
}
```

### 4. Integration Points

#### Completion (`handleCompletion`)

```go
func (s *Server) handleCompletion(params *CompletionParams) (*CompletionList, error) {
    doc := s.documents[params.TextDocument.URI]
    filekind := s.classifier.Classify(string(params.TextDocument.URI))

    // Get dialect-specific builtins
    builtins, _ := s.builtins.Builtins(s.dialect, filekind)

    items := []CompletionItem{}

    // 1. Builtins (filtered by trigger context)
    for _, fn := range builtins.Functions {
        items = append(items, CompletionItem{
            Label:         fn.Name,
            Kind:          CompletionItemKindFunction,
            Detail:        fn.ReturnType,
            Documentation: fn.Doc,
        })
    }

    // 2. Loaded symbols
    for _, load := range doc.Index.Loads {
        // ... add imported symbols
    }

    // 3. Local definitions
    for _, def := range doc.Index.Defs {
        // ... add local functions
    }

    return &CompletionList{Items: items}, nil
}
```

#### Hover (`handleHover`)

```go
func (s *Server) handleHover(params *HoverParams) (*Hover, error) {
    // ... find symbol under cursor ...

    // Check builtins first
    if fn := s.findBuiltinFunction(symbolName); fn != nil {
        return &Hover{
            Contents: MarkupContent{
                Kind:  Markdown,
                Value: formatBuiltinDoc(fn),
            },
        }, nil
    }

    // Fall back to docgen for local symbols
    // ... existing implementation ...
}
```

#### Diagnostics (`publishDiagnostics`)

```go
func (s *Server) publishDiagnostics(uri DocumentURI) {
    doc := s.documents[uri]
    filekind := s.classifier.Classify(string(uri))

    // Get predeclared names from builtins
    builtins, _ := s.builtins.Builtins(s.dialect, filekind)
    predeclared := builtinsToMap(builtins)

    // Configure checker with dialect builtins
    opts := checker.Options{
        Predeclared:  predeclared,
        Universal:    checker.DefaultUniversal,
        ReportUnused: s.config.Features.ReportUnusedBindings,
    }

    chk := checker.New(opts)
    diagnostics := chk.Check(doc.Content)

    // ... publish diagnostics ...
}
```

## Implementation Plan

### Phase 1: Wire Up Existing Infrastructure

**Effort**: 1-2 days

1. Initialize builtin providers in LSP server
2. Use classifier to detect file kind
3. Pass predeclared names to checker
4. **Result**: No more false "undefined name" errors for builtins

```go
// server.go
type Server struct {
    // ... existing fields ...
    builtins   builtins.Provider
    classifier *classifier.DefaultClassifier
    dialect    string
}
```

### Phase 2: Implement Completion

**Effort**: 2-3 days

1. Replace completion stub with real implementation
2. Merge builtins + loaded symbols + local definitions
3. Context-aware filtering (after `.`, inside call, etc.)
4. **Result**: Autocomplete for `cc_lib` -> `cc_library`

### Phase 3: Configuration Support

**Effort**: 2-3 days

1. Add `.sky/config.json` parser
2. Implement path-based JSON loader
3. Support inline builtin definitions
4. Watch for config file changes
5. **Result**: Custom dialect configuration per workspace

### Phase 4: Enhanced Hover & Signatures

**Effort**: 1-2 days

1. Show builtin documentation on hover
2. Add signature help (parameter hints)
3. **Result**: Rich documentation while typing

### Phase 5: Custom Dialect Authoring Tools

**Effort**: Optional/Future

1. `sky builtins init` - Generate template config
2. `sky builtins extract` - Extract builtins from Python stubs
3. `sky builtins validate` - Validate JSON schema
4. **Result**: Easy onboarding for custom dialects

## Example: Copybara Dialect

### `.sky/config.json`

```json
{
  "dialect": "copybara",
  "builtins": {
    "paths": [".sky/copybara-builtins.json"]
  }
}
```

### `.sky/copybara-builtins.json`

```json
{
  "functions": [
    {
      "name": "core.workflow",
      "doc": "Defines a migration workflow from origin to destination",
      "params": [
        { "name": "name", "type": "string", "required": true },
        { "name": "origin", "type": "origin", "required": true },
        { "name": "destination", "type": "destination", "required": true },
        { "name": "authoring", "type": "authoring", "required": true },
        { "name": "transformations", "type": "list[transformation]", "default": "[]" }
      ],
      "return_type": "None"
    },
    {
      "name": "git.origin",
      "doc": "Defines a Git repository as the source of truth",
      "params": [
        { "name": "url", "type": "string", "required": true },
        { "name": "ref", "type": "string", "default": "\"master\"" }
      ],
      "return_type": "origin"
    }
  ],
  "types": [
    {
      "name": "origin",
      "doc": "Source repository for a workflow"
    },
    {
      "name": "destination",
      "doc": "Target repository for a workflow"
    }
  ]
}
```

## Example: Custom Internal Tool

For a tool like `bazelbump` that might define custom Starlark functions:

### `.sky/config.json`

```json
{
  "dialect": "starlark",
  "builtins": {
    "inline": {
      "functions": [
        {
          "name": "bump_version",
          "doc": "Increments the version according to semver rules",
          "params": [
            { "name": "current", "type": "string", "required": true },
            { "name": "bump_type", "type": "string", "default": "\"patch\"" }
          ],
          "return_type": "string"
        },
        {
          "name": "parse_changelog",
          "doc": "Parses a CHANGELOG.md file and returns structured data",
          "params": [
            { "name": "path", "type": "string", "required": true }
          ],
          "return_type": "dict"
        }
      ]
    }
  }
}
```

## Open Questions

1. **Config file location**: `.sky/config.json` vs `sky.config.json` vs `pyproject.toml [tool.sky]`?
2. **Initialization options**: Should LSP `initializationOptions` override config file?
3. **Remote builtins**: Support loading from URLs? (`https://example.com/builtins.json`)
4. **Python parser choice**: Tree-sitter (like Tilt) vs go-python vs custom regex-based?
5. **Starpls compatibility**: Should we aim for 100% compatible textproto format with starpls?
6. **Directory conventions**: Follow Tilt's `__init__.py` module structure or flat files?

## Implementation Priority for Formats

| Priority | Format       | Rationale                                   |
| -------- | ------------ | ------------------------------------------- |
| P0       | JSON         | Already implemented, easy to author         |
| P0       | Binary Proto | Already implemented, used for Bazel/Buck2   |
| P1       | Textproto    | Starpls compatibility, human-readable proto |
| P2       | Python Stubs | Tilt compatibility, rich ecosystem          |

## Cross-Tool Alignment

Custom dialect support is being implemented across multiple Starlark tools. We should align formats for user consistency.

### Related Work

| Tool                       | PR/Implementation                                                                                                                  | Format                        | Status      |
| -------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- | ----------------------------- | ----------- |
| **Sky (this)**             | RFC                                                                                                                                | JSON, textproto, Python stubs | Proposed    |
| **JetBrains Hirschgarten** | [PR#1](https://github.com/albertocavalcante/fork-jetbrains-hirschgarten/pull/1)                                                    | JSON + glob rules             | Implemented |
| **starpls**                | [PR#2](https://github.com/albertocavalcante/fork-starpls/pull/2), [PR#3](https://github.com/albertocavalcante/fork-starpls/pull/3) | JSON with modules             | In Progress |

### starpls Extension Format (PR#2)

```json
{
  "name": "tilt-dialect",
  "description": "Tilt-specific Starlark extensions",
  "applies_to": {
    "file_patterns": ["Tiltfile", "tilt_modules/**/*.star"]
  },
  "modules": {
    "exec": [
      {
        "name": "sh",
        "type": "function",
        "doc": "Execute shell command",
        "callable": {
          "params": [{"name": "cmd", "type": "string"}],
          "return_type": "ExecResult"
        }
      }
    ]
  }
}
```

### JetBrains Hirschgarten Format

### Hirschgarten Config: `.starlark-dialects.json`

```json
{
  "version": 1,
  "rules": [
    { "glob": "Tiltfile", "dialectId": "tilt", "priority": 100 },
    { "glob": "*.bara.sky", "dialectId": "copybara", "priority": 100 }
  ],
  "builtinFilesByDialect": {
    "tilt": ["starlark/tilt.builtins.json"],
    "copybara": ["starlark/copybara.builtins.json"]
  }
}
```

### Hirschgarten Builtins Format

```json
{
  "functions": [
    {
      "name": "docker_build",
      "doc": "Build an image",
      "params": [
        { "name": "ref", "positional": true, "named": false, "required": true }
      ]
    }
  ]
}
```

### Proposed Alignment

To maximize user convenience, we could:

1. **Support the same config filename**: `.starlark-dialects.json`
2. **Support the same builtins JSON schema** (with extensions for types/globals)
3. **Users write once, works in both Sky LSP and JetBrains**

| Field                 | Hirschgarten        | Sky (Current) | Aligned             |
| --------------------- | ------------------- | ------------- | ------------------- |
| `name`                | ✅                  | ✅            | ✅                  |
| `doc`                 | ✅                  | ✅            | ✅                  |
| `params[].name`       | ✅                  | ✅            | ✅                  |
| `params[].required`   | ✅                  | ✅            | ✅                  |
| `params[].positional` | ✅                  | ❌            | Add                 |
| `params[].named`      | ✅                  | ❌            | Add                 |
| `params[].type`       | ❌                  | ✅            | Add to Hirschgarten |
| `params[].default`    | ✅ (`defaultValue`) | ✅            | Alias both          |
| `return_type`         | ❌                  | ✅            | Add to Hirschgarten |
| `types`               | ❌                  | ✅            | Sky extension       |
| `globals`             | ❌                  | ✅            | Sky extension       |

## References

- [JSON Loader Documentation](../internal/starlark/builtins/loader/JSON_LOADER.md)
- [JetBrains Hirschgarten BYO Dialects PR](https://github.com/albertocavalcante/fork-jetbrains-hirschgarten/pull/1) - IntelliJ implementation
- [starlark-lsp (Tilt)](https://github.com/tilt-dev/starlark-lsp) - Python stub approach
- [starpls](https://github.com/withered-magic/starpls) - Proto-based approach
- [starpls#379: Support custom stubs](https://github.com/withered-magic/starpls/issues/379) - Discussion on format support
- [Pyright Configuration](https://github.com/microsoft/pyright/blob/main/docs/configuration.md)
- [LSP Specification](https://microsoft.github.io/language-server-protocol/)
- [Bazel builtins.proto](https://github.com/bazelbuild/bazel/blob/master/src/main/java/com/google/devtools/build/docgen/starlark/proto/starlark_doc_extract.proto)
