package lower

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// A cell is one native Dictionary, even for None, null, false and empty data.
// Its identity is granted by checked source/types, never by inspecting arbitrary
// dictionary keys. The payload is ordinary typed native data, not a plist null.
const valueCellMarker = "_cherri_value_cell_v1"
const valueCellKind = "kind"
const valueCellPresence = "has_value"
const valueCellPayload = "value"

func presentType(value types.Type) types.Type {
	if optional, ok := value.(types.OptionalType); ok {
		return presentType(optional.Inner)
	}
	return value
}

func (l *Lowerer) transportType(expr syntax.Expression) types.Type {
	if name, ok := expr.(*syntax.IdentExpr); ok {
		if binding, found := l.bindings[name.Name]; found {
			if binding.TaggedTransport != nil {
				return binding.TaggedTransport
			}
			if binding.Type != nil {
				return binding.Type
			}
		}
	}
	return l.exprTypes[expr]
}

func (l *Lowerer) lowerTypedRuntimeValue(expr syntax.Expression, target types.Type) (any, error) {
	if literal, okay := expr.(*syntax.LiteralExpr); okay && literal.Type == syntax.TokenNone && types.MayBeAbsent(target) {
		return nil, nil
	}
	actual := l.transportType(expr)
	if types.PrivateValueTransport(target) && !types.PrivateValueTransport(actual) {
		if list, ok := expr.(*syntax.ListExpr); ok && types.TaggedListElements(target) {
			return l.lowerTaggedList(list)
		}
		if dictionary, ok := expr.(*syntax.MapExpr); ok && types.TaggedMapValues(target) {
			return l.lowerTaggedMap(dictionary)
		}
		return nil, fmt.Errorf("E_TARGET_VALUE_SHAPE: conversion from %v to private structured %v requires an explicit value codec at %s", actual, target, expr.NodeSpan())
	}
	if target != nil && types.PrivateValueTransport(target) &&
		(types.TaggedListElements(target) != types.TaggedListElements(actual) ||
			types.TaggedMapValues(target) != types.TaggedMapValues(actual)) {
		return nil, fmt.Errorf("E_TARGET_VALUE_SHAPE: incompatible private structured representations at %s", expr.NodeSpan())
	}
	return l.lowerRuntimeLiteral(expr)
}

func (l *Lowerer) emitValueCell(kind string, value, presence any) *ir.AttachmentToken {
	emit := func(present bool) *ir.AttachmentToken {
		flag := "0"
		fields := map[string]any{valueCellMarker: "1", valueCellKind: kind}
		if present {
			flag = "1"
			if kind != "null" {
				fields[valueCellPayload] = l.functionEnvelopeValue(value)
			}
		} else {
			fields[valueCellKind] = "none"
		}
		fields[valueCellPresence] = flag
		id := l.GenerateUUID()
		l.emitAction(&ir.NativeActionNode{NodeID: id,
			AppleIdentifier: "is.workflow.actions.dictionary", OutputUUID: id,
			OutputName: "ValueCell", Parameters: map[string]any{"WFItems": fields}})
		return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: id, OutputName: "ValueCell"}
	}
	if flag, known := presence.(string); known {
		return emit(flag == "1")
	}
	group, result := l.GenerateUUID(), l.GenerateUUID()
	l.emitIfStart(group, l.materializeToAttachment(presence), 4, "1", true)
	emit(true)
	l.emitIfElse(group)
	emit(false)
	l.emitIfEnd(group, result, "ValueCell")
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: result, OutputName: "ValueCell"}
}

func valueKind(value types.Type) string {
	value = presentType(value)
	if value == nil {
		return "value"
	}
	if _, union := value.(types.UnionType); union && types.ContainsJSONValue(value) {
		return "json"
	}
	switch value.Kind() {
	case types.KindJsonNull:
		return "null"
	case types.KindJsonValue:
		return "json"
	case types.KindList:
		return "list"
	case types.KindMap, types.KindRecord:
		return "map"
	}
	switch {
	case value.Equals(types.Bool):
		return "bool"
	case value.Equals(types.Number):
		return "number"
	case value.Equals(types.Text):
		return "text"
	}
	return "value"
}

