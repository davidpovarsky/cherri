package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

const p15NativeOperandFirst = "8556b10c-8b87-46c6-a95b-dcf849951001"
const p15NativeOperandSecond = "8556b10c-8b87-46c6-a95b-dcf849951002"

func p15NativeOperandSource() string {
	producer := func(name, uuid string) string {
		return fmt.Sprintf("let %s = native.action(\"is.workflow.actions.image.resize\", parameters: native.plist({\"UUID\":%q, \"WFImageResizeWidth\":\"100\", \"WFImage\":{\"Type\":\"Variable\", \"Variable\":{\"WFSerializationType\":\"WFTextTokenAttachment\", \"Value\":{\"Type\":\"ExtensionInput\"}}}}))\n", name, uuid)
	}
	reference := func(name string) string {
		return "native.ref(" + name + ", outputName: \"Resized Image\", aggrandizements: [{\"Type\":\"WFPropertyVariableAggrandizement\", \"PropertyName\":\"Height\", \"PropertyUserInfo\":\"WFImageHeight\"}])"
	}
	return producer("first", p15NativeOperandFirst) + producer("second", p15NativeOperandSecond) +
		"if modern: " + reference("first") + " > " + reference("second") + " { text(\"synthetic\") }"
}

func p15NativeOperandAttachment(uuid string) map[string]any {
	attachment := p12RawActionOutAttachment(uuid, "Resized Image")
	attachment["Value"].(map[string]any)["Aggrandizements"] = []any{
		map[string]any{
			"Type":             "WFPropertyVariableAggrandizement",
			"PropertyName":     "Height",
			"PropertyUserInfo": "WFImageHeight",
		},
	}
	return attachment
}

func p15NativeOperandWire(actions []ShortcutAction) error {
	if err := p15M02MainVoidDebugError(actions); err != nil {
		return err
	}
	if len(actions) != 6 {
		return fmt.Errorf("native operand action count %d", len(actions))
	}
	for i, uuid := range []string{p15NativeOperandFirst, p15NativeOperandSecond} {
		if actions[i].WFWorkflowActionIdentifier != "is.workflow.actions.image.resize" ||
			actions[i].WFWorkflowActionParameters["UUID"] != uuid {
			return fmt.Errorf("raw producer %d changed: %s %#v", i, actions[i].WFWorkflowActionIdentifier, actions[i].WFWorkflowActionParameters)
		}
	}
	start, end := actions[2], actions[4]
	p := start.WFWorkflowActionParameters
	lhs := map[string]any{"Type": "Variable", "Variable": p15NativeOperandAttachment(p15NativeOperandFirst)}
	if start.WFWorkflowActionIdentifier != "is.workflow.actions.conditional" ||
		!plistValuesStrictlyEqual(p["WFInput"], lhs) ||
		!plistValuesStrictlyEqual(p["WFNumberValue"], p15NativeOperandAttachment(p15NativeOperandSecond)) ||
		fmt.Sprint(p["WFControlFlowMode"]) != "0" || fmt.Sprint(p["WFCondition"]) != "2" ||
		p["WFConditionalActionString"] != nil || p["WFDate"] != nil ||
		end.WFWorkflowActionIdentifier != "is.workflow.actions.conditional" ||
		fmt.Sprint(end.WFWorkflowActionParameters["WFControlFlowMode"]) != "2" ||
		p["GroupingIdentifier"] != end.WFWorkflowActionParameters["GroupingIdentifier"] {
		return fmt.Errorf("raw conditional operands, kind, or control region changed")
	}
	return nil
}

