package main

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
)

// These helpers fail instead of inventing JSON null, false or zero for an
// unsupported or malformed field. They do not stand in for Apple execution.
func p10DecodeJSONDictionary(raw any, resolved map[string]any) (map[string]any, error) {
	outer, ok := raw.(map[string]any)
	if !ok || outer["WFSerializationType"] != "WFDictionaryFieldValue" {
		return nil, fmt.Errorf("invalid Dictionary wrapper")
	}
	value, ok := outer["Value"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing Dictionary value")
	}
	rows, ok := value["WFDictionaryFieldValueItems"].([]any)
	if !ok {
		return nil, fmt.Errorf("missing explicit Dictionary item list")
	}
	result := make(map[string]any, len(rows))
	for i, rawRow := range rows {
		row, ok := rawRow.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid Dictionary row %d", i)
		}
		key, err := p10DecodeJSONText(row["WFKey"], resolved)
		if err != nil {
			return nil, fmt.Errorf("Dictionary key %d: %w", i, err)
		}
		if _, duplicate := result[key]; duplicate {
			return nil, fmt.Errorf("duplicate Dictionary key %q", key)
		}
		item, err := p10DecodeJSONItem(row, resolved)
		if err != nil {
			return nil, fmt.Errorf("Dictionary field %q: %w", key, err)
		}
		result[key] = item
	}
	return result, nil
}

func p10DecodeJSONReference(ref map[string]any, resolved map[string]any) (any, error) {
	var kind, ok = ref["Type"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid reference Type metadata")
	}
	var identityKey string
	var allowed map[string]bool
	switch kind {
	case "ActionOutput":
		identityKey = "OutputUUID"
		allowed = map[string]bool{"Type": true, "OutputUUID": true, "OutputName": true}
	case "Variable":
		identityKey = "VariableName"
		allowed = map[string]bool{"Type": true, "VariableName": true}
	default:
		return nil, fmt.Errorf("unsupported reference kind %q", kind)
	}
	// This decoder only proves plain references. Ignoring a property, coercion
	// or unknown field would silently substitute the original producer value.
	for name, raw := range ref {
		if !allowed[name] {
			return nil, fmt.Errorf("unsupported reference field %q", name)
		}
		if _, valid := raw.(string); !valid {
			return nil, fmt.Errorf("invalid reference %s metadata", name)
		}
	}
	var identity, valid = ref[identityKey].(string)
	if !valid || identity == "" {
		return nil, fmt.Errorf("missing reference %s", identityKey)
	}
	var key = kind + ":" + identity
	var value, exists = resolved[key]
	if !exists || value == nil {
		return nil, fmt.Errorf("unresolved actual reference %q", key)
	}
	return value, nil
}

func p10DecodeJSONText(raw any, resolved map[string]any) (string, error) {
	if literal, ok := raw.(string); ok {
		return literal, nil
	}
	outer, ok := raw.(map[string]any)
	if !ok || outer["WFSerializationType"] != "WFTextTokenString" {
		return "", fmt.Errorf("invalid Text wrapper")
	}
	value, ok := outer["Value"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("missing Text value")
	}
	text, ok := value["string"].(string)
	if !ok {
		return "", fmt.Errorf("missing explicit Text string")
	}
	attachments, hasAttachments := value["attachmentsByRange"]
	if !hasAttachments {
		if strings.Contains(text, "\ufffc") {
			return "", fmt.Errorf("unresolved Text marker")
		}
		return text, nil
	}
	refs, ok := attachments.(map[string]any)
	if !ok {
		return "", fmt.Errorf("invalid Text attachments")
	}
	if len(refs) == 0 && !strings.Contains(text, "\ufffc") {
		return text, nil
	}
	if text != "\ufffc" || len(refs) != 1 {
		return "", fmt.Errorf("unsupported Text interpolation shape")
	}
	ref, ok := refs["{0, 1}"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("invalid Text attachment range")
	}
	item, err := p10DecodeJSONReference(ref, resolved)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(item), nil
}

