package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"howett.net/plist"
)

func p15EmptyParams(action map[string]any) map[string]any {
	params, _ := action["WFWorkflowActionParameters"].(map[string]any)
	return params
}

// Read the real serialized graph, following the consumed reference to the
// existing native Nothing -> Set Variable (without input) clearing mechanism.
// This is a structural oracle; a new signed run must still prove native counts.
func p15EmptyOrigin(actions []map[string]any, before int, raw any) (string, error) {
	wrapper, ok := raw.(map[string]any)
	if !ok || wrapper["WFSerializationType"] != "WFTextTokenAttachment" {
		return "", fmt.Errorf("empty value is not a native attachment: %#v", raw)
	}
	token, ok := wrapper["Value"].(map[string]any)
	if !ok || token["Aggrandizements"] != nil {
		return "", fmt.Errorf("empty value has no plain native identity")
	}
	switch token["Type"] {
	case "ActionOutput":
		id, _ := token["OutputUUID"].(string)
		producer := -1
		for i, action := range actions {
			if p15EmptyParams(action)["UUID"] == id && id != "" {
				if producer >= 0 {
					return "", fmt.Errorf("ambiguous empty output UUID")
				}
				producer = i
			}
		}
		if producer < 0 || producer >= before {
			return "", fmt.Errorf("dangling or forward empty output")
		}
		params := p15EmptyParams(actions[producer])
		if actions[producer]["WFWorkflowActionIdentifier"] != "is.workflow.actions.getvariable" {
			return "", fmt.Errorf("empty output uses %v, not the existing cleared-variable bridge", actions[producer]["WFWorkflowActionIdentifier"])
		}
		if name, named := token["OutputName"]; named && params["CustomOutputName"] != name {
			return "", fmt.Errorf("empty output name differs from producer")
		}
		return p15EmptyOrigin(actions, producer, params["WFVariable"])
	case "Variable":
		name, _ := token["VariableName"].(string)
		if name == "" {
			return "", fmt.Errorf("missing cleared-variable identity")
		}
		for i := before - 1; i >= 0; i-- {
			if p15EmptyParams(actions[i])["WFVariableName"] != name {
				continue
			}
			if actions[i]["WFWorkflowActionIdentifier"] != "is.workflow.actions.setvariable" {
				return "", fmt.Errorf("cleared value was subsequently appended")
			}
			if _, input := p15EmptyParams(actions[i])["WFInput"]; input {
				return "", fmt.Errorf("empty assignment inherited a producer")
			}
			if i == 0 || actions[i-1]["WFWorkflowActionIdentifier"] != "is.workflow.actions.nothing" {
				return "", fmt.Errorf("empty assignment did not clear pipeline input")
			}
			return name, nil
		}
		return "", fmt.Errorf("missing prior empty assignment")
	default:
		return "", fmt.Errorf("unsupported empty value reference %v", token["Type"])
	}
}

func p15EmptyCountOrigin(actions []map[string]any) (string, error) {
	found := -1
	for i, action := range actions {
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.count" {
			if found >= 0 {
				return "", fmt.Errorf("test requires exactly one Count consumer")
			}
			found = i
		}
	}
	if found < 0 {
		return "", fmt.Errorf("missing Count consumer")
	}
	return p15EmptyOrigin(actions, found, p15EmptyParams(actions[found])["Input"])
}

func TestP15EmptyListRuntimeBindings(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"immutable", "let items=[]\ncount(items)"},
		{"mutable", "var items=[]\ncount(items)"},
		{"assignment", "var items=[\"stale\"]\nitems=[]\ncount(items)"},
		{"snapshot", "var items=[]\nlet snapshot=items\nitems=[\"later\"]\ncount(snapshot)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, "text(\"PRECEDING_CONTENT_MUST_NOT_BECOME_AN_ITEM\")\n"+tc.source)
			if _, err := p15EmptyCountOrigin(actions); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15EmptyListCollectInput(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"direct", "let results=for item in [] { yield item }\ncount(results)"},
		{"bound", "let items=[]\nlet results=for item in items { yield item }\ncount(results)"},
		{"mutable", "var items=[]\nlet results=for item in items { yield item }\ncount(results)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, "text(\"PRECEDING_CONTENT\")\n"+tc.source)
			start, end, count := -1, -1, -1
			for i, a := range actions {
				switch a["WFWorkflowActionIdentifier"] {
				case "is.workflow.actions.repeat.each":
					switch p15Int(p15EmptyParams(a)["WFControlFlowMode"]) {
					case 0:
						start = i
					case 2:
						end = i
					}
				case "is.workflow.actions.count":
					count = i
				}
			}
			if start < 0 || end <= start || count <= end {
				t.Fatal("missing actual Repeat/Count flow")
			}
			if _, err := p15EmptyOrigin(actions, start, p15EmptyParams(actions[start])["WFInput"]); err != nil {
				t.Fatal(err)
			}
			if p15EmptyParams(actions[start])["GroupingIdentifier"] != p15EmptyParams(actions[end])["GroupingIdentifier"] ||
				p15DiagnosticValue(p15EmptyParams(actions[count])["Input"])["OutputUUID"] != p15EmptyParams(actions[end])["UUID"] {
				t.Fatal("collect result does not retain the native Repeat result identity")
			}
		})
	}
}

