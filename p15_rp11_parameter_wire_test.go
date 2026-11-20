package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// These expectations come from captured Apple parameter-state serialization,
// never from Cherri's parameter encoder or generated registry.
type p15RP11AppleStates map[string]map[string]any

func p15RP11ReadAppleStates(t *testing.T) p15RP11AppleStates {
	t.Helper()
	raw, err := os.ReadFile("docs/apple-runtime/macos-27/encoding-table.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct{ StateClasses p15RP11AppleStates }
	if err := json.Unmarshal(raw, &table); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile("docs/apple-runtime/macos-27/parameter-encodings.json")
	if err != nil {
		t.Fatal(err)
	}
	var encodings struct {
		ActionParameters map[string][]struct{ Key, Class, SingleStateClass string }
	}
	if err := json.Unmarshal(raw, &encodings); err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ action, key, class, state string }{
		{"is.workflow.actions.image.resize", "WFImageResizeWidth", "WFNumberFieldParameter", "WFNumberStringSubstitutableState"},
		{"is.workflow.actions.delay", "WFDelayTime", "WFStepperParameter", "WFNumberSubstitutableState"},
		{"is.workflow.actions.gettext", "WFTextActionText", "WFTextInputParameter", "WFVariableStringParameterState"},
	} {
		found := 0
		for _, parameter := range encodings.ActionParameters[want.action] {
			if parameter.Key == want.key {
				found++
				if parameter.Class != want.class || parameter.SingleStateClass != want.state {
					t.Fatalf("independent parameter-state binding changed: %+v", parameter)
				}
			}
		}
		if found != 1 || table.StateClasses[want.state] == nil {
			t.Fatalf("missing unique Apple state for %s.%s", want.action, want.key)
		}
	}
	return table.StateClasses
}

func p15RP11Clone(t *testing.T, value any) any {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

type p15RP11WireCase struct {
	name, source, action, key, state, nativeSample, reference string
	literal                                                   any
	contextual                                                bool
}

func p15RP11WireCases() []p15RP11WireCase {
	return []p15RP11WireCase{
		{name: "semantic_width", source: "resizeImage(ShortcutInput, 640)", action: "is.workflow.actions.image.resize", key: "WFImageResizeWidth", state: "WFNumberStringSubstitutableState", nativeSample: "value:number", literal: float64(640)},
		{name: "bound_semantic_width", source: "let width: Number = 640\nresizeImage(ShortcutInput, width)", action: "is.workflow.actions.image.resize", key: "WFImageResizeWidth", state: "WFNumberStringSubstitutableState", nativeSample: "variable:ActionOutput", reference: "number", literal: float64(640)},
		{name: "literal_width_text", source: "resizeImage(ShortcutInput, \"9007199254740993\")", action: "is.workflow.actions.image.resize", key: "WFImageResizeWidth", state: "WFNumberStringSubstitutableState", nativeSample: "value:string", literal: "9007199254740993"},
		{name: "mutable_width", source: "var width: Number = 640\nresizeImage(ShortcutInput, width)", action: "is.workflow.actions.image.resize", key: "WFImageResizeWidth", state: "WFNumberStringSubstitutableState", nativeSample: "variable:MyVar", reference: "variable"},
		{name: "output_width", source: "let width = number(\"640\")\nresizeImage(ShortcutInput, width)", action: "is.workflow.actions.image.resize", key: "WFImageResizeWidth", state: "WFNumberStringSubstitutableState", nativeSample: "variable:ActionOutput", reference: "number", literal: "640"},
		{name: "literal_stepper", source: "wait(5)", action: "is.workflow.actions.delay", key: "WFDelayTime", state: "WFNumberSubstitutableState", nativeSample: "number:5", literal: float64(5)},
		{name: "mutable_stepper", source: "var width: Number = 5\nwait(width)", action: "is.workflow.actions.delay", key: "WFDelayTime", state: "WFNumberSubstitutableState", nativeSample: "variable:MyVar", reference: "variable"},
		{name: "output_stepper", source: "let width = number(\"5\")\nwait(width)", action: "is.workflow.actions.delay", key: "WFDelayTime", state: "WFNumberSubstitutableState", nativeSample: "variable:ActionOutput", reference: "number", literal: "5"},
		{name: "mutable_text", source: "var value: Text = \"hello\"\ntext(value)", action: "is.workflow.actions.gettext", key: "WFTextActionText", state: "WFVariableStringParameterState", nativeSample: "variable:MyVar", reference: "variable"},
		{name: "output_text", source: "let value = text(\"hello\")\ntext(value)", action: "is.workflow.actions.gettext", key: "WFTextActionText", state: "WFVariableStringParameterState", nativeSample: "variable:ActionOutput", reference: "gettext", literal: "hello"},
		{name: "contextual_text", source: "var width: Number = 640\ntext(f\"😀:{width}:é\")", action: "is.workflow.actions.gettext", key: "WFTextActionText", state: "WFVariableStringParameterState", nativeSample: "variableString:Hello <MyVar>!", reference: "variable", contextual: true},
	}
}

func TestP15RP11SemanticNumberAndContext(t *testing.T) {
	_ = p15RP11ReadAppleStates(t)
	for _, fixture := range p15RP11WireCases() {
		t.Run(fixture.name, func(t *testing.T) {
			program, analyzer := p15T08Analyzer(t, fixture.source)
			if strings.Contains(fixture.source, "width: Number") || strings.Contains(fixture.source, "let width = number") {
				typ := analyzer.ExprTypes()[p15BindingValue(t, program, "width")]
				if typ == nil || !typ.Equals(types.Number) {
					t.Fatalf("numeric width lost its semantic Number type: %v", typ)
				}
			}
			compileDebugFixture(t, fixture.source, false)
		})
	}
	// The scalar stepper is still numeric; unrelated Text is not admitted merely
	// because another parameter supports a number/string native state.
	for _, source := range []string{"wait(\"hello\")", "var value: Text = \"hello\"\nwait(value)"} {
		raw, err := CompileSourceToPlist("rp11-number-rejection.cherri", source)
		if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeArgumentType) {
			t.Fatalf("plain Text bypassed Number checking: bytes=%d error=%v", len(raw), err)
		}
	}
}

