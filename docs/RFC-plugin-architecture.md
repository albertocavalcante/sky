# RFC: Plugin-First Architecture for Sky CLI

## Summary

Redesign Sky CLI to be plugin-first while supporting two distribution modes:

1. **Modular**: Minimal `sky` binary + individual plugins installed on demand
2. **Bundled**: Single `sky` binary with all core tools compiled in

## Current State

The existing architecture already has strong foundations:

```
cmd/sky/main.go          # Dispatcher (535 lines)
├── Core command mapping  # fmt → skyfmt, lint → skylint, etc.
├── Plugin resolution     # Falls back to plugins for unknown commands
└── Plugin management     # install, uninstall, list, search, update

internal/plugins/
├── store.go             # Plugin storage (~/.config/sky/plugins/)
├── runner_exec.go       # Execute native binaries
├── runner_wasi.go       # Execute WASM via wazero
├── marketplace.go       # Remote plugin registry
└── protocol.go          # Metadata protocol (API v1)
```

**Problem**: Core tools are currently external binaries that `sky` shells out to. This creates:

- Larger distribution footprint (8 separate binaries)
- Slower execution (process spawn overhead)
- Complex installation (need all binaries in PATH)

## Proposed Architecture

### Core Concept: Unified Plugin Interface

```go
// internal/plugin/interface.go
package plugin

// Plugin is the interface all commands must implement
type Plugin interface {
    // Metadata returns plugin information
    Metadata() Metadata

    // Run executes the plugin with given args
    // Returns exit code
    Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int
}

type Metadata struct {
    Name        string
    Version     string
    Summary     string
    Commands    []Command
}

type Command struct {
    Name        string
    Description string
    Usage       string
}
```

### Registry Pattern

```go
// internal/plugin/registry.go
package plugin

var registry = make(map[string]Plugin)

// Register adds a plugin to the embedded registry
// Called from init() in each core plugin package
func Register(name string, p Plugin) {
    registry[name] = p
}

// Get returns an embedded plugin by name
func Get(name string) (Plugin, bool) {
    p, ok := registry[name]
    return p, ok
}

// List returns all embedded plugins
func List() map[string]Plugin {
    return registry
}
```

### Core Tool Refactoring (Minimal Change)

**Goal**: Keep each tool independently installable via `go install` while allowing embedding.

Each `cmd/*` package exports a `Run()` function. The `main.go` just calls it:

```
cmd/skylint/
├── main.go      # func main() { os.Exit(Run(os.Args[1:])) }
├── run.go       # func Run(args []string) int { ... }
└── BUILD.bazel
```

This preserves:

- `go install github.com/user/sky/cmd/skylint@latest` ✓
- Independent releases per tool ✓
- Tool can have its own go.mod if desired (multi-module repo) ✓

Example refactor:

```go
// cmd/skylint/main.go
package main

import "os"

func main() {
    os.Exit(Run(os.Args[1:]))
}
```

```go
// cmd/skylint/run.go
package main

import (
    "context"
    "io"
)

// Run executes skylint with the given arguments.
// Returns exit code.
func Run(args []string) int {
    return RunWithIO(context.Background(), args, os.Stdin, os.Stdout, os.Stderr)
}

// RunWithIO allows custom IO for embedding/testing.
func RunWithIO(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
    // existing logic here...
}
```

The `sky` binary imports and calls these directly:

```go
// cmd/sky/embedded.go
//go:build sky_full

package main

import (
    "github.com/user/sky/cmd/skylint"
    "github.com/user/sky/cmd/skyfmt"
    // ...
)

var embeddedTools = map[string]func([]string) int{
    "lint": skylint.Run,
    "fmt":  skyfmt.Run,
    // ...
}
```

**No `internal/tools/` needed.** Each tool stays in `cmd/` as its own thing.

### Build Tags for Distribution Modes

```go
// cmd/sky/embedded_full.go
//go:build sky_full

package main

import (
    // Import all core tools - their init() registers them
    _ "github.com/user/sky/internal/tools/lint"
    _ "github.com/user/sky/internal/tools/fmt"
    _ "github.com/user/sky/internal/tools/test"
    _ "github.com/user/sky/internal/tools/check"
    _ "github.com/user/sky/internal/tools/query"
    _ "github.com/user/sky/internal/tools/repl"
    _ "github.com/user/sky/internal/tools/doc"
    _ "github.com/user/sky/internal/tools/cov"
)
```

```go
// cmd/sky/embedded_minimal.go
//go:build !sky_full

package main

// Minimal build - no embedded tools
// All commands resolved via plugin system
```

### Command Resolution Priority

