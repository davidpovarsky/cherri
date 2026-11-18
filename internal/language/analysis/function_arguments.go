package analysis

import (
	"fmt"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// checkUserFunctionArguments binds and analyzes each argument in its parameter context.
// Binding order deliberately matches the function envelope lowerer: primary,
// explicit name (including the public name of an underscore-prefixed parameter),
// then remaining positional arguments. Track every supplied argument so this
// compatibility order cannot silently discard or reuse a value.
func (a *Analyzer) checkUserFunctionArguments(fn *syntax.FunctionDecl, call *syntax.CallExpr, scope *Scope) {
	usedNamed := make([]bool, len(call.NamedArgs))
	unnamedArgIndex := 0
	if call.PrimaryArg != nil && len(fn.Parameters) == 0 {
		a.error(CodePositionalArg, call.PrimaryArg.NodeSpan(), fmt.Sprintf("function %q does not accept positional arguments", fn.Name))
		a.analyzeExpression(call.PrimaryArg, scope)
	}
	for i, param := range fn.Parameters {
		var argument syntax.Expression
		if i == 0 {
			argument = call.PrimaryArg
		}
		if argument == nil {
			for index, named := range call.NamedArgs {
				if named.Label != "" && (named.Label == param.Name || strings.TrimPrefix(param.Name, "_") == named.Label) {
					if usedNamed[index] {
						a.error(CodeDuplicateArgument, named.Span, fmt.Sprintf("argument label %q is ambiguous across parameters in call to %q", named.Label, fn.Name))
					}
					usedNamed[index] = true
					argument = named.Value
					break
				}
			}
		}
		if argument == nil {
			for unnamedArgIndex < len(call.NamedArgs) {
				index := unnamedArgIndex
				named := call.NamedArgs[index]
				unnamedArgIndex++
				if named.Label == "" {
					usedNamed[index] = true
					argument = named.Value
					break
				}
			}
		}
		if argument == nil {
			if !param.Optional && param.DefaultExpr == nil {
				a.error(CodeMissingArgument, call.Span, fmt.Sprintf("missing required argument %q in call to %q", param.Name, fn.Name))
			}
			continue
		}
		actual := a.analyzeEnumContext(argument, scope, a.enumCatalog.TypeName(param.TypeExpr))
		if param.TypeExpr == nil {
			continue
		}
		expected := a.resolveType(param.TypeExpr)
		if actual != nil && !actual.AssignableTo(expected) {
			a.error(CodeArgumentType, argument.NodeSpan(), fmt.Sprintf("argument for %s expects %s, got %s", param.Name, expected, actual))
		}
	}
	for index, named := range call.NamedArgs {
		if usedNamed[index] {
			continue
		}
		a.analyzeExpression(named.Value, scope)
		if named.Label == "" {
			a.error(CodePositionalArg, named.Span, fmt.Sprintf("too many positional arguments in call to %q", fn.Name))
			continue
		}
		known := false
		for _, param := range fn.Parameters {
			if named.Label == param.Name || named.Label == strings.TrimPrefix(param.Name, "_") {
				known = true
				break
			}
		}
		if known {
			a.error(CodeDuplicateArgument, named.Span, fmt.Sprintf("duplicate argument for parameter %q in call to %q", named.Label, fn.Name))
		} else {
			a.error(CodeUnknownArgument, named.Span, fmt.Sprintf("unknown argument label %q in call to %q", named.Label, fn.Name))
		}
	}
}
