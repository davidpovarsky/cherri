package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func p15FN01Source(first, factor string) string {
	return fmt.Sprintf("function multiply(value: Number, factor: Number) -> Number {\n return value * factor\n}\nlet result = multiply(%s, factor: %s)\nlet continuation = result + 1\nsetClipboard(f\"{result}:{continuation}\")\n", first, factor)
}

const p15FN02Source = `function defaults(flag: Bool = true, count: Number = 9, label: Text = "def") -> Text {
 return f"{flag}:{count}:{label}"
}
let explicit = defaults(false, count: 0, label: "")
let omitted = defaults()
let mixed = defaults(flag: false)
setClipboard(f"{explicit}|{omitted}|{mixed}")
`

func p15FN03Source(argument string) string {
	return `function fact(n: Number) -> Number {
 if n <= 1 { return 1 }
 let previous = fact(n - 1)
 return n * previous
}
let result = fact(` + argument + `)
setClipboard(f"{result}")
`
}

func p15FunctionTypedProgram(t *testing.T, source string, expressions map[string]string) *syntax.Program {
	t.Helper()
	program, errors := p15ParseProgram(source)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	analyzer := analysis.NewAnalyzer(schema.DefaultRegistry())
	analyzer.Analyze(program)
	if diagnostics := analyzer.Diagnostics(); len(diagnostics) != 0 {
		t.Fatalf("valid authored function rejected: %#v", diagnostics)
	}
	seen := map[string]int{}
	for expression, inferred := range analyzer.ExprTypes() {
		span := expression.NodeSpan()
		if span.Start.ByteOffset < 0 || span.End.ByteOffset > len(source) {
			t.Fatal("type map source span escaped source")
		}
		text := source[span.Start.ByteOffset:span.End.ByteOffset]
		if want, relevant := expressions[text]; relevant {
			seen[text]++
			if fmt.Sprint(inferred) != want {
				t.Fatalf("%q inferred %v, want %s", text, inferred, want)
			}
		}
	}
	for text := range expressions {
		if seen[text] != 1 {
			t.Fatalf("exact AST expression %q has %d typed observations", text, seen[text])
		}
	}
	return program
}

func p15FunctionRejects(t *testing.T, source, argument, code, message string) {
	t.Helper()
	diagnostics := p15TypeDiagnostics(t, source)
	if len(diagnostics) != 1 {
		t.Fatalf("want one precise rejection, got %#v", diagnostics)
	}
	if err := p15ExactLiteralDiagnostic(diagnostics, source, argument, code); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diagnostics[0].Message, message) {
		t.Fatalf("missing diagnostic explanation %q: %#v", message, diagnostics)
	}
	data, err := CompileSourceToPlist("p15-function-negative.cherri", source)
	if err == nil || len(data) != 0 || !strings.Contains(err.Error(), code) {
		t.Fatalf("invalid function reached emission: bytes=%d err=%v", len(data), err)
	}
	wrong := append([]analysis.Diagnostic(nil), diagnostics...)
	wrong[0].Code = "E_WRONG_CONTROL"
	if p15ExactLiteralDiagnostic(wrong, source, argument, code) == nil {
		t.Fatal("wrong diagnostic code accepted")
	}
	wrong = append([]analysis.Diagnostic(nil), diagnostics...)
	wrong[0].Span.Start.ByteOffset++
	if p15ExactLiteralDiagnostic(wrong, source, argument, code) == nil {
		t.Fatal("wrong diagnostic span accepted")
	}
}

