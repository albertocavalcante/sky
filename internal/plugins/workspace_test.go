package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindWorkspaceRootFrom(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(dir string) // creates marker files
		startSubdir   string           // relative subdir to start from
		expectRelRoot string           // expected root relative to temp dir
	}{
		{
			name: "sky config file",
			setup: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, ".sky.yaml"), []byte(""), 0644)
			},
			startSubdir:   "",
			expectRelRoot: "",
		},
		{
			name: "sky yml config file",
			setup: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, ".sky.yml"), []byte(""), 0644)
			},
			startSubdir:   "",
			expectRelRoot: "",
		},
		{
			name: "git directory",
			setup: func(dir string) {
				_ = os.Mkdir(filepath.Join(dir, ".git"), 0755)
			},
			startSubdir:   "",
			expectRelRoot: "",
		},
		{
			name: "nested subdir with git root",
			setup: func(dir string) {
				_ = os.Mkdir(filepath.Join(dir, ".git"), 0755)
				_ = os.MkdirAll(filepath.Join(dir, "src", "pkg"), 0755)
			},
			startSubdir:   "src/pkg",
			expectRelRoot: "",
		},
		{
			name: "sky config takes precedence over git",
			setup: func(dir string) {
				_ = os.Mkdir(filepath.Join(dir, ".git"), 0755)
				_ = os.WriteFile(filepath.Join(dir, ".sky.yaml"), []byte(""), 0644)
			},
			startSubdir:   "",
			expectRelRoot: "",
		},
		{
			name: "no markers returns start dir",
			setup: func(dir string) {
				_ = os.MkdirAll(filepath.Join(dir, "some", "nested", "dir"), 0755)
			},
			startSubdir:   "some/nested/dir",
			expectRelRoot: "some/nested/dir",
		},
		{
			name: "nested sky config",
			setup: func(dir string) {
				_ = os.MkdirAll(filepath.Join(dir, "project", "src"), 0755)
				_ = os.WriteFile(filepath.Join(dir, "project", ".sky.yaml"), []byte(""), 0644)
			},
			startSubdir:   "project/src",
			expectRelRoot: "project",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tc.setup(tmpDir)

			startDir := filepath.Join(tmpDir, tc.startSubdir)
			expectedRoot := filepath.Join(tmpDir, tc.expectRelRoot)

			got := FindWorkspaceRootFrom(startDir)
			if got != expectedRoot {
				t.Errorf("FindWorkspaceRootFrom(%q) = %q, want %q", startDir, got, expectedRoot)
			}
		})
	}
}

func TestFindWorkspaceRoot(t *testing.T) {
	// Save and restore working directory
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	// Resolve symlinks for macOS (/var -> /private/var)
	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	_ = os.WriteFile(filepath.Join(tmpDir, ".sky.yaml"), []byte(""), 0644)
	_ = os.MkdirAll(filepath.Join(tmpDir, "sub", "dir"), 0755)

	if err := os.Chdir(filepath.Join(tmpDir, "sub", "dir")); err != nil {
		t.Fatal(err)
	}

	got := FindWorkspaceRoot()
	if got != tmpDir {
		t.Errorf("FindWorkspaceRoot() = %q, want %q", got, tmpDir)
	}
}
