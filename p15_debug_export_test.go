package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/types"
	"howett.net/plist"
)

func TestP15DebugExportCanonicalNativeMetadata(t *testing.T) {
	data, err := os.ReadFile("docs/apple-runtime/macos-27/builtin-actions.json")
	if err != nil {
		t.Fatal(err)
	}
	var native map[string]map[string]any
	if err := json.Unmarshal(data, &native); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ suffix, class, input string }{
		{"getitemtype", "WFGetItemTypeAction", "WFInput"},
		{"base64encode", "WFBase64EncodingAction", "WFInput"},
		{"gettypeaction", "WFGetTypeAction", "WFInput"},
		{"setitemname", "WFSetItemNameAction", "WFInput"},
	} {
		entry := native["is.workflow.actions."+tc.suffix]
		if entry["ActionClass"] != tc.class || debugParamMap(entry["Input"])["ParameterKey"] != tc.input {
			t.Fatalf("independent native contract changed: %s", tc.suffix)
		}
	}
	fileTypes := debugParamMap(native["is.workflow.actions.gettypeaction"]["Output"])["Types"]
	if !reflect.DeepEqual(fileTypes, []any{"public.data"}) {
		t.Fatalf("Get File of Type is not a File witness: %v", fileTypes)
	}
	var hasNone bool
	for _, raw := range native["is.workflow.actions.base64encode"]["Parameters"].([]any) {
		param := debugParamMap(raw)
		if param["Key"] == "WFBase64LineBreakMode" {
			for _, item := range param["Items"].([]any) {
				hasNone = hasNone || item == "None"
			}
		}
	}
	if !hasNone {
		t.Fatal("native base64 no-line-break contract missing")
	}
	macro, ok := schema.DefaultRegistry().LookupAction("debugExport")
	if !ok || macro.OutputType != types.File || !macro.CompilerConstruct {
		t.Fatal("File macro schema missing")
	}
	for _, param := range macro.Parameters {
		if !param.Literal {
			t.Fatalf("%s must be literal", param.ID)
		}
	}
	if len(schema.DefaultRegistry().AllActions()) != 505 {
		t.Fatal("native catalog denominator changed")
	}
}

func TestP15DebugExportBothFrontendsPrivacyAndIdentity(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, option := range []string{"", "\"json\", false, false, true", "\"json\", false, true, false", "\"text\", false, true, false"} {
			t.Run(fmt.Sprintf("legacy=%t/options=%s", legacy, option), func(t *testing.T) {
				decl := "let"
				if legacy {
					decl = "const"
				}
				src := "debugClearLogs()\n" + decl + " original = text(\"synthetic-private-token\")\ndebugLog()\n" +
					decl + " exported = debugExport(" + option + ")\ngetVariable(original)\ngetVariable(exported)\n"
				acts := compileDebugFixture(t, src, legacy)
				original := acts[2].WFWorkflowActionParameters["UUID"]
				appendRef := debugParamMap(debugParamMap(acts[3].WFWorkflowActionParameters["WFInput"])["Value"])
				if appendRef["OutputUUID"] != original {
					t.Fatal("original log reference changed")
				}
				tail := acts[len(acts)-2:]
				originalRef := debugParamMap(debugParamMap(tail[0].WFWorkflowActionParameters["WFVariable"])["Value"])
				if originalRef["OutputUUID"] != original {
					t.Fatal("export rewrote the raw producer identity")
				}
				exported := acts[len(acts)-3]
				if exported.WFWorkflowActionIdentifier != "is.workflow.actions.setitemname" {
					t.Fatal("export did not end in named File")
				}
				exportRef := debugParamMap(debugParamMap(tail[1].WFWorkflowActionParameters["WFVariable"])["Value"])
				if exportRef["OutputUUID"] != exported.WFWorkflowActionParameters["UUID"] {
					t.Fatal("returned File reference changed")
				}
				seen := map[string]map[string]any{}
				for _, a := range acts[4 : len(acts)-2] {
					p := a.WFWorkflowActionParameters
					id := strings.TrimPrefix(a.WFWorkflowActionIdentifier, "is.workflow.actions.")
					switch id {
					case "gettext", "getitemtype", "base64encode", "repeat.each", "conditional", "text.combine", "gettypeaction", "setitemname":
					default:
						t.Fatalf("export performs unexpected external effect or mutation: %s", id)
					}
					if id == "base64encode" {
						if p["WFEncodeMode"] != "Encode" || p["WFBase64LineBreakMode"] != "None" {
							t.Fatal("unsafe base64 contract")
						}
						ref := debugParamMap(debugParamMap(p["WFInput"])["Value"])
						if option == "" || strings.HasSuffix(option, "false, true") {
							producer, exists := seen[fmt.Sprint(ref["OutputUUID"])]
							if !exists || producer["id"] != "getitemtype" {
								t.Fatal("default export encodes raw payload")
							}
						}
					}
					if id == "gettypeaction" && p["WFFileType"] != "public.utf8-plain-text" {
						t.Fatal("file content encoding changed")
					}
					if id == "setitemname" && p["WFName"] != "cherri-debug.json" && p["WFName"] != "cherri-debug.txt" {
						t.Fatal("uncontrolled export filename")
					}
					if u, ok := p["UUID"].(string); ok {
						seen[u] = map[string]any{"id": id}
					}
				}
			})
		}
	}
}

