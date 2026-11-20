package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/lower"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/modules"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func labelTestTokenUUID(raw any) string {
	m := debugParamMap(raw)
	if m["WFSerializationType"] == "WFTextTokenAttachment" {
		v := debugParamMap(m["Value"])
		id, _ := v["OutputUUID"].(string)
		return id
	}
	if m["WFSerializationType"] == "WFTextTokenString" {
		v := debugParamMap(m["Value"])
		a := debugParamMap(v["attachmentsByRange"])
		token := debugParamMap(a["{0, 1}"])
		id, _ := token["OutputUUID"].(string)
		return id
	}
	return ""
}
func labelTestHasRef(raw any, id string) bool {
	data, _ := json.Marshal(raw)
	var value any
	_ = json.Unmarshal(data, &value)
	var walk func(any) bool
	walk = func(v any) bool {
		switch x := v.(type) {
		case map[string]any:
			if x["Type"] == "ActionOutput" && x["OutputUUID"] == id {
				return true
			}
			for _, child := range x {
				if walk(child) {
					return true
				}
			}
		case []any:
			for _, child := range x {
				if walk(child) {
					return true
				}
			}
		}
		return false
	}
	return walk(value)
}
func labelTestFields(raw any) map[string]map[string]any {
	m := debugParamMap(raw)
	v := debugParamMap(m["Value"])
	data, _ := json.Marshal(v["WFDictionaryFieldValueItems"])
	var items []map[string]any
	_ = json.Unmarshal(data, &items)
	result := map[string]map[string]any{}
	for _, item := range items {
		key := debugParamMap(debugParamMap(item["WFKey"])["Value"])
		name, _ := key["string"].(string)
		if name == "" {
			name, _ = key["String"].(string)
		}
		result[name] = item
	}
	return result
}

// Independent causal graph contract: label range comes from the raw append's
// cardinality, and scalar metadata cannot contain or replace the original value.
// This proves the emitted graph only, not native content-class cardinality.
func validateLabelTestGraph(acts []ShortcutAction, producerID, label string) error {
	byID := map[string]int{}
	rawIndex := -1
	metadataIndex := -1
	for i, a := range acts {
		p := a.WFWorkflowActionParameters
		if id, ok := p["UUID"].(string); ok && id != "" {
			if _, dup := byID[id]; dup {
				return fmt.Errorf("duplicate UUID")
			}
			byID[id] = i
		}
		if a.WFWorkflowActionIdentifier == "is.workflow.actions.appendvariable" {
			if p["WFVariableName"] == debugLogVariableName {
				if rawIndex >= 0 {
					return fmt.Errorf("more than one raw append")
				}
				rawIndex = i
			}
			if p["WFVariableName"] == debugLabelVariableName {
				if metadataIndex >= 0 {
					return fmt.Errorf("more than one label append")
				}
				metadataIndex = i
			}
		}
	}
	if rawIndex < 0 || metadataIndex <= rawIndex || labelTestTokenUUID(acts[rawIndex].WFWorkflowActionParameters["WFInput"]) != producerID {
		return fmt.Errorf("raw identity/order")
	}
	raw := acts[rawIndex].WFWorkflowActionParameters["WFInput"]
	dictIndex, ok := byID[labelTestTokenUUID(acts[metadataIndex].WFWorkflowActionParameters["WFInput"])]
	if !ok || dictIndex <= rawIndex || dictIndex >= metadataIndex || acts[dictIndex].WFWorkflowActionIdentifier != "is.workflow.actions.dictionary" {
		return fmt.Errorf("metadata dictionary causality")
	}
	fields := labelTestFields(acts[dictIndex].WFWorkflowActionParameters["WFItems"])
	if len(fields) != 3 || fields["first"] == nil || fields["last"] == nil || fields["label"] == nil {
		return fmt.Errorf("metadata keys")
	}
	if labelTestHasRef(acts[dictIndex].WFWorkflowActionParameters, producerID) {
		return fmt.Errorf("raw value boxed in metadata")
	}
	if fields["first"]["WFItemType"] != float64(itemTypeNumber) || fields["last"]["WFItemType"] != float64(itemTypeNumber) || fields["label"]["WFItemType"] != float64(itemTypeText) {
		return fmt.Errorf("metadata scalar types")
	}
	literal := debugParamMap(debugParamMap(fields["label"]["WFValue"])["Value"])
	if literal["string"] != label || literal["attachmentsByRange"] != nil {
		return fmt.Errorf("literal label changed")
	}
	lastIndex, ok := byID[labelTestTokenUUID(fields["last"]["WFValue"])]
	if !ok || lastIndex <= rawIndex || lastIndex >= dictIndex {
		return fmt.Errorf("range end causality")
	}
	last := acts[lastIndex]
	if last.WFWorkflowActionIdentifier != "is.workflow.actions.count" || last.WFWorkflowActionParameters["WFCountType"] != "Items" || !debugLabelIsVariable(last.WFWorkflowActionParameters["Input"], debugLogVariableName) {
		return fmt.Errorf("range end must count raw buffer")
	}
	firstIndex, ok := byID[labelTestTokenUUID(fields["first"]["WFValue"])]
	if !ok || firstIndex <= lastIndex || firstIndex >= dictIndex {
		return fmt.Errorf("range start causality")
	}
	first := acts[firstIndex]
	if first.WFWorkflowActionIdentifier != "is.workflow.actions.math" || first.WFWorkflowActionParameters["WFMathOperation"] != "+" || fmt.Sprint(first.WFWorkflowActionParameters["WFMathOperand"]) != "1" {
		return fmt.Errorf("range start one-based conversion")
	}
	beforeIndex, ok := byID[labelTestTokenUUID(first.WFWorkflowActionParameters["WFInput"])]
	if !ok || beforeIndex <= lastIndex || beforeIndex >= firstIndex {
		return fmt.Errorf("range subtraction causality")
	}
	before := acts[beforeIndex]
	if before.WFWorkflowActionIdentifier != "is.workflow.actions.math" || before.WFWorkflowActionParameters["WFMathOperation"] != "-" || labelTestTokenUUID(before.WFWorkflowActionParameters["WFInput"]) != last.WFWorkflowActionParameters["UUID"] {
		return fmt.Errorf("range subtraction")
	}
	countIndex, ok := byID[labelTestTokenUUID(before.WFWorkflowActionParameters["WFMathOperand"])]
	if !ok || countIndex <= rawIndex || countIndex >= beforeIndex {
		return fmt.Errorf("appended size causality")
	}
	count := acts[countIndex]
	if count.WFWorkflowActionIdentifier != "is.workflow.actions.count" || count.WFWorkflowActionParameters["WFCountType"] != "Items" || !reflect.DeepEqual(count.WFWorkflowActionParameters["Input"], raw) {
		return fmt.Errorf("size must count original raw output")
	}
	restored := false
	for i := metadataIndex + 1; i < len(acts); i++ {
		a := acts[i]
		if a.WFWorkflowActionIdentifier == "is.workflow.actions.getvariable" && reflect.DeepEqual(a.WFWorkflowActionParameters["WFVariable"], raw) {
			restored = true
		}
	}
	if !restored {
		return fmt.Errorf("original native output not restored")
	}
	return nil
}

