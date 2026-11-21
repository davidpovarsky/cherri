package main

import (
	"encoding/json"
	"fmt"
	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/types"
	"os"
	"reflect"
	"strings"
	"testing"
)

// Independent Apple metadata, never a second Cherri registry or emitter oracle.
func p15T03AppleNumberContract(t *testing.T) (string, string) {
	t.Helper()
	data, err := os.ReadFile("docs/apple-runtime/macos-27/builtin-actions.json")
	if err != nil {
		t.Fatal(err)
	}
	var actions map[string]struct {
		ActionClass string
		Output      struct {
			Multiple bool
			Types    []string
		}
		Parameters []struct {
			Key, Class           string
			AllowsDecimalNumbers bool
		}
	}
	if err := json.Unmarshal(data, &actions); err != nil {
		t.Fatal(err)
	}
	id, key := "", ""
	for name, native := range actions {
		if native.ActionClass != "WFNumberAction" {
			continue
		}
		if id != "" || native.Output.Multiple || !reflect.DeepEqual(native.Output.Types, []string{"NSDecimalNumber"}) {
			t.Fatal("ambiguous or changed native Number output")
		}
		id = name
		for _, parameter := range native.Parameters {
			if parameter.Class == "WFNumberFieldParameter" && parameter.AllowsDecimalNumbers {
				if key != "" {
					t.Fatal("ambiguous native Number parameter")
				}
				key = parameter.Key
			}
		}
	}
	if id == "" || key == "" {
		t.Fatal("independent native Number contract missing")
	}
	data, err = os.ReadFile("docs/apple-runtime/macos-27/parameter-encodings.json")
	if err != nil {
		t.Fatal(err)
	}
	var encodings struct {
		ActionParameters map[string][]struct {
			Class, Key, SingleStateClass string
			VariableTypes                []string
		}
	}
	if err := json.Unmarshal(data, &encodings); err != nil {
		t.Fatal(err)
	}
	parameters := encodings.ActionParameters[id]
	if len(parameters) != 1 || parameters[0].Key != key || parameters[0].Class != "WFNumberFieldParameter" || parameters[0].SingleStateClass != "WFNumberStringSubstitutableState" {
		t.Fatal("independent number/string state disagrees with action")
	}
	data, err = os.ReadFile("docs/apple-runtime/macos-27/encoding-table.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct{ StateClasses map[string]map[string]any }
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	state := table.StateClasses[parameters[0].SingleStateClass]
	if state["value:string"] != "hello" || state["value:number"] != float64(5) {
		t.Fatal("Apple state does not independently show both string and numeric wire forms")
	}
	round := actions["is.workflow.actions.round"]
	if round.ActionClass != "WFRoundNumberAction" || !reflect.DeepEqual(round.Output.Types, []string{"NSDecimalNumber"}) {
		t.Fatal("independent numeric consumer contract changed")
	}
	roundInput := false
	for _, parameter := range encodings.ActionParameters["is.workflow.actions.round"] {
		if parameter.Key == "WFInput" && parameter.Class == "WFNumberFieldParameter" && parameter.SingleStateClass == "WFNumberStringSubstitutableState" {
			roundInput = true
		}
	}
	if !roundInput {
		t.Fatal("numeric parameter lacks independent number/string state")
	}
	return id, key
}

func TestP15T03NumberStringSemanticSignature(t *testing.T) {
	nativeID, nativeKey := p15T03AppleNumberContract(t)
	registry := schema.DefaultRegistry()
	constructor, ok := registry.LookupAction("number")
	if !ok || constructor.AppleIdentifier != nativeID || !constructor.OutputType.Equals(types.Number) {
		t.Fatal("public constructor lost native numeric output")
	}
	input, ok := constructor.PrimaryParameter()
	if !ok || input.WireKey != nativeKey || input.Codec != "token" {
		t.Fatal("existing native number/string codec binding changed")
	}
	for _, name := range []string{"wait", "round"} {
		consumer, ok := registry.LookupAction(name)
		if !ok {
			t.Fatal("canonical numeric consumer missing")
		}
		parameter, ok := consumer.PrimaryParameter()
		if !ok || !parameter.Type.Equals(types.Number) {
			t.Fatalf("%s no longer requires Number", name)
		}
	}
	for _, expression := range []string{"number(\"42\")", "number(\"-5.125\")", "number(text(\"42\"))"} {
		src := "let converted: Number = " + expression + "\nwait(converted)"
		program, analyzer := p15T08Analyzer(t, src)
		actual := analyzer.ExprTypes()[p15BindingValue(t, program, "converted")]
		if actual == nil || !actual.Equals(types.Number) {
			t.Fatalf("wire text changed semantic result into %v", actual)
		}
		compileDebugFixture(t, src, false)
	}
	for _, expression := range []string{"\"not-a-number\"", "text(\"42\")"} {
		src := "wait(" + expression + ")"
		if err := p15ExactLiteralDiagnostic(p15TypeDiagnostics(t, src), src, expression, analysis.CodeArgumentType); err != nil {
			t.Fatal(err)
		}
		raw, err := CompileSourceToPlist("p15-t03-invalid.cherri", src)
		if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeArgumentType) {
			t.Fatalf("plain Text silently became Number: bytes=%d err=%v", len(raw), err)
		}
	}
}

