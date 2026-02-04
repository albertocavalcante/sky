# RFC-001: Sky Test Framework Evolution

| Field   | Value                                                             |
| ------- | ----------------------------------------------------------------- |
| Status  | Draft (Rev 2)                                                     |
| Created | 2026-02-04                                                        |
| Revised | 2026-02-04                                                        |
| Authors | Sky Team                                                          |
| Related | FEATURE_REQUESTS_TESTING.md, RFC-001-open-questions-discussion.md |

## Abstract

This RFC proposes a phased evolution of `sky test` to become a world-class testing framework for Starlark, drawing inspiration from the best features of pytest (Python), Go's testing package, Jest (JavaScript), Cargo test (Rust), and other mature ecosystems.

## Motivation

Starlark is becoming the lingua franca for build configuration (Bazel, Buck2, Pants, Tilt). As codebases grow, robust testing becomes critical. Current `sky test` is minimal—it discovers `test_*` functions and runs them with basic assertions.

To achieve adoption, we must match or exceed the developer experience of established testing frameworks.

## Starlark Constraints

**Critical:** Starlark is not Python. These limitations fundamentally shape our design:

| Constraint                | Impact                                                           | Workaround                                                               |
| ------------------------- | ---------------------------------------------------------------- | ------------------------------------------------------------------------ |
| **No `with` statement**   | Can't use context managers for soft assertions, resource cleanup | Builder pattern: `check = assert.checker(); check.done()`                |
| **No `yield`/generators** | Can't implement lazy test case generation                        | All test data must be upfront; property testing needs Go-side generation |
| **No exceptions**         | Only `fail()` which terminates; can't catch errors in Starlark   | `assert.fails(fn)` catches at Go level                                   |
| **No `defer`**            | Can't register cleanup functions Python/Go style                 | `testing.add_cleanup(fn)` registration pattern                           |
| **Frozen globals**        | After `load()`, module globals are immutable                     | Tests in same file share frozen globals—isolation via fresh threads      |
| **Parse-time `load()`**   | Can't mock modules at runtime—already resolved                   | Fixture injection pattern; prelude shadowing                             |
| **No `@decorator(args)`** | Only `@expr` where `expr` is a callable                          | Hybrid: `@mark.slow` for simple; `__test_meta__` dict for complex        |
| **No `__getattr__`**      | Can't implement dynamic attribute access                         | Explicit struct/dict patterns                                            |

### What This Means

1. **Soft assertions** require builder pattern, not `with` blocks
2. **Module mocking** uses fixture injection, not runtime patching
3. **Parametrization** uses `__test_params__` dict, not `@parametrize([...])`
4. **Cleanup** uses `testing.add_cleanup(fn)`, not `defer`
5. **Property testing** requires Go-side generation, limiting expressiveness

## Ecosystem Benchmark

### Feature Matrix Comparison

