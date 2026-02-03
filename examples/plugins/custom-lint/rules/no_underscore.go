package rules

import (
	"fmt"
	"strings"

	"github.com/bazelbuild/buildtools/build"
)

// NoUnderscore flags public functions that start with underscore.
var NoUnderscore = Rule{
	Name:        "no-underscore-public",
	Description: "Public functions should not start with underscore",
	Check:       checkNoUnderscore,
}

func checkNoUnderscore(file *build.File, path string) []Finding {
	var findings []Finding

	for _, stmt := range file.Stmt {
		def, ok := stmt.(*build.DefStmt)
		if !ok {
			continue
		}

		// Skip truly private functions (start with _)
		// This rule flags functions that look like they should be private
		// but are not prefixed consistently

		// We flag functions that start with underscore but are documented
		// (suggesting they're meant to be part of the API)
		if strings.HasPrefix(def.Name, "_") && !strings.HasPrefix(def.Name, "__") {
			// Check if the function has a docstring
			if hasDocstring(def) {
				start, _ := def.Span()
				findings = append(findings, Finding{
					File:   path,
					Line:   start.Line,
					Column: start.LineRune,
					Rule:   "no-underscore-public",
					Message: fmt.Sprintf(
						"function %q starts with underscore but has docstring; "+
							"remove underscore for public API or remove docstring for private function",
						def.Name,
					),
				})
			}
		}
	}

	return findings
}

func hasDocstring(def *build.DefStmt) bool {
	if len(def.Body) == 0 {
		return false
	}

	// In buildtools, the first statement in a function body
	// can be a StringExpr directly if it's a docstring
	first := def.Body[0]
	str, ok := first.(*build.StringExpr)
	if !ok {
		return false
	}

	// Check if it's a triple-quoted string (typical docstring)
	return str.TripleQuote
}
