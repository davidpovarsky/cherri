package analysis

import (
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// Native Repeat Results cannot currently preserve one nested List per iteration,
// and it does not carry a compiler-owned presence flag for each Optional element.
// Unknown/AnyContent remains an unverified native-last-action boundary.
func (a *Analyzer) requireNativeLoopResultShape(body *syntax.BlockStmt) {
	// Only the actual closed Items Count consumer owns a cell transport.
	if body != nil && body == a.loopResultCountBody {
		return
	}
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

// analyzeLoopResultYield gives only a registered final cell payload the same
// literal-array context as an authored Dictionary field. The existing codec
// serializes that value inside the compiler-owned cell; earlier/discarded
// yields and ordinary bindings still create ordinary native Lists.
func (a *Analyzer) analyzeLoopResultYield(yield *syntax.YieldStmt, scope *Scope) types.Type {
	if _, literal := yield.Value.(*syntax.ListExpr); literal &&
		loopResultCellFinalYield(a.loopResultCountBody, yield) {
		return a.analyzeDictionaryValue(yield.Value, scope)
	}
	return a.analyzeExpression(yield.Value, scope)
}

// Match the lowerer's normal final-yield registration. This does not admit
// every yield in a Count body: a later statement can replace an earlier yield,
// and a return-only path never contributes a cell.
func loopResultCellFinalYield(statement syntax.Statement, target *syntax.YieldStmt) bool {
	if statement == nil || !statementMayContinue(statement) {
		return false
	}
	switch node := statement.(type) {
	case *syntax.BlockStmt:
		if node == nil || len(node.Statements) == 0 {
			return false
		}
		return loopResultCellFinalYield(node.Statements[len(node.Statements)-1], target)
	case *syntax.YieldStmt:
		return node == target
	case *syntax.IfStmt:
		return loopResultCellFinalYield(node.ThenBlock, target) ||
			loopResultCellFinalYield(node.ElseBlock, target)
	case *syntax.MenuStmt:
		for _, item := range node.Cases {
			if loopResultCellFinalYield(item.BodyBlock, target) {
				return true
			}
		}
	}
	return false
}
