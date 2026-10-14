package lsp

import (
	"context"
	"log"

	"github.com/albertocavalcante/sky/internal/protocol"
	"github.com/albertocavalcante/sky/internal/starlark/checker"
	"github.com/albertocavalcante/sky/internal/starlark/linter"
)

// publishDiagnostics runs linter and checker on a document and publishes results.
func (s *Server) publishDiagnostics(ctx context.Context, uri string, content string) {
	// Guard against nil connection (e.g., in tests)
	conn := s.getConn()
	if conn == nil {
		return
	}

	path := uriToPath(uri)
	var diagnostics []protocol.Diagnostic

	// Run linter (only when the buffer matches what is on disk; see lintBuffer)
	if findings, err := s.lintBuffer(path, content); err == nil {
		for _, f := range findings {
			diagnostics = append(diagnostics, lintFindingToDiagnosticIn(content, f))
		}
	} else {
		log.Printf("linter error: %v", err)
	}

	// Run semantic checker (uses content from memory)
	if checkerDiags, err := s.checker.CheckFile(path, []byte(content)); err == nil {
		for _, d := range checkerDiags {
			diagnostics = append(diagnostics, checkerDiagnosticToLSPIn(content, d))
		}
	} else {
		log.Printf("checker error: %v", err)
	}

	// Publish diagnostics to client
	if diagnostics == nil {
		diagnostics = []protocol.Diagnostic{}
	}
	if err := conn.Notify(ctx, "textDocument/publishDiagnostics", protocol.PublishDiagnosticsParams{
		Uri:         uri,
		Diagnostics: diagnostics,
	}); err != nil {
		log.Printf("failed to publish diagnostics: %v", err)
	}

	log.Printf("published %d diagnostics for %s", len(diagnostics), path)
}

// lintBuffer lints the in-memory document content. Findings always describe
// exactly the given bytes, never the on-disk file.
func (s *Server) lintBuffer(path, content string) ([]linter.Finding, error) {
	return s.lintDriver.RunContent(path, []byte(content))
}

// lintFindingToDiagnostic converts a linter finding to an LSP diagnostic.
// Columns are treated as already being LSP columns.
func lintFindingToDiagnostic(f linter.Finding) protocol.Diagnostic {
	return lintFindingToDiagnosticIn("", f)
}

// lintFindingToDiagnosticIn converts a linter finding (1-based line and
// rune column) to an LSP diagnostic with UTF-16 columns computed against content.
func lintFindingToDiagnosticIn(content string, f linter.Finding) protocol.Diagnostic {
	col := func(line, c int) uint32 {
		if c <= 0 {
			return 0
		}
		text, _ := lineText(content, line-1)
		return uint32(runeColToUTF16(text, c-1))
	}
	// Convert 1-based to 0-based positions
	startLine := uint32(0)
	if f.Line > 0 {
		startLine = uint32(f.Line - 1)
	}
	startChar := col(f.Line, f.Column)
	endLine := startLine
	if f.EndLine > 0 {
		endLine = uint32(f.EndLine - 1)
	}
	endChar := startChar + 1 // Default to single character
	if f.EndColumn > 0 {
		endLine2 := f.EndLine
		if endLine2 <= 0 {
			endLine2 = f.Line
		}
		endChar = col(endLine2, f.EndColumn)
	}

	return protocol.Diagnostic{
		Range: protocol.Range{
			Start: protocol.Position{Line: startLine, Character: startChar},
			End:   protocol.Position{Line: endLine, Character: endChar},
		},
		Severity: lintSeverityToLSP(f.Severity),
		Code:     protocol.Or_int32_string{Value: f.Rule},
		Source:   "skylint",
		Message:  f.Message,
	}
}

// checkerDiagnosticToLSP converts a checker diagnostic to an LSP diagnostic.
// Columns are treated as already being LSP columns.
func checkerDiagnosticToLSP(d checker.Diagnostic) protocol.Diagnostic {
	return checkerDiagnosticToLSPIn("", d)
}

// checkerDiagnosticToLSPIn converts a checker diagnostic (1-based line and
// rune column, as defined by syntax.Position) to an LSP diagnostic with UTF-16 columns computed against content.
func checkerDiagnosticToLSPIn(content string, d checker.Diagnostic) protocol.Diagnostic {
	col := func(line, c int) uint32 {
		if c <= 0 {
			return 0
		}
		text, ok := lineText(content, line-1)
		if !ok {
			return uint32(c - 1)
		}
		return uint32(runeColToUTF16(text, c-1))
	}
	// Convert 1-based to 0-based positions
	startLine := uint32(0)
	if d.Pos.Line > 0 {
		startLine = uint32(d.Pos.Line - 1)
	}
	startChar := col(int(d.Pos.Line), int(d.Pos.Col))
	endLine := startLine
	endChar := startChar + 1 // Default to single character
	if d.End.Line > 0 {
		endLine = uint32(d.End.Line - 1)
		if d.End.Col > 0 {
			endChar = col(int(d.End.Line), int(d.End.Col))
		}
	}

	return protocol.Diagnostic{
		Range: protocol.Range{
			Start: protocol.Position{Line: startLine, Character: startChar},
			End:   protocol.Position{Line: endLine, Character: endChar},
		},
		Severity: checkerSeverityToLSP(d.Severity),
		Code:     protocol.Or_int32_string{Value: d.Code},
		Source:   "skycheck",
		Message:  d.Message,
	}
}

// lintSeverityToLSP converts linter severity to LSP severity.
// Linter: Error=0, Warning=1, Info=2, Hint=3
// LSP: Error=1, Warning=2, Information=3, Hint=4
func lintSeverityToLSP(s linter.Severity) protocol.DiagnosticSeverity {
	switch s {
	case linter.SeverityError:
		return protocol.DiagnosticSeverityError
	case linter.SeverityWarning:
		return protocol.DiagnosticSeverityWarning
	case linter.SeverityInfo:
		return protocol.DiagnosticSeverityInformation
	case linter.SeverityHint:
		return protocol.DiagnosticSeverityHint
	default:
		return protocol.DiagnosticSeverityWarning
	}
}

// checkerSeverityToLSP converts checker severity to LSP severity.
func checkerSeverityToLSP(s checker.Severity) protocol.DiagnosticSeverity {
	switch s {
	case checker.SeverityError:
		return protocol.DiagnosticSeverityError
	case checker.SeverityWarning:
		return protocol.DiagnosticSeverityWarning
	case checker.SeverityInfo:
		return protocol.DiagnosticSeverityInformation
	default:
		return protocol.DiagnosticSeverityWarning
	}
}
