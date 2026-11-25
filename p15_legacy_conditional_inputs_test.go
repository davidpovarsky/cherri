package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Canonicalize only graph identities, after checking that every output edge
// still names its actual producer. All native operations, parameter values,
// ordering and control edges remain in the comparison.
func p15ConditionGraphIdentity(actions []ShortcutAction) ([]map[string]any, error) {
	if err := p15LegacyInlineReferenceOracle(actions); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(actions)
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	ids, names, groups := map[string]string{}, map[string]string{}, map[string]string{}
	for i, action := range actions {
		p := action.WFWorkflowActionParameters
		if uuid, ok := p["UUID"].(string); ok && uuid != "" {
			ids[uuid] = fmt.Sprintf("producer-%d", i)
			if name, ok := p["CustomOutputName"].(string); ok {
				names[uuid] = name
			}
		}
		if group, ok := p["GroupingIdentifier"].(string); ok && group != "" {
			if p15Int(p["WFControlFlowMode"]) == 0 {
				if _, exists := groups[group]; exists {
					return nil, fmt.Errorf("duplicate group start")
				}
				groups[group] = fmt.Sprintf("group-%d", i)
			} else if groups[group] == "" {
				return nil, fmt.Errorf("group continuation without start")
			}
		}
	}
	var walk func(any) error
	walk = func(value any) error {
		switch v := value.(type) {
		case map[string]any:
			if v["Type"] == "ActionOutput" {
				uuid, _ := v["OutputUUID"].(string)
				if ids[uuid] == "" {
					return fmt.Errorf("missing output producer")
				}
				if name, exists := v["OutputName"]; exists && names[uuid] != "" {
					if name != names[uuid] {
						return fmt.Errorf("output name detached from producer: %v / %v", name, names[uuid])
					}
					v["OutputName"] = ids[uuid]
				}
				v["OutputUUID"] = ids[uuid]
			}
			if uuid, ok := v["UUID"].(string); ok {
				if ids[uuid] == "" {
					return fmt.Errorf("unknown producer UUID")
				}
				v["UUID"] = ids[uuid]
				if _, exists := v["CustomOutputName"]; exists {
					v["CustomOutputName"] = ids[uuid]
				}
			}
			if group, ok := v["GroupingIdentifier"].(string); ok {
				if groups[group] == "" {
					return fmt.Errorf("unknown control group")
				}
				v["GroupingIdentifier"] = groups[group]
			}
			keys := make([]string, 0, len(v))
			for key := range v {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				if err := walk(v[key]); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range v {
				if err := walk(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, row := range rows {
		if err := walk(row); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

type p15ConditionInputCase struct{ name, input, use string }

var p15ConditionInputCases = []p15ConditionInputCase{
	{"one", "1", "%s == 1"},
	{"zero", "0", "%s == 0"},
	{"negative", "-3", "%s < 0"},
	{"decimal", "1.5", "%s > 1"},
	{"true", "true", "%s == true"},
	{"false", "false", "%s == false"},
	{"text", "\"owner@example.invalid\"", "%s == \"owner@example.invalid\""},
	{"empty", "\"\"", "%s == \"\""},
	{"raw", "'Ask #define from sharesheet'", "%s == 'Ask #define from sharesheet'"},
	{"unicode", "\"שלום 🍒\"", "%s == \"שלום 🍒\""},
	{"text_action", "text(\"ONCE\")", "%s == \"ONCE\""},
	{"number_action", "number(7)", "%s == 7"},
	{"dictionary_presence", "{\"flag\":false,\"zero\":0,\"empty\":\"\"}", "%s"},
	{"is_true", "true", "isTrue(%s)"},
	{"is_false", "false", "isFalse(%s)"},
	{"not_empty", "\"\"", "!%s"},
}

func p15ConditionInputSource(tc p15ConditionInputCase, version string, explicit, loop bool) string {
	body := "text(\"CHERRI_BEFORE\")\n"
	input := tc.input
	if explicit {
		body += "const conditionInput = " + input + "\n"
		input = "conditionInput"
	}
	body += "if " + fmt.Sprintf(tc.use, input) + " {\nwait(1)\n} else {\nwait(2)\n}\ntext(\"CHERRI_AFTER\")\n"
	if loop {
		body = "repeat index for 2 {\n" + body + "}\n"
	}
	return "#define version " + version + "\n" + body
}

func TestP15LegacyConditionalLiteralInputs(t *testing.T) {
	for _, tc := range p15ConditionInputCases {
		for _, version := range []string{"17", "27"} {
			for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
				t.Run(tc.name+"/v"+version+"/"+string(profile), func(t *testing.T) {
					control, err := compileBuildProfileFixture(t, p15ConditionInputSource(tc, version, true, false), true, profile)
					if err != nil {
						t.Fatalf("explicit original control: %v", err)
					}
					want, err := p15ConditionGraphIdentity(control)
					if err != nil {
						t.Fatalf("original control graph: %v", err)
					}
					actual, err := compileBuildProfileFixture(t, p15ConditionInputSource(tc, version, false, false), true, profile)
					if err != nil {
						t.Fatalf("inline literal/action condition: %v", err)
					}
					got, err := p15ConditionGraphIdentity(actual)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("native conditional graph differs from explicit value contract\ngot=%v\nwant=%v", got, want)
					}
				})
			}
		}
	}
}

func TestP15LegacyConditionalInlineControlPlacement(t *testing.T) {
	for _, tc := range []p15ConditionInputCase{p15ConditionInputCases[0], p15ConditionInputCases[10], p15ConditionInputCases[15]} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run(tc.name+"/"+string(profile), func(t *testing.T) {
				control, err := compileBuildProfileFixture(t, p15ConditionInputSource(tc, "27", true, true), true, profile)
				if err != nil {
					t.Fatal(err)
				}
				want, err := p15ConditionGraphIdentity(control)
				if err != nil {
					t.Fatal(err)
				}
				actual, err := compileBuildProfileFixture(t, p15ConditionInputSource(tc, "27", false, true), true, profile)
				if err != nil {
					t.Fatal(err)
				}
				got, err := p15ConditionGraphIdentity(actual)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("input producer escaped authored repeat or changed graph\ngot=%v\nwant=%v", got, want)
				}
			})
		}
	}
}

func TestP15LegacyConditionalLiteralOracleControls(t *testing.T) {
	control, err := compileBuildProfileFixture(t, p15ConditionInputSource(p15ConditionInputCases[0], "27", true, false), true, BuildProfileDebug)
	if err != nil {
		t.Fatal(err)
	}
	want, err := p15ConditionGraphIdentity(control)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"drop_input", "duplicate_input", "wrong_value", "wrong_compare", "wrong_edge", "wrong_group", "wrong_branch", "wrong_name"} {
		t.Run(fault, func(t *testing.T) {
			bad := p15C03Clone(t, control)
			input, conditional := -1, -1
			for i, a := range bad {
				if strings.HasSuffix(a.WFWorkflowActionIdentifier, ".number") {
					input = i
				}
				if conditional < 0 && strings.HasSuffix(a.WFWorkflowActionIdentifier, ".conditional") {
					conditional = i
				}
			}
			if input < 0 || conditional < 0 {
				t.Fatal("control missing number or condition")
			}
			switch fault {
			case "drop_input":
				bad = append(bad[:input], bad[input+1:]...)
			case "duplicate_input":
				bad = append(bad, bad[input])
			case "wrong_value":
				bad[input].WFWorkflowActionParameters["WFNumberActionNumber"] = 99
			case "wrong_compare":
				bad[conditional].WFWorkflowActionParameters["WFNumberValue"] = 99
			case "wrong_edge":
				bad[input].WFWorkflowActionParameters["UUID"] = "foreign-producer"
			case "wrong_group":
				bad[conditional].WFWorkflowActionParameters["GroupingIdentifier"] = "foreign-group"
			case "wrong_branch":
				bad[conditional+1].WFWorkflowActionParameters["WFDelayTime"] = 99
			case "wrong_name":
				bad[input].WFWorkflowActionParameters["CustomOutputName"] = "foreign-name"
			}
			got, err := p15ConditionGraphIdentity(bad)
			if err == nil && reflect.DeepEqual(got, want) {
				t.Fatal("graph oracle accepted fault", fault)
			}
		})
	}
}
