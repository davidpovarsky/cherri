package analysis

import (
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// Native Repeat Results cannot currently preserve one nested List per iteration,
// and it does not carry a compiler-owned presence flag for each Optional element.
// Unknown/AnyContent remains an unverified native-last-action boundary.
func (a *Analyzer) requireNativeLoopResultShape(body *syntax.BlockStmt) {
	if !statementMayContinue(body) {
		return
	}
	value := a.controlBranchResultType(body)
	if !mayHaveListCardinality(value) && !types.MayBeAbsent(value) {
		return
	}
	span := body.Span
	if result := syntax.BlockResultExpression(body); result != nil {
		span = result.NodeSpan()
	}
	if mayHaveListCardinality(value) {
		a.error(CodeTargetValueShape, span, "List-valued loop results are unsupported: native Repeat Results do not preserve nested List cardinality")
		return
	}
	a.error(CodeTargetValueShape, span, "current value-loop result transport cannot preserve optional absence as one element per iteration")
}
