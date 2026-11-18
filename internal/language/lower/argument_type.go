package lower

import (
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// Optional compiler-only call metadata. It never changes an action wire value.
func (l *Lowerer) argumentStaticType(expr syntax.Expression) string {
	t := l.exprTypes[expr]
	if t == nil {
		if id, ok := expr.(*syntax.IdentExpr); ok {
			if binding, found := l.bindings[id.Name]; found {
				t = binding.Type
			}
		}
	}
	if t == nil {
		return ""
	}
	if optional, ok := t.(types.OptionalType); ok {
		t = optional.Inner
	}
	if t.Kind() == types.KindRecord {
		return "Record"
	}
	return t.Name()
}
