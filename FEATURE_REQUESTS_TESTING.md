# Sky Test: Feature Requests for World-Class Starlark Testing

> Goal: Make `sky test` as mature and developer-friendly as pytest, while respecting Starlark's deterministic, sandboxed nature.

## Executive Summary

Starlark is becoming the lingua franca for configuration (Bazel, Buck2, Pants, Tilt, etc.). As projects grow, so does the need for robust testing. This document proposes features to make `sky test` the pytest of the Starlark world.

---

## 1. Prelude & Globals System

### Problem

Tests often need shared definitions (mock functions, helpers, domain-specific builtins). Currently, each test file must be self-contained or use `load()` with careful path management.

### Proposal

```bash
# Load prelude before every test file
sky test --prelude ./test_helpers.star tests/

# Multiple preludes (loaded in order)
sky test --prelude ./mocks.star --prelude ./fixtures.star tests/

# Project-level config
# sky.toml or .sky/config.star
[test]
prelude = ["test/prelude.star", "test/mocks.star"]
```

### Use Cases

- Mocking external builtins (like bazelbump's `ensure_load`, `recipe`, etc.)
- Shared test utilities across a project
- Domain-specific assertion helpers

---

## 2. Fixtures (pytest-style)

### Problem

No built-in way to share setup/teardown logic or inject test dependencies.

### Proposal

```starlark
# conftest.star (auto-discovered)
def fixture_temp_dict():
    """Provides a fresh dict for each test."""
    return {}

def fixture_sample_config(temp_dict):
    """Fixtures can depend on other fixtures."""
    temp_dict["version"] = "1.0.0"
    return temp_dict

# test_example.star
def test_with_fixture(sample_config):
    # sample_config is automatically injected
    assert.eq(sample_config["version"], "1.0.0")
```

### Features

- **Auto-discovery**: `conftest.star` files discovered up the directory tree
- **Dependency injection**: Fixtures can depend on other fixtures
- **Scopes**: `fixture(scope="file")`, `fixture(scope="session")`
- **Lazy evaluation**: Fixtures only created when needed
- **Teardown**: `yield` pattern or explicit teardown function

```starlark
def fixture_with_teardown():
    resource = create_resource()
    yield resource
    cleanup_resource(resource)  # Runs after test
```

---

## 3. Parametrized Tests

### Problem

Repetitive tests that only differ in inputs/outputs.

### Proposal

```starlark
# Option A: Decorator-style
@parametrize([
    ("1.0.0", "2.0.0", -1),
    ("2.0.0", "1.0.0", 1),
    ("1.0.0", "1.0.0", 0),
])
def test_version_compare(v1, v2, expected):
    assert.eq(version_compare(v1, v2), expected)

# Option B: Table-driven (more Starlark-idiomatic)
VERSION_COMPARE_CASES = [
    {"v1": "1.0.0", "v2": "2.0.0", "expected": -1, "name": "less_than"},
    {"v1": "2.0.0", "v2": "1.0.0", "expected": 1, "name": "greater_than"},
    {"v1": "1.0.0", "v2": "1.0.0", "expected": 0, "name": "equal"},
]

def test_version_compare(case):
    assert.eq(version_compare(case["v1"], case["v2"]), case["expected"])

# Register as parametrized
__test_params__ = {
    "test_version_compare": VERSION_COMPARE_CASES,
}
```

### Output

```
test_version_compare[less_than]     PASS
test_version_compare[greater_than]  PASS
test_version_compare[equal]         PASS
```

---

## 4. Test Markers & Filtering

### Problem

Can't categorize tests or selectively run subsets.

### Proposal

```starlark
# Mark tests
@mark.slow
def test_large_codebase():
    ...

@mark.integration
@mark.requires_network
def test_bcr_lookup():
    ...

@mark.skip(reason="Not implemented yet")
def test_future_feature():
    ...

@mark.xfail(reason="Known bug #123")
def test_known_issue():
    ...
```

```bash
# Run only fast tests
sky test -m "not slow"

# Run integration tests
sky test -m integration

# Run tests matching pattern
sky test -k "test_version"

# Combine filters
sky test -m "not slow" -k "parse"
```

---

## 5. Snapshot / Golden File Testing

### Problem

Comparing complex outputs (structs, formatted strings) is tedious.

### Proposal

```starlark
def test_recipe_output():
    result = generate_recipe_config()

    # First run: creates snapshot
    # Subsequent runs: compares against snapshot
    assert.snapshot(result, "recipe_config")

    # With custom serializer
    assert.snapshot(result, "recipe_config", format="json")
```

```bash
# Update snapshots when output intentionally changes
sky test --update-snapshots

# Review snapshot changes interactively
sky test --snapshot-review
```

Snapshots stored in `__snapshots__/` directory alongside tests.

---

## 6. Mocking & Stubbing

### Problem

Can't mock `load()`ed modules or replace functions during tests.

### Proposal

```starlark
def test_with_mock(mock):
    # Mock a function
    mock.patch("my_module.fetch_data", return_value={"key": "value"})

    result = process_data()
    assert.eq(result["key"], "value")

    # Verify calls
    assert.eq(mock.call_count("my_module.fetch_data"), 1)
    assert.eq(mock.call_args("my_module.fetch_data")[0], ("arg1",))

def test_with_spy(mock):
    # Spy: call real function but track calls
    mock.spy("my_module.transform")

    result = my_module.transform("input")

    assert.true(mock.was_called("my_module.transform"))
    # Real function was called, result is real
```

### Module-level mocking

```starlark
# conftest.star
def fixture_mock_fs():
    """Mock filesystem module for all tests."""
    return mock.module("fs", {
        "read": lambda path: "mocked content",
        "exists": lambda path: True,
        "glob": lambda pattern: ["file1.star", "file2.star"],
    })
```

---

## 7. Subtests & Soft Assertions

### Problem

First failure stops the test; can't see all failures at once.

### Proposal

```starlark
def test_multiple_validations():
    config = load_config()

    # Soft assertions: collect all failures
    with assert.soft() as soft:
        soft.eq(config.name, "expected_name")
        soft.true(config.enabled)
        soft.contains(config.tags, "production")
    # Reports all failures, not just first

def test_subtests():
    cases = [("a", 1), ("b", 2), ("c", 3)]

    for name, value in cases:
        with subtest(name):
            # Each subtest reported separately
            assert.eq(process(name), value)
```

### Output

```
test_multiple_validations    FAIL
  - assert.eq failed: config.name = "wrong_name", expected "expected_name"
  - assert.true failed: config.enabled = False

test_subtests[a]    PASS
test_subtests[b]    FAIL
test_subtests[c]    PASS
```

---

## 8. Property-Based Testing (Fuzzing Lite)

### Problem

Hard to think of all edge cases manually.

### Proposal

```starlark
load("hypothesis", "given", "strategies")

@given(strategies.text())
def test_roundtrip_any_string(s):
    encoded = encode(s)
    decoded = decode(encoded)
    assert.eq(decoded, s)

@given(
    strategies.integers(min=0, max=100),
    strategies.lists(strategies.text(), max_size=10),
)
def test_with_multiple_generated_args(count, items):
    result = process(count, items)
    assert.true(len(result) <= count)
```

### Built-in Strategies

```starlark
strategies.text()           # Random strings
strategies.integers()       # Random integers
strategies.floats()         # Random floats
strategies.booleans()       # True/False
strategies.lists(element)   # Lists of elements
strategies.dicts(keys, vals) # Dictionaries
strategies.one_of(a, b, c)  # Choice from options
strategies.just(value)      # Constant value

# Domain-specific
strategies.semver()         # Valid semver strings
strategies.glob_pattern()   # Valid glob patterns
strategies.starlark_identifier()  # Valid identifiers
```

---

## 9. Better Assertion Introspection

### Problem

Assertion failures show minimal context.

### Current

```
assert.eq failed: got "abc", expected "abd"
```

### Proposed

```
assert.eq failed at test_example.star:42

  Left:  "abc"
  Right: "abd"
         ~~~^

  Diff:
    - "abd"
    + "abc"

  Context:
    40│     result = transform(input)
    41│     expected = "abd"
  > 42│     assert.eq(result, expected)
    43│
```

### Rich Comparisons

```starlark
# List diff
assert.eq([1, 2, 3, 4], [1, 2, 5, 4])
# Shows:
#   Index 2: got 3, expected 5

# Dict diff
assert.eq({"a": 1, "b": 2}, {"a": 1, "b": 3, "c": 4})
# Shows:
#   Key "b": got 2, expected 3
#   Key "c": missing in actual

# Struct diff
assert.eq(struct(a=1, b=2), struct(a=1, b=3))
# Shows field-by-field comparison
```

---

## 10. Test Dependencies & Ordering

### Problem

Some tests logically depend on others; running them in wrong order wastes time.

### Proposal

```starlark
def test_parse_config():
    """Must pass before other tests make sense."""
    ...

@depends_on("test_parse_config")
def test_validate_config():
    """Only runs if test_parse_config passes."""
    ...

@depends_on("test_parse_config", "test_validate_config")
def test_apply_config():
    ...
```

```bash
# Visualize dependency graph
sky test --show-deps

# Stop at first failure in dependency chain
sky test --fail-fast-deps
```

---

## 11. Watch Mode & Incremental Testing

### Problem

Slow feedback loop during development.

### Proposal

```bash
# Watch for changes, re-run affected tests
sky test --watch

# Watch specific directories
sky test --watch --watch-dir=src --watch-dir=tests

# Only re-run failed tests until they pass
sky test --watch --failed-first

# Smart detection: only run tests affected by changed files
sky test --watch --affected-only
```

### Affected Test Detection

- Track which files each test loads
- When a file changes, run tests that depend on it
- Cache test results, invalidate on dependency change

---

## 12. Parallel Execution

### Problem

Large test suites are slow.

### Proposal

```bash
# Run tests in parallel (auto-detect CPU count)
sky test -j auto

# Specific parallelism
sky test -j 4

# Parallel by file (safer)
sky test -j auto --parallel-mode=file

# Parallel by test (faster, needs isolation)
sky test -j auto --parallel-mode=test
```

### Isolation Modes

```starlark
# conftest.star
__test_config__ = {
    "parallel_safe": True,  # Tests in this file can run in parallel
    "isolation": "file",    # Each file gets fresh global state
}
```

---

## 13. Benchmarking

### Problem

No built-in way to measure performance.

### Proposal

```starlark
def bench_parse_large_file(b):
    """Benchmark parsing performance."""
    content = generate_large_content()

    b.reset_timer()  # Don't count setup
    for _ in range(b.n):
        parse(content)

def bench_with_setup(b):
    # Setup (not timed)
    data = prepare_data()

    b.run(lambda: process(data))
```

```bash
sky test --bench
sky test --bench --bench-time=5s
sky test --bench --bench-compare=baseline.json
```

### Output

```
bench_parse_large_file    1000 iterations    1.23ms/op    512 allocs/op
bench_with_setup          5000 iterations    0.45ms/op    128 allocs/op

Comparison with baseline:
  bench_parse_large_file: +15% slower (was 1.07ms/op)
  bench_with_setup: -5% faster (was 0.47ms/op)
```

---

## 14. Coverage Improvements

### Current State

Coverage is experimental and basic.

### Proposed Enhancements

```bash
# Branch coverage (not just line)
sky test --coverage --branch

# Coverage thresholds (fail if below)
sky test --coverage --cov-fail-under=80

# Coverage report formats
sky test --coverage --cov-report=html
sky test --coverage --cov-report=xml  # For CI
sky test --coverage --cov-report=json

# Diff coverage (only check changed lines)
sky test --coverage --cov-diff=main

# Exclude patterns
sky test --coverage --cov-exclude="*_test.star" --cov-exclude="testdata/*"
```

### Coverage Comments

```starlark
def complex_function():
    if rare_condition:  # pragma: no cover
        handle_rare_case()

    # pragma: no branch
    if always_true_in_tests:
        normal_path()
```

---

## 15. Configuration File

### Problem

Repeating CLI flags is tedious.

### Proposal: `sky.toml` or `.sky/config.star`

```toml
# sky.toml
[test]
prelude = ["test/prelude.star"]
parallel = "auto"
timeout = "30s"
markers = ["slow", "integration", "unit"]

[test.coverage]
enabled = true
fail_under = 80
exclude = ["*_test.star", "testdata/*"]

[test.discovery]
pattern = ["*_test.star", "test_*.star"]
ignore = ["vendor/*", ".git/*"]
```

Or Starlark-native:

```starlark
# .sky/config.star
test_config(
    prelude = ["test/prelude.star"],
    parallel = "auto",
    coverage = coverage_config(
        enabled = True,
        fail_under = 80,
    ),
)
```

---

## 16. Custom Reporters & Plugins

### Problem

Different CI systems need different output formats.

### Proposal

```bash
# Built-in reporters
sky test --reporter=dots      # Minimal
sky test --reporter=verbose   # Detailed
sky test --reporter=tap       # Test Anything Protocol
sky test --reporter=teamcity  # TeamCity format

# Custom reporter
sky test --reporter=./my_reporter.star
```

### Plugin API

```starlark
# my_reporter.star
def on_test_start(test):
    print("Starting:", test.name)

def on_test_end(test, result):
    emoji = "✓" if result.passed else "✗"
    print(emoji, test.name, result.duration)

def on_suite_end(results):
    print("Total:", results.total, "Passed:", results.passed)

reporter(
    on_test_start = on_test_start,
    on_test_end = on_test_end,
    on_suite_end = on_suite_end,
)
```

---

## 17. Test Data Management

### Problem

Managing test fixtures and data files is ad-hoc.

### Proposal

```starlark
def test_with_data_file(testdata):
    # testdata fixture provides access to test data
    content = testdata.read("input.json")
    expected = testdata.read("expected.json")

    result = process(content)
    assert.eq(result, expected)

def test_with_temp_dir(tmp_dir):
    # tmp_dir is automatically cleaned up
    tmp_dir.write("config.star", "name = 'test'")

    result = load_from_dir(tmp_dir.path)
    assert.eq(result.name, "test")
```

### Directory Structure

```
tests/
  test_parser.star
  testdata/
    test_parser/
      input.json
      expected.json
```

---

## 18. Debugging Support

### Problem

Hard to debug failing tests.

### Proposal

```bash
# Drop into REPL on failure
sky test --pdb

# Print all variable values on failure
sky test --show-locals

# Verbose tracing
sky test --trace

# Run single test with maximum verbosity
sky test --debug test_file.star::test_specific_function
```

### In-test debugging

```starlark
def test_complex_logic():
    data = prepare()

    debug.print(data)  # Only prints with --debug flag
    debug.breakpoint()  # Pauses execution with --pdb

    result = process(data)
    assert.true(result.valid)
```

---

## 19. Async/Concurrent Test Support

### Problem

Testing concurrent or async-like patterns is difficult.

### Proposal

```starlark
def test_concurrent_operations(executor):
    """Test that operations can run concurrently."""
    results = executor.map(process, [1, 2, 3, 4, 5])
    assert.eq(len(results), 5)

def test_timeout():
    """Test that slow operations timeout."""
    with assert.timeout(seconds=1):
        potentially_slow_operation()

def test_race_condition(stress):
    """Run test many times to catch race conditions."""
    stress.iterations = 1000

    counter = make_counter()
    stress.parallel(lambda: counter.increment())

    assert.eq(counter.value, 1000)
```

---

## 20. IDE Integration

### Problem

No IDE support for running/debugging tests.

### Proposal

- **LSP extension**: Report test locations, run individual tests
- **VS Code extension**: Test explorer, inline run/debug
- **Test lens**: Click to run test above function

```json
// .vscode/settings.json
{
  "sky.test.autoDiscover": true,
  "sky.test.runOnSave": true,
  "sky.test.showInlineResults": true
}
```

---

## Summary: Priority Matrix

| Feature             | Impact | Effort | Priority |
| ------------------- | ------ | ------ | -------- |
| Prelude/Globals     | High   | Low    | P0       |
| Better Assertions   | High   | Low    | P0       |
| Fixtures            | High   | Medium | P1       |
| Parametrized Tests  | High   | Medium | P1       |
| Markers & Filtering | Medium | Low    | P1       |
| Mocking             | High   | High   | P1       |
| Snapshot Testing    | Medium | Medium | P2       |
| Parallel Execution  | High   | High   | P2       |
| Watch Mode          | Medium | Medium | P2       |
| Config File         | Medium | Low    | P2       |
| Subtests            | Medium | Low    | P2       |
| Property Testing    | Medium | High   | P3       |
| Benchmarking        | Low    | Medium | P3       |
| Plugins             | Medium | High   | P3       |

---

## Appendix: Comparison with pytest

| Feature          | pytest            | sky test (current) | sky test (proposed) |
| ---------------- | ----------------- | ------------------ | ------------------- |
| Fixtures         | ✓                 | ✗                  | ✓                   |
| Parametrize      | ✓                 | ✗                  | ✓                   |
| Markers          | ✓                 | ✗                  | ✓                   |
| Parallel         | ✓ (plugin)        | ✗                  | ✓                   |
| Coverage         | ✓ (plugin)        | Experimental       | ✓                   |
| Mocking          | ✓ (unittest.mock) | ✗                  | ✓                   |
| Snapshots        | ✓ (plugin)        | ✗                  | ✓                   |
| Watch            | ✓ (plugin)        | ✗                  | ✓                   |
| Config file      | ✓ (pytest.ini)    | ✗                  | ✓                   |
| Plugins          | ✓                 | ✗                  | ✓                   |
| Property testing | ✓ (hypothesis)    | ✗                  | ✓                   |
| Benchmarks       | ✓ (plugin)        | ✗                  | ✓                   |

---

## Call to Action

The Starlark ecosystem deserves first-class testing tools. With these features, `sky test` could become the standard for testing Starlark code across all platforms that use it.

Let's make Starlark testing as delightful as Python testing.

---

_Feature requests from the bazelbump project team_
_Date: 2025-02-04_
