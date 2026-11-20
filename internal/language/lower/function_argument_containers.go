package lower

import (
	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// A function's Dictionary envelope contains semantic argument values. Preserve
// an authored Map's AST identity so ordinary native-looking keys and literal
// braces cannot select raw native/template codecs. Scalar and native-reference
// argument routes retain their existing representation.
func (l *Lowerer) lowerFunctionArgumentValue(call *syntax.CallExpr, argumentIndex int, expr syntax.Expression, optional bool) (interface{}, interface{}, error) {
	if literal, ok := expr.(*syntax.LiteralExpr); ok && literal.Type == syntax.TokenNone {
		// The slot binder validates an Optional parameter and writes value absence
		// independently from the caller's supplied/default-presence flag.
		return nil, "0", nil
	}
	if !optional {
		if err := l.requirePresentTransport(expr, "function argument"); err != nil {
			return nil, nil, err
		}
	}
	previous := l.inExprStmt
	l.inExprStmt = false
	defer func() { l.inExprStmt = previous }()

	var value interface{}
	var err error
	_, literalMap := expr.(*syntax.MapExpr)
	if literalMap {
		value, err = l.lowerSemanticValue(expr)
	} else {
		value, err = l.lowerCallValue(expr)
	}
	if err != nil {
		return nil, nil, err
	}
	writes := l.callWritesAfter(call, argumentIndex)
	var presence interface{}
	if optional {
		presence, err = l.optionalValuePresence(expr, value)
		if err != nil {
			return nil, nil, err
		}
		// Capture metadata before the corresponding value snapshot. The last
		// emitted output remains the data consumed by a later authored action.
		if name, ok := expr.(*syntax.IdentExpr); ok {
			if binding, found := l.bindings[name.Name]; found && binding.IsVariable && writes[binding.VariableName] {
				presence = l.snapshotOptionalPresence(presence)
			}
		}
	}
	if !literalMap {
		captured, err := l.captureCallArgumentValue(expr, value, writes)
		return captured, presence, err
	}
	if len(writes) == 0 {
		return value, presence, nil
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
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid, OutputName: "Dictionary"}, presence, nil
}
