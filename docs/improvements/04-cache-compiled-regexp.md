# 04 — Cache compiled regexp in query `filter()`

## Category

Performance

## Effort

~15 minutes

## Files

- `internal/starlark/query/funcs.go` — around line 159

## Problem

The `filter()` function inside `Engine.Eval()` compiles a regexp from the user's
pattern string on every call:

```go
re, err := regexp.Compile(patternStr.Value)
```

If the same query is run repeatedly (e.g. in watch mode or editor integration),
the same pattern gets recompiled each time.

## Fix

Add a small cache at the `Engine` level (or package level):

```go
type Engine struct {
    // ...
    regexpCache map[string]*regexp.Regexp
}

func (e *Engine) cachedRegexp(pattern string) (*regexp.Regexp, error) {
    if re, ok := e.regexpCache[pattern]; ok {
        return re, nil
    }
    re, err := regexp.Compile(pattern)
    if err != nil {
        return nil, err
    }
    e.regexpCache[pattern] = re
    return re, nil
}
```

Then call `e.cachedRegexp(patternStr.Value)` in the filter path.

If the Engine is long-lived, consider bounding the cache size or using an LRU.

## Acceptance Criteria

- Repeated `filter()` calls with the same pattern reuse the compiled regexp
- Invalid patterns still return errors
- Existing tests pass
- Cache is bounded or scoped to Engine lifetime
