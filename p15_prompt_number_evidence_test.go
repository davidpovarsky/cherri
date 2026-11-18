package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// These six actions are synthetic. No original Shortcut data is embedded here.
func TestP15PromptNumberOrderedImport(t *testing.T) {
	inputID := "8e0ca510-3388-4fb9-80cf-10f758a76001"
	groupID := "8e0ca510-3388-4fb9-80cf-10f758a76002"
	actions := []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.ask",
			"WFWorkflowActionParameters": map[string]any{
				"UUID": inputID, "WFAskActionPrompt": "P15 synthetic count", "WFInputType": "Number",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": groupID, "WFControlFlowMode": int64(0),
				"WFCondition": int64(2), "WFNumberValue": "7",
				"WFInput": p12ActionOutAttachment(inputID, "Provided Input"),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{"WFTextActionText": "high"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": groupID, "WFControlFlowMode": int64(1)},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{"WFTextActionText": "low"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": groupID, "WFControlFlowMode": int64(2)},
		},
	}
	expected := make(map[int]string, len(actions))
	for i := range actions {
		expected[i] = "NATURAL_VERIFIED"
	}
	assertP12ParDualVerified(t, "P15_PROMPT_NUMBER_ORDERED", actions, expected, nil, nil)
}

func TestP15PromptNumberIndependentNativeContract(t *testing.T) {
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
		ParameterOverrides map[string]struct {
			Key                 string
			IntentEnumOverrides map[string]string
		}
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatal(err)
	}
	native, exists := metadata["is.workflow.actions.ask"]
	if !exists || native.ActionClass != "WFAskForInputAction" || native.Output.Multiple ||
		!slices.Contains(native.Output.Types, "NSDecimalNumber") ||
		native.ParameterOverrides["type"].Key != "WFInputType" ||
		native.ParameterOverrides["type"].IntentEnumOverrides["number"] != "Number" ||
		native.ParameterOverrides["numberAnswer"].Key != "WFAskActionDefaultAnswerNumber" {
		t.Fatal("captured Apple Number-mode evidence changed")
	}
	canonical, exists := schema.DefaultRegistry().LookupAction("prompt")
	if !exists || canonical.AppleIdentifier != "is.workflow.actions.ask" ||
		!canonical.Builtin || canonical.Custom || !canonical.OutputType.Equals(types.Unknown) {
		t.Fatal("the general prompt contract must remain Unknown")
	}
	parameter, exists := canonical.ParameterByLabel("inputType")
	if !exists || parameter.WireKey != native.ParameterOverrides["type"].Key ||
		parameter.EnumName != "inputType" || !slices.Contains(parameter.EnumValues, "Number") {
		t.Fatal("canonical selector lost native evidence")
	}
}

func p15PromptConditionUUID(raw any) string {
	input, ok := raw.(map[string]any)
	if !ok || input["Type"] != "Variable" {
		return ""
	}
	return requireOutputUUID(input["Variable"])
}

func p15PromptNumberWire(actions []ShortcutAction) error {
	if len(actions) != 4 {
		return fmt.Errorf("unexpected action count %d", len(actions))
	}
	ask, conditional := actions[0], actions[1]
	if ask.WFWorkflowActionIdentifier != "is.workflow.actions.ask" ||
		ask.WFWorkflowActionParameters["WFInputType"] != "Number" {
		return fmt.Errorf("wrong numeric input producer or native mode")
	}
	uuid, _ := ask.WFWorkflowActionParameters["UUID"].(string)
	if uuid == "" || conditional.WFWorkflowActionIdentifier != "is.workflow.actions.conditional" ||
		p15PromptConditionUUID(conditional.WFWorkflowActionParameters["WFInput"]) != uuid {
		return fmt.Errorf("comparison is not attached to the actual Ask result")
	}
	parameters := conditional.WFWorkflowActionParameters
	if fmt.Sprint(parameters["WFControlFlowMode"]) != "0" ||
		fmt.Sprint(parameters["WFCondition"]) != "2" ||
		fmt.Sprint(parameters["WFNumberValue"]) != "7" ||
		parameters["WFConditionalActionString"] != nil || parameters["WFDate"] != nil {
		return fmt.Errorf("numeric ordered condition changed")
	}
	if actions[2].WFWorkflowActionIdentifier != "is.workflow.actions.gettext" ||
		actions[3].WFWorkflowActionIdentifier != "is.workflow.actions.conditional" ||
		fmt.Sprint(actions[3].WFWorkflowActionParameters["WFControlFlowMode"]) != "2" ||
		parameters["GroupingIdentifier"] != actions[3].WFWorkflowActionParameters["GroupingIdentifier"] {
		return fmt.Errorf("condition region changed or converter inserted")
	}
	return nil
}

