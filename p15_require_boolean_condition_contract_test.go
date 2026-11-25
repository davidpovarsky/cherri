package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func p15RequireCloneActions(t *testing.T, actions []ShortcutAction) []ShortcutAction {
	t.Helper()
	data, err := json.Marshal(actions)
	if err != nil {
		t.Fatal(err)
	}
	var copy []ShortcutAction
	if err = json.Unmarshal(data, &copy); err != nil {
		t.Fatal(err)
	}
	return copy
}

func TestP15RequireBooleanConditionNativeProducers(t *testing.T) {
	for _, storage := range []string{"", "immutable", "mutable"} {
		for _, legacy := range []bool{false, true} {
			for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
				name := storage
				if name == "" {
					name = "direct"
				}
				t.Run(fmt.Sprintf("%s/legacy=%t/%s", name, legacy, profile), func(t *testing.T) {
					tc := p15RequireConditionCase{name: name, storage: storage, native: true}
					source := p15RequireConditionSource(tc, legacy)
					actions, err := p15RequireConditionCompile(t, source, legacy, profile)
					if err != nil {
						t.Fatal(err)
					}
					if err = p15RequireConditionGraph(actions, tc, legacy); err != nil {
						t.Fatal(err)
					}
					if !legacy {
						p15RequireConditionAnalyzedBool(t, source)
					}
				})
			}
		}
	}
}

