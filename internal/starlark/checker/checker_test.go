package checker

import (
	"strings"
	"testing"
)

func TestChecker_UndefinedName(t *testing.T) {
	src := `
def foo():
    return bar  # bar is not defined
`
	c := New(DefaultOptions())
	diags, err := c.CheckFile("test.star", []byte(src))
	if err != nil {
		t.Fatalf("CheckFile failed: %v", err)
	}

	if len(diags) == 0 {
		t.Fatal("expected at least one diagnostic for undefined 'bar'")
	}

	found := false
	for _, d := range diags {
		if d.Code == "undefined" && strings.Contains(d.Message, "bar") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected diagnostic about undefined 'bar', got: %v", diags)
	}
}

func TestChecker_UnusedVariable(t *testing.T) {
	src := `
def foo():
    x = 1  # x is never used
    return 42
`
	c := New(DefaultOptions())
	diags, err := c.CheckFile("test.star", []byte(src))
	if err != nil {
		t.Fatalf("CheckFile failed: %v", err)
	}

	found := false
	for _, d := range diags {
		if d.Code == "unused" && strings.Contains(d.Message, "x") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected diagnostic about unused 'x', got: %v", diags)
	}
}

func TestChecker_UnderscoreIgnored(t *testing.T) {
	src := `
def foo():
    _ = 1       # _ is conventionally unused
    _unused = 2  # _prefixed is conventionally unused
    return 42
`
	c := New(DefaultOptions())
	diags, err := c.CheckFile("test.star", []byte(src))
	if err != nil {
		t.Fatalf("CheckFile failed: %v", err)
	}

	for _, d := range diags {
		if d.Code == "unused" {
			t.Errorf("unexpected unused diagnostic: %v", d)
		}
	}
}

func TestChecker_ValidCode(t *testing.T) {
	src := `
def greet(name):
    return "Hello, " + name

message = greet("World")
print(message)
`
	c := New(DefaultOptions())
	diags, err := c.CheckFile("test.star", []byte(src))
	if err != nil {
		t.Fatalf("CheckFile failed: %v", err)
	}

	// Filter out any non-error diagnostics for this test
	var errors []Diagnostic
	for _, d := range diags {
		if d.Severity == SeverityError {
			errors = append(errors, d)
		}
	}

	if len(errors) > 0 {
		t.Errorf("expected no errors for valid code, got: %v", errors)
	}
}

func TestChecker_ParseError(t *testing.T) {
	src := `
def foo(
    # missing closing paren and body
`
	c := New(DefaultOptions())
	diags, err := c.CheckFile("test.star", []byte(src))
	if err != nil {
		t.Fatalf("CheckFile failed: %v", err)
	}

	if len(diags) == 0 {
		t.Fatal("expected at least one diagnostic for parse error")
	}

	found := false
	for _, d := range diags {
		if d.Code == "parse-error" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected parse-error diagnostic, got: %v", diags)
	}
}

func TestChecker_Predeclared(t *testing.T) {
	src := `
# Use a predeclared name that wouldn't normally exist
result = custom_builtin(42)
`
	opts := DefaultOptions()
	opts.Predeclared["custom_builtin"] = true

	c := New(opts)
	diags, err := c.CheckFile("test.star", []byte(src))
	if err != nil {
		t.Fatalf("CheckFile failed: %v", err)
	}

	// Should not report custom_builtin as undefined
	for _, d := range diags {
		if d.Code == "undefined" && strings.Contains(d.Message, "custom_builtin") {
			t.Errorf("custom_builtin should be predeclared, but got: %v", d)
		}
	}
}

func TestChecker_NestedScopes(t *testing.T) {
	src := `
def outer():
    x = 1
    def inner():
        return x  # captures x from outer scope
    return inner()
`
	c := New(DefaultOptions())
	diags, err := c.CheckFile("test.star", []byte(src))
	if err != nil {
		t.Fatalf("CheckFile failed: %v", err)
	}

	// Should not report x as undefined (it's captured from outer scope)
	for _, d := range diags {
		if d.Code == "undefined" && strings.Contains(d.Message, "x") {
			t.Errorf("x should be captured from outer scope, but got: %v", d)
		}
	}
}

func TestChecker_LoadStatement(t *testing.T) {
	// Load statements create bindings at global scope
	src := `
load("module.star", "foo", bar = "baz")
result = foo() + bar
`
	c := New(DefaultOptions())
	diags, err := c.CheckFile("test.star", []byte(src))
	if err != nil {
		t.Fatalf("CheckFile failed: %v", err)
	}

	// Should not report foo or bar as undefined
	for _, d := range diags {
		if d.Code == "undefined" {
			if strings.Contains(d.Message, "foo") || strings.Contains(d.Message, "bar") {
				t.Errorf("loaded names should be defined, but got: %v", d)
			}
		}
	}
}

func TestResult_Counts(t *testing.T) {
	r := Result{
		Diagnostics: []Diagnostic{
			{Severity: SeverityError, Code: "e1"},
			{Severity: SeverityWarning, Code: "w1"},
			{Severity: SeverityError, Code: "e2"},
			{Severity: SeverityInfo, Code: "i1"},
		},
	}

	if !r.HasErrors() {
		t.Error("HasErrors() should return true")
	}
	if got := r.ErrorCount(); got != 2 {
		t.Errorf("ErrorCount() = %d, want 2", got)
	}
	if got := r.WarningCount(); got != 1 {
		t.Errorf("WarningCount() = %d, want 1", got)
	}
}
