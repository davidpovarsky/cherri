package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
)

const p15PaginateSyntheticAuthorization = "Bearer PAGINATE_SYNTHETIC_ONLY"
const p15PaginateSyntheticClient = "Cherri-P15"

func p15PaginateHeadersFixture(legacy, dynamic, named bool, pages, items int, call string) string {
	decl := "let"
	if legacy {
		decl = "const"
	}
	prefix := ""
	auth := fmt.Sprintf("%q", p15PaginateSyntheticAuthorization)
	if dynamic {
		prefix = fmt.Sprintf("%s authorization = text(%q)\n", decl, p15PaginateSyntheticAuthorization)
		auth = "authorization"
		if legacy {
			auth = "\"{authorization}\""
		}
	}
	headers := fmt.Sprintf("{\"Authorization\": %s, \"X-Cherri-Client\": %q}", auth, p15PaginateSyntheticClient)
	args := fmt.Sprintf("%q, \"items\", \"next\", %d, %d, %s", "https://example.invalid/pages/one.json", pages, items, headers)
	if named {
		args = fmt.Sprintf("%q, maxPages: %d, maxItems: %d, headers: %s", "https://example.invalid/pages/one.json", pages, items, headers)
	}
	if call == "downloadURL" {
		args = fmt.Sprintf("%q, %s", "https://example.invalid/pages/one.json", headers)
	}
	return prefix + call + "(" + args + ")\n"
}

