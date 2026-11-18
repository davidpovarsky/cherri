package main

import (
	"encoding/json"
	"howett.net/plist"
	"os"
	"path/filepath"
	"testing"
)

func TestP15R06ProductionImportFormats(t *testing.T) {
	root := baseTestHeader()
	root["WFWorkflowActions"] = []any{
		map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.comment", "WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "P15 intake"}},
		map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.gettext", "WFWorkflowActionParameters": map[string]any{"WFTextActionText": "\u05e9\u05dc\u05d5\u05dd \U0001f30d", "UUID": "ABCD0000-1111-2222-3333-444444444444"}},
	}
	xml := buildNativePlistBytes(t, root, plist.XMLFormat)
	binary := buildNativePlistBytes(t, root, plist.BinaryFormat)
	js, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, format := range []struct {
		name string
		data []byte
	}{
		{"xml.plist", xml}, {"binary.plist", binary}, {"unsigned.shortcut", binary}, {"workflow.json", js},
	} {
		original, err := DecodeNativeShortcutDocument(format.data)
		if err != nil {
			t.Fatal(err)
		}
		input := filepath.Join(dir, format.name)
		if err = os.WriteFile(input, format.data, 0644); err != nil {
			t.Fatal(err)
		}
		for _, mode := range []string{"legacy", "v2"} {
			t.Run(format.name+"/"+mode, func(t *testing.T) {
				res, err := ExecuteImportPipeline(ImportOptions{InputPath: input, OutputPath: filepath.Join(dir, format.name+"."+mode+".cherri"), Mode: mode, Fidelity: "lossless"})
				if err != nil {
					t.Fatal(err)
				}
				actual, err := compileHybridSource(res.FinalSource, mode == "legacy")
				if err != nil {
					t.Fatal(err)
				}
				if !plistValuesStrictlyEqual(original.RawRoot, actual) {
					t.Fatalf("native tree mismatch: %s", firstPlistDiffPath(original.RawRoot, actual, ""))
				}
			})
		}
	}
}
