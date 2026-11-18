package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func p15T09NegativeSources() []struct{ name, source, span, code string } {
	return []struct{ name, source, span, code string }{
		{"text_to_date_literal", "let claimed: Date = \"2026-11-18\"", "\"2026-11-18\"", analysis.CodeArgumentType},
		{"text_to_date_output", "let claimed: Date = text(\"2026-11-18\")", "text(\"2026-11-18\")", analysis.CodeArgumentType},
		{"date_to_text_output", "let claimed: Text = currentDate()", "currentDate()", analysis.CodeArgumentType},
		{"optional_leaf", "let claimed: Date? = text(\"2026-11-18\")", "text(\"2026-11-18\")", analysis.CodeArgumentType},
		{"union_leaf", "let claimed: Date | Number = text(\"2026-11-18\")", "text(\"2026-11-18\")", analysis.CodeArgumentType},
		{"list_leaf", "let claimed: List<Date> = [text(\"2026-11-18\")]", "[text(\"2026-11-18\")]", analysis.CodeArgumentType},
		{"map_leaf", "let claimed: Map<Text, Date> = {\"key\":text(\"2026-11-18\")}", "{\"key\":text(\"2026-11-18\")}", analysis.CodeArgumentType},
		{"record_leaf", "function claim(value:{when:Text})->{when:Date}{return value}", "return value", analysis.CodeReturnType},
		{"function_argument", "function accept(value:Date)->Date{return value}\naccept(text(\"2026-11-18\"))", "text(\"2026-11-18\")", analysis.CodeArgumentType},
		{"function_return", "function claim()->Date{return text(\"2026-11-18\")}", "return text(\"2026-11-18\")", analysis.CodeReturnType},
		{"anycontent", "let claimed: Date = Clipboard", "Clipboard", analysis.CodeArgumentType},
		{"unknown", "let claimed: Date = chooseFromList(Clipboard)", "chooseFromList(Clipboard)", analysis.CodeArgumentType},
		{"invented_view", "let claimed: Date = Clipboard.p15SafeDate", "Clipboard.p15SafeDate", analysis.CodeArgumentType},
		{"opaque_raw_view", "let claimed: Date = native.ref(\"P15_OPAQUE\", coerce: \"WFP15UnverifiedContentItem\")", "native.ref(\"P15_OPAQUE\", coerce: \"WFP15UnverifiedContentItem\")", analysis.CodeArgumentType},
	}
}

func TestP15T09ContentConversionEvidence(t *testing.T) {
	if !types.Text.AssignableTo(types.Date) || !types.Date.AssignableTo(types.Text) {
		t.Fatal("existing native parameter compatibility changed")
	}
	if types.HasAssignmentEvidence(types.Text, types.Date) || types.HasAssignmentEvidence(types.Date, types.Text) {
		t.Error("compatibility alone minted an authored conversion guarantee")
	}
	for _, tc := range p15T09NegativeSources() {
		t.Run(tc.name, func(t *testing.T) {
			if err := p15ExactLiteralDiagnostic(p15TypeDiagnostics(t, tc.source), tc.source, tc.span, tc.code); err != nil {
				t.Error(err)
			}
			raw, err := CompileSourceToPlist("p15-t09-negative.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("unverified conversion emitted bytes=%d err=%v", len(raw), err)
			}
			svc := service.NewService(nil)
			uri := "file:///p15-t09-" + tc.name + ".cherri"
			svc.OpenDocument(uri, 1, tc.source)
			diagnostics, _ := svc.Analyze(uri)
			if err := p15ExactLiteralDiagnostic(diagnostics, tc.source, tc.span, tc.code); err != nil {
				t.Fatal("service", err)
			}
		})
	}
	for _, source := range []string{
		"let raw=text(\"2026-11-18\")\nlet converted:Date=raw.date\nshow(converted)",
		"let raw=currentDate()\nlet converted:Text=raw.text\nshow(converted)",
		"let converted:Number=number(text(\"42\"))\nwait(converted)",
		"let converted:Text=text(currentDate())\nshow(converted)",
		"let converted:Date=date(text(\"2026-11-18\"))\nshow(converted)",
		"let opaque:AnyContent=Clipboard\nshow(opaque)",
	} {
		p15T08Analyzer(t, source)
		compileDebugFixture(t, source, false)
	}
}

type p15T09ConversionCase struct {
	name, source, sourceID, actionID, actionClass, wireKey string
	outputs                                                []string
	multiple                                               bool
}

