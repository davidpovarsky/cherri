/*
 * Copyright (c) Cherri Language v2.0
 * Shared Pure Reference, Aggrandizement, Text Token, and Structure Codec
 */

package main

import (
	"fmt"
	"strings"
	"unicode/utf16"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

// ReferenceDescriptor is the shared pure description of a symbolic reference.
type ReferenceDescriptor struct {
	Kind            string // "ActionOutput", "Variable", "ExtensionInput", "Ask"
	OutputUUID      string
	OutputName      string
	VariableName    string
	Prompt          string
	Aggrandizements []Aggrandizement
}

// SharedCodecHook allows test interception of the shared reference encoder (for Section 19 verification).
var SharedCodecHook func(desc *ReferenceDescriptor, val *Value)

// EncodeReferenceValue converts a ReferenceDescriptor into a canonical Cherri Value.
func EncodeReferenceValue(desc ReferenceDescriptor) Value {
	val := Value{
		Type: desc.Kind,
	}
	switch desc.Kind {
	case "ActionOutput":
		val.OutputUUID = desc.OutputUUID
		val.OutputName = desc.OutputName
	case "Variable":
		val.VariableName = desc.VariableName
	case "ExtensionInput":
		val.VariableName = desc.VariableName
	case "Ask":
		val.Prompt = desc.Prompt
	default:
		if desc.OutputUUID != "" {
			val.Type = "ActionOutput"
			val.OutputUUID = desc.OutputUUID
			val.OutputName = desc.OutputName
		} else if desc.VariableName != "" {
			val.Type = "Variable"
			val.VariableName = desc.VariableName
		}
	}

	if len(desc.Aggrandizements) > 0 {
		val.Aggrandizements = make([]Aggrandizement, len(desc.Aggrandizements))
		copy(val.Aggrandizements, desc.Aggrandizements)
	}

	if SharedCodecHook != nil {
		SharedCodecHook(&desc, &val)
	}

	return val
}

// EncodeReferenceAttachment wraps an encoded reference value with serialization metadata.
func EncodeReferenceAttachment(desc ReferenceDescriptor, serializationType string) any {
	val := EncodeReferenceValue(desc)
	if serializationType == "" {
		return val
	}
	return WFTextTokenAttachment{
		Value:               val,
		WFSerializationType: serializationType,
	}
}

// ReferenceToDescriptor converts a backend.Reference into a ReferenceDescriptor.
func ReferenceToDescriptor(ref *backend.Reference) ReferenceDescriptor {
	if ref == nil {
		return ReferenceDescriptor{}
	}
	var desc ReferenceDescriptor
	switch ref.Kind {
	case backend.RefActionResult, backend.RefLoopResult:
		desc.Kind = "ActionOutput"
		desc.OutputUUID = ref.ProducerID
		desc.OutputName = ref.ProducerName
		if desc.OutputName == "" {
			desc.OutputName = ref.ProducerID
		}
	case backend.RefMutableBinding:
		desc.Kind = "Variable"
		desc.VariableName = ref.ProducerName
		if desc.VariableName == "" {
			desc.VariableName = ref.ProducerID
		}
	case backend.RefLoopItem, backend.RefLoopIndex:
		desc.Kind = "Variable"
		desc.VariableName = ref.ProducerName
		if desc.VariableName == "" {
			desc.VariableName = ref.ProducerID
		}
	case backend.RefSystemValue, backend.RefExtensionInput:
		desc.Kind = "ExtensionInput"
		desc.VariableName = ref.ProducerName
	case backend.RefAskEachTime:
		desc.Kind = "Ask"
	default:
		if ref.ProducerID != "" {
			desc.Kind = "ActionOutput"
			desc.OutputUUID = ref.ProducerID
			desc.OutputName = ref.ProducerName
		} else {
			desc.Kind = "Variable"
			desc.VariableName = ref.ProducerName
		}
	}

	if len(ref.Transformations) > 0 {
		for _, t := range ref.Transformations {
			aggr := Aggrandizement{
				Type:              t.Type,
				PropertyName:      t.PropertyName,
				CoercionItemClass: t.CoercionItemClass,
				DictionaryKey:     t.DictionaryKey,
				PropertyUserInfo:  t.PropertyUserInfo,
			}
			if aggr.Type == "WFPropertyVariableAggrandizement" && aggr.PropertyUserInfo == nil {
				aggr.PropertyUserInfo = 0
			}
			desc.Aggrandizements = append(desc.Aggrandizements, aggr)
		}
	}
	return desc
}

// EncodeTextSegmentsShared encodes text segments with attachments using shared UTF-16 offset logic.
func EncodeTextSegmentsShared(segments []backend.TextSegment) any {
	var fullText strings.Builder
	attachmentsByRange := make(map[string]Value)

	for _, seg := range segments {
		if !seg.IsExpr {
			fullText.WriteString(seg.Text)
		} else if seg.Reference != nil {
			startUTF16 := len(utf16.Encode([]rune(fullText.String())))
			fullText.WriteString("\uFFFC") // Object replacement char
			rangeKey := fmt.Sprintf("{%d, 1}", startUTF16)

			desc := ReferenceToDescriptor(seg.Reference)
			attachmentsByRange[rangeKey] = EncodeReferenceValue(desc)
		} else {
			fullText.WriteString(formatLiteral(seg.Value))
		}
	}

	if len(attachmentsByRange) == 0 {
		return fullText.String()
	}

	return WFTextTokenString{
		WFSerializationType: "WFTextTokenString",
		Value: WFTextTokenStringValue{
			String:             fullText.String(),
			AttachmentsByRange: attachmentsByRange,
		},
	}
}

// EncodeDictionaryItemShared creates a typed WFDictionaryFieldValueItem for plist dictionaries.
func EncodeDictionaryItemShared(key string, value any) WFDictionaryFieldValueItem {
	return makeDictionaryItem(key, value)
}
