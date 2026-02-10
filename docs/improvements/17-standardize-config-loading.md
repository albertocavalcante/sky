# 17 — Standardize configuration loading pattern

## Category

API consistency

## Effort

~3–4 hours

## Files

- `internal/starlark/linter/config.go` — `LoadConfig() *Config`
- `internal/skyconfig/` — separate config loader
- `internal/cmd/skytest/run.go` — flag-based config
- Other `internal/cmd/*/run.go` files

## Problem

Different tools handle configuration differently:

| Tool        | Config Source   | Pattern                             |
| ----------- | --------------- | ----------------------------------- |
| `skylint`   | JSON file       | `LoadConfig(path) (*Config, error)` |
| `skytest`   | CLI flags only  | Direct `flag.Parse()`               |
| `skyconfig` | TOML + Starlark | Separate package with `Load()`      |

Users must learn different config mechanisms for each tool. Adding a new config
option requires understanding the specific tool's approach.

## Proposed Design

### Unified config loading interface:

```go
// internal/skyconfig/loader.go
type Loader interface {
    // Load reads configuration from the standard discovery chain:
    // 1. CLI flags (highest priority)
    // 2. sky.toml / config.sky in current or ancestor directory
    // 3. Defaults
    Load(ctx context.Context) (*Config, error)
}
```

### Standard discovery:

```go
// FindConfigFile walks up from dir to repo root looking for sky.toml or config.sky
func FindConfigFile(dir string) (string, error)
```

### Per-tool config sections:

```toml
# sky.toml
[lint]
config = "path/to/lint.json"
fix = true

[test]
parallel = 4
verbose = true

[format]
dialect = "bazel"
```

Each tool reads its section, with CLI flags overriding file values.

## Approach

1. Define the `Loader` interface in `internal/skyconfig/`
2. Implement a default loader that handles TOML + flag merging
3. Migrate one tool (e.g. `skylint`) as a proof of concept
4. Document the pattern for other tools to follow

## Acceptance Criteria

- `Loader` interface defined and documented
- At least one tool migrated to the new pattern
- CLI flags still override file config
- Config discovery works (walk up to repo root)
- Existing tests pass
- Migration guide documented for remaining tools
