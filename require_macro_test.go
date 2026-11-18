package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func requireOutputUUID(raw any) string {
	ref, ok := raw.(map[string]any)
	if !ok || ref["WFSerializationType"] != "WFTextTokenAttachment" {
		return ""
	}
	body, _ := ref["Value"].(map[string]any)
	id, _ := body["OutputUUID"].(string)
	return id
}

// Follow only type-preserving native Get Variable aliases. A witness, formatted
// Text, or different producer cannot masquerade as the original value.
func requirePassthroughOrigin(acts []ShortcutAction, target string) error {
	var alias map[string]any
	producers := make(map[string]ShortcutAction)
	for _, a := range acts {
		if id, ok := a.WFWorkflowActionParameters["UUID"].(string); ok {
			producers[id] = a
		}
	}
	for i := len(acts) - 1; i >= 0; i-- {
		if acts[i].WFWorkflowActionIdentifier == "is.workflow.actions.getvariable" {
			alias = acts[i].WFWorkflowActionParameters
			break
		}
	}
	if alias == nil {
		return fmt.Errorf("missing passthrough")
	}
	id := requireOutputUUID(alias["WFVariable"])
	seen := make(map[string]bool)
	for id != "" && id != target {
		if seen[id] {
			return fmt.Errorf("passthrough cycle")
		}
		seen[id] = true
		a, ok := producers[id]
		if !ok || a.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" {
			return fmt.Errorf("passthrough replaced producer %q with %q", target, id)
		}
		id = requireOutputUUID(a.WFWorkflowActionParameters["WFVariable"])
	}
	if id != target {
		return fmt.Errorf("missing original producer reference")
	}
	return nil
}

func TestP15RequireDualFrontendTypedPassthrough(t *testing.T) {
	for _, tc := range []struct{ name, producer, kind, id string }{
		{"text", `text("payload")`, "text", "gettext"},
		{"number", `number(0)`, "number", "number"},
		{"dictionary", `{"answer": 42}`, "dictionary", "dictionary"},
		{"list", `["first", "second"]`, "", "list"},
		{"file", `base64Decode("AQ==")`, "", "base64encode"},
	} {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/legacy=%t", tc.name, legacy), func(t *testing.T) {
				src := "let original = " + tc.producer + "\nlet checked = require(original, nonEmpty: true"
				if tc.kind != "" {
					src += ", type: \"" + tc.kind + "\""
				}
				src += ")\nquicklook(checked)"
				if legacy {
					src = "const original = " + tc.producer + "\nconst checked = require(original, \"" + tc.kind + "\", true)\nquicklook(checked)"
					if tc.name == "list" {
						src = "@original = " + tc.producer + "\nconst checked = require(@original, \"\", true)\nquicklook(checked)"
					}
					if tc.name == "file" {
						src = "#include 'actions/crypto'\n" + src
					}
				}
				acts := compileDebugFixture(t, src, legacy)
				source := ""
				for _, a := range acts {
					if a.WFWorkflowActionIdentifier == "is.workflow.actions."+tc.id {
						source, _ = a.WFWorkflowActionParameters["UUID"].(string)
						break
					}
				}
				if source == "" && legacy && tc.name == "list" {
					captures := 0
					for _, a := range acts {
						if a.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" {
							continue
						}
						ref, _ := a.WFWorkflowActionParameters["WFVariable"].(map[string]any)
						value, _ := ref["Value"].(map[string]any)
						if value["Type"] == "Variable" && value["VariableName"] == "original" {
							captures++
							source, _ = a.WFWorkflowActionParameters["UUID"].(string)
						}
					}
					if captures != 1 {
						t.Fatalf("mutable Legacy list must be captured exactly once, got %d", captures)
					}
				}
				if source == "" {
					t.Fatal("missing original producer or variable snapshot UUID")
				}
				if err := requirePassthroughOrigin(acts, source); err != nil {
					t.Fatal(err)
				}
				if len(acts) > 40 {
					t.Fatalf("unexpected require expansion size: %d", len(acts))
				}
				inExpansion := false
				for _, a := range acts {
					// Legacy mutable list construction legitimately uses Add to Variable.
					// The require expansion starts only after its once-only snapshot.
					if a.WFWorkflowActionParameters["UUID"] == source {
						inExpansion = true
					}
					if inExpansion && (a.WFWorkflowActionIdentifier == "is.workflow.actions.appendvariable" || a.WFWorkflowActionIdentifier == "is.workflow.actions.detect.dictionary" || a.WFWorkflowActionIdentifier == "is.workflow.actions.detect.number" || a.WFWorkflowActionIdentifier == "is.workflow.actions.text.combine") {
						t.Fatalf("value conversion/accumulator introduced by require: %s", a.WFWorkflowActionIdentifier)
					}
					for key := range a.WFWorkflowActionParameters {
						if strings.HasPrefix(key, "WFRequire") || strings.HasPrefix(key, "__CherriRequire") {
							t.Fatalf("compiler option leaked into native wire: %s", key)
						}
					}
				}
			})
		}
	}
}

