package main

// Full original RP12 programs, with their complete positive graph checks.
// The unchanged RP12 Unit/Native parents run in the same required Go stream;
// their diagnostic and corruption controls are not replaced by this exporter.
import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"howett.net/plist"
)

type p15NativeRP12Case struct {
	id, source, filename, oracle, name, key, call, caller, nativeAction string
	order                                                               []string
	native, typecheck                                                   bool
}

func p15NativeRP12SHA(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }

const p15NativeRP12CatalogueSHA = "3f91519429fc3d4d99724435df1bb0ba3a201da503231fc02911c991fa959ef7"

func p15NativeRP12Cases() []p15NativeRP12Case {
	return []p15NativeRP12Case{
		{id: "written_order/function_reverse", source: p15C01Function + "c01Function(c:text(\"P15_C01_C\"),b:text(\"P15_C01_B\"),a:text(\"P15_C01_A\"))", filename: "p15-short-circuit.cherri", oracle: "written", name: "function_reverse", native: false, order: []string{"P15_C01_C", "P15_C01_B", "P15_C01_A"}},
		{id: "written_order/function_mixed", source: p15C01Function + "c01Function(text(\"P15_C01_A\"),c:text(\"P15_C01_C\"),b:text(\"P15_C01_B\"))", filename: "p15-short-circuit.cherri", oracle: "written", name: "function_mixed", native: false, order: []string{"P15_C01_A", "P15_C01_C", "P15_C01_B"}},
		{id: "written_order/function_positional", source: p15C01Function + "c01Function(text(\"P15_C01_A\"),text(\"P15_C01_B\"),text(\"P15_C01_C\"))", filename: "p15-short-circuit.cherri", oracle: "written", name: "function_positional", native: false, order: []string{"P15_C01_A", "P15_C01_B", "P15_C01_C"}},
		{id: "written_order/function_alias", source: p15C01Alias + "c01Function(c:text(\"P15_C01_C\"),a:text(\"P15_C01_A\"),b:text(\"P15_C01_B\"))", filename: "p15-short-circuit.cherri", oracle: "written", name: "function_alias", native: false, order: []string{"P15_C01_C", "P15_C01_A", "P15_C01_B"}},
		{id: "written_order/custom_reverse", source: p15C01Custom + "c01Native(b:text(\"P15_C01_B\"),a:text(\"P15_C01_A\"))", filename: "p15-short-circuit.cherri", oracle: "written", name: "custom_reverse", native: true, order: []string{"P15_C01_B", "P15_C01_A"}},
		{id: "written_order/custom_primary", source: p15C01Custom + "c01Native(text(\"P15_C01_B\"),text(\"P15_C01_A\"))", filename: "p15-short-circuit.cherri", oracle: "written", name: "custom_primary", native: true, order: []string{"P15_C01_B", "P15_C01_A"}},
		{id: "written_order/native_action_healthy_control", source: "resizeImage(ShortcutInput,height:number(202),width:number(101))", filename: "p15-short-circuit.cherri", oracle: "written-image"},
		{id: "flow/named_write_before_read", source: "function c01Flow(a:Number,b:Number)->Number{return a}\nvar value:Number\n" + "c01Flow(b:measure(\"init\"){value=1;value},a:value)", filename: "p15-c01-flow.cherri", oracle: "typed", typecheck: true},
		{id: "flow/positional_write_before_read", source: "function c01Flow(a:Number,b:Number)->Number{return a}\nvar value:Number\n" + "c01Flow(measure(\"init\"){value=1;value},value)", filename: "p15-c01-flow.cherri", oracle: "typed", typecheck: true},
		{id: "default_effects/call_0", source: "function c01Defaults(a:Text=text(\"P15_C01_DEFAULT_A\"),b:Text=text(\"P15_C01_DEFAULT_B\"))->Text{return b}\n" + "c01Defaults()", filename: "p15-short-circuit.cherri", oracle: "defaults", call: "c01Defaults()"},
		{id: "default_effects/call_1", source: "function c01Defaults(a:Text=text(\"P15_C01_DEFAULT_A\"),b:Text=text(\"P15_C01_DEFAULT_B\"))->Text{return b}\n" + "c01Defaults(text(\"P15_C01_EXPLICIT_A\"))", filename: "p15-short-circuit.cherri", oracle: "defaults", call: "c01Defaults(text(\"P15_C01_EXPLICIT_A\"))"},
		{id: "default_effects/call_2", source: "function c01Defaults(a:Text=text(\"P15_C01_DEFAULT_A\"),b:Text=text(\"P15_C01_DEFAULT_B\"))->Text{return b}\n" + "c01Defaults(b:text(\"P15_C01_EXPLICIT_B\"))", filename: "p15-short-circuit.cherri", oracle: "defaults", call: "c01Defaults(b:text(\"P15_C01_EXPLICIT_B\"))"},
		{id: "default_effects/call_3", source: "function c01Defaults(a:Text=text(\"P15_C01_DEFAULT_A\"),b:Text=text(\"P15_C01_DEFAULT_B\"))->Text{return b}\n" + "c01Defaults(b:text(\"P15_C01_EXPLICIT_B\"),a:text(\"P15_C01_EXPLICIT_A\"))", filename: "p15-short-circuit.cherri", oracle: "defaults", call: "c01Defaults(b:text(\"P15_C01_EXPLICIT_B\"),a:text(\"P15_C01_EXPLICIT_A\"))"},
		{id: "default_effects/earlier_parameter_scope", source: "function c01Defaults(a:Text=text(\"P15_C01_DEFAULT_A\"),b:Text=text(a))->Text{return b}\nc01Defaults()", filename: "p15-c01-default-parameter.cherri", oracle: "typed", typecheck: true},
		{id: "default_guards/calls[0]", source: "function c01Defaults(a:Text=text(\"P15_C01_DEFAULT_A\"),b:Text=text(\"P15_C01_DEFAULT_B\"))->Text{return b}\n" + "c01Defaults()", filename: "p15-short-circuit.cherri", oracle: "guards", call: "c01Defaults()"},
		{id: "default_guards/calls[1]", source: "function c01Defaults(a:Text=text(\"P15_C01_DEFAULT_A\"),b:Text=text(\"P15_C01_DEFAULT_B\"))->Text{return b}\n" + "c01Defaults(text(\"explicit\"))", filename: "p15-short-circuit.cherri", oracle: "guards", call: "c01Defaults(text(\"explicit\"))"},
		{id: "default_guards/calls[2]", source: "function c01Defaults(a:Text=text(\"P15_C01_DEFAULT_A\"),b:Text=text(\"P15_C01_DEFAULT_B\"))->Text{return b}\n" + "c01Defaults(b:text(\"explicit\"))", filename: "p15-short-circuit.cherri", oracle: "guards", call: "c01Defaults(b:text(\"explicit\"))"},
		{id: "default_guards/calls[3]", source: "function c01Defaults(a:Text=text(\"P15_C01_DEFAULT_A\"),b:Text=text(\"P15_C01_DEFAULT_B\"))->Text{return b}\n" + "c01Defaults(b:text(\"second\"),a:text(\"first\"))", filename: "p15-short-circuit.cherri", oracle: "guards", call: "c01Defaults(b:text(\"second\"),a:text(\"first\"))"},
		{id: "default_guards/caller[0]", source: "function c01Defaults(a:Text=text(\"P15_C01_DEFAULT_A\"),b:Text=text(a))->Text{return b}\n" + "" + "c01Defaults()", filename: "p15-short-circuit.cherri", oracle: "caller-scope", caller: ""},
		{id: "default_guards/caller[1]", source: "function c01Defaults(a:Text=text(\"P15_C01_DEFAULT_A\"),b:Text=text(a))->Text{return b}\n" + "var a=text(\"CALLER_A\")\n" + "c01Defaults()", filename: "p15-short-circuit.cherri", oracle: "caller-scope", caller: "var a=text(\"CALLER_A\")\n"},
		{id: "scalar_snapshots/function", source: "function c01Pair(a:Text,b:Text)->Text{return a}\n" + "var value=text(\"P15_C01_INITIAL\")\n" + "c01Pair(a:value,b:measure(\"change\"){value=text(\"P15_C01_NEW\");value})", filename: "p15-short-circuit.cherri", oracle: "scalar", name: "function", key: "arg_0", native: false},
		{id: "scalar_snapshots/custom", source: p15C01Custom + "var value=text(\"P15_C01_INITIAL\")\n" + "c01Native(a:value,b:measure(\"change\"){value=text(\"P15_C01_NEW\");value})", filename: "p15-short-circuit.cherri", oracle: "scalar", name: "custom", key: "A", native: true},
		{id: "scalar_snapshots/function_fstring", source: "function c01Pair(a:Text,b:Text)->Text{return a}\n" + "var value=text(\"P15_C01_INITIAL\")\n" + "c01Pair(a:f\"value:{value}\",b:measure(\"change\"){value=text(\"P15_C01_NEW\");value})", filename: "p15-short-circuit.cherri", oracle: "scalar", name: "function_fstring", key: "arg_0", native: false},
		{id: "scalar_snapshots/custom_fstring", source: p15C01Custom + "var value=text(\"P15_C01_INITIAL\")\n" + "c01Native(a:f\"value:{value}\",b:measure(\"change\"){value=text(\"P15_C01_NEW\");value})", filename: "p15-short-circuit.cherri", oracle: "scalar", name: "custom_fstring", key: "A", native: true},
		{id: "snapshot_boundaries/image", source: "var dimension=number(12)\nresizeImage(ShortcutInput,width:dimension,height:measure(\"change\"){dimension=number(99);dimension})", filename: "p15-short-circuit.cherri", oracle: "snapshot-image"},
		{id: "snapshot_boundaries/bodies[0]", source: "function c01Pair(a:Text,b:Text)->Text{return a}\nvar value=text(\"outer\")\nvar other=text(\"other\")\nc01Pair(a:value,b:measure(\"local\"){" + "var value=text(\"local\");value=text(\"changed\");value" + "})", filename: "p15-short-circuit.cherri", oracle: "shadow"},
		{id: "snapshot_boundaries/bodies[1]", source: "function c01Pair(a:Text,b:Text)->Text{return a}\nvar value=text(\"outer\")\nvar other=text(\"other\")\nc01Pair(a:value,b:measure(\"local\"){" + "other=text(\"changed\");other" + "})", filename: "p15-short-circuit.cherri", oracle: "shadow"},
		{id: "snapshot_boundaries/standalone", source: "text(\"unused\")", filename: "p15-short-circuit.cherri", oracle: "standalone"},
		{id: "nested_custom/original_source", source: "function c01Pair(a:Text,b:Text)->Text{return a}\n" + p15C01Custom + "c01Pair(b:text(\"P15_C01_FIRST\"),a:c01Native(text(\"P15_C01_SECOND\"),text(\"P15_C01_THIRD\")))", filename: "p15-short-circuit.cherri", oracle: "nested-custom"},
		{id: "aggregate_snapshots/list", source: "function c01Aggregate(a:" + "List<Text>" + ",b:Text)->Text{return b}\nvar value=text(\"initial\")\nc01Aggregate(a:" + "[\"prefix\", value]" + ",b:measure(\"change\"){value=text(\"changed\");value})", filename: "p15-short-circuit.cherri", oracle: "aggregate", nativeAction: "is.workflow.actions.list"},
		{id: "aggregate_snapshots/map", source: "function c01Aggregate(a:" + "Map<Text, Text>" + ",b:Text)->Text{return b}\nvar value=text(\"initial\")\nc01Aggregate(a:" + "{\"key\": value}" + ",b:measure(\"change\"){value=text(\"changed\");value})", filename: "p15-short-circuit.cherri", oracle: "aggregate", nativeAction: "is.workflow.actions.dictionary"},
		{id: "accepted_bindings/custom[0]", source: p15C02Custom + "c02Native(\"selected\", a: 11)", filename: "debug-stdlib-test.cherri", oracle: "accepted-custom", typecheck: true},
		{id: "accepted_bindings/custom[1]", source: p15C02Custom + "c02Native(b: \"selected\", a: 11)", filename: "debug-stdlib-test.cherri", oracle: "accepted-custom", typecheck: true},
		{id: "accepted_bindings/custom[2]", source: p15C02Custom + "c02Native(\"selected\", 11)", filename: "debug-stdlib-test.cherri", oracle: "accepted-custom", typecheck: true},
		{id: "accepted_bindings/general[0]", source: "number(0)", filename: "debug-stdlib-test.cherri", oracle: "accepted-general", typecheck: true},
		{id: "accepted_bindings/general[1]", source: "number(number: 0)", filename: "debug-stdlib-test.cherri", oracle: "accepted-general", typecheck: true},
		{id: "accepted_bindings/general[2]", source: "text(\"\")", filename: "debug-stdlib-test.cherri", oracle: "accepted-general", typecheck: true},
		{id: "accepted_bindings/general[3]", source: "text(text: \"\")", filename: "debug-stdlib-test.cherri", oracle: "accepted-general", typecheck: true},
		{id: "accepted_bindings/general[4]", source: "resizeImage(ShortcutInput, width: 800)", filename: "debug-stdlib-test.cherri", oracle: "accepted-general", typecheck: true},
		{id: "accepted_bindings/general[5]", source: "resizeImage(ShortcutInput, height: 600, width: 800)", filename: "debug-stdlib-test.cherri", oracle: "accepted-general", typecheck: true},
		{id: "accepted_bindings/general[6]", source: "resizeImage(ShortcutInput, 800, 600)", filename: "debug-stdlib-test.cherri", oracle: "accepted-general", typecheck: true},
		{id: "accepted_bindings/general[7]", source: p15C02Function + "c02Choose(0)", filename: "debug-stdlib-test.cherri", oracle: "accepted-general", typecheck: true},
		{id: "accepted_bindings/general[8]", source: p15C02Function + "c02Choose(b: 3, a: 1)", filename: "debug-stdlib-test.cherri", oracle: "accepted-general", typecheck: true},
		{id: "accepted_bindings/general[9]", source: p15C02Function + "c02Choose(1, 2)", filename: "debug-stdlib-test.cherri", oracle: "accepted-general", typecheck: true},
		{id: "accepted_bindings/general[10]", source: p15C02Alias + "c02Alias(value: 1)", filename: "debug-stdlib-test.cherri", oracle: "accepted-general", typecheck: true},
		{id: "accepted_bindings/general[11]", source: p15C02Alias + "c02Alias(_value: 1)", filename: "debug-stdlib-test.cherri", oracle: "accepted-general", typecheck: true},
		{id: "accepted_bindings/general[12]", source: "function c02Names(x: Number, _x: Number) -> Number { return x }\nc02Names(1, _x: 2)", filename: "debug-stdlib-test.cherri", oracle: "accepted-general", typecheck: true},
		{id: "accepted_bindings/general[13]", source: "function c02Names(x: Number, _x: Number) -> Number { return x }\nc02Names(_x: 2, x: 1)", filename: "debug-stdlib-test.cherri", oracle: "accepted-general", typecheck: true},
		{id: "accepted_bindings/general[14]", source: "function c02Optional(value?: Number) -> Void { comment(\"ok\") }\nc02Optional()", filename: "debug-stdlib-test.cherri", oracle: "accepted-general", typecheck: true},
		{id: "accepted_bindings/general[15]", source: "function c02Nullable(value: Number?) -> Void { comment(\"ok\") }\nc02Nullable(none)", filename: "debug-stdlib-test.cherri", oracle: "accepted-general", typecheck: true},
	}
}

