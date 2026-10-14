package ci

import "time"

// TestResults mirrors the JSON emitted by `skytest -json`
// (see tester.JSONReporter in internal/starlark/tester/reporter.go).
type TestResults struct {
	Passed     int          `json:"passed"`
	Failed     int          `json:"failed"`
	Total      int          `json:"total"`
	Files      int          `json:"files"`
	DurationMS float64      `json:"duration_ms"`
	Results    []FileResult `json:"results"`
}

// FileResult represents test results for a single file.
type FileResult struct {
	File       string       `json:"file"`
	Tests      []TestResult `json:"tests"`
	DurationMS float64      `json:"duration_ms"`
}

// TestResult represents a single test result.
type TestResult struct {
	Name       string  `json:"name"`
	Passed     bool    `json:"passed"`
	DurationMS float64 `json:"duration_ms"`
	Error      string  `json:"error,omitempty"`
}

// Duration returns the total run duration.
func (r *TestResults) Duration() time.Duration {
	return time.Duration(r.DurationMS * float64(time.Millisecond))
}

// Summary computes test summary statistics from the individual tests.
// skytest -json does not report skipped tests, so skipped is always 0.
func (r *TestResults) Summary() (passed, failed, skipped, total int) {
	for _, f := range r.Results {
		for _, t := range f.Tests {
			total++
			if t.Passed {
				passed++
			} else {
				failed++
			}
		}
	}
	return
}

// HasFailures returns true if any test failed.
func (r *TestResults) HasFailures() bool {
	_, failed, _, _ := r.Summary()
	return failed > 0 || r.Failed > 0
}
