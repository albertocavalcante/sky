package index

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestIsStarlarkFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		// Exact filenames
		{"BUILD", true},
		{"BUILD.bazel", true},
		{"WORKSPACE", true},
		{"WORKSPACE.bazel", true},
		{"MODULE.bazel", true},
		{"BUCK", true},
		{"Tiltfile", true},

		// Extensions
		{"defs.bzl", true},
		{"queries.bxl", true},
		{"script.star", true},
		{"config.starlark", true},
		{"copy.bara.sky", true},
		{"types.skyi", true},
		{"rules.axl", true},
		{"deploy.ipd", true},
		{"build.plz", true},
		{"config.pconf", true},
		{"helpers.pinc", true},
		{"mutable.mpconf", true},

		// Non-Starlark files
		{"README.md", false},
		{"main.go", false},
		{"script.py", false},
		{"config.json", false},
		{"somefile", false},
		{"build", false},      // lowercase
		{"workspace", false},  // lowercase
		{"BUILD_INFO", false}, // prefix
		{"WORKSPACE.lock", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsStarlarkFile(tt.name)
			if got != tt.want {
				t.Errorf("IsStarlarkFile(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestDiscover(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"BUILD":                     "",
		"defs.bzl":                  "",
		"script.star":               "",
		"README.md":                 "",
		"pkg/BUILD.bazel":           "",
		"pkg/rules.bzl":             "",
		"pkg/sub/BUILD":             "",
		"pkg/sub/utils.star":        "",
		"other/config.star":         "",
		".hidden/BUILD":             "", // Should be skipped
		".hidden/file.bzl":          "", // Should be skipped
		"internal/mod/WORKSPACE":    "",
		"internal/mod/MODULE.bazel": "",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", fullPath, err)
		}
	}

	tests := []struct {
		name        string
		pattern     string
		wantFiles   []string
		wantErr     bool
		checkSubset bool // Only check that these files are present (not exact match)
	}{
		{
			name:    "root recursive //...",
			pattern: "//...",
			wantFiles: []string{
				"BUILD",
				"defs.bzl",
				"script.star",
				"pkg/BUILD.bazel",
				"pkg/rules.bzl",
				"pkg/sub/BUILD",
				"pkg/sub/utils.star",
				"other/config.star",
				"internal/mod/WORKSPACE",
				"internal/mod/MODULE.bazel",
			},
		},
		{
			name:    "package recursive //pkg/...",
			pattern: "//pkg/...",
			wantFiles: []string{
				"pkg/BUILD.bazel",
				"pkg/rules.bzl",
				"pkg/sub/BUILD",
				"pkg/sub/utils.star",
			},
		},
		{
			name:      "specific file //defs.bzl",
			pattern:   "//defs.bzl",
			wantFiles: []string{"defs.bzl"},
		},
		{
			name:      "label syntax //pkg:rules.bzl",
			pattern:   "//pkg:rules.bzl",
			wantFiles: []string{"pkg/rules.bzl"},
		},
		{
			name:      "glob *.bzl",
			pattern:   "*.bzl",
			wantFiles: []string{"defs.bzl"},
		},
		{
			name:    "recursive glob **/*.bzl",
			pattern: "**/*.bzl",
			wantFiles: []string{
				"defs.bzl",
				"pkg/rules.bzl",
			},
		},
		{
			name:    "recursive glob **/*.star",
			pattern: "**/*.star",
			wantFiles: []string{
				"script.star",
				"pkg/sub/utils.star",
				"other/config.star",
			},
		},
		{
			name:      "non-existent file",
			pattern:   "//nonexistent.bzl",
			wantFiles: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Discover(tt.pattern, tmpDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("Discover() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Convert to relative paths for comparison
			var relPaths []string
			for _, p := range got {
				rel, err := filepath.Rel(tmpDir, p)
				if err != nil {
					t.Fatalf("Failed to make path relative: %v", err)
				}
				relPaths = append(relPaths, rel)
			}

			// Sort both for comparison
			sort.Strings(relPaths)
			sort.Strings(tt.wantFiles)

			if len(relPaths) != len(tt.wantFiles) {
				t.Errorf("Discover() returned %d files, want %d\ngot: %v\nwant: %v", len(relPaths), len(tt.wantFiles), relPaths, tt.wantFiles)
				return
			}

			for i, want := range tt.wantFiles {
				if relPaths[i] != want {
					t.Errorf("Discover()[%d] = %q, want %q", i, relPaths[i], want)
				}
			}
		})
	}
}

func TestDiscoverDirectory(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create files in the root
	files := []string{"BUILD", "rules.bzl", "README.md"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, f), []byte{}, 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	// Test discovering a directory (non-recursive)
	got, err := Discover("//", tmpDir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	// Convert to base names for comparison
	var baseNames []string
	for _, p := range got {
		baseNames = append(baseNames, filepath.Base(p))
	}
	sort.Strings(baseNames)

	want := []string{"BUILD", "rules.bzl"}
	sort.Strings(want)

	if len(baseNames) != len(want) {
		t.Errorf("Discover() returned %d files, want %d", len(baseNames), len(want))
		return
	}

	for i, w := range want {
		if baseNames[i] != w {
			t.Errorf("Discover()[%d] = %q, want %q", i, baseNames[i], w)
		}
	}
}

func TestDiscoverHiddenDirectories(t *testing.T) {
	// Create a temporary directory with hidden directories
	tmpDir := t.TempDir()

	// Create hidden directory with files
	hiddenDir := filepath.Join(tmpDir, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatalf("Failed to create hidden directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "BUILD"), []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Create normal file
	if err := os.WriteFile(filepath.Join(tmpDir, "BUILD"), []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Discover should skip hidden directories
	got, err := Discover("//...", tmpDir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	// Should only find the non-hidden BUILD file
	if len(got) != 1 {
		t.Errorf("Discover() returned %d files, want 1 (hidden dir should be skipped)", len(got))
	}

	if len(got) > 0 && filepath.Base(got[0]) != "BUILD" {
		t.Errorf("Discover() returned %q, want BUILD", filepath.Base(got[0]))
	}
}

func TestDiscoverEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	got, err := Discover("//...", tmpDir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(got) != 0 {
		t.Errorf("Discover() in empty dir returned %d files, want 0", len(got))
	}
}

func BenchmarkIsStarlarkFile(b *testing.B) {
	names := []string{
		"BUILD",
		"BUILD.bazel",
		"defs.bzl",
		"script.star",
		"README.md",
		"main.go",
		"config.json",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		name := names[i%len(names)]
		IsStarlarkFile(name)
	}
}
