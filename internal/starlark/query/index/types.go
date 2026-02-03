// Package index provides file indexing capabilities for skyquery.
// It extracts structured data from Starlark ASTs including function definitions,
// load statements, function calls, and top-level assignments.
package index

import (
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// File represents a parsed Starlark file with extracted structural information.
type File struct {
	// Path is the file path relative to the workspace root.
	Path string

	// Kind is the type of Starlark file (BUILD, bzl, star, etc.).
	Kind filekind.Kind

	// Defs contains all function definitions in the file.
	Defs []Def

	// Loads contains all load statements in the file.
	Loads []Load

	// Calls contains all top-level function calls in the file.
	Calls []Call

	// Assigns contains all top-level assignments in the file.
	Assigns []Assign
}

// Def represents a function definition.
type Def struct {
	// Name is the function name.
	Name string

	// File is the path to the file containing this definition.
	File string

	// Line is the line number where the definition starts (1-based).
	Line int

	// Params is the list of parameter names.
	Params []string

	// Docstring is the function's docstring, if present.
	Docstring string
}

// Load represents a load statement.
type Load struct {
	// Module is the module being loaded (e.g., "//lib:utils.bzl" or "@repo//pkg:file.star").
	Module string

	// Symbols maps local names to exported names.
	// For `load("//lib:utils.bzl", "foo", bar = "baz")`:
	// - "foo" -> "foo" (same local and exported name)
	// - "bar" -> "baz" (local "bar" refers to exported "baz")
	Symbols map[string]string

	// File is the path to the file containing this load statement.
	File string

	// Line is the line number of the load statement (1-based).
	Line int
}

// Call represents a function call at the top level of a file.
type Call struct {
	// Function is the name of the function being called.
	Function string

	// Args contains the arguments passed to the function.
	Args []Arg

	// File is the path to the file containing this call.
	File string

	// Line is the line number of the call (1-based).
	Line int
}

// Arg represents a function argument.
type Arg struct {
	// Name is the argument name for keyword arguments, empty for positional arguments.
	Name string

	// Value is the string representation of the argument value.
	Value string
}

// Assign represents a top-level assignment.
type Assign struct {
	// Name is the name of the variable being assigned.
	Name string

	// File is the path to the file containing this assignment.
	File string

	// Line is the line number of the assignment (1-based).
	Line int
}
