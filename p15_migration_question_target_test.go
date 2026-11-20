package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/electrikmilk/cherri/internal/shortcutcompare"
	"howett.net/plist"
)

func p15RP33QuestionTargets(t *testing.T, paths ...string) {
	t.Helper()
	for ordinal, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]any
		if _, err := plist.Unmarshal(raw, &document); err != nil {
			t.Fatal(err)
		}
		actions, okay := document["WFWorkflowActions"].([]any)
		if !okay {
			t.Fatal("missing actual action array")
		}
		questions, okay := document["WFWorkflowImportQuestions"].([]any)
		if !okay || len(questions) != 1 {
			t.Fatal("missing unique import question", document["WFWorkflowImportQuestions"])
		}
		question := questions[0].(map[string]any)
		target, err := strconv.Atoi(fmt.Sprint(question["ActionIndex"]))
		if err != nil || target < 0 || target >= len(actions) {
			t.Fatal("question target is not a valid raw native action index", question)
		}
		wantIndex, found := -1, 0
		for index, rawAction := range actions {
			action := rawAction.(map[string]any)
			parameters, _ := action["WFWorkflowActionParameters"].(map[string]any)
			if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.alert" && parameters["WFAlertActionTitle"] == "RP33 setup" {
				if parameters["WFAlertActionMessage"] != "RP33" {
					t.Fatal("setup default changed", parameters)
				}
				wantIndex, found = index, found+1
			}
		}
		if found != 1 || target != wantIndex || question["ParameterKey"] != "WFAlertActionMessage" || question["Text"] != "Recipient?" || question["DefaultValue"] != "RP33" {
			t.Fatalf("question is detached from the original declared target: target=%d expected=%d matches=%d question=%#v", target, wantIndex, found, question)
		}
		encoded, _ := json.Marshal(map[string]any{"document": ordinal, "raw_action_count": len(actions), "actual_index": target, "expected_index": wantIndex, "parameter": question["ParameterKey"], "prompt": question["Text"], "default": question["DefaultValue"]})
		t.Log("RP33_RAW_QUESTION_TARGET:" + string(encoded))
		artifact, _ := json.Marshal(map[string]any{"document": ordinal, "sha256": fmt.Sprintf("%x", sha256.Sum256(raw)), "bytes": len(raw), "base64": base64.StdEncoding.EncodeToString(raw)})
		t.Log("RP33_NATIVE_SOURCE_ARTIFACT:" + string(artifact))
		for _, fault := range []string{"target_zero", "target_previous", "target_next", "parameter_key", "prompt", "default"} {
			t.Run(fmt.Sprintf("question_document_%d/%s", ordinal, fault), func(t *testing.T) {
				var broken map[string]any
				if _, err := plist.Unmarshal(raw, &broken); err != nil {
					t.Fatal(err)
				}
				q := broken["WFWorkflowImportQuestions"].([]any)[0].(map[string]any)
				switch fault {
				case "target_zero":
					q["ActionIndex"] = 0
				case "target_previous":
					q["ActionIndex"] = target - 1
				case "target_next":
					q["ActionIndex"] = target + 1
				case "parameter_key":
					q["ParameterKey"] = "WFAlertActionTitle"
				case "prompt":
					q["Text"] = "Changed prompt"
				case "default":
					q["DefaultValue"] = "Changed default"
				}
				if shortcutcompare.CompareDocumentsNormalized(document, broken).Equal {
					t.Fatal("native question corruption was normalized away", fault)
				}
			})
		}
	}
}
