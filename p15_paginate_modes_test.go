package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/types"
)

type p15PaginateTraversalCase struct {
	name, mode, start, queryKey, prefix, suffix string
}

var p15PaginateTraversalCases = []p15PaginateTraversalCase{
	{"cursor", "cursor", "https://example.invalid/pages?filter=kept", "", "https://example.invalid/pages?filter=kept&cursor=", ""},
	{"cursor_custom", "cursor", "https://example.invalid/pages?filter=kept", "page[cursor]", "https://example.invalid/pages?filter=kept&page%5Bcursor%5D=", ""},
	{"cursor_initial", "cursor", "https://example.invalid/pages?cursor=%D7%90%2B%2F%3D&filter=kept", "", "https://example.invalid/pages?cursor=", "&filter=kept"},
	{"page", "page", "https://example.invalid/pages?page=1&filter=kept", "", "https://example.invalid/pages?page=", "&filter=kept"},
	{"offset", "offset", "https://example.invalid/pages?offset=0&filter=kept", "", "https://example.invalid/pages?offset=", "&filter=kept"},
}

func p15PaginateTraversalSource(tc p15PaginateTraversalCase, legacy, named, dynamic bool, pages, items int, consume bool) string {
	decl, prefix := "let", ""
	if legacy {
		decl = "const"
	}
	headers := "{}"
	if dynamic {
		prefix = fmt.Sprintf("%s authorization = text(%q)\n", decl, p15PaginateSyntheticAuthorization)
		auth := "authorization"
		if legacy {
			auth = `"{authorization}"`
		}
		headers = fmt.Sprintf("{\"Authorization\": %s, \"X-Cherri-Client\": %q}", auth, p15PaginateSyntheticClient)
	}
	args := fmt.Sprintf("%q, \"items\", \"next\", %d, %d, %s", tc.start, pages, items, headers)
	if tc.mode != "" {
		args += fmt.Sprintf(", %q", tc.mode)
	}
	if tc.queryKey != "" {
		args += fmt.Sprintf(", %q", tc.queryKey)
	}
	if named {
		args = fmt.Sprintf("%q, maxPages: %d, maxItems: %d, headers: %s, mode: %q", tc.start, pages, items, headers, tc.mode)
		if tc.queryKey != "" {
			args += fmt.Sprintf(", queryKey: %q", tc.queryKey)
		}
	}
	call := "paginate(" + args + ")\n"
	if consume {
		call = decl + " rows = " + call + "getVariable(rows)\n"
	}
	return prefix + call
}

