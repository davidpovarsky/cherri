package main

import (
	"encoding/json"
	"fmt"
	"github.com/electrikmilk/cherri/internal/language/backend"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

const p15T05NullCode = "E_NATIVE_JSON_NULL"

func p15T05NullDiagnostic(diags []analysis.Diagnostic, source string) error {
	start := strings.LastIndex(source, "null")
	matches := 0
	for _, diag := range diags {
		if diag.Code == p15T05NullCode && diag.Severity == analysis.SeverityError && diag.Span.Start.ByteOffset == start && diag.Span.End.ByteOffset == start+4 {
			matches++
		}
	}
	if start < 0 || matches != 1 {
		return fmt.Errorf("expected one exact authored-null diagnostic at %d, got %+v", start, diags)
	}
	return nil
}

func TestP15T05AuthoredNullRejected(t *testing.T) {
	cases := []struct{ name, src string }{
		{"binding", `let value:JsonNull=null;show("end")`},
		{"map", `let value={"missing":null};show(value)`},
		{"list", `let value=[null];show(value)`},
		{"nested", `let value={"nested":[null]};show(value)`},
		{"yield", `let value=if true {yield null}else{yield none};show(value)`},
		{"default", `function identity(value:JsonNull = null)->JsonNull{return value}`},
		{"return", `function produce()->JsonNull{return null};produce()`},
		{"interpolation", `let value=f"prefix{null}";show(value)`},
		{"menu", `let value=menu "Choose" {case null {yield "x"}};show(value)`},
		{"after_raw", `native.action("com.example.p15.raw", {"value":0});let value:JsonNull=null`},
		{"nested_call_in_raw", `native.action("com.example.p15.raw", {"value":text(null)})`},
		{"control_in_raw", `native.action("com.example.p15.raw", {"value":if true {yield null}else{yield none}})`},
		{"scoped_call_in_raw", `native.action("com.example.p15.raw", {"value":measure("probe"){yield null}})`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			switch tc.name {
			case "binding", "default", "return", "after_raw":
				// Preserve the exact original specimen and require its complete
				// internal carrier graph. No native JSON-null export is granted.
				p15CheckTaggedInternalNull(t, tc.name, tc.src)
				return
			}
			var expected *p15TaggedBoundaryCase
			for _, boundary := range p15TaggedNullBoundaryCases() {
				if boundary.name == tc.name {
					copy := boundary
					expected = &copy
				}
			}
			if expected == nil || expected.source != tc.src {
				t.Fatal("original negative source is not exactly enrolled")
			}
			p15CheckTaggedBoundary(t, *expected)
			wrong := append([]analysis.Diagnostic(nil), p15TypeDiagnostics(t, tc.src)...)
			for i := range wrong {
				wrong[i].Code = "E_UNRELATED"
			}
			if p15TaggedExactBoundary(wrong, *expected) == nil {
				t.Error("wrong diagnostic satisfied null consumer boundary")
			}
		})
	}
}

type p15T05Argument struct {
	key   string
	kind  backend.SemanticValueKind
	value any
}
type p15T05PresenceCase struct {
	name, source, definition, identifier string
	args                                 []p15T05Argument
	params                               map[string]any
}

func p15T05PresenceCases() []p15T05PresenceCase {
	body := p15T05Argument{"WFNotificationActionBody", backend.ValString, "body"}
	number := p15T05Argument{"WFNumber", backend.ValInt, int64(1)}
	return []p15T05PresenceCase{
		{"omitted", `showNotification("body")`, "showNotification", "is.workflow.actions.notification", []p15T05Argument{body}, map[string]any{"WFNotificationActionBody": "body"}},
		{"false_empty", `showNotification("body",title:"",playSound:false)`, "showNotification", "is.workflow.actions.notification", []p15T05Argument{body, {"WFNotificationActionTitle", backend.ValString, ""}, {"WFNotificationActionSound", backend.ValBool, false}}, map[string]any{"WFNotificationActionBody": "body", "WFNotificationActionTitle": "", "WFNotificationActionSound": false}},
		{"none", `showNotification("body",title:none,playSound:none)`, "showNotification", "is.workflow.actions.notification", []p15T05Argument{body, {"WFNotificationActionTitle", backend.ValNil, nil}, {"WFNotificationActionSound", backend.ValNil, nil}}, map[string]any{"WFNotificationActionBody": "body"}},
		{"zero", `formatNumber(1,decimalPlaces:0)`, "formatNumber", "is.workflow.actions.format.number", []p15T05Argument{number, {"WFNumberFormatDecimalPlaces", backend.ValInt, int64(0)}}, map[string]any{"WFNumber": uint64(1), "WFNumberFormatDecimalPlaces": uint64(0)}},
		{"default", `formatNumber(1)`, "formatNumber", "is.workflow.actions.format.number", []p15T05Argument{number}, map[string]any{"WFNumber": uint64(1)}},
		{"none_number", `formatNumber(1,decimalPlaces:none)`, "formatNumber", "is.workflow.actions.format.number", []p15T05Argument{number, {"WFNumberFormatDecimalPlaces", backend.ValNil, nil}}, map[string]any{"WFNumber": uint64(1)}},
	}
}

