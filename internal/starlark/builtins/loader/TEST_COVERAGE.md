# Proto Loader Test Coverage

This document outlines the test coverage for the proto loader implementation.

## Test Overview

The test suite (`proto_loader_test.go`) provides comprehensive coverage for all aspects of the proto loader functionality, including unit tests, integration tests, and benchmarks.

## Unit Tests

### Initialization Tests

#### `TestNewProtoProvider`

- Verifies that `NewProtoProvider()` correctly initializes the provider
- Checks cache and dataFS initialization
- Ensures no nil returns

#### `TestSupportedDialects`

- Verifies the complete list of supported dialects
- Expected dialects: `bazel`, `buck2`, `starlark`
- Ensures no duplicates or missing dialects

### Filename Mapping Tests

#### `TestProtoFilename`

- Tests all supported dialect and file kind combinations
- Verifies case-insensitive dialect handling
- Tests unsupported combinations return empty strings

**Covered combinations:**

- Bazel: BUILD, bzl, WORKSPACE, MODULE, bzlmod
- Buck2: BUCK, bzl (Buck variant), buckconfig
- Starlark: generic starlark, skyi

### Proto Parsing Tests

#### `TestParseProtoFile`

- Tests binary proto format parsing
- Tests text proto format parsing
- Tests invalid binary proto error handling
- Tests invalid text proto error handling
- Verifies proto equality after parsing

### Conversion Tests

#### `TestConvertCallableToSignature`

Table-driven tests covering:

- Simple functions with required parameters
- Functions with optional parameters and defaults
- Variadic functions (*args)
- Keyword argument functions (**kwargs)
- Mixed parameter types

#### `TestConvertProtoToBuiltins`

Table-driven tests covering:

- Empty builtins
- Types with fields only
- Types with methods
- Global functions
- Global constants
- Complete builtins (types + functions + globals)

Verifies:

- Correct separation of fields vs methods
- Proper handling of callable vs non-callable values
- Accurate parameter conversion
- Documentation preservation

### Data Loading Tests

#### `TestLoadProtoData`

- Tests loading existing files
- Tests fallback from `.pb` to `.pbtxt`
- Tests missing file error handling

### Interface Tests

#### `TestBuiltins_Interface`

- Tests successful proto loading via main interface
- Tests unsupported dialect errors
- Tests unsupported file kind errors
- Tests missing proto file errors

### Caching Tests

#### `TestBuiltins_Caching`

- Verifies cache population on first load
- Verifies cache usage on subsequent loads
- Tests cache key structure (dialect + file kind)
- Ensures data integrity through cache

#### `TestBuiltins_ConcurrentAccess`

- Tests thread-safe cache access
- Runs 10 concurrent goroutines
- Verifies no race conditions or errors

#### `TestBuiltins_AllDialectsAndKinds`

- Systematically tests all valid dialect/kind combinations
- Ensures no nil slices in results
- Verifies consistent behavior across all combinations

## Integration Tests

#### `TestIntegration_ComprehensiveProto`

End-to-end test with realistic proto data:

- Multiple types with fields and methods
- Multiple functions with various parameter patterns
- Global constants of different types
- Validates complete conversion pipeline
- Tests proper categorization of:
  - Type fields vs methods
  - Functions vs globals
  - Variadic parameters
  - Required vs optional parameters

## Benchmark Tests

### Performance Benchmarks

#### `BenchmarkProtoLoader_FirstLoad`

- Measures cold load performance
- Includes proto unmarshaling and conversion
- Reset and creates new provider for each iteration

#### `BenchmarkProtoLoader_CachedLoad`

- Measures cached load performance
- Warms up cache before timing
- Demonstrates cache effectiveness

#### `BenchmarkConvertProtoToBuiltins`

- Measures conversion performance in isolation
- Uses moderately complex proto with:
  - 2 types
  - 3-4 fields/methods per type
  - 3 values (function + constants)

## Test Fixtures

Located in `testdata/proto/`:

### `test_simple.pbtxt`

- Minimal proto for basic testing
- 1 type with fields and method
- 1 function with varied parameters
- 2 constants

### `test_empty.pbtxt`

- Empty proto for edge case testing

### `test_invalid.pbtxt`

- Invalid proto for error handling tests

### `test_comprehensive.pbtxt`

- Realistic proto resembling actual Starlark builtins
- Multiple types (File, Provider)
- Multiple functions (glob, select, print, dict_merge)
- Various parameter patterns
- Global constants

## Coverage Metrics

### Functions Tested

- ✅ `NewProtoProvider()`
- ✅ `SupportedDialects()`
- ✅ `protoFilename()`
- ✅ `parseProtoFile()`
- ✅ `convertProtoToBuiltins()`
- ✅ `convertCallableToSignature()`
- ✅ `loadProtoData()`
- ✅ `Builtins()` (main interface method)

### Test Categories

- ✅ Happy path tests
- ✅ Error handling tests
- ✅ Edge case tests (empty protos, missing files)
- ✅ Concurrency tests
- ✅ Cache behavior tests
- ✅ Integration tests
- ✅ Performance benchmarks

### Code Coverage Areas

- ✅ Initialization
- ✅ All supported dialects (bazel, buck2, starlark)
- ✅ All file kinds per dialect
- ✅ Binary proto format
- ✅ Text proto format
- ✅ Cache population
- ✅ Cache retrieval
- ✅ Error paths
- ✅ Concurrent access
- ✅ Proto to Go struct conversion
- ✅ Parameter type conversions (required, optional, variadic, kwargs)

## Running Tests

### Run all tests

```bash
go test ./internal/starlark/builtins/loader/...
```

### Run with coverage

```bash
go test -cover ./internal/starlark/builtins/loader/...
```

### Run benchmarks

```bash
go test -bench=. ./internal/starlark/builtins/loader/...
```

### Run with race detection

```bash
go test -race ./internal/starlark/builtins/loader/...
```

### Using Bazel

```bash
bazel test //internal/starlark/builtins/loader:loader_test
```

## Future Test Enhancements

Potential areas for additional testing:

- [ ] Test with actual embedded proto files (requires `//go:embed`)
- [ ] Fuzz testing for proto parsing
- [ ] Memory usage benchmarks
- [ ] Large proto file stress tests
- [ ] Additional concurrent access patterns
- [ ] Cache eviction strategies (if implemented)
