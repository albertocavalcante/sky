package tester

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
)

// Reporter formats test results for output.
type Reporter interface {
	// ReportFile reports results for a single file.
	ReportFile(w io.Writer, result *FileResult)

	// ReportSummary reports the final summary.
	ReportSummary(w io.Writer, result *RunResult)
}

// TextReporter outputs results in human-readable text format.
type TextReporter struct {
	// Verbose enables detailed output.
	Verbose bool

	// ShowDuration shows timing information.
	ShowDuration bool
}

// ReportFile implements Reporter.
func (r *TextReporter) ReportFile(w io.Writer, result *FileResult) {
	if result.SetupError != nil {
		_, _ = fmt.Fprintf(w, "SETUP FAILED: %s\n  %v\n", result.File, result.SetupError)
		return
	}

	for _, t := range result.Tests {
		status := "PASS"
		if !t.Passed {
			status = "FAIL"
		}

		if r.ShowDuration {
			_, _ = fmt.Fprintf(w, "%s  %s  (%s)\n", status, t.Name, t.Duration.Round(time.Millisecond))
		} else {
			_, _ = fmt.Fprintf(w, "%s  %s\n", status, t.Name)
		}

		if !t.Passed && t.Error != nil {
			// Indent error message
			errStr := t.Error.Error()
			for _, line := range strings.Split(errStr, "\n") {
				_, _ = fmt.Fprintf(w, "      %s\n", line)
			}
		}

		if r.Verbose && t.Output != "" {
			_, _ = fmt.Fprintf(w, "      Output:\n")
			for _, line := range strings.Split(t.Output, "\n") {
				_, _ = fmt.Fprintf(w, "        %s\n", line)
			}
		}
	}

	if result.TeardownError != nil {
		_, _ = fmt.Fprintf(w, "TEARDOWN FAILED: %s\n  %v\n", result.File, result.TeardownError)
	}
}

// ReportSummary implements Reporter.
func (r *TextReporter) ReportSummary(w io.Writer, result *RunResult) {
	passed, failed, files := result.Summary()
	total := passed + failed

	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "Results: %d passed, %d failed, %d total in %d file(s)\n",
		passed, failed, total, files)

	if r.ShowDuration {
		_, _ = fmt.Fprintf(w, "Duration: %s\n", result.Duration.Round(time.Millisecond))
	}
}

// JUnitReporter outputs results in JUnit XML format.
type JUnitReporter struct{}

// JUnit XML structures
type junitTestSuites struct {
	XMLName xml.Name         `xml:"testsuites"`
	Suites  []junitTestSuite `xml:"testsuite"`
	Tests   int              `xml:"tests,attr"`
	Errors  int              `xml:"errors,attr"`
	Time    float64          `xml:"time,attr"`
}

type junitTestSuite struct {
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	Errors    int             `xml:"errors,attr"`
	Time      float64         `xml:"time,attr"`
	TestCases []junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Time      float64       `xml:"time,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
	Error     *junitError   `xml:"error,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Content string `xml:",chardata"`
}

type junitError struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Content string `xml:",chardata"`
}

// ReportFile implements Reporter (no-op for JUnit, all output in summary).
func (r *JUnitReporter) ReportFile(w io.Writer, result *FileResult) {
	// JUnit outputs everything in summary
}

// ReportSummary implements Reporter.
func (r *JUnitReporter) ReportSummary(w io.Writer, result *RunResult) {
	suites := junitTestSuites{
		Time: result.Duration.Seconds(),
	}

	for _, fr := range result.Files {
		suite := junitTestSuite{
			Name:  fr.File,
			Tests: len(fr.Tests),
			Time:  fr.Duration.Seconds(),
		}

		for _, t := range fr.Tests {
			tc := junitTestCase{
				Name:      t.Name,
				ClassName: fr.File,
				Time:      t.Duration.Seconds(),
			}

			if !t.Passed && t.Error != nil {
				suite.Failures++
				tc.Failure = &junitFailure{
					Message: t.Error.Error(),
					Type:    "AssertionError",
					Content: t.Error.Error(),
				}
			}

			suite.TestCases = append(suite.TestCases, tc)
		}

		// Handle setup/teardown errors
		if fr.SetupError != nil {
			suite.Errors++
			suite.TestCases = append(suite.TestCases, junitTestCase{
				Name:      "setup",
				ClassName: fr.File,
				Error: &junitError{
					Message: fr.SetupError.Error(),
					Type:    "SetupError",
					Content: fr.SetupError.Error(),
				},
			})
		}
		if fr.TeardownError != nil {
			suite.Errors++
			suite.TestCases = append(suite.TestCases, junitTestCase{
				Name:      "teardown",
				ClassName: fr.File,
				Error: &junitError{
					Message: fr.TeardownError.Error(),
					Type:    "TeardownError",
					Content: fr.TeardownError.Error(),
				},
			})
		}

		suites.Suites = append(suites.Suites, suite)
		suites.Tests += suite.Tests
		suites.Errors += suite.Errors
	}

	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	_, _ = fmt.Fprint(w, xml.Header)
	_ = enc.Encode(suites)
	_, _ = fmt.Fprintln(w)
}

