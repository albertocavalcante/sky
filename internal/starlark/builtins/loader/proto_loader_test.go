package loader

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"

	"github.com/albertocavalcante/sky/internal/starlark/builtins"
	builtinspb "github.com/albertocavalcante/sky/internal/starlark/builtins/proto"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// TestNewProtoProvider verifies that NewProtoProvider correctly initializes the provider.
func TestNewProtoProvider(t *testing.T) {
	provider := NewProtoProvider()
	if provider == nil {
		t.Fatal("NewProtoProvider returned nil")
	}

	if provider.cache == nil {
		t.Error("cache not initialized")
	}

	if provider.dataFS == nil {
		t.Error("dataFS not initialized")
	}
}

// TestSupportedDialects verifies the list of supported dialects.
func TestSupportedDialects(t *testing.T) {
	provider := newTestProtoProvider()
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

// TestProtoFilename verifies dialect and file kind mapping to filenames.
func TestProtoFilename(t *testing.T) {
	provider := newTestProtoProvider()

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
			expected: "data/proto/bazel_build.pb",
		},
		{
			name:     "bazel bzl",
			dialect:  "bazel",
			kind:     filekind.KindBzl,
			expected: "data/proto/bazel_bzl.pb",
		},
		{
			name:     "bazel WORKSPACE",
			dialect:  "bazel",
			kind:     filekind.KindWORKSPACE,
			expected: "data/proto/bazel_workspace.pb",
		},
		{
			name:     "bazel MODULE",
			dialect:  "bazel",
			kind:     filekind.KindMODULE,
			expected: "data/proto/bazel_module.pb",
		},
		{
			name:     "bazel bzlmod",
			dialect:  "bazel",
			kind:     filekind.KindBzlmod,
			expected: "data/proto/bazel_bzlmod.pb",
		},
		// Buck2 file kinds
		{
			name:     "buck2 BUCK",
			dialect:  "buck2",
			kind:     filekind.KindBUCK,
			expected: "data/proto/buck2_buck.pb",
		},
		{
			name:     "buck2 bzl",
			dialect:  "buck2",
			kind:     filekind.KindBzlBuck,
			expected: "data/proto/buck2_bzl.pb",
		},
		{
			name:     "buck2 buckconfig",
			dialect:  "buck2",
			kind:     filekind.KindBuckconfig,
			expected: "data/proto/buck2_buckconfig.pb",
		},
		// Starlark file kinds
		{
			name:     "starlark generic",
			dialect:  "starlark",
			kind:     filekind.KindStarlark,
			expected: "data/proto/starlark_generic.pb",
		},
		{
			name:     "starlark skyi",
			dialect:  "starlark",
			kind:     filekind.KindSkyI,
			expected: "data/proto/starlark_skyi.pb",
		},
		// Case insensitivity
		{
			name:     "BAZEL uppercase",
			dialect:  "BAZEL",
			kind:     filekind.KindBUILD,
			expected: "data/proto/bazel_build.pb",
		},
		{
			name:     "Bazel mixed case",
			dialect:  "Bazel",
			kind:     filekind.KindBUILD,
			expected: "data/proto/bazel_build.pb",
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
			result := provider.protoFilename(tt.dialect, tt.kind)
			if result != tt.expected {
				t.Errorf("protoFilename(%q, %q) = %q, want %q",
					tt.dialect, tt.kind, result, tt.expected)
			}
		})
	}
}

