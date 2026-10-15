package linter

import (
	"fmt"
	"io"
	"path/filepath"

	sarif "github.com/albertocavalcante/sarif-go"
)

// SARIFReporter adapts lint results and source snapshots to SARIF 2.1.0.
type SARIFReporter struct {
	Root, Version string
	Rules         []*Rule
}

func (r *SARIFReporter) Report(w io.Writer, result *Result) error {
	root, err := filepath.Abs(r.Root)
	if err != nil {
		return err
	}
	uri, err := sarif.RootURI(root)
	if err != nil {
		return err
	}
	tool := sarif.Tool{Name: "skylint", Version: r.Version, RootURI: uri, InformationURI: "https://github.com/albertocavalcante/sky"}
	for _, rule := range r.Rules {
		tool.Rules = append(tool.Rules, sarif.Rule{ID: rule.Name, Description: rule.Doc, HelpURI: rule.URL})
	}
	var findings []sarif.Finding
	sources := map[string]*sarif.Source{}
	for _, f := range result.Findings {
		if sources[f.FilePath] == nil {
			content, ok := result.Sources[f.FilePath]
			if !ok {
				return fmt.Errorf("missing source snapshot for %s", f.FilePath)
			}
			source, err := sarif.NewSource(content)
			if err != nil {
				return err
			}
			sources[f.FilePath] = source
		}
		absolute, err := filepath.Abs(f.FilePath)
		if err != nil {
			return err
		}
		path, err := sarif.RelativePath(root, absolute)
		if err != nil {
			return err
		}
		region, err := sources[f.FilePath].RuneRegion(f.Line, max(f.Column, 1), f.EndLine, f.EndColumn)
		if err != nil {
			return err
		}
		level := "note"
		switch f.Severity {
		case SeverityError:
			level = "error"
		case SeverityWarning:
			level = "warning"
		}
		finding := sarif.Finding{Path: path, RuleID: f.Rule, Message: f.Message, Level: level, Line: region.Line, Column: region.Column, EndLine: region.EndLine, EndColumn: region.EndColumn}
		if f.Column == 0 {
			finding.Column = 0
			finding.EndLine = 0
			finding.EndColumn = 0
		}
		if finding.EndLine == finding.Line && finding.EndColumn == finding.Column {
			finding.EndLine = 0
			finding.EndColumn = 0
		}
		if f.Replacement != nil {
			edit := f.Replacement
			region, err := sources[f.FilePath].Region(edit.Start, edit.End)
			if err != nil {
				return err
			}
			finding.Fixes = []sarif.Fix{{Description: "Apply suggested fix", Edits: []sarif.Edit{{Path: path, Region: region, Text: edit.Content}}}}
		}
		findings = append(findings, finding)
	}
	var failures []string
	for _, failure := range result.Errors {
		message := failure.Err.Error()
		if failure.Path != "" {
			message = failure.Path + ": " + message
		}
		failures = append(failures, message)
	}
	report, err := sarif.Build(tool, findings, failures)
	if err != nil {
		return err
	}
	return sarif.Write(w, report)
}
