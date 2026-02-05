# Starlark Coverage

Collect line and function coverage from your Starlark test runs.

## Quick Start

Enable coverage in `sky.toml`:

```toml
[test.coverage]
enabled = true
output = "coverage.lcov"
```

Run tests:

```bash
skytest tests/
# Coverage report written to coverage.lcov
```

## Output Formats

Skytest supports five output formats. The format is determined by the file extension.

| Extension | Format    | Use Case                            |
| --------- | --------- | ----------------------------------- |
| `.txt`    | Text      | Terminal viewing, quick inspection  |
| `.json`   | JSON      | Programmatic access, custom tooling |
| `.xml`    | Cobertura | CI integration (Jenkins, GitLab)    |
| `.lcov`   | LCOV      | IDE extensions, `genhtml`           |
| `.html`   | HTML      | Browser viewing, sharing with team  |

### Text Format

Human-readable summary for terminal output.

```toml
[test.coverage]
enabled = true
output = "coverage.txt"
```

Output:

```
Coverage Report
===============

src/parser.star                                              87.5% (14/16 lines)
  Missing: 23-24
src/lexer.star                                               100.0% (12/12 lines)
src/utils.star                                               66.7% (8/12 lines)
  Missing: 5, 18-20

Total: 85.0% (34/40 lines)
```

### JSON Format

Structured data for programmatic access.

```toml
[test.coverage]
enabled = true
output = "coverage.json"
```

Output:

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "total_lines": 40,
  "covered_lines": 34,
  "percentage": 85.0,
  "files": [
    {
      "path": "src/parser.star",
      "total_lines": 16,
      "covered_lines": 14,
      "percentage": 87.5,
      "missing_lines": [23, 24]
    },
    {
      "path": "src/lexer.star",
      "total_lines": 12,
      "covered_lines": 12,
      "percentage": 100.0
    }
  ]
}
```

### Cobertura XML Format

Standard format for CI systems.

```toml
[test.coverage]
enabled = true
output = "coverage.xml"
```

Supported by:

- Jenkins (Cobertura plugin)
- GitLab CI (built-in)
- Azure DevOps
- Codecov
- Coveralls

Output structure:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<coverage line-rate="0.8500" branch-rate="0" version="1.0" timestamp="1705312200">
  <sources>
    <source>.</source>
  </sources>
  <packages>
    <package name="src" line-rate="0.8500">
      <classes>
        <class name="parser.star" filename="src/parser.star" line-rate="0.8750">
          <lines>
            <line number="1" hits="1"/>
            <line number="2" hits="3"/>
            <line number="23" hits="0"/>
            <line number="24" hits="0"/>
          </lines>
        </class>
      </classes>
    </package>
  </packages>
</coverage>
```

### LCOV Format

Tracefile format compatible with `genhtml` and IDE extensions.

```toml
[test.coverage]
enabled = true
output = "coverage.lcov"
```

Supported by:

- VS Code Coverage Gutters extension
- JetBrains IDEs
- `genhtml` (generates HTML from LCOV)
- SonarQube

Output:

```
TN:
SF:src/parser.star
FN:10,parse_expression
FNDA:5,parse_expression
FN:25,parse_statement
FNDA:3,parse_statement
FNF:2
FNH:2
DA:1,1
DA:2,3
DA:10,5
DA:11,5
DA:23,0
DA:24,0
LF:16
LH:14
end_of_record
TN:
SF:src/lexer.star
...
end_of_record
```

LCOV field reference:

| Field   | Meaning                        |
| ------- | ------------------------------ |
| `TN:`   | Test name (empty)              |
| `SF:`   | Source file path               |
| `FN:`   | Function: `line,name`          |
| `FNDA:` | Function hit data: `hits,name` |
| `FNF:`  | Functions found (total)        |
| `FNH:`  | Functions hit (covered)        |
| `DA:`   | Line data: `line,hits`         |
| `LF:`   | Lines found (total)            |
| `LH:`   | Lines hit (covered)            |

Generate HTML from LCOV:

```bash
genhtml coverage.lcov -o coverage-html/
open coverage-html/index.html
```

### HTML Format

Self-contained HTML report with embedded CSS.

```toml
[test.coverage]
enabled = true
output = "coverage.html"
```

Features:

- Dark theme
- Collapsible file sections
- Color-coded coverage badges (green >80%, yellow 50-80%, red <50%)
- Per-line hit counts
- No external dependencies

Open in browser:

```bash
skytest tests/
open coverage.html
```

## Configuration

### TOML Configuration

```toml
[test.coverage]
# Enable coverage collection
enabled = true

# Output file (extension determines format)
output = "coverage.lcov"

# Fail if coverage drops below threshold
fail_under = 80.0
```

### Starlark Configuration

```python
def configure():
    ci = getenv("CI", "") != ""
    return {
        "test": {
            "coverage": {
                "enabled": ci,
                "output": "coverage.xml" if ci else "coverage.html",
                "fail_under": 80 if ci else 0,
            },
        },
    }
```

### Configuration Options

