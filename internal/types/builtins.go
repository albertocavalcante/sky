package types

// BuiltinSignature defines a builtin function's type signature.
type BuiltinSignature struct {
	Name       string
	Params     []ParamType
	ReturnType TypeRef
	Doc        string
}

// builtinReturns maps builtin function names to their return types.
// This provides quick lookup for type inference.
var builtinReturns = map[string]TypeRef{
	// Core conversion builtins
	"str":   Str(),
	"int":   Int(),
	"bool":  Bool(),
	"float": Float(),

	// Type constructors
	"list":  List(Unknown()),
	"dict":  Dict(Unknown(), Unknown()),
	"tuple": Tuple(),
	"set":   Set(Unknown()),

	// Sequence operations
	"len":       Int(),
	"range":     List(Int()),
	"sorted":    List(Unknown()),
	"reversed":  List(Unknown()),
	"zip":       List(Tuple()),
	"enumerate": List(Tuple(Int(), Unknown())),

	// Predicates
	"all":     Bool(),
	"any":     Bool(),
	"hasattr": Bool(),

	// Introspection
	"type":    Str(),
	"repr":    Str(),
	"hash":    Int(),
	"dir":     List(Str()),
	"getattr": Unknown(),

	// Aggregation (return type depends on args)
	"min": Unknown(),
	"max": Unknown(),
	"sum": Int(), // Typically int, but could be float

	// Output/control
	"print": None(),
	"fail":  None(),

	// String operations
	"abs": Int(), // Could be float too
	"ord": Int(),
	"chr": Str(),
	"hex": Str(),
	"oct": Str(),
	"bin": Str(),
}