func TestP15EmptyListDoesNotCollapseEmptyText(t *testing.T) {
	actions := p15CompiledActions(t, "let items=[\"\"]\ncount(items)")
	list, count := -1, -1
	for i, a := range actions {
		if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.list" {
			list = i
		}
		if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.count" {
			count = i
		}
	}
	if list < 0 || count <= list {
		t.Fatal("missing singleton List and Count")
	}
	rows, ok := p15EmptyParams(actions[list])["WFItems"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatal("singleton empty Text lost its item")
	}
	row, ok := rows[0].(map[string]any)
	if !ok || p15Int(row["WFItemType"]) != 0 || row["WFValue"] != "" {
		t.Fatalf("empty Text row changed: %#v", rows)
	}
	if p15DiagnosticValue(p15EmptyParams(actions[count])["Input"])["OutputUUID"] != p15EmptyParams(actions[list])["UUID"] {
		t.Fatal("singleton Count changed producer")
	}
}

func TestP15EmptyListIndependentIdentity(t *testing.T) {
	actions := p15CompiledActions(t, "var __cherri_scope_1_emptyList=[\"authored\"]\nlet first=[]\nlet second=[]\ncount(first)\ncount(second)\ngetVariable(__cherri_scope_1_emptyList)")
	seen := map[string]bool{}
	for i, a := range actions {
		if a["WFWorkflowActionIdentifier"] != "is.workflow.actions.count" {
			continue
		}
		name, err := p15EmptyOrigin(actions, i, p15EmptyParams(a)["Input"])
		if err != nil {
			t.Fatal(err)
		}
		if seen[name] || name == "__cherri_scope_1_emptyList" {
			t.Fatal("independent empty values reuse authored or mutable storage")
		}
		seen[name] = true
	}
	if len(seen) != 2 {
		t.Fatal("missing independent empty consumers")
	}
}

func TestP15EmptyListRawNativeAndImportedArraysUnchanged(t *testing.T) {
	const id = "16A3AA8C-5B08-47D2-AED3-000000000001"
	actions := []any{
		p15AcceptanceAction("is.workflow.actions.list", map[string]any{"WFItems": []any{}, "UUID": id, "CustomOutputName": "Empty native List"}),
		p15AcceptanceAction("is.workflow.actions.getvariable", map[string]any{"WFVariable": map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "ActionOutput", "OutputUUID": id, "OutputName": "Empty native List"}}}),
		p15AcceptanceAction("com.example.p15.opaque.empty", map[string]any{"raw": []any{}, "nested": map[string]any{"items": []any{}}}),
	}
	root := baseTestHeader()
	root["WFWorkflowActions"] = actions

	for _, observation := range p15NativeImportRoundTrip(t, "empty-native-list", root, plist.XMLFormat, plist.BinaryFormat) {
		if observation.plan != nil {
			for i, expected := range []string{"EXACT_NATIVE_FALLBACK", "NATURAL_VERIFIED", "EXACT_NATIVE_FALLBACK"} {
				if observation.plan.Decisions[i].Classification != expected {
					t.Fatalf("baseline import classification changed at %d: %s", i, observation.plan.Decisions[i].Classification)
				}
			}
		}
	}
	assertP12ParDualVerified(t, "empty-native-list-natural", actions, map[int]string{0: "EXACT_NATIVE_FALLBACK", 1: "NATURAL_VERIFIED", 2: "EXACT_NATIVE_FALLBACK"}, nil, nil)
	// Exact native metadata uses the established native.plist escape. A plain
	// source Map is handled by the existing semantic Dictionary codec.
	want := map[string]any{"raw": []any{}, "nested": map[string]any{"items": []any{}}}
	payload, err := plist.Marshal(want, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	raw := p15CompiledActions(t, fmt.Sprintf("native.action(%q, parameters: native.plist(%q))", "com.example.p15.opaque.empty", string(payload)))
	if len(raw) != 1 || !reflect.DeepEqual(p15EmptyParams(raw[0]), want) {
		t.Fatalf("exact raw arrays acquired runtime actions: %#v", raw)
	}

}