| Option       | Type   | Default           | Description                       |
| ------------ | ------ | ----------------- | --------------------------------- |
| `enabled`    | bool   | `false`           | Enable coverage collection        |
| `output`     | string | `"coverage.json"` | Output file path                  |
| `fail_under` | float  | `0`               | Minimum coverage % (0 = disabled) |

## CI Integration

### GitHub Actions

#### Complete Example (Tests + Coverage + PR Annotations)

```yaml
name: Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run tests (JUnit XML for PR annotations)
        run: skytest -junit -r tests/ > test-results.xml

      - name: Run tests with coverage
        run: skytest --coverage --coverage-output=coverage.xml -r tests/

      - name: Test Report (shows in PR checks)
        uses: dorny/test-reporter@v1
        if: always()
        with:
          name: Starlark Tests
          path: test-results.xml
          reporter: java-junit
          fail-on-error: false

      - name: Upload coverage to Codecov
        uses: codecov/codecov-action@v4
        with:
          files: coverage.xml
```

The [dorny/test-reporter](https://github.com/marketplace/actions/test-reporter) action displays test results directly in PR checks with pass/fail annotations.

### GitLab CI

```yaml
test:
  script:
    - skytest --coverage --coverage-output=coverage.xml tests/
  coverage: '/Total: (\d+\.\d+)%/'
  artifacts:
    reports:
      coverage_report:
        coverage_format: cobertura
        path: coverage.xml
```

### Jenkins

```groovy
pipeline {
    stages {
        stage('Test') {
            steps {
                sh 'skytest --coverage --coverage-output=coverage.xml tests/'
            }
            post {
                always {
                    cobertura coberturaReportFile: 'coverage.xml'
                }
            }
        }
    }
}
```

## Viewing Coverage in IDEs

### VS Code (Recommended)

Install the [Coverage Gutters](https://marketplace.visualstudio.com/items?itemName=ryanluker.vscode-coverage-gutters) extension (805K+ installs, 5/5 rating):

```bash
code --install-extension ryanluker.vscode-coverage-gutters
```

Setup:

1. Generate LCOV output:
   ```toml
   [test.coverage]
   enabled = true
   output = "coverage.lcov"
   ```
2. Run tests: `skytest tests/`
3. Press `Cmd+Shift+7` (macOS) or `Ctrl+Shift+7` (Windows/Linux)
4. Or click "Watch" in the status bar for auto-updates

Lines are highlighted:

- Green: covered
- Red: not covered

### JetBrains IDEs

JetBrains IDEs (IntelliJ, GoLand, WebStorm) do **not** natively support LCOV import. Use HTML output instead:

```toml
[test.coverage]
enabled = true
output = "coverage.html"
```

```bash
skytest tests/
open coverage.html
```

The HTML report opens in your browser with file list, coverage badges, and line details.

## Coverage Metrics

Skytest tracks two types of coverage:

### Line Coverage

Counts how many times each line executed.

```
DA:10,5    # Line 10 executed 5 times
DA:11,0    # Line 11 never executed
```

### Function Coverage

Counts function calls.

```
FN:10,parse_expression      # Function starts at line 10
FNDA:5,parse_expression     # Called 5 times
```

## Merging Coverage

Combine coverage from multiple test runs:

```bash
# Run different test suites
skytest --coverage --coverage-output=unit.lcov tests/unit/
skytest --coverage --coverage-output=integration.lcov tests/integration/

# Merge with lcov (if installed)
lcov -a unit.lcov -a integration.lcov -o combined.lcov

# Generate HTML
genhtml combined.lcov -o coverage-report/
```

## Enforcing Coverage Thresholds

Fail the build if coverage drops below a threshold:

```toml
[test.coverage]
enabled = true
output = "coverage.lcov"
fail_under = 80.0
```

Output when threshold not met:

```
FAIL: Coverage 75.5% is below threshold 80.0%
```

## Excluding Files

Exclude test helpers and generated code from coverage:

```toml
[test.coverage]
enabled = true
output = "coverage.lcov"
exclude = [
    "test/helpers.star",
    "generated/*.star",
]
```

## Troubleshooting

### No coverage data generated

1. Verify coverage is enabled:
   ```bash
   skytest -v tests/
   # Look for: "coverage: enabled"
   ```

2. Check the output path is writable:
   ```bash
   touch coverage.lcov
   ```

### Coverage percentage seems wrong

Coverage only includes lines that are executed during testing. Lines in functions that are never called show as uncovered.

Check which lines are missing:

```bash
# Text format shows missing lines
skytest --coverage --coverage-output=coverage.txt tests/
cat coverage.txt
```

### IDE not showing coverage

1. Ensure you're using LCOV format (`.lcov` extension)
2. Check the file path matches your project structure
3. Reload the coverage data after running tests

### Format not recognized

The format is determined by file extension:

| Extension | Format    |
| --------- | --------- |
| `.txt`    | Text      |
| `.json`   | JSON      |
| `.xml`    | Cobertura |
| `.lcov`   | LCOV      |
| `.html`   | HTML      |

Other extensions default to JSON.
