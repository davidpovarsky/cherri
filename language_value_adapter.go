/*
 * Copyright (c) Cherri Language v2.0
 * Canonical Value and Reference Adapter
 */

package main

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

func convertAggrandizements(transforms []backend.Transformation) []Aggrandizement {
	desc := ReferenceToDescriptor(&backend.Reference{Transformations: transforms})
	return desc.Aggrandizements
}

func encodeReferenceValue(ref *backend.Reference) Value {
	desc := ReferenceToDescriptor(ref)
	return EncodeReferenceValue(desc)
}

func encodeReferenceAttachment(ref *backend.Reference) WFTextTokenAttachment {
	desc := ReferenceToDescriptor(ref)
	res := EncodeReferenceAttachment(desc, "WFTextTokenAttachment")
	if att, ok := res.(WFTextTokenAttachment); ok {
		return att
	}
	return WFTextTokenAttachment{
		Value:               EncodeReferenceValue(desc),
		WFSerializationType: "WFTextTokenAttachment",
	}
}

func encodeReferenceMap(ref *backend.Reference) map[string]any {
	val := encodeReferenceValue(ref)
	valMap := map[string]any{
		"Type": val.Type,
	}
	if val.OutputUUID != "" {
		valMap["OutputUUID"] = val.OutputUUID
	}
	if val.OutputName != "" {
		valMap["OutputName"] = val.OutputName
	}
	if val.VariableName != "" {
		valMap["VariableName"] = val.VariableName
	}
	if len(val.Aggrandizements) > 0 {
		aggrs := make([]map[string]any, len(val.Aggrandizements))
		for i, a := range val.Aggrandizements {
			m := map[string]any{"Type": a.Type}
			if a.PropertyName != "" {
				m["PropertyName"] = a.PropertyName
			}
			if a.CoercionItemClass != "" {
				m["CoercionItemClass"] = a.CoercionItemClass
			}
			if a.DictionaryKey != "" {
				m["DictionaryKey"] = a.DictionaryKey
			}
			if a.PropertyUserInfo != nil {
				m["PropertyUserInfo"] = a.PropertyUserInfo
			}
			aggrs[i] = m
		}
		valMap["Aggrandizements"] = aggrs
	}
	return map[string]any{
		"Value":               valMap,
		"WFSerializationType": "WFTextTokenAttachment",
	}
}

func encodeTextSegments(segments []backend.TextSegment) any {
	return EncodeTextSegmentsShared(segments)
}

func formatLiteral(val backend.SemanticValue) string {
	switch val.Kind {
	case backend.ValBool:
		return fmt.Sprintf("%t", val.BoolVal)
	case backend.ValInt:
		return fmt.Sprintf("%d", val.IntVal)
	case backend.ValFloat:
		if val.FloatVal == float64(int64(val.FloatVal)) {
			return fmt.Sprintf("%d", int64(val.FloatVal))
		}
		return fmt.Sprintf("%g", val.FloatVal)
	case backend.ValString:
		return val.StrVal
	default:
		return ""
	}
}

// ConvertSemanticValueToParam transforms a backend.SemanticValue into an Apple Shortcuts parameter value.
func ConvertSemanticValueToParam(v backend.SemanticValue) any {
	if v.Omitted {
		return nil
	}
	switch v.Kind {
	case backend.ValNil:
		return nil
	case backend.ValBool:
		return v.BoolVal
	case backend.ValInt:
		return v.IntVal
	case backend.ValFloat:
		return v.FloatVal
	case backend.ValString:
		return v.StrVal
	case backend.ValReference:
		return encodeReferenceMap(v.Ref)
	case backend.ValTextSegments:
		return encodeTextSegments(v.Segments)
	case backend.ValList:
		items := make([]any, len(v.ListVal))
		for i, el := range v.ListVal {
			items[i] = ConvertSemanticValueToParam(el)
		}
		return items
	case backend.ValDict:
		m := make(map[string]any, len(v.DictVal))
		for _, entry := range v.DictVal {
			m[entry.Key] = ConvertSemanticValueToParam(entry.Value)
		}
		return m
	case backend.ValRawNative:
		return v.RawNativeVal
	default:
		return nil
	}
}
