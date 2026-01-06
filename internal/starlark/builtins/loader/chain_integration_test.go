package loader

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"

	"github.com/albertocavalcante/sky/internal/starlark/builtins"
	builtinspb "github.com/albertocavalcante/sky/internal/starlark/builtins/proto"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// TestChainProvider_ProtoAndJSON verifies that ChainProvider works with both providers.
func TestChainProvider_ProtoAndJSON(t *testing.T) {
	protoProvider := newTestProtoProvider()
	jsonProvider := newTestJSONProvider()

	// Create proto data with a function
	protoBuiltins := &builtinspb.Builtins{
		Values: []*builtinspb.Value{
			{
				Name: "proto_func",
				Type: "function",
				Callable: &builtinspb.Callable{
					ReturnType: "str",
				},
			},
		},
	}
	protoData, err := proto.Marshal(protoBuiltins)
	if err != nil {
		t.Fatalf("Failed to marshal proto: %v", err)
	}
	protoProvider.injectTestData("data/proto/starlark_generic.pb", protoData)

	// Create JSON data with a different function
	jsonBuiltins := builtins.Builtins{
		Functions: []builtins.Signature{
			{
				Name:       "json_func",
				ReturnType: "int",
			},
		},
	}
	jsonData, err := json.Marshal(jsonBuiltins)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}
	jsonProvider.injectTestData("data/json/starlark-core.json", jsonData)

	// Create chain provider
	chain := builtins.NewChainProvider(protoProvider, jsonProvider)

	// Load builtins - should merge both
	result, err := chain.Builtins("starlark", filekind.KindStarlark)
	if err != nil {
		t.Fatalf("ChainProvider.Builtins failed: %v", err)
	}

	// Should have both functions
	if len(result.Functions) != 2 {
		t.Errorf("Expected 2 functions (merged from proto and JSON), got %d", len(result.Functions))
	}

	// Verify both functions are present
	hasProtoFunc := false
	hasJSONFunc := false
	for _, fn := range result.Functions {
		if fn.Name == "proto_func" {
			hasProtoFunc = true
		}
		if fn.Name == "json_func" {
			hasJSONFunc = true
		}
	}

	if !hasProtoFunc {
		t.Error("proto_func not found in merged result")
	}
	if !hasJSONFunc {
		t.Error("json_func not found in merged result")
	}
}

// TestChainProvider_FallbackBehavior verifies fallback from proto to JSON.
func TestChainProvider_FallbackBehavior(t *testing.T) {
	protoProvider := newTestProtoProvider()
	jsonProvider := newTestJSONProvider()

	// Only provide JSON data (proto should fail)
	jsonBuiltins := builtins.Builtins{
		Functions: []builtins.Signature{
			{
				Name:       "json_only_func",
				ReturnType: "str",
			},
		},
	}
	jsonData, err := json.Marshal(jsonBuiltins)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}
	jsonProvider.injectTestData("data/json/starlark-core.json", jsonData)

	// Create chain: proto first, JSON second
	chain := builtins.NewChainProvider(protoProvider, jsonProvider)

	// Load builtins - proto should fail silently, JSON should succeed
	result, err := chain.Builtins("starlark", filekind.KindStarlark)
	if err != nil {
		t.Fatalf("ChainProvider.Builtins failed: %v", err)
	}

	// Should have the JSON function
	if len(result.Functions) != 1 {
		t.Errorf("Expected 1 function from JSON, got %d", len(result.Functions))
	}
	if result.Functions[0].Name != "json_only_func" {
		t.Errorf("Expected json_only_func, got %q", result.Functions[0].Name)
	}
}

// TestChainProvider_MergeComplexStructures verifies merging of complex structures.
func TestChainProvider_MergeComplexStructures(t *testing.T) {
	protoProvider := newTestProtoProvider()
	jsonProvider := newTestJSONProvider()

	// Proto provides types
	protoBuiltins := &builtinspb.Builtins{
		Types: []*builtinspb.Type{
			{
				Name: "ProtoType",
				Fields: []*builtinspb.Field{
					{Name: "field1", Type: "str"},
				},
			},
		},
	}
	protoData, err := proto.Marshal(protoBuiltins)
	if err != nil {
		t.Fatalf("Failed to marshal proto: %v", err)
	}
	protoProvider.injectTestData("data/proto/bazel_build.pb", protoData)

	// JSON provides functions and globals
	jsonBuiltins := builtins.Builtins{
		Functions: []builtins.Signature{
			{Name: "json_func", ReturnType: "None"},
		},
		Globals: []builtins.Field{
			{Name: "JSON_CONST", Type: "str"},
		},
	}
	jsonData, err := json.Marshal(jsonBuiltins)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}
	jsonProvider.injectTestData("data/json/bazel-build.json", jsonData)

	// Create chain provider
	chain := builtins.NewChainProvider(protoProvider, jsonProvider)

	// Load builtins
	result, err := chain.Builtins("bazel", filekind.KindBUILD)
	if err != nil {
		t.Fatalf("ChainProvider.Builtins failed: %v", err)
	}

	// Verify all sections are merged
	if len(result.Types) != 1 {
		t.Errorf("Expected 1 type from proto, got %d", len(result.Types))
	}
	if len(result.Functions) != 1 {
		t.Errorf("Expected 1 function from JSON, got %d", len(result.Functions))
	}
	if len(result.Globals) != 1 {
		t.Errorf("Expected 1 global from JSON, got %d", len(result.Globals))
	}

	// Verify content
	if result.Types[0].Name != "ProtoType" {
		t.Errorf("Expected ProtoType, got %q", result.Types[0].Name)
	}
	if result.Functions[0].Name != "json_func" {
		t.Errorf("Expected json_func, got %q", result.Functions[0].Name)
	}
	if result.Globals[0].Name != "JSON_CONST" {
		t.Errorf("Expected JSON_CONST, got %q", result.Globals[0].Name)
	}
}