func p15FunctionCaptureError(diagnostics []analysis.Diagnostic, source string) error {
	start := strings.Index(source, "return outer")
	if start < 0 || len(diagnostics) != 2 {
		return fmt.Errorf("want the exact capture and consequent return-type diagnostics")
	}
	want := map[string]struct {
		start, end int
		message    string
	}{
		analysis.CodeUnknownName: {start + len("return "), start + len("return outer"), `cannot capture outer variable "outer" inside function "bad"; pass it as a parameter`},
		analysis.CodeReturnType:  {start, start + len("return outer"), "return value of type Unknown does not match expected return type Number"},
	}
	for _, diagnostic := range diagnostics {
		expected, found := want[diagnostic.Code]
		if !found || diagnostic.Severity != analysis.SeverityError || diagnostic.Span.Start.ByteOffset != expected.start || diagnostic.Span.End.ByteOffset != expected.end || diagnostic.Message != expected.message {
			return fmt.Errorf("unexpected capture diagnostic: %#v", diagnostic)
		}
		delete(want, diagnostic.Code)
	}
	if len(want) != 0 {
		return fmt.Errorf("missing capture diagnostic")
	}
	return nil
}

func TestP15FN01FunctionTyping(t *testing.T) {
	program := p15FunctionTypedProgram(t, p15FN01Source("7", "3"), map[string]string{
		"value * factor": "Number", "multiply(7, factor: 3)": "Number", "result + 1": "Number",
	})
	if len(program.Declarations) != 1 || len(program.Statements) != 3 {
		t.Fatal("authored function/caller AST was not retained")
	}
	function, ok := program.Declarations[0].(*syntax.FunctionDecl)
	if !ok || function.Name != "multiply" || len(function.Parameters) != 2 || len(function.Body.Statements) != 1 {
		t.Fatal("missing AST-defined function and parameters")
	}
	if _, localReturn := function.Body.Statements[0].(*syntax.ReturnStmt); !localReturn {
		t.Fatal("function body lost its local return")
	}
	for _, tc := range []struct{ name, source, argument, code, message string }{
		{"wrong_return", `function wrong() -> Number { return "text" }
let result = wrong()`, `return "text"`, analysis.CodeReturnType, "does not match expected return type Number"},
		{"wrong_parameter", `function multiply(value: Number, factor: Number) -> Number { return value * factor }
let result = multiply(7, factor: "three")`, `"three"`, analysis.CodeArgumentType, "Number"},
	} {
		t.Run(tc.name, func(t *testing.T) { p15FunctionRejects(t, tc.source, tc.argument, tc.code, tc.message) })
	}
	if err := p15FN01NativeError(p15CompiledActions(t, p15FN01Source("7", "3")), "7", "3"); err != nil {
		t.Fatalf("valid function failed after diagnostic rejection: %v", err)
	}
}

func p15FunctionPayloadRow(t *testing.T, actions []map[string]any, index int, key string) map[string]any {
	t.Helper()
	outer, _ := p15C01Params(actions[index])["WFItems"].(map[string]any)
	inner, _ := outer["Value"].(map[string]any)
	rows, _ := inner["WFDictionaryFieldValueItems"].([]any)
	for _, raw := range rows {
		row, _ := raw.(map[string]any)
		name, err := p15LiteralText(row["WFKey"])
		if err == nil && name == key {
			return row
		}
	}
	t.Fatalf("native payload field %s absent at %d", key, index)
	return nil
}

