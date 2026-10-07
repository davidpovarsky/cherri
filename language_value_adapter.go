/*
 * Copyright (c) Cherri Language v2.0
 * Canonical Value and Reference Adapter
 */

package main

import (
	"fmt"
	"strings"
	"unicode/utf16"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

func convertAggrandizements(transforms []backend.Transformation) []Aggrandizement {
	if len(transforms) == 0 {
		return nil
	}
	res := make([]Aggrandizement, len(transforms))
	for i, t := range transforms {
		res[i] = Aggrandizement{
			Type:              t.Type,
			PropertyName:      t.PropertyName,
			CoercionItemClass: t.CoercionItemClass,
			DictionaryKey:     t.DictionaryKey,
			PropertyUserInfo:  t.PropertyUserInfo,
		}
	}
	return res
}

func encodeReferenceValue(ref *backend.Reference) Value {
	if ref == nil {
		return Value{}
	}
	val := Value{}
	switch ref.Kind {
	case backend.RefActionResult, backend.RefLoopResult:
		val.Type = "ActionOutput"
		val.OutputUUID = ref.ProducerID
		val.OutputName = ref.ProducerName
	case backend.RefMutableBinding:
		val.Type = "Variable"
		val.VariableName = ref.ProducerName
		if val.VariableName == "" {
			val.VariableName = ref.ProducerID
		}
	case backend.RefSystemValue, backend.RefExtensionInput:
		val.Type = "ExtensionInput"
		val.VariableName = ref.ProducerName
	case backend.RefAskEachTime:
		val.Type = "Ask"
	default:
		if ref.ProducerID != "" {
			val.Type = "ActionOutput"
			val.OutputUUID = ref.ProducerID
			val.OutputName = ref.ProducerName
		} else {
			val.Type = "Variable"
			val.VariableName = ref.ProducerName
		}
	}
	if len(ref.Transformations) > 0 {
		val.Aggrandizements = convertAggrandizements(ref.Transformations)
	}
	return val
}

func encodeReferenceAttachment(ref *backend.Reference) WFTextTokenAttachment {
	return WFTextTokenAttachment{
		Value:               encodeReferenceValue(ref),
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
	var fullText strings.Builder
	attachmentsByRange := make(map[string]any)

	for _, seg := range segments {
		if !seg.IsExpr {
			fullText.WriteString(seg.Text)
		} else if seg.Reference != nil {
			startUTF16 := len(utf16.Encode([]rune(fullText.String())))
			fullText.WriteString("\uFFFC") // Object replacement char
			rangeKey := fmt.Sprintf("{%d, 1}", startUTF16)

			val := encodeReferenceValue(seg.Reference)
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
			attachmentsByRange[rangeKey] = valMap
		} else {
			fullText.WriteString(formatLiteral(seg.Value))
		}
	}

	if len(attachmentsByRange) == 0 {
		return fullText.String()
	}

	return map[string]any{
		"WFSerializationType": "WFTextTokenString",
		"Value": map[string]any{
			"string":             fullText.String(),
			"attachmentsByRange": attachmentsByRange,
		},
	}
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
