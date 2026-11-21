package main

import (
	"fmt"
	"reflect"
	"regexp"
	"testing"

	"howett.net/plist"
)

var p15K05UUIDPattern = regexp.MustCompile("^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$")

// This public fixture has one Text producer, one Show Result consumer and the
// exact bare V2 implicit-Void terminator. Compare every native action and field,
// deriving only the unpredictable producer UUID from the actual output.
func p15K05AuthoredNativeGraph(raw []byte, expectedText string) (int, error) {
	var document map[string]any
	if _, err := plist.Unmarshal(raw, &document); err != nil {
		return 0, fmt.Errorf("K05 native document: %w", err)
	}
	actions, ok := document["WFWorkflowActions"].([]any)
	if !ok || len(actions) != 3 {
		return 0, fmt.Errorf("K05 expected exact Text/Show Result/bare Nothing graph, got %d actions", len(actions))
	}
	producer, ok := actions[0].(map[string]any)
	if !ok {
		return 0, fmt.Errorf("K05 Text producer is not a native action")
	}
	parameters, ok := producer["WFWorkflowActionParameters"].(map[string]any)
	if !ok {
		return 0, fmt.Errorf("K05 Text producer has no native parameters")
	}
	uuid, ok := parameters["UUID"].(string)
	if !ok || !p15K05UUIDPattern.MatchString(uuid) || uuid == "00000000-0000-0000-0000-000000000000" {
		return 0, fmt.Errorf("K05 Text producer has no valid nonzero UUID")
	}
	expected := []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"CustomOutputName": "message", "UUID": uuid, "WFTextActionText": expectedText,
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.showresult",
			"WFWorkflowActionParameters": map[string]any{
				"Text": map[string]any{
					"WFSerializationType": "WFTextTokenString",
					"Value": map[string]any{
						"string": "\ufffc",
						"attachmentsByRange": map[string]any{
							"{0, 1}": map[string]any{
								"OutputName": "message", "OutputUUID": uuid, "Type": "ActionOutput",
							},
						},
					},
				},
			},
		},
		map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"},
	}
	if !reflect.DeepEqual(actions, expected) {
		return 0, fmt.Errorf("K05 native graph changed action order, fields, value, unique producer or causal Show Result reference")
	}
	return len(actions), nil
}

// Mutate the actual successful wrapper output, so each guard is demonstrated on
// the same source and native fixture used by the preservation test.
func p15K05AssertNativeGraphControls(t *testing.T, original []byte) {
	t.Helper()
	const expectedText = "P15_K05_ORIGINAL"
	clone := func() map[string]any {
		var document map[string]any
		if _, err := plist.Unmarshal(original, &document); err != nil {
			t.Fatal(err)
		}
		return document
	}
	encode := func(document map[string]any) []byte {
		raw, err := plist.Marshal(document, plist.XMLFormat)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	consumerAttachment := func(actions []any) map[string]any {
		return actions[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["Text"].(map[string]any)["Value"].(map[string]any)["attachmentsByRange"].(map[string]any)["{0, 1}"].(map[string]any)
	}
	t.Run("oracle_consistent_new_uuid", func(t *testing.T) {
		document := clone()
		actions := document["WFWorkflowActions"].([]any)
		const alternate = "7B0F2D68-491A-4D3E-BBC5-063D74C94128"
		actions[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["UUID"] = alternate
		consumerAttachment(actions)["OutputUUID"] = alternate
		if _, err := p15K05AuthoredNativeGraph(encode(document), expectedText); err != nil {
			t.Fatal("consistent producer/reference identity was rejected", err)
		}
	})
	for _, fault := range []string{
		"missing_terminator", "extra_terminator", "malformed_terminator", "terminator_has_uuid",
		"reordered_actions", "wrong_consumer", "detached_consumer_uuid", "missing_consumer_uuid",
		"wrong_output_name", "wrong_reference_span", "coerced_consumer", "changed_token_text",
		"changed_source_text", "wrong_producer_kind", "missing_producer_uuid", "malformed_producer_uuid",
		"zero_producer_uuid", "duplicate_producer", "duplicate_uuid_on_consumer",
		"extra_producer_field", "extra_consumer_field",
	} {
		t.Run("oracle_"+fault, func(t *testing.T) {
			document := clone()
			actions := document["WFWorkflowActions"].([]any)
			producer := actions[0].(map[string]any)
			producerParameters := producer["WFWorkflowActionParameters"].(map[string]any)
			consumer := actions[1].(map[string]any)
			consumerParameters := consumer["WFWorkflowActionParameters"].(map[string]any)
			token := consumerParameters["Text"].(map[string]any)["Value"].(map[string]any)
			switch fault {
			case "missing_terminator":
				document["WFWorkflowActions"] = actions[:2]
			case "extra_terminator":
				document["WFWorkflowActions"] = append(actions, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
			case "malformed_terminator":
				actions[2].(map[string]any)["WFWorkflowActionParameters"] = map[string]any{}
			case "terminator_has_uuid":
				actions[2].(map[string]any)["UUID"] = producerParameters["UUID"]
			case "reordered_actions":
				actions[0], actions[1] = actions[1], actions[0]
			case "wrong_consumer":
				consumer["WFWorkflowActionIdentifier"] = "is.workflow.actions.getvariable"
			case "detached_consumer_uuid":
				consumerAttachment(actions)["OutputUUID"] = "11111111-2222-4333-8444-555555555555"
			case "missing_consumer_uuid":
				delete(consumerAttachment(actions), "OutputUUID")
			case "wrong_output_name":
				consumerAttachment(actions)["OutputName"] = "other"
			case "wrong_reference_span":
				ranges := token["attachmentsByRange"].(map[string]any)
				ranges["{1, 1}"] = ranges["{0, 1}"]
				delete(ranges, "{0, 1}")
			case "coerced_consumer":
				consumerParameters["Text"] = expectedText
			case "changed_token_text":
				token["string"] = "changed \ufffc"
			case "changed_source_text":
				producerParameters["WFTextActionText"] = "P15_K05_CHANGED"
			case "wrong_producer_kind":
				producer["WFWorkflowActionIdentifier"] = "is.workflow.actions.number"
			case "missing_producer_uuid":
				delete(producerParameters, "UUID")
			case "malformed_producer_uuid":
				producerParameters["UUID"] = "not-a-uuid"
			case "zero_producer_uuid":
				producerParameters["UUID"] = "00000000-0000-0000-0000-000000000000"
				consumerAttachment(actions)["OutputUUID"] = producerParameters["UUID"]
			case "duplicate_producer":
				document["WFWorkflowActions"] = append([]any{producer}, actions...)
			case "duplicate_uuid_on_consumer":
				consumerParameters["UUID"] = producerParameters["UUID"]
			case "extra_producer_field":
				producerParameters["Unexpected"] = true
			case "extra_consumer_field":
				consumerParameters["Unexpected"] = true
			}
			broken := encode(document)
			if reflect.DeepEqual(document, clone()) {
				t.Fatal("control did not alter the actual native graph")
			}
			if _, err := p15K05AuthoredNativeGraph(broken, expectedText); err == nil {
				t.Fatal("accepted corrupted actual K05 native graph")
			}
		})
	}
}
