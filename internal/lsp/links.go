package lsp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/bazelbuild/buildtools/build"
	"go.lsp.dev/protocol"
)

// handleDocumentLink returns document links for load() statements.
func (s *Server) handleDocumentLink(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.DocumentLinkParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	s.mu.RLock()
	doc, ok := s.documents[p.TextDocument.URI]
	s.mu.RUnlock()
	if !ok {
		return []protocol.DocumentLink{}, nil
	}

	path := uriToPath(p.TextDocument.URI)
	file, err := build.ParseDefault(path, []byte(doc.Content))
	if err != nil {
		return []protocol.DocumentLink{}, nil
	}

	var links []protocol.DocumentLink
	for _, stmt := range file.Stmt {
		load, ok := stmt.(*build.LoadStmt)
		if !ok {
			continue
		}

		// Resolve the load path
		targetPath := resolveLoadPath(load.Module.Value, path, string(s.rootURI))
		if targetPath == "" {
			continue
		}

		// Get the range of the module string
		start, end := load.Module.Span()
		targetURI := protocol.DocumentURI("file://" + targetPath)

		links = append(links, protocol.DocumentLink{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      uint32(start.Line - 1),
					Character: uint32(start.LineRune - 1),
				},
				End: protocol.Position{
					Line:      uint32(end.Line - 1),
					Character: uint32(end.LineRune - 1),
				},
			},
			Target: targetURI,
		})
	}

	return links, nil
}

// resolveLoadPath resolves a Starlark load path to an absolute file path.
// Returns empty string if the path cannot be resolved (e.g., external repos).
func resolveLoadPath(module, fromPath, workspaceRoot string) string {
	// External repo - cannot resolve locally
	if strings.HasPrefix(module, "@") {
		return ""
	}

	// Strip file:// prefix from workspace root
	workspaceRoot = strings.TrimPrefix(workspaceRoot, "file://")
	if workspaceRoot == "" {
		workspaceRoot = filepath.Dir(fromPath)
	}

	// Workspace-relative: //pkg:file.bzl or //pkg/file.bzl
	if strings.HasPrefix(module, "//") {
		relPath := strings.TrimPrefix(module, "//")
		// Handle //pkg:file.bzl format
		if idx := strings.Index(relPath, ":"); idx != -1 {
			pkg := relPath[:idx]
			file := relPath[idx+1:]
			return filepath.Join(workspaceRoot, pkg, file)
		}
		// Handle //pkg/file.bzl format
		return filepath.Join(workspaceRoot, relPath)
	}

	// Package-relative: :file.bzl
	if strings.HasPrefix(module, ":") {
		file := strings.TrimPrefix(module, ":")
		return filepath.Join(filepath.Dir(fromPath), file)
	}

	// Bare filename: file.bzl (same directory)
	if !strings.Contains(module, "/") {
		return filepath.Join(filepath.Dir(fromPath), module)
	}

	// Relative path
	return filepath.Join(filepath.Dir(fromPath), module)
}
