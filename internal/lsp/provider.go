// Package lsp provides LSP server functionality for Starlark files.
package lsp

import (
	"github.com/albertocavalcante/sky/internal/starlark/builtins"
	"github.com/albertocavalcante/sky/internal/starlark/builtins/loader"
	"github.com/albertocavalcante/sky/internal/starlark/classifier"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
	"go.lsp.dev/protocol"
)

// NewDefaultProvider creates a default builtins provider that chains
// proto-based data (for Bazel) and JSON-based data (for core Starlark).
// This is used by NewServer to provide builtins for completion and hover.
func NewDefaultProvider() builtins.Provider {
	// ProtoProvider has Bazel builtins extracted from bazelbuild/starlark
	proto := loader.NewProtoProvider()

	// JSONProvider has core Starlark builtins
	json := loader.NewJSONProvider()

	// Chain providers: proto first (more specific), JSON second (fallback)
	return builtins.NewChainProvider(proto, json)
}

// getDialectAndKind determines the dialect and file kind based on the document URI.
// Uses the classifier to determine file type from the path.
func (s *Server) getDialectAndKind(uri protocol.DocumentURI) (string, filekind.Kind) {
	path := uriToPath(uri)

	// Use the default classifier to determine dialect and kind from path
	cls := classifier.NewDefaultClassifier()
	classification, err := cls.Classify(path)
	if err != nil {
		// Fallback to generic starlark if classification fails
		return "starlark", filekind.KindUnknown
	}

	return classification.Dialect, classification.FileKind
}
