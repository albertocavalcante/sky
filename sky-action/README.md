# Sky Test Action

GitHub Action for running Starlark tests with [skytest](https://github.com/albertocavalcante/sky).

## Usage

### Basic Usage

```yaml
- name: Test Starlark files
  uses: albertocavalcante/sky/sky-action@<commit-sha>
  with:
    path: tests/
    version: <commit-sha>
```

### With Coverage Threshold

```yaml
- name: Test with coverage check
  uses: albertocavalcante/sky/sky-action@<commit-sha>
  with:
    path: .
    coverage-threshold: 80
    version: <commit-sha>
```

### Full Example

```yaml
name: Test

on:
  push:
    branches: [main]
  workflow_dispatch:

jobs:
  test:
    runs-on: ${{ matrix.os }}
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]

    steps:
      - uses: actions/checkout@v4

      - name: Run Starlark tests
        id: test
        uses: albertocavalcante/sky/sky-action@<commit-sha>
        with:
          path: .
          version: <commit-sha>
          recursive: true
          annotations: true
          summary: true

      - name: Check results
        shell: bash
        run: |
          echo "Passed: ${{ steps.test.outputs.passed }}"
          echo "Failed: ${{ steps.test.outputs.failed }}"
```

## Inputs

| Input                | Description                                      | Default  |
| -------------------- | ------------------------------------------------ | -------- |
| `path`               | Path to test files                               | `.`      |
| `recursive`          | Search directories recursively                   | `true`   |
| `coverage-threshold` | Minimum coverage percentage (0 to disable)       | `0`      |
| `annotations`        | Enable GitHub annotations                        | `true`   |
| `summary`            | Write to job summary                             | `true`   |
| `version`            | Sky module version, branch, or commit hash       | action ref |
| `fail-fast`          | Stop on first test failure                       | `false`  |
| `timeout`            | Timeout per test (Go duration format, e.g., 30s) | `30s`    |

## Outputs

| Output     | Description                                    |
| ---------- | ---------------------------------------------- |
| `passed`   | Number of passed tests                         |
| `failed`   | Number of failed tests                         |
| `coverage` | Coverage percentage (when coverage is enabled) |

## Pipeline

`skytest` runs tests. `sky-ci` turns JSON output into GitHub annotations,
outputs, and job summaries.

## GitHub Annotations

When `annotations: true` (default), test failures appear as GitHub annotations:

- Failed tests show as error annotations with file and line info
- Skipped tests show as notice annotations

## Job Summary

When `summary: true` (default), a Markdown summary is added showing:

- Total tests, passed, failed, skipped counts
- Collapsible details for failed tests
- Test duration

## Cross-Platform Support

Supported runners:

- **Linux** (ubuntu-latest, ubuntu-22.04, etc.)
- **macOS** (macos-latest, macos-14, etc.)
- **Windows** (windows-latest, windows-2022, etc.)

## Using sky-ci Directly

You can also use the `sky-ci` plugin directly in custom workflows:

```yaml
- name: Run tests with custom reporter
  run: |
    skytest -json ./tests | sky-ci --system=github
```

## License

Apache-2.0
