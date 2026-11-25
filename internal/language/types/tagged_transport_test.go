package types

import "testing"

func TestTaggedValueLayouts(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		value                     Type
		list, dictionary, private bool
	}{
		{"plain_list", NewList(Number), false, false, false},
		{"nested_list", NewList(NewList(Number)), true, false, true},
		{"optional_list", NewList(OptionalType{Inner: Bool}), true, false, true},
		{"null_list", NewList(JsonNull), true, false, true},
		{"plain_dictionary_array", NewMap(Text, NewList(NewList(Number))), false, false, false},
		{"optional_dictionary", NewMap(Text, OptionalType{Inner: Bool}), false, true, true},
		{"null_dictionary", NewMap(Text, JsonValue), false, true, true},
		{"null", JsonNull, false, false, true},
		{"opaque", AnyContent, false, false, false},
		{"unknown", Unknown, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if TaggedListElements(tc.value) != tc.list || TaggedMapValues(tc.value) != tc.dictionary ||
				PrivateValueTransport(tc.value) != tc.private {
				t.Fatalf("wrong transport for %s", tc.value)
			}
		})
	}
	if !DefinitelyPresent(JsonNull) || !JsonNull.AssignableTo(OptionalType{Inner: JsonValue}) ||
		JsonNull.AssignableTo(AnyContent) || JsonValue.AssignableTo(AnyContent) {
		t.Fatal("JSON null presence or private/native boundary changed")
	}
}
