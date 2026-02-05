# Sky Test Action

GitHub Action for running Starlark tests with [skytest](https://github.com/albertocavalcante/sky).

## Features

- Native GitHub PR annotations (no third-party actions needed)
- Automatic job summary generation
- Coverage collection and threshold checking
- Parallel test execution support
- Fail-fast mode

## Usage

### Basic Usage

```yaml
- name: Test Starlark files
  uses: albertocavalcante/sky-action@v1
  with:
    path: tests/
```

### With Coverage

```yaml
- name: Test with coverage
  uses: albertocavalcante/sky-action@v1
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
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run Starlark tests
        id: test
        uses: albertocavalcante/sky-action@v1
        with:
          path: .
          recursive: true
          coverage: true
          coverage-threshold: 80
          annotations: true
          summary: true

      - name: Check results
        run: |
          echo "Passed: ${{ steps.test.outputs.passed }}"
          echo "Failed: ${{ steps.test.outputs.failed }}"
          echo "Coverage: ${{ steps.test.outputs.coverage }}%"
```

## Inputs

| Input                | Description                                | Default  |
| -------------------- | ------------------------------------------ | -------- |
| `path`               | Path to test files                         | `.`      |
| `recursive`          | Search directories recursively             | `true`   |
| `coverage`           | Enable coverage collection                 | `false`  |
| `coverage-threshold` | Minimum coverage percentage (0 to disable) | `0`      |
| `annotations`        | Enable GitHub PR annotations               | `true`   |
| `summary`            | Write to job summary                       | `true`   |
| `version`            | Sky version to install                     | `latest` |
| `fail-fast`          | Stop on first test failure                 | `false`  |

## Outputs

| Output     | Description                                    |
| ---------- | ---------------------------------------------- |
| `passed`   | Number of passed tests                         |
| `failed`   | Number of failed tests                         |
| `coverage` | Coverage percentage (when coverage is enabled) |

## How It Works

1. **Installs skytest** - Downloads and installs the skytest binary
2. **Runs tests with annotations** - Uses `-github` flag to output GitHub workflow commands
3. **Generates job summary** - Uses `-markdown` flag to create a beautiful summary
4. **Collects coverage** - Optionally collects and checks coverage against threshold

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

## License

Apache-2.0
