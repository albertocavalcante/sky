// Package tester provides mocking support for Starlark tests.
package tester

import (
	"fmt"
	"sync"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// MockManagerKey is the thread-local key for the mock manager.
const MockManagerKey = "skytest.mock_manager"

// MockManager tracks all mocks and their configurations for a test.
type MockManager struct {
	mu      sync.Mutex
	mocks   map[*MockWrapper]*mockConfig
	nextID  int
	wrapped map[starlark.Value]*MockWrapper
}

// mockConfig holds configuration for a mock wrapper.
type mockConfig struct {
	// returnValue is the default return value
	returnValue starlark.Value
	// returnValues maps argument patterns to return values
	returnValues map[string]starlark.Value
	// calls tracks all calls made to this mock
	calls []mockCall
}

// mockCall records a single call to a mock.
type mockCall struct {
	args   starlark.Tuple
	kwargs []starlark.Tuple
}

// NewMockManager creates a new mock manager.
func NewMockManager() *MockManager {
	return &MockManager{
		mocks:   make(map[*MockWrapper]*mockConfig),
		wrapped: make(map[starlark.Value]*MockWrapper),
	}
}

// Reset clears all mock configurations and call history.
func (mm *MockManager) Reset() {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.mocks = make(map[*MockWrapper]*mockConfig)
	mm.wrapped = make(map[starlark.Value]*MockWrapper)
	mm.nextID = 0
}

// Wrap wraps a callable to track calls and allow configuration.
func (mm *MockManager) Wrap(fn starlark.Value) (*MockWrapper, error) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Check if already wrapped
	if wrapper, ok := mm.wrapped[fn]; ok {
		return wrapper, nil
	}

	// Verify it's callable
	if _, ok := fn.(starlark.Callable); !ok {
		return nil, fmt.Errorf("mock.wrap: expected callable, got %s", fn.Type())
	}

	mm.nextID++
	wrapper := &MockWrapper{
		id:      mm.nextID,
		wrapped: fn,
		manager: mm,
	}

	mm.mocks[wrapper] = &mockConfig{
		returnValues: make(map[string]starlark.Value),
	}
	mm.wrapped[fn] = wrapper

	return wrapper, nil
}

// getConfig returns the configuration for a mock wrapper.
func (mm *MockManager) getConfig(wrapper *MockWrapper) *mockConfig {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	return mm.mocks[wrapper]
}

// recordCall records a call to a mock.
func (mm *MockManager) recordCall(wrapper *MockWrapper, args starlark.Tuple, kwargs []starlark.Tuple) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	cfg := mm.mocks[wrapper]
	if cfg != nil {
		cfg.calls = append(cfg.calls, mockCall{args: args, kwargs: kwargs})
	}
}

// setReturn sets the default return value for a mock.
func (mm *MockManager) setReturn(wrapper *MockWrapper, value starlark.Value) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	cfg := mm.mocks[wrapper]
	if cfg != nil {
		cfg.returnValue = value
	}
}

// setReturnForArgs sets the return value for specific arguments.
func (mm *MockManager) setReturnForArgs(wrapper *MockWrapper, argsKey string, value starlark.Value) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	cfg := mm.mocks[wrapper]
	if cfg != nil {
		cfg.returnValues[argsKey] = value
	}
}

// getReturn returns the configured return value for given arguments.
func (mm *MockManager) getReturn(wrapper *MockWrapper, args starlark.Tuple) starlark.Value {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	cfg := mm.mocks[wrapper]
	if cfg == nil {
		return nil
	}

	// Check for specific args match
	argsKey := argsToKey(args)
	if ret, ok := cfg.returnValues[argsKey]; ok {
		return ret
	}

	// Fall back to default return value
	return cfg.returnValue
}

// wasCalled returns true if the mock was called at least once.
func (mm *MockManager) wasCalled(wrapper *MockWrapper) bool {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	cfg := mm.mocks[wrapper]
	return cfg != nil && len(cfg.calls) > 0
}

