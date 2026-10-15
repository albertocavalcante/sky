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

	// Single file (regular files only: never open a FIFO, device or socket)
	if info.Mode().IsRegular() && IsStarlarkFile(filepath.Base(filePath)) {
		return []string{filePath}, nil
	}

	return nil, nil
}

// discoverGlobPattern handles glob patterns like *.bzl, lib/*.bzl, **/*.star.
// Patterns are matched against root-relative, slash-separated paths; "**"
// matches zero or more directories.
func discoverGlobPattern(pattern string, root string) ([]string, error) {
	if !hasRecursiveSegment(pattern) {
		// Plain glob (*.bzl, lib/*.bzl): keep filepath.Glob semantics, which
		// follow symlinks and allow explicitly named hidden directories.
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			return nil, err
		}
		var result []string
		for _, match := range matches {
			if !IsStarlarkFile(filepath.Base(match)) {
				continue
			}
			// Follow symlinks; keep only regular files.
			if !isFileCandidate(match) {
				continue
			}
			result = append(result, match)
		}
		return result, nil
	}
	return discoverRecursiveGlob(root, pattern)
}

// discoverRecursive finds all Starlark files recursively under a directory.
func discoverRecursive(dir string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories
		if entry.IsDir() && path != dir && strings.HasPrefix(entry.Name(), ".") {
			return filepath.SkipDir
		}

		// Skip non-files
		if entry.IsDir() {
			return nil
		}

		// Check if it's a Starlark file
		if IsStarlarkFile(entry.Name()) && isFileEntry(path, entry) {
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
		full := filepath.Join(dir, entry.Name())
		if IsStarlarkFile(entry.Name()) && isFileEntry(full, entry) {
			files = append(files, full)
		}
	}

	return files, nil
}

// discoverRecursiveGlob walks root and returns Starlark files whose
// root-relative slash path matches pattern.
func discoverRecursiveGlob(root string, pattern string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			// Skip hidden directories, but never the walk root itself.
			if path != root && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		if IsStarlarkFile(entry.Name()) && matchGlob(pattern, filepath.ToSlash(rel)) && isFileEntry(path, entry) {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// isFileEntry reports whether a directory entry that is not itself a directory
// is a candidate file. Only regular files qualify: FIFOs, devices and sockets
// are skipped because reading them can block forever. Symlinks are followed
// (a link to a file is read through the link, like filepath.Glob matches); a
// link to a directory, to a non-regular file, or a dangling link is skipped.
func isFileEntry(path string, entry os.DirEntry) bool {
	if entry.Type()&os.ModeSymlink == 0 {
		return entry.Type().IsRegular()
	}
	return isFileCandidate(path)
}

// isFileCandidate stats path (following symlinks) and reports whether it
// should be indexed. A target that does not exist is skipped silently. Any
// other stat error (for example EACCES behind an unreadable directory) keeps
// the path as a candidate, so indexing it reports the error to the user
// instead of dropping the file without a trace.
func isFileCandidate(path string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return !os.IsNotExist(err)
	}
	return st.Mode().IsRegular()
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
