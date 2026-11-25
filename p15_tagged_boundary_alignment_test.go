package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
)

type p15TaggedBoundaryCase struct {
	name, source, target, code, message string
}

// These are the same authored negative specimens. Their boundary is now the
// consuming native expression, rather than construction of a supported cell.
func p15TaggedListBoundaryCase(original p15NativeListAnalysisCase) p15TaggedBoundaryCase {
	result := p15TaggedBoundaryCase{name: original.name, source: original.source,
		code: analysis.CodeTargetValueShape, message: "structured return requires an explicit external value codec"}
	switch original.name {
	case "literal_rows", "unicode_prefix":
		result.target = "value"
	case "empty_row":
		result.target = "[[]]"
	case "immutable_alias", "function_parameter":
		result.target = "[row]"
	case "broad_alias":
		result.target, result.code = "row", analysis.CodeNestedNativeList
		result.message = "nested List-valued element has no verified native content-row encoding"
	case "function_result":
		result.target = "[row()]"
	case "value_if":
		result.target = "[if true {yield [1,2]} else {yield [3,4]}]"
	case "value_menu":
		result.target = "[menu(\"Pick\") {case \"A\" {yield [1,2]} case \"B\" {yield [3,4]}}]"
	case "dictionary_field_call":
		result.target = "[[1,2]]"
		result.message = "function parameter cannot erase a structured representation"
	case "dictionary_field_control":
		result.target = "if true {yield [[1,2]]} else {yield []}"
		result.message = "dynamic native value requires an explicit Dictionary-array conversion"
	case "dictionary_context_restored", "raw_context_restored":
		result.target = "[[3,4]]"
	default:
		panic("unregistered exact List boundary specimen: " + original.name)
	}
	return result
}

func p15TaggedNullBoundaryCases() []p15TaggedBoundaryCase {
	return []p15TaggedBoundaryCase{
		{"map", `let value={"missing":null};show(value)`, "value", analysis.CodeTargetValueShape, "show requires an explicit native/JSON conversion for structured values"},
		{"list", `let value=[null];show(value)`, "value", analysis.CodeTargetValueShape, "show requires an explicit native/JSON conversion for structured values"},
		{"nested", `let value={"nested":[null]};show(value)`, "value", analysis.CodeTargetValueShape, "show requires an explicit native/JSON conversion for structured values"},
		{"yield", `let value=if true {yield null}else{yield none};show(value)`, "value", analysis.CodeTargetValueShape, "show requires an explicit native/JSON conversion for structured values"},
		{"interpolation", `let value=f"prefix{null}";show(value)`, "null", analysis.CodeTargetValueShape, "structured interpolation requires an explicit text/JSON codec"},
		{"menu", `let value=menu "Choose" {case null {yield "x"}};show(value)`, "null", analysis.CodeTargetValueShape, "menu label requires an explicit text/JSON codec"},
		{"nested_call_in_raw", `native.action("com.example.p15.raw", {"value":text(null)})`, "null", analysis.CodeArgumentType, "argument for text expects Text, got JsonNull"},
		{"control_in_raw", `native.action("com.example.p15.raw", {"value":if true {yield null}else{yield none}})`, "if true {yield null}else{yield none}", analysis.CodeTargetValueShape, "raw native metadata cannot consume a private structured value"},
		{"scoped_call_in_raw", `native.action("com.example.p15.raw", {"value":measure("probe"){yield null}})`, `measure("probe"){yield null}`, analysis.CodeTargetValueShape, "raw native metadata cannot consume a private structured value"},
	}
}

func p15TaggedExactBoundary(diagnostics []analysis.Diagnostic, expected p15TaggedBoundaryCase) error {
	start := strings.LastIndex(expected.source, expected.target)
	if start < 0 || expected.target == "" || expected.code == "" || expected.message == "" {
		return fmt.Errorf("incomplete independent boundary expectation %s", expected.name)
	}
	var errors []analysis.Diagnostic
	for _, item := range diagnostics {
		if item.Severity == analysis.SeverityError {
			errors = append(errors, item)
		}
	}
	if len(errors) != 1 {
		return fmt.Errorf("expected one exact consumer diagnostic, got %+v", errors)
	}
	actual := errors[0]
	if actual.Code != expected.code || actual.Span.Start.ByteOffset != start ||
		actual.Span.End.ByteOffset != start+len(expected.target) || !strings.Contains(actual.Message, expected.message) {
		return fmt.Errorf("consumer code/span/detail changed: got %+v want %s at %d..%d (%s)", actual, expected.code, start, start+len(expected.target), expected.message)
	}
	return nil
}