func TestDebugLabelsPublicLiteralOmissionContract(t *testing.T) {
	registry := schema.NewRegistry(nil, nil)
	for _, name := range []string{"debugLog", "debugLogPrevious"} {
		def := actions[name]
		if len(def.parameters) != 4 || def.parameters[3].name != "label" || def.parameters[3].defaultValue != nil || !def.parameters[3].optional {
			t.Fatalf("legacy label contract: %#v", def.parameters)
		}
		sc, ok := registry.LookupAction(name)
		if !ok || len(sc.Parameters) != 4 {
			t.Fatal("missing shared schema")
		}
		p := sc.Parameters[3]
		if p.ID != "label" || p.WireKey != "WFDebugLogLabel" || p.DefaultValue != nil || !p.Optional || !reflect.DeepEqual(p.AcceptedForms, []string{"string"}) {
			t.Fatalf("schema literal/omission contract: %+v", p)
		}
	}
	for _, legacy := range []bool{true, false} {
		for _, name := range []string{"debugLog", "debugLogPrevious"} {
			t.Run(fmt.Sprintf("%s/legacy=%t", name, legacy), func(t *testing.T) {
				plain := compileDebugFixture(t, "text(\"value\")\n"+name+"()\n", legacy)
				checkDebugFallthroughSequence(t, plain, legacy, "gettext", "appendvariable")
				for _, label := range []string{"", "API Response", "שָׁלוֹם 🍒 {not_a_variable} \uFFFC"} {
					src := fmt.Sprintf("text(\"value\")\n%s(false,false,false,%q)\n", name, label)
					acts := compileDebugFixture(t, src, legacy)
					id, _ := acts[0].WFWorkflowActionParameters["UUID"].(string)
					if err := validateLabelTestGraph(acts, id, label); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

func TestDebugLabelsTypedProducersAndLaterConsumers(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		for _, producer := range []string{"text(\"raw\")", "number(42)", "{\"key\": \"value\", \"n\": 2}", "list(\"a\",\"b\")", "currentDate()"} {
			t.Run(fmt.Sprintf("%t/%s", legacy, producer), func(t *testing.T) {
				declaration := "let"
				prefix := ""
				if legacy {
					declaration = "const"
					if producer == "currentDate()" {
						prefix = "#include 'actions/calendar'\n"
					}
				}
				src := prefix + declaration + " retained = " + producer + "\ndebugLog(true,false,false,\"API Response\")\ngetVariable(retained)\n"
				acts := compileDebugFixture(t, src, legacy)
				var producerID string
				for _, a := range acts {
					if a.WFWorkflowActionIdentifier == "is.workflow.actions.appendvariable" && a.WFWorkflowActionParameters["WFVariableName"] == debugLogVariableName {
						producerID = labelTestTokenUUID(a.WFWorkflowActionParameters["WFInput"])
					}
				}
				if err := validateLabelTestGraph(acts, producerID, "API Response"); err != nil {
					t.Fatal(err)
				}
				consumerIndex := len(acts) - 1
				if !legacy {
					checkDebugMainVoidBoundary(t, acts)
					consumerIndex--
				}
				if acts[consumerIndex].WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" || !labelTestHasRef(acts[consumerIndex].WFWorkflowActionParameters, producerID) {
					t.Fatal("later source consumer lost original producer")
				}
				for i, a := range acts {
					if a.WFWorkflowActionIdentifier == "is.workflow.actions.detect.date" {
						if i == 0 || acts[i-1].WFWorkflowActionParameters["UUID"] != producerID || !labelTestHasRef(a.WFWorkflowActionParameters, producerID) {
							t.Fatal("Creation Date not directly after actual source")
						}
					}
				}
			})
		}
	}
}

func TestDebugLabelsCausalGraphRejectsCorruption(t *testing.T) {
	acts := compileDebugFixture(t, "text(\"raw\")\ndebugLog(false,false,false,\"label\")\n", false)
	producer, _ := acts[0].WFWorkflowActionParameters["UUID"].(string)
	checkDebugMainVoidBoundary(t, acts)
	if err := validateLabelTestGraph(acts, producer, "label"); err != nil {
		t.Fatal(err)
	}
	// Bind the mutation to the actual metadata restore, independently of the
	// main's terminal Nothing. This fixture has no authored later consumer.
	metadataIndex, restoreIndex := -1, -1
	for i, action := range acts {
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.appendvariable" && action.WFWorkflowActionParameters["WFVariableName"] == debugLabelVariableName {
			metadataIndex = i
		}
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.getvariable" && reflect.DeepEqual(action.WFWorkflowActionParameters["WFVariable"], acts[1].WFWorkflowActionParameters["WFInput"]) {
			if restoreIndex >= 0 {
				t.Fatal("restore mutation target is ambiguous")
			}
			restoreIndex = i
		}
	}
	if metadataIndex < 0 || restoreIndex <= metadataIndex {
		t.Fatal("restore mutation target is missing or precedes metadata")
	}
	mutations := map[string]func([]ShortcutAction){
		"raw-input-boxed": func(a []ShortcutAction) { a[1].WFWorkflowActionParameters["WFInput"] = debugOutputRef("OTHER") },
		"count-text-not-items": func(a []ShortcutAction) {
			for i := range a {
				if a[i].WFWorkflowActionIdentifier == "is.workflow.actions.count" {
					a[i].WFWorkflowActionParameters["WFCountType"] = "Characters"
					return
				}
			}
		},
		"count-wrong-source": func(a []ShortcutAction) {
			for i := range a {
				if a[i].WFWorkflowActionIdentifier == "is.workflow.actions.count" && !debugLabelIsVariable(a[i].WFWorkflowActionParameters["Input"], debugLogVariableName) {
					a[i].WFWorkflowActionParameters["Input"] = debugOutputRef("OTHER")
					return
				}
			}
		},
		"reversed-range": func(a []ShortcutAction) {
			for i := range a {
				if a[i].WFWorkflowActionParameters["WFMathOperation"] == "-" {
					a[i].WFWorkflowActionParameters["WFMathOperation"] = "+"
					return
				}
			}
		},
		"zero-based-native-range": func(a []ShortcutAction) {
			for i := range a {
				if a[i].WFWorkflowActionParameters["WFMathOperation"] == "+" {
					a[i].WFWorkflowActionParameters["WFMathOperand"] = 0
					return
				}
			}
		},
		"metadata-append-detached": func(a []ShortcutAction) {
			for i := range a {
				if a[i].WFWorkflowActionParameters["WFVariableName"] == debugLabelVariableName {
					a[i].WFWorkflowActionParameters["WFInput"] = debugOutputRef("OTHER")
					return
				}
			}
		},
		"restore-coerces-text": func(a []ShortcutAction) { a[restoreIndex].WFWorkflowActionIdentifier = "is.workflow.actions.gettext" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			data, _ := json.Marshal(acts)
			var a []ShortcutAction
			_ = json.Unmarshal(data, &a)
			mutate(a)
			if err := validateLabelTestGraph(a, producer, "label"); err == nil {
				t.Fatal("corrupted label causality accepted")
			}
		})
	}
}

func TestDebugLabelsClearPreviewOwnershipAndLoopBackfill(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		for _, display := range []string{"debugShowLogs()", "debugShowLogs(false,true)", "debugShowLogs(true)", "debugShowLogs(true,true)", "debugFinish()"} {
			t.Run(fmt.Sprintf("%t/%s", legacy, display), func(t *testing.T) {
				// Preview precedes label in source; subsequent runtime iterations must still
				// use metadata. The clear precedes the first label as a separate owned site.
				src := "debugClearLogs()\ntext(\"seed\")\ndebugLog()\n" + display + "\ntext(\"raw\")\ndebugLog(false,false,false,\"label\")\ndebugClearLogs()\n"
				acts := compileDebugFixture(t, src, legacy)
				rawClears, metadataClears, metadataGuards := 0, 0, 0
				for i, a := range acts {
					p := a.WFWorkflowActionParameters
					if a.WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" && (p["WFVariableName"] == debugLogVariableName || p["WFVariableName"] == debugLabelVariableName) {
						if i == 0 || acts[i-1].WFWorkflowActionIdentifier != "is.workflow.actions.nothing" {
							t.Fatal("clear is not zero-output")
						}
						if _, bad := p["WFInput"]; bad {
							t.Fatal("clear acquired seed")
						}
						if p["WFVariableName"] == debugLogVariableName {
							rawClears++
						} else {
							metadataClears++
						}
					}
					if a.WFWorkflowActionIdentifier == "is.workflow.actions.count" && debugLabelIsVariable(p["Input"], debugLabelVariableName) {
						metadataGuards++
					}
				}
				if rawClears != 2 || metadataClears != 2 || metadataGuards != 1 {
					t.Fatalf("clear/earlier-preview not synchronized: %d/%d/%d", rawClears, metadataClears, metadataGuards)
				}
			})
		}
	}
	resetDebugInstrumentationManifest()
	raw := []ShortcutAction{{WFWorkflowActionIdentifier: "is.workflow.actions.nothing", WFWorkflowActionParameters: map[string]any{}}, {WFWorkflowActionIdentifier: "is.workflow.actions.setvariable", WFWorkflowActionParameters: map[string]any{"WFVariableName": debugLogVariableName}}, {WFWorkflowActionIdentifier: "is.workflow.actions.previewdocument", WFWorkflowActionParameters: map[string]any{"WFInput": debugLogValueRef(debugLogVariableName)}}}
	got, inserted, err := backfillDebugLabelContext(raw)
	if err != nil || inserted != 0 || !reflect.DeepEqual(got, raw) {
		t.Fatal("raw lookalike changed")
	}
	recordDebugExpandedActions("debugClearLogs", 0, raw[:2])
	owned, n, err := backfillDebugLabelContext(raw)
	if err != nil || n != 2 {
		t.Fatalf("owned clear backfill: %v %d", err, n)
	}
	again, n, err := backfillDebugLabelContext(owned)
	if err != nil || n != 0 || !reflect.DeepEqual(again, owned) {
		t.Fatal("clear backfill not idempotent")
	}
	for _, seed := range []any{"", debugOutputRef("SEED")} {
		mutated := append([]ShortcutAction(nil), owned...)
		p := map[string]any{"WFVariableName": debugLabelVariableName, "WFInput": seed, "UUID": owned[3].WFWorkflowActionParameters["UUID"]}
		mutated[3].WFWorkflowActionParameters = p
		if _, _, err := backfillDebugLabelContext(mutated); err == nil {
			t.Fatal("nonempty metadata seed accepted")
		}
	}
}

