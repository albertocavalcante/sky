package lsp

import (
	"testing"

	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

func TestGetDialectAndKind_Star(t *testing.T) {
	server := NewServer(nil)

	tests := []struct {
		uri      string
		wantDial string
		wantKind string
	}{
		{"file:///test.star", "starlark", "starlark"},
		{"file:///workspace/BUILD", "bazel", "BUILD"},
		{"file:///workspace/BUILD.bazel", "bazel", "BUILD"},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			dialect, kind := server.getDialectAndKind(string(tt.uri))
			t.Logf("Uri: %s -> dialect=%s, kind=%s", tt.uri, dialect, kind)
			if dialect != tt.wantDial {
				t.Errorf("dialect = %q, want %q", dialect, tt.wantDial)
			}
		})
	}
}

// TestNewServer_InitializesDefaultProvider verifies that NewServer
// initializes a default builtins provider automatically.
func TestNewServer_InitializesDefaultProvider(t *testing.T) {
	server := NewServer(nil)
	if server == nil {
		t.Fatal("NewServer returned nil")
	}

	// The server should have a builtins provider
	if server.builtins == nil {
		t.Fatal("Server.builtins should not be nil - NewServer should create a default provider")
	}
}

func TestNewDefaultProvider_ReturnsBuiltins(t *testing.T) {
	provider := NewDefaultProvider()
	if provider == nil {
		t.Fatal("NewDefaultProvider returned nil")
	}

	// Try to get starlark builtins (use filekind.Kind)
	builtins, err := provider.Builtins("starlark", filekind.KindStarlark)
	if err != nil {
		t.Fatalf("Builtins error: %v", err)
	}

	t.Logf("Got %d functions, %d types, %d globals",
		len(builtins.Functions), len(builtins.Types), len(builtins.Globals))

	if len(builtins.Functions) == 0 {
		t.Error("Expected functions from provider")
	}

	// Check for print
	foundPrint := false
	for _, fn := range builtins.Functions {
		if fn.Name == "print" {
			foundPrint = true
			break
		}
	}
	if !foundPrint {
		t.Error("Expected print function in builtins")
	}
}
