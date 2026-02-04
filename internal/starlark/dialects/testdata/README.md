# Dialect Test Corpus

Test fixtures for Starlark dialect support including config files, builtin definitions, and source files.

## Directory Structure

```
testdata/
├── builtins/                    # Builtin definition files
│   ├── json/                    # JSON format (.builtins.json)
│   │   ├── tilt.builtins.json
│   │   ├── copybara.builtins.json
│   │   ├── custom.builtins.json
│   │   ├── minimal.builtins.json
│   │   ├── comprehensive.builtins.json
│   │   └── invalid.builtins.json
│   ├── textproto/               # Textproto format (.builtins.textproto)
│   │   ├── tilt.builtins.textproto
│   │   ├── bazel.builtins.textproto
│   │   └── invalid.builtins.textproto
│   └── pyi/                     # Python stub format (.builtins.pyi)
│       ├── tilt.builtins.pyi
│       ├── copybara.builtins.pyi
│       └── invalid.builtins.pyi
├── configs/                     # Config file examples
│   ├── minimal.config.json
│   ├── full.config.json
│   ├── multi-dialect.config.json
│   ├── remote-urls.config.json
│   ├── invalid-syntax.config.json
│   └── invalid-schema.config.json
├── sources/                     # Starlark source files for testing
│   ├── tilt/
│   │   ├── Tiltfile
│   │   └── tilt_modules/helper.star
│   ├── copybara/
│   │   └── copy.bara.sky
│   ├── bazel/
│   │   ├── BUILD
│   │   └── rules.bzl
│   └── custom/
│       └── workflow.star
└── projects/                    # Complete project fixtures
    ├── simple/                  # Single dialect project
    ├── complex/                 # Multi-file project
    └── multi-dialect/           # Mixed dialect project
```

## Test Categories

### 1. Builtin Loading Tests

- Valid JSON parsing and schema compliance
- Valid textproto parsing
- Valid Python stub parsing
- Invalid file handling
- Missing file handling
- Format auto-detection by extension

### 2. Config Loading Tests

- Minimal config (just dialect)
- Full config (rules, dialects, settings)
- Glob pattern matching
- Remote URL references
- Config inheritance/extends
- Invalid syntax handling
- Invalid schema handling

### 3. Integration Tests

- Completion with builtins
- Hover documentation from builtins
- Diagnostics with dialect-aware predeclared names
- Multi-dialect project support

## Adding New Fixtures

1. Add the fixture file to the appropriate directory
2. Update this README
3. Add corresponding test cases
4. Ensure both positive and negative test cases exist