func p10DecodeJSONItem(row map[string]any, resolved map[string]any) (any, error) {
	kind := uint64(99)
	switch raw := row["WFItemType"].(type) {
	case uint64:
		kind = raw
	case int64:
		if raw >= 0 {
			kind = uint64(raw)
		}
	case int:
		if raw >= 0 {
			kind = uint64(raw)
		}
	}
	if kind > 4 {
		return nil, fmt.Errorf("unsupported Dictionary item type")
	}
	rawValue, exists := row["WFValue"]
	if !exists || rawValue == nil {
		return nil, fmt.Errorf("missing present Dictionary field value")
	}
	value, ok := rawValue.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid Dictionary field wrapper")
	}
	switch kind {
	case 0:
		if value["WFSerializationType"] == "WFTextTokenAttachment" {
			ref, ok := value["Value"].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid reference payload")
			}
			return p10DecodeJSONReference(ref, resolved)
		}
		return p10DecodeJSONText(value, resolved)
	case 1:
		return p10DecodeJSONDictionary(value, resolved)
	case 2:
		if value["WFSerializationType"] != "WFArrayParameterState" {
			return nil, fmt.Errorf("invalid Array wrapper")
		}
		rows, ok := value["Value"].([]any)
		if !ok {
			return nil, fmt.Errorf("missing explicit Array values")
		}
		result := make([]any, len(rows))
		for i, raw := range rows {
			child, ok := raw.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid Array item %d", i)
			}
			decoded, err := p10DecodeJSONItem(child, resolved)
			if err != nil {
				return nil, fmt.Errorf("Array item %d: %w", i, err)
			}
			result[i] = decoded
		}
		return result, nil
	case 3:
		text, err := p10DecodeJSONText(value, resolved)
		if err != nil {
			return nil, err
		}
		number, err := strconv.ParseFloat(text, 64)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return nil, fmt.Errorf("invalid finite Number field")
		}
		return number, nil
	case 4:
		if value["WFSerializationType"] != "WFNumberSubstitutableState" {
			return nil, fmt.Errorf("invalid Bool wrapper")
		}
		boolean, ok := value["Value"].(bool)
		if !ok {
			return nil, fmt.Errorf("invalid explicit Bool value")
		}
		return boolean, nil
	}
	return nil, fmt.Errorf("unreachable Dictionary item")
}

func p10CloneJSONWire(raw any) any {
	switch value := raw.(type) {
	case map[string]any:
		copy := make(map[string]any, len(value))
		for k, v := range value {
			copy[k] = p10CloneJSONWire(v)
		}
		return copy
	case []any:
		copy := make([]any, len(value))
		for i, v := range value {
			copy[i] = p10CloneJSONWire(v)
		}
		return copy
	default:
		return raw
	}
}

func p10JSONWireField(t *testing.T, wire any, key string) map[string]any {
	t.Helper()
	rows := wire.(map[string]any)["Value"].(map[string]any)["WFDictionaryFieldValueItems"].([]any)
	for _, raw := range rows {
		row := raw.(map[string]any)
		name, err := p10DecodeJSONText(row["WFKey"], nil)
		if err != nil {
			t.Fatal(err)
		}
		if name == key {
			return row
		}
	}
	t.Fatalf("missing mutation field %s", key)
	return nil
}