```go
// cmd/sky/main.go

func resolveCommand(name string) (runner, error) {
    // 1. Check embedded plugins (compile-time)
    if p, ok := plugin.Get(name); ok {
        return &embeddedRunner{p}, nil
    }

    // 2. Check installed plugins (runtime)
    if p, err := plugins.DefaultStore().FindPlugin(name); err == nil {
        return &externalRunner{p}, nil
    }

    // 3. Check PATH for sky-<name> binary (convention)
    if path, err := exec.LookPath("sky-" + name); err == nil {
        return &pathRunner{path}, nil
    }

    return nil, fmt.Errorf("unknown command: %s", name)
}
```

## Distribution Modes

### Mode 1: Bundled (Single Binary)

```bash
# Build with all core tools embedded
bazel build //cmd/sky:sky_full
# or
go build -tags=sky_full -o sky ./cmd/sky

# Result: Single ~15MB binary with everything
./sky lint file.star       # Uses embedded lint
./sky fmt file.star        # Uses embedded fmt
./sky myplugin args        # Falls back to plugin system
```

**Pros**:

- Single binary distribution
- Faster execution (no process spawn)
- Simpler installation
- Works offline

**Cons**:

- Larger binary size
- Can't update individual tools

### Mode 2: Modular (Plugin-First)

```bash
# Build minimal sky binary
bazel build //cmd/sky:sky
# or
go build -o sky ./cmd/sky

# Result: Small ~3MB binary, plugins installed on demand
./sky lint file.star
# → "lint" not found. Install with: sky plugin install lint

./sky plugin install lint
# → Installing lint v1.0.0 from marketplace...

./sky lint file.star
# → Uses installed plugin
```

**Pros**:

- Minimal initial footprint
- Update tools independently
- Install only what you need
- Community plugins same as core

**Cons**:

- Network required for initial setup
- Slightly slower (process spawn)

### Mode 3: Hybrid (Recommended Default)

```bash
# Build with core tools embedded, but allow overrides
bazel build //cmd/sky:sky_hybrid

./sky lint file.star       # Uses embedded v1.0.0

# User installs newer version
./sky plugin install lint@2.0.0

./sky lint file.star       # Uses plugin v2.0.0 (override)

# Remove override to use embedded
./sky plugin uninstall lint
```

Resolution order in hybrid mode:

1. Installed plugins (explicit user choice)
2. Embedded plugins (compiled defaults)
3. PATH lookup (sky-* convention)

## Plugin Installation UX

### Install from Marketplace

```bash
sky plugin install lint              # Latest from default marketplace
sky plugin install lint@1.2.3        # Specific version
sky plugin install acme/lint         # From specific publisher
```

### Install from URL

```bash
sky plugin install --url https://example.com/myplugin-v1.0.0-darwin-arm64
sky plugin install --url https://example.com/myplugin.wasm
```

### Install from Local Path

```bash
sky plugin install --path ./my-plugin
sky plugin install --path ./my-plugin.wasm
```

### List & Manage

```bash
sky plugin list                      # Show installed plugins
sky plugin list --all                # Include embedded
sky plugin update                    # Update all plugins
sky plugin update lint               # Update specific plugin
sky plugin uninstall lint            # Remove plugin
```

## Plugin Development

### Creating a Plugin (Go)

```bash
mkdir my-sky-plugin && cd my-sky-plugin
sky plugin init my-plugin

# Creates:
# ├── main.go
# ├── go.mod
# └── plugin.json
```

```go
// main.go
package main

import (
    "encoding/json"
    "fmt"
    "os"
)

func main() {
    if os.Getenv("SKY_PLUGIN_MODE") == "metadata" {
        json.NewEncoder(os.Stdout).Encode(map[string]any{
            "api_version": 1,
            "name":        "my-plugin",
            "version":     "1.0.0",
            "summary":     "My custom plugin",
            "commands": []map[string]string{
                {"name": "my-plugin", "description": "Does something"},
            },
        })
        return
    }

    // Your plugin logic here
    fmt.Println("Hello from my-plugin!")
}
```

### Creating a Plugin (WASM)

```bash
sky plugin init my-plugin --wasm

# Build to WASM
GOOS=wasip1 GOARCH=wasm go build -o my-plugin.wasm
```

### Testing Plugins

```bash
sky plugin run ./my-plugin -- arg1 arg2    # Test local plugin
sky plugin metadata ./my-plugin            # Verify metadata
```

## Migration Path

### Phase 1: Export Run() from Core Tools

Minimal refactor - each tool exports `Run(args) int` and `RunWithIO(ctx, args, stdin, stdout, stderr) int`:

```bash
# For each tool:
cmd/skylint/main.go  →  just calls Run()
cmd/skylint/run.go   →  exported Run() + RunWithIO()
```

This is backwards compatible. Standalone binaries work exactly as before.

### Phase 2: Build Tags for sky Binary

Add `cmd/sky/embedded.go` with build tag that imports all tools:

```go
//go:build sky_full

var embeddedTools = map[string]func([]string) int{
    "lint": skylint.Run,
    ...
}
```

Update resolution logic to check `embeddedTools` first.

### Phase 3: Plugin CLI Polish

1. Better error messages ("lint not found, install with: sky plugin install lint")
2. `sky plugin init` scaffolding for new plugins
3. Plugin update notifications

### Phase 4: Documentation

1. Plugin development guide
2. "go install" instructions for individual tools
3. Distribution guide (full vs minimal)

## Build Configuration

### Bazel Targets

```starlark
# cmd/sky/BUILD.bazel

# Minimal binary (plugins only)
go_binary(
    name = "sky",
    srcs = glob(["*.go"], exclude = ["embedded_full.go"]),
    deps = ["//internal/plugins", ...],
)

# Full binary (all core tools embedded)
go_binary(
    name = "sky_full",
    srcs = glob(["*.go"], exclude = ["embedded_minimal.go"]),
    deps = [
        "//internal/plugins",
        "//internal/tools/lint",
        "//internal/tools/fmt",
        ...
    ],
    gotags = ["sky_full"],
)

# Hybrid binary (embedded + override support)
go_binary(
    name = "sky_hybrid",
    srcs = glob(["*.go"], exclude = ["embedded_minimal.go"]),
    deps = [...],
    gotags = ["sky_full", "sky_hybrid"],
)
```

### Justfile Recipes

```just
# Build minimal sky
build-minimal:
    bazel build //cmd/sky:sky

# Build full sky (recommended for distribution)
build-full:
    bazel build //cmd/sky:sky_full

# Build all variants
build-all:
    bazel build //cmd/sky:sky //cmd/sky:sky_full

# Create release archives
release:
    ./scripts/release.sh
```

## Size Estimates

| Binary               | Size (est.) | Contents                   |
| -------------------- | ----------- | -------------------------- |
| sky (minimal)        | ~3 MB       | Dispatcher + plugin system |
| sky (full)           | ~15 MB      | All 8 core tools embedded  |
| skylint (standalone) | ~8 MB       | Just linter                |
| lint.wasm            | ~5 MB       | WASM version               |

## Alternative: Multi-Module Repo

If tools need full independence (separate versioning, release cycles), consider Go workspaces:

```
sky/
├── go.work              # Workspace file
├── go.mod               # Root module (sky CLI + shared code)
├── internal/            # Shared packages
├── cmd/sky/             # Main CLI
├── cmd/skylint/
│   └── go.mod           # github.com/user/sky/cmd/skylint
├── cmd/skyfmt/
│   └── go.mod           # github.com/user/sky/cmd/skyfmt
└── ...
```

Benefits:

- Each tool has independent semver (`skylint@v2.0.0` vs `skyfmt@v1.3.0`)
- Users install exactly what they need
- Tools can depend on different versions of shared code (via replace directives)

Tradeoff:

- More complex release process
- Bazel handles this fine, but `go install` from root module gets trickier

**Recommendation**: Start with single module, split later if needed.

## Questions to Resolve

1. **Default mode**: Should releases default to full or minimal?
   - Recommendation: Full for simplicity, minimal available for advanced users

2. **Plugin override behavior**: Should installed plugins always override embedded?
   - Recommendation: Yes, explicit user action should win

3. **WASM support**: Priority for WASM plugins vs native?
   - Recommendation: Native first, WASM for portability

4. **Marketplace hosting**: Where should the default marketplace live?
   - Options: GitHub releases, dedicated server, CDN

5. **Multi-module vs single module**?
   - Recommendation: Single module for now, revisit if tools diverge significantly

## Success Metrics

- [ ] Single binary installation works
- [ ] `sky plugin install X` completes in <5 seconds
- [ ] Embedded commands have <10ms overhead vs direct call
- [ ] Plugin development guide enables community contributions
- [ ] CI builds all distribution variants

## References

- Current plugin system: `internal/plugins/`
- Protocol definition: `internal/plugins/protocol.go`
- Similar projects:
  - [Terraform plugin system](https://developer.hashicorp.com/terraform/plugin)
  - [kubectl plugins (krew)](https://krew.sigs.k8s.io/)
  - [gh CLI extensions](https://cli.github.com/manual/gh_extension)