func TestP15NativeReferenceOrderedOperandWire(t *testing.T) {
	source := p15NativeOperandSource()
	program, analyzer := p15T08Analyzer(t, source)
	for _, name := range []string{"first", "second"} {
		got := analyzer.ExprTypes()[p15BindingValue(t, program, name)]
		if got == nil || !got.Equals(types.AnyContent) {
			t.Fatalf("raw producer became a scalar: %v", got)
		}
	}
	refs := 0
	for expression, result := range analyzer.ExprTypes() {
		call, ok := expression.(*syntax.CallExpr)
		if !ok {
			continue
		}
		member, ok := call.Callee.(*syntax.MemberExpr)
		if !ok || member.Property != "ref" {
			continue
		}
		refs++
		if result == nil || !result.Equals(types.AnyContent) {
			t.Fatalf("native.ref gained scalar evidence: %v", result)
		}
	}
	if refs != 2 {
		t.Fatalf("expected exactly two raw reference operands, got %d", refs)
	}
	actions := compileDebugFixture(t, source, false)
	if err := p15NativeOperandWire(actions); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"property_user_info", "property_name", "extra_coercion", "producer", "condition", "rhs_missing"} {
		t.Run(fault, func(t *testing.T) {
			changed := compileDebugFixture(t, source, false)
			p := changed[2].WFWorkflowActionParameters
			value := p["WFInput"].(map[string]any)["Variable"].(map[string]any)["Value"].(map[string]any)
			aggrandizements := value["Aggrandizements"].([]any)
			property := aggrandizements[0].(map[string]any)
			switch fault {
			case "property_user_info":
				delete(property, "PropertyUserInfo")
			case "property_name":
				property["PropertyName"] = "Width"
			case "extra_coercion":
				value["Aggrandizements"] = append(aggrandizements, map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFNumberContentItem"})
			case "producer":
				value["OutputUUID"] = "wrong-producer"
			case "condition":
				p["WFCondition"] = uint64(4)
			case "rhs_missing":
				delete(p, "WFNumberValue")
			}
			if p15NativeOperandWire(changed) == nil {
				t.Fatalf("%s native corruption passed", fault)
			}
		})
	}
	svc := service.NewService(nil)
	uri := "file:///p15-native-operands.cherri"
	svc.OpenDocument(uri, 1, source)
	diagnostics, _ := svc.Analyze(uri)
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == analysis.SeverityError {
			t.Fatal(diagnostic)
		}
	}
}