// callCount returns the number of times the mock was called.
func (mm *MockManager) callCount(wrapper *MockWrapper) int {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	cfg := mm.mocks[wrapper]
	if cfg == nil {
		return 0
	}
	return len(cfg.calls)
}

// getCalls returns all recorded calls for a mock.
func (mm *MockManager) getCalls(wrapper *MockWrapper) []mockCall {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	cfg := mm.mocks[wrapper]
	if cfg == nil {
		return nil
	}
	// Return a copy to avoid race conditions
	result := make([]mockCall, len(cfg.calls))
	copy(result, cfg.calls)
	return result
}

// argsToKey converts arguments to a string key for lookup.
func argsToKey(args starlark.Tuple) string {
	if len(args) == 0 {
		return "()"
	}
	return args.String()
}

// MockWrapper wraps a callable to track calls and configure behavior.
type MockWrapper struct {
	id      int
	wrapped starlark.Value
	manager *MockManager
}

// String implements starlark.Value.
func (m *MockWrapper) String() string {
	return fmt.Sprintf("<mock #%d wrapping %s>", m.id, m.wrapped.String())
}

// Type implements starlark.Value.
func (m *MockWrapper) Type() string {
	return "mock"
}

// Freeze implements starlark.Value.
func (m *MockWrapper) Freeze() {}

// Truth implements starlark.Value.
func (m *MockWrapper) Truth() starlark.Bool {
	return true
}

// Hash implements starlark.Value.
func (m *MockWrapper) Hash() (uint32, error) {
	return uint32(m.id), nil
}

// Name implements starlark.Callable.
func (m *MockWrapper) Name() string {
	if callable, ok := m.wrapped.(starlark.Callable); ok {
		return callable.Name()
	}
	return "mock"
}

// CallInternal implements starlark.Callable.
func (m *MockWrapper) CallInternal(thread *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// Record the call
	m.manager.recordCall(m, args, kwargs)

	// Check for configured return value
	if ret := m.manager.getReturn(m, args); ret != nil {
		return ret, nil
	}

	// Fall through to wrapped function if no return configured
	callable, ok := m.wrapped.(starlark.Callable)
	if !ok {
		return starlark.None, nil
	}

	return starlark.Call(thread, callable, args, kwargs)
}

// MockWhen is a builder for configuring mock behavior.
type MockWhen struct {
	wrapper *MockWrapper
	args    starlark.Tuple
}

// String implements starlark.Value.
func (w *MockWhen) String() string {
	return fmt.Sprintf("<mock.when(%s)>", w.wrapper.String())
}

// Type implements starlark.Value.
func (w *MockWhen) Type() string {
	return "mock_when"
}

// Freeze implements starlark.Value.
func (w *MockWhen) Freeze() {}

// Truth implements starlark.Value.
func (w *MockWhen) Truth() starlark.Bool {
	return true
}

// Hash implements starlark.Value.
func (w *MockWhen) Hash() (uint32, error) {
	return 0, fmt.Errorf("mock_when is not hashable")
}

// Attr implements starlark.HasAttrs.
func (w *MockWhen) Attr(name string) (starlark.Value, error) {
	switch name {
	case "then_return":
		return starlark.NewBuiltin("then_return", w.thenReturn), nil
	case "called_with":
		return starlark.NewBuiltin("called_with", w.calledWith), nil
	default:
		return nil, nil
	}
}

// AttrNames implements starlark.HasAttrs.
func (w *MockWhen) AttrNames() []string {
	return []string{"then_return", "called_with"}
}

// thenReturn sets the return value for this mock configuration.
func (w *MockWhen) thenReturn(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var value starlark.Value
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "value", &value); err != nil {
		return nil, err
	}

	if w.args != nil {
		// Set return for specific args
		w.wrapper.manager.setReturnForArgs(w.wrapper, argsToKey(w.args), value)
	} else {
		// Set default return
		w.wrapper.manager.setReturn(w.wrapper, value)
	}

	return w.wrapper, nil
}

// calledWith specifies the arguments this configuration applies to.
func (w *MockWhen) calledWith(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return &MockWhen{
		wrapper: w.wrapper,
		args:    args,
	}, nil
}

