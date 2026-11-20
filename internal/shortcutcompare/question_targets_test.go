package shortcutcompare

import (
	"reflect"
	"testing"
)

func parityQuestionFixture(withForwarding bool, target int) map[string]any {
	text := map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.gettext", "WFWorkflowActionParameters": map[string]any{"UUID": "source-text", "WFTextActionText": "public"}}
	alert := func() any {
		return map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.alert", "WFWorkflowActionParameters": map[string]any{"WFAlertActionMessage": "same", "WFAlertActionTitle": "same"}}
	}
	actions := []any{text}
	if withForwarding {
		actions = append(actions, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable", "WFWorkflowActionParameters": map[string]any{
			"WFVariableName": "captured", "WFInput": map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "ActionOutput", "OutputUUID": "source-text"}},
		}})
	}
	actions = append(actions, alert(), alert())
	return map[string]any{"WFWorkflowActions": actions, "WFWorkflowImportQuestions": []any{map[string]any{"ActionIndex": target, "ParameterKey": "WFAlertActionMessage", "Text": "Prompt?", "DefaultValue": "same"}}}
}

func TestParityImportQuestionRetainsExactActionIdentity(t *testing.T) {
	before, after := parityQuestionFixture(true, 3), parityQuestionFixture(false, 2)
	beforeCopy, afterCopy := deepCloneMap(before), deepCloneMap(after)
	if result := CompareDocumentsNormalized(before, after); !result.Equal {
		t.Fatalf("equivalent question target was detached by action filtering: %+v", result.Differences)
	}
	if !reflect.DeepEqual(before, beforeCopy) || !reflect.DeepEqual(after, afterCopy) {
		t.Fatal("normalization mutated source documents")
	}
	for _, fault := range []string{"identical_first_target", "target_zero", "parameter", "prompt", "default", "missing_question"} {
		t.Run(fault, func(t *testing.T) {
			broken := deepCloneMap(after)
			question := broken["WFWorkflowImportQuestions"].([]any)[0].(map[string]any)
			switch fault {
			case "identical_first_target":
				question["ActionIndex"] = 1
			case "target_zero":
				question["ActionIndex"] = 0
			case "parameter":
				question["ParameterKey"] = "WFAlertActionTitle"
			case "prompt":
				question["Text"] = "Changed"
			case "default":
				question["DefaultValue"] = "Changed"
			case "missing_question":
				delete(broken, "WFWorkflowImportQuestions")
			}
			if CompareDocumentsNormalized(before, broken).Equal {
				t.Fatal("question mutation was erased", fault)
			}
		})
	}
}

func TestParityImportQuestionZeroAndOriginCollision(t *testing.T) {
	before, after := parityQuestionFixture(true, 0), parityQuestionFixture(false, 0)
	for _, document := range []map[string]any{before, after} {
		document["__cherri_parity_question_origin"] = "authored top level"
		document["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["__cherri_parity_question_origin_"] = "authored nested"
	}
	if result := CompareDocumentsNormalized(before, after); !result.Equal {
		t.Fatal("index-zero target changed", result.Differences)
	}
	normalized := NormalizeDocumentForParity(before)
	if normalized["__cherri_parity_question_origin"] != "authored top level" {
		t.Fatal("authored colliding metadata removed")
	}
	params := normalized["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	if params["__cherri_parity_question_origin_"] != "authored nested" {
		t.Fatal("authored nested metadata removed")
	}
	q := normalized["WFWorkflowImportQuestions"].([]any)[0].(map[string]any)
	if index, okay := toNumeric(q["ActionIndex"]); !okay || index != 0 {
		t.Fatal("zero was treated as absent", q)
	}
}

func TestParityImportQuestionRemovedOrInvalidTargetStaysRaw(t *testing.T) {
	for _, target := range []any{1, -1, 100, 0.5, "0"} {
		t.Run(reflect.TypeOf(target).String()+"_"+fmtQuestionTarget(target), func(t *testing.T) {
			document := parityQuestionFixture(true, 3)
			document["WFWorkflowImportQuestions"].([]any)[0].(map[string]any)["ActionIndex"] = target
			before := deepCloneMap(document)
			if got := NormalizeDocumentForParity(document); !reflect.DeepEqual(got, before) {
				t.Fatal("removed/invalid target silently received a different action", target, got)
			}
		})
	}
}

func fmtQuestionTarget(value any) string {
	switch v := value.(type) {
	case int:
		if v == 1 {
			return "removed"
		}
		if v < 0 {
			return "negative"
		}
		return "outside"
	case float64:
		return "fractional"
	default:
		return "text"
	}
}

func TestParityImportQuestionAmbiguousTargetFailsClosed(t *testing.T) {
	document := parityQuestionFixture(true, 3)
	actions := document["WFWorkflowActions"].([]any)
	origins, okay := bindParityQuestionTargets(document, actions)
	if !okay {
		t.Fatal("valid raw target could not be witnessed")
	}
	duplicated := append(append([]any{}, actions...), deepCloneValue(actions[3]))
	if rebindParityQuestionTargets(document, duplicated, origins) {
		t.Fatal("duplicated action origin was accepted as a unique target")
	}
	question := document["WFWorkflowImportQuestions"].([]any)[0].(map[string]any)
	if question["ActionIndex"] != 3 {
		t.Fatal("failed remap partially rewrote the question")
	}
}
