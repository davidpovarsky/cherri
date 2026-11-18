package analysis

import (
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// The canonical List constructor writes text rows. Preserve the established
// List<Text> source guarantee when every consumed item is already proven Text;
// an opaque native reference is not evidence of a narrower element type.
func (a *Analyzer) variadicListOutputType(action *schema.ActionSchema, call *syntax.CallExpr) (types.Type, bool) {
	if !action.Builtin || action.CallableName != "list" || action.AppleIdentifier != "is.workflow.actions.list" ||
		len(action.Parameters) != 1 || !action.Parameters[0].Variadic || action.Parameters[0].Type == nil || !action.Parameters[0].Type.Equals(types.Text) {
		return nil, false
	}
	values := make([]syntax.Expression, 0, len(call.NamedArgs)+1)
	if call.PrimaryArg != nil {
		values = append(values, call.PrimaryArg)
	}
	for _, argument := range call.NamedArgs {
		values = append(values, argument.Value)
	}
	for _, value := range values {
		inferred := a.exprTypes[value]
		if inferred == nil || !inferred.Equals(types.Text) {
			return types.NewList(types.AnyContent), true
		}
	}
	return types.NewList(types.Text), true
}
