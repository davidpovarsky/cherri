package lower

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func (l *Lowerer) optionalFunctionParameter(annotation syntax.TypeAnnotation) bool {
	return types.MayBeAbsent(l.resolveTypeAnnotation(annotation))
}

// These are semantic values inside the compiler's private function carrier.
// Retain explicit empty array/dictionary fields instead of Legacy's omitted
// empty parameter shape. Opaque native.plist values never enter this switch.
func functionEnvelopeValue(value interface{}) interface{} {
	switch container := value.(type) {
	case []interface{}:
		if len(container) == 0 {
			return backend.SemanticValue{Kind: backend.ValList, ListVal: []backend.SemanticValue{}}
		}
	case map[string]interface{}:
		if len(container) == 0 {
			return backend.SemanticValue{Kind: backend.ValDict, DictVal: []backend.DictEntry{}}
		}
	}
	return value
}

// Store the presence before the corresponding value write. The authored RHS
// has already been evaluated; keeping the value write last preserves its
// ordinary native output for a following action with implicit input.
func (l *Lowerer) storeOptionalPresence(name string, presence interface{}) *ir.AttachmentToken {
	input := l.materializeToAttachment(presence)
	l.emitAction(&ir.NativeActionNode{
		NodeID: l.GenerateUUID(), AppleIdentifier: "is.workflow.actions.setvariable",
		Parameters: map[string]interface{}{"WFVariableName": name, "WFInput": input},
	})
	return &ir.AttachmentToken{Type: "Variable", OutputName: name}
}

func (l *Lowerer) newMutableOptionalPresence(presence interface{}) *ir.AttachmentToken {
	return l.storeOptionalPresence("_cherri_optional_presence_"+l.GenerateUUID(), presence)
}

// An immutable value snapshot must freeze its presence too. A deferred
// mutable flag would otherwise observe a later assignment to the source.
func (l *Lowerer) snapshotOptionalPresence(presence interface{}) interface{} {
	token, variable := presence.(*ir.AttachmentToken)
	if !variable || token.Type != "Variable" {
		return presence
	}
	uuid := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID: uuid, AppleIdentifier: "is.workflow.actions.getvariable", OutputUUID: uuid,
		Parameters: map[string]interface{}{"WFVariable": token},
	})
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid}
}

func (l *Lowerer) assignOptionalPresence(binding BindingRef, assignment *syntax.AssignStmt, value interface{}) (interface{}, error) {
	tracked := binding.OptionalPresence != nil || types.MayBeAbsent(binding.Type) || value == nil || l.hasOptionalPresence(assignment.Value)
	if !tracked {
		return nil, nil
	}
	var presence interface{} = "1"
	if assignment.Op == syntax.TokenAssign {
		var err error
		presence, err = l.optionalValuePresence(assignment.Value, value)
		if err != nil {
			return nil, err
		}
	}
	if token, okay := binding.OptionalPresence.(*ir.AttachmentToken); okay && token.Type == "Variable" {
		return l.storeOptionalPresence(token.OutputName, presence), nil
	}
	return l.newMutableOptionalPresence(presence), nil
}

// Only compiler-owned presence participates in these predicates. Imported raw
// native predicates and unrelated action-content tests retain their own wire.
func (l *Lowerer) hasOptionalPresence(expr syntax.Expression) bool {
	if l.optionalExpressionPresence[expr] != nil {
		return true
	}
	if name, ok := expr.(*syntax.IdentExpr); ok {
		binding, found := l.bindings[name.Name]
		return found && binding.OptionalPresence != nil
	}
	call, ok := expr.(*syntax.CallExpr)
	if !ok {
		return false
	}
	name := ""
	switch callee := call.Callee.(type) {
	case *syntax.IdentExpr:
		name = callee.Name
	case *syntax.MemberExpr:
		if target, ok := callee.Target.(*syntax.IdentExpr); ok {
			name = target.Name + "." + callee.Property
			_, imported := l.modules[target.Name]
			if l.functions[name] == nil && !imported && target.Name != "native" {
				name = callee.Property
			}
		}
	case *syntax.EnumMemberExpr:
		name = callee.TypeName + "." + callee.Member
	}
	fn := l.functions[name]
	return fn != nil && l.optionalFunctionParameter(fn.ReturnType)
}

func (l *Lowerer) lowerOptionalNoneCondition(expr syntax.Expression) (*ir.AttachmentToken, int, bool, error) {
	negated := false
	for {
		unary, ok := expr.(*syntax.UnaryExpr)
		if !ok || unary.Op != syntax.TokenBang {
			break
		}
		negated = !negated
		expr = unary.Operand
	}
	binary, ok := expr.(*syntax.BinaryExpr)
	if !ok || (binary.Op != syntax.TokenEqual && binary.Op != syntax.TokenNotEqual) {
		return nil, 0, false, nil
	}
	isNone := func(value syntax.Expression) bool {
		literal, ok := value.(*syntax.LiteralExpr)
		return ok && literal.Type == syntax.TokenNone
	}
	var operand syntax.Expression
	if isNone(binary.Right) {
		operand = binary.Left
	} else if isNone(binary.Left) {
		operand = binary.Right
	} else {
		return nil, 0, false, nil
	}
	if !l.hasOptionalPresence(operand) {
		return nil, 0, false, nil
	}
	// Evaluate a producing call once before reading its result-presence edge.
	value, err := l.lowerExpression(operand)
	if err != nil {
		return nil, 0, true, err
	}
	presence, err := l.optionalValuePresence(operand, value)
	if err != nil {
		return nil, 0, true, err
	}
	code := 4
	if (binary.Op == syntax.TokenNotEqual) != negated {
		code = 5
	}
	return l.materializeToAttachment(presence), code, true, nil
}

