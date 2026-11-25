package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"howett.net/plist"
)

type p15ExportMetadataOptions struct {
	dates, labels, types, redact bool
	format                       string
}

func p15ExportMetadataOptionsSource(o p15ExportMetadataOptions) string {
	args := []string{strconv.Quote(o.format), strconv.FormatBool(o.dates), strconv.FormatBool(o.types), strconv.FormatBool(o.redact)}
	if o.labels {
		args = append(args, "true")
	}
	return strings.Join(args, ", ")
}

func p15ExportMetadataFixture(o p15ExportMetadataOptions, legacy bool) string {
	decl := "let"
	if legacy {
		decl = "const"
	}
	log := "debugLog()\n"
	if o.labels {
		log = "debugLog(false, false, false, " + strconv.Quote("LABEL שלום 🍒 \"line\"\n{literal}") + ")\n"
	}
	return "debugClearLogs()\n" + decl + " original = text(\"CHERRI_METADATA_PAYLOAD\")\n" + log +
		"text(\"CHERRI_EXPORT_BEFORE\")\n" + decl + " exported = debugExport(" + p15ExportMetadataOptionsSource(o) + ")\ngetVariable(original)\ngetVariable(exported)\n"
}

func p15ExportMetadataGraph(t *testing.T, actions []ShortcutAction) []ShortcutAction {
	t.Helper()
	start, end := -1, -1
	for i, action := range actions {
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.gettext" && action.WFWorkflowActionParameters["WFTextActionText"] == "CHERRI_EXPORT_BEFORE" {
			start = i + 1
		}
		if start >= 0 && action.WFWorkflowActionIdentifier == "is.workflow.actions.setitemname" {
			end = i + 1
		}
	}
	if start < 0 || end <= start {
		t.Fatal("authored export boundary missing")
	}
	return actions[start:end]
}

func p15ExportMetadataAllStrings(value any) []string {
	var out []string
	var walk func(any)
	walk = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		case string:
			out = append(out, v)
		}
	}
	walk(value)
	return out
}

func p15ExportMetadataStringContains(actions []ShortcutAction, literal string) bool {
	for _, action := range actions {
		for _, value := range p15ExportMetadataAllStrings(action.WFWorkflowActionParameters) {
			if strings.Contains(value, literal) {
				return true
			}
		}
	}
	return false
}

func p15ExportMetadataOutputRef(value any) string {
	m, _ := value.(map[string]any)
	if m["WFSerializationType"] == "WFTextTokenAttachment" {
		return p15ExportMetadataOutputRef(m["Value"])
	}
	if m["Type"] == "ActionOutput" {
		id, _ := m["OutputUUID"].(string)
		return id
	}
	if m["Type"] == "Variable" && m["Variable"] != nil {
		return p15ExportMetadataOutputRef(m["Variable"])
	}
	if m["WFSerializationType"] == "WFTextTokenString" {
		v, _ := m["Value"].(map[string]any)
		a, _ := v["attachmentsByRange"].(map[string]any)
		if len(a) == 1 {
			for _, r := range a {
				return p15ExportMetadataOutputRef(r)
			}
		}
	}
	return ""
}

