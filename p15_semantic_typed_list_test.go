package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestP15SemanticUnconsumedListRetainsTypedGraph(t *testing.T) {
	original, err := CompileSourceToPlist("native-list.cherri", "let unused = [10,20]\n")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	input, output := filepath.Join(dir, "original.shortcut"), filepath.Join(dir, "imported.cherri")
	if err := os.WriteFile(input, original, 0600); err != nil {
		t.Fatal(err)
	}
	imported, err := ExecuteImportPipeline(ImportOptions{InputPath: input, OutputPath: output, Mode: "v2", Fidelity: "semantic"})
	if err != nil {
		t.Fatal(err)
	}
	recompiled, err := CompileSourceToPlist("native-list-reimport.cherri", imported.FinalSource)
	if err != nil {
		t.Fatalf("semantic imported native List failed: %v\n%s", err, imported.FinalSource)
	}
	for _, raw := range [][]byte{original, recompiled} {
		root, err := decodeShortcutRawMap(raw)
		if err != nil {
			t.Fatal(err)
		}
		actions, ok := root["WFWorkflowActions"].([]any)
		if !ok || len(actions) == 0 {
			t.Fatalf("unconsumed native List or typed producers were lost: %#v", root["WFWorkflowActions"])
		}
		numbers := map[string]string{}
		var rows []any
		listCount := 0
		for _, value := range actions {
			action := value.(map[string]any)
			parameters := action["WFWorkflowActionParameters"].(map[string]any)
			switch action["WFWorkflowActionIdentifier"] {
			case "is.workflow.actions.number":
				numbers[fmt.Sprint(parameters["UUID"])] = fmt.Sprint(parameters["WFNumberActionNumber"])
			case "is.workflow.actions.list":
				listCount++
				rows, _ = parameters["WFItems"].([]any)
			case "is.workflow.actions.setvariable", "is.workflow.actions.getvariable":
				// Semantic import may express native bindings as immutable
				// source names; these do not coerce List elements.
			default:
				t.Fatalf("typed List import inserted a coercion action: %v", action["WFWorkflowActionIdentifier"])
			}
		}
		if len(numbers) != 2 || listCount != 1 || len(rows) != 2 {
			t.Fatalf("typed List cardinality changed: numbers=%v lists=%d rows=%v", numbers, listCount, rows)
		}
		for index, value := range rows {
			row := value.(map[string]any)
			var refs []string
			var collect func(any)
			collect = func(value any) {
				switch item := value.(type) {
				case map[string]any:
					if item["Type"] == "ActionOutput" {
						refs = append(refs, fmt.Sprint(item["OutputUUID"]))
					}
					for _, child := range item {
						collect(child)
					}
				case []any:
					for _, child := range item {
						collect(child)
					}
				}
			}
			collect(row["WFValue"])
			if len(refs) != 1 || numbers[refs[0]] != []string{"10", "20"}[index] {
				t.Fatalf("native List row lost its typed Number producer: row=%v numbers=%v", row, numbers)
			}
		}
	}
}