func TestP15NativeReferenceOperandDoesNotProveScalar(t *testing.T) {
	for _, tc := range []struct{ name, source, argument string }{
		{"assignment", "let n: Number = native.ref(\"ShortcutInput\")", "native.ref(\"ShortcutInput\")"},
		{"alias", "let value = native.ref(\"ShortcutInput\")\nif value > 7 { text(\"x\") }", "value"},
		{"ordinary_unknown", "if run(\"P15_OPAQUE_SYNTHETIC\") > 7 { text(\"x\") }", "run(\"P15_OPAQUE_SYNTHETIC\")"},
		{"ordinary_any", "if ShortcutInput > 7 { text(\"x\") }", "ShortcutInput"},
		{"raw_plist", "if native.plist({\"Value\":7}) > 7 { text(\"x\") }", "native.plist({\"Value\":7})"},
		{"raw_action", "if native.action(\"com.example.synthetic\", parameters:{}) > 7 { text(\"x\") }", "native.action(\"com.example.synthetic\", parameters:{})"},
		{"function_lookalike", "function nativeRef(value: AnyContent) -> AnyContent { return value }\nif nativeRef(ShortcutInput) > 7 { text(\"x\") }", "nativeRef(ShortcutInput)"},
		{"custom_action_lookalike", "action nativeRef(value: AnyContent) -> AnyContent { identifier:\"native.ref\"\nprimary:\"value\"\nbindings: { value: {key: \"Value\", codec: \"text\"} } }\nif nativeRef(ShortcutInput) > 7 { text(\"x\") }", "nativeRef(ShortcutInput)"},
		{"namespace_alias", "let alias = native\nif alias.ref(\"ShortcutInput\") > 7 { text(\"x\") }", "alias.ref(\"ShortcutInput\")"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schema.ResetDefaultRegistry()
			t.Cleanup(schema.ResetDefaultRegistry)
			diagnostics := p15TypeDiagnostics(t, tc.source)
			if err := p15ExactLiteralDiagnostic(diagnostics, tc.source, tc.argument, analysis.CodeArgumentType); err != nil {
				t.Fatal(err)
			}
			raw, err := CompileSourceToPlist("p15-native-unproved.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeArgumentType) {
				t.Fatalf("unproved scalar emitted bytes=%d err=%v", len(raw), err)
			}
			svc := service.NewService(nil)
			uri := "file:///p15-native-unproved.cherri"
			svc.OpenDocument(uri, 1, tc.source)
			diagnostics, _ = svc.Analyze(uri)
			if err := p15ExactLiteralDiagnostic(diagnostics, tc.source, tc.argument, analysis.CodeArgumentType); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15NativeReferenceNamespaceIdentity(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"top_level", "let native = {\"ref\":\"not callable\"}\nif native.ref(\"ShortcutInput\") > 7 { text(\"x\") }"},
		{"parameter", "function compare(native: AnyContent) -> Bool { return native.ref(\"ShortcutInput\") > 7 }\ncompare(ShortcutInput)"},
		{"scoped", "let result = measure(\"scope\") {\nlet native = {\"ref\":\"not callable\"}\nnative.ref(\"ShortcutInput\") > 7\n}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := p15TypeDiagnostics(t, tc.source)
			if err := p15ExactLiteralDiagnostic(diagnostics, tc.source, "native.ref", analysis.CodeUnknownAction); err != nil {
				t.Fatal(err)
			}
			raw, err := CompileSourceToPlist("p15-native-shadow.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeUnknownAction) {
				t.Fatalf("shadowed native namespace emitted bytes=%d err=%v", len(raw), err)
			}
		})
	}
	// Reusing one analyzer must resolve a fresh namespace for each document.
	analyzer := analysis.NewAnalyzer(schema.DefaultRegistry())
	for _, source := range []string{p15NativeOperandSource(), "let native={\"ref\":\"shadow\"}\nif native.ref(\"ShortcutInput\") > 7 { text(\"x\") }"} {
		program, errs := p15ParseProgram(source)
		if len(errs) != 0 {
			t.Fatal(errs)
		}
		analyzer.Analyze(program)
	}
	found := false
	for _, diagnostic := range analyzer.Diagnostics() {
		if diagnostic.Code == analysis.CodeUnknownAction {
			found = true
		}
	}
	if !found {
		t.Fatal("prior document native identity leaked into shadow")
	}
}

func TestP15NativeReferenceOperandValidationStillRequired(t *testing.T) {
	for _, expression := range []string{
		"native.ref(7)",
		"native.ref({\"Type\":\"ActionOutput\"})",
		"native.ref(\"ActionOutput\")",
		"native.ref(\"Variable\")",
		"native.ref(\"UnknownKind\", uuid:\"8556b10c-8b87-46c6-a95b-dcf849951001\")",
		"native.ref(\"ShortcutInput\", aggrandizements:7)",
		"native.ref(\"ShortcutInput\", aggrandizements:[7])",
		"native.ref(native.plist({\"Type\":\"ActionOutput\"}))",
	} {
		t.Run(expression, func(t *testing.T) {
			source := "if " + expression + " > 7 { text(\"x\") }"
			svc := service.NewService(nil)
			uri := "file:///p15-native-invalid-reference.cherri"
			svc.OpenDocument(uri, 1, source)
			diagnostics, _ := svc.Analyze(uri)
			if err := p15ExactLiteralDiagnostic(diagnostics, source, expression, analysis.CodeArgumentType); err != nil {
				t.Fatalf("service accepted an unvalidated reference: %v", err)
			}
			diagnostics = p15TypeDiagnostics(t, source)
			if err := p15ExactLiteralDiagnostic(diagnostics, source, expression, analysis.CodeArgumentType); err != nil {
				t.Fatal(err)
			}
			raw, err := CompileSourceToPlist("p15-native-invalid-reference.cherri", source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeArgumentType) {
				t.Fatalf("invalid/non-reference escaped validation: bytes=%d err=%v", len(raw), err)
			}
		})
	}
}

