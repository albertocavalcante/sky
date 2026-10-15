package index

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

// LoadGraph represents the load dependency graph.
// It tracks which files load which modules and enables
// both forward (file -> modules it loads) and reverse
// (module -> files that load it) lookups.
type LoadGraph struct {
	// Forward maps a file path to the modules it loads.
	Forward map[string][]string

	// Reverse maps a module to the files that load it.
	Reverse map[string][]string

	// reverseByPath indexes internal loads by normalized file path so any
	// accepted spelling of a module finds them.
	reverseByPath map[string][]string

	// lines maps a loading file to the resolved module label and the line of
	// the first direct load statement naming it. Forward only keeps resolved
	// labels, so this preserves where each direct load was written.
	lines map[string]map[string]int
}

// BuildLoadGraph builds the load graph from indexed files.
// It scans all files in the index and extracts load relationships.
func (idx *Index) BuildLoadGraph() *LoadGraph {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	g := &LoadGraph{
		Forward: make(map[string][]string),
		Reverse: make(map[string][]string),

		reverseByPath: make(map[string][]string),
		lines:         make(map[string]map[string]int),
	}

	for _, f := range idx.files {
		var modules []string
		for _, load := range f.Loads {
			// Resolve package-relative labels (":x.bzl") against the loading
			// file's package so they are not confused with root-package files.
			module := resolveRelativeLabel(idx.packageDir(f.Path), load.Module)
			modules = append(modules, module)
			if g.lines[f.Path] == nil {
				g.lines[f.Path] = make(map[string]int)
			}
			if _, seen := g.lines[f.Path][module]; !seen {
				g.lines[f.Path][module] = load.Line
			}
			key := canonicalLabel(module)
			g.Reverse[key] = append(g.Reverse[key], f.Path)
			if p := moduleToPath(key); p != "" {
				g.reverseByPath[p] = append(g.reverseByPath[p], f.Path)
			}
		}
		if len(modules) > 0 {
			g.Forward[f.Path] = modules
		}
	}

	return g
}

// LoadedBy returns the file paths that load the given module.
// The module can be specified in label format (e.g., "//lib:utils.bzl").
func (g *LoadGraph) LoadedBy(module string) []string {
	if g == nil {
		return nil
	}
	module = normalizeLookup(module)
	key := canonicalLabel(module)

	// A module can be spelled several ways ("//pkg:sub/base.bzl" and
	// "//pkg/sub:base.bzl" name the same file), and loaders may have used any
	// of them, so merge the exact-label and normalized-path indexes.
	var result []string
	seen := make(map[string]bool)
	add := func(files []string) {
		for _, f := range files {
			if !seen[f] {
				seen[f] = true
				result = append(result, f)
			}
		}
	}
	add(g.Reverse[module])
	add(g.Reverse[key])
	if g.reverseByPath != nil {
		if p := moduleToPath(key); p != "" {
			add(g.reverseByPath[p])
		}
	}
	return result
}

// LoadLine returns the 1-based line of the direct load statement in file that
// loads the resolved module label (as listed in Forward), or 0 if file has no
// such direct load.
func (g *LoadGraph) LoadLine(file, module string) int {
	if g == nil {
		return 0
	}
	return g.lines[normalizeLookup(file)][module]
}

// packageDir returns the slash-separated directory of the Bazel package that
// contains file: the nearest ancestor directory holding a BUILD or BUILD.bazel
// file (indexed or on disk). When there is none (plain Starlark trees), it
// falls back to the file's own directory. The caller must hold idx.mu.
func (idx *Index) packageDir(file string) string {
	file = filepath.ToSlash(file)
	dir := path.Dir(file)
	for d := dir; ; d = path.Dir(d) {
		prefix := d + "/"
		if d == "." {
			prefix = ""
		}
		for _, name := range []string{"BUILD.bazel", "BUILD"} {
			if _, ok := idx.files[prefix+name]; ok {
				return strings.TrimSuffix(prefix, "/")
			}
			if st, err := os.Stat(filepath.Join(idx.root, filepath.FromSlash(prefix+name))); err == nil && !st.IsDir() {
				return strings.TrimSuffix(prefix, "/")
			}
		}
		if d == "." || d == "/" {
			break
		}
	}
	if dir == "." {
		return ""
	}
	return dir
}