func TestP15RequireBooleanConditionSourceProof(t *testing.T) {
	attachment := p15FunctionEdge("AUTHORED_BOOL_PRODUCER")
	variable := p15FunctionVariable("authored_bool")
	for _, tc := range []struct {
		name         string
		raw          any
		hint         any
		setHint      bool
		condition    any
		setCondition bool
		okay         bool
	}{
		{"literal_true", true, nil, false, nil, false, true},
		{"literal_false", false, nil, false, nil, false, true},
		{"proved_literal", false, "Bool", true, nil, false, true},
		{"proved_output", attachment, "Bool", true, nil, false, true},
		{"proved_variable", variable, "Bool", true, nil, false, true},
		{"proved_boolean_spelling", attachment, "Boolean", true, nil, false, true},
		{"strict_condition_precedence", attachment, "Bool", true, "", true, false},
		{"proved_condition", attachment, "AnyContent", true, "Bool", true, true},
		{"unproved_output", attachment, nil, false, nil, false, false},
		{"unproved_variable", variable, nil, false, nil, false, false},
		{"anycontent", attachment, "AnyContent", true, nil, false, false},
		{"optional", attachment, "Bool?", true, nil, false, false},
		{"optional_generic", attachment, "Optional<Bool>", true, nil, false, false},
		{"unknown", attachment, "Unknown", true, nil, false, false},
		{"number_hint", attachment, "Number", true, nil, false, false},
		{"invalid_hint", attachment, true, true, nil, false, false},
		{"zero", 0, "Bool", true, nil, false, false},
		{"one", 1, "Bool", true, nil, false, false},
		{"text", "false", "Bool", true, nil, false, false},
		{"none", nil, "Bool", true, nil, false, false},
		{"list", []any{false}, "Bool", true, nil, false, false},
		{"map", map[string]any{"flag": false}, "Bool", true, nil, false, false},
		{"absent_body", map[string]any{"WFSerializationType": "WFTextTokenAttachment"}, "Bool", true, nil, false, false},
		{"wrong_body_type", map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": "false"}, "Bool", true, nil, false, false},
		{"missing_uuid", p15FunctionAttachment(map[string]any{"Type": "ActionOutput"}), "Bool", true, nil, false, false},
		{"nonstring_uuid", p15FunctionAttachment(map[string]any{"Type": "ActionOutput", "OutputUUID": 1}), "Bool", true, nil, false, false},
		{"missing_name", p15FunctionAttachment(map[string]any{"Type": "Variable"}), "Bool", true, nil, false, false},
		{"system_reference", p15FunctionAttachment(map[string]any{"Type": "Clipboard"}), "Bool", true, nil, false, false},
		{"wrong_serialization", map[string]any{"WFSerializationType": "WFTextTokenString", "Value": map[string]any{"Type": "ActionOutput", "OutputUUID": "AUTHORED_BOOL_PRODUCER"}}, "Bool", true, nil, false, false},
		{"transformed_output", p15FunctionAttachment(map[string]any{"Type": "ActionOutput", "OutputUUID": "AUTHORED_BOOL_PRODUCER", "Aggrandizements": []any{map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFNumberContentItem"}}}), "Bool", true, nil, false, false},
		{"malformed_transforms", p15FunctionAttachment(map[string]any{"Type": "ActionOutput", "OutputUUID": "AUTHORED_BOOL_PRODUCER", "Aggrandizements": false}), "Bool", true, nil, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := map[string]any{"WFInput": tc.raw}
			if tc.setHint {
				params[requireSourceTypeKey] = tc.hint
			}
			if tc.setCondition {
				params[requireConditionTypeKey] = tc.condition
			}
			actions, err := buildRequireMacroActions(params, "REQUIRE_RESULT", "checked")
			if tc.okay {
				if err != nil {
					t.Fatal(err)
				}
				conditions := 0
				for _, a := range actions {
					if a.WFWorkflowActionIdentifier == "is.workflow.actions.conditional" && p15Int(a.WFWorkflowActionParameters["WFControlFlowMode"]) == 0 {
						conditions++
						if p15Int(a.WFWorkflowActionParameters["WFCondition"]) != 4 || fmt.Sprint(a.WFWorkflowActionParameters["WFNumberValue"]) != "0" {
							t.Fatal("condition did not use explicit numeric false equality")
						}
					}
				}
				if conditions != 1 || len(actions) == 0 || actions[len(actions)-1].WFWorkflowActionParameters["UUID"] != "REQUIRE_RESULT" {
					t.Fatal("condition lost single guard or typed result identity")
				}
			} else if err == nil || len(actions) != 0 || !strings.Contains(err.Error(), "source-proved Bool") {
				t.Fatalf("unproved condition emitted output: actions=%d err=%v", len(actions), err)
			}
		})
	}
}

func TestP15RequireBooleanConditionLegacyTypeProof(t *testing.T) {
	saved := variables
	defer func() { variables = saved }()
	ref := func(name string) actionArgument {
		return actionArgument{valueType: Variable, value: varValue{valueType: Variable, value: name}}
	}
	boolDefinition := varValue{valueType: Bool, value: false, constant: true}
	call := action{ident: "declaredBooleanProducer", def: &actionDefinition{outputType: Bool}}
	for _, tc := range []struct {
		name        string
		argument    actionArgument
		environment map[string]varValue
		okay        bool
	}{
		{"literal", actionArgument{valueType: Bool, value: false}, nil, true},
		{"declaration", ref("flag"), map[string]varValue{"flag": boolDefinition}, true},
		{"action", actionArgument{valueType: Action, value: call}, nil, true},
		{"action_declaration", ref("flag"), map[string]varValue{"flag": {valueType: Action, value: call, constant: true}}, true},
		{"alias_chain", ref("alias"), map[string]varValue{"flag": boolDefinition, "middle": {valueType: Variable, value: varValue{value: "flag"}}, "alias": {valueType: Variable, value: varValue{value: "middle"}}}, true},
		{"cycle", ref("alias"), map[string]varValue{"alias": {valueType: Variable, value: varValue{value: "alias"}}}, false},
		{"missing", ref("missing"), nil, false},
		{"number", actionArgument{valueType: Integer, value: 0}, nil, false},
		{"unknown_producer", actionArgument{valueType: Action, value: action{ident: "unknown", def: &actionDefinition{outputType: Variable}}}, nil, false},
		{"repeat_item", ref("flag"), map[string]varValue{"flag": {valueType: Bool, value: false, repeatItem: true}}, false},
		{"coercion", actionArgument{valueType: Variable, value: varValue{value: "flag", coerce: "boolean"}}, map[string]varValue{"flag": boolDefinition}, false},
		{"property", actionArgument{valueType: Variable, value: varValue{value: "flag", getAs: "value"}}, map[string]varValue{"flag": boolDefinition}, false},
		{"raw_descriptor", actionArgument{valueType: Variable, value: varValue{value: "flag", descriptor: &ReferenceDescriptor{Kind: "ActionOutput", OutputUUID: "OTHER"}}}, map[string]varValue{"flag": boolDefinition}, false},
		{"clipboard", ref("Clipboard"), nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			variables = tc.environment
			got := requireLegacyConditionType(tc.argument)
			if (got == "Bool") != tc.okay {
				t.Fatalf("type proof %q, want proved=%t", got, tc.okay)
			}
		})
	}
}

func TestP15RequireBooleanConditionRejectsUnprovedSource(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"number", "require(0)"},
		{"text", "require(\"false\")"},
		{"none", "require(none)"},
		{"list", "require([false])"},
		{"map", "require({\"flag\":false})"},
		{"optional", "var flag: Bool? = none\nrequire(flag)"},
		{"native_unknown", "require(Clipboard)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CompileSourceToPlist("require-condition-proof.cherri", tc.source)
			if err == nil || !strings.Contains(err.Error(), "require condition must be a source-proved Bool") {
				t.Fatalf("missing precise condition proof diagnostic: %v", err)
			}
		})
	}
}

