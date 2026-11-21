package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

const p15C01Function = "function c01Function(a: Text, b: Text, c: Text) -> Text {return a}\n"
const p15C01Alias = "function c01Function(_a: Text, b: Text, c: Text) -> Text {return _a}\n"
const p15C01Custom = "action c01Native(a: Text, b: Text) -> Text {\nidentifier: \"com.example.c01.order\"\nprimary: \"b\"\nbindings: {a: {key: \"A\", codec: \"text\"}, b: {key: \"B\", codec: \"text\"}}\n}\n"

func p15C01Params(action map[string]any) map[string]any {
	parameters, _ := action["WFWorkflowActionParameters"].(map[string]any)
	return parameters
}

func p15C01Clone(t *testing.T, actions []map[string]any) []map[string]any {
	t.Helper()
	raw, err := json.Marshal(actions)
	if err != nil {
		t.Fatal(err)
	}
	var clone []map[string]any
	if err := json.Unmarshal(raw, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func p15C01MarkerOrder(actions []map[string]any, expected []string) (map[string]string, error) {
	var actual []string
	ids := map[string]string{}
	for _, action := range actions {
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" {
			continue
		}
		p := p15C01Params(action)
		text, ok := p["WFTextActionText"].(string)
		if !ok || !strings.HasPrefix(text, "P15_C01_") {
			continue
		}
		actual = append(actual, text)
		id, _ := p["UUID"].(string)
		ids[text] = id
	}
	if !reflect.DeepEqual(actual, expected) {
		return nil, fmt.Errorf("actual producer order %v, want %v exactly once", actual, expected)
	}
	for _, marker := range expected {
		if ids[marker] == "" {
			return nil, fmt.Errorf("marker %s has no producer UUID", marker)
		}
	}
	return ids, nil
}

func p15C01CallFields(t *testing.T, actions []map[string]any, native bool) map[string]any {
	t.Helper()
	var result map[string]any
	found := 0
	for _, action := range actions {
		if native {
			if action["WFWorkflowActionIdentifier"] == "com.example.c01.order" {
				result = p15C01Params(action)
				found++
			}
			continue
		}
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.dictionary" {
			continue
		}
		fields := p15LiteralDictionaryFields(t, p15C01Params(action)["WFItems"])
		if fields["_cherri_magic"] == nil {
			continue
		}
		found++
		result = map[string]any{}
		for key, field := range fields {
			result[key] = field["WFValue"]
		}
	}
	if found != 1 {
		t.Fatalf("call payloads=%d want one", found)
	}
	return result
}

func p15C01References(fields map[string]any, expected map[string]string, ids map[string]string) error {
	for key, marker := range expected {
		outputs, variables := p15V02NativeReferences(fields[key])
		if len(variables) != 0 || len(outputs) != 1 || outputs[0] != ids[marker] {
			return fmt.Errorf("%s lost source-bound producer for %s: outputs=%v variables=%v", key, marker, outputs, variables)
		}
	}
	return nil
}

// Source-order effects must precede every encoding or invocation that consumes
// them. Relative producer order and a matching UUID alone do not prove causality.
func p15C01EffectsBeforeConsumers(actions []map[string]any, ids map[string]string) error {
	positions := map[string]int{}
	for index, action := range actions {
		if uuid, ok := p15C01Params(action)["UUID"].(string); ok && uuid != "" {
			if _, duplicate := positions[uuid]; duplicate {
				return fmt.Errorf("duplicate producer identity")
			}
			positions[uuid] = index
		}
	}
	for marker, uuid := range ids {
		producer, exists := positions[uuid]
		if !exists {
			return fmt.Errorf("missing effect producer %s", marker)
		}
		consumers := 0
		for index, action := range actions {
			refs, _ := p15V02NativeReferences(p15C01Params(action))
			for _, ref := range refs {
				if ref != uuid {
					continue
				}
				consumers++
				if producer >= index {
					return fmt.Errorf("effect %s was not produced before its binding/consumer", marker)
				}
			}
		}
		if consumers == 0 {
			return fmt.Errorf("effect %s has no actual binding/consumer", marker)
		}
	}
	return nil
}