// builtinSignatures provides detailed signatures for builtins.
// Used for hover and signature help.
var builtinSignatures = map[string]*BuiltinSignature{
	"len": {
		Name:       "len",
		Params:     []ParamType{{Name: "obj", Type: Unknown()}},
		ReturnType: Int(),
		Doc:        "Return the length (number of items) of an object.",
	},
	"str": {
		Name:       "str",
		Params:     []ParamType{{Name: "x", Type: Unknown()}},
		ReturnType: Str(),
		Doc:        "Return a string representation of the value.",
	},
	"int": {
		Name:       "int",
		Params:     []ParamType{{Name: "x", Type: Unknown()}},
		ReturnType: Int(),
		Doc:        "Return an integer value.",
	},
	"bool": {
		Name:       "bool",
		Params:     []ParamType{{Name: "x", Type: Unknown()}},
		ReturnType: Bool(),
		Doc:        "Return a boolean value.",
	},
	"float": {
		Name:       "float",
		Params:     []ParamType{{Name: "x", Type: Unknown()}},
		ReturnType: Float(),
		Doc:        "Return a floating-point value.",
	},
	"list": {
		Name:       "list",
		Params:     []ParamType{{Name: "iterable", Type: Unknown(), Optional: true}},
		ReturnType: List(Unknown()),
		Doc:        "Return a list containing the items of the iterable.",
	},
	"dict": {
		Name:       "dict",
		Params:     []ParamType{{Name: "pairs", Type: Unknown(), Optional: true}},
		ReturnType: Dict(Unknown(), Unknown()),
		Doc:        "Return a new dictionary.",
	},
	"tuple": {
		Name:       "tuple",
		Params:     []ParamType{{Name: "iterable", Type: Unknown(), Optional: true}},
		ReturnType: Tuple(),
		Doc:        "Return a tuple containing the items of the iterable.",
	},
	"range": {
		Name: "range",
		Params: []ParamType{
			{Name: "start_or_stop", Type: Int()},
			{Name: "stop", Type: Int(), Optional: true},
			{Name: "step", Type: Int(), Optional: true},
		},
		ReturnType: List(Int()),
		Doc:        "Return a sequence of integers.",
	},
	"sorted": {
		Name: "sorted",
		Params: []ParamType{
			{Name: "iterable", Type: Unknown()},
			{Name: "key", Type: Unknown(), Optional: true},
			{Name: "reverse", Type: Bool(), Optional: true},
		},
		ReturnType: List(Unknown()),
		Doc:        "Return a new sorted list.",
	},
	"reversed": {
		Name:       "reversed",
		Params:     []ParamType{{Name: "sequence", Type: Unknown()}},
		ReturnType: List(Unknown()),
		Doc:        "Return a reversed iterator.",
	},
	"enumerate": {
		Name: "enumerate",
		Params: []ParamType{
			{Name: "iterable", Type: Unknown()},
			{Name: "start", Type: Int(), Optional: true},
		},
		ReturnType: List(Tuple(Int(), Unknown())),
		Doc:        "Return an enumerate object.",
	},
	"zip": {
		Name: "zip",
		Params: []ParamType{
			{Name: "iterables", Type: Unknown(), IsArgs: true},
		},
		ReturnType: List(Tuple()),
		Doc:        "Return a zip object yielding tuples.",
	},
	"all": {
		Name:       "all",
		Params:     []ParamType{{Name: "iterable", Type: Unknown()}},
		ReturnType: Bool(),
		Doc:        "Return True if all elements are true.",
	},
	"any": {
		Name:       "any",
		Params:     []ParamType{{Name: "iterable", Type: Unknown()}},
		ReturnType: Bool(),
		Doc:        "Return True if any element is true.",
	},
	"hasattr": {
		Name: "hasattr",
		Params: []ParamType{
			{Name: "obj", Type: Unknown()},
			{Name: "name", Type: Str()},
		},
		ReturnType: Bool(),
		Doc:        "Return True if the object has the named attribute.",
	},
	"getattr": {
		Name: "getattr",
		Params: []ParamType{
			{Name: "obj", Type: Unknown()},
			{Name: "name", Type: Str()},
			{Name: "default", Type: Unknown(), Optional: true},
		},
		ReturnType: Unknown(),
		Doc:        "Return the value of the named attribute.",
	},
	"type": {
		Name:       "type",
		Params:     []ParamType{{Name: "obj", Type: Unknown()}},
		ReturnType: Str(),
		Doc:        "Return the type of an object as a string.",
	},
	"repr": {
		Name:       "repr",
		Params:     []ParamType{{Name: "obj", Type: Unknown()}},
		ReturnType: Str(),
		Doc:        "Return a string containing a printable representation.",
	},
	"hash": {
		Name:       "hash",
		Params:     []ParamType{{Name: "obj", Type: Unknown()}},
		ReturnType: Int(),
		Doc:        "Return the hash value of the object.",
	},
	"dir": {
		Name:       "dir",
		Params:     []ParamType{{Name: "obj", Type: Unknown()}},
		ReturnType: List(Str()),
		Doc:        "Return a list of names in the object's namespace.",
	},
	"min": {
		Name: "min",
		Params: []ParamType{
			{Name: "args", Type: Unknown(), IsArgs: true},
		},
		ReturnType: Unknown(),
		Doc:        "Return the smallest item.",
	},
	"max": {
		Name: "max",
		Params: []ParamType{
			{Name: "args", Type: Unknown(), IsArgs: true},
		},
		ReturnType: Unknown(),
		Doc:        "Return the largest item.",
	},
	"print": {
		Name: "print",
		Params: []ParamType{
			{Name: "args", Type: Unknown(), IsArgs: true},
		},
		ReturnType: None(),
		Doc:        "Print values to the output.",
	},
	"fail": {
		Name: "fail",
		Params: []ParamType{
			{Name: "msg", Type: Str(), Optional: true},
		},
		ReturnType: None(),
		Doc:        "Fail the build with an error message.",
	},
	"abs": {
		Name:       "abs",
		Params:     []ParamType{{Name: "x", Type: Int()}},
		ReturnType: Int(),
		Doc:        "Return the absolute value.",
	},
}

// BuiltinReturnType returns the return type of a builtin function.
// Returns nil if the function is not a known builtin.
func BuiltinReturnType(name string) TypeRef {
	if t, ok := builtinReturns[name]; ok {
		return t
	}
	return nil
}

// GetBuiltinSignature returns the full signature of a builtin function.
// Returns nil if the function is not a known builtin.
func GetBuiltinSignature(name string) *BuiltinSignature {
	return builtinSignatures[name]
}

// IsBuiltin returns true if the name is a known builtin function.
func IsBuiltin(name string) bool {
	_, ok := builtinReturns[name]
	return ok
}
