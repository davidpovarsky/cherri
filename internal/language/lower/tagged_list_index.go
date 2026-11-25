package lower

import (
	"fmt"
	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// Retain the existing integer/lower/upper bound guards; decode only an actual
// source-owned tagged sequence after the checked item has been selected.
func (l *Lowerer) lowerTaggedListIndex(index *syntax.IndexExpr, target, ordinal any) (*ir.AttachmentToken, error) {
	list, okay := presentType(l.transportType(index.Target)).(types.ListType)
	if !okay || !types.TaggedListElements(list) {
		return nil, fmt.Errorf("E_TARGET_VALUE_SHAPE: cell index has no source-owned sequence")
	}
	targetToken := l.materializeToAttachment(target)
	valueName := l.freshLexicalName("taggedIndexValue_" + l.GenerateUUID())
	presenceName := l.freshLexicalName("taggedIndexPresence_" + l.GenerateUUID())
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
	value, presence := l.decodeValueCell(cell, list.Element)
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
