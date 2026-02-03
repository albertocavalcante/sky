package rules

import (
	"fmt"

	"github.com/bazelbuild/buildtools/build"
)

const maxParameters = 5

// MaxParams flags functions with too many parameters.
var MaxParams = Rule{
	Name:        "max-params",
	Description: fmt.Sprintf("Functions should have at most %d parameters", maxParameters),
	Check:       checkMaxParams,
}

func checkMaxParams(file *build.File, path string) []Finding {
	var findings []Finding

	for _, stmt := range file.Stmt {
		def, ok := stmt.(*build.DefStmt)
		if !ok {
			continue
		}

		paramCount := len(def.Params)
		if paramCount > maxParameters {
			start, _ := def.Span()
			findings = append(findings, Finding{
				File:   path,
				Line:   start.Line,
				Column: start.LineRune,
				Rule:   "max-params",
				Message: fmt.Sprintf(
					"function %q has %d parameters; consider refactoring (max: %d)",
					def.Name, paramCount, maxParameters,
				),
			})
		}
	}

	return findings
}
