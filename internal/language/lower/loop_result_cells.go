package lower

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// A frame exists only while the actual canonical default Items Count lowers
// its inline loop argument. It is not attached to source bindings and cannot
// escape into an alias, index, native export, or function envelope.
type loopResultFrame struct {
	accumulator string
}

func (l *Lowerer) loopResultNeedsCells(expression syntax.Expression) bool {
	// A return-only body contributes no normally completed iteration result.
	// Match analysis and final-yield registration before inspecting the type of
	// an unreachable trailing yield. The loop itself may still execute zero times.
	var body *syntax.BlockStmt
	switch loop := expression.(type) {
	case *syntax.RepeatStmt:
		body = loop.BodyBlock
	case *syntax.ForStmt:
		body = loop.BodyBlock
	}
	if !optionalJoinMayContinue(body) {
		return false
	}
	result, ok := l.exprTypes[expression].(types.ListType)
	return ok && (nativeListElementType(result.Element) || types.MayBeAbsent(result.Element))
}

func collectLoopResultYields(body *syntax.BlockStmt, leaves map[*syntax.YieldStmt]bool) error {
	if !optionalJoinMayContinue(body) {
		return nil
	}
	if body == nil || len(body.Statements) == 0 {
		return fmt.Errorf("E_TARGET_VALUE_SHAPE: loop cell collection requires an explicit normal result")
	}
	switch last := body.Statements[len(body.Statements)-1].(type) {
	case *syntax.YieldStmt:
		leaves[last] = true
		return nil
	case *syntax.IfStmt:
		return collectOptionalJoinYields(last, leaves)
	case *syntax.MenuStmt:
		return collectOptionalJoinYields(last, leaves)
	default:
		return fmt.Errorf("E_TARGET_VALUE_SHAPE: loop cell collection requires a normal final yield at %s", body.NodeSpan())
	}
}

func (l *Lowerer) beginLoopResultItemsCount(call *syntax.CallExpr, action *schema.ActionSchema) (func(), error) {
	body := schema.LoopResultItemsCountBody(call, action)
	if body == nil || !l.loopResultNeedsCells(call.PrimaryArg) {
		return func() {}, nil
	}
	leaves := map[*syntax.YieldStmt]bool{}
	if err := collectLoopResultYields(body, leaves); err != nil {
		return nil, err
	}
	if len(leaves) == 0 {
		return nil, fmt.Errorf("E_TARGET_VALUE_SHAPE: loop cell collection has no normal result")
	}
	frame := &loopResultFrame{accumulator: l.freshLexicalName("loopResults_" + l.GenerateUUID())}
	previousFrames, previousYields := l.loopResultFrames, l.loopResultYields
	l.loopResultFrames = make(map[syntax.Expression]*loopResultFrame, len(previousFrames)+1)
	for expression, previous := range previousFrames {
		l.loopResultFrames[expression] = previous
	}
	l.loopResultFrames[call.PrimaryArg] = frame
	l.loopResultYields = make(map[*syntax.YieldStmt]*loopResultFrame, len(previousYields)+len(leaves))
	for yield, previous := range previousYields {
		l.loopResultYields[yield] = previous
	}
	for yield := range leaves {
		l.loopResultYields[yield] = frame
	}
	return func() {
		l.loopResultFrames, l.loopResultYields = previousFrames, previousYields
	}, nil
}

// Explicit clearing precedes the native loop, including the zero-iteration
// path. Nothing prevents an earlier authored result from becoming Set's input.
func (l *Lowerer) initializeLoopResultCells(expression syntax.Expression) {
	if frame := l.loopResultFrames[expression]; frame != nil {
		l.emitClearedVariable(frame.accumulator)
	}
}

func (l *Lowerer) readLoopResultCells(frame *loopResultFrame) *ir.AttachmentToken {
	uuid := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID: uuid, AppleIdentifier: "is.workflow.actions.getvariable",
		OutputUUID: uuid, OutputName: "LoopResultCells",
		Parameters: map[string]interface{}{
			"WFVariable": &ir.AttachmentToken{Type: "Variable", OutputName: frame.accumulator},
		},
	})
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid, OutputName: "LoopResultCells"}
}

// Exactly one Dictionary is appended for each normally completed final yield.
// The Dictionary's fields retain the payload and source-derived presence.
// Earlier overwritten yields and return-only paths are not registered leaves.
func (l *Lowerer) lowerLoopResultYield(yield *syntax.YieldStmt, frame *loopResultFrame) error {
	previous := l.inExprStmt
	l.inExprStmt = false
	defer func() { l.inExprStmt = previous }()
	var value interface{}
	var err error
	switch yield.Value.(type) {
	case *syntax.MapExpr, *syntax.ListExpr:
		value, err = l.lowerSemanticValue(yield.Value)
	default:
		value, err = l.lowerRuntimeLiteral(yield.Value)
	}
	if err != nil {
		return err
	}
	presence, err := l.optionalValuePresence(yield.Value, value)
	if err != nil {
		return err
	}
	cell := l.optionalCarrier(value, presence, "has_value", "value", "_cherri_loop_result_cell_v1")
	l.emitAction(&ir.NativeActionNode{
		NodeID: l.GenerateUUID(), AppleIdentifier: "is.workflow.actions.appendvariable",
		Parameters: map[string]interface{}{
			"WFVariableName": frame.accumulator,
			"WFInput":        cell,
		},
	})
	return nil
}
