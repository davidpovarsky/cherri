package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"howett.net/plist"
)

func TestP15RawNativeResultBindings(t *testing.T) {
	var cases []struct {
		Name                    string
		Source                  string
		ErrorCode               string
		ExpectedExactParameters map[string]any
		AllowOpaqueInvalidWire  bool
	}
	if err := json.Unmarshal([]byte(rawNativeResultCasesJSON), &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			data, err := CompileSourceToPlist(tc.Name+".cherri", tc.Source)
			if tc.ErrorCode != "" {
				if err == nil || !strings.Contains(err.Error(), tc.ErrorCode) {
					t.Fatalf("expected %s for an actually consumed native result, got %v", tc.ErrorCode, err)
				}
				if len(data) != 0 {
					t.Fatal("invalid result returned serialized Shortcut bytes")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var workflow map[string]any
			if _, err := plist.Unmarshal(data, &workflow); err != nil {
				t.Fatal(err)
			}
			normalized, err := json.Marshal(workflow)
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(normalized, &workflow); err != nil {
				t.Fatal(err)
			}
			actions, ok := workflow["WFWorkflowActions"].([]any)
			if !ok || len(actions) == 0 {
				t.Fatal("no actual native actions")
			}
			first := actions[0].(map[string]any)["WFWorkflowActionParameters"]
			if tc.ExpectedExactParameters != nil && !reflect.DeepEqual(first, tc.ExpectedExactParameters) {
				t.Fatalf("exact raw parameters changed: got %#v want %#v", first, tc.ExpectedExactParameters)
			}
			producers := map[string]int{}
			for _, item := range actions {
				a := item.(map[string]any)
				p, _ := a["WFWorkflowActionParameters"].(map[string]any)
				if id, ok := p["UUID"].(string); ok && id != "" {
					producers[id]++
				}
			}
			var references []string
			var walk func(any)
			walk = func(value any) {
				switch v := value.(type) {
				case map[string]any:
					if v["Type"] == "ActionOutput" {
						if id, ok := v["OutputUUID"].(string); ok {
							references = append(references, id)
						}
					}
					for key, child := range v {
						if key != "WFWorkflow" {
							walk(child)
						}
					}
				case []any:
					for _, child := range v {
						walk(child)
					}
				}
			}
			walk(actions)
			if !tc.AllowOpaqueInvalidWire {
				for _, id := range references {
					if producers[id] != 1 {
						t.Fatalf("source-generated result does not resolve to exactly one native producer: %q count=%d", id, producers[id])
					}
				}
			}
			if tc.Name == "plain-outputuuid-used" || tc.Name == "exact-explicit-uuid-used" {
				if len(references) != 1 || references[0] != "11111111-1111-4111-8111-111111111111" {
					t.Fatalf("explicit UUID was not preserved in the consumer: %v", references)
				}
			}
			if strings.Contains(tc.Name, "unused") && tc.Name != "exact-var-unused" && !strings.Contains(tc.Name, "explicit-opaque") && len(references) != 0 {
				t.Fatalf("unused binding added a consumer: %v", references)
			}
			if tc.Name == "opaque-workflow-duplicate-dangling-control" {
				if producers["11111111-1111-4111-8111-111111111111"] != 2 || len(references) != 1 || references[0] != "MISSING-IMPORTED-UUID" {
					t.Fatal("opaque SOURCE_INVALID wire was changed")
				}
			}
		})
	}
}