// expectedParameter substitutes only the independent graph's producer identity
// into the captured Apple state template. It never derives a wrapper or range
// from the consumer parameter being checked.
func (fixture p15RP11WireCase) expectedParameter(t *testing.T, states p15RP11AppleStates, actions []ShortcutAction) (any, error) {
	t.Helper()
	sample, ok := states[fixture.state][fixture.nativeSample]
	if !ok {
		return nil, fmt.Errorf("missing independent native sample")
	}
	if fixture.reference == "" {
		if reflect.TypeOf(sample) != reflect.TypeOf(fixture.literal) {
			return nil, fmt.Errorf("literal kind disagrees with native state")
		}
		return fixture.literal, nil
	}
	expected, ok := p15RP11Clone(t, sample).(map[string]any)
	if !ok {
		return nil, fmt.Errorf("reference sample is not a native state")
	}
	value, ok := expected["Value"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("reference state has no Value")
	}
	token := value
	if fixture.state == "WFVariableStringParameterState" {
		attachments, ok := value["attachmentsByRange"].(map[string]any)
		if !ok || len(attachments) != 1 {
			return nil, fmt.Errorf("native text sample lost its unique attachment")
		}
		for _, raw := range attachments {
			token, ok = raw.(map[string]any)
		}
		if !ok {
			return nil, fmt.Errorf("native text sample token malformed")
		}
		if fixture.contextual {
			value["string"] = "😀:\uFFFC:é"
			value["attachmentsByRange"] = map[string]any{"{3, 1}": token}
		}
	}
	if fixture.reference == "variable" {
		var name string
		stores := 0
		for _, action := range actions {
			if action.WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" {
				stores++
				name, _ = action.WFWorkflowActionParameters["WFVariableName"].(string)
			}
		}
		if stores != 1 || name == "" {
			return nil, fmt.Errorf("mutable source lost its unique native store")
		}
		token["VariableName"] = name
	} else {
		if len(actions) != 2 || actions[0].WFWorkflowActionIdentifier != "is.workflow.actions."+fixture.reference {
			return nil, fmt.Errorf("action-output producer or once-only order changed")
		}
		producer := actions[0].WFWorkflowActionParameters
		producerKey := "WFNumberActionNumber"
		if fixture.reference == "gettext" {
			producerKey = "WFTextActionText"
		}
		if !reflect.DeepEqual(p15RP11Clone(t, producer[producerKey]), fixture.literal) {
			return nil, fmt.Errorf("source producer literal changed")
		}
		uuid, _ := producer["UUID"].(string)
		if uuid == "" {
			return nil, fmt.Errorf("producer identity is empty")
		}
		name, _ := producer["CustomOutputName"].(string)
		if name == "" {
			if fixture.reference == "number" {
				name = "Number"
			} else {
				name = "Text"
			}
		}
		token["OutputUUID"], token["OutputName"] = uuid, name
	}
	return expected, nil
}