func TestP15DebugExportLiteralOptionsAndRelease(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			for _, bad := range []string{
				"debugExport(\"xml\")\n",
				"debugExport(\"json\", true)\n",
				"debugExport(\"../escape.json\")\n",
			} {
				if _, err := compileBuildProfileFixture(t, bad, legacy, profile); err == nil {
					t.Fatalf("legacy=%t profile=%s accepted invalid export: %q", legacy, profile, bad)
				}
			}
			decl := "let"
			if legacy {
				decl = "const"
			}
			_, err := compileBuildProfileFixture(t, decl+" dynamic = text(\"json\")\ndebugExport(dynamic)\n", legacy, profile)
			if err == nil {
				t.Fatal("dynamic format accepted")
			}
		}
		release, err := compileBuildProfileFixture(t, "text(\"kept producer\")\ndebugLog()\ndebugExport()\n", legacy, BuildProfileRelease)
		if err != nil {
			t.Fatal(err)
		}
		if len(release) != 1 || release[0].WFWorkflowActionIdentifier != "is.workflow.actions.gettext" {
			t.Fatalf("Release retained export or removed source: %v", actionSuffixes(release))
		}
		decl := "let"
		if legacy {
			decl = "const"
		}
		_, err = compileBuildProfileFixture(t, decl+" artifact = debugExport()\ngetVariable(artifact)\n", legacy, BuildProfileRelease)
		if err == nil || !strings.Contains(err.Error(), "RELEASE_DIAGNOSTIC_VALUE") {
			t.Fatalf("Release did not reject consumed removed export: %v", err)
		}
	}
}

func TestP15DebugExportZeroOneThousandCompileBudget(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, count := range []int{0, 1, 1000} {
			src := "debugClearLogs()\n" + strings.Repeat("text(\"entry\")\ndebugLog()\n", count) + "debugExport()\n"
			acts := compileDebugFixture(t, src, legacy)
			if len(acts) != 2+2*count+9 {
				t.Fatalf("legacy=%t count=%d: unbounded expansion %d", legacy, count, len(acts))
			}
		}
	}
}

func TestP15DebugExportRedactionRejectsNilAndDynamic(t *testing.T) {
	for _, raw := range []any{nil, "false", 0, debugLogValueRef("runtimeFlag")} {
		for _, key := range []string{"WFDebugExportRedact", "WFDebugExportTypes", "WFDebugExportDates"} {
			if _, err := parseDebugExportOptions(map[string]any{key: raw}); err == nil {
				t.Fatalf("nonliteral option %s=%T accepted; it must not weaken default redaction", key, raw)
			}
		}
	}
}

func TestP15DebugExportCanonicalTextCodecUTF16Identity(t *testing.T) {
	encoded, err := plist.Marshal(debugExportTextToken("\u05e9\u05dc\u05d5\u05dd\U0001f352 ", debugOutputRefNamed("EXACT-A", "Value-A"), " / ", debugOutputRefNamed("EXACT-B", "Value-B")), plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	var token map[string]any
	if _, err := plist.Unmarshal(encoded, &token); err != nil {
		t.Fatal(err)
	}
	value := debugParamMap(token["Value"])
	if token["WFSerializationType"] != "WFTextTokenString" || value["string"] != "\u05e9\u05dc\u05d5\u05dd\U0001f352 \uFFFC / \uFFFC" {
		t.Fatal("canonical Unicode text serialization changed")
	}
	ranges := debugParamMap(value["attachmentsByRange"])
	if len(ranges) != 2 {
		t.Fatal("attachment count changed")
	}
	for key, expected := range map[string]string{"{7, 1}": "A", "{11, 1}": "B"} {
		ref := debugParamMap(ranges[key])
		if ref["Type"] != "ActionOutput" || ref["OutputUUID"] != "EXACT-"+expected || ref["OutputName"] != "Value-"+expected {
			t.Fatalf("canonical UTF-16 range or exact UUID/name changed: %s %v", key, ref)
		}
	}
	if literal := debugExportTextToken("literal", " only"); literal != "literal only" {
		t.Fatalf("literal-only text bypassed canonical normalization: %v", literal)
	}
}
