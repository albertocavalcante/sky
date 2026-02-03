# Plan: Plugin CLI Polish & Build Improvements

## Overview

Two work streams to improve the developer and user experience:

1. **Plugin CLI Polish** - Better UX for the plugin system
2. **Build Improvements** - Address PR #30 feedback (Bazel cross-compilation, reduce duplication)

---

## Part 1: Plugin CLI Polish

### 1.1 Better Error Messages

**Current behavior:**

```bash
$ sky lint file.star
unknown command "lint"
install plugins with: sky plugin search <query>
```

**Desired behavior:**

```bash
$ sky lint file.star
sky: command "lint" not found

Did you mean one of these?
  sky fmt     - format Starlark files
  sky check   - static analysis

Or install a plugin:
  sky plugin install lint
  sky plugin search lint
```

**Implementation:**

```go
// cmd/sky/main.go

func runInstalledPlugin(args []string, stdout, stderr io.Writer) int {
    // ... existing lookup ...

    if plugin == nil {
        cmdName := args[0]

        // Check for similar core commands
        suggestions := findSimilarCommands(cmdName, coreCommands)

        writef(stderr, "sky: command %q not found\n\n", cmdName)

        if len(suggestions) > 0 {
            writeln(stderr, "Did you mean one of these?")
            for _, s := range suggestions {
                writef(stderr, "  sky %-8s - %s\n", s.name, s.desc)
            }
            writeln(stderr)
        }

        writeln(stderr, "Or install a plugin:")
        writef(stderr, "  sky plugin install %s\n", cmdName)
        writef(stderr, "  sky plugin search %s\n", cmdName)

        return 2
    }
    // ...
}

func findSimilarCommands(input string, commands map[string]string) []suggestion {
    // Use Levenshtein distance or prefix matching
    // Return commands with distance <= 2 or matching prefix
}
```

### 1.2 Plugin Init Scaffolding

**Command:**

```bash
sky plugin init <name> [--wasm]
```

**Creates:**

```
my-plugin/
├── main.go
├── go.mod
└── README.md
```

**Template (main.go):**

```go
package main

import (
    "encoding/json"
    "fmt"
    "os"
)

const (
    pluginName    = "{{.Name}}"
    pluginVersion = "0.1.0"
    pluginSummary = "A Sky plugin"
)

func main() {
    if os.Getenv("SKY_PLUGIN_MODE") == "metadata" {
        json.NewEncoder(os.Stdout).Encode(map[string]any{
            "api_version": 1,
            "name":        pluginName,
            "version":     pluginVersion,
            "summary":     pluginSummary,
            "commands": []map[string]string{
                {"name": pluginName, "description": pluginSummary},
            },
        })
        return
    }

    if len(os.Args) > 1 && os.Args[1] == "--version" {
        fmt.Printf("%s %s\n", pluginName, pluginVersion)
        return
    }

    // Your plugin logic here
    fmt.Println("Hello from", pluginName)
}
```

**Implementation:**

- Add `sky plugin init` subcommand in `cmd/sky/main.go`
- Create `internal/plugins/scaffold.go` with templates
- Support `--wasm` flag for WASM plugin template

### 1.3 Plugin Update Notifications (Optional)

Check for updates when running plugins (with caching to avoid slowdown).

```go
// internal/plugins/update.go

type UpdateChecker struct {
    cacheFile string  // ~/.config/sky/update-check.json
    interval  time.Duration  // 24h
}

func (c *UpdateChecker) CheckForUpdates(plugins []Plugin) []UpdateAvailable {
    // Check cache timestamp
    // If stale, query marketplaces in background
    // Return available updates
}
```

**UX:**

```bash
$ sky myplugin args
# ... plugin output ...

Note: Update available for myplugin (1.0.0 → 1.2.0)
Run: sky plugin update myplugin
```

---

## Part 2: Build Improvements (PR #30 Feedback)

### 2.1 Reduce BUILD.bazel Duplication

**Current:**

```starlark
go_library(
    name = "sky_lib",
    srcs = ["embedded.go", "embedded_minimal.go", "main.go"],
    deps = ["//internal/plugins", "//internal/version"],
)

go_library(
    name = "sky_full_lib",
    srcs = ["embedded.go", "embedded_full.go", "main.go"],
    deps = [
        "//internal/cmd/skycheck",
        # ... 8 more deps ...
        "//internal/plugins",
        "//internal/version",
    ],
)
```

**Improved:**

