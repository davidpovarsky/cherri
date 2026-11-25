package schema

import "github.com/electrikmilk/cherri/internal/language/syntax"

// Capture the authored declaration, not just its spelling. Inferred types have
// a deliberately nil annotation; deleting an authored annotation after planning
// is a different source and must not inherit that inferred admission.
type LoopResultBindingSource struct {
	Name       string
	Value      syntax.Expression
	Annotation syntax.TypeAnnotation
	Mutable    bool
	Source     *syntax.BindingStmt
	Body       *syntax.BlockStmt
}

type LoopResultAssignmentSource struct {
	Value  syntax.Expression
	Source *syntax.BindingStmt
	Body   *syntax.BlockStmt
	Op     syntax.TokenType
}

func (plan *LoopResultLocalPlan) BindingSourceMatches(binding *syntax.BindingStmt) bool {
	if plan == nil || binding == nil {
		return false
	}
	var source, found = plan.BindingSources[binding]
	return found && source.Name == binding.Name && source.Value == binding.Value &&
		source.Annotation == binding.TypeExpr && source.Mutable == binding.Mutable &&
		source.Body == LoopResultLocalBody(binding.Value)
}

func (plan *LoopResultLocalPlan) BindingValueSourceMatches(binding *syntax.BindingStmt) bool {
	if !plan.BindingSourceMatches(binding) || !plan.BindingValues[binding.Value] {
		return false
	}
	var source = plan.BindingSources[binding].Source
	if name, identifier := binding.Value.(*syntax.IdentExpr); identifier {
		return source != nil && plan.BindingSourceMatches(source) && name.Name == source.Name
	}
	var body = LoopResultLocalBody(binding.Value)
	return source == nil && body != nil && plan.Bodies[body]
}

func (plan *LoopResultLocalPlan) AssignmentSourceMatches(assignment *syntax.AssignStmt) bool {
	if plan == nil || assignment == nil {
		return false
	}
	var target = plan.Assignments[assignment]
	var source, found = plan.AssignmentSources[assignment]
	if !found || !plan.BindingSourceMatches(target) || !target.Mutable ||
		!plan.MutableBindings[target] || target.Name != assignment.Name ||
		(assignment.Op != syntax.TokenAssign && assignment.Op != syntax.TokenPlusAssign) ||
		source.Op != assignment.Op || source.Value != assignment.Value ||
		!plan.BindingValues[assignment.Value] || !plan.AllowedValues[assignment.Value] {
		return false
	}
	if name, identifier := assignment.Value.(*syntax.IdentExpr); identifier {
		return source.Source != nil && plan.BindingSourceMatches(source.Source) &&
			name.Name == source.Source.Name
	}
	var body = LoopResultLocalBody(assignment.Value)
	return source.Source == nil && body != nil && source.Body == body && plan.Bodies[body]
}