func p15ExportMetadataGraphError(graph []ShortcutAction, o p15ExportMetadataOptions) error {
	if len(graph) < 4 {
		return fmt.Errorf("metadata exporter has no actual envelope")
	}
	byID := map[string]ShortcutAction{}
	byIndex := map[string]int{}
	for i, action := range graph {
		p := action.WFWorkflowActionParameters
		id := strings.TrimPrefix(action.WFWorkflowActionIdentifier, "is.workflow.actions.")
		switch id {
		case "gettext", "getitemtype", "base64encode", "repeat.each", "conditional", "text.combine", "setitemname",
			"getvariable", "count", "setvariable", "getvalueforkey", "format.date", "number", "math", "nothing",
			"dictionary", "round", "getitemfromlist", "repeat.count", "appendvariable":
		default:
			return fmt.Errorf("unexpected export effect %s", id)
		}
		if output, _ := p["UUID"].(string); output != "" {
			if _, exists := byID[output]; exists {
				return fmt.Errorf("duplicate producer")
			}
			byID[output], byIndex[output] = action, i
		}
		if id == "setvariable" || id == "appendvariable" {
			name, _ := p["WFVariableName"].(string)
			if !strings.HasPrefix(name, "__cherri_debug_export_") {
				return fmt.Errorf("export changed raw/shared state %q", name)
			}
		}
		if id == "base64encode" && p["WFEncodeMode"] == "Encode" && p["WFBase64LineBreakMode"] != "None" {
			return fmt.Errorf("metadata base64 encoding gained line breaks")
		}
	}
	// Keep the actual old carrier chain, without treating its native class as proved.
	last := graph[len(graph)-1]
	decoder := byID[p15ExportMetadataOutputRef(last.WFWorkflowActionParameters["WFInput"])]
	encoder := byID[p15ExportMetadataOutputRef(decoder.WFWorkflowActionParameters["WFInput"])]
	content := byID[p15ExportMetadataOutputRef(encoder.WFWorkflowActionParameters["WFInput"])]
	extension := "json"
	if o.format == "text" {
		extension = "txt"
	}
	if last.WFWorkflowActionIdentifier != "is.workflow.actions.setitemname" || last.WFWorkflowActionParameters["WFName"] != "cherri-debug."+extension ||
		decoder.WFWorkflowActionIdentifier != "is.workflow.actions.base64encode" || decoder.WFWorkflowActionParameters["WFEncodeMode"] != "Decode" ||
		encoder.WFWorkflowActionIdentifier != "is.workflow.actions.base64encode" || encoder.WFWorkflowActionParameters["WFEncodeMode"] != "Encode" ||
		content.WFWorkflowActionIdentifier != "is.workflow.actions.gettext" {
		return fmt.Errorf("existing materialization identity changed")
	}
	if _, err := p15ExportMetadataCanonicalGraph(graph); err != nil {
		return err
	}

	formatters := p15ActionsNamed(graph, "format.date")
	if !o.dates {
		if len(formatters) != 0 {
			return fmt.Errorf("omitted dates performed date conversion")
		}
	} else {
		if len(formatters) != 1 {
			return fmt.Errorf("source Creation Date formatter missing or repeated: %d", len(formatters))
		}
		p := formatters[0].WFWorkflowActionParameters
		if p["WFDateFormatStyle"] != "ISO 8601" || p["WFISO8601IncludeTime"] != true {
			return fmt.Errorf("date metadata lost explicit ISO8601 time contract")
		}
		dateID := p15ExportMetadataOutputRef(p["WFDate"])
		dateSource, exists := byID[dateID]
		if !exists || dateSource.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" {
			return fmt.Errorf("date formatter does not read the actual item metadata")
		}
		if !p15ExportMetadataStringContains([]ShortcutAction{dateSource}, "WFFileCreationDate") {
			return fmt.Errorf("date came from export time or unrelated input")
		}
		if p15ExportMetadataStringContains([]ShortcutAction{dateSource}, "CurrentDate") {
			return fmt.Errorf("source Creation Date replaced by export timestamp")
		}
		for _, marker := range []string{"source-creation-date", "iso8601", "\"date\":", "ABSENT", "AMBIGUOUS", "UNSUPPORTED_DATE_TYPE"} {
			if !p15ExportMetadataStringContains(graph, marker) {
				return fmt.Errorf("observable date status/contract %q missing", marker)
			}
		}
		counted, typed := false, false
		for _, action := range graph {
			p := action.WFWorkflowActionParameters
			if action.WFWorkflowActionIdentifier == "is.workflow.actions.count" && p15ExportMetadataOutputRef(p["Input"]) == dateID && p["WFCountType"] == "Items" {
				counted = byIndex[fmt.Sprint(p["UUID"])] < byIndex[fmt.Sprint(formatters[0].WFWorkflowActionParameters["UUID"])]
			}
			if action.WFWorkflowActionIdentifier == "is.workflow.actions.getitemtype" && p15ExportMetadataOutputRef(p["WFInput"]) == dateID {
				typed = true
			}
		}
		if !counted || !typed {
			return fmt.Errorf("date conversion lacks cardinality/type evidence")
		}
	}

	if o.labels {
		if !p15ExportMetadataStringContains(graph, "\"label\":") {
			return fmt.Errorf("requested label metadata missing")
		}
		if o.redact {
			for _, action := range graph {
				if action.WFWorkflowActionIdentifier == "is.workflow.actions.getvalueforkey" && action.WFWorkflowActionParameters["WFDictionaryKey"] == "label" {
					return fmt.Errorf("redacted export extracted a label value")
				}
				if p15ExportMetadataStringContains([]ShortcutAction{action}, debugLabelVariableName) {
					return fmt.Errorf("redacted export read label metadata")
				}
			}
			if !p15ExportMetadataStringContains(graph, "REDACTED") {
				return fmt.Errorf("redacted label status missing")
			}
		} else {
			keys := map[string]int{}
			for _, action := range graph {
				if action.WFWorkflowActionIdentifier == "is.workflow.actions.getvalueforkey" {
					key, _ := action.WFWorkflowActionParameters["WFDictionaryKey"].(string)
					keys[key]++
				}
			}
			for _, key := range []string{"first", "last", "label"} {
				if keys[key] == 0 {
					return fmt.Errorf("label association field %q missing", key)
				}
			}
			if !p15ExportMetadataStringContains(graph, debugLabelVariableName) {
				return fmt.Errorf("labels do not use the original metadata buffer")
			}
			for _, marker := range []string{"ABSENT", "AMBIGUOUS", "UNSUPPORTED_LABEL_METADATA"} {
				if !p15ExportMetadataStringContains(graph, marker) {
					return fmt.Errorf("observable label status %q missing", marker)
				}
			}
		}
	}
	return nil
}

