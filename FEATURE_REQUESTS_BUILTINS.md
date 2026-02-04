# Sky: Builtin Feature Requests

These are feature requests discovered while implementing bazelbump's recipe testing integration with skytest.

---

## 1. `struct` Builtin

### Problem

Vanilla Starlark doesn't have `struct`, but it's ubiquitous in Bazel/Starlark codebases. Without it, we must use dicts everywhere:

```starlark
# What we want
result = struct(name="foo", version="1.0.0")
print(result.name)  # Attribute access

# What we have to do
result = {"name": "foo", "version": "1.0.0"}
print(result["name"])  # Dict access only
```

### Impact

- **Readability**: `config.name` is cleaner than `config["name"]`
- **Type safety**: Structs are immutable, dicts are mutable
- **Bazel compatibility**: Most Starlark code uses struct extensively
- **IDE support**: Attribute access enables better autocomplete

### Proposal

Add `struct` as a predeclared builtin in skytest (and ideally all sky tools):

```go
// In tester/tester.go buildPredeclared()
import "go.starlark.net/starlarkstruct"

predeclared["struct"] = starlark.NewBuiltin("struct", starlarkstruct.Make)
```

### Priority: **P0** (High impact, low effort)

---

## 2. `provider` Builtin

### Problem

For more complex type definitions, Bazel uses `provider()`:

```starlark
# Bazel pattern
MyInfo = provider(fields = ["name", "deps"])
info = MyInfo(name="foo", deps=[])
```

### Use Case

Creating custom "types" for testing frameworks, configuration schemas, etc.

### Priority: **P2** (Medium impact, medium effort)

---

## 3. Mutable Prelude State

### Problem

Prelude globals get frozen after loading, making operation tracking impossible:

```starlark
# In prelude.star
_operations = []  # This list gets frozen!

def record_operation(op):
    _operations.append(op)  # ERROR: cannot append to frozen list
```

### Workaround We Used

Operations return dicts instead of recording to global state. Tests verify returned dicts directly.

### Potential Solutions

1. **Option A**: Don't freeze prelude globals (breaking change?)
2. **Option B**: Provide a `mutable_state()` helper that creates unfrozen containers
3. **Option C**: Per-test fresh prelude execution (expensive but clean)
4. **Option D**: Built-in test context fixture with mutable state

### Recommended: Option D

```starlark
# Injected by skytest when test function has 'ctx' parameter
def test_with_context(ctx):
    ctx.record("operation", type="ensure_load", rule="cc_library")
    ops = ctx.get_recorded()
    assert.eq(len(ops), 1)
```

### Priority: **P1** (High impact for testing patterns)

---

## 4. `conftest.star` Auto-Discovery

### Current State

The `--prelude` flag works but requires explicit paths.

### Proposal

