package lower

import (
	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// A function's Dictionary envelope contains semantic argument values. Preserve
// an authored Map's AST identity so ordinary native-looking keys and literal
// braces cannot select raw native/template codecs. Scalar and native-reference
// argument routes retain their existing representation.
func (l *Lowerer) lowerFunctionArgumentValue(call *syntax.CallExpr, argumentIndex int, expr syntax.Expression) (interface{}, error) {
	if literal, ok := expr.(*syntax.LiteralExpr); ok && literal.Type == syntax.TokenNone {
		// The slot binder validates an Optional parameter and writes value absence
		// independently from the caller's supplied/default-presence flag.
		return nil, nil
	}
	if err := l.requirePresentTransport(expr, "function argument"); err != nil {
		return nil, err
	}
	if _, literalMap := expr.(*syntax.MapExpr); !literalMap {
		return l.lowerCallArgumentValue(call, argumentIndex, expr)
	}
	previous := l.inExprStmt
	l.inExprStmt = false
	defer func() { l.inExprStmt = previous }()

	value, err := l.lowerSemanticValue(expr)
	if err != nil {
		return nil, err
	}
	if len(l.callWritesAfter(call, argumentIndex)) == 0 {
		return value, nil
	}
	// The aggregate can contain deferred mutable references. Consume the typed
	// Dictionary now, before a subsequent written argument mutates those values.
	// This is the same argument-order boundary as the existing aggregate path.
	uuid := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID: uuid, AppleIdentifier: "is.workflow.actions.dictionary",
		OutputUUID: uuid, OutputName: "Dictionary",
		Parameters: map[string]interface{}{"WFItems": value},
	})
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid, OutputName: "Dictionary"}, nil
}
