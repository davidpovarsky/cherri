package main

import (
	"strings"
	"testing"
)

// These cases exercise actual unsigned production output. They do not execute
// Apple actions or change the independent N03 comparison/oracle controls.
func p15N03ExplicitUniqueAction(t *testing.T, actions []map[string]any, identifier, key string, value any) int {
	t.Helper()
	found := -1
	for index, action := range actions {
		if action["WFWorkflowActionIdentifier"] != identifier {
			continue
		}
		if key != "" && p15N03ExactParams(action)[key] != value {
			continue
		}
		if found >= 0 {
			t.Fatalf("duplicate %s action for %s=%v", identifier, key, value)
		}
		found = index
	}
	if found < 0 {
		t.Fatalf("missing %s action for %s=%v", identifier, key, value)
	}
	return found
}

func p15N03ExplicitSnapshot(t *testing.T, actions []map[string]any, name string) int {
	t.Helper()
	return p15N03ExplicitUniqueAction(t, actions, "is.workflow.actions.gettext", "CustomOutputName", name)
}

func p15N03ExplicitSnapshotSource(t *testing.T, actions []map[string]any, snapshot, producer int) {
	t.Helper()
	payload, okay := p15N03ExactParams(actions[snapshot])["WFTextActionText"].(map[string]any)
	if !okay || payload["WFSerializationType"] != "WFTextTokenString" {
		t.Fatal("RHS snapshot lost its actual native text template")
	}
	body, okay := payload["Value"].(map[string]any)
	if !okay || body["string"] != "~\uFFFC" {
		t.Fatal("RHS snapshot substituted or changed the actual Text operand")
	}
	attachments, okay := body["attachmentsByRange"].(map[string]any)
	if !okay || len(attachments) != 1 {
		t.Fatal("RHS snapshot attachment inventory changed")
	}
	edge, err := p15N03ExactToken(attachments["{1, 1}"])
	uuid, okay := p15N03ExactParams(actions[producer])["UUID"].(string)
	if !okay || uuid == "" || err != nil || edge != uuid || snapshot <= producer {
		t.Fatalf("RHS snapshot did not consume the sole actual RHS producer: producer=%d snapshot=%d error=%v", producer, snapshot, err)
	}
}

func p15N03ExplicitComparisonGraph(t *testing.T, actions []map[string]any, producer int, dynamicLeft bool) {
	t.Helper()
	left := p15N03ExplicitSnapshot(t, actions, "_n03_exact_left")
	right := p15N03ExplicitSnapshot(t, actions, "_n03_exact_right")
	if dynamicLeft {
		if left >= producer {
			t.Fatal("mutable LHS was not captured before the explicit RHS")
		}
		payload, okay := p15N03ExactParams(actions[left])["WFTextActionText"].(map[string]any)
		if !okay || payload["WFSerializationType"] != "WFTextTokenString" {
			t.Fatal("mutable LHS snapshot was replaced with a literal")
		}
		body, okay := payload["Value"].(map[string]any)
		if !okay || body["string"] != "~\uFFFC" {
			t.Fatal("mutable LHS snapshot lost its exact prefix/reference")
		}
		attachments, okay := body["attachmentsByRange"].(map[string]any)
		if !okay || len(attachments) != 1 {
			t.Fatal("mutable LHS snapshot attachment inventory changed")
		}
		reference, okay := attachments["{1, 1}"].(map[string]any)
		if !okay || len(reference) != 2 || reference["Type"] != "Variable" || reference["VariableName"] != "left" {
			t.Fatal("mutable LHS snapshot does not read the original left variable")
		}
	} else if left <= producer || p15N03ExactParams(actions[left])["WFTextActionText"] != "~LEFT" {
		t.Fatal("literal LHS baseline unexpectedly acquired an early owned snapshot")
	}
	p15N03ExplicitSnapshotSource(t, actions, right, producer)
	if err := p15N03ExactPrimitiveError(actions, 1); err != nil {
		t.Fatal(err)
	}
}