const rawNativeResultCasesJSON = `[
  {
    "name": "plain-let-used",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: {\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\"})\noutput(x)\n",
    "errorCode": "",
    "expectedExactParameters": null,
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-let-unused",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\"}))\n",
    "errorCode": "",
    "expectedExactParameters": {
      "WFTextActionText": "SYNTHETIC_RAW_VALUE"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-let-used",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\"}))\noutput(x)\n",
    "errorCode": "NATIVE_OUTPUT_ID_MISSING",
    "expectedExactParameters": {
      "WFTextActionText": "SYNTHETIC_RAW_VALUE"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-alias-unused",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\"}))\nlet y = x\n",
    "errorCode": "",
    "expectedExactParameters": {
      "WFTextActionText": "SYNTHETIC_RAW_VALUE"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-alias-used",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\"}))\nlet y = x\noutput(y)\n",
    "errorCode": "NATIVE_OUTPUT_ID_MISSING",
    "expectedExactParameters": {
      "WFTextActionText": "SYNTHETIC_RAW_VALUE"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-var-unused",
    "source": "var x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\"}))\n",
    "errorCode": "NATIVE_OUTPUT_ID_MISSING",
    "expectedExactParameters": {
      "WFTextActionText": "SYNTHETIC_RAW_VALUE"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-inline-used",
    "source": "output(native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\"})))\n",
    "errorCode": "NATIVE_OUTPUT_ID_MISSING",
    "expectedExactParameters": {
      "WFTextActionText": "SYNTHETIC_RAW_VALUE"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "plain-inline-used",
    "source": "output(native.action(\"is.workflow.actions.gettext\", parameters: {\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\"}))\n",
    "errorCode": "",
    "expectedExactParameters": null,
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-explicit-uuid-used",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}))\noutput(x)\n",
    "errorCode": "",
    "expectedExactParameters": {
      "CustomOutputName": "NativeName",
      "UUID": "11111111-1111-4111-8111-111111111111",
      "WFTextActionText": "SYNTHETIC_RAW_VALUE"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-explicit-uuid-unused",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}))\n",
    "errorCode": "",
    "expectedExactParameters": {
      "CustomOutputName": "NativeName",
      "UUID": "11111111-1111-4111-8111-111111111111",
      "WFTextActionText": "SYNTHETIC_RAW_VALUE"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-empty-uuid-used",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"\"}))\noutput(x)\n",
    "errorCode": "NATIVE_OUTPUT_ID_MISSING",
    "expectedExactParameters": {
      "UUID": "",
      "WFTextActionText": "SYNTHETIC_RAW_VALUE"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-numeric-uuid-unused",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":17}))\n",
    "errorCode": "",
    "expectedExactParameters": {
      "UUID": 17,
      "WFTextActionText": "SYNTHETIC_RAW_VALUE"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-numeric-uuid-used",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":17}))\noutput(x)\n",
    "errorCode": "NATIVE_OUTPUT_ID_MISSING",
    "expectedExactParameters": {
      "UUID": 17,
      "WFTextActionText": "SYNTHETIC_RAW_VALUE"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "plain-outputuuid-used",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: {\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\"}, outputUUID: \"11111111-1111-4111-8111-111111111111\")\noutput(x)\n",
    "errorCode": "",
    "expectedExactParameters": null,
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-arg-uuid-used",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\"}), outputUUID: \"11111111-1111-4111-8111-111111111111\")\noutput(x)\n",
    "errorCode": "NATIVE_OUTPUT_IDENTITY_CONFLICT",
    "expectedExactParameters": {
      "WFTextActionText": "SYNTHETIC_RAW_VALUE"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-conflicting-uuid-used",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}), outputUUID: \"22222222-2222-4222-8222-222222222222\")\noutput(x)\n",
    "errorCode": "NATIVE_OUTPUT_IDENTITY_CONFLICT",
    "expectedExactParameters": {
      "CustomOutputName": "NativeName",
      "UUID": "11111111-1111-4111-8111-111111111111",
      "WFTextActionText": "SYNTHETIC_RAW_VALUE"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-duplicate-unused",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}))\nnative.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}))\n",
    "errorCode": "",
    "expectedExactParameters": {
      "CustomOutputName": "NativeName",
      "UUID": "11111111-1111-4111-8111-111111111111",
      "WFTextActionText": "SYNTHETIC_RAW_VALUE"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-duplicate-used",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}))\nnative.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}))\noutput(x)\n",
    "errorCode": "NATIVE_OUTPUT_AMBIGUOUS",
    "expectedExactParameters": {
      "CustomOutputName": "NativeName",
      "UUID": "11111111-1111-4111-8111-111111111111",
      "WFTextActionText": "SYNTHETIC_RAW_VALUE"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-explicit-ref-control",
    "source": "native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}))\noutput(native.ref(\"ActionOutput\", uuid: \"11111111-1111-4111-8111-111111111111\", outputName: \"NativeName\"))\n",
    "errorCode": "",
    "expectedExactParameters": {
      "CustomOutputName": "NativeName",
      "UUID": "11111111-1111-4111-8111-111111111111",
      "WFTextActionText": "SYNTHETIC_RAW_VALUE"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-opaque-embedded-control",
    "source": "native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"WFWorkflow\":{\"WFWorkflowActions\":[{\"WFWorkflowActionIdentifier\":\"is.workflow.actions.gettext\",\"WFWorkflowActionParameters\":{\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}},{\"WFWorkflowActionIdentifier\":\"is.workflow.actions.gettext\",\"WFWorkflowActionParameters\":{\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}}]}}))\n",
    "errorCode": "",
    "expectedExactParameters": {
      "WFTextActionText": "SYNTHETIC_RAW_VALUE",
      "WFWorkflow": {
        "WFWorkflowActions": [
          {
            "WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
            "WFWorkflowActionParameters": {
              "CustomOutputName": "NativeName",
              "UUID": "11111111-1111-4111-8111-111111111111",
              "WFTextActionText": "SYNTHETIC_RAW_VALUE"
            }
          },
          {
            "WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
            "WFWorkflowActionParameters": {
              "CustomOutputName": "NativeName",
              "UUID": "11111111-1111-4111-8111-111111111111",
              "WFTextActionText": "SYNTHETIC_RAW_VALUE"
            }
          }
        ]
      }
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "opaque-workflow-duplicate-dangling-control",
    "source": "native.workflow(native.plist({\"WFWorkflowName\":\"SYNTHETIC_RAW_OPAQUE\",\"WFWorkflowActions\":[{\"WFWorkflowActionIdentifier\":\"is.workflow.actions.gettext\",\"WFWorkflowActionParameters\":{\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}},{\"WFWorkflowActionIdentifier\":\"is.workflow.actions.output\",\"WFWorkflowActionParameters\":{\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"WFInput\":{\"WFSerializationType\":\"WFTextTokenAttachment\",\"Value\":{\"Type\":\"ActionOutput\",\"OutputUUID\":\"MISSING-IMPORTED-UUID\"}}}}]}), exact: true)\n",
    "errorCode": "",
    "expectedExactParameters": null,
    "allowOpaqueInvalidWire": true
  },
  {
    "name": "unused-binding-with-explicit-opaque-duplicate-reference",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}))\nnative.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}))\noutput(native.ref(\"ActionOutput\", uuid: \"11111111-1111-4111-8111-111111111111\"))\n",
    "errorCode": "",
    "expectedExactParameters": {
      "WFTextActionText": "SYNTHETIC_RAW_VALUE",
      "UUID": "11111111-1111-4111-8111-111111111111",
      "CustomOutputName": "NativeName"
    },
    "allowOpaqueInvalidWire": true
  },
  {
    "name": "header-cannot-bypass-consumed-missing-result",
    "source": "native.workflow(native.plist({\"WFWorkflowName\":\"SYNTHETIC_HEADER\",\"WFWorkflowActions\":[]}), exact: true)\nlet x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\"}))\noutput(x)\n",
    "errorCode": "NATIVE_OUTPUT_ID_MISSING",
    "expectedExactParameters": null,
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-fstring-used",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}))\noutput(f\"result={x}\")\n",
    "errorCode": "",
    "expectedExactParameters": {
      "WFTextActionText": "SYNTHETIC_RAW_VALUE",
      "UUID": "11111111-1111-4111-8111-111111111111",
      "CustomOutputName": "NativeName"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-dictionary-used",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}))\nlet data = {\"value\": x}\nlet total = count(data)\noutput(f\"{total}\")\n",
    "errorCode": "",
    "expectedExactParameters": {
      "WFTextActionText": "SYNTHETIC_RAW_VALUE",
      "UUID": "11111111-1111-4111-8111-111111111111",
      "CustomOutputName": "NativeName"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-list-used",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}))\nlet data = [x]\nlet total = count(data)\noutput(f\"{total}\")\n",
    "errorCode": "",
    "expectedExactParameters": {
      "WFTextActionText": "SYNTHETIC_RAW_VALUE",
      "UUID": "11111111-1111-4111-8111-111111111111",
      "CustomOutputName": "NativeName"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-mutable-used",
    "source": "var x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}))\noutput(x)\n",
    "errorCode": "",
    "expectedExactParameters": {
      "WFTextActionText": "SYNTHETIC_RAW_VALUE",
      "UUID": "11111111-1111-4111-8111-111111111111",
      "CustomOutputName": "NativeName"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-alias-valid-used",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}))\nlet y = x\noutput(y)\n",
    "errorCode": "",
    "expectedExactParameters": {
      "WFTextActionText": "SYNTHETIC_RAW_VALUE",
      "UUID": "11111111-1111-4111-8111-111111111111",
      "CustomOutputName": "NativeName"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-late-duplicate",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}))\nlet captured = text(f\"{x}\")\nnative.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}))\noutput(captured)\n",
    "errorCode": "NATIVE_OUTPUT_AMBIGUOUS",
    "expectedExactParameters": {
      "WFTextActionText": "SYNTHETIC_RAW_VALUE",
      "UUID": "11111111-1111-4111-8111-111111111111",
      "CustomOutputName": "NativeName"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-explicit-ref-alias-preserves-raw-name",
    "source": "native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}))\nlet captured = native.ref(\"ActionOutput\", uuid: \"11111111-1111-4111-8111-111111111111\")\noutput(captured)\n",
    "errorCode": "",
    "expectedExactParameters": {
      "WFTextActionText": "SYNTHETIC_RAW_VALUE",
      "UUID": "11111111-1111-4111-8111-111111111111",
      "CustomOutputName": "NativeName"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-date-backfill-keeps-output-identity",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"11111111-1111-4111-8111-111111111111\",\"CustomOutputName\":\"NativeName\"}))\ndebugLog()\ndebugShowLogs(withDates: true)\noutput(x)\n",
    "errorCode": "",
    "expectedExactParameters": {
      "WFTextActionText": "SYNTHETIC_RAW_VALUE",
      "UUID": "11111111-1111-4111-8111-111111111111",
      "CustomOutputName": "NativeName"
    },
    "allowOpaqueInvalidWire": false
  },
  {
    "name": "exact-lowercase-uuid-used",
    "source": "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\":\"SYNTHETIC_RAW_VALUE\",\"UUID\":\"deadbeef-dead-4eef-aead-beef00000001\",\"CustomOutputName\":\"NativeName\"}))\noutput(x)\n",
    "errorCode": "",
    "expectedExactParameters": {
      "WFTextActionText": "SYNTHETIC_RAW_VALUE",
      "UUID": "deadbeef-dead-4eef-aead-beef00000001",
      "CustomOutputName": "NativeName"
    },
    "allowOpaqueInvalidWire": false
  }
]`

