package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

const p15RP14Source = `var numberValue: Number = 0
let savedNumber = numberValue
numberValue = 7
var boolValue: Bool = false
let savedBool = boolValue
boolValue = true
var textValue: Text = ""
let savedText = textValue
textValue = "changed"
var listValue: List<AnyContent> = [0, false, ""]
let savedList = listValue
listValue = ["changed"]
var mapValue: Map<Text, AnyContent> = {"number": 0, "bool": false, "text": "", "list": [], "map": {}}
let savedMap = mapValue
mapValue = {"changed": true}
setClipboard(text("rp14_clip_before"))
let savedClipboard = Clipboard
setClipboard(text("rp14_clip_after"))
native.action("com.example.p15.typed-capture", parameters: {"number": savedNumber, "bool": savedBool, "text": savedText, "list": savedList, "map": savedMap, "clipboard": savedClipboard})
`

var p15RP14Fields = []string{"number", "bool", "text", "list", "map", "clipboard"}
var p15RP14Saved = map[string]string{"number": "savedNumber", "bool": "savedBool", "text": "savedText", "list": "savedList", "map": "savedMap", "clipboard": "savedClipboard"}

func TestP15RP14TypedCaptureUnit(t *testing.T) {
	program, errors := p15ParseProgram(p15RP14Source)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	analyzer := analysis.NewAnalyzer(schema.DefaultRegistry())
	analyzer.Analyze(program)
	for _, d := range analyzer.Diagnostics() {
		if d.Severity == analysis.SeverityError {
			t.Fatal(d)
		}
	}
	expected := map[string]string{"savedNumber": "Number", "savedBool": "Bool", "savedText": "Text", "savedList": "List<AnyContent>", "savedMap": "Map<Text, AnyContent>", "savedClipboard": "AnyContent"}
	observed := map[string]string{}
	for _, statement := range program.Statements {
		if binding, ok := statement.(*syntax.BindingStmt); ok {
			if _, wanted := expected[binding.Name]; wanted {
				inferred := analyzer.ExprTypes()[binding.Value]
				if inferred == nil {
					t.Fatal("saved binding has no inferred type", binding.Name)
				}
				observed[binding.Name] = inferred.String()
			}
		}
	}
	for _, field := range p15RP14Fields {
		name := p15RP14Saved[field]
		t.Run(name, func(t *testing.T) {
			if observed[name] != expected[name] {
				t.Fatalf("saved type %s = %q, want %q", name, observed[name], expected[name])
			}
		})
	}
	for _, tc := range []struct{ name, before, after string }{
		{"number_to_text", "var numberValue: Number = 0", `var numberValue: Number = "0"`},
		{"bool_to_number", "var boolValue: Bool = false", "var boolValue: Bool = 0"},
		{"list_to_map", `var listValue: List<AnyContent> = [0, false, ""]`, `var listValue: List<AnyContent> = {"value":0}`},
		{"map_to_list", `var mapValue: Map<Text, AnyContent> = {"number": 0, "bool": false, "text": "", "list": [], "map": {}}`, `var mapValue: Map<Text, AnyContent> = [0]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := strings.Replace(p15RP14Source, tc.before, tc.after, 1)
			if source == p15RP14Source {
				t.Fatal("negative source did not change")
			}
			found := false
			for _, d := range p15TypeDiagnostics(t, source) {
				if d.Severity == analysis.SeverityError {
					found = true
				}
			}
			raw, err := CompileSourceToPlist("rp14-invalid.cherri", source)
			if !found || err == nil || len(raw) != 0 {
				t.Fatal("wrong typed capture did not fail before emission")
			}
		})
	}
}

func p15RP14Numeric(value any, want string) bool {
	if value == nil {
		return false
	}
	switch reflect.TypeOf(value).Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return fmt.Sprint(value) == want
	}
	return false
}
func p15RP14Direct(value any, uuid, name string) bool {
	return reflect.DeepEqual(value, map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "ActionOutput", "OutputUUID": uuid, "OutputName": name}})
}
func p15RP14Producer(actions []map[string]any, positions map[string]int, before int, value any) (int, error) {
	refs, variables := p15V02NativeReferences(value)
	if len(refs) != 1 || len(variables) != 0 {
		return -1, fmt.Errorf("assignment has no unique typed producer")
	}
	index, exists := positions[refs[0]]
	if !exists || index >= before {
		return -1, fmt.Errorf("assignment producer is absent or late")
	}
	name, _ := p15C01Params(actions[index])["CustomOutputName"].(string)
	if name == "" {
		name = "Text"
	}
	if !p15RP14Direct(value, refs[0], name) {
		return -1, fmt.Errorf("assignment producer was coerced or wrapped")
	}
	return index, nil
}
func p15RP14ValueError(actions []map[string]any, positions map[string]int, index int, field string, initial bool) error {
	action := actions[index]
	p := p15C01Params(action)
	kind := action["WFWorkflowActionIdentifier"]
	switch field {
	case "number", "bool":
		want := "0"
		if !initial {
			want = "7"
			if field == "bool" {
				want = "1"
			}
		}
		// The established standalone Bool carrier is Number 0/1; the unit producer
		// independently requires semantic Bool. Dictionary/list Bool below stays Bool.
		if kind != "is.workflow.actions.number" || !p15RP14Numeric(p["WFNumberActionNumber"], want) {
			return fmt.Errorf("%s lost numeric value carrier", field)
		}
	case "text":
		want := ""
		if !initial {
			want = "changed"
		}
		if kind != "is.workflow.actions.gettext" || p["WFTextActionText"] != want {
			return fmt.Errorf("Text value changed")
		}
	case "list":
		rows, ok := p["WFItems"].([]any)
		if kind != "is.workflow.actions.list" || !ok {
			return fmt.Errorf("List converted or absent")
		}
		if !initial {
			if !reflect.DeepEqual(rows, []any{map[string]any{"WFItemType": uint64(0), "WFValue": "changed"}}) {
				return fmt.Errorf("later List changed")
			}
			break
		}
		if len(rows) != 3 {
			return fmt.Errorf("saved List cardinality changed")
		}
		first, ok := rows[0].(map[string]any)
		if !ok || len(first) != 2 || first["WFItemType"] != uint64(0) {
			return fmt.Errorf("Number List row lost native content shape")
		}
		producer, err := p15RP14Producer(actions, positions, index, first["WFValue"])
		if err != nil {
			return err
		}
		if err = p15RP14ValueError(actions, positions, producer, "number", true); err != nil {
			return err
		}
		if !reflect.DeepEqual(rows[1], map[string]any{"WFItemType": uint64(0), "WFValue": false}) || !reflect.DeepEqual(rows[2], map[string]any{"WFItemType": uint64(0), "WFValue": ""}) {
			return fmt.Errorf("List false/empty Text changed or became Number/Text")
		}
	case "map":
		want := map[string]any{"changed": true}
		if initial {
			want = map[string]any{"number": p15ContainerNumber("0"), "bool": false, "text": "", "list": []any{}, "map": map[string]any{}}
		}
		_, wire := p15ContainerFieldValue(want)
		if kind != "is.workflow.actions.dictionary" || !reflect.DeepEqual(p["WFItems"], wire) {
			return fmt.Errorf("Map typed values or explicit empty containers changed")
		}
	}
	return nil
}
func p15RP14CaptureError(actions []map[string]any) error {
	positions := map[string]int{}
	for index, action := range actions {
		if uuid, ok := p15C01Params(action)["UUID"].(string); ok && uuid != "" {
			if _, duplicate := positions[uuid]; duplicate {
				return fmt.Errorf("duplicate capture/producer identity")
			}
			positions[uuid] = index
		}
	}
	consumer := -1
	for index, action := range actions {
		if action["WFWorkflowActionIdentifier"] == "com.example.p15.typed-capture" {
			if consumer >= 0 {
				return fmt.Errorf("duplicate final typed consumer")
			}
			consumer = index
		}
		refs, _ := p15V02NativeReferences(p15C01Params(action))
		for _, ref := range refs {
			if producer, exists := positions[ref]; !exists || producer >= index {
				return fmt.Errorf("dangling or forward producer reference")
			}
		}
	}
	if consumer < 0 || consumer != len(actions)-2 || actions[len(actions)-1]["WFWorkflowActionIdentifier"] != "is.workflow.actions.nothing" {
		return fmt.Errorf("missing final typed consumer/void boundary")
	}
	fields := p15C01Params(actions[consumer])
	if len(fields) != len(p15RP14Fields) {
		return fmt.Errorf("missing or extra saved consumer field")
	}
	for _, field := range p15RP14Fields {
		saved := p15RP14Saved[field]
		capture := -1
		writes := []int{}
		for index, action := range actions {
			p := p15C01Params(action)
			kind := action["WFWorkflowActionIdentifier"]
			if field == "clipboard" {
				if kind == "is.workflow.actions.setclipboard" {
					writes = append(writes, index)
				}
				if kind == "is.workflow.actions.getclipboard" {
					if capture >= 0 || p["CustomOutputName"] != saved || len(p) != 2 {
						return fmt.Errorf("Clipboard capture was repeated, renamed or parameterized")
					}
					capture = index
				}
			} else {
				if kind == "is.workflow.actions.setvariable" && p["WFVariableName"] == field+"Value" {
					writes = append(writes, index)
				}
				if kind == "is.workflow.actions.getvariable" && p["CustomOutputName"] == saved {
					if capture >= 0 {
						return fmt.Errorf("duplicate saved read")
					}
					capture = index
					if !reflect.DeepEqual(p["WFVariable"], map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "Variable", "VariableName": field + "Value"}}) {
						return fmt.Errorf("saved read changed scope/source")
					}
				}
			}
		}
		if len(writes) != 2 || capture < 0 || !(writes[0] < capture && capture < writes[1] && writes[1] < consumer) {
			return fmt.Errorf("%s was not captured between original and later write", field)
		}
		uuid, _ := p15C01Params(actions[capture])["UUID"].(string)
		if uuid == "" || !p15RP14Direct(fields[field], uuid, saved) {
			return fmt.Errorf("%s consumer rereads or coerces instead of using saved value", field)
		}
		for index, write := range writes {
			producer, err := p15RP14Producer(actions, positions, write, p15C01Params(actions[write])["WFInput"])
			if err != nil {
				return err
			}
			if field == "clipboard" {
				want := []string{"rp14_clip_before", "rp14_clip_after"}[index]
				if actions[producer]["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" || p15C01Params(actions[producer])["WFTextActionText"] != want {
					return fmt.Errorf("Clipboard write value changed")
				}
			} else if err = p15RP14ValueError(actions, positions, producer, field, index == 0); err != nil {
				return err
			}
		}
	}
	return nil
}
func TestP15RP14TypedCaptureNative(t *testing.T) {
	original := p15CompiledActions(t, p15RP14Source)
	if err := p15RP14CaptureError(original); err != nil {
		t.Fatal(err)
	}
	for _, field := range p15RP14Fields {
		for _, fault := range []string{"late_capture", "late_read", "text_coercion", "wrong_scope"} {
			t.Run(field+"/"+fault, func(t *testing.T) {
				broken := p15ContainerCloneNative(t, original)
				capture, consumer := -1, -1
				lastWrite := -1
				for index, action := range broken {
					p := p15C01Params(action)
					if p["CustomOutputName"] == p15RP14Saved[field] {
						capture = index
					}
					if action["WFWorkflowActionIdentifier"] == "com.example.p15.typed-capture" {
						consumer = index
					}
					if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" && p["WFVariableName"] == field+"Value" || field == "clipboard" && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setclipboard" {
						lastWrite = index
					}
				}
				if capture < 0 || consumer < 0 || lastWrite < capture {
					t.Fatal("fault missed real capture sequence")
				}
				switch fault {
				case "late_capture":
					var moved []map[string]any
					for index, action := range broken {
						if index != capture {
							moved = append(moved, action)
						}
						if index == lastWrite {
							moved = append(moved, broken[capture])
						}
					}
					broken = moved
				case "late_read":
					token := map[string]any{"Type": "Variable", "VariableName": field + "Value"}
					if field == "clipboard" {
						token = map[string]any{"Type": "Clipboard"}
					}
					p15C01Params(broken[consumer])[field] = map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": token}
				case "text_coercion":
					p15C01Params(broken[consumer])[field] = map[string]any{"WFSerializationType": "WFTextTokenString", "Value": map[string]any{"string": "coerced"}}
				case "wrong_scope":
					if field == "clipboard" {
						broken[capture]["WFWorkflowActionIdentifier"] = "is.workflow.actions.date"
					} else {
						p15C01Params(broken[capture])["WFVariable"].(map[string]any)["Value"].(map[string]any)["VariableName"] = "otherScope"
					}
				}
				if p15RP14CaptureError(broken) == nil {
					t.Fatal("accepted typed capture corruption")
				}
			})
		}
	}
	for _, fault := range []string{"number_as_text", "bool_to_true", "list_false_to_zero", "list_reordered", "map_false_to_zero", "missing_empty_list", "missing_empty_map", "duplicate_capture", "missing_consumer"} {
		t.Run(fault, func(t *testing.T) {
			broken := p15ContainerCloneNative(t, original)
			firstBoolProducer := ""
			for _, action := range broken {
				p := p15C01Params(action)
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" && p["WFVariableName"] == "boolValue" {
					refs, variables := p15V02NativeReferences(p["WFInput"])
					if len(refs) != 1 || len(variables) != 0 {
						t.Fatal("missing actual Bool producer for control")
					}
					firstBoolProducer = refs[0]
					break
				}
			}
			changed := false
			for _, action := range broken {
				p := p15C01Params(action)
				kind := action["WFWorkflowActionIdentifier"]
				if fault == "number_as_text" && !changed && kind == "is.workflow.actions.number" {
					action["WFWorkflowActionIdentifier"] = "is.workflow.actions.gettext"
					changed = true
				}
				if fault == "bool_to_true" && p["UUID"] == firstBoolProducer {
					p["WFNumberActionNumber"] = float64(1)
					changed = true
				}
				if (fault == "list_false_to_zero" || fault == "list_reordered") && kind == "is.workflow.actions.list" {
					rows := p["WFItems"].([]any)
					if len(rows) == 3 {
						if fault == "list_reordered" {
							rows[0], rows[1] = rows[1], rows[0]
						} else {
							rows[1].(map[string]any)["WFValue"] = uint64(0)
						}
						changed = true
					}
				}
				if (fault == "map_false_to_zero" || fault == "missing_empty_list" || fault == "missing_empty_map") && kind == "is.workflow.actions.dictionary" {
					fields, err := p15ContainerFields(p["WFItems"])
					if err != nil {
						t.Fatal(err)
					}
					if fields["bool"] != nil {
						switch fault {
						case "map_false_to_zero":
							fields["bool"]["WFValue"].(map[string]any)["Value"] = uint64(0)
						case "missing_empty_list":
							delete(fields["list"]["WFValue"].(map[string]any), "Value")
						case "missing_empty_map":
							delete(fields["map"]["WFValue"].(map[string]any)["Value"].(map[string]any), "WFDictionaryFieldValueItems")
						}
						changed = true
					}
				}
				if fault == "duplicate_capture" && p["CustomOutputName"] == "savedMap" {
					broken = append(broken, action)
					changed = true
					break
				}
				if fault == "missing_consumer" && kind == "com.example.p15.typed-capture" {
					delete(p, "map")
					changed = true
				}
			}
			if !changed {
				t.Fatal("typed corruption missed actual producer")
			}
			if p15RP14CaptureError(broken) == nil {
				t.Fatal("accepted typed shape/value corruption")
			}
		})
	}
}