// NewMockFixture creates the mock fixture value.
// This is injected into tests that request a "mock" parameter.
func NewMockFixture(manager *MockManager) *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "mock",
		Members: starlark.StringDict{
			"wrap":       starlark.NewBuiltin("mock.wrap", mockWrap(manager)),
			"when":       starlark.NewBuiltin("mock.when", mockWhen(manager)),
			"was_called": starlark.NewBuiltin("mock.was_called", mockWasCalled(manager)),
			"call_count": starlark.NewBuiltin("mock.call_count", mockCallCount(manager)),
			"calls":      starlark.NewBuiltin("mock.calls", mockCalls(manager)),
			"reset":      starlark.NewBuiltin("mock.reset", mockReset(manager)),
		},
	}
}

// mockWrap wraps a callable to track calls.
func mockWrap(manager *MockManager) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var fn starlark.Value
		if err := starlark.UnpackArgs(b.Name(), args, kwargs, "fn", &fn); err != nil {
			return nil, err
		}

		wrapper, err := manager.Wrap(fn)
		if err != nil {
			return nil, err
		}

		return wrapper, nil
	}
}

// mockWhen starts configuring a mock's behavior.
func mockWhen(manager *MockManager) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var fn starlark.Value
		if err := starlark.UnpackArgs(b.Name(), args, kwargs, "fn", &fn); err != nil {
			return nil, err
		}

		wrapper, ok := fn.(*MockWrapper)
		if !ok {
			return nil, fmt.Errorf("mock.when: expected mock wrapper, got %s (use mock.wrap() first)", fn.Type())
		}

		return &MockWhen{wrapper: wrapper}, nil
	}
}

// mockWasCalled checks if a mock was called.
func mockWasCalled(manager *MockManager) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var fn starlark.Value
		if err := starlark.UnpackArgs(b.Name(), args, kwargs, "fn", &fn); err != nil {
			return nil, err
		}

		wrapper, ok := fn.(*MockWrapper)
		if !ok {
			return nil, fmt.Errorf("mock.was_called: expected mock wrapper, got %s", fn.Type())
		}

		return starlark.Bool(manager.wasCalled(wrapper)), nil
	}
}

// mockCallCount returns the number of times a mock was called.
func mockCallCount(manager *MockManager) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var fn starlark.Value
		if err := starlark.UnpackArgs(b.Name(), args, kwargs, "fn", &fn); err != nil {
			return nil, err
		}

		wrapper, ok := fn.(*MockWrapper)
		if !ok {
			return nil, fmt.Errorf("mock.call_count: expected mock wrapper, got %s", fn.Type())
		}

		return starlark.MakeInt(manager.callCount(wrapper)), nil
	}
}

// mockCalls returns a list of all calls made to a mock.
func mockCalls(manager *MockManager) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var fn starlark.Value
		if err := starlark.UnpackArgs(b.Name(), args, kwargs, "fn", &fn); err != nil {
			return nil, err
		}

		wrapper, ok := fn.(*MockWrapper)
		if !ok {
			return nil, fmt.Errorf("mock.calls: expected mock wrapper, got %s", fn.Type())
		}

		calls := manager.getCalls(wrapper)
		result := make([]starlark.Value, len(calls))
		for i, call := range calls {
			// Convert to a dict with "args" and "kwargs" keys
			callDict := starlark.NewDict(2)
			callDict.SetKey(starlark.String("args"), starlark.Tuple(call.args))

			// Convert kwargs to dict
			kwargsDict := starlark.NewDict(len(call.kwargs))
			for _, kw := range call.kwargs {
				if len(kw) == 2 {
					kwargsDict.SetKey(kw[0], kw[1])
				}
			}
			callDict.SetKey(starlark.String("kwargs"), kwargsDict)

			result[i] = callDict
		}

		return starlark.NewList(result), nil
	}
}

// mockReset clears all mock configurations.
func mockReset(manager *MockManager) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		manager.Reset()
		return starlark.None, nil
	}
}