func TestP15RawNativeResultTypedTraversalPreservesOpaqueValues(t *testing.T) {
	session := NewCanonicalBackendSession()
	defer session.Close()
	params := map[string]any{"UUID": "native-producer", "CustomOutputName": "Original"}
	if err := session.EmitExactRawAction("is.workflow.actions.gettext", params, true); err != nil {
		t.Fatal(err)
	}
	if err := session.BindNativeActionResult("private-result", "unused-generated", true, false, ""); err != nil {
		t.Fatal(err)
	}
	original := map[string]any{
		"large":            uint64(18446744073709551615),
		"negative":         int64(-42),
		"zero":             int64(0),
		"false":            false,
		"bytes":            []byte{0, 1, 255},
		"date":             time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC),
		"emptyMap":         map[string]any{},
		"emptyList":        []any{},
		"opaqueUUIDFields": map[string]any{"UUID": "private-result", "OutputUUID": "private-result"},
		"token": WFTextTokenString{
			WFSerializationType: "WFTextTokenString",
			Value: WFTextTokenStringValue{
				String:             "\uFFFC",
				AttachmentsByRange: map[string]Value{"{0, 1}": {Type: "ActionOutput", OutputUUID: "private-result", OutputName: "private-result"}},
			},
		},
	}
	resolved, err := session.resolveNativeActionResultValue(original)
	if err != nil {
		t.Fatal(err)
	}
	got := resolved.(map[string]any)
	for key, value := range original {
		if key != "token" && !reflect.DeepEqual(got[key], value) {
			t.Fatalf("opaque %s changed Go representation: %#v -> %#v", key, value, got[key])
		}
	}
	token := got["token"].(WFTextTokenString).Value.AttachmentsByRange["{0, 1}"]
	if token.Type != "ActionOutput" || token.OutputUUID != "native-producer" || token.OutputName != "native-producer" {
		t.Fatalf("typed reference was not resolved: %#v", token)
	}
	if original["token"].(WFTextTokenString).Value.AttachmentsByRange["{0, 1}"].OutputUUID != "private-result" {
		t.Fatal("resolving copied arguments mutated the original reference")
	}
	if !reflect.DeepEqual(session.shortcut.WFWorkflowActions[0].WFWorkflowActionParameters, params) {
		t.Fatal("exact producer parameters mutated during consumption")
	}
}

