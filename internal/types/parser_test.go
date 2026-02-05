package types

import (
	"testing"
)

func TestParseTypeComment_SimpleTypes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"# type: int", "int"},
		{"# type: str", "str"},
		{"# type: bool", "bool"},
		{"# type: float", "float"},
		{"# type: bytes", "bytes"},
		{"# type: None", "None"},
		{"# type: Any", "Any"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTypeComment(tt.input)
			if err != nil {
				t.Fatalf("ParseTypeComment(%q) error: %v", tt.input, err)
			}
			if got.String() != tt.want {
				t.Errorf("ParseTypeComment(%q) = %q, want %q", tt.input, got.String(), tt.want)
			}
		})
	}
}

func TestParseTypeComment_GenericTypes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"# type: list[str]", "list[str]"},
		{"# type: list[int]", "list[int]"},
		{"# type: dict[str, int]", "dict[str, int]"},
		{"# type: dict[str,int]", "dict[str, int]"}, // no space
		{"# type: set[str]", "set[str]"},
		{"# type: tuple[int, str]", "tuple[int, str]"},
		{"# type: tuple[int, str, bool]", "tuple[int, str, bool]"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTypeComment(tt.input)
			if err != nil {
				t.Fatalf("ParseTypeComment(%q) error: %v", tt.input, err)
			}
			if got.String() != tt.want {
				t.Errorf("ParseTypeComment(%q) = %q, want %q", tt.input, got.String(), tt.want)
			}
		})
	}
}

func TestParseTypeComment_NestedGenerics(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"# type: list[list[int]]", "list[list[int]]"},
		{"# type: dict[str, list[int]]", "dict[str, list[int]]"},
		{"# type: list[dict[str, int]]", "list[dict[str, int]]"},
		{"# type: dict[str, dict[str, int]]", "dict[str, dict[str, int]]"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTypeComment(tt.input)
			if err != nil {
				t.Fatalf("ParseTypeComment(%q) error: %v", tt.input, err)
			}
			if got.String() != tt.want {
				t.Errorf("ParseTypeComment(%q) = %q, want %q", tt.input, got.String(), tt.want)
			}
		})
	}
}

func TestParseTypeComment_UnionTypes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"# type: int | None", "int | None"},
		{"# type: str | int", "str | int"},
		{"# type: int | str | None", "int | str | None"},
		{"# type: list[str] | None", "list[str] | None"},
		{"# type: int|None", "int | None"}, // no spaces
		{"# type: int | str | bool", "int | str | bool"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTypeComment(tt.input)
			if err != nil {
				t.Fatalf("ParseTypeComment(%q) error: %v", tt.input, err)
			}
			if got.String() != tt.want {
				t.Errorf("ParseTypeComment(%q) = %q, want %q", tt.input, got.String(), tt.want)
			}
		})
	}
}

func TestParseTypeComment_Optional(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"# type: Optional[int]", "int | None"},
		{"# type: Optional[str]", "str | None"},
		{"# type: Optional[list[str]]", "list[str] | None"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTypeComment(tt.input)
			if err != nil {
				t.Fatalf("ParseTypeComment(%q) error: %v", tt.input, err)
			}
			if got.String() != tt.want {
				t.Errorf("ParseTypeComment(%q) = %q, want %q", tt.input, got.String(), tt.want)
			}
		})
	}
}

func TestParseTypeComment_FunctionTypes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"# type: () -> None", "() -> None"},
		{"# type: () -> int", "() -> int"},
		{"# type: (int) -> str", "(int) -> str"},
		{"# type: (int, str) -> bool", "(int, str) -> bool"},
		{"# type: (int,str)->bool", "(int, str) -> bool"}, // compact
		{"# type: (list[str]) -> int", "(list[str]) -> int"},
		{"# type: (int, str, bool) -> None", "(int, str, bool) -> None"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTypeComment(tt.input)
			if err != nil {
				t.Fatalf("ParseTypeComment(%q) error: %v", tt.input, err)
			}
			if got.String() != tt.want {
				t.Errorf("ParseTypeComment(%q) = %q, want %q", tt.input, got.String(), tt.want)
			}
		})
	}
}

func TestParseTypeComment_Callable(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"# type: Callable[[], None]", "() -> None"},
		{"# type: Callable[[int], str]", "(int) -> str"},
		{"# type: Callable[[int, str], bool]", "(int, str) -> bool"},
		{"# type: Callable[..., int]", "() -> int"}, // variadic
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTypeComment(tt.input)
			if err != nil {
				t.Fatalf("ParseTypeComment(%q) error: %v", tt.input, err)
			}
			if got.String() != tt.want {
				t.Errorf("ParseTypeComment(%q) = %q, want %q", tt.input, got.String(), tt.want)
			}
		})
	}
}

