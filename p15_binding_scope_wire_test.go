package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func p15V02NativeReferences(value any) (outputs, variables []string) {
	var walk func(any)
	walk = func(value any) {
		switch item := value.(type) {
		case map[string]any:
			if item["Type"] == "ActionOutput" {
				if id, ok := item["OutputUUID"].(string); ok {
					outputs = append(outputs, id)
				}
			}
			if item["Type"] == "Variable" {
				if name, ok := item["VariableName"].(string); ok {
					variables = append(variables, name)
				}
			}
			for _, child := range item {
				walk(child)
			}
		case []any:
			for _, child := range item {
				walk(child)
			}
		}
	}
	walk(value)
	return
}

func p15V02OuterReference(actions []map[string]any, outer string, mutable bool) error {
	outputs, variables := p15V02NativeReferences(actions[len(actions)-1])
	refs := outputs
	if mutable {
		refs = variables
	}
	if len(refs) != 1 || refs[0] != outer {
		return fmt.Errorf("after scope references=%v, want original identity %q", refs, outer)
	}
	return nil
}

func p15V02CorruptLastReference(t *testing.T, actions []map[string]any, mutable bool) []map[string]any {
	t.Helper()
	data, err := json.Marshal(actions)
	if err != nil {
		t.Fatal(err)
	}
	var mutated []map[string]any
	if err := json.Unmarshal(data, &mutated); err != nil {
		t.Fatal(err)
	}
	kind, key := "ActionOutput", "OutputUUID"
	if mutable {
		kind, key = "Variable", "VariableName"
	}
	changes := 0
	var walk func(any)
	walk = func(value any) {
		switch item := value.(type) {
		case map[string]any:
			if item["Type"] == kind {
				if _, found := item[key]; found {
					item[key] = "WRONG_SCOPE_IDENTITY"
					changes++
				}
			}
			for _, child := range item {
				walk(child)
			}
		case []any:
			for _, child := range item {
				walk(child)
			}
		}
	}
	walk(mutated[len(mutated)-1])
	if changes != 1 {
		t.Fatalf("causal control changed %d actual native references, want one", changes)
	}
	return mutated
}

func TestP15V02LexicalWire(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		mutable      bool
	}{
		{"let_if", "let value=text(\"outer\")\nif true {let value=text(\"inner\");text(value)}\ntext(value)", false},
		{"let_else", "let value=text(\"outer\")\nif true {let value=text(\"inner\")} else {text(value)}\ntext(value)", false},
		{"let_sibling", "let value=text(\"outer\")\nif true {let value=text(\"one\");text(value)} else {let value=text(\"two\");text(value)}\ntext(value)", false},
		{"let_native_predicates", "let value=text(\"outer\")\nif value==\"outer\" && value!=\"missing\" {let value=text(\"inner\");text(value)}\ntext(value)", false},
		{"let_repeat", "let value=text(\"outer\")\nrepeat 2 {let value=text(\"inner\");text(value)}\ntext(value)", false},
		{"let_for", "let value=text(\"outer\")\nfor item in [\"item\"] {let value=text(\"inner\");text(value)}\ntext(value)", false},
		{"let_menu", "let value=text(\"outer\")\nmenu \"Pick\" {case \"A\": {let value=text(\"inner\");text(value)} case \"B\": {text(value)}}\ntext(value)", false},
		{"let_initializer", "let value=text(\"outer\")\nif true {let value=text(value);text(value)}\ntext(value)", false},
		{"var_if", "var value=text(\"outer\")\nif true {var value=text(\"inner\");value=text(\"changed\");text(value)}\ntext(value)", true},
		{"var_repeat", "var value=text(\"outer\")\nrepeat 2 {var value=text(\"inner\");text(value)}\ntext(value)", true},
		{"var_for", "var value=text(\"outer\")\nfor item in [\"item\"] {var value=text(\"inner\");text(value)}\ntext(value)", true},
		{"var_menu", "var value=text(\"outer\")\nmenu \"Pick\" {case \"A\": {var value=text(\"inner\");text(value)} case \"B\": {text(value)}}\ntext(value)", true},
		{"for_item", "var value=text(\"outer\")\nfor value in [\"inner\"] {text(value)}\ntext(value)", true},
		{"repeat_index", "var value=number(99)\nrepeat 2 as value {number(value)}\nnumber(value)", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, tc.source)
			outer := ""
			names := []string{}
			for _, action := range actions {
				params := p15Param(action)
				if text, ok := params["WFTextActionText"].(string); ok && text == "outer" {
					outer, _ = params["UUID"].(string)
				}
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" {
					name, _ := params["WFVariableName"].(string)
					names = append(names, name)
				}
			}
			if tc.mutable {
				if len(names) < 2 || names[0] == names[1] {
					t.Fatalf("shadowing native names collide: %v", names)
				}
				outer = names[0]
				if tc.name == "var_if" && (len(names) != 3 || names[1] != names[2]) {
					t.Fatalf("inner assignment lost local identity: %v", names)
				}
			}
			if outer == "" {
				t.Fatal("outer producer absent")
			}
			if err := p15V02OuterReference(actions, outer, tc.mutable); err != nil {
				t.Fatal(err)
			}
			// Corrupt the observed serialized reference, keeping the independent
			// original-identity expectation unchanged.
			mutated := p15V02CorruptLastReference(t, actions, tc.mutable)
			if p15V02OuterReference(mutated, outer, tc.mutable) == nil {
				t.Fatal("wrong native reference mutation passed")
			}
			if !strings.Contains(tc.name, "for_item") && tc.name != "repeat_index" {
				// Some inner paths must still consume their distinct local value.
				localRead := false
				for _, action := range actions[:len(actions)-1] {
					outputs, variables := p15V02NativeReferences(action)
					if tc.mutable {
						for _, name := range variables {
							if name != outer && strings.HasPrefix(name, "__cherri_scope_") {
								localRead = true
							}
						}
					} else {
						for _, uuid := range outputs {
							if uuid != outer {
								localRead = true
							}
						}
					}
				}
				if !localRead && tc.name != "let_else" {
					t.Fatal("inner local value was never consumed")
				}
			}
		})
	}
	// An assignment without a declaration remains a write to the outer value.
	actions := p15CompiledActions(t, "var value=text(\"outer\")\nif true {value=text(\"changed\")}\ntext(value)")
	for _, action := range actions {
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" && p15Param(action)["WFVariableName"] != "value" {
			t.Fatal("outer assignment was renamed", p15Param(action))
		}
	}
	if err := p15V02OuterReference(actions, "value", true); err != nil {
		t.Fatal(err)
	}
}

