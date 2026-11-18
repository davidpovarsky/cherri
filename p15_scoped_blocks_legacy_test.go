package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func p15LegacyMeasureActions(t *testing.T, source string, profile BuildProfile) []ShortcutAction {
	t.Helper()
	result, err := compileBuildProfileFixture(t, source, true, profile)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var wire []map[string]any
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	producers := map[string]int{}
	for i, action := range result {
		if id, ok := action.WFWorkflowActionParameters["UUID"].(string); ok && id != "" {
			if _, exists := producers[id]; exists {
				t.Fatalf("duplicate producer UUID %s", id)
			}
			producers[id] = i
		}
	}
	var validate func(any, int)
	validate = func(value any, consumer int) {
		switch value := value.(type) {
		case map[string]any:
			if value["Type"] == "ActionOutput" {
				id, _ := value["OutputUUID"].(string)
				producer, found := producers[id]
				if !found || producer >= consumer {
					t.Fatalf("missing/forward native producer at action %d: %v", consumer, value)
				}
			}
			for _, nested := range value {
				validate(nested, consumer)
			}
		case []any:
			for _, nested := range value {
				validate(nested, consumer)
			}
		}
	}
	for i, action := range wire {
		validate(action, i)
	}
	return result
}

func p15LegacyOrigin(t *testing.T, actions []ShortcutAction, reference any) map[string]any {
	t.Helper()
	ref := p15OutputReference(reference)
	byID := map[string]ShortcutAction{}
	for _, action := range actions {
		if id, ok := action.WFWorkflowActionParameters["UUID"].(string); ok && id != "" {
			byID[id] = action
		}
	}
	seen := map[string]bool{}
	for ref["Type"] == "ActionOutput" {
		id, _ := ref["OutputUUID"].(string)
		if id == "" || seen[id] {
			t.Fatalf("invalid/cyclic scoped reference: %v", ref)
		}
		seen[id] = true
		producer, found := byID[id]
		if !found {
			t.Fatalf("missing scoped producer %s: %v", id, actionSuffixes(actions))
		}
		if producer.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" {
			break
		}
		ref = p15OutputReference(producer.WFWorkflowActionParameters["WFVariable"])
	}
	return ref
}

func TestP15_LegacyMeasureTypedIdentityAndProfiles(t *testing.T) {
	cases := []struct{ name, body, producer string }{
		{"dictionary", "const payload = {\"flag\": false, \"nested\": [1, {\"name\": \"typed\"}]}\nyield payload", "dictionary"},
		{"text", "text(\"scope \\u05e9\\u05dc\\u05d5\\u05dd \\U0001f352\")", "gettext"},
		{"list", "@payload = [\"one\", \"two\"]\nyield @payload", "list"},
		{"file", "const payload = getType(text(\"file bytes\"), \"public.utf8-plain-text\")\nyield payload", "gettypeaction"},
		{"number", "yield 7", "number"},
	}
	// Replace Go-only Unicode escapes in the synthetic source explicitly.
	cases[1].body = "text(\"scope \u05e9\u05dc\u05d5\u05dd \U0001f352\")"
	for _, c := range cases {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run(c.name+"/"+string(profile), func(t *testing.T) {
				actions := p15LegacyMeasureActions(t, "const measured = measure(\"typed\") {\n"+c.body+"\n}\ngetVariable(measured)\n", profile)
				t.Logf("Legacy measure %s %s nativeActions=%d", c.name, profile, len(actions))
				if len(actions) < 2 {
					t.Fatal("missing scoped body")
				}
				consumer := actions[len(actions)-1]
				origin := p15LegacyOrigin(t, actions, consumer.WFWorkflowActionParameters["WFVariable"])
				if c.name == "list" {
					if origin["Type"] != "Variable" || !strings.HasPrefix(fmt.Sprint(origin["VariableName"]), "__cherri_block_") {
						t.Fatalf("list storage lost: %v", origin)
					}
				} else {
					found := false
					for _, producer := range p15ActionsNamed(actions, c.producer) {
						if producer.WFWorkflowActionParameters["UUID"] == origin["OutputUUID"] {
							found = true
						}
					}
					if !found {
						t.Fatalf("typed result lost %s identity: %v", c.producer, origin)
					}
				}
				timers := len(p15ActionsNamed(actions, "gettimebetweendates"))
				if (profile == BuildProfileDebug && timers != 1) || (profile == BuildProfileRelease && timers != 0) {
					t.Fatalf("profile timing=%d", timers)
				}
				if c.name == "dictionary" && len(p15ActionsNamed(actions, "dictionary")) != 1 {
					t.Fatal("dictionary body rerun")
				}
				if c.name == "file" {
					file := p15ActionsNamed(actions, "gettypeaction")[0]
					texts := p15ActionsNamed(actions, "gettext")
					if len(texts) != 1 || texts[0].WFWorkflowActionParameters["WFTextActionText"] != "file bytes" {
						t.Fatalf("nested file input removed/rerun: %v", actionSuffixes(actions))
					}
					input := p15LegacyOrigin(t, actions, file.WFWorkflowActionParameters["WFInput"])
					if input["OutputUUID"] != texts[0].WFWorkflowActionParameters["UUID"] {
						t.Fatalf("file input lost typed producer: %v", input)
					}
				}
				if c.name == "file" && len(p15ActionsNamed(actions, "gettypeaction")) != 1 {
					t.Fatal("file body rerun")
				}
			})
		}
	}
}

