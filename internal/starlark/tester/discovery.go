package tester

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultTestPatterns are the default file patterns for test discovery.
var DefaultTestPatterns = []string{
	"*_test.star",
	"test_*.star",
}

// DiscoverFiles finds test files matching the given patterns in a directory.
// If patterns is empty, uses DefaultTestPatterns.
// If recursive is true, searches subdirectories as well.
func DiscoverFiles(dir string, patterns []string, recursive bool) ([]string, error) {
	if len(patterns) == 0 {
		patterns = DefaultTestPatterns
	}

	var files []string
	seen := make(map[string]bool)

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories (unless at root in non-recursive mode)
		if info.IsDir() {
			if !recursive && path != dir {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file matches any pattern
		base := filepath.Base(path)
		for _, pattern := range patterns {
			matched, err := filepath.Match(pattern, base)
			if err != nil {
				return err
			}
			if matched && !seen[path] {
				files = append(files, path)
				seen[path] = true
				break
			}
		}

		return nil
	}

	if err := filepath.Walk(dir, walkFn); err != nil {
		return nil, err
	}

	return files, nil
}

// IsTestFile checks if a filename matches test file patterns.
func IsTestFile(filename string, patterns []string) bool {
	if len(patterns) == 0 {
		patterns = DefaultTestPatterns
	}

	base := filepath.Base(filename)
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}
	return false
}

// ClassifyPath determines how to process a path argument.
// Returns:
//   - "file" if path is a single file
//   - "dir" if path is a directory
//   - "glob" if path contains glob characters
func ClassifyPath(path string) string {
	// Check for glob characters
	if strings.ContainsAny(path, "*?[") {
		return "glob"
	}

	info, err := os.Stat(path)
	if err != nil {
		// Assume it's a file pattern that doesn't exist yet
		return "file"
	}

	if info.IsDir() {
		return "dir"
	}
	return "file"
}

// ExpandPaths expands a list of paths into test files.
// Handles files, directories, and glob patterns.
func ExpandPaths(paths []string, patterns []string, recursive bool) ([]string, error) {
	if len(patterns) == 0 {
		patterns = DefaultTestPatterns
	}

	var result []string
	seen := make(map[string]bool)

	for _, path := range paths {
		switch ClassifyPath(path) {
		case "glob":
			matches, err := filepath.Glob(path)
			if err != nil {
				return nil, err
			}
			for _, m := range matches {
				if !seen[m] {
					result = append(result, m)
					seen[m] = true
				}
			}

		case "dir":
			files, err := DiscoverFiles(path, patterns, recursive)
			if err != nil {
				return nil, err
			}
			for _, f := range files {
				if !seen[f] {
					result = append(result, f)
					seen[f] = true
				}
			}

		default: // file
			if !seen[path] {
				result = append(result, path)
				seen[path] = true
			}
		}
	}

	return result, nil
}
