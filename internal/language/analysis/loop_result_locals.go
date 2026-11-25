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

func (a *Analyzer) validateLocalLoopResultBinding(binding *syntax.BindingStmt, declared types.Type, scope *Scope) types.Type {
	element := a.loopResultLocalValues[binding.Value]
	if element == nil {
		return nil
	}
	mutableAllowed := !binding.Mutable
	if binding.Mutable && binding.TypeExpr != nil && a.loopResultLocalPlan != nil &&
		a.loopResultLocalPlan.MutableBindings[binding] {
		if name, okay := binding.Value.(*syntax.IdentExpr); okay {
			rhs, found := scope.Lookup(name.Name)
			mutableAllowed = found && !rhs.Mutable && rhs.loopResultElement != nil &&
				element.Equals(rhs.loopResultElement)
		}
	}
	if !mutableAllowed || declared == nil || !types.NewList(element).Equals(declared) {
		a.error(CodeTargetValueShape, binding.Span, "local loop-result cells require an exact List binding; mutable aliases require explicit type and source-owned enrollment")
	}
	return element
}

// Cell metadata is preserved only across the exact admitted whole assignment.
// Ordinary assignability does not prove a private representation's identity.
func (a *Analyzer) validateLocalLoopResultAssignment(assignment *syntax.AssignStmt, target *Symbol, scope *Scope) {
	element := a.loopResultLocalValues[assignment.Value]
	if target.loopResultElement == nil && element == nil {
		return
	}
	var enrolled *syntax.BindingStmt
	if a.loopResultLocalPlan != nil {
		enrolled = a.loopResultLocalPlan.Assignments[assignment]
	}
	name, identifier := assignment.Value.(*syntax.IdentExpr)
	var rhs *Symbol
	if identifier {
		rhs, _ = scope.Lookup(name.Name)
	}
	if enrolled == nil || !enrolled.Mutable || !a.loopResultLocalPlan.MutableBindings[enrolled] ||
		!a.loopResultLocalPlan.AllowedValues[assignment.Value] ||
		enrolled.Name != assignment.Name || enrolled.TypeExpr == nil ||
		assignment.Op != syntax.TokenAssign || !target.Mutable ||
		rhs == nil || rhs.Mutable || rhs.loopResultElement == nil ||
		target.loopResultElement == nil || element == nil ||
		!target.loopResultElement.Equals(element) || !rhs.loopResultElement.Equals(element) ||
		target.Type == nil || !types.NewList(element).Equals(target.Type) ||
		rhs.Type == nil || !target.Type.Equals(rhs.Type) ||
		a.exprTypes[assignment.Value] == nil || !target.Type.Equals(a.exprTypes[assignment.Value]) {
		a.error(CodeTargetValueShape, assignment.Span, "local loop-result assignment requires its enrolled exact List alias and immutable source-owned identifier")
	}
}
