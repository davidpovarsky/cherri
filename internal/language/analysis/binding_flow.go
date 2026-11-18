package analysis

import (
	"maps"

	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// A symbol's declared Type remains the assignment contract. Its flowType is
// only a path-local read fact; replacing the value never changes the contract.
func (s *Symbol) readType() types.Type {
	if s.flowType != nil {
		return s.flowType
	}
	return s.Type
}

func narrowAssignedValue(symbol *Symbol, value types.Type) {
	symbol.flowType = nil
	optional, ok := symbol.Type.(types.OptionalType)
	if !ok || value == nil || value.Kind() == types.KindUnknown || value.Kind() == types.KindAnyContent {
		return
	}
	if value.AssignableTo(optional.Inner) {
		symbol.flowType = value
	}
}

type bindingFlowState struct {
	initialized bool
	readType    types.Type
}
type bindingFlow map[*Symbol]bindingFlowState

func captureBindingFlow(scope *Scope) bindingFlow {
	result := bindingFlow{}
	for current := scope; current != nil; current = current.parent {
		for _, symbol := range current.symbols {
			result[symbol] = bindingFlowState{symbol.Initialized, symbol.readType()}
		}
	}
	return result
}

func (state bindingFlow) restore() {
	for symbol, value := range state {
		symbol.Initialized = value.initialized
		symbol.flowType = nil
		if value.readType != nil && symbol.Type != nil && !value.readType.Equals(symbol.Type) {
			symbol.flowType = value.readType
		}
	}
}

// A value is definitely initialized only when every continuing path has an
// initializer. A narrower read type survives only when the paths agree.
// No state from a child lexical declaration becomes visible to its parent.
func mergeBindingFlow(entry bindingFlow, paths ...bindingFlow) {
	entry.restore()
	if len(paths) == 0 {
		return
	}
	for symbol := range entry {
		first, ok := paths[0][symbol]
		if !ok {
			continue
		}
		initialized, sameType := first.initialized, true
		for _, path := range paths[1:] {
			value, found := path[symbol]
			initialized = initialized && found && value.initialized
			sameType = sameType && found && first.readType != nil && value.readType != nil && first.readType.Equals(value.readType)
		}
		symbol.Initialized = initialized
		symbol.flowType = nil
		if initialized && sameType && first.readType != nil && symbol.Type != nil && !first.readType.Equals(symbol.Type) {
			symbol.flowType = first.readType
		}
	}
}

func statementMayContinue(statement syntax.Statement) bool {
	switch node := statement.(type) {
	case *syntax.ReturnStmt:
		return false
	case *syntax.BlockStmt:
		if node == nil {
			return true
		}
		for _, child := range node.Statements {
			if !statementMayContinue(child) {
				return false
			}
		}
	case *syntax.IfStmt:
		return statementMayContinue(node.ThenBlock) || node.ElseBlock == nil || statementMayContinue(node.ElseBlock)
	case *syntax.MenuStmt:
		if len(node.Cases) == 0 {
			return true
		}
		for _, item := range node.Cases {
			if statementMayContinue(item.BodyBlock) {
				return true
			}
		}
		return false
	}
	return true
}

// Only a checked optional binding gains a non-optional read fact. Truthiness,
// arbitrary content, JSON null, and unknown values do not establish a cast.
func (a *Analyzer) refineCondition(expr syntax.Expression, truth bool, scope *Scope, excluded map[*Symbol]bool) {
	switch node := expr.(type) {
	case *syntax.UnaryExpr:
		if node.Op == syntax.TokenBang {
			a.refineCondition(node.Operand, !truth, scope, excluded)
		}
	case *syntax.BinaryExpr:
		if (node.Op == syntax.TokenAnd && truth) || (node.Op == syntax.TokenOr && !truth) {
			// The RHS executes later and may replace a value proved by the LHS.
			leftExcluded := maps.Clone(excluded)
			if leftExcluded == nil {
				leftExcluded = map[*Symbol]bool{}
			}
			for symbol := range writtenBindings(node.Right, scope) {
				leftExcluded[symbol] = true
			}
			a.refineCondition(node.Left, truth, scope, leftExcluded)
			a.refineCondition(node.Right, truth, scope, excluded)
			return
		}
		if node.Op != syntax.TokenEqual && node.Op != syntax.TokenNotEqual {
			return
		}
		if truth != (node.Op == syntax.TokenNotEqual) {
			return
		}
		identifier, none := node.Left, node.Right
		if literal, ok := node.Left.(*syntax.LiteralExpr); ok && literal.Type == syntax.TokenNone {
			identifier, none = node.Right, node.Left
		}
		literal, isLiteral := none.(*syntax.LiteralExpr)
		name, isName := identifier.(*syntax.IdentExpr)
		if !isLiteral || literal.Type != syntax.TokenNone || !isName {
			return
		}
		symbol, found := scope.Lookup(name.Name)
		if !found || !symbol.Initialized || excluded[symbol] {
			return
		}
		if optional, ok := symbol.Type.(types.OptionalType); ok {
			symbol.flowType = optional.Inner
		}
	}
}

// The RHS is conditional: its writes cannot establish definite initialization
// after &&/||. Its own analysis can use facts guaranteed by the tested LHS.
func (a *Analyzer) analyzeLogicalRight(expr *syntax.BinaryExpr, scope *Scope) types.Type {
	entry := captureBindingFlow(scope)
	a.refineCondition(expr.Left, expr.Op == syntax.TokenAnd, scope, nil)
	result := a.analyzeExpression(expr.Right, scope)
	right := captureBindingFlow(scope)
	mergeBindingFlow(entry, entry, right)
	return result
}

// Find writes to already-visible bindings, respecting source-order lexical
// declarations. This excludes a local shadow but includes a write in that
// shadow's initializer, which still executes in the enclosing environment.
func writtenBindings(node syntax.Node, scope *Scope) map[*Symbol]bool {
	writes := map[*Symbol]bool{}
	var walk func(syntax.Node, map[string]bool)
	walk = func(node syntax.Node, locals map[string]bool) {
		if node == nil {
			return
		}
		switch value := node.(type) {
		case *syntax.BindingStmt:
			walk(value.Value, locals)
			locals[value.Name] = true
		case *syntax.AssignStmt:
			walk(value.Value, locals)
			if !locals[value.Name] {
				if symbol, found := scope.Lookup(value.Name); found {
					writes[symbol] = true
				}
			}
		case *syntax.ExprStmt:
			walk(value.Expr, locals)
		case *syntax.ReturnStmt:
			walk(value.Value, locals)
		case *syntax.YieldStmt:
			walk(value.Value, locals)
		case *syntax.BlockStmt:
			if value == nil {
				return
			}
			child := maps.Clone(locals)
			for _, statement := range value.Statements {
				walk(statement, child)
			}
		case *syntax.IfStmt:
			walk(value.Condition, locals)
			walk(value.ThenBlock, maps.Clone(locals))
			walk(value.ElseBlock, maps.Clone(locals))
		case *syntax.ForStmt:
			walk(value.Iterable, locals)
			child := maps.Clone(locals)
			child[value.ItemVar], child[value.IndexVar] = true, true
			child["RepeatItem"], child["RepeatIndex"] = true, true
			walk(value.BodyBlock, child)
		case *syntax.RepeatStmt:
			walk(value.Count, locals)
			child := maps.Clone(locals)
			child[value.IndexVar], child["RepeatIndex"] = true, true
			walk(value.BodyBlock, child)
		case *syntax.MenuStmt:
			walk(value.Prompt, locals)
			for _, item := range value.Cases {
				walk(item.Value, locals)
				walk(item.BodyBlock, maps.Clone(locals))
			}
		case *syntax.CallExpr:
			walk(value.Callee, locals)
			walk(value.PrimaryArg, locals)
			for _, argument := range value.NamedArgs {
				walk(argument.Value, locals)
			}
			if value.Block != nil {
				walk(value.Block.Body, maps.Clone(locals))
			}
		case *syntax.UnaryExpr:
			walk(value.Operand, locals)
		case *syntax.BinaryExpr:
			walk(value.Left, locals)
			walk(value.Right, locals)
		case *syntax.FStringExpr:
			for _, part := range value.Parts {
				if part.IsExpr {
					walk(part.Expr, locals)
				}
			}
		case *syntax.MemberExpr:
			walk(value.Target, locals)
		case *syntax.IndexExpr:
			walk(value.Target, locals)
			walk(value.Index, locals)
		case *syntax.ListExpr:
			for _, element := range value.Elements {
				walk(element, locals)
			}
		case *syntax.MapExpr:
			for _, item := range value.Entries {
				walk(item.Key, locals)
				walk(item.Value, locals)
			}
		case *syntax.RecordExpr:
			for _, field := range value.Fields {
				walk(field.Value, locals)
			}
		}
	}
	walk(node, map[string]bool{})
	return writes
}

func invalidateLoopRefinements(body *syntax.BlockStmt, scope *Scope) {
	// A write in a later iteration invalidates a fact established before the
	// loop, even for a read appearing earlier in the textual body. A guard
	// inside the body can establish a fresh fact for the current iteration.
	for symbol := range writtenBindings(body, scope) {
		symbol.flowType = nil
	}
}
