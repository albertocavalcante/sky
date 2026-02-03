package resolver

import (
	"testing"
)

func TestResolution_OK(t *testing.T) {
	tests := []struct {
		name       string
		resolution Resolution
		want       bool
	}{
		{
			name: "success with candidates",
			resolution: Resolution{
				ModuleID:   "//foo:bar.bzl",
				Candidates: []string{"/workspace/foo/bar.bzl"},
			},
			want: true,
		},
		{
			name: "success with multiple candidates",
			resolution: Resolution{
				ModuleID:   "//foo:bar.bzl",
				Candidates: []string{"/workspace/foo/bar.bzl", "/external/foo/bar.bzl"},
			},
			want: true,
		},
		{
			name: "failure with error",
			resolution: Resolution{
				Error: ErrModuleNotFound,
			},
			want: false,
		},
		{
			name: "failure with empty candidates",
			resolution: Resolution{
				ModuleID:   "//foo:bar.bzl",
				Candidates: []string{},
			},
			want: false,
		},
		{
			name: "failure with nil candidates",
			resolution: Resolution{
				ModuleID: "//foo:bar.bzl",
			},
			want: false,
		},
		{
			name: "failure with both error and candidates",
			resolution: Resolution{
				ModuleID:   "//foo:bar.bzl",
				Candidates: []string{"/workspace/foo/bar.bzl"},
				Error:      ErrInvalidLoadString,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.resolution.OK(); got != tt.want {
				t.Errorf("Resolution.OK() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolverFunc_ResolveLoad(t *testing.T) {
	tests := []struct {
		name       string
		resolver   ResolverFunc
		fromFile   string
		loadString string
		wantOK     bool
		wantErr    error
	}{
		{
			name: "with ResolveFn returning success",
			resolver: ResolverFunc{
				ResolveFn: func(fromFile, loadString string) Resolution {
					return Resolution{
						ModuleID:   ModuleID(loadString),
						Candidates: []string{"/workspace/foo/bar.bzl"},
					}
				},
			},
			fromFile:   "/workspace/BUILD",
			loadString: "//foo:bar.bzl",
			wantOK:     true,
			wantErr:    nil,
		},
		{
			name: "with ResolveFn returning error",
			resolver: ResolverFunc{
				ResolveFn: func(fromFile, loadString string) Resolution {
					return Resolution{
						Error: ErrModuleNotFound,
					}
				},
			},
			fromFile:   "/workspace/BUILD",
			loadString: "//missing:lib.bzl",
			wantOK:     false,
			wantErr:    ErrModuleNotFound,
		},
		{
			name:       "without ResolveFn",
			resolver:   ResolverFunc{},
			fromFile:   "/workspace/BUILD",
			loadString: "//foo:bar.bzl",
			wantOK:     false,
			wantErr:    ErrNoResolver,
		},
		{
			name: "with nil ResolveFn explicitly",
			resolver: ResolverFunc{
				ResolveFn:         nil,
				WorkspaceRootPath: "/workspace",
			},
			fromFile:   "/workspace/BUILD",
			loadString: "//foo:bar.bzl",
			wantOK:     false,
			wantErr:    ErrNoResolver,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.resolver.ResolveLoad(tt.fromFile, tt.loadString)
			if got.OK() != tt.wantOK {
				t.Errorf("ResolverFunc.ResolveLoad() OK = %v, want %v", got.OK(), tt.wantOK)
			}
			if got.Error != tt.wantErr {
				t.Errorf("ResolverFunc.ResolveLoad() Error = %v, want %v", got.Error, tt.wantErr)
			}
		})
	}
}

func TestResolverFunc_WorkspaceRoot(t *testing.T) {
	tests := []struct {
		name     string
		resolver ResolverFunc
		want     string
	}{
		{
			name: "with workspace root set",
			resolver: ResolverFunc{
				WorkspaceRootPath: "/home/user/workspace",
			},
			want: "/home/user/workspace",
		},
		{
			name:     "without workspace root",
			resolver: ResolverFunc{},
			want:     "",
		},
		{
			name: "with empty workspace root",
			resolver: ResolverFunc{
				WorkspaceRootPath: "",
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.resolver.WorkspaceRoot(); got != tt.want {
				t.Errorf("ResolverFunc.WorkspaceRoot() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantMsg string
	}{
		{
			name:    "ErrNoResolver",
			err:     ErrNoResolver,
			wantMsg: "no resolver configured",
		},
		{
			name:    "ErrModuleNotFound",
			err:     ErrModuleNotFound,
			wantMsg: "module not found",
		},
		{
			name:    "ErrInvalidLoadString",
			err:     ErrInvalidLoadString,
			wantMsg: "invalid load string",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("error.Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestResolverFunc_ImplementsLoadResolver(t *testing.T) {
	// Compile-time check that ResolverFunc implements LoadResolver
	var _ LoadResolver = ResolverFunc{}
	var _ LoadResolver = &ResolverFunc{}
}
