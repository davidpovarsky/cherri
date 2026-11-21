package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func p15LegacyInlineReferenceOracle(actions []ShortcutAction) error {
	producers := map[string]int{}
	for index, action := range actions {
		if uuid, ok := action.WFWorkflowActionParameters["UUID"].(string); ok && uuid != "" {
			if _, exists := producers[uuid]; exists {
				return fmt.Errorf("duplicate native producer UUID")
			}
			producers[uuid] = index
		}
	}
	var walk func(any, int) error
	walk = func(value any, consumer int) error {
		switch value := value.(type) {
		case map[string]any:
			if value["Type"] == "ActionOutput" {
				uuid, ok := value["OutputUUID"].(string)
				producer, found := producers[uuid]
				if !ok || uuid == "" || !found || producer >= consumer {
					return fmt.Errorf("missing or forward native producer at %d: %v", consumer, value)
				}
			}
			for _, child := range value {
				if err := walk(child, consumer); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range value {
				if err := walk(child, consumer); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for index, action := range actions {
		if err := walk(action.WFWorkflowActionParameters, index); err != nil {
			return err
		}
	}
	return nil
}

func p15LegacyInlineChainOracle(actions []ShortcutAction, ids []string, marker string) error {
	if len(actions) != len(ids) {
		return fmt.Errorf("expected %d written actions, got %d", len(ids), len(actions))
	}
	for index, id := range ids {
		if actions[index].WFWorkflowActionIdentifier != "is.workflow.actions."+id {
			return fmt.Errorf("native action %d: %s", index, actions[index].WFWorkflowActionIdentifier)
		}
		if index == 0 {
			if actions[index].WFWorkflowActionParameters["WFTextActionText"] != marker {
				return fmt.Errorf("original literal producer changed")
			}
		} else {
			key := "WFInput"
			if id == "gettext" {
				key = "WFTextActionText"
			}
			if id == "gettext" {
				uuid, err := p15C03TokenUUID(actions[index].WFWorkflowActionParameters[key])
				if err != nil || uuid != actions[index-1].WFWorkflowActionParameters["UUID"] {
					return fmt.Errorf("text consumer %d lost the exact preceding producer: %v", index, err)
				}
			} else {
				ref := p15OutputReference(actions[index].WFWorkflowActionParameters[key])
				if ref["Type"] != "ActionOutput" || ref["OutputUUID"] != actions[index-1].WFWorkflowActionParameters["UUID"] {
					return fmt.Errorf("consumer %d lost the exact preceding typed producer: %v", index, ref)
				}
			}
		}
	}
	return p15LegacyInlineReferenceOracle(actions)
}

func TestP15LegacyInlineWrittenProducers(t *testing.T) {
	for _, tc := range []struct {
		name, source, control string
		ids                   []string
		marker                string
	}{
		{"three_deep", "text(text(text(\"DEEPEST\")))\n", "const a = text(\"DEEPEST\")\nconst b = text(a)\ntext(b)\n", []string{"gettext", "gettext", "gettext"}, "DEEPEST"},
		{"typed_file", "typeOf(getType(text(\"BYTES\"), \"public.plain-text\"))\n", "const a = text(\"BYTES\")\nconst b = getType(a, \"public.plain-text\")\ntypeOf(b)\n", []string{"gettext", "gettypeaction", "getitemtype"}, "BYTES"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, mode := range []string{"explicit_control", "inline"} {
				t.Run(mode, func(t *testing.T) {
					source := tc.source
					if mode == "explicit_control" {
						source = tc.control
					}
					actions, err := p15C03Actions(source, true)
					if err != nil {
						t.Fatal(err)
					}
					if err = p15LegacyInlineChainOracle(actions, tc.ids, tc.marker); err != nil {
						t.Fatal(err)
					}
					if tc.name == "typed_file" && actions[1].WFWorkflowActionParameters["WFFileType"] != "public.plain-text" {
						t.Fatal("file conversion option changed")
					}
				})
			}
		})
	}
	for _, tc := range []struct{ name, source string }{
		{"explicit_control", "const a = text(\"FIRST\")\nconst b = text(\"SECOND\")\nconst c = text(\"THIRD\")\nurl(a,b,c)\n"},
		{"inline", "url(text(\"FIRST\"),text(\"SECOND\"),text(\"THIRD\"))\n"},
	} {
		t.Run("written_order/"+tc.name, func(t *testing.T) {
			actions, err := p15C03Actions(tc.source, true)
			if err != nil {
				t.Fatal(err)
			}
			if err = p15C03EffectOracle(actions, true); err != nil {
				t.Fatal(err)
			}
			if err = p15LegacyInlineReferenceOracle(actions); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15LegacyInlineControlPlacement(t *testing.T) {
	for _, tc := range []struct{ name, source, marker, group string }{
		{"if", "const flag = number(0)\nif flag == 1 {\nurl(text(\"BRANCH\"))\n}\ntext(\"TAIL\")\n", "BRANCH", "is.workflow.actions.conditional"},
		{"repeat", "repeat i for 2 {\nurl(text(\"LOOP\"))\n}\ntext(\"TAIL\")\n", "LOOP", "is.workflow.actions.repeat.count"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions, err := p15C03Actions(tc.source, true)
			if err != nil {
				t.Fatal(err)
			}
			if err = p15LegacyInlineReferenceOracle(actions); err != nil {
				t.Fatal(err)
			}
			inside := false
			producer := ""
			found := 0
			consumer := 0
			for _, action := range actions {
				params := action.WFWorkflowActionParameters
				if action.WFWorkflowActionIdentifier == tc.group {
					switch p15Int(params["WFControlFlowMode"]) {
					case 0:
						inside = true
					case 2:
						inside = false
					}
				}
				if action.WFWorkflowActionIdentifier == "is.workflow.actions.gettext" && params["WFTextActionText"] == tc.marker {
					if !inside {
						t.Fatal("inline producer hoisted outside its authored control region")
					}
					producer, _ = params["UUID"].(string)
					found++
				}
				if action.WFWorkflowActionIdentifier == "is.workflow.actions.url" {
					if !inside {
						t.Fatal("consumer left its authored control region")
					}
					items := p15C03Array(params["WFURLActionURL"])
					if len(items) != 1 {
						t.Fatal("consumer item count changed")
					}
					uuid, e := p15C03TokenUUID(items[0])
					if e != nil || producer == "" || uuid != producer {
						t.Fatalf("control consumer lost preceding producer: %v", e)
					}
					consumer++
				}
			}
			if found != 1 || consumer != 1 {
				t.Fatalf("source call must appear once in region: producers=%d consumers=%d", found, consumer)
			}
		})
	}
}

func TestP15LegacyInlineOracleMutations(t *testing.T) {
	actions, err := p15C03Actions("const a = text(\"FIRST\")\nconst b = text(\"SECOND\")\nconst c = text(\"THIRD\")\nurl(a,b,c)\n", true)
	if err != nil {
		t.Fatal(err)
	}
	if err = p15C03EffectOracle(actions, true); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"drop", "duplicate", "reverse", "foreign_reference"} {
		t.Run(fault, func(t *testing.T) {
			broken := p15C03Clone(t, actions)
			switch fault {
			case "drop":
				broken = broken[1:]
			case "duplicate":
				broken[1].WFWorkflowActionParameters["UUID"] = broken[0].WFWorkflowActionParameters["UUID"]
			case "reverse":
				broken[0], broken[1] = broken[1], broken[0]
			case "foreign_reference":
				broken[0].WFWorkflowActionParameters["UUID"] = "not-the-referenced-producer"
			}
			if p15C03EffectOracle(broken, true) == nil && p15LegacyInlineReferenceOracle(broken) == nil {
				t.Fatal("causal native-graph mutation was not detected")
			}
		})
	}
}

func TestP15LegacyInlineFunctionCLI(t *testing.T) {
	bin := roundTripBinary(t)
	for _, tc := range []struct{ name, source string }{
		{"native_argument", "function echo(text value): text {\noutput(\"{@value}\")\n}\nconst result = echo(text(\"FUNCTION_INPUT\"))\ntext(\"TAIL\")\n"},
		{"function_argument", "function echo(text value): text {\noutput(\"{@value}\")\n}\nurl(echo(\"FUNCTION_INPUT\"))\ntext(\"TAIL\")\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "inline.cherri")
			output := filepath.Join(dir, "inline.shortcut")
			if err := os.WriteFile(file, []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			stdout, stderr, err := p15RunCLI(t, bin, "", nil, "compile", file, "--legacy", "--skip-sign", "-o", output)
			if err != nil {
				t.Fatalf("actual Legacy CLI: %v\n%s\n%s", err, stdout, stderr)
			}
			raw, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := decodeShortcutRawMap(raw)
			if err != nil {
				t.Fatal(err)
			}
			data := p15C03Array(doc["WFWorkflowActions"])
			actions := make([]ShortcutAction, 0, len(data))
			for _, entry := range data {
				row := debugParamMap(entry)
				actions = append(actions, ShortcutAction{WFWorkflowActionIdentifier: fmt.Sprint(row["WFWorkflowActionIdentifier"]), WFWorkflowActionParameters: debugParamMap(row["WFWorkflowActionParameters"])})
			}
			if err = p15LegacyInlineReferenceOracle(actions); err != nil {
				t.Fatal(err)
			}
			tail := 0
			calls := 0
			inputs := 0
			for _, action := range actions {
				switch action.WFWorkflowActionIdentifier {
				case "is.workflow.actions.gettext":
					if action.WFWorkflowActionParameters["WFTextActionText"] == "TAIL" {
						tail++
					}
					if action.WFWorkflowActionParameters["WFTextActionText"] == "FUNCTION_INPUT" {
						inputs++
					}
				case "is.workflow.actions.runworkflow":
					calls++
				}
			}
			if tail != 1 || calls != 1 {
				t.Fatalf("function evaluation/cursor changed: tail=%d calls=%d", tail, calls)
			}
			if tc.name == "native_argument" && inputs != 1 {
				t.Fatalf("inline native function argument producer missing or repeated: %d", inputs)
			}
			if tc.name == "function_argument" {
				urls := p15ActionsNamed(actions, "url")
				if len(urls) != 1 {
					t.Fatalf("outer URL call disappeared: %d", len(urls))
				}
				items := p15C03Array(urls[0].WFWorkflowActionParameters["WFURLActionURL"])
				if len(items) != 1 {
					t.Fatal("function result did not stay one argument")
				}
				if _, err = p15C03TokenUUID(items[0]); err != nil {
					t.Fatalf("function output lost typed native reference: %v", err)
				}
			}
			if strings.Contains(string(raw), "Action argument") {
				t.Fatal("action value was stringified")
			}
		})
	}
}

func TestP15LegacyInlineArgumentBoundariesCLI(t *testing.T) {
	bin := roundTripBinary(t)
	for _, tc := range []struct{ name, control, rejected, diagnostic string }{
		{"void", "url(text(\"VALUE\"))\n", "url(show(\"SIDE_EFFECT\"))\n", "E_ARGUMENT_VALUE_REQUIRED"},
		{"missing_parent_parameter", "#include 'actions/text'\nreplaceText(\"a\",text(\"b\"),text(\"a\"))\n", "#include 'actions/text'\nreplaceText(\"a\",text(\"b\"))\n", ""},
		{"literal_parameter", "makeVCard(\"TITLE\",\"NOTE\")\n", "makeVCard(text(\"TITLE\"),\"NOTE\")\n", "Shortcuts does not allow variable values"},
		{"mixed_dynamic_contact", "emailAddress(\"one@example.test\",\"two@example.test\")\n", "emailAddress(text(\"one@example.test\"),\"two@example.test\")\n", ""},
		{"mixed_dynamic_app", "quitAllApps(\"com.example.one\",\"com.example.two\")\n", "quitAllApps(text(\"com.example.one\"),\"com.example.two\")\n", ""},
		{"canonical_name_shadow", "url(getVariable(ShortcutInput))\n", "action 'gettext' getVariable(text input: 'WFTextActionText'): text\nurl(getVariable(\"SHADOW\"))\n", "Duplicate declaration"},
		{"zero_function_args", "function ping(): text {\noutput(\"PONG\")\n}\nurl(ping())\ntext(\"TAIL\")\n", "function ping(): text {\noutput(\"PONG\")\n}\nurl(ping(\"EXTRA\"))\ntext(\"TAIL\")\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, negative := range []bool{false, true} {
				directory := t.TempDir()
				source := tc.control
				if negative {
					source = tc.rejected
				}
				file, output := filepath.Join(directory, "argument.cherri"), filepath.Join(directory, "argument.shortcut")
				if err := os.WriteFile(file, []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
				stdout, stderr, err := p15RunCLI(t, bin, "", nil, "compile", file, "--legacy", "--skip-sign", "-o", output)
				if !negative {
					if err != nil {
						t.Fatalf("healthy canonical control failed: %v\n%s\n%s", err, stdout, stderr)
					}
					if data, e := os.ReadFile(output); e != nil || len(data) == 0 {
						t.Fatalf("healthy control omitted output: %v", e)
					}
				} else {
					if err == nil {
						t.Fatalf("invalid nested argument compiled: %s", tc.rejected)
					}
					if _, e := os.Stat(output); !os.IsNotExist(e) {
						t.Fatalf("rejected argument wrote output: %v", e)
					}
					if tc.diagnostic != "" && !strings.Contains(string(stdout)+string(stderr), tc.diagnostic) {
						t.Fatalf("missing causal diagnostic %s: %s %s", tc.diagnostic, stdout, stderr)
					}
				}
			}
		})
	}
}

func TestP15LegacyInlineEarlierVariableSnapshot(t *testing.T) {
	source := "@snapshot = \"BEFORE\"\nurl(@snapshot, measure(\"later\") {\n@snapshot = \"AFTER\"\ntext(\"TAIL\")\n})\n"
	actions, err := p15C03Actions(source, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = p15C03SnapshotOracle(actions, true); err != nil {
		t.Fatal(err)
	}
	if err = p15LegacyInlineReferenceOracle(actions); err != nil {
		t.Fatal(err)
	}

	for _, fault := range []string{"omit_capture", "move_capture_after_mutation", "retarget_consumer"} {
		t.Run(fault, func(t *testing.T) {
			broken := p15C03Clone(t, actions)
			capture, mutation := -1, -1
			stores := 0
			for i, action := range broken {
				if action.WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" && action.WFWorkflowActionParameters["WFVariableName"] == "snapshot" {
					stores++
					if stores == 2 {
						mutation = i
					}
				}
				if action.WFWorkflowActionIdentifier == "is.workflow.actions.getvariable" {
					ref := p15OutputReference(action.WFWorkflowActionParameters["WFVariable"])
					if ref["Type"] == "Variable" && ref["VariableName"] == "snapshot" {
						capture = i
					}
				}
			}
			if capture < 0 || mutation < 0 {
				t.Fatal("healthy snapshot precondition missing")
			}
			switch fault {
			case "omit_capture":
				broken = append(broken[:capture], broken[capture+1:]...)
			case "move_capture_after_mutation":
				broken[capture], broken[mutation] = broken[mutation], broken[capture]
			case "retarget_consumer":
				broken[capture].WFWorkflowActionParameters["UUID"] = "foreign-output"
			}
			if p15C03SnapshotOracle(broken, true) == nil && p15LegacyInlineReferenceOracle(broken) == nil {
				t.Fatal("snapshot corruption was accepted")
			}
		})
	}
}

func TestP15LegacyInlineTypedCustomCLI(t *testing.T) {
	bin := roundTripBinary(t)
	for _, source := range []string{
		"action 'gettext' customTypedValue(text input: 'WFTextActionText'): text\nurl(customTypedValue(\"CUSTOM_VALUE\"))\n",
		"action 'gettext' customTypedValue(text input: 'WFTextActionText'): text {}\nurl(customTypedValue(\"CUSTOM_VALUE\"))\n",
		"action 'gettext' customTypedValue(text input: 'WFTextActionText'): text\nurl(measure(\"shadow\") {\ncustomTypedValue(\"CUSTOM_VALUE\")\n})\n",
	} {
		dir := t.TempDir()
		file, output := filepath.Join(dir, "shadow.cherri"), filepath.Join(dir, "shadow.shortcut")
		if err := os.WriteFile(file, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, err := p15RunCLI(t, bin, "", nil, "compile", file, "--legacy", "--skip-sign", "-o", output)
		if err != nil {
			t.Fatalf("selected typed custom action rejected: %v\n%s\n%s", err, stdout, stderr)
		}
		raw, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := decodeShortcutRawMap(raw)
		if err != nil {
			t.Fatal(err)
		}
		var actions []ShortcutAction
		for _, entry := range p15C03Array(doc["WFWorkflowActions"]) {
			row := debugParamMap(entry)
			actions = append(actions, ShortcutAction{WFWorkflowActionIdentifier: fmt.Sprint(row["WFWorkflowActionIdentifier"]), WFWorkflowActionParameters: debugParamMap(row["WFWorkflowActionParameters"])})
		}
		if err := p15LegacyInlineReferenceOracle(actions); err != nil {
			t.Fatal(err)
		}
		found, consumed := 0, 0
		for _, action := range actions {
			if action.WFWorkflowActionIdentifier == "is.workflow.actions.gettext" && action.WFWorkflowActionParameters["WFTextActionText"] == "CUSTOM_VALUE" {
				found++
			}
			if action.WFWorkflowActionIdentifier == "is.workflow.actions.url" {
				items := p15C03Array(action.WFWorkflowActionParameters["WFURLActionURL"])
				if len(items) != 1 {
					t.Fatal("custom result is not one argument")
				}
				if _, err := p15C03TokenUUID(items[0]); err != nil {
					t.Fatal(err)
				}
				consumed++
			}
		}
		if found != 1 || consumed != 1 {
			t.Fatalf("typed custom producer/consumer changed: %d/%d", found, consumed)
		}
	}
}

func TestP15LegacyInlineTemplateSnapshot(t *testing.T) {
	for _, source := range []string{
		"@snapshot = \"BEFORE\"\nurl(\"{@snapshot}\", measure(\"later\") {\n@snapshot = \"AFTER\"\ntext(\"TAIL\")\n})\n",
		"@snapshot = \"BEFORE\"\nconst output = measure(\"outer\") {\nurl(\"{@snapshot}\", measure(\"later\") {\n@snapshot = \"AFTER\"\ntext(\"TAIL\")\n})\n}\n",
	} {
		actions, err := p15C03Actions(source, true)
		if err != nil {
			t.Fatal(err)
		}
		if err = p15C03SnapshotOracle(actions, true); err != nil {
			t.Fatal(err)
		}
		if err = p15LegacyInlineReferenceOracle(actions); err != nil {
			t.Fatal(err)
		}
	}
}

func TestP15LegacyInlineCapturedNativeDefinition(t *testing.T) {
	if _, err := p15C03Actions("@snapshot = \"BEFORE\"\ntext(\"CONTROL\")\n", true); err != nil {
		t.Fatal(err)
	}
	original := actions["getVariable"]
	defer func() { actions["getVariable"] = original }()
	reference := actionArgument{valueType: Variable, value: varValue{variableType: "Variable", valueType: Variable, value: "snapshot"}}
	call := legacyInlineSnapshotAction(reference)
	// A conflicting late callable lookup must not replace the captured native
	// operation. The same-name source declaration is independently rejected by
	// the actual CLI test above.
	actions["getVariable"] = &actionDefinition{identifier: "gettext", parameters: []parameterDefinition{{name: "text", validType: String, key: "WFTextActionText"}}, outputType: String}
	var value any = call
	id := "11111111-2222-4333-8444-555555555555"
	makeVariableValue(&WFActionReference{UUID: id, CustomOutputName: "CAPTURE"}, Action, &value)
	emitted := shortcut.WFWorkflowActions[len(shortcut.WFWorkflowActions)-1]
	if emitted.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" || emitted.WFWorkflowActionParameters["UUID"] != id {
		t.Fatal("captured native definition was rebound by source lookup")
	}
	raw, err := json.Marshal(emitted.WFWorkflowActionParameters)
	if err != nil {
		t.Fatal(err)
	}
	var params map[string]any
	if err := json.Unmarshal(raw, &params); err != nil {
		t.Fatal(err)
	}
	ref := p15OutputReference(params["WFVariable"])
	if ref["Type"] != "Variable" || ref["VariableName"] != "snapshot" {
		t.Fatalf("snapshot identity changed: %v", ref)
	}
}