// TestChainProvider_SupportedDialects verifies dialect aggregation.
func TestChainProvider_SupportedDialects(t *testing.T) {
	protoProvider := newTestProtoProvider()
	jsonProvider := newTestJSONProvider()

	chain := builtins.NewChainProvider(protoProvider, jsonProvider)

	dialects := chain.SupportedDialects()

	// Should include dialects from both providers (deduplicated)
	expected := []string{"bazel", "buck2", "starlark"}
	if len(dialects) != len(expected) {
		t.Errorf("Expected %d dialects, got %d", len(expected), len(dialects))
	}

	dialectMap := make(map[string]bool)
	for _, d := range dialects {
		dialectMap[d] = true
	}

	for _, exp := range expected {
		if !dialectMap[exp] {
			t.Errorf("Expected dialect %q not found", exp)
		}
	}
}

// TestChainProvider_EmptyProviders verifies behavior with empty providers.
func TestChainProvider_EmptyProviders(t *testing.T) {
	// Create chain with no providers
	chain := builtins.NewChainProvider()

	result, err := chain.Builtins("bazel", filekind.KindBUILD)
	if err != nil {
		t.Fatalf("Expected no error with empty chain, got: %v", err)
	}

	// Should return empty builtins
	if len(result.Functions) != 0 || len(result.Types) != 0 || len(result.Globals) != 0 {
		t.Error("Expected empty builtins from empty chain")
	}
}

// TestChainProvider_OrderMatters verifies provider order affects results.
func TestChainProvider_OrderMatters(t *testing.T) {
	provider1 := newTestJSONProvider()
	provider2 := newTestJSONProvider()

	// Both provide the same function with different return types
	builtins1 := builtins.Builtins{
		Functions: []builtins.Signature{
			{Name: "func", ReturnType: "str"},
		},
	}
	builtins2 := builtins.Builtins{
		Functions: []builtins.Signature{
			{Name: "func", ReturnType: "int"},
		},
	}

	data1, _ := json.Marshal(builtins1)
	data2, _ := json.Marshal(builtins2)

	provider1.injectTestData("data/json/starlark-core.json", data1)
	provider2.injectTestData("data/json/starlark-core.json", data2)

	// Create chain with provider1 first
	chain := builtins.NewChainProvider(provider1, provider2)

	result, err := chain.Builtins("starlark", filekind.KindStarlark)
	if err != nil {
		t.Fatalf("ChainProvider.Builtins failed: %v", err)
	}

	// Should have both functions (merged, not replaced)
	if len(result.Functions) != 2 {
		t.Errorf("Expected 2 functions (both versions), got %d", len(result.Functions))
	}

	// First one should be from provider1
	if result.Functions[0].ReturnType != "str" {
		t.Errorf("Expected first function to return 'str', got %q", result.Functions[0].ReturnType)
	}
}

// BenchmarkChainProvider_ProtoAndJSON measures performance of chained providers.
func BenchmarkChainProvider_ProtoAndJSON(b *testing.B) {
	protoProvider := newTestProtoProvider()
	jsonProvider := newTestJSONProvider()

	// Create test data
	protoBuiltins := &builtinspb.Builtins{
		Values: []*builtinspb.Value{
			{Name: "proto_func", Type: "function", Callable: &builtinspb.Callable{ReturnType: "str"}},
		},
	}
	protoData, _ := proto.Marshal(protoBuiltins)
	protoProvider.injectTestData("data/proto/starlark_generic.pb", protoData)

	jsonBuiltins := builtins.Builtins{
		Functions: []builtins.Signature{
			{Name: "json_func", ReturnType: "int"},
		},
	}
	jsonData, _ := json.Marshal(jsonBuiltins)
	jsonProvider.injectTestData("data/json/starlark-core.json", jsonData)

	chain := builtins.NewChainProvider(protoProvider, jsonProvider)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := chain.Builtins("starlark", filekind.KindStarlark)
		if err != nil {
			b.Fatalf("ChainProvider.Builtins failed: %v", err)
		}
	}
}