func TestParseTypeComment_WhitespaceVariations(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"#type:int", "int"},
		{"# type:int", "int"},
		{"# type: int", "int"},
		{"# type:  int", "int"},
		{"# type :int", "int"}, // space before colon
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTypeComment(tt.input)
			if err != nil {
				t.Fatalf("ParseTypeComment(%q) error: %v", tt.input, err)
			}
			if got.String() != tt.want {
				t.Errorf("ParseTypeComment(%q) = %q, want %q", tt.input, got.String(), tt.want)
			}
		})
	}
}

func TestParseTypeComment_QualifiedNames(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"# type: module.Type", "module.Type"},
		{"# type: pkg.module.Type", "pkg.module.Type"},
		{"# type: list[module.Type]", "list[module.Type]"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTypeComment(tt.input)
			if err != nil {
				t.Fatalf("ParseTypeComment(%q) error: %v", tt.input, err)
			}
			if got.String() != tt.want {
				t.Errorf("ParseTypeComment(%q) = %q, want %q", tt.input, got.String(), tt.want)
			}
		})
	}
}

func TestParseTypeComment_BazelTypes(t *testing.T) {
	// Common Bazel/Starlark types
	tests := []struct {
		input string
		want  string
	}{
		{"# type: Label", "Label"},
		{"# type: Target", "Target"},
		{"# type: File", "File"},
		{"# type: ctx", "ctx"},
		{"# type: list[Label]", "list[Label]"},
		{"# type: list[File]", "list[File]"},
		{"# type: depset[File]", "depset[File]"},
		{"# type: struct", "struct"},
		{"# type: Provider", "Provider"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTypeComment(tt.input)
			if err != nil {
				t.Fatalf("ParseTypeComment(%q) error: %v", tt.input, err)
			}
			if got.String() != tt.want {
				t.Errorf("ParseTypeComment(%q) = %q, want %q", tt.input, got.String(), tt.want)
			}
		})
	}
}

func TestParseTypeComment_Errors(t *testing.T) {
	tests := []struct {
		input   string
		wantErr string
	}{
		{"not a type comment", "not a type comment"},
		{"# type:", "empty type"},
		{"# type: ", "empty type"},
		{"# type: list[", "unexpected end"},
		{"# type: list[int", "expected ']'"},
		{"# type: (int) ->", "unexpected end"},
		{"# type: (int ->", "expected ')'"},
		{"# type: |", "expected type name"},
		{"# type: int |", "unexpected end"},
		{"# type: int extra", "unexpected characters"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := ParseTypeComment(tt.input)
			if err == nil {
				t.Errorf("ParseTypeComment(%q) expected error containing %q", tt.input, tt.wantErr)
				return
			}
			if !contains(err.Error(), tt.wantErr) {
				t.Errorf("ParseTypeComment(%q) error = %q, want error containing %q", tt.input, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParseTypeString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"int", "int"},
		{"list[str]", "list[str]"},
		{"int | None", "int | None"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTypeString(tt.input)
			if err != nil {
				t.Fatalf("ParseTypeString(%q) error: %v", tt.input, err)
			}
			if got.String() != tt.want {
				t.Errorf("ParseTypeString(%q) = %q, want %q", tt.input, got.String(), tt.want)
			}
		})
	}
}

func TestParseFunctionTypeComment(t *testing.T) {
	tests := []struct {
		input      string
		wantParams int
		wantReturn string
	}{
		{"# type: () -> None", 0, "None"},
		{"# type: (int) -> str", 1, "str"},
		{"# type: (int, str) -> bool", 2, "bool"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseFunctionTypeComment(tt.input)
			if err != nil {
				t.Fatalf("ParseFunctionTypeComment(%q) error: %v", tt.input, err)
			}
			if len(got.Params) != tt.wantParams {
				t.Errorf("got %d params, want %d", len(got.Params), tt.wantParams)
			}
			if got.Return.String() != tt.wantReturn {
				t.Errorf("return = %q, want %q", got.Return.String(), tt.wantReturn)
			}
		})
	}
}

func TestParseFunctionTypeComment_NotFunction(t *testing.T) {
	_, err := ParseFunctionTypeComment("# type: int")
	if err == nil {
		t.Error("expected error for non-function type")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
