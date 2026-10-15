package skycov

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/albertocavalcante/sky/internal/atomicfile"
	"github.com/albertocavalcante/sky/internal/skyconfig"
	"github.com/albertocavalcante/sky/internal/starlark/coverage"
	"github.com/albertocavalcante/sky/internal/version"
)

// Exit codes
const (
	exitOK       = 0
	exitBelowMin = 1
	exitError    = 2
)

// Run executes skycov with the given arguments.
// Returns exit code.
func Run(args []string) int {
	return RunWithIO(context.Background(), args, os.Stdin, os.Stdout, os.Stderr)
}

// RunWithIO allows custom IO for embedding/testing.
func RunWithIO(_ context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var (
		formatFlag  string
		outputFlag  string
		minFlag     float64
		sourceFlag  string
		versionFlag bool
		verboseFlag bool
		demoFlag    bool
	)

	fs := flag.NewFlagSet("skycov", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&formatFlag, "format", "text", "output format: text, json, html, cobertura, lcov")
	fs.StringVar(&outputFlag, "o", "", "output file (default: stdout)")
	fs.Float64Var(&minFlag, "min", 0, "minimum coverage percentage (fail if below)")
	fs.StringVar(&sourceFlag, "source", "", "source directory for relative paths")
	fs.BoolVar(&versionFlag, "version", false, "print version and exit")
	fs.BoolVar(&verboseFlag, "v", false, "verbose output")
	fs.BoolVar(&demoFlag, "demo", false, "show a built-in sample report instead of reading coverage data")

	fs.Usage = func() {
		writeln(stderr, "Usage: skycov [flags] <coverage-data>")
		writeln(stderr)
		writeln(stderr, "Coverage reporter for Starlark code.")
		writeln(stderr)
		writeln(stderr, "NOTE: Coverage collection requires starlark-go-x instrumentation,")
		writeln(stderr, "which is not yet implemented. This tool currently processes")
		writeln(stderr, "coverage data files or demonstrates the report formats.")
		writeln(stderr)
		writeln(stderr, "Output Formats:")
		writeln(stderr, "  text      Human-readable summary (default)")
		writeln(stderr, "  json      JSON format for tooling")
		writeln(stderr, "  html      HTML report with line coverage visualization")
		writeln(stderr, "  cobertura Cobertura XML for CI (Jenkins, GitLab, etc.)")
		writeln(stderr, "  lcov      LCOV tracefile for genhtml and IDEs")
		writeln(stderr)
		writeln(stderr, "Flags:")
		fs.PrintDefaults()
		writeln(stderr)
		writeln(stderr, "Examples:")
		writeln(stderr, "  skycov coverage.json               # Display text report")
		writeln(stderr, "  skycov -format=cobertura -o cov.xml coverage.json")
		writeln(stderr, "  skycov -min=80 coverage.json       # Fail if < 80% coverage")
		writeln(stderr)
		writeln(stderr, "Future Usage (once starlark-go-x supports coverage):")
		writeln(stderr, "  skytest --coverage tests/          # Generate coverage data")
		writeln(stderr, "  skycov coverage.json               # Process results")
		writeln(stderr)
		writeln(stderr, "starlark-go-x API Required:")
		writeln(stderr, "  The Collector interface in internal/starlark/coverage defines")
		writeln(stderr, "  the instrumentation hooks needed in starlark-go-x:")
		writeln(stderr, "  - BeforeExec(filename, line)   # Called before each statement")
		writeln(stderr, "  - EnterFunction(filename, name, line)")
		writeln(stderr, "  - Thread.SetCoverageCollector(c)")
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return exitOK
		}
		return exitError
	}

	if versionFlag {
		writef(stdout, "skycov %s\n", version.String())
		return exitOK
	}

	// A NaN, infinite or out-of-range minimum would make every comparison
	// false and silently disable the gate.
	if err := skyconfig.ValidateCoverageThreshold(minFlag); err != nil {
		writef(stderr, "skycov: -min: %v\n", err)
		return exitError
	}

	// Validate the format before touching any input or output.
	var reporter coverage.Reporter
	switch formatFlag {
	case "text":
		reporter = &coverage.TextReporter{Verbose: verboseFlag, ShowMissing: verboseFlag}
	case "json":
		reporter = &coverage.JSONReporter{Pretty: true}
	case "html":
		reporter = &coverage.HTMLReporter{}
	case "cobertura":
		reporter = &coverage.CoberturaReporter{SourceDir: sourceFlag}
	case "lcov":
		reporter = &coverage.LCOVReporter{}
	default:
		writef(stderr, "skycov: unknown format %q\n", formatFlag)
		return exitError
	}

	inputFiles := fs.Args()
	var report *coverage.Report

	switch {
	case demoFlag:
		if minFlag > 0 {
			writeln(stderr, "skycov: -min cannot be combined with --demo")
			return exitError
		}
		report = demoReport()
	case len(inputFiles) == 0:
		writeln(stderr, "skycov: missing coverage data file (use --demo to show sample output)")
		fs.Usage()
		return exitError
	default:
		var err error
		report, err = loadCoverageData(inputFiles[0])
		if err != nil {
			writef(stderr, "skycov: %v\n", err)
			writeln(stderr)
			writeln(stderr, "Note: Coverage collection is not yet implemented.")
			writeln(stderr, "Run 'skycov --help' for more information.")
			return exitError
		}
	}

	// Render fully in memory so a render error never leaves a partial file.
	var buf bytes.Buffer
	if err := reporter.Report(&buf, report); err != nil {
		writef(stderr, "skycov: %v\n", err)
		return exitError
	}
	if outputFlag != "" {
		if err := writeFileAtomic(outputFlag, buf.Bytes()); err != nil {
			writef(stderr, "skycov: %v\n", err)
			return exitError
		}
	} else if _, err := stdout.Write(buf.Bytes()); err != nil {
		writef(stderr, "skycov: %v\n", err)
		return exitError
	}

	// Check minimum coverage
	report.Compute()
	// Like sky-ci, a report with no measured lines is "unavailable", not 100%.
	if minFlag > 0 && report.TotalLines == 0 {
		writef(stderr, "skycov: -min=%.1f set but coverage unavailable: no lines were measured in %s\n", minFlag, inputFiles[0])
		return exitError
	}
	if minFlag > 0 && report.Percentage() < minFlag {
		writef(stderr, "skycov: coverage %.1f%% is below minimum %.1f%%\n",
			report.Percentage(), minFlag)
		return exitBelowMin
	}

	return exitOK
}

