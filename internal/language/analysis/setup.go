package analysis

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// A setup declaration is an import-time parameter binding, not a runtime value.
// Scope lookup is authoritative: a local binding with the same spelling is an
// ordinary value and must not consume or rebind the outer import question.
func (a *Analyzer) analyzeSetupArgument(action *schema.ActionSchema, parameter *schema.ParameterSchema, expression syntax.Expression, scope *Scope, callName string) types.Type {
	identifier, direct := expression.(*syntax.IdentExpr)
	if !direct {
		return a.analyzeEnumArgument(parameter, expression, scope, callName)
	}
	symbol, found := scope.Lookup(identifier.Name)
	if !found || symbol.Setup == nil {
		return a.analyzeEnumArgument(parameter, expression, scope, callName)
	}
	supported := parameter.WireKey != "" && !parameter.Variadic && parameter.EnumName == "" && len(parameter.EnumValues) == 0 && !requiresLiteralArgument(parameter)
	for _, macro := range a.registry.CompilerMacros() {
		if macro.ID == action.ID {
			supported = false
			break
		}
	}
	if !supported {
		a.error(CodeSetupBinding, expression.NodeSpan(), fmt.Sprintf("setup %q cannot bind to %s.%s: one concrete non-literal native parameter is required", identifier.Name, callName, parameter.Label))
		return a.analyzeEnumArgument(parameter, expression, scope, callName)
	}
	a.setupUses[symbol.Setup]++
	if a.setupUses[symbol.Setup] > 1 {
		a.error(CodeSetupBinding, expression.NodeSpan(), fmt.Sprintf("setup %q is reused; declare a separate question for each native parameter", identifier.Name))
	}
	previous := a.allowedSetupExpression
	a.allowedSetupExpression = expression
	defer func() { a.allowedSetupExpression = previous }()
	return a.analyzeEnumArgument(parameter, expression, scope, callName)
}
