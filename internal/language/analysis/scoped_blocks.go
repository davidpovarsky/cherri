package analysis

import (
	"strings"

	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func (a *Analyzer) analyzeScopedBlockCall(call *syntax.CallExpr, scope *Scope) types.Type {
	name, simple := call.Callee.(*syntax.IdentExpr)
	if !simple {
		a.error("E_BLOCK_UNSUPPORTED", call.Span, "scoped blocks require a declared compiler block construct")
		return types.Unknown
	}
	contract, found := a.registry.LookupAction(name.Name)
	if !found || !contract.ScopedBlock || a.userFunctions[name.Name] {
		a.error("E_BLOCK_UNSUPPORTED", call.Span, "this call does not accept a scoped block; use for/yield to collect Repeat Results")
		return types.Unknown
	}
	if name.Name != "measure" {
		a.error("E_BLOCK_UNSUPPORTED", call.Span, "no lowering contract exists for this scoped block construct")
		return types.Unknown
	}

	var label syntax.Expression = call.PrimaryArg
	for _, argument := range call.NamedArgs {
		if argument.Label != "label" || label != nil {
			a.error(CodeUnknownArgument, argument.Span, "measure accepts exactly one literal label")
		}
		label = argument.Value
	}
	if label == nil {
		a.error(CodeMissingArgument, call.Span, "measure requires a non-empty literal label")
	} else {
		a.analyzeExpression(label, scope)
		literal, ok := label.(*syntax.LiteralExpr)
		if !ok || (literal.Type != syntax.TokenString && literal.Type != syntax.TokenRawString) || strings.TrimSpace(literal.Value) == "" {
			a.error(CodeLiteralRequired, label.NodeSpan(), "measure requires a non-empty literal label; dynamic labels are not evaluated")
		}
	}

	if call.Block == nil || call.Block.Body == nil {
		a.error("E_BLOCK_REQUIRED", call.Span, "measure requires a scoped body")
		return types.Unknown
	}
	body := call.Block.Body
	blockScope := NewScope(scope)
	previousValueBlock, previousYields := a.inValueBlk, a.yieldedTypes
	a.inValueBlk, a.yieldedTypes = true, nil
	defer func() { a.inValueBlk, a.yieldedTypes = previousValueBlock, previousYields }()
	result := syntax.BlockResultExpression(body)
	for index, statement := range body.Statements {
		if index == len(body.Statements)-1 && result != nil {
			continue
		}
		if _, early := statement.(*syntax.YieldStmt); early {
			a.error("E_BLOCK_EARLY_YIELD", statement.NodeSpan(), "yield does not exit a scoped block; put its result in the final statement")
		}
		a.analyzeStatement(statement, blockScope)
	}
	if result == nil {
		a.error("E_BLOCK_VALUE_REQUIRED", body.Span, "measure currently requires a final expression or final yield; empty/void blocks are unsupported")
		return types.Void
	}
	resultType := a.analyzeExpression(result, blockScope)
	resultType = a.scopedControlResultType(result, resultType)
	a.exprTypes[result] = resultType
	if resultType == nil || resultType.Kind() == types.KindVoid {
		a.error("E_BLOCK_VALUE_REQUIRED", result.NodeSpan(), "measure requires a value-producing final expression; a void action cannot provide a result")
		return types.Void
	}
	return resultType
}

// Existing control expressions expose native results. Inside a typed scoped
// block require an unambiguous final yield on each path, and retain all result
// types rather than treating the first branch's type as proof of every branch.
func (a *Analyzer) scopedControlResultType(expr syntax.Expression, fallback types.Type) types.Type {
	switch control := expr.(type) {
	case *syntax.IfStmt:
		thenType := a.scopedBranchResultType(control.ThenBlock)
		var elseType types.Type
		switch alternate := control.ElseBlock.(type) {
		case *syntax.BlockStmt:
			elseType = a.scopedBranchResultType(alternate)
		case *syntax.IfStmt:
			elseType = a.scopedControlResultType(alternate, types.Unknown)
		default:
			a.error("E_BLOCK_CONTROL_RESULT", control.Span, "a measured if result requires both branches and a final yield on each path")
			elseType = types.Unknown
		}
		return types.NewUnion(thenType, elseType)
	case *syntax.ForStmt:
		return types.NewList(a.scopedBranchResultType(control.BodyBlock))
	case *syntax.RepeatStmt:
		return types.NewList(a.scopedBranchResultType(control.BodyBlock))
	case *syntax.MenuStmt:
		var results []types.Type
		for _, item := range control.Cases {
			results = append(results, a.scopedBranchResultType(item.BodyBlock))
		}
		if len(results) == 0 {
			a.error("E_BLOCK_CONTROL_RESULT", control.Span, "a measured menu result requires value-producing cases")
			return types.Unknown
		}
		return types.NewUnion(results...)
	}
	return fallback
}

func (a *Analyzer) scopedBranchResultType(body *syntax.BlockStmt) types.Type {
	result := syntax.BlockResultExpression(body)
	if body == nil || result == nil {
		if body != nil {
			a.error("E_BLOCK_CONTROL_RESULT", body.Span, "each measured control branch must end in yield or a value-producing control expression")
		}
		return types.Unknown
	}
	last := body.Statements[len(body.Statements)-1]
	if _, explicit := last.(*syntax.YieldStmt); explicit {
		if inferred, ok := a.exprTypes[result]; ok {
			return inferred
		}
		return types.Unknown
	}
	switch result.(type) {
	case *syntax.IfStmt, *syntax.ForStmt, *syntax.RepeatStmt, *syntax.MenuStmt:
		return a.scopedControlResultType(result, types.Unknown)
	default:
		a.error("E_BLOCK_CONTROL_RESULT", last.NodeSpan(), "use a final yield for a measured control branch result; an implicit last action is not a typed return contract")
		return types.Unknown
	}
}