func p15NativeRP12Graph(t *testing.T, tc p15NativeRP12Case, actions []map[string]any, typedActions []ShortcutAction) {
	t.Helper()
	switch tc.oracle {
	case "written":
		{
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
			if err := p15C01EffectsBeforeConsumers(actions, ids); err != nil {
				t.Fatal(err)
			}
		}
	case "written-image":
		{
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
			if err := p15C01EffectsBeforeConsumers(actions, map[string]string{"height": ids[202], "width": ids[101]}); err != nil {
				t.Fatal(err)
			}
			for key, value := range map[string]int{"WFImageResizeHeight": 202, "WFImageResizeWidth": 101} {
				outputs, variables := p15V02NativeReferences(resize[key])
				if len(variables) != 0 || len(outputs) != 1 || outputs[0] != ids[value] {
					t.Fatalf("native argument %s bound wrong output: %v %v", key, outputs, variables)
				}
			}
		}
	case "defaults":
		{
			call := tc.call
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
		}
	case "typed":
		{
			// Original successful type diagnostics and nonempty compile are checked before this switch.
		}
	case "guards":
		{
			if _, err := p15C01DefaultGuards(actions); err != nil {
				t.Fatal(tc.call, err)
			}
		}
	case "caller-scope":
		{
			caller := tc.caller
			hasA := ""
			groupA := ""
			resultA := ""
			matched := 0
			for _, a := range actions {
				p := p15C01Params(a)
				if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvalueforkey" && p["WFDictionaryKey"] == "has_arg_0" {
					hasA, _ = p["UUID"].(string)
				}
				if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" {
					outputs, _ := p15V02NativeReferences(p["WFInput"])
					if p15Int(p["WFControlFlowMode"]) == 0 && len(outputs) == 1 && outputs[0] == hasA {
						groupA, _ = p["GroupingIdentifier"].(string)
					}
					if p15Int(p["WFControlFlowMode"]) == 2 && p["GroupingIdentifier"] == groupA {
						resultA, _ = p["UUID"].(string)
					}
				}
				if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" {
					outputs, variables := p15V02NativeReferences(p["WFTextActionText"])
					if len(outputs) == 1 && outputs[0] == resultA && resultA != "" && len(variables) == 0 {
						matched++
					}
				}
			}
			if matched != 1 {
				t.Fatalf("earlier parameter default reference count=%d; caller=%q", matched, caller)
			}
		}
	case "scalar":
		{
			fields := p15C01CallFields(t, actions, tc.native)
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
		}
	case "snapshot-image":
		{
			var resize map[string]any
			mutation := -1
			positions := map[string]int{}
			for i, a := range actions {
				p := p15C01Params(a)
				if uuid, ok := p["UUID"].(string); ok {
					positions[uuid] = i
				}
				if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.image.resize" {
					resize = p
				}
				if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" && p["WFVariableName"] == "dimension" {
					mutation = i
				}
			}
			outputs, variables := p15V02NativeReferences(resize["WFImageResizeWidth"])
			if len(outputs) != 1 || len(variables) != 0 || positions[outputs[0]] >= mutation {
				t.Fatalf("canonical argument snapshot lost: outputs=%v variables=%v mutation=%d", outputs, variables, mutation)
			}
		}
	case "shadow":
		{
			field := p15C01CallFields(t, actions, false)["arg_0"]
			outputs, variables := p15V02NativeReferences(field)
			if len(outputs) != 0 || !reflect.DeepEqual(variables, []string{"value"}) {
				t.Fatalf("unrelated/local shadow write caused unnecessary outer capture: %v %v", outputs, variables)
			}
		}
	case "standalone":
		{
			if len(actions) != 2 || actions[0]["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" || p15C01Params(actions[0])["UUID"] != nil {
				t.Fatal("unused actions output identity changed")
			}
			if err := p15M02MainVoidBoundaryError(actions); err != nil {
				t.Fatal(err)
			}
		}
	case "nested-custom":
		{
			if _, err := p15C01MarkerOrder(actions, []string{"P15_C01_FIRST", "P15_C01_SECOND", "P15_C01_THIRD"}); err != nil {
				t.Fatal(err)
			}
			fields := p15C01CallFields(t, actions, false)
			outputs, variables := p15V02NativeReferences(fields["arg_0"])
			if len(outputs) != 1 || len(variables) != 0 {
				t.Fatalf("nested custom result reference absent: %v %v", outputs, variables)
			}
			found := 0
			for _, action := range actions {
				if action["WFWorkflowActionIdentifier"] == "com.example.c01.order" {
					found++
					if p15C01Params(action)["UUID"] != outputs[0] {
						t.Fatalf("consumed custom result has dangling UUID %s; producer UUID=%v", outputs[0], p15C01Params(action)["UUID"])
					}
				}
			}
			if found != 1 {
				t.Fatalf("custom argument executed %d times", found)
			}
		}
	case "aggregate":
		{
			outputs, variables := p15V02NativeReferences(p15C01CallFields(t, actions, false)["arg_0"])
			if len(outputs) != 1 || len(variables) != 0 {
				t.Fatalf("aggregate argument is not a captured producer: %v %v", outputs, variables)
			}
			producer, mutation := -1, -1
			for i, action := range actions {
				parameters := p15C01Params(action)
				if parameters["UUID"] == outputs[0] {
					if action["WFWorkflowActionIdentifier"] != tc.nativeAction {
						t.Fatalf("aggregate was coerced to a different native type: %v", action["WFWorkflowActionIdentifier"])
					}
					_, names := p15V02NativeReferences(parameters)
					if len(names) != 1 || names[0] != "value" {
						t.Fatalf("aggregate source value lost: %v", names)
					}
					producer = i
				}
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" && parameters["WFVariableName"] == "value" {
					mutation = i
				}
			}
			if producer < 0 || mutation < 0 || producer >= mutation {
				t.Fatalf("aggregate producer=%d mutation=%d", producer, mutation)
			}
		}
	case "accepted-custom":
		{
			if err := p15C02CustomWire(typedActions); err != nil {
				t.Fatal(err)
			}
		}
	case "accepted-general":
		{
			actions := typedActions
			src := tc.source
			if len(actions) == 0 {
				t.Fatal("accepted call emitted no native action")
			}
			if strings.Contains(src, "function ") {
				found := false
				for _, action := range actions {
					if action.WFWorkflowActionIdentifier == "is.workflow.actions.runworkflow" {
						if requireOutputUUID(action.WFWorkflowActionParameters["WFInput"]) == "" {
							t.Fatal("function call lost envelope reference")
						}
						found = true
					}
				}
				if !found {
					t.Fatal("accepted user-function call was omitted")
				}
			}
		}
	default:
		t.Fatal("unknown original RP12 oracle", tc.oracle)
	}
}

