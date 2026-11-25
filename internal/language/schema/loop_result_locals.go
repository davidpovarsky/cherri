package schema

import (
	"strings"

	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// LoopResultLocalPlan is source-owned admission for one finite local storage
// and consumer slice. It is keyed by actual AST identities, never a carrier marker or span.
// Immutable and mutable bindings of source-owned values in structured flow
// carry the private sequence only to Items Count, index and for consumers.
// Every other use makes this optional plan absent.
type LoopResultLocalPlan struct {
	ScopedBodies      map[*syntax.CallExpr]*syntax.BlockStmt
	PageCallbacks     map[*syntax.CallExpr]PageCallbackSource
	BindingValues     map[syntax.Expression]bool
	AllowedValues     map[syntax.Expression]bool
	Bodies            map[*syntax.BlockStmt]bool
	MutableBindings   map[*syntax.BindingStmt]bool
	Assignments       map[*syntax.AssignStmt]*syntax.BindingStmt
	BindingSources    map[*syntax.BindingStmt]LoopResultBindingSource
	AssignmentSources map[*syntax.AssignStmt]LoopResultAssignmentSource
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
		ScopedBodies:      map[*syntax.CallExpr]*syntax.BlockStmt{},
		PageCallbacks:     map[*syntax.CallExpr]PageCallbackSource{},
		BindingValues:     map[syntax.Expression]bool{},
		AllowedValues:     map[syntax.Expression]bool{},
		Bodies:            map[*syntax.BlockStmt]bool{},
		MutableBindings:   map[*syntax.BindingStmt]bool{},
		Assignments:       map[*syntax.AssignStmt]*syntax.BindingStmt{},
		BindingSources:    map[*syntax.BindingStmt]LoopResultBindingSource{},
		AssignmentSources: map[*syntax.AssignStmt]LoopResultAssignmentSource{},
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
			if !okay {
				return nil, false
			}
			if _, shadowed := scope.names[name.Name]; shadowed {
				return nil, false
			}
			action, found := registry.LookupAction(name.Name)
			if found && name.Name == "paginate" {
				body := LoopResultScopedPaginateBody(value, action)
				if body == nil {
					return nil, false
				}
				if _, okay := expression(value.PrimaryArg, scope, false, cellIteration); !okay {
					return nil, false
				}
				for _, argument := range value.NamedArgs {
					if _, okay := expression(argument.Value, scope, false, cellIteration); !okay {
						return nil, false
					}
				}
				parameter := PaginatePageParameter(value)
				plan.ScopedBodies[value] = body
				plan.PageCallbacks[value] = PageCallbackSource{Block: value.Block, Body: body, Parameter: parameter, Name: parameter.Name}
				// The page binding is local and immutable; captured mutable
				// declarations join zero pages and every actual page backedge.
				entry, head := scope.clone(), scope.clone()
				head.names[parameter.Name] = loopResultLocalBinding{}
				for {
					iteration := head.clone()
					if !block(body, iteration, cellIteration) {
						return nil, false
					}
					if !head.join(entry, iteration) {
						break
					}
				}
				scope.takeWrites(head)
				return nil, true
			}
			if found && LoopResultExplicitDelay(value, action) {
				_, okay := expression(value.PrimaryArg, scope, false, cellIteration)
				return nil, okay
			}
			if found && name.Name == "measure" {
				body := LoopResultScopedMeasureBody(value, action)
				if body == nil {
					return nil, false
				}
				plan.ScopedBodies[value] = body
				// The original body executes once. Its lexical declarations
				// stay local; actual writes to captured outer slots survive.
				// The final body expression still follows the existing
				// no-private-cell-escape rules enforced by statement().
				return nil, block(body, scope, cellIteration)
			}
			if found && name.Name == "cache" {
				body := LoopResultScopedCacheBody(value, action)
				if body == nil {
					return nil, false
				}
				plan.ScopedBodies[value] = body
				// A hit leaves captured slots untouched; a miss executes this exact body
				// once. Join both paths instead of treating lazy effects as unconditional.
				entry, miss := scope.clone(), scope.clone()
				if !block(body, miss, cellIteration) {
					return nil, false
				}
				options, _ := CacheCallOptions(value)
				if options.TTL == 0 {
					scope.takeWrites(miss)
				} else {
					scope.join(entry, miss)
				}
				return nil, true
			}
			if !found || name.Name != "count" || !LoopResultItemsCountConsumer(value, action) || value.PrimaryArg == nil {
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
			if len(origins) != 0 {
				var source *syntax.BindingStmt
				if name, identifier := value.Value.(*syntax.IdentExpr); identifier {
					source = scope.names[name.Name].declaration
					if source == nil {
						return false
					}
				}
				plan.BindingSources[value] = LoopResultBindingSource{
					Name: value.Name, Value: value.Value, Annotation: value.TypeExpr,
					Mutable: value.Mutable, Source: source, Body: LoopResultLocalBody(value.Value),
				}
				if value.Mutable {
					plan.MutableBindings[value] = true
				}
				bindingOrigins[value.Value] = mergeLoopResultOrigins(bindingOrigins[value.Value], origins)
			}
			scope.declare(value, origins)
			return true
		case *syntax.AssignStmt:
			var target = scope.names[value.Name]
			if len(scope.origins(value.Name)) == 0 {
				origins, okay := expression(value.Value, scope, false, cellIteration)
				return okay && len(origins) == 0
			}
			if (value.Op != syntax.TokenAssign && value.Op != syntax.TokenPlusAssign) ||
				target.declaration == nil || !plan.MutableBindings[target.declaration] {
				return false
			}
			origins, okay := expression(value.Value, scope, true, cellIteration)
			if !okay || len(origins) == 0 {
				return false
			}
			var source *syntax.BindingStmt
			if name, identifier := value.Value.(*syntax.IdentExpr); identifier {
				source = scope.names[name.Name].declaration
				if source == nil {
					return false
				}
			}
			plan.Assignments[value] = target.declaration
			plan.AssignmentSources[value] = LoopResultAssignmentSource{Value: value.Value, Source: source, Body: LoopResultLocalBody(value.Value), Op: value.Op}
			assignmentOrigins[value] = mergeLoopResultOrigins(assignmentOrigins[value], origins)
			bindingOrigins[value.Value] = mergeLoopResultOrigins(bindingOrigins[value.Value], origins)
			if value.Op == syntax.TokenPlusAssign {
				// The RHS can itself write the target. Native Append reads the
				// target after evaluating that RHS, not its earlier value.
				origins = mergeLoopResultOrigins(scope.values[target.declaration], origins)
			}
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
	// A mutable declaration has one representation throughout its lifetime.
	// Every initialized or stored version must use it, including versions
	// overwritten before a consumer. This is storage admission, not an
	// invented consumer demand; all closed-input/escape checks above remain.
	var storageByBinding = map[*syntax.BindingStmt]loopResultOriginSet{}
	for binding := range plan.MutableBindings {
		storageByBinding[binding] = bindingOrigins[binding.Value]
	}
	for assignment, origins := range assignmentOrigins {
		var target = plan.Assignments[assignment]
		storageByBinding[target] = mergeLoopResultOrigins(storageByBinding[target], origins)
	}
	var storedOrigins = loopResultOriginSet{}
	for _, origins := range storageByBinding {
		for origin := range origins {
			storedOrigins[origin] = true
		}
	}
	var represented = func(origins loopResultOriginSet) bool {
		for origin := range origins {
			if !consumedOrigins[origin] && !storedOrigins[origin] {
				return false
			}
		}
		return len(origins) != 0
	}
	for value, origins := range bindingOrigins {
		if !represented(origins) {
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
// The semantic payload check separately requires a checked recursive container
// shape; it cannot admit an opaque native value from a matching marker.
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
			return loopResultLocalPayloadSupported(node.Value, expressionTypes)
		}
		return false
	}
	return walk(body)
}
