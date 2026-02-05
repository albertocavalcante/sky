package ci

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// GenericHandler outputs test results in a generic text format.
// Used as fallback for unsupported CI systems.
type GenericHandler struct {
	Config Config
	Name   string
}

// Handle processes test results for generic CI systems.
func (h *GenericHandler) Handle(results *TestResults, stdout, stderr io.Writer) error {
	if h.Config.Quiet {
		return nil
	}

	passed, failed, skipped, total := results.Summary()

	// Print summary
	fmt.Fprintf(stdout, "Test Results (%s)\n", h.Name)
	fmt.Fprintln(stdout, strings.Repeat("=", 40))
	fmt.Fprintf(stdout, "Passed:  %d\n", passed)
	fmt.Fprintf(stdout, "Failed:  %d\n", failed)
	if skipped > 0 {
		fmt.Fprintf(stdout, "Skipped: %d\n", skipped)
	}
	fmt.Fprintf(stdout, "Total:   %d\n", total)

	if results.Duration != "" {
		fmt.Fprintf(stdout, "Duration: %s\n", results.Duration)
	}
	fmt.Fprintln(stdout)

	// Print failed tests
	if failed > 0 {
		fmt.Fprintln(stdout, "Failed Tests:")
		fmt.Fprintln(stdout, strings.Repeat("-", 40))
		for _, file := range results.Files {
			for _, test := range file.Tests {
				if !test.Passed && !test.Skipped {
					fmt.Fprintf(stdout, "  %s::%s\n", filepath.Base(file.Path), test.Name)
					if test.Error != "" {
						fmt.Fprintf(stdout, "    %s\n", test.Error)
					}
				}
			}
		}
	}

	return nil
}
