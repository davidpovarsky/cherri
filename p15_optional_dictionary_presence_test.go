package main

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
)

type p15OptionalDictionaryCase struct{ name, typ, value string }

func p15OptionalDictionaryCases() []p15OptionalDictionaryCase {
	return []p15OptionalDictionaryCase{
		{"text", "Text", `"ready"`}, {"empty_text", "Text", `""`},
		{"false", "Bool", `false`}, {"zero", "Number", `0`},
		{"empty_list", "List<Text>", `[]`}, {"list", "List<Text>", `["one","two"]`},
		{"empty_map", "Map<Text,AnyContent>", `{}`}, {"map", "Map<Text,Text>", `{"nested":"VALUE"}`},
	}
}

func p15OptionalDictionarySelection(tc p15OptionalDictionaryCase, route string) (prefix, selected string) {
	dictionary := `{"key":` + tc.value + `}`
	switch route {
	case "literal":
		return "", dictionary + `["key"]`
	case "alias":
		return "let source=" + dictionary + ";let alias=source;", `alias["key"]`
	case "require":
		return "let checked=require(" + dictionary + `,type:"dictionary");`, `checked["key"]`
	case "named_require_alias":
		return "let source=" + dictionary + `;let checked=require(type:"dictionary",value:source);let alias=checked;`, `alias["key"]`
	default:
		panic("unknown Dictionary provenance route")
	}
}

func p15OptionalDictionarySource(tc p15OptionalDictionaryCase, route, consumer string) string {
	prefix, selected := p15OptionalDictionarySelection(tc, route)
	switch consumer {
	case "binding":
		return prefix + "let value:" + tc.typ + "?=" + selected + ";getVariable(value!=none)"
	case "argument":
		return "function echo(value:" + tc.typ + "?)->" + tc.typ + "?{return value}\n" + prefix + "getVariable(echo(" + selected + "))"
	case "return":
		return "function pick()->" + tc.typ + "?{" + prefix + "return " + selected + "}\ngetVariable(pick())"
	default:
		panic("unknown Dictionary Optional consumer")
	}
}

func TestP15OptionalDictionaryPublicRoutes(t *testing.T) {
	for _, tc := range p15OptionalDictionaryCases() {
		for _, route := range []string{"literal", "alias", "require", "named_require_alias"} {
			for _, consumer := range []string{"binding", "argument", "return"} {
				t.Run(tc.name+"/"+route+"/"+consumer, func(t *testing.T) {
					source := p15OptionalDictionarySource(tc, route, consumer)
					for _, d := range p15TypeDiagnostics(t, source) {
						if d.Severity == analysis.SeverityError {
							t.Fatal(d)
						}
					}
					if raw, err := CompileSourceToPlist("p15-owned-dictionary.cherri", source); err != nil || len(raw) == 0 {
						t.Fatalf("source-owned Dictionary slot failed: bytes=%d error=%v", len(raw), err)
					}
					svc := service.NewService(nil)
					uri := "file:///p15-owned-dictionary.cherri"
					svc.OpenDocument(uri, 1, source)
					diagnostics, _ := svc.Analyze(uri)
					for _, d := range diagnostics {
						if d.Severity == analysis.SeverityError {
							t.Fatal("editor disagrees", d)
						}
					}
				})
			}
		}
	}
}

func p15OptionalDictionaryNegatives() []p15OptionalIndexNegative {
	return []p15OptionalIndexNegative{
		{"missing_key", `let selected:Text?={"other":"ready"}["key"]`, `{"other":"ready"}["key"]`, nil},
		{"duplicate_key", `let selected:Text?={"key":"first","key":"last"}["key"]`, `{"key":"first","key":"last"}["key"]`, nil},
		{"dynamic_key", `let key="key";let selected:Text?={"key":"ready"}[key]`, `{"key":"ready"}[key]`, nil},
		{"computed_key", `let selected:Text?={text("key"):"ready"}["key"]`, `{text("key"):"ready"}["key"]`, nil},
		{"mutable_target", `var values={"key":"ready"};let selected:Text?=values["key"]`, `values["key"]`, nil},
		{"mutable_alias", `var values={"key":"ready"};let alias=values;values={"other":"changed"};let selected:Text?=alias["key"]`, `alias["key"]`, nil},
		{"require_mutable_alias", `var values={"key":"ready"};let alias=values;let checked=require(alias,type:"dictionary");let selected:Text?=checked["key"]`, `checked["key"]`, nil},
		{"require_missing_key", `let checked=require({"other":"ready"},type:"dictionary");let selected:Text?=checked["key"]`, `checked["key"]`, nil},
		{"user_require", `function require(value:Map<Text,Text>)->Map<Text,Text>{return {"other":"different"}};let checked=require({"key":"ready"});let selected:Text?=checked["key"]`, `checked["key"]`, nil},
		{"opaque_target", `let checked=require(native.action("is.workflow.actions.nothing",{}),type:"dictionary");let selected:AnyContent?=checked["key"]`, `checked["key"]`, nil},
		{"optional_entry", `let maybe:Text?=none;let selected:Text?={"key":maybe}["key"]`, `{"key":maybe}["key"]`, []string{"maybe"}},
		{"opaque_entry", `let selected:AnyContent?={"key":native.action("is.workflow.actions.nothing",{})}["key"]`, `{"key":native.action("is.workflow.actions.nothing",{})}["key"]`, nil},
	}
}

func TestP15OptionalDictionaryRejectsUnprovenOrigins(t *testing.T) {
	for _, tc := range p15OptionalDictionaryNegatives() {
		t.Run(tc.name, func(t *testing.T) {
			if err := p15OptionalIndexBoundaryError(p15TypeDiagnostics(t, tc.source), tc); err != nil {
				t.Fatal(err)
			}
			if raw, err := CompileSourceToPlist("p15-unproved-dictionary.cherri", tc.source); err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeTargetValueShape) {
				t.Fatalf("unproved Dictionary value emitted bytes=%d error=%v", len(raw), err)
			}
			svc := service.NewService(nil)
			uri := "file:///p15-unproved-dictionary.cherri"
			svc.OpenDocument(uri, 1, tc.source)
			got, _ := svc.Analyze(uri)
			if err := p15OptionalIndexBoundaryError(got, tc); err != nil {
				t.Fatal(err)
			}
		})
	}
}
