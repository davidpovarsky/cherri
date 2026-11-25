package schema

import (
	"strings"

	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// LoopResultLocalPlan is source-owned admission for one finite local consumer
// slice. It is keyed by actual AST identities, never a carrier marker or span.
// Immutable aliases and explicitly typed mutable aliases in structured flow
// carry the private sequence only to Items Count, index and for consumers.
// Every other use makes this optional plan absent.
type LoopResultLocalPlan struct {
	BindingValues   map[syntax.Expression]bool
	AllowedValues   map[syntax.Expression]bool
	Bodies          map[*syntax.BlockStmt]bool
	MutableBindings map[*syntax.BindingStmt]bool
	Assignments     map[*syntax.AssignStmt]*syntax.BindingStmt
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
	var plan = &LoopResultLocalPlan{
		BindingValues:   map[syntax.Expression]bool{},
		AllowedValues:   map[syntax.Expression]bool{},
		Bodies:          map[*syntax.BlockStmt]bool{},
		MutableBindings: map[*syntax.BindingStmt]bool{},
		Assignments:     map[*syntax.AssignStmt]*syntax.BindingStmt{},
	}
	var bindingOrigins = map[syntax.Expression]loopResultOriginSet{}
	var assignmentOrigins = map[*syntax.AssignStmt]loopResultOriginSet{}
	var consumedOrigins = loopResultOriginSet{}
	var consume = func(origins loopResultOriginSet) {
		for origin := range origins {
			consumedOrigins[origin] = true
		}
	}
	var expression func(syntax.Expression, loopResultEnvironment, bool, bool) (loopResultOriginSet, bool)
	var statement func(syntax.Statement, loopResultEnvironment, bool) bool
	var block func(*syntax.BlockStmt, loopResultEnvironment, bool) bool
	block = func(body *syntax.BlockStmt, scope loopResultEnvironment, cellIteration bool) bool {
		if body == nil {
			return true
		}
		var local = scope.clone()
		for _, child := range body.Statements {
			if !statement(child, local, cellIteration) {
				return false
			}
		}
		scope.takeWrites(local)
		return true
	}
	var loop = func(node syntax.Expression, scope loopResultEnvironment, cellIteration bool) bool {
		var body *syntax.BlockStmt
		var indexName, itemName string
		switch value := node.(type) {
		case *syntax.RepeatStmt:
			if _, okay := expression(value.Count, scope, false, cellIteration); !okay {
				return false
			}
			body, indexName = value.BodyBlock, value.IndexVar
		case *syntax.ForStmt:
			origins, okay := expression(value.Iterable, scope, true, cellIteration)
			if !okay {
				return false
			}
			consume(origins)
			// The iterable is evaluated once, before the first iteration.
			// Backedge writes do not change which sequence this for reads.
			if len(origins) != 0 && (strings.HasPrefix(value.ItemVar, "RepeatItem") || value.ItemVar == "") {
				return false
			}
			cellIteration = cellIteration || len(origins) != 0
			body, itemName, indexName = value.BodyBlock, value.ItemVar, value.IndexVar
		default:
			return false
		}
		var entry = scope.clone()
		var head = entry.clone()
		if itemName != "" {
			head.names[itemName] = loopResultLocalBinding{}
		}
		if indexName != "" {
			head.names[indexName] = loopResultLocalBinding{}
		}
		// The zero-iteration entry and every backedge can reach the next
		// iteration. Each step only grows finite sets of actual producer
		// AST identities; immutable snapshots are recomputed at their
		// authored position, including dependencies several writes away.
		for {
			var iteration = head.clone()
			if !block(body, iteration, cellIteration) {
				return false
			}
			if !head.join(entry, iteration) {
				break
			}
		}
		scope.takeWrites(head)
		return true
	}
	expression = func(node syntax.Expression, scope loopResultEnvironment, allowCells, cellIteration bool) (loopResultOriginSet, bool) {
		if node == nil {
			return nil, true
		}
		switch value := node.(type) {
		case *syntax.IdentExpr:
			if cellIteration && strings.HasPrefix(value.Name, "RepeatItem") {
				return nil, false
			}
			var origins = scope.origins(value.Name)
			if len(origins) != 0 {
				if !allowCells {
					return nil, false
				}
				plan.AllowedValues[node] = true
			}
			return origins, true
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
			origins, target := expression(value.Target, scope, true, cellIteration)
			_, index := expression(value.Index, scope, false, cellIteration)
			if target && index {
				consume(origins)
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
			if _, shadowed := scope.names[name.Name]; shadowed {
				return nil, false
			}
			action, found := registry.LookupAction(name.Name)
			if !found || !LoopResultItemsCountConsumer(value, action) || value.PrimaryArg == nil {
				return nil, false
			}
			origins, okay := expression(value.PrimaryArg, scope, true, cellIteration)
			if okay {
				consume(origins)
			}
			return nil, okay
		case *syntax.RepeatStmt, *syntax.ForStmt:
			if !loop(node, scope, cellIteration) {
				return nil, false
			}
			if !allowCells {
				return nil, true
			}
			plan.AllowedValues[node] = true
			return loopResultOriginSet{node: true}, true
		case *syntax.IfStmt:
			if _, okay := expression(value.Condition, scope, false, cellIteration); !okay {
				return nil, false
			}
			var thenPath, elsePath = scope.clone(), scope.clone()
			if !block(value.ThenBlock, thenPath, cellIteration) ||
				(value.ElseBlock != nil && !statement(value.ElseBlock, elsePath, cellIteration)) {
				return nil, false
			}
			scope.join(thenPath, elsePath)
			return nil, true
		case *syntax.MenuStmt:
			if _, okay := expression(value.Prompt, scope, false, cellIteration); !okay {
				return nil, false
			}
			// Labels are evaluated before any selected body.
			for _, item := range value.Cases {
				if _, okay := expression(item.Value, scope, false, cellIteration); !okay {
					return nil, false
				}
			}
			var paths []loopResultEnvironment
			for _, item := range value.Cases {
				var path = scope.clone()
				if !block(item.BodyBlock, path, cellIteration) {
					return nil, false
				}
				paths = append(paths, path)
			}
			if len(paths) != 0 {
				scope.join(paths...)
			}
			return nil, true
		}
		return nil, false
	}
	statement = func(node syntax.Statement, scope loopResultEnvironment, cellIteration bool) bool {
		switch value := node.(type) {
		case *syntax.BindingStmt:
			origins, okay := expression(value.Value, scope, true, cellIteration)
			if !okay {
				return false
			}
			if value.Mutable && len(origins) != 0 {
				name, identifier := value.Value.(*syntax.IdentExpr)
				if value.TypeExpr == nil || !identifier {
					return false
				}
				var rhs = scope.names[name.Name]
				if rhs.declaration == nil || rhs.declaration.Mutable || len(rhs.origins) == 0 {
					return false
				}
				plan.MutableBindings[value] = true
			}
			scope.declare(value, origins)
			if len(origins) != 0 {
				bindingOrigins[value.Value] = mergeLoopResultOrigins(bindingOrigins[value.Value], origins)
			}
			return true
		case *syntax.AssignStmt:
			var target = scope.names[value.Name]
			if len(scope.origins(value.Name)) == 0 {
				origins, okay := expression(value.Value, scope, false, cellIteration)
				return okay && len(origins) == 0
			}
			name, identifier := value.Value.(*syntax.IdentExpr)
			if value.Op != syntax.TokenAssign || !identifier ||
				target.declaration == nil || !plan.MutableBindings[target.declaration] {
				return false
			}
			var rhs = scope.names[name.Name]
			if rhs.declaration == nil || rhs.declaration.Mutable || len(rhs.origins) == 0 {
				return false
			}
			origins, okay := expression(value.Value, scope, true, cellIteration)
			if !okay || len(origins) == 0 {
				return false
			}
			plan.Assignments[value] = target.declaration
			assignmentOrigins[value] = mergeLoopResultOrigins(assignmentOrigins[value], origins)
			scope.values[target.declaration] = origins
			return true
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
	var scope = newLoopResultEnvironment()
	for _, child := range program.Statements {
		if !statement(child, scope, false) {
			return nil
		}
	}
	var demanded = func(origins loopResultOriginSet) bool {
		for origin := range origins {
			if !consumedOrigins[origin] {
				return false
			}
		}
		return len(origins) != 0
	}
	// Every possible stored origin still needs an enrolled consumer.
	// Representation joins never turn an unconsumed ordinary value into cells.
	for binding := range plan.MutableBindings {
		if !demanded(bindingOrigins[binding.Value]) {
			return nil
		}
	}
	for _, origins := range assignmentOrigins {
		if !demanded(origins) {
			return nil
		}
	}
	for value, origins := range bindingOrigins {
		if !demanded(origins) {
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