func TestP15PromptNumberLiteralOutput(t *testing.T) {
	for _, input := range []struct{ name, args string }{
		{"named", "\"P15 synthetic count\", inputType: \"Number\""},
		{"positional", "\"P15 synthetic count\", \"Number\""},
		{"qualified_enum", "\"P15 synthetic count\", inputType: inputType.Number"},
		{"contextual_enum", "\"P15 synthetic count\", inputType: .Number"},
		{"parenthesized", "\"P15 synthetic count\", inputType: (\"Number\")"},
		{"folded_literal", "\"P15 synthetic count\", inputType: \"Num\" + \"ber\""},
	} {
		t.Run(input.name, func(t *testing.T) {
			source := "let answer = prompt(" + input.args + ")\nif answer > 7 { text(\"high\") }"
			program, analyzer := p15T08Analyzer(t, source)
			got := analyzer.ExprTypes()[p15BindingValue(t, program, "answer")]
			if got == nil || !got.Equals(types.Number) {
				t.Fatalf("literal Number result got %v", got)
			}
			actions := compileDebugFixture(t, source, false)
			if err := p15PromptNumberWire(actions); err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"mode", "producer", "condition", "rhs"} {
				first, second := actions[0].WFWorkflowActionParameters, actions[1].WFWorkflowActionParameters
				mode, uuid, condition, rhs := first["WFInputType"], first["UUID"], second["WFCondition"], second["WFNumberValue"]
				switch fault {
				case "mode":
					first["WFInputType"] = "Text"
				case "producer":
					first["UUID"] = "wrong-producer"
				case "condition":
					second["WFCondition"] = int64(4)
				case "rhs":
					second["WFNumberValue"] = 8.0
				}
				if p15PromptNumberWire(actions) == nil {
					t.Fatalf("%s mutation passed", fault)
				}
				first["WFInputType"], first["UUID"], second["WFCondition"], second["WFNumberValue"] = mode, uuid, condition, rhs
			}
		})
	}
}