// TestParseProtoFile verifies parsing of both binary and text proto formats.
func TestParseProtoFile(t *testing.T) {
	provider := newTestProtoProvider()

	// Create a test proto message
	testProto := &builtinspb.Builtins{
		Types: []*builtinspb.Type{
			{
				Name: "TestType",
				Doc:  "Test documentation",
				Fields: []*builtinspb.Field{
					{
						Name: "field1",
						Type: "str",
						Doc:  "Field documentation",
					},
				},
			},
		},
		Values: []*builtinspb.Value{
			{
				Name: "test_func",
				Type: "function",
				Doc:  "Function documentation",
				Callable: &builtinspb.Callable{
					Params: []*builtinspb.Param{
						{
							Name:        "arg1",
							Type:        "str",
							IsMandatory: true,
						},
					},
					ReturnType: "None",
				},
			},
		},
	}

	t.Run("parse binary proto", func(t *testing.T) {
		// Marshal to binary format
		data, err := proto.Marshal(testProto)
		if err != nil {
			t.Fatalf("Failed to marshal proto: %v", err)
		}

		// Parse it back
		result, err := provider.parseProtoFile(data, "test.pb")
		if err != nil {
			t.Fatalf("parseProtoFile failed: %v", err)
		}

		// Verify the result matches
		if !proto.Equal(result, testProto) {
			t.Error("Parsed proto does not match original")
		}
	})

	t.Run("parse text proto", func(t *testing.T) {
		// Use the actual test fixture
		data := []byte(`types {
  name: "TestType"
  doc: "Test documentation"
  fields {
    name: "field1"
    type: "str"
    doc: "Field documentation"
  }
}
`)

		result, err := provider.parseProtoFile(data, "test.pbtxt")
		if err != nil {
			t.Fatalf("parseProtoFile failed: %v", err)
		}

		if len(result.Types) != 1 {
			t.Errorf("Expected 1 type, got %d", len(result.Types))
		}
		if result.Types[0].Name != "TestType" {
			t.Errorf("Expected type name 'TestType', got %q", result.Types[0].Name)
		}
	})

	t.Run("parse invalid binary proto", func(t *testing.T) {
		data := []byte{0xFF, 0xFF, 0xFF, 0xFF} // Invalid proto data

		_, err := provider.parseProtoFile(data, "test.pb")
		if err == nil {
			t.Error("Expected error parsing invalid binary proto, got nil")
		}
	})

	t.Run("parse invalid text proto", func(t *testing.T) {
		data := []byte("this is not valid { proto text }")

		_, err := provider.parseProtoFile(data, "test.pbtxt")
		if err == nil {
			t.Error("Expected error parsing invalid text proto, got nil")
		}
	})
}

