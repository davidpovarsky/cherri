package main

// This is the deliberately bounded first local export contract. Native log
// items remain untouched. Arbitrary Content Graph objects are not JSON values;
// only an explicitly opted-in Text item has a payload in this version.
import (
	"fmt"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/google/uuid"
)

const debugExportSchemaVersion = "cherri.debug-export/1"

func registerDebugExportMacro() {
	actions["debugExport"] = &actionDefinition{
		identifier: "setitemname", outputType: Variable,
		doc: selfDoc{
			title:       "Export Local Debug File",
			description: "Return one local JSON or text File without saving, sharing or uploading. Payloads are fully redacted by default. Explicit redactSensitive:false exports Text as base64-encoded UTF-8; other native payloads report UNSUPPORTED_PAYLOAD_TYPE. Opt-in dates read item Creation Date metadata; labels are separately opt-in and stay redacted by default. Native File-class acceptance remains open. Options are compile-time literals.",
			category:    "debug",
		},
		parameters: []parameterDefinition{
			{name: "format", key: "WFDebugExportFormat", validType: RawString, optional: true, defaultValue: "json"},
			{name: "includeDates", key: "WFDebugExportDates", validType: Bool, optional: true, defaultValue: false},
			{name: "includeTypes", key: "WFDebugExportTypes", validType: Bool, optional: true, defaultValue: true},
			{name: "redactSensitive", key: "WFDebugExportRedact", validType: Bool, optional: true, defaultValue: true},
			{name: "includeLabels", key: "WFDebugExportLabels", validType: Bool, optional: true, defaultValue: false},
		},
	}
}

type debugExportOptions struct {
	format        string
	includeTypes  bool
	redact        bool
	includeDates  bool
	includeLabels bool
}

func parseDebugExportOptions(params map[string]any) (debugExportOptions, error) {
	out := debugExportOptions{format: "json", includeTypes: true, redact: true}
	if raw, exists := params["WFDebugExportFormat"]; exists {
		value, ok := raw.(string)
		if !ok {
			return out, fmt.Errorf("DEBUG_EXPORT_LITERAL: format must be a compile-time literal")
		}
		out.format = value
	}
	if out.format != "json" && out.format != "text" {
		return out, fmt.Errorf("DEBUG_EXPORT_FORMAT: expected json or text")
	}
	for _, item := range []struct {
		key    string
		target *bool
	}{
		{"WFDebugExportDates", &out.includeDates},
		{"WFDebugExportLabels", &out.includeLabels},
		{"WFDebugExportTypes", &out.includeTypes},
		{"WFDebugExportRedact", &out.redact},
	} {
		if _, exists := params[item.key]; !exists {
			continue
		}
		value, ok := params[item.key].(bool)
		if !ok {
			return out, fmt.Errorf("DEBUG_EXPORT_LITERAL: %s must be a Boolean literal", item.key)
		}
		*item.target = value
	}
	return out, nil
}

type debugExportExpansion struct{ actions []ShortcutAction }

func (b *debugExportExpansion) add(id string, params map[string]any) {
	b.actions = append(b.actions, ShortcutAction{
		WFWorkflowActionIdentifier: "is.workflow.actions." + id,
		WFWorkflowActionParameters: params,
	})
}

func (b *debugExportExpansion) produce(id string, params map[string]any) WFTextTokenAttachment {
	out := strings.ToUpper(uuid.NewString())
	params["UUID"] = out
	b.add(id, params)
	return debugOutputRef(out)
}

// Every dynamic JSON field is native Base64 Encode output with no line breaks.
// This helper only assembles semantic segments; the canonical shared codec owns
// serialization, placeholder creation and UTF-16 attachment ranges.
func debugExportTextToken(parts ...any) any {
	segments := make([]backend.TextSegment, 0, len(parts))
	for _, part := range parts {
		switch value := part.(type) {
		case string:
			segments = append(segments, backend.TextSegment{Text: value})
		case WFTextTokenAttachment:
			ref := value.Value
			if value.WFSerializationType != "WFTextTokenAttachment" || ref.Type != "ActionOutput" ||
				ref.OutputUUID == "" || len(ref.Aggrandizements) != 0 {
				panic("debug export requires an exact untransformed internal output reference")
			}
			segments = append(segments, backend.TextSegment{IsExpr: true, Reference: &backend.Reference{
				Kind: backend.RefActionResult, ProducerID: ref.OutputUUID, ProducerName: ref.OutputName,
			}})
		default:
			panic("invalid internal debug export text segment")
		}
	}
	return EncodeTextSegmentsShared(segments)
}

func (b *debugExportExpansion) base64(input any) WFTextTokenAttachment {
	return b.produce("base64encode", map[string]any{
		"WFInput": input, "WFEncodeMode": "Encode", "WFBase64LineBreakMode": "None",
	})
}

