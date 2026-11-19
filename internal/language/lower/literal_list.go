package lower

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// lowerListDictionary retains the AST's explicit Dictionary identity. Native
// field names in user data cannot turn an authored MapExpr into a serialized
// List row or text token. Nested values use the existing semantic backend codec.
func (l *Lowerer) lowerListDictionary(expr *syntax.MapExpr) (*ir.AttachmentToken, error) {
	value, err := l.lowerSemanticValue(expr)
	if err != nil {
		return nil, err
	}
	uuid := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID:          uuid,
		AppleIdentifier: "is.workflow.actions.dictionary",
		OutputUUID:      uuid,
		OutputName:      "Dictionary",
		Parameters:      map[string]interface{}{"WFItems": value},
	})
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid, OutputName: "Dictionary"}, nil
}

// Dictionary array fields remain inline data. Only elements of a native List
// are materialized as producers, so existing nested Dictionary codecs and exact
// import payloads retain their own representation.
func (l *Lowerer) lowerListExpression(expr *syntax.ListExpr, nativeItems bool) (interface{}, error) {
	list := make([]interface{}, len(expr.Elements))
	for i, element := range expr.Elements {
		if nativeItems && l.isNativeListElement(element) {
			return nil, fmt.Errorf("E_NESTED_NATIVE_LIST: nested List-valued element at item %d has no verified native content-row encoding; preserve nested arrays in Dictionary fields", i)
		}
		var value interface{}
		var err error
		if dictionary, ok := element.(*syntax.MapExpr); ok && nativeItems {
			value, err = l.lowerListDictionary(dictionary)
		} else if nested, ok := element.(*syntax.ListExpr); ok && !nativeItems {
			value, err = l.lowerListExpression(nested, false)
		} else {
			value, err = l.lowerExpression(element)
		}
		if err != nil {
			return nil, err
		}
		list[i] = value
	}
	return list, nil
}

// Raw native metadata is inline plist data, not an authored runtime List.
// Reuse the existing recursive Dictionary-array path without creating actions.
func (l *Lowerer) lowerInlineNativeData(expr syntax.Expression) (interface{}, error) {
	if list, ok := expr.(*syntax.ListExpr); ok {
		return l.lowerListExpression(list, false)
	}
	return l.lowerExpression(expr)
}

// Runtime Dictionary literals retain their AST identity before entering the
// shared value codec. Raw action metadata still uses lowerInlineNativeData and
// lowerExpression, so reserved native fields remain exact source data there.
func (l *Lowerer) lowerRuntimeLiteral(expr syntax.Expression) (interface{}, error) {
	if dictionary, ok := expr.(*syntax.MapExpr); ok {
		return l.lowerListDictionary(dictionary)
	}
	return l.lowerExpression(expr)
}

// A List-valued variable, call, value block, or optional has the same native
// content-row limitation as a nested literal. Checking only AST ListExpr nodes
// lets inference silently bypass that boundary.
func (l *Lowerer) isNativeListElement(expr syntax.Expression) bool {
	if _, literal := expr.(*syntax.ListExpr); literal {
		return true
	}
	var inferred = l.exprTypes[expr]
	// A broad/unknown annotation can erase the analyzed identifier type while
	// its actual lowered initializer still establishes a List-shaped binding.
	if inferred == nil || inferred.Kind() == types.KindUnknown || inferred.Kind() == types.KindAnyContent {
		if identifier, ok := expr.(*syntax.IdentExpr); ok {
			if binding, found := l.bindings[identifier.Name]; found {
				inferred = binding.Type
			}
		}
	}
	return nativeListElementType(inferred)
}

func nativeListElementType(inferred types.Type) bool {
	if inferred == nil {
		return false
	}
	switch typed := inferred.(type) {
	case types.OptionalType:
		return nativeListElementType(typed.Inner)
	case types.UnionType:
		for _, member := range typed.Members {
			if nativeListElementType(member) {
				return true
			}
		}
	}
	return inferred.Kind() == types.KindList || inferred.Name() == "Array" || inferred.Name() == "List"
}
