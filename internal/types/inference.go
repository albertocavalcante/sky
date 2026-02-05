package types

import (
	"strings"

	"github.com/bazelbuild/buildtools/build"
)

// InferExprType infers the type of a Starlark expression.
// Returns Unknown if the type cannot be determined.
func InferExprType(expr build.Expr) TypeRef {
	if expr == nil {
		return Unknown()
	}

	switch e := expr.(type) {
	case *build.LiteralExpr:
		return inferLiteral(e)

	case *build.StringExpr:
		return Str()

	case *build.ListExpr:
		return inferList(e)

	case *build.DictExpr:
		return inferDict(e)

	case *build.TupleExpr:
		return inferTuple(e)

	case *build.SetExpr:
		return inferSet(e)

	case *build.CallExpr:
		return inferCall(e)

	case *build.BinaryExpr:
		return inferBinary(e)

	case *build.UnaryExpr:
		return inferUnary(e)

	case *build.IndexExpr:
		return inferIndex(e)

	case *build.SliceExpr:
		return inferSlice(e)

	case *build.Comprehension:
		return inferComprehension(e)

	case *build.ConditionalExpr:
		return inferConditional(e)

	case *build.Ident:
		// Check for builtin constants
		switch e.Name {
		case "True", "False":
			return Bool()
		case "None":
			return None()
		}
		// For general identifiers, we would need scope information
		return Unknown()

	default:
		return Unknown()
	}
}

// inferLiteral infers the type of a literal expression (int, float, bool).
func inferLiteral(e *build.LiteralExpr) TypeRef {
	token := e.Token

	// Boolean literals
	if token == "True" || token == "False" {
		return Bool()
	}

	// Float: contains decimal point, exponent, or is explicitly float
	if strings.Contains(token, ".") ||
		strings.ContainsAny(token, "eE") {
		return Float()
	}

	// Otherwise, it's an integer
	return Int()
}

// inferList infers the type of a list literal.
func inferList(e *build.ListExpr) TypeRef {
	if len(e.List) == 0 {
		return List(Unknown())
	}

	// Infer element type from first element
	elemType := InferExprType(e.List[0])

	// Optionally, check if all elements have the same type
	// For simplicity, we just use the first element's type
	return List(elemType)
}

// inferDict infers the type of a dict literal.
func inferDict(e *build.DictExpr) TypeRef {
	if len(e.List) == 0 {
		return Dict(Unknown(), Unknown())
	}

	// Infer from first key-value pair
	first := e.List[0]
	keyType := InferExprType(first.Key)
	valType := InferExprType(first.Value)

	return Dict(keyType, valType)
}

// inferTuple infers the type of a tuple literal.
func inferTuple(e *build.TupleExpr) TypeRef {
	elemTypes := make([]TypeRef, len(e.List))
	for i, elem := range e.List {
		elemTypes[i] = InferExprType(elem)
	}
	return Tuple(elemTypes...)
}

// inferSet infers the type of a set literal.
func inferSet(e *build.SetExpr) TypeRef {
	if len(e.List) == 0 {
		return Set(Unknown())
	}

	// Infer element type from first element
	elemType := InferExprType(e.List[0])
	return Set(elemType)
}

// inferCall infers the type of a function call.
func inferCall(e *build.CallExpr) TypeRef {
	switch fn := e.X.(type) {
	case *build.Ident:
		// Check builtin functions
		if ret := BuiltinReturnType(fn.Name); ret != nil {
			return ret
		}

		// Type conversion calls might be more specific
		// For example, list() on a known iterable
		switch fn.Name {
		case "list":
			// If argument is known, we might be able to infer element type
			if len(e.List) > 0 {
				argType := InferExprType(e.List[0])
				if elemType := ElementType(argType); elemType != nil {
					return List(elemType)
				}
			}
		case "dict":
			// dict() with keyword args
			if len(e.List) > 0 {
				// Check if arguments are all keyword assignments
				allKeyword := true
				var valType TypeRef
				for _, arg := range e.List {
					if assign, ok := arg.(*build.AssignExpr); ok {
						if valType == nil {
							valType = InferExprType(assign.RHS)
						}
					} else {
						allKeyword = false
						break
					}
				}
				if allKeyword && valType != nil {
					return Dict(Str(), valType)
				}
			}
		case "tuple":
			if len(e.List) > 0 {
				argType := InferExprType(e.List[0])
				if elemType := ElementType(argType); elemType != nil {
					return Tuple(elemType)
				}
			}
		case "set":
			if len(e.List) > 0 {
				argType := InferExprType(e.List[0])
				if elemType := ElementType(argType); elemType != nil {
					return Set(elemType)
				}
			}
		case "struct":
			return &NamedType{Name: "struct"}
		case "depset":
			if len(e.List) > 0 {
				argType := InferExprType(e.List[0])
				if elemType := ElementType(argType); elemType != nil {
					return &NamedType{Name: "depset", Args: []TypeRef{elemType}}
				}
			}
			return &NamedType{Name: "depset", Args: []TypeRef{Unknown()}}
		}

	case *build.DotExpr:
		// Method calls: x.method()
		// We could infer based on known method return types
		return inferMethodCall(fn, e.List)
	}

	return Unknown()
}

