package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// This is the untouched authored input of the historical rejection leaf.
const p15F04CanonicalDelayOriginal = "let values=repeat 2 {yield []}\nwait(0)\nreturn count(values)"

func p15F04CanonicalDelayCase() p15F04LocalCellCase {
	return p15F04LocalCellCase{
		name: "unselected_action_after_cells", producer: "repeat", value: "empty_list",
		consumer: "count", iterations: 2, historical: true, canonicalDelay: true,
	}
}

func p15F04CheckPromotedCanonicalDelay(t *testing.T, text string) {
	t.Helper()
	if text != p15F04CanonicalDelayOriginal {
		t.Fatal("historical Delay fixture was replaced")
	}
	p := syntax.NewParser(source.NewFile("p15-f04-delay.cherri", "p15-f04-delay.cherri", 1, text))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 || len(program.Statements) != 3 {
		t.Fatal("original authored fixture did not parse", p.Errors())
	}
	binding, ok := program.Statements[0].(*syntax.BindingStmt)
	if !ok {
		t.Fatal("actual collection declaration lost")
	}
	statement, ok := program.Statements[1].(*syntax.ExprStmt)
	if !ok {
		t.Fatal("actual Delay statement lost")
	}
	call, ok := statement.Expr.(*syntax.CallExpr)
	if !ok {
		t.Fatal("actual Delay call lost")
	}
	registry := schema.DefaultRegistry()
	action, exists := registry.LookupAction("wait")
	plan := schema.PlanLoopResultLocals(program, registry)
	if !exists || !schema.LoopResultExplicitDelay(call, action) || plan == nil || !plan.BindingSourceMatches(binding) {
		t.Fatal("canonical explicit Delay or original collection identity not enrolled")
	}
	before := syntax.Format(program)
	analyzer := analysis.NewAnalyzer(registry)
	analyzer.Analyze(program)
	for _, diagnostic := range analyzer.Diagnostics() {
		if diagnostic.Severity == analysis.SeverityError {
			t.Fatalf("canonical explicit Delay rejected: %+v", diagnostic)
		}
	}
	if syntax.Format(program) != before || program.Statements[0] != binding ||
		program.Statements[1].(*syntax.ExprStmt).Expr != call || !plan.BindingSourceMatches(binding) {
		t.Fatal("analysis replaced the authored AST to admit Delay")
	}
	if _, err := p15F04LocalCellGraphError(p15CompiledActions(t, text), p15F04CanonicalDelayCase()); err != nil {
		t.Fatal(err)
	}
}

// All operations remain in the complete graph. These faults cannot be hidden
// by deleting or filtering Delay, private-cell helpers, or final consumers.
func TestP15F04CanonicalDelayFullGraphRejectsCorruption(t *testing.T) {
	tc := p15F04CanonicalDelayCase()
	original := p15CompiledActions(t, p15F04CanonicalDelayOriginal)
	trace, err := p15F04LocalCellGraphError(original, tc)
	if err != nil {
		t.Fatal("healthy original Delay graph", err)
	}
	for _, fault := range []string{
		"omitted_seconds", "wrong_seconds", "implicit_input", "foreign_action",
		"missing_delay", "duplicate_delay", "delay_before_collection",
		"count_reads_cell", "output_reads_cells", "duplicate_append",
		"drop_append", "extra_nothing",
	} {
		t.Run(fault, func(t *testing.T) {
			bad := p15C01Clone(t, original)
			if _, err := p15F04LocalCellGraphError(bad, tc); err != nil {
				t.Fatal("unchanged clone failed", err)
			}
			before := p15C01Clone(t, bad)
			at := trace.positions["explicit_delay"]
			p := p15Param(bad[at])
			switch fault {
			case "omitted_seconds":
				delete(p, "WFDelayTime")
			case "wrong_seconds":
				p["WFDelayTime"] = 1
			case "implicit_input":
				p["WFInput"] = p15FunctionVariable("values")
			case "foreign_action":
				bad[at]["WFWorkflowActionIdentifier"] = "com.example.delay"
			case "missing_delay":
				bad = append(bad[:at], bad[at+1:]...)
			case "duplicate_delay":
				copyOfDelay := p15C01Clone(t, []map[string]any{bad[at]})[0]
				expanded := append([]map[string]any{}, bad[:at+1]...)
				expanded = append(expanded, copyOfDelay)
				bad = append(expanded, bad[at+1:]...)
			case "delay_before_collection":
				bad[at], bad[0] = bad[0], bad[at]
			case "count_reads_cell":
				id, _ := p15Param(bad[trace.positions["cell_producer"]])["UUID"].(string)
				p15Param(bad[trace.positions["count_consumer"]])["Input"] = p15FunctionEdge(id)
			case "output_reads_cells":
				id, _ := p15Param(bad[trace.positions["collection_read"]])["UUID"].(string)
				p15Param(bad[trace.positions["final_output"]])["WFOutput"] = p15FunctionText("\ufffc", id)
			case "duplicate_append":
				at = trace.positions["append"]
				copyOfAppend := p15C01Clone(t, []map[string]any{bad[at]})[0]
				expanded := append([]map[string]any{}, bad[:at+1]...)
				expanded = append(expanded, copyOfAppend)
				bad = append(expanded, bad[at+1:]...)
			case "drop_append":
				at = trace.positions["append"]
				bad = append(bad[:at], bad[at+1:]...)
			case "extra_nothing":
				bad = append(bad, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
			}
			if reflect.DeepEqual(before, bad) {
				t.Fatal("causal mutation changed no graph field")
			}
			if _, err := p15F04LocalCellGraphError(bad, tc); err == nil || !strings.Contains(err.Error(), "F04_LOCAL_GRAPH") {
				t.Fatal("complete graph accepted a changed Delay/cell/count contract", fault, err)
			}
		})
	}
}
