package lower

import (
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// A function fallback is an authored semantic value, never opaque native
// metadata. Preserve a Map's AST identity before entering the existing codec.
// Ordinary parameters need a branch-output Dictionary producer; Optional
// parameters already have a typed carrier and keep the Dictionary inline.
// Other defaults retain their existing value, effect and empty-List routes.
func (l *Lowerer) lowerFunctionDefaultValue(expr syntax.Expression, insideOptionalCarrier bool, expected types.Type) (interface{}, error) {
	if types.PrivateValueTransport(expected) || types.PrivateValueTransport(l.transportType(expr)) {
		previous := l.inExprStmt
		l.inExprStmt = false
		defer func() { l.inExprStmt = previous }()
		return l.lowerTypedRuntimeValue(expr, expected)
	}
	if _, dictionary := expr.(*syntax.MapExpr); !dictionary {
		return l.lowerCallValue(expr)
	}
	previous := l.inExprStmt
	l.inExprStmt = false
	defer func() { l.inExprStmt = previous }()
	if insideOptionalCarrier {
		return l.lowerSemanticValue(expr)
	}
	return l.lowerRuntimeLiteral(expr)
}
