package loader

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/albertocavalcante/sky/internal/starlark/builtins"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// TestNewJSONProvider verifies that NewJSONProvider correctly initializes the provider.
func TestNewJSONProvider(t *testing.T) {
	provider := NewJSONProvider()
	if provider == nil {
		t.Fatal("NewJSONProvider returned nil")
	}

	if provider.cache == nil {
		t.Error("cache not initialized")
	}

	if provider.dataFS == nil {
		t.Error("dataFS not initialized")
	}
}

// TestJSONSupportedDialects verifies the list of supported dialects.
func TestJSONSupportedDialects(t *testing.T) {
	provider := newTestJSONProvider()
	dialects := provider.SupportedDialects()

	expected := []string{"bazel", "buck2", "starlark"}
	if len(dialects) != len(expected) {
		t.Errorf("Expected %d dialects, got %d", len(expected), len(dialects))
	}

	// Check each expected dialect is present
	dialectMap := make(map[string]bool)
	for _, d := range dialects {
		dialectMap[d] = true
	}

	for _, exp := range expected {
		if !dialectMap[exp] {
			t.Errorf("Expected dialect %q not found in supported dialects", exp)
		}
	}
}

// TestJSONFilename verifies dialect and file kind mapping to filenames.
func TestJSONFilename(t *testing.T) {
	provider := newTestJSONProvider()

	tests := []struct {
		name     string
		dialect  string
		kind     filekind.Kind
		expected string
	}{
		// Bazel file kinds
		{
			name:     "bazel BUILD",
			dialect:  "bazel",
			kind:     filekind.KindBUILD,
			expected: "data/json/bazel-build.json",
		},
		{
			name:     "bazel bzl",
			dialect:  "bazel",
			kind:     filekind.KindBzl,
			expected: "data/json/bazel-bzl.json",
		},
		{
			name:     "bazel WORKSPACE",
			dialect:  "bazel",
			kind:     filekind.KindWORKSPACE,
			expected: "data/json/bazel-workspace.json",
		},
		{
			name:     "bazel MODULE",
			dialect:  "bazel",
			kind:     filekind.KindMODULE,
			expected: "data/json/bazel-module.json",
		},
		{
			name:     "bazel bzlmod",
			dialect:  "bazel",
			kind:     filekind.KindBzlmod,
			expected: "data/json/bazel-bzlmod.json",
		},
		// Buck2 file kinds
		{
			name:     "buck2 BUCK",
			dialect:  "buck2",
			kind:     filekind.KindBUCK,
			expected: "data/json/buck2-buck.json",
		},
		{
			name:     "buck2 bzl",
			dialect:  "buck2",
			kind:     filekind.KindBzlBuck,
			expected: "data/json/buck2-bzl.json",
		},
		{
			name:     "buck2 buckconfig",
			dialect:  "buck2",
			kind:     filekind.KindBuckconfig,
			expected: "data/json/buck2-buckconfig.json",
		},
		// Starlark file kinds
		{
			name:     "starlark generic",
			dialect:  "starlark",
			kind:     filekind.KindStarlark,
			expected: "data/json/starlark-core.json",
		},
		{
			name:     "starlark skyi",
			dialect:  "starlark",
			kind:     filekind.KindSkyI,
			expected: "data/json/starlark-skyi.json",
		},
		// Case insensitivity
		{
			name:     "BAZEL uppercase",
			dialect:  "BAZEL",
			kind:     filekind.KindBUILD,
			expected: "data/json/bazel-build.json",
		},
		{
			name:     "Bazel mixed case",
			dialect:  "Bazel",
			kind:     filekind.KindBUILD,
			expected: "data/json/bazel-build.json",
		},
		// Unsupported combinations
		{
			name:     "unsupported dialect",
			dialect:  "unknown",
			kind:     filekind.KindBUILD,
			expected: "",
		},
		{
			name:     "unsupported kind for bazel",
			dialect:  "bazel",
			kind:     filekind.KindBUCK,
			expected: "",
		},
		{
			name:     "unsupported kind for buck2",
			dialect:  "buck2",
			kind:     filekind.KindBUILD,
			expected: "",
		},
		{
			name:     "unsupported kind for starlark",
			dialect:  "starlark",
			kind:     filekind.KindBUILD,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.jsonFilename(tt.dialect, tt.kind)
			if result != tt.expected {
				t.Errorf("jsonFilename(%q, %q) = %q, want %q",
					tt.dialect, tt.kind, result, tt.expected)
			}
		})
	}
}

