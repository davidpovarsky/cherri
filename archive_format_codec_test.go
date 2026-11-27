package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"howett.net/plist"
)

type archiveFormatFixture struct {
	source string
	wire   string
}

func archiveFormatNativeFixtures(t *testing.T) []archiveFormatFixture {
	t.Helper()
	var data, err = os.ReadFile("docs/apple-runtime/macos-27/toolkit-registry.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		Tools []struct {
			Identifier string
			Parameters []struct {
				Key  string
				Type struct {
					Cases []struct{ ID, Title string }
				}
			}
		}
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	var nativeIDs = map[string]string{}
	for _, tool := range metadata.Tools {
		if tool.Identifier != "is.workflow.actions.makezip" {
			continue
		}
		for _, parameter := range tool.Parameters {
			if parameter.Key == "WFArchiveFormat" {
				for _, member := range parameter.Type.Cases {
					nativeIDs[member.Title] = member.ID
				}
			}
		}
	}
	// Public spellings come from the existing API; expected wire IDs come
	// independently from the retained Apple metadata, never from the encoder.
	var fixtures []archiveFormatFixture
	for _, value := range []string{".zip", ".tar.gz", ".tar.bz2", ".tar.xz", ".tar", ".gz", ".cpio", ".iso"} {
		var nativeID, found = nativeIDs[value]
		if !found || nativeID == "" || nativeID == value {
			t.Fatalf("independent native metadata missing for %q: %q", value, nativeID)
		}
		fixtures = append(fixtures, archiveFormatFixture{value, nativeID})
	}
	return fixtures
}

func archiveFormatSource(source string, legacy bool) string {
	if legacy {
		return "#include 'actions/documents'\n" + source
	}
	return source
}

func archiveFormatAction(t *testing.T, actions []ShortcutAction) map[string]any {
	t.Helper()
	var result map[string]any
	for _, action := range actions {
		if action.WFWorkflowActionIdentifier != "is.workflow.actions.makezip" {
			continue
		}
		if result != nil {
			t.Fatal("fixture produced more than one Make Archive action")
		}
		result = action.WFWorkflowActionParameters
	}
	if result == nil {
		t.Fatal("fixture produced no Make Archive action")
	}
	return result
}

func TestP10R15ArchiveFormatWireIDs(t *testing.T) {
	var fixtures = archiveFormatNativeFixtures(t)
	for _, legacy := range []bool{false, true} {
		for _, fixture := range fixtures {
			t.Run(fmt.Sprintf("legacy_%v/%s", legacy, fixture.source), func(t *testing.T) {
				var source = fmt.Sprintf("makeArchive(Ask, %q, \"kept-name\")\n", fixture.source)
				var actions = compileDebugFixture(t, archiveFormatSource(source, legacy), legacy)
				var params = archiveFormatAction(t, actions)
				if params["WFArchiveFormat"] != fixture.wire {
					t.Fatalf("WFArchiveFormat = %#v, want native ID %q for public spelling %q",
						params["WFArchiveFormat"], fixture.wire, fixture.source)
				}
				if params["WFZIPName"] != "kept-name" {
					t.Fatalf("archive name changed: %#v", params["WFZIPName"])
				}
				var wantInput = map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value":               map[string]any{"Type": "Ask"},
				}
				if !reflect.DeepEqual(params["WFInput"], wantInput) {
					t.Fatalf("archive input changed: %#v", params["WFInput"])
				}
			})
		}
	}
}

func TestP10R15ArchiveFormatOmissionAndDefault(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy_%v", legacy), func(t *testing.T) {
			var omitted = archiveFormatAction(t, compileDebugFixture(t,
				archiveFormatSource("makeArchive(Ask)\n", legacy), legacy))
			if _, present := omitted["WFArchiveFormat"]; present {
				t.Fatalf("omitted format gained a wire value: %#v", omitted)
			}
			if _, present := omitted["WFZIPName"]; present {
				t.Fatalf("omitted name gained a wire value: %#v", omitted)
			}
			var withName = "makeArchive(Ask, name: \"kept-name\")\n"
			if legacy {
				withName = "makeArchive(Ask, nil, \"kept-name\")\n"
			}
			var named = archiveFormatAction(t, compileDebugFixture(t,
				archiveFormatSource(withName, legacy), legacy))
			if _, present := named["WFArchiveFormat"]; present {
				t.Fatalf("format omission before name changed: %#v", named)
			}
			if named["WFZIPName"] != "kept-name" {
				t.Fatalf("name after omitted format changed: %#v", named)
			}
			var explicit = archiveFormatAction(t, compileDebugFixture(t,
				archiveFormatSource("makeArchive(Ask, \".zip\")\n", legacy), legacy))
			if explicit["WFArchiveFormat"] != "zip" {
				t.Fatalf("explicit public default did not encode native zip: %#v", explicit)
			}
		})
	}
}