// TestIntegration_RealWorldScenario simulates a real-world usage pattern.
func TestIntegration_RealWorldScenario(t *testing.T) {
	// Scenario: Proto provides core Bazel BUILD rules, JSON provides custom extensions
	protoProvider := newTestProtoProvider()
	jsonProvider := newTestJSONProvider()

	// Proto: Standard Bazel rules
	protoBuiltins := &builtinspb.Builtins{
		Values: []*builtinspb.Value{
			{
				Name: "cc_library",
				Type: "function",
				Doc:  "C++ library rule",
				Callable: &builtinspb.Callable{
					Params: []*builtinspb.Param{
						{Name: "name", Type: "str", IsMandatory: true},
						{Name: "srcs", Type: "list[str]", DefaultValue: "[]"},
					},
					ReturnType: "None",
				},
			},
		},
		Types: []*builtinspb.Type{
			{
				Name: "Label",
				Doc:  "A build label",
				Fields: []*builtinspb.Field{
					{Name: "name", Type: "str"},
					{Name: "package", Type: "str"},
				},
			},
		},
	}
	protoData, _ := proto.Marshal(protoBuiltins)
	protoProvider.injectTestData("data/proto/bazel_build.pb", protoData)

	// JSON: Custom project-specific rules
	jsonBuiltins := builtins.Builtins{
		Functions: []builtins.Signature{
			{
				Name: "custom_deploy_rule",
				Doc:  "Custom deployment rule",
				Params: []builtins.Param{
					{Name: "name", Type: "str", Required: true},
					{Name: "target", Type: "str", Required: true},
				},
				ReturnType: "None",
			},
		},
		Globals: []builtins.Field{
			{Name: "DEPLOY_ENV", Type: "str", Doc: "Deployment environment"},
		},
	}
	jsonData, _ := json.Marshal(jsonBuiltins)
	jsonProvider.injectTestData("data/json/bazel-build.json", jsonData)

	// Create chain: proto first (standard), JSON second (custom)
	chain := builtins.NewChainProvider(protoProvider, jsonProvider)

	// Load builtins for Bazel BUILD files
	result, err := chain.Builtins("bazel", filekind.KindBUILD)
	if err != nil {
		t.Fatalf("Failed to load builtins: %v", err)
	}

	// Verify we have both standard and custom rules
	if len(result.Functions) != 2 {
		t.Errorf("Expected 2 functions (1 standard + 1 custom), got %d", len(result.Functions))
	}
	if len(result.Types) != 1 {
		t.Errorf("Expected 1 type (Label), got %d", len(result.Types))
	}
	if len(result.Globals) != 1 {
		t.Errorf("Expected 1 global (DEPLOY_ENV), got %d", len(result.Globals))
	}

	// Verify standard rule
	ccLib := findFunction(result.Functions, "cc_library")
	if ccLib == nil {
		t.Error("Standard cc_library rule not found")
	} else {
		if len(ccLib.Params) != 2 {
			t.Errorf("cc_library: expected 2 params, got %d", len(ccLib.Params))
		}
	}

	// Verify custom rule
	customRule := findFunction(result.Functions, "custom_deploy_rule")
	if customRule == nil {
		t.Error("Custom deploy rule not found")
	} else {
		if len(customRule.Params) != 2 {
			t.Errorf("custom_deploy_rule: expected 2 params, got %d", len(customRule.Params))
		}
	}

	// Verify type
	labelType := findType(result.Types, "Label")
	if labelType == nil {
		t.Error("Label type not found")
	} else {
		if len(labelType.Fields) != 2 {
			t.Errorf("Label: expected 2 fields, got %d", len(labelType.Fields))
		}
	}
}

// TestChainProvider_CachingBehavior verifies that caching works with chained providers.
func TestChainProvider_CachingBehavior(t *testing.T) {
	provider1 := newTestJSONProvider()
	provider2 := newTestJSONProvider()

	testData := builtins.Builtins{
		Functions: []builtins.Signature{
			{Name: "func1", ReturnType: "str"},
		},
	}
	data, _ := json.Marshal(testData)

	provider1.injectTestData("data/json/starlark-core.json", data)
	provider2.injectTestData("data/json/starlark-core.json", data)

	chain := builtins.NewChainProvider(provider1, provider2)

	// First call
	result1, err := chain.Builtins("starlark", filekind.KindStarlark)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}

	// Corrupt the data
	provider1.injectTestData("data/json/starlark-core.json", []byte("invalid"))
	provider2.injectTestData("data/json/starlark-core.json", []byte("invalid"))

	// Second call - should use cached data from each provider
	result2, err := chain.Builtins("starlark", filekind.KindStarlark)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}

	// Results should be identical
	if diff := cmp.Diff(result1, result2); diff != "" {
		t.Errorf("Results differ after corruption (cache not working): %s", diff)
	}
}
