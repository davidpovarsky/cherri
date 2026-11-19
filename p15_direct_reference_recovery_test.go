package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/shortcutcompare"
	"howett.net/plist"
)

const p15BRG07Producer = "12345678-1234-4234-8234-123456789abc"

func p15BRG07ExpectedReference(kind string, nativeExtension bool) map[string]any {
	var value = map[string]any{"Type": kind, "Aggrandizements": []any{
		map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFDictionaryContentItem", "P15Extra": false},
		map[string]any{"Type": "WFDictionaryValueVariableAggrandizement", "DictionaryKey": "payload"},
		map[string]any{"Type": "WFPropertyVariableAggrandizement", "PropertyName": "Name", "PropertyUserInfo": []any{map[string]any{"Value": false, "Zero": int64(0), "Empty": ""}}},
	}}
	switch kind {
	case "ActionOutput":
		value["OutputUUID"] = p15BRG07Producer
	case "Variable":
		value["VariableName"] = "BRG07Saved"
	}
	if nativeExtension {
		value["P15FutureReference"] = map[string]any{"flag": false, "ordered": []any{"first", int64(0), "last"}}
	}
	return map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": value}
}

func p15BRG07NativeRoot(nativeExtension bool) map[string]any {
	var root = baseTestHeader()
	var plainProducer = map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "ActionOutput", "OutputUUID": p15BRG07Producer}}
	root["WFWorkflowActions"] = []any{
		p15AcceptanceAction("is.workflow.actions.gettext", map[string]any{"WFTextActionText": "BRG07 source", "UUID": p15BRG07Producer}),
		p15AcceptanceAction("is.workflow.actions.setvariable", map[string]any{"WFVariableName": "BRG07Saved", "WFInput": plainProducer}),
		p15AcceptanceAction("com.example.cherri.brg07.references", map[string]any{
			"Action":   p15BRG07ExpectedReference("ActionOutput", nativeExtension),
			"Variable": p15BRG07ExpectedReference("Variable", nativeExtension),
			"System":   p15BRG07ExpectedReference("ExtensionInput", nativeExtension),
		}),
	}
	return root
}

func p15BRG07Consumer(t *testing.T, root map[string]any) map[string]any {
	t.Helper()
	var actions, ok = root["WFWorkflowActions"].([]any)
	if !ok || len(actions) != 3 {
		t.Fatalf("reference fixture emitted %d actions; want its exact three-action graph", len(actions))
	}
	var consumer = actions[2].(map[string]any)
	if consumer["WFWorkflowActionIdentifier"] != "com.example.cherri.brg07.references" {
		t.Fatal("reference consumer changed")
	}
	return consumer["WFWorkflowActionParameters"].(map[string]any)
}

func p15BRG07Decoded(t *testing.T, root map[string]any) map[string]any {
	t.Helper()
	var decoded, err = DecodeNativeShortcutDocument(buildNativePlistBytes(t, root, plist.XMLFormat))
	if err != nil {
		t.Fatal(err)
	}
	return decoded.RawRoot
}

func TestP15BRG07DirectReferenceKindsRoundTrip(t *testing.T) {
	var source, err = os.ReadFile(filepath.Join("tests", "language-v2", "v2-direct-reference-chains.cherri"))
	if err != nil {
		t.Fatal(err)
	}
	var emitted, compileErr = CompileSourceToPlist("brg07-direct.cherri", string(source))
	if compileErr != nil {
		t.Fatal(compileErr)
	}
	var actual, decodeErr = DecodeNativeShortcutDocument(emitted)
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}
	var expected = p15BRG07Decoded(t, p15BRG07NativeRoot(false))
	var want, got = p15BRG07Consumer(t, expected), p15BRG07Consumer(t, actual.RawRoot)
	for _, name := range []string{"Action", "Variable", "System"} {
		t.Run("direct_"+name, func(t *testing.T) {
			if !plistValuesStrictlyEqual(want[name], got[name]) {
				t.Fatalf("direct kind, producer, ordered chain or extension changed at %s", firstPlistDiffPath(want[name], got[name], name))
			}
		})
	}
	if !plistValuesStrictlyEqual(expected["WFWorkflowActions"], actual.RawRoot["WFWorkflowActions"]) {
		t.Errorf("direct references changed their explicit producer/variable graph at %s", firstPlistDiffPath(expected["WFWorkflowActions"], actual.RawRoot["WFWorkflowActions"], "actions"))
	}
	// The imported specimen adds unknown reference-level fields that have no
	// authored native.ref argument. Both real import routes must retain them.
	for _, extended := range []bool{false, true} {
		var original = p15BRG07NativeRoot(extended)
		var decoded = p15BRG07Decoded(t, original)
		var results = p15NativeImportRoundTrip(t, fmt.Sprintf("brg07-extended-%t", extended), original, plist.XMLFormat, plist.BinaryFormat)
		if len(results) != 8 {
			t.Fatalf("incomplete reference-kind import matrix: got %d, want XML/binary x hybrid/lossless x Legacy/v2", len(results))
		}
		for _, result := range results {
			if comparison := shortcutcompare.CompareDocumentsStrict(decoded, result.compiled); !comparison.Equal {
				t.Fatalf("strict production comparator rejected reference roundtrip: %+v", comparison.Differences)
			}
		}
	}
}

