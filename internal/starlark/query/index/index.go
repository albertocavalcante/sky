package index

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bazelbuild/buildtools/build"

	"github.com/albertocavalcante/sky/internal/starlark/classifier"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// Index holds parsed Starlark files for querying.
type Index struct {
	root       string
	files      map[string]*File
	classifier classifier.Classifier
	mu         sync.RWMutex
}

// New creates a new index rooted at the given directory.
func New(root string) *Index {
	return &Index{
		root:       root,
		files:      make(map[string]*File),
		classifier: classifier.NewDefaultClassifier(),
	}
}

// Add parses and adds a file to the index.
// Returns an error if the file cannot be read or parsed.
func (idx *Index) Add(path string) error {
	// Make path relative to root if it's absolute
	relPath, err := idx.relativePath(path)
	if err != nil {
		return err
	}

	// Read file content
	absPath := idx.absolutePath(relPath)
	content, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	// Classify the file
	classification, err := idx.classifier.Classify(relPath)
	if err != nil {
		// Default to generic Starlark if classification fails
		classification = classifier.Classification{
			FileKind: filekind.KindStarlark,
		}
	}

	// Parse the file
	file, err := parseFile(content, relPath, classification.FileKind)
	if err != nil {
		return fmt.Errorf("parsing file: %w", err)
	}

	// Extract index data
	indexedFile := ExtractFile(file, relPath, classification.FileKind)

	// Add to index
	idx.mu.Lock()
	idx.files[relPath] = indexedFile
	idx.mu.Unlock()

	return nil
}

// AddPattern adds all files matching a pattern to the index.
// Returns the number of files added and any errors encountered.
// Non-fatal errors (e.g., parse errors) are collected but don't stop processing.
func (idx *Index) AddPattern(pattern string) (int, []error) {
	paths, err := Discover(pattern, idx.root)
	if err != nil {
		return 0, []error{err}
	}

	var errors []error
	count := 0

	for _, path := range paths {
		if err := idx.Add(path); err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", path, err))
			continue
		}
		count++
	}

	return count, errors
}

// Files returns all indexed files.
func (idx *Index) Files() []*File {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	files := make([]*File, 0, len(idx.files))
	for _, f := range idx.files {
		files = append(files, f)
	}
	return files
}

// Get returns a specific file by path.
// Returns nil if the file is not in the index.
func (idx *Index) Get(path string) *File {
	relPath, err := idx.relativePath(path)
	if err != nil {
		return nil
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return idx.files[relPath]
}

// Root returns the root directory of the index.
func (idx *Index) Root() string {
	return idx.root
}

// Count returns the number of indexed files.
func (idx *Index) Count() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return len(idx.files)
}

// Clear removes all files from the index.
func (idx *Index) Clear() {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.files = make(map[string]*File)
}

// MatchFiles returns files matching a pattern.
// Patterns are matched against the indexed file paths.
// Supported patterns:
//   - "//..."          - all files
//   - "//pkg/..."      - all files under pkg/
//   - "//pkg:file.bzl" - specific file (label style)
//   - "*.star"         - glob in current directory
//   - "**/*.bzl"       - recursive glob
func (idx *Index) MatchFiles(pattern string) []*File {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var result []*File

	// Handle //... pattern (all files)
	if pattern == "//..." {
		for _, f := range idx.files {
			result = append(result, f)
		}
		return result
	}

	// Handle //path/... pattern (files under path)
	if strings.HasPrefix(pattern, "//") && strings.HasSuffix(pattern, "/...") {
		prefix := strings.TrimPrefix(pattern, "//")
		prefix = strings.TrimSuffix(prefix, "/...")
		if prefix == "" {
			// //... case already handled above
			for _, f := range idx.files {
				result = append(result, f)
			}
			return result
		}
		for _, f := range idx.files {
			if strings.HasPrefix(f.Path, prefix+"/") || f.Path == prefix {
				result = append(result, f)
			}
		}
		return result
	}

	// Handle //path:file pattern (label-style specific file)
	if strings.HasPrefix(pattern, "//") && strings.Contains(pattern, ":") {
		path := strings.TrimPrefix(pattern, "//")
		path = strings.Replace(path, ":", "/", 1)
		if f, ok := idx.files[path]; ok {
			result = append(result, f)
		}
		return result
	}

	// Handle //path/file pattern (specific file without colon)
	if strings.HasPrefix(pattern, "//") {
		path := strings.TrimPrefix(pattern, "//")
		if f, ok := idx.files[path]; ok {
			result = append(result, f)
		}
		return result
	}

	// Handle glob patterns
	if strings.Contains(pattern, "*") {
		// Check for recursive glob pattern (**)
		isRecursive := strings.Contains(pattern, "**")

		for _, f := range idx.files {
			if isRecursive {
				// For **/*.bzl, match the extension pattern against all files
				if strings.HasPrefix(pattern, "**/") {
					subPattern := strings.TrimPrefix(pattern, "**/")
					matched, err := filepath.Match(subPattern, filepath.Base(f.Path))
					if err == nil && matched {
						result = append(result, f)
					}
				}
			} else {
				// For non-recursive patterns like *.bzl, only match files at root level
				// (i.e., paths without directory separators)
				if !strings.Contains(f.Path, "/") {
					matched, err := filepath.Match(pattern, f.Path)
					if err == nil && matched {
						result = append(result, f)
					}
				}
			}
		}
		return result
	}

	// Direct path match
	if f, ok := idx.files[pattern]; ok {
		result = append(result, f)
	}

	return result
}

// relativePath converts an absolute path to a path relative to the index root.
func (idx *Index) relativePath(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return path, nil
	}

	rel, err := filepath.Rel(idx.root, path)
	if err != nil {
		return "", fmt.Errorf("making path relative: %w", err)
	}

	return rel, nil
}

// absolutePath converts a relative path to an absolute path based on the index root.
func (idx *Index) absolutePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(idx.root, path)
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