func TestP15RequireTypeWitnessAndMultiItemGuard(t *testing.T) {
	acts := compileDebugFixture(t, `let input = run("P15_REQUIRE_SOURCE", input: true)
let checked = require(input, type: "dictionary", nonEmpty: true)
quicklook(checked)`, false)
	var runs, types, comparison, firstType int
	firstType = -1
	guard := -1
	for i, a := range acts {
		p := a.WFWorkflowActionParameters
		switch a.WFWorkflowActionIdentifier {
		case "is.workflow.actions.runworkflow":
			runs++
		case "is.workflow.actions.getitemtype":
			types++
			if firstType < 0 {
				firstType = i
			}
		case "is.workflow.actions.conditional":
			if p15Int(p["WFControlFlowMode"]) != 0 {
				continue
			}
			if p15Int(p["WFCondition"]) == 5 && p15Int(p["WFNumberValue"]) == 1 {
				guard = i
			}
			if p15Int(p["WFCondition"]) == 5 && p["WFConditionalActionString"] != nil {
				text, ok := p["WFConditionalActionString"].(map[string]any)
				if !ok || text["WFSerializationType"] != "WFTextTokenString" {
					t.Fatalf("type guard hardcodes/coerces type label: %#v", p)
				}
				val, _ := text["Value"].(map[string]any)
				if val["string"] != "\uFFFC" {
					t.Fatal("type comparison must use a runtime witness")
				}
				comparison++
			}
		}
	}
	if runs != 1 || types != 2 || comparison != 1 || guard < 0 || guard >= firstType {
		t.Fatalf("invalid once-only/type/scalar guards: runs=%d types=%d compares=%d guard=%d firstType=%d", runs, types, comparison, guard, firstType)
	}
}

func TestP15RequireFalseZeroAndEmptyLiteralShapes(t *testing.T) {
	for _, value := range []string{"false", "true", "0", `""`, "[]", "{}", "none"} {
		t.Run(value, func(t *testing.T) {
			acts := compileDebugFixture(t, "let checked = require("+value+", nonEmpty: true)\nquicklook(checked)", false)
			if len(acts) == 0 {
				t.Fatal("missing guards")
			}
			for _, a := range acts {
				if a.WFWorkflowActionIdentifier == "is.workflow.actions.conditional" {
					code := p15Int(a.WFWorkflowActionParameters["WFCondition"])
					if code == 100 || code == 101 {
						t.Fatal("presence predicate conflates false/zero with emptiness")
					}
				}
			}
		})
	}
}

func TestP15RequireRejectsUnsupportedAndDynamicGuards(t *testing.T) {
	for _, src := range []string{
		`require("value")`,
		`require("value", type: "file")`,
		`require("value", type: "list")`,
		`require(false, type: "boolean")`,
		`require(1, type: "dictionary")`,
		`require(["a"], type: "text")`,
		"var flag = true\nrequire(0, nonEmpty: flag)",
		"var kind = \"text\"\nrequire(\"x\", type: kind)",
	} {
		if _, err := CompileSourceToPlist("require-negative.cherri", src); err == nil {
			t.Fatalf("invalid guard compiled: %s", src)
		}
	}
}

func TestP15RequirePreservesInferredSourceType(t *testing.T) {
	for _, src := range []string{
		`let checked: Number = require(0, nonEmpty: true)`,
		`let checked: Bool = require(false, nonEmpty: true)`,
		`let checked: List<Text> = require(["first", "second"], nonEmpty: true)`,
		`let checked: Text = require("value", type: "text")`,
		`let checked = require({"key": 3}, type: "dictionary")
let value: Number? = checked["key"]`,
	} {
		if _, err := CompileSourceToPlist("require-inferred.cherri", src); err != nil {
			t.Fatalf("typed passthrough failed: %v\n%s", err, src)
		}
	}
}

func TestP15RequireMutableAndSystemValuesCapturedOnce(t *testing.T) {
	acts := compileDebugFixture(t, `let checked = require(Clipboard, nonEmpty: true)
quicklook(checked)`, false)
	var captures int
	for _, a := range acts {
		if a.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" {
			continue
		}
		ref, _ := a.WFWorkflowActionParameters["WFVariable"].(map[string]any)
		body, _ := ref["Value"].(map[string]any)
		if body["Type"] == "Clipboard" {
			captures++
		}
	}
	if captures != 1 {
		t.Fatalf("clipboard must be snapshotted once, got %d", captures)
	}
}