func (l *Lowerer) lowerValueCell(expr syntax.Expression) (*ir.AttachmentToken, error) {
	value, err := l.lowerRuntimeLiteral(expr)
	if err != nil {
		return nil, err
	}
	presence, err := l.optionalValuePresence(expr, value)
	if err != nil {
		return nil, err
	}
	kind := valueKind(l.transportType(expr))
	if kind == "null" || kind == "json" {
		// JsonValue already uses the same cell schema, including present null.
		token, okay := value.(*ir.AttachmentToken)
		if !okay {
			return nil, fmt.Errorf("E_TARGET_VALUE_SHAPE: JsonValue lost its source-owned value cell")
		}
		if flag, known := presence.(string); known && flag == "1" {
			return token, nil
		}
		group, result := l.GenerateUUID(), l.GenerateUUID()
		l.emitIfStart(group, l.materializeToAttachment(presence), 4, "1", true)
		l.materializeYieldValue(token)
		l.emitIfElse(group)
		l.emitValueCell("none", nil, "0")
		l.emitIfEnd(group, result, "ValueCell")
		return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: result}, nil
	}
	return l.emitValueCell(kind, value, presence), nil
}

func (l *Lowerer) lowerTaggedList(expr *syntax.ListExpr) (any, error) {
	items := make([]any, 0, len(expr.Elements))
	// Each cell consumes its payload now, before any later authored element
	// can mutate an alias. Deferred references never cross that source point.
	for _, element := range expr.Elements {
		cell, err := l.lowerValueCell(element)
		if err != nil {
			return nil, err
		}
		items = append(items, cell)
	}
	return items, nil
}

func (l *Lowerer) lowerTaggedMapSemantic(expr *syntax.MapExpr) (backend.SemanticValue, error) {
	entries := make([]backend.DictEntry, 0, len(expr.Entries))
	expressions := make([]syntax.Expression, 0, len(expr.Entries)*2)
	for _, entry := range expr.Entries {
		expressions = append(expressions, entry.Key, entry.Value)
	}
	laterWrites := l.semanticSiblingWrites(expressions)
	for index, entry := range expr.Entries {
		if err := l.requirePresentTransport(entry.Key, "Dictionary key"); err != nil {
			return backend.SemanticValue{}, err
		}
		key, err := l.lowerSemanticMapKey(entry.Key)
		if err != nil {
			return backend.SemanticValue{}, err
		}
		// A value expression may mutate its own computed key.
		key = l.captureCallSemanticValue(key, laterWrites[index*2])
		cell, err := l.lowerValueCell(entry.Value)
		if err != nil {
			return backend.SemanticValue{}, err
		}
		entries = append(entries, backend.DictEntry{Key: key.StrVal, KeySegments: key.Segments,
			Value: l.toSemanticValue(cell)})
	}
	return backend.SemanticValue{Kind: backend.ValDict, DictVal: entries}, nil
}

func (l *Lowerer) lowerTaggedMap(expr *syntax.MapExpr) (*ir.AttachmentToken, error) {
	value, err := l.lowerTaggedMapSemantic(expr)
	if err != nil {
		return nil, err
	}
	id := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{NodeID: id, AppleIdentifier: "is.workflow.actions.dictionary",
		OutputUUID: id, OutputName: "Dictionary", Parameters: map[string]any{"WFItems": value}})
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: id, OutputName: "Dictionary"}, nil
}

func (l *Lowerer) taggedIndexHasPresence(expr syntax.Expression) bool {
	index, ok := expr.(*syntax.IndexExpr)
	return ok && (types.TaggedListElements(l.transportType(index.Target)) ||
		types.TaggedMapValues(l.transportType(index.Target)))
}

func (l *Lowerer) decodeValueCell(cell string, element types.Type) (value, presence string) {
	kind := valueKind(element)
	if kind == "null" || kind == "json" {
		return cell, l.optionalField(cell, valueCellPresence, "")
	}
	return l.decodeOptionalCarrier(cell, valueCellPresence, valueCellPayload, "")
}

func (l *Lowerer) taggedForItem(iterable syntax.Expression, nativeItem string) (any, any, types.Type, bool) {
	list, ok := presentType(l.transportType(iterable)).(types.ListType)
	if !ok || !types.TaggedListElements(list) || l.localLoopResultElement(iterable) != nil {
		return nil, nil, nil, false
	}
	cell := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{NodeID: cell, AppleIdentifier: "is.workflow.actions.getvariable",
		OutputUUID: cell, Parameters: map[string]any{"WFVariable": &ir.AttachmentToken{Type: "Variable", OutputName: nativeItem}}})
	value, presence := l.decodeValueCell(cell, list.Element)
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: value},
		&ir.AttachmentToken{Type: "ActionOutput", OutputUUID: presence}, list.Element, true
}

func (l *Lowerer) rememberCellIndexPresence(index syntax.Expression, presence any) {
	if l.optionalExpressionPresence == nil {
		l.optionalExpressionPresence = make(map[syntax.Expression]any)
	}
	l.optionalExpressionPresence[index] = presence
}

