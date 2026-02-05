# Plan: CI Reporter Plugin

## Summary

Refactor the GitHub Actions integration from core `skytest` into a standalone `sky ci` plugin, following Sky's plugin-first architecture.

## Current State

```
skytest (core binary)
├── internal/cmd/skytest/action.go   # CI-specific logic (248 lines)
├── internal/cmd/skytest/run.go      # Routes "action" subcommand
└── Reporters (built-in)
    ├── TextReporter
    ├── JSONReporter
    ├── JUnitReporter
    ├── MarkdownReporter
    └── GitHubReporter
```

**Problem:** CI-specific code is baked into core, violating plugin-first philosophy.

## Target State

```
skytest (core)                    sky-ci (plugin)
├── text                          ├── Auto-detect CI environment
├── json  ───── stdin ──────────► ├── GitHub Actions
├── junit                         ├── GitLab CI
└── markdown                      ├── CircleCI
                                  ├── Azure DevOps
                                  └── Generic (env vars)
```

## Architecture

### Data Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                        GitHub Actions                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   ┌──────────┐      JSON       ┌──────────┐                     │
│   │ skytest  │ ──────────────► │  sky ci  │                     │
│   │  -json   │    (stdin)      │ (plugin) │                     │
│   └──────────┘                 └────┬─────┘                     │
│                                     │                            │
│                    ┌────────────────┼────────────────┐          │
│                    ▼                ▼                ▼          │
│            $GITHUB_OUTPUT   $GITHUB_STEP_SUMMARY   stdout       │
│            (passed=N)       (Markdown table)       (annotations)│
│            (failed=N)                              (::error::)  │
│            (coverage=N)                            (::notice::) │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### CI Environment Detection

The plugin auto-detects the CI system from environment variables:

| CI System      | Detection Variable | Value  |
| -------------- | ------------------ | ------ |
| GitHub Actions | `GITHUB_ACTIONS`   | `true` |
| GitLab CI      | `GITLAB_CI`        | `true` |
| CircleCI       | `CIRCLECI`         | `true` |
| Azure DevOps   | `TF_BUILD`         | `True` |
| Jenkins        | `JENKINS_URL`      | (set)  |
| Travis CI      | `TRAVIS`           | `true` |
| Generic        | (fallback)         | -      |

### Plugin Interface

```go
// Input: JSON test results from stdin
type TestResults struct {
    Files    []FileResult `json:"files"`
    Duration string       `json:"duration"`
    Summary  Summary      `json:"summary"`
}

type Summary struct {
    Passed  int `json:"passed"`
    Failed  int `json:"failed"`
    Skipped int `json:"skipped"`
    Total   int `json:"total"`
}

// Output: CI-specific formats
// - GitHub: workflow commands, GITHUB_OUTPUT, GITHUB_STEP_SUMMARY
// - GitLab: artifacts, job output
// - Generic: structured text
```

## Implementation Plan

### Phase 1: Create Plugin Structure

```
cmd/sky-ci/
├── main.go              # Entry point, uses skyplugin.Serve()
├── ci.go                # Main logic, CI detection
├── github.go            # GitHub Actions handler
├── gitlab.go            # GitLab CI handler (stub)
├── generic.go           # Generic fallback
└── types.go             # Shared types
```

### Phase 2: Implement GitHub Handler

The GitHub handler will:

1. **Read JSON from stdin** - Parse test results
2. **Write annotations to stdout** - `::error::` and `::notice::` commands
3. **Write to $GITHUB_OUTPUT** - `passed`, `failed`, `coverage`
4. **Write to $GITHUB_STEP_SUMMARY** - Markdown table

```go
func (g *GitHubHandler) Handle(results *TestResults) error {
    // 1. Output annotations for failures
    for _, file := range results.Files {
        for _, test := range file.Tests {
            if !test.Passed {
                fmt.Printf("::error file=%s,line=%d::%s: %s\n",
                    file.Path, test.Line, test.Name, test.Error)
            }
        }
    }

    // 2. Write outputs
    g.writeOutput("passed", results.Summary.Passed)
    g.writeOutput("failed", results.Summary.Failed)

    // 3. Write summary
    g.writeSummary(results)

    return nil
}
```

### Phase 3: Update skytest JSON Output

