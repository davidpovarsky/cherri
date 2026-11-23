package schema

import (
	"strings"

	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// LoopResultLocalPlan is source-owned admission for one finite local consumer
// slice. It is keyed by actual AST identities, never a carrier marker or span.
// Only immutable aliases, Items Count, index targets and for iterables can
// carry the private sequence. Every other use makes this optional plan absent.
type LoopResultLocalPlan struct {
	BindingValues map[syntax.Expression]bool
	AllowedValues map[syntax.Expression]bool
	Bodies        map[*syntax.BlockStmt]bool
}

// PlanLoopResultLocals deliberately admits a closed, explicit-input program.
// Unknown/native/user actions and function/import interfaces are not covered.
// This prevents private Nothing/Set/Append/Get helpers from becoming a source
// action's omitted Previous input. Programs outside the slice retain the
// original loop-result diagnostics; their ordinary non-cell lowering is intact.
func PlanLoopResultLocals(program *syntax.Program, registry *Registry) *LoopResultLocalPlan {
	if program == nil || registry == nil {
		return nil
	}
	mode, modeError := syntax.ResolveMainOutputMode(program)
	if modeError != nil || mode != syntax.MainOutputExplicit {
		// Private cells are not a legacy incidental main result.
		return nil
	}
	for _, declaration := range program.Declarations {
		switch declaration.(type) {
		case *syntax.ShortcutDecl, *syntax.TypeDecl, *syntax.EnumDecl:
		default:
			return nil
		}
	}
	plan := &LoopResultLocalPlan{
		BindingValues: map[syntax.Expression]bool{},
		AllowedValues: map[syntax.Expression]bool{},
		Bodies:        map[*syntax.BlockStmt]bool{},
	}
	bindingOrigins := map[syntax.Expression]syntax.Expression{}
	consumedOrigins := map[syntax.Expression]bool{}
	type environment map[string]syntax.Expression
	copyEnvironment := func(source environment) environment {
		target := environment{}
		for name, origin := range source {
			target[name] = origin
		}
		return target
	}
	var expression func(syntax.Expression, environment, bool, bool) (syntax.Expression, bool)
	var statement func(syntax.Statement, environment, bool) bool
	var block func(*syntax.BlockStmt, environment, bool) bool
	block = func(body *syntax.BlockStmt, scope environment, cellIteration bool) bool {
		if body == nil {
			return true
		}
		local := copyEnvironment(scope)
		for _, child := range body.Statements {
			if !statement(child, local, cellIteration) {
				return false
			}
		}
		return true
	}
	loop := func(node syntax.Expression, scope environment, cellIteration bool) bool {
		local := copyEnvironment(scope)
		switch value := node.(type) {
		case *syntax.RepeatStmt:
			if _, okay := expression(value.Count, scope, false, cellIteration); !okay {
				return false
			}
			local[value.IndexVar] = nil
			return block(value.BodyBlock, local, cellIteration)
		case *syntax.ForStmt:
			origin, okay := expression(value.Iterable, scope, true, cellIteration)
			if !okay {
				return false
			}
			if origin != nil {
				consumedOrigins[origin] = true
			}
			local[value.ItemVar], local[value.IndexVar] = nil, nil
			// The new slice uses the declared item, not raw RepeatItem aliases.
			if origin != nil && (strings.HasPrefix(value.ItemVar, "RepeatItem") || value.ItemVar == "") {
				return false
			}
			return block(value.BodyBlock, local, cellIteration || origin != nil)
		}
		return false
	}
	expression = func(node syntax.Expression, scope environment, allowCells, cellIteration bool) (syntax.Expression, bool) {
		if node == nil {
			return nil, true
		}
		switch value := node.(type) {
		case *syntax.IdentExpr:
			if cellIteration && strings.HasPrefix(value.Name, "RepeatItem") {
				return nil, false
			}
			origin := scope[value.Name]
			if origin != nil {
				if !allowCells {
					return nil, false
				}
				plan.AllowedValues[node] = true
			}
			return origin, true
		case *syntax.LiteralExpr, *syntax.EnumMemberExpr:
			return nil, true
		case *syntax.UnaryExpr:
			_, okay := expression(value.Operand, scope, false, cellIteration)
			return nil, okay
		case *syntax.BinaryExpr:
			_, left := expression(value.Left, scope, false, cellIteration)
			_, right := expression(value.Right, scope, false, cellIteration)
			return nil, left && right
		case *syntax.FStringExpr:
			for _, part := range value.Parts {
				if part.IsExpr {
					if _, okay := expression(part.Expr, scope, false, cellIteration); !okay {
						return nil, false
					}
				}
			}
			return nil, true
		case *syntax.MemberExpr:
			_, okay := expression(value.Target, scope, false, cellIteration)
			return nil, okay
		case *syntax.IndexExpr:
			origin, target := expression(value.Target, scope, true, cellIteration)
			_, index := expression(value.Index, scope, false, cellIteration)
			if origin != nil && target && index {
				consumedOrigins[origin] = true
			}
			return nil, target && index
		case *syntax.ListExpr:
			for _, element := range value.Elements {
				if _, okay := expression(element, scope, false, cellIteration); !okay {
					return nil, false
				}
			}
			return nil, true
		case *syntax.MapExpr:
			for _, item := range value.Entries {
				if _, okay := expression(item.Key, scope, false, cellIteration); !okay {
					return nil, false
				}
				if _, okay := expression(item.Value, scope, false, cellIteration); !okay {
					return nil, false
				}
			}
			return nil, true
		case *syntax.CallExpr:
			name, okay := value.Callee.(*syntax.IdentExpr)
			if !okay || name.Name != "count" {
				return nil, false
			}
			// A local declaration with the same spelling is not the
			// canonical registry action, even when its type is Number.
			if _, shadowed := scope[name.Name]; shadowed {
				return nil, false
			}
			action, found := registry.LookupAction(name.Name)
			if !found || !LoopResultItemsCountConsumer(value, action) || value.PrimaryArg == nil {
				return nil, false
			}
			origin, okay := expression(value.PrimaryArg, scope, true, cellIteration)
			if origin != nil && okay {
				consumedOrigins[origin] = true
			}
			return nil, okay
		case *syntax.RepeatStmt, *syntax.ForStmt:
			if !loop(node, scope, cellIteration) {
				return nil, false
			}
			if !allowCells {
				// Ordinary scalar-result loops can still be emitted normally.
				// List/Optional results retain the original diagnostic because
				// this expression is not enrolled as a local cell producer.
				return nil, true
			}
			plan.AllowedValues[node] = true
			return node, true
		case *syntax.IfStmt:
			_, okay := expression(value.Condition, scope, false, cellIteration)
			return nil, okay && block(value.ThenBlock, scope, cellIteration) && (value.ElseBlock == nil || statement(value.ElseBlock, copyEnvironment(scope), cellIteration))
		case *syntax.MenuStmt:
			if _, okay := expression(value.Prompt, scope, false, cellIteration); !okay {
				return nil, false
			}
			for _, item := range value.Cases {
				if _, okay := expression(item.Value, scope, false, cellIteration); !okay || !block(item.BodyBlock, scope, cellIteration) {
					return nil, false
				}
			}
			return nil, true
		}
		return nil, false
	}
	statement = func(node syntax.Statement, scope environment, cellIteration bool) bool {
		switch value := node.(type) {
		case *syntax.BindingStmt:
			origin, okay := expression(value.Value, scope, true, cellIteration)
			if !okay || value.Mutable && origin != nil {
				return false
			}
			scope[value.Name] = origin
			if origin != nil {
				bindingOrigins[value.Value] = origin
			}
			return true
		case *syntax.AssignStmt:
			origin, okay := expression(value.Value, scope, false, cellIteration)
			return okay && origin == nil && scope[value.Name] == nil
		case *syntax.ExprStmt:
			_, okay := expression(value.Expr, scope, false, cellIteration)
			return okay
		case *syntax.ReturnStmt:
			_, okay := expression(value.Value, scope, false, cellIteration)
			return okay
		case *syntax.YieldStmt:
			_, okay := expression(value.Value, scope, false, cellIteration)
			return okay
		case *syntax.BlockStmt:
			return block(value, scope, cellIteration)
		case *syntax.IfStmt, *syntax.MenuStmt:
			_, okay := expression(node.(syntax.Expression), scope, false, cellIteration)
			return okay
		case *syntax.ForStmt, *syntax.RepeatStmt:
			return loop(node.(syntax.Expression), scope, cellIteration)
		}
		return false
	}
	scope := environment{}
	for _, child := range program.Statements {
		if !statement(child, scope, false) {
			return nil
		}
	}
	// Demand is keyed by the actual producer through lexical aliases.
	// Unconsumed potential producers remain on the original shape path.
	// This also leaves an unused ordinary scalar loop binding unchanged.
	for value, origin := range bindingOrigins {
		if !consumedOrigins[origin] {
			continue
		}
		plan.BindingValues[value] = true
		if body := LoopResultLocalBody(value); body != nil {
			plan.Bodies[body] = true
		}
	}
	if len(plan.BindingValues) == 0 {
		return nil
	}
	return plan
}

// The closed Count's exact no-implicit-action body restriction is reused.
// The separate semantic payload check below keeps nested/dynamic List payload
// decoding outside this first local consumer increment.
func LoopResultLocalBody(expression syntax.Expression) *syntax.BlockStmt {
	var body *syntax.BlockStmt
	switch loop := expression.(type) {
	case *syntax.RepeatStmt:
		body = loop.BodyBlock
	case *syntax.ForStmt:
		body = loop.BodyBlock
	}
	if body == nil || !loopResultExplicitStatement(body) {
		return nil
	}
	return body
}

func LoopResultLocalPayloadsSupported(body *syntax.BlockStmt, expressionTypes map[syntax.Expression]types.Type) bool {
	var walk func(syntax.Statement) bool
	walk = func(statement syntax.Statement) bool {
		switch node := statement.(type) {
		case *syntax.BlockStmt:
			return node != nil && len(node.Statements) != 0 && walk(node.Statements[len(node.Statements)-1])
		case *syntax.ReturnStmt:
			return true
		case *syntax.IfStmt:
			return walk(node.ThenBlock) && walk(node.ElseBlock)
		case *syntax.MenuStmt:
			if len(node.Cases) == 0 {
				return false
			}
			for _, item := range node.Cases {
				if !walk(item.BodyBlock) {
					return false
				}
			}
			return true
		case *syntax.YieldStmt:
			if literal, okay := node.Value.(*syntax.LiteralExpr); okay && literal.Type == syntax.TokenNone {
				return true
			}
			t := expressionTypes[node.Value]
			if t == nil {
				return false
			}
			if optional, okay := t.(types.OptionalType); okay {
				t = optional.Inner
			}
			switch t.Kind() {
			case types.KindPrimitive:
				return t.Equals(types.Text) || t.Equals(types.Number) || t.Equals(types.Bool)
			case types.KindUnknown:
				// Only an independently carried None can pass lowering's
				// optionalValuePresence check; opaque Unknown stays rejected.
				return types.MayBeAbsent(expressionTypes[node.Value])
			case types.KindMap:
				_, literal := node.Value.(*syntax.MapExpr)
				return literal
			case types.KindList:
				literal, okay := node.Value.(*syntax.ListExpr)
				if !okay {
					return false
				}
				for _, element := range literal.Elements {
					itemType := expressionTypes[element]
					if itemType == nil || types.MayBeAbsent(itemType) {
						return false
					}
					switch itemType.Kind() {
					case types.KindPrimitive, types.KindMap:
					default:
						return false
					}
				}
				return true
			}
		}
		return false
	}
	return walk(body)
}
