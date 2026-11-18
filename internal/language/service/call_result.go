package service

import (
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// inferredCallResultAt uses the actual analyzed call, not a same-name call
// elsewhere in the document or a schema-wide output category. Columns are the
// service's existing 1-based rune positions, matching AST source spans.
func inferredCallResultAt(doc *DocumentState, name string, line, column int) (types.Type, bool) {
	if doc == nil || doc.Analyzer == nil {
		return nil, false
	}
	for expr, result := range doc.Analyzer.ExprTypes() {
		call, ok := expr.(*syntax.CallExpr)
		if !ok || result == nil {
			continue
		}
		callee, ok := call.Callee.(*syntax.IdentExpr)
		if !ok || callee.Name != name {
			continue
		}
		span := callee.NodeSpan()
		if span.Start.Line == line && span.End.Line == line && column >= span.Start.Column && column < span.End.Column {
			return result, true
		}
	}
	return nil, false
}
