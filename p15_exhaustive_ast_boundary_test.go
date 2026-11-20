package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/lower"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/modules"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"howett.net/plist"
)

// Embedding only supplies the sealed interface marker. These concrete types
// intentionally have no production syntax implementation.
type p15RP16UnknownExpr struct{ syntax.Expression }

func (*p15RP16UnknownExpr) NodeSpan() source.Span { return source.Span{} }

type p15RP16UnknownStmt struct{ syntax.Statement }

func (*p15RP16UnknownStmt) NodeSpan() source.Span { return source.Span{} }

type p15RP16UnknownDecl struct{ syntax.Declaration }

func (*p15RP16UnknownDecl) NodeSpan() source.Span { return source.Span{} }

func p15RP16CheckedProgram(t *testing.T, text string) (*syntax.Program, *analysis.Analyzer, *modules.Graph) {
	t.Helper()
	var file = source.NewFile("rp16-boundary", "rp16-boundary.cherri", 1, text)
	var parser = syntax.NewParser(file)
	var program = parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		t.Fatalf("positive source did not parse: %v", parser.Errors())
	}
	var registry = schema.DefaultRegistry()
	var graph, issue = modules.Resolve(program, "rp16-boundary.cherri", nil, registry, migrate.MigrateSource)
	if issue != nil {
		t.Fatalf("positive source did not resolve: %v", issue)
	}
	var analyzer = analysis.NewAnalyzer(registry)
	analyzer.SetModuleGraph(graph)
	analyzer.Analyze(program)
	for _, diagnostic := range analyzer.Diagnostics() {
		if diagnostic.Severity == analysis.SeverityError {
			t.Fatalf("positive source was not checked: %+v", diagnostic)
		}
	}
	return program, analyzer, graph
}

// This drives the same checked-lowering entry and actual canonical session as
// CompileSourceToPlist. Mutation is confined to an already checked test AST;
// no parser error or unresolved module stands in for an unknown node.
func p15RP16CanonicalBoundary(t *testing.T, text string, mutate func(*syntax.Program)) ([]byte, bool, error) {
	t.Helper()
	var program, analyzer, graph = p15RP16CheckedProgram(t, text)
	if mutate != nil {
		mutate(program)
	}
	var session = NewCanonicalBackendSession()
	defer session.Close()
	var lowerer = lower.NewLowererWithSession(analyzer.Registry(), session)
	lowerer.SetBaseFilePath("rp16-boundary.cherri")
	lowerer.SetModuleGraph(graph)
	lowerer.SetModuleRegistries(analyzer.ModuleRegistries())
	lowerer.SetExprTypes(analyzer.ExprTypes())
	lowerer.SetEnumValues(analyzer.EnumValues())
	if err := lowerer.LowerProgramToSession(program); err != nil {
		return nil, false, err
	}
	var raw, err = session.Finalize()
	return raw, true, err
}