func p15CheckTaggedBoundary(t *testing.T, tc p15TaggedBoundaryCase) {
	t.Helper()
	diagnostics := p15TypeDiagnostics(t, tc.source)
	if err := p15TaggedExactBoundary(diagnostics, tc); err != nil {
		t.Error("analysis:", err)
	}
	svc := service.NewService(nil)
	uri := "file:///p15-tagged-boundary-" + tc.name + ".cherri"
	svc.OpenDocument(uri, 1, tc.source)
	observed, _ := svc.Analyze(uri)
	if err := p15TaggedExactBoundary(observed, tc); err != nil {
		t.Error("service:", err)
	}
	raw, err := CompileSourceToPlist("p15-tagged-boundary-"+tc.name+".cherri", tc.source)
	if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), tc.code) || !strings.Contains(err.Error(), tc.message) {
		t.Errorf("compile did not retain the same precise boundary: bytes=%d err=%v", len(raw), err)
	}
}

func TestP15TaggedBoundaryAnalysisParity(t *testing.T) {
	for _, original := range p15NativeListAnalysisCases() {
		tc := p15TaggedListBoundaryCase(original)
		t.Run("list/"+tc.name, func(t *testing.T) { p15CheckTaggedBoundary(t, tc) })
	}
	for _, tc := range p15TaggedNullBoundaryCases() {
		t.Run("null/"+tc.name, func(t *testing.T) { p15CheckTaggedBoundary(t, tc) })
	}
	for _, tc := range []p15TaggedBoundaryCase{
		{"native_storage_assignment", "var data=[]\ndata=[[]]\ngetVariable(data)", "data=[[]]", analysis.CodeTargetValueShape, "assignment cannot change a native storage slot into private cells without an explicit conversion"},
		{"widened_alias", "let original=[[1]]\nlet alias:AnyContent=original\nshow(alias)", "alias", analysis.CodeTargetValueShape, "show requires an explicit native/JSON conversion for structured values"},
		{"raw_alias", "let original=null\nnative.action(\"com.example.raw\",{\"value\":original})", "original", analysis.CodeTargetValueShape, "raw native metadata cannot consume a private structured value"},
		{"menu_prompt", "let original=null\nlet selected=menu(original) {case \"x\" {yield 1}}\nshow(selected)", "original", analysis.CodeTargetValueShape, "menu label requires an explicit text/JSON codec"},
		{"anycontent_null", `function echo(value:Map<Text,AnyContent>)->Map<Text,AnyContent>{return value};echo({"value":null})`, `{"value":null}`, analysis.CodeArgumentType, "argument for value expects Map<Text, AnyContent>, got Map<Text, JsonNull>"},
	} {
		t.Run(tc.name, func(t *testing.T) { p15CheckTaggedBoundary(t, tc) })
	}
}

func TestP15TaggedExactBoundaryOracleFaults(t *testing.T) {
	tc := p15TaggedListBoundaryCase(p15NativeListAnalysisCases()[0])
	actual := p15TypeDiagnostics(t, tc.source)
	if err := p15TaggedExactBoundary(actual, tc); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"removed", "duplicate", "wrong_code", "wrong_message", "shift_start", "shift_end", "warning_only"} {
		t.Run(fault, func(t *testing.T) {
			bad := append([]analysis.Diagnostic(nil), actual...)
			for i := range bad {
				if bad[i].Severity != analysis.SeverityError {
					continue
				}
				switch fault {
				case "wrong_code":
					bad[i].Code = "E_UNRELATED"
				case "wrong_message":
					bad[i].Message = "unrelated failure"
				case "shift_start":
					bad[i].Span.Start.ByteOffset++
				case "shift_end":
					bad[i].Span.End.ByteOffset--
				case "warning_only":
					bad[i].Severity = analysis.SeverityWarning
				}
			}
			if fault == "removed" {
				bad = nil
			}
			if fault == "duplicate" {
				bad = append(bad, bad[len(bad)-1])
			}
			if p15TaggedExactBoundary(bad, tc) == nil {
				t.Fatal("oracle accepted " + fault)
			}
		})
	}
}

func p15TaggedInternalNullCases() []struct{ name, source string } {
	return []struct{ name, source string }{
		{"binding", `let value:JsonNull=null;show("end")`},
		{"default", `function identity(value:JsonNull = null)->JsonNull{return value}`},
		{"return", `function produce()->JsonNull{return null};produce()`},
		{"after_raw", `native.action("com.example.p15.raw", {"value":0});let value:JsonNull=null`},
	}
}

// The null cell has an explicit kind and presence and deliberately has no value
// field. This is a Dictionary carrier, never a fabricated native plist-null.
func p15TaggedNullCell(g *p15FunctionGraph) string {
	rows := []any{
		map[string]any{"WFKey": p15FunctionText("_cherri_value_cell_v1"), "WFItemType": 0, "WFValue": p15FunctionText("1")},
		map[string]any{"WFKey": p15FunctionText("has_value"), "WFItemType": 0, "WFValue": p15FunctionText("1")},
		map[string]any{"WFKey": p15FunctionText("kind"), "WFItemType": 0, "WFValue": p15FunctionText("null")},
	}
	return g.value("dictionary", map[string]any{"WFItems": map[string]any{"WFSerializationType": "WFDictionaryFieldValue", "Value": map[string]any{"WFDictionaryFieldValueItems": rows}}})
}

