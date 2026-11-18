package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func p15T08NativeChoiceContract(t *testing.T) (string, string) {
	t.Helper()
	raw, err := os.ReadFile("docs/apple-runtime/macos-27/builtin-actions.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata map[string]struct {
		ActionClass string
		Input       struct {
			InputTypeDeterminesOutputType bool
			ParameterKey                  string
		}
		Output     struct{ Multiple bool }
		Parameters []struct {
			Key, Class, Description string
			DefaultValue            any
		}
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatal(err)
	}
	for id, action := range metadata {
		if action.ActionClass != "WFChooseFromListAction" {
			continue
		}
		if !action.Input.InputTypeDeterminesOutputType || action.Input.ParameterKey != "WFInput" || action.Output.Multiple {
			t.Fatal("independent native input/default output contract changed")
		}
		for _, parameter := range action.Parameters {
			if parameter.Class == "WFSwitchParameter" && parameter.Description == "When enabled, multiple items may be chosen from the list." {
				if parameter.DefaultValue != false {
					t.Fatal("native single-selection default changed")
				}
				return id, parameter.Key
			}
		}
	}
	t.Fatal("independent native conditional-cardinality contract absent")
	return "", ""
}

func p15T08Analyzer(t *testing.T, src string) (*syntax.Program, *analysis.Analyzer) {
	t.Helper()
	program, parseErrors := p15ParseProgram(src)
	if len(parseErrors) != 0 {
		t.Fatal(parseErrors)
	}
	analyzer := analysis.NewAnalyzer(schema.DefaultRegistry())
	analyzer.Analyze(program)
	for _, d := range analyzer.Diagnostics() {
		if d.Severity == analysis.SeverityError {
			t.Fatal(d)
		}
	}
	return program, analyzer
}

func p15T08ChoiceWire(acts []ShortcutAction, id, modeKey string, mode any) error {
	var listUUID, selectedUUID string
	var choice, consumer *ShortcutAction
	for i := range acts {
		action := &acts[i]
		switch action.WFWorkflowActionIdentifier {
		case "is.workflow.actions.list":
			listUUID, _ = action.WFWorkflowActionParameters["UUID"].(string)
		case id:
			if choice != nil {
				return fmt.Errorf("multiple selection actions")
			}
			choice = action
			selectedUUID, _ = action.WFWorkflowActionParameters["UUID"].(string)
		case "is.workflow.actions.previewdocument":
			consumer = action
		}
	}
	if listUUID == "" || selectedUUID == "" || choice == nil || consumer == nil {
		return fmt.Errorf("missing native list/choice/consumer")
	}
	if requireOutputUUID(choice.WFWorkflowActionParameters["WFInput"]) != listUUID || requireOutputUUID(consumer.WFWorkflowActionParameters["WFInput"]) != selectedUUID {
		return fmt.Errorf("input or output producer identity changed")
	}
	actual, present := choice.WFWorkflowActionParameters[modeKey]
	switch want := mode.(type) {
	case nil:
		if present {
			return fmt.Errorf("omitted mode was materialized: %#v", actual)
		}
	case bool:
		if !present || actual != want {
			return fmt.Errorf("wrong literal mode: %#v want %v", actual, want)
		}
	case string:
		if !present || requireOutputUUID(actual) == "" {
			return fmt.Errorf("runtime mode is not a producer attachment: %#v", actual)
		}
	}
	return nil
}

