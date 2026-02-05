package skytest

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/albertocavalcante/sky/internal/starlark/tester"
)

// ActionConfig holds configuration for the GitHub Action mode.
type ActionConfig struct {
	Path              string
	Recursive         bool
	Coverage          bool
	CoverageThreshold float64
	Annotations       bool
	Summary           bool
	FailFast          bool
	Filter            string
	Timeout           time.Duration
}

// RunAction executes skytest in GitHub Action mode.
// This is a cross-platform replacement for the shell script.
func RunAction(args []string, stdout, stderr io.Writer) int {
	cfg := ActionConfig{
		Path:        ".",
		Recursive:   true,
		Annotations: true,
		Summary:     true,
	}

	fs := flag.NewFlagSet("skytest action", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&cfg.Path, "path", cfg.Path, "path to test files")
	fs.BoolVar(&cfg.Recursive, "recursive", cfg.Recursive, "search directories recursively")
	fs.BoolVar(&cfg.Coverage, "coverage", cfg.Coverage, "enable coverage collection")
	fs.Float64Var(&cfg.CoverageThreshold, "coverage-threshold", 0, "minimum coverage percentage (0 to disable)")
	fs.BoolVar(&cfg.Annotations, "annotations", cfg.Annotations, "enable GitHub PR annotations")
	fs.BoolVar(&cfg.Summary, "summary", cfg.Summary, "write to job summary")
	fs.BoolVar(&cfg.FailFast, "fail-fast", cfg.FailFast, "stop on first test failure")
	fs.StringVar(&cfg.Filter, "filter", "", "filter tests by name pattern")
	fs.DurationVar(&cfg.Timeout, "timeout", 30*time.Second, "timeout per test")

	fs.Usage = func() {
		writeln(stderr, "Usage: skytest action [flags]")
		writeln(stderr)
		writeln(stderr, "Run tests in GitHub Action mode with native cross-platform support.")
		writeln(stderr)
		writeln(stderr, "This command:")
		writeln(stderr, "  - Outputs GitHub workflow commands for PR annotations")
		writeln(stderr, "  - Writes Markdown summary to $GITHUB_STEP_SUMMARY")
		writeln(stderr, "  - Writes test results to $GITHUB_OUTPUT")
		writeln(stderr, "  - Handles coverage threshold checking")
		writeln(stderr)
		writeln(stderr, "Flags:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return exitOK
		}
		return exitError
	}

	return runAction(cfg, stdout, stderr)
}

func runAction(cfg ActionConfig, stdout, stderr io.Writer) int {
	// Track overall exit code
	exitCode := exitOK

	// Discover test files
	files, err := tester.ExpandPaths([]string{cfg.Path}, nil, cfg.Recursive)
	if err != nil {
		writef(stderr, "skytest: %v\n", err)
		return exitError
	}

	if len(files) == 0 {
		writeln(stderr, "skytest: no test files found")
		return exitError
	}

	// Create test options
	opts := tester.DefaultOptions()
	opts.Verbose = false
	opts.Coverage = cfg.Coverage
	opts.Filter = cfg.Filter
	opts.Timeout = cfg.Timeout
	opts.FailFast = cfg.FailFast

	// Run tests and collect results
	result, err := runTestsForAction(files, opts, stdout, stderr, cfg.Annotations)
	if err != nil {
		writef(stderr, "skytest: %v\n", err)
		return exitError
	}

	// Get summary counts
	passed, failed, _ := result.Summary()

	// Check if tests failed
	if result.HasFailures() {
		exitCode = exitFailed
	}

	// Write Markdown summary to $GITHUB_STEP_SUMMARY
	if cfg.Summary {
		if err := writeGitHubSummary(result); err != nil {
			writef(stderr, "skytest: writing summary: %v\n", err)
			// Don't fail the run for summary errors
		}
	}

	// Handle coverage
	var coveragePercent float64
	if cfg.Coverage {
		// Note: Coverage collection requires starlark-go-x with OnExec hook
		// For now, we report 0% if not available
		coveragePercent = 0.0

		// Check threshold
		if cfg.CoverageThreshold > 0 && coveragePercent < cfg.CoverageThreshold {
			writef(stdout, "::error::Coverage %.1f%% is below threshold %.1f%%\n",
				coveragePercent, cfg.CoverageThreshold)
			exitCode = exitFailed
		}
	}

	// Write outputs to $GITHUB_OUTPUT
	if err := writeGitHubOutput(passed, failed, coveragePercent); err != nil {
		writef(stderr, "skytest: writing output: %v\n", err)
		// Don't fail the run for output errors
	}

	return exitCode
}

// runTestsForAction runs all test files and optionally outputs GitHub annotations.
func runTestsForAction(
	files []string,
	opts tester.Options,
	stdout, _ io.Writer,
	annotations bool,
) (*tester.RunResult, error) {
	result := &tester.RunResult{}
	start := time.Now()

	// Use GitHub reporter for annotations
	var reporter tester.Reporter
	if annotations {
		reporter = &tester.GitHubReporter{}
	} else {
		reporter = &tester.TextReporter{Verbose: false}
	}

	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("reading file %s: %w", file, err)
		}

		absPath, _ := filepath.Abs(file)
		if absPath == "" {
			absPath = file
		}

		runner := tester.New(opts)
		fileResult, err := runner.RunFile(absPath, src)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}

		result.Files = append(result.Files, *fileResult)

		// Report file immediately (outputs GitHub annotations)
		reporter.ReportFile(stdout, fileResult)

		// Fail-fast: stop processing more files after first failure
		if opts.FailFast && fileResult.HasFailures() {
			break
		}
	}

	result.Duration = time.Since(start)

	// Print summary line for text reporter
	if !annotations {
		reporter.ReportSummary(stdout, result)
	}

	return result, nil
}

// writeGitHubSummary writes the Markdown summary to $GITHUB_STEP_SUMMARY.
func writeGitHubSummary(result *tester.RunResult) error {
	summaryPath := os.Getenv("GITHUB_STEP_SUMMARY")
	if summaryPath == "" {
		return nil // Not running in GitHub Actions
	}

	// Open file for appending
	f, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening summary file: %w", err)
	}
	defer f.Close()

	// Use Markdown reporter
	reporter := &tester.MarkdownReporter{}
	reporter.ReportSummary(f, result)

	return nil
}

// writeGitHubOutput writes action outputs to $GITHUB_OUTPUT.
func writeGitHubOutput(passed, failed int, coverage float64) error {
	outputPath := os.Getenv("GITHUB_OUTPUT")
	if outputPath == "" {
		return nil // Not running in GitHub Actions
	}

	// Open file for appending
	f, err := os.OpenFile(outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening output file: %w", err)
	}
	defer f.Close()

	// Write outputs
	fmt.Fprintf(f, "passed=%d\n", passed)
	fmt.Fprintf(f, "failed=%d\n", failed)
	fmt.Fprintf(f, "coverage=%.1f\n", coverage)

	return nil
}

// isActionSubcommand checks if the first argument is "action".
func isActionSubcommand(args []string) bool {
	return len(args) > 0 && args[0] == "action"
}