func TestP15RequirePassthroughOracleRejectsSubstitution(t *testing.T) {
	acts := compileDebugFixture(t, `let original = text("preserve")
let checked = require(original, type: "text", nonEmpty: true)`, false)
	target, _ := acts[0].WFWorkflowActionParameters["UUID"].(string)
	if err := requirePassthroughOrigin(acts, target); err != nil {
		t.Fatal(err)
	}
	for i := len(acts) - 1; i >= 0; i-- {
		if acts[i].WFWorkflowActionIdentifier == "is.workflow.actions.getvariable" {
			acts[i].WFWorkflowActionParameters["WFVariable"] = map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "ActionOutput", "OutputUUID": "MUTATED"}}
			break
		}
	}
	if err := requirePassthroughOrigin(acts, target); err == nil {
		t.Fatal("substituted output falsely passed")
	}
}

func TestP15RequireSingletonListDoesNotUseTextEmptiness(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		legacy    bool
	}{
		{"v2-list", `let values = [""]
let checked = require(values, nonEmpty: true)`, false},
		{"v2-text", `let value = ""
let checked = require(value, nonEmpty: true)`, false},
		{"legacy-list", `@values = [""]
const checked = require(@values, "", true)`, true},
		{"legacy-text", `const value = ""
const checked = require(value, "", true)`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			acts := compileDebugFixture(t, tc.src, tc.legacy)
			charCount := 0
			itemCount := 0
			typeCount := 0
			for _, act := range acts {
				if act.WFWorkflowActionIdentifier == "is.workflow.actions.count" {
					switch act.WFWorkflowActionParameters["WFCountType"] {
					case "Items":
						itemCount++
					case "Characters":
						charCount++
					}
				}
				if act.WFWorkflowActionIdentifier == "is.workflow.actions.getitemtype" {
					typeCount++
				}
			}
			if strings.Contains(tc.name, "list") {
				if charCount != 0 || itemCount != 1 || typeCount != 0 {
					t.Fatalf("known singleton List must count items only; characters=%d items=%d runtimeTypes=%d", charCount, itemCount, typeCount)
				}
			} else if charCount != 1 || typeCount != 0 {
				t.Fatalf("known Text must count characters; characters=%d runtimeTypes=%d", charCount, typeCount)
			}
		})
	}
}

func TestP15RequireNamedInputAndEscapedMessage(t *testing.T) {
	message := "quote \" / backslash \\ / newline\nשלום 🌍"
	src := "let values = [\"\"]\nlet checked: List<Text> = require(value: values, nonEmpty: true, message: " + fmt.Sprintf("%q", message) + ")"
	acts := compileDebugFixture(t, src, false)
	alerts := 0
	for _, a := range acts {
		if a.WFWorkflowActionIdentifier == "is.workflow.actions.alert" {
			alerts++
			raw := a.WFWorkflowActionParameters["WFAlertActionMessage"]
			text, ok := raw.(string)
			if !ok {
				wrapper, _ := raw.(map[string]any)
				value, _ := wrapper["Value"].(map[string]any)
				text, _ = value["string"].(string)
			}
			if text != message {
				t.Fatalf("failure message changed: %q != %q", text, message)
			}
		}
	}
	if alerts != 1 {
		t.Fatalf("known List nonempty guard should need one failure branch, got %d", alerts)
	}
}

func TestP15RequireCountUsesCanonicalWireKey(t *testing.T) {
	canonical := compileDebugFixture(t, `let original=text("probe")
count(original,type:"Characters")`, false)
	guarded := compileDebugFixture(t, `let original=text("probe")
let checked=require(original,nonEmpty:true)`, false)
	var expected, actual map[string]any
	for _, act := range canonical {
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.count" {
			expected = act.WFWorkflowActionParameters
		}
	}
	for _, act := range guarded {
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.count" {
			actual = act.WFWorkflowActionParameters
		}
	}
	if expected == nil || actual == nil {
		t.Fatal("missing canonical/macro Count")
	}
	if expected["Input"] == nil || actual["Input"] == nil || actual["WFInput"] != nil {
		t.Fatalf("Count input key differs from real canonical action: expected=%#v actual=%#v", expected, actual)
	}
	if expected["WFCountType"] != actual["WFCountType"] {
		t.Fatal("Count unit differs")
	}
	if requireOutputUUID(actual["Input"]) == "" {
		t.Fatal("Count lost original typed input")
	}

}

