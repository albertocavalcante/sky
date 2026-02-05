package types

import (
	"testing"
)

func TestNamedType_String(t *testing.T) {
	tests := []struct {
		name string
		typ  TypeRef
		want string
	}{
		{"int", Int(), "int"},
		{"str", Str(), "str"},
		{"bool", Bool(), "bool"},
		{"float", Float(), "float"},
		{"list[str]", List(Str()), "list[str]"},
		{"dict[str, int]", Dict(Str(), Int()), "dict[str, int]"},
		{"set[int]", Set(Int()), "set[int]"},
		{"tuple[int, str]", Tuple(Int(), Str()), "tuple[int, str]"},
		{"nested list", List(List(Int())), "list[list[int]]"},
		{"nested dict", Dict(Str(), List(Int())), "dict[str, list[int]]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.typ.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUnionType_String(t *testing.T) {
	tests := []struct {
		name string
		typ  TypeRef
		want string
	}{
		{"int | None", Union(Int(), None()), "int | None"},
		{"str | int", Union(Str(), Int()), "str | int"},
		{"int | str | None", Union(Int(), Str(), None()), "int | str | None"},
		{"list[str] | None", Optional(List(Str())), "list[str] | None"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.typ.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFunctionType_String(t *testing.T) {
	tests := []struct {
		name string
		typ  TypeRef
		want string
	}{
		{
			"no params",
			&FunctionType{Return: None()},
			"() -> None",
		},
		{
			"single param",
			SimpleFunc([]TypeRef{Int()}, Str()),
			"(int) -> str",
		},
		{
			"multiple params",
			SimpleFunc([]TypeRef{Int(), Str()}, Bool()),
			"(int, str) -> bool",
		},
		{
			"named params",
			Func([]ParamType{
				{Name: "x", Type: Int()},
				{Name: "y", Type: Str()},
			}, Bool()),
			"(x: int, y: str) -> bool",
		},
		{
			"nil return",
			SimpleFunc([]TypeRef{Int()}, nil),
			"(int) -> None",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.typ.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSpecialTypes_String(t *testing.T) {
	tests := []struct {
		name string
		typ  TypeRef
		want string
	}{
		{"None", None(), "None"},
		{"Any", Any(), "Any"},
		{"Unknown", Unknown(), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.typ.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTypeRef_Equal(t *testing.T) {
	tests := []struct {
		name  string
		a     TypeRef
		b     TypeRef
		equal bool
	}{
		{"int == int", Int(), Int(), true},
		{"int != str", Int(), Str(), false},
		{"list[int] == list[int]", List(Int()), List(Int()), true},
		{"list[int] != list[str]", List(Int()), List(Str()), false},
		{"dict[str,int] == dict[str,int]", Dict(Str(), Int()), Dict(Str(), Int()), true},
		{"union order matters", Union(Int(), Str()), Union(Str(), Int()), false},
		{"None == None", None(), None(), true},
		{"Any == Any", Any(), Any(), true},
		{"Unknown == Unknown", Unknown(), Unknown(), true},
		{"int != None", Int(), None(), false},
		{"func == func", SimpleFunc([]TypeRef{Int()}, Str()), SimpleFunc([]TypeRef{Int()}, Str()), true},
		{"func != func (params)", SimpleFunc([]TypeRef{Int()}, Str()), SimpleFunc([]TypeRef{Bool()}, Str()), false},
		{"func != func (return)", SimpleFunc([]TypeRef{Int()}, Str()), SimpleFunc([]TypeRef{Int()}, Bool()), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equal(tt.b); got != tt.equal {
				t.Errorf("Equal() = %v, want %v", got, tt.equal)
			}
		})
	}
}

func TestTypeRef_IsUnknown(t *testing.T) {
	tests := []struct {
		name    string
		typ     TypeRef
		unknown bool
	}{
		{"int", Int(), false},
		{"None", None(), false},
		{"Any", Any(), false},
		{"Unknown", Unknown(), true},
		{"Unknown with reason", UnknownWithReason("test"), true},
		{"list[int]", List(Int()), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.typ.IsUnknown(); got != tt.unknown {
				t.Errorf("IsUnknown() = %v, want %v", got, tt.unknown)
			}
		})
	}
}

func TestUnion_Flattening(t *testing.T) {
	// Nested unions should be flattened
	nested := Union(Union(Int(), Str()), Bool())
	if got := nested.String(); got != "int | str | bool" {
		t.Errorf("Flattened union = %q, want %q", got, "int | str | bool")
	}
}

func TestUnion_Deduplication(t *testing.T) {
	// Duplicate types should be removed
	duped := Union(Int(), Int(), Str())
	if got := duped.String(); got != "int | str" {
		t.Errorf("Deduplicated union = %q, want %q", got, "int | str")
	}
}

func TestUnion_SingleType(t *testing.T) {
	// Single type union should unwrap
	single := Union(Int())
	if _, ok := single.(*UnionType); ok {
		t.Error("Single-type union should not be UnionType")
	}
	if got := single.String(); got != "int" {
		t.Errorf("Single-type union = %q, want %q", got, "int")
	}
}

func TestOptional(t *testing.T) {
	opt := Optional(Int())
	if got := opt.String(); got != "int | None" {
		t.Errorf("Optional(int) = %q, want %q", got, "int | None")
	}
}

func TestIsPredicates(t *testing.T) {
	tests := []struct {
		name      string
		typ       TypeRef
		isNone    bool
		isAny     bool
		isFunc    bool
		isUnion   bool
		isCollect bool
	}{
		{"int", Int(), false, false, false, false, false},
		{"None", None(), true, false, false, false, false},
		{"Any", Any(), false, true, false, false, false},
		{"func", SimpleFunc(nil, Int()), false, false, true, false, false},
		{"union", Union(Int(), Str()), false, false, false, true, false},
		{"list", List(Int()), false, false, false, false, true},
		{"dict", Dict(Str(), Int()), false, false, false, false, true},
		{"set", Set(Int()), false, false, false, false, true},
		{"tuple", Tuple(Int(), Str()), false, false, false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNone(tt.typ); got != tt.isNone {
				t.Errorf("IsNone() = %v, want %v", got, tt.isNone)
			}
			if got := IsAny(tt.typ); got != tt.isAny {
				t.Errorf("IsAny() = %v, want %v", got, tt.isAny)
			}
			if got := IsFunction(tt.typ); got != tt.isFunc {
				t.Errorf("IsFunction() = %v, want %v", got, tt.isFunc)
			}
			if got := IsUnion(tt.typ); got != tt.isUnion {
				t.Errorf("IsUnion() = %v, want %v", got, tt.isUnion)
			}
			if got := IsCollection(tt.typ); got != tt.isCollect {
				t.Errorf("IsCollection() = %v, want %v", got, tt.isCollect)
			}
		})
	}
}

func TestElementType(t *testing.T) {
	tests := []struct {
		name string
		typ  TypeRef
		elem string // "" means nil
	}{
		{"list[int]", List(Int()), "int"},
		{"set[str]", Set(Str()), "str"},
		{"dict[str, int]", Dict(Str(), Int()), "int"}, // value type
		{"tuple", Tuple(Int(), Str()), "Unknown"},     // heterogeneous
		{"int", Int(), ""},                            // not a collection
		{"list (no args)", &NamedType{Name: "list"}, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elem := ElementType(tt.typ)
			if tt.elem == "" {
				if elem != nil {
					t.Errorf("ElementType() = %v, want nil", elem)
				}
			} else {
				if elem == nil {
					t.Errorf("ElementType() = nil, want %q", tt.elem)
				} else if got := elem.String(); got != tt.elem {
					t.Errorf("ElementType() = %q, want %q", got, tt.elem)
				}
			}
		})
	}
}

func TestContainsUnknown(t *testing.T) {
	tests := []struct {
		name     string
		typ      TypeRef
		contains bool
	}{
		{"int", Int(), false},
		{"Unknown", Unknown(), true},
		{"list[int]", List(Int()), false},
		{"list[Unknown]", List(Unknown()), true},
		{"dict[str, Unknown]", Dict(Str(), Unknown()), true},
		{"union with Unknown", Union(Int(), Unknown()), true},
		{"func with Unknown param", SimpleFunc([]TypeRef{Unknown()}, Int()), true},
		{"func with Unknown return", SimpleFunc([]TypeRef{Int()}, Unknown()), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsUnknown(tt.typ); got != tt.contains {
				t.Errorf("ContainsUnknown() = %v, want %v", got, tt.contains)
			}
		})
	}
}
