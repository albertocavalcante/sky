# RFC-001 Open Questions: Discussion & Proposals

## Question 1: Decorator Syntax

### The Problem

Starlark supports `@decorator` but **not** `@decorator(args)`:

```starlark
# Works in Starlark
@some_decorator
def foo():
    pass

# Does NOT work in Starlark
@parametrize([("a", 1), ("b", 2)])  # SyntaxError!
def test_something(name, value):
    pass
```

This is a fundamental limitation—decorators in Starlark are just syntactic sugar for `foo = decorator(foo)`, and the grammar doesn't allow call expressions.

### Proposed Solution: Hybrid Approach

Use **three complementary patterns** based on complexity:

#### Pattern A: Simple markers via `@mark.X`

For boolean markers, use zero-arg decorators with a `mark` module:

```starlark
load("testing", "mark")

@mark.slow
def test_large_file():
    ...

@mark.integration
@mark.requires_network
def test_api_call():
    ...

@mark.skip  # No reason needed for simple skip
def test_wip():
    ...
```

**Implementation:** `mark` is a struct with callable attributes:

```starlark
# In sky's testing module
def _make_marker(name):
    def marker(fn):
        if not hasattr(fn, "_markers"):
            fn._markers = []
        fn._markers.append(name)
        return fn
    return marker

mark = struct(
    slow = _make_marker("slow"),
    integration = _make_marker("integration"),
    skip = _make_marker("skip"),
    # etc.
)
```

#### Pattern B: Metadata dict for complex markers

For markers that need arguments (skip reason, xfail condition), use a **metadata dict**:

```starlark
def test_known_bug():
    ...

def test_future_feature():
    ...

def test_platform_specific():
    ...

# Metadata at module level (like __test_params__)
__test_meta__ = {
    "test_known_bug": {"xfail": "Bug #123 not fixed yet"},
    "test_future_feature": {"skip": "Not implemented"},
    "test_platform_specific": {"skip_if": "platform != 'linux'"},
}
```

**Why this works:**

- Pure data, no syntax tricks
- Starlark-idiomatic (data over magic)
- Easy to generate programmatically
- Serializable for tooling

#### Pattern C: Table-driven for parametrization

Parametrization is inherently data-driven. Embrace it:

```starlark
VERSION_CASES = [
    {"name": "less", "v1": "1.0", "v2": "2.0", "want": -1},
    {"name": "greater", "v1": "2.0", "v2": "1.0", "want": 1},
    {"name": "equal", "v1": "1.0", "v2": "1.0", "want": 0},
]

def test_version_compare(case):
    assert.eq(compare(case["v1"], case["v2"]), case["want"])

__test_params__ = {
    "test_version_compare": VERSION_CASES,
}
```

**Output:**

```
test_version_compare[less]    PASS
test_version_compare[greater] PASS
test_version_compare[equal]   PASS
```

### The Full Picture

```starlark
load("testing", "mark")

# Simple markers: decorators
@mark.slow
@mark.integration
def test_big_operation():
    ...

# Complex markers: metadata dict
def test_windows_only():
    ...

# Parametrized: table-driven
CASES = [...]
def test_with_cases(case):
    ...

# Combined: all three
__test_meta__ = {
    "test_windows_only": {"skip_if": "os != 'windows'"},
}

__test_params__ = {
    "test_with_cases": CASES,
}
```

### Rejected Alternatives

1. **String-based markers**: `def test_slow__skip__integration():` - Ugly, error-prone
2. **Runtime registration**: `testing.mark(test_foo, "slow")` - Breaks locality
3. **Comment-based**: `# @slow` - Not introspectable, fragile

---

## Question 2: Module Mocking

### The Problem

Starlark's `load()` is resolved at **parse time**, not runtime:

```starlark
load("mymodule", "fetch")  # Resolved before any code runs

def test_with_mock():
    # Too late! fetch is already bound
    mock.patch("mymodule.fetch", ...)
```

This is fundamentally different from Python where `import` is a runtime statement.

### Proposed Solution: Prelude-based Dependency Injection

**Key Insight:** We can't mock after load, but we can **provide mock-ready modules before load**.

