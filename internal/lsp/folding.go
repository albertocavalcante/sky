package lsp

import (
	"context"
	"encoding/json"

	"github.com/albertocavalcante/sky/internal/protocol"
	"github.com/bazelbuild/buildtools/build"
)

// handleFoldingRange returns folding ranges for the document.
func (s *Server) handleFoldingRange(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.FoldingRangeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	s.mu.RLock()
	doc, ok := s.documents[p.TextDocument.Uri]
	s.mu.RUnlock()
	if !ok {
		return []protocol.FoldingRange{}, nil
	}

	path := uriToPath(p.TextDocument.Uri)
	file, err := build.ParseDefault(path, []byte(doc.Content))
	if err != nil {
		return []protocol.FoldingRange{}, nil
	}

	var ranges []protocol.FoldingRange
	collectFoldingRanges(file.Stmt, &ranges)

	return ranges, nil
}

func collectFoldingRanges(stmts []build.Expr, ranges *[]protocol.FoldingRange) {
	for _, stmt := range stmts {
		collectFoldingRangesFromExpr(stmt, ranges)
	}
}

func collectFoldingRangesFromExpr(expr build.Expr, ranges *[]protocol.FoldingRange) {
	switch e := expr.(type) {
	case *build.DefStmt:
		start, end := e.Span()
		if end.Line > start.Line {
			*ranges = append(*ranges, protocol.FoldingRange{
				StartLine: uint32(start.Line - 1),
				EndLine:   uint32(end.Line - 1),
				Kind:      "region",
			})
		}
		collectFoldingRanges(e.Body, ranges)

	case *build.ForStmt:
		start, end := e.Span()
		if end.Line > start.Line {
			*ranges = append(*ranges, protocol.FoldingRange{
				StartLine: uint32(start.Line - 1),
				EndLine:   uint32(end.Line - 1),
				Kind:      "region",
			})
		}
		collectFoldingRanges(e.Body, ranges)

	case *build.IfStmt:
		start, end := e.Span()
		if end.Line > start.Line {
			*ranges = append(*ranges, protocol.FoldingRange{
				StartLine: uint32(start.Line - 1),
				EndLine:   uint32(end.Line - 1),
				Kind:      "region",
			})
		}
		collectFoldingRanges(e.True, ranges)
		collectFoldingRanges(e.False, ranges)

	case *build.CallExpr:
		start, end := e.Span()
		if end.Line > start.Line {
			*ranges = append(*ranges, protocol.FoldingRange{
				StartLine: uint32(start.Line - 1),
				EndLine:   uint32(end.Line - 1),
				Kind:      "region",
			})
		}

	case *build.ListExpr:
		start, end := e.Span()
		if end.Line > start.Line {
			*ranges = append(*ranges, protocol.FoldingRange{
				StartLine: uint32(start.Line - 1),
				EndLine:   uint32(end.Line - 1),
				Kind:      "region",
			})
		}

	case *build.DictExpr:
		start, end := e.Span()
		if end.Line > start.Line {
			*ranges = append(*ranges, protocol.FoldingRange{
				StartLine: uint32(start.Line - 1),
				EndLine:   uint32(end.Line - 1),
				Kind:      "region",
			})
		}
	}
}