func TestDebugLabelsNestedPreviewUsesSnapshotAndStrictBranchFallback(t *testing.T) {
	for _, dates := range []bool{false, true} {
		for _, text := range []bool{false, true} {
			t.Run(fmt.Sprintf("dates=%t/text=%t", dates, text), func(t *testing.T) {
				acts, err := buildDebugLabeledPreview(dates, text, "RESULT", "Shown")
				if err != nil {
					t.Fatal(err)
				}
				// Every outer value is captured before the inner metadata loop; Set Name
				// must use the snapshot UUID, never the shadowed native Repeat Item.
				var snapshots []string
				innerStarted := false
				loops := 0
				typed := 0
				fallback := 0
				for _, a := range acts {
					p := a.WFWorkflowActionParameters
					if a.WFWorkflowActionIdentifier == "is.workflow.actions.repeat.each" && fmt.Sprint(p["WFControlFlowMode"]) == "0" {
						loops++
						if loops == 2 {
							innerStarted = true
						}
					}
					if a.WFWorkflowActionIdentifier == "is.workflow.actions.getvariable" && debugLabelIsVariable(p["WFVariable"], "Repeat Item") && !innerStarted {
						snapshots = append(snapshots, p["UUID"].(string))
					}
					if a.WFWorkflowActionIdentifier == "is.workflow.actions.setitemname" && labelTestTokenUUID(p["WFInput"]) != "" {
						typed++
						if len(snapshots) != 1 || labelTestTokenUUID(p["WFInput"]) != snapshots[0] {
							t.Fatal("preview renamed metadata/inner Repeat Item")
						}
					}
					if a.WFWorkflowActionIdentifier == "is.workflow.actions.previewdocument" {
						fallback++
					}
				}
				if len(snapshots) != 1 || fallback != 2 || (!text && typed == 0) || text && typed != 0 {
					t.Fatalf("typed/explicit-text/fallback contract: %v %d %d", snapshots, typed, fallback)
				}
				last := acts[len(acts)-1].WFWorkflowActionParameters
				if last["UUID"] != "RESULT" || last["CustomOutputName"] != "Shown" {
					t.Fatal("public preview result metadata changed")
				}
			})
		}
	}
	literal := debugLabelText("שָׁלוֹם 👩‍💻 é ", map[string]any{"Type": "ActionOutput", "OutputUUID": "RAW"})
	v := literal["Value"].(map[string]any)
	a := v["attachmentsByRange"].(map[string]any)
	if len(a) != 1 || a[fmt.Sprintf("{%d, 1}", utf16CodeUnitCount("שָׁלוֹם 👩‍💻 é "))] == nil {
		t.Fatal("UTF16 label offset corrupted")
	}
}

