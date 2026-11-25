package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/electrikmilk/args-parser"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/shortcutcompare"
	"github.com/google/uuid"
	"howett.net/plist"
)

func derivedUUIDFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("tests", "derive-uuids-"+name+".cherri"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func compileDerivedUUIDLegacy(t *testing.T, source, name string, derive, reset bool) map[string]any {
	t.Helper()
	var previousArgs, previousEmbedded = args.Args, embeddedCompilerMode
	defer func() {
		args.Args, embeddedCompilerMode = previousArgs, previousEmbedded
		if recovered := recover(); recovered != nil {
			t.Fatalf("legacy UUID fixture: %v", recovered)
		}
	}()
	if reset {
		resetCompilerStateFully()
	}
	embeddedCompilerMode = true
	args.Args = map[string]string{"no-ansi": ""}
	if derive {
		args.Args["derive-uuids"] = ""
	}
	filePath, relativePath, inputPath, outputPath = "", "", "", ""
	filename, workflowName, contents = name+".cherri", name, source
	initParse()
	generateShortcut()
	var merged = mergeWorkflowHeaderMapWithDialect("legacy", importedWorkflowHeader, shortcut.WFWorkflowActions, exactActionParamsPresence, shortcut, false)
	data, err := plist.Marshal(merged, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	root, err := decodeShortcutRawMap(data)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// The file entry point is intentional: the public flag used to be ignored by V2.
func compileDerivedUUIDV2(t *testing.T, source, name string, derive bool) map[string]any {
	t.Helper()
	var previousArgs = args.Args
	defer func() { args.Args = previousArgs }()
	args.Args = map[string]string{"no-ansi": ""}
	if derive {
		args.Args["derive-uuids"] = ""
	}
	var directory = t.TempDir()
	var input = filepath.Join(directory, name+".cherri")
	var output = filepath.Join(directory, "result.shortcut")
	if err := os.WriteFile(input, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if err := CompileFileV2(input, output, true); err != nil {
		t.Fatal(err)
	}
	root, err := shortcutcompare.LoadShortcutDocument(output)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func derivedUUIDSource(t *testing.T, source string, legacy bool) string {
	t.Helper()
	if legacy {
		return source
	}
	migrated, diagnostics, err := migrate.MigrateSourceWithDiagnostics(source)
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("UUID fixture migration: %v %v", err, diagnostics)
	}
	return migrated
}

func compileDerivedUUID(t *testing.T, source, name string, derive, legacy bool) map[string]any {
	t.Helper()
	if legacy {
		return compileDerivedUUIDLegacy(t, source, name, derive, true)
	}
	return compileDerivedUUIDV2(t, source, name, derive)
}

func derivedUUIDGraph(t *testing.T, root map[string]any) (map[string]int, int) {
	t.Helper()
	var actions = root["WFWorkflowActions"].([]any)
	var producers = map[string]int{}
	var groups = map[string]string{}
	var stack []string
	var refs = 0
	var inspect func(any, int)
	inspect = func(value any, consumer int) {
		switch node := value.(type) {
		case map[string]any:
			if node["Type"] == "ActionOutput" {
				var id, _ = node["OutputUUID"].(string)
				producer, found := producers[id]
				if !found || producer >= consumer {
					t.Fatalf("missing/forward producer at %d: %#v", consumer, node)
				}
				refs++
			}
			for _, child := range node {
				inspect(child, consumer)
			}
		case []any:
			for _, child := range node {
				inspect(child, consumer)
			}
		}
	}
	for index, raw := range actions {
		var action = raw.(map[string]any)
		var parameters, _ = action["WFWorkflowActionParameters"].(map[string]any)
		inspect(parameters, index)
		if id, ok := parameters["UUID"].(string); ok && id != "" {
			if previous, found := producers[id]; found {
				t.Fatalf("duplicate producer UUID at actions %d and %d: %s", previous, index, id)
			}
			producers[id] = index
		}
		if group, ok := parameters["GroupingIdentifier"].(string); ok && group != "" {
			var identifier = action["WFWorkflowActionIdentifier"].(string)
			switch fmt.Sprint(parameters["WFControlFlowMode"]) {
			case "0":
				if _, duplicate := groups[group]; duplicate {
					t.Fatalf("duplicate control start at %d: %s", index, group)
				}
				groups[group] = identifier
				stack = append(stack, group)
			case "1", "2":
				if len(stack) == 0 || stack[len(stack)-1] != group || groups[group] != identifier {
					t.Fatalf("control marker %d has wrong group/identifier: %#v", index, action)
				}
				if fmt.Sprint(parameters["WFControlFlowMode"]) == "2" {
					stack = stack[:len(stack)-1]
				}
			default:
				t.Fatalf("unexpected control mode at %d: %#v", index, parameters)
			}
		}
	}
	if len(stack) != 0 {
		t.Fatalf("unclosed groups: %v", stack)
	}
	if refs == 0 {
		t.Fatal("fixture has no producer references")
	}
	return producers, len(groups)
}

func TestDerivedUUIDGraphAndReplay(t *testing.T) {
	t.Cleanup(resetCompilerStateFully)
	for _, fixture := range []string{"arrays", "controls", "macros"} {
		for _, legacy := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/legacy=%t", fixture, legacy), func(t *testing.T) {
				var source = derivedUUIDSource(t, derivedUUIDFixture(t, fixture), legacy)
				var first = compileDerivedUUID(t, source, "UUIDReplay", true, legacy)
				var ids, groups = derivedUUIDGraph(t, first)
				if fixture == "controls" && groups < 5 {
					t.Fatalf("control coverage shrank: %d", groups)
				}
				var second = compileDerivedUUID(t, source, "UUIDReplay", true, legacy)
				derivedUUIDGraph(t, second)
				if !reflect.DeepEqual(first, second) {
					t.Fatalf("derived compilation is not exactly repeatable: %s", firstPlistDiffPath(first, second, ""))
				}
				for id := range ids {
					parsed, err := uuid.Parse(id)
					if err != nil || parsed.Version() != 5 {
						t.Fatalf("derived producer is not a v5 UUID: %s (%v)", id, err)
					}
				}
				var random = compileDerivedUUID(t, source, "UUIDReplay", false, legacy)
				var randomIDs, _ = derivedUUIDGraph(t, random)
				// Macro display labels can contain their generated UUID spelling.
				// Keep those literal differences visible; do not weaken the comparator.
				if fixture != "macros" {
					if result := shortcutcompare.CompareDocumentsStrict(first, random); !result.Equal {
						t.Fatalf("derived mode changed native graph: %+v", result.Differences)
					}
				}
				if reflect.DeepEqual(ids, randomIDs) {
					t.Fatal("random mode reused deterministic identities")
				}
				for id := range randomIDs {
					parsed, err := uuid.Parse(id)
					if err != nil || parsed.Version() != 4 {
						t.Fatalf("default producer is not a random v4 UUID: %s (%v)", id, err)
					}
				}
			})
		}
	}
}

func TestDerivedUUIDLegacySequentialParseReset(t *testing.T) {
	resetCompilerStateFully()
	t.Cleanup(resetCompilerStateFully)
	var source = derivedUUIDFixture(t, "controls")
	var first = compileDerivedUUIDLegacy(t, source, "UUIDSequence", true, false)
	// No resetCompilerState call between complete workflow parses.
	compileDerivedUUIDLegacy(t, derivedUUIDFixture(t, "arrays"), "Unrelated", true, false)
	var second = compileDerivedUUIDLegacy(t, source, "UUIDSequence", true, false)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("sequential derived compile leaked state: %s", firstPlistDiffPath(first, second, ""))
	}
	var different = compileDerivedUUIDLegacy(t, source, "OtherWorkflow", true, false)
	var ids, _ = derivedUUIDGraph(t, second)
	var otherIDs, _ = derivedUUIDGraph(t, different)
	for id := range ids {
		if _, exists := otherIDs[id]; exists {
			t.Fatalf("different workflows share generated ID %s", id)
		}
	}
}