// TestParseJSONFile verifies parsing of JSON format.
func TestParseJSONFile(t *testing.T) {
	_ = newTestJSONProvider() // TODO: Use provider in tests

	t.Run("parse valid JSON", func(t *testing.T) {
		testJSON := `{
  "functions": [
    {
      "name": "test_func",
      "doc": "Test function",
      "params": [
        {
          "name": "arg1",
          "type": "str",
          "required": true
        }
      ],
      "return_type": "None"
    }
  ],
  "types": [
    {
      "name": "TestType",
      "doc": "Test type",
      "fields": [
        {
          "name": "field1",
          "type": "str"
        }
      ],
      "methods": []
    }
  ],
  "globals": [
    {
      "name": "TEST_CONST",
      "type": "str",
      "doc": "Test constant"
    }
  ]
}`

		var result builtins.Builtins
		err := json.Unmarshal([]byte(testJSON), &result)
		if err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		if len(result.Functions) != 1 {
			t.Errorf("Expected 1 function, got %d", len(result.Functions))
		}
		if len(result.Types) != 1 {
			t.Errorf("Expected 1 type, got %d", len(result.Types))
		}
		if len(result.Globals) != 1 {
			t.Errorf("Expected 1 global, got %d", len(result.Globals))
		}
	})

	t.Run("parse invalid JSON", func(t *testing.T) {
		invalidJSON := `{"functions": [invalid json}`

		var result builtins.Builtins
		err := json.Unmarshal([]byte(invalidJSON), &result)
		if err == nil {
			t.Error("Expected error parsing invalid JSON, got nil")
		}
	})

	t.Run("parse empty JSON", func(t *testing.T) {
		emptyJSON := `{}`

		var result builtins.Builtins
		err := json.Unmarshal([]byte(emptyJSON), &result)
		if err != nil {
			t.Fatalf("Failed to parse empty JSON: %v", err)
		}

		// Empty slices should be nil (not initialized by JSON unmarshaling)
		if result.Functions != nil {
			t.Errorf("Expected nil functions, got %v", result.Functions)
		}
	})
}

// TestJSONBuiltins_Interface verifies the main Builtins interface method.
func TestJSONBuiltins_Interface(t *testing.T) {
	provider := newTestJSONProvider()

	// Create test JSON data
	testJSON := `{
  "functions": [
    {
      "name": "test_func",
      "doc": "Test function",
      "params": [
        {
          "name": "arg1",
          "type": "str",
          "required": true
        }
      ],
      "return_type": "str"
    }
  ],
  "types": [],
  "globals": []
}`

	// Add test data to embedded filesystem
	provider.injectTestData("data/json/starlark-core.json", []byte(testJSON))

	t.Run("load valid JSON", func(t *testing.T) {
		result, err := provider.Builtins("starlark", filekind.KindStarlark)
		if err != nil {
			t.Fatalf("Builtins failed: %v", err)
		}

		if len(result.Functions) != 1 {
			t.Errorf("Expected 1 function, got %d", len(result.Functions))
		}
		if result.Functions[0].Name != "test_func" {
			t.Errorf("Expected function name 'test_func', got %q", result.Functions[0].Name)
		}
	})

	t.Run("unsupported dialect", func(t *testing.T) {
		_, err := provider.Builtins("unknown", filekind.KindBUILD)
		if err == nil {
			t.Error("Expected error for unsupported dialect, got nil")
		}
	})

	t.Run("unsupported file kind", func(t *testing.T) {
		_, err := provider.Builtins("bazel", filekind.KindBUCK)
		if err == nil {
			t.Error("Expected error for unsupported file kind, got nil")
		}
	})

	t.Run("missing JSON file", func(t *testing.T) {
		// Create a new provider without test data
		emptyProvider := NewJSONProvider()
		_, err := emptyProvider.Builtins("bazel", filekind.KindBUILD)
		if err == nil {
			t.Error("Expected error for missing JSON file, got nil")
		}
	})
}

