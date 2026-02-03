// Package cli provides shared utilities for Sky CLI tools.
package cli

// Standard exit codes for Sky CLI tools.
//
// These follow Unix conventions:
//   - 0: Success
//   - 1: General error (parse failures, runtime errors, etc.)
//   - 2: Warnings or check failures (lint warnings, format needed, etc.)
const (
	// ExitOK indicates successful execution with no issues.
	ExitOK = 0

	// ExitError indicates a fatal error occurred (parse error, I/O error, etc.).
	ExitError = 1

	// ExitWarning indicates the tool completed but found warnings or issues
	// that don't constitute errors. For example:
	//   - skylint found warnings (but no errors)
	//   - skyfmt --check found files that need formatting
	//   - skycheck found warnings (but no errors)
	ExitWarning = 2
)