func p10AssertJSONWireMutations(t *testing.T, original any, resolved map[string]any, expected map[string]any) {
	t.Helper()
	for _, fault := range []string{"unknown_item_type", "missing_item_type", "missing_value", "null_value", "malformed_bool", "missing_bool", "malformed_number", "nonfinite_number", "unknown_wrapper", "duplicate_key", "wrong_reference", "reference_property", "reference_coercion", "reference_unknown_field", "reference_type_type", "reference_uuid_type", "reference_name_type", "reference_missing_uuid", "reference_extra_variable_name", "reference_empty_aggrandizements", "missing_array", "unknown_array_item"} {
		t.Run("strict-wire/"+fault, func(t *testing.T) {
			copy := p10CloneJSONWire(original)
			if got, err := p10DecodeJSONDictionary(copy, resolved); err != nil || !reflect.DeepEqual(got, expected) {
				t.Fatal("unchanged copy rejected", err)
			}
			stream := p10JSONWireField(t, copy, "stream")
			zero := p10JSONWireField(t, copy, "zero")
			switch fault {
			case "unknown_item_type":
				stream["WFItemType"] = uint64(5)
			case "missing_item_type":
				delete(stream, "WFItemType")
			case "missing_value":
				delete(stream, "WFValue")
			case "null_value":
				stream["WFValue"] = nil
			case "malformed_bool":
				stream["WFValue"].(map[string]any)["Value"] = "false"
			case "missing_bool":
				delete(stream["WFValue"].(map[string]any), "Value")
			case "malformed_number":
				zero["WFValue"].(map[string]any)["Value"].(map[string]any)["string"] = "not-a-number"
			case "nonfinite_number":
				zero["WFValue"].(map[string]any)["Value"].(map[string]any)["string"] = "NaN"
			case "unknown_wrapper":
				stream["WFValue"].(map[string]any)["WFSerializationType"] = "Unknown"
			case "duplicate_key":
				inner := copy.(map[string]any)["Value"].(map[string]any)
				inner["WFDictionaryFieldValueItems"] = append(inner["WFDictionaryFieldValueItems"].([]any), p10CloneJSONWire(stream))
			case "wrong_reference":
				model := p10JSONWireField(t, copy, "model")["WFValue"].(map[string]any)
				ref := model["Value"].(map[string]any)
				ref["OutputUUID"] = "unrelated-producer"
			case "reference_unknown_field", "reference_type_type", "reference_uuid_type", "reference_name_type", "reference_missing_uuid", "reference_extra_variable_name", "reference_empty_aggrandizements":
				model := p10JSONWireField(t, copy, "model")["WFValue"].(map[string]any)
				ref := model["Value"].(map[string]any)
				switch fault {
				case "reference_unknown_field":
					ref["UnknownMetadata"] = "unreviewed"
				case "reference_type_type":
					ref["Type"] = 7
				case "reference_uuid_type":
					ref["OutputUUID"] = 7
				case "reference_name_type":
					ref["OutputName"] = []any{"dynamicModel"}
				case "reference_missing_uuid":
					delete(ref, "OutputUUID")
				case "reference_extra_variable_name":
					ref["VariableName"] = "unreviewed"
				case "reference_empty_aggrandizements":
					ref["Aggrandizements"] = []any{}
				}
			case "reference_property", "reference_coercion":
				model := p10JSONWireField(t, copy, "model")["WFValue"].(map[string]any)
				ref := model["Value"].(map[string]any)
				transform := map[string]any{"Type": "WFPropertyVariableAggrandizement", "PropertyName": "Name"}
				if fault == "reference_coercion" {
					transform = map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFNumberContentItem"}
				}
				ref["Aggrandizements"] = []any{transform}
			case "missing_array", "unknown_array_item":
				nested := p10JSONWireField(t, copy, "nested")["WFValue"]
				array := p10JSONWireField(t, nested, "flags")["WFValue"].(map[string]any)
				if fault == "missing_array" {
					delete(array, "Value")
				} else {
					array["Value"].([]any)[0] = map[string]any{"WFItemType": uint64(5), "WFValue": map[string]any{}}
				}
			}
			if got, err := p10DecodeJSONDictionary(copy, resolved); err == nil {
				t.Fatalf("malformed native wire accepted: %s %#v", fault, got)
			}
		})
	}
}

func TestP10JSONOriginalNilRequestFailsPrecisely(t *testing.T) {
	for _, literal := range []string{"nil", "none"} {
		t.Run(literal, func(t *testing.T) {
			source := fmt.Sprintf(p10OriginalJSONLiteralSource, "http://127.0.0.1:1/unused")
			if literal != "nil" {
				source = strings.Replace(source, "\"empty_val\": nil", "\"empty_val\": "+literal, 1)
			}
			tc := p15OptionalTransportCase{name: "original_json_" + literal, source: source, target: literal, context: "Dictionary value"}
			diagnostics := p15TypeDiagnostics(t, source)
			if err := p15OptionalTransportDiagnostic(diagnostics, tc); err != nil {
				t.Fatal(err)
			}
			raw, err := CompileSourceToPlist("p10-original-json-"+literal+".cherri", source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeTargetValueShape) {
				t.Fatalf("original absent value was fabricated, omitted or emitted: %d %v", len(raw), err)
			}
			svc := service.NewService(nil)
			uri := "file:///p10-original-json-" + literal + ".cherri"
			svc.OpenDocument(uri, 1, source)
			editor, _ := svc.Analyze(uri)
			if err := p15OptionalTransportDiagnostic(editor, tc); err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"code", "span", "context"} {
				t.Run(fault, func(t *testing.T) {
					altered := append([]analysis.Diagnostic(nil), diagnostics...)
					for i := range altered {
						if altered[i].Severity != analysis.SeverityError {
							continue
						}
						switch fault {
						case "code":
							altered[i].Code = "E_UNRELATED"
						case "span":
							altered[i].Span.Start.ByteOffset++
						case "context":
							altered[i].Message = "unrelated failure"
						}
					}
					if p15OptionalTransportDiagnostic(altered, tc) == nil {
						t.Fatal("invalid rejection diagnostic accepted")
					}
				})
			}
		})
	}
}