// TestJSONBuiltins_Caching verifies that the cache works correctly.
func TestJSONBuiltins_Caching(t *testing.T) {
	provider := newTestJSONProvider()

	testJSON := `{
  "functions": [
    {
      "name": "test_func",
      "return_type": "str"
    }
  ]
}`

	provider.injectTestData("data/json/starlark-core.json", []byte(testJSON))

	// First load
	result1, err := provider.Builtins("starlark", filekind.KindStarlark)
	if err != nil {
		t.Fatalf("First Builtins call failed: %v", err)
	}

	// Verify cache was populated
	if _, ok := provider.cache["starlark"]; !ok {
		t.Error("Cache not populated for starlark dialect")
	}
	if _, ok := provider.cache["starlark"][filekind.KindStarlark]; !ok {
		t.Error("Cache not populated for Starlark kind")
	}

	// Modify the dataFS to ensure cache is used
	provider.injectTestData("data/json/starlark-core.json", []byte("invalid json"))

	// Second load should use cache
	result2, err := provider.Builtins("starlark", filekind.KindStarlark)
	if err != nil {
		t.Fatalf("Second Builtins call failed: %v", err)
	}

	// Results should be identical (cached)
	if diff := cmp.Diff(result1, result2); diff != "" {
		t.Errorf("Cached result differs from original (-want +got):\n%s", diff)
	}

	// Verify we got the same data even though we corrupted the source
	if len(result2.Functions) != 1 {
		t.Error("Cache not used; got reparsed corrupt data")
	}
}

// TestJSONBuiltins_ConcurrentAccess verifies thread-safe cache access.
func TestJSONBuiltins_ConcurrentAccess(t *testing.T) {
	provider := newTestJSONProvider()

	testJSON := `{
  "functions": [
    {
      "name": "test_func",
      "return_type": "str"
    }
  ]
}`

	provider.injectTestData("data/json/starlark-core.json", []byte(testJSON))

	// Run multiple goroutines accessing the same data
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			_, err := provider.Builtins("starlark", filekind.KindStarlark)
			if err != nil {
				errors <- err
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	close(errors)
	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}
}

// TestJSONBuiltins_AllDialectsAndKinds verifies all supported dialect/kind combinations.
func TestJSONBuiltins_AllDialectsAndKinds(t *testing.T) {
	provider := newTestJSONProvider()

	// Create minimal test JSON data
	testJSON := `{}`

	combinations := []struct {
		dialect  string
		kind     filekind.Kind
		filename string
	}{
		{"bazel", filekind.KindBUILD, "data/json/bazel-build.json"},
		{"bazel", filekind.KindBzl, "data/json/bazel-bzl.json"},
		{"bazel", filekind.KindWORKSPACE, "data/json/bazel-workspace.json"},
		{"bazel", filekind.KindMODULE, "data/json/bazel-module.json"},
		{"bazel", filekind.KindBzlmod, "data/json/bazel-bzlmod.json"},
		{"buck2", filekind.KindBUCK, "data/json/buck2-buck.json"},
		{"buck2", filekind.KindBzlBuck, "data/json/buck2-bzl.json"},
		{"buck2", filekind.KindBuckconfig, "data/json/buck2-buckconfig.json"},
		{"starlark", filekind.KindStarlark, "data/json/starlark-core.json"},
		{"starlark", filekind.KindSkyI, "data/json/starlark-skyi.json"},
	}

	for _, combo := range combinations {
		t.Run(combo.dialect+"/"+combo.kind.String(), func(t *testing.T) {
			// Add test data for this combination
			provider.injectTestData(combo.filename, []byte(testJSON))

			// Should not return error
			result, err := provider.Builtins(combo.dialect, combo.kind)
			if err != nil {
				t.Errorf("Builtins(%q, %q) failed: %v", combo.dialect, combo.kind, err)
			}

			// Result should be valid (slices can be nil for empty JSON)
			_ = result
		})
	}
}