func TestP15FN01ReturnAndContinuationNative(t *testing.T) {
	for _, tc := range []struct{ name, first, factor string }{{"ordinary", "7", "3"}, {"zero", "0", "4"}, {"fractional", "2.5", "2"}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := p15FN01NativeError(p15CompiledActions(t, p15FN01Source(tc.first, tc.factor)), tc.first, tc.factor); err != nil {
				t.Fatal(err)
			}
		})
	}
	actions := p15CompiledActions(t, p15FN01Source("7", "3"))
	for name, mutate := range map[string]func([]map[string]any){
		"return_becomes_global_exit": func(a []map[string]any) { a[26]["WFWorkflowActionIdentifier"] = "is.workflow.actions.exit" },
		"wrong_return_edge": func(a []map[string]any) {
			p15C01Params(a[26])["WFOutput"] = p15FunctionText("\ufffc", p15C01Params(a[22])["UUID"].(string))
		},
		"wrong_function_target": func(a []map[string]any) {
			p15FunctionPayloadRow(t, a, 30, "function")["WFValue"] = p15FunctionText("other")
		},
		"wrong_argument_type":  func(a []map[string]any) { p15FunctionPayloadRow(t, a, 30, "arg_1")["WFItemType"] = 0 },
		"wrong_argument_value": func(a []map[string]any) { p15FunctionPayloadRow(t, a, 30, "arg_1")["WFValue"] = p15FunctionText("4") },
		"call_is_not_self":     func(a []map[string]any) { p15C01Params(a[31])["WFWorkflow"].(map[string]any)["isSelf"] = false },
		"call_wrong_payload": func(a []map[string]any) {
			p15C01Params(a[31])["WFInput"] = p15FunctionEdge(p15C01Params(a[25])["UUID"].(string))
		},
		"return_type_cast_lost":       func(a []map[string]any) { a[32]["WFWorkflowActionIdentifier"] = "is.workflow.actions.gettext" },
		"caller_continuation_wrong":   func(a []map[string]any) { p15C01Params(a[33])["WFMathOperation"] = "-" },
		"caller_observation_detached": func(a []map[string]any) { p15C01Params(a[34])["WFInput"] = p15FunctionText("21:22") },
		"caller_entered_as_function":  func(a []map[string]any) { p15C01Params(a[29])["WFControlFlowMode"] = 2 },
		"envelope_magic_wrong":        func(a []map[string]any) { p15C01Params(a[7])["WFConditionalActionString"] = "wrong" },
		"duplicate_identity":          func(a []map[string]any) { p15C01Params(a[33])["UUID"] = p15C01Params(a[32])["UUID"] },
	} {
		t.Run(name, func(t *testing.T) {
			changed := p15C01Clone(t, actions)
			mutate(changed)
			if p15FN01NativeError(changed, "7", "3") == nil {
				t.Fatal("native corruption accepted")
			}
		})
	}
}

func TestP15FN02DefaultTyping(t *testing.T) {
	program := p15FunctionTypedProgram(t, p15FN02Source, map[string]string{
		"defaults(false, count: 0, label: \"\")": "Text", "defaults()": "Text", "defaults(flag: false)": "Text",
		"true": "Bool", "9": "Number", `"def"`: "Text",
	})
	function, ok := program.Declarations[0].(*syntax.FunctionDecl)
	if !ok || len(function.Parameters) != 3 {
		t.Fatal("missing typed defaults")
	}
	for i, want := range []string{"Bool", "Number", "Text"} {
		parameter := function.Parameters[i]
		annotation, ok := parameter.TypeExpr.(*syntax.NamedTypeAnnotation)
		if !ok || annotation.Name != want || parameter.DefaultExpr == nil {
			t.Fatalf("parameter %d lost explicit type/default", i)
		}
	}
	for _, tc := range []struct{ name, source, argument string }{
		{"wrong_bool_default", `function bad(flag: Bool = "true") -> Bool { return flag }
let value = bad()`, `"true"`},
		{"wrong_number_default", `function bad(count: Number = "9") -> Number { return count }
let value = bad()`, `"9"`},
		{"wrong_text_default", `function bad(label: Text = false) -> Text { return label }
let value = bad()`, `false`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p15FunctionRejects(t, tc.source, tc.argument, analysis.CodeArgumentType, "default value of type")
		})
	}
	if err := p15FN02NativeError(p15CompiledActions(t, p15FN02Source)); err != nil {
		t.Fatalf("valid defaults failed after invalid input: %v", err)
	}
}