func TestP15V02ImportedScopeIdentity(t *testing.T) {
	const uuid = "f1500002-0000-4000-8000-000000000001"
	for _, kind := range []string{"output", "variable", "opaque"} {
		t.Run(kind, func(t *testing.T) {
			producer := map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
				"WFWorkflowActionParameters": map[string]any{"UUID": uuid, "CustomOutputName": "Pinned", "WFTextActionText": "original"},
			}
			if kind == "opaque" {
				producer["WFWorkflowActionIdentifier"] = "com.example.p15.opaque"
				producer["WFWorkflowActionParameters"] = map[string]any{"UUID": uuid, "CustomOutputName": "Pinned", "OpaqueField": "synthetic"}
			}
			original := []any{producer}
			if kind == "variable" {
				original = append(original, map[string]any{
					"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
					"WFWorkflowActionParameters": map[string]any{"WFVariableName": "PinnedVariable", "WFInput": p12RawActionOutAttachment(uuid, "Pinned")},
				})
			}
			if kind != "variable" {
				original = append(original, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.getvariable", "WFWorkflowActionParameters": map[string]any{"WFVariable": p12RawActionOutAttachment(uuid, "Pinned")}})
			}
			doc := p12BuildNativeDoc(t, "P15_V02_SCOPE_"+kind, original)
			plan, legacy, v2, pre, err := verifyCorpusDualHybridWithPreOverlay(doc, "P15_V02_SCOPE_"+kind, true)
			if err != nil {
				t.Fatal(err)
			}
			for name, result := range map[string]map[string]any{"legacy": legacy, "v2": v2} {
				if !plistValuesStrictlyEqual(doc.RawRoot, result) {
					t.Fatalf("%s original full tree changed", name)
				}
				for index := range doc.Actions {
					if err := verifyPreOverlayActionSemanticParity("P15_V02_SCOPE_"+kind, name, index, doc.Actions[index], pre[name][index].(map[string]any)); err != nil {
						t.Fatal(err)
					}
				}
			}
			source, err := RenderHybridV2Source(doc, plan)
			if err != nil {
				t.Fatal(err)
			}
			symbol := ""
			for name, metadata := range plan.Symbols {
				if kind == "variable" {
					if strVal(metadata["Kind"]) == "Variable" && strVal(metadata["VariableName"]) == "PinnedVariable" {
						symbol = name
					}
				} else if strVal(metadata["UUID"]) == uuid {
					symbol = name
				}
			}
			if symbol == "" {
				t.Fatal("original native identity missing from real import plan")
			}
			declaration := "let"
			outer := uuid
			if kind == "variable" {
				declaration = "var"
				outer = "PinnedVariable"
			}
			edited := source + "\nif true {" + declaration + " " + symbol + "=text(\"authored-inner\");text(" + symbol + ")}\ntext(" + symbol + ")\n"
			actions := p15CompiledActions(t, edited)
			if err := p15V02OuterReference(actions, outer, kind == "variable"); err != nil {
				t.Fatal(err)
			}
			originalIDs := 0
			localRead := false
			for _, action := range actions {
				params := p15Param(action)
				if params["UUID"] == uuid {
					originalIDs++
				}
				_, variables := p15V02NativeReferences(action)
				for _, name := range variables {
					if strings.HasPrefix(name, "__cherri_scope_") {
						localRead = true
					}
				}
				if text, ok := params["WFTextActionText"].(string); ok && text == "authored-inner" && params["UUID"] == uuid {
					t.Fatal("authored shadow reused pinned producer UUID")
				}
			}
			if originalIDs != 1 {
				t.Fatalf("pinned UUID repeated %d times", originalIDs)
			}
			if kind == "variable" && !localRead {
				t.Fatal("authored mutable shadow did not use its distinct native identity")
			}

			// A newly authored Bool shadow must also stop inheriting the
			// original symbol's imported presence-test exemption.
			boolEdited := source + "\nif true {var " + symbol + "=false; if " + symbol + " {text(\"BOOL_BRANCH\")}}\ntext(" + symbol + ")\n"
			boolActions := p15CompiledActions(t, boolEdited)
			p15AssertBooleanConditions(t, boolActions, "authored same-name imported Bool shadow")
			if err := p15V02OuterReference(boolActions, outer, kind == "variable"); err != nil {
				t.Fatal(err)
			}
		})
	}
	// Original native variables can first be defined in a control region. Their
	// explicit provenance remains global; this is not an authored local shadow.
	actions := p15CompiledActions(t, "var pinned=text(\"discard\")\nif true {pinned=text(\"original\")}\ntext(pinned)")
	native := make([]any, 0, len(actions)-2)
	for _, action := range actions[2:] {
		native = append(native, action)
	}
	doc := p12BuildNativeDoc(t, "P15_V02_ORIGINAL_CONTROL", native)
	plan, legacy, v2, pre, err := verifyCorpusDualHybridWithPreOverlay(doc, "P15_V02_ORIGINAL_CONTROL", true)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Lifecycle == nil || plan.Lifecycle.DemotedActionCount != 0 {
		t.Fatal("original control identity demoted")
	}
	for name, result := range map[string]map[string]any{"legacy": legacy, "v2": v2} {
		if !plistValuesStrictlyEqual(doc.RawRoot, result) {
			t.Fatalf("%s original control full tree changed", name)
		}
		for index := range doc.Actions {
			if err := verifyPreOverlayActionSemanticParity("P15_V02_ORIGINAL_CONTROL", name, index, doc.Actions[index], pre[name][index].(map[string]any)); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestP15V02RepeatedImportedLoopIdentity(t *testing.T) {
	for _, source := range []string{
		"for item in [\"a\"] {text(item)}\nfor item in [\"b\"] {text(item)}",
		"repeat 2 as index {number(index)}\nrepeat 3 as index {number(index)}",
	} {
		actions := p15CompiledActions(t, source)
		native := make([]any, len(actions))
		for i, action := range actions {
			native[i] = action
		}
		doc := p12BuildNativeDoc(t, "P15_V02_REPEAT_IDENTITY", native)
		plan, legacy, v2, pre, err := verifyCorpusDualHybridWithPreOverlay(doc, "P15_V02_REPEAT_IDENTITY", true)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Lifecycle == nil || plan.Lifecycle.DemotedActionCount != 0 {
			t.Fatalf("original repeated control identity demoted: %+v", plan.Lifecycle)
		}
		for name, result := range map[string]map[string]any{"legacy": legacy, "v2": v2} {
			if !plistValuesStrictlyEqual(doc.RawRoot, result) {
				t.Fatalf("%s original repeated control changed", name)
			}
			for index := range doc.Actions {
				if err := verifyPreOverlayActionSemanticParity("P15_V02_REPEAT_IDENTITY", name, index, doc.Actions[index], pre[name][index].(map[string]any)); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}