// TestIntegration_ComprehensiveJSON verifies end-to-end functionality with a realistic JSON.
func TestIntegration_ComprehensiveJSON(t *testing.T) {
	provider := newTestJSONProvider()

	// Load the comprehensive test fixture
	testJSON := `{
  "functions": [
    {
      "name": "glob",
      "doc": "Returns files matching a pattern",
      "params": [
        {
          "name": "include",
          "type": "list[str]",
          "required": true
        },
        {
          "name": "exclude",
          "type": "list[str]",
          "default": "[]",
          "required": false
        }
      ],
      "return_type": "list[str]"
    },
    {
      "name": "print",
      "doc": "Prints values",
      "params": [
        {
          "name": "args",
          "type": "any",
          "variadic": true
        },
        {
          "name": "sep",
          "type": "str",
          "default": "\" \"",
          "required": false
        }
      ],
      "return_type": "None"
    }
  ],
  "types": [
    {
      "name": "File",
      "doc": "A Starlark file object",
      "fields": [
        {
          "name": "path",
          "type": "str",
          "doc": "The file path"
        },
        {
          "name": "basename",
          "type": "str",
          "doc": "The base filename"
        }
      ],
      "methods": []
    },
    {
      "name": "Provider",
      "doc": "A provider that supplies information",
      "fields": [
        {
          "name": "name",
          "type": "str",
          "doc": "Provider name"
        }
      ],
      "methods": [
        {
          "name": "get_value",
          "doc": "Get a value from the provider",
          "params": [
            {
              "name": "key",
              "type": "str",
              "required": true
            },
            {
              "name": "default",
              "type": "any",
              "default": "None",
              "required": false
            }
          ],
          "return_type": "any"
        }
      ]
    }
  ],
  "globals": [
    {
      "name": "True",
      "type": "bool",
      "doc": "Boolean true constant"
    },
    {
      "name": "False",
      "type": "bool",
      "doc": "Boolean false constant"
    },
    {
      "name": "WORKSPACE_ROOT",
      "type": "str",
      "doc": "The path to the workspace root"
    }
  ]
}`

	provider.injectTestData("data/json/starlark-core.json", []byte(testJSON))

	// Load builtins
	result, err := provider.Builtins("starlark", filekind.KindStarlark)
	if err != nil {
		t.Fatalf("Builtins failed: %v", err)
	}

	// Verify types
	if len(result.Types) != 2 {
		t.Errorf("Expected 2 types, got %d", len(result.Types))
	}

	// Verify File type
	fileType := findType(result.Types, "File")
	if fileType == nil {
		t.Fatal("File type not found")
	}
	if len(fileType.Fields) != 2 {
		t.Errorf("File: expected 2 fields, got %d", len(fileType.Fields))
	}
	if len(fileType.Methods) != 0 {
		t.Errorf("File: expected 0 methods, got %d", len(fileType.Methods))
	}

	// Verify Provider type
	providerType := findType(result.Types, "Provider")
	if providerType == nil {
		t.Fatal("Provider type not found")
	}
	if len(providerType.Fields) != 1 {
		t.Errorf("Provider: expected 1 field, got %d", len(providerType.Fields))
	}
	if len(providerType.Methods) != 1 {
		t.Errorf("Provider: expected 1 method, got %d", len(providerType.Methods))
	}
	if providerType.Methods[0].Name != "get_value" {
		t.Errorf("Provider method: expected 'get_value', got %q", providerType.Methods[0].Name)
	}

	// Verify functions
	if len(result.Functions) != 2 {
		t.Errorf("Expected 2 functions, got %d", len(result.Functions))
	}

	globFunc := findFunction(result.Functions, "glob")
	if globFunc == nil {
		t.Fatal("glob function not found")
	}
	if len(globFunc.Params) != 2 {
		t.Errorf("glob: expected 2 params, got %d", len(globFunc.Params))
	}

	printFunc := findFunction(result.Functions, "print")
	if printFunc == nil {
		t.Fatal("print function not found")
	}
	if len(printFunc.Params) != 2 {
		t.Errorf("print: expected 2 params, got %d", len(printFunc.Params))
	}
	if !printFunc.Params[0].Variadic {
		t.Error("print: first param should be variadic")
	}

	// Verify globals
	if len(result.Globals) != 3 {
		t.Errorf("Expected 3 globals, got %d", len(result.Globals))
	}

	trueGlobal := findGlobal(result.Globals, "True")
	if trueGlobal == nil {
		t.Fatal("True global not found")
	}
	if trueGlobal.Type != "bool" {
		t.Errorf("True: expected type 'bool', got %q", trueGlobal.Type)
	}
}

