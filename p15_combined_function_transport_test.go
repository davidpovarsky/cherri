package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

// RP24 owns this joined add/continuation/recursion scenario. Semantic types,
// complete native structure and actual Apple results are separate obligations.
func TestP15RP24CombinedReturnRecursionUnit(t *testing.T) {
	var data, err = os.ReadFile(filepath.Join("tests", "language-v2", "v2-rp24-return-recursion.cherri"))
	if err != nil {
		t.Fatal(err)
	}
	var source = string(data)
	p15FunctionTypedProgram(t, source, map[string]string{
		"a + b":           "Number",
		"n <= 1":          "Bool",
		"n - 1":           "Number",
		"fact(n - 1)":     "Number",
		"n * fact(n - 1)": "Number",
		"add(2, b: 3)":    "Number",
		"sum * 2":         "Number",
		"fact(3)":         "Number",
		"recursive + 1":   "Number",
	})
	for _, tc := range []struct{ name, source string }{
		{"immutable_capture", "let outer = 7\nfunction bad() -> Number { return outer }\nlet value = bad()"},
		{"mutable_capture", "var outer = 7\nfunction bad() -> Number { return outer }\nlet value = bad()"},
		{"later_capture", "function bad() -> Number { return outer }\nlet outer = 7\nlet value = bad()"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var diagnostics = p15TypeDiagnostics(t, tc.source)
			if err := p15FunctionCaptureError(diagnostics, tc.source); err != nil {
				t.Fatal(err)
			}
			var raw, err = CompileSourceToPlist("p15-rp24-illegal-capture.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeUnknownName) || !strings.Contains(err.Error(), "cannot capture outer variable") {
				t.Fatalf("implicit main capture emitted native source or lost its causal diagnostic: bytes=%d error=%v", len(raw), err)
			}
			for i := range diagnostics {
				var changed = append([]analysis.Diagnostic(nil), diagnostics...)
				changed[i].Code = "E_UNRELATED"
				if p15FunctionCaptureError(changed, tc.source) == nil {
					t.Fatal("capture code damage accepted")
				}
				changed = append([]analysis.Diagnostic(nil), diagnostics...)
				changed[i].Span.Start.ByteOffset++
				if p15FunctionCaptureError(changed, tc.source) == nil {
					t.Fatal("capture span damage accepted")
				}
			}
			if p15FunctionCaptureError(diagnostics[:1], tc.source) == nil {
				t.Fatal("missing capture consequence accepted")
			}
		})
	}
	for _, tc := range []struct{ name, source, argument, code, message string }{
		{"wrong_argument", "function add(a:Number,b:Number)->Number{return a+b}\nlet sum=add(2,b:\"three\")", "\"three\"", analysis.CodeArgumentType, "Number"},
		{"wrong_return", "function add(a:Number,b:Number)->Number{return \"five\"}\nlet sum=add(2,b:3)", "return \"five\"", analysis.CodeReturnType, "Number"},
	} {
		t.Run(tc.name, func(t *testing.T) { p15FunctionRejects(t, tc.source, tc.argument, tc.code, tc.message) })
	}
	t.Log("RP24 semantic add/continuation/recursion and capture checks only; native graph and Apple values are not inferred.")
}

