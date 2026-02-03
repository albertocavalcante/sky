# star-counter

A Sky plugin that analyzes Starlark files and counts definitions, loads, and
function calls.

This example demonstrates a real-world plugin that uses external dependencies
(buildtools) to parse and analyze Starlark files.

## Features

- Counts function definitions (`def` statements)
- Counts load statements
- Counts function calls
- Counts assignments
- Counts lines
- Supports JSON and text output
- Recursive directory scanning

## Build

```bash
go build -o plugin
```

## Install

```bash
sky plugin install --path ./plugin star-counter
```

## Usage

```bash
# Analyze a single file
sky star-counter path/to/file.bzl

# Analyze all Starlark files in current directory
sky star-counter .

# Recursively scan a directory
sky star-counter -r ./src

# Output as JSON
sky star-counter -json path/to/file.bzl

# Multiple files
sky star-counter file1.bzl file2.star BUILD
```

## Output (Text)

```
FILE                                       DEFS  LOADS  CALLS ASSIGN  LINES
------------------------------------------------------------------------------
example.bzl                                   2      1      5      3     45
utils.star                                    5      2     12      8     98
BUILD.bazel                                   0      1     15      0     52
------------------------------------------------------------------------------
TOTAL (3 files)                               7      4     32     11    195
```

## Output (JSON)

```json
{
  "files": [
    {
      "path": "example.bzl",
      "defs": 2,
      "loads": 1,
      "calls": 5,
      "assigns": 3,
      "lines": 45
    }
  ],
  "errors": [],
  "totals": {
    "path": "TOTAL",
    "defs": 2,
    "loads": 1,
    "calls": 5,
    "assigns": 3,
    "lines": 45
  }
}
```

## Recognized File Types

The plugin analyzes files with these extensions/names:

- `*.star` - Starlark files
- `*.bzl` - Bazel extension files
- `BUILD` / `BUILD.bazel` - Bazel BUILD files
- `WORKSPACE` / `WORKSPACE.bazel` - Bazel workspace files

## Dependencies

This plugin uses:

- [buildtools](https://github.com/bazelbuild/buildtools) - Bazel's build
  language tools for parsing Starlark

## Code Structure

```
star-counter/
├── main.go           # Plugin entry point
├── counter/
│   ├── counter.go    # Analysis logic
│   └── counter_test.go
├── go.mod
├── go.sum
├── README.md
└── BUILD.bazel
```

## Testing

```bash
go test ./...
```

## Environment Variables

This plugin respects:

- `SKY_OUTPUT_FORMAT=json` - Equivalent to `-json` flag
