package lower

import (
	"fmt"
	"sort"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// A native List and an inline Dictionary array need different layouts when
// the child is itself a List. Compare every nested layout, not just the outer
// cell bit. Scalar typing remains the analyzer's independent responsibility.
func taggedStorageLayout(value types.Type, inline bool) string {
	value = presentType(value)
	if value == nil {
		return "native"
	}
	if types.ContainsJSONValue(value) {
		return "json-cell"
	}
	switch typed := value.(type) {
	case types.ListType:
		if types.TaggedListElements(typed) && (!inline || types.InlineDataNeedsCells(typed)) {
			return "list<cell<" + taggedStorageLayout(typed.Element, false) + ">>"
		}
		return "list<" + taggedStorageLayout(typed.Element, inline) + ">"
	case types.MapType:
		if types.TaggedMapValues(typed) {
			return "map<cell<" + taggedStorageLayout(typed.Value, false) + ">>"
		}
		return "map<" + taggedStorageLayout(typed.Value, true) + ">"
	case types.UnionType:
		set := map[string]bool{}
		for _, member := range typed.Members {
			set[taggedStorageLayout(member, inline)] = true
		}
		layouts := make([]string, 0, len(set))
		for layout := range set {
			layouts = append(layouts, layout)
		}
		sort.Strings(layouts)
		return strings.Join(layouts, "|")
	}
	return "native"
}

// Within one cell, an authored union member keeps its own kind. The cell
// supplies the common kind/presence wrapper; its payload still needs the exact
// selected member's recursive layout.
func taggedPayloadTarget(expected, actual types.Type) (types.Type, error) {
	target := presentType(expected)
	union, ok := target.(types.UnionType)
	if !ok {
		return expected, nil
	}
	var selected types.Type
	for _, member := range union.Members {
		candidate := presentType(member)
		observed := presentType(actual)
		if candidate == nil || observed == nil || candidate.Kind() != observed.Kind() {
			continue
		}
		if selected != nil && taggedStorageLayout(selected, false) != taggedStorageLayout(member, false) {
			return nil, fmt.Errorf("E_TARGET_VALUE_SHAPE: union payload has ambiguous structured storage")
		}
		selected = member
	}
	if selected == nil {
		return actual, nil
	}
	return selected, nil
}

func (l *Lowerer) lowerTypedValueCell(expr syntax.Expression, expected types.Type) (any, error) {
	if literal, ok := expr.(*syntax.LiteralExpr); ok && literal.Type == syntax.TokenNone {
		return l.emitValueCell("none", nil, "0"), nil
	}
	target, err := taggedPayloadTarget(expected, l.transportType(expr))
	if err != nil {
		return nil, err
	}
	if types.PrivateValueTransport(l.transportType(expr)) && !types.PrivateValueTransport(target) {
		return nil, fmt.Errorf("E_TARGET_VALUE_SHAPE: a cell payload cannot erase its private structured layout at %s", expr.NodeSpan())
	}
	value, err := l.lowerTypedRuntimeValue(expr, target)
	if err != nil {
		return nil, err
	}
	return l.finishValueCell(expr, value)
}

func (l *Lowerer) lowerTypedList(expr *syntax.ListExpr, target types.ListType, inline bool) (any, error) {
	items := make([]any, 0, len(expr.Elements))
	laterWrites := l.semanticSiblingWrites(expr.Elements)
	cellElements := types.TaggedListElements(target) && (!inline || types.InlineDataNeedsCells(target))
	for i, element := range expr.Elements {
		var value any
		var err error
		if cellElements {
			value, err = l.lowerTypedValueCell(element, target.Element)
		} else {
			if err = l.requirePresentTransport(element, "List element"); err != nil {
				return nil, err
			}
			if inline {
				value, err = l.lowerTypedInlineValue(element, target.Element)
			} else {
				value, err = l.lowerTypedRuntimeValue(element, target.Element)
			}
			if err == nil {
				value, err = l.captureCallArgumentValue(element, value, laterWrites[i])
			}
		}
		if err != nil {
			return nil, err
		}
		items = append(items, value)
	}
	return items, nil
}

func (l *Lowerer) lowerTypedMapSemantic(expr *syntax.MapExpr, target types.MapType) (backend.SemanticValue, error) {
	expressions := make([]syntax.Expression, 0, len(expr.Entries)*2)
	for _, entry := range expr.Entries {
		expressions = append(expressions, entry.Key, entry.Value)
	}
	laterWrites := l.semanticSiblingWrites(expressions)
	entries := make([]backend.DictEntry, 0, len(expr.Entries))
	for i, entry := range expr.Entries {
		if err := l.requirePresentTransport(entry.Key, "Dictionary key"); err != nil {
			return backend.SemanticValue{}, err
		}
		key, err := l.lowerSemanticMapKey(entry.Key)
		if err != nil {
			return backend.SemanticValue{}, err
		}
		key = l.captureCallSemanticValue(key, laterWrites[i*2])
		var value backend.SemanticValue
		if types.TaggedMapValues(target) {
			cell, cellErr := l.lowerTypedValueCell(entry.Value, target.Value)
			if cellErr != nil {
				return backend.SemanticValue{}, cellErr
			}
			value = l.toSemanticValue(cell)
		} else {
			if err = l.requirePresentTransport(entry.Value, "Dictionary value"); err != nil {
				return backend.SemanticValue{}, err
			}
			value, err = l.lowerTypedInlineValue(entry.Value, target.Value)
			if err != nil {
				return backend.SemanticValue{}, err
			}
			value = l.captureCallSemanticValue(value, laterWrites[i*2+1])
		}
		entries = append(entries, backend.DictEntry{Key: key.StrVal, KeySegments: key.Segments, Value: value})
	}
	return backend.SemanticValue{Kind: backend.ValDict, DictVal: entries}, nil
}

func (l *Lowerer) lowerTypedInlineValue(expr syntax.Expression, expected types.Type) (backend.SemanticValue, error) {
	// A literal native Dictionary array does not need a uniform runtime
	// cell payload layout. Preserve its per-element semantic kinds, including
	// heterogeneous nested arrays and native-looking keys in authored Maps.
	if !types.InlineDataNeedsCells(expected) && !types.InlineDataNeedsCells(l.transportType(expr)) {
		switch expr.(type) {
		case *syntax.ListExpr, *syntax.MapExpr:
			return l.lowerSemanticValue(expr)
		}
	}
	target, err := taggedPayloadTarget(expected, l.transportType(expr))
	if err != nil {
		return backend.SemanticValue{}, err
	}
	switch source := expr.(type) {
	case *syntax.ListExpr:
		if list, ok := presentType(target).(types.ListType); ok {
			value, err := l.lowerTypedList(source, list, true)
			if err != nil {
				return backend.SemanticValue{}, err
			}
			return l.toSemanticValue(value), nil
		}
	case *syntax.MapExpr:
		if dictionary, ok := presentType(target).(types.MapType); ok {
			return l.lowerTypedMapSemantic(source, dictionary)
		}
	}
	actual := l.transportType(expr)
	if (types.PrivateValueTransport(actual) || types.PrivateValueTransport(target)) &&
		taggedStorageLayout(actual, false) != taggedStorageLayout(target, true) {
		return backend.SemanticValue{}, fmt.Errorf("E_TARGET_VALUE_SHAPE: dynamic native value requires an explicit Dictionary-array conversion at %s", expr.NodeSpan())
	}
	return l.lowerSemanticValue(expr)
}