// inferMethodCall infers the type of a method call on an object.
func inferMethodCall(dot *build.DotExpr, args []build.Expr) TypeRef {
	method := dot.Name

	// String methods
	stringMethods := map[string]TypeRef{
		"capitalize":   Str(),
		"count":        Int(),
		"endswith":     Bool(),
		"find":         Int(),
		"format":       Str(),
		"index":        Int(),
		"isalnum":      Bool(),
		"isalpha":      Bool(),
		"isdigit":      Bool(),
		"islower":      Bool(),
		"isspace":      Bool(),
		"istitle":      Bool(),
		"isupper":      Bool(),
		"join":         Str(),
		"lower":        Str(),
		"lstrip":       Str(),
		"partition":    Tuple(Str(), Str(), Str()),
		"removeprefix": Str(),
		"removesuffix": Str(),
		"replace":      Str(),
		"rfind":        Int(),
		"rindex":       Int(),
		"rpartition":   Tuple(Str(), Str(), Str()),
		"rsplit":       List(Str()),
		"rstrip":       Str(),
		"split":        List(Str()),
		"splitlines":   List(Str()),
		"startswith":   Bool(),
		"strip":        Str(),
		"title":        Str(),
		"upper":        Str(),
	}

	if ret, ok := stringMethods[method]; ok {
		// Check if the receiver is a string
		recvType := InferExprType(dot.X)
		if _, isStr := recvType.(*NamedType); isStr {
			if recvType.String() == "str" {
				return ret
			}
		}
	}

	// List methods
	switch method {
	case "append", "extend", "insert", "remove", "clear", "reverse":
		return None()
	case "pop":
		recvType := InferExprType(dot.X)
		if elemType := ElementType(recvType); elemType != nil {
			return elemType
		}
	case "index":
		return Int()
	case "count":
		return Int()
	case "copy":
		return InferExprType(dot.X)
	}

	// Dict methods
	switch method {
	case "get":
		recvType := InferExprType(dot.X)
		if named, ok := recvType.(*NamedType); ok && named.Name == "dict" && len(named.Args) > 1 {
			// Could return value type or None
			return Union(named.Args[1], None())
		}
	case "keys":
		recvType := InferExprType(dot.X)
		if named, ok := recvType.(*NamedType); ok && named.Name == "dict" && len(named.Args) > 0 {
			return List(named.Args[0])
		}
		return List(Unknown())
	case "values":
		recvType := InferExprType(dot.X)
		if named, ok := recvType.(*NamedType); ok && named.Name == "dict" && len(named.Args) > 1 {
			return List(named.Args[1])
		}
		return List(Unknown())
	case "items":
		recvType := InferExprType(dot.X)
		if named, ok := recvType.(*NamedType); ok && named.Name == "dict" && len(named.Args) > 1 {
			return List(Tuple(named.Args[0], named.Args[1]))
		}
		return List(Tuple(Unknown(), Unknown()))
	case "update", "setdefault", "pop", "popitem", "clear":
		return None()
	}

	return Unknown()
}

