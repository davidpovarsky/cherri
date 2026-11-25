package schema

import (
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// A source-enrolled local loop may carry a checked dynamic container as one
// Dictionary payload. Unknown native content does not prove recursive shape.
// Literal children retain their own types, including explicit empty arrays.
func loopResultLocalPayloadSupported(expr syntax.Expression, expressionTypes map[syntax.Expression]types.Type) bool {
	if literal, okay := expr.(*syntax.LiteralExpr); okay && literal.Type == syntax.TokenNone {
		return true
	}
	value := expressionTypes[expr]
	if value == nil {
		return false
	}
	if optional, okay := value.(types.OptionalType); okay {
		value = optional.Inner
	}
	switch literal := expr.(type) {
	case *syntax.MapExpr:
		// Preserve the established semantic Dictionary literal codec. Its
		// own child checks and Optional lowering still establish presence.
		return value.Kind() == types.KindMap
	case *syntax.ListExpr:
		if value.Kind() != types.KindList {
			return false
		}
		for _, element := range literal.Elements {
			if !loopResultLocalPayloadSupported(element, expressionTypes) {
				return false
			}
		}
		return true
	}
	if value.Kind() == types.KindUnknown {
		// As before, only an independently carried None can pass lowering's
		// presence check. A bare Unknown cannot acquire a payload shape.
		return types.MayBeAbsent(expressionTypes[expr])
	}
	return loopResultKnownPayloadType(value)
}

func loopResultKnownPayloadType(value types.Type) bool {
	if value == nil {
		return false
	}
	switch kind := value.(type) {
	case types.OptionalType:
		return loopResultKnownPayloadType(kind.Inner)
	case types.UnionType:
		for _, member := range kind.Members {
			if !loopResultKnownPayloadType(member) {
				return false
			}
		}
		return len(kind.Members) != 0
	case types.ListType:
		return loopResultKnownPayloadType(kind.Element)
	case types.MapType:
		return kind.Key.Equals(types.Text) && loopResultKnownPayloadType(kind.Value)
	}
	return value.Equals(types.Text) || value.Equals(types.Number) || value.Equals(types.Bool) ||
		value.Kind() == types.KindJsonNull || value.Kind() == types.KindJsonValue
}
