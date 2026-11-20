package main

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

type p15RP16UnknownType struct{ syntax.TypeAnnotation }

func (*p15RP16UnknownType) NodeSpan() source.Span { return source.Span{} }

func TestP15RP16UnknownTypeAnnotationsCanonicalBoundary(t *testing.T) {
	var cases = []struct {
		name, source string
		mutate       func(*syntax.Program)
	}{
		{"alias", "type Alias = Text\ntext(\"RP16_HEALTHY\")", func(p *syntax.Program) { p.Declarations[0].(*syntax.TypeDecl).TypeExpr = &p15RP16UnknownType{} }},
		{"binding", "let value: Text = \"RP16_HEALTHY\"\nshow(value)", func(p *syntax.Program) { p.Statements[0].(*syntax.BindingStmt).TypeExpr = &p15RP16UnknownType{} }},
		{"function_parameter", "function target(value: Text): Text { return value }\nshow(target(value: \"RP16_HEALTHY\"))", func(p *syntax.Program) {
			p.Declarations[0].(*syntax.FunctionDecl).Parameters[0].TypeExpr = &p15RP16UnknownType{}
		}},
		{"function_return", "function target(): Text { return \"RP16_HEALTHY\" }\nshow(target())", func(p *syntax.Program) { p.Declarations[0].(*syntax.FunctionDecl).ReturnType = &p15RP16UnknownType{} }},
		{"action_parameter", "action marker(value: Text): Text { identifier: \"is.workflow.actions.gettext\" primary: \"value\" bindings: {value: {key: \"WFTextActionText\"}} }\nmarker(\"RP16_HEALTHY\")", func(p *syntax.Program) {
			p.Declarations[0].(*syntax.ActionDecl).Parameters[0].TypeExpr = &p15RP16UnknownType{}
		}},
		{"action_return", "action marker(value: Text): Text { identifier: \"is.workflow.actions.gettext\" primary: \"value\" bindings: {value: {key: \"WFTextActionText\"}} }\nmarker(\"RP16_HEALTHY\")", func(p *syntax.Program) { p.Declarations[0].(*syntax.ActionDecl).ReturnType = &p15RP16UnknownType{} }},
		{"setup", "setup answer: Text { prompt: \"RP16_PROMPT\" default: \"RP16_HEALTHY\" }\ntext(answer)", func(p *syntax.Program) { p.Declarations[0].(*syntax.SetupDecl).TypeExpr = &p15RP16UnknownType{} }},
		{"optional_child", "type Alias = Text?\ntext(\"RP16_HEALTHY\")", func(p *syntax.Program) {
			p.Declarations[0].(*syntax.TypeDecl).TypeExpr.(*syntax.OptionalTypeAnnotation).Inner = &p15RP16UnknownType{}
		}},
		{"generic_child", "type Alias = List<Text>\ntext(\"RP16_HEALTHY\")", func(p *syntax.Program) {
			p.Declarations[0].(*syntax.TypeDecl).TypeExpr.(*syntax.GenericTypeAnnotation).TypeArgs[0] = &p15RP16UnknownType{}
		}},
		{"union_child", "type Alias = Text | Number\ntext(\"RP16_HEALTHY\")", func(p *syntax.Program) {
			p.Declarations[0].(*syntax.TypeDecl).TypeExpr.(*syntax.UnionTypeAnnotation).Members[0] = &p15RP16UnknownType{}
		}},
		{"record_child", "type Alias = {name: Text}\ntext(\"RP16_HEALTHY\")", func(p *syntax.Program) {
			p.Declarations[0].(*syntax.TypeDecl).TypeExpr.(*syntax.RecordTypeAnnotation).Fields[0].TypeExpr = &p15RP16UnknownType{}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if raw, err := CompileSourceToPlist("rp16-type-positive.cherri", tc.source); err != nil || len(raw) == 0 {
				t.Fatalf("positive annotation source failed: %v", err)
			}
			var raw, finalized, err = p15RP16CanonicalBoundary(t, tc.source, tc.mutate)
			if err == nil || !strings.Contains(err.Error(), "unhandled type annotation") || finalized || len(raw) != 0 {
				t.Fatalf("unknown type annotation crossed native boundary: error=%v finalized=%v bytes=%d", err, finalized, len(raw))
			}
			if restored, yes, restoreErr := p15RP16CanonicalBoundary(t, "text(\"RP16_RESTORED\")", nil); restoreErr != nil || !yes || len(restored) == 0 {
				t.Fatalf("post-rejection session was not restored: %v", restoreErr)
			}
		})
	}
}
