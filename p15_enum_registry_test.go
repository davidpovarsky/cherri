package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/enums"
	"github.com/electrikmilk/cherri/internal/language/lower"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func TestP15C04ProvidedRegistryEnumDomain(t *testing.T) {
	action := func(name string, values []string) *schema.ActionSchema {
		return &schema.ActionSchema{
			ID: name, CallableName: name, AppleIdentifier: "com.example.p15." + name,
			PrimaryParameterID: "mode", OutputType: types.Text, OutputTypeName: "Text",
			Parameters: []schema.ParameterSchema{{
				ID: "mode", Label: "mode", Type: types.Text, TypeName: "Text",
				EnumName: "P15RegistryMode", EnumValues: values, WireKey: "NativeMode", Codec: "enum",
				AcceptedForms: []string{"literal"},
			}},
		}
	}
	// The supplied parameter contracts are complete even though their domain
	// is absent from Registry.AllEnums. No DefaultRegistry mutation is used.
	registry := schema.NewRegistry([]*schema.ActionSchema{
		action("registryPick", []string{"Fast", "Accurate", ""}),
		action("registryFastOnly", []string{"Fast"}),
	}, nil)
	if _, exists := registry.LookupEnum("P15RegistryMode"); exists {
		t.Fatal("fixture accidentally has a global enum declaration")
	}
	if _, exists := schema.DefaultRegistry().LookupAction("registryPick"); exists {
		t.Fatal("fixture leaked into the default registry")
	}

	for _, tc := range []struct{ source, want string }{
		{"registryPick(\"Accurate\")", "Accurate"},
		{"registryPick(.Accurate)", "Accurate"},
		{"registryPick(P15RegistryMode.Accurate)", "Accurate"},
		{"registryPick(mode: \"\")", ""},
		{"registryFastOnly(.Fast)", "Fast"},
	} {
		program, errors := p15ParseProgram(tc.source)
		if len(errors) != 0 {
			t.Fatal(errors)
		}
		analyzer := analysis.NewAnalyzer(registry)
		analyzer.Analyze(program)
		for _, d := range analyzer.Diagnostics() {
			if d.Severity == analysis.SeverityError {
				t.Fatal(tc.source, d)
			}
		}
		compiler := lower.NewLowerer(registry)
		compiler.SetExprTypes(analyzer.ExprTypes())
		compiler.SetEnumValues(analyzer.EnumValues())
		workflow, err := compiler.LowerProgram(program)
		if err != nil || workflow == nil || len(workflow.Actions) != 2 {
			t.Fatalf("%s: %v %+v", tc.source, err, workflow)
		}
		if workflow.Actions[1].AppleIdentifier != "is.workflow.actions.nothing" ||
			len(workflow.Actions[1].Parameters) != 0 {
			t.Fatal("provided-registry main does not end with an explicit empty result")
		}
		if got := workflow.Actions[0].Parameters["NativeMode"]; got != tc.want {
			t.Fatalf("%s wire=%#v want %q", tc.source, got, tc.want)
		}
	}
	for _, source := range []string{
		"registryPick(.Missing)", "registryPick(P15RegistryMode.Missing)", "registryPick(\"missing\")",
		"registryFastOnly(.Accurate)", "registryFastOnly(P15RegistryMode.Accurate)", "registryFastOnly(\"Accurate\")",
	} {
		program, errors := p15ParseProgram(source)
		if len(errors) != 0 {
			t.Fatal(errors)
		}
		analyzer := analysis.NewAnalyzer(registry)
		analyzer.Analyze(program)
		count := 0
		for _, d := range analyzer.Diagnostics() {
			if d.Severity == analysis.SeverityError {
				if d.Code != analysis.CodeEnumMember {
					t.Fatal(d)
				}
				count++
			}
		}
		if count != 1 {
			t.Fatalf("invalid provided-registry enum accepted or duplicated: %s %+v", source, analyzer.Diagnostics())
		}
	}
	svc := service.NewService(registry)
	for index, marked := range []string{"registryPick(mode: .Acc|)", "registryPick(P15RegistryMode.Acc|)"} {
		cursor := strings.Index(marked, "|")
		source := strings.Replace(marked, "|", "", 1)
		uri := fmt.Sprintf("file:///p15-registry-%d.cherri", index)
		svc.OpenDocument(uri, 1, source)
		line, col := p15C04CompletionPosition(source, cursor)
		found := false
		for _, item := range svc.Complete(uri, line, col) {
			if item.Kind == service.CompletionKindEnum && item.Label == "Accurate" && item.Detail == "enum P15RegistryMode" {
				completed := p15C04ApplyCompletion(source, cursor, item.InsertText)
				svc.ChangeDocument(uri, 2, completed)
				diags, _ := svc.Analyze(uri)
				for _, d := range diags {
					if d.Severity == analysis.SeverityError {
						t.Fatal("provided registry completion invalid", d)
					}
				}
				found = true
			}
		}
		if !found {
			t.Fatal("provided action parameter domain absent from completion", marked)
		}
	}
	catalog := enums.New(registry, nil)
	if wire, ok := catalog.Resolve("P15RegistryMode", "Accurate"); !ok || wire != "Accurate" {
		t.Fatal("shared catalog ignored supplied parameter domain")
	}
	if _, ok := catalog.Resolve("P15RegistryMode", "Missing"); ok {
		t.Fatal("unknown enum member mutation passed")
	}
	if _, exists := registry.LookupEnum("P15RegistryMode"); exists {
		t.Fatal("catalog construction mutated the supplied registry")
	}
}
