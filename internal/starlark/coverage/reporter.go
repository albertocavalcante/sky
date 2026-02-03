package coverage

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
)

// Reporter outputs coverage data in various formats.
type Reporter interface {
	// Write outputs the coverage report to the writer.
	Write(w io.Writer, report *Report) error
}

// -----------------------------------------------------------------------------
// Text Reporter
// -----------------------------------------------------------------------------

// TextReporter outputs coverage in human-readable text format.
type TextReporter struct {
	// Verbose enables detailed per-file output.
	Verbose bool

	// ShowMissing shows line numbers that weren't covered.
	ShowMissing bool
}

// Write implements Reporter.
func (r *TextReporter) Write(w io.Writer, report *Report) error {
	report.Compute()

	// Header
	writef(w, "Coverage Report\n")
	writef(w, "===============\n\n")

	// Per-file details
	if r.Verbose {
		for _, path := range report.FilePaths() {
			fc := report.Files[path]
			pct := fc.Lines.Percentage()
			writef(w, "%-60s %6.1f%% (%d/%d lines)\n",
				truncatePath(path, 60),
				pct,
				fc.Lines.CoveredLines,
				fc.Lines.TotalLines,
			)

			if r.ShowMissing && fc.Lines.CoveredLines < fc.Lines.TotalLines {
				missing := r.getMissingLines(fc)
				if len(missing) > 0 {
					writef(w, "  Missing: %s\n", formatLineRanges(missing))
				}
			}
		}
		writef(w, "\n")
	}

	// Summary
	writef(w, "Total: %.1f%% (%d/%d lines)\n",
		report.Percentage(),
		report.CoveredLines,
		report.TotalLines,
	)

	return nil
}

func (r *TextReporter) getMissingLines(fc *FileCoverage) []int {
	var missing []int
	for _, line := range fc.Lines.Lines() {
		if fc.Lines.Hits[line] == 0 {
			missing = append(missing, line)
		}
	}
	return missing
}

// formatLineRanges formats line numbers as ranges (e.g., "1-5, 10, 15-20").
func formatLineRanges(lines []int) string {
	if len(lines) == 0 {
		return ""
	}

	var parts []string
	start := lines[0]
	end := lines[0]

	for i := 1; i < len(lines); i++ {
		if lines[i] == end+1 {
			end = lines[i]
		} else {
			parts = append(parts, formatRange(start, end))
			start = lines[i]
			end = lines[i]
		}
	}
	parts = append(parts, formatRange(start, end))

	return strings.Join(parts, ", ")
}

func formatRange(start, end int) string {
	if start == end {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d-%d", start, end)
}

func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}

// -----------------------------------------------------------------------------
// JSON Reporter
// -----------------------------------------------------------------------------

// JSONReporter outputs coverage in JSON format.
type JSONReporter struct {
	// Pretty enables indented output.
	Pretty bool
}

// JSONReport is the JSON output structure.
type JSONReport struct {
	Timestamp    string        `json:"timestamp"`
	TotalLines   int           `json:"total_lines"`
	CoveredLines int           `json:"covered_lines"`
	Percentage   float64       `json:"percentage"`
	Files        []JSONFileCov `json:"files"`
}

// JSONFileCov is per-file coverage in JSON.
type JSONFileCov struct {
	Path         string  `json:"path"`
	TotalLines   int     `json:"total_lines"`
	CoveredLines int     `json:"covered_lines"`
	Percentage   float64 `json:"percentage"`
	Lines        []int   `json:"missing_lines,omitempty"`
}

// Write implements Reporter.
func (r *JSONReporter) Write(w io.Writer, report *Report) error {
	report.Compute()

	jr := JSONReport{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		TotalLines:   report.TotalLines,
		CoveredLines: report.CoveredLines,
		Percentage:   report.Percentage(),
	}

	for _, path := range report.FilePaths() {
		fc := report.Files[path]
		jfc := JSONFileCov{
			Path:         path,
			TotalLines:   fc.Lines.TotalLines,
			CoveredLines: fc.Lines.CoveredLines,
			Percentage:   fc.Lines.Percentage(),
		}

		// Include missing lines
		for _, line := range fc.Lines.Lines() {
			if fc.Lines.Hits[line] == 0 {
				jfc.Lines = append(jfc.Lines, line)
			}
		}

		jr.Files = append(jr.Files, jfc)
	}

	var data []byte
	var err error
	if r.Pretty {
		data, err = json.MarshalIndent(jr, "", "  ")
	} else {
		data, err = json.Marshal(jr)
	}
	if err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}

	_, _ = w.Write(data)
	_, _ = w.Write([]byte("\n"))
	return nil
}

// -----------------------------------------------------------------------------
// Cobertura XML Reporter
// -----------------------------------------------------------------------------

// CoberturaReporter outputs coverage in Cobertura XML format.
// This is compatible with most CI systems (Jenkins, GitLab, etc.).
type CoberturaReporter struct {
	// SourceDir is the source directory for relative paths.
	SourceDir string
}

