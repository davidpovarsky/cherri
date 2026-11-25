package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Alpha-rename only proven private native Variable identities. URL/header Text
// and every native operation/parameter/control edge remain exact comparisons.
func p15PaginateDefaultGraphIdentity(actions []ShortcutAction) ([]map[string]any, error) {
	rows, err := p15ConditionGraphIdentity(actions)
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	prefix := ""
	for _, action := range actions {
		if action.WFWorkflowActionIdentifier != "is.workflow.actions.setvariable" {
			continue
		}
		name, okay := action.WFWorkflowActionParameters["WFVariableName"].(string)
		if !okay || !strings.HasPrefix(name, "__cherri_paginate_") {
			return nil, fmt.Errorf("unowned pagination declaration")
		}
		if prefix == "" {
			if !strings.HasSuffix(name, "_results") {
				return nil, fmt.Errorf("first declaration is not results")
			}
			prefix = strings.TrimSuffix(name, "results")
		}
		if !strings.HasPrefix(name, prefix) {
			return nil, fmt.Errorf("private variable belongs to another invocation")
		}
		names[name] = "owned:" + strings.TrimPrefix(name, prefix)
	}
	var walk func(any) error
	walk = func(raw any) error {
		switch v := raw.(type) {
		case map[string]any:
			if v["Type"] == "Variable" {
				name, _ := v["VariableName"].(string)
				if strings.HasPrefix(name, "__cherri_paginate_") {
					if names[name] == "" {
						return fmt.Errorf("unbound private variable")
					}
					v["VariableName"] = names[name]
				}
			}
			for _, child := range v {
				if err := walk(child); err != nil {
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
		p := debugParamMap(row["WFWorkflowActionParameters"])
		id, _ := row["WFWorkflowActionIdentifier"].(string)
		if id == "is.workflow.actions.setvariable" || id == "is.workflow.actions.appendvariable" {
			name, _ := p["WFVariableName"].(string)
			if names[name] == "" {
				return nil, fmt.Errorf("private write detached from declaration")
			}
			p["WFVariableName"] = names[name]
		}
		if err := walk(row); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

func TestP15PaginateTraversalDefaultGraphEquivalence(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run(fmt.Sprintf("legacy=%t/%s", legacy, profile), func(t *testing.T) {
				tc := p15PaginateTraversalCase{start: "https://example.invalid/pages/ABCD1234-0000-0000-0000-000000000000.json"}
				omitted := p15PaginateTraversalSource(tc, legacy, false, false, 3, 6, true)
				tc.mode = "url"
				explicit := p15PaginateTraversalSource(tc, legacy, false, false, 3, 6, true)
				a, err := compileBuildProfileFixture(t, omitted, legacy, profile)
				if err != nil {
					t.Fatal(err)
				}
				b, err := compileBuildProfileFixture(t, explicit, legacy, profile)
				if err != nil {
					t.Fatal(err)
				}
				left, err := p15PaginateDefaultGraphIdentity(a)
				if err != nil {
					t.Fatal(err)
				}
				right, err := p15PaginateDefaultGraphIdentity(b)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(left, right) {
					t.Fatal("explicit URL mode changed the complete native default graph")
				}
				if dir := os.Getenv("P15_PAGINATE_MODES_OUT"); dir != "" {
					data, err := json.MarshalIndent(map[string]any{"test": t.Name(), "legacy": legacy, "profile": profile,
						"omittedSource": omitted, "explicitSource": explicit, "omittedActions": a, "explicitActions": b}, "", "  ")
					if err != nil {
						t.Fatal(err)
					}
					name := strings.NewReplacer("/", "__", "=", "-").Replace(t.Name()) + ".json"
					if err := os.WriteFile(filepath.Join(dir, name), append(data, '\n'), 0644); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

func TestP15PaginateTraversalDefaultIdentityRejectsMutations(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			tc := p15PaginateTraversalCase{start: "https://example.invalid/pages/ABCD1234-0000-0000-0000-000000000000.json"}
			actions, err := compileBuildProfileFixture(t, p15PaginateTraversalSource(tc, legacy, false, false, 3, 6, true), legacy, BuildProfileDebug)
			if err != nil {
				t.Fatal(err)
			}
			expected, err := p15PaginateDefaultGraphIdentity(actions)
			if err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"literal_uuid_text", "page_cap", "selector", "foreign_private_slot", "detached_reference", "extra_effect"} {
				t.Run(fault, func(t *testing.T) {
					raw, _ := json.Marshal(actions)
					var broken []ShortcutAction
					if err := json.Unmarshal(raw, &broken); err != nil {
						t.Fatal(err)
					}
					changed := false
					for index := range broken {
						p := broken[index].WFWorkflowActionParameters
						id := broken[index].WFWorkflowActionIdentifier
						switch fault {
						case "literal_uuid_text":
							if p["WFTextActionText"] == tc.start {
								p["WFTextActionText"] = strings.Replace(tc.start, "ABCD1234", "DEAD5678", 1)
								changed = true
							}
						case "page_cap":
							if _, ok := p["WFRepeatCount"]; ok {
								p["WFRepeatCount"] = 4
								changed = true
							}
						case "selector":
							if p["WFDictionaryKey"] == "items" {
								p["WFDictionaryKey"] = "other"
								changed = true
							}
						case "foreign_private_slot":
							if id == "is.workflow.actions.setvariable" {
								p["WFVariableName"] = "__cherri_paginate_FOREIGN_results"
								changed = true
							}
						case "detached_reference":
							if id == "is.workflow.actions.setvariable" {
								debugParamMap(debugParamMap(p["WFInput"])["Value"])["OutputUUID"] = "MISSING_PRODUCER"
								changed = true
							}
						case "extra_effect":
							broken[index].WFWorkflowActionIdentifier = "is.workflow.actions.wait"
							changed = true
						}
						if changed {
							break
						}
					}
					if !changed {
						t.Fatal("causal fault did not alter the actual graph")
					}
					observed, err := p15PaginateDefaultGraphIdentity(broken)
					if err == nil && reflect.DeepEqual(expected, observed) {
						t.Fatal("native graph corruption was erased by identity normalization")
					}
				})
			}
		})
	}
}