// Bind the value to one actual producer and a unique native identity. Map
// cardinality alone cannot distinguish duplicated actions sharing one UUID.
func p10JSONModelBindings(actions []map[string]any) (map[string]any, error) {
	var resolved = map[string]any{}
	var identities = map[string]bool{}
	var producers int
	var producerPosition = -1
	var consumerPosition = -1
	for index, action := range actions {
		var identifier, valid = action["WFWorkflowActionIdentifier"].(string)
		if !valid || identifier == "" {
			return nil, fmt.Errorf("invalid actual action identifier")
		}
		// The canonical main Nothing action can omit its parameter envelope.
		// No other missing or malformed action shape supplies a binding.
		if _, present := action["WFWorkflowActionParameters"]; !present && identifier == "is.workflow.actions.nothing" {
			continue
		}
		var fields, ok = action["WFWorkflowActionParameters"].(map[string]any)
		if !ok || fields == nil {
			return nil, fmt.Errorf("invalid actual action parameter shape")
		}
		var uuid string
		if raw, present := fields["UUID"]; present {
			uuid, valid = raw.(string)
			if !valid || uuid == "" {
				return nil, fmt.Errorf("invalid actual action UUID metadata")
			}
			if identities[uuid] {
				return nil, fmt.Errorf("duplicated actual action UUID")
			}
			identities[uuid] = true
		}
		var outputName string
		if raw, present := fields["CustomOutputName"]; present {
			outputName, valid = raw.(string)
			if !valid {
				return nil, fmt.Errorf("invalid actual output name metadata")
			}
		}
		if identifier == "is.workflow.actions.downloadurl" && fields["WFHTTPBodyType"] == "JSON" {
			if consumerPosition >= 0 {
				return nil, fmt.Errorf("duplicated actual JSON consumer")
			}
			consumerPosition = index
		}
		if outputName != "dynamicModel" {
			continue
		}
		producers++
		producerPosition = index
		var text, literal = fields["WFTextActionText"].(string)
		if producers != 1 || uuid == "" || identifier != "is.workflow.actions.gettext" || !literal || text != "gpt-4o-mini" {
			return nil, fmt.Errorf("actual dynamic model producer changed or duplicated")
		}
		resolved["ActionOutput:"+uuid] = text
	}
	if producers != 1 {
		return nil, fmt.Errorf("missing actual dynamic model producer")
	}
	// This fixed source has one JSON consumer. A future producer cannot supply
	// its already executed request merely because the UUID exists somewhere.
	if consumerPosition < 0 || producerPosition >= consumerPosition {
		return nil, fmt.Errorf("actual model producer must precede its single JSON consumer")
	}
	return resolved, nil
}

func TestP10JSONProducerBindingIntegrity(t *testing.T) {
	source := strings.Replace(fmt.Sprintf(p10OriginalJSONLiteralSource, "http://127.0.0.1:1/unused"), "\"empty_val\": nil", "\"empty_val\": \"\"", 1)
	original := p15CompiledActions(t, source)
	expected, err := p10JSONModelBindings(original)
	if err != nil || len(expected) != 1 {
		t.Fatal("real source binding rejected", err)
	}
	model, consumer := -1, -1
	for i, action := range original {
		p := p15C01Params(action)
		if p["CustomOutputName"] == "dynamicModel" {
			model = i
		}
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.downloadurl" {
			consumer = i
		}
	}
	if model < 0 || consumer < 0 {
		t.Fatal("missing actual mutation target")
	}
	for _, fault := range []string{"duplicate_model_same_uuid", "duplicate_model_distinct_uuid", "duplicate_consumer_uuid", "remove_model", "wrong_kind", "changed_value", "wrong_name", "missing_uuid", "wrong_uuid_type", "empty_uuid", "consumer_uuid_type", "wrong_output_name_type", "missing_model_parameters", "missing_consumer_parameters", "malformed_action_identifier", "duplicate_nonmodel_uuid", "producer_after_consumer", "missing_json_consumer", "duplicate_json_consumer", "non_json_consumer"} {
		t.Run(fault, func(t *testing.T) {
			changed := p15ContainerCloneNative(t, original)
			if bindings, err := p10JSONModelBindings(changed); err != nil || !reflect.DeepEqual(bindings, expected) {
				t.Fatal("unchanged graph copy rejected", err)
			}
			fields := p15C01Params(changed[model])
			switch fault {
			case "duplicate_model_same_uuid", "duplicate_model_distinct_uuid":
				duplicate := p10CloneJSONWire(changed[model]).(map[string]any)
				if fault == "duplicate_model_distinct_uuid" {
					p15C01Params(duplicate)["UUID"] = "01f2bd81-53e5-4b99-9d50-4e84ea2a8ad5"
				}
				changed = append(changed, duplicate)
			case "duplicate_consumer_uuid":
				p15C01Params(changed[consumer])["UUID"] = fields["UUID"]
			case "remove_model":
				changed = append(changed[:model], changed[model+1:]...)
			case "wrong_kind":
				changed[model]["WFWorkflowActionIdentifier"] = "is.workflow.actions.number"
			case "changed_value":
				fields["WFTextActionText"] = "unrelated"
			case "wrong_name":
				fields["CustomOutputName"] = "unrelated"
			case "missing_uuid":
				delete(fields, "UUID")
			case "wrong_uuid_type":
				fields["UUID"] = 7
			case "empty_uuid":
				fields["UUID"] = ""
			case "consumer_uuid_type":
				p15C01Params(changed[consumer])["UUID"] = []any{"unreviewed"}
			case "wrong_output_name_type":
				p15C01Params(changed[consumer])["CustomOutputName"] = []any{"unreviewed"}
			case "missing_model_parameters":
				delete(changed[model], "WFWorkflowActionParameters")
			case "missing_consumer_parameters":
				delete(changed[consumer], "WFWorkflowActionParameters")
			case "malformed_action_identifier":
				changed[consumer]["WFWorkflowActionIdentifier"] = 7
			case "duplicate_nonmodel_uuid":
				changed = append(changed, p10CloneJSONWire(changed[consumer]).(map[string]any))
			case "missing_json_consumer":
				changed = append(changed[:consumer], changed[consumer+1:]...)
			case "duplicate_json_consumer":
				var duplicate = p10CloneJSONWire(changed[consumer]).(map[string]any)
				p15C01Params(duplicate)["UUID"] = "adebc120-169d-4cc9-9eab-91166f2b3275"
				changed = append(changed, duplicate)
			case "non_json_consumer":
				p15C01Params(changed[consumer])["WFHTTPBodyType"] = "File"
			case "producer_after_consumer":
				var producer = changed[model]
				changed = append(changed[:model], changed[model+1:]...)
				var movedConsumer = consumer
				if model < consumer {
					movedConsumer--
				}
				var insertion = movedConsumer + 1
				changed = append(changed, nil)
				copy(changed[insertion+1:], changed[insertion:])
				changed[insertion] = producer
			}
			if got, err := p10JSONModelBindings(changed); err == nil {
				t.Fatalf("corrupt producer binding accepted: %s %#v", fault, got)
			}
		})
	}
}

