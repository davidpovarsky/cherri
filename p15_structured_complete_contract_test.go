package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/types"
	"howett.net/plist"
)

// These joined producers add complete authored semantic/native-structure
// scenarios. Their exact graph expectations do not execute Apple's runtime.
const p15CompleteT05Source = "let values=[none,false,0,\"\",[],{},null]\nlet alias=values\nreturn count(alias)"
const p15CompleteL01Source = "let values:List<List<Optional<Bool>>>=[[false,none],[],[true]]\nlet alias=values\nreturn count(alias)"

func p15CompleteCompile(t *testing.T, source string, profile BuildProfile) []map[string]any {
	t.Helper()
	raw, err := CompileSourceToPlistWithOptions("p15-complete-structured.cherri", source, CompileOptions{BuildProfile: profile})
	if err != nil {
		t.Fatal(err)
	}
	if directory := os.Getenv("CHERRI_STRUCTURED_VALUE_ARTIFACTS"); directory != "" {
		caseDir, e := os.MkdirTemp(directory, "native-")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(caseDir, "source.cherri"), []byte(source), 0600); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(caseDir, "actual.shortcut"), raw, 0600); e != nil {
			t.Fatal(e)
		}
		digest := sha256.Sum256(raw)
		t.Logf("STRUCTURED_NATIVE_ARTIFACT profile=%s sha256=%s path=%s", profile, hex.EncodeToString(digest[:]), filepath.Join(caseDir, "actual.shortcut"))
	}
	var document map[string]any
	if _, err = plist.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	rows, ok := document["WFWorkflowActions"].([]any)
	if !ok {
		t.Fatal("actual plist has no action array")
	}
	actions := make([]map[string]any, len(rows))
	for i, row := range rows {
		action, ok := row.(map[string]any)
		if !ok {
			t.Fatalf("action %d is not a Dictionary", i)
		}
		actions[i] = action
	}
	return actions
}

func p15CompleteCellList(g *p15FunctionGraph, cells ...string) string {
	rows := []any{}
	for _, cell := range cells {
		rows = append(rows, map[string]any{"WFItemType": 0, "WFValue": p15FunctionEdge(cell)})
	}
	return g.value("list", map[string]any{"WFItems": rows})
}

func p15CompleteT05Graph(actions []map[string]any) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	absent := p15TaggedAbsentCellGraph(g)
	falsity := p15TaggedCellGraph(g, "bool", p15OptionalJoinValue("false"))
	zero := p15TaggedCellGraph(g, "number", p15OptionalJoinValue("zero"))
	text := p15TaggedCellGraph(g, "text", p15OptionalJoinValue("empty_text"))
	list := p15TaggedCellGraph(g, "list", p15OptionalJoinValue("empty_list"))
	emptyMap := g.value("dictionary", map[string]any{"WFItems": p15OptionalJoinValue("empty_map").value})
	dictionary := p15TaggedCellGraph(g, "map", p15FunctionArgument{true, 0, p15FunctionEdge(emptyMap)})
	null := p15TaggedNullCell(g)
	values := p15CompleteCellList(g, absent, falsity, zero, text, list, dictionary, null)
	count := g.value("count", map[string]any{"Input": p15FunctionEdge(values)})
	g.output(count)
	return g.compare()
}

func p15CompleteL01Graph(actions []map[string]any) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	falsity := p15TaggedCellGraph(g, "bool", p15OptionalJoinValue("false"))
	absent := p15TaggedAbsentCellGraph(g)
	first := p15TaggedCellGraph(g, "list", p15F04ArrayValue(
		p15FunctionArgument{true, 0, p15FunctionEdge(falsity)}, p15FunctionArgument{true, 0, p15FunctionEdge(absent)}))
	second := p15TaggedCellGraph(g, "list", p15F04ArrayValue())
	truth := p15TaggedCellGraph(g, "bool", p15FunctionArgument{true, 4, map[string]any{"WFSerializationType": "WFNumberSubstitutableState", "Value": true}})
	third := p15TaggedCellGraph(g, "list", p15F04ArrayValue(p15FunctionArgument{true, 0, p15FunctionEdge(truth)}))
	list := p15CompleteCellList(g, first, second, third)
	count := g.value("count", map[string]any{"Input": p15FunctionEdge(list)})
	g.output(count)
	return g.compare()
}

