# custom-lint

A Sky plugin that provides custom lint rules for Starlark files.

This example demonstrates how to create custom lint rules that can extend
Sky's built-in linting capabilities.

## Available Rules

| Rule                   | Description                                       |
| ---------------------- | ------------------------------------------------- |
| `no-print`             | Disallow print() statements in production code    |
| `max-params`           | Functions should have at most 5 parameters        |
| `no-underscore-public` | Public functions should not start with underscore |

## Build

```bash
go build -o plugin
```

## Install

```bash
sky plugin install --path ./plugin custom-lint
```

## Usage

```bash
# Lint a single file
sky custom-lint path/to/file.bzl

# Lint all Starlark files in current directory
sky custom-lint .

# Recursively scan a directory
sky custom-lint -r ./src

# Output as JSON
sky custom-lint -json path/to/file.bzl

# List available rules
sky custom-lint -list
```

## Output (Text)

```
example.bzl:15:5: no-print: print() should not be used in production code
example.bzl:20:1: max-params: function "complex" has 8 parameters; consider refactoring (max: 5)

Found 2 issue(s) in 1 file(s)
```

## Output (JSON)

```json
{
  "findings": [
    {
      "file": "example.bzl",
      "line": 15,
      "column": 5,
      "rule": "no-print",
      "message": "print() should not be used in production code"
    }
  ],
  "errors": [],
  "summary": {
    "files": 1,
    "findings": 1,
    "errors": 0
  }
}
```

## Creating Custom Rules

To add your own rules, create a new file in the `rules/` directory:

```go
package rules

import "github.com/bazelbuild/buildtools/build"

var MyRule = Rule{
    Name:        "my-rule",
    Description: "Description of what this rule checks",
    Check:       checkMyRule,
}

func checkMyRule(file *build.File, path string) []Finding {
    var findings []Finding

    // Walk the AST and check for issues
    build.Walk(file, func(expr build.Expr, stack []build.Expr) {
        // Check conditions and append findings
    })

    return findings
}
```

Then add your rule to `AllRules` in `rules.go`:

```go
var AllRules = []Rule{
    NoPrint,
    MaxParams,
    NoUnderscore,
    MyRule,  // Add your rule here
}
```

## Dependencies

This plugin uses:

- [buildtools](https://github.com/bazelbuild/buildtools) - Bazel's build
  language tools for parsing Starlark and walking the AST

## Code Structure

```
custom-lint/
├── main.go           # Plugin entry point
├── rules/
│   ├── rules.go      # Rule definitions and runner
│   ├── no_print.go   # no-print rule
│   ├── max_params.go # max-params rule
│   ├── no_underscore.go
│   └── rules_test.go
├── go.mod
├── go.sum
├── README.md
└── BUILD.bazel
```

## Testing

```bash
go test ./...
```

## Integration with skylint

This plugin can be used alongside `sky lint` to provide additional
project-specific checks. Consider running both:

```bash
sky lint .           # Built-in rules
sky custom-lint .    # Custom rules
```

## Environment Variables

This plugin respects:

- `SKY_OUTPUT_FORMAT=json` - Equivalent to `-json` flag