func TestP15DebugExportMetadataSourceGraphs(t *testing.T) {
	for _, mode := range []struct {
		name          string
		dates, labels bool
	}{{"dates", true, false}, {"labels", false, true}, {"dates_labels", true, true}} {
		for _, legacy := range []bool{false, true} {
			for _, format := range []string{"json", "text"} {
				for _, includeTypes := range []bool{false, true} {
					for _, redact := range []bool{false, true} {
						t.Run(fmt.Sprintf("%s/legacy_%t/%s/types_%t/redact_%t", mode.name, legacy, format, includeTypes, redact), func(t *testing.T) {
							o := p15ExportMetadataOptions{dates: mode.dates, labels: mode.labels, types: includeTypes, redact: redact, format: format}
							// An ordinary old-API control establishes that this source's
							// includes, declarations, label literals and consumers are valid.
							control := o
							control.dates, control.labels = false, false
							controlSource := strings.Replace(p15ExportMetadataFixture(o, legacy), "debugExport("+p15ExportMetadataOptionsSource(o)+")", "debugExport("+p15ExportMetadataOptionsSource(control)+")", 1)
							if _, err := compileBuildProfileFixture(t, controlSource, legacy, BuildProfileDebug); err != nil {
								t.Fatalf("ordinary export source control: %v", err)
							}
							actions, err := compileBuildProfileFixture(t, p15ExportMetadataFixture(o, legacy), legacy, BuildProfileDebug)
							if err != nil {
								t.Fatal(err)
							}
							graph := p15ExportMetadataGraph(t, actions)
							if err = p15ExportMetadataGraphError(graph, o); err != nil {
								t.Fatal(err)
							}
							// Export must not rewrite the original logged value or its consumer.
							original := ""
							for _, action := range actions {
								if action.WFWorkflowActionIdentifier == "is.workflow.actions.gettext" && action.WFWorkflowActionParameters["WFTextActionText"] == "CHERRI_METADATA_PAYLOAD" {
									original = fmt.Sprint(action.WFWorkflowActionParameters["UUID"])
								}
							}
							count := 0
							for _, action := range actions {
								if action.WFWorkflowActionIdentifier == "is.workflow.actions.getvariable" && p15ExportMetadataOutputRef(action.WFWorkflowActionParameters["WFVariable"]) == original {
									count++
								}
							}
							if original == "" || count < 1 {
								t.Fatal("original native producer consumer lost")
							}
							if out := os.Getenv("CHERRI_EXPORT_METADATA_GRAPH_OUT"); out != "" {
								artifact := map[string]any{
									"id": t.Name(), "options": map[string]any{
										"dates": o.dates, "labels": o.labels, "types": o.types, "redact": o.redact, "format": o.format, "legacy": legacy,
									}, "actions": graph,
								}
								data, err := json.MarshalIndent(artifact, "", "  ")
								if err != nil {
									t.Fatal(err)
								}
								path := filepath.Join(out, strings.ReplaceAll(t.Name(), "/", "-")+".json")
								f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
								if err != nil {
									t.Fatal(err)
								}
								if _, err = f.Write(append(data, '\n')); err != nil {
									t.Fatal(err)
								}
								if err = f.Close(); err != nil {
									t.Fatal(err)
								}
							}
						})
					}
				}
			}
		}
	}
}

