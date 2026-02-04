// Package tester provides a test runner for Starlark files.
package tester

import (
	"fmt"
	"strings"

	"go.starlark.net/starlark"
)

// FixtureScope defines when a fixture is instantiated.
type FixtureScope string

const (
	// ScopeTest creates a fresh fixture instance for each test (default).
	ScopeTest FixtureScope = "test"
	// ScopeFile shares a fixture instance within a file.
	ScopeFile FixtureScope = "file"
)

// FixturePrefix is the prefix for fixture function names.
const FixturePrefix = "fixture_"

// Fixture represents a fixture function and its metadata.
type Fixture struct {
	// Name is the fixture name (without fixture_ prefix).
	Name string
	// Fn is the Starlark function that creates the fixture value.
	Fn *starlark.Function
	// Scope determines when the fixture is instantiated.
	Scope FixtureScope
}

// FixtureRegistry holds all available fixtures for a test file.
type FixtureRegistry struct {
	fixtures map[string]*Fixture
	// cache holds computed fixture values for file-scoped fixtures
	cache map[string]starlark.Value
	// builtins holds pre-computed builtin fixture values (e.g., mock)
	builtins map[string]starlark.Value
}

// NewFixtureRegistry creates a new fixture registry.
func NewFixtureRegistry() *FixtureRegistry {
	return &FixtureRegistry{
		fixtures: make(map[string]*Fixture),
		cache:    make(map[string]starlark.Value),
		builtins: make(map[string]starlark.Value),
	}
}

// RegisterBuiltin registers a built-in fixture value that doesn't need computation.
func (r *FixtureRegistry) RegisterBuiltin(name string, value starlark.Value) {
	r.builtins[name] = value
}

// Register adds a fixture to the registry.
func (r *FixtureRegistry) Register(f *Fixture) {
	r.fixtures[f.Name] = f
}

// Get returns a fixture by name.
func (r *FixtureRegistry) Get(name string) (*Fixture, bool) {
	f, ok := r.fixtures[name]
	return f, ok
}

// ClearTestCache clears cached fixture values for test-scoped fixtures.
// This should be called between tests.
func (r *FixtureRegistry) ClearTestCache() {
	// Only clear test-scoped fixtures from cache
	for name, fixture := range r.fixtures {
		if fixture.Scope == ScopeTest {
			delete(r.cache, name)
		}
	}
}

// GetOrCompute returns the fixture value, computing it if necessary.
func (r *FixtureRegistry) GetOrCompute(thread *starlark.Thread, name string, registry *FixtureRegistry) (starlark.Value, error) {
	// Check builtins first (e.g., mock)
	if builtin, ok := r.builtins[name]; ok {
		return builtin, nil
	}

	fixture, ok := r.fixtures[name]
	if !ok {
		return nil, fmt.Errorf("fixture %q not found", name)
	}

	// Check cache for file-scoped fixtures
	if fixture.Scope == ScopeFile {
		if val, ok := r.cache[name]; ok {
			return val, nil
		}
	}

	// Compute the fixture value
	// First resolve any dependencies the fixture might have
	args, err := r.resolveFixtureArgs(thread, fixture.Fn, registry)
	if err != nil {
		return nil, fmt.Errorf("resolving fixture %q dependencies: %w", name, err)
	}

	val, err := starlark.Call(thread, fixture.Fn, args, nil)
	if err != nil {
		return nil, fmt.Errorf("calling fixture %q: %w", name, err)
	}

	// Cache file-scoped fixtures
	if fixture.Scope == ScopeFile {
		r.cache[name] = val
	}

	return val, nil
}

// resolveFixtureArgs resolves dependencies for a fixture function.
func (r *FixtureRegistry) resolveFixtureArgs(thread *starlark.Thread, fn *starlark.Function, registry *FixtureRegistry) (starlark.Tuple, error) {
	numParams := fn.NumParams()
	if numParams == 0 {
		return nil, nil
	}

	args := make(starlark.Tuple, numParams)
	for i := 0; i < numParams; i++ {
		paramName, _ := fn.Param(i)
		val, err := registry.GetOrCompute(thread, paramName, registry)
		if err != nil {
			return nil, err
		}
		args[i] = val
	}
	return args, nil
}

// FindFixtures extracts fixture functions from globals.
func FindFixtures(globals starlark.StringDict) *FixtureRegistry {
	registry := NewFixtureRegistry()

	for name, val := range globals {
		fn, ok := val.(*starlark.Function)
		if !ok {
			continue
		}

		if !strings.HasPrefix(name, FixturePrefix) {
			continue
		}

		fixtureName := strings.TrimPrefix(name, FixturePrefix)
		scope := ScopeTest // default scope

		// Check for scope configuration via __fixture_config__ dict
		if configVal, ok := globals["__fixture_config__"]; ok {
			if configDict, ok := configVal.(*starlark.Dict); ok {
				if scopeVal, found, _ := configDict.Get(starlark.String(fixtureName)); found {
					if scopeStr, ok := scopeVal.(starlark.String); ok {
						switch string(scopeStr) {
						case "file":
							scope = ScopeFile
						case "test":
							scope = ScopeTest
						}
					}
				}
			}
		}

		registry.Register(&Fixture{
			Name:  fixtureName,
			Fn:    fn,
			Scope: scope,
		})
	}

	return registry
}

// ResolveTestArgs resolves fixture arguments for a test function.
func ResolveTestArgs(thread *starlark.Thread, testFn *starlark.Function, registry *FixtureRegistry) (starlark.Tuple, error) {
	numParams := testFn.NumParams()
	if numParams == 0 {
		return nil, nil
	}

	args := make(starlark.Tuple, numParams)
	for i := 0; i < numParams; i++ {
		paramName, _ := testFn.Param(i)
		val, err := registry.GetOrCompute(thread, paramName, registry)
		if err != nil {
			return nil, err
		}
		args[i] = val
	}
	return args, nil
}
