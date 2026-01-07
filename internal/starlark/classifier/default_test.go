package classifier

import (
	"testing"

	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

func TestDefaultClassifier_Classify(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		wantDialect  string
		wantFileKind filekind.Kind
		wantErr      bool
	}{
		// Bazel BUILD files
		{
			name:         "BUILD file",
			path:         "BUILD",
			wantDialect:  "bazel",
			wantFileKind: filekind.KindBUILD,
		},
		{
			name:         "BUILD.bazel file",
			path:         "BUILD.bazel",
			wantDialect:  "bazel",
			wantFileKind: filekind.KindBUILD,
		},
		{
			name:         "BUILD in directory",
			path:         "pkg/foo/BUILD",
			wantDialect:  "bazel",
			wantFileKind: filekind.KindBUILD,
		},
		{
			name:         "BUILD.bazel in directory",
			path:         "pkg/bar/BUILD.bazel",
			wantDialect:  "bazel",
			wantFileKind: filekind.KindBUILD,
		},
		{
			name:         "absolute path to BUILD",
			path:         "/workspace/pkg/BUILD",
			wantDialect:  "bazel",
			wantFileKind: filekind.KindBUILD,
		},

		// Bazel .bzl files
		{
			name:         ".bzl file",
			path:         "defs.bzl",
			wantDialect:  "bazel",
			wantFileKind: filekind.KindBzl,
		},
		{
			name:         ".bzl file in directory",
			path:         "tools/build_defs.bzl",
			wantDialect:  "bazel",
			wantFileKind: filekind.KindBzl,
		},
		{
			name:         "absolute path to .bzl",
			path:         "/workspace/third_party/rules.bzl",
			wantDialect:  "bazel",
			wantFileKind: filekind.KindBzl,
		},

		// Bazel WORKSPACE files
		{
			name:         "WORKSPACE file",
			path:         "WORKSPACE",
			wantDialect:  "bazel",
			wantFileKind: filekind.KindWORKSPACE,
		},
		{
			name:         "WORKSPACE.bazel file",
			path:         "WORKSPACE.bazel",
			wantDialect:  "bazel",
			wantFileKind: filekind.KindWORKSPACE,
		},
		{
			name:         "WORKSPACE in subdirectory",
			path:         "external/repo/WORKSPACE",
			wantDialect:  "bazel",
			wantFileKind: filekind.KindWORKSPACE,
		},

		// Bazel MODULE.bazel files
		{
			name:         "MODULE.bazel file",
			path:         "MODULE.bazel",
			wantDialect:  "bazel",
			wantFileKind: filekind.KindMODULE,
		},
		{
			name:         "MODULE.bazel in subdirectory",
			path:         "external/mod/MODULE.bazel",
			wantDialect:  "bazel",
			wantFileKind: filekind.KindMODULE,
		},

		// Buck2 BUCK files
		{
			name:         "BUCK file",
			path:         "BUCK",
			wantDialect:  "buck2",
			wantFileKind: filekind.KindBUCK,
		},
		{
			name:         "BUCK in directory",
			path:         "src/app/BUCK",
			wantDialect:  "buck2",
			wantFileKind: filekind.KindBUCK,
		},
		{
			name:         "absolute path to BUCK",
			path:         "/workspace/lib/BUCK",
			wantDialect:  "buck2",
			wantFileKind: filekind.KindBUCK,
		},

		// Generic Starlark files
		{
			name:         ".star file",
			path:         "script.star",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindStarlark,
		},
		{
			name:         ".star file in directory",
			path:         "scripts/build.star",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindStarlark,
		},
		{
			name:         "absolute path to .star",
			path:         "/home/user/config.star",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindStarlark,
		},

		// Type stub files
		{
			name:         ".skyi file",
			path:         "types.skyi",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindSkyI,
		},
		{
			name:         ".skyi file in directory",
			path:         "stubs/bazel.skyi",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindSkyI,
		},

		// Other Starlark file variants
		{
			name:         ".sky file",
			path:         "config.sky",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindStarlark,
		},
		{
			name:         ".bara.sky file (Copybara)",
			path:         "copy.bara.sky",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindStarlark,
		},
		{
			name:         ".axl file",
			path:         "rules.axl",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindStarlark,
		},
		{
			name:         ".axl file in directory",
			path:         "config/app.axl",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindStarlark,
		},

		// Tilt
		{
			name:         "Tiltfile",
			path:         "Tiltfile",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindStarlark,
		},
		{
			name:         "Tiltfile in directory",
			path:         "services/web/Tiltfile",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindStarlark,
		},

		// Isopod
		{
			name:         ".ipd file (Isopod)",
			path:         "deploy.ipd",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindStarlark,
		},

		// Please Build
		{
			name:         ".plz file (Please)",
			path:         "rules.plz",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindBUILD,
		},

		// Drone CI / Cirrus CI (compound extensions - .star suffix)
		{
			name:         ".drone.star file",
			path:         ".drone.star",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindStarlark,
		},
		{
			name:         ".cirrus.star file",
			path:         ".cirrus.star",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindStarlark,
		},

		// Buck2 BXL
		{
			name:         ".bxl file (Buck2 BXL)",
			path:         "queries.bxl",
			wantDialect:  "buck2",
			wantFileKind: filekind.KindBzlBuck,
		},

		// Protoconf
		{
			name:         ".pconf file (Protoconf)",
			path:         "config.pconf",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindStarlark,
		},
		{
			name:         ".pinc file (Protoconf include)",
			path:         "helpers.pinc",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindStarlark,
		},
		{
			name:         ".mpconf file (Protoconf mutable)",
			path:         "mutable.mpconf",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindStarlark,
		},

		// Full .starlark extension
		{
			name:         ".starlark file",
			path:         "script.starlark",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindStarlark,
		},

		// Edge cases - unknown files
		{
			name:         "unknown extension",
			path:         "README.md",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindUnknown,
		},
		{
			name:         "no extension",
			path:         "somefile",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindUnknown,
		},
		{
			name:         "python file",
			path:         "script.py",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindUnknown,
		},
		{
			name:         "empty path",
			path:         "",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindUnknown,
		},

		// Edge cases - case sensitivity
		{
			name:         "lowercase build",
			path:         "build",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindUnknown,
		},
		{
			name:         "lowercase workspace",
			path:         "workspace",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindUnknown,
		},
		{
			name:         "lowercase buck",
			path:         "buck",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindUnknown,
		},

		// Edge cases - partial matches
		{
			name:         "BUILD prefix",
			path:         "BUILD_INFO",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindUnknown,
		},
		{
			name:         "WORKSPACE prefix",
			path:         "WORKSPACE_CONFIG",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindUnknown,
		},
		{
			name:         "BUCK suffix",
			path:         "prebuck",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindUnknown,
		},

		// Edge cases - extensions with dots
		{
			name:         "multiple dots in name",
			path:         "config.build.bzl",
			wantDialect:  "bazel",
			wantFileKind: filekind.KindBzl,
		},
		{
			name:         "BUILD with extra extension",
			path:         "BUILD.old",
			wantDialect:  "starlark",
			wantFileKind: filekind.KindUnknown,
		},
	}

	classifier := NewDefaultClassifier()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := classifier.Classify(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Classify() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.Dialect != tt.wantDialect {
				t.Errorf("Classify() dialect = %v, want %v", got.Dialect, tt.wantDialect)
			}
			if got.FileKind != tt.wantFileKind {
				t.Errorf("Classify() fileKind = %v, want %v", got.FileKind, tt.wantFileKind)
			}
		})
	}
}