func p15RP24Graph(actions []map[string]any) (*p15FTCursor, error) {
	var c = p15FTNew(actions)
	c.header()
	c.beginFunction("add")
	c.mark("add_left_slot")
	var left = c.field("arg_0", c.envelope, "a")
	left = c.numberCast(left, "a")
	c.mark("add_right_slot")
	var right = c.field("arg_1", c.envelope, "b")
	right = c.numberCast(right, "b")
	c.mark("add_operation")
	var sum = c.math("+", left, p15SCAttachment(right), "")
	c.mark("add_return")
	c.output(sum)
	c.beginFunction("fact")
	c.mark("fact_slot")
	var n = c.field("arg_0", c.envelope, "n")
	n = c.numberCast(n, "n")
	c.mark("fact_base_guard")
	var base = c.condition(n, 1, "WFNumberValue", float64(1))
	c.mark("fact_base_value")
	var one = c.number(uint64(1), "Number", true)
	c.mark("fact_base_return")
	c.output(one)
	c.end(base, "", false)
	c.mark("recursive_decrement")
	var previous = c.math("-", n, uint64(1), "")
	c.mark("recursive_payload")
	var payload = c.payload("fact", p15FTArgument{true, p15ContainerReference(previous)})
	c.mark("recursive_call")
	var recursive = c.call(payload, "factResult")
	c.mark("recursive_cast")
	recursive = c.numberCast(recursive, "factResult")
	c.mark("recursive_product")
	var product = c.math("×", n, p15SCAttachment(recursive), "")
	c.mark("fact_return")
	c.output(product)
	c.caller()
	c.mark("main_add_payload")
	payload = c.payload("add", p15FTArgument{true, p15ContainerNumber("2")}, p15FTArgument{true, p15ContainerNumber("3")})
	c.mark("main_add_call")
	sum = c.call(payload, "sum")
	c.mark("main_add_cast")
	sum = c.numberCast(sum, "sum")
	c.mark("continuation_product")
	var doubled = c.math("×", sum, uint64(2), "doubled")
	c.mark("main_fact_payload")
	payload = c.payload("fact", p15FTArgument{true, p15ContainerNumber("3")})
	c.mark("main_fact_call")
	recursive = c.call(payload, "recursive")
	recursive = c.numberCast(recursive, "recursive")
	c.mark("continuation_add")
	var continued = c.math("+", recursive, uint64(1), "continued")
	c.mark("observation")
	var text = c.take("gettext", "Text", map[string]any{"WFTextActionText": p15SCToken("\ufffc|\ufffc|\ufffc", map[string]any{"{0, 1}": doubled, "{2, 1}": recursive, "{4, 1}": continued})}, true)
	c.mark("main_return")
	c.output(text)
	return c, c.finish()
}
func TestP15RP24CombinedReturnRecursionNative(t *testing.T) {
	var source, err = os.ReadFile(filepath.Join("tests", "language-v2", "v2-rp24-return-recursion.cherri"))
	if err != nil {
		t.Fatal(err)
	}
	var actions = p15FTCompile(t, string(source))
	var view, graphErr = p15RP24Graph(actions)
	if graphErr != nil {
		t.Fatal(graphErr)
	}
	var verify = func(a []map[string]any) error { _, err := p15RP24Graph(a); return err }
	p15FTCommonControls(t, actions, verify)
	for _, fault := range []string{"named_right_slot", "add_operator", "add_return", "base_predicate", "base_threshold", "base_result", "base_return", "decrement_operator", "recursive_argument", "recursive_callee", "recursive_self_target", "recursive_cast", "recursive_product", "recursive_return", "main_first_argument", "main_named_argument", "main_argument_presence", "main_callee", "main_number_cast", "continuation_product", "initial_recursion_argument", "continuation_add", "final_observation", "final_return", "authored_name_reconciled"} {
		t.Run("oracle/"+fault, func(t *testing.T) {
			var changed = p15SCClone(t, actions)
			switch fault {
			case "named_right_slot":
				p15SCParams(changed[view.points["add_right_slot"]])["WFDictionaryKey"] = "arg_0"
			case "add_operator":
				p15SCParams(changed[view.points["add_operation"]])["WFMathOperation"] = "×"
			case "add_return":
				p15SCParams(changed[view.points["add_return"]])["WFOutput"] = p15ContainerText("5")
			case "base_predicate":
				p15SCParams(changed[view.points["fact_base_guard"]])["WFCondition"] = uint64(4)
			case "base_threshold":
				p15SCParams(changed[view.points["fact_base_guard"]])["WFNumberValue"] = float64(0)
			case "base_result":
				p15SCParams(changed[view.points["fact_base_value"]])["WFNumberActionNumber"] = uint64(0)
			case "base_return":
				changed[view.points["fact_base_return"]]["WFWorkflowActionIdentifier"] = "is.workflow.actions.exit"
			case "decrement_operator":
				p15SCParams(changed[view.points["recursive_decrement"]])["WFMathOperation"] = "+"
			case "recursive_argument":
				p15FN05Fields(t, changed, view.points["recursive_payload"])["arg_0"]["WFValue"] = p15SCAttachment(map[string]any{"Type": "ActionOutput", "OutputUUID": p15SCParams(changed[view.points["fact_slot"]+1])["UUID"], "OutputName": "n"})
			case "recursive_callee":
				p15FN05Fields(t, changed, view.points["recursive_payload"])["function"]["WFValue"] = p15ContainerText("add")
			case "recursive_self_target":
				p15SCParams(changed[view.points["recursive_call"]])["WFWorkflow"].(map[string]any)["isSelf"] = false
			case "recursive_cast":
				changed[view.points["recursive_cast"]]["WFWorkflowActionIdentifier"] = "is.workflow.actions.detect.dictionary"
			case "recursive_product":
				p15SCParams(changed[view.points["recursive_product"]])["WFMathOperation"] = "+"
			case "recursive_return":
				p15SCParams(changed[view.points["fact_return"]])["WFOutput"] = p15SCRefToken(map[string]any{"Type": "ActionOutput", "OutputUUID": p15SCParams(changed[view.points["recursive_cast"]])["UUID"], "OutputName": "factResult"})
			case "main_first_argument":
				p15FN05Fields(t, changed, view.points["main_add_payload"])["arg_0"]["WFValue"] = p15ContainerText("4")
			case "main_named_argument":
				p15FN05Fields(t, changed, view.points["main_add_payload"])["arg_1"]["WFValue"] = p15ContainerText("4")
			case "main_argument_presence":
				p15FN05Fields(t, changed, view.points["main_add_payload"])["has_arg_1"]["WFValue"] = p15ContainerText("0")
			case "main_callee":
				p15FN05Fields(t, changed, view.points["main_add_payload"])["function"]["WFValue"] = p15ContainerText("fact")
			case "main_number_cast":
				changed[view.points["main_add_cast"]]["WFWorkflowActionIdentifier"] = "is.workflow.actions.detect.dictionary"
			case "continuation_product":
				p15SCParams(changed[view.points["continuation_product"]])["WFMathOperation"] = "+"
			case "initial_recursion_argument":
				p15FN05Fields(t, changed, view.points["main_fact_payload"])["arg_0"]["WFValue"] = p15ContainerText("4")
			case "continuation_add":
				p15SCParams(changed[view.points["continuation_add"]])["WFMathOperand"] = uint64(2)
			case "final_observation":
				p15SCParams(changed[view.points["observation"]])["WFTextActionText"] = p15ContainerText("10|6|7")
			case "final_return":
				p15SCParams(changed[view.points["main_return"]])["WFOutput"] = p15ContainerText("10|6|7")
			case "authored_name_reconciled":
				for _, action := range changed {
					p15FTRewrite(action, "CustomOutputName", "doubled", "other")
					p15FTRewrite(action, "OutputName", "doubled", "other")
				}
			}
			if verify(changed) == nil {
				t.Fatal("combined return/continuation/recursion corruption accepted")
			}
		})
	}
	t.Logf("RP24 complete typed native graph and %d actual actions; expected source arithmetic 10 and factorial(3)=6 is not an observed Apple result", len(actions))
}
