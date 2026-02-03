package tester

import (
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"
)

// NewAssertModule creates the built-in assert module.
//
// Available functions:
//   - assert.eq(a, b, msg=None) - Assert a == b
//   - assert.ne(a, b, msg=None) - Assert a != b
//   - assert.true(cond, msg=None) - Assert cond is truthy
//   - assert.false(cond, msg=None) - Assert cond is falsy
//   - assert.contains(container, item, msg=None) - Assert item in container
//   - assert.fails(fn, pattern=None) - Assert fn() raises error matching pattern
func NewAssertModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "assert",
		Members: starlark.StringDict{
			"eq":       starlark.NewBuiltin("assert.eq", assertEq),
			"ne":       starlark.NewBuiltin("assert.ne", assertNe),
			"true":     starlark.NewBuiltin("assert.true", assertTrue),
			"false":    starlark.NewBuiltin("assert.false", assertFalse),
			"contains": starlark.NewBuiltin("assert.contains", assertContains),
			"fails":    starlark.NewBuiltin("assert.fails", assertFails),
			"lt":       starlark.NewBuiltin("assert.lt", assertLt),
			"le":       starlark.NewBuiltin("assert.le", assertLe),
			"gt":       starlark.NewBuiltin("assert.gt", assertGt),
			"ge":       starlark.NewBuiltin("assert.ge", assertGe),
		},
	}
}

// assertEq asserts that two values are equal.
func assertEq(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var a, expected starlark.Value
	var msg starlark.Value = starlark.None
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "a", &a, "b", &expected, "msg?", &msg); err != nil {
		return nil, err
	}

	eq, err := starlark.Equal(a, expected)
	if err != nil {
		return nil, err
	}
	if !eq {
		return nil, assertionError(msg, "expected %s == %s", a, expected)
	}
	return starlark.None, nil
}

// assertNe asserts that two values are not equal.
func assertNe(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var a, unexpected starlark.Value
	var msg starlark.Value = starlark.None
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "a", &a, "b", &unexpected, "msg?", &msg); err != nil {
		return nil, err
	}

	eq, err := starlark.Equal(a, unexpected)
	if err != nil {
		return nil, err
	}
	if eq {
		return nil, assertionError(msg, "expected %s != %s", a, unexpected)
	}
	return starlark.None, nil
}

// assertTrue asserts that a value is truthy.
func assertTrue(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var cond starlark.Value
	var msg starlark.Value = starlark.None
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "cond", &cond, "msg?", &msg); err != nil {
		return nil, err
	}

	if !cond.Truth() {
		return nil, assertionError(msg, "expected %s to be true", cond)
	}
	return starlark.None, nil
}

// assertFalse asserts that a value is falsy.
func assertFalse(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var cond starlark.Value
	var msg starlark.Value = starlark.None
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "cond", &cond, "msg?", &msg); err != nil {
		return nil, err
	}

	if cond.Truth() {
		return nil, assertionError(msg, "expected %s to be false", cond)
	}
	return starlark.None, nil
}

// assertContains asserts that a container contains an item.
func assertContains(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var container, item starlark.Value
	var msg starlark.Value = starlark.None
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "container", &container, "item", &item, "msg?", &msg); err != nil {
		return nil, err
	}

	// Check if container supports "in" operator
	switch c := container.(type) {
	case *starlark.List:
		for i := 0; i < c.Len(); i++ {
			eq, _ := starlark.Equal(c.Index(i), item)
			if eq {
				return starlark.None, nil
			}
		}
	case *starlark.Tuple:
		for i := 0; i < c.Len(); i++ {
			eq, _ := starlark.Equal(c.Index(i), item)
			if eq {
				return starlark.None, nil
			}
		}
	case *starlark.Dict:
		_, found, _ := c.Get(item)
		if found {
			return starlark.None, nil
		}
	case *starlark.Set:
		found, _ := c.Has(item)
		if found {
			return starlark.None, nil
		}
	case starlark.String:
		if s, ok := item.(starlark.String); ok {
			if contains(string(c), string(s)) {
				return starlark.None, nil
			}
		}
	default:
		return nil, fmt.Errorf("assert.contains: unsupported container type %s", container.Type())
	}

	return nil, assertionError(msg, "expected %s to contain %s", container, item)
}

// assertFails asserts that a function raises an error.
func assertFails(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var fn starlark.Callable
	var pattern starlark.String
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "fn", &fn, "pattern?", &pattern); err != nil {
		return nil, err
	}

	_, err := starlark.Call(thread, fn, nil, nil)
	if err == nil {
		return nil, fmt.Errorf("assert.fails: expected function to fail, but it succeeded")
	}

	// If pattern is provided, check that error message matches
	if pattern != "" {
		if !contains(err.Error(), string(pattern)) {
			return nil, fmt.Errorf("assert.fails: error %q does not match pattern %q", err.Error(), pattern)
		}
	}

	return starlark.None, nil
}

// assertLt asserts a < b.
func assertLt(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var a, expected starlark.Value
	var msg starlark.Value = starlark.None
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "a", &a, "b", &expected, "msg?", &msg); err != nil {
		return nil, err
	}

	cmp, err := starlark.Compare(syntax.LT, a, expected)
	if err != nil {
		return nil, err
	}
	if !cmp {
		return nil, assertionError(msg, "expected %s < %s", a, expected)
	}
	return starlark.None, nil
}

// assertLe asserts a <= b.
func assertLe(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var a, expected starlark.Value
	var msg starlark.Value = starlark.None
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "a", &a, "b", &expected, "msg?", &msg); err != nil {
		return nil, err
	}

	cmp, err := starlark.Compare(syntax.LE, a, expected)
	if err != nil {
		return nil, err
	}
	if !cmp {
		return nil, assertionError(msg, "expected %s <= %s", a, expected)
	}
	return starlark.None, nil
}

// assertGt asserts a > b.
func assertGt(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var a, expected starlark.Value
	var msg starlark.Value = starlark.None
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "a", &a, "b", &expected, "msg?", &msg); err != nil {
		return nil, err
	}

	cmp, err := starlark.Compare(syntax.GT, a, expected)
	if err != nil {
		return nil, err
	}
	if !cmp {
		return nil, assertionError(msg, "expected %s > %s", a, expected)
	}
	return starlark.None, nil
}

// assertGe asserts a >= b.
func assertGe(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var a, expected starlark.Value
	var msg starlark.Value = starlark.None
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "a", &a, "b", &expected, "msg?", &msg); err != nil {
		return nil, err
	}

	cmp, err := starlark.Compare(syntax.GE, a, expected)
	if err != nil {
		return nil, err
	}
	if !cmp {
		return nil, assertionError(msg, "expected %s >= %s", a, expected)
	}
	return starlark.None, nil
}

// assertionError creates an assertion error with optional custom message.
func assertionError(customMsg starlark.Value, format string, args ...any) error {
	if customMsg != starlark.None {
		if s, ok := customMsg.(starlark.String); ok && s != "" {
			return fmt.Errorf("assertion failed: %s", string(s))
		}
	}
	return fmt.Errorf("assertion failed: "+format, args...)
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(substr) <= len(s) && (substr == "" || findSubstring(s, substr) >= 0)
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