func TestDebugLabelsProfilesAndInvalidValues(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		src := "debugClearLogs()\ntext(\"retained\")\ndebugLog(false,false,false,\"LABEL_ONLY_MARKER\")\ndebugShowLogs()\ndebugAssert(false,\"required\")\n"
		release, err := compileBuildProfileFixture(t, src, legacy, BuildProfileRelease)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(release)
		if strings.Contains(string(data), "LABEL_ONLY_MARKER") || strings.Contains(string(data), debugLabelVariableName) {
			t.Fatal("label metadata leaked to release")
		}
		suffix := strings.Join(actionSuffixes(release), ",")
		if !strings.Contains(suffix, "alert,exit") || !strings.HasPrefix(suffix, "gettext") {
			t.Fatal("source/assertion removed")
		}
		declaration := "let"
		if legacy {
			declaration = "const"
		}
		dynamic := declaration + " label = text(\"dynamic\")\ntext(\"raw\")\ndebugLog(false,false,false,label)\n"
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			if _, err := compileBuildProfileFixture(t, dynamic, legacy, profile); err == nil {
				t.Fatal("dynamic label accepted")
			}
		}
	}
	for _, invalid := range []any{nil, true, 7, debugOutputRef("OTHER"), map[string]any{"WFSerializationType": "WFTextTokenString"}} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			resetDebugInstrumentationManifest()
			prior := []ShortcutAction{{WFWorkflowActionIdentifier: "is.workflow.actions.gettext", WFWorkflowActionParameters: map[string]any{"WFTextActionText": "raw", "UUID": "UNCHANGED"}}}
			_, err := resolveCompilerMacroCall("debugLog", map[string]any{"WFDebugLogLabel": invalid}, prior, "", "", profile)
			if err == nil || prior[0].WFWorkflowActionParameters["UUID"] != "UNCHANGED" {
				t.Fatal("invalid label accepted/mutated source")
			}
		}
	}
	for _, name := range []string{debugLabelVariableName, debugLabelDisplayVariablePrefix + "TEST"} {
		if err := inspectReleaseMacroReferences(map[string]any{"Type": "Variable", "VariableName": name}, nil); err == nil {
			t.Fatal("release consumed stripped label state")
		}
	}
}

