# Proto Loader Testing Guide

This document provides a quick reference for working with the proto loader test suite.

## Quick Start

### Run all tests

```bash
cd /Users/adsc/dev/ws/sky/main
go test ./internal/starlark/builtins/loader/...
```

### Run with verbose output

```bash
go test -v ./internal/starlark/builtins/loader/...
```

### Run specific test

```bash
go test -v -run TestProtoFilename ./internal/starlark/builtins/loader/...
```

### Run benchmarks

```bash
go test -bench=. -benchmem ./internal/starlark/builtins/loader/...
```

### Run with coverage report

```bash
go test -cover -coverprofile=coverage.out ./internal/starlark/builtins/loader/...
go tool cover -html=coverage.out
```

### Run with race detector

```bash
go test -race ./internal/starlark/builtins/loader/...
```

## Test Structure

### Unit Tests (20+ test cases)

#### Initialization & Configuration

- `TestNewProtoProvider` - Provider initialization
- `TestSupportedDialects` - Dialect enumeration

#### Filename Mapping

- `TestProtoFilename` - Dialect/kind to filename mapping (25+ combinations)

#### Proto Parsing

- `TestParseProtoFile` - Binary and text proto parsing

#### Conversion Logic

- `TestConvertCallableToSignature` - Callable to signature conversion
- `TestConvertProtoToBuiltins` - Complete proto conversion (7+ scenarios)

#### Data Loading

- `TestLoadProtoData` - Embedded filesystem operations

#### Interface & Caching

- `TestBuiltins_Interface` - Main interface method
- `TestBuiltins_Caching` - Cache behavior verification
- `TestBuiltins_ConcurrentAccess` - Thread safety
- `TestBuiltins_AllDialectsAndKinds` - Systematic combination testing

### Integration Tests

#### `TestIntegration_ComprehensiveProto`

End-to-end test with realistic proto data covering:

- Type definitions with fields and methods
- Function definitions with various parameter patterns
- Global constants
- Complete conversion pipeline

### Benchmark Tests

- `BenchmarkProtoLoader_FirstLoad` - Cold load performance
- `BenchmarkProtoLoader_CachedLoad` - Cached load performance
- `BenchmarkConvertProtoToBuiltins` - Conversion performance

## Test Fixtures

Located in `testdata/proto/`:

1. **test_simple.pbtxt** - Basic proto with 1 type, 1 function, 2 constants
2. **test_empty.pbtxt** - Empty proto for edge cases
3. **test_invalid.pbtxt** - Invalid proto for error handling
4. **test_comprehensive.pbtxt** - Realistic proto with multiple types and functions

## What's Tested

### All Core Functions

✅ `NewProtoProvider()` - Initialization
✅ `SupportedDialects()` - Dialect enumeration
✅ `protoFilename()` - Filename mapping
✅ `parseProtoFile()` - Proto parsing
✅ `convertProtoToBuiltins()` - Proto conversion
✅ `convertCallableToSignature()` - Signature conversion
✅ `loadProtoData()` - Data loading
✅ `Builtins()` - Main interface method

### All Dialects

✅ Bazel (BUILD, bzl, WORKSPACE, MODULE, bzlmod)
✅ Buck2 (BUCK, bzl_buck, buckconfig)
✅ Starlark (generic, skyi)

### All Scenarios

✅ Valid proto files (binary and text)
✅ Invalid/corrupt proto files
✅ Missing proto files
✅ Empty proto files
✅ Cache population and retrieval
✅ Concurrent access
✅ All parameter types (required, optional, variadic, kwargs)
✅ Types with fields and methods
✅ Global functions and constants

## Expected Test Results

All tests should pass. Expected output:

```
ok  	github.com/albertocavalcante/sky/internal/starlark/builtins/loader	X.XXXs
```

## Common Issues

### Missing Dependencies

If you see errors about missing go.sum entries:

```bash
go mod tidy
```

### Import Path Issues

Ensure the module path in go.mod is:

```
module github.com/albertocavalcante/sky
```

### Protobuf Version Issues

The tests require:

- `google.golang.org/protobuf` (for proto parsing)
- `github.com/google/go-cmp` (for test comparisons)

Install with:

```bash
go get google.golang.org/protobuf/proto
go get google.golang.org/protobuf/encoding/prototext
go get github.com/google/go-cmp/cmp
```

## Test Coverage Goals

Target coverage: **>90%** of all code paths

To check current coverage:

```bash
go test -cover ./internal/starlark/builtins/loader/...
```

For detailed coverage:

```bash
go test -coverprofile=coverage.out ./internal/starlark/builtins/loader/...
go tool cover -func=coverage.out
```

## Adding New Tests

When extending the test suite:

1. **For new features**: Add corresponding test cases to existing test functions or create new test functions
2. **For new dialects**: Update `TestProtoFilename` and `TestBuiltins_AllDialectsAndKinds`
3. **For new file kinds**: Add to the combination tables in filename mapping tests
4. **For bug fixes**: Add regression test before fixing the bug

### Test Template

```go
func TestNewFeature(t *testing.T) {
    provider := NewProtoProvider()

    // Setup test data
    testProto := &builtinspb.Builtins{
        // ... proto definition
    }
    data, _ := proto.Marshal(testProto)
    provider.dataFS.files["test.pb"] = data

    // Execute
    result, err := provider.SomeMethod()

    // Verify
    if err != nil {
        t.Fatalf("Expected no error, got %v", err)
    }
    if diff := cmp.Diff(expected, result); diff != "" {
        t.Errorf("Mismatch (-want +got):\n%s", diff)
    }
}
```

## Continuous Integration

These tests are designed to run in CI environments. They:

- Have no external dependencies
- Use deterministic test data
- Complete quickly (<1s for unit tests)
- Are safe to run concurrently

## Performance Expectations

Benchmark baseline (on modern hardware):

- First load: ~100-500 µs per proto
- Cached load: ~50-100 ns per proto (>1000x faster)
- Conversion: ~50-200 µs per proto

Actual performance depends on proto complexity and hardware.

## See Also

- `TEST_COVERAGE.md` - Detailed coverage documentation
- `testdata/README.md` - Test fixture documentation
- `proto_loader.go` - Implementation being tested
- `BUILD.bazel` - Bazel build configuration