func TestDefaultClassifier_SupportsDialect(t *testing.T) {
	tests := []struct {
		name    string
		dialect string
		want    bool
	}{
		{
			name:    "supports bazel",
			dialect: "bazel",
			want:    true,
		},
		{
			name:    "supports buck2",
			dialect: "buck2",
			want:    true,
		},
		{
			name:    "supports starlark",
			dialect: "starlark",
			want:    true,
		},
		{
			name:    "unknown dialect",
			dialect: "python",
			want:    false,
		},
		{
			name:    "empty dialect",
			dialect: "",
			want:    false,
		},
	}

	classifier := NewDefaultClassifier()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifier.SupportsDialect(tt.dialect)
			if got != tt.want {
				t.Errorf("SupportsDialect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultClassifier_NilSafety(t *testing.T) {
	// Ensure NewDefaultClassifier returns a non-nil classifier
	classifier := NewDefaultClassifier()
	if classifier == nil {
		t.Fatal("NewDefaultClassifier() returned nil")
	}

	// Ensure Classify works on the returned classifier
	_, err := classifier.Classify("BUILD")
	if err != nil {
		t.Errorf("Classify() on new classifier failed: %v", err)
	}
}

func BenchmarkDefaultClassifier_Classify(b *testing.B) {
	classifier := NewDefaultClassifier()
	paths := []string{
		"BUILD",
		"BUILD.bazel",
		"WORKSPACE",
		"MODULE.bazel",
		"BUCK",
		"defs.bzl",
		"script.star",
		"types.skyi",
		"pkg/foo/BUILD",
		"tools/rules.bzl",
		"README.md",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := paths[i%len(paths)]
		_, err := classifier.Classify(path)
		if err != nil {
			b.Fatalf("Classify(%q) failed: %v", path, err)
		}
	}
}