// Inspect provenance while the actual native emission session still owns its
// manifest. The public bytes-only compile API closes/restores that session;
// classifying those bytes with later global state cannot prove ownership.
func compileLabelSourceWithProvenance(t *testing.T, content string) ([]ShortcutAction, []DebugTraceActionMapping) {
	t.Helper()
	file := source.NewFile(source.DocumentID("label-provenance.cherri"), "label-provenance.cherri", 1, content)
	parser := syntax.NewParser(file)
	program := parser.ParseProgram()
	if len(parser.Errors()) > 0 {
		t.Fatal(parser.Errors())
	}
	registry := schema.DefaultRegistry()
	graph, issue := modules.Resolve(program, "label-provenance.cherri", nil, registry, migrate.MigrateModuleSource)
	if issue != nil {
		t.Fatal(issue)
	}
	analyzer := analysis.NewAnalyzer(registry)
	analyzer.SetModuleGraph(graph)
	analyzer.Analyze(program)
	for _, d := range analyzer.Diagnostics() {
		if d.Severity == analysis.SeverityError {
			t.Fatal(d)
		}
	}
	session := NewCanonicalBackendSession()
	defer session.Close()
	emitter := lower.NewLowererWithSession(analyzer.Registry(), session)
	emitter.WorkflowName = "label-provenance"
	emitter.SetBaseFilePath("label-provenance.cherri")
	emitter.SetModuleGraph(graph)
	emitter.SetModuleRegistries(analyzer.ModuleRegistries())
	emitter.SetExprTypes(analyzer.ExprTypes())
	emitter.SetEnumValues(analyzer.EnumValues())
	if err := emitter.LowerProgramToSession(program); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Finalize(); err != nil {
		t.Fatal(err)
	}
	acts := append([]ShortcutAction(nil), session.shortcut.WFWorkflowActions...)
	return acts, ClassifyShortcutTraceActions(acts)
}

