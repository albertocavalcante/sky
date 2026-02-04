package tester

import (
	"testing"

	"go.starlark.net/starlark"
)

func TestMockManager_Wrap(t *testing.T) {
	mm := NewMockManager()

	// Create a simple function to wrap
	fn := starlark.NewBuiltin("test_fn", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return starlark.String("original"), nil
	})

	wrapper, err := mm.Wrap(fn)
	if err != nil {
		t.Fatalf("Wrap failed: %v", err)
	}

	if wrapper == nil {
		t.Fatal("Wrap returned nil wrapper")
	}

	// Verify wrapper type
	if wrapper.Type() != "mock" {
		t.Errorf("Expected type 'mock', got %q", wrapper.Type())
	}

	// Verify wrapping the same function returns the same wrapper
	wrapper2, err := mm.Wrap(fn)
	if err != nil {
		t.Fatalf("Second Wrap failed: %v", err)
	}
	if wrapper != wrapper2 {
		t.Error("Expected same wrapper for same function")
	}
}

func TestMockManager_WrapNonCallable(t *testing.T) {
	mm := NewMockManager()

	// Try to wrap a non-callable
	_, err := mm.Wrap(starlark.String("not callable"))
	if err == nil {
		t.Error("Expected error wrapping non-callable")
	}
}

func TestMockWrapper_CallTracking(t *testing.T) {
	mm := NewMockManager()

	fn := starlark.NewBuiltin("test_fn", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return starlark.String("original"), nil
	})

	wrapper, _ := mm.Wrap(fn)

	// Initially not called
	if mm.wasCalled(wrapper) {
		t.Error("Expected wasCalled to be false initially")
	}
	if mm.callCount(wrapper) != 0 {
		t.Errorf("Expected callCount 0, got %d", mm.callCount(wrapper))
	}

	// Call the wrapper
	thread := &starlark.Thread{Name: "test"}
	result, err := wrapper.CallInternal(thread, starlark.Tuple{starlark.String("arg1")}, nil)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	// Should return original result (no mock configured)
	if result != starlark.String("original") {
		t.Errorf("Expected 'original', got %v", result)
	}

	// Now should be called
	if !mm.wasCalled(wrapper) {
		t.Error("Expected wasCalled to be true")
	}
	if mm.callCount(wrapper) != 1 {
		t.Errorf("Expected callCount 1, got %d", mm.callCount(wrapper))
	}

	// Check recorded calls
	calls := mm.getCalls(wrapper)
	if len(calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(calls))
	}
	if len(calls[0].args) != 1 {
		t.Errorf("Expected 1 arg, got %d", len(calls[0].args))
	}
}

func TestMockManager_SetReturn(t *testing.T) {
	mm := NewMockManager()

	fn := starlark.NewBuiltin("test_fn", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return starlark.String("original"), nil
	})

	wrapper, _ := mm.Wrap(fn)

	// Configure return value
	mm.setReturn(wrapper, starlark.MakeInt(42))

	// Call should return configured value
	thread := &starlark.Thread{Name: "test"}
	result, err := wrapper.CallInternal(thread, nil, nil)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	if result != starlark.MakeInt(42) {
		t.Errorf("Expected 42, got %v", result)
	}
}

func TestMockManager_SetReturnForArgs(t *testing.T) {
	mm := NewMockManager()

	fn := starlark.NewBuiltin("test_fn", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return starlark.String("original"), nil
	})

	wrapper, _ := mm.Wrap(fn)

	// Configure return for specific args
	argsKey := argsToKey(starlark.Tuple{starlark.String("special")})
	mm.setReturnForArgs(wrapper, argsKey, starlark.String("mocked"))

	// Configure default return
	mm.setReturn(wrapper, starlark.String("default"))

	thread := &starlark.Thread{Name: "test"}

	// Call with matching args
	result, _ := wrapper.CallInternal(thread, starlark.Tuple{starlark.String("special")}, nil)
	if result != starlark.String("mocked") {
		t.Errorf("Expected 'mocked' for special args, got %v", result)
	}

	// Call with other args
	result, _ = wrapper.CallInternal(thread, starlark.Tuple{starlark.String("other")}, nil)
	if result != starlark.String("default") {
		t.Errorf("Expected 'default' for other args, got %v", result)
	}
}

