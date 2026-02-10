# 14 — Cache WASM binaries for repeated plugin execution

## Category

Performance

## Effort

~1–2 hours

## Files

- `internal/plugins/runner_wasi.go` — line 15

## Problem

Every WASM plugin invocation reads the entire binary into memory from disk:

```go
func runWasm(ctx context.Context, plugin Plugin, ...) (int, error) {
    wasmBytes, err := os.ReadFile(plugin.Path)
    // ...
    runtime := wazero.NewRuntime(ctx)
    // ...
}
```

WASM binaries can be large. Re-reading and re-compiling them on every execution
is wasteful, especially for frequently-used plugins.

## Fix

Use wazero's `CompilationCache` to cache compiled WASM modules:

```go
var (
    compilationCache wazero.CompilationCache
    cacheOnce        sync.Once
)

func getCompilationCache() wazero.CompilationCache {
    cacheOnce.Do(func() {
        compilationCache = wazero.NewCompilationCache()
    })
    return compilationCache
}

func runWasm(ctx context.Context, plugin Plugin, ...) (int, error) {
    wasmBytes, err := os.ReadFile(plugin.Path)
    if err != nil {
        return 1, err
    }

    config := wazero.NewRuntimeConfig().
        WithCompilationCache(getCompilationCache())
    runtime := wazero.NewRuntimeWithConfig(ctx, config)
    defer runtime.Close(ctx)
    // ...
}
```

For even better performance, consider a file-based mmap approach or
`os.ReadFile` → `sync.Map` keyed by `plugin.Path + modtime`.

## Acceptance Criteria

- Repeated WASM plugin executions reuse compiled modules
- Cache respects file changes (invalidation by path + mtime)
- No memory leaks (cache is bounded or cleaned up)
- Existing plugin tests pass
