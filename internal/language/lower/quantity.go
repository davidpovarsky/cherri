package lower

import (
	"fmt"
	"math"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func (l *Lowerer) lowerQuantity(call *syntax.CallExpr) (any, error) {
	var arguments, diagnostics = analysis.BindValueConstructorArguments(call, "qty")
	if len(diagnostics) != 0 {
		return nil, fmt.Errorf("%s", diagnostics[0].String())
	}
	for _, argument := range arguments {
		if argument.Parameter == 1 && !analysis.QuantityUnitLiteral(argument.Expression) {
			return nil, fmt.Errorf("%s at %s: qty() unit requires a text literal or an authored text template", analysis.CodeLiteralRequired, argument.Expression.NodeSpan())
		}
	}
	var quantity = &backend.QuantityValue{}
	for _, argument := range arguments {
		var value, err = l.lowerSemanticValue(argument.Expression)
		if err != nil {
			return nil, err
		}
		value = l.captureCallSemanticValue(value, l.callWritesAfter(call, argument.SourceIndex))
		if argument.Parameter == 0 {
			var numeric = value.Kind == backend.ValInt || value.Kind == backend.ValReference
			if value.Kind == backend.ValFloat {
				numeric = !math.IsNaN(value.FloatVal) && !math.IsInf(value.FloatVal, 0)
			}
			if value.Kind == backend.ValRawNative {
				_, numeric = value.RawNativeVal.(uint64)
			}
			if !numeric {
				return nil, fmt.Errorf("%s at %s: qty() magnitude expects Number", analysis.CodeArgumentType, argument.Expression.NodeSpan())
			}
			quantity.Magnitude = value
		} else {
			if value.Kind != backend.ValString && value.Kind != backend.ValTextSegments {
				return nil, fmt.Errorf("%s at %s: qty() unit expects Text", analysis.CodeArgumentType, argument.Expression.NodeSpan())
			}
			quantity.Unit = value
		}
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