func TestP15RequireRuntimeFixtureCompiles(t *testing.T) {
	source, err := os.ReadFile("tests/language-v2/v2-require-runtime.cherri")
	if err != nil {
		t.Fatal(err)
	}
	acts := compileDebugFixture(t, string(source), false)
	var counts int
	for _, act := range acts {
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.count" {
			counts++
			if act.WFWorkflowActionParameters["Input"] == nil {
				t.Fatal("Count lost canonical Input parameter")
			}
		}
	}
	if counts < 7 {
		t.Fatalf("missing fixture guards: %d", counts)
	}
}

func TestP15RequireRemainsSemanticInTraceClassification(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		src := `let checked = require(0, nonEmpty: true)
debugLog()`
		if legacy {
			src = `const checked = require(0, "", true)
debugLog()`
		}
		acts := compileDebugFixture(t, src, legacy)
		mappings := ClassifyShortcutTraceActions(acts)
		source, debug := 0, 0
		for _, mapping := range mappings {
			switch mapping.Kind {
			case "SOURCE_ACTION":
				source++
			case "INLINE_DEBUG_ACTION":
				debug++
			}
		}
		if source != len(acts)-1 || debug != 1 {
			t.Fatalf("semantic guard classified as debug (legacy=%t): source=%d debug=%d actions=%d", legacy, source, debug, len(acts))
		}
	}
}

func TestP15RequireTypeInspectionMatchesCanonicalAndAppleMetadata(t *testing.T) {
	data, err := os.ReadFile("docs/apple-runtime/macos-27/builtin-actions.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata map[string]struct {
		ActionClass string
		Name        string
		Input       struct{ ParameterKey string }
		Output      struct{ Types []string }
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	canonical := compileDebugFixture(t, `let original = text("native")
let inspected = typeOf(original)`, false)
	canonicalID := ""
	for _, act := range canonical {
		meta := metadata[act.WFWorkflowActionIdentifier]
		if meta.ActionClass == "WFGetItemTypeAction" {
			if canonicalID != "" {
				t.Fatal("canonical typeOf emits multiple type inspectors")
			}
			canonicalID = act.WFWorkflowActionIdentifier
			if meta.Name != "Get Type" || meta.Input.ParameterKey != "WFInput" || len(meta.Output.Types) != 1 || meta.Output.Types[0] != "NSString" {
				t.Fatalf("canonical typeOf disagrees with Apple inspection metadata: %+v", meta)
			}
		}
	}
	if canonicalID == "" {
		t.Fatal("canonical typeOf did not emit Apple's native type inspector")
	}
	for _, legacy := range []bool{false, true} {
		src := `let original = text("native")
let checked = require(original, type: "text")`
		if legacy {
			src = `const original = text("native")
const checked = require(original, "text")`
		}
		acts := compileDebugFixture(t, src, legacy)
		producers := make(map[string]ShortcutAction)
		for _, act := range acts {
			if id, ok := act.WFWorkflowActionParameters["UUID"].(string); ok {
				producers[id] = act
			}
		}
		inspections := 0
		for _, act := range acts {
			p := act.WFWorkflowActionParameters
			if act.WFWorkflowActionIdentifier != "is.workflow.actions.conditional" || p["WFConditionalActionString"] == nil {
				continue
			}
			input, _ := p["WFInput"].(map[string]any)
			actualID := requireOutputUUID(input["Variable"])
			token, _ := p["WFConditionalActionString"].(map[string]any)
			value, _ := token["Value"].(map[string]any)
			attachments, _ := value["attachmentsByRange"].(map[string]any)
			witness, _ := attachments["{0, 1}"].(map[string]any)
			witnessID, _ := witness["OutputUUID"].(string)
			for _, id := range []string{actualID, witnessID} {
				producer, ok := producers[id]
				if !ok {
					t.Fatalf("type comparison lost inspection producer %q", id)
				}
				meta := metadata[producer.WFWorkflowActionIdentifier]
				if producer.WFWorkflowActionIdentifier != canonicalID || meta.ActionClass != "WFGetItemTypeAction" {
					t.Fatalf("require compares output from %s (%s), want canonical typeOf %s; legacy=%t", producer.WFWorkflowActionIdentifier, meta.Name, canonicalID, legacy)
				}
				if producer.WFWorkflowActionParameters[meta.Input.ParameterKey] == nil {
					t.Fatal("native inspector input is missing")
				}
				inspections++
			}
		}
		if inspections != 2 {
			t.Fatalf("expected actual and witness native type inspectors, got %d", inspections)
		}
	}
}