| Feature                  | pytest          | Go test       | Jest                | Cargo test       | sky test (now) | sky test (goal) |
| ------------------------ | --------------- | ------------- | ------------------- | ---------------- | -------------- | --------------- |
| **Discovery**            |                 |               |                     |                  |                |                 |
| Auto-discovery           | ✓               | ✓             | ✓                   | ✓                | ✓              | ✓               |
| Pattern filtering (`-k`) | ✓               | `-run`        | `--testNamePattern` | `--test`         | ✗              | ✓               |
| File filtering           | ✓               | `./...`       | `--testPathPattern` | `--test`         | ✓              | ✓               |
| **Setup/Teardown**       |                 |               |                     |                  |                |                 |
| Per-test setup           | ✓               | `t.Cleanup()` | `beforeEach`        | `#[fixture]`     | `setup()`      | ✓               |
| Per-file setup           | ✓               | `TestMain`    | `beforeAll`         | module init      | ✗              | ✓               |
| Fixtures/DI              | ✓               | ✗             | ✗                   | ✗                | ✗              | ✓               |
| Scoped fixtures          | ✓               | ✗             | ✗                   | ✗                | ✗              | ✓               |
| **Assertions**           |                 |               |                     |                  |                |                 |
| Rich diffs               | ✓               | `go-cmp`      | ✓                   | ✓                | basic          | ✓               |
| Snapshot testing         | plugin          | ✗             | ✓                   | `insta`          | ✗              | ✓               |
| Soft assertions          | ✓               | ✗             | ✗                   | ✗                | ✗              | ✓               |
| **Parametrization**      |                 |               |                     |                  |                |                 |
| Table-driven             | `@parametrize`  | native        | `each`              | `#[case]`        | ✗              | ✓               |
| Generated params         | hypothesis      | ✗             | ✗                   | proptest         | ✗              | P3              |
| **Organization**         |                 |               |                     |                  |                |                 |
| Markers/tags             | ✓               | `//go:build`  | ✗                   | `#[ignore]`      | ✗              | ✓               |
| Skip/xfail               | ✓               | `t.Skip()`    | `skip`/`todo`       | `#[ignore]`      | ✗              | ✓               |
| Subtests                 | ✓               | `t.Run()`     | `describe/it`       | nested           | ✗              | ✓               |
| **Execution**            |                 |               |                     |                  |                |                 |
| Parallel                 | plugin          | `-parallel`   | `--runInBand`       | default          | ✗              | ✓               |
| Watch mode               | plugin          | ✗             | `--watch`           | `cargo-watch`    | ✗              | ✓               |
| Fail-fast                | ✓               | `-failfast`   | `--bail`            | `--no-fail-fast` | ✗              | ✓               |
| **Mocking**              |                 |               |                     |                  |                |                 |
| Function mock            | `unittest.mock` | interfaces    | `jest.fn()`         | `mockall`        | ✗              | ✓               |
| Module mock              | ✓               | ✗             | `jest.mock()`       | ✗                | ✗              | Limited¹        |
| **Reporting**            |                 |               |                     |                  |                |                 |
| Coverage                 | plugin          | `-cover`      | `--coverage`        | `--coverage`     | experimental   | ✓               |
| JUnit XML                | plugin          | `go-junit`    | `--reporters`       | ✗                | ✗              | ✓               |
| TAP                      | plugin          | ✗             | ✗                   | ✗                | ✗              | ✓               |
| **Performance**          |                 |               |                     |                  |                |                 |
| Benchmarks               | plugin          | `-bench`      | ✗                   | `#[bench]`       | ✗              | P3              |
| Profiling                | plugin          | `-cpuprofile` | ✗                   | ✗                | ✗              | P3              |
| **Config**               |                 |               |                     |                  |                |                 |
| Config file              | `pytest.ini`    | ✗             | `jest.config`       | `Cargo.toml`     | ✗              | ✓               |
| Prelude/helpers          | `conftest.py`   | `_test.go`    | `setupFiles`        | `mod.rs`         | ✗              | ✓               |

¹ **Module mocking is limited** in Starlark because `load()` resolves at parse time. Use fixture injection pattern instead of runtime patching. See Phase 3.1.

### Key Learnings from Each Ecosystem

#### pytest (Python)

**Strengths:**

- Fixtures with dependency injection and scopes
- `conftest.py` auto-discovery for shared setup
- Rich plugin ecosystem
- Excellent assertion introspection (rewrites AST)

**Adopt:**

- Fixture concept with `conftest.star`
- `@mark` decorators
- `-k` and `-m` filtering syntax

#### Go testing

**Strengths:**

- Zero dependencies (stdlib only)
- Table-driven tests are idiomatic
- `t.Run()` for subtests
- Built-in benchmarking and coverage
- `t.Cleanup()` for teardown

**Adopt:**

