// Package linter provides a configurable Starlark linter with extensible rules.
package linter

import (
	"bytes"
	"fmt"
	"os"

	"github.com/albertocavalcante/sky/internal/starlark/sortutil"
	"github.com/pmezard/go-difflib/difflib"
)

// FixResult represents the result of applying fixes to a file.
type FixResult struct {
	// Path is the file path.
	Path string

	// OriginalContent is the original file content.
	OriginalContent []byte

	// FixedContent is the content after applying fixes.
	FixedContent []byte

	// AppliedFixes is the number of fixes that were applied.
	AppliedFixes int

	// SkippedFixes is the number of fixes skipped due to conflicts.
	SkippedFixes int
}

// HasChanges returns true if fixes were applied.
func (r *FixResult) HasChanges() bool {
	return !bytes.Equal(r.OriginalContent, r.FixedContent)
}

// Diff returns a unified diff between original and fixed content.
func (r *FixResult) Diff() string {
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(r.OriginalContent)),
		B:        difflib.SplitLines(string(r.FixedContent)),
		FromFile: r.Path,
		ToFile:   r.Path,
		Context:  3,
	}
	text, _ := difflib.GetUnifiedDiffString(diff)
	return text
}

// ApplyFixes applies the given fixes to the content.
// Fixes are applied in reverse order (from end to start) to preserve byte offsets.
// When fixes overlap, the one with the earlier start position wins.
func ApplyFixes(content []byte, fixes []*Replacement) ([]byte, int, int) {
	if len(fixes) == 0 {
		return content, 0, 0
	}

	// Filter out nil fixes and validate
	var validFixes []*Replacement
	for _, fix := range fixes {
		if fix != nil && fix.Start >= 0 && fix.End >= fix.Start && fix.End <= len(content) {
			validFixes = append(validFixes, fix)
		}
	}

	if len(validFixes) == 0 {
		return content, 0, 0
	}

	// Sort fixes by start position (ascending) to detect overlaps correctly
	sortutil.Asc(validFixes, func(f *Replacement) int { return f.Start })

	// Detect and skip overlapping fixes (prefer earlier fixes)
	var nonOverlapping []*Replacement
	skipped := 0
	lastEnd := 0

	for _, fix := range validFixes {
		if fix.Start >= lastEnd {
			// No overlap with previously accepted fixes
			nonOverlapping = append(nonOverlapping, fix)
			lastEnd = fix.End
		} else {
			// Overlaps with a previously accepted fix, skip it
			skipped++
		}
	}

	// Sort by start position descending to apply from end to start
	sortutil.Desc(nonOverlapping, func(f *Replacement) int { return f.Start })

	// Apply fixes
	result := content
	for _, fix := range nonOverlapping {
		result = applyFix(result, fix)
	}

	return result, len(nonOverlapping), skipped
}

// applyFix applies a single fix to content.
func applyFix(content []byte, fix *Replacement) []byte {
	// Build new content: before + replacement + after
	before := content[:fix.Start]
	after := content[fix.End:]

	result := make([]byte, 0, len(before)+len(fix.Content)+len(after))
	result = append(result, before...)
	result = append(result, fix.Content...)
	result = append(result, after...)

	return result
}

// FixFiles applies fixes to the given files based on findings.
// Returns the fix results and any errors encountered.
func FixFiles(findings []Finding) ([]FixResult, error) {
	// Group findings by file
	byFile := make(map[string][]Finding)
	for _, f := range findings {
		if f.Replacement != nil {
			byFile[f.FilePath] = append(byFile[f.FilePath], f)
		}
	}

	var results []FixResult
	for path, fileFindings := range byFile {
		result, err := fixFile(path, fileFindings)
		if err != nil {
			return nil, fmt.Errorf("fixing %s: %w", path, err)
		}
		results = append(results, result)
	}

	return results, nil
}

// fixFile applies fixes to a single file.
func fixFile(path string, findings []Finding) (FixResult, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return FixResult{}, fmt.Errorf("reading file: %w", err)
	}

	// Extract replacements from findings
	var fixes []*Replacement
	for _, f := range findings {
		if f.Replacement != nil {
			fixes = append(fixes, f.Replacement)
		}
	}

	fixed, applied, skipped := ApplyFixes(content, fixes)

	return FixResult{
		Path:            path,
		OriginalContent: content,
		FixedContent:    fixed,
		AppliedFixes:    applied,
		SkippedFixes:    skipped,
	}, nil
}

// WriteFixResults writes the fixed content back to files.
// Only writes files that have changes.
func WriteFixResults(results []FixResult) error {
	for _, r := range results {
		if r.HasChanges() {
			if err := os.WriteFile(r.Path, r.FixedContent, 0644); err != nil {
				return fmt.Errorf("writing %s: %w", r.Path, err)
			}
		}
	}
	return nil
}

// FixableCount returns the number of findings that have fixes.
func FixableCount(findings []Finding) int {
	count := 0
	for _, f := range findings {
		if f.Replacement != nil {
			count++
		}
	}
	return count
}