// TestJSONSchemaMapping verifies that JSON schema maps directly to Builtins struct.
func TestJSONSchemaMapping(t *testing.T) {
	// Create a complete builtins struct
	original := builtins.Builtins{
		Functions: []builtins.Signature{
			{
				Name:       "test_func",
				Doc:        "Test function",
				ReturnType: "str",
				Params: []builtins.Param{
					{
						Name:     "arg1",
						Type:     "str",
						Required: true,
					},
					{
						Name:     "arg2",
						Type:     "int",
						Default:  "0",
						Required: false,
					},
					{
						Name:     "args",
						Type:     "any",
						Variadic: true,
					},
					{
						Name:   "kwargs",
						Type:   "any",
						KWArgs: true,
					},
				},
			},
		},
		Types: []builtins.TypeDef{
			{
				Name: "TestType",
				Doc:  "Test type",
				Fields: []builtins.Field{
					{
						Name: "field1",
						Type: "str",
						Doc:  "Field 1",
					},
				},
				Methods: []builtins.Signature{
					{
						Name:       "method1",
						Doc:        "Method 1",
						ReturnType: "None",
						Params: []builtins.Param{
							{
								Name:     "param",
								Type:     "str",
								Required: true,
							},
						},
					},
				},
			},
		},
		Globals: []builtins.Field{
			{
				Name: "GLOBAL1",
				Type: "str",
				Doc:  "Global 1",
			},
		},
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal to JSON: %v", err)
	}

	// Unmarshal back
	var roundtrip builtins.Builtins
	if err := json.Unmarshal(jsonData, &roundtrip); err != nil {
		t.Fatalf("Failed to unmarshal from JSON: %v", err)
	}

	// Compare
	if diff := cmp.Diff(original, roundtrip); diff != "" {
		t.Errorf("Roundtrip mismatch (-want +got):\n%s", diff)
	}
}

// BenchmarkJSONLoader_FirstLoad measures cold load performance.
func BenchmarkJSONLoader_FirstLoad(b *testing.B) {
	testJSON := `{
  "functions": [
    {
      "name": "test_func",
      "params": [
        {"name": "arg1", "type": "str", "required": true},
        {"name": "arg2", "type": "int", "default": "0"}
      ],
      "return_type": "str"
    }
  ],
  "types": [
    {
      "name": "TestType",
      "fields": [
        {"name": "field1", "type": "str"},
        {"name": "field2", "type": "int"}
      ],
      "methods": []
    }
  ],
  "globals": []
}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		provider := newTestJSONProvider()
		provider.injectTestData("data/json/starlark-core.json", []byte(testJSON))
		b.StartTimer()

		_, err := provider.Builtins("starlark", filekind.KindStarlark)
		if err != nil {
			b.Fatalf("Builtins failed: %v", err)
		}
	}
}

// BenchmarkJSONLoader_CachedLoad measures cached load performance.
func BenchmarkJSONLoader_CachedLoad(b *testing.B) {
	testJSON := `{
  "functions": [
    {
      "name": "test_func",
      "params": [
        {"name": "arg1", "type": "str", "required": true}
      ],
      "return_type": "str"
    }
  ]
}`

	provider := newTestJSONProvider()
	provider.injectTestData("data/json/starlark-core.json", []byte(testJSON))

	// Warm up the cache
	_, err := provider.Builtins("starlark", filekind.KindStarlark)
	if err != nil {
		b.Fatalf("Failed to warm up cache: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := provider.Builtins("starlark", filekind.KindStarlark)
		if err != nil {
			b.Fatalf("Builtins failed: %v", err)
		}
	}
}

// BenchmarkJSONUnmarshal measures JSON unmarshaling performance.
func BenchmarkJSONUnmarshal(b *testing.B) {
	testJSON := []byte(`{
  "functions": [
    {
      "name": "func1",
      "params": [
        {"name": "arg1", "type": "str", "required": true},
        {"name": "arg2", "type": "int", "default": "0"}
      ],
      "return_type": "str"
    },
    {
      "name": "func2",
      "params": [{"name": "arg", "type": "any", "variadic": true}],
      "return_type": "None"
    }
  ],
  "types": [
    {
      "name": "Type1",
      "fields": [
        {"name": "field1", "type": "str"},
        {"name": "field2", "type": "int"}
      ],
      "methods": [
        {
          "name": "method1",
          "params": [{"name": "arg", "type": "str"}],
          "return_type": "str"
        }
      ]
    }
  ],
  "globals": [
    {"name": "CONST1", "type": "str"},
    {"name": "CONST2", "type": "int"}
  ]
}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result builtins.Builtins
		if err := json.Unmarshal(testJSON, &result); err != nil {
			b.Fatalf("Unmarshal failed: %v", err)
		}
	}
}