func TestP15T08ConditionalCardinalityProduction(t *testing.T) {
	nativeID, modeKey := p15T08NativeChoiceContract(t)
	action, ok := schema.DefaultRegistry().LookupAction("chooseFromList")
	if !ok || action.AppleIdentifier != nativeID {
		t.Fatal("public call lost independently identified native action")
	}
	listText := types.NewList(types.Text)
	for _, tc := range []struct {
		name, prefix, args string
		want               types.Type
		wire               any
	}{
		{"default", "", "", types.Text, nil},
		{"false", "", ", selectMultiple: false", types.Text, false},
		{"true", "", ", selectMultiple: true", listText, true},
		{"folded_false", "", ", selectMultiple: !true", types.Text, false},
		{"folded_true", "", ", selectMultiple: !false", listText, true},
		{"positional_true", "", ", \"pick\", true", listText, true},
		{"runtime", "let mode = number(1) == 1\n", ", selectMultiple: mode", types.NewUnion(types.Text, listText), "dynamic"},
		{"immutable_binding", "let mode = true\n", ", selectMultiple: mode", types.NewUnion(types.Text, listText), "dynamic"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := tc.prefix + "let items=[\"one\", \"two\"]\nlet selected=chooseFromList(items" + tc.args + ")\nquicklook(selected)"
			program, analyzer := p15T08Analyzer(t, src)
			actual := analyzer.ExprTypes()[p15BindingValue(t, program, "selected")]
			if actual == nil || !actual.Equals(tc.want) {
				t.Fatalf("cardinality %v want %s", actual, tc.want)
			}
			acts := compileDebugFixture(t, src, false)
			if err := p15T08ChoiceWire(acts, nativeID, modeKey, tc.wire); err != nil {
				t.Fatal(err)
			}
			for i := range acts {
				if acts[i].WFWorkflowActionIdentifier == nativeID {
					parameters := acts[i].WFWorkflowActionParameters
					oldMode, present := parameters[modeKey]
					wrongMode := true
					if value, literal := tc.wire.(bool); literal {
						wrongMode = !value
					}
					parameters[modeKey] = wrongMode
					if p15T08ChoiceWire(acts, nativeID, modeKey, tc.wire) == nil {
						t.Fatal("wrong-mode mutation passed")
					}
					if present {
						parameters[modeKey] = oldMode
					} else {
						delete(parameters, modeKey)
					}
					acts[i].WFWorkflowActionParameters["WFInput"] = map[string]any{"Value": map[string]any{"OutputUUID": "wrong-producer"}}
				}
			}
			if p15T08ChoiceWire(acts, nativeID, modeKey, tc.wire) == nil {
				t.Fatal("wrong input mutation passed")
			}
		})
	}
	for _, tc := range []struct {
		source string
		want   types.Type
	}{
		{"let selected=chooseFromList([\"one\", 2])", types.NewUnion(types.Text, types.Number)},
		{"let selected=chooseFromList([2, \"one\"])", types.NewUnion(types.Number, types.Text)},
		{"let selected=chooseFromList([\"one\", 2], selectMultiple: true)", types.NewList(types.NewUnion(types.Text, types.Number))},
		{"let selected=chooseFromList(list: [\"one\"], selectMultiple: true)", types.NewList(types.Text)},
		{"let selected=chooseFromList(ShortcutInput)", types.Unknown},
		{"let selected=chooseFromList(ShortcutInput, selectMultiple: true)", types.NewList(types.Unknown)},
	} {
		program, analyzer := p15T08Analyzer(t, tc.source)
		actual := analyzer.ExprTypes()[p15BindingValue(t, program, "selected")]
		if actual == nil || !actual.Equals(tc.want) {
			t.Errorf("%s: %v want %v", tc.source, actual, tc.want)
		}
	}
	// Joining every element prevents a first-item annotation from claiming the
	// type of the remainder. Unknown members must not disappear in either order.
	for _, tc := range []struct {
		source string
		want   types.Type
	}{
		{"let value=[\"one\", 2]", types.NewList(types.NewUnion(types.Text, types.Number))},
		{"let value=[2, \"one\"]", types.NewList(types.NewUnion(types.Number, types.Text))},
		{"let value={\"first\": \"one\", \"second\": 2}", types.NewMap(types.Text, types.NewUnion(types.Text, types.Number))},
		{"let value={\"first\": 2, \"second\": \"one\"}", types.NewMap(types.Text, types.NewUnion(types.Number, types.Text))},
		{"let raw=run(\"P15_T08_OPAQUE\")\nlet value=[raw, \"one\"]", types.NewList(types.NewUnion(types.Unknown, types.Text))},
		{"let raw=run(\"P15_T08_OPAQUE\")\nlet value=[\"one\", raw]", types.NewList(types.NewUnion(types.Text, types.Unknown))},
	} {
		program, analyzer := p15T08Analyzer(t, tc.source)
		actual := analyzer.ExprTypes()[p15BindingValue(t, program, "value")]
		if actual == nil || !actual.Equals(tc.want) {
			t.Errorf("collection join %s: %v want %v", tc.source, actual, tc.want)
		}
	}
	for _, tc := range []struct{ name, src, argument string }{
		{"mixed_list_annotation", "let items=[\"one\", 2]\nlet wrong: List<Text> = items", "items"},
		{"mixed_map_annotation", "let items={\"a\": \"one\", \"b\": 2}\nlet wrong: Map<Text, Text> = items", "items"},
		{"multiple_as_scalar", "let chosen=chooseFromList([\"one\"], selectMultiple:true)\nlet wrong: Text=chosen", "chosen"},
		{"dynamic_as_scalar", "let mode=true\nlet chosen=chooseFromList([\"one\"], selectMultiple:mode)\nlet wrong: Text=chosen", "chosen"},
		{"multiple_record_field", "function pick(items: List<{name:Text}>) -> Text {\nlet chosen=chooseFromList(items, selectMultiple:true)\nreturn chosen.name\n}", "chosen.name"},
		{"dynamic_record_field", "function pick(items: List<{name:Text}>, mode:Bool) -> Text {\nlet chosen=chooseFromList(items, selectMultiple:mode)\nreturn chosen.name\n}", "chosen.name"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := p15TypeDiagnostics(t, tc.src)
			if err := p15ExactLiteralDiagnostic(diagnostics, tc.src, tc.argument, analysis.CodeArgumentType); err != nil {
				t.Fatal(err)
			}
			raw, err := CompileSourceToPlist("p15-t08.cherri", tc.src)
			if err == nil || len(raw) != 0 {
				t.Fatal("unproved scalar/field access reached native emission")
			}
			for i := range diagnostics {
				if diagnostics[i].Code == analysis.CodeArgumentType {
					diagnostics[i].Code = analysis.CodeUnknownName
				}
			}
			if p15ExactLiteralDiagnostic(diagnostics, tc.src, tc.argument, analysis.CodeArgumentType) == nil {
				t.Fatal("unrelated-error mutation passed")
			}
		})
	}
	good := "function pick(items: List<{name:Text}>) -> Text {\nlet chosen=chooseFromList(items, selectMultiple:false)\nreturn chosen.name\n}"
	p15T08Analyzer(t, good)
	compileDebugFixture(t, good, false)
}

