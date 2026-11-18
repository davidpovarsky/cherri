package migrate

import (
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// Migrate canonical Legacy variadics after parsing, so nested calls, literals,
// comments and lexical shadowing retain their actual source structure. The
// canonical registry supplies the one repeated parameter's public label.
func migrateVariadicCalls(program *syntax.Program, registry *schema.Registry) {
	declaredCallables := map[string]bool{}
	for _, declaration := range program.Declarations {
		switch declared := declaration.(type) {
		case *syntax.FunctionDecl:
			declaredCallables[declared.Name] = true
		case *syntax.ActionDecl:
			declaredCallables[declared.Name] = true
		}
	}
	walkMigrationBindings(program, nil, nil, func(call *syntax.CallExpr, scope *migrationBindingScope) {
		callee, ok := call.Callee.(*syntax.IdentExpr)
		if !ok || declaredCallables[callee.Name] {
			return
		}
		for current := scope; current != nil; current = current.parent {
			if _, shadowed := current.names[callee.Name]; shadowed {
				return
			}
		}
		action, ok := registry.LookupAction(callee.Name)
		// All canonical Legacy variadic constructors have one repeated parameter.
		// A future mixed signature requires its own binding contract rather than
		// silently packing unrelated fixed arguments.
		if !ok || len(action.Parameters) != 1 || !action.Parameters[0].Variadic {
			return
		}
		parameter := action.Parameters[0]
		if call.PrimaryArg == nil && len(call.NamedArgs) == 1 &&
			(call.NamedArgs[0].Label == parameter.Label || call.NamedArgs[0].Label == parameter.ID) {
			if _, packed := call.NamedArgs[0].Value.(*syntax.ListExpr); packed {
				return
			}
		}
		elements := make([]syntax.Expression, 0, len(call.NamedArgs)+1)
		if call.PrimaryArg != nil {
			elements = append(elements, call.PrimaryArg)
		}
		for _, argument := range call.NamedArgs {
			if argument.Label != "" && argument.Label != parameter.Label && argument.Label != parameter.ID {
				return
			}
			elements = append(elements, argument.Value)
		}
		call.PrimaryArg = nil
		call.NamedArgs = []syntax.NamedArg{{Span: call.Span, Label: parameter.Label,
			Value: &syntax.ListExpr{Span: call.Span, Elements: elements}}}
	})
}