func TestP15_LegacyMeasureNestedOnceAndDateSnapshot(t *testing.T) {
	source := "const measured = measure(\"same\") {\nconst inner = measure(\"same\") {\nrun(\"CHERRI_SYNTHETIC_ONCE\")\n}\nyield inner\n}\ngetVariable(measured)\n"
	for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
		actions := p15LegacyMeasureActions(t, source, profile)
		calls := p15ActionsNamed(actions, "runworkflow")
		if len(calls) != 1 {
			t.Fatalf("body effect count=%d", len(calls))
		}
		ref := p15LegacyOrigin(t, actions, actions[len(actions)-1].WFWorkflowActionParameters["WFVariable"])
		if ref["OutputUUID"] != calls[0].WFWorkflowActionParameters["UUID"] {
			t.Fatalf("nested result lost producer: %v", ref)
		}
		keys := map[string]bool{}
		for _, a := range p15ActionsNamed(actions, "setvariable") {
			name, _ := a.WFWorkflowActionParameters["WFVariableName"].(string)
			if strings.HasPrefix(name, "__cherri_debug_timer_") {
				keys[name] = true
			}
		}
		if profile == BuildProfileDebug && len(keys) != 2 {
			t.Fatalf("nested timer collision: %v", keys)
		}
		if profile == BuildProfileRelease && len(keys) != 0 {
			t.Fatal("release timer storage retained")
		}
	}
	actions := p15LegacyMeasureActions(t, "const measured = measure(\"date\") {\nyield CurrentDate\n}\ngetVariable(measured)\n", BuildProfileDebug)
	snapshot, elapsed := -1, -1
	var snapshotID any
	for i, a := range actions {
		if a.WFWorkflowActionIdentifier == "is.workflow.actions.gettimebetweendates" {
			elapsed = i
		}
		if a.WFWorkflowActionIdentifier == "is.workflow.actions.getvariable" && p15OutputReference(a.WFWorkflowActionParameters["WFVariable"])["Type"] == "CurrentDate" {
			snapshot = i
			snapshotID = a.WFWorkflowActionParameters["UUID"]
		}
	}
	if snapshot < 0 || elapsed <= snapshot {
		t.Fatalf("date was not frozen before timing work: %v", actionSuffixes(actions))
	}
	aliases := 0
	for _, a := range p15ActionsNamed(actions, "getvariable") {
		if p15OutputReference(a.WFWorkflowActionParameters["WFVariable"])["OutputUUID"] == snapshotID {
			aliases++
		}
	}
	if aliases < 1 {
		t.Fatal("date snapshot not forwarded")
	}
}