// Cobertura XML structures
type coberturaCoverage struct {
	XMLName         xml.Name          `xml:"coverage"`
	LineRate        string            `xml:"line-rate,attr"`
	BranchRate      string            `xml:"branch-rate,attr"`
	Version         string            `xml:"version,attr"`
	Timestamp       int64             `xml:"timestamp,attr"`
	LinesValid      int               `xml:"lines-valid,attr"`
	LinesCovered    int               `xml:"lines-covered,attr"`
	BranchesValid   int               `xml:"branches-valid,attr"`
	BranchesCovered int               `xml:"branches-covered,attr"`
	Complexity      int               `xml:"complexity,attr"`
	Sources         coberturaSources  `xml:"sources"`
	Packages        coberturaPackages `xml:"packages"`
}

type coberturaSources struct {
	Source []string `xml:"source"`
}

type coberturaPackages struct {
	Package []coberturaPackage `xml:"package"`
}

type coberturaPackage struct {
	Name       string           `xml:"name,attr"`
	LineRate   string           `xml:"line-rate,attr"`
	BranchRate string           `xml:"branch-rate,attr"`
	Complexity int              `xml:"complexity,attr"`
	Classes    coberturaClasses `xml:"classes"`
}

type coberturaClasses struct {
	Class []coberturaClass `xml:"class"`
}

type coberturaClass struct {
	Name       string         `xml:"name,attr"`
	Filename   string         `xml:"filename,attr"`
	LineRate   string         `xml:"line-rate,attr"`
	BranchRate string         `xml:"branch-rate,attr"`
	Complexity int            `xml:"complexity,attr"`
	Lines      coberturaLines `xml:"lines"`
}

type coberturaLines struct {
	Line []coberturaLine `xml:"line"`
}

type coberturaLine struct {
	Number int `xml:"number,attr"`
	Hits   int `xml:"hits,attr"`
}

// Write implements Reporter.
func (r *CoberturaReporter) Write(w io.Writer, report *Report) error {
	report.Compute()

	cov := coberturaCoverage{
		LineRate:      fmt.Sprintf("%.4f", report.Percentage()/100.0),
		BranchRate:    "0",
		Version:       "1.0",
		Timestamp:     time.Now().Unix(),
		LinesValid:    report.TotalLines,
		LinesCovered:  report.CoveredLines,
		BranchesValid: 0,
		Complexity:    0,
	}

	if r.SourceDir != "" {
		cov.Sources.Source = []string{r.SourceDir}
	}

	// Group files by directory (package)
	packages := make(map[string][]string)
	for _, path := range report.FilePaths() {
		dir := filepath.Dir(path)
		packages[dir] = append(packages[dir], path)
	}

	for pkgName, files := range packages {
		pkg := coberturaPackage{
			Name:       pkgName,
			BranchRate: "0",
			Complexity: 0,
		}

		var pkgTotal, pkgCovered int

		for _, path := range files {
			fc := report.Files[path]
			pkgTotal += fc.Lines.TotalLines
			pkgCovered += fc.Lines.CoveredLines

			class := coberturaClass{
				Name:       filepath.Base(path),
				Filename:   path,
				LineRate:   fmt.Sprintf("%.4f", fc.Lines.Percentage()/100.0),
				BranchRate: "0",
				Complexity: 0,
			}

			for _, line := range fc.Lines.Lines() {
				class.Lines.Line = append(class.Lines.Line, coberturaLine{
					Number: line,
					Hits:   fc.Lines.Hits[line],
				})
			}

			pkg.Classes.Class = append(pkg.Classes.Class, class)
		}

		if pkgTotal > 0 {
			pkg.LineRate = fmt.Sprintf("%.4f", float64(pkgCovered)/float64(pkgTotal))
		} else {
			pkg.LineRate = "1.0"
		}

		cov.Packages.Package = append(cov.Packages.Package, pkg)
	}

	_, _ = w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(cov); err != nil {
		return fmt.Errorf("encoding Cobertura XML: %w", err)
	}
	_, _ = w.Write([]byte("\n"))
	return nil
}

// -----------------------------------------------------------------------------
// LCOV Reporter
// -----------------------------------------------------------------------------

// LCOVReporter outputs coverage in LCOV tracefile format.
// This is compatible with genhtml and many IDE extensions.
type LCOVReporter struct{}

// Write implements Reporter.
func (r *LCOVReporter) Write(w io.Writer, report *Report) error {
	report.Compute()

	for _, path := range report.FilePaths() {
		fc := report.Files[path]

		// Test name (TN:)
		writef(w, "TN:\n")

		// Source file (SF:)
		writef(w, "SF:%s\n", path)

		// Function coverage (FN:, FNDA:, FNF:, FNH:)
		fnTotal := len(fc.Functions)
		fnHit := 0
		for name, fn := range fc.Functions {
			writef(w, "FN:%d,%s\n", fn.StartLine, name)
			writef(w, "FNDA:%d,%s\n", fn.Hits, name)
			if fn.Hits > 0 {
				fnHit++
			}
		}
		writef(w, "FNF:%d\n", fnTotal)
		writef(w, "FNH:%d\n", fnHit)

		// Line coverage (DA:, LF:, LH:)
		for _, line := range fc.Lines.Lines() {
			writef(w, "DA:%d,%d\n", line, fc.Lines.Hits[line])
		}
		writef(w, "LF:%d\n", fc.Lines.TotalLines)
		writef(w, "LH:%d\n", fc.Lines.CoveredLines)

		// End of record
		writef(w, "end_of_record\n")
	}

	return nil
}

// Helper for writing to io.Writer
func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}