func (l *Lowerer) materializeOptionalPredicate(input *ir.AttachmentToken, code int) *ir.AttachmentToken {
	group, result := l.GenerateUUID(), l.GenerateUUID()
	l.emitIfStartTyped(group, input, code, "0", nil, backend.OperandString, true)
	l.emitAction(&ir.NativeActionNode{NodeID: l.GenerateUUID(), AppleIdentifier: "is.workflow.actions.number", Parameters: map[string]interface{}{"WFNumberActionNumber": 1.0}})
	l.emitIfElse(group)
	l.emitAction(&ir.NativeActionNode{NodeID: l.GenerateUUID(), AppleIdentifier: "is.workflow.actions.number", Parameters: map[string]interface{}{"WFNumberActionNumber": 0.0}})
	l.emitIfEnd(group, result, "")
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: result}
}

// Argument omission selects a default in the outer dispatcher branch. A
// supplied Optional has a separate value-presence flag, so explicit None does
// not become empty Text and false/zero/empty values never select the default.
// The flag is compiler-owned metadata, not a native truthiness observation.
func (l *Lowerer) optionalField(dictionary, key, name string) string {
	uuid := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID: uuid, AppleIdentifier: "is.workflow.actions.getvalueforkey",
		OutputUUID: uuid, OutputName: name,
		Parameters: map[string]interface{}{
			"WFInput":         &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: dictionary},
			"WFDictionaryKey": key, "WFGetDictionaryValueType": "Value",
		},
	})
	return uuid
}

func (l *Lowerer) decodeOptionalCarrier(dictionary, presenceKey, valueKey, name string) (value, presence string) {
	presence = l.optionalField(dictionary, presenceKey, "")
	group := l.GenerateUUID()
	l.emitIfStart(group, &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: presence}, 4, "1", true)
	l.optionalField(dictionary, valueKey, "")
	l.emitIfElse(group)
	l.materializeYieldValue(nil)
	value = l.GenerateUUID()
	l.emitIfEnd(group, value, name)
	return value, presence
}

// Presence comes from source semantics or a compiler-owned tagged carrier.
// No HasAnyValue/Count/Text conversion is used to infer it from native content.
func (l *Lowerer) optionalValuePresence(expr syntax.Expression, value interface{}) (interface{}, error) {
	if value == nil {
		return "0", nil
	}
	if name, ok := expr.(*syntax.IdentExpr); ok {
		if binding, found := l.bindings[name.Name]; found && binding.OptionalPresence != nil {
			return binding.OptionalPresence, nil
		}
	}
	if presence := l.optionalExpressionPresence[expr]; presence != nil {
		return presence, nil
	}
	if types.MayBeAbsent(l.exprTypes[expr]) {
		return nil, fmt.Errorf("E_TARGET_VALUE_SHAPE: current function result transport lacks compiler-owned optional absence at %s", expr.NodeSpan())
	}
	return "1", nil
}

func (l *Lowerer) optionalCarrier(value, presence interface{}, presenceKey, valueKey, marker string) *ir.AttachmentToken {
	emit := func(present bool) *ir.AttachmentToken {
		flag := "0"
		if present {
			flag = "1"
		}
		fields := map[string]interface{}{presenceKey: flag}
		if marker != "" {
			fields[marker] = "1"
		}
		if present {
			fields[valueKey] = functionEnvelopeValue(value)
		}
		uuid := l.GenerateUUID()
		l.emitAction(&ir.NativeActionNode{
			NodeID: uuid, AppleIdentifier: "is.workflow.actions.dictionary",
			OutputUUID: uuid, OutputName: "OptionalValue",
			Parameters: map[string]interface{}{"WFItems": fields},
		})
		return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid, OutputName: "OptionalValue"}
	}
	if flag, known := presence.(string); known {
		return emit(flag == "1")
	}
	group := l.GenerateUUID()
	l.emitIfStart(group, l.materializeToAttachment(presence), 4, "1", true)
	emit(true)
	l.emitIfElse(group)
	emit(false)
	result := l.GenerateUUID()
	l.emitIfEnd(group, result, "OptionalValue")
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: result, OutputName: "OptionalValue"}
}

func (l *Lowerer) lowerOptionalParameter(dictionary string, slot int, name string, fallback syntax.Expression) (value, presence string, err error) {
	presenceKey, valueKey := fmt.Sprintf("has_value_%d", slot), fmt.Sprintf("arg_%d", slot)
	if fallback != nil {
		supplied := l.optionalField(dictionary, fmt.Sprintf("has_arg_%d", slot), "")
		group := l.GenerateUUID()
		l.emitIfStart(group, &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: supplied}, 4, "1", true)
		// Reuse the supplied envelope without touching an absent arg_n field.
		l.materializeYieldValue(&ir.AttachmentToken{Type: "ActionOutput", OutputUUID: dictionary})
		l.emitIfElse(group)
		defaultValue, err := l.lowerCallValue(fallback)
		if err != nil {
			return "", "", err
		}
		defaultPresence, err := l.optionalValuePresence(fallback, defaultValue)
		if err != nil {
			return "", "", err
		}
		l.optionalCarrier(defaultValue, defaultPresence, presenceKey, valueKey, "")
		selected := l.GenerateUUID()
		l.emitIfEnd(group, selected, "OptionalArgument")
		dictionary = selected
	}
	value, presence = l.decodeOptionalCarrier(dictionary, presenceKey, valueKey, name)
	return value, presence, nil
}
