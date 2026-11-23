package analysis

import (
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func (a *Analyzer) beginLocalLoopResultBinding(binding *syntax.BindingStmt) func() {
	previous := a.loopResultLocalBody
	if a.loopResultLocalPlan != nil && !binding.Mutable && a.loopResultLocalPlan.BindingValues[binding.Value] {
		if body := schema.LoopResultLocalBody(binding.Value); a.loopResultLocalPlan.Bodies[body] {
			a.loopResultLocalBody = body
		}
	}
	return func() { a.loopResultLocalBody = previous }
}

func (a *Analyzer) localLoopResultShapeSupported(body *syntax.BlockStmt) bool {
	return body != nil && body == a.loopResultLocalBody &&
		a.loopResultLocalPlan != nil && a.loopResultLocalPlan.Bodies[body] &&
		schema.LoopResultLocalPayloadsSupported(body, a.exprTypes)
}

// Transport is derived from the actual checked producer or lexical binding.
// A matching runtime Dictionary marker never enters this source provenance map.
func (a *Analyzer) recordLocalLoopResult(expression syntax.Expression, t types.Type, scope *Scope) {
	if a.loopResultLocalPlan == nil {
		return
	}
	var element types.Type
	switch value := expression.(type) {
	case *syntax.IdentExpr:
		if symbol, found := scope.Lookup(value.Name); found {
			element = symbol.loopResultElement
		}
	case *syntax.RepeatStmt:
		if list, okay := t.(types.ListType); okay && a.localLoopResultShapeSupported(value.BodyBlock) &&
			(mayHaveListCardinality(list.Element) || types.MayBeAbsent(list.Element)) {
			element = list.Element
		}
	case *syntax.ForStmt:
		if list, okay := t.(types.ListType); okay && a.localLoopResultShapeSupported(value.BodyBlock) &&
			(mayHaveListCardinality(list.Element) || types.MayBeAbsent(list.Element)) {
			element = list.Element
		}
	}
	if element == nil {
		return
	}
	if !a.loopResultLocalPlan.AllowedValues[expression] {
		a.error(CodeTargetValueShape, expression.NodeSpan(), "private loop-result cells require an enrolled local binding, Items Count, checked index or for consumer")
		return
	}
	a.loopResultLocalValues[expression] = element
}

func (a *Analyzer) validateLocalLoopResultBinding(binding *syntax.BindingStmt, declared types.Type) types.Type {
	element := a.loopResultLocalValues[binding.Value]
	if element == nil {
		return nil
	}
	if binding.Mutable || declared == nil || !types.NewList(element).Equals(declared) {
		a.error(CodeTargetValueShape, binding.Span, "local loop-result cells require an immutable exact List type; mutable/widened representation joins remain unsupported")
	}
	return element
}
