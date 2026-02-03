package index

import (
	"os"
	"path/filepath"
	"strings"
)

// starlarkExtensions is the set of recognized Starlark file extensions.
var starlarkExtensions = map[string]bool{
	".bzl":      true,
	".bxl":      true,
	".star":     true,
	".starlark": true,
	".sky":      true,
	".skyi":     true,
	".axl":      true,
	".ipd":      true,
	".plz":      true,
	".pconf":    true,
	".pinc":     true,
	".mpconf":   true,
}

// starlarkFilenames is the set of recognized Starlark filenames (without extension).
var starlarkFilenames = map[string]bool{
	"BUILD":           true,
	"BUILD.bazel":     true,
	"WORKSPACE":       true,
	"WORKSPACE.bazel": true,
	"MODULE.bazel":    true,
	"BUCK":            true,
	"Tiltfile":        true,
}

// Discover finds all Starlark files matching a pattern.
// Supported patterns:
//   - "//..." - all files recursively from root
//   - "//pkg/..." - all files recursively under pkg/
//   - "//path/to/file.star" - specific file
//   - "*.bzl" - glob in current directory
//   - "**/*.star" - recursive glob
func Discover(pattern string, root string) ([]string, error) {
	// Handle Bazel-style patterns
	if strings.HasPrefix(pattern, "//") {
		return discoverBazelPattern(pattern, root)
	}

	// Handle glob patterns
	return discoverGlobPattern(pattern, root)
}

// discoverBazelPattern handles Bazel-style patterns like //..., //pkg/..., //path/to/file.star
func discoverBazelPattern(pattern string, root string) ([]string, error) {
	// Strip the // prefix
	pattern = strings.TrimPrefix(pattern, "//")

	// Check if it's a recursive pattern
	if strings.HasSuffix(pattern, "/...") {
		// Recursive pattern: //pkg/...
		pkg := strings.TrimSuffix(pattern, "/...")
		searchPath := filepath.Join(root, pkg)
		return discoverRecursive(searchPath)
	} else if pattern == "..." {
		// Root recursive pattern: //...
		return discoverRecursive(root)
	}

	// Specific file pattern: //path/to/file.star
	// Also handle label-style patterns: //pkg:file.bzl
	pattern = strings.ReplaceAll(pattern, ":", "/")
	filePath := filepath.Join(root, pattern)

	// Check if it's a directory
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Return empty slice for non-existent paths
		}
		return nil, err
	}

	if info.IsDir() {
		return discoverInDirectory(filePath)
	}

	// Single file
	if IsStarlarkFile(filepath.Base(filePath)) {
		return []string{filePath}, nil
	}

	return nil, nil
}

// discoverGlobPattern handles glob patterns like *.bzl, **/*.star
func discoverGlobPattern(pattern string, root string) ([]string, error) {
	// Check for recursive glob
	if strings.HasPrefix(pattern, "**/") {
		// Recursive glob: **/*.bzl
		suffix := strings.TrimPrefix(pattern, "**/")
		return discoverRecursiveGlob(root, suffix)
	}

	// Simple glob: *.bzl
	fullPattern := filepath.Join(root, pattern)
	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return nil, err
	}

	// Filter to only Starlark files
	var result []string
	for _, match := range matches {
		if IsStarlarkFile(filepath.Base(match)) {
			result = append(result, match)
		}
	}

	return result, nil
}

// discoverRecursive finds all Starlark files recursively under a directory.
func discoverRecursive(dir string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
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
		if IsStarlarkFile(entry.Name()) {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// discoverInDirectory finds all Starlark files in a single directory (non-recursive).
func discoverInDirectory(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if IsStarlarkFile(entry.Name()) {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}

	return files, nil
}

// discoverRecursiveGlob finds files matching a pattern recursively.
func discoverRecursiveGlob(root string, suffix string) ([]string, error) {
	var files []string

	// Extract extension from suffix for filtering
	ext := filepath.Ext(suffix)

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories
		if entry.IsDir() && strings.HasPrefix(entry.Name(), ".") && entry.Name() != "." {
			return filepath.SkipDir
		}

		// Skip directories
		if entry.IsDir() {
			return nil
		}

		// Check if the file matches the pattern
		if ext != "" && filepath.Ext(entry.Name()) == ext {
			files = append(files, path)
		} else if matched, _ := filepath.Match(suffix, entry.Name()); matched {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// IsStarlarkFile checks if a filename is a Starlark file.
func IsStarlarkFile(name string) bool {
	// Check exact filename matches
	if starlarkFilenames[name] {
		return true
	}

	// Check extension matches
	ext := filepath.Ext(name)
	return starlarkExtensions[ext]
}