// Decoder-only controls distinguish a bound present empty value from no binding;
// these synthetic values are not Apple or compiler runtime observations.
func TestP10JSONPlainReferenceShape(t *testing.T) {
	var id = "db740247-8a09-480c-b7ec-11ff01483f13"
	for _, kind := range []string{"ActionOutput", "Variable"} {
		for _, tc := range []struct {
			name  string
			value any
		}{{"text", "value"}, {"empty_text", ""}, {"false", false}, {"zero", float64(0)}} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				var ref = map[string]any{"Type": kind}
				var identity = "source"
				if kind == "ActionOutput" {
					identity = id
					ref["OutputUUID"] = id
					ref["OutputName"] = "display name"
				} else {
					ref["VariableName"] = identity
				}
				var value, err = p10DecodeJSONReference(ref, map[string]any{kind + ":" + identity: tc.value})
				if err != nil || !reflect.DeepEqual(value, tc.value) {
					t.Fatalf("present plain value changed: %#v %v", value, err)
				}
			})
		}
	}
	for _, fault := range []string{"name_type", "empty_name", "extra_output_uuid", "extra_output_name", "property", "unknown_field", "missing_binding", "nil_binding"} {
		t.Run("variable_reject/"+fault, func(t *testing.T) {
			var ref = map[string]any{"Type": "Variable", "VariableName": "source"}
			var resolved = map[string]any{"Variable:source": ""}
			if value, err := p10DecodeJSONReference(ref, resolved); err != nil || value != "" {
				t.Fatal("unchanged plain reference rejected", value, err)
			}
			switch fault {
			case "name_type":
				ref["VariableName"] = 7
			case "empty_name":
				ref["VariableName"] = ""
			case "extra_output_uuid":
				ref["OutputUUID"] = id
			case "extra_output_name":
				ref["OutputName"] = "unreviewed"
			case "property":
				ref["Aggrandizements"] = []any{map[string]any{"Type": "WFPropertyVariableAggrandizement", "PropertyName": "Name"}}
			case "unknown_field":
				ref["UnknownMetadata"] = "unreviewed"
			case "missing_binding":
				delete(resolved, "Variable:source")
			case "nil_binding":
				resolved["Variable:source"] = nil
			}
			if value, err := p10DecodeJSONReference(ref, resolved); err == nil {
				t.Fatalf("unsupported variable reference accepted: %#v", value)
			}
		})
	}
}