func p15T09Conversions() []p15T09ConversionCase {
	return []p15T09ConversionCase{
		{"text_to_number", "let original=text(\"42\")\nlet converted=number(original)\nshow(converted)", "is.workflow.actions.gettext", "is.workflow.actions.number", "WFNumberAction", "WFNumberActionNumber", []string{"NSDecimalNumber"}, false},
		{"date_to_text", "let original=currentDate()\nlet converted=text(original)\nshow(converted)", "is.workflow.actions.date", "is.workflow.actions.gettext", "WFTextAction", "WFTextActionText", []string{"NSString"}, false},
		{"text_to_date", "let original=text(\"2026-11-18\")\nlet converted=date(original)\nshow(converted)", "is.workflow.actions.gettext", "is.workflow.actions.date", "WFDateAction", "WFDateActionDate", []string{"NSDate", "NSDateComponents"}, false},
		{"json_text_to_dictionary", "let original=text(\"{\\\"value\\\":1}\")\nlet converted=getDictionary(original)\nshow(converted)", "is.workflow.actions.gettext", "is.workflow.actions.detect.dictionary", "WFCoercionAction", "WFInput", []string{"WFDictionaryContentItem"}, true},
	}
}

func p15T09ConversionWire(actions []ShortcutAction, tc p15T09ConversionCase) error {
	if len(actions) != 3 || actions[0].WFWorkflowActionIdentifier != tc.sourceID ||
		actions[1].WFWorkflowActionIdentifier != tc.actionID ||
		actions[2].WFWorkflowActionIdentifier != "is.workflow.actions.showresult" {
		return fmt.Errorf("explicit conversion producer/order changed: %#v", actions)
	}
	original, _ := actions[0].WFWorkflowActionParameters["UUID"].(string)
	converted, _ := actions[1].WFWorkflowActionParameters["UUID"].(string)
	if original == "" || converted == "" || original == converted {
		return fmt.Errorf("conversion output identity missing or reused")
	}
	outputs, variables := p15V02NativeReferences(actions[1].WFWorkflowActionParameters[tc.wireKey])
	if !reflect.DeepEqual(outputs, []string{original}) || len(variables) != 0 {
		return fmt.Errorf("conversion did not consume exact original output: %v %v", outputs, variables)
	}
	outputs, variables = p15V02NativeReferences(actions[2].WFWorkflowActionParameters)
	if !reflect.DeepEqual(outputs, []string{converted}) || len(variables) != 0 {
		return fmt.Errorf("consumer bypassed conversion: %v %v", outputs, variables)
	}
	return nil
}

func TestP15T09ContentConversionNativeWire(t *testing.T) {
	raw, err := os.ReadFile("docs/apple-runtime/macos-27/builtin-actions.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata map[string]struct {
		ActionClass string
		Output      struct {
			Multiple bool
			Types    []string
		}
		Parameters []struct{ Key string }
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatal(err)
	}
	for _, tc := range p15T09Conversions() {
		t.Run(tc.name, func(t *testing.T) {
			native, ok := metadata[tc.actionID]
			if !ok || native.ActionClass != tc.actionClass || native.Output.Multiple != tc.multiple || !reflect.DeepEqual(native.Output.Types, tc.outputs) {
				t.Fatalf("independent native conversion metadata changed: %+v", native)
			}
			fieldFound := false
			for _, p := range native.Parameters {
				fieldFound = fieldFound || p.Key == tc.wireKey
			}
			if !fieldFound {
				t.Fatal("independent native conversion field absent")
			}
			actions := compileDebugFixture(t, tc.source, false)
			if err := p15T09ConversionWire(actions, tc); err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"missing_conversion", "wrong_identifier", "wrong_input", "bypassed_conversion", "aliased_uuid"} {
				raw, err := json.Marshal(actions)
				if err != nil {
					t.Fatal(err)
				}
				var broken []ShortcutAction
				if err := json.Unmarshal(raw, &broken); err != nil {
					t.Fatal(err)
				}
				switch fault {
				case "missing_conversion":
					broken = append(broken[:1], broken[2:]...)
				case "wrong_identifier":
					broken[1].WFWorkflowActionIdentifier = "com.example.unverified.cast"
				case "wrong_input":
					broken[1].WFWorkflowActionParameters[tc.wireKey] = p13TokenAttachment("wrong-original", "opaque")
				case "bypassed_conversion":
					broken[2].WFWorkflowActionParameters = map[string]any{"WFInput": p13TokenAttachment(fmt.Sprint(broken[0].WFWorkflowActionParameters["UUID"]), "original")}
				case "aliased_uuid":
					broken[1].WFWorkflowActionParameters["UUID"] = broken[0].WFWorkflowActionParameters["UUID"]
				}
				if p15T09ConversionWire(broken, tc) == nil {
					t.Fatalf("conversion oracle accepted %s", fault)
				}
			}
		})
	}
}
