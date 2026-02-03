// Package coverage provides code coverage tracking for Starlark execution.
//
// This package defines the coverage data structures and the API required
// from starlark-go-x for instrumentation. The actual instrumentation is
// not yet implemented in starlark-go-x, but this package serves as the
// specification for what changes are needed.
package coverage

import (
	"sort"
	"sync"
)

// LineCoverage tracks execution counts for lines in a file.
type LineCoverage struct {
	// Hits maps line numbers (1-based) to execution counts.
	Hits map[int]int

	// TotalLines is the total number of executable lines.
	TotalLines int

	// CoveredLines is the number of lines executed at least once.
	CoveredLines int
}

// NewLineCoverage creates a new LineCoverage instance.
func NewLineCoverage() *LineCoverage {
	return &LineCoverage{
		Hits: make(map[int]int),
	}
}

// RecordHit records an execution of the given line.
func (lc *LineCoverage) RecordHit(line int) {
	lc.Hits[line]++
}

// Compute calculates TotalLines and CoveredLines from Hits.
func (lc *LineCoverage) Compute() {
	lc.TotalLines = len(lc.Hits)
	lc.CoveredLines = 0
	for _, count := range lc.Hits {
		if count > 0 {
			lc.CoveredLines++
		}
	}
}

// Percentage returns the coverage percentage (0-100).
func (lc *LineCoverage) Percentage() float64 {
	if lc.TotalLines == 0 {
		return 100.0 // No lines = 100% covered
	}
	return float64(lc.CoveredLines) / float64(lc.TotalLines) * 100.0
}

// Lines returns sorted list of all line numbers.
func (lc *LineCoverage) Lines() []int {
	lines := make([]int, 0, len(lc.Hits))
	for line := range lc.Hits {
		lines = append(lines, line)
	}
	sort.Ints(lines)
	return lines
}

// FileCoverage contains coverage data for a single file.
type FileCoverage struct {
	// Path is the file path (absolute or relative).
	Path string

	// Lines contains line-level coverage data.
	Lines *LineCoverage

	// Functions contains function-level coverage (optional).
	Functions map[string]*FunctionCoverage
}

// NewFileCoverage creates a new FileCoverage for the given path.
func NewFileCoverage(path string) *FileCoverage {
	return &FileCoverage{
		Path:      path,
		Lines:     NewLineCoverage(),
		Functions: make(map[string]*FunctionCoverage),
	}
}

// FunctionCoverage contains coverage data for a single function.
type FunctionCoverage struct {
	// Name is the function name.
	Name string

	// StartLine is the first line of the function.
	StartLine int

	// EndLine is the last line of the function.
	EndLine int

	// Hits is the number of times the function was called.
	Hits int
}

// Report contains aggregated coverage data for multiple files.
type Report struct {
	mu sync.RWMutex

	// Files maps file paths to their coverage data.
	Files map[string]*FileCoverage

	// TotalLines is the sum of all executable lines.
	TotalLines int

	// CoveredLines is the sum of all covered lines.
	CoveredLines int
}

// NewReport creates a new empty coverage report.
func NewReport() *Report {
	return &Report{
		Files: make(map[string]*FileCoverage),
	}
}

// AddFile adds or returns existing file coverage.
func (r *Report) AddFile(path string) *FileCoverage {
	r.mu.Lock()
	defer r.mu.Unlock()

	if fc, ok := r.Files[path]; ok {
		return fc
	}

	fc := NewFileCoverage(path)
	r.Files[path] = fc
	return fc
}

// GetFile returns file coverage or nil if not found.
func (r *Report) GetFile(path string) *FileCoverage {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Files[path]
}

// Compute calculates aggregate statistics.
func (r *Report) Compute() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.TotalLines = 0
	r.CoveredLines = 0

	for _, fc := range r.Files {
		fc.Lines.Compute()
		r.TotalLines += fc.Lines.TotalLines
		r.CoveredLines += fc.Lines.CoveredLines
	}
}

// Percentage returns the overall coverage percentage.
func (r *Report) Percentage() float64 {
	if r.TotalLines == 0 {
		return 100.0
	}
	return float64(r.CoveredLines) / float64(r.TotalLines) * 100.0
}

// FilePaths returns sorted list of all file paths.
func (r *Report) FilePaths() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	paths := make([]string, 0, len(r.Files))
	for path := range r.Files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// Merge combines another report into this one.
func (r *Report) Merge(other *Report) {
	r.mu.Lock()
	defer r.mu.Unlock()

	other.mu.RLock()
	defer other.mu.RUnlock()

	for path, otherFC := range other.Files {
		fc, ok := r.Files[path]
		if !ok {
			fc = NewFileCoverage(path)
			r.Files[path] = fc
		}

		// Merge line hits
		for line, count := range otherFC.Lines.Hits {
			fc.Lines.Hits[line] += count
		}

		// Merge function hits
		for name, otherFn := range otherFC.Functions {
			if fn, ok := fc.Functions[name]; ok {
				fn.Hits += otherFn.Hits
			} else {
				fc.Functions[name] = &FunctionCoverage{
					Name:      otherFn.Name,
					StartLine: otherFn.StartLine,
					EndLine:   otherFn.EndLine,
					Hits:      otherFn.Hits,
				}
			}
		}
	}
}

// -----------------------------------------------------------------------------
// Starlark-go-x API Specification
// -----------------------------------------------------------------------------
// The following types define what we need from starlark-go-x.
// These are interfaces that the fork should implement.

// Collector is the interface that starlark-go-x should implement
// to enable coverage collection during execution.
//
// SPEC: This interface should be implemented in starlark-go-x
// and attached to a starlark.Thread to collect coverage data.
type Collector interface {
	// BeforeExec is called before each statement execution.
	// filename is the source file, line is the 1-based line number.
	BeforeExec(filename string, line int)

	// AfterExec is called after each statement execution.
	AfterExec(filename string, line int)

	// EnterFunction is called when entering a function.
	EnterFunction(filename string, name string, line int)

	// ExitFunction is called when exiting a function.
	ExitFunction(filename string, name string)

	// Report returns the collected coverage data.
	Report() *Report
}

// DefaultCollector is a basic implementation of Collector.
// This can be used once starlark-go-x supports the instrumentation hooks.
type DefaultCollector struct {
	mu     sync.Mutex
	report *Report
}

// NewCollector creates a new coverage collector.
func NewCollector() *DefaultCollector {
	return &DefaultCollector{
		report: NewReport(),
	}
}

// BeforeExec implements Collector.
func (c *DefaultCollector) BeforeExec(filename string, line int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	fc := c.report.AddFile(filename)
	fc.Lines.RecordHit(line)
}

// AfterExec implements Collector.
func (c *DefaultCollector) AfterExec(_ string, _ int) {
	// No-op for basic line coverage
}

// EnterFunction implements Collector.
func (c *DefaultCollector) EnterFunction(filename string, name string, line int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	fc := c.report.AddFile(filename)
	if fn, ok := fc.Functions[name]; ok {
		fn.Hits++
	} else {
		fc.Functions[name] = &FunctionCoverage{
			Name:      name,
			StartLine: line,
			Hits:      1,
		}
	}
}

// ExitFunction implements Collector.
func (c *DefaultCollector) ExitFunction(_ string, _ string) {
	// No-op for basic function coverage
}

// Report implements Collector.
func (c *DefaultCollector) Report() *Report {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.report.Compute()
	return c.report
}