func p15RP11CheckWire(t *testing.T, fixture p15RP11WireCase, states p15RP11AppleStates, actions []ShortcutAction) error {
	t.Helper()
	if len(actions) == 0 || actions[len(actions)-1].WFWorkflowActionIdentifier != fixture.action {
		return fmt.Errorf("consumer action identity/order changed")
	}
	if fixture.reference == "" && len(actions) != 1 {
		return fmt.Errorf("literal emitted unexpected producer actions")
	}
	want, err := fixture.expectedParameter(t, states, actions)
	if err != nil {
		return err
	}
	got := p15RP11Clone(t, actions[len(actions)-1].WFWorkflowActionParameters[fixture.key])
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("parameter %s disagrees with captured native state: got=%#v want=%#v", fixture.key, got, want)
	}
	return nil
}

func TestP15RP11NativeParameterDifferential(t *testing.T) {
	states := p15RP11ReadAppleStates(t)
	for _, fixture := range p15RP11WireCases() {
		t.Run(fixture.name, func(t *testing.T) {
			actions := compileDebugFixture(t, fixture.source, false)
			if err := p15RP11CheckWire(t, fixture, states, actions); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15RP11NativeOracleRejectsFaults(t *testing.T) {
	states := p15RP11ReadAppleStates(t)
	for _, fixture := range p15RP11WireCases() {
		actions := compileDebugFixture(t, fixture.source, false)
		if err := p15RP11CheckWire(t, fixture, states, actions); err != nil {
			t.Fatal(err)
		}
		faults := []string{"wrong_action", "missing_parameter", "blanket_float", "blanket_attachment"}
		if fixture.reference != "" {
			faults = append(faults, "wrong_reference")
		}
		if fixture.reference == "number" || fixture.reference == "gettext" {
			faults = append(faults, "producer_literal")
		}
		if fixture.contextual {
			faults = append(faults, "utf8_range", "lost_context")
		}
		for _, fault := range faults {
			t.Run(fixture.name+"/"+fault, func(t *testing.T) {
				raw, err := json.Marshal(actions)
				if err != nil {
					t.Fatal(err)
				}
				var broken []ShortcutAction
				if err := json.Unmarshal(raw, &broken); err != nil {
					t.Fatal(err)
				}
				last := &broken[len(broken)-1]
				switch fault {
				case "wrong_action":
					last.WFWorkflowActionIdentifier = "is.workflow.actions.nothing"
				case "missing_parameter":
					delete(last.WFWorkflowActionParameters, fixture.key)
				case "blanket_float":
					last.WFWorkflowActionParameters[fixture.key] = float64(12345)
				case "blanket_attachment":
					last.WFWorkflowActionParameters[fixture.key] = map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "Variable", "VariableName": "RP11_FOREIGN"}}
				case "producer_literal":
					key := "WFNumberActionNumber"
					if fixture.reference == "gettext" {
						key = "WFTextActionText"
					}
					broken[0].WFWorkflowActionParameters[key] = "RP11_WRONG_LITERAL"
				case "wrong_reference":
					wrapper := last.WFWorkflowActionParameters[fixture.key].(map[string]any)
					value := wrapper["Value"].(map[string]any)
					token := value
					if attachments, ok := value["attachmentsByRange"].(map[string]any); ok {
						for _, raw := range attachments {
							token = raw.(map[string]any)
						}
					}
					if fixture.reference == "variable" {
						token["VariableName"] = "RP11_FOREIGN"
					} else {
						token["OutputUUID"] = "RP11_FOREIGN"
					}
				case "utf8_range":
					value := last.WFWorkflowActionParameters[fixture.key].(map[string]any)["Value"].(map[string]any)
					attachments := value["attachmentsByRange"].(map[string]any)
					value["attachmentsByRange"] = map[string]any{"{5, 1}": attachments["{3, 1}"]}
				case "lost_context":
					last.WFWorkflowActionParameters[fixture.key].(map[string]any)["Value"].(map[string]any)["string"] = "\uFFFC"
				}
				if p15RP11CheckWire(t, fixture, states, broken) == nil {
					t.Fatalf("%s corruption passed the independent native oracle", fault)
				}
			})
		}
	}
}