func TestP10R15ArchiveFormatInvalidValues(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, value := range []string{"zip", ".cpgz", ".not-an-archive"} {
			t.Run(fmt.Sprintf("legacy_%v/%s", legacy, value), func(t *testing.T) {
				var source = archiveFormatSource(fmt.Sprintf("makeArchive(Ask, %q)\n", value), legacy)
				var output, err = compileHybridSource(source, legacy)
				if err == nil || len(output) != 0 {
					t.Fatalf("invalid public enum %q reached emission: error=%v output=%#v", value, err, output)
				}
				var message = strings.ToLower(err.Error())
				if !strings.Contains(message, "invalid") || !strings.Contains(message, strings.ToLower(value)) {
					t.Fatalf("failure was not the intended enum rejection: %v", err)
				}
			})
		}
	}
}

func TestP10R15ArchiveFormatReferencesAndRaw(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, mutable := range []bool{false, true} {
			t.Run(fmt.Sprintf("reference/legacy_%v/mutable_%v", legacy, mutable), func(t *testing.T) {
				var declaration, reference = "let format = text(\".zip\")\n", "format"
				if mutable {
					declaration = "var format = text(\".zip\")\n"
				}
				if legacy {
					declaration = "const format = text(\".zip\")\n"
					if mutable {
						declaration, reference = "@format = text(\".zip\")\n", "@format"
					}
				}
				var source = declaration + "makeArchive(Ask, " + reference + ", \"kept-name\")\n"
				var actions = compileDebugFixture(t, archiveFormatSource(source, legacy), legacy)
				var params = archiveFormatAction(t, actions)
				var wantRef map[string]any
				if mutable {
					wantRef = map[string]any{"Type": "Variable", "VariableName": "format"}
				} else {
					for _, action := range actions {
						if action.WFWorkflowActionIdentifier == "is.workflow.actions.gettext" {
							var producer = action.WFWorkflowActionParameters
							wantRef = map[string]any{
								"Type": "ActionOutput", "OutputUUID": producer["UUID"],
								"OutputName": producer["CustomOutputName"],
							}
						}
					}
				}
				if wantRef == nil {
					t.Fatal("reference producer missing")
				}
				var want = map[string]any{
					"WFSerializationType": "WFTextTokenString",
					"Value": map[string]any{
						"string":             "\uFFFC",
						"attachmentsByRange": map[string]any{"{0, 1}": wantRef},
					},
				}
				if !reflect.DeepEqual(params["WFArchiveFormat"], want) {
					t.Fatalf("runtime format reference was changed: got %#v, want %#v", params["WFArchiveFormat"], want)
				}
			})
		}
		for _, value := range []string{".zip", "zip", "future.archive"} {
			t.Run(fmt.Sprintf("raw/legacy_%v/%s", legacy, value), func(t *testing.T) {
				var source = fmt.Sprintf("native.action(\"is.workflow.actions.makezip\", parameters: {\"WFArchiveFormat\": %q, \"WFZIPName\": \"kept-name\"})\n", value)
				if legacy {
					source = fmt.Sprintf("rawAction(\"is.workflow.actions.makezip\", {\"WFArchiveFormat\": %q, \"WFZIPName\": \"kept-name\"})\n", value)
				}
				var params = archiveFormatAction(t, compileDebugFixture(t, source, legacy))
				if params["WFArchiveFormat"] != value || params["WFZIPName"] != "kept-name" {
					t.Fatalf("explicit raw parameters changed: %#v", params)
				}
			})
		}
	}
}

