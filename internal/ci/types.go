package ci

import (
	"encoding/json"
	"time"
)

// TestResults mirrors the JSON emitted by `skytest -json`
// (see tester.JSONReporter in internal/starlark/tester/reporter.go).
type TestResults struct {
	Passed     int          `json:"passed"`
	Failed     int          `json:"failed"`
	Total      int          `json:"total"`
	Files      int          `json:"files"`
	DurationMS float64      `json:"duration_ms"`
	Results    []FileResult `json:"results"`

	// hasAggregate is set when the input carried the aggregate counts.
	hasAggregate bool
}

// UnmarshalJSON decodes the skytest JSON and records whether the aggregate
// counts (passed/failed/total) were present.
func (r *TestResults) UnmarshalJSON(data []byte) error {
	type plain TestResults
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return err
	}
	*r = TestResults(p)
	present := func(k string) bool {
		v, ok := keys[k]
		return ok && string(v) != "null"
	}
	r.hasAggregate = present("passed") && present("failed") && present("total")
	return nil
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

// Summary returns test counts. When the input carries skytest's aggregate
// counts they are authoritative: a skipped test is listed with passed=true in
// the per-test results but is excluded from the aggregate passed/failed/total,
// so skipped is the number of per-test passes beyond the aggregate. Without
// aggregate counts (hand-written input) the counts come from the tests.
func (r *TestResults) Summary() (passed, failed, skipped, total int) {
	var tPassed, tFailed int
	for _, f := range r.Results {
		for _, t := range f.Tests {
			if t.Passed {
				tPassed++
			} else {
				tFailed++
			}
		}
	}
	// Only trust the aggregate when it is complete and consistent with the
	// individual results; otherwise a partial or stale aggregate could hide
	// failures that the per-test entries show.
	if !r.hasAggregate || r.Failed != tFailed || r.Passed > tPassed || r.Total != r.Passed+r.Failed {
		return tPassed, tFailed, 0, tPassed + tFailed
	}
	passed, failed, total = r.Passed, r.Failed, r.Total
	if extra := tPassed - passed; extra > 0 {
		skipped = extra
	}
	return
}

// HasFailures returns true if any test failed.
func (r *TestResults) HasFailures() bool {
	_, failed, _, _ := r.Summary()
	return failed > 0 || r.Failed > 0
}
