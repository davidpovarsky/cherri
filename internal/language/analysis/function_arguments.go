package analysis

import (
	"fmt"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// checkUserFunctionArguments checks arguments after they have been analyzed.
// Binding order deliberately matches the function envelope lowerer: primary,
// explicit name (including the public name of an underscore-prefixed parameter),
// then remaining positional arguments. A declaration is a static contract, not
// a validation action that can establish the shape of opaque external content.
func (a *Analyzer) checkUserFunctionArguments(fn *syntax.FunctionDecl, call *syntax.CallExpr) {
	unnamedArgIndex := 0
	for i, param := range fn.Parameters {
		var argument syntax.Expression
		if i == 0 {
			argument = call.PrimaryArg
		}
		if argument == nil {
			for _, named := range call.NamedArgs {
				if named.Label != "" && (named.Label == param.Name || strings.TrimPrefix(param.Name, "_") == named.Label) {
					argument = named.Value
					break
				}
			}
		}
		if argument == nil {
			for unnamedArgIndex < len(call.NamedArgs) {
				named := call.NamedArgs[unnamedArgIndex]
				unnamedArgIndex++
				if named.Label == "" {
					argument = named.Value
					break
				}
			}
		}
		if argument == nil || param.TypeExpr == nil {
			continue
		}
		actual := a.exprTypes[argument]
		expected := a.resolveType(param.TypeExpr)
		if actual != nil && !actual.AssignableTo(expected) {
			a.error(CodeArgumentType, argument.NodeSpan(), fmt.Sprintf("argument for %s expects %s, got %s", param.Name, expected, actual))
		}
	}
}