- Table-driven as the primary pattern (fits Starlark's data-oriented nature)
- `b.N` pattern for benchmarks
- Simple, no-magic approach

#### Jest (JavaScript)

**Strengths:**

- Excellent watch mode with affected-test detection
- Snapshot testing built-in
- `--runInBand` vs parallel is explicit
- Clear `describe/it/test` hierarchy

**Adopt:**

- Watch mode with file dependency tracking
- Snapshot testing with `--update-snapshots`
- Clear parallel/serial mode flags

#### Cargo test (Rust)

**Strengths:**

- Parallel by default (tests must be isolated)
- Doc tests (test code examples in docs)
- `#[ignore]` for slow tests
- Integration tests in `tests/` directory

**Adopt:**

- Parallel-by-default mindset
- Distinction between unit and integration tests
- `#[ignore]` equivalent for slow tests

## Design Principles

Based on the ecosystem analysis, `sky test` should follow these principles:

### 1. **Starlark-Native Idioms**

Starlark is data-oriented. Prefer:

```starlark
# Good: Data-driven (Starlark-idiomatic)
VERSION_CASES = [
    {"v1": "1.0", "v2": "2.0", "cmp": -1},
    {"v1": "2.0", "v2": "1.0", "cmp": 1},
]

def test_version_compare(case):
    assert.eq(compare(case["v1"], case["v2"]), case["cmp"])

# Less good: Decorator magic (Python-idiomatic)
@parametrize([...])
def test_version_compare(v1, v2, expected):
    ...
```

### 2. **Explicit Over Magic**

Go's approach of explicit table-driven tests over pytest's fixture injection magic. But offer both:

```starlark
# Explicit (preferred)
def test_with_temp_dir():
    dir = testing.temp_dir()
    testing.add_cleanup(lambda: testing.remove(dir))
    ...

# Implicit (available for convenience)
def test_with_temp_dir(tmp_dir):  # injected
    ...
```

### 3. **Deterministic by Default**

Starlark's sandboxing is a feature. Tests should be:

- Hermetic (no filesystem, no network by default)
- Reproducible (same inputs → same outputs)
- Parallelizable (no shared mutable state)

### 4. **Progressive Disclosure**

Simple things should be simple:

```starlark
def test_basic():
    assert.eq(1 + 1, 2)
```

Complex things should be possible:

```starlark
@mark.slow
@parametrize(LARGE_DATASET)
def test_complex(case, mock, snapshot):
    mock.patch("module.fetch", return_value=case["data"])
    result = process()
    assert.snapshot(result, "expected_output")
```

## Proposed Implementation Phases

### Phase 0: Foundation (Current)

- [x] Test discovery (`test_*` functions)
- [x] Basic assertions (`assert.eq`, `assert.true`, etc.)
- [x] `setup()` function
- [x] Exit codes (pass/fail)
- [x] Verbose mode (`-v`)

### Phase 1: Essential Improvements

**Timeline:** 4-5 weeks (revised from 2-3)

#### 1.1 Better Assertions

```starlark
# Rich diffs for collections
assert.eq([1, 2, 3], [1, 2, 4])
# Output:
#   Lists differ at index 2:
#     - 3
#     + 4

# Context in failure messages
assert.eq(result, expected, msg="processing failed for input: %s" % input)

# Type-aware comparisons
assert.eq(struct(a=1), struct(a=1))  # Deep equality
```

**Implementation:**

- Enhance `checker/diagnostic.go` for rich diff formatting
- Add `assert.contains`, `assert.matches`, `assert.raises`
- Show source context on failure (file:line + surrounding code)

#### 1.2 Prelude System

```bash
sky test --prelude=./test/helpers.star tests/
```

**Implementation:**

- Load prelude files before each test file
- Prelude globals become available in test scope
- Multiple `--prelude` flags loaded in order

#### 1.3 Test Filtering

```bash
sky test -k "parse"           # Name contains "parse"
sky test -k "not slow"        # Exclude "slow" in name
sky test tests/unit/          # Directory
sky test test_parser.star::test_specific  # Specific test
```

**Implementation:**

- Add `-k` flag for name filtering (glob or regex)
- Support `::` syntax for test-level granularity
- Track which tests matched vs skipped in output

#### 1.4 Test Timeouts

```bash
sky test --timeout=30s         # Global timeout per test
sky test --timeout=0           # Disable timeouts
```

```starlark
# Per-test timeout via metadata
__test_meta__ = {
    "test_slow_operation": {"timeout": 60},  # seconds
}
```

**Implementation:**

- Default timeout: 30 seconds per test
- Context cancellation in Go test runner
- Clear error message on timeout

#### 1.5 Fail-Fast Mode

```bash
sky test --bail               # Stop on first failure
sky test --fail-fast          # Alias for --bail
sky test -x                   # Short form
```

**Implementation:**

- Early exit when any test fails
- Report partial results
- Essential for large test suites in CI

### Phase 2: Fixtures & Parametrization

**Timeline:** 6-8 weeks (revised from 3-4)

#### 2.1 Fixtures

```starlark
# conftest.star (auto-discovered)
def fixture_sample_data():
    return {"users": [], "config": {}}

# Fixtures can depend on other fixtures
def fixture_user_service(sample_data, mock):
    return UserService(data=sample_data, http=mock.wrap(http_client))

# test_example.star
def test_with_fixture(sample_data):
    assert.eq(sample_data["users"], [])
```

**Fixture Scopes:**

| Scope              | Lifetime                 | Use Case                        |
| ------------------ | ------------------------ | ------------------------------- |
| `"test"` (default) | Fresh per test function  | Mutable state, test isolation   |
| `"file"`           | Shared within test file  | Expensive setup safe to share   |
| `"session"`        | Shared across entire run | DB connections, service clients |

```starlark
# Set scope via function attribute
def fixture_db_connection():
    return connect_to_db()

fixture_db_connection.scope = "session"

# Or via metadata dict
__fixture_config__ = {
    "fixture_db_connection": {"scope": "session"},
}
```

**Scope Rules:**

- Fixtures can only depend on equal or wider scopes
- Session-scoped fixtures disable parallel execution for dependent tests
- Scope hierarchy: `session > file > test`

**Implementation:**

- Discover `conftest.star` up the directory tree
- Detect fixture dependencies via function parameter names
- Topological sort for dependency resolution
- Scope validation at test collection time

#### 2.2 Table-Driven Tests

```starlark
CASES = [
    {"name": "empty", "input": "", "want": []},
    {"name": "single", "input": "a", "want": ["a"]},
]

def test_split(case):
    assert.eq(split(case["input"]), case["want"])

__test_params__ = {"test_split": CASES}
```

**Implementation:**

- Detect `__test_params__` dict in module
- Generate virtual test for each case
- Report as `test_split[empty]`, `test_split[single]`

#### 2.3 Markers (Hybrid Approach)

**Note:** Starlark supports `@decorator` but **not** `@decorator(args)`. Use hybrid pattern:

**Simple markers:** Zero-arg decorators work

```starlark
load("testing", "mark")

@mark.slow
@mark.integration
def test_large_file():
    ...

@mark.skip  # Simple skip (no reason)
def test_wip():
    ...
```

**Complex markers:** Use `__test_meta__` dict for markers with arguments

```starlark
def test_known_bug():
    ...

def test_platform_specific():
    ...

__test_meta__ = {
    "test_known_bug": {"xfail": "Bug #123 not fixed yet"},
    "test_platform_specific": {"skip_if": "platform != 'linux'"},
}
```

```bash
sky test -m "not slow"
sky test -m "integration and not flaky"
```

**Implementation:**

- `mark` module provides zero-arg decorator functions
- Decorators attach `_markers` list to function
- `__test_meta__` dict for complex marker configuration
- Build marker expression evaluator for `-m` filtering
- Track expected failures separately in results

### Phase 3: Advanced Features

**Timeline:** 10-14 weeks (revised from 4-6)

#### 3.1 Mocking (Fixture Injection Pattern)

**Note:** Module-level mocking (`mock.patch("module.fn")`) is **not possible** in Starlark because `load()` resolves at parse time. Instead, use fixture injection:

```starlark
# conftest.star
load("mymodule", _real_fetch = "fetch")

def fixture_fetch(mock):
    """Injectable fetch function that can be mocked."""
    return mock.wrap(_real_fetch)

# test_example.star
def test_with_mock(fetch, mock):
    # Configure the mock
    mock.when(fetch).called_with("url").then_return({"data": 123})

    # Call code that uses the injected fetch
    result = process_with_fetch(fetch)

    assert.eq(result["data"], 123)
    assert.true(mock.was_called(fetch))
```

**Implementation:**

- Inject `mock` fixture when parameter present
- `mock.wrap(fn)` returns a trackable wrapper
- `mock.when(fn).then_return(value)` configures return values
- Track calls and arguments for verification
- **Limitation:** Code under test must accept injected dependencies

**Alternative: Prelude shadowing** for simpler cases:

```starlark
# test_prelude.star (via --prelude)
_mocks = {}

def mock_fn(name, impl):
    _mocks[name] = impl

def fetch(url):  # Shadows any 'fetch' loaded later
    if "fetch" in _mocks:
        return _mocks["fetch"](url)
    fail("fetch called but not mocked")
```

#### 3.2 Snapshot Testing

```starlark
def test_output(snapshot):
    result = generate_report()
    assert.snapshot(result, "report_output")
```

**Implementation:**

- Store snapshots in `__snapshots__/test_name.snap`
- On first run: create snapshot
- On subsequent runs: compare and fail if different
- `--update-snapshots` to accept changes

#### 3.3 Parallel Execution

```bash
sky test -j auto              # Detect CPU count
sky test -j 4                 # Explicit parallelism
sky test -j 1                 # Sequential (debugging)
```

**Implementation:**

- Worker pool with test queue
- Capture stdout/stderr per test
- Aggregate results at end
- Default: parallel by file (safer)

#### 3.4 Watch Mode

```bash
sky test --watch
sky test --watch --affected-only
```

**Implementation:**

- File watcher on test directories
- Track `load()` dependencies
- Re-run affected tests on change
- Clear screen and show results

### Phase 4: Polish & Ecosystem

**Timeline:** Ongoing

#### 4.1 Configuration File

```toml
# sky.toml
[test]
prelude = ["test/helpers.star"]
parallel = "auto"
markers = ["slow", "integration"]

[test.coverage]
enabled = true
fail_under = 80
```

#### 4.2 Benchmarking

```starlark
def bench_parse(b):
    data = load_fixture()
    b.reset_timer()

    for _ in range(b.n):
        parse(data)
```

```bash
sky test --bench
sky test --bench --bench-time=5s
```

#### 4.3 Property-Based Testing

```starlark
@given(strategies.text())
def test_roundtrip(s):
    assert.eq(decode(encode(s)), s)
```

#### 4.4 IDE Integration

- LSP: Report test locations
- VS Code: Test explorer integration
- Inline run/debug buttons

## Compatibility & Migration

### Breaking Changes

None planned. All new features are additive.

### Deprecations

- `setup()` function: Still supported, but fixtures preferred

### Configuration Discovery

Priority order:

1. CLI flags (highest)
2. `sky.toml` in current directory
3. `.sky/config.star` in current directory
4. Parent directories (up to git root)
5. `~/.config/sky/config.toml` (lowest)

## Alternatives Considered

### 1. Port pytest directly

**Rejected:** pytest relies heavily on Python features (AST rewriting, dynamic imports, `__getattr__`). Starlark lacks these.

### 2. Use Go's testing patterns exactly

**Partially adopted:** Table-driven tests are great, but Go's `testing.T` interface is imperative. Starlark benefits from more declarative patterns.

### 3. Build on existing Starlark test frameworks

**Considered:** `rules_testing` in Bazel has some patterns, but it's tightly coupled to Bazel's execution model.

## Open Questions (Resolved)

See [RFC-001-open-questions-discussion.md](RFC-001-open-questions-discussion.md) for full discussion.

1. **Fixture scope semantics:** ✅ RESOLVED
   - `scope="test"` (default): Fresh per test function
   - `scope="file"`: One instance shared across all tests in the file
   - `scope="session"`: One instance for entire test run
   - Hierarchy: `session > file > test`; fixtures can only depend on equal or wider scopes

2. **Decorator syntax:** ✅ RESOLVED - Hybrid approach
   - `@mark.slow`: Zero-arg decorators work for simple markers
   - `__test_meta__` dict: For markers needing arguments (skip reason, xfail condition)
   - `__test_params__` dict: For parametrization (data-driven, Starlark-idiomatic)

3. **Async tests:** ✅ RESOLVED - Non-issue
   - Starlark is synchronous and sandboxed; async patterns don't apply
   - If testing code that interacts with async Go code, the Go test harness handles it

4. **Module mocking:** ✅ RESOLVED - Fixture injection
   - Runtime module patching is **not possible** (load resolves at parse time)
   - Use fixture injection: inject mock-wrapped functions as test parameters
   - Use prelude shadowing for simpler cases
   - This limitation is a **feature**: forces explicit, testable design

## References

- [pytest Documentation](https://docs.pytest.org/)
- [Go testing Package](https://pkg.go.dev/testing)
- [Jest Documentation](https://jestjs.io/docs/getting-started)
- [Cargo test Book](https://doc.rust-lang.org/cargo/commands/cargo-test.html)
- [Starlark Specification](https://github.com/bazelbuild/starlark/blob/master/spec.md)
- [Hypothesis Documentation](https://hypothesis.readthedocs.io/)
- [insta (Rust snapshots)](https://insta.rs/)

## Appendix A: Assertion API Reference

```starlark
# Equality
assert.eq(actual, expected)
assert.neq(actual, expected)

# Boolean
assert.true(value)
assert.false(value)

# Collections
assert.contains(container, item)
assert.not_contains(container, item)
assert.len(container, expected_len)
assert.empty(container)

# Strings
assert.matches(string, pattern)  # regex
assert.starts_with(string, prefix)
assert.ends_with(string, suffix)

# Exceptions
assert.raises(callable, exception_type)
assert.raises_match(callable, pattern)

# Comparisons
assert.lt(a, b)
assert.le(a, b)
assert.gt(a, b)
assert.ge(a, b)

# Types
assert.type(value, expected_type)
assert.instance(value, type)

# Approximate
assert.almost_eq(actual, expected, delta=0.001)

# Soft assertions (collect all failures)
# NOTE: Starlark has no `with` statement, use builder pattern:
check = assert.checker()
check.eq(a, 1)
check.eq(b, 2)
check.eq(c, 3)
check.done()  # Reports all failures, not just first

# Custom message
assert.eq(a, b, msg="Custom failure message")
```

## Appendix B: Example Test File

```starlark
"""Tests for the version comparison module."""

load("version", "compare", "parse")

# Table-driven test data
VERSION_CASES = [
    {"name": "major_diff", "v1": "1.0.0", "v2": "2.0.0", "expected": -1},
    {"name": "minor_diff", "v1": "1.1.0", "v2": "1.2.0", "expected": -1},
    {"name": "patch_diff", "v1": "1.0.1", "v2": "1.0.2", "expected": -1},
    {"name": "equal", "v1": "1.0.0", "v2": "1.0.0", "expected": 0},
    {"name": "prerelease", "v1": "1.0.0-alpha", "v2": "1.0.0", "expected": -1},
]

__test_params__ = {
    "test_version_compare": VERSION_CASES,
}

def test_version_compare(case):
    """Parametrized version comparison test."""
    result = compare(case["v1"], case["v2"])
    assert.eq(result, case["expected"])

def test_parse_valid():
    """Test parsing a valid semver string."""
    v = parse("1.2.3")
    assert.eq(v.major, 1)
    assert.eq(v.minor, 2)
    assert.eq(v.patch, 3)

def test_parse_invalid():
    """Test that invalid versions raise an error."""
    assert.raises(lambda: parse("not-a-version"), "ValueError")

@mark.slow
def test_large_comparison():
    """Test comparing many versions (slow)."""
    versions = ["%d.%d.%d" % (i, j, k) for i in range(10) for j in range(10) for k in range(10)]
    for i in range(len(versions) - 1):
        assert.le(compare(versions[i], versions[i + 1]), 0)
```

## Appendix C: conftest.star Example

```starlark
"""Shared test fixtures and configuration."""

def fixture_sample_versions():
    """Provides a list of sample version strings."""
    return ["1.0.0", "2.0.0", "1.0.0-alpha", "1.0.0-beta.1"]

def fixture_version_pairs(sample_versions):
    """Provides pairs of versions for comparison tests."""
    pairs = []
    for i, v1 in enumerate(sample_versions):
        for v2 in sample_versions[i + 1:]:
            pairs.append((v1, v2))
    return pairs

def fixture_mock_registry(mock):
    """Provides a mocked version registry."""
    mock.patch("registry.fetch", return_value={"latest": "3.0.0"})
    return mock

# Test configuration
__test_config__ = {
    "parallel_safe": True,
    "timeout": 30,  # seconds
}
```