func (l *Lowerer) lowerTaggedMapIndex(index *syntax.IndexExpr, target, key any) (*ir.AttachmentToken, error) {
	dictionary, okay := presentType(l.transportType(index.Target)).(types.MapType)
	if !okay || !types.TaggedMapValues(dictionary) {
		return nil, fmt.Errorf("E_TARGET_VALUE_SHAPE: Dictionary cell lookup lacks source-owned storage")
	}
	input := l.materializeToAttachment(target)
	cell := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{NodeID: cell, AppleIdentifier: "is.workflow.actions.getvalueforkey", OutputUUID: cell,
		Parameters: map[string]any{"WFInput": input, "WFDictionaryKey": key, "WFGetDictionaryValueType": "Value"}})
	// A cell always has explicit metadata keys, even when its payload is None.
	// Test only the existence of this source-owned cell, never payload truthiness.
	keys := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{NodeID: keys, AppleIdentifier: "is.workflow.actions.getvalueforkey", OutputUUID: keys,
		Parameters: map[string]any{"WFInput": &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: cell}, "WFGetDictionaryValueType": "All Keys"}})
	count := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{NodeID: count, AppleIdentifier: "is.workflow.actions.count", OutputUUID: count,
		Parameters: map[string]any{"Input": &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: keys}, "WFCountType": "Items"}})
	presenceName := l.freshLexicalName("cellPresence_" + l.GenerateUUID())
	presenceToken := l.storeOptionalPresence(presenceName, "0")
	group, result := l.GenerateUUID(), l.GenerateUUID()
	l.emitIfStart(group, &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: count}, 3, 1.0, true)
	value, presence := l.decodeValueCell(cell, dictionary.Value)
	l.storeOptionalPresence(presenceName, &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: presence})
	l.materializeYieldValue(&ir.AttachmentToken{Type: "ActionOutput", OutputUUID: value})
	l.emitIfElse(group)
	l.materializeYieldValue(nil)
	l.emitIfEnd(group, result, "")
	l.rememberCellIndexPresence(index, presenceToken)
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: result}, nil
}

func isNullLiteral(expr syntax.Expression) bool {
	literal, okay := expr.(*syntax.LiteralExpr)
	return okay && literal.Type == syntax.TokenNull
}

func (l *Lowerer) lowerNullComparison(expr syntax.Expression) (any, bool, error) {
	binary, okay := expr.(*syntax.BinaryExpr)
	if !okay || (binary.Op != syntax.TokenEqual && binary.Op != syntax.TokenNotEqual) {
		return nil, false, nil
	}
	var operand syntax.Expression
	if isNullLiteral(binary.Right) {
		operand = binary.Left
	} else if isNullLiteral(binary.Left) {
		operand = binary.Right
	} else {
		return nil, false, nil
	}
	value, err := l.lowerRuntimeLiteral(operand)
	if err != nil {
		return nil, true, err
	}
	kind := valueKind(l.transportType(operand))
	if kind != "json" && kind != "null" {
		return binary.Op == syntax.TokenNotEqual, true, nil
	}
	token := l.materializeToAttachment(value)
	if token == nil {
		return binary.Op == syntax.TokenNotEqual, true, nil
	}
	presence, err := l.optionalValuePresence(operand, value)
	if err != nil {
		return nil, true, err
	}
	presenceGroup := ""
	if flag, known := presence.(string); known {
		if flag == "0" {
			return binary.Op == syntax.TokenNotEqual, true, nil
		}
	} else {
		presenceGroup = l.GenerateUUID()
		l.emitIfStart(presenceGroup, l.materializeToAttachment(presence), 4, "1", true)
	}
	field := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{NodeID: field, AppleIdentifier: "is.workflow.actions.getvalueforkey", OutputUUID: field,
		Parameters: map[string]any{"WFInput": token, "WFDictionaryKey": valueCellKind, "WFGetDictionaryValueType": "Value"}})
	group, result := l.GenerateUUID(), l.GenerateUUID()
	code := 4
	if binary.Op == syntax.TokenNotEqual {
		code = 5
	}
	l.emitIfStartTyped(group, &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: field}, code, "null", nil, backend.OperandString, true)
	l.emitAction(&ir.NativeActionNode{NodeID: l.GenerateUUID(), AppleIdentifier: "is.workflow.actions.number", Parameters: map[string]any{"WFNumberActionNumber": 1.0}})
	l.emitIfElse(group)
	l.emitAction(&ir.NativeActionNode{NodeID: l.GenerateUUID(), AppleIdentifier: "is.workflow.actions.number", Parameters: map[string]any{"WFNumberActionNumber": 0.0}})
	l.emitIfEnd(group, result, "")
	if presenceGroup != "" {
		l.emitIfElse(presenceGroup)
		absentResult := 0.0
		if binary.Op == syntax.TokenNotEqual {
			absentResult = 1.0
		}
		l.emitAction(&ir.NativeActionNode{NodeID: l.GenerateUUID(), AppleIdentifier: "is.workflow.actions.number", Parameters: map[string]any{"WFNumberActionNumber": absentResult}})
		result = l.GenerateUUID()
		l.emitIfEnd(presenceGroup, result, "")
	}
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: result}, true, nil
}

