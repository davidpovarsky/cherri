package lower

import (
	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func (l *Lowerer) lowerQuantity(call *syntax.CallExpr) (any, error) {
	var arguments []backend.SemanticValue
	if call.PrimaryArg != nil {
		var value, err = l.lowerSemanticValue(call.PrimaryArg)
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, l.captureCallSemanticValue(value, l.callWritesAfter(call, -1)))
	}
	for index, argument := range call.NamedArgs {
		var value, err = l.lowerSemanticValue(argument.Value)
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, l.captureCallSemanticValue(value, l.callWritesAfter(call, index)))
	}
	var quantity = &backend.QuantityValue{
		Magnitude: backend.SemanticValue{Kind: backend.ValString},
		Unit:      backend.SemanticValue{Kind: backend.ValString},
	}
	if len(arguments) > 0 {
		quantity.Magnitude = arguments[0]
	}
	if len(arguments) > 1 {
		quantity.Unit = arguments[1]
	}
	return quantity, nil
}

func isQuantityValue(value any) bool {
	if _, ok := value.(*backend.QuantityValue); ok {
		return true
	}
	var native, ok = value.(map[string]interface{})
	return ok && native["WFSerializationType"] == "WFQuantityFieldValue"
}

func (l *Lowerer) captureQuantityValue(quantity *backend.QuantityValue, writes map[string]bool) *backend.QuantityValue {
	if quantity == nil || len(writes) == 0 {
		return quantity
	}
	return &backend.QuantityValue{
		Magnitude: l.captureCallSemanticValue(quantity.Magnitude, writes),
		Unit:      l.captureCallSemanticValue(quantity.Unit, writes),
	}
}