func TestP15RP16UnknownNodesCanonicalBoundary(t *testing.T) {
	var cases = []struct {
		name      string
		source    string
		mutate    func(*syntax.Program)
		errorPart string
	}{
		{"declaration", "text(\"RP16_HEALTHY\")", func(p *syntax.Program) { p.Declarations = append(p.Declarations, &p15RP16UnknownDecl{}) }, "unhandled declaration"},
		{"statement", "text(\"RP16_HEALTHY\")", func(p *syntax.Program) { p.Statements = append(p.Statements, &p15RP16UnknownStmt{}) }, "unhandled statement"},
		{"expression", "text(\"RP16_HEALTHY\")", func(p *syntax.Program) { p.Statements[0].(*syntax.ExprStmt).Expr = &p15RP16UnknownExpr{} }, "unhandled expression"},
		{"call_argument", "text(\"RP16_HEALTHY\")", func(p *syntax.Program) {
			p.Statements[0].(*syntax.ExprStmt).Expr.(*syntax.CallExpr).PrimaryArg = &p15RP16UnknownExpr{}
		}, "unhandled expression"},
		{"map_key", "let value = {\"key\": \"RP16_HEALTHY\"}\nshow(value)", func(p *syntax.Program) {
			p.Statements[0].(*syntax.BindingStmt).Value.(*syntax.MapExpr).Entries[0].Key = &p15RP16UnknownExpr{}
		}, "unhandled expression"},
		{"map_value", "let value = {\"key\": \"RP16_HEALTHY\"}\nshow(value)", func(p *syntax.Program) {
			p.Statements[0].(*syntax.BindingStmt).Value.(*syntax.MapExpr).Entries[0].Value = &p15RP16UnknownExpr{}
		}, "unhandled expression"},
		{"list_item", "let value = [\"RP16_HEALTHY\"]\nshow(value)", func(p *syntax.Program) {
			p.Statements[0].(*syntax.BindingStmt).Value.(*syntax.ListExpr).Elements[0] = &p15RP16UnknownExpr{}
		}, "unhandled expression"},
		{"text_reference", "let value = \"RP16_HEALTHY\"\ntext(f\"prefix:{value}\")", func(p *syntax.Program) {
			p.Statements[1].(*syntax.ExprStmt).Expr.(*syntax.CallExpr).PrimaryArg.(*syntax.FStringExpr).Parts[1].Expr = &p15RP16UnknownExpr{}
		}, "unhandled expression"},
		{"called_function", "function target(): Text { return \"RP16_HEALTHY\" }\nshow(target())", func(p *syntax.Program) {
			p.Declarations[0].(*syntax.FunctionDecl).Body.Statements[0].(*syntax.ReturnStmt).Value = &p15RP16UnknownExpr{}
		}, "unhandled expression"},
		{"unused_function", "function target(): Text { return \"RP16_HEALTHY\" }\ntext(\"RP16_AFTER\")", func(p *syntax.Program) {
			p.Declarations[0].(*syntax.FunctionDecl).Body.Statements[0].(*syntax.ReturnStmt).Value = &p15RP16UnknownExpr{}
		}, "unhandled expression"},
		{"unused_copy", "copy target { text(\"RP16_HEALTHY\") }\ntext(\"RP16_AFTER\")", func(p *syntax.Program) {
			p.Declarations[0].(*syntax.CopyBlockStmt).Body.Statements[0].(*syntax.ExprStmt).Expr = &p15RP16UnknownExpr{}
		}, "unhandled expression"},
		{"metadata_record", "shortcut \"RP16\" { icon: {glyph: 59789, color: 4282601983} }\ntext(\"RP16_HEALTHY\")", func(p *syntax.Program) {
			p.Declarations[0].(*syntax.ShortcutDecl).Metadata.Fields[0].Value = &p15RP16UnknownExpr{}
		}, "unhandled expression"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var healthy, err = CompileSourceToPlist("rp16-positive.cherri", tc.source)
			var native map[string]any
			if err != nil || len(healthy) == 0 {
				t.Fatalf("real positive compile failed: %v", err)
			}
			if _, err = plist.Unmarshal(healthy, &native); err != nil {
				t.Fatal(err)
			}
			if _, ok := native["WFWorkflowActions"].([]any); !ok {
				t.Fatalf("positive is not a native workflow: %v", native)
			}
			var raw, finalized, loweringErr = p15RP16CanonicalBoundary(t, tc.source, tc.mutate)
			if loweringErr == nil || !strings.Contains(loweringErr.Error(), tc.errorPart) || finalized || len(raw) != 0 {
				t.Fatalf("unknown node crossed actual native boundary: error=%v finalized=%v bytes=%d; want explicit %q and no bytes", loweringErr, finalized, len(raw), tc.errorPart)
			}
			// Closing a rejected real session must leave the next compilation valid.
			var restored, restoredFinalized, restoreErr = p15RP16CanonicalBoundary(t, "text(\"RP16_RESTORED\")", nil)
			if restoreErr != nil || !restoredFinalized {
				t.Fatalf("post-rejection compile failed: %v", restoreErr)
			}
			if _, err = plist.Unmarshal(restored, &native); err != nil {
				t.Fatal(err)
			}
			var actions = native["WFWorkflowActions"].([]any)
			if len(actions) != 1 {
				t.Fatalf("rejected session leaked actions: %v", actions)
			}
			var action = actions[0].(map[string]any)
			var params = action["WFWorkflowActionParameters"].(map[string]any)
			if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" || params["WFTextActionText"] != "RP16_RESTORED" {
				t.Fatal(fmt.Sprintf("restored native graph changed: %#v", actions))
			}
		})
	}
}