func p15C01RejectLateProducers(t *testing.T, actions []map[string]any, ids map[string]string) {
	t.Helper()
	if err := p15C01EffectsBeforeConsumers(actions, ids); err != nil {
		t.Fatal(err)
	}
	var retained, effects []map[string]any
	for _, action := range p15C01Clone(t, actions) {
		isEffect := false
		for _, uuid := range ids {
			if p15C01Params(action)["UUID"] == uuid {
				isEffect = true
			}
		}
		if isEffect {
			effects = append(effects, action)
		} else {
			retained = append(retained, action)
		}
	}
	if len(effects) != len(ids) {
		t.Fatal("late-producer control missed actual effects")
	}
	broken := append(retained, effects...)
	if err := p15C01EffectsBeforeConsumers(broken, ids); err == nil || !strings.Contains(err.Error(), "before its binding/consumer") {
		t.Fatalf("late effects did not fail specifically at the consumer-order gate: %v", err)
	}
}

func TestP15C01SourceOrderNativeStructure(t *testing.T) {
	for _, tc := range []struct {
		name, prefix, call string
		order              []string
		native             bool
	}{
		{"function_reverse", p15C01Function, "c01Function(c:text(\"P15_C01_C\"),b:text(\"P15_C01_B\"),a:text(\"P15_C01_A\"))", []string{"P15_C01_C", "P15_C01_B", "P15_C01_A"}, false},
		{"function_mixed", p15C01Function, "c01Function(text(\"P15_C01_A\"),c:text(\"P15_C01_C\"),b:text(\"P15_C01_B\"))", []string{"P15_C01_A", "P15_C01_C", "P15_C01_B"}, false},
		{"function_positional", p15C01Function, "c01Function(text(\"P15_C01_A\"),text(\"P15_C01_B\"),text(\"P15_C01_C\"))", []string{"P15_C01_A", "P15_C01_B", "P15_C01_C"}, false},
		{"function_alias", p15C01Alias, "c01Function(c:text(\"P15_C01_C\"),a:text(\"P15_C01_A\"),b:text(\"P15_C01_B\"))", []string{"P15_C01_C", "P15_C01_A", "P15_C01_B"}, false},
		{"custom_reverse", p15C01Custom, "c01Native(b:text(\"P15_C01_B\"),a:text(\"P15_C01_A\"))", []string{"P15_C01_B", "P15_C01_A"}, true},
		{"custom_primary", p15C01Custom, "c01Native(text(\"P15_C01_B\"),text(\"P15_C01_A\"))", []string{"P15_C01_B", "P15_C01_A"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, tc.prefix+tc.call)
			ids, err := p15C01MarkerOrder(actions, tc.order)
			if err != nil {
				t.Fatal(err)
			}
			fields := p15C01CallFields(t, actions, tc.native)
			expected := map[string]string{"arg_0": "P15_C01_A", "arg_1": "P15_C01_B", "arg_2": "P15_C01_C"}
			if tc.native {
				expected = map[string]string{"A": "P15_C01_A", "B": "P15_C01_B"}
			}
			if err := p15C01References(fields, expected, ids); err != nil {
				t.Fatal(err)
			}
			t.Run("producer_after_binding", func(t *testing.T) {
				p15C01RejectLateProducers(t, actions, ids)
			})
			corrupt := p15C01Clone(t, actions)
			corruptFields := p15C01CallFields(t, corrupt, tc.native)
			key := "arg_0"
			if tc.native {
				key = "A"
			}
			if p15C01ChangeUUID(corruptFields[key], ids["P15_C01_A"], ids["P15_C01_B"]) != 1 {
				t.Fatal("reference fault did not change the real field")
			}
			if p15C01References(corruptFields, expected, ids) == nil {
				t.Fatal("actual wrong producer reference passed")
			}

			indices := []int{}
			for i, a := range actions {
				if text, ok := p15C01Params(a)["WFTextActionText"].(string); ok && strings.HasPrefix(text, "P15_C01_") {
					indices = append(indices, i)
				}
			}
			swapped := p15C01Clone(t, actions)
			swapped[indices[0]], swapped[indices[1]] = swapped[indices[1]], swapped[indices[0]]
			if _, err := p15C01MarkerOrder(swapped, tc.order); err == nil {
				t.Fatal("actual effect order swap passed")
			}
			duplicated := append(p15C01Clone(t, actions), actions[indices[0]])
			if _, err := p15C01MarkerOrder(duplicated, tc.order); err == nil {
				t.Fatal("duplicate actual producer passed")
			}
			missing := append(p15C01Clone(t, actions[:indices[0]]), actions[indices[0]+1:]...)
			if _, err := p15C01MarkerOrder(missing, tc.order); err == nil {
				t.Fatal("missing actual producer passed")
			}
		})
	}
	t.Run("native_action_healthy_control", func(t *testing.T) {
		actions := p15CompiledActions(t, "resizeImage(ShortcutInput,height:number(202),width:number(101))")
		values := []int{}
		ids := map[int]string{}
		var resize map[string]any
		for _, a := range actions {
			p := p15C01Params(a)
			if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.number" {
				v := p15Int(p["WFNumberActionNumber"])
				values = append(values, v)
				ids[v], _ = p["UUID"].(string)
			}
			if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.image.resize" {
				resize = p
			}
		}
		if !reflect.DeepEqual(values, []int{202, 101}) {
			t.Fatalf("native explicit source order=%v", values)
		}
		t.Run("producer_after_binding", func(t *testing.T) {
			p15C01RejectLateProducers(t, actions, map[string]string{"height": ids[202], "width": ids[101]})
		})
		for key, value := range map[string]int{"WFImageResizeHeight": 202, "WFImageResizeWidth": 101} {
			outputs, variables := p15V02NativeReferences(resize[key])
			if len(variables) != 0 || len(outputs) != 1 || outputs[0] != ids[value] {
				t.Fatalf("native argument %s bound wrong output: %v %v", key, outputs, variables)
			}
		}
	})
}

