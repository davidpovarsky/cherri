package main

import (
	"sort"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

// encodeTypedLiteralParam carries authored semantic kinds through the canonical
// adapter. Ordinary text and keys never enter Legacy's inline-reference parser;
// f-strings and references already carry their explicit semantic identities.
func encodeTypedLiteralParam(value backend.SemanticValue) any {
	switch value.Kind {
	case backend.ValDict:
		return WFDictionaryFieldValue{
			WFSerializationType: "WFDictionaryFieldValue",
			Value: WFExplicitDictionaryFieldValueWrapper{
				WFDictionaryFieldValueItems: encodeTypedLiteralDictionaryItems(value.DictVal),
			},
		}
	case backend.ValList:
		items := make([]WFDictionaryFieldValueItem, 0, len(value.ListVal))
		for _, item := range value.ListVal {
			items = append(items, EncodeDictionaryItemShared("", encodeTypedLiteralParam(item)))
		}
		array := makeArrayValue(items)
		array.explicitEmpty = true
		return array
	case backend.ValString:
		return WFTextTokenString{
			WFSerializationType: "WFTextTokenString",
			Value:               WFTextTokenStringValue{String: value.StrVal},
		}
	case backend.ValReference:
		return EncodeReferenceAttachment(ReferenceToDescriptor(value.Ref), "WFTextTokenAttachment")
	case backend.ValTextSegments:
		return EncodeTextSegmentsShared(value.Segments)
	default:
		return ConvertSemanticValueToParam(value)
	}
}

func dictionaryHasComputedKeys(entries []backend.DictEntry) bool {
	for _, entry := range entries {
		if len(entry.KeySegments) > 0 {
			return true
		}
	}
	return false
}

// Keep canonical sorted order for literal keys. Computed keys can collide with
// each other or a literal key, so their rows retain authored source order.
func encodeTypedLiteralDictionaryItems(entries []backend.DictEntry) []WFDictionaryFieldValueItem {
	ordered := append([]backend.DictEntry(nil), entries...)
	if !dictionaryHasComputedKeys(ordered) {
		sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Key < ordered[j].Key })
	}
	items := make([]WFDictionaryFieldValueItem, 0, len(ordered))
	for _, entry := range ordered {
		item := EncodeDictionaryItemShared("", encodeTypedLiteralParam(entry.Value))
		key := entry.Key
		if len(entry.KeySegments) > 0 {
			encoded := EncodeTextSegmentsShared(entry.KeySegments)
			if literal, ok := encoded.(string); ok {
				key = literal
			} else {
				item.WFKey = encoded
			}
		}
		if item.WFKey == nil {
			item.WFKey = WFTextTokenString{
				WFSerializationType: "WFTextTokenString",
				Value:               WFTextTokenStringValue{String: key},
			}
		}
		items = append(items, item)
	}
	return items
}