func TestMockManager_Reset(t *testing.T) {
	mm := NewMockManager()

	fn := starlark.NewBuiltin("test_fn", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return starlark.String("original"), nil
	})

	wrapper, _ := mm.Wrap(fn)

	// Record a call
	thread := &starlark.Thread{Name: "test"}
	wrapper.CallInternal(thread, nil, nil)

	if !mm.wasCalled(wrapper) {
		t.Error("Expected wasCalled to be true")
	}

	// Reset
	mm.Reset()

	// The wrapper is now invalid (not in mocks map)
	// Create new wrapper after reset
	wrapper2, _ := mm.Wrap(fn)
	if mm.wasCalled(wrapper2) {
		t.Error("Expected wasCalled to be false after reset")
	}
}

func TestMockWhen_ThenReturn(t *testing.T) {
	mm := NewMockManager()

	fn := starlark.NewBuiltin("test_fn", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return starlark.String("original"), nil
	})

	wrapper, _ := mm.Wrap(fn)

	when := &MockWhen{wrapper: wrapper}

	thread := &starlark.Thread{Name: "test"}

	// Configure via then_return
	thenReturnFn, _ := when.Attr("then_return")
	builtin := thenReturnFn.(*starlark.Builtin)
	builtin.CallInternal(thread, starlark.Tuple{starlark.String("mocked value")}, nil)

	// Call wrapper should return mocked value
	result, _ := wrapper.CallInternal(thread, nil, nil)
	if result != starlark.String("mocked value") {
		t.Errorf("Expected 'mocked value', got %v", result)
	}
}

func TestMockWhen_CalledWith_ThenReturn(t *testing.T) {
	mm := NewMockManager()

	fn := starlark.NewBuiltin("test_fn", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return starlark.String("original"), nil
	})

	wrapper, _ := mm.Wrap(fn)

	when := &MockWhen{wrapper: wrapper}

	thread := &starlark.Thread{Name: "test"}

	// Get called_with
	calledWithFn, _ := when.Attr("called_with")
	builtin := calledWithFn.(*starlark.Builtin)
	whenWithArgs, _ := builtin.CallInternal(thread, starlark.Tuple{starlark.String("specific_arg")}, nil)

	// Configure via then_return
	thenReturnFn, _ := whenWithArgs.(*MockWhen).Attr("then_return")
	thenReturnBuiltin := thenReturnFn.(*starlark.Builtin)
	thenReturnBuiltin.CallInternal(thread, starlark.Tuple{starlark.String("specific result")}, nil)

	// Call with matching args
	result, _ := wrapper.CallInternal(thread, starlark.Tuple{starlark.String("specific_arg")}, nil)
	if result != starlark.String("specific result") {
		t.Errorf("Expected 'specific result', got %v", result)
	}

	// Call with different args (should fall through to original)
	result, _ = wrapper.CallInternal(thread, starlark.Tuple{starlark.String("other_arg")}, nil)
	if result != starlark.String("original") {
		t.Errorf("Expected 'original' for non-matching args, got %v", result)
	}
}

func TestNewMockFixture(t *testing.T) {
	mm := NewMockManager()
	fixture := NewMockFixture(mm)

	if fixture == nil {
		t.Fatal("NewMockFixture returned nil")
	}

	if fixture.Name != "mock" {
		t.Errorf("Expected name 'mock', got %q", fixture.Name)
	}

	// Check all expected members exist
	expectedMembers := []string{"wrap", "when", "was_called", "call_count", "calls", "reset"}
	for _, name := range expectedMembers {
		if _, ok := fixture.Members[name]; !ok {
			t.Errorf("Missing expected member %q", name)
		}
	}
}
