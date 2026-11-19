package main

import "encoding/json"

// Explicit native transformation dictionaries are source data. Keep their exact
// fields and presence alongside the typed view used by canonical consumers.
func (value Aggrandizement) MarshalPlist() (any, error) {
	if value.rawNativeFields != nil {
		return deepClonePlistMap(value.rawNativeFields), nil
	}
	type plainAggrandizement Aggrandizement
	return plainAggrandizement(value), nil
}

func (value Aggrandizement) MarshalJSON() ([]byte, error) {
	if value.rawNativeFields != nil {
		return json.Marshal(value.rawNativeFields)
	}
	type plainAggrandizement Aggrandizement
	return json.Marshal(plainAggrandizement(value))
}