func TestP15NativeRP12OriginalFixtureExport(t *testing.T) {
	t.Cleanup(schema.ResetDefaultRegistry)
	var catalogue struct {
		OriginalMapSHA string `json:"originalMapSha256"`
		OriginalFiles  []struct {
			Path  string `json:"repositoryPath"`
			Bytes int    `json:"bytes"`
			SHA   string `json:"sha256"`
		} `json:"originalFiles"`
		Cases []map[string]any `json:"cases"`
	}
	catalogueRaw, err := os.ReadFile("tests/language-v2/native-rp12-original-cases.json")
	if err != nil || p15NativeRP12SHA(catalogueRaw) != p15NativeRP12CatalogueSHA {
		t.Fatal("original RP12 catalogue changed", err)
	}
	if err := json.Unmarshal(catalogueRaw, &catalogue); err != nil {
		t.Fatal(err)
	}
	if catalogue.OriginalMapSHA != "fc33704fd0eafc5e06c59f25b208f0078bfce01dbbc4149a386ed4db17c6cf80" || len(catalogue.OriginalFiles) != 9 {
		t.Fatal("original source authority")
	}
	for _, pin := range catalogue.OriginalFiles {
		raw, err := os.ReadFile(pin.Path)
		if err != nil || len(raw) != pin.Bytes || p15NativeRP12SHA(raw) != pin.SHA {
			t.Fatal("original full source changed", pin.Path, err)
		}
	}
	cases := p15NativeRP12Cases()
	if len(cases) != 50 || len(catalogue.Cases) != 50 {
		t.Fatal("original positive source inventory")
	}
	seen := map[string]bool{}
	for i, tc := range cases {
		spec := catalogue.Cases[i]
		if seen[tc.id] || spec["mapId"] != tc.id || spec["source"] != tc.source || spec["compileFilename"] != tc.filename || spec["oracleKind"] != tc.oracle {
			t.Fatal("original case/source binding", tc.id)
		}
		seen[tc.id] = true
		t.Run(tc.id, func(t *testing.T) {
			if tc.typecheck {
				for _, d := range p15TypeDiagnostics(t, tc.source) {
					if d.Severity == analysis.SeverityError {
						t.Fatal(d)
					}
				}
			}
			raw, err := CompileSourceToPlist(tc.filename, tc.source)
			if err != nil || len(raw) == 0 {
				t.Fatal("original positive compile", err)
			}
			var workflow map[string]any
			if _, err := plist.Unmarshal(raw, &workflow); err != nil {
				t.Fatal(err)
			}
			wireActions, ok := workflow["WFWorkflowActions"].([]any)
			if !ok || len(wireActions) == 0 {
				t.Fatal("complete original workflow has no actions")
			}
			actions := make([]map[string]any, len(wireActions))
			typedActions := make([]ShortcutAction, len(wireActions))
			for j, value := range wireActions {
				action, ok := value.(map[string]any)
				if !ok {
					t.Fatal("original action is not a dictionary", j)
				}
				identifier, ok := action["WFWorkflowActionIdentifier"].(string)
				if !ok || identifier == "" {
					t.Fatal("original action identifier", j)
				}
				var parameters map[string]any
				if value, present := action["WFWorkflowActionParameters"]; present {
					parameters, ok = value.(map[string]any)
					if !ok {
						t.Fatal("present original action parameters are malformed", j)
					}
				}
				actions[j] = action
				typedActions[j] = ShortcutAction{WFWorkflowActionIdentifier: identifier, WFWorkflowActionParameters: parameters}
			}
			p15NativeRP12Graph(t, tc, actions, typedActions)
			if t.Failed() {
				return
			}
			name := ""
			nameValue, namePresent := workflow["WFWorkflowName"]
			if namePresent {
				var ok bool
				name, ok = nameValue.(string)
				if !ok {
					t.Fatal("original workflow name is not text")
				}
			}
			originalCase := make(map[string]any, len(spec)-2)
			for key, value := range spec {
				if key != "source" && key != "sourceExpression" {
					originalCase[key] = value
				}
			}
			fixture := map[string]any{
				"schema": "cherri-native-rp12-original-fixture/1", "requirement": "RP12", "caseId": tc.id,
				"originalMapSha256": catalogue.OriginalMapSHA, "originalCase": originalCase,
				"sourceBase64": base64.StdEncoding.EncodeToString([]byte(tc.source)), "sourceSha256": p15NativeRP12SHA([]byte(tc.source)), "sourceBytes": len([]byte(tc.source)),
				"unsignedBase64": base64.StdEncoding.EncodeToString(raw), "unsignedSha256": p15NativeRP12SHA(raw), "unsignedBytes": len(raw),
				"actionCount": len(actions), "workflowName": name, "workflowNamePresent": namePresent,
				"nativeExecuted": false, "formalAcceptanceGranted": false,
			}
			encoded, err := json.Marshal(fixture)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("P15_NATIVE_RP12_ORIGINAL_FIXTURE sha256=%s bytes=%d base64=%s", p15NativeRP12SHA(encoded), len(encoded), base64.StdEncoding.EncodeToString(encoded))
		})
	}
}