#### Approach 1: Load from testing context

```starlark
# test_example.star
load("@testing//mymodule", "fetch")  # Load from testing context, not real module

def test_fetch():
    # @testing//mymodule is a mock-ready version
    fetch._mock.set_return({"data": 123})
    result = fetch()
    assert.eq(result["data"], 123)
```

**How it works:**

- `sky test` sets up a module resolver that intercepts `@testing//` loads
- Returns instrumented versions of real modules
- Each function is wrapped with mock capabilities

#### Approach 2: Fixture-based injection (preferred)

Don't `load` the module directly. Inject it as a fixture:

```starlark
# conftest.star
load("mymodule", _real_fetch = "fetch")

def fixture_fetch(mock):
    """Injectable fetch function that can be mocked."""
    return mock.wrap(_real_fetch)

# test_example.star
def test_with_mock(fetch, mock):
    mock.when(fetch).called_with("url").then_return({"data": 123})

    result = process_with_fetch(fetch)

    assert.eq(result["data"], 123)
    assert.true(mock.was_called(fetch))
```

**Why this works:**

- Explicit dependency injection
- Test controls what the code receives
- No magic module resolution
- Works with Starlark's existing semantics

#### Approach 3: Prelude globals shadowing

The prelude can shadow module globals:

```starlark
# test_prelude.star (loaded via --prelude)
_mocks = {}

def mock_module(name, impl):
    """Register a mock module."""
    _mocks[name] = impl

def get_mock(name):
    """Get the mock for a module."""
    return _mocks.get(name)

# Override the 'fetch' that tests will see
def fetch(url):
    if "fetch" in _mocks:
        return _mocks["fetch"](url)
    fail("fetch called but not mocked")
```

```starlark
# test_example.star (prelude already loaded)
def test_fetch():
    mock_module("fetch", lambda url: {"mocked": True})
    result = fetch("http://example.com")
    assert.eq(result["mocked"], True)
```

### Recommendation

**Use Approach 2 (fixture injection)** as the primary pattern:

1. **Explicit:** Clear what's mocked and what's real
2. **Testable:** Easy to verify mock expectations
3. **Flexible:** Different tests can use different mocks
4. **Idiomatic:** Follows Starlark's "explicit over magic" philosophy

For convenience, provide Approach 3 (prelude shadowing) for simpler cases where full DI is overkill.

### What We Cannot Do (And That's OK)

- **Transparent mocking:** Can't make `load("module", "fn")` magically return a mock
- **Global module replacement:** Can't swap out a module for all loaders

These limitations are **features**—they force explicit, testable design.

---

## Question 3: Fixture Scopes

### The Question

What does `scope="file"` mean?

- Option A: One instance **per file** (fresh for each test file)
- Option B: One instance **shared within the file** (reused across tests in same file)

### Proposed Solution: Semantic Scopes

Define four scopes with clear, unambiguous meanings:

#### `scope="test"` (default)

Fresh instance for **every test function**.

```starlark
def fixture_counter():
    return {"count": 0}

def test_one(counter):
    counter["count"] += 1
    assert.eq(counter["count"], 1)

def test_two(counter):
    counter["count"] += 1
    assert.eq(counter["count"], 1)  # Still 1, fresh instance
```

**Use for:** Mutable state, test isolation

#### `scope="file"`

One instance **shared by all tests in the same file**.

```starlark
# conftest.star
def fixture_expensive_setup():
    return expensive_computation()  # Called once per file

fixture_expensive_setup.scope = "file"

# test_foo.star
def test_a(expensive_setup):
    # Uses instance #1
    ...

def test_b(expensive_setup):
    # Reuses instance #1
    ...

# test_bar.star
def test_c(expensive_setup):
    # Uses instance #2 (different file)
    ...
```

**Use for:** Expensive setup that's safe to share within a file

#### `scope="session"`

One instance **shared across the entire test run**.

```starlark
def fixture_database_connection():
    return connect_to_test_db()

fixture_database_connection.scope = "session"
```

**Use for:** Very expensive resources (DB connections, service clients)

**Warning:** Tests using session-scoped fixtures cannot run in parallel!

#### `scope="function"` (alias for "test")