func p15TaggedInternalNullGraph(actual []map[string]any, name string) (int, error) {
	g := &p15FunctionGraph{actual: actual, groups: map[string]bool{}}
	cellIndex := -1
	switch name {
	case "binding":
		cellIndex = len(g.expected)
		p15TaggedNullCell(g)
		g.emit("showresult", map[string]any{"Text": "end"})
	case "default":
		// An uncalled declaration performs no evaluation. The complete empty
		// main graph is checked; this case grants no executed-default proof.
	case "return":
		g.header("produce")
		cellIndex = len(g.expected)
		cell := p15TaggedNullCell(g)
		g.output(cell)
		g.caller()
		g.call(g.payload("produce"))
		g.boundary(g.outer, 2, false)
	case "after_raw":
		g.emit("placeholder", map[string]any{"value": 0})
		g.expected[0]["WFWorkflowActionIdentifier"] = "com.example.p15.raw"
		cellIndex = len(g.expected)
		p15TaggedNullCell(g)
	default:
		return -1, fmt.Errorf("unregistered original null case %s", name)
	}
	g.emit("nothing", nil)
	return cellIndex, g.compare()
}

func p15CheckTaggedInternalNull(t *testing.T, name, source string) []map[string]any {
	t.Helper()
	for _, diagnostic := range p15TypeDiagnostics(t, source) {
		if diagnostic.Severity == analysis.SeverityError {
			t.Fatal(diagnostic)
		}
	}
	svc := service.NewService(nil)
	uri := "file:///p15-internal-null-" + name + ".cherri"
	svc.OpenDocument(uri, 1, source)
	diagnostics, _ := svc.Analyze(uri)
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == analysis.SeverityError {
			t.Fatal(diagnostic)
		}
	}
	actual := p15CompiledActions(t, source)
	if _, err := p15TaggedInternalNullGraph(actual, name); err != nil {
		t.Fatal(err)
	}
	return actual
}

func TestP15TaggedInternalNullGraphs(t *testing.T) {
	for _, tc := range p15TaggedInternalNullCases() {
		t.Run(tc.name, func(t *testing.T) { p15CheckTaggedInternalNull(t, tc.name, tc.source) })
	}
}

func TestP15TaggedInternalNullGraphFaults(t *testing.T) {
	for _, tc := range p15TaggedInternalNullCases() {
		t.Run(tc.name, func(t *testing.T) {
			actual := p15CheckTaggedInternalNull(t, tc.name, tc.source)
			cellIndex, err := p15TaggedInternalNullGraph(actual, tc.name)
			if err != nil {
				t.Fatal(err)
			}
			faults := []string{"extra_action", "missing_void", "invented_native_null"}
			if cellIndex >= 0 {
				faults = append(faults, "absent_instead_of_null", "empty_instead_of_null", "missing_marker", "invented_payload", "missing_cell")
			}
			if tc.name == "return" {
				faults = append(faults, "wrong_return_producer", "wrong_function")
			}
			for _, fault := range faults {
				t.Run(fault, func(t *testing.T) {
					bad := p15TaggedClone(t, actual)
					switch fault {
					case "extra_action":
						bad = append(bad, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
					case "missing_void":
						bad = bad[:len(bad)-1]
					case "invented_native_null":
						bad[len(bad)-1]["WFWorkflowActionParameters"] = map[string]any{"value": nil}
					case "missing_cell":
						bad = append(bad[:cellIndex], bad[cellIndex+1:]...)
					case "wrong_return_producer":
						p15Param(bad[cellIndex+1])["WFOutput"] = p15FunctionEdge("UNRELATED")
					case "wrong_function":
						for _, action := range bad {
							p := p15Param(action)
							if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.runworkflow" {
								p["WFWorkflowName"] = "OTHER_WORKFLOW"
								break
							}
						}
					default:
						wrapper := p15Param(bad[cellIndex])["WFItems"].(map[string]any)["Value"].(map[string]any)
						rows := wrapper["WFDictionaryFieldValueItems"].([]any)
						switch fault {
						case "absent_instead_of_null":
							rows[1].(map[string]any)["WFValue"] = p15FunctionText("0")
						case "empty_instead_of_null":
							rows[2].(map[string]any)["WFValue"] = p15FunctionText("")
						case "missing_marker":
							wrapper["WFDictionaryFieldValueItems"] = rows[1:]
						case "invented_payload":
							wrapper["WFDictionaryFieldValueItems"] = append(rows, map[string]any{"WFKey": p15FunctionText("value"), "WFItemType": 0, "WFValue": p15FunctionText("")})
						}
					}
					if reflect.DeepEqual(bad, actual) {
						t.Fatal("mutation did not change graph")
					}
					if _, err := p15TaggedInternalNullGraph(bad, tc.name); err == nil {
						t.Fatal("oracle accepted " + fault)
					}
				})
			}
		})
	}
}
