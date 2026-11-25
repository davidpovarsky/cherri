package analysis

import (
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func (a *Analyzer) beginLocalLoopResultValue(value syntax.Expression) func() {
	var previous = a.loopResultLocalBody
	if a.loopResultLocalPlan != nil && a.loopResultLocalPlan.BindingValues[value] {
		if body := schema.LoopResultLocalBody(value); a.loopResultLocalPlan.Bodies[body] {
			a.loopResultLocalBody = body
		}
	}
	return func() { a.loopResultLocalBody = previous }
}

func (a *Analyzer) beginLocalLoopResultBinding(binding *syntax.BindingStmt) func() {
	if a.loopResultLocalPlan == nil || !a.loopResultLocalPlan.BindingValueSourceMatches(binding) {
		return func() {}
	}
	return a.beginLocalLoopResultValue(binding.Value)
}

func (a *Analyzer) beginLocalLoopResultAssignment(assignment *syntax.AssignStmt) func() {
	if a.loopResultLocalPlan == nil || !a.loopResultLocalPlan.AssignmentSourceMatches(assignment) {
		return func() {}
	}
	return a.beginLocalLoopResultValue(assignment.Value)
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
	var element = a.loopResultLocalValues[binding.Value]
	if element == nil {
		return nil
	}
	var valid = a.loopResultLocalPlan != nil && a.loopResultLocalPlan.BindingValueSourceMatches(binding) &&
		(!binding.Mutable || a.loopResultLocalPlan.MutableBindings[binding])
	if name, identifier := binding.Value.(*syntax.IdentExpr); identifier {
		rhs, found := scope.Lookup(name.Name)
		valid = valid && found && rhs.loopResultElement != nil &&
			rhs.Mutable == a.loopResultLocalPlan.BindingSources[binding].Source.Mutable &&
			element.Equals(rhs.loopResultElement) && rhs.Type != nil && types.NewList(element).Equals(rhs.Type)
	}
	if !valid || declared == nil || !types.NewList(element).Equals(declared) {
		a.error(CodeTargetValueShape, binding.Span, "local loop-result cells require their enrolled source and exact inferred or declared List type")
	}
	return element
}

// The source stamp distinguishes actual mutable reads and inferred declarations
// from mutated metadata. Every store still requires exact semantic equality.
func (a *Analyzer) validateLocalLoopResultAssignment(assignment *syntax.AssignStmt, target *Symbol, scope *Scope) {
	var element = a.loopResultLocalValues[assignment.Value]
	if target.loopResultElement == nil && element == nil {
		return
	}
	var valid = a.loopResultLocalPlan != nil && a.loopResultLocalPlan.AssignmentSourceMatches(assignment)
	if name, identifier := assignment.Value.(*syntax.IdentExpr); identifier {
		rhs, found := scope.Lookup(name.Name)
		valid = valid && found && rhs.loopResultElement != nil &&
			rhs.Mutable == a.loopResultLocalPlan.AssignmentSources[assignment].Source.Mutable &&
			element != nil && rhs.loopResultElement.Equals(element) &&
			rhs.Type != nil && types.NewList(element).Equals(rhs.Type)
	}
	if !valid || !target.Mutable || target.loopResultElement == nil || element == nil ||
		!target.loopResultElement.Equals(element) || target.Type == nil ||
		!types.NewList(element).Equals(target.Type) || a.exprTypes[assignment.Value] == nil ||
		!target.Type.Equals(a.exprTypes[assignment.Value]) {
		a.error(CodeTargetValueShape, assignment.Span, "local loop-result assignment requires its enrolled source and an exact List representation")
	}
}
