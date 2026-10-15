package skycheck

import (
	"io"
	"path/filepath"

	sarif "github.com/albertocavalcante/sarif-go"
	"github.com/albertocavalcante/sky/internal/starlark/checker"
	"github.com/albertocavalcante/sky/internal/version"
)

func outputSARIF(w, stderr io.Writer, root string, result checker.Result, contents map[string][]byte, failures []string) int {
	abs, err := filepath.Abs(root)
	if err != nil {
		writef(stderr, "skycheck: %v\n", err)
		return exitError
	}
	uri, err := sarif.RootURI(abs)
	if err != nil {
		writef(stderr, "skycheck: %v\n", err)
		return exitError
	}
	var findings []sarif.Finding
	sources := map[string]*sarif.Source{}
	for _, d := range result.Diagnostics {
		file := d.Pos.Filename()
		if sources[file] == nil {
			source, err := sarif.NewSource(contents[file])
			if err != nil {
				failures = append(failures, err.Error())
				continue
			}
			sources[file] = source
		}
		absolute, err := filepath.Abs(file)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		path, err := sarif.RelativePath(abs, absolute)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		region, err := sources[file].RuneRegion(int(d.Pos.Line), max(int(d.Pos.Col), 1), int(d.End.Line), int(d.End.Col))
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		level := d.Severity.String()
		if level == "info" {
			level = "note"
		}
		f := sarif.Finding{Path: path, RuleID: d.Code, Message: d.Message, Level: level, Line: region.Line, Column: region.Column, EndLine: region.EndLine, EndColumn: region.EndColumn}
		if f.EndLine == f.Line && f.EndColumn == f.Column {
			f.EndLine = 0
			f.EndColumn = 0
		}
		findings = append(findings, f)
	}
	rules := []sarif.Rule{{ID: "parse-error", Description: "Starlark syntax error"}, {ID: "undefined", Description: "Reference to an undefined name"}, {ID: "unused", Description: "Unused local binding"}}
	for i := range rules {
		rules[i].HelpURI = "https://albertocavalcante.github.io/sky/tools/skycheck/"
	}
	report, err := sarif.Build(sarif.Tool{Name: "skycheck", Version: version.String(), RootURI: uri, InformationURI: "https://github.com/albertocavalcante/sky", Rules: rules}, findings, failures)
	if err == nil {
		err = sarif.Write(w, report)
	}
	if err != nil {
		writef(stderr, "skycheck: SARIF: %v\n", err)
		return exitError
	}
	if len(failures) > 0 || result.HasErrors() {
		return exitError
	}
	if result.WarningCount() > 0 {
		return exitWarning
	}
	return exitOK
}