func TestP15_LegacyMeasureLexicalBindings(t *testing.T) {
	source := `const value = text("outer")
const measured = measure("scope") {
    text("BEFORE:{value}")
    const value = text("inner")
    const saved = value
    const nested = measure("scope") {
        const value = text("deep")
        text("DEEP:{value}")
    }
    text("INNER:{value}|SAVED:{saved}|NESTED:{nested}")
}
text("AFTER:{value}|RESULT:{measured}")
`
	actions := p15LegacyMeasureActions(t, source, BuildProfileRelease)
	named := map[string]any{}
	for _, a := range p15ActionsNamed(actions, "gettext") {
		if plain, ok := a.WFWorkflowActionParameters["WFTextActionText"].(string); ok {
			named[plain] = a.WFWorkflowActionParameters["UUID"]
		}
	}
	if len(named) != 3 || named["outer"] == nil || named["inner"] == nil || named["deep"] == nil {
		t.Fatalf("literal producers missing: %v", named)
	}
	for _, a := range p15ActionsNamed(actions, "gettext") {
		value := debugParamMap(a.WFWorkflowActionParameters["WFTextActionText"])
		payload := debugParamMap(value["Value"])
		text, _ := payload["string"].(string)
		if text == "" {
			text, _ = payload["String"].(string)
		}
		attachments := debugParamMap(payload["attachmentsByRange"])
		if len(attachments) == 0 {
			attachments = debugParamMap(payload["AttachmentsByRange"])
		}
		expected := ""
		switch {
		case strings.HasPrefix(text, "BEFORE:") || strings.HasPrefix(text, "AFTER:"):
			expected = "outer"
		case strings.HasPrefix(text, "DEEP:"):
			expected = "deep"
		case strings.HasPrefix(text, "INNER:"):
			expected = "inner"
		}
		if expected == "" {
			continue
		}
		var first any
		for key, ref := range attachments {
			if key == "{7, 1}" || key == "{6, 1}" || key == "{5, 1}" {
				first = ref
				break
			}
		}
		if first == nil {
			t.Fatalf("missing first attachment %q: %v", text, attachments)
		}
		if origin := p15LegacyOrigin(t, actions, first); origin["OutputUUID"] != named[expected] {
			t.Fatalf("%s binding changed: %v", expected, origin)
		}
	}
	if _, err := compileBuildProfileFixture(t, source+"getVariable(saved)\n", true, BuildProfileRelease); err == nil {
		t.Fatal("block local escaped")
	}
}

func TestP15_LegacyMeasureRejectsUnprovenForms(t *testing.T) {
	cases := []struct{ source, want string }{
		{"measure(\"empty\") {\n}\n", "E_BLOCK_VALUE_REQUIRED"},
		{"measure(\"void\") {\nwait(1)\n}\n", "E_BLOCK_VALUE_REQUIRED"},
		{"measure(\"nil\") {\nyield nil\n}\n", "E_BLOCK_VALUE_REQUIRED"},
		{"measure(\"early\") {\nyield \"value\"\ntext(\"later\")\n}\n", "E_BLOCK_EARLY_YIELD"},
		{"measure(\"raw\") {\nrawAction(\"is.workflow.actions.gettext\", {\"WFTextActionText\":\"raw\"})\n}\n", "E_BLOCK_RAW_NATIVE"},
		{"measure(\"control\") {\nconst result = if 1 == 1 {\ntext(\"x\")\n}\nyield result\n}\n", "E_BLOCK_LEGACY_CONTROL_VALUE"},
		{"measure(\"nested yield\") {\nif 1 == 1 {\nyield \"x\"\n}\ntext(\"done\")\n}\n", "E_BLOCK_EARLY_YIELD"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			_, err := compileLegacySourceToRawMapInMemoryWithProfile(c.source, BuildProfileDebug)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want %s, got %v", c.want, err)
			}
		})
	}
}