func TestP15RawNativeResultFinalizationRejectsIdentityCorruption(t *testing.T) {
	for _, mutation := range []string{"late-duplicate", "changed-producer", "opaque-private-reference"} {
		t.Run(mutation, func(t *testing.T) {
			session := NewCanonicalBackendSession()
			defer session.Close()
			if err := session.EmitExactRawAction("is.workflow.actions.gettext", map[string]any{"UUID": "native-producer", "WFTextActionText": "synthetic"}, true); err != nil {
				t.Fatal(err)
			}
			if err := session.BindNativeActionResult("private-result", "unused-generated", true, false, ""); err != nil {
				t.Fatal(err)
			}
			ref := WFTextTokenAttachment{WFSerializationType: "WFTextTokenAttachment", Value: Value{Type: "ActionOutput", OutputUUID: "private-result"}}
			expected := ""
			if mutation == "opaque-private-reference" {
				expected = "NATIVE_OUTPUT_REFERENCE_UNRESOLVED"
				if err := session.EmitExactRawAction("com.example.synthetic.consumer", map[string]any{"WFInput": ref}, true); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := session.EmitRawAction("is.workflow.actions.output", map[string]any{"WFOutput": ref}, "", "", ""); err != nil {
					t.Fatal(err)
				}
				switch mutation {
				case "late-duplicate":
					expected = "NATIVE_OUTPUT_AMBIGUOUS"
					if err := session.EmitExactRawAction("is.workflow.actions.gettext", map[string]any{"UUID": "native-producer"}, true); err != nil {
						t.Fatal(err)
					}
				case "changed-producer":
					expected = "NATIVE_OUTPUT_IDENTITY_CONFLICT"
					session.shortcut.WFWorkflowActions[0].WFWorkflowActionParameters["UUID"] = "changed-after-consumption"
				}
			}
			data, err := session.Finalize()
			if err == nil || !strings.Contains(err.Error(), expected) || len(data) != 0 {
				t.Fatalf("identity corruption was not rejected with %s: %v", expected, err)
			}
		})
	}
}