// demoReport creates a sample report to demonstrate output formats.
func demoReport() *coverage.Report {
	report := coverage.NewReport()

	// File 1: Well-covered
	fc1 := report.AddFile("src/math.star")
	for i := 1; i <= 20; i++ {
		fc1.Lines.RecordHit(i)
	}
	fc1.Functions["add"] = &coverage.FunctionCoverage{Name: "add", StartLine: 1, EndLine: 5, Hits: 10}
	fc1.Functions["subtract"] = &coverage.FunctionCoverage{Name: "subtract", StartLine: 7, EndLine: 11, Hits: 5}
	fc1.Functions["multiply"] = &coverage.FunctionCoverage{Name: "multiply", StartLine: 13, EndLine: 17, Hits: 8}

	// File 2: Partially covered
	fc2 := report.AddFile("src/utils.star")
	for i := 1; i <= 10; i++ {
		if i <= 7 {
			fc2.Lines.RecordHit(i)
		} else {
			fc2.Lines.Hits[i] = 0 // Not covered
		}
	}
	fc2.Functions["helper"] = &coverage.FunctionCoverage{Name: "helper", StartLine: 1, EndLine: 5, Hits: 3}
	fc2.Functions["unused"] = &coverage.FunctionCoverage{Name: "unused", StartLine: 7, EndLine: 10, Hits: 0}

	// File 3: Poorly covered
	fc3 := report.AddFile("src/legacy.star")
	fc3.Lines.RecordHit(1)
	fc3.Lines.RecordHit(2)
	for i := 3; i <= 15; i++ {
		fc3.Lines.Hits[i] = 0
	}

	return report
}

// loadCoverageData loads coverage from a JSON file.
// The format is the JSON output from skytest --coverage.
func loadCoverageData(path string) (*coverage.Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	// Parse the JSON format from skytest --coverage
	var raw struct {
		Files map[string]struct {
			Lines map[string]int `json:"lines"`
		} `json:"files"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	report := coverage.NewReport()
	for filePath, fileData := range raw.Files {
		fc := report.AddFile(filePath)
		for lineStr, hits := range fileData.Lines {
			line, err := strconv.Atoi(lineStr)
			if err != nil {
				continue
			}
			fc.Lines.Hits[line] = hits
		}
	}
	report.Compute()

	return report, nil
}

// Helper functions for writing output.
func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func writeln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}

// writeFileAtomic atomically replaces path, preserving an existing file's
// permission bits and honoring the umask for a new one.
func writeFileAtomic(path string, data []byte) error {
	return atomicfile.Write(path, data, 0o666)
}
