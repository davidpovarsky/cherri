package syntax

import (
	"math"
	"strconv"
)

// StaticListElement identifies a source-owned slot without evaluating either
// the List's elements or its index. Only a literal List and a nonnegative,
// integral Number literal within its authored cardinality are admitted. The
// caller must still establish that this element's semantic value is present;
// this helper proves neither opaque content nor Optional element transport.
func StaticListElement(expr Expression) (Expression, bool) {
	indexed, ok := expr.(*IndexExpr)
	if !ok {
		return nil, false
	}
	list, ok := indexed.Target.(*ListExpr)
	if !ok {
		return nil, false
	}
	literal, ok := indexed.Index.(*LiteralExpr)
	if !ok || literal.Type != TokenNumber {
		return nil, false
	}
	index, err := strconv.ParseFloat(literal.Value, 64)
	if err != nil || math.IsNaN(index) || math.IsInf(index, 0) || index < 0 ||
		index != math.Trunc(index) || index >= float64(len(list.Elements)) {
		return nil, false
	}
	element := list.Elements[int(index)]
	return element, element != nil
}
