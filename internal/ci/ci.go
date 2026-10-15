// Package ci provides CI system integrations for test result reporting.
//
// It auto-detects CI environments and outputs results in the appropriate format.
package ci

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/albertocavalcante/sky/internal/skyconfig"
)

// Exit codes
const (
	exitOK     = 0
	exitFailed = 1
	exitError  = 2
)

// System represents a supported CI system.
type System string

const (
	SystemGitHub  System = "github"
	SystemGitLab  System = "gitlab"
	SystemCircle  System = "circleci"
	SystemAzure   System = "azure"
	SystemJenkins System = "jenkins"
	SystemGeneric System = "generic"
)

// Handler processes test results for a specific CI system.
type Handler interface {
	// Handle processes test results and outputs CI-specific formats.
	Handle(results *TestResults, stdout, stderr io.Writer) error
}

// Config holds configuration for the CI reporter.
type Config struct {
	System            System
	CoverageThreshold float64
	CoverageFile      string
	Coverage          float64 // measured coverage percentage, if available
	coveredLines      int     // exact counts behind Coverage, for the threshold comparison
	totalLines        int
	Annotations       bool
	Summary           bool
	Quiet             bool
}

// Run executes the CI reporter with the given arguments.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg := Config{
		Annotations: true,
		Summary:     true,
	}

	var systemFlag string

	fs := flag.NewFlagSet("sky-ci", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&systemFlag, "system", "", "CI system (github, gitlab, circleci, azure, generic); auto-detected if not set")
	fs.Float64Var(&cfg.CoverageThreshold, "coverage-threshold", 0, "fail if coverage below threshold (0 to disable)")
	fs.StringVar(&cfg.CoverageFile, "coverage-file", "coverage.json", "skytest coverage JSON file used for --coverage-threshold")
	fs.BoolVar(&cfg.Annotations, "annotations", true, "enable PR annotations")
	fs.BoolVar(&cfg.Summary, "summary", true, "write job summary")
	fs.BoolVar(&cfg.Quiet, "quiet", false, "suppress stdout output")

	fs.Usage = func() {
		println(stderr, "Usage: sky ci [flags]")
		println(stderr)
		println(stderr, "CI reporter plugin for Sky. Reads JSON test results from stdin")
		println(stderr, "and outputs CI-specific formats (annotations, summaries, outputs).")
		println(stderr)
		println(stderr, "Auto-detects CI system from environment variables:")
		println(stderr, "  GitHub Actions:  GITHUB_ACTIONS=true")
		println(stderr, "  GitLab CI:       GITLAB_CI=true")
		println(stderr, "  CircleCI:        CIRCLECI=true")
		println(stderr, "  Azure DevOps:    TF_BUILD=True")
		println(stderr, "  Jenkins:         JENKINS_URL set")
		println(stderr)
		println(stderr, "Examples:")
		println(stderr, "  skytest -json . | sky ci")
		println(stderr, "  skytest -json . | sky ci --system=github")
		println(stderr, "  skytest -json . | sky ci --coverage-threshold=80")
		println(stderr)
		println(stderr, "Flags:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return exitOK
		}
		return exitError
	}

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if err := skyconfig.ValidateCoverageThreshold(cfg.CoverageThreshold); err != nil {
		printf(stderr, "sky-ci: --coverage-threshold: %v\n", err)
		return exitError
	}

	// Detect or validate CI system
	if systemFlag != "" {
		cfg.System = System(systemFlag)
	} else {
		cfg.System = detectSystem()
	}

	// Read JSON from stdin
	results, err := readResults(stdin)
	if err != nil {
		printf(stderr, "sky-ci: reading input: %v\n", err)
		return exitError
	}

	// Config file provides defaults for the threshold (coverage.fail_under)
	// and coverage file (coverage.output); explicit flags win, including an
	// explicit --coverage-threshold=0 which disables the check.
	if !set["coverage-threshold"] || !set["coverage-file"] {
		if skycfg, _, err := skyconfig.DiscoverConfig(""); err != nil {
			if !set["coverage-threshold"] {
				// The config would have supplied the threshold; do not silently
				// skip enforcement because it could not be loaded.
				printf(stderr, "sky-ci: loading config: %v\n", err)
				return exitError
			}
			printf(stderr, "sky-ci: warning: loading config: %v\n", err)
		} else if skycfg != nil {
			if !set["coverage-threshold"] {
				cfg.CoverageThreshold = skycfg.Test.Coverage.FailUnder
			}
			if !set["coverage-file"] && skycfg.Test.Coverage.Output != "" {
				cfg.CoverageFile = skycfg.Test.Coverage.Output
			}
		}
	}

	// Load coverage (required when a threshold is set, optional otherwise)
	coverageKnown := false
	if cov, covered, total, err := readCoverage(cfg.CoverageFile); err == nil {
		cfg.Coverage = cov
		cfg.coveredLines, cfg.totalLines = covered, total
		coverageKnown = true
	} else if cfg.CoverageThreshold > 0 {
		printf(stderr, "sky-ci: --coverage-threshold set but coverage unavailable: %v\n", err)
		return exitError
	}

	// Get handler for the CI system
	handler := getHandler(cfg)

	// Process results
	if err := handler.Handle(results, stdout, stderr); err != nil {
		printf(stderr, "sky-ci: %v\n", err)
		return exitError
	}

	// Check for test failures
	if results.HasFailures() {
		return exitFailed
	}

	if cfg.CoverageThreshold > 0 && coverageKnown && !skyconfig.CoverageMeets(cfg.coveredLines, cfg.totalLines, cfg.CoverageThreshold) {
		printf(stderr, "sky-ci: coverage %.1f%% is below threshold %.1f%%\n", cfg.Coverage, cfg.CoverageThreshold)
		return exitFailed
	}

	return exitOK
}

