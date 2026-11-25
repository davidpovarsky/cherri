package analysis

import (
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// Infer only the element holes created by actual authored empty List literals.
// Source-owned storage edges impose equality, not a permissive union: every
// concrete producer and explicit annotation in a component must agree. Unknown
// native values, omitted results and unrelated same-name declarations provide
// no evidence. The final analyzer pass still runs every ordinary diagnostic.
func (a *Analyzer) inferLocalLoopEmptyLists() map[*syntax.ListExpr]types.Type {
	plan := a.loopResultLocalPlan
	if plan == nil {
		return nil
	}
	parent := map[syntax.Expression]syntax.Expression{}
	var find func(syntax.Expression) syntax.Expression
	find = func(node syntax.Expression) syntax.Expression {
		p, found := parent[node]
		if !found {
			parent[node] = node
			return node
		}
		if p != node {
			parent[node] = find(p)
		}
		return parent[node]
	}
	join := func(left, right syntax.Expression) {
		if left != nil && right != nil {
			parent[find(right)] = find(left)
		}
	}
	for binding, stamp := range plan.BindingSources {
		if !plan.BindingValues[binding.Value] {
			continue
		}
		if !plan.BindingValueSourceMatches(binding) {
			return nil
		}
		find(binding.Value)
		if stamp.Source != nil {
			join(binding.Value, stamp.Source.Value)
		}
	}
	for assignment, target := range plan.Assignments {
		if !plan.AssignmentSourceMatches(assignment) {
			return nil
		}
		join(assignment.Value, target.Value)
		if source := plan.AssignmentSources[assignment].Source; source != nil {
			join(assignment.Value, source.Value)
		}
	}
	components := map[syntax.Expression][]syntax.Expression{}
	for node := range parent {
		key := find(node)
		components[key] = append(components[key], node)
	}
	annotations := map[syntax.Expression][]types.Type{}
	for binding := range plan.BindingSources {
		if !plan.BindingValues[binding.Value] {
			continue
		}
		if binding.TypeExpr != nil {
			annotations[find(binding.Value)] = append(annotations[find(binding.Value)], a.typeAnnotations[binding.TypeExpr])
		}
	}
	result := map[*syntax.ListExpr]types.Type{}
	for key, nodes := range components {
		var wanted types.Type
		valid := true
		constrain := func(t types.Type) {
			if !concreteLoopInferenceType(t) {
				valid = false
				return
			}
			if wanted == nil {
				wanted = t
			} else if !wanted.Equals(t) {
				valid = false
			}
		}
		for _, t := range annotations[key] {
			constrain(t)
		}
		for _, node := range nodes {
			if _, alias := node.(*syntax.IdentExpr); alias {
				continue
			}
			if t := a.exprTypes[node]; concreteLoopInferenceType(t) {
				constrain(t)
			}
		}
		if !valid || wanted == nil {
			continue
		}
		if _, list := wanted.(types.ListType); !list {
			continue
		}
		candidate := map[*syntax.ListExpr]types.Type{}
		for _, node := range nodes {
			if _, alias := node.(*syntax.IdentExpr); alias {
				continue
			}
			if !a.proveLoopEmptyContext(node, wanted, candidate) {
				valid = false
				break
			}
		}
		if valid {
			for literal, t := range candidate {
				if previous, exists := result[literal]; exists && !previous.Equals(t) {
					return nil
				}
				result[literal] = t
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func concreteLoopInferenceType(t types.Type) bool {
	if t == nil || t.Kind() == types.KindUnknown || t.Kind() == types.KindAnyContent {
		return false
	}
	switch value := t.(type) {
	case types.ListType:
		return concreteLoopInferenceType(value.Element)
	case types.MapType:
		return concreteLoopInferenceType(value.Key) && concreteLoopInferenceType(value.Value)
	case types.OptionalType:
		return concreteLoopInferenceType(value.Inner)
	case types.UnionType:
		for _, member := range value.Members {
			if !concreteLoopInferenceType(member) {
				return false
			}
		}
	}
	return true
}

func (a *Analyzer) proveLoopEmptyContext(expr syntax.Expression, wanted types.Type, context map[*syntax.ListExpr]types.Type) bool {
	if actual := a.exprTypes[expr]; actual != nil && actual.Equals(wanted) {
		return true
	}
	switch node := expr.(type) {
	case *syntax.ListExpr:
		list, okay := wanted.(types.ListType)
		if !okay {
			return false
		}
		if len(node.Elements) == 0 {
			if previous, found := context[node]; found && !previous.Equals(wanted) {
				return false
			}
			context[node] = wanted
			return true
		}
		for _, child := range node.Elements {
			if !a.proveLoopEmptyContext(child, list.Element, context) {
				return false
			}
		}
		return true
	case *syntax.RepeatStmt, *syntax.ForStmt:
		list, okay := wanted.(types.ListType)
		body := schema.LoopResultLocalBody(expr)
		if !okay || body == nil || !a.loopResultLocalPlan.Bodies[body] {
			return false
		}
		return a.proveLoopEmptyResult(body, list.Element, context)
	}
	return false
}

func (a *Analyzer) proveLoopEmptyResult(statement syntax.Statement, wanted types.Type, context map[*syntax.ListExpr]types.Type) bool {
	switch node := statement.(type) {
	case *syntax.BlockStmt:
		return node != nil && len(node.Statements) != 0 &&
			a.proveLoopEmptyResult(node.Statements[len(node.Statements)-1], wanted, context)
	case *syntax.YieldStmt:
		return a.proveLoopEmptyContext(node.Value, wanted, context)
	case *syntax.ReturnStmt:
		return true // This path terminates and contributes no loop element.
	case *syntax.IfStmt:
		return a.proveLoopEmptyResult(node.ThenBlock, wanted, context) &&
			a.proveLoopEmptyResult(node.ElseBlock, wanted, context)
	case *syntax.MenuStmt:
		if len(node.Cases) == 0 {
			return false
		}
		for _, item := range node.Cases {
			if !a.proveLoopEmptyResult(item.BodyBlock, wanted, context) {
				return false
			}
		}
		return true
	case *syntax.ExprStmt:
		switch control := node.Expr.(type) {
		case *syntax.IfStmt:
			return a.proveLoopEmptyResult(control, wanted, context)
		case *syntax.MenuStmt:
			return a.proveLoopEmptyResult(control, wanted, context)
		}
	}
	return false
}