func TestP15FN02PresenceDefaultsNative(t *testing.T) {
	actions := p15CompiledActions(t, p15FN02Source)
	if err := p15FN02NativeError(actions); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func([]map[string]any){
		"false_presence_lost": func(a []map[string]any) {
			p15FunctionPayloadRow(t, a, 46, "has_arg_0")["WFValue"] = p15FunctionText("0")
		},
		"zero_presence_lost": func(a []map[string]any) {
			p15FunctionPayloadRow(t, a, 46, "has_arg_1")["WFValue"] = p15FunctionText("0")
		},
		"empty_presence_lost": func(a []map[string]any) {
			p15FunctionPayloadRow(t, a, 46, "has_arg_2")["WFValue"] = p15FunctionText("0")
		},
		"omitted_argument_invented": func(a []map[string]any) {
			p15FunctionPayloadRow(t, a, 48, "has_arg_0")["WFValue"] = p15FunctionText("1")
		},
		"truthiness_instead_of_presence": func(a []map[string]any) { p15C01Params(a[22])["WFCondition"] = 100 },
		"presence_gets_argument":         func(a []map[string]any) { p15C01Params(a[21])["WFDictionaryKey"] = "arg_0" },
		"supplied_value_wrong_envelope": func(a []map[string]any) {
			p15C01Params(a[23])["WFInput"] = p15FunctionEdge(p15C01Params(a[4])["UUID"].(string))
		},
		"fallback_branch_reversed": func(a []map[string]any) { p15C01Params(a[24])["WFControlFlowMode"] = 2 },
		"false_cast_reads_fallback": func(a []map[string]any) {
			p15C01Params(a[27])["WFInput"] = p15FunctionEdge(p15C01Params(a[25])["UUID"].(string))
		},
		"number_default_wrong": func(a []map[string]any) { p15C01Params(a[32])["WFNumberActionNumber"] = 8 },
		"text_default_wrong":   func(a []map[string]any) { p15C01Params(a[39])["WFTextActionText"] = "wrong" },
		"body_uses_raw_argument": func(a []map[string]any) {
			p15C01Params(a[41])["WFTextActionText"] = p15FunctionText("\ufffc:\ufffc:\ufffc", p15C01Params(a[23])["UUID"].(string), p15C01Params(a[34])["UUID"].(string), p15C01Params(a[40])["UUID"].(string))
		},
		"mixed_result_substituted": func(a []map[string]any) {
			p15C01Params(a[52])["WFInput"] = p15FunctionText("\ufffc|\ufffc|\ufffc", p15C01Params(a[47])["UUID"].(string), p15C01Params(a[49])["UUID"].(string), p15C01Params(a[47])["UUID"].(string))
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := p15C01Clone(t, actions)
			mutate(changed)
			if p15FN02NativeError(changed) == nil {
				t.Fatal("presence/default corruption accepted")
			}
		})
	}
}

func TestP15FN03CaptureAndRecursionTyping(t *testing.T) {
	p15FunctionTypedProgram(t, p15FN03Source("3"), map[string]string{"n <= 1": "Bool", "n - 1": "Number", "fact(n - 1)": "Number", "n * previous": "Number", "fact(3)": "Number"})
	for _, tc := range []struct{ name, source string }{
		{"immutable_capture", `let outer = 7
function bad() -> Number { return outer }
let value = bad()`},
		{"mutable_capture", `var outer = 7
function bad() -> Number { return outer }
let value = bad()`},
		{"capture_declared_later", `function bad() -> Number { return outer }
let outer = 7
let value = bad()`},
		{"nested_capture", `let outer = 7
function bad(flag: Bool) -> Number { if flag { return outer }; return 0 }
let value = bad(true)`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := p15TypeDiagnostics(t, tc.source)
			if err := p15FunctionCaptureError(diagnostics, tc.source); err != nil {
				t.Fatalf("%v: %#v", err, diagnostics)
			}
			data, err := CompileSourceToPlist("p15-function-capture.cherri", tc.source)
			// The public compile API returns the first semantic error; the analyzer above independently requires its full pair.
			if err == nil || len(data) != 0 || !strings.Contains(err.Error(), analysis.CodeUnknownName) || !strings.Contains(err.Error(), `cannot capture outer variable "outer"`) {
				t.Fatalf("capture reached emission or lost its first diagnostic: bytes=%d err=%v", len(data), err)
			}
			for index := range diagnostics {
				wrong := append([]analysis.Diagnostic(nil), diagnostics...)
				wrong[index].Code = "E_WRONG_CONTROL"
				if p15FunctionCaptureError(wrong, tc.source) == nil {
					t.Fatal("wrong capture code accepted")
				}
				wrong = append([]analysis.Diagnostic(nil), diagnostics...)
				wrong[index].Span.Start.ByteOffset++
				if p15FunctionCaptureError(wrong, tc.source) == nil {
					t.Fatal("wrong capture span accepted")
				}
			}
			if p15FunctionCaptureError(diagnostics[:1], tc.source) == nil {
				t.Fatal("missing consequent diagnostic accepted")
			}
		})
	}
	p15FunctionTypedProgram(t, `let outer = 7
function explicit(outer: Number) -> Number { return outer }
let value = explicit(outer)
setClipboard(f"{value}")`, map[string]string{"explicit(outer)": "Number"})
	if err := p15FN03NativeError(p15CompiledActions(t, p15FN03Source("3")), "3"); err != nil {
		t.Fatalf("recursion failed after rejected captures: %v", err)
	}
}