func p15T05CallPresence(call backend.ResolvedCall, tc p15T05PresenceCase) error {
	if call.DefinitionID != tc.definition || call.AppleIdentifier != tc.identifier || len(call.Arguments) != len(tc.args) {
		return fmt.Errorf("call identity/presence changed: %+v", call)
	}
	for index, want := range tc.args {
		actual := call.Arguments[index]
		if actual.ParameterID != want.key || actual.Position != index || actual.Omitted || actual.Value.Omitted || actual.Value.Kind != want.kind {
			return fmt.Errorf("explicit argument identity/kind/presence changed: %+v", actual)
		}
		var value any
		switch want.kind {
		case backend.ValString:
			value = actual.Value.StrVal
		case backend.ValBool:
			value = actual.Value.BoolVal
		case backend.ValInt:
			value = actual.Value.IntVal
		case backend.ValNil:
			value = nil
		default:
			return fmt.Errorf("unsupported synthetic expectation")
		}
		if !reflect.DeepEqual(value, want.value) {
			return fmt.Errorf("explicit argument value changed: %v != %v", value, want.value)
		}
	}
	return nil
}

func TestP15T05PresenceBinding(t *testing.T) {
	for _, tc := range p15T05PresenceCases() {
		t.Run(tc.name, func(t *testing.T) {
			var calls []backend.ResolvedCall
			previous := backendEmitHook
			backendEmitHook = func(c backend.ResolvedCall) error {
				copy := c
				copy.Arguments = append([]backend.CallArgument(nil), c.Arguments...)
				calls = append(calls, copy)
				return nil
			}
			defer func() { backendEmitHook = previous }()
			compileDebugFixture(t, tc.source, false)
			if len(calls) != 1 {
				t.Fatalf("expected one actual canonical call, got %d", len(calls))
			}
			if err := p15T05CallPresence(calls[0], tc); err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"dropped_argument", "omitted_flag", "changed_kind"} {
				broken := calls[0]
				broken.Arguments = append([]backend.CallArgument(nil), calls[0].Arguments...)
				last := len(broken.Arguments) - 1
				switch fault {
				case "dropped_argument":
					broken.Arguments = broken.Arguments[:last]
				case "omitted_flag":
					broken.Arguments[last].Omitted = true
				case "changed_kind":
					broken.Arguments[last].Value.Kind = backend.ValRawNative
				}
				if p15T05CallPresence(broken, tc) == nil {
					t.Fatal("presence oracle accepted " + fault)
				}
			}
		})
	}
	for _, src := range []string{`let value:Bool?=false;show(value)`, `let value:Number?=0;show(value)`, `let value:Text?="";show(value)`, `var value:Text?=none;show("end")`, `let value=menu "Choose" {case none {yield "x"}};show(value)`} {
		p15T08Analyzer(t, src)
		compileDebugFixture(t, src, false)
	}
}

func p15T05FlatWire(actions []ShortcutAction, tc p15T05PresenceCase) error {
	if err := p15M02MainVoidDebugError(actions); err != nil {
		return err
	}
	if len(actions) != 2 || actions[0].WFWorkflowActionIdentifier != tc.identifier || !reflect.DeepEqual(actions[0].WFWorkflowActionParameters, tc.params) {
		return fmt.Errorf("native explicit values or absent fields changed: %+v", actions)
	}
	return nil
}

func p15T05EmptyContainerWire(actions []ShortcutAction) error {
	if err := p15M02MainVoidDebugError(actions); err != nil {
		return err
	}
	if len(actions) != 3 || actions[0].WFWorkflowActionIdentifier != "is.workflow.actions.dictionary" || actions[1].WFWorkflowActionIdentifier != "is.workflow.actions.showresult" {
		return fmt.Errorf("container producers changed")
	}
	text := func(s string) any {
		return map[string]any{"WFSerializationType": "WFTextTokenString", "Value": map[string]any{"string": s}}
	}
	expected := map[string]any{"WFSerializationType": "WFDictionaryFieldValue", "Value": map[string]any{"WFDictionaryFieldValueItems": []any{
		map[string]any{"WFKey": text("a"), "WFItemType": uint64(2), "WFValue": map[string]any{"WFSerializationType": "WFArrayParameterState", "Value": []any{}}},
		map[string]any{"WFKey": text("d"), "WFItemType": uint64(1), "WFValue": map[string]any{"WFSerializationType": "WFDictionaryFieldValue", "Value": map[string]any{"WFDictionaryFieldValueItems": []any{}}}},
		map[string]any{"WFKey": text("f"), "WFItemType": uint64(4), "WFValue": map[string]any{"WFSerializationType": "WFNumberSubstitutableState", "Value": false}},
		map[string]any{"WFKey": text("s"), "WFItemType": uint64(0), "WFValue": text("")},
		map[string]any{"WFKey": text("z"), "WFItemType": uint64(3), "WFValue": text("0")},
	}}}
	if !reflect.DeepEqual(actions[0].WFWorkflowActionParameters["WFItems"], expected) {
		return fmt.Errorf("explicit container values/presence changed: %#v", actions[0].WFWorkflowActionParameters["WFItems"])
	}
	uuid, _ := actions[0].WFWorkflowActionParameters["UUID"].(string)
	outputs, variables := p15V02NativeReferences(actions[1].WFWorkflowActionParameters)
	if uuid == "" || !reflect.DeepEqual(outputs, []string{uuid}) || len(variables) != 0 {
		return fmt.Errorf("consumer lost exact Dictionary producer")
	}
	return nil
}

