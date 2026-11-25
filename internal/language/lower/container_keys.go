package lower

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// Key and value expressions share one authored evaluation order. A key must be
// captured before its own value can write it; an earlier value must also be
// captured before a later key can write it.
func (l *Lowerer) lowerSemanticMap(expr *syntax.MapExpr) (backend.SemanticValue, error) {
	if types.PrivateValueTransport(l.transportType(expr)) {
		return l.lowerTaggedMapSemantic(expr)
	}
	expressions := make([]syntax.Expression, 0, len(expr.Entries)*2)
	for _, entry := range expr.Entries {
		expressions = append(expressions, entry.Key, entry.Value)
	}
	laterWrites := l.semanticSiblingWrites(expressions)
	entries := make([]backend.DictEntry, 0, len(expr.Entries))
	for index, entry := range expr.Entries {
		if err := l.requirePresentTransport(entry.Key, "Dictionary key"); err != nil {
			return backend.SemanticValue{}, err
		}
		key, err := l.lowerSemanticMapKey(entry.Key)
		if err != nil {
			return backend.SemanticValue{}, err
		}
		key = l.captureCallSemanticValue(key, laterWrites[index*2])
		if err := l.requirePresentTransport(entry.Value, "Dictionary value"); err != nil {
			return backend.SemanticValue{}, err
		}
		value, err := l.lowerTypedInlineValue(entry.Value, l.transportType(entry.Value))
		if err != nil {
			return backend.SemanticValue{}, err
		}
		entries = append(entries, backend.DictEntry{
			Key: key.StrVal, KeySegments: key.Segments,
			Value: l.captureCallSemanticValue(value, laterWrites[index*2+1]),
		})
	}
	return backend.SemanticValue{Kind: backend.ValDict, DictVal: entries}, nil
}

// Computed Text keys use the same explicit text/reference contract as other
// text parameters. Ordinary scalar/literal key spelling remains unchanged.
// This does not reinterpret literal braces through the Legacy interpolator.
func (l *Lowerer) lowerSemanticMapKey(expr syntax.Expression) (backend.SemanticValue, error) {
	if _, interpolated := expr.(*syntax.FStringExpr); interpolated {
		return l.lowerSemanticValue(expr)
	}
	value, err := l.lowerExpression(expr)
	if err != nil {
		return backend.SemanticValue{}, err
	}
	semantic := l.toSemanticValue(value)
	if semantic.Kind == backend.ValReference {
		return backend.SemanticValue{
			Kind:     backend.ValTextSegments,
			Segments: []backend.TextSegment{{IsExpr: true, Reference: semantic.Ref}},
		}, nil
	}
	return backend.SemanticValue{Kind: backend.ValString, StrVal: fmt.Sprint(value)}, nil
}