func TestP10R15ArchiveFormatNativeRoundTrip(t *testing.T) {
	var cases = archiveFormatNativeFixtures(t)
	for _, fixture := range append([]archiveFormatFixture(nil), cases...) {
		cases = append(cases, archiveFormatFixture{source: "", wire: fixture.source})
	}
	cases = append(cases, archiveFormatFixture{wire: "cpgz"}, archiveFormatFixture{wire: "future.archive"})
	for _, fixture := range cases {
		t.Run(fixture.wire, func(t *testing.T) {
			// Retain the existing input/header encoder, but independently set the
			// native field under test before any decompiler or provenance overlay.
			var workflow, err = compileHybridSource(archiveFormatSource("makeArchive(Ask)\n", true), true)
			if err != nil {
				t.Fatal(err)
			}
			var actions = workflow["WFWorkflowActions"].([]any)
			if len(actions) != 1 {
				t.Fatalf("native roundtrip fixture has %d actions", len(actions))
			}
			var params = actions[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
			params["WFArchiveFormat"] = fixture.wire
			params["WFZIPName"] = "kept-name"
			var raw, encodeErr = plist.Marshal(workflow, plist.XMLFormat)
			if encodeErr != nil {
				t.Fatal(encodeErr)
			}
			var decompiled = decompileForMobileWithOptions(raw, "ArchiveFormatRoundTrip", "legacy", "semantic")
			if !decompiled.OK {
				t.Fatalf("decompile failed: %s", decompiled.Error)
			}
			var rawFallback = strings.Contains(decompiled.Source, "rawAction(")
			if fixture.source == "" {
				if !rawFallback {
					t.Fatalf("noncanonical native value %q lost raw fallback:\n%s", fixture.wire, decompiled.Source)
				}
			} else if rawFallback || !strings.Contains(decompiled.Source, "makeArchive(") ||
				!strings.Contains(decompiled.Source, fmt.Sprintf("%q", fixture.source)) {
				t.Fatalf("native ID %q did not reconstruct its public spelling:\n%s", fixture.wire, decompiled.Source)
			}
			if strings.Contains(decompiled.Source, "__importProvenance") || strings.Contains(decompiled.Source, "native.workflow") {
				t.Fatal("roundtrip relied on a provenance overlay")
			}
			var rebuilt, compileErr = compileHybridSource(decompiled.Source, true)
			if compileErr != nil {
				t.Fatalf("recompile failed: %v\n%s", compileErr, decompiled.Source)
			}
			if !reflect.DeepEqual(rebuilt["WFWorkflowActions"], actions) {
				t.Fatalf("native action graph changed before any overlay:\ngot %#v\nwant %#v",
					rebuilt["WFWorkflowActions"], actions)
			}
		})
	}
}

func TestP10R15ArchiveFormatPreviewOrigin(t *testing.T) {
	const source = "makeArchive(Ask, \".zip\", \"editable-name\")\n"
	var preview, actions = rp41PreviewControlCompile(t, source, CompileOptions{})
	if archiveFormatAction(t, actions)["WFArchiveFormat"] != "zip" {
		t.Fatal("preview did not use the corrected native format")
	}
	var archiveIndex = -1
	for index, action := range actions {
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.makezip" {
			archiveIndex = index
		}
	}
	var nameEditable bool
	for _, entry := range preview.PreviewSourceMap.Entries {
		if entry.ActionIndex != archiveIndex {
			continue
		}
		for _, parameter := range entry.Parameters {
			if parameter.WireKey == "WFArchiveFormat" {
				t.Fatal("mapped format incorrectly acquired direct source-edit origin")
			}
			nameEditable = nameEditable || parameter.WireKey == "WFZIPName"
		}
	}
	if !nameEditable {
		t.Fatal("untransformed archive name lost its existing source-edit origin")
	}
	if _, err := rp41ApplyPreviewEdit(t, source, preview, archiveIndex, "WFArchiveFormat", "\".tar\""); err == nil {
		t.Fatal("mapped format unexpectedly allowed a direct preview source edit")
	}
	var updated, err = rp41ApplyPreviewEdit(t, source, preview, archiveIndex, "WFZIPName", "\"updated-name\"")
	if err != nil || updated != strings.Replace(source, "\"editable-name\"", "\"updated-name\"", 1) {
		t.Fatalf("ordinary name edit changed behavior: source=%q error=%v", updated, err)
	}
}