Ensure `skytest -json` outputs all needed fields:

```json
{
  "files": [
    {
      "path": "/path/to/test.star",
      "tests": [
        {
          "name": "test_example",
          "passed": false,
          "skipped": false,
          "duration": "1.234ms",
          "error": "assertion failed: expected 1, got 2",
          "line": 42
        }
      ]
    }
  ],
  "duration": "123.456ms",
  "summary": {
    "passed": 10,
    "failed": 1,
    "skipped": 2,
    "total": 13
  }
}
```

### Phase 4: Update GitHub Action

```yaml
# sky-action/action.yml
- name: Install sky-ci plugin
  shell: bash
  run: |
    sky plugin install ci --url https://github.com/albertocavalcante/sky/releases/download/${{ inputs.version }}/sky-ci-${{ runner.os }}-${{ runner.arch }}

- name: Run tests
  id: test
  shell: bash
  run: |
    skytest -json -r "${{ inputs.path }}" | sky ci
```

Or if we embed sky-ci in the full build:

```yaml
- name: Run tests
  shell: bash
  run: |
    skytest -json -r "${{ inputs.path }}" | sky ci
```

### Phase 5: Remove Core Action Code

Delete from skytest:

- `internal/cmd/skytest/action.go`
- Action subcommand routing in `run.go`

### Phase 6: Add Other CI Systems (Future)

Stub implementations for:

- GitLab CI (artifacts, job output)
- CircleCI (test metadata)
- Azure DevOps (##vso commands)

## CLI Interface

```bash
# Auto-detect CI system
skytest -json . | sky ci

# Explicit CI system
skytest -json . | sky ci --system=github

# With options
skytest -json . | sky ci --coverage-threshold=80

# Help
sky ci --help
```

### Flags

| Flag                   | Description                                          | Default      |
| ---------------------- | ---------------------------------------------------- | ------------ |
| `--system`             | CI system (github, gitlab, circleci, azure, generic) | auto-detect  |
| `--coverage-threshold` | Fail if coverage below threshold                     | 0 (disabled) |
| `--annotations`        | Enable PR annotations                                | true         |
| `--summary`            | Write job summary                                    | true         |
| `--quiet`              | Suppress stdout                                      | false        |

## File Changes Summary

### New Files

```
cmd/sky-ci/
├── main.go
├── ci.go
├── github.go
├── gitlab.go
├── generic.go
└── types.go
```

### Modified Files

```
sky-action/action.yml     # Use pipeline pattern
sky-action/README.md      # Update docs
```

### Deleted Files

```
internal/cmd/skytest/action.go
```

## Testing Strategy

1. **Unit tests** for each CI handler
2. **Integration test** with mock CI environment variables
3. **E2E test** in GitHub Actions workflow

```go
func TestGitHubHandler(t *testing.T) {
    // Set up mock environment
    os.Setenv("GITHUB_ACTIONS", "true")
    os.Setenv("GITHUB_OUTPUT", "/tmp/github_output")
    os.Setenv("GITHUB_STEP_SUMMARY", "/tmp/github_summary")

    // Run handler
    results := &TestResults{...}
    handler := &GitHubHandler{}
    err := handler.Handle(results)

    // Verify outputs
    assert.NoError(t, err)
    assert.FileContains(t, "/tmp/github_output", "passed=10")
}
```

## Migration Path

1. **v0.x.0**: Add `sky ci` plugin alongside existing `skytest action`
2. **v0.x.1**: Deprecate `skytest action`, update action.yml to use `sky ci`
3. **v0.x.2**: Remove `skytest action` from core

## Benefits

| Aspect         | Before (Core)      | After (Plugin)    |
| -------------- | ------------------ | ----------------- |
| Binary size    | +248 lines in core | Separate binary   |
| Extensibility  | Modify core        | Add plugin        |
| New CI systems | Fork/PR to core    | Community plugins |
| Updates        | skytest release    | Plugin release    |
| Philosophy     | Monolithic         | Plugin-first ✓    |

## Open Questions

1. **Distribution**: Should `sky-ci` be:
   - Separate binary in releases?
   - Embedded in `sky_full` build?
   - Installed via marketplace?

2. **JSON Schema**: Should we formalize the test results JSON schema?

3. **Streaming**: Should the plugin support streaming results (for long test runs)?

---

## Future Consideration: Reporter Plugin System (Option B)

> **Status**: Documented for future review. Current implementation uses Option D (pipeline pattern).

### Concept

Instead of piping JSON between commands, add native reporter plugin support to skytest:

```bash
# Single command with plugin reporter
skytest --reporter=plugin:ci-reporter .
skytest --reporter=plugin:my-custom-reporter --reporter-arg=format=html .
```

### How It Would Work

```
┌─────────────────────────────────────────────────────────────┐
│                         skytest                              │
│  ┌──────────┐    ┌──────────────┐    ┌──────────────────┐  │
│  │  Runner  │───►│ Reporter Mux │───►│ Plugin Reporter  │  │
│  └──────────┘    └──────────────┘    │ (subprocess)     │  │
│                         │            └────────┬─────────┘  │
│                         │                     │            │
│                         ▼                     ▼            │
│                  Built-in Reporters     Plugin stdin/stdout│
│                  (text, json, junit)                       │
└─────────────────────────────────────────────────────────────┘
```

### Implementation Sketch

```go
// In skytest reporter selection
var reporter tester.Reporter
if strings.HasPrefix(reporterFlag, "plugin:") {
    pluginName := strings.TrimPrefix(reporterFlag, "plugin:")
    reporter = &PluginReporter{
        Name: pluginName,
        Args: reporterArgs,
    }
} else {
    // Built-in reporters
    switch reporterFlag {
    case "json":
        reporter = &tester.JSONReporter{}
    // ...
    }
}

// PluginReporter wraps a plugin as a Reporter
type PluginReporter struct {
    Name string
    Args []string
}

func (p *PluginReporter) ReportFile(w io.Writer, result *FileResult) {
    // Stream file result to plugin stdin as JSON
    p.sendToPlugin(result)
}

func (p *PluginReporter) ReportSummary(w io.Writer, result *RunResult) {
    // Send final summary, collect plugin output
    output := p.finishPlugin(result)
    w.Write(output)
}
```

### Proposed CLI

```bash
# Use a plugin as reporter
skytest --reporter=plugin:ci-reporter .

# With arguments
skytest --reporter=plugin:html-reporter --reporter-arg=output=report.html .

# Multiple reporters (fan-out)
skytest --reporter=text --reporter=plugin:ci-reporter .
```

### Reporter Plugin Protocol

```go
// Plugin receives on stdin (streaming JSON Lines)
{"type": "file", "data": {...FileResult...}}
{"type": "file", "data": {...FileResult...}}
{"type": "summary", "data": {...RunResult...}}

// Plugin outputs to stdout (passed through to user)
// Plugin writes to files/external systems as needed
```

### Advantages Over Option D

| Aspect           | Option D (Pipeline)     | Option B (Reporter Plugin)   |
| ---------------- | ----------------------- | ---------------------------- |
| Commands         | 2 (`skytest \| sky ci`) | 1 (`skytest --reporter=...`) |
| Streaming        | Buffered                | Real-time                    |
| Multiple outputs | Complex                 | Native (`--reporter` x N)    |
| Plugin discovery | Manual                  | Via skytest                  |
| Error handling   | Exit codes              | Integrated                   |

### Disadvantages

| Concern       | Details                                          |
| ------------- | ------------------------------------------------ |
| Complexity    | Requires changes to skytest core reporter system |
| Protocol      | Need to design streaming JSON protocol           |
| Lifecycle     | Plugin process management during test run        |
| Compatibility | Breaking change to reporter interface            |

### When to Revisit

Consider implementing Option B when:

- Users request real-time streaming to CI systems
- Multiple simultaneous reporters become common
- Plugin ecosystem matures with many reporter plugins

### Migration Path

1. Implement Option D first (current plan)
2. Gather feedback from users
3. If demand exists, implement Option B
4. Option D plugins can be wrapped as Option B plugins

## Timeline

- [ ] Phase 1: Plugin structure (1 hour)
- [ ] Phase 2: GitHub handler (1 hour)
- [ ] Phase 3: JSON output verification (30 min)
- [ ] Phase 4: Update action.yml (30 min)
- [ ] Phase 5: Remove core code (15 min)
- [ ] Phase 6: Future CI systems (later)

**Total: ~3-4 hours**