func TestP15T05PresenceNativeWire(t *testing.T) {
	raw, err := os.ReadFile("docs/apple-runtime/macos-27/builtin-actions.json")
	if err != nil {
		t.Fatal(err)
	}
	var native map[string]struct {
		ActionClass string
		Parameters  []struct {
			Key, Class        string
			DefaultValue      any
			AllowedValueTypes []int
		}
	}
	if err := json.Unmarshal(raw, &native); err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, id := range []string{"is.workflow.actions.notification", "is.workflow.actions.format.number", "is.workflow.actions.dictionary"} {
		for _, p := range native[id].Parameters {
			switch p.Key {
			case "WFNotificationActionSound":
				found[p.Key] = p.Class == "WFSwitchParameter" && p.DefaultValue == true
			case "WFNumberFormatDecimalPlaces":
				found[p.Key] = p.Class == "WFStepperParameter" && p.DefaultValue == float64(2)
			case "WFItems":
				found[p.Key] = p.Class == "WFDictionaryParameter" && reflect.DeepEqual(p.AllowedValueTypes, []int{0, 1, 2, 3, 4})
			}
		}
	}
	for _, key := range []string{"WFNotificationActionSound", "WFNumberFormatDecimalPlaces", "WFItems"} {
		if !found[key] {
			t.Fatal("independent native presence contract changed: " + key)
		}
	}
	for _, tc := range p15T05PresenceCases() {
		t.Run(tc.name, func(t *testing.T) {
			actions := compileDebugFixture(t, tc.source, false)
			if err := p15T05FlatWire(actions, tc); err != nil {
				t.Fatal(err)
			}
			for key := range tc.params {
				changed := append([]ShortcutAction(nil), actions...)
				params := map[string]any{}
				for k, v := range actions[0].WFWorkflowActionParameters {
					params[k] = v
				}
				delete(params, key)
				changed[0].WFWorkflowActionParameters = params
				if p15T05FlatWire(changed, tc) == nil {
					t.Fatal("wire oracle accepted dropped explicit field " + key)
				}
			}
			changed := append([]ShortcutAction(nil), actions...)
			params := map[string]any{}
			for k, v := range actions[0].WFWorkflowActionParameters {
				params[k] = v
			}
			params["P15UnexpectedNull"] = nil
			changed[0].WFWorkflowActionParameters = params
			if p15T05FlatWire(changed, tc) == nil {
				t.Fatal("wire oracle accepted invented null/field")
			}
		})
	}
	source := `let value={"f":false,"z":0,"s":"","a":[],"d":{}};show(value)`
	actions := compileDebugFixture(t, source, false)
	if err := p15T05EmptyContainerWire(actions); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		changed := compileDebugFixture(t, source, false)
		value := changed[0].WFWorkflowActionParameters["WFItems"].(map[string]any)["Value"].(map[string]any)
		rows := value["WFDictionaryFieldValueItems"].([]any)
		value["WFDictionaryFieldValueItems"] = append(rows[:i], rows[i+1:]...)
		if p15T05EmptyContainerWire(changed) == nil {
			t.Fatal("container oracle accepted dropped explicit value")
		}
	}
	// JSON text is preserved as Text. This is not a JsonNull native value proof.
	actions = compileDebugFixture(t, `let value=text("{\"missing\":null}");show(value)`, false)
	if err := p15M02MainVoidDebugError(actions); err != nil {
		t.Fatal(err)
	}
	if len(actions) != 3 || actions[0].WFWorkflowActionIdentifier != "is.workflow.actions.gettext" ||
		actions[1].WFWorkflowActionIdentifier != "is.workflow.actions.showresult" ||
		actions[0].WFWorkflowActionParameters["WFTextActionText"] != `{"missing":null}` {
		t.Fatal("JSON text bytes changed")
	}
	// The opaque native escape retains its existing exact, non-null payload route.
	actions = compileDebugFixture(t, `native.action("com.example.p15.presence", {"flag":false,"count":0,"empty":""})`, false)
	if err := p15M02MainVoidDebugError(actions); err != nil {
		t.Fatal(err)
	}
	if len(actions) != 2 || actions[0].WFWorkflowActionIdentifier != "com.example.p15.presence" ||
		!reflect.DeepEqual(actions[0].WFWorkflowActionParameters, map[string]any{"flag": false, "count": uint64(0), "empty": ""}) {
		t.Fatal("opaque native payload changed")
	}
}