// resolveRelativeLabel resolves a package-relative label (":x.bzl") against
// the given package directory. Other labels are returned unchanged.
func resolveRelativeLabel(pkgDir, module string) string {
	if !strings.HasPrefix(module, ":") {
		return module
	}
	return "//" + pkgDir + module
}

// canonicalLabel normalizes a module reference to the "//pkg:name" form.
// File paths ("lib/utils.bzl") and slash labels ("//lib/utils.bzl") map to
// "//lib:utils.bzl". External labels and labels already containing a colon
// are returned unchanged (apart from a missing "//" prefix on pkg:name).
func canonicalLabel(module string) string {
	if strings.HasPrefix(module, "@") {
		return module
	}
	m := strings.TrimPrefix(module, "//")
	m = strings.TrimPrefix(m, "./")
	if strings.Contains(m, ":") {
		return "//" + m
	}
	dir, base := path.Split(m)
	return "//" + strings.TrimSuffix(dir, "/") + ":" + base
}

// AllLoads returns all transitive loads for a file.
// This includes direct loads and all modules that those loads depend on.
// Handles cycles gracefully by tracking visited modules.
func (g *LoadGraph) AllLoads(file string) []string {
	if g == nil {
		return nil
	}

	file = normalizeLookup(file)
	visited := make(map[string]bool)
	var result []string

	g.collectLoads(file, visited, &result)

	return result
}

// collectLoads is a helper that recursively collects all transitive loads.
func (g *LoadGraph) collectLoads(file string, visited map[string]bool, result *[]string) {
	modules, ok := g.Forward[file]
	if !ok {
		return
	}

	for _, module := range modules {
		if visited[module] {
			continue
		}
		visited[module] = true
		*result = append(*result, module)

		// Recursively collect loads from modules that are also indexed files.
		// Convert module label to file path for lookup.
		// Module labels like "//lib:utils.bzl" map to "lib/utils.bzl"
		filePath := moduleToPath(module)
		g.collectLoads(filePath, visited, result)
	}
}

// moduleToPath converts a module label to a file path.
// Examples:
//   - "//lib:utils.bzl" -> "lib/utils.bzl"
//   - "//pkg/sub:file.star" -> "pkg/sub/file.star"
//   - "//:utils.bzl" -> "utils.bzl"
//   - "@repo//lib:utils.bzl" -> "" (external repos not supported)
func moduleToPath(module string) string {
	// Skip external repository references
	if len(module) > 0 && module[0] == '@' {
		return ""
	}

	// Remove leading //
	if len(module) >= 2 && module[:2] == "//" {
		module = module[2:]
	}

	// Replace : with / (handling root package case where path before : is empty)
	for i := 0; i < len(module); i++ {
		if module[i] == ':' {
			if i == 0 {
				// Root package case: //:file.bzl -> file.bzl
				return module[1:]
			}
			return module[:i] + "/" + module[i+1:]
		}
	}

	return module
}

// DetectCycles detects cycles in the load graph.
// Returns a list of cycle paths, empty if no cycles are found.
// Each cycle is represented as a slice of file/module paths forming the cycle.
func (g *LoadGraph) DetectCycles() [][]string {
	if g == nil {
		return nil
	}

	var cycles [][]string
	visited := make(map[string]int) // 0: unvisited, 1: in-progress, 2: done
	var path []string

	var dfs func(file string)
	dfs = func(file string) {
		if visited[file] == 2 {
			return
		}
		if visited[file] == 1 {
			// Found a cycle - extract it from the path
			cycleStart := -1
			for i, p := range path {
				if p == file {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				cycle := make([]string, len(path)-cycleStart+1)
				copy(cycle, path[cycleStart:])
				cycle[len(cycle)-1] = file // Close the cycle
				cycles = append(cycles, cycle)
			}
			return
		}

		visited[file] = 1
		path = append(path, file)

		for _, module := range g.Forward[file] {
			// Convert module to file path for recursive check
			filePath := moduleToPath(module)
			if filePath != "" {
				dfs(filePath)
			}
		}

		path = path[:len(path)-1]
		visited[file] = 2
	}

	// Start DFS from all files in the graph
	for file := range g.Forward {
		if visited[file] == 0 {
			dfs(file)
		}
	}

	return cycles
}
