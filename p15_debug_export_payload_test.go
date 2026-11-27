package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func p15PayloadGraph(t *testing.T, legacy bool, options string) []ShortcutAction {
	t.Helper()
	decl := "let"
	if legacy {
		decl = "const"
	}
	src := "debugClearLogs()\n" + decl + " original = text(\"PAYLOAD_RAW_SENTINEL\")\ndebugLog()\ntext(\"CHERRI_EXPORT_BEFORE\")\n" + decl + " exported = debugExport(" + options + ")\ngetVariable(original)\ngetVariable(exported)\n"
	return p15ExportMetadataGraph(t, compileDebugFixture(t, src, legacy))
}

func TestP15DebugExportTypedPayloadSource(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, format := range []string{"json", "text"} {
			for _, metadata := range []bool{false, true} {
				t.Run(fmt.Sprintf("legacy_%t/%s/metadata_%t", legacy, format, metadata), func(t *testing.T) {
					options := fmt.Sprintf("%q,%t,true,false,%t", format, metadata, metadata)
					graph := p15PayloadGraph(t, legacy, options)
					data, err := json.Marshal(graph)
					if err != nil {
						t.Fatal(err)
					}
					for _, marker := range []string{"NATIVE_NUMBER_TEXT_ENCODED", "DATE_ISO8601_ENCODED", "DICTIONARY_FIELD_DIAGNOSTICS", "ABSENT_NULL_OR_EMPTY_COLLECTION", "UNSUPPORTED_MULTIPLE_NATIVE_ITEMS", "UNSUPPORTED_NESTED_DICTIONARY", "source-types-not-reconstructed"} {
						if !strings.Contains(string(data), marker) {
							t.Fatalf("missing original typed diagnostic obligation %s", marker)
						}
					}
					if path := os.Getenv("CHERRI_EXPORT_PAYLOAD_GRAPHS"); path != "" {
						if err := os.MkdirAll(path, 0755); err != nil {
							t.Fatal(err)
						}
						record := map[string]any{"legacy": legacy, "format": format, "metadata": metadata, "redact": false, "keys": []string{}, "actions": p15ExportMetadataNativeWire(t, graph)}
						encoded, err := json.MarshalIndent(record, "", "  ")
						if err != nil {
							t.Fatal(err)
						}
						if err = os.WriteFile(filepath.Join(path, fmt.Sprintf("typed-%t-%s-%t.json", legacy, format, metadata)), append(encoded, '\n'), 0644); err != nil {
							t.Fatal(err)
						}
					}
				})
			}
		}
	}
}

func TestP15DebugExportSelectiveFieldsSource(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, redact := range []bool{false, true} {
			for _, metadata := range []bool{false, true} {
				t.Run(fmt.Sprintf("legacy_%t/redact_%t/metadata_%t", legacy, redact, metadata), func(t *testing.T) {
					options := fmt.Sprintf("\"json\",%t,true,%t,%t,[\"My.Secret\",\"סוד\"]", metadata, redact, metadata)
					graph := p15PayloadGraph(t, legacy, options)
					data, err := json.Marshal(graph)
					if err != nil {
						t.Fatal(err)
					}
					for _, marker := range []string{"REDACTED_SENSITIVE_FIELD", "UNSUPPORTED_KEY_PATH", "DICTIONARY_FIELD_DIAGNOSTICS", "my", "authorization", "URL", "source-types-not-reconstructed"} {
						if !strings.Contains(strings.ToLower(string(data)), strings.ToLower(marker)) {
							t.Fatalf("missing selective contract marker %s", marker)
						}
					}
					for _, a := range graph {
						p := a.WFWorkflowActionParameters
						if a.WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" || a.WFWorkflowActionIdentifier == "is.workflow.actions.appendvariable" {
							name, _ := p["WFVariableName"].(string)
							if !strings.HasPrefix(name, "__cherri_debug_export_") {
								t.Fatalf("mutated raw or external state %q", name)
							}
						}
					}
					if path := os.Getenv("CHERRI_EXPORT_PAYLOAD_GRAPHS"); path != "" {
						if err := os.MkdirAll(path, 0755); err != nil {
							t.Fatal(err)
						}
						record := map[string]any{"legacy": legacy, "format": "json", "metadata": metadata, "redact": redact, "keys": []string{"My.Secret", "סוד"}, "actions": p15ExportMetadataNativeWire(t, graph)}
						encoded, err := json.MarshalIndent(record, "", "  ")
						if err != nil {
							t.Fatal(err)
						}
						if err = os.WriteFile(filepath.Join(path, fmt.Sprintf("selective-%t-%t-%t.json", legacy, redact, metadata)), append(encoded, '\n'), 0644); err != nil {
							t.Fatal(err)
						}
					}
				})
			}
		}
	}
}