func TestP15N03ExactTextExplicitOperandInputIsolation(t *testing.T) {
	const custom = `action plain(value: Text) -> Text {
  identifier: "is.workflow.actions.gettext"
  primary: "value"
  bindings: {value: {key: "WFTextActionText", codec: "text"}}
}
`
	const raw = `f"{native.action("is.workflow.actions.gettext", {"WFTextActionText":"RIGHT"})}"`
	const beforeVoid = "var left: Text = \"LEFT\"\nwait(0)\n"
	for _, fixture := range []struct {
		name, declaration, rhs  string
		comparison, dynamicLeft bool
	}{
		{"custom_rhs_without_comparison", custom, `plain("RIGHT")`, false, false},
		{"raw_rhs_without_comparison", "", raw, false, false},
		{"custom_literal_left_after_void", custom, `plain("RIGHT")`, true, false},
		{"raw_literal_left_after_void", "", raw, true, false},
		{"custom_mutable_left_after_void", custom, `plain("RIGHT")`, true, true},
		{"raw_mutable_left_after_void", "", raw, true, true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			expression := fixture.rhs
			if fixture.comparison {
				left := `"LEFT"`
				if fixture.dynamicLeft {
					left = "left"
				}
				expression = left + " == " + expression
			}
			source := fixture.declaration + beforeVoid + "let answer = " + expression + "\ngetVariable(answer)\n"
			actions := p15N03ExactCompile(t, source)
			wait := p15N03ExplicitUniqueAction(t, actions, "is.workflow.actions.delay", "", nil)
			target := p15N03ExplicitUniqueAction(t, actions, "is.workflow.actions.gettext", "WFTextActionText", "RIGHT")
			if target <= wait {
				t.Fatal("reachable explicit RHS was not emitted after the original void action")
			}
			if _, present := p15N03ExactParams(actions[target])["WFInput"]; present {
				t.Fatal("explicit Text payload was replaced with an invented WFInput")
			}
			for index := wait + 1; index < target; index++ {
				if actions[index]["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvariable" {
					t.Fatal("explicit Text RHS acquired an unrelated Previous-input restoration")
				}
			}
			if fixture.comparison {
				p15N03ExplicitComparisonGraph(t, actions, target, fixture.dynamicLeft)
			} else if err := p15N03ExactPrimitiveError(actions, 0); err != nil {
				t.Fatal(err)
			}
		})
	}

	t.Run("omitted_detector_restores_known_seed", func(t *testing.T) {
		const declaration = "action previousText() -> Text {\nidentifier: \"is.workflow.actions.detect.text\"\n}\n"
		source := declaration + "var left:Text=\"LEFT\"\nlet seed=text(\"N03_PREVIOUS_INPUT\")\nlet answer=left==previousText()\ngetVariable(answer)\n"
		actions := p15N03ExactCompile(t, source)
		seed := p15N03ExplicitUniqueAction(t, actions, "is.workflow.actions.gettext", "WFTextActionText", "N03_PREVIOUS_INPUT")
		target := p15N03ExplicitUniqueAction(t, actions, "is.workflow.actions.detect.text", "", nil)
		left := p15N03ExplicitSnapshot(t, actions, "_n03_exact_left")
		if !(seed < left && left < target) {
			t.Fatal("owned snapshot did not separate the seed and omitted-input detector")
		}
		if _, present := p15N03ExactParams(actions[target])["WFInput"]; present {
			t.Fatal("source omitted input was rewritten as explicit")
		}
		previous := actions[target-1]
		edge, err := p15N03ExactToken(p15N03ExactParams(previous)["WFVariable"])
		uuid, okay := p15N03ExactParams(actions[seed])["UUID"].(string)
		if previous["WFWorkflowActionIdentifier"] != "is.workflow.actions.getvariable" || !okay || uuid == "" || err != nil || edge != uuid {
			t.Fatal("omitted detector failed to restore the actual source seed")
		}
		p15N03ExplicitComparisonGraph(t, actions, target, true)
	})

	t.Run("explicit_wfinput_after_void", func(t *testing.T) {
		const declaration = "action explicitText(value:AnyContent) -> Text {\nidentifier: \"is.workflow.actions.detect.text\"\nprimary:\"value\"\nbindings:{value:{key:\"WFInput\",codec:\"token\"}}\n}\n"
		actions := p15N03ExactCompile(t, declaration+beforeVoid+"let answer=left==explicitText(ShortcutInput)\ngetVariable(answer)\n")
		target := p15N03ExplicitUniqueAction(t, actions, "is.workflow.actions.detect.text", "", nil)
		input, okay := p15N03ExactParams(actions[target])["WFInput"].(map[string]any)
		if !okay || len(input) != 2 || input["WFSerializationType"] != "WFTextTokenAttachment" {
			t.Fatal("explicit input lost its native attachment")
		}
		value, okay := input["Value"].(map[string]any)
		if !okay || len(value) != 1 || value["Type"] != "ExtensionInput" {
			t.Fatal("explicit input is no longer the original ShortcutInput")
		}
		p15N03ExplicitComparisonGraph(t, actions, target, true)
	})

	for _, fixture := range []struct{ name, rhs string }{
		{"omitted_detector_after_void_rejected", `f"{native.action("is.workflow.actions.detect.text", {})}"`},
		{"unknown_payload_does_not_authorize_bypass", `f"{native.action("com.example.synthetic.n03-unknown", {"Payload":"RIGHT"})}"`},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			source := beforeVoid + "let answer=left==" + fixture.rhs + "\ngetVariable(answer)\n"
			response := compileForMobileMode(source, "P15_N03_EXPLICIT_INPUT_REJECTION", "v2", false)
			if response.OK || response.Mode != "v2" || response.PlistBase64 != "" || response.SignedBase64 != "" ||
				!strings.Contains(response.Error, "S01_IMPLICIT_INPUT_UNAVAILABLE") {
				t.Fatalf("unproved omitted input was not rejected by its original boundary: ok=%v mode=%q error=%q", response.OK, response.Mode, response.Error)
			}
		})
	}
}
