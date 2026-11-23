package lower

import (
	"reflect"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// The paired flag is compiler metadata. Emit it only for source bindings whose
// presence has a consumer. Dependencies deliberately overapproximate lexical
// names: collisions may retain a flag, but must never remove a needed one.
// The result is keyed by the original AST binding, so generated/renamed nodes
// conservatively retain their existing behavior.
func (l *Lowerer) prepareOptionalPresenceDemand(program *syntax.Program) {
	roots := []any{program}
	for _, function := range l.functions {
		roots = append(roots, function)
	}
	functions := map[string]bool{}
	optionalDemandWalk(roots, func(node syntax.Node) {
		if function, ok := node.(*syntax.FunctionDecl); ok {
			functions[function.Name] = true
			if separator := strings.LastIndex(function.Name, "."); separator >= 0 {
				functions[function.Name[separator+1:]] = true
			}
		}
	})
	bindings := map[*syntax.BindingStmt]string{}
	dependencies := map[string]map[string]bool{}
	dependencyExpressions := map[string][]syntax.Expression{}
	l.optionalJoinDemand = map[syntax.Expression]bool{}
	optionalDemandWalk(roots, func(node syntax.Node) {
		switch value := node.(type) {
		case *syntax.IfStmt:
			l.optionalJoinDemand[value] = false
		case *syntax.MenuStmt:
			l.optionalJoinDemand[value] = false
		}
	})
	needed := map[string]bool{}
	addNames := func(target map[string]bool, expression syntax.Expression) {
		optionalDemandWalk(expression, func(node syntax.Node) {
			if identifier, ok := node.(*syntax.IdentExpr); ok {
				target[identifier.Name] = true
			}
		})
	}
	markJoins := func(expression syntax.Expression) {
		optionalDemandWalk(expression, func(node syntax.Node) {
			switch value := node.(type) {
			case *syntax.IfStmt:
				l.optionalJoinDemand[value] = true
			case *syntax.MenuStmt:
				l.optionalJoinDemand[value] = true
			}
		})
	}
	seed := func(expression syntax.Expression) {
		addNames(needed, expression)
		markJoins(expression)
	}
	depend := func(name string, expression syntax.Expression) {
		dependencyExpressions[name] = append(dependencyExpressions[name], expression)
		if dependencies[name] == nil {
			dependencies[name] = map[string]bool{}
		}
		addNames(dependencies[name], expression)
	}
	isNone := func(expression syntax.Expression) bool {
		literal, ok := expression.(*syntax.LiteralExpr)
		return ok && literal != nil && literal.Type == syntax.TokenNone
	}
	optionalDemandWalk(roots, func(node syntax.Node) {
		switch value := node.(type) {
		case *syntax.BindingStmt:
			bindings[value] = value.Name
			depend(value.Name, value.Value)
			if l.loopResultLocalPlan != nil && l.loopResultLocalPlan.BindingValues[value.Value] &&
				l.loopResultNeedsCells(value.Value) {
				// Each local cell consumes its value's independent presence,
				// even if the binding itself is only counted or unused.
				seed(value.Value)
			}
		case *syntax.AssignStmt:
			depend(value.Name, value.Value)
		case *syntax.ReturnStmt:
			// All authored returns are retained conservatively; only Optional
			// bindings actually emit paired flags during lowering.
			seed(value.Value)
		case *syntax.FunctionDecl:
			for _, parameter := range value.Parameters {
				seed(parameter.DefaultExpr)
			}
		case *syntax.CallExpr:
			name := ""
			switch callee := value.Callee.(type) {
			case *syntax.IdentExpr:
				name = callee.Name
			case *syntax.MemberExpr:
				name = callee.Property
			case *syntax.EnumMemberExpr:
				name = callee.Member
			}
			if action, found := l.registry.LookupAction(name); found && schema.LoopResultItemsCountBody(value, action) != nil {
				// Private loop cells consume presence even if Count's Number
				// result is not subsequently returned or compared with None.
				seed(value.PrimaryArg)
			}
			if functions[name] {
				// Include every written argument of a user-function call.
				// Ordinary canonical action inputs do not consume this flag.
				seed(value.PrimaryArg)
				for _, argument := range value.NamedArgs {
					seed(argument.Value)
				}
			}
		case *syntax.BinaryExpr:
			if value.Op == syntax.TokenEqual || value.Op == syntax.TokenNotEqual {
				if isNone(value.Right) {
					seed(value.Left)
				}
				if isNone(value.Left) {
					seed(value.Right)
				}
			}
		}
	})
	queue := []string{}
	for name := range needed {
		queue = append(queue, name)
	}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		for dependency := range dependencies[name] {
			if !needed[dependency] {
				needed[dependency] = true
				queue = append(queue, dependency)
			}
		}
	}
	for name := range needed {
		for _, expression := range dependencyExpressions[name] {
			markJoins(expression)
		}
	}
	// Imported variables have no authored BindingStmt. Retain explicit demand
	// decisions only for names with analyzed assignments/bindings; unknown or
	// generated names must still conservatively keep their presence metadata.
	l.importedOptionalPresenceDemand = map[string]bool{}
	for name := range dependencies {
		l.importedOptionalPresenceDemand[name] = needed[name]
	}
	l.optionalPresenceDemand = map[*syntax.BindingStmt]bool{}
	for binding, name := range bindings {
		l.optionalPresenceDemand[binding] = needed[name]
	}
}

func (l *Lowerer) importedOptionalPresenceNeeded(name string) bool {
	needed, analyzed := l.importedOptionalPresenceDemand[name]
	return !analyzed || needed
}

func (l *Lowerer) optionalPresenceNeeded(binding *syntax.BindingStmt) bool {
	needed, analyzed := l.optionalPresenceDemand[binding]
	return !analyzed || needed
}

// Walk only syntax-owned structures, including argument/Map/FString records.
// Source spans, comments and inactive FString text cannot create a demand.
// This is conservative dependency discovery, not an AST validity validator.
func optionalDemandWalk(root any, visit func(syntax.Node)) {
	seen := map[uintptr]bool{}
	var walk func(reflect.Value)
	walk = func(value reflect.Value) {
		if !value.IsValid() {
			return
		}
		switch value.Kind() {
		case reflect.Interface:
			if !value.IsNil() {
				walk(value.Elem())
			}
		case reflect.Pointer:
			if value.IsNil() || seen[value.Pointer()] {
				return
			}
			seen[value.Pointer()] = true
			if node, ok := value.Interface().(syntax.Node); ok {
				visit(node)
			}
			walk(value.Elem())
		case reflect.Struct:
			if value.Type().PkgPath() != reflect.TypeOf(syntax.Program{}).PkgPath() {
				return
			}
			if part, ok := value.Interface().(syntax.FStringPart); ok && !part.IsExpr {
				return
			}
			for i := 0; i < value.NumField(); i++ {
				walk(value.Field(i))
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < value.Len(); i++ {
				walk(value.Index(i))
			}
		case reflect.Map:
			iterator := value.MapRange()
			for iterator.Next() {
				walk(iterator.Value())
			}
		}
	}
	walk(reflect.ValueOf(root))
}
