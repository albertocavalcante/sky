# Proto Loader Test Fixtures

This directory contains test fixtures for the proto loader implementation.

## Structure

- `proto/` - Contains proto test data files in both binary and text formats

## Test Files

### test_simple.pbtxt

A simple proto text file containing:

- One type (`TestType`) with fields and a method
- One function (`test_function`) with various parameter types
- Two constants (`TEST_CONSTANT`, `TEST_NUMBER`)

Used for testing basic proto parsing and conversion.

### test_empty.pbtxt

An empty proto file for testing edge cases.

### test_invalid.pbtxt

An invalid proto file for testing error handling.

## Usage

These fixtures are used by `proto_loader_test.go` to verify:

- Proto parsing (binary and text formats)
- Conversion from proto to Go structs
- Error handling for invalid data
- Cache behavior
- Concurrent access

## Adding New Fixtures

When adding new test fixtures:

1. Create the `.pbtxt` file with clear naming
2. Document the purpose in this README
3. Add corresponding test cases in `proto_loader_test.go`
