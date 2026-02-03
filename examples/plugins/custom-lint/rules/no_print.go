package rules

import (
	"github.com/bazelbuild/buildtools/build"
)

// NoPrint flags uses of the print() function.
var NoPrint = Rule{
	Name:        "no-print",
	Description: "Disallow print() statements in production code",
	Check:       checkNoPrint,
}

func checkNoPrint(file *build.File, path string) []Finding {
	var findings []Finding

	build.Walk(file, func(expr build.Expr, stack []build.Expr) {
		call, ok := expr.(*build.CallExpr)
		if !ok {
			return
		}

		ident, ok := call.X.(*build.Ident)
		if !ok {
			return
		}

		if ident.Name == "print" {
			start, _ := call.Span()
			findings = append(findings, Finding{
				File:    path,
				Line:    start.Line,
				Column:  start.LineRune,
				Rule:    "no-print",
				Message: "print() should not be used in production code; use a proper logging mechanism",
			})
		}
	})

	return findings
}
