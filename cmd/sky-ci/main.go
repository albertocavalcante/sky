// Command sky-ci is a CI reporter plugin for Sky.
//
// It reads JSON test results from stdin and outputs CI-specific formats.
// Auto-detects the CI system from environment variables.
//
// Usage:
//
//	skytest -json . | sky ci
//	skytest -json . | sky ci --system=github
//	skytest -json . | sky ci --coverage-threshold=80
package main

import (
	"os"

	"github.com/albertocavalcante/sky/internal/ci"
)

func main() {
	os.Exit(ci.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
