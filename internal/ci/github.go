package ci

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// GitHubHandler outputs test results in GitHub Actions format.
type GitHubHandler struct {
	Config Config
}

// Handle processes test results for GitHub Actions.
func (h *GitHubHandler) Handle(results *TestResults, stdout, stderr io.Writer) error {
	// Output annotations for test failures
	if h.Config.Annotations {
		h.writeAnnotations(results, stdout)
	}

	// Write job summary to $GITHUB_STEP_SUMMARY
	if h.Config.Summary {
		if err := h.writeSummary(results); err != nil {
			printf(stderr, "sky-ci: warning: writing summary: %v\n", err)
		}
	}

	// Write outputs to $GITHUB_OUTPUT
	if err := h.writeOutputs(results); err != nil {
		printf(stderr, "sky-ci: warning: writing outputs: %v\n", err)
	}

	return nil
}

// writeAnnotations outputs GitHub workflow commands for PR annotations.
func (h *GitHubHandler) writeAnnotations(results *TestResults, w io.Writer) {
	for _, file := range results.Results {
		// Make path relative for cleaner annotations
		relPath := file.File
		if cwd, err := os.Getwd(); err == nil {
			if rel, err := filepath.Rel(cwd, file.File); err == nil {
				relPath = rel
			}
		}

		for _, test := range file.Tests {
			if test.Passed {
				continue
			}
			errMsg := test.Error
			if errMsg == "" {
				errMsg = "test failed"
			}
			printf(w, "::error file=%s::%s\n",
				EscapeProperty(relPath),
				EscapeData(test.Name+": "+errMsg))
		}
	}
}

// writeSummary writes Markdown summary to $GITHUB_STEP_SUMMARY.
func (h *GitHubHandler) writeSummary(results *TestResults) error {
	summaryPath := os.Getenv("GITHUB_STEP_SUMMARY")
	if summaryPath == "" {
		return nil // Not in GitHub Actions
	}

	f, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	passed, failed, skipped, total := results.Summary()

	// Header
	println(f, "## 🧪 Starlark Test Results")
	println(f)

	// Summary table
	println(f, "| Status | Count |")
	println(f, "|--------|-------|")
	printf(f, "| ✅ Passed | %d |\n", passed)
	printf(f, "| ❌ Failed | %d |\n", failed)
	if skipped > 0 {
		printf(f, "| ⏭️ Skipped | %d |\n", skipped)
	}
	printf(f, "| **Total** | **%d** |\n", total)
	println(f)

	// Duration
	if results.DurationMS > 0 {
		printf(f, "⏱️ Duration: %s\n", results.Duration())
		println(f)
	}

	// Failed tests details
	if failed > 0 {
		println(f, "<details>")
		println(f, "<summary>❌ Failed Tests</summary>")
		println(f)
		println(f, "```")
		for _, file := range results.Results {
			for _, test := range file.Tests {
				if !test.Passed {
					printf(f, "%s::%s\n", filepath.Base(file.File), test.Name)
					if test.Error != "" {
						printf(f, "  %s\n", test.Error)
					}
				}
			}
		}
		println(f, "```")
		println(f, "</details>")
	}

	return nil
}

// writeOutputs writes action outputs to $GITHUB_OUTPUT.
func (h *GitHubHandler) writeOutputs(results *TestResults) error {
	outputPath := os.Getenv("GITHUB_OUTPUT")
	if outputPath == "" {
		return nil // Not in GitHub Actions
	}

	f, err := os.OpenFile(outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	passed, failed, _, _ := results.Summary()

	// GITHUB_OUTPUT is opened for the lifetime of the action and
	// writes can't be meaningfully recovered from here, so explicitly
	// discard the (n, err) returns.
	printf(f, "passed=%d\n", passed)
	printf(f, "failed=%d\n", failed)
	printf(f, "coverage=%.1f\n", h.Config.Coverage)

	return nil
}

// EscapeData escapes a workflow command message.
func EscapeData(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "\r", "%0D")
	s = strings.ReplaceAll(s, "\n", "%0A")
	return s
}

// EscapeProperty escapes a workflow command property value.
func EscapeProperty(s string) string {
	s = EscapeData(s)
	s = strings.ReplaceAll(s, ":", "%3A")
	s = strings.ReplaceAll(s, ",", "%2C")
	return s
}