// These controls prove only explicit reference shape. A raw UUID/name is not a
// declaration that the referenced producer or variable exists at runtime.
func TestP15NativeReferenceStaticShapeControls(t *testing.T) {
	for _, source := range []string{
		`if native.ref("ShortcutInput") > 7 { text("x") }`,
		`if native.ref("ShortcutInput", aggrandizements: native.plist(value: [])) > 7 { text("x") }`,
		`if native.ref(native.ref("ShortcutInput")) > 7 { text("x") }`,
		`if native.ref(kind: "Variable", name: "synthetic explicit name") > 7 { text("x") }`,
		`if native.ref(type: "ActionOutput", outputUUID: "8556b10c-8b87-46c6-a95b-dcf849951001") > 7 { text("x") }`,
		"let value = number(7)\nif native.ref(value) > 7 { text(\"x\") }",
		`if native.ref("ShortcutInput", aggrandizements: native.plist({"Aggrandizements": [{"Type": "OpaqueNativeType", "Extra": {"Keep": [7, false]}}]})) > 7 { text("x") }`,
	} {
		t.Run(source, func(t *testing.T) {
			p15T08Analyzer(t, source)
			if raw, err := CompileSourceToPlist("p15-native-static-shape.cherri", source); err != nil || len(raw) == 0 {
				t.Fatalf("validated explicit shape failed: bytes=%d err=%v", len(raw), err)
			}
			svc := service.NewService(nil)
			uri := "file:///p15-native-static-shape.cherri"
			svc.OpenDocument(uri, 1, source)
			diagnostics, _ := svc.Analyze(uri)
			for _, diagnostic := range diagnostics {
				if diagnostic.Severity == analysis.SeverityError {
					t.Fatal(diagnostic)
				}
			}
		})
	}
}

func TestP15NativeReferenceDynamicShapeIsNotExempt(t *testing.T) {
	for _, tc := range []struct{ prefix, expression string }{
		{"let kind = \"ShortcutInput\"\n", `native.ref(kind: kind)`},
		{"let uuid = \"8556b10c-8b87-46c6-a95b-dcf849951001\"\n", `native.ref("ActionOutput", uuid: uuid)`},
		{"let metadata = [{\"Type\": \"Opaque\"}]\n", `native.ref("ShortcutInput", aggrandizements: metadata)`},
		{"", `native.ref("ShortcutInput", property: text("Height"))`},
		{"", `native.ref("ShortcutInput", unrecognized: "ignored by raw lowerer")`},
	} {
		t.Run(tc.expression, func(t *testing.T) {
			source := tc.prefix + "if " + tc.expression + " > 7 { text(\"x\") }"
			diagnostics := p15TypeDiagnostics(t, source)
			if err := p15ExactLiteralDiagnostic(diagnostics, source, tc.expression, analysis.CodeArgumentType); err != nil {
				t.Fatal(err)
			}
			if raw, err := CompileSourceToPlist("p15-native-dynamic-shape.cherri", source); err == nil || len(raw) != 0 {
				t.Fatalf("unproven shape emitted bytes=%d err=%v", len(raw), err)
			}
			svc := service.NewService(nil)
			uri := "file:///p15-native-dynamic-shape.cherri"
			svc.OpenDocument(uri, 1, source)
			diagnostics, _ = svc.Analyze(uri)
			if err := p15ExactLiteralDiagnostic(diagnostics, source, tc.expression, analysis.CodeArgumentType); err != nil {
				t.Fatal(err)
			}
		})
	}
}
