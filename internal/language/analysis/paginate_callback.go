package analysis

import (
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// Analyze the same authored call arguments first, then a distinct lexical body
// which can run zero or more times. The page parameter is an immutable native
// Dictionary; it is not an escaping closure or a typed item selector.
func (a *Analyzer) analyzePaginateBlock(call *syntax.CallExpr, scope *Scope, contract *schema.ActionSchema) types.Type {
	if _, shadowed := scope.Lookup("paginate"); shadowed {
		a.error("E_PAGINATE_CONTRACT", call.Span, "a lexical declaration cannot acquire the compiler page callback")
		return types.Unknown
	}
	result := a.analyzeCallArguments(call, scope)
	parameter := schema.PaginatePageParameter(call)
	if parameter == nil {
		a.error("E_PAGINATE_PARAMETER", call.Span, "paginate requires exactly one immutable page parameter: { page in ... }")
		return result
	}
	if schema.LoopResultScopedPaginateBody(call, contract) == nil {
		a.error("E_BLOCK_UNSUPPORTED", call.Span, "paginate page blocks require the canonical compiler construct")
		return types.Unknown
	}
	body := call.Block.Body
	pageScope := NewScope(scope)
	pageScope.Define(&Symbol{
		Name: parameter.Name, Type: types.NewMap(types.Text, types.AnyContent),
		Mutable: false, Initialized: true, Span: parameter.Span,
		nativeReferenceTarget: true,
	})
	entry := captureBindingFlow(scope)
	invalidateLoopRefinements(body, pageScope)
	a.loopDepth++
	bodyType := a.analyzeScopedValueBodyInScope(body, pageScope)
	a.loopDepth--
	mergeBindingFlow(entry, entry, captureBindingFlow(scope))
	a.rejectPaginateControlExit(body)
	if bodyType == nil || bodyType.Kind() != types.KindVoid {
		a.error("E_PAGINATE_BODY_VOID", body.Span, "a page callback performs effects and has no yielded or returned item value")
	}
	return result
}

// Only statement-level control exits belong to this callback. A binding's
// Repeat/if expression keeps its own typed yield contract and is not scanned
// as though it were a return from the page callback.
func (a *Analyzer) rejectPaginateControlExit(statement syntax.Statement) {
	switch node := statement.(type) {
	case *syntax.BlockStmt:
		for _, child := range node.Statements {
			a.rejectPaginateControlExit(child)
		}
	case *syntax.ReturnStmt, *syntax.YieldStmt:
		a.error("E_PAGINATE_BODY_VOID", statement.NodeSpan(), "page callbacks cannot yield or return values; typed item selectors are a separate contract")
	case *syntax.IfStmt:
		a.rejectPaginateControlExit(node.ThenBlock)
		if node.ElseBlock != nil {
			a.rejectPaginateControlExit(node.ElseBlock)
		}
	case *syntax.ForStmt:
		a.rejectPaginateControlExit(node.BodyBlock)
	case *syntax.RepeatStmt:
		a.rejectPaginateControlExit(node.BodyBlock)
	case *syntax.MenuStmt:
		for _, item := range node.Cases {
			a.rejectPaginateControlExit(item.BodyBlock)
		}
	case *syntax.ExprStmt:
		switch node.Expr.(type) {
		case *syntax.IfStmt, *syntax.ForStmt, *syntax.RepeatStmt, *syntax.MenuStmt:
			a.rejectPaginateControlExit(node.Expr.(syntax.Statement))
		}
	}
}