func TestDebugLabelsNamedDisplayModesAndProvenance(t *testing.T) {
	for _, mode := range []struct{ dates, show, text bool }{{false, false, false}, {true, false, false}, {false, true, false}, {true, true, false}, {false, false, true}, {true, false, true}} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			source := fmt.Sprintf("debugClearLogs()\nlet retained = base64Decode(\"AQ==\")\ndebugLog(label: \"API Response\", withDates: %t, showLogs: %t, showLogsAsText: %t)\ngetVariable(retained)\n", mode.dates, mode.show, mode.text)
			acts, mappings := compileLabelSourceWithProvenance(t, source)
			var raw, producer string
			rawAppends := 0
			metadataAppends := 0
			var projections []string
			for _, a := range acts {
				p := a.WFWorkflowActionParameters
				if _, leak := p["WFDebugLogLabel"]; leak {
					t.Fatal("compiler-only label option leaked to native action")
				}
				if a.WFWorkflowActionIdentifier == "is.workflow.actions.appendvariable" {
					if p["WFVariableName"] == debugLogVariableName {
						rawAppends++
						raw = labelTestTokenUUID(p["WFInput"])
					}
					if p["WFVariableName"] == debugLabelVariableName {
						metadataAppends++
					}
				}
				if a.WFWorkflowActionIdentifier == "is.workflow.actions.base64encode" {
					producer, _ = p["UUID"].(string)
				}
			}
			if rawAppends != 1 || metadataAppends != 1 || raw != producer || producer == "" {
				t.Fatal("raw File producer replaced or append duplicated")
			}
			if err := validateLabelTestGraph(acts, producer, "API Response"); err != nil {
				t.Fatal(err)
			}
			for _, m := range mappings {
				if m.Kind == "SOURCE_ACTION" {
					projections = append(projections, m.ActionIdentifier)
				}
			}
			_, baseline := compileLabelSourceWithProvenance(t, "let retained = base64Decode(\"AQ==\")\ngetVariable(retained)\n")
			var expected []string
			for _, m := range baseline {
				if m.Kind == "SOURCE_ACTION" {
					expected = append(expected, m.ActionIdentifier)
				}
			}
			if !reflect.DeepEqual(projections, expected) {
				t.Fatalf("compiler helper lost provenance/source was consumed: got%v expected%v", projections, expected)
			}
			checkDebugMainVoidBoundary(t, acts)
			consumer := acts[len(acts)-2]
			if consumer.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" || !labelTestHasRef(consumer.WFWorkflowActionParameters, producer) {
				t.Fatal("later source consumer is no longer original File reference")
			}
		})
	}
	// Fresh invocation has no metadata state from the previous compilation.
	fresh := compileDebugFixture(t, "text(\"plain\")\ndebugLog()\ndebugShowLogs()\n", false)
	checkDebugFallthroughSequence(t, fresh, false, "gettext", "appendvariable", "previewdocument")
}

