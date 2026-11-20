package lower

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func optionalFunctionParameter(annotation syntax.TypeAnnotation) bool {
	_, optional := annotation.(*syntax.OptionalTypeAnnotation)
	return optional
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
