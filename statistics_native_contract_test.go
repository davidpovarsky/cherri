package main

import (
	"encoding/json"
	"os"
	"testing"
)

// This independent key comes from captured Apple parameter metadata, not the
// Cherri action declaration or another compiler's output. Live macOS 27
// WFCalculateStatisticsAction metadata confirmed it again on 2026-11-25.
func statisticAppleInputKey(t *testing.T) string {
	t.Helper()
	var raw, err = os.ReadFile("docs/apple-runtime/parameter-encodings.json")
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct {
		ActionParameters map[string][]struct {
			Key              string `json:"key"`
			Class            string `json:"class"`
			SingleStateClass string `json:"singleStateClass"`
		} `json:"actionParameters"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatal(err)
	}
	var key string
	for _, parameter := range snapshot.ActionParameters["is.workflow.actions.statistics"] {
		if parameter.Class != "WFVariablePickerParameter" {
			continue
		}
		if key != "" || parameter.SingleStateClass != "WFVariableParameterState" {
			t.Fatal("ambiguous or changed Apple statistics input contract")
		}
		key = parameter.Key
	}
	if key != "Input" {
		t.Fatalf("Apple statistics metadata input key = %q, want Input", key)
	}
	return key
}

func TestStatisticsInputMatchesAppleMetadata(t *testing.T) {
	var key = statisticAppleInputKey(t)
	for _, legacy := range []bool{true, false} {
		var name = "v2"
		var code = "let values = [3, 5, 7]\nstatistic(\"Sum\", input: values)\nstatistic(\"Average\", input: values)"
		if legacy {
			name = "legacy"
			code = "#include 'actions/math'\n@values = [3, 5, 7]\nstatistic(\"Sum\", @values)\nstatistic(\"Average\", @values)"
		}
		t.Run(name, func(t *testing.T) {
			var actions = compileDebugFixture(t, code, legacy)
			var stats = p15ActionsNamed(actions, "statistics")
			if len(stats) != 2 {
				t.Fatalf("got %d statistics actions, want 2", len(stats))
			}
			for i, operation := range []string{"Sum", "Average"} {
				var params = stats[i].WFWorkflowActionParameters
				if params["WFInput"] != nil || params[key] == nil || params["WFStatisticsOperation"] != operation {
					t.Fatalf("Apple statistics contract mismatch: %v", params)
				}
				var attachment, ok = params[key].(map[string]any)
				if !ok || attachment["WFSerializationType"] != "WFTextTokenAttachment" {
					t.Fatalf("statistics input lost its typed attachment: %T %v", params[key], params[key])
				}
				var value, valueOK = attachment["Value"].(map[string]any)
				if !valueOK {
					t.Fatalf("statistics input value is %T", attachment["Value"])
				}
				if legacy {
					if value["Type"] != "Variable" || value["VariableName"] != "values" {
						t.Fatalf("legacy statistics lost named input: %v", value)
					}
				} else {
					var lists = p15ActionsNamed(actions, "list")
					if len(lists) != 1 || value["Type"] != "ActionOutput" || value["OutputUUID"] != lists[0].WFWorkflowActionParameters["UUID"] {
						t.Fatalf("V2 statistics lost original numeric List producer: %v", value)
					}
				}
			}
		})
	}
}
