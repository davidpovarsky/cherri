package main

// MarshalPlist preserves explicit empty authored array fields without changing
// the historical omission shape of Legacy and raw/import array parameters.
// Both paths retain the same native WFArrayParameterState representation.
func (value WFArrayValue) MarshalPlist() (any, error) {
	if !value.explicitEmpty {
		type plainArrayValue WFArrayValue
		return plainArrayValue(value), nil
	}
	items := value.Value
	if items == nil {
		items = []WFDictionaryFieldValueItem{}
	}
	return map[string]any{
		"WFSerializationType": value.WFSerializationType,
		"Value":               items,
	}, nil
}