func TestP15T05CompleteOptionalUnit(t *testing.T) {
	for _, tc := range []struct {
		name, expression string
		want             types.Type
		absent           bool
	}{
		{"none", "none", nil, true},
		{"false", "false", types.Bool, false},
		{"zero", "0", types.Number, false},
		{"empty_text", "\"\"", types.Text, false},
		{"empty_list", "[]", nil, false},
		{"empty_map", "{}", nil, false},
		{"json_null", "null", types.JsonNull, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, a := p15F03Analysis(t, "let value="+tc.expression+"\nreturn 1")
			if errors := p15F03Errors(a); len(errors) != 0 {
				t.Fatal(errors)
			}
			actual := a.ExprTypes()[p15BindingValue(t, p, "value")]
			if actual == nil || tc.want != nil && !actual.Equals(tc.want) ||
				types.MayBeAbsent(actual) != tc.absent || types.DefinitelyPresent(actual) == tc.absent {
				t.Fatalf("independent presence/type lost for %s: %v", tc.name, actual)
			}
			if tc.name == "json_null" && (actual.AssignableTo(types.AnyContent) || !actual.AssignableTo(types.NewOptional(types.JsonValue))) {
				t.Fatal("present null became ordinary native content or lost Optional<JsonValue> membership")
			}
		})
	}
	for _, source := range []string{
		"let values:List<Optional<Bool>>=[none,false,true]\nreturn count(values)",
		"let values:Map<Text,Optional<Bool>>={\"absent\":none,\"false\":false}\nreturn 1",
		p15CompleteT05Source,
	} {
		if d := p15TypeDiagnostics(t, source); len(d) != 0 {
			t.Fatal(d)
		}
	}
}

func TestP15L01CompleteHierarchyUnit(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		want         types.Type
	}{
		{"nested_number", "let values=[[1,2],[]]\nreturn count(values)", types.NewList(types.NewUnion(types.NewList(types.Number), types.NewList(types.Unknown)))},
		{"dictionary_arrays", "let values={\"rows\":[[false],[]]}\nreturn 1", types.NewMap(types.Text, types.NewList(types.NewUnion(types.NewList(types.Bool), types.NewList(types.Unknown))))},
		{"nested_optional", p15CompleteL01Source, types.NewList(types.NewUnion(types.NewList(types.NewUnion(types.Bool, types.OptionalType{Inner: types.Unknown})), types.NewList(types.Unknown), types.NewList(types.Bool)))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, a := p15F03Analysis(t, tc.source)
			if errors := p15F03Errors(a); len(errors) != 0 {
				t.Fatal(errors)
			}
			value := a.ExprTypes()[p15BindingValue(t, p, "values")]
			if value == nil || !value.Equals(tc.want) {
				t.Fatalf("hierarchy type: %v want %v", value, tc.want)
			}
			if !types.DefinitelyPresent(value) {
				t.Fatal("present container inherited child absence")
			}
			if tc.name == "nested_optional" {
				// The initializer keeps its exact inferred alternatives; the
				// declared binding/alias carries the contextual Optional layout.
				alias := a.ExprTypes()[p15BindingValue(t, p, "alias")]
				declared := types.NewList(types.NewList(types.NewOptional(types.Bool)))
				if alias == nil || !alias.Equals(declared) {
					t.Fatalf("declared nested alias layout lost: %v", alias)
				}
			}
			if err := p15VBTypeError(p, a, map[string]types.Type{"values": types.NewList(types.Bool)}); err == nil {
				t.Fatal("flattened semantic expectation accepted nested source")
			}
		})
	}
}

func p15CompleteGraphControls(t *testing.T, source string, oracle func([]map[string]any) error) {
	t.Helper()
	for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
		t.Run(string(profile), func(t *testing.T) {
			actions := p15CompleteCompile(t, source, profile)
			if err := oracle(actions); err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"drop_cell", "reorder_cells", "duplicate_identity", "wrong_presence", "wrong_kind", "lost_payload", "invented_plist_null", "wrong_consumer", "extra_action"} {
				t.Run(fault, func(t *testing.T) {
					broken := p15TaggedClone(t, actions)
					switch fault {
					case "drop_cell":
						broken = append(broken[:1], broken[2:]...)
					case "reorder_cells":
						broken[0], broken[1] = broken[1], broken[0]
					case "duplicate_identity":
						p15Param(broken[1])["UUID"] = p15Param(broken[0])["UUID"]
					case "wrong_presence", "wrong_kind", "lost_payload":
						// The first present Bool cell is at 1 for T05 and 0 for L01.
						at := 0
						if source == p15CompleteT05Source {
							at = 1
						}
						fields, err := p15ContainerFields(p15Param(broken[at])["WFItems"])
						if err != nil {
							t.Fatal(err)
						}
						key, value := "has_value", any(p15FunctionText("0"))
						if fault == "wrong_kind" {
							key, value = "kind", p15FunctionText("null")
						}
						if fault == "lost_payload" {
							key, value = "value", nil
						}
						fields[key]["WFValue"] = value
					case "invented_plist_null":
						p15Param(broken[0])["P15InventedNull"] = nil
					case "wrong_consumer":
						p15Param(broken[len(broken)-2])["Input"] = p15FunctionEdge("UNRELATED")
					case "extra_action":
						broken = append(broken, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
					}
					if err := oracle(broken); err == nil {
						t.Fatal("accepted " + fault)
					}
				})
			}
		})
	}
}
func TestP15T05CompleteOptionalNative(t *testing.T) {
	p15CompleteGraphControls(t, p15CompleteT05Source, p15CompleteT05Graph)
}
func TestP15L01CompleteHierarchyNative(t *testing.T) {
	p15CompleteGraphControls(t, p15CompleteL01Source, p15CompleteL01Graph)
}

