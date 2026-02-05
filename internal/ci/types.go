package ci

// TestResults represents the JSON output from skytest -json.
type TestResults struct {
	Files    []FileResult `json:"files"`
	Duration string       `json:"duration"`
}

// FileResult represents test results for a single file.
type FileResult struct {
	Path   string       `json:"path"`
	Tests  []TestResult `json:"tests"`
	Passed bool         `json:"passed"`
}

// TestResult represents a single test result.
type TestResult struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Skipped  bool   `json:"skipped"`
	Duration string `json:"duration"`
	Error    string `json:"error,omitempty"`
	Line     int    `json:"line,omitempty"`
	Output   string `json:"output,omitempty"`
}

// Summary computes test summary statistics.
func (r *TestResults) Summary() (passed, failed, skipped, total int) {
	for _, f := range r.Files {
		for _, t := range f.Tests {
			total++
			switch {
			case t.Skipped:
				skipped++
			case t.Passed:
				passed++
			default:
				failed++
			}
		}
	}
	return
}

// HasFailures returns true if any test failed.
func (r *TestResults) HasFailures() bool {
	_, failed, _, _ := r.Summary()
	return failed > 0
}
