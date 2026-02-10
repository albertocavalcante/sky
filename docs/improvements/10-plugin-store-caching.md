# 10 — Add in-memory cache to plugin store

## Category

Performance

## Effort

~1 hour

## Files

- `internal/plugins/store.go` — lines 137, 152+, 184, 338–350

## Problem

`FindPlugin()` calls `LoadPlugins()` which reads, unmarshals, and sorts the
entire `plugins.json` file on every invocation:

```go
func (s Store) FindPlugin(name string) (*Plugin, error) {
    plugins, err := s.LoadPlugins()  // full file I/O + JSON parse + sort
    // linear search...
}
```

The CLI hits this path for every non-core command. The `readJSON` helper also
reads the entire file into memory before unmarshaling (instead of streaming).

Additionally, `sort.Slice` runs on every load even when the data is already
sorted.

## Fix

1. **Add mtime-based cache** to the `Store` struct:

```go
type Store struct {
    dir     string
    mu      sync.Mutex
    cached  []Plugin
    modTime time.Time
}

func (s *Store) LoadPlugins() ([]Plugin, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    info, err := os.Stat(s.pluginsPath())
    if err != nil { /* handle */ }

    if s.cached != nil && info.ModTime().Equal(s.modTime) {
        return s.cached, nil
    }
    // ... read, parse, sort, cache ...
}
```

2. **Use streaming JSON** in `readJSON`:

```go
func readJSON(path string, target any) error {
    f, err := os.Open(path)
    // ...
    defer f.Close()
    return json.NewDecoder(f).Decode(target)
}
```

3. **Store must become a pointer receiver** (currently value receiver `Store`).

## Acceptance Criteria

- Repeated `FindPlugin()` calls reuse cached data when file hasn't changed
- Cache invalidates when `plugins.json` is modified
- `readJSON` uses streaming decode
- Store uses pointer receivers
- Existing tests pass
