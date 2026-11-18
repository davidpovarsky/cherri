package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func TestP15PaginateNativeMetadataAndPublicBoundary(t *testing.T) {
	data, err := os.ReadFile("docs/apple-runtime/macos-27/builtin-actions.json")
	if err != nil {
		t.Fatal(err)
	}
	var native map[string]map[string]any
	if err := json.Unmarshal(data, &native); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ id, class, input string }{
		{"downloadurl", "WFDownloadURLAction", "WFURL"},
		{"detect.dictionary", "WFCoercionAction", "WFInput"},
		{"getvalueforkey", "WFGetDictionaryValueAction", "WFInput"},
		{"hash", "WFGenerateHashAction", "WFInput"},
		{"count", "WFCountAction", "Input"},
		{"repeat.each", "WFForEachRepeatAction", "WFInput"},
		{"getitemfromlist", "WFGetItemFromListAction", "WFInput"},
		{"appendvariable", "WFAppendVariableAction", "WFInput"},
	} {
		entry := native["is.workflow.actions."+row.id]
		if entry["ActionClass"] != row.class || debugParamMap(entry["Input"])["ParameterKey"] != row.input {
			t.Fatalf("independent native metadata mismatch for %s", row.id)
		}
	}
	foundHash := false
	for _, value := range native["is.workflow.actions.hash"]["Parameters"].([]any) {
		p := debugParamMap(value)
		if p["Key"] == "WFHashType" {
			for _, item := range p["Items"].([]any) {
				foundHash = foundHash || item == "SHA256"
			}
		}
	}
	if !foundHash {
		t.Fatal("native SHA256 selector missing")
	}
	reg := schema.DefaultRegistry()
	macro, found := reg.LookupAction("paginate")
	if !found || !macro.CompilerConstruct || macro.ScopedBlock ||
		!macro.OutputType.Equals(types.NewList(types.NewMap(types.Text, types.AnyContent))) {
		t.Fatal("bounded Dictionary-list macro metadata missing")
	}
	for _, parameter := range macro.Parameters {
		if !parameter.Literal {
			t.Fatalf("unbounded dynamic option: %s", parameter.ID)
		}
	}
	if len(reg.AllActions()) != 505 {
		t.Fatal("compiler macro changed native catalog denominator")
	}
	if isRemovableDiagnosticMacro("paginate") {
		t.Fatal("semantic network traversal cannot be stripped in Release")
	}
}

func TestP15PaginateBothFrontendsProfilesAndTypedOutput(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run(fmt.Sprintf("legacy=%t/%s", legacy, profile), func(t *testing.T) {
				decl := "let"
				if legacy {
					decl = "const"
				}
				source := decl + " rows = paginate(\"https://example.invalid/pages/one.json\", \"rows\", \"following\", 3, 6)\ngetVariable(rows)\n"
				actions, err := compileBuildProfileFixture(t, source, legacy, profile)
				if err != nil {
					t.Fatal(err)
				}
				requests, pageLoops := 0, 0
				producers := map[string]string{}
				var visit func(any)
				visit = func(value any) {
					switch v := value.(type) {
					case map[string]any:
						if v["Type"] == "ActionOutput" {
							id, _ := v["OutputUUID"].(string)
							name, _ := v["OutputName"].(string)
							if expected, exists := producers[id]; !exists || expected != name {
								t.Fatalf("reference lacks exact previous producer UUID/name: %s", id)
							}
						}
						for _, child := range v {
							visit(child)
						}
					case []any:
						for _, child := range v {
							visit(child)
						}
					}
				}
				for _, action := range actions {
					p := action.WFWorkflowActionParameters
					visit(p)
					if id, ok := p["UUID"].(string); ok {
						if _, exists := producers[id]; exists {
							t.Fatal("duplicate producer UUID")
						}
						producers[id], _ = p["CustomOutputName"].(string)
					}
					switch action.WFWorkflowActionIdentifier {
					case "is.workflow.actions.downloadurl":
						requests++
						if p["WFHTTPMethod"] != "GET" || debugParamMap(p["WFURL"])["WFSerializationType"] != "WFTextTokenString" {
							t.Fatal("GET URL bypassed canonical text codec")
						}
						for _, key := range []string{"WFHTTPHeaders", "WFHTTPBodyType", "WFJSONValues", "WFRequestVariable"} {
							if _, exists := p[key]; exists {
								t.Fatalf("unsupported authenticated/body contract emitted %s", key)
							}
						}
					case "is.workflow.actions.repeat.count":
						if count, exists := p["WFRepeatCount"]; exists {
							pageLoops++
							if fmt.Sprint(count) != "3" {
								t.Fatal("literal page budget lost")
							}
						}
					case "is.workflow.actions.alert":
						message, _ := p["WFAlertActionMessage"].(string)
						if !strings.HasPrefix(message, "PAGINATE_") || strings.Contains(message, "example.invalid") {
							t.Fatal("guard exposes a URL or response body")
						}
					}
				}
				if requests != 1 || pageLoops != 1 {
					t.Fatalf("expected one statically bounded GET node, got %d/%d", requests, pageLoops)
				}
				returned := actions[len(actions)-2].WFWorkflowActionParameters["UUID"]
				tail := debugParamMap(debugParamMap(actions[len(actions)-1].WFWorkflowActionParameters["WFVariable"])["Value"])
				if tail["OutputUUID"] != returned {
					t.Fatal("consumer lost original typed aggregate result")
				}
			})
		}
	}
}

