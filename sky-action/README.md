# Sky Test Action

GitHub Action for running Starlark tests with [skytest](https://github.com/albertocavalcante/sky).

## Features

- **Cross-platform**: Works on Linux, macOS, and Windows runners
- Native GitHub PR annotations (no third-party actions needed)
- Automatic job summary generation
- Coverage collection and threshold checking
- Parallel test execution support
- Fail-fast mode

## Usage

### Basic Usage

```yaml
- name: Test Starlark files
  uses: albertocavalcante/sky/sky-action@v1
  with:
    path: tests/
```

### With Coverage

```yaml
- name: Test with coverage
  uses: albertocavalcante/sky/sky-action@v1
  with:
    path: .
    coverage: true
    coverage-threshold: 80
```

### Full Example

```yaml
name: Test

on:
  push:
    branches: [main]
  pull_request:

jobs:
  test:
    # Works on any platform!
    runs-on: ${{ matrix.os }}
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]

    steps:
      - uses: actions/checkout@v4

      - name: Run Starlark tests
        id: test
        uses: albertocavalcante/sky/sky-action@v1
        with:
          path: .
          recursive: true
          coverage: true
          coverage-threshold: 80
          annotations: true
          summary: true

      - name: Check results
        shell: bash
        run: |
          echo "Passed: ${{ steps.test.outputs.passed }}"
          echo "Failed: ${{ steps.test.outputs.failed }}"
          echo "Coverage: ${{ steps.test.outputs.coverage }}%"
```

## Inputs

| Input                | Description                                      | Default  |
| -------------------- | ------------------------------------------------ | -------- |
| `path`               | Path to test files                               | `.`      |
| `recursive`          | Search directories recursively                   | `true`   |
| `coverage`           | Enable coverage collection                       | `false`  |
| `coverage-threshold` | Minimum coverage percentage (0 to disable)       | `0`      |
| `annotations`        | Enable GitHub PR annotations                     | `true`   |
| `summary`            | Write to job summary                             | `true`   |
| `version`            | Sky version to install                           | `latest` |
| `fail-fast`          | Stop on first test failure                       | `false`  |
| `timeout`            | Timeout per test (Go duration format, e.g., 30s) | `30s`    |

## Outputs

| Output     | Description                                    |
| ---------- | ---------------------------------------------- |
| `passed`   | Number of passed tests                         |
| `failed`   | Number of failed tests                         |
| `coverage` | Coverage percentage (when coverage is enabled) |

## How It Works

1. **Installs skytest** - Downloads and installs the cross-platform skytest binary via `go install`
2. **Runs tests with `skytest action`** - The built-in action subcommand handles:
   - GitHub workflow commands for PR annotations (`-github` flag internally)
   - Markdown summary generation (`-markdown` flag internally)
   - Writing outputs to `$GITHUB_OUTPUT`
   - Coverage threshold checking

The entire action is powered by a single Go binary, ensuring consistent behavior across all platforms.

## PR Annotations

When `annotations: true` (default), test failures appear as annotations directly in your PR:

- Failed tests show as error annotations in the diff
- Skipped tests show as notice annotations
- Line numbers are included when available

## Job Summary

When `summary: true` (default), a Markdown summary is added to the job output showing:

- Total tests, passed, failed, skipped counts
- Collapsible details for failed tests
- Coverage statistics (when enabled)

## Cross-Platform Support

This action works identically on:

- **Linux** (ubuntu-latest, ubuntu-22.04, etc.)
- **macOS** (macos-latest, macos-14, etc.)
- **Windows** (windows-latest, windows-2022, etc.)

The cross-platform support is achieved through:

1. Go's native cross-compilation - skytest is a single binary with no dependencies
2. GitHub Actions' bash support on all platforms (Windows uses Git Bash)
3. All file path handling and I/O done in Go code, not shell scripts

## License

Apache-2.0