func TestP15_LegacyMeasureControlScopesAndCapturedMutation(t *testing.T) {
	source := `@counter = 0
const condition = 1
const value = text("outer")
const measured = measure("control") {
    const value = text("scope")
    if condition == 1 {
        const value = text("branch")
        @counter += 1
        text("IF:{value}")
    } else {
        const value = text("other")
        text("ELSE:{value}")
    }
    repeat index for 2 {
        const value = text("repeat")
        text("REPEAT:{index}:{value}")
    }
    @items = ["a", "b"]
    for item in @items {
        const value = text("each")
        text("EACH:{item}:{value}")
    }
    menu "choose" {
        item "one":
            const value = text("first")
            text("ONE:{value}")
        item "two":
            const value = text("second")
            text("TWO:{value}")
    }
    text("SCOPE:{value}")
    yield @counter
}
text("OUTER:{value}")
getVariable(measured)
`
	for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
		actions := p15LegacyMeasureActions(t, source, profile)
		groups := map[string][]string{}
		counterWrites, mutations := 0, 0
		for _, a := range actions {
			p := a.WFWorkflowActionParameters
			if group, ok := p["GroupingIdentifier"].(string); ok {
				groups[group] = append(groups[group], fmt.Sprint(p["WFControlFlowMode"]))
			}
			if a.WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" && p["WFVariableName"] == "counter" {
				counterWrites++
			}
			if a.WFWorkflowActionIdentifier == "is.workflow.actions.math" && p["WFMathOperation"] == "+" {
				mutations++
			}
		}
		if len(groups) != 4 {
			t.Fatalf("control group count=%d: %v", len(groups), groups)
		}
		for _, modes := range groups {
			if modes[0] != "0" || modes[len(modes)-1] != "2" {
				t.Fatalf("unbalanced control modes: %v", modes)
			}
		}
		if counterWrites != 2 || mutations != 1 {
			t.Fatalf("captured mutation removed/rerun: writes=%d math=%d", counterWrites, mutations)
		}
		producer := map[string]any{}
		texts := map[string]map[string]any{}
		for _, a := range p15ActionsNamed(actions, "gettext") {
			p := a.WFWorkflowActionParameters
			if plain, ok := p["WFTextActionText"].(string); ok {
				producer[plain] = p["UUID"]
				continue
			}
			payload := debugParamMap(debugParamMap(p["WFTextActionText"])["Value"])
			text, _ := payload["string"].(string)
			texts[text] = debugParamMap(payload["attachmentsByRange"])
		}
		for prefix, expected := range map[string]string{"IF:": "branch", "ELSE:": "other", "ONE:": "first", "TWO:": "second", "SCOPE:": "scope", "OUTER:": "outer"} {
			found := false
			for value, refs := range texts {
				if strings.HasPrefix(value, prefix) {
					found = true
					ref := refs[fmt.Sprintf("{%d, 1}", len(prefix))]
					if got := p15LegacyOrigin(t, actions, ref); got["OutputUUID"] != producer[expected] {
						t.Fatalf("%s shadow mismatch: %v", prefix, got)
					}
				}
			}
			if !found {
				t.Fatalf("missing text %s", prefix)
			}
		}
		ref := p15LegacyOrigin(t, actions, actions[len(actions)-1].WFWorkflowActionParameters["WFVariable"])
		if ref["Type"] != "Variable" || ref["VariableName"] != "counter" {
			t.Fatalf("captured result retargeted: %v", ref)
		}
		for _, a := range p15ActionsNamed(actions, "setvariable") {
			n, _ := a.WFWorkflowActionParameters["WFVariableName"].(string)
			if n == "index" || n == "item" || n == "items" {
				t.Fatalf("loop local native name not hygienic: %s", n)
			}
		}
	}
	for _, name := range []string{"index", "item", "items"} {
		if _, err := compileBuildProfileFixture(t, source+"getVariable(@"+name+")\n", true, BuildProfileRelease); err == nil {
			t.Fatalf("%s local escaped", name)
		}
	}
}

func TestP15_LegacyMeasureActionArgumentsRunOnce(t *testing.T) {
	cases := []string{
		`const result = measure("arg") {
            getVariable(run("CHERRI_SCOPED_NESTED_EFFECT"))
        }
        getVariable(result)`,
		`getVariable(measure("arg") {
            run("CHERRI_SCOPED_NESTED_EFFECT")
        })`,
		`const result = measure("outer") {
            getVariable(measure("inner") {
                run("CHERRI_SCOPED_NESTED_EFFECT")
            })
        }
        getVariable(result)`,
	}
	for i, source := range cases {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run(fmt.Sprintf("%d/%s", i, profile), func(t *testing.T) {
				actions := p15LegacyMeasureActions(t, source, profile)
				calls := p15ActionsNamed(actions, "runworkflow")
				if len(calls) != 1 {
					t.Fatalf("argument side effects=%d", len(calls))
				}
				ref := p15LegacyOrigin(t, actions, actions[len(actions)-1].WFWorkflowActionParameters["WFVariable"])
				if ref["OutputUUID"] != calls[0].WFWorkflowActionParameters["UUID"] {
					t.Fatalf("argument native identity changed: %v", ref)
				}
			})
		}
	}
}