func TestDebugLabelsActualLoopBranchesKeepSourceProjection(t *testing.T) {
	plain := `debugClearLogs()
let flag = number(0)
repeat 2 {
    debugShowLogs()
    if flag.number == 1 {
        text("branch")
        debugLog()
    } else {
        repeat 2 {
            text("nested")
            debugLog()
        }
    }
    debugClearLogs()
}
text("after")
`
	labeled := strings.Replace(plain, "debugLog()", "debugLog(label: \"branch\")", 1)
	labeled = strings.Replace(labeled, "debugLog()", "debugLog(label: \"nested\")", 1)
	sourceProjection := func(src string) ([]string, []ShortcutAction) {
		acts, mappings := compileLabelSourceWithProvenance(t, src)
		var result []string
		for _, m := range mappings {
			if m.Kind == "SOURCE_ACTION" {
				result = append(result, m.ActionIdentifier)
			}
		}
		return result, acts
	}
	// The baseline contains only authored effects/control flow. In particular,
	// Quick Look emitted by debugShowLogs is instrumentation, not a source action.
	baseline := plain
	for _, macro := range []string{"debugClearLogs()", "debugShowLogs()", "debugLog()"} {
		baseline = strings.ReplaceAll(baseline, macro, "")
	}
	expected, _ := sourceProjection(baseline)
	observed, acts := sourceProjection(labeled)
	if !reflect.DeepEqual(expected, observed) {
		t.Fatalf("source effects/control flow changed: got %v expected %v", observed, expected)
	}
	outerGroup := ""
	guardInsideOuter := false
	rawClears, metadataClears := 0, 0
	active := []string{}
	for _, a := range acts {
		p := a.WFWorkflowActionParameters
		id := a.WFWorkflowActionIdentifier
		mode := fmt.Sprint(p["WFControlFlowMode"])
		if (id == "is.workflow.actions.repeat.count" || id == "is.workflow.actions.repeat.each") && mode == "0" {
			g, _ := p["GroupingIdentifier"].(string)
			active = append(active, g)
			if outerGroup == "" && id == "is.workflow.actions.repeat.count" {
				outerGroup = g
			}
		}
		if id == "is.workflow.actions.count" && debugLabelIsVariable(p["Input"], debugLabelVariableName) && len(active) > 0 && active[0] == outerGroup {
			guardInsideOuter = true
		}
		if (id == "is.workflow.actions.repeat.count" || id == "is.workflow.actions.repeat.each") && mode == "2" {
			g, _ := p["GroupingIdentifier"].(string)
			if len(active) == 0 || active[len(active)-1] != g {
				t.Fatal("label preview changed native loop nesting")
			}
			active = active[:len(active)-1]
		}
		if id == "is.workflow.actions.setvariable" {
			if p["WFVariableName"] == debugLogVariableName {
				rawClears++
			}
			if p["WFVariableName"] == debugLabelVariableName {
				metadataClears++
			}
		}
	}
	if !guardInsideOuter || rawClears != 2 || metadataClears != 2 || len(active) != 0 {
		t.Fatalf("loop/clear metadata contract: %t %d/%d active%v", guardInsideOuter, rawClears, metadataClears, active)
	}
}

func TestDebugLabelsExactTwoReferenceUnicodeString(t *testing.T) {
	prefix := "שָׁלוֹם 👩‍💻 é "
	first := map[string]any{"Type": "ActionOutput", "OutputUUID": "RAW"}
	second := map[string]any{"Type": "ActionOutput", "OutputUUID": "DATE"}
	value := debugLabelText(prefix, first, " - ", second)["Value"].(map[string]any)
	expected := prefix + "\uFFFC - \uFFFC"
	if value["string"] != expected {
		t.Fatalf("native token placeholders changed: %q", value["string"])
	}
	attachments := value["attachmentsByRange"].(map[string]any)
	firstOffset := utf16CodeUnitCount(prefix)
	if len(attachments) != 2 || !reflect.DeepEqual(attachments[fmt.Sprintf("{%d, 1}", firstOffset)], first) || !reflect.DeepEqual(attachments[fmt.Sprintf("{%d, 1}", firstOffset+4)], second) {
		t.Fatalf("native Unicode ranges changed: %v", attachments)
	}
	if strings.Contains(value["string"].(string), `\uFFFC`) {
		t.Fatal("escaped textual placeholder emitted instead of native U+FFFC")
	}
}

