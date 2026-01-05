// Package resolver provides interfaces for resolving Starlark load() statements.
package resolver

// ModuleID uniquely identifies a module for caching and deduplication.
type ModuleID string

// Resolution contains the result of resolving a load statement.
type Resolution struct {
	// ModuleID is the canonical identifier for this module.
	// Used for caching and detecting duplicate loads.
	ModuleID ModuleID

	// Candidates contains file paths that could satisfy this load,
	// in priority order (first is preferred).
	Candidates []string

	// External is true if this is an external/remote dependency
	// (e.g., from a Bazel external repository).
	External bool

	// Error is non-nil if resolution failed.
	Error error
}

// OK returns true if the resolution succeeded.
func (r Resolution) OK() bool {
	return r.Error == nil && len(r.Candidates) > 0
}

// LoadResolver resolves load() statements to file paths.
type LoadResolver interface {
	// ResolveLoad resolves a load string from a source file.
	//
	// fromFile is the absolute path of the file containing the load() statement.
	// loadString is the first argument to load(), e.g., "//foo:bar.bzl" or ":lib.bzl".
	//
	// Returns a Resolution containing candidate file paths.
	ResolveLoad(fromFile, loadString string) Resolution

	// WorkspaceRoot returns the workspace root path, if known.
	// Returns empty string if the workspace root cannot be determined.
	WorkspaceRoot() string
}

// ResolverFunc is a function type that implements LoadResolver.
type ResolverFunc struct {
	ResolveFn         func(fromFile, loadString string) Resolution
	WorkspaceRootPath string
}

// ResolveLoad implements the LoadResolver interface.
func (f ResolverFunc) ResolveLoad(fromFile, loadString string) Resolution {
	if f.ResolveFn != nil {
		return f.ResolveFn(fromFile, loadString)
	}
	return Resolution{Error: ErrNoResolver}
}

// WorkspaceRoot implements the LoadResolver interface.
func (f ResolverFunc) WorkspaceRoot() string {
	return f.WorkspaceRootPath
}

// Common errors for resolution.
var (
	// ErrNoResolver indicates no resolver is configured.
	ErrNoResolver = &resolverError{msg: "no resolver configured"}

	// ErrModuleNotFound indicates the module could not be found.
	ErrModuleNotFound = &resolverError{msg: "module not found"}

	// ErrInvalidLoadString indicates the load string is malformed.
	ErrInvalidLoadString = &resolverError{msg: "invalid load string"}
)

type resolverError struct {
	msg string
}

func (e *resolverError) Error() string {
	return e.msg
}