// TestConvertCallableToSignature verifies callable to signature conversion.
func TestConvertCallableToSignature(t *testing.T) {
	provider := newTestProtoProvider()

	tests := []struct {
		name     string
		funcName string
		callable *builtinspb.Callable
		expected builtins.Signature
	}{
		{
			name:     "simple function",
			funcName: "simple_func",
			callable: &builtinspb.Callable{
				Doc:        "Simple function doc",
				ReturnType: "str",
				Params: []*builtinspb.Param{
					{
						Name:        "arg1",
						Type:        "str",
						IsMandatory: true,
					},
				},
			},
			expected: builtins.Signature{
				Name:       "simple_func",
				Doc:        "Simple function doc",
				ReturnType: "str",
				Params: []builtins.Param{
					{
						Name:     "arg1",
						Type:     "str",
						Required: true,
					},
				},
			},
		},
		{
			name:     "function with optional params",
			funcName: "optional_func",
			callable: &builtinspb.Callable{
				ReturnType: "int",
				Params: []*builtinspb.Param{
					{
						Name:        "required",
						Type:        "str",
						IsMandatory: true,
					},
					{
						Name:         "optional",
						Type:         "int",
						DefaultValue: "42",
						IsMandatory:  false,
					},
				},
			},
			expected: builtins.Signature{
				Name:       "optional_func",
				ReturnType: "int",
				Params: []builtins.Param{
					{
						Name:     "required",
						Type:     "str",
						Required: true,
					},
					{
						Name:     "optional",
						Type:     "int",
						Default:  "42",
						Required: false,
					},
				},
			},
		},
		{
			name:     "function with variadic args",
			funcName: "variadic_func",
			callable: &builtinspb.Callable{
				ReturnType: "None",
				Params: []*builtinspb.Param{
					{
						Name:      "args",
						Type:      "any",
						IsStarArg: true,
					},
					{
						Name:          "kwargs",
						Type:          "any",
						IsStarStarArg: true,
					},
				},
			},
			expected: builtins.Signature{
				Name:       "variadic_func",
				ReturnType: "None",
				Params: []builtins.Param{
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.convertCallableToSignature(tt.funcName, tt.callable)

			if diff := cmp.Diff(tt.expected, result); diff != "" {
				t.Errorf("convertCallableToSignature mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestConvertProtoToBuiltins verifies proto to Builtins struct conversion.
func TestConvertProtoToBuiltins(t *testing.T) {
	provider := newTestProtoProvider()

	tests := []struct {
		name     string
		proto    *builtinspb.Builtins
		expected builtins.Builtins
	}{
		{
			name: "empty builtins",
			proto: &builtinspb.Builtins{},
			expected: builtins.Builtins{
				Functions: []builtins.Signature{},
				Types:     []builtins.TypeDef{},
				Globals:   []builtins.Field{},
			},
		},
		{
			name: "type with fields only",
			proto: &builtinspb.Builtins{
				Types: []*builtinspb.Type{
					{
						Name: "MyType",
						Doc:  "Type documentation",
						Fields: []*builtinspb.Field{
							{
								Name: "field1",
								Type: "str",
								Doc:  "Field 1 doc",
							},
							{
								Name: "field2",
								Type: "int",
								Doc:  "Field 2 doc",
							},
						},
					},
				},
			},
			expected: builtins.Builtins{
				Types: []builtins.TypeDef{
					{
						Name: "MyType",
						Doc:  "Type documentation",
						Fields: []builtins.Field{
							{
								Name: "field1",
								Type: "str",
								Doc:  "Field 1 doc",
							},
							{
								Name: "field2",
								Type: "int",
								Doc:  "Field 2 doc",
							},
						},
						Methods: []builtins.Signature{},
					},
				},
				Functions: []builtins.Signature{},
				Globals:   []builtins.Field{},
			},
		},
		{
			name: "type with methods",
			proto: &builtinspb.Builtins{
				Types: []*builtinspb.Type{
					{
						Name: "MyType",
						Fields: []*builtinspb.Field{
							{
								Name: "my_field",
								Type: "str",
							},
							{
								Name: "my_method",
								Type: "function",
								Callable: &builtinspb.Callable{
									Params: []*builtinspb.Param{
										{
											Name:        "arg",
											Type:        "str",
											IsMandatory: true,
										},
									},
									ReturnType: "str",
								},
							},
						},
					},
				},
			},
			expected: builtins.Builtins{
				Types: []builtins.TypeDef{
					{
						Name: "MyType",
						Fields: []builtins.Field{
							{
								Name: "my_field",
								Type: "str",
							},
						},
						Methods: []builtins.Signature{
							{
								Name:       "my_method",
								ReturnType: "str",
								Params: []builtins.Param{
									{
										Name:     "arg",
										Type:     "str",
										Required: true,
									},
								},
							},
						},
					},
				},
				Functions: []builtins.Signature{},
				Globals:   []builtins.Field{},
			},
		},
		{
			name: "global functions",
			proto: &builtinspb.Builtins{
				Values: []*builtinspb.Value{
					{
						Name: "my_function",
						Type: "function",
						Callable: &builtinspb.Callable{
							Doc: "Function doc",
							Params: []*builtinspb.Param{
								{
									Name:        "param1",
									Type:        "str",
									IsMandatory: true,
								},
							},
							ReturnType: "None",
						},
					},
				},
			},
			expected: builtins.Builtins{
				Functions: []builtins.Signature{
					{
						Name:       "my_function",
						Doc:        "Function doc",
						ReturnType: "None",
						Params: []builtins.Param{
							{
								Name:     "param1",
								Type:     "str",
								Required: true,
							},
						},
					},
				},
				Types:   []builtins.TypeDef{},
				Globals: []builtins.Field{},
			},
		},
		{
			name: "global constants",
			proto: &builtinspb.Builtins{
				Values: []*builtinspb.Value{
					{
						Name: "MY_CONSTANT",
						Type: "str",
						Doc:  "Constant doc",
					},
					{
						Name: "MY_NUMBER",
						Type: "int",
					},
				},
			},
			expected: builtins.Builtins{
				Globals: []builtins.Field{
					{
						Name: "MY_CONSTANT",
						Type: "str",
						Doc:  "Constant doc",
					},
					{
						Name: "MY_NUMBER",
						Type: "int",
					},
				},
				Functions: []builtins.Signature{},
				Types:     []builtins.TypeDef{},
			},
		},
		{
			name: "complete builtins",
			proto: &builtinspb.Builtins{
				Types: []*builtinspb.Type{
					{
						Name: "TestType",
						Doc:  "Type doc",
						Fields: []*builtinspb.Field{
							{
								Name: "field",
								Type: "str",
							},
							{
								Name: "method",
								Type: "function",
								Callable: &builtinspb.Callable{
									ReturnType: "None",
								},
							},
						},
					},
				},
				Values: []*builtinspb.Value{
					{
						Name: "func",
						Type: "function",
						Callable: &builtinspb.Callable{
							ReturnType: "str",
						},
					},
					{
						Name: "CONST",
						Type: "str",
					},
				},
			},
			expected: builtins.Builtins{
				Types: []builtins.TypeDef{
					{
						Name: "TestType",
						Doc:  "Type doc",
						Fields: []builtins.Field{
							{
								Name: "field",
								Type: "str",
							},
						},
						Methods: []builtins.Signature{
							{
								Name:       "method",
								ReturnType: "None",
								Params:     []builtins.Param{},
							},
						},
					},
				},
				Functions: []builtins.Signature{
					{
						Name:       "func",
						ReturnType: "str",
						Params:     []builtins.Param{},
					},
				},
				Globals: []builtins.Field{
					{
						Name: "CONST",
						Type: "str",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.convertProtoToBuiltins(tt.proto)

			if diff := cmp.Diff(tt.expected, result); diff != "" {
				t.Errorf("convertProtoToBuiltins mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestBuiltins_LoadProtoData verifies loading proto data from embedded filesystem.
func TestLoadProtoData(t *testing.T) {
	provider := newTestProtoProvider()

	t.Run("load existing file", func(t *testing.T) {
		testData := []byte("test data")
		provider.injectTestData("test.pb", testData)

		result, err := provider.loadProtoData("test.pb")
		if err != nil {
			t.Fatalf("loadProtoData failed: %v", err)
		}

		if string(result) != string(testData) {
			t.Errorf("Expected %q, got %q", testData, result)
		}
	})

	t.Run("fallback to pbtxt", func(t *testing.T) {
		testData := []byte("text proto data")
		provider.injectTestData("test.pbtxt", testData)

		result, err := provider.loadProtoData("test.pb")
		if err != nil {
			t.Fatalf("loadProtoData failed: %v", err)
		}

		if string(result) != string(testData) {
			t.Errorf("Expected %q, got %q", testData, result)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := provider.loadProtoData("nonexistent.pb")
		if err == nil {
			t.Error("Expected error for nonexistent file, got nil")
		}
	})
}

// TestBuiltins_Interface verifies the main Builtins interface method.
func TestBuiltins_Interface(t *testing.T) {
	provider := newTestProtoProvider()

	// Create test proto data
	testProto := &builtinspb.Builtins{
		Values: []*builtinspb.Value{
			{
				Name: "test_func",
				Type: "function",
				Callable: &builtinspb.Callable{
					ReturnType: "str",
				},
			},
		},
	}
	data, err := proto.Marshal(testProto)
	if err != nil {
		t.Fatalf("Failed to marshal test proto: %v", err)
	}

	// Add test data to embedded filesystem
	provider.injectTestData("data/proto/bazel_build.pb", data)

	t.Run("load valid proto", func(t *testing.T) {
		result, err := provider.Builtins("bazel", filekind.KindBUILD)
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

	t.Run("missing proto file", func(t *testing.T) {
		// Create a new provider without test data
		emptyProvider := NewProtoProvider()
		_, err := emptyProvider.Builtins("bazel", filekind.KindBUILD)
		if err == nil {
			t.Error("Expected error for missing proto file, got nil")
		}
	})
}

// TestBuiltins_Caching verifies that the cache works correctly.
func TestBuiltins_Caching(t *testing.T) {
	provider := newTestProtoProvider()

	// Create test proto data
	testProto := &builtinspb.Builtins{
		Values: []*builtinspb.Value{
			{
				Name: "test_func",
				Type: "function",
				Callable: &builtinspb.Callable{
					ReturnType: "str",
				},
			},
		},
	}
	data, err := proto.Marshal(testProto)
	if err != nil {
		t.Fatalf("Failed to marshal test proto: %v", err)
	}

	provider.injectTestData("data/proto/bazel_build.pb", data)

	// First load
	result1, err := provider.Builtins("bazel", filekind.KindBUILD)
	if err != nil {
		t.Fatalf("First Builtins call failed: %v", err)
	}

	// Verify cache was populated
	if _, ok := provider.cache["bazel"]; !ok {
		t.Error("Cache not populated for bazel dialect")
	}
	if _, ok := provider.cache["bazel"][filekind.KindBUILD]; !ok {
		t.Error("Cache not populated for BUILD kind")
	}

	// Modify the dataFS to ensure cache is used
	provider.injectTestData("data/proto/bazel_build.pb", []byte("invalid data"))

	// Second load should use cache
	result2, err := provider.Builtins("bazel", filekind.KindBUILD)
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

// TestBuiltins_ConcurrentAccess verifies thread-safe cache access.
func TestBuiltins_ConcurrentAccess(t *testing.T) {
	provider := newTestProtoProvider()

	// Create test proto data
	testProto := &builtinspb.Builtins{
		Values: []*builtinspb.Value{
			{
				Name: "test_func",
				Type: "function",
				Callable: &builtinspb.Callable{
					ReturnType: "str",
				},
			},
		},
	}
	data, err := proto.Marshal(testProto)
	if err != nil {
		t.Fatalf("Failed to marshal test proto: %v", err)
	}

	provider.injectTestData("data/proto/bazel_build.pb", data)

	// Run multiple goroutines accessing the same data
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			_, err := provider.Builtins("bazel", filekind.KindBUILD)
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

// TestBuiltins_AllDialectsAndKinds verifies all supported dialect/kind combinations.
func TestBuiltins_AllDialectsAndKinds(t *testing.T) {
	provider := newTestProtoProvider()

	// Create minimal test proto data
	testProto := &builtinspb.Builtins{}
	data, err := proto.Marshal(testProto)
	if err != nil {
		t.Fatalf("Failed to marshal test proto: %v", err)
	}

	combinations := []struct {
		dialect  string
		kind     filekind.Kind
		filename string
	}{
		{"bazel", filekind.KindBUILD, "data/proto/bazel_build.pb"},
		{"bazel", filekind.KindBzl, "data/proto/bazel_bzl.pb"},
		{"bazel", filekind.KindWORKSPACE, "data/proto/bazel_workspace.pb"},
		{"bazel", filekind.KindMODULE, "data/proto/bazel_module.pb"},
		{"bazel", filekind.KindBzlmod, "data/proto/bazel_bzlmod.pb"},
		{"buck2", filekind.KindBUCK, "data/proto/buck2_buck.pb"},
		{"buck2", filekind.KindBzlBuck, "data/proto/buck2_bzl.pb"},
		{"buck2", filekind.KindBuckconfig, "data/proto/buck2_buckconfig.pb"},
		{"starlark", filekind.KindStarlark, "data/proto/starlark_generic.pb"},
		{"starlark", filekind.KindSkyI, "data/proto/starlark_skyi.pb"},
	}

	for _, combo := range combinations {
		t.Run(combo.dialect+"/"+combo.kind.String(), func(t *testing.T) {
			// Add test data for this combination
			provider.injectTestData(combo.filename, data)

			// Should not return error
			result, err := provider.Builtins(combo.dialect, combo.kind)
			if err != nil {
				t.Errorf("Builtins(%q, %q) failed: %v", combo.dialect, combo.kind, err)
			}

			// Result should be valid (even if empty)
			if result.Functions == nil || result.Types == nil || result.Globals == nil {
				t.Error("Result has nil slices")
			}
		})
	}
}

// BenchmarkProtoLoader_FirstLoad measures cold load performance.
func BenchmarkProtoLoader_FirstLoad(b *testing.B) {
	// Create test proto data
	testProto := &builtinspb.Builtins{
		Types: []*builtinspb.Type{
			{
				Name: "TestType",
				Doc:  "Test documentation",
				Fields: []*builtinspb.Field{
					{Name: "field1", Type: "str"},
					{Name: "field2", Type: "int"},
					{Name: "field3", Type: "bool"},
				},
			},
		},
		Values: []*builtinspb.Value{
			{
				Name: "test_func",
				Type: "function",
				Callable: &builtinspb.Callable{
					Params: []*builtinspb.Param{
						{Name: "arg1", Type: "str", IsMandatory: true},
						{Name: "arg2", Type: "int", DefaultValue: "0"},
					},
					ReturnType: "str",
				},
			},
		},
	}
	data, err := proto.Marshal(testProto)
	if err != nil {
		b.Fatalf("Failed to marshal test proto: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		provider := newTestProtoProvider()
		provider.injectTestData("data/proto/bazel_build.pb", data)
		b.StartTimer()

		_, err := provider.Builtins("bazel", filekind.KindBUILD)
		if err != nil {
			b.Fatalf("Builtins failed: %v", err)
		}
	}
}

// BenchmarkProtoLoader_CachedLoad measures cached load performance.
func BenchmarkProtoLoader_CachedLoad(b *testing.B) {
	// Create test proto data
	testProto := &builtinspb.Builtins{
		Types: []*builtinspb.Type{
			{
				Name: "TestType",
				Doc:  "Test documentation",
				Fields: []*builtinspb.Field{
					{Name: "field1", Type: "str"},
					{Name: "field2", Type: "int"},
					{Name: "field3", Type: "bool"},
				},
			},
		},
		Values: []*builtinspb.Value{
			{
				Name: "test_func",
				Type: "function",
				Callable: &builtinspb.Callable{
					Params: []*builtinspb.Param{
						{Name: "arg1", Type: "str", IsMandatory: true},
						{Name: "arg2", Type: "int", DefaultValue: "0"},
					},
					ReturnType: "str",
				},
			},
		},
	}
	data, err := proto.Marshal(testProto)
	if err != nil {
		b.Fatalf("Failed to marshal test proto: %v", err)
	}

	provider := newTestProtoProvider()
	provider.injectTestData("data/proto/bazel_build.pb", data)

	// Warm up the cache
	_, err = provider.Builtins("bazel", filekind.KindBUILD)
	if err != nil {
		b.Fatalf("Failed to warm up cache: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := provider.Builtins("bazel", filekind.KindBUILD)
		if err != nil {
			b.Fatalf("Builtins failed: %v", err)
		}
	}
}

// BenchmarkConvertProtoToBuiltins measures conversion performance.
func BenchmarkConvertProtoToBuiltins(b *testing.B) {
	provider := newTestProtoProvider()

	// Create a moderately complex proto
	testProto := &builtinspb.Builtins{
		Types: []*builtinspb.Type{
			{
				Name: "Type1",
				Fields: []*builtinspb.Field{
					{Name: "field1", Type: "str"},
					{Name: "field2", Type: "int"},
					{
						Name: "method1",
						Type: "function",
						Callable: &builtinspb.Callable{
							Params:     []*builtinspb.Param{{Name: "arg", Type: "str"}},
							ReturnType: "str",
						},
					},
				},
			},
			{
				Name: "Type2",
				Fields: []*builtinspb.Field{
					{Name: "field1", Type: "str"},
					{Name: "field2", Type: "int"},
				},
			},
		},
		Values: []*builtinspb.Value{
			{
				Name: "func1",
				Type: "function",
				Callable: &builtinspb.Callable{
					Params: []*builtinspb.Param{
						{Name: "arg1", Type: "str", IsMandatory: true},
						{Name: "arg2", Type: "int", DefaultValue: "0"},
					},
					ReturnType: "str",
				},
			},
			{Name: "CONST1", Type: "str"},
			{Name: "CONST2", Type: "int"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.convertProtoToBuiltins(testProto)
	}
}

// TestIntegration_ComprehensiveProto verifies end-to-end functionality with a realistic proto.
func TestIntegration_ComprehensiveProto(t *testing.T) {
	provider := newTestProtoProvider()

	// Load the comprehensive test fixture
	testProto := &builtinspb.Builtins{
		Types: []*builtinspb.Type{
			{
				Name: "File",
				Doc:  "A Starlark file object",
				Fields: []*builtinspb.Field{
					{Name: "path", Type: "str", Doc: "The file path"},
					{Name: "basename", Type: "str", Doc: "The base filename"},
				},
			},
			{
				Name: "Provider",
				Doc:  "A provider that supplies information",
				Fields: []*builtinspb.Field{
					{Name: "name", Type: "str", Doc: "Provider name"},
					{
						Name: "get_value",
						Type: "function",
						Doc:  "Get a value from the provider",
						Callable: &builtinspb.Callable{
							Params: []*builtinspb.Param{
								{Name: "key", Type: "str", IsMandatory: true},
								{Name: "default", Type: "any", DefaultValue: "None", IsMandatory: false},
							},
							ReturnType: "any",
							Doc:        "Returns the value for the key",
						},
					},
				},
			},
		},
		Values: []*builtinspb.Value{
			{
				Name: "glob",
				Type: "function",
				Callable: &builtinspb.Callable{
					Params: []*builtinspb.Param{
						{Name: "include", Type: "list[str]", IsMandatory: true},
						{Name: "exclude", Type: "list[str]", DefaultValue: "[]", IsMandatory: false},
					},
					ReturnType: "list[str]",
				},
			},
			{
				Name: "print",
				Type: "function",
				Callable: &builtinspb.Callable{
					Params: []*builtinspb.Param{
						{Name: "args", Type: "any", IsStarArg: true},
						{Name: "sep", Type: "str", DefaultValue: "\" \"", IsMandatory: false},
					},
					ReturnType: "None",
				},
			},
			{Name: "True", Type: "bool", Doc: "Boolean true constant"},
			{Name: "False", Type: "bool", Doc: "Boolean false constant"},
			{Name: "WORKSPACE_ROOT", Type: "str", Doc: "The path to the workspace root"},
		},
	}

	data, err := proto.Marshal(testProto)
	if err != nil {
		t.Fatalf("Failed to marshal test proto: %v", err)
	}

	provider.injectTestData("data/proto/starlark_generic.pb", data)

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

// Helper functions for integration test

func findType(types []builtins.TypeDef, name string) *builtins.TypeDef {
	for i := range types {
		if types[i].Name == name {
			return &types[i]
		}
	}
	return nil
}

func findFunction(functions []builtins.Signature, name string) *builtins.Signature {
	for i := range functions {
		if functions[i].Name == name {
			return &functions[i]
		}
	}
	return nil
}

func findGlobal(globals []builtins.Field, name string) *builtins.Field {
	for i := range globals {
		if globals[i].Name == name {
			return &globals[i]
		}
	}
	return nil
}