func TestP15T08ConditionalCardinalityService(t *testing.T) {
	svc := service.NewService(nil)
	uri := "file:///p15-t08-service.cherri"
	src := "let mode=true\nlet one=chooseFromList([\"a\"])\nlet many=chooseFromList([\"a\"], selectMultiple:true)\nlet maybe=chooseFromList([\"a\"], selectMultiple:mode)"
	svc.OpenDocument(uri, 1, src)
	diagnostics, _ := svc.Analyze(uri)
	for _, d := range diagnostics {
		if d.Severity == analysis.SeverityError {
			t.Fatal(d)
		}
	}
	for _, tc := range []struct {
		line   int
		result string
	}{{2, "Text"}, {3, "List<Text>"}, {4, "Text | List<Text>"}} {
		line := strings.Split(src, "\n")[tc.line-1]
		column := strings.Index(line, "chooseFromList") + 1
		hover := svc.Hover(uri, tc.line, column)
		expected := "**Returns:** " + string(rune(96)) + tc.result + string(rune(96))
		if hover == nil || !strings.Contains(hover.Contents, expected) {
			t.Fatalf("contextual hover line%d want%s got%+v", tc.line, expected, hover)
		}
		signature := svc.SignatureHelp(uri, tc.line, len([]rune(line)))
		if signature == nil || !strings.HasSuffix(signature.Label, " -> "+tc.result) {
			t.Fatalf("contextual signature line%d: %+v", tc.line, signature)
		}
	}
	// A changed literal mode must update the same call's hover and diagnostics.
	changed := strings.Replace(src, "let one=chooseFromList([\"a\"])", "let one=chooseFromList([\"a\"], selectMultiple:true)", 1)
	svc.OpenDocument(uri, 2, changed)
	hover := svc.Hover(uri, 2, strings.Index(strings.Split(changed, "\n")[1], "chooseFromList")+1)
	if hover == nil || !strings.Contains(hover.Contents, "**Returns:** "+string(rune(96))+"List<Text>"+string(rune(96))) {
		t.Fatal("mode edit retained stale scalar hover")
	}
	bad := "function pick(items: List<{name:Text}>) -> Text {\nlet chosen=chooseFromList(items, selectMultiple:true)\nreturn chosen.name\n}"
	svc.OpenDocument(uri, 3, bad)
	diagnostics, _ = svc.Analyze(uri)
	if err := p15ExactLiteralDiagnostic(diagnostics, bad, "chosen.name", analysis.CodeArgumentType); err != nil {
		t.Fatal(err)
	}
	good := strings.Replace(bad, "selectMultiple:true", "selectMultiple:false", 1)
	svc.OpenDocument(uri, 4, good)
	diagnostics, _ = svc.Analyze(uri)
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == analysis.SeverityError {
			t.Fatal("single-mode edit retained stale cardinality error", diagnostic)
		}
	}
}