func TestP15RawNativeResultReleaseRequiresAnEmittedConsumer(t *testing.T) {
	prefix := "let x = native.action(\"is.workflow.actions.gettext\", parameters: native.plist({\"WFTextActionText\": \"synthetic\"}))\n"
	for _, tc := range []struct {
		name      string
		profile   BuildProfile
		suffix    string
		errorCode string
	}{
		{"debug-consumer", BuildProfileDebug, "debugTrace(x)\n", "NATIVE_OUTPUT_ID_MISSING"},
		{"release-pruned-consumer", BuildProfileRelease, "debugTrace(x)\n", ""},
		{"release-real-consumer-remains", BuildProfileRelease, "debugTrace(x)\noutput(x)\n", "NATIVE_OUTPUT_ID_MISSING"},
		{"release-does-not-prune-require", BuildProfileRelease, "require(x, type: \"text\")\n", "NATIVE_OUTPUT_ID_MISSING"},
		{"release-rejects-unproved-condition", BuildProfileRelease, "require(x)\n", "E_ARGUMENT_TYPE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := CompileSourceToPlistWithOptions(tc.name+".cherri", prefix+tc.suffix, CompileOptions{BuildProfile: tc.profile})
			if tc.errorCode != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errorCode) || len(data) != 0 {
					t.Fatalf("expected %s, got %v", tc.errorCode, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var workflow map[string]any
			if _, err := plist.Unmarshal(data, &workflow); err != nil {
				t.Fatal(err)
			}
			actions := workflow["WFWorkflowActions"].([]any)
			if err := p15M02MainVoidRowsError(actions); err != nil {
				t.Fatal(err)
			}
			if len(actions) != 2 || actions[0].(map[string]any)["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" {
				t.Fatalf("Release changed raw producer side effects or retained the removed consumer: %d actions", len(actions))
			}
			params := actions[0].(map[string]any)["WFWorkflowActionParameters"]
			if !reflect.DeepEqual(params, map[string]any{"WFTextActionText": "synthetic"}) {
				t.Fatalf("unused exact raw producer changed: %#v", params)
			}
		})
	}
}