func (b *debugExportExpansion) row(status string, typeValue, payload *WFTextTokenAttachment, metadata ...any) WFTextTokenAttachment {
	parts := []any{`{"status":"`, status, `","payload":`}
	if payload == nil {
		parts = append(parts, "null")
	} else {
		parts = append(parts, `{"encoding":"base64-utf8","value":"`, *payload, `"}`)
	}
	if typeValue != nil {
		parts = append(parts, `,"nativeType":{"encoding":"base64-utf8","value":"`, *typeValue, `"}`)
	}
	parts = append(parts, metadata...)
	parts = append(parts, "}")
	return b.produce("gettext", map[string]any{"WFTextActionText": debugExportTextToken(parts...)})
}

func buildDebugExportActions(params map[string]any, resultUUID, resultName string) ([]ShortcutAction, error) {
	options, err := parseDebugExportOptions(params)
	if err != nil {
		return nil, err
	}
	if options.includeDates || options.includeLabels {
		return buildDebugExportMetadataActions(options, resultUUID, resultName), nil
	}
	b := &debugExportExpansion{}
	var textWitness WFTextTokenAttachment
	if !options.redact {
		sample := b.produce("gettext", map[string]any{"WFTextActionText": "Cherri export Text witness"})
		textWitness = b.produce("getitemtype", map[string]any{"WFInput": sample})
	}
	group := strings.ToUpper(uuid.NewString())
	b.add("repeat.each", map[string]any{
		"WFInput": debugLogValueRef(debugLogVariableName), "GroupingIdentifier": group, "WFControlFlowMode": 0,
	})
	item := debugLogValueRef("Repeat Item")
	var actualType WFTextTokenAttachment
	if options.includeTypes || !options.redact {
		actualType = b.produce("getitemtype", map[string]any{"WFInput": item})
	}
	var typeValue *WFTextTokenAttachment
	if options.includeTypes {
		encoded := b.base64(actualType)
		typeValue = &encoded
	}
	if options.redact {
		b.row("REDACTED", typeValue, nil)
	} else {
		conditional := strings.ToUpper(uuid.NewString())
		b.add("conditional", backend.BuildIfStartTyped(conditional,
			map[string]any{"Type": "Variable", "Variable": actualType}, 4,
			requireComparisonText(textWitness), nil, backend.OperandString, true))
		encoded := b.base64(item)
		b.row("TEXT_PAYLOAD_ENCODED", typeValue, &encoded)
		b.add("conditional", BuildIfElse(conditional))
		b.row("UNSUPPORTED_PAYLOAD_TYPE", typeValue, nil)
		b.add("conditional", BuildIfEnd(conditional, strings.ToUpper(uuid.NewString())))
	}
	rowsUUID := strings.ToUpper(uuid.NewString())
	b.add("repeat.each", map[string]any{
		"WFControlFlowMode": 2, "GroupingIdentifier": group, "UUID": rowsUUID,
	})
	separator := ","
	if options.format == "text" {
		separator = "\n"
	}
	combined := b.produce("text.combine", map[string]any{
		"text": debugOutputRefNamed(rowsUUID, "Repeat Results"), "WFTextSeparator": "Custom", "WFTextCustomSeparator": separator,
	})
	policy := "redacted"
	if !options.redact {
		policy = "text-only-base64"
	}
	var envelope any
	if options.format == "json" {
		envelope = debugExportTextToken(
			`{"schemaVersion":"`+debugExportSchemaVersion+`","scope":"run-local-native-items","payloadPolicy":"`+policy+
				`","dates":"not-collected","labels":"not-collected","entries":[`, combined, "]}")
	} else {
		envelope = debugExportTextToken("Cherri Debug Export 1\nScope: run-local native items\nPayload policy: "+policy+
			"\nDates: not collected\nLabels: not collected\nEntry encoding: JSON lines; Text payloads use base64-utf8\n\n", combined)
	}
	content := b.produce("gettext", map[string]any{"WFTextActionText": envelope})
	// Get File of Type does not establish a byte-preserving Text conversion.
	// Encode the completed envelope, then decode those exact bytes into a new
	// native File before naming it. Payload redaction has already happened.
	encodedFile := b.base64(content)
	file := b.produce("base64encode", map[string]any{
		"WFInput": encodedFile, "WFEncodeMode": "Decode",
	})
	if resultUUID == "" {
		resultUUID = strings.ToUpper(uuid.NewString())
	}
	extension := "json"
	if options.format == "text" {
		extension = "txt"
	}
	result := map[string]any{
		"WFInput": file, "WFName": "cherri-debug." + extension,
		"WFDontIncludeFileExtension": false, "UUID": resultUUID,
	}
	if resultName != "" {
		result["CustomOutputName"] = resultName
	}
	b.add("setitemname", result)
	return b.actions, nil
}