func p15T03LiteralWire(actions []ShortcutAction, nativeID, nativeKey, literal string, legacy bool) error {
	count := 2
	if !legacy {
		count++
		if err := p15M02MainVoidDebugError(actions); err != nil {
			return err
		}
	}
	if len(actions) != count || actions[0].WFWorkflowActionIdentifier != nativeID || actions[1].WFWorkflowActionIdentifier != "is.workflow.actions.delay" {
		return fmt.Errorf("number/consumer action identity or once-only order changed")
	}
	parameters := actions[0].WFWorkflowActionParameters
	value, ok := parameters[nativeKey].(string)
	if !ok || value != literal {
		return fmt.Errorf("exact numeric-string wire changed: %T %#v", parameters[nativeKey], parameters[nativeKey])
	}
	uuid, ok := parameters["UUID"].(string)
	if !ok || uuid == "" || requireOutputUUID(actions[1].WFWorkflowActionParameters["WFDelayTime"]) != uuid {
		return fmt.Errorf("numeric consumer no longer uses exact Number producer")
	}
	if _, exists := parameters["WFTextActionText"]; exists {
		return fmt.Errorf("Number was replaced by Text")
	}
	return nil
}

func TestP15T03NumberStringNativeWire(t *testing.T) {
	nativeID, nativeKey := p15T03AppleNumberContract(t)
	for _, legacy := range []bool{false, true} {
		for _, literal := range []string{"0", "-5", "42.125", "9007199254740993"} {
			t.Run(fmt.Sprintf("legacy=%t/value=%s", legacy, literal), func(t *testing.T) {
				declaration := "let"
				if legacy {
					declaration = "const"
				}
				src := declaration + " converted = number(" + fmt.Sprintf("%q", literal) + ")\nwait(converted)"
				actions := compileDebugFixture(t, src, legacy)
				if err := p15T03LiteralWire(actions, nativeID, nativeKey, literal, legacy); err != nil {
					t.Fatal(err)
				}
				for _, fault := range []string{"numeric_instead_of_string", "changed_string", "wrong_consumer_uuid", "wrong_native_action"} {
					// Detached mutation copy; the positive oracle examines actual plist types.
					var broken []ShortcutAction
					data, err := json.Marshal(actions)
					if err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal(data, &broken); err != nil {
						t.Fatal(err)
					}
					switch fault {
					case "numeric_instead_of_string":
						broken[0].WFWorkflowActionParameters[nativeKey] = float64(42)
					case "changed_string":
						broken[0].WFWorkflowActionParameters[nativeKey] = literal + "0"
					case "wrong_consumer_uuid":
						broken[1].WFWorkflowActionParameters["WFDelayTime"] = p13TokenAttachment("wrong-producer", "Number")
					case "wrong_native_action":
						broken[0].WFWorkflowActionIdentifier = "is.workflow.actions.gettext"
					}
					if p15T03LiteralWire(broken, nativeID, nativeKey, literal, legacy) == nil {
						t.Fatalf("%s corruption passed exact wire oracle", fault)
					}
				}
			})
		}
	}
	// The public Number parameter retains its exact native string codec too.
	for _, literal := range []string{"0", "-5", "42.125", "9007199254740993"} {
		src := "round(" + fmt.Sprintf("%q", literal) + ")"
		_, _ = p15T08Analyzer(t, src)
		actions := compileDebugFixture(t, src, false)
		if err := p15M02MainVoidDebugError(actions); err != nil {
			t.Fatal(err)
		}
		if len(actions) != 2 || actions[0].WFWorkflowActionIdentifier != "is.workflow.actions.round" || actions[0].WFWorkflowActionParameters["WFInput"] != literal {
			t.Fatalf("public numeric parameter lost exact string wire: %#v", actions)
		}
	}
	// Explicit conversion: the consumer uses Number, without bypassing it for Text.
	actions := compileDebugFixture(t, "let raw=text(\"42\")\nlet converted:Number=number(raw)\nwait(converted)", false)
	if err := p15M02MainVoidDebugError(actions); err != nil {
		t.Fatal(err)
	}
	if len(actions) != 4 || actions[0].WFWorkflowActionIdentifier != "is.workflow.actions.gettext" || actions[1].WFWorkflowActionIdentifier != nativeID || actions[2].WFWorkflowActionIdentifier != "is.workflow.actions.delay" {
		t.Fatalf("explicit conversion sequence changed: %v", actionSuffixes(actions))
	}
	textUUID, _ := actions[0].WFWorkflowActionParameters["UUID"].(string)
	numberUUID, _ := actions[1].WFWorkflowActionParameters["UUID"].(string)
	if textUUID == "" || numberUUID == "" || textUUID == numberUUID || requireOutputUUID(actions[1].WFWorkflowActionParameters[nativeKey]) != textUUID || requireOutputUUID(actions[2].WFWorkflowActionParameters["WFDelayTime"]) != numberUUID {
		t.Fatal("explicit conversion lost original producer identities")
	}
}
