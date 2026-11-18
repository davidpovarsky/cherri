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

// Keep the existing canonical sorted-key order. Only the source's explicit
// literal identity changes: a key spelling "{name}" remains that exact key.
func encodeTypedLiteralDictionaryItems(entries []backend.DictEntry) []WFDictionaryFieldValueItem {
	sorted := append([]backend.DictEntry(nil), entries...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Key < sorted[j].Key })
	items := make([]WFDictionaryFieldValueItem, 0, len(sorted))
	for _, entry := range sorted {
		item := EncodeDictionaryItemShared("", encodeTypedLiteralParam(entry.Value))
		item.WFKey = WFTextTokenString{
			WFSerializationType: "WFTextTokenString",
			Value:               WFTextTokenStringValue{String: entry.Key},
		}
		items = append(items, item)
	}
	return items
}
