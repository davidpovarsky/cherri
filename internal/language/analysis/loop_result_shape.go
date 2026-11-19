package analysis

import "github.com/electrikmilk/cherri/internal/language/syntax"

// Native Repeat Results cannot currently preserve one nested List value per
// iteration. Reject only a statically established List-shaped final result;
// Unknown/AnyContent remains an unverified native-last-action boundary.
func (a *Analyzer) requireNativeLoopResultShape(body *syntax.BlockStmt) {
	if !statementMayContinue(body) || !mayHaveListCardinality(a.controlBranchResultType(body)) {
		return
	}
	span := body.Span
	if result := syntax.BlockResultExpression(body); result != nil {
		span = result.NodeSpan()
	}
	a.error(CodeTargetValueShape, span, "List-valued loop results are unsupported: native Repeat Results do not preserve nested List cardinality")
}