Explicit alias for clarity when reading code.

### Scope Hierarchy

```
session (1 per run)
  └── file (1 per test file)
        └── test (1 per test function)
```

Fixtures can depend on fixtures of **equal or wider scope**:

```starlark
# OK: file-scoped depends on session-scoped
def fixture_db_transaction(database_connection):  # session
    return database_connection.begin()

fixture_db_transaction.scope = "file"

# ERROR: session-scoped depends on file-scoped
def fixture_bad(some_file_scoped):
    ...

fixture_bad.scope = "session"  # Error: can't depend on narrower scope!
```

### Lifecycle Visualization

```
┌─────────────────────────────────────────────────┐
│ Session Start                                   │
│   └── Create session fixtures                   │
│                                                 │
│   ┌─────────────────────────────────────────┐   │
│   │ File: test_a.star                       │   │
│   │   └── Create file fixtures              │   │
│   │                                         │   │
│   │   ┌─────────────────────────────────┐   │   │
│   │   │ test_one                        │   │   │
│   │   │   └── Create test fixtures      │   │   │
│   │   │   └── Run test                  │   │   │
│   │   │   └── Teardown test fixtures    │   │   │
│   │   └─────────────────────────────────┘   │   │
│   │                                         │   │
│   │   ┌─────────────────────────────────┐   │   │
│   │   │ test_two                        │   │   │
│   │   │   └── Create test fixtures      │   │   │
│   │   │   └── Run test                  │   │   │
│   │   │   └── Teardown test fixtures    │   │   │
│   │   └─────────────────────────────────┘   │   │
│   │                                         │   │
│   │   └── Teardown file fixtures            │   │
│   └─────────────────────────────────────────┘   │
│                                                 │
│   ┌─────────────────────────────────────────┐   │
│   │ File: test_b.star                       │   │
│   │   └── (same pattern)                    │   │
│   └─────────────────────────────────────────┘   │
│                                                 │
│   └── Teardown session fixtures                 │
└─────────────────────────────────────────────────┘
```

### Syntax Options for Declaring Scope

#### Option A: Function attribute (recommended)

```starlark
def fixture_db():
    return connect()

fixture_db.scope = "session"
```

**Pros:** Simple, Starlark-native
**Cons:** Attribute assignment after def

#### Option B: Return wrapper

```starlark
def fixture_db():
    return fixture(connect(), scope="session")
```

**Pros:** Explicit in return
**Cons:** Requires wrapper type

#### Option C: Naming convention

```starlark
def fixture_session_db():  # "session_" prefix = session scope
    return connect()

def fixture_file_cache():  # "file_" prefix = file scope
    return {}
```

**Pros:** Discoverable by name
**Cons:** Verbose, can't change scope without rename

#### Option D: Metadata dict

```starlark
def fixture_db():
    return connect()

__fixture_config__ = {
    "fixture_db": {"scope": "session"},
}
```

**Pros:** Consistent with `__test_params__` pattern
**Cons:** Separated from definition

### Recommendation

**Use Option A (function attribute)** with **Option D as fallback** for conftest-defined fixtures:

```starlark
# Direct definition
def fixture_db():
    return connect()
fixture_db.scope = "session"

# Or via metadata (useful when fixture is in conftest)
__fixture_config__ = {
    "fixture_db": {"scope": "session", "autouse": True},
}
```

---

## Summary Table

| Question       | Recommendation                                                               |
| -------------- | ---------------------------------------------------------------------------- |
| Decorators     | Hybrid: `@mark.X` for simple, `__test_meta__` for complex, tables for params |
| Module mocking | Fixture injection (explicit DI) + prelude shadowing for convenience          |
| Fixture scopes | Four scopes: test (default), file, session; attribute syntax                 |

---

## Next Steps

1. **Prototype prelude system** - Enables all mocking approaches
2. **Implement `@mark.X`** - Simple decorators are high value, low effort
3. **Add `__test_params__`** - Table-driven is idiomatic and unblocks parametrization
4. **Design fixture DI** - Core for advanced testing patterns

These build on each other—prelude enables mocks, params enable fixtures, fixtures enable scopes.