func TestP15PaginateZeroBudgetsEmitNoRequest(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, limits := range [][2]int{{0, 1000}, {20, 0}, {0, 0}} {
			source := fmt.Sprintf("paginate(\"https://example.invalid/one.json\", \"items\", \"next\", %d, %d)\n", limits[0], limits[1])
			actions, err := compileBuildProfileFixture(t, source, legacy, BuildProfileRelease)
			if err != nil {
				t.Fatal(err)
			}
			if got := actionSuffixes(actions); !reflect.DeepEqual(got, []string{"list", "setvariable", "getvariable"}) {
				t.Fatalf("zero budget performed work: legacy=%t %v", legacy, got)
			}
			items, ok := actions[0].WFWorkflowActionParameters["WFItems"].([]any)
			if !ok || len(items) != 0 {
				t.Fatal("zero budget manufactured a Text/scalar sentinel")
			}
		}
	}
}

func TestP15PaginateAnalysisAndCompileRejectUnsupportedContracts(t *testing.T) {
	cases := []string{
		"paginate(\"file:///private/input\")\n",
		"paginate(\"https://user:synthetic@example.invalid/page\")\n",
		"paginate(\"https://example.invalid/page#fragment\")\n",
		"paginate(\"https://example.invalid/page\", \"items.path\")\n",
		"paginate(\"https://example.invalid/page\", \"same\", \"same\")\n",
		"paginate(\"https://example.invalid/page\", \"items\", \"next\", 101)\n",
		"paginate(\"https://example.invalid/page\", \"items\", \"next\", 20, 10001)\n",
		"paginate(\"https://example.invalid/page\", \"items\", \"next\", 1.5)\n",
	}
	for _, source := range cases {
		svc := service.NewService(schema.DefaultRegistry())
		svc.OpenDocument("file:///paginate-analysis.cherri", 1, source)
		diagnostics, _ := svc.Analyze("file:///paginate-analysis.cherri")
		hasError := false
		for _, diagnostic := range diagnostics {
			hasError = hasError || diagnostic.Severity == analysis.SeverityError
		}
		if !hasError {
			t.Fatalf("analysis advertised unsupported contract: %s", source)
		}
		for _, legacy := range []bool{false, true} {
			for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
				if _, err := compileBuildProfileFixture(t, source, legacy, profile); err == nil {
					t.Fatalf("legacy=%t profile=%s accepted %s", legacy, profile, source)
				}
			}
		}
	}
	valid := "let rows = paginate(\"https://example.invalid/pages/one.json\", maxPages: 3, maxItems: 6)\nfor row in rows { getValue(row, \"id\") }\n"
	svc := service.NewService(schema.DefaultRegistry())
	svc.OpenDocument("file:///paginate-valid.cherri", 1, valid)
	diagnostics, _ := svc.Analyze("file:///paginate-valid.cherri")
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == analysis.SeverityError {
			t.Fatalf("valid literal holes/typed iteration rejected: %+v", diagnostic)
		}
	}
	if _, err := CompileSourceToPlist("paginate-valid.cherri", valid); err != nil {
		t.Fatal(err)
	}
}