func containsNullPredicate(expr syntax.Expression) bool {
	switch value := expr.(type) {
	case *syntax.BinaryExpr:
		return ((value.Op == syntax.TokenEqual || value.Op == syntax.TokenNotEqual) &&
			(isNullLiteral(value.Left) || isNullLiteral(value.Right))) ||
			containsNullPredicate(value.Left) || containsNullPredicate(value.Right)
	case *syntax.UnaryExpr:
		return containsNullPredicate(value.Operand)
	}
	return false
}

func (l *Lowerer) validateTaggedNativeBoundary(name string, call *syntax.CallExpr) error {
	if l.functions[name] != nil {
		return nil
	}
	// Exact imported native metadata retains its independent opaque route.
	if l.hasImportProvenance {
		return nil
	}
	if name == "count" && l.actions[name] == nil && call.PrimaryArg != nil && len(call.NamedArgs) == 0 {
		if action, found := l.registry.LookupAction(name); found && action.AppleIdentifier == "is.workflow.actions.count" {
			if _, list := presentType(l.transportType(call.PrimaryArg)).(types.ListType); list {
				return nil
			}
		}
	}
	arguments := []syntax.Expression{call.PrimaryArg}
	for _, argument := range call.NamedArgs {
		arguments = append(arguments, argument.Value)
	}
	for _, argument := range arguments {
		if argument == nil {
			continue
		}
		if len(name) > 7 && name[:7] == "native." {
			// Literal metadata is never enrolled as a value cell. The analyzer
			// separately rejects raw null, and dynamic private references cannot
			// cross this boundary just because they occur inside a literal.
			if err := l.validateTaggedRawReference(argument); err != nil {
				return err
			}
			continue
		}
		if types.PrivateValueTransport(l.transportType(argument)) {
			return fmt.Errorf("E_TARGET_VALUE_SHAPE: %s requires an explicit native/JSON conversion for structured values at %s", name, argument.NodeSpan())
		}
	}
	if types.PrivateValueTransport(l.exprTypes[call]) {
		return fmt.Errorf("E_TARGET_VALUE_SHAPE: native %s cannot manufacture a compiler-owned structured result at %s", name, call.Span)
	}
	return nil
}

func (l *Lowerer) validateTaggedRawReference(expr syntax.Expression) error {
	switch value := expr.(type) {
	case *syntax.LiteralExpr:
		if value.Type == syntax.TokenNull {
			return fmt.Errorf("E_NATIVE_JSON_NULL: raw metadata has no plist null")
		}
		return nil
	case *syntax.MapExpr:
		for _, entry := range value.Entries {
			if err := l.validateTaggedRawReference(entry.Key); err != nil {
				return err
			}
			if err := l.validateTaggedRawReference(entry.Value); err != nil {
				return err
			}
		}
		return nil
	case *syntax.ListExpr:
		for _, element := range value.Elements {
			if err := l.validateTaggedRawReference(element); err != nil {
				return err
			}
		}
		return nil
	}
	if types.PrivateValueTransport(l.transportType(expr)) {
		return fmt.Errorf("E_TARGET_VALUE_SHAPE: raw native metadata cannot consume a private structured value at %s", expr.NodeSpan())
	}
	return nil
}

func (l *Lowerer) validateTaggedOperator(expr syntax.Expression) error {
	switch value := expr.(type) {
	case *syntax.BinaryExpr:
		if types.PrivateValueTransport(l.transportType(value.Left)) || types.PrivateValueTransport(l.transportType(value.Right)) {
			return fmt.Errorf("E_TARGET_VALUE_SHAPE: this structured operator requires an explicit value operation at %s", value.Span)
		}
	case *syntax.UnaryExpr:
		if types.PrivateValueTransport(l.transportType(value.Operand)) {
			return fmt.Errorf("E_TARGET_VALUE_SHAPE: this structured unary operator requires an explicit value operation at %s", value.Span)
		}
	case *syntax.FStringExpr:
		for _, part := range value.Parts {
			if part.IsExpr && types.PrivateValueTransport(l.transportType(part.Expr)) {
				return fmt.Errorf("E_TARGET_VALUE_SHAPE: structured interpolation requires an explicit text/JSON codec at %s", part.Expr.NodeSpan())
			}
		}
	}
	return nil
}
