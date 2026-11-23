package lower

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func (l *Lowerer) localLoopResultElement(expression syntax.Expression) types.Type {
	if name, okay := expression.(*syntax.IdentExpr); okay {
		return l.bindings[name.Name].LoopResultElement
	}
	return nil
}

func (l *Lowerer) beginLocalLoopResultBinding(binding *syntax.BindingStmt) (types.Type, func(), error) {
	nothing := func() {}
	if l.loopResultLocalPlan == nil || !l.loopResultLocalPlan.BindingValues[binding.Value] {
		return nil, nothing, nil
	}
	element := l.localLoopResultElement(binding.Value)
	if element == nil {
		result, okay := l.exprTypes[binding.Value].(types.ListType)
		if !okay || !l.loopResultNeedsCells(binding.Value) {
			return nil, nothing, nil
		}
		body := schema.LoopResultLocalBody(binding.Value)
		if body == nil || !l.loopResultLocalPlan.Bodies[body] ||
			!schema.LoopResultLocalPayloadsSupported(body, l.exprTypes) {
			return nil, nothing, nil
		}
		element = result.Element
		leaves := map[*syntax.YieldStmt]bool{}
		if err := collectLoopResultYields(body, leaves); err != nil {
			return nil, nothing, err
		}
		if len(leaves) == 0 {
			return nil, nothing, fmt.Errorf("E_TARGET_VALUE_SHAPE: local loop cells require a normal final result")
		}
		frame := &loopResultFrame{accumulator: l.freshLexicalName("loopResults_" + l.GenerateUUID())}
		oldFrames, oldYields := l.loopResultFrames, l.loopResultYields
		l.loopResultFrames = make(map[syntax.Expression]*loopResultFrame, len(oldFrames)+1)
		for expression, old := range oldFrames {
			l.loopResultFrames[expression] = old
		}
		l.loopResultFrames[binding.Value] = frame
		l.loopResultYields = make(map[*syntax.YieldStmt]*loopResultFrame, len(oldYields)+len(leaves))
		for yield, old := range oldYields {
			l.loopResultYields[yield] = old
		}
		for yield := range leaves {
			l.loopResultYields[yield] = frame
		}
		nothing = func() { l.loopResultFrames, l.loopResultYields = oldFrames, oldYields }
	}
	if binding.Mutable || binding.TypeExpr != nil && !types.NewList(element).Equals(l.resolveTypeAnnotation(binding.TypeExpr)) {
		nothing()
		return nil, func() {}, fmt.Errorf("E_TARGET_VALUE_SHAPE: local loop cells require an immutable exact List binding at %s", binding.Span)
	}
	return element, nothing, nil
}

func (l *Lowerer) localLoopResultIndexHasPresence(expression syntax.Expression) bool {
	index, okay := expression.(*syntax.IndexExpr)
	return okay && l.localLoopResultElement(index.Target) != nil
}

// Only the actual enrolled iterable supplies this decoder. An arbitrary
// Dictionary with matching keys is never treated as a compiler cell.
func (l *Lowerer) localLoopResultForItem(iterable syntax.Expression, nativeItem string) (any, any, types.Type) {
	item := &ir.AttachmentToken{Type: "Variable", OutputName: nativeItem}
	element := l.localLoopResultElement(iterable)
	if element == nil {
		return item, nil, nil
	}
	cell := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID: cell, AppleIdentifier: "is.workflow.actions.getvariable",
		OutputUUID: cell, OutputName: "LoopResultCell",
		Parameters: map[string]interface{}{"WFVariable": item},
	})
	value, presence := l.decodeOptionalCarrier(cell, "has_value", "value", "")
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: value},
		&ir.AttachmentToken{Type: "ActionOutput", OutputUUID: presence}, element
}

// Preserve all existing integer/lower/upper index checks. Target and index have
// already been evaluated exactly once. Bounds failure and a selected None cell
// both expose absent Optional values, while present false/0/empty keep flag 1.
func (l *Lowerer) lowerLocalLoopResultIndex(index *syntax.IndexExpr, target, ordinal any) (*ir.AttachmentToken, error) {
	if l.localLoopResultElement(index.Target) == nil {
		return nil, fmt.Errorf("E_TARGET_VALUE_SHAPE: cell index has no source-owned sequence")
	}
	targetToken := l.materializeToAttachment(target)
	valueName := l.freshLexicalName("loopIndexValue_" + l.GenerateUUID())
	presenceName := l.freshLexicalName("loopIndexPresence_" + l.GenerateUUID())
	l.emitClearedVariable(valueName)
	presenceToken := l.storeOptionalPresence(presenceName, "0")

	integralGroup, integralEnd := l.beginIntegralIndexGuard(ordinal)
	oneBased := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID: oneBased, AppleIdentifier: "is.workflow.actions.math", OutputUUID: oneBased,
		Parameters: map[string]interface{}{"WFMathOperation": "+", "WFInput": ordinal, "WFMathOperand": 1.0},
	})
	oneBasedToken := &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: oneBased}
	count := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID: count, AppleIdentifier: "is.workflow.actions.count", OutputUUID: count,
		Parameters: map[string]interface{}{"Input": targetToken, "WFCountType": "Items"},
	})
	difference := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID: difference, AppleIdentifier: "is.workflow.actions.math", OutputUUID: difference,
		Parameters: map[string]interface{}{
			"WFMathOperation": "-", "WFInput": &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: count},
			"WFMathOperand": oneBasedToken,
		},
	})
	lowerGroup, lowerEnd := l.GenerateUUID(), l.GenerateUUID()
	l.emitIfStart(lowerGroup, oneBasedToken, 3, 1.0, true)
	upperGroup, upperEnd := l.GenerateUUID(), l.GenerateUUID()
	l.emitIfStart(upperGroup, &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: difference}, 3, 0.0, true)
	cell := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID: cell, AppleIdentifier: "is.workflow.actions.getitemfromlist", OutputUUID: cell,
		Parameters: map[string]interface{}{"WFInput": targetToken, "WFItemSpecifier": "Item At Index", "WFItemIndex": oneBasedToken},
	})
	value, presence := l.decodeOptionalCarrier(cell, "has_value", "value", "")
	l.storeOptionalPresence(presenceName, &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: presence})
	l.emitAction(&ir.NativeActionNode{
		NodeID: l.GenerateUUID(), AppleIdentifier: "is.workflow.actions.setvariable",
		Parameters: map[string]interface{}{"WFVariableName": valueName, "WFInput": &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: value}},
	})
	l.emitIfElse(upperGroup)
	l.emitAction(&ir.NativeActionNode{NodeID: l.GenerateUUID(), AppleIdentifier: "is.workflow.actions.nothing"})
	l.emitIfEnd(upperGroup, upperEnd, "")
	l.emitIfElse(lowerGroup)
	l.emitAction(&ir.NativeActionNode{NodeID: l.GenerateUUID(), AppleIdentifier: "is.workflow.actions.nothing"})
	l.emitIfEnd(lowerGroup, lowerEnd, "")
	l.endIntegralIndexGuard(integralGroup, integralEnd, lowerEnd)

	result := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID: result, AppleIdentifier: "is.workflow.actions.getvariable", OutputUUID: result,
		Parameters: map[string]interface{}{"WFVariable": &ir.AttachmentToken{Type: "Variable", OutputName: valueName}},
	})
	if l.optionalExpressionPresence == nil {
		l.optionalExpressionPresence = make(map[syntax.Expression]interface{})
	}
	l.optionalExpressionPresence[index] = presenceToken
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: result}, nil
}