// inferBinary infers the type of a binary operation.
func inferBinary(e *build.BinaryExpr) TypeRef {
	op := e.Op

	// Comparison operators always return bool
	switch op {
	case "==", "!=", "<", ">", "<=", ">=", "in", "not in", "is", "is not":
		return Bool()
	case "and", "or":
		// Logical operators return one of the operands
		// In Starlark, 'and' returns the first falsy or the last value
		// 'or' returns the first truthy or the last value
		// For type inference, we can say it returns one of the types
		leftType := InferExprType(e.X)
		rightType := InferExprType(e.Y)
		if leftType.Equal(rightType) {
			return leftType
		}
		return Union(leftType, rightType)
	}

	leftType := InferExprType(e.X)
	rightType := InferExprType(e.Y)

	switch op {
	case "+":
		// String concatenation
		if isStringType(leftType) && isStringType(rightType) {
			return Str()
		}
		// List concatenation
		if isListType(leftType) && isListType(rightType) {
			return leftType
		}
		// Numeric addition
		if isNumericType(leftType) && isNumericType(rightType) {
			// If either is float, result is float
			if isFloatType(leftType) || isFloatType(rightType) {
				return Float()
			}
			return Int()
		}

	case "-", "*", "//", "%":
		// Numeric operations
		if isNumericType(leftType) && isNumericType(rightType) {
			if isFloatType(leftType) || isFloatType(rightType) {
				return Float()
			}
			return Int()
		}
		// String repetition
		if op == "*" && isStringType(leftType) && isIntType(rightType) {
			return Str()
		}
		if op == "*" && isListType(leftType) && isIntType(rightType) {
			return leftType
		}

	case "/":
		// Division always returns float in Python 3 style
		return Float()

	case "|":
		// Dict merge (Python 3.9+)
		if isDictType(leftType) && isDictType(rightType) {
			return leftType
		}
		// Set union
		if isSetType(leftType) && isSetType(rightType) {
			return leftType
		}
		// Bitwise or for ints
		if isIntType(leftType) && isIntType(rightType) {
			return Int()
		}

	case "&", "^":
		// Set operations or bitwise
		if isSetType(leftType) && isSetType(rightType) {
			return leftType
		}
		if isIntType(leftType) && isIntType(rightType) {
			return Int()
		}

	case "<<", ">>":
		// Bit shifts
		return Int()
	}

	return Unknown()
}

// inferUnary infers the type of a unary operation.
func inferUnary(e *build.UnaryExpr) TypeRef {
	switch e.Op {
	case "not":
		return Bool()
	case "-", "+":
		innerType := InferExprType(e.X)
		if isNumericType(innerType) {
			return innerType
		}
		return Int()
	case "~":
		return Int()
	}
	return Unknown()
}

// inferIndex infers the type of an index expression (e.g., x[i]).
func inferIndex(e *build.IndexExpr) TypeRef {
	baseType := InferExprType(e.X)

	// String indexing returns string
	if isStringType(baseType) {
		return Str()
	}

	// List indexing returns element type
	if elemType := ElementType(baseType); elemType != nil {
		return elemType
	}

	// Dict indexing returns value type
	if named, ok := baseType.(*NamedType); ok && named.Name == "dict" && len(named.Args) > 1 {
		return named.Args[1]
	}

	// Tuple indexing could return specific element type
	// but we'd need to know the index value

	return Unknown()
}

// inferSlice infers the type of a slice expression (e.g., x[i:j]).
func inferSlice(e *build.SliceExpr) TypeRef {
	baseType := InferExprType(e.X)

	// Slicing preserves the collection type
	if isStringType(baseType) {
		return Str()
	}
	if isListType(baseType) {
		return baseType
	}
	if named, ok := baseType.(*NamedType); ok && named.Name == "tuple" {
		// Tuple slice returns tuple (but we lose specific element types)
		return Tuple(Unknown())
	}

	return Unknown()
}

// inferComprehension infers the type of a list/dict/set comprehension.
func inferComprehension(e *build.Comprehension) TypeRef {
	// Determine if it's a list, dict, or set comprehension
	// based on the body expression
	switch body := e.Body.(type) {
	case *build.KeyValueExpr:
		// Dict comprehension
		keyType := InferExprType(body.Key)
		valType := InferExprType(body.Value)
		return Dict(keyType, valType)
	default:
		// List comprehension (or generator)
		elemType := InferExprType(e.Body)
		// Check if curly braces indicate set comprehension
		// The AST doesn't distinguish easily, so we assume list
		if e.Curly {
			return Set(elemType)
		}
		return List(elemType)
	}
}

// inferConditional infers the type of a conditional expression (x if cond else y).
func inferConditional(e *build.ConditionalExpr) TypeRef {
	thenType := InferExprType(e.Then)
	elseType := InferExprType(e.Else)

	if thenType.Equal(elseType) {
		return thenType
	}
	return Union(thenType, elseType)
}

// Type checking helpers

func isStringType(t TypeRef) bool {
	if named, ok := t.(*NamedType); ok {
		return named.Name == "str"
	}
	return false
}

func isIntType(t TypeRef) bool {
	if named, ok := t.(*NamedType); ok {
		return named.Name == "int"
	}
	return false
}

func isFloatType(t TypeRef) bool {
	if named, ok := t.(*NamedType); ok {
		return named.Name == "float"
	}
	return false
}

func isNumericType(t TypeRef) bool {
	return isIntType(t) || isFloatType(t)
}

func isListType(t TypeRef) bool {
	if named, ok := t.(*NamedType); ok {
		return named.Name == "list"
	}
	return false
}

func isDictType(t TypeRef) bool {
	if named, ok := t.(*NamedType); ok {
		return named.Name == "dict"
	}
	return false
}

func isSetType(t TypeRef) bool {
	if named, ok := t.(*NamedType); ok {
		return named.Name == "set"
	}
	return false
}