func TestP15DebugExportMetadataDefaultFlagsControl(t *testing.T) {
	for _, format := range []string{"json", "text"} {
		for _, redact := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/redact_%t", format, redact), func(t *testing.T) {
				params := map[string]any{"WFDebugExportFormat": format, "WFDebugExportRedact": redact}
				old, err := buildDebugExportActions(params, "EXACT-RESULT", "Exact Result")
				if err != nil {
					t.Fatal(err)
				}
				params["WFDebugExportDates"] = false
				params["WFDebugExportLabels"] = false
				current, err := buildDebugExportActions(params, "EXACT-RESULT", "Exact Result")
				if err != nil {
					t.Fatal(err)
				}
				oldWire := p15ExportMetadataNativeWire(t, old)
				currentWire := p15ExportMetadataNativeWire(t, current)
				a, err := p15ExportMetadataCanonicalGraph(oldWire)
				if err != nil {
					t.Fatal(err)
				}
				b, err := p15ExportMetadataCanonicalGraph(currentWire)
				if err != nil {
					t.Fatal(err)
				}
				baseline, err := os.ReadFile(filepath.Join("testdata", "p15-debug-export-metadata", "default", fmt.Sprintf("%s-redact_%t.json", format, redact)))
				if err != nil {
					t.Fatal(err)
				}
				actual, err := json.MarshalIndent(a, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if string(append(actual, '\n')) != string(baseline) {
					t.Fatal("default native graph differs from the actual pre-implementation source75c6 baseline")
				}
				if !reflect.DeepEqual(a, b) {
					t.Fatalf("omitted metadata differs from explicit false flags: old=%#v; current=%#v", a, b)
				}

				for _, fault := range []string{"missing_producer", "empty_producer", "nonpreceding_producer", "duplicate_producer"} {
					t.Run(fault, func(t *testing.T) {
						bad, err := buildDebugExportActions(params, "EXACT-RESULT", "Exact Result")
						if err != nil {
							t.Fatal(err)
						}
						last := len(bad) - 1
						switch fault {
						case "missing_producer":
							bad[last].WFWorkflowActionParameters["WFInput"] = debugOutputRef("NONEXISTENT")
						case "empty_producer":
							bad[last].WFWorkflowActionParameters["WFInput"] = debugOutputRef("")
						case "nonpreceding_producer":
							bad[last].WFWorkflowActionParameters["WFInput"] = debugOutputRef("EXACT-RESULT")
						case "duplicate_producer":
							for i := 0; i < last; i++ {
								if id, ok := bad[i].WFWorkflowActionParameters["UUID"].(string); ok && id != "" {
									bad[last].WFWorkflowActionParameters["UUID"] = id
									break
								}
							}
						}
						if _, err = p15ExportMetadataCanonicalGraph(p15ExportMetadataNativeWire(t, bad)); err == nil {
							t.Fatalf("actual plist wire oracle accepted %s", fault)
						}
					})
				}

				// Arbitrary payload strings that happen to equal a producer UUID
				// are never alpha-renamed by this graph identity comparison.
				for _, fault := range []string{"uuid_literal_payload", "other_producer_output_name"} {
					t.Run(fault, func(t *testing.T) {
						changed := p15ExportMetadataNativeWire(t, old)
						var actualIDs []string
						for _, action := range changed {
							if id, ok := action.WFWorkflowActionParameters["UUID"].(string); ok && id != "" && id != "EXACT-RESULT" {
								actualIDs = append(actualIDs, id)
							}
						}
						if len(actualIDs) < 2 {
							t.Fatal("missing independently produced native UUIDs")
						}
						mutated := false
						if fault == "uuid_literal_payload" {
							for i := range changed {
								if changed[i].WFWorkflowActionIdentifier == "is.workflow.actions.gettext" {
									changed[i].WFWorkflowActionParameters["WFTextActionText"] = actualIDs[0]
									mutated = true
									break
								}
							}
						} else {
							var walk func(any)
							walk = func(value any) {
								switch v := value.(type) {
								case map[string]any:
									if !mutated && v["Type"] == "ActionOutput" && v["OutputName"] == v["OutputUUID"] {
										other := actualIDs[0]
										if other == v["OutputUUID"] {
											other = actualIDs[1]
										}
										v["OutputName"] = other
										mutated = true
										return
									}
									for _, child := range v {
										walk(child)
									}
								case []any:
									for _, child := range v {
										walk(child)
									}
								}
							}
							for _, action := range changed {
								walk(action.WFWorkflowActionParameters)
							}
						}
						if !mutated {
							t.Fatalf("missing causal fault target: %s", fault)
						}
						got, err := p15ExportMetadataCanonicalGraph(changed)
						if err != nil {
							t.Fatal(err)
						}
						if reflect.DeepEqual(a, got) {
							t.Fatalf("canonical comparison erased semantic mutation %s", fault)
						}
					})
				}

				if out := os.Getenv("CHERRI_EXPORT_METADATA_DEFAULT_GOLDEN_OUT"); out != "" {
					data, err := json.MarshalIndent(a, "", "  ")
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(out, fmt.Sprintf("%s-redact_%t.json", format, redact))
					f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
					if err != nil {
						t.Fatal(err)
					}
					if _, err = f.Write(append(data, '\n')); err != nil {
						t.Fatal(err)
					}
					if err = f.Close(); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

func p15ExportMetadataNativeWire(t *testing.T, actions []ShortcutAction) []ShortcutAction {
	t.Helper()
	// Exercise the actual plist wire rules. JSON of the intermediate Go Value
	// struct includes zero fields that plist intentionally omits.
	data, err := plist.Marshal(actions, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	var raw []map[string]any
	if _, err = plist.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	wire := make([]ShortcutAction, len(raw))
	for i, action := range raw {
		id, ok := action["WFWorkflowActionIdentifier"].(string)
		if !ok {
			t.Fatalf("native action %d has no identifier", i)
		}
		params, _ := action["WFWorkflowActionParameters"].(map[string]any)
		wire[i] = ShortcutAction{WFWorkflowActionIdentifier: id, WFWorkflowActionParameters: params}
	}
	return wire
}

func p15ExportMetadataCanonicalGraph(actions []ShortcutAction) (any, error) {
	var visit func(any) error
	visit = func(value any) error {
		switch v := value.(type) {
		case map[string]any:
			if v["Type"] == "ActionOutput" {
				id, ok := v["OutputUUID"].(string)
				if !ok || id == "" {
					return fmt.Errorf("native ActionOutput has no producer UUID")
				}
			}
			for _, child := range v {
				if err := visit(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range v {
				if err := visit(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, action := range actions {
		if err := visit(action.WFWorkflowActionParameters); err != nil {
			return nil, err
		}
	}
	canonical, err := p15MeasureVoidCanonicalGraph(actions)
	if err != nil {
		return nil, err
	}
	// The shared text codec defaults an unnamed ActionOutput's OutputName to
	// that SAME producer UUID. Normalize only this verified reference field;
	// UUID-shaped text payloads and names of other producers remain semantic.
	var normalizeProducerName func(any, any)
	normalizeProducerName = func(original, normalized any) {
		switch src := original.(type) {
		case map[string]any:
			dst, ok := normalized.(map[string]any)
			if !ok {
				return
			}
			if src["Type"] == "ActionOutput" && src["OutputName"] == src["OutputUUID"] {
				if id, ok := src["OutputUUID"].(string); ok && id != "" {
					dst["OutputName"] = dst["OutputUUID"]
				}
			}
			for key, child := range src {
				normalizeProducerName(child, dst[key])
			}
		case []any:
			dst, ok := normalized.([]any)
			if !ok || len(dst) != len(src) {
				return
			}
			for i, child := range src {
				normalizeProducerName(child, dst[i])
			}
		}
	}
	normalized := canonical.([]any)
	for i, action := range actions {
		normalizeProducerName(action.WFWorkflowActionParameters, normalized[i].(map[string]any)["parameters"])
	}
	return canonical, nil
}

func TestP15DebugExportMetadataReleaseAndStorage(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy_%t", legacy), func(t *testing.T) {
			for _, format := range []string{"json", "text"} {
				t.Run("release_"+format, func(t *testing.T) {
					source := "text(\"kept-before\")\ndebugLog()\ndebugExport(" + strconv.Quote(format) + ", true, true, false, true)\ntext(\"kept-after\")\n"
					got, err := compileBuildProfileFixture(t, source, legacy, BuildProfileRelease)
					if err != nil {
						t.Fatal(err)
					}
					want, err := compileBuildProfileFixture(t, "text(\"kept-before\")\ntext(\"kept-after\")\n", legacy, BuildProfileRelease)
					if err != nil {
						t.Fatal(err)
					}
					a, err := p15ExportMetadataCanonicalGraph(got)
					if err != nil {
						t.Fatal(err)
					}
					b, err := p15ExportMetadataCanonicalGraph(want)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(a, b) {
						t.Fatal("Release metadata introduced helpers or changed authored effects")
					}
				})
			}
			t.Run("storage_only_backfill", func(t *testing.T) {
				source := "debugClearLogs()\ntext(\"logged\")\ndebugLog()\ndebugShowLogs()\n" +
					"debugExport(\"json\", false, false, false, true)\n" +
					"debugExport(\"json\", false, false, false, true)\n"
				actions, err := compileBuildProfileFixture(t, source, legacy, BuildProfileDebug)
				if err != nil {
					t.Fatal(err)
				}
				if len(p15ActionsNamed(actions, "previewdocument")) != 1 {
					t.Fatal("export label storage rewrote or duplicated an earlier preview")
				}
				clears := 0
				for i, action := range actions {
					p := action.WFWorkflowActionParameters
					if action.WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" && p["WFVariableName"] == debugLabelVariableName {
						clears++
						if _, exists := p["WFInput"]; exists || i == 0 ||
							actions[i-1].WFWorkflowActionIdentifier != "is.workflow.actions.nothing" {
							t.Fatal("label storage backfill must preserve the exact zero-output clear")
						}
					}
				}
				if clears != 1 {
					t.Fatalf("metadata storage clear was not idempotent: %d", clears)
				}
			})
			t.Run("later_label_in_outer_loop", func(t *testing.T) {
				begin := "repeat 2 {\n"
				if legacy {
					begin = "repeat outerIndex for 2 {\n"
				}
				source := "debugClearLogs()\n" + begin +
					"text(\"before\")\ndebugLog()\ndebugExport(\"json\", true, true, false, true)\n" +
					"text(\"after\")\ndebugLog(false, false, false, \"later label\")\n}\n"
				actions, err := compileBuildProfileFixture(t, source, legacy, BuildProfileDebug)
				if err != nil {
					t.Fatal(err)
				}
				clearCount, labelAppends := 0, 0
				for _, action := range actions {
					p := action.WFWorkflowActionParameters
					if p["WFVariableName"] != debugLabelVariableName {
						continue
					}
					if action.WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" {
						clearCount++
					}
					if action.WFWorkflowActionIdentifier == "is.workflow.actions.appendvariable" {
						labelAppends++
					}
				}
				if clearCount != 1 || labelAppends != 1 {
					t.Fatal("future loop labels were cleared at export or lost their original append")
				}
			})
		})
	}
}
