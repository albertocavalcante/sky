// Package ci provides CI system integrations for test result reporting.
//
// It auto-detects CI environments and outputs results in the appropriate format.
package ci

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
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
	fs.BoolVar(&cfg.Annotations, "annotations", true, "enable PR annotations")
	fs.BoolVar(&cfg.Summary, "summary", true, "write job summary")
	fs.BoolVar(&cfg.Quiet, "quiet", false, "suppress stdout output")

	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: sky ci [flags]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "CI reporter plugin for Sky. Reads JSON test results from stdin")
		fmt.Fprintln(stderr, "and outputs CI-specific formats (annotations, summaries, outputs).")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Auto-detects CI system from environment variables:")
		fmt.Fprintln(stderr, "  GitHub Actions:  GITHUB_ACTIONS=true")
		fmt.Fprintln(stderr, "  GitLab CI:       GITLAB_CI=true")
		fmt.Fprintln(stderr, "  CircleCI:        CIRCLECI=true")
		fmt.Fprintln(stderr, "  Azure DevOps:    TF_BUILD=True")
		fmt.Fprintln(stderr, "  Jenkins:         JENKINS_URL set")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Examples:")
		fmt.Fprintln(stderr, "  skytest -json . | sky ci")
		fmt.Fprintln(stderr, "  skytest -json . | sky ci --system=github")
		fmt.Fprintln(stderr, "  skytest -json . | sky ci --coverage-threshold=80")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Flags:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return exitOK
		}
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
		fmt.Fprintf(stderr, "sky-ci: reading input: %v\n", err)
		return exitError
	}

	// Get handler for the CI system
	handler := getHandler(cfg)

	// Process results
	if err := handler.Handle(results, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, "sky-ci: %v\n", err)
		return exitError
	}

	// Check for test failures
	if results.HasFailures() {
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
	var results TestResults
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&results); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}
	return &results, nil
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
