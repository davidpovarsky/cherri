package main

import "github.com/electrikmilk/cherri/internal/language/backend"

// encodeTypedLiteralParam carries an authored AST's semantic kinds through the
// canonical raw-action adapter. In particular, ValDict is always a Dictionary,
// even if its ordinary keys spell WFSerializationType, WFItemType or Value.
// Native/import payloads do not enter this path. All native structures are made
// by the existing shared encoders; there is no second wire representation.
func encodeTypedLiteralParam(value backend.SemanticValue) any {
	switch value.Kind {
	case backend.ValDict:
		fields := make(map[string]any, len(value.DictVal))
		for _, entry := range value.DictVal {
			fields[entry.Key] = encodeTypedLiteralParam(entry.Value)
		}
		var raw any = fields
		return makeDictionaryValue(&raw)
	case backend.ValList:
		items := make([]WFDictionaryFieldValueItem, 0, len(value.ListVal))
		for _, item := range value.ListVal {
			items = append(items, EncodeDictionaryItemShared("", encodeTypedLiteralParam(item)))
		}
		return makeArrayValue(items)
	case backend.ValReference:
		return EncodeReferenceAttachment(ReferenceToDescriptor(value.Ref), "WFTextTokenAttachment")
	case backend.ValTextSegments:
		return EncodeTextSegmentsShared(value.Segments)
	default:
		return ConvertSemanticValueToParam(value)
	}
}