func TestP15EmptyListOracleRejectsCorruption(t *testing.T) {
	const source = "let preceding = text(\"PRECEDING\")\nlet items=[]\ncount(items)"
	good := p15CompiledActions(t, source)
	if _, err := p15EmptyCountOrigin(good); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"no_nothing", "inherited_input", "wrong_variable", "missing_producer", "wrong_output_name", "raw_empty_input"} {
		t.Run(fault, func(t *testing.T) {
			encoded, _ := json.Marshal(good)
			var actions []map[string]any
			if err := json.Unmarshal(encoded, &actions); err != nil {
				t.Fatal(err)
			}
			for i, a := range actions {
				switch {
				case fault == "no_nothing" && a["WFWorkflowActionIdentifier"] == "is.workflow.actions.nothing":
					a["WFWorkflowActionIdentifier"] = "is.workflow.actions.gettext"
				case fault == "inherited_input" && a["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable":
					p15EmptyParams(a)["WFInput"] = p15AcceptanceRef(p15EmptyParams(actions[0])["UUID"].(string), nil)
				case fault == "wrong_variable" && a["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvariable":
					p15DiagnosticValue(p15EmptyParams(a)["WFVariable"])["VariableName"] = "MISSING"
				case a["WFWorkflowActionIdentifier"] == "is.workflow.actions.count":
					token := p15DiagnosticValue(p15EmptyParams(a)["Input"])
					switch fault {
					case "missing_producer":
						token["OutputUUID"] = "MISSING"
					case "wrong_output_name":
						token["OutputName"] = "MISSING"
					case "raw_empty_input":
						p15EmptyParams(actions[i])["Input"] = []any{}
					}
				}
			}
			if _, err := p15EmptyCountOrigin(actions); err == nil {
				t.Fatalf("accepted %s", fault)
			}
		})
	}
}

func TestP15EmptyListSignedFixtureSourceRemainsStrict(t *testing.T) {
	source, err := os.ReadFile("tools/shortcuts-simulator-qa/fixtures/qa-collect-empty-split-v2.cherri")
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(source)); got != "15d1e0ab8bae11e87cc91d81ea1a9a3cc0466429956ad7bff9b6b18570854a3a" {
		t.Fatalf("original signed fixture bytes changed: %s", got)
	}
	if !strings.Contains(string(source), "let emptyInput = []") || !strings.Contains(string(source), `let emptyTextList = [""]`) {
		t.Fatal("original native differential source was replaced")
	}
}

func TestP15EmptyListAttachmentOnlyArgumentBoundary(t *testing.T) {
	for _, source := range []string{"count([])", "count(input:[])"} {
		_, err := CompileSourceToPlist("empty-literal-argument.cherri", source)
		if err == nil || !strings.Contains(err.Error(), "E_ARGUMENT_TYPE") || !strings.Contains(err.Error(), "attachment") {
			t.Fatalf("attachment-only parameter boundary changed: %s: %v", source, err)
		}
	}
}

func TestP15EmptyListExistingLegacyClearControl(t *testing.T) {
	legacy := compileDebugFixture(t, "text(\"PRECEDING\")\n@items = []\ncount(@items)", true)
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	var actions []map[string]any
	if err = json.Unmarshal(data, &actions); err != nil {
		t.Fatal(err)
	}
	if _, err = p15EmptyCountOrigin(actions); err != nil {
		t.Fatal(err)
	}
}

func TestP15EmptyListReturnAndYield(t *testing.T) {
	t.Run("return", func(t *testing.T) {
		actions := p15CompiledActions(t, "text(\"PRECEDING\")\nreturn []")
		last := len(actions) - 1
		if actions[last]["WFWorkflowActionIdentifier"] != "is.workflow.actions.output" {
			t.Fatal("missing actual Return output")
		}
		// Output's existing native field is a single token string. Assert its
		// exact attachment shape before following that value's producer; this
		// does not claim native List cardinality after the Output action.
		output, _ := p15EmptyParams(actions[last])["WFOutput"].(map[string]any)
		value, _ := output["Value"].(map[string]any)
		attachments, _ := value["attachmentsByRange"].(map[string]any)
		if output["WFSerializationType"] != "WFTextTokenString" || value["string"] != "\uFFFC" || len(attachments) != 1 {
			t.Fatal("Return's existing single attachment shape changed")
		}
		ref := map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": attachments["{0, 1}"]}
		if _, err := p15EmptyOrigin(actions, last, ref); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("yield", func(t *testing.T) {
		actions := p15CompiledActions(t, "var truth=true\nlet chosen=if truth { yield [] } else { yield [] }\ncount(chosen)")
		yields := 0
		for i, a := range actions {
			if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.list" {
				t.Fatal("empty Yield still emits native List[]")
			}
			if a["WFWorkflowActionIdentifier"] != "is.workflow.actions.getvariable" {
				continue
			}
			params := p15EmptyParams(a)
			ref := p15DiagnosticValue(params["WFVariable"])
			name, _ := ref["VariableName"].(string)
			if !strings.Contains(name, "_emptyList_") {
				continue
			}
			if _, err := p15EmptyOrigin(actions, i, params["WFVariable"]); err != nil {
				t.Fatal(err)
			}
			if i+1 >= len(actions) || actions[i+1]["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" ||
				(p15Int(p15EmptyParams(actions[i+1])["WFControlFlowMode"]) != 1 && p15Int(p15EmptyParams(actions[i+1])["WFControlFlowMode"]) != 2) {
				t.Fatal("empty Yield is not the last actual branch value")
			}
			yields++
		}
		if yields != 2 {
			t.Fatalf("expected two independent empty branch values, got %d", yields)
		}
	})
}
