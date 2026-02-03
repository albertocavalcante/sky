package plugins

import (
	"os"
	"path/filepath"
)

// WorkspaceMarkers are files that indicate the root of a workspace.
var WorkspaceMarkers = []string{
	".sky.yaml",
	".sky.yml",
	".git",
}

// FindWorkspaceRoot locates the workspace root by searching for marker files.
// It searches upward from the current working directory for:
//  1. .sky.yaml or .sky.yml (Sky config files)
//  2. .git directory (version control root)
//
// If no markers are found, it returns the current working directory.
func FindWorkspaceRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return FindWorkspaceRootFrom(cwd)
}

// FindWorkspaceRootFrom locates the workspace root starting from the given directory.
func FindWorkspaceRootFrom(startDir string) string {
	dir := startDir
	for {
		for _, marker := range WorkspaceMarkers {
			path := filepath.Join(dir, marker)
			if _, err := os.Stat(path); err == nil {
				return dir
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root, return start directory
			return startDir
		}
		dir = parent
	}
}
