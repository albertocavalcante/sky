package index

// LoadGraph represents the load dependency graph.
// It tracks which files load which modules and enables
// both forward (file -> modules it loads) and reverse
// (module -> files that load it) lookups.
type LoadGraph struct {
	// Forward maps a file path to the modules it loads.
	Forward map[string][]string

	// Reverse maps a module to the files that load it.
	Reverse map[string][]string
}

// BuildLoadGraph builds the load graph from indexed files.
// It scans all files in the index and extracts load relationships.
func (idx *Index) BuildLoadGraph() *LoadGraph {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	g := &LoadGraph{
		Forward: make(map[string][]string),
		Reverse: make(map[string][]string),
	}

	for _, f := range idx.files {
		var modules []string
		for _, load := range f.Loads {
			modules = append(modules, load.Module)
			g.Reverse[load.Module] = append(g.Reverse[load.Module], f.Path)
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
	return g.Reverse[module]
}

// AllLoads returns all transitive loads for a file.
// This includes direct loads and all modules that those loads depend on.
// Handles cycles gracefully by tracking visited modules.
func (g *LoadGraph) AllLoads(file string) []string {
	if g == nil {
		return nil
	}

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
