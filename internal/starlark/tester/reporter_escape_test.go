package tester

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestJSONReporterControlCharactersAreValidJSON(t *testing.T) {
	name := "test_\a\x00\x01_\"q\"\\   é <&>"
	msg := "fail: \a \x1b[31m \x00 \x7f\ttab\nnewline"
	file := "bel\a_test.star"
	result := &RunResult{Files: []FileResult{{
		File: file,
		Tests: []TestResult{
			{Name: name, Passed: false, Error: errors.New(msg)},
			{Name: "ok", Passed: true},
		},
	}}}

	var buf strings.Builder
	(&JSONReporter{}).ReportSummary(&buf, result)

	var got struct {
		Passed  int `json:"passed"`
		Failed  int `json:"failed"`
		Total   int `json:"total"`
		Files   int `json:"files"`
		Results []struct {
			File  string `json:"file"`
			Tests []struct {
				Name   string `json:"name"`
				Passed bool   `json:"passed"`
				Error  string `json:"error"`
			} `json:"tests"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(buf.String()), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if got.Passed != 1 || got.Failed != 1 || got.Total != 2 || got.Files != 1 {
		t.Errorf("counts = %+v", got)
	}
	if len(got.Results) != 1 || got.Results[0].File != file {
		t.Fatalf("results = %+v", got.Results)
	}
	tc := got.Results[0].Tests
	if len(tc) != 2 || tc[0].Name != name || tc[0].Error != msg || tc[1].Error != "" {
		t.Errorf("tests = %+v", tc)
	}
}

func TestJSONReporterEmptyResultsAreValidJSON(t *testing.T) {
	var buf strings.Builder
	(&JSONReporter{}).ReportSummary(&buf, &RunResult{Files: []FileResult{{File: "a.star"}}})
	var v map[string]any
	if err := json.Unmarshal([]byte(buf.String()), &v); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if _, ok := v["results"].([]any); !ok {
		t.Errorf("results is not an array: %v", v["results"])
	}
}

func TestGitHubReporterEscapesNamesAndPaths(t *testing.T) {
	evil := "case\n::error::injected"
	file := "dir/a\n::warning::file.star"
	fr := &FileResult{
		File: file,
		Tests: []TestResult{
			{Name: evil + "p", Passed: true},
			{Name: evil + "f", Passed: false, Error: errors.New("boom")},
			{Name: evil + "s", Passed: true, Skipped: true},
			{Name: evil + "x", Passed: false, XPass: true},
			{Name: evil + "xf", Passed: true, XFail: true},
		},
	}
	var buf strings.Builder
	if err := (&GitHubReporter{}).Report(&buf, fr); err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.HasPrefix(line, "::error::injected") || strings.HasPrefix(line, "::warning::file.star") {
			t.Errorf("injected workflow command at start of line: %q\nfull output:\n%s", line, buf.String())
		}
	}
	if !strings.Contains(buf.String(), "case%0A::error::injectedp") {
		t.Errorf("expected escaped name in status line:\n%s", buf.String())
	}
}
