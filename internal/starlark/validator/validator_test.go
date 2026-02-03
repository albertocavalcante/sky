package validator

import (
	"errors"
	"testing"

	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

func TestSeverity_String(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		want     string
	}{
		{"error", SeverityError, "error"},
		{"warning", SeverityWarning, "warning"},
		{"info", SeverityInfo, "info"},
		{"hint", SeverityHint, "hint"},
		{"unknown", Severity(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.severity.String(); got != tt.want {
				t.Errorf("Severity.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDiagnostic_IsError(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		want     bool
	}{
		{"error is error", SeverityError, true},
		{"warning is not error", SeverityWarning, false},
		{"info is not error", SeverityInfo, false},
		{"hint is not error", SeverityHint, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Diagnostic{Severity: tt.severity}
			if got := d.IsError(); got != tt.want {
				t.Errorf("Diagnostic.IsError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidatorFunc_Name(t *testing.T) {
	vf := ValidatorFunc{NameVal: "test-validator"}
	if got := vf.Name(); got != "test-validator" {
		t.Errorf("ValidatorFunc.Name() = %q, want %q", got, "test-validator")
	}
}

func TestValidatorFunc_SupportedKinds(t *testing.T) {
	tests := []struct {
		name  string
		kinds []filekind.Kind
		want  []filekind.Kind
	}{
		{"nil kinds", nil, nil},
		{"empty kinds", []filekind.Kind{}, []filekind.Kind{}},
		{"single kind", []filekind.Kind{filekind.KindBUILD}, []filekind.Kind{filekind.KindBUILD}},
		{
			"multiple kinds",
			[]filekind.Kind{filekind.KindBUILD, filekind.KindBzl},
			[]filekind.Kind{filekind.KindBUILD, filekind.KindBzl},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vf := ValidatorFunc{Kinds: tt.kinds}
			got := vf.SupportedKinds()
			if len(got) != len(tt.want) {
				t.Errorf("ValidatorFunc.SupportedKinds() len = %d, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ValidatorFunc.SupportedKinds()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestValidatorFunc_Validate(t *testing.T) {
	tests := []struct {
		name       string
		validateFn func(ctx Context) ([]Diagnostic, error)
		wantDiags  int
		wantErr    bool
	}{
		{
			name:       "nil function returns nil",
			validateFn: nil,
			wantDiags:  0,
			wantErr:    false,
		},
		{
			name: "function returns diagnostics",
			validateFn: func(_ Context) ([]Diagnostic, error) {
				return []Diagnostic{
					{Severity: SeverityError, Message: "test error"},
					{Severity: SeverityWarning, Message: "test warning"},
				}, nil
			},
			wantDiags: 2,
			wantErr:   false,
		},
		{
			name: "function returns error",
			validateFn: func(_ Context) ([]Diagnostic, error) {
				return nil, errors.New("validation failed")
			},
			wantDiags: 0,
			wantErr:   true,
		},
		{
			name: "function returns empty slice",
			validateFn: func(_ Context) ([]Diagnostic, error) {
				return []Diagnostic{}, nil
			},
			wantDiags: 0,
			wantErr:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vf := ValidatorFunc{ValidateFn: tt.validateFn}
			got, err := vf.Validate(Context{})
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatorFunc.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != tt.wantDiags {
				t.Errorf("ValidatorFunc.Validate() returned %d diagnostics, want %d", len(got), tt.wantDiags)
			}
		})
	}
}

func TestNewRunner(t *testing.T) {
	v1 := ValidatorFunc{NameVal: "v1"}
	v2 := ValidatorFunc{NameVal: "v2"}

	tests := []struct {
		name       string
		validators []Validator
		wantLen    int
	}{
		{"no validators", nil, 0},
		{"single validator", []Validator{v1}, 1},
		{"multiple validators", []Validator{v1, v2}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewRunner(tt.validators...)
			if len(runner.validators) != tt.wantLen {
				t.Errorf("NewRunner() created runner with %d validators, want %d", len(runner.validators), tt.wantLen)
			}
		})
	}
}

func TestRunner_Run(t *testing.T) {
	tests := []struct {
		name       string
		validators []Validator
		ctx        Context
		wantDiags  int
		wantErr    bool
	}{
		{
			name:       "no validators",
			validators: nil,
			ctx:        Context{FileKind: filekind.KindBUILD},
			wantDiags:  0,
			wantErr:    false,
		},
		{
			name: "single validator returns diagnostics",
			validators: []Validator{
				ValidatorFunc{
					NameVal: "test",
					ValidateFn: func(_ Context) ([]Diagnostic, error) {
						return []Diagnostic{{Severity: SeverityError}}, nil
					},
				},
			},
			ctx:       Context{FileKind: filekind.KindBUILD},
			wantDiags: 1,
			wantErr:   false,
		},
		{
			name: "multiple validators aggregate diagnostics",
			validators: []Validator{
				ValidatorFunc{
					NameVal: "v1",
					ValidateFn: func(_ Context) ([]Diagnostic, error) {
						return []Diagnostic{{Severity: SeverityError}}, nil
					},
				},
				ValidatorFunc{
					NameVal: "v2",
					ValidateFn: func(_ Context) ([]Diagnostic, error) {
						return []Diagnostic{{Severity: SeverityWarning}, {Severity: SeverityInfo}}, nil
					},
				},
			},
			ctx:       Context{FileKind: filekind.KindBUILD},
			wantDiags: 3,
			wantErr:   false,
		},
		{
			name: "validator error stops execution",
			validators: []Validator{
				ValidatorFunc{
					NameVal: "failing",
					ValidateFn: func(_ Context) ([]Diagnostic, error) {
						return nil, errors.New("validator failed")
					},
				},
				ValidatorFunc{
					NameVal: "not-reached",
					ValidateFn: func(_ Context) ([]Diagnostic, error) {
						return []Diagnostic{{Severity: SeverityError}}, nil
					},
				},
			},
			ctx:       Context{FileKind: filekind.KindBUILD},
			wantDiags: 0,
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewRunner(tt.validators...)
			got, err := runner.Run(tt.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Runner.Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != tt.wantDiags {
				t.Errorf("Runner.Run() returned %d diagnostics, want %d", len(got), tt.wantDiags)
			}
		})
	}
}

func TestRunner_Run_FileKindFiltering(t *testing.T) {
	buildOnlyValidator := ValidatorFunc{
		NameVal: "build-only",
		Kinds:   []filekind.Kind{filekind.KindBUILD},
		ValidateFn: func(_ Context) ([]Diagnostic, error) {
			return []Diagnostic{{Severity: SeverityError, Message: "build error"}}, nil
		},
	}

	bzlOnlyValidator := ValidatorFunc{
		NameVal: "bzl-only",
		Kinds:   []filekind.Kind{filekind.KindBzl},
		ValidateFn: func(_ Context) ([]Diagnostic, error) {
			return []Diagnostic{{Severity: SeverityWarning, Message: "bzl warning"}}, nil
		},
	}

	allKindsValidator := ValidatorFunc{
		NameVal: "all-kinds",
		Kinds:   nil, // empty means all kinds
		ValidateFn: func(_ Context) ([]Diagnostic, error) {
			return []Diagnostic{{Severity: SeverityInfo, Message: "info for all"}}, nil
		},
	}

	multiKindValidator := ValidatorFunc{
		NameVal: "multi-kind",
		Kinds:   []filekind.Kind{filekind.KindBUILD, filekind.KindWORKSPACE},
		ValidateFn: func(_ Context) ([]Diagnostic, error) {
			return []Diagnostic{{Severity: SeverityHint, Message: "hint"}}, nil
		},
	}

	tests := []struct {
		name       string
		validators []Validator
		fileKind   filekind.Kind
		wantDiags  int
	}{
		{
			name:       "BUILD file matches BUILD-only validator",
			validators: []Validator{buildOnlyValidator},
			fileKind:   filekind.KindBUILD,
			wantDiags:  1,
		},
		{
			name:       "bzl file does not match BUILD-only validator",
			validators: []Validator{buildOnlyValidator},
			fileKind:   filekind.KindBzl,
			wantDiags:  0,
		},
		{
			name:       "bzl file matches bzl-only validator",
			validators: []Validator{bzlOnlyValidator},
			fileKind:   filekind.KindBzl,
			wantDiags:  1,
		},
		{
			name:       "any file matches all-kinds validator",
			validators: []Validator{allKindsValidator},
			fileKind:   filekind.KindStarlark,
			wantDiags:  1,
		},
		{
			name:       "BUILD file matches multi-kind validator",
			validators: []Validator{multiKindValidator},
			fileKind:   filekind.KindBUILD,
			wantDiags:  1,
		},
		{
			name:       "WORKSPACE file matches multi-kind validator",
			validators: []Validator{multiKindValidator},
			fileKind:   filekind.KindWORKSPACE,
			wantDiags:  1,
		},
		{
			name:       "bzl file does not match multi-kind validator",
			validators: []Validator{multiKindValidator},
			fileKind:   filekind.KindBzl,
			wantDiags:  0,
		},
		{
			name:       "mixed validators filter correctly",
			validators: []Validator{buildOnlyValidator, bzlOnlyValidator, allKindsValidator},
			fileKind:   filekind.KindBUILD,
			wantDiags:  2, // build-only + all-kinds
		},
		{
			name:       "mixed validators filter correctly for bzl",
			validators: []Validator{buildOnlyValidator, bzlOnlyValidator, allKindsValidator},
			fileKind:   filekind.KindBzl,
			wantDiags:  2, // bzl-only + all-kinds
		},
		{
			name:       "no validators match file kind",
			validators: []Validator{buildOnlyValidator, bzlOnlyValidator},
			fileKind:   filekind.KindMODULE,
			wantDiags:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewRunner(tt.validators...)
			ctx := Context{FileKind: tt.fileKind}
			got, err := runner.Run(ctx)
			if err != nil {
				t.Errorf("Runner.Run() unexpected error: %v", err)
				return
			}
			if len(got) != tt.wantDiags {
				t.Errorf("Runner.Run() returned %d diagnostics, want %d", len(got), tt.wantDiags)
			}
		})
	}
}

func TestRunner_Run_ContextPassedToValidator(t *testing.T) {
	var receivedCtx Context
	captureValidator := ValidatorFunc{
		NameVal: "capture",
		ValidateFn: func(ctx Context) ([]Diagnostic, error) {
			receivedCtx = ctx
			return nil, nil
		},
	}

	expectedCtx := Context{
		Dialect:     "bazel",
		FileKind:    filekind.KindBUILD,
		FilePath:    "/path/to/BUILD",
		FileContent: []byte("load(':foo.bzl', 'bar')"),
	}

	runner := NewRunner(captureValidator)
	_, err := runner.Run(expectedCtx)
	if err != nil {
		t.Fatalf("Runner.Run() unexpected error: %v", err)
	}

	if receivedCtx.Dialect != expectedCtx.Dialect {
		t.Errorf("Context.Dialect = %q, want %q", receivedCtx.Dialect, expectedCtx.Dialect)
	}
	if receivedCtx.FileKind != expectedCtx.FileKind {
		t.Errorf("Context.FileKind = %v, want %v", receivedCtx.FileKind, expectedCtx.FileKind)
	}
	if receivedCtx.FilePath != expectedCtx.FilePath {
		t.Errorf("Context.FilePath = %q, want %q", receivedCtx.FilePath, expectedCtx.FilePath)
	}
	if string(receivedCtx.FileContent) != string(expectedCtx.FileContent) {
		t.Errorf("Context.FileContent = %q, want %q", string(receivedCtx.FileContent), string(expectedCtx.FileContent))
	}
}