func TestP15RequireBooleanConditionGraphFaults(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			tc := p15RequireConditionCase{name: "mutable_false", storage: "mutable"}
			source := p15RequireConditionSource(tc, legacy)
			original, err := p15RequireConditionCompile(t, source, legacy, BuildProfileDebug)
			if err != nil {
				t.Fatal(err)
			}
			if err = p15RequireConditionGraph(original, tc, legacy); err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"literal_value", "literal_kind", "duplicate_producer", "lost_snapshot", "duplicate_snapshot", "snapshot_variable", "truthiness", "opposite_predicate", "wrong_operand", "wrong_guard_source", "cancel_button", "message", "missing_stop", "wrong_group", "wrong_result_source", "consumer_bypass", "wrong_operand_wire", "private_option_leak", "assignment_wrapper_presence", "assignment_wrapper_value"} {
				t.Run(fault, func(t *testing.T) {
					bad := p15RequireCloneActions(t, original)
					condition, alert, stop, snapshot, result, consumer := -1, -1, -1, -1, -1, -1
					for i, a := range bad {
						switch a.WFWorkflowActionIdentifier {
						case "is.workflow.actions.conditional":
							if p15Int(a.WFWorkflowActionParameters["WFControlFlowMode"]) == 0 {
								condition = i
							}
						case "is.workflow.actions.alert":
							alert = i
						case "is.workflow.actions.exit":
							stop = i
						case "is.workflow.actions.getvariable":
							ref, _ := a.WFWorkflowActionParameters["WFVariable"].(map[string]any)
							body, _ := ref["Value"].(map[string]any)
							if body["Type"] == "Variable" {
								snapshot = i
							} else if a.WFWorkflowActionParameters["UUID"] != nil {
								result = i
							} else {
								consumer = i
							}
						}
					}
					if condition < 0 || alert < 0 || stop < 0 || snapshot < 0 || result < 0 || consumer < 0 {
						t.Fatal("missing actual mutation target")
					}
					firstID := bad[0].WFWorkflowActionParameters["UUID"].(string)
					switch fault {
					case "literal_value":
						bad[0].WFWorkflowActionParameters["WFNumberActionNumber"] = 0.5
					case "literal_kind":
						bad[0].WFWorkflowActionIdentifier = "is.workflow.actions.gettext"
					case "duplicate_producer":
						bad = append(bad[:1], append([]ShortcutAction{bad[0]}, bad[1:]...)...)
					case "lost_snapshot":
						bad = append(bad[:snapshot], bad[snapshot+1:]...)
					case "duplicate_snapshot":
						bad = append(bad[:snapshot], append([]ShortcutAction{bad[snapshot]}, bad[snapshot:]...)...)
					case "snapshot_variable":
						bad[snapshot].WFWorkflowActionParameters["WFVariable"] = p15FunctionVariable("different_flag")
					case "truthiness":
						bad[condition].WFWorkflowActionParameters["WFCondition"] = 100
					case "opposite_predicate":
						bad[condition].WFWorkflowActionParameters["WFCondition"] = 5
					case "wrong_operand":
						bad[condition].WFWorkflowActionParameters["WFNumberValue"] = 1
					case "wrong_guard_source":
						bad[condition].WFWorkflowActionParameters["WFInput"] = map[string]any{"Type": "Variable", "Variable": p15FunctionEdge(firstID)}
					case "cancel_button":
						bad[alert].WFWorkflowActionParameters["WFAlertActionCancelButtonShown"] = true
					case "message":
						bad[alert].WFWorkflowActionParameters["WFAlertActionMessage"] = "different failure"
					case "missing_stop":
						bad = append(bad[:stop], bad[stop+1:]...)
					case "wrong_group":
						bad[stop+1].WFWorkflowActionParameters["GroupingIdentifier"] = "different_group"
					case "wrong_result_source":
						bad[result].WFWorkflowActionParameters["WFVariable"] = p15FunctionEdge(firstID)
					case "consumer_bypass":
						bad[consumer].WFWorkflowActionParameters["WFVariable"] = p15FunctionEdge(firstID)
					case "wrong_operand_wire":
						delete(bad[condition].WFWorkflowActionParameters, "WFNumberValue")
						bad[condition].WFWorkflowActionParameters["WFConditionalActionString"] = "0"
					case "assignment_wrapper_presence":
						if legacy {
							delete(bad[1].WFWorkflowActionParameters, "WFSerializationType")
						} else {
							bad[1].WFWorkflowActionParameters["WFSerializationType"] = "WFTextTokenAttachment"
						}
					case "assignment_wrapper_value":
						bad[1].WFWorkflowActionParameters["WFSerializationType"] = "WFTextTokenString"
					case "private_option_leak":
						bad[condition].WFWorkflowActionParameters[requireConditionTypeKey] = "Bool"
					}
					if err := p15RequireConditionGraph(bad, tc, legacy); err == nil {
						t.Fatal("full Boolean condition oracle accepted causal mutation")
					}
				})
			}
		})
	}
}
