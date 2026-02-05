# Sky Test Action

GitHub Action for running Starlark tests with [skytest](https://github.com/albertocavalcante/sky).

## Features

- **Cross-platform**: Works on Linux, macOS, and Windows runners
- **Plugin architecture**: Uses `sky-ci` plugin for CI-specific output
- Native GitHub PR annotations (no third-party actions needed)
- Automatic job summary generation
- Coverage threshold checking
- Fail-fast mode

## Usage

### Basic Usage

```yaml
- name: Test Starlark files
  uses: albertocavalcante/sky/sky-action@v1
  with:
    path: tests/
```

### With Coverage Threshold

```yaml
- name: Test with coverage check
  uses: albertocavalcante/sky/sky-action@v1
  with:
    path: .
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

## Architecture

This action follows Sky's plugin-first philosophy:

```
skytest -json .  ───►  sky-ci  ───►  GitHub-specific outputs
     │                   │
     │                   ├── PR annotations (::error::, ::notice::)
     │                   ├── $GITHUB_OUTPUT (passed, failed, coverage)
     │                   └── $GITHUB_STEP_SUMMARY (Markdown table)
     │
     └── Core tool (test execution, JSON output)
```

The pipeline pattern (`skytest | sky-ci`) keeps the core tool minimal while allowing CI-specific logic in a dedicated plugin.

## PR Annotations

When `annotations: true` (default), test failures appear as annotations directly in your PR:

- Failed tests show as error annotations with file and line info
- Skipped tests show as notice annotations

## Job Summary

When `summary: true` (default), a Markdown summary is added showing:

- Total tests, passed, failed, skipped counts
- Collapsible details for failed tests
- Test duration

## Cross-Platform Support

This action works identically on:

- **Linux** (ubuntu-latest, ubuntu-22.04, etc.)
- **macOS** (macos-latest, macos-14, etc.)
- **Windows** (windows-latest, windows-2022, etc.)

Cross-platform support is achieved through:

1. Go's native cross-compilation - both `skytest` and `sky-ci` are single binaries
2. GitHub Actions' bash support on all platforms (Windows uses Git Bash)
3. All file path handling done in Go, not shell scripts

## Using sky-ci Directly

You can also use the `sky-ci` plugin directly in custom workflows:

```yaml
- name: Run tests with custom reporter
  run: |
    skytest -json ./tests | sky-ci --system=github
```

The `sky-ci` plugin auto-detects the CI system from environment variables:

| CI System      | Detection Variable |
| -------------- | ------------------ |
| GitHub Actions | `GITHUB_ACTIONS`   |
| GitLab CI      | `GITLAB_CI`        |
| CircleCI       | `CIRCLECI`         |
| Azure DevOps   | `TF_BUILD`         |
| Jenkins        | `JENKINS_URL`      |

## License

Apache-2.0