func TestDerivedUUIDImportedIdentity(t *testing.T) {
	t.Cleanup(resetCompilerStateFully)
	var original = compileDerivedUUIDLegacy(t, "const payload = text(\"explicit identity\")\nif payload == \"explicit identity\" {\ntext(payload)\n}\noutput(payload)\n", "ImportedIDs", false, true)
	data, err := plist.Marshal(original, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	document, err := DecodeNativeShortcutDocument(data)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PlanHybridImport(document)
	if err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []bool{true, false} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			var source string
			var err error
			if legacy {
				source, err = RenderHybridLegacySource(document, plan)
			} else {
				source, err = RenderHybridV2Source(document, plan)
			}
			if err != nil {
				t.Fatal(err)
			}
			var compiled = compileDerivedUUID(t, source, "ImportedIDs", true, legacy)
			if result := shortcutcompare.CompareDocumentsExact(original, compiled); !result.Equal {
				t.Fatalf("derive mode changed imported explicit identity/native data: %+v", result.Differences)
			}
		})
	}
}

func TestDerivedUUIDComparatorRejectsRetargetedEdge(t *testing.T) {
	t.Cleanup(resetCompilerStateFully)
	var original = compileDerivedUUIDLegacy(t, derivedUUIDFixture(t, "arrays"), "UUIDEdges", false, true)
	var changed = deepClonePlistMap(original)
	var actions = changed["WFWorkflowActions"].([]any)
	var first, second string
	var target map[string]any
	for _, raw := range actions {
		var parameters = raw.(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		if id, ok := parameters["UUID"].(string); ok {
			if first == "" {
				first = id
			} else if second == "" {
				second = id
			}
		}
		if raw.(map[string]any)["WFWorkflowActionIdentifier"] == "is.workflow.actions.appendvariable" && target == nil {
			target = debugParamMap(debugParamMap(parameters["WFInput"])["Value"])
		}
	}
	if first == "" || second == "" || target == nil {
		t.Fatal("edge mutation fixture incomplete")
	}
	target["OutputUUID"] = second
	if result := shortcutcompare.CompareDocumentsStrict(original, changed); result.Equal {
		t.Fatal("existing comparator accepted a retargeted array edge")
	}
}

func TestDerivedUUIDOptionsIsolation(t *testing.T) {
	var previousArgs = args.Args
	defer func() { args.Args = previousArgs }()
	// Library calls must choose their own policy, even in a CLI-configured process.
	args.Args = map[string]string{"derive-uuids": "", "no-ansi": ""}
	var source = "let payload = text(\"API identity\")\noutput(payload)"
	compile := func(name string, options CompileOptions) map[string]any {
		data, err := CompileSourceToPlistWithOptions(name+".cherri", source, options)
		if err != nil {
			t.Fatal(err)
		}
		root, err := decodeShortcutRawMap(data)
		if err != nil {
			t.Fatal(err)
		}
		return root
	}
	var first = compile("APIIdentity", CompileOptions{DeriveUUIDs: true})
	var unrelated = compile("OtherIdentity", CompileOptions{DeriveUUIDs: true})
	var random = compile("APIIdentity", CompileOptions{})
	var second = compile("APIIdentity", CompileOptions{DeriveUUIDs: true})
	if !reflect.DeepEqual(first, second) {
		t.Fatal("explicit API sequence leaked across compilations")
	}
	var firstIDs, _ = derivedUUIDGraph(t, first)
	var unrelatedIDs, _ = derivedUUIDGraph(t, unrelated)
	var randomIDs, _ = derivedUUIDGraph(t, random)
	for id := range firstIDs {
		if _, exists := unrelatedIDs[id]; exists {
			t.Fatalf("different workflow names share %s", id)
		}
		if _, exists := randomIDs[id]; exists {
			t.Fatalf("default API inherited deterministic identity %s", id)
		}
	}
	for id := range randomIDs {
		parsed, err := uuid.Parse(id)
		if err != nil || parsed.Version() != 4 {
			t.Fatalf("default API inherited derived mode: %s", id)
		}
	}
	if result := shortcutcompare.CompareDocumentsStrict(first, random); !result.Equal {
		t.Fatalf("API mode changed graph: %+v", result.Differences)
	}

	var explicit = "12345678-1234-4234-8234-123456789abc"
	for _, nativeSource := range []string{
		fmt.Sprintf(`let original = native.action("is.workflow.actions.number", {"WFNumberActionNumber":2}, uuid:%q)
output(original)`, explicit),
		fmt.Sprintf(`let original = native.action("is.workflow.actions.number", parameters:native.plist({"UUID":%q, "WFNumberActionNumber":2}))
output(original)`, explicit),
	} {
		data, err := CompileSourceToPlistWithOptions("NativeIdentity.cherri", nativeSource, CompileOptions{DeriveUUIDs: true})
		if err != nil {
			t.Fatal(err)
		}
		native, err := decodeShortcutRawMap(data)
		if err != nil {
			t.Fatal(err)
		}
		var ids, _ = derivedUUIDGraph(t, native)
		if _, present := ids[explicit]; !present {
			t.Fatal("derived mode rewrote explicit native.action UUID")
		}
	}
}

func TestDerivedUUIDCanonicalActionInstances(t *testing.T) {
	t.Cleanup(resetCompilerStateFully)
	const source = "const result = transcribeText(ShortcutInput)\nrun(\"UUID target\")\nrunSelf(ShortcutInput)\noutput(result)"
	for _, legacy := range []bool{true, false} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			var fixture = derivedUUIDSource(t, source, legacy)
			var first = compileDerivedUUID(t, fixture, "CanonicalInstances", true, legacy)
			var second = compileDerivedUUID(t, fixture, "CanonicalInstances", true, legacy)
			if !reflect.DeepEqual(first, second) {
				t.Fatalf("canonical generated placeholder is not repeatable: %s", firstPlistDiffPath(first, second, ""))
			}
			derivedUUIDGraph(t, first)
			var placeholders = 0
			for _, raw := range first["WFWorkflowActions"].([]any) {
				var parameters = raw.(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
				if workflow, ok := parameters["WFWorkflow"].(map[string]any); ok {
					if id, ok := workflow["workflowIdentifier"].(string); ok {
						parsed, err := uuid.Parse(id)
						if err != nil || parsed.Version() != 5 {
							t.Fatalf("workflow placeholder is not derived: %s", id)
						}
						placeholders++
					}
				}
			}
			if placeholders == 0 {
				t.Fatal("no generated workflow placeholder exercised")
			}
		})
	}
}
