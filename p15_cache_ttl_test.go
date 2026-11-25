package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type p15CacheScalarCase struct {
	name, literal, kind string
	global, sensitive   bool
}

func p15CacheScalarCases() []p15CacheScalarCase {
	return []p15CacheScalarCase{
		{"unicode_text", "\"שלום 🍒\\nquoted \\\"value\\\"\"", "text", false, false},
		{"empty_text", "\"\"", "text", false, false},
		{"zero_number", "0", "number", false, false},
		{"fraction_number", "-2.5", "number", false, false},
		{"false_boolean", "false", "bool", false, false},
		{"true_boolean", "true", "bool", false, false},
		{"global_text", "\"shared value\"", "text", true, false},
		{"sensitive_opt_in", "\"explicitly allowed\"", "text", false, true},
	}
}
func p15CacheScalarSource(tc p15CacheScalarCase, legacy bool, ttl int) string {
	if legacy {
		return fmt.Sprintf("#define version 27\nconst cached = cache(\"P15_SCALAR_KEY\", %d, %t, %t, %t) {\ntext(\"P15_CACHE_BODY_ONCE\")\nyield %s\n}\ngetVariable(cached)\n", ttl, tc.global, tc.sensitive, tc.sensitive, tc.literal)
	}
	return fmt.Sprintf("shortcut \"cache scalar\" { version: \"27\" }\nlet cached = cache(key:\"P15_SCALAR_KEY\", ttl:%d, global:%t, sensitive:%t, allowSensitive:%t) {\ntext(\"P15_CACHE_BODY_ONCE\")\nyield %s\n}\nreturn cached\n", ttl, tc.global, tc.sensitive, tc.sensitive, tc.literal)
}
func p15CacheCompileRecord(t *testing.T, source string, legacy bool, profile BuildProfile) ([]ShortcutAction, error) {
	t.Helper()
	actual, err := compileBuildProfileFixture(t, source, legacy, profile)
	if out := os.Getenv("P15_CACHE_TTL_OUT"); out != "" {
		record := map[string]any{"test": t.Name(), "source": source, "legacy": legacy, "profile": profile, "actions": actual}
		if err != nil {
			record["error"] = err.Error()
		}
		data, encodeErr := json.MarshalIndent(record, "", "  ")
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		name := strings.NewReplacer("/", "_", "\\", "_").Replace(t.Name()) + ".json"
		if writeErr := os.WriteFile(filepath.Join(out, name), append(data, '\n'), 0600); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	return actual, err
}
func p15CacheNamespace(kind string, global bool) string {
	sum := sha256.Sum256([]byte("P15_SCALAR_KEY"))
	scope := "local"
	if global {
		scope = "global"
	}
	return "cherri.cache.v1." + scope + "." + kind + "." + hex.EncodeToString(sum[:])
}
func p15CacheGraphError(actual []ShortcutAction, tc p15CacheScalarCase, ttl int) error {
	if len(actual) == 0 {
		return fmt.Errorf("empty native graph")
	}
	reads, writes, body, bodyIndex, writeIndex := 0, 0, 0, -1, -1
	producers := map[string]int{}
	memberships := make([][]string, len(actual))
	var stack []string
	groupKinds := map[string]string{}
	for i, a := range actual {
		p := a.WFWorkflowActionParameters
		id, _ := p["UUID"].(string)
		if id != "" {
			if _, exists := producers[id]; exists {
				return fmt.Errorf("duplicate UUID")
			}
			producers[id] = i
		}
		kind := strings.TrimPrefix(a.WFWorkflowActionIdentifier, "is.workflow.actions.")
		if group, ok := p["GroupingIdentifier"].(string); ok {
			mode := fmt.Sprint(p["WFControlFlowMode"])
			switch mode {
			case "0":
				if _, exists := groupKinds[group]; exists {
					return fmt.Errorf("duplicate control group")
				}
				groupKinds[group] = kind
				stack = append(stack, group)
			case "1":
				if len(stack) == 0 || stack[len(stack)-1] != group {
					return fmt.Errorf("unmatched else")
				}
			case "2":
				if len(stack) == 0 || stack[len(stack)-1] != group {
					return fmt.Errorf("unmatched end")
				}
				stack = stack[:len(stack)-1]
			default:
				return fmt.Errorf("unexpected control mode %q", mode)
			}
		}
		memberships[i] = append([]string(nil), stack...)
		switch kind {
		case "getstoredcontent", "setstoredcontent":
			if p["WFStoredContentKey"] != p15CacheNamespace(tc.kind, tc.global) || p["WFStoredContentGlobalValue"] != tc.global {
				return fmt.Errorf("wrong namespace/scope: %v", p)
			}
			if kind == "getstoredcontent" {
				reads++
			} else {
				writes++
				writeIndex = i
			}
		case "gettext":
			if p["WFTextActionText"] == "P15_CACHE_BODY_ONCE" {
				body++
				bodyIndex = i
			}
		case "runworkflow":
			return fmt.Errorf("cache body became a separate shortcut")
		}
	}
	if len(stack) != 0 {
		return fmt.Errorf("unclosed native control")
	}
	var refs func(any, int) error
	refs = func(v any, i int) error {
		switch x := v.(type) {
		case map[string]any:
			if x["Type"] == "ActionOutput" {
				id, _ := x["OutputUUID"].(string)
				if producer, okay := producers[id]; !okay || producer >= i {
					return fmt.Errorf("missing/forward producer %q at %d", id, i)
				}
			}
			for _, value := range x {
				if err := refs(value, i); err != nil {
					return err
				}
			}
		case []any:
			for _, value := range x {
				if err := refs(value, i); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for i, a := range actual {
		if err := refs(a.WFWorkflowActionParameters, i); err != nil {
			return err
		}
	}
	if body != 1 {
		return fmt.Errorf("body effect count %d", body)
	}
	if ttl == 0 {
		if reads != 0 || writes != 0 {
			return fmt.Errorf("zero TTL retained persistence")
		}
		for _, a := range actual {
			switch a.WFWorkflowActionIdentifier {
			case "is.workflow.actions.date", "is.workflow.actions.gettimebetweendates", "is.workflow.actions.detect.dictionary":
				return fmt.Errorf("zero TTL retained cache lifecycle %s", a.WFWorkflowActionIdentifier)
			}
		}
		return nil
	}
	if reads != 1 || writes != 1 {
		return fmt.Errorf("get/set inventory %d/%d", reads, writes)
	}
	if bodyIndex < 0 || writeIndex <= bodyIndex {
		return fmt.Errorf("body did not precede store")
	}
	if len(memberships[bodyIndex]) == 0 {
		return fmt.Errorf("body eagerly emitted outside miss branch")
	}
	branch := memberships[bodyIndex][len(memberships[bodyIndex])-1]
	if groupKinds[branch] != "conditional" {
		return fmt.Errorf("miss is not an actual native conditional")
	}
	found := false
	for _, g := range memberships[writeIndex] {
		found = found || g == branch
	}
	if !found {
		return fmt.Errorf("store escaped body miss branch")
	}
	dates := len(p15ActionsNamed(actual, "gettimebetweendates"))
	if dates != 2 {
		return fmt.Errorf("need separate hit time and post-body creation time, got %d", dates)
	}
	return nil
}
func TestP15CacheScalarLazyGraphs(t *testing.T) {
	for _, tc := range p15CacheScalarCases() {
		for _, legacy := range []bool{false, true} {
			for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
				t.Run(fmt.Sprintf("%s/legacy=%t/%s", tc.name, legacy, profile), func(t *testing.T) {
					actual, err := p15CacheCompileRecord(t, p15CacheScalarSource(tc, legacy, 300), legacy, profile)
					if err != nil {
						t.Fatalf("cache capability missing: %v", err)
					}
					if err := p15CacheGraphError(actual, tc, 300); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
func TestP15CacheZeroTTLBypassesPersistence(t *testing.T) {
	tc := p15CacheScalarCases()[0]
	for _, legacy := range []bool{false, true} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run(fmt.Sprintf("legacy=%t/%s", legacy, profile), func(t *testing.T) {
				actual, err := p15CacheCompileRecord(t, p15CacheScalarSource(tc, legacy, 0), legacy, profile)
				if err != nil {
					t.Fatalf("zero TTL cache capability missing: %v", err)
				}
				if err := p15CacheGraphError(actual, tc, 0); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
func TestP15CacheExistingScopedValueControl(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run(fmt.Sprintf("legacy=%t/%s", legacy, profile), func(t *testing.T) {
				source := "let measured = measure(\"ordinary control\") { text(\"P15_CACHE_MEASURE_CONTROL\") }\nreturn measured"
				if legacy {
					source = "const measured = measure(\"ordinary control\") {\ntext(\"P15_CACHE_MEASURE_CONTROL\")\n}\ngetVariable(measured)\n"
				}
				actual, err := p15CacheCompileRecord(t, source, legacy, profile)
				if err != nil {
					t.Fatal(err)
				}
				found := 0
				for _, a := range p15ActionsNamed(actual, "gettext") {
					if a.WFWorkflowActionParameters["WFTextActionText"] == "P15_CACHE_MEASURE_CONTROL" {
						found++
					}
				}
				if found != 1 {
					t.Fatalf("existing value body duplicated/lost: %d", found)
				}
			})
		}
	}
}
