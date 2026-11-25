package lower

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func (l *Lowerer) localLoopResultProducerElement(expression syntax.Expression) types.Type {
	if l.loopResultLocalPlan == nil || !l.loopResultLocalPlan.BindingValues[expression] {
		return nil
	}
	var result, okay = l.exprTypes[expression].(types.ListType)
	if !okay || !l.loopResultNeedsCells(expression) {
		return nil
	}
	var body = schema.LoopResultLocalBody(expression)
	if body == nil || !l.loopResultLocalPlan.Bodies[body] ||
		!schema.LoopResultLocalPayloadsSupported(body, l.exprTypes) {
		return nil
	}
	return result.Element
}

func (l *Lowerer) localLoopResultElement(expression syntax.Expression) types.Type {
	if name, okay := expression.(*syntax.IdentExpr); okay {
		return l.bindings[name.Name].LoopResultElement
	}
	return l.localLoopResultProducerElement(expression)
}

// A declaration and a whole assignment enter the same actual producer frame.
// Neither creates a synthetic binding or substitutes a reaching-origin value.
func (l *Lowerer) beginLocalLoopResultValue(expression syntax.Expression) (types.Type, func(), error) {
	var nothing = func() {}
	var element = l.localLoopResultElement(expression)
	if element == nil {
		return nil, nothing, nil
	}
	if _, identifier := expression.(*syntax.IdentExpr); identifier {
		return element, nothing, nil
	}
	var body = schema.LoopResultLocalBody(expression)
	var leaves = map[*syntax.YieldStmt]bool{}
	if err := collectLoopResultYields(body, leaves); err != nil {
		return nil, nothing, err
	}
	if len(leaves) == 0 {
		return nil, nothing, fmt.Errorf("E_TARGET_VALUE_SHAPE: local loop cells require a normal final result")
	}
	var frame = &loopResultFrame{accumulator: l.freshLexicalName("loopResults_" + l.GenerateUUID())}
	var oldFrames, oldYields = l.loopResultFrames, l.loopResultYields
	l.loopResultFrames = make(map[syntax.Expression]*loopResultFrame, len(oldFrames)+1)
	for source, old := range oldFrames {
		l.loopResultFrames[source] = old
	}
	l.loopResultFrames[expression] = frame
	l.loopResultYields = make(map[*syntax.YieldStmt]*loopResultFrame, len(oldYields)+len(leaves))
	for yield, old := range oldYields {
		l.loopResultYields[yield] = old
	}
	for yield := range leaves {
		l.loopResultYields[yield] = frame
	}
	return element, func() { l.loopResultFrames, l.loopResultYields = oldFrames, oldYields }, nil
}

func (l *Lowerer) beginLocalLoopResultBinding(binding *syntax.BindingStmt) (types.Type, func(), error) {
	var nothing = func() {}
	var element = l.localLoopResultElement(binding.Value)
	if element == nil {
		return nil, nothing, nil
	}
	var valid = l.loopResultLocalPlan != nil && l.loopResultLocalPlan.BindingValueSourceMatches(binding) &&
		(!binding.Mutable || l.loopResultLocalPlan.MutableBindings[binding])
	if name, identifier := binding.Value.(*syntax.IdentExpr); identifier {
		var rhs, found = l.bindings[name.Name]
		valid = valid && found && rhs.LoopResultElement != nil &&
			rhs.IsVariable == l.loopResultLocalPlan.BindingSources[binding].Source.Mutable &&
			element.Equals(rhs.LoopResultElement) && rhs.Type != nil && types.NewList(element).Equals(rhs.Type)
	}
	if !valid || l.exprTypes[binding.Value] == nil || !types.NewList(element).Equals(l.exprTypes[binding.Value]) || binding.TypeExpr != nil && !types.NewList(element).Equals(l.resolveTypeAnnotation(binding.TypeExpr)) {
		return nil, nothing, fmt.Errorf("E_TARGET_VALUE_SHAPE: local loop cells require their enrolled source and exact inferred or declared List type at %s", binding.Span)
	}
	return l.beginLocalLoopResultValue(binding.Value)
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

// Validate before lowering the RHS or entering ordinary assignment. The actual
// binding token supplies the value; the plan's origins never replace that read.
func (l *Lowerer) validateLocalLoopResultAssignment(assignment *syntax.AssignStmt) error {
	var target = l.bindings[assignment.Name]
	var element = l.localLoopResultElement(assignment.Value)
	if target.LoopResultElement == nil && element == nil {
		return nil
	}
	var valid = l.loopResultLocalPlan != nil && l.loopResultLocalPlan.AssignmentSourceMatches(assignment)
	if name, identifier := assignment.Value.(*syntax.IdentExpr); identifier {
		var rhs, found = l.bindings[name.Name]
		valid = valid && found && rhs.LoopResultElement != nil &&
			rhs.IsVariable == l.loopResultLocalPlan.AssignmentSources[assignment].Source.Mutable &&
			element != nil && rhs.LoopResultElement.Equals(element) &&
			rhs.Type != nil && types.NewList(element).Equals(rhs.Type)
	}
	if !valid || !target.IsVariable || target.LoopResultElement == nil || element == nil ||
		!target.LoopResultElement.Equals(element) || target.Type == nil ||
		!types.NewList(element).Equals(target.Type) ||
		l.exprTypes[assignment.Value] == nil || !target.Type.Equals(l.exprTypes[assignment.Value]) {
		return fmt.Errorf("E_TARGET_VALUE_SHAPE: local loop-result assignment requires its enrolled source and an exact List representation at %s", assignment.Span)
	}
	return nil
}