// detectSystem auto-detects the CI system from environment variables.
func detectSystem() System {
	switch {
	case os.Getenv("GITHUB_ACTIONS") == "true":
		return SystemGitHub
	case os.Getenv("GITLAB_CI") == "true":
		return SystemGitLab
	case os.Getenv("CIRCLECI") == "true":
		return SystemCircle
	case os.Getenv("TF_BUILD") == "True":
		return SystemAzure
	case os.Getenv("JENKINS_URL") != "":
		return SystemJenkins
	default:
		return SystemGeneric
	}
}

// readResults reads and parses JSON test results from stdin.
func readResults(r io.Reader) (*TestResults, error) {
	decoder := json.NewDecoder(r)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}
	// Exactly one report: anything after it (a second report, garbage) could
	// hide failures, so reject it rather than ignore it.
	// Scan the remainder in fixed-size chunks: stop at the first
	// non-whitespace byte and never buffer the stream (it may be endless).
	if err := rejectTrailingData(io.MultiReader(decoder.Buffered(), r)); err != nil {
		return nil, err
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		return nil, fmt.Errorf("parsing JSON: report must be an object: %w", err)
	}
	if v, ok := keys["results"]; !ok || string(v) == "null" {
		return nil, fmt.Errorf("invalid report: missing \"results\" array (not skytest -json output?)")
	}
	var results TestResults
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}
	return &results, nil
}

// rejectTrailingData returns an error at the first byte of r that is not JSON
// whitespace, reading at most one chunk beyond it. Linear in the whitespace.
func rejectTrailingData(r io.Reader) error {
	var buf [4096]byte
	for {
		n, err := r.Read(buf[:])
		if len(bytes.Trim(buf[:n], " \t\r\n")) != 0 {
			return fmt.Errorf("unexpected data after the JSON report; expected exactly one skytest -json report")
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
	}
}

// getHandler returns the appropriate handler for the CI system.
func getHandler(cfg Config) Handler {
	switch cfg.System {
	case SystemGitHub:
		return &GitHubHandler{Config: cfg}
	case SystemGitLab:
		return &GenericHandler{Config: cfg, Name: "GitLab CI"}
	case SystemCircle:
		return &GenericHandler{Config: cfg, Name: "CircleCI"}
	case SystemAzure:
		return &GenericHandler{Config: cfg, Name: "Azure DevOps"}
	case SystemJenkins:
		return &GenericHandler{Config: cfg, Name: "Jenkins"}
	default:
		return &GenericHandler{Config: cfg, Name: "Generic"}
	}
}

// readCoverage reads the total percentage from a skytest coverage JSON file.
// It fails closed: a report without any measured lines (missing, empty or
// zero-line data, which Report.Percentage would otherwise render as 100%) or
// with inconsistent numbers is an error, never a passing coverage value.
func readCoverage(path string) (pct float64, covered, total int, err error) {
	if path == "" {
		return 0, 0, 0, fmt.Errorf("no coverage file specified")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, 0, err
	}
	var c struct {
		TotalLines   int      `json:"total_lines"`
		CoveredLines int      `json:"covered_lines"`
		Percentage   *float64 `json:"percentage"`
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return 0, 0, 0, fmt.Errorf("parsing %s: %w", path, err)
	}
	if c.TotalLines <= 0 {
		return 0, 0, 0, fmt.Errorf("no coverage data in %s (total_lines is 0; coverage collection may not be available)", path)
	}
	if c.CoveredLines < 0 || c.CoveredLines > c.TotalLines {
		return 0, 0, 0, fmt.Errorf("inconsistent coverage data in %s: %d of %d lines covered", path, c.CoveredLines, c.TotalLines)
	}
	if c.Percentage == nil {
		return 0, 0, 0, fmt.Errorf("%s has no percentage field", path)
	}
	pct = float64(c.CoveredLines) / float64(c.TotalLines) * 100
	if math.IsNaN(*c.Percentage) || math.Abs(*c.Percentage-pct) > 0.5 {
		return 0, 0, 0, fmt.Errorf("inconsistent coverage data in %s: percentage %v does not match %d/%d lines", path, *c.Percentage, c.CoveredLines, c.TotalLines)
	}
	return pct, c.CoveredLines, c.TotalLines, nil
}