func TestP15C01SourceOrderFlow(t *testing.T) {
	prefix := "function c01Flow(a:Number,b:Number)->Number{return a}\nvar value:Number\n"
	for _, tc := range []struct {
		name, call string
		valid      bool
	}{
		{"named_write_before_read", "c01Flow(b:measure(\"init\"){value=1;value},a:value)", true},
		{"named_read_before_write", "c01Flow(b:value,a:measure(\"init\"){value=1;value})", false},
		{"positional_write_before_read", "c01Flow(measure(\"init\"){value=1;value},value)", true},
		{"positional_read_before_write", "c01Flow(value,measure(\"init\"){value=1;value})", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := prefix + tc.call
			var errors []analysis.Diagnostic
			for _, d := range p15TypeDiagnostics(t, source) {
				if d.Severity == analysis.SeverityError {
					errors = append(errors, d)
				}
			}
			if tc.valid {
				if len(errors) != 0 {
					t.Fatal(errors)
				}
				if raw, err := CompileSourceToPlist("p15-c01-flow.cherri", source); err != nil || len(raw) == 0 {
					t.Fatalf("valid ordered argument compile: %v", err)
				}
			} else {
				offset := len(prefix) + strings.Index(tc.call, "value")
				if len(errors) != 1 || errors[0].Code != analysis.CodeReadBeforeInit || errors[0].Span.Start.ByteOffset != offset || errors[0].Span.End.ByteOffset != offset+len("value") {
					t.Fatalf("source-first read must reject exactly at [%d,%d): %+v", offset, offset+len("value"), errors)
				}
				if raw, err := CompileSourceToPlist("p15-c01-flow-invalid.cherri", source); err == nil || len(raw) != 0 {
					t.Fatal("read-before-init emitted native output")
				}
			}
		})
	}
}

