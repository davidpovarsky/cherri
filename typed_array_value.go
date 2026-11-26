package main

// MarshalPlist keeps the required Value array for every compiler-generated
// WFArrayParameterState, including a nil or empty item slice. Native Dictionary
// deserialization treats an omitted Value as missing state, not an empty array.
// Opaque raw/import maps bypass this typed encoder and retain their source shape.
func (value WFArrayValue) MarshalPlist() (any, error) {
	items := value.Value
	if items == nil {
		items = []WFDictionaryFieldValueItem{}
	}
	return map[string]any{
		"WFSerializationType": value.WFSerializationType,
		"Value":               items,
	}, nil
}