func TestDebugLabelsClearRejectsUnexpectedNothingInput(t *testing.T) {
	for _, metadata := range []bool{false, true} {
		resetDebugInstrumentationManifest()
		raw, _ := buildDebugStdlibActions("debugClearLogs", nil, "", "")
		recordDebugExpandedActions("debugClearLogs", 0, raw)
		if metadata {
			var err error
			raw, _, err = backfillDebugLabelContext(raw)
			if err != nil {
				t.Fatal(err)
			}
		}
		index := 0
		if metadata {
			index = 2
		}
		raw[index].WFWorkflowActionParameters["WFInput"] = debugOutputRef("UNREVIEWED")
		if _, _, err := backfillDebugLabelContext(raw); err == nil {
			t.Fatal("Nothing with injected input accepted as exact zero clear")
		}
	}
}

func TestDebugLabelsMultipleEntriesAndUnlabeledAppend(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			declaration := "let"
			if legacy {
				declaration = "const"
			}
			source := fmt.Sprintf("debugClearLogs()\n%s first = number(42)\ndebugLog(false,false,false,\"first\")\n%s second = {\"name\":\"dictionary\"}\ndebugLog(false,false,false,\"שני 🍒\")\ntext(\"unlabeled\")\ndebugLog()\ndebugShowLogs(true)\ngetVariable(first)\ngetVariable(second)\n", declaration, declaration)
			acts := compileDebugFixture(t, source, legacy)
			byID := map[string]int{}
			var rawIDs, labels []string
			var rawIndices []int
			for i, a := range acts {
				p := a.WFWorkflowActionParameters
				if id, ok := p["UUID"].(string); ok {
					byID[id] = i
				}
				if a.WFWorkflowActionIdentifier == "is.workflow.actions.appendvariable" && p["WFVariableName"] == debugLogVariableName {
					rawIDs = append(rawIDs, labelTestTokenUUID(p["WFInput"]))
					rawIndices = append(rawIndices, i)
				}
				if a.WFWorkflowActionIdentifier == "is.workflow.actions.dictionary" {
					fields := labelTestFields(p["WFItems"])
					if field := fields["label"]; field != nil {
						value := debugParamMap(debugParamMap(field["WFValue"])["Value"])
						labels = append(labels, fmt.Sprint(value["string"]))
					}
				}
			}
			if len(rawIDs) != 3 || !reflect.DeepEqual(labels, []string{"first", "שני 🍒"}) {
				t.Fatalf("per-entry labels collapsed or raw count changed: %v %v", rawIDs, labels)
			}
			for _, id := range rawIDs {
				producer := byID[id]
				if acts[producer+1].WFWorkflowActionIdentifier != "is.workflow.actions.detect.date" || !labelTestHasRef(acts[producer+1].WFWorkflowActionParameters, id) {
					t.Fatal("delayed date request lost original producer timing")
				}
			}
			third := byID[rawIDs[2]]
			if rawIndices[2] != third+2 {
				t.Fatal("omitted label added operations beyond required Date plus one raw append")
			}
			firstConsumer := len(acts) - 2
			if !legacy {
				checkDebugMainVoidBoundary(t, acts)
				firstConsumer--
			}
			for i, id := range rawIDs[:2] {
				consumer := acts[firstConsumer+i]
				if consumer.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" || !labelTestHasRef(consumer.WFWorkflowActionParameters, id) {
					t.Fatal("later typed source consumer changed")
				}
			}
		})
	}
	original := debugOutputRefNamed("ORIGINAL-UUID", "Original File")
	acts, err := buildDebugStdlibActions("debugLog", map[string]any{"WFInput": original, "WFDebugLogLabel": "label"}, "LOG-RESULT", "Log Result")
	if err != nil {
		t.Fatal(err)
	}
	if acts[0].WFWorkflowActionParameters["UUID"] != "LOG-RESULT" || acts[0].WFWorkflowActionParameters["CustomOutputName"] != "Log Result" || !reflect.DeepEqual(acts[0].WFWorkflowActionParameters["WFInput"], original) {
		t.Fatal("public macro result or original output metadata changed")
	}
	if !reflect.DeepEqual(acts[len(acts)-1].WFWorkflowActionParameters["WFVariable"], original) {
		t.Fatal("implicit passthrough no longer original native object")
	}
}