func TestP15_LegacyMeasureCommentsAndPropertyReferences(t *testing.T) {
	source := `const measured = measure("property") {
    const payload = {"flag": false, "text": "kept"}
    yield payload['flag']
}
getVariable(measured)`
	actions := p15LegacyMeasureActions(t, source, BuildProfileRelease)
	result := p15LegacyOrigin(t, actions, actions[len(actions)-1].WFWorkflowActionParameters["WFVariable"])
	aggrandizements, ok := result["Aggrandizements"].([]interface{})
	if !ok || len(aggrandizements) != 1 || debugParamMap(aggrandizements[0])["DictionaryKey"] != "flag" {
		t.Fatalf("typed property transformation lost: %v", result)
	}
	for _, source := range []string{
		"measure(\"comment\") {\ntext(\"value\")\n// trailing source comment\n}\n",
		"measure(\"comment\") {\nyield \"value\"\n// trailing source comment\n}\n",
	} {
		actions := p15LegacyMeasureActions(t, source, BuildProfileRelease)
		texts := p15ActionsNamed(actions, "gettext")
		if len(texts) != 1 || texts[0].WFWorkflowActionParameters["WFTextActionText"] != "value" {
			t.Fatal("comment changed final value")
		}
		if p15LegacyOrigin(t, actions, actions[len(actions)-1].WFWorkflowActionParameters["WFVariable"])["OutputUUID"] != texts[0].WFWorkflowActionParameters["UUID"] {
			t.Fatal("comment changed forwarded output")
		}
	}
}

func TestP15_LegacyMeasureStateAndDiagnostics(t *testing.T) {
	negatives := []struct{ source, want string }{
		{"measure(\"unterminated\") {\ntext(\"value\")\n", "E_BLOCK_UNTERMINATED"},
		{"measure(\"redeclaration\") {\nconst value = 1\nconst value = 2\nyield value\n}\n", "E_BLOCK_REDECLARATION"},
		{"const label = \"dynamic\"\nmeasure(label) { text(\"value\") }\n", "literal"},
		{"const label = \"dynamic\"\nmeasure(\"{label}\") { text(\"value\") }\n", "E_BLOCK_LABEL"},
		{"measure(\"\") { text(\"value\") }\n", "E_BLOCK_LABEL"},
		{"measure(\"metadata\") {\n#define name \"escaped\"\ntext(\"value\")\n}\n", "E_BLOCK_DIRECTIVE"},
		{"measure(\"void-arg\") {\ngetVariable(wait(1))\n}\n", "E_BLOCK_VALUE_REQUIRED"},
	}
	for _, c := range negatives {
		_, err := compileLegacySourceToRawMapInMemoryWithProfile(c.source, BuildProfileDebug)
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(c.want)) {
			t.Errorf("want %q for %s, got %v", c.want, c.source, err)
		}
		// A rejected block cannot leak parser bindings or macro state into the
		// next real compilation in the same long-lived process.
		next := p15LegacyMeasureActions(t, "const value = text(\"ordinary\")\ngetVariable(value)\n", BuildProfileRelease)
		if len(next) != 2 {
			t.Fatalf("failed scoped parse leaked state: %v", actionSuffixes(next))
		}
	}
}

func TestP15_LegacyMeasureImportedHintsDoNotRetargetLocalShadow(t *testing.T) {
	resetCompilerState()
	defer resetCompilerState()
	const local = "__cherri_block_test_local"
	variables = map[string]varValue{"value": {variableType: "Variable", valueType: Dict, constant: false}}
	uuids = map[string]string{"value": "outer-producer", local: "local-producer"}
	importedSymbolIsVariable = map[string]bool{"value": true}
	importedSymbolValueTypes = map[string]tokenType{"value": Dict}
	importedSymbolDictAggr = map[string]bool{"value": true}
	importedSymbolAppendVars = map[string]bool{"value": true}
	localValue := varValue{variableType: "Variable", valueType: String, value: "local text", constant: true}
	step := legacyScopedStep{vars: map[string]varValue{"value": localValue, local: localValue}, bindings: map[string]string{"value": local}}
	withLegacyScopedEnvironment(step, func() {
		reference := variableValue(varValue{value: "value", descriptor: &ReferenceDescriptor{}})
		actual := p15OutputReference(reference)
		if actual["Type"] != "ActionOutput" || actual["OutputUUID"] != "local-producer" {
			t.Fatalf("imported outer hint retargeted local descriptor: %v", actual)
		}
		if importedSymbolDictAggr["value"] || isArrayVariable("value") {
			t.Fatal("imported Dictionary/array hint escaped into local type")
		}
	})
	restored := p15OutputReference(variableValue(varValue{value: "value", descriptor: &ReferenceDescriptor{}}))
	if restored["Type"] != "Variable" || restored["VariableName"] != "value" {
		t.Fatalf("captured imported variable identity not restored: %v", restored)
	}
	if !importedSymbolDictAggr["value"] || !importedSymbolAppendVars["value"] || importedSymbolValueTypes["value"] != Dict || uuids["value"] != "outer-producer" {
		t.Fatal("captured import metadata not restored")
	}
}