Auto-discover `conftest.star` files (like pytest's `conftest.py`):

```
tests/
  conftest.star      # Loaded for all tests in tests/
  unit/
    conftest.star    # Loaded for tests in unit/, after parent conftest
    test_foo.star
  integration/
    test_bar.star    # Uses only tests/conftest.star
```

### Behavior

1. Walk up from test file to find all `conftest.star` files
2. Load them in order (root first, deepest last)
3. Each conftest's globals become available to tests

### Priority: **P2** (Quality of life improvement)

---

## 5. Test Context Object

### Problem

Tests often need:

- Temporary directories
- Mutable state for tracking
- Test metadata (name, file, etc.)

### Proposal

Inject a `testing` context when requested:

```starlark
def test_with_context(t):
    # Test metadata
    print(t.name)  # "test_with_context"
    print(t.file)  # "test_example.star"

    # Mutable state
    t.set("key", "value")
    assert.eq(t.get("key"), "value")

    # Skip/fail helpers
    if some_condition:
        t.skip("not applicable")

    # Cleanup registration
    t.cleanup(lambda: cleanup_resources())
```

### Priority: **P1** (Enables advanced testing patterns)

---

## 6. `load()` Mocking

### Problem

Can't mock modules loaded via `load()` because resolution happens at parse time.

### Current Workaround

Use dependency injection via fixtures instead of direct loads.

### Potential Solution

```bash
# Override load paths for testing
skytest --load-override "//real/module=//test/mock_module" tests/
```

Or in conftest.star:

```starlark
__load_overrides__ = {
    "//production/api": "//test/mocks/api",
}
```

### Priority: **P3** (Complex, workarounds exist)

---

## 7. Assertion Improvements

### Current

```starlark
assert.eq(actual, expected)
# Output: assert.eq failed: got X, expected Y
```

### Proposed Additions

```starlark
# Custom message
assert.eq(actual, expected, msg="config parsing failed")

# Collection assertions
assert.len(items, 3)
assert.empty(items)
assert.not_empty(items)

# String assertions
assert.contains_string(text, "substring")
assert.matches(text, r"pattern.*")
assert.starts_with(text, "prefix")
assert.ends_with(text, "suffix")

# Numeric assertions
assert.greater(a, b)
assert.less(a, b)
assert.between(value, min, max)
assert.approx(actual, expected, tolerance=0.001)

# Type assertions
assert.is_string(value)
assert.is_list(value)
assert.is_dict(value)
assert.is_none(value)
assert.is_not_none(value)

# Exception assertions
assert.fails(lambda: risky_operation(), "expected error")
```

### Priority: **P1** (High value, incremental)

---

## 8. Test Tagging via Docstrings

### Problem

Can't use `@mark.slow` decorator with arguments in Starlark.

### Proposal

Parse docstrings for tags:

```starlark
def test_slow_operation():
    """Test description.

    Tags: slow, integration
    Skip: CI environment not configured
    Timeout: 5m
    """
    ...
```

Skytest parses the docstring and extracts:

- `Tags:` for `-m` filtering
- `Skip:` for conditional skip with reason
- `Timeout:` for per-test timeout override

### Priority: **P2** (Creative solution to decorator limitation)

---

## 9. Built-in `json` Module

### Problem

No way to parse/serialize JSON in tests.

### Proposal

```starlark
load("json", "json")  # Or as predeclared

def test_json_parsing():
    data = json.decode('{"key": "value"}')
    assert.eq(data["key"], "value")

    text = json.encode({"foo": [1, 2, 3]})
    assert.eq(text, '{"foo":[1,2,3]}')
```

### Priority: **P2** (Common need)

---

## 10. Test Data Files

### Problem

No built-in way to load test fixtures from files.

### Proposal

```starlark
def test_with_fixture(testdata):
    # testdata is relative to test file location
    content = testdata.read("input.json")
    expected = testdata.read("expected.json")

    result = process(json.decode(content))
    assert.eq(result, json.decode(expected))
```

Directory structure:

```
tests/
  test_parser.star
  testdata/
    test_parser/      # Named after test file
      input.json
      expected.json
```

### Priority: **P2** (Convenience feature)

---

## Summary

| Feature                   | Priority | Effort | Impact | Status            |
| ------------------------- | -------- | ------ | ------ | ----------------- |
| `struct` builtin          | P0       | Low    | High   | ✅ Done           |
| `json` module             | P2       | Low    | Medium | ✅ Done           |
| `assert.len()`            | P1       | Low    | High   | ✅ Done           |
| `assert.empty()`          | P1       | Low    | High   | ✅ Done           |
| `assert.not_empty()`      | P1       | Low    | High   | ✅ Done           |
| Assertion improvements    | P1       | Medium | High   | Partial           |
| Test context object       | P1       | Medium | High   | Open              |
| Mutable prelude state     | P1       | High   | High   | Open              |
| `conftest.star` discovery | P2       | Medium | Medium | ✅ Done (Phase 2) |
| `provider` builtin        | P2       | Low    | Medium | Open              |
| Docstring tags            | P2       | Medium | Medium | Open              |
| Test data files           | P2       | Medium | Medium | Open              |
| `load()` mocking          | P3       | High   | Low    | Open              |

---

## Quick Wins - Implemented

The following quick wins have been implemented:

1. ✅ **`struct` builtin** - Added to predeclared (single line)
2. ✅ **`json` module** - Added for JSON encode/decode in tests
3. ✅ **`assert.len(container, expected)`** - Assert length
4. ✅ **`assert.empty(container)`** - Assert container is empty
5. ✅ **`assert.not_empty(container)`** - Assert container is not empty
6. ✅ **`assert.fails(fn, pattern)`** - Already existed!
7. ✅ **`assert.contains(container, item)`** - Already existed (incl. strings)

## Quick Wins - Still Open

1. **Add `assert.matches(text, pattern)`** - Regex matching
2. **Add `assert.starts_with(text, prefix)`** - String prefix check
3. **Add `assert.ends_with(text, suffix)`** - String suffix check

---

_From bazelbump integration experience_
_Date: 2025-02-04_