func p15PaginateTraversalCompile(t *testing.T, source string, legacy bool, profile BuildProfile) ([]ShortcutAction, error) {
	t.Helper()
	actions, err := compileBuildProfileFixture(t, source, legacy, profile)
	if dir := os.Getenv("P15_PAGINATE_MODES_OUT"); dir != "" {
		name := strings.NewReplacer("/", "__", "=", "-").Replace(t.Name())
		result := map[string]any{"test": t.Name(), "source": source, "legacy": legacy, "profile": profile, "actions": actions}
		if err != nil {
			result["error"] = err.Error()
		}
		data, marshalErr := json.MarshalIndent(result, "", "  ")
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		f, createErr := os.OpenFile(filepath.Join(dir, name+".json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if createErr != nil {
			t.Fatal(createErr)
		}
		_, writeErr := f.Write(append(data, '\n'))
		closeErr := f.Close()
		if writeErr != nil {
			t.Fatal(writeErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	return actions, err
}

func p15PaginateTraversalGraphError(actions []ShortcutAction, tc p15PaginateTraversalCase, legacy bool) error {
	if err := p15LegacyInlineReferenceOracle(actions); err != nil {
		return err
	}
	if err := p15PaginateResultError(actions, legacy); err != nil {
		return err
	}
	requests, repeats, encodes, decodes, rounds, prefixes := 0, 0, 0, 0, 0, 0
	for _, action := range actions {
		p := action.WFWorkflowActionParameters
		switch action.WFWorkflowActionIdentifier {
		case "is.workflow.actions.downloadurl":
			requests++
			if p["WFHTTPMethod"] != "GET" {
				return fmt.Errorf("traversal changed GET method")
			}
			for _, key := range []string{"WFHTTPBodyType", "WFJSONValues", "WFFormValues", "WFRequestVariable"} {
				if _, exists := p[key]; exists {
					return fmt.Errorf("unexpected request behavior %s", key)
				}
			}
		case "is.workflow.actions.repeat.count":
			if count, exists := p["WFRepeatCount"]; exists {
				repeats++
				if fmt.Sprint(count) != "3" {
					return fmt.Errorf("page cap changed")
				}
			}
		case "is.workflow.actions.urlencode":
			if p["WFEncodeMode"] == "Encode" {
				encodes++
			}
			if p["WFEncodeMode"] == "Decode" {
				decodes++
			}
		case "is.workflow.actions.round":
			if p["WFRoundTo"] != "Ones Place" || p["WFRoundMode"] != "Normal" {
				return fmt.Errorf("integer witness changed")
			}
			rounds++
		case "is.workflow.actions.gettext":
			raw := debugParamMap(p["WFTextActionText"])
			value := debugParamMap(raw["Value"])
			if text, ok := value["string"].(string); ok && strings.HasPrefix(text, tc.prefix) && tc.prefix != "" {
				if raw["WFSerializationType"] != "WFTextTokenString" || text != tc.prefix+"￼"+tc.suffix || len(debugParamMap(value["attachmentsByRange"])) != 1 {
					return fmt.Errorf("query URL lost exact dynamic marker attachment")
				}
				prefixes++
			}
		case "is.workflow.actions.alert":
			message, okay := p["WFAlertActionMessage"].(string)
			if !okay || !strings.HasPrefix(message, "PAGINATE_") || strings.Contains(message, "example.invalid") ||
				strings.Contains(message, p15PaginateSyntheticAuthorization) {
				return fmt.Errorf("request/response value leaked through guard")
			}
		}
	}
	if requests != 1 || repeats != 1 {
		return fmt.Errorf("bounded request nodes changed: %d/%d", requests, repeats)
	}
	if tc.mode == "cursor" && (encodes != 1 || decodes != 1 || rounds != 0 || prefixes != 1) {
		return fmt.Errorf("opaque cursor lacks single lossless URL-component encoding: %d/%d/%d/%d", encodes, decodes, rounds, prefixes)
	}
	if (tc.mode == "page" || tc.mode == "offset") && (rounds != 1 || encodes != 0 || decodes != 0 || prefixes != 1) {
		return fmt.Errorf("numeric marker lacks exact integer/query path: %d/%d/%d/%d", encodes, decodes, rounds, prefixes)
	}
	return nil
}

func TestP15PaginateTraversalModesBothDialectsProfiles(t *testing.T) {
	for _, tc := range p15PaginateTraversalCases {
		for _, legacy := range []bool{false, true} {
			for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
				t.Run(fmt.Sprintf("%s/legacy=%t/%s", tc.name, legacy, profile), func(t *testing.T) {
					source := p15PaginateTraversalSource(tc, legacy, false, false, 3, 6, true)
					actions, err := p15PaginateTraversalCompile(t, source, legacy, profile)
					if err != nil {
						t.Fatal(err)
					}
					if err := p15PaginateTraversalGraphError(actions, tc, legacy); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestP15PaginateTraversalNamedOptionsRetainRuntimeHeaders(t *testing.T) {
	for _, tc := range []p15PaginateTraversalCase{p15PaginateTraversalCases[0], p15PaginateTraversalCases[3], p15PaginateTraversalCases[4]} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run(fmt.Sprintf("%s/%s", tc.name, profile), func(t *testing.T) {
				source := p15PaginateTraversalSource(tc, false, true, true, 3, 6, true)
				svc := service.NewService(schema.DefaultRegistry())
				svc.OpenDocument("file:///paginate-modes.cherri", 1, source)
				diagnostics, _ := svc.Analyze("file:///paginate-modes.cherri")
				for _, d := range diagnostics {
					if d.Severity == analysis.SeverityError {
						t.Errorf("analysis: %+v", d)
					}
				}
				actions, err := p15PaginateTraversalCompile(t, source, false, profile)
				if err != nil {
					t.Fatal(err)
				}
				if err := p15PaginateTraversalGraphError(actions, tc, false); err != nil {
					t.Fatal(err)
				}
				if err := p15PaginateHeadersError(actions, true); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestP15PaginateTraversalZeroBudgets(t *testing.T) {
	for _, tc := range []p15PaginateTraversalCase{p15PaginateTraversalCases[0], p15PaginateTraversalCases[3], p15PaginateTraversalCases[4]} {
		for _, caps := range [][2]int{{0, 6}, {3, 0}} {
			for _, legacy := range []bool{false, true} {
				for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
					t.Run(fmt.Sprintf("%s/%d-%d/legacy=%t/%s", tc.name, caps[0], caps[1], legacy, profile), func(t *testing.T) {
						source := p15PaginateTraversalSource(tc, legacy, false, false, caps[0], caps[1], false)
						actions, err := p15PaginateTraversalCompile(t, source, legacy, profile)
						if err != nil {
							t.Fatal(err)
						}
						want := []string{"list", "setvariable", "getvariable"}
						if !legacy {
							want = append(want, "nothing")
						}
						if !reflect.DeepEqual(actionSuffixes(actions), want) {
							t.Fatalf("zero cap performed work: %v", actionSuffixes(actions))
						}
						items, okay := actions[0].WFWorkflowActionParameters["WFItems"].([]any)
						if !okay || len(items) != 0 {
							t.Fatal("empty List replaced with scalar/sentinel")
						}
					})
				}
			}
		}
	}
}

func TestP15PaginateTraversalExplicitURLAndDefault(t *testing.T) {
	for _, mode := range []string{"", "url"} {
		for _, legacy := range []bool{false, true} {
			for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
				t.Run(fmt.Sprintf("mode=%s/legacy=%t/%s", mode, legacy, profile), func(t *testing.T) {
					tc := p15PaginateTraversalCase{mode: mode, start: "https://example.invalid/pages/one.json"}
					actions, err := p15PaginateTraversalCompile(t, p15PaginateTraversalSource(tc, legacy, false, false, 3, 6, true), legacy, profile)
					if err != nil {
						t.Fatal(err)
					}
					if err := p15PaginateTraversalGraphError(actions, tc, legacy); err != nil {
						t.Fatal(err)
					}
					for _, action := range actions {
						if action.WFWorkflowActionIdentifier == "is.workflow.actions.urlencode" ||
							action.WFWorkflowActionIdentifier == "is.workflow.actions.round" {
							t.Fatal("URL default gained a marker conversion")
						}
					}
				})
			}
		}
	}
}

func TestP15PaginateTraversalSchemaPreservesTypedContract(t *testing.T) {
	reg := schema.DefaultRegistry()
	macro, found := reg.LookupAction("paginate")
	if !found || !macro.ScopedBlock || !macro.ScopedBlockOptional || !macro.CompilerConstruct ||
		!macro.OutputType.Equals(types.NewList(types.NewMap(types.Text, types.AnyContent))) {
		t.Fatal("typed macro boundary changed")
	}
	want := []string{"startURL", "itemsKey", "nextKey", "maxPages", "maxItems", "headers", "mode", "queryKey", "itemKey", "itemType"}
	var ids []string
	for _, parameter := range macro.Parameters {
		ids = append(ids, parameter.ID)
		if !parameter.Literal {
			t.Fatalf("dynamic unbounded traversal option %s", parameter.ID)
		}
		if parameter.ID == "mode" || parameter.ID == "queryKey" || parameter.ID == "itemKey" || parameter.ID == "itemType" {
			if !parameter.Optional || !parameter.Type.Equals(types.Text) {
				t.Fatalf("additive literal mode contract lost: %+v", parameter)
			}
		}
	}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("parameter order/availability: %v", ids)
	}
	if len(reg.AllActions()) != 505 || isRemovableDiagnosticMacro("paginate") {
		t.Fatal("catalog/profile contract changed")
	}
}