// JSONReporter outputs results in JSON format.
type JSONReporter struct{}

// ReportFile implements Reporter (no-op for JSON, all output in summary).
func (r *JSONReporter) ReportFile(w io.Writer, result *FileResult) {
	// JSON outputs everything in summary
}

// ReportSummary implements Reporter.
func (r *JSONReporter) ReportSummary(w io.Writer, result *RunResult) {
	passed, failed, files := result.Summary()

	type jsonTest struct {
		Name     string  `json:"name"`
		Passed   bool    `json:"passed"`
		Duration float64 `json:"duration_ms"`
		Error    string  `json:"error,omitempty"`
	}

	type jsonFile struct {
		File     string     `json:"file"`
		Tests    []jsonTest `json:"tests"`
		Duration float64    `json:"duration_ms"`
	}

	type jsonOutput struct {
		Passed   int        `json:"passed"`
		Failed   int        `json:"failed"`
		Total    int        `json:"total"`
		Files    int        `json:"files"`
		Duration float64    `json:"duration_ms"`
		Results  []jsonFile `json:"results"`
	}

	out := jsonOutput{
		Passed:   passed,
		Failed:   failed,
		Total:    passed + failed,
		Files:    files,
		Duration: float64(result.Duration.Milliseconds()),
	}

	for _, fr := range result.Files {
		jf := jsonFile{
			File:     fr.File,
			Duration: float64(fr.Duration.Milliseconds()),
		}
		for _, t := range fr.Tests {
			jt := jsonTest{
				Name:     t.Name,
				Passed:   t.Passed,
				Duration: float64(t.Duration.Milliseconds()),
			}
			if t.Error != nil {
				jt.Error = t.Error.Error()
			}
			jf.Tests = append(jf.Tests, jt)
		}
		out.Results = append(out.Results, jf)
	}

	// Manual JSON encoding to avoid importing encoding/json for simple output
	_, _ = fmt.Fprintf(w, "{\n")
	_, _ = fmt.Fprintf(w, "  \"passed\": %d,\n", out.Passed)
	_, _ = fmt.Fprintf(w, "  \"failed\": %d,\n", out.Failed)
	_, _ = fmt.Fprintf(w, "  \"total\": %d,\n", out.Total)
	_, _ = fmt.Fprintf(w, "  \"files\": %d,\n", out.Files)
	_, _ = fmt.Fprintf(w, "  \"duration_ms\": %.0f,\n", out.Duration)
	_, _ = fmt.Fprintf(w, "  \"results\": [\n")

	for i, jf := range out.Results {
		_, _ = fmt.Fprintf(w, "    {\n")
		_, _ = fmt.Fprintf(w, "      \"file\": %q,\n", jf.File)
		_, _ = fmt.Fprintf(w, "      \"duration_ms\": %.0f,\n", jf.Duration)
		_, _ = fmt.Fprintf(w, "      \"tests\": [\n")

		for j, jt := range jf.Tests {
			_, _ = fmt.Fprintf(w, "        {\n")
			_, _ = fmt.Fprintf(w, "          \"name\": %q,\n", jt.Name)
			_, _ = fmt.Fprintf(w, "          \"passed\": %t,\n", jt.Passed)
			_, _ = fmt.Fprintf(w, "          \"duration_ms\": %.0f", jt.Duration)
			if jt.Error != "" {
				_, _ = fmt.Fprintf(w, ",\n          \"error\": %q\n", jt.Error)
			} else {
				_, _ = fmt.Fprintf(w, "\n")
			}
			if j < len(jf.Tests)-1 {
				_, _ = fmt.Fprintf(w, "        },\n")
			} else {
				_, _ = fmt.Fprintf(w, "        }\n")
			}
		}

		_, _ = fmt.Fprintf(w, "      ]\n")
		if i < len(out.Results)-1 {
			_, _ = fmt.Fprintf(w, "    },\n")
		} else {
			_, _ = fmt.Fprintf(w, "    }\n")
		}
	}

	_, _ = fmt.Fprintf(w, "  ]\n")
	_, _ = fmt.Fprintf(w, "}\n")
}
