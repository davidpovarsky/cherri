package main

import "github.com/electrikmilk/cherri/internal/language/backend"

func quantityActionArguments(quantity *backend.QuantityValue) []actionArgument {
	if quantity == nil {
		return nil
	}
	return []actionArgument{
		convertCallArgToActionArg(backend.CallArgument{Value: quantity.Magnitude}),
		convertCallArgToActionArg(backend.CallArgument{Value: quantity.Unit}),
	}
}

func encodeQuantityValue(quantity *backend.QuantityValue) any {
	if quantity == nil {
		return nil
	}
	return makeQuantityFieldValue(quantityActionArguments(quantity))
}
