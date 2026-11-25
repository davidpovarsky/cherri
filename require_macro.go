package main

import (
	"fmt"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

func registerRequireMacro() {
	actions["require"] = &actionDefinition{
		identifier: "getvariable", outputType: Variable,
		doc: selfDoc{title: "Require Typed Value", description: "Validate native content and return the original value without text or JSON conversion. Supported type guards: text, number, dictionary. nonEmpty checks characters, keys or content items.", category: "control"},
		parameters: []parameterDefinition{
			{name: "value", key: "WFInput", validType: Variable},
			{name: "type", key: "WFRequireType", validType: RawString, optional: true},
			{name: "nonEmpty", key: "WFRequireNonEmpty", validType: Bool, optional: true},
			{name: "message", key: "WFAlertActionMessage", validType: String, optional: true, defaultValue: "Requirement failed"},
		},
		makeParams: func(args []actionArgument) map[string]any {
			out := make(map[string]any)
			if len(args) > 0 {
				out[requireSourceTypeKey] = requireLegacySourceType(args[0])
			}
			keys := []string{"WFInput", "WFRequireType", "WFRequireNonEmpty", "WFAlertActionMessage"}
			for i, key := range keys {
				if i >= len(args) {
					break
				}
				if args[i].valueType == Nil || args[i].value == nil {
					if i == 0 {
						out[key] = nil
					}
					continue
				}
				out[key] = argumentValue(args, i)
			}
			return out
		},
	}
}

type requireExpansion struct {
	actions   []ShortcutAction
	input     any
	typeRef   WFTextTokenAttachment
	itemsRef  WFTextTokenAttachment
	witnesses map[string]WFTextTokenAttachment
	message   any
}

func (b *requireExpansion) add(id string, params map[string]any) {
	b.actions = append(b.actions, ShortcutAction{WFWorkflowActionIdentifier: "is.workflow.actions." + id, WFWorkflowActionParameters: params})
}

func (b *requireExpansion) produce(id string, params map[string]any) WFTextTokenAttachment {
	out := strings.ToUpper(nextGeneratedUUID("require"))
	params["UUID"] = out
	b.add(id, params)
	return debugOutputRef(out)
}

func requireInputReference(value any) bool {
	return isDirectDebugLogInput(value)
}

// Literal inputs use the existing native literal codecs. Existing attachments
// are never decoded, converted, re-evaluated or replaced with a text token.
func (b *requireExpansion) materialize(value any) (any, error) {
	if requireInputReference(value) {
		plain := false
		switch ref := value.(type) {
		case WFTextTokenAttachment:
			plain = ref.Value.Type == "ActionOutput" && ref.Value.OutputUUID != "" && len(ref.Value.Aggrandizements) == 0
		case *WFTextTokenAttachment:
			plain = ref != nil && ref.Value.Type == "ActionOutput" && ref.Value.OutputUUID != "" && len(ref.Value.Aggrandizements) == 0
		case map[string]any:
			if body, ok := ref["Value"].(map[string]any); ok {
				_, hasTransform := body["Aggrandizements"]
				plain = body["Type"] == "ActionOutput" && body["OutputUUID"] != "" && !hasTransform
			}
		}
		if plain {
			return value, nil
		}
		// Capture mutable/system/derived values once before inspecting them.
		return b.produce("getvariable", map[string]any{"WFVariable": value}), nil
	}
	switch v := value.(type) {
	case nil:
		return b.produce("nothing", map[string]any{}), nil
	case bool:
		n := 0
		if v {
			n = 1
		}
		return b.produce("number", map[string]any{"WFNumberActionNumber": fmt.Sprint(n)}), nil
	case int, int64, uint64, float64, float32:
		return b.produce("number", map[string]any{"WFNumberActionNumber": fmt.Sprint(v)}), nil
	case string, WFTextTokenString:
		return b.produce("gettext", map[string]any{"WFTextActionText": v}), nil
	case WFDictionaryFieldValue:
		return b.produce("dictionary", map[string]any{"WFItems": toExplicitDictionaryFieldValue(v)}), nil
	case []any:
		items := make([]WFDictionaryFieldValueItem, 0, len(v))
		for _, item := range v {
			items = append(items, makeDictionaryItem("", item))
		}
		return b.produce("list", map[string]any{"WFItems": items}), nil
	case WFArrayValue:
		return b.produce("list", map[string]any{"WFItems": v.Value}), nil
	default:
		return nil, fmt.Errorf("require: unsupported literal input representation %T; bind the native producer before requiring it", value)
	}
}

func requireComparisonText(ref WFTextTokenAttachment) WFTextTokenString {
	return WFTextTokenString{
		WFSerializationType: "WFTextTokenString",
		Value:               WFTextTokenStringValue{String: "\uFFFC", AttachmentsByRange: map[string]Value{"{0, 1}": ref.Value}},
	}
}

func (b *requireExpansion) begin(input any, code int, value any, kind backend.ConditionOperandType) string {
	group := strings.ToUpper(nextGeneratedUUID("require"))
	b.add("conditional", backend.BuildIfStartTyped(group, map[string]any{"Type": "Variable", "Variable": input}, code, value, nil, kind, true))
	return group
}

func (b *requireExpansion) end(group string)       { b.add("conditional", BuildIfEnd(group, "")) }
func (b *requireExpansion) otherwise(group string) { b.add("conditional", BuildIfElse(group)) }

func (b *requireExpansion) failIf(input any, code int, value any, kind backend.ConditionOperandType) {
	group := b.begin(input, code, value, kind)
	b.add("alert", map[string]any{"WFAlertActionMessage": b.message, "WFAlertActionCancelButtonShown": false})
	b.add("exit", map[string]any{})
	b.end(group)
}

func (b *requireExpansion) count(input any, kind string) WFTextTokenAttachment {
	return b.produce("count", map[string]any{"Input": input, "WFCountType": kind})
}

func (b *requireExpansion) items() WFTextTokenAttachment {
	if b.itemsRef.Value.OutputUUID == "" {
		b.itemsRef = b.count(b.input, "Items")
	}
	return b.itemsRef
}

func (b *requireExpansion) actualType() WFTextTokenAttachment {
	if b.typeRef.Value.OutputUUID == "" {
		b.typeRef = b.produce("getitemtype", map[string]any{"WFInput": b.input})
	}
	return b.typeRef
}

// Compare runtime-derived type labels with another runtime-derived label.
// Never assume Apple's display text is English or stable across locales.
func (b *requireExpansion) witness(kind string) WFTextTokenAttachment {
	if ref, ok := b.witnesses[kind]; ok {
		return ref
	}
	var sample any
	switch kind {
	case "text":
		sample = b.produce("gettext", map[string]any{"WFTextActionText": "Cherri type witness"})
	case "number":
		sample = b.produce("number", map[string]any{"WFNumberActionNumber": "1"})
	case "dictionary":
		var raw any = map[string]any{"cherri_type_witness": true}
		sample = b.produce("dictionary", map[string]any{"WFItems": makeExplicitDictionaryValue(&raw)})
	}
	ref := b.produce("getitemtype", map[string]any{"WFInput": sample})
	b.witnesses[kind] = ref
	return ref
}

func (b *requireExpansion) requireCountNonzero(kind string) {
	input := b.input
	countKind := "Items"
	if kind == "dictionary" {
		input = b.produce("getvalueforkey", map[string]any{"WFInput": b.input, "WFGetDictionaryValueType": "All Keys"})
	} else if kind == "text" {
		countKind = "Characters"
	}
	var count WFTextTokenAttachment
	if kind == "dictionary" || kind == "text" {
		count = b.count(input, countKind)
	} else {
		count = b.items()
	}
	b.failIf(count, 4, 0, backend.OperandNumber)
}

func buildRequireMacroActions(params map[string]any, resultUUID, resultName string) ([]ShortcutAction, error) {
	kind := ""
	if raw, exists := params["WFRequireType"]; exists {
		literal, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("require: type must be a literal string")
		}
		kind = strings.ToLower(strings.TrimSpace(literal))
	}
	switch kind {
	case "", "text", "number", "dictionary":
	default:
		return nil, fmt.Errorf("require: unsupported type guard %q; supported native categories are text, number and dictionary", kind)
	}
	nonEmpty, err := debugLiteralFlag(params, "WFRequireNonEmpty", "require")
	if err != nil {
		return nil, err
	}
	if kind == "" && !nonEmpty {
		return nil, fmt.Errorf("require: specify a supported type guard or nonEmpty:true")
	}
	raw, provided := params["WFInput"]
	if !provided {
		return nil, fmt.Errorf("require(value, ...) requires a value")
	}
	msg := params["WFAlertActionMessage"]
	if msg == nil {
		msg = "Requirement failed"
	}
	b := &requireExpansion{message: msg, witnesses: make(map[string]WFTextTokenAttachment)}
	b.input, err = b.materialize(raw)
	if err != nil {
		return nil, err
	}

	sourceType, _ := params[requireSourceTypeKey].(string)
	sourceCategory := requireSemanticCategory(sourceType)
	if sourceCategory == "" {
		sourceCategory = requireLiteralCategory(raw)
	}
	if kind != "" {
		// Get Type may itself yield a collection or no output. Require exactly
		// one item before comparing labels. Native singleton collection/scalar
		// identity is erased; known List input is rejected by the analyzer.
		b.failIf(b.items(), 5, 1, backend.OperandNumber)
		actual := b.actualType()
		expected := b.witness(kind)
		b.failIf(actual, 5, requireComparisonText(expected), backend.OperandString)
	}
	if nonEmpty {
		if kind != "" {
			b.requireCountNonzero(kind)
		} else if sourceCategory != "" {
			b.requireCountNonzero(sourceCategory)
		} else {
			// Empty content lists fail; false and zero each remain one present item.
			b.failIf(b.items(), 4, 0, backend.OperandNumber)
			// Multi-item lists are already nonempty. Do not flatten their items merely
			// because Get Type reports text or dictionary for their elements.
			one := b.begin(b.items(), 4, 1, backend.OperandNumber)
			actual := b.actualType()
			dictType := b.witness("dictionary")
			textType := b.witness("text")
			dictBranch := b.begin(actual, 4, requireComparisonText(dictType), backend.OperandString)
			b.requireCountNonzero("dictionary")
			b.otherwise(dictBranch)
			textBranch := b.begin(actual, 4, requireComparisonText(textType), backend.OperandString)
			b.requireCountNonzero("text")
			b.end(textBranch)
			b.end(dictBranch)
			b.end(one)
		}
	}
	passthrough := map[string]any{"WFVariable": b.input}
	if resultUUID != "" {
		passthrough["UUID"] = resultUUID
	}
	if resultName != "" {
		passthrough["CustomOutputName"] = resultName
	}
	b.add("getvariable", passthrough)
	return b.actions, nil
}
