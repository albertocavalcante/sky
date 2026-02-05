# CI Integration Improvement Ideas

> Captured from coverage documentation research (2026-02-05)

## Context

While documenting Starlark coverage and CI integration, we identified several gaps and improvement opportunities. This document tracks these ideas for future implementation.

## Current State

**What works today:**

- `skytest -junit` outputs JUnit XML for test results
- `skytest -json` outputs JSON test results
- `skytest --coverage --coverage-output=file` outputs coverage (Cobertura, LCOV, JSON, HTML, Text)
- Third-party actions like `dorny/test-reporter` can display JUnit XML in PRs

**Pain points:**

- Running tests twice (once for JUnit, once for coverage)
- Manual scripting needed for GitHub Job Summaries
- No native GitHub annotations without third-party actions
- No single "CI mode" command

---

## Improvement Ideas

### 1. Combined JUnit + Coverage Output

**Problem:** CI workflows run tests twice:

```yaml
- run: skytest -junit tests/ > test-results.xml      # Run 1
- run: skytest --coverage tests/                      # Run 2
```

**Solution:** Allow both flags together:

```bash
skytest -junit --coverage --coverage-output=coverage.xml tests/ > test-results.xml
```

**Implementation:**

- Modify runner to support multiple reporters simultaneously
- Output JUnit XML to stdout, coverage to file (or vice versa)
- Consider `--test-output` flag for explicit test result file

**Effort:** Low
**Value:** High - halves CI run time for test+coverage workflows

---

### 2. Native GitHub Annotations Output

**Problem:** Users need third-party actions (`dorny/test-reporter`, `mikepenz/action-junit-report`) to get PR annotations.

**Solution:** `--github` flag that outputs [GitHub workflow commands](https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions):

```bash
skytest --github tests/
```

Output:

```
::error file=math_test.star,line=15,col=5::assertion failed: eq(3, 5)
::error file=utils_test.star,line=42::test_divide failed: division by zero
::notice file=parser.star,line=10::Line not covered by tests
```

**Features:**

- `::error` for test failures (appears as red annotation in PR diff)
- `::warning` for uncovered lines (optional, with `--coverage`)
- `::notice` for skipped tests
- `::group`/`::endgroup` for collapsible sections

**Implementation:**

- New `GitHubReporter` in reporter.go
- Parse file paths and line numbers from test failures
- Format as GitHub workflow commands

**Effort:** Medium
**Value:** High - zero-dependency PR annotations

---

### 3. Markdown Reporter for Job Summaries

**Problem:** GitHub Job Summaries (`$GITHUB_STEP_SUMMARY`) require manual Markdown generation.

**Solution:** `--format=markdown` or `--summary` flag:

```bash
skytest --format=markdown tests/ >> $GITHUB_STEP_SUMMARY
```

Output:

```markdown
## 🧪 Test Results

**45 tests** in 3 files completed in **1.23s**

| Status    | Count |
| --------- | ----- |
| ✅ Passed | 42    |
| ❌ Failed | 2     |
| ⏭️ Skipped | 1     |

### ❌ Failed Tests

<details>
<summary><code>math_test.star::test_divide</code></summary>
```

assertion failed: eq(None, 5)
at math_test.star:15

```
</details>

<details>
<summary><code>utils_test.star::test_parse</code></summary>
```

assertion failed: contains(result, "expected")
at utils_test.star:42

```
</details>

### 📊 Coverage

| Metric | Value |
|--------|-------|
| Line Coverage | 85.2% |
| Files | 12 |
| Covered Lines | 340/399 |
```

**Implementation:**

- New `MarkdownReporter` in reporter.go
- Support collapsible sections with `<details>` tags
- Include coverage summary if `--coverage` is also specified

**Effort:** Low
**Value:** Medium - beautiful job summaries without scripting

---

### 4. CI Mode - Consolidated Output

**Problem:** CI pipelines need multiple outputs (test results, coverage, summary). Users must configure each separately.

**Solution:** `--ci` flag or `skytest ci` subcommand:

```bash
skytest --ci tests/
# Or
skytest ci tests/
```

Produces:

```
test-results.xml     # JUnit XML
coverage.xml         # Cobertura XML
SUMMARY.md           # Markdown summary
```

Plus stdout:

```
::group::Test Results
... GitHub annotations ...
::endgroup::
```

**Configuration via sky.toml:**

```toml
[test.ci]
test_output = "test-results.xml"
coverage_output = "coverage.xml"
summary_output = "SUMMARY.md"
annotations = true
```

**Implementation:**

- New `ci` subcommand or `--ci` flag
- Runs all reporters in single test execution
- Sensible defaults for output paths

**Effort:** Medium
**Value:** High - one command for complete CI setup

---

### 5. Official GitHub Action

**Problem:** Users must write boilerplate YAML for Sky integration.

**Solution:** `albertocavalcante/sky-action` GitHub Action:

```yaml
- name: Test Starlark
  uses: albertocavalcante/sky-action@v1
  with:
    command: test
    path: tests/
    coverage: true
    coverage-threshold: 80
    annotate: true
    summary: true
    upload-coverage: codecov
```

**Features:**

- Auto-installs skytest
- Runs tests with specified options
- Generates GitHub annotations
- Writes job summary
- Uploads coverage to Codecov/Coveralls
- Caches Go binary for faster runs

**Implementation:**

- Separate repository: `albertocavalcante/sky-action`
- Composite action or JavaScript action
- Wraps skytest CLI with GitHub-specific features

**Effort:** Medium
**Value:** Medium - simplifies adoption but not strictly necessary

---

### 6. TAP (Test Anything Protocol) Format

**Problem:** RFC-001 mentions TAP support as planned but not implemented.

**Solution:** `--format=tap` flag:

```bash
skytest --format=tap tests/
```

Output:

```
TAP version 14
1..5
ok 1 - test_addition
ok 2 - test_subtraction
not ok 3 - test_divide
  ---
  message: assertion failed: eq(None, 5)
  severity: fail
  at:
    file: math_test.star
    line: 15
  ...
ok 4 - test_string_ops
ok 5 - test_list_ops # SKIP not implemented
```

**Implementation:**

- New `TAPReporter` in reporter.go
- Follow TAP 14 specification
- Support YAML diagnostics for failures

**Effort:** Low
**Value:** Low - TAP is less common than JUnit XML in modern CI

---

## Priority Matrix

| Idea                      | Effort | Value  | Priority |
| ------------------------- | ------ | ------ | -------- |
| Combined JUnit + Coverage | Low    | High   | **P0**   |
| GitHub Annotations        | Medium | High   | **P0**   |
| Markdown Reporter         | Low    | Medium | **P1**   |
| CI Mode                   | Medium | High   | **P1**   |
| GitHub Action             | Medium | Medium | **P2**   |
| TAP Format                | Low    | Low    | **P3**   |

---

## Implementation Roadmap

### Phase 1: Quick Wins

- [ ] Combined `-junit --coverage` in single run
- [ ] `--format=markdown` reporter

### Phase 2: GitHub Native

- [ ] `--github` flag for workflow commands
- [ ] Integrate annotations with coverage (uncovered lines as warnings)

### Phase 3: CI Experience

- [ ] `--ci` mode with sensible defaults
- [ ] `sky.toml` CI configuration section

### Phase 4: Ecosystem

- [ ] `sky-action` GitHub Action
- [ ] TAP format (if requested)

---

## References

- [GitHub Workflow Commands](https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions)
- [GitHub Job Summaries](https://github.blog/news-insights/product-news/supercharging-github-actions-with-job-summaries/)
- [dorny/test-reporter](https://github.com/marketplace/actions/test-reporter) - current recommended action
- [TAP Specification](https://testanything.org/tap-version-14-specification.html)
- [JUnit XML Format](https://github.com/testmoapp/junitxml)
- gvy CI workflow: `/Users/adsc/dev/ws/gvy/main/.github/workflows/ci.yml`

---

## Notes

- Research conducted while documenting coverage features
- gvy project uses `dorny/test-reporter@v2` with Kover for Kotlin coverage
- JetBrains IDEs do NOT support LCOV import (corrected in docs)
- VS Code Coverage Gutters (805K installs) is the recommended extension