func TestP15PromptNumberUnprovedModesRemainUnknown(t *testing.T) {
	for _, tc := range []struct{ name, prefix, expression string }{
		{"omitted", "", "prompt(\"synthetic\")"},
		{"text_mode", "", "prompt(\"synthetic\", inputType: \"Text\")"},
		{"date_mode", "", "prompt(\"synthetic\", inputType: \"Date\")"},
		{"bound_mode", "let mode = \"Number\"\n", "prompt(\"synthetic\", inputType: mode)"},
		{"computed_mode", "", "prompt(\"synthetic\", inputType: text(\"Number\"))"},
		{"record_mode", "let modes = {\"selected\":\"Number\"}\n", "prompt(\"synthetic\", inputType: modes.selected)"},
		{"opaque_action", "", "run(\"P15_OPAQUE_SYNTHETIC\")"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			binding := tc.prefix + "let answer = " + tc.expression
			program, analyzer := p15T08Analyzer(t, binding)
			got := analyzer.ExprTypes()[p15BindingValue(t, program, "answer")]
			if got == nil || !got.Equals(types.Unknown) {
				t.Fatalf("unproved result got %v", got)
			}
			source := binding + "\nif answer > 7 { text(\"high\") }"
			diagnostics := p15TypeDiagnostics(t, source)
			if err := p15ExactLiteralDiagnostic(diagnostics, source, "answer", analysis.CodeArgumentType); err != nil {
				t.Fatal(err)
			}
			raw, err := CompileSourceToPlist("p15-prompt-unknown.cherri", source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeArgumentType) {
				t.Fatalf("unproved Number reached emission: bytes=%d err=%v", len(raw), err)
			}
			svc := service.NewService(nil)
			uri := "file:///p15-prompt-unknown.cherri"
			svc.OpenDocument(uri, 1, source)
			diagnostics, _ = svc.Analyze(uri)
			if err := p15ExactLiteralDiagnostic(diagnostics, source, "answer", analysis.CodeArgumentType); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15PromptNumberDescriptorGuards(t *testing.T) {
	base, _ := schema.DefaultRegistry().LookupAction("prompt")
	for _, fault := range []string{"not_builtin", "custom", "identifier", "wire", "enum_name", "enum_number_absent"} {
		t.Run(fault, func(t *testing.T) {
			action := *base
			action.Parameters = append([]schema.ParameterSchema(nil), base.Parameters...)
			parameter, _ := action.ParameterByLabel("inputType")
			switch fault {
			case "not_builtin":
				action.Builtin = false
			case "custom":
				action.Custom = true
			case "identifier":
				action.AppleIdentifier = "com.example.synthetic.unproved"
			case "wire":
				parameter.WireKey = "OtherMode"
			case "enum_name":
				parameter.EnumName = ""
			case "enum_number_absent":
				parameter.EnumValues = nil
			}
			registry := schema.NewRegistry([]*schema.ActionSchema{&action}, nil)
			source := "let answer = prompt(\"synthetic\", inputType: \"Number\")"
			program, parseErrors := p15ParseProgram(source)
			if len(parseErrors) != 0 {
				t.Fatal(parseErrors)
			}
			analyzer := analysis.NewAnalyzer(registry)
			analyzer.Analyze(program)
			for _, diagnostic := range analyzer.Diagnostics() {
				if diagnostic.Severity == analysis.SeverityError && !(fault == "enum_number_absent" && diagnostic.Code == analysis.CodeEnumMember) {
					t.Fatal(diagnostic)
				}
			}
			got := analyzer.ExprTypes()[p15BindingValue(t, program, "answer")]
			if got == nil || !got.Equals(types.Unknown) {
				t.Fatalf("%s manufactured Number: %v", fault, got)
			}
		})
	}
}

func TestP15PromptNumberService(t *testing.T) {
	svc := service.NewService(nil)
	uri := "file:///p15-prompt-service.cherri"
	source := "let answer = prompt(\"synthetic\", inputType: .Number)\nif answer > 7 { text(\"high\") }"
	svc.OpenDocument(uri, 1, source)
	diagnostics, _ := svc.Analyze(uri)
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == analysis.SeverityError {
			t.Fatal(diagnostic)
		}
	}
	firstLine := strings.Split(source, "\n")[0]
	hover := svc.Hover(uri, 1, strings.Index(firstLine, "prompt")+1)
	if hover == nil || !strings.Contains(hover.Contents, "**Returns:** "+string(rune(96))+"Number"+string(rune(96))) {
		t.Fatalf("contextual hover lost Number: %+v", hover)
	}
	signature := svc.SignatureHelp(uri, 1, len(firstLine))
	if signature == nil || !strings.HasSuffix(signature.Label, " -> Number") {
		t.Fatalf("signature lost Number: %+v", signature)
	}
	changed := strings.Replace(source, ".Number", ".Text", 1)
	svc.OpenDocument(uri, 2, changed)
	diagnostics, _ = svc.Analyze(uri)
	if err := p15ExactLiteralDiagnostic(diagnostics, changed, "answer", analysis.CodeArgumentType); err != nil {
		t.Fatal(err)
	}
	hover = svc.Hover(uri, 1, strings.Index(firstLine, "prompt")+1)
	if hover == nil || !strings.Contains(hover.Contents, "**Returns:** "+string(rune(96))+"Unknown"+string(rune(96))) {
		t.Fatalf("mode edit retained stale Number evidence: %+v", hover)
	}
}