func TestP15DebugExportSelectiveOptionValidation(t *testing.T) {
	for i, value := range []any{nil, "secret", true, 12, debugLogValueRef("runtimeKeys"), []any{"secret", nil}, []any{"secret", true}, []any{""}, []any{strings.Repeat("a", 129)}, []any{"with\nnewline"}} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if _, err := parseDebugExportOptions(map[string]any{"WFDebugExportRedactKeys": value}); err == nil {
				t.Fatalf("malformed/dynamic selective keys accepted: %#v", value)
			}
		})
	}
	for _, value := range []any{[]any{}, []any{"Secret", "סוד", "my.field"}} {
		if _, err := parseDebugExportOptions(map[string]any{"WFDebugExportRedactKeys": value}); err != nil {
			t.Fatal(err)
		}
	}
	tooMany := make([]any, 65)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("key%d", i)
	}
	if _, err := parseDebugExportOptions(map[string]any{"WFDebugExportRedactKeys": tooMany}); err == nil {
		t.Fatal("unbounded selective key table accepted")
	}
}

func TestP15DebugExportEmptySelectiveKeysStayRedacted(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		absent := p15PayloadGraph(t, legacy, "")
		empty := p15PayloadGraph(t, legacy, "\"json\",false,true,true,false,[]")
		a, err := p15ExportMetadataCanonicalGraph(p15ExportMetadataNativeWire(t, absent))
		if err != nil {
			t.Fatal(err)
		}
		b, err := p15ExportMetadataCanonicalGraph(p15ExportMetadataNativeWire(t, empty))
		if err != nil {
			t.Fatal(err)
		}
		x, _ := json.Marshal(a)
		y, _ := json.Marshal(b)
		if string(x) != string(y) {
			t.Fatal("empty selective keys widened default disclosure or changed default graph")
		}
	}
}

// Opting into metadata or typed payloads must retain the compiler's public
// deterministic identity policy, including private names and control groups.
func TestP15DebugExportDerivedUUIDReplay(t *testing.T) {
	t.Cleanup(resetCompilerStateFully)
	for _, legacy := range []bool{true, false} {
		for _, mode := range []struct{ name, options string }{
			{"metadata_redacted", `"json",true,true,true,true`},
			{"typed_metadata", `"json",true,true,false,true`},
			{"selective", `"json",false,true,true,false,["secret"]`},
		} {
			t.Run(fmt.Sprintf("legacy_%t/%s", legacy, mode.name), func(t *testing.T) {
				source := "debugClearLogs()\ntext(\"UUID payload\")\ndebugLog()\nconst exported = debugExport(" + mode.options + ")\ngetVariable(exported)\n"
				source = derivedUUIDSource(t, source, legacy)
				first := compileDerivedUUID(t, source, "DebugExportReplay", true, legacy)
				second := compileDerivedUUID(t, source, "DebugExportReplay", true, legacy)
				derivedUUIDGraph(t, first)
				derivedUUIDGraph(t, second)
				if !reflect.DeepEqual(first, second) {
					t.Fatalf("debugExport opt-in bypassed derived identity: %s", firstPlistDiffPath(first, second, ""))
				}
			})
		}
	}
}

func TestP15DebugExportPayloadCherriFixture(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("tests", "debug-export-payload-diagnostics.cherri"))
	if err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []bool{true, false} {
		t.Run(fmt.Sprintf("legacy_%t", legacy), func(t *testing.T) {
			compileDebugFixture(t, derivedUUIDSource(t, string(source), legacy), legacy)
		})
	}
}