```starlark
load("@rules_go//go:def.bzl", "go_binary", "go_library")

# Shared sources and deps
_COMMON_SRCS = ["embedded.go", "main.go"]
_COMMON_DEPS = [
    "//internal/plugins",
    "//internal/version",
]

_EMBEDDED_TOOL_DEPS = [
    "//internal/cmd/skycheck",
    "//internal/cmd/skycov",
    "//internal/cmd/skydoc",
    "//internal/cmd/skyfmt",
    "//internal/cmd/skylint",
    "//internal/cmd/skyquery",
    "//internal/cmd/skyrepl",
    "//internal/cmd/skytest",
]

# Minimal build
go_library(
    name = "sky_lib",
    srcs = _COMMON_SRCS + ["embedded_minimal.go"],
    importpath = "github.com/albertocavalcante/sky/cmd/sky",
    visibility = ["//visibility:private"],
    deps = _COMMON_DEPS,
)

go_binary(
    name = "sky",
    embed = [":sky_lib"],
    visibility = ["//visibility:public"],
)

# Full build
go_library(
    name = "sky_full_lib",
    srcs = _COMMON_SRCS + ["embedded_full.go"],
    importpath = "github.com/albertocavalcante/sky/cmd/sky",
    visibility = ["//visibility:private"],
    deps = _COMMON_DEPS + _EMBEDDED_TOOL_DEPS,
)

go_binary(
    name = "sky_full",
    embed = [":sky_full_lib"],
    visibility = ["//visibility:public"],
)
```

### 2.2 Use Bazel for Cross-Compilation

**Current:** Uses `go build` directly
**Improved:** Use Bazel with platform transitions

**Justfile refactor:**

```just
# Output directory
dist_dir := "dist"

# Platform configurations
platforms := "linux_amd64 linux_arm64 darwin_arm64 windows_amd64"

# Helper: Build with Bazel for a specific platform
_bazel-build platform target output:
    bazel build --platforms=@io_bazel_rules_go//go/toolchain:{{platform}} //cmd/sky:{{target}}
    @mkdir -p {{dist_dir}}
    @cp bazel-bin/cmd/sky/{{target}}_/{{target}} {{dist_dir}}/{{output}}

# Build sky_full for current platform
build-sky-full:
    bazel build //cmd/sky:sky_full
    @mkdir -p {{dist_dir}}
    @cp bazel-bin/cmd/sky/sky_full_/sky_full {{dist_dir}}/sky_full

# Build sky_full for all platforms
build-all:
    just _bazel-build linux_amd64 sky_full sky-linux-amd64
    just _bazel-build linux_arm64 sky_full sky-linux-arm64
    just _bazel-build darwin_arm64 sky_full sky-darwin-arm64
    just _bazel-build windows_amd64 sky_full sky-windows-amd64.exe
    @echo "Built all platforms in {{dist_dir}}/"

# Build minimal sky for all platforms
build-all-minimal:
    just _bazel-build linux_amd64 sky sky-minimal-linux-amd64
    just _bazel-build linux_arm64 sky sky-minimal-linux-arm64
    just _bazel-build darwin_arm64 sky sky-minimal-darwin-arm64
    just _bazel-build windows_amd64 sky sky-minimal-windows-amd64.exe
    @echo "Built all minimal platforms in {{dist_dir}}/"
```

**Note:** Bazel cross-compilation requires:

1. `rules_go` platform definitions
2. Possibly `--incompatible_enable_cc_toolchain_resolution`
3. Testing on actual CI (may need toolchain registration)

**Alternative:** Keep `go build` for simplicity, document why:

- Faster for local dev (no Bazel startup)
- Works without Bazel installed
- Cross-compilation is a release concern, not dev workflow

---

## Implementation Order

### Phase 1: Quick Wins (1-2 hours)

1. Reduce BUILD.bazel duplication (safe refactor)
2. Better error messages for unknown commands

### Phase 2: Plugin Init (1-2 hours)

1. Add `sky plugin init` command
2. Create scaffold templates

### Phase 3: Build Improvements (Optional, 2-3 hours)

1. Evaluate Bazel cross-compilation complexity
2. If complex, document the `go build` approach reasoning
3. If straightforward, implement Bazel-based cross-compilation

### Phase 4: Update Notifications (Optional, 1-2 hours)

1. Add background update checker
2. Cache results to avoid slowdown

---

## Files to Modify

| File                           | Changes                                  |
| ------------------------------ | ---------------------------------------- |
| `cmd/sky/main.go`              | Better error messages, `sky plugin init` |
| `cmd/sky/BUILD.bazel`          | Reduce duplication with variables        |
| `internal/plugins/scaffold.go` | New file for plugin templates            |
| `justfile`                     | Parameterize build recipes               |

---

## Success Criteria

- [ ] `sky unknowncmd` shows helpful error with suggestions
- [ ] `sky plugin init myplug` creates working plugin scaffold
- [ ] `go build ./myplug && sky plugin install --path ./myplug myplug` works
- [ ] BUILD.bazel has no duplicated source/dep lists
- [ ] Justfile recipes are DRY with parameters