// This oracle reads the actual native HTTP dictionary and authored Text producer.
// It does not execute Shortcuts or invent a binding for a missing producer.
func p15PaginateHeadersError(actions []ShortcutAction, dynamic bool) error {
	var request map[string]any
	requestIndex, producerIndex := -1, -1
	producerID, producerName := "", ""
	owners := map[string]int{}
	for index, action := range actions {
		p := action.WFWorkflowActionParameters
		if id, okay := p["UUID"].(string); okay && id != "" {
			owners[id]++
		}
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.downloadurl" {
			if request != nil {
				return fmt.Errorf("duplicate native request node")
			}
			request, requestIndex = p, index
		}
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.gettext" &&
			p["WFTextActionText"] == p15PaginateSyntheticAuthorization {
			if producerIndex >= 0 {
				return fmt.Errorf("duplicate authored authorization producer")
			}
			producerIndex = index
			producerID, _ = p["UUID"].(string)
			producerName, _ = p["CustomOutputName"].(string)
		}
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.alert" {
			message, okay := p["WFAlertActionMessage"].(string)
			if !okay || !strings.HasPrefix(message, "PAGINATE_") ||
				strings.Contains(message, p15PaginateSyntheticAuthorization) ||
				strings.Contains(message, "example.invalid") {
				return fmt.Errorf("pagination diagnostic leaked request data or lost its fixed code")
			}
		}
	}
	if request == nil || request["WFHTTPMethod"] != "GET" {
		return fmt.Errorf("missing exact GET request")
	}
	for _, key := range []string{"WFHTTPBodyType", "WFJSONValues", "WFFormValues", "WFRequestVariable", "ShowHeaders"} {
		if _, exists := request[key]; exists {
			return fmt.Errorf("unexpected request behavior %s", key)
		}
	}
	resolved := map[string]any{}
	if dynamic {
		if producerIndex < 0 || producerIndex >= requestIndex || producerID == "" || owners[producerID] != 1 {
			return fmt.Errorf("dynamic header lost its unique preceding authored producer")
		}
		resolved["ActionOutput:"+producerID] = p15PaginateSyntheticAuthorization
	} else if producerIndex >= 0 {
		return fmt.Errorf("literal headers gained an unrelated producer")
	}
	references := 0
	var inspect func(any) error
	inspect = func(raw any) error {
		switch value := raw.(type) {
		case map[string]any:
			if kind, exists := value["Type"]; exists {
				if !dynamic || kind != "ActionOutput" || value["OutputUUID"] != producerID {
					return fmt.Errorf("header references an unrelated or missing producer")
				}
				if name, exists := value["OutputName"]; exists && producerName != "" && name != producerName {
					return fmt.Errorf("header changed its actual producer name")
				}
				references++
			}
			for _, child := range value {
				if err := inspect(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range value {
				if err := inspect(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := inspect(request["WFHTTPHeaders"]); err != nil {
		return err
	}
	if (dynamic && references != 1) || (!dynamic && references != 0) {
		return fmt.Errorf("header attachment cardinality changed")
	}
	headers, err := p10DecodeJSONDictionary(request["WFHTTPHeaders"], resolved)
	if err != nil {
		return fmt.Errorf("native HTTP dictionary: %w", err)
	}
	expected := map[string]any{"Authorization": p15PaginateSyntheticAuthorization, "X-Cherri-Client": p15PaginateSyntheticClient}
	if !reflect.DeepEqual(headers, expected) {
		return fmt.Errorf("native request headers changed")
	}
	return nil
}

func p15PaginateCopyValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, child := range v {
			result[key] = p15PaginateCopyValue(child)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for index, child := range v {
			result[index] = p15PaginateCopyValue(child)
		}
		return result
	default:
		return value
	}
}

func TestP15PaginateHeadersUseExistingHTTPCodec(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			for _, dynamic := range []bool{false, true} {
				t.Run(fmt.Sprintf("legacy=%t/%s/dynamic=%t", legacy, profile, dynamic), func(t *testing.T) {
					// The existing manual GET is an independent baseline for the
					// request dictionary contract, not a pagination model.
					manual, err := compileBuildProfileFixture(t, p15PaginateHeadersFixture(legacy, dynamic, false, 3, 6, "downloadURL"), legacy, profile)
					if err != nil {
						t.Fatal("existing manual GET baseline", err)
					}
					if err := p15PaginateHeadersError(manual, dynamic); err != nil {
						t.Fatal("existing manual GET oracle", err)
					}
					actions, err := compileBuildProfileFixture(t, p15PaginateHeadersFixture(legacy, dynamic, false, 3, 6, "paginate"), legacy, profile)
					if err != nil {
						t.Fatal(err)
					}
					if err := p15PaginateHeadersError(actions, dynamic); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestP15PaginateHeadersNamedHolesAndZeroBudgets(t *testing.T) {
	for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
		t.Run(string(profile)+"/named", func(t *testing.T) {
			source := p15PaginateHeadersFixture(false, true, true, 3, 6, "paginate")
			svc := service.NewService(schema.DefaultRegistry())
			svc.OpenDocument("file:///paginate-headers.cherri", 1, source)
			diagnostics, _ := svc.Analyze("file:///paginate-headers.cherri")
			for _, diagnostic := range diagnostics {
				if diagnostic.Severity == analysis.SeverityError {
					t.Fatal(diagnostic)
				}
			}
			actions, err := compileBuildProfileFixture(t, source, false, profile)
			if err != nil {
				t.Fatal(err)
			}
			if err := p15PaginateHeadersError(actions, true); err != nil {
				t.Fatal(err)
			}
		})
		for _, legacy := range []bool{false, true} {
			for _, caps := range [][2]int{{0, 6}, {3, 0}} {
				t.Run(fmt.Sprintf("%s/legacy=%t/caps=%v", profile, legacy, caps), func(t *testing.T) {
					actions, err := compileBuildProfileFixture(t, p15PaginateHeadersFixture(legacy, false, false, caps[0], caps[1], "paginate"), legacy, profile)
					if err != nil {
						t.Fatal(err)
					}
					want := []string{"list", "setvariable", "getvariable"}
					if !legacy {
						want = append(want, "nothing")
					}
					if !reflect.DeepEqual(actionSuffixes(actions), want) {
						t.Fatal("zero-cap headers performed extra work")
					}
				})
			}
		}
	}
}

func TestP15PaginateHeadersOuterDictionaryRemainsLiteral(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		decl := "let"
		if legacy {
			decl = "const"
		}
		source := decl + " headers = {\"Authorization\": \"SYNTHETIC_ONLY\"}\npaginate(\"https://example.invalid/page\", \"items\", \"next\", 3, 6, headers)\n"
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run(fmt.Sprintf("legacy=%t/%s", legacy, profile), func(t *testing.T) {
				if _, err := compileBuildProfileFixture(t, source, legacy, profile); err == nil {
					t.Fatal("dynamic outer Dictionary bypassed native literal-container contract")
				}
			})
		}
		if !legacy {
			svc := service.NewService(schema.DefaultRegistry())
			svc.OpenDocument("file:///paginate-bad-headers.cherri", 1, source)
			diagnostics, _ := svc.Analyze("file:///paginate-bad-headers.cherri")
			found := false
			for _, diagnostic := range diagnostics {
				found = found || diagnostic.Severity == analysis.SeverityError
			}
			if !found {
				t.Fatal("editor accepted unsupported dynamic header container")
			}
		}
	}
}

func TestP15PaginateHeadersOracleRejectsRequestCorruption(t *testing.T) {
	// The unchanged manual action supplies a healthy independent control even
	// before paginate learns the new argument.
	actions, err := compileBuildProfileFixture(t, p15PaginateHeadersFixture(false, true, false, 3, 6, "downloadURL"), false, BuildProfileRelease)
	if err != nil {
		t.Fatal(err)
	}
	if err := p15PaginateHeadersError(actions, true); err != nil {
		t.Fatal("healthy manual control", err)
	}
	for _, fault := range []string{"drop_headers", "wrong_wrapper", "drop_field", "duplicate_field", "wrong_value", "orphan_reference", "drop_producer", "future_producer", "duplicate_producer", "post_method", "request_body", "request_duplicate"} {
		t.Run(fault, func(t *testing.T) {
			broken := make([]ShortcutAction, len(actions))
			request, producer := -1, -1
			for index, action := range actions {
				broken[index] = ShortcutAction{WFWorkflowActionIdentifier: action.WFWorkflowActionIdentifier}
				if action.WFWorkflowActionParameters != nil {
					broken[index].WFWorkflowActionParameters = p15PaginateCopyValue(action.WFWorkflowActionParameters).(map[string]any)
				}
				if action.WFWorkflowActionIdentifier == "is.workflow.actions.downloadurl" {
					request = index
				}
				if action.WFWorkflowActionIdentifier == "is.workflow.actions.gettext" && action.WFWorkflowActionParameters["WFTextActionText"] == p15PaginateSyntheticAuthorization {
					producer = index
				}
			}
			p := broken[request].WFWorkflowActionParameters
			switch fault {
			case "drop_headers":
				delete(p, "WFHTTPHeaders")
			case "wrong_wrapper":
				p["WFHTTPHeaders"].(map[string]any)["WFSerializationType"] = "WFTextTokenAttachment"
			case "drop_field", "duplicate_field", "wrong_value", "orphan_reference":
				value := p["WFHTTPHeaders"].(map[string]any)["Value"].(map[string]any)
				rows := value["WFDictionaryFieldValueItems"].([]any)
				switch fault {
				case "drop_field":
					value["WFDictionaryFieldValueItems"] = rows[:1]
				case "duplicate_field":
					value["WFDictionaryFieldValueItems"] = append(rows, p15PaginateCopyValue(rows[0]))
				case "wrong_value":
					// Mutate the actual authored producer, not an assumed field order.
					broken[producer].WFWorkflowActionParameters["WFTextActionText"] = "OTHER_SYNTHETIC_VALUE"
				case "orphan_reference":
					var corrupt func(any)
					corrupt = func(raw any) {
						switch v := raw.(type) {
						case map[string]any:
							if v["Type"] == "ActionOutput" {
								v["OutputUUID"] = "MISSING_HEADER_PRODUCER"
							}
							for _, child := range v {
								corrupt(child)
							}
						case []any:
							for _, child := range v {
								corrupt(child)
							}
						}
					}
					corrupt(value)
				}
			case "drop_producer":
				delete(broken[producer].WFWorkflowActionParameters, "UUID")
			case "future_producer":
				broken[producer], broken[request] = broken[request], broken[producer]
			case "duplicate_producer":
				broken = append(broken, broken[producer])
			case "post_method":
				p["WFHTTPMethod"] = "POST"
			case "request_body":
				p["WFRequestVariable"] = "UNRELATED_BODY"
			case "request_duplicate":
				broken = append(broken, broken[request])
			}
			if p15PaginateHeadersError(broken, true) == nil {
				t.Fatal("corrupted native request accepted")
			}
		})
	}
}
