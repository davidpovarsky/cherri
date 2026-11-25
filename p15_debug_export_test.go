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
		{"setitemname", "WFSetItemNameAction", "WFInput"},
	} {
		entry := native["is.workflow.actions."+tc.suffix]
		if entry["ActionClass"] != tc.class || debugParamMap(entry["Input"])["ParameterKey"] != tc.input {
			t.Fatalf("independent native contract changed: %s", tc.suffix)
		}
	}
	fileTypes := debugParamMap(native["is.workflow.actions.base64encode"]["Output"])["Types"]
	if !reflect.DeepEqual(fileTypes, []any{"WFStringContentItem", "public.data"}) {
		t.Fatalf("Base64 native Text/File output contract changed: %v", fileTypes)
	}
	var hasNone, hasDecode bool
	for _, raw := range native["is.workflow.actions.base64encode"]["Parameters"].([]any) {
		param := debugParamMap(raw)
		if param["Key"] == "WFEncodeMode" {
			for _, item := range param["Items"].([]any) {
				hasDecode = hasDecode || item == "Decode"
			}
		}
		if param["Key"] == "WFBase64LineBreakMode" {
			for _, item := range param["Items"].([]any) {
				hasNone = hasNone || item == "None"
			}
		}
	}
	if !hasNone || !hasDecode {
		t.Fatal("native Base64 Decode/no-line-break contract missing")
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
				consumerEnd := len(acts)
				if !legacy {
					checkDebugMainVoidBoundary(t, acts)
					consumerEnd--
				}
				tail := acts[consumerEnd-2 : consumerEnd]
				if tail[0].WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" || tail[1].WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" {
					t.Fatal("authored original/export consumers were replaced")
				}
				originalRef := debugParamMap(debugParamMap(tail[0].WFWorkflowActionParameters["WFVariable"])["Value"])
				if originalRef["OutputUUID"] != original {
					t.Fatal("export rewrote the raw producer identity")
				}
				exported := acts[consumerEnd-3]
				if exported.WFWorkflowActionIdentifier != "is.workflow.actions.setitemname" {
					t.Fatal("export did not end in named File")
				}
				exportRef := debugParamMap(debugParamMap(tail[1].WFWorkflowActionParameters["WFVariable"])["Value"])
				if exportRef["OutputUUID"] != exported.WFWorkflowActionParameters["UUID"] {
					t.Fatal("returned File reference changed")
				}
				fileEncoderUUID := acts[consumerEnd-5].WFWorkflowActionParameters["UUID"]
				fileDecoderUUID := acts[consumerEnd-4].WFWorkflowActionParameters["UUID"]
				seen := map[string]map[string]any{}
				for _, a := range acts[4 : consumerEnd-2] {
					p := a.WFWorkflowActionParameters
					id := strings.TrimPrefix(a.WFWorkflowActionIdentifier, "is.workflow.actions.")
					switch id {
					case "gettext", "getitemtype", "base64encode", "repeat.each", "conditional", "text.combine", "setitemname":
					default:
						t.Fatalf("export performs unexpected external effect or mutation: %s", id)
					}
					if id == "base64encode" {
						ref := debugParamMap(debugParamMap(p["WFInput"])["Value"])
						switch p["WFEncodeMode"] {
						case "Encode":
							if p["WFBase64LineBreakMode"] != "None" {
								t.Fatal("unsafe base64 line breaks")
							}
							if (option == "" || strings.HasSuffix(option, "false, true")) && p["UUID"] != fileEncoderUUID {
								producer, exists := seen[fmt.Sprint(ref["OutputUUID"])]
								if !exists || producer["id"] != "getitemtype" {
									t.Fatal("default export encodes raw payload")
								}
							}
						case "Decode":
							if p["UUID"] != fileDecoderUUID || ref["OutputUUID"] != fileEncoderUUID {
								t.Fatal("only the completed envelope may become a decoded File")
							}
						default:
							t.Fatal("unsafe base64 direction")
						}
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
				"debugExport(\"../escape.json\")\n",
			} {
				if _, err := compileBuildProfileFixture(t, bad, legacy, profile); err == nil {
					t.Fatalf("legacy=%t profile=%s accepted invalid export: %q", legacy, profile, bad)
				}
			}
			// This exact historical unsupported-date source is now an
			// admitted opt-in. Keep it in both profiles and frontends.
			dated, err := compileBuildProfileFixture(t, "debugExport(\"json\", true)\n", legacy, profile)
			if err != nil {
				t.Fatalf("admitted date source failed: legacy=%t profile=%s: %v", legacy, profile, err)
			}
			if profile == BuildProfileRelease {
				validate := func(actions []ShortcutAction) error {
					if legacy {
						// Legacy has no synthetic main fallthrough action. Nil
						// and empty slices are the same zero-action graph.
						if len(actions) == 0 {
							return nil
						}
					} else if len(actions) == 1 {
						return debugMainVoidBoundaryError(actions)
					}
					return fmt.Errorf("Release export leaked actions: %#v", actions)
				}
				if err := validate(dated); err != nil {
					t.Fatal(err)
				}
				for _, leak := range []string{"gettext", "quicklook", "nothing"} {
					t.Run(fmt.Sprintf("release_legacy_%t/leaked_%s", legacy, leak), func(t *testing.T) {
						mutated := append(append([]ShortcutAction(nil), dated...), ShortcutAction{
							WFWorkflowActionIdentifier: "is.workflow.actions." + leak,
						})
						if err := validate(mutated); err == nil {
							t.Fatalf("Release oracle accepted an additional %s action", leak)
						}
						if err := validate(dated); err != nil {
							t.Fatalf("unchanged Release control no longer accepted: %v", err)
						}
					})
				}
			} else {
				last := -1
				for i, action := range dated {
					if action.WFWorkflowActionIdentifier == "is.workflow.actions.setitemname" {
						last = i
					}
				}
				if last < 0 {
					t.Fatal("date source lost original export boundary")
				}
				if err := p15ExportMetadataGraphError(dated[:last+1], p15ExportMetadataOptions{dates: true, types: true, redact: true, format: "json"}); err != nil {
					t.Fatal(err)
				}
			}
			decl := "let"
			if legacy {
				decl = "const"
			}
			_, err = compileBuildProfileFixture(t, decl+" dynamic = text(\"json\")\ndebugExport(dynamic)\n", legacy, profile)
			if err == nil {
				t.Fatal("dynamic format accepted")
			}
		}
		release, err := compileBuildProfileFixture(t, "text(\"kept producer\")\ndebugLog()\ndebugExport()\n", legacy, BuildProfileRelease)
		if err != nil {
			t.Fatal(err)
		}
		checkDebugFallthroughSequence(t, release, legacy, "gettext")
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
			wantCount := 2 + 2*count + 10
			if !legacy {
				checkDebugMainVoidBoundary(t, acts)
				wantCount++
			}
			if len(acts) != wantCount {
				t.Fatalf("legacy=%t count=%d: unbounded expansion %d", legacy, count, len(acts))
			}
		}
	}
}

func TestP15DebugExportRedactionRejectsNilAndDynamic(t *testing.T) {
	for _, raw := range []any{nil, "false", 0, debugLogValueRef("runtimeFlag")} {
		for _, key := range []string{"WFDebugExportRedact", "WFDebugExportTypes", "WFDebugExportDates", "WFDebugExportLabels"} {
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