func TestP15FN03TerminatingRecursionNative(t *testing.T) {
	for _, argument := range []string{"0", "1", "3", "5"} {
		t.Run("input_"+argument, func(t *testing.T) {
			if err := p15FN03NativeError(p15CompiledActions(t, p15FN03Source(argument)), argument); err != nil {
				t.Fatal(err)
			}
		})
	}
	actions := p15CompiledActions(t, p15FN03Source("3"))
	for name, mutate := range map[string]func([]map[string]any){
		"base_predicate_changed":     func(a []map[string]any) { p15C01Params(a[23])["WFCondition"] = 4 },
		"base_threshold_changed":     func(a []map[string]any) { p15C01Params(a[23])["WFNumberValue"] = 0 },
		"base_result_changed":        func(a []map[string]any) { p15C01Params(a[24])["WFNumberActionNumber"] = 0 },
		"base_return_lost":           func(a []map[string]any) { a[25]["WFWorkflowActionIdentifier"] = "is.workflow.actions.exit" },
		"argument_does_not_decrease": func(a []map[string]any) { p15C01Params(a[27])["WFMathOperation"] = "+" },
		"recursion_wrong_argument": func(a []map[string]any) {
			p15FunctionPayloadRow(t, a, 28, "arg_0")["WFValue"] = p15FunctionEdge(p15C01Params(a[22])["UUID"].(string))
		},
		"recursion_wrong_function": func(a []map[string]any) {
			p15FunctionPayloadRow(t, a, 28, "function")["WFValue"] = p15FunctionText("other")
		},
		"recursive_result_not_typed": func(a []map[string]any) {
			a[30]["WFWorkflowActionIdentifier"] = "is.workflow.actions.detect.dictionary"
		},
		"recursive_product_changed": func(a []map[string]any) { p15C01Params(a[31])["WFMathOperation"] = "+" },
		"return_bypasses_product": func(a []map[string]any) {
			p15C01Params(a[32])["WFOutput"] = p15FunctionText("\ufffc", p15C01Params(a[30])["UUID"].(string))
		},
		"initial_argument_changed": func(a []map[string]any) { p15FunctionPayloadRow(t, a, 36, "arg_0")["WFValue"] = p15FunctionText("4") },
		"observed_result_literal":  func(a []map[string]any) { p15C01Params(a[39])["WFInput"] = p15FunctionText("6") },
	} {
		t.Run(name, func(t *testing.T) {
			changed := p15C01Clone(t, actions)
			mutate(changed)
			if p15FN03NativeError(changed, "3") == nil {
				t.Fatal("recursive dataflow corruption accepted")
			}
		})
	}
}
