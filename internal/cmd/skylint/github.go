package skylint

import (
	"fmt"
	"io"

	"github.com/albertocavalcante/sky/internal/ci"
	"github.com/albertocavalcante/sky/internal/starlark/linter"
	"github.com/albertocavalcante/sky/internal/starlark/sortutil"
)

// githubReporter prints GitHub Actions workflow commands with the same
// property/data escaping as internal/ci, so file names, rule names and
// messages cannot inject extra workflow commands.
type githubReporter struct{}

func (githubReporter) Report(w io.Writer, result *linter.Result) error {
	findings := append([]linter.Finding(nil), result.Findings...)
	sortutil.ByLocation(findings,
		func(f linter.Finding) string { return f.FilePath },
		func(f linter.Finding) int { return f.Line },
		func(f linter.Finding) int { return f.Column },
	)
	for _, f := range findings {
		level := "notice"
		switch f.Severity {
		case linter.SeverityError:
			level = "error"
		case linter.SeverityWarning:
			level = "warning"
		}
		title := fmt.Sprintf("%s (%s)", f.Rule, f.Category)
		if f.Rule == "" {
			title = f.Category
		}
		if title == "" {
			title = "lint"
		}
		loc := fmt.Sprintf("file=%s,line=%d", ci.EscapeProperty(f.FilePath), f.Line)
		if f.Column > 0 {
			loc += fmt.Sprintf(",col=%d", f.Column)
		}
		if f.EndLine > 0 && f.EndLine != f.Line {
			loc += fmt.Sprintf(",endLine=%d", f.EndLine)
		}
		if f.EndColumn > 0 && f.EndColumn != f.Column {
			loc += fmt.Sprintf(",endColumn=%d", f.EndColumn)
		}
		if _, err := fmt.Fprintf(w, "::%s %s,title=%s::%s\n", level, loc, ci.EscapeProperty(title), ci.EscapeData(f.Message)); err != nil {
			return err
		}
	}
	for _, e := range result.Errors {
		if _, err := fmt.Fprintf(w, "::error file=%s::%s\n", ci.EscapeProperty(e.Path),
			ci.EscapeData(fmt.Sprintf("Failed to process file: %v", e.Err))); err != nil {
			return err
		}
	}
	return nil
}
