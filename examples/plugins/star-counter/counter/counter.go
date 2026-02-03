// Package counter provides Starlark file analysis functionality.
package counter

import (
	"os"

	"github.com/bazelbuild/buildtools/build"
)

// FileStats holds statistics about a Starlark file.
type FileStats struct {
	Path    string `json:"path"`
	Defs    int    `json:"defs"`    // Function definitions
	Loads   int    `json:"loads"`   // Load statements
	Calls   int    `json:"calls"`   // Function calls
	Assigns int    `json:"assigns"` // Assignments
	Lines   int    `json:"lines"`   // Total lines
}

// Analyze parses a Starlark file and returns statistics.
func Analyze(path string) (FileStats, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return FileStats{}, err
	}

	stats := FileStats{
		Path:  path,
		Lines: countLines(content),
	}

	file, err := build.ParseDefault(path, content)
	if err != nil {
		return FileStats{}, err
	}

	// Count different statement types
	for _, stmt := range file.Stmt {
		switch stmt.(type) {
		case *build.DefStmt:
			stats.Defs++
		case *build.LoadStmt:
			stats.Loads++
		case *build.AssignExpr:
			stats.Assigns++
		}
	}

	// Walk the AST to count function calls
	build.Walk(file, func(expr build.Expr, stack []build.Expr) {
		if _, ok := expr.(*build.CallExpr); ok {
			stats.Calls++
		}
	})

	return stats, nil
}

func countLines(content []byte) int {
	if len(content) == 0 {
		return 0
	}
	lines := 1
	for _, b := range content {
		if b == '\n' {
			lines++
		}
	}
	return lines
}