func p15CompleteF04Source(count int) string {
	return fmt.Sprintf("let values=repeat %d {yield [none,false,null,[]]}\nlet alias=values\nreturn count(alias)", count)
}
func p15CompleteF04Graph(actions []map[string]any, count int) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	accumulator := p15F04ClearedCollection(g, "loopResults")
	loop := p15DiscoveryLoop(g, "repeat.count", "WFRepeatCount", count)
	absent := p15TaggedAbsentCellGraph(g)
	falsity := p15TaggedCellGraph(g, "bool", p15OptionalJoinValue("false"))
	null := p15TaggedNullCell(g)
	empty := p15TaggedCellGraph(g, "list", p15OptionalJoinValue("empty_list"))
	payload := p15F04ArrayValue(
		p15FunctionArgument{true, 0, p15FunctionEdge(absent)},
		p15FunctionArgument{true, 0, p15FunctionEdge(falsity)},
		p15FunctionArgument{true, 0, p15FunctionEdge(null)},
		p15FunctionArgument{true, 0, p15FunctionEdge(empty)})
	cell := p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_loop_result_cell_v1", true, payload)
	g.emit("appendvariable", map[string]any{"WFVariableName": accumulator, "WFInput": p15FunctionEdge(cell)})
	p15DiscoveryLoopEnd(g, "repeat.count", loop, false)
	values := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(accumulator)})
	size := g.value("count", map[string]any{"Input": p15FunctionEdge(values)})
	g.output(size)
	return g.compare()
}
func TestP15F04CompleteIterationUnit(t *testing.T) {
	for _, count := range []int{0, 1, 3} {
		t.Run(fmt.Sprintf("repeat_%d", count), func(t *testing.T) {
			p, a := p15F03Analysis(t, p15CompleteF04Source(count))
			if errors := p15F03Errors(a); len(errors) != 0 {
				t.Fatal(errors)
			}
			actual := a.ExprTypes()[p15BindingValue(t, p, "values")]
			outer, ok := actual.(types.ListType)
			if !ok || outer.Element.Kind() != types.KindList || !types.DefinitelyPresent(actual) {
				t.Fatalf("loop lost one list-valued semantic element: %v", actual)
			}
			alias := a.ExprTypes()[p15BindingValue(t, p, "alias")]
			if alias == nil || !alias.Equals(actual) {
				t.Fatal("immutable alias changed exact nested element type")
			}
		})
	}
}
func TestP15F04CompleteIterationNative(t *testing.T) {
	for _, count := range []int{0, 1, 3} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run(fmt.Sprintf("repeat_%d/%s", count, profile), func(t *testing.T) {
				actions := p15CompleteCompile(t, p15CompleteF04Source(count), profile)
				if err := p15CompleteF04Graph(actions, count); err != nil {
					t.Fatal(err)
				}
				for _, fault := range []string{"not_cleared", "append_payload", "append_twice", "wrong_iterations", "wrong_loop", "count_payload"} {
					t.Run(fault, func(t *testing.T) {
						broken := p15TaggedClone(t, actions)
						appendIndex := -1
						for i, a := range broken {
							if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.appendvariable" {
								appendIndex = i
							}
						}
						if appendIndex < 0 {
							t.Fatal("actual loop has no append")
						}
						switch fault {
						case "not_cleared":
							p15Param(broken[1])["WFInput"] = p15FunctionEdge("PREVIOUS_OUTPUT")
						case "append_payload":
							p15Param(broken[appendIndex])["WFInput"] = p15FunctionEdge(p15Param(broken[3])["UUID"].(string))
						case "append_twice":
							broken = append(broken[:appendIndex+1], append([]map[string]any{broken[appendIndex]}, broken[appendIndex+1:]...)...)
						case "wrong_iterations":
							p15Param(broken[2])["WFRepeatCount"] = count + 1
						case "wrong_loop":
							p15Param(broken[appendIndex+1])["GroupingIdentifier"] = "FOREIGN"
						case "count_payload":
							p15Param(broken[len(broken)-2])["Input"] = p15FunctionEdge(p15Param(broken[3])["UUID"].(string))
						}
						if err := p15CompleteF04Graph(broken, count); err == nil {
							t.Fatal("accepted " + fault)
						}
					})
				}
			})
		}
	}
}

// Keep an explicit boundary witness: neither serialization nor an internal
// cell grants permission to send a private representation to an Apple action.
func TestP15StructuredCompleteNativeEscapeBoundary(t *testing.T) {
	for _, source := range []string{
		strings.Replace(p15CompleteT05Source, "return count(alias)", "show(alias)", 1),
		strings.Replace(p15CompleteL01Source, "return count(alias)", "return alias", 1),
		strings.Replace(p15CompleteF04Source(3), "return count(alias)", "show(alias)", 1),
	} {
		raw, err := CompileSourceToPlist("p15-complete-native-boundary.cherri", source)
		if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), "E_TARGET_VALUE_SHAPE") {
			t.Fatalf("private carrier escaped: bytes=%d error=%v", len(raw), err)
		}
	}
	if reflect.DeepEqual(types.JsonNull, types.NewOptional(types.Bool)) {
		t.Fatal("null and Optional were conflated")
	}
}