func TestP15BRG07ReferenceMutationControls(t *testing.T) {
	var original = p15BRG07Decoded(t, p15BRG07NativeRoot(true))
	if comparison := shortcutcompare.CompareDocumentsStrict(original, p15BRG07Decoded(t, p15BRG07NativeRoot(true))); !comparison.Equal {
		t.Fatal("strict comparison rejected the unchanged native control")
	}
	for _, name := range []string{"Action", "Variable", "System"} {
		for _, fault := range []string{"remove_coercion", "remove_key", "remove_property", "reorder_chain", "change_kind", "change_producer", "drop_transform_extension", "drop_reference_extension"} {
			t.Run(name+"/"+fault, func(t *testing.T) {
				var changed = p15BRG07Decoded(t, p15BRG07NativeRoot(true))
				var reference = p15BRG07Consumer(t, changed)[name].(map[string]any)["Value"].(map[string]any)
				var chain = reference["Aggrandizements"].([]any)
				switch fault {
				case "remove_coercion", "remove_key", "remove_property":
					var index = map[string]int{"remove_coercion": 0, "remove_key": 1, "remove_property": 2}[fault]
					reference["Aggrandizements"] = append(chain[:index:index], chain[index+1:]...)
				case "reorder_chain":
					chain[0], chain[2] = chain[2], chain[0]
				case "change_kind":
					reference["Type"] = "Clipboard"
				case "change_producer":
					switch name {
					case "Action":
						reference["OutputUUID"] = "98765432-1234-4234-8234-123456789abc"
					case "Variable":
						reference["VariableName"] = "DifferentSavedValue"
					case "System":
						reference["Type"] = "CurrentDate"
					}
				case "drop_transform_extension":
					delete(chain[0].(map[string]any), "P15Extra")
				case "drop_reference_extension":
					delete(reference, "P15FutureReference")
				}
				for _, comparison := range []*shortcutcompare.ComparisonResult{
					shortcutcompare.CompareDocumentsStrict(original, changed),
					shortcutcompare.CompareDocumentsStrict(changed, original),
				} {
					if comparison.Equal || len(comparison.Differences) == 0 {
						t.Fatal("strict production comparison accepted the reference mutation")
					}
				}
			})
		}
	}
}

func TestP15BRG07SharedReferencePaths(t *testing.T) {
	var fixture, err = os.ReadFile(filepath.Join("tests", "language-v2", "v2-direct-reference-chains.cherri"))
	if err != nil {
		t.Fatal(err)
	}
	var lines = strings.Split(string(fixture), "\n")
	var prefix = strings.Join(lines[2:4], "\n") + "\n"
	for _, kind := range []string{"ActionOutput", "Variable", "ExtensionInput"} {
		var reference = p15BRG07ExpectedReference(kind, false)
		var body = reference["Value"].(map[string]any)
		var rawChain, err = json.Marshal(body["Aggrandizements"])
		if err != nil {
			t.Fatal(err)
		}
		var expression = fmt.Sprintf("native.ref(%q", kind)
		switch kind {
		case "ActionOutput":
			expression += fmt.Sprintf(", uuid:%q, outputName:%q", p15BRG07Producer, "BRG07 source")
			body["OutputName"] = "BRG07 source"
		case "Variable":
			expression += ", name:\"BRG07Saved\""
		}
		expression += ", aggrandizements:" + string(rawChain) + ")"
		for _, path := range []string{"typed_call", "inline_text"} {
			t.Run(kind+"/"+path, func(t *testing.T) {
				var source = prefix + "getVariable(" + expression + ")"
				if path == "inline_text" {
					source = prefix + "text(f\"{" + expression + "}\")"
				}
				var raw, err = CompileSourceToPlist("brg07-shared.cherri", source)
				if err != nil {
					t.Fatal(err)
				}
				var decoded, decodeErr = DecodeNativeShortcutDocument(raw)
				if decodeErr != nil {
					t.Fatal(decodeErr)
				}
				var actions = decoded.RawRoot["WFWorkflowActions"].([]any)
				if len(actions) != 3 {
					t.Fatalf("shared reference path changed the three-action graph: %d", len(actions))
				}
				var params = actions[2].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
				var got = params["WFVariable"]
				var want any = reference
				if path == "inline_text" {
					var token = params["WFTextActionText"].(map[string]any)
					var value = token["Value"].(map[string]any)
					var attachments = value["attachmentsByRange"].(map[string]any)
					if token["WFSerializationType"] != "WFTextTokenString" || value["string"] != "\uFFFC" || len(attachments) != 1 {
						t.Fatal("shared inline reference wrapper/range changed")
					}
					got, want = attachments["{0, 1}"], body
				}
				var wantNative = p15BRG07Decoded(t, map[string]any{"WFWorkflowActions": []any{}, "Reference": want})["Reference"]
				if !plistValuesStrictlyEqual(wantNative, got) {
					t.Fatalf("shared %s native reference lost typed/opaque fields at %s", path, firstPlistDiffPath(wantNative, got, kind))
				}
			})
		}
	}
}
