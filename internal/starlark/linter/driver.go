package linter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bazelbuild/buildtools/build"

	"github.com/albertocavalcante/sky/internal/starlark/classifier"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// Driver executes lint rules on files.
type Driver struct {
	registry   *Registry
	classifier classifier.Classifier
}

// NewDriver creates a new driver with the given registry.
func NewDriver(registry *Registry) *Driver {
	return &Driver{
		registry:   registry,
		classifier: classifier.NewDefaultClassifier(),
	}
}

// Run executes all enabled rules on the specified files and returns the results.
// The files parameter can include individual files or directories (which will be walked).
func (d *Driver) Run(ctx context.Context, paths []string) (*Result, error) {
	// Expand paths to individual files
	files, err := d.expandPaths(paths)
	if err != nil {
		return nil, err
	}

	result := &Result{
		Files:    len(files),
		Findings: []Finding{},
		Errors:   []FileError{},
	}

	// Process each file
	for _, path := range files {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		findings, err := d.RunFile(path)
		if err != nil {
			result.Errors = append(result.Errors, FileError{
				Path: path,
				Err:  err,
			})
			continue
		}

		result.Findings = append(result.Findings, findings...)
	}

	return result, nil
}

// RunFile executes all enabled rules on a single file.
func (d *Driver) RunFile(path string) ([]Finding, error) {
	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	// Classify the file to determine its kind
	classification, err := d.classifier.Classify(path)
	if err != nil {
		// If classification fails, try to parse as generic Starlark
		classification = classifier.Classification{
			FileKind: filekind.KindStarlark,
		}
	}

	// Parse the file
	file, err := parseFile(content, path, classification.FileKind)
	if err != nil {
		return nil, fmt.Errorf("parsing file: %w", err)
	}

	// Parse suppression comments
	suppressionParser := NewSuppressionParser(content)

	// Get enabled rules in dependency order
	rules := d.registry.EnabledRules()

	// Filter rules by file kind
	var applicableRules []*Rule
	for _, rule := range rules {
		if d.isApplicable(rule, classification.FileKind) {
			applicableRules = append(applicableRules, rule)
		}
	}

	// Execute rules and collect findings
	var findings []Finding
	results := make(map[*Rule]any) // Store results for dependent rules

	for _, rule := range applicableRules {
		// Get config for this rule
		config := d.registry.GetConfig(rule.Name)

		// Create pass context
		pass := &Pass{
			File:     file,
			FilePath: path,
			FileKind: classification.FileKind,
			Content:  content,
			Config:   config,
			Report: func(f Finding) {
				// Set the file path
				f.FilePath = path
				// Apply config severity override if set
				if config.Severity != 0 {
					f.Severity = config.Severity
				}
				findings = append(findings, f)
			},
			ResultOf: func(r *Rule) any {
				return results[r]
			},
		}

		// Execute the rule
		result, err := rule.Run(pass)
		if err != nil {
			return nil, fmt.Errorf("rule %s: %w", rule.Name, err)
		}

		// Store result for dependent rules
		if result != nil {
			results[rule] = result
		}
	}

	// Filter out suppressed findings
	findings = FilterSuppressed(findings, suppressionParser)

	return findings, nil
}

// isApplicable checks if a rule applies to a given file kind.
func (d *Driver) isApplicable(rule *Rule, kind filekind.Kind) bool {
	// If FileKinds is empty, the rule applies to all file kinds
	if len(rule.FileKinds) == 0 {
		return true
	}

	// Check if the file kind is in the rule's list
	for _, k := range rule.FileKinds {
		if k == kind {
			return true
		}
	}

	return false
}

// expandPaths expands a list of paths into individual files.
// Directories are walked recursively to find Starlark files.
func (d *Driver) expandPaths(paths []string) ([]string, error) {
	var files []string
	seen := make(map[string]bool) // Deduplicate files

	for _, path := range paths {
		expanded, err := d.expandPath(path)
		if err != nil {
			return nil, err
		}

		for _, f := range expanded {
			// Normalize path and deduplicate
			absPath, err := filepath.Abs(f)
			if err != nil {
				absPath = f
			}
			if !seen[absPath] {
				seen[absPath] = true
				files = append(files, f)
			}
		}
	}

	return files, nil
}

// expandPath expands a single path into files.
func (d *Driver) expandPath(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		// Single file
		return []string{path}, nil
	}

	// Directory - walk it
	var files []string
	err = filepath.WalkDir(path, func(p string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories
		if entry.IsDir() && strings.HasPrefix(entry.Name(), ".") && entry.Name() != "." {
			return filepath.SkipDir
		}

		// Skip non-files
		if entry.IsDir() {
			return nil
		}

		// Check if it's a Starlark file
		if d.isStarlarkFile(entry.Name()) {
			files = append(files, p)
		}

		return nil
	})

	return files, err
}

// isStarlarkFile checks if a filename is a Starlark file.
func (d *Driver) isStarlarkFile(name string) bool {
	// Exact matches
	switch name {
	case "BUILD", "BUILD.bazel", "WORKSPACE", "WORKSPACE.bazel", "MODULE.bazel",
		"BUCK", "Tiltfile":
		return true
	}

	// Extension matches
	ext := filepath.Ext(name)
	switch ext {
	case ".bzl", ".bxl", ".star", ".starlark", ".sky", ".skyi",
		".axl", ".ipd", ".plz", ".pconf", ".pinc", ".mpconf":
		return true
	}

	return false
}

// parseFile parses a Starlark file based on its kind.
func parseFile(content []byte, path string, kind filekind.Kind) (*build.File, error) {
	switch kind {
	case filekind.KindBUILD, filekind.KindBUCK:
		return build.ParseBuild(path, content)
	case filekind.KindWORKSPACE:
		return build.ParseWorkspace(path, content)
	case filekind.KindMODULE:
		return build.ParseModule(path, content)
	case filekind.KindBzl, filekind.KindBzlmod, filekind.KindBzlBuck:
		return build.ParseBzl(path, content)
	default:
		// KindStarlark, KindSkyI, KindUnknown, or any other
		return build.ParseDefault(path, content)
	}
}