func TestP15C01DefaultEffectsOnce(t *testing.T) {
	prefix := "function c01Defaults(a:Text=text(\"P15_C01_DEFAULT_A\"),b:Text=text(\"P15_C01_DEFAULT_B\"))->Text{return b}\n"
	for _, call := range []string{"c01Defaults()", "c01Defaults(text(\"P15_C01_EXPLICIT_A\"))", "c01Defaults(b:text(\"P15_C01_EXPLICIT_B\"))", "c01Defaults(b:text(\"P15_C01_EXPLICIT_B\"),a:text(\"P15_C01_EXPLICIT_A\"))"} {
		t.Run(call, func(t *testing.T) {
			actions := p15CompiledActions(t, prefix+call)
			counts := map[string]int{}
			for _, a := range actions {
				if v, ok := p15C01Params(a)["WFTextActionText"].(string); ok && strings.HasPrefix(v, "P15_C01_") {
					counts[v]++
				}
			}
			for _, marker := range []string{"P15_C01_DEFAULT_A", "P15_C01_DEFAULT_B"} {
				if counts[marker] != 1 {
					t.Errorf("default %s emitted %d times, want one guarded dispatcher producer", marker, counts[marker])
				}
			}
			fields := p15C01CallFields(t, actions, false)
			for index, marker := range []string{"P15_C01_EXPLICIT_A", "P15_C01_EXPLICIT_B"} {
				want := "0"
				if strings.Contains(call, marker) {
					want = "1"
					if counts[marker] != 1 {
						t.Errorf("explicit producer count=%d", counts[marker])
					}
				}
				v := fields[fmt.Sprintf("has_arg_%d", index)].(map[string]any)["Value"].(map[string]any)["string"]
				if v != want {
					t.Fatalf("supplied/default flag=%v want %s", v, want)
				}
			}
		})
	}
	t.Run("earlier_parameter_scope", func(t *testing.T) {
		source := "function c01Defaults(a:Text=text(\"P15_C01_DEFAULT_A\"),b:Text=text(a))->Text{return b}\nc01Defaults()"
		for _, d := range p15TypeDiagnostics(t, source) {
			if d.Severity == analysis.SeverityError {
				t.Fatal(d)
			}
		}
		if raw, err := CompileSourceToPlist("p15-c01-default-parameter.cherri", source); err != nil || len(raw) == 0 {
			t.Fatalf("callee-scoped earlier default reference failed: %v", err)
		}
	})
}

func TestP15C01ArgumentValueSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name, prefix, call, key string
		custom                  bool
	}{
		{"function", "function c01Pair(a:Text,b:Text)->Text{return a}\n", "c01Pair(a:value,b:measure(\"change\"){value=text(\"P15_C01_NEW\");value})", "arg_0", false},
		{"custom", p15C01Custom, "c01Native(a:value,b:measure(\"change\"){value=text(\"P15_C01_NEW\");value})", "A", true},
		{"function_fstring", "function c01Pair(a:Text,b:Text)->Text{return a}\n", "c01Pair(a:f\"value:{value}\",b:measure(\"change\"){value=text(\"P15_C01_NEW\");value})", "arg_0", false},
		{"custom_fstring", p15C01Custom, "c01Native(a:f\"value:{value}\",b:measure(\"change\"){value=text(\"P15_C01_NEW\");value})", "A", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, tc.prefix+"var value=text(\"P15_C01_INITIAL\")\n"+tc.call)
			fields := p15C01CallFields(t, actions, tc.custom)
			outputs, variables := p15V02NativeReferences(fields[tc.key])
			if len(variables) != 0 || len(outputs) != 1 {
				t.Fatalf("first argument remains a late mutable read: outputs=%v variables=%v", outputs, variables)
			}
			capture, mutation := -1, -1
			for index, action := range actions {
				parameters := p15C01Params(action)
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvariable" && parameters["UUID"] == outputs[0] {
					_, names := p15V02NativeReferences(parameters["WFVariable"])
					if len(names) == 1 && names[0] == "value" {
						capture = index
					}
				}
				if strings.HasSuffix(tc.name, "_fstring") && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" && parameters["UUID"] == outputs[0] {
					_, names := p15V02NativeReferences(parameters["WFTextActionText"])
					if len(names) == 1 && names[0] == "value" {
						capture = index
					}
				}
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" && parameters["WFVariableName"] == "value" {
					mutation = index
				}
			}
			if capture < 0 || mutation < 0 || capture >= mutation {
				t.Fatalf("first argument capture=%d must precede later argument mutation=%d", capture, mutation)
			}
		})
	}
}
