package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/migrate"
)

// The public synthetic setters are compiled and imported only. These tests
// never execute Reminder or Calendar mutations on a device.
type p15SetterCase struct {
	name, mode string
	value      *string
}

func p15SetterCases() []p15SetterCase {
	value := func(s string) *string { return &s }
	return []p15SetterCase{
		{"remove_property", "Remove", nil},
		{"remove_value", "Remove", value("OLD")},
		{"set_value", "Set", value("NEW")},
		{"set_remove_text", "Set", value("Remove")},
		{"set_remove_text_default", "Set", value("Remove")},
		{"set_set_text", "Set", value("Set")},
		{"set_empty_text", "Set", value("")},
	}
}

func p15SetterSource(action string, tc p15SetterCase) string {
	source := "let target = \"PUBLIC_SYNTHETIC_TARGET\"\n"
	if tc.value != nil {
		source += fmt.Sprintf("let value = %q\n", *tc.value)
	}
	source += action + "(target, detail: \"URL\""
	if tc.value != nil {
		source += ", newValue: value"
	}
	if strings.HasSuffix(tc.name, "_default") {
		return source + ")\n"
	}
	return source + fmt.Sprintf(", mode: %q)\n", tc.mode)
}

// The complete expected graph is authored from each scenario, including the
// exact data producers, setter mode, property, field presence and input edges.
func p15SetterGraphError(actual []map[string]any, action string, tc p15SetterCase) error {
	g := &p15FunctionGraph{actual: actual, groups: map[string]bool{}}
	target := g.value("gettext", map[string]any{"WFTextActionText": "PUBLIC_SYNTHETIC_TARGET"})
	params := map[string]any{"WFInput": p15FunctionEdge(target), "WFContentItemPropertyName": "URL", "Mode": tc.mode}
	kind, property := "setters.calendarevents", "WFCalendarEventContentItemURL"
	if action == "editReminder" {
		kind, property = "setters.reminders", "WFReminderContentItemURL"
	}
	if tc.mode == "Set" {
		params["ValueLabel"] = true
	}
	if tc.value != nil {
		value := g.value("gettext", map[string]any{"WFTextActionText": *tc.value})
		key := property
		if tc.mode == "Remove" {
			key = "RemoveSpecifiedValue"
		}
		params[key] = p15FunctionEdge(value)
	}
	g.emit(kind, params)
	// The shared builder's finish closes a user-function dispatcher. This
	// source has no dispatcher: compare its complete straight-line graph here.
	if g.err != nil {
		return g.err
	}
	if len(g.actual) != len(g.expected) {
		return fmt.Errorf("native action count %d != %d", len(g.actual), len(g.expected))
	}
	seen := map[string]bool{}
	for i, actual := range g.actual {
		if uuid, present := p15Param(actual)["UUID"]; present {
			id, ok := uuid.(string)
			if !ok || id == "" || seen[id] {
				return fmt.Errorf("invalid or duplicate producer identity at %d", i)
			}
			seen[id] = true
		}
		if !reflect.DeepEqual(p15FunctionComparable(actual, false), p15FunctionComparable(g.expected[i], false)) {
			return fmt.Errorf("native setter operation or edge mismatch at %d: got %#v want %#v", i, p15FunctionComparable(actual, false), p15FunctionComparable(g.expected[i], false))
		}
	}
	return nil
}

func p15SetterDocumentActions(t *testing.T, doc map[string]any) []map[string]any {
	t.Helper()
	raw, ok := doc["WFWorkflowActions"].([]any)
	if !ok {
		t.Fatalf("missing native action array: %T", doc["WFWorkflowActions"])
	}
	actions := make([]map[string]any, len(raw))
	for i, value := range raw {
		actions[i], ok = value.(map[string]any)
		if !ok {
			t.Fatalf("action %d is %T", i, value)
		}
	}
	return actions
}

func TestP15SetterModeSemanticRoundTrip(t *testing.T) {
	for _, action := range []string{"editEvent", "editReminder"} {
		for _, tc := range p15SetterCases() {
			t.Run(action+"/"+tc.name, func(t *testing.T) {
				r := newRoundTripRunner(t)
				first := r.compileSource("original", p15SetterSource(action, tc))
				if err := p15SetterGraphError(p15SetterDocumentActions(t, loadRoundTripDocument(t, first)), action, tc); err != nil {
					t.Fatalf("original graph: %v", err)
				}
				decompiled := r.decompileShortcut("imported", first)
				if !strings.Contains(decompiled, action+"(") {
					t.Fatalf("semantic import lost the natural setter call:\n%s", decompiled)
				}
				second := r.compileSource("recompiled", decompiled)
				if err := p15SetterGraphError(p15SetterDocumentActions(t, loadRoundTripDocument(t, second)), action, tc); err != nil {
					t.Fatalf("round-trip graph: %v\nsource:\n%s", err, decompiled)
				}
			})
		}
	}
}

func TestP15SetterShorthandMigration(t *testing.T) {
	for _, action := range []string{"editEvent", "editReminder"} {
		for _, tc := range []struct{ name, argument, mode, label string }{
			{"remove_double", "\"Remove\"", "Remove", "mode"},
			{"remove_single", "'Remove'", "Remove", "mode"},
			{"remove_lower", "\"remove\"", "Remove", "mode"},
			{"remove_upper", "\"REMOVE\"", "Remove", "mode"},
			{"remove_comment_before", "/* KEEP_REMOVE_COMMENT */ \"Remove\"", "Remove", "mode"},
			{"remove_comment_after", "\"Remove\" /* KEEP_REMOVE_COMMENT */", "Remove", "mode"},
			{"set_is_data", "\"Set\"", "Set", "newValue"},
			{"similar_text_is_data", "\"Remove, Set\"", "Set", "newValue"},
			{"explicit_remove_data", "newValue: \"Remove\"", "Set", "newValue"},
		} {
			t.Run(action+"/"+tc.name, func(t *testing.T) {
				input := "let target = \"PUBLIC_SYNTHETIC_TARGET\"\n" + action + "(target, \"URL\", " + tc.argument + ")\n"
				migrated, diagnostics, err := migrate.MigrateSourceWithDiagnostics(input)
				if err != nil || len(diagnostics) != 0 {
					t.Fatalf("migration: %v %v", err, diagnostics)
				}
				if strings.Contains(tc.argument, "KEEP_REMOVE_COMMENT") && !strings.Contains(migrated, "KEEP_REMOVE_COMMENT") {
					t.Fatal("migration discarded the argument comment")
				}
				if !strings.Contains(migrated, tc.label+": ") {
					t.Errorf("missing intended %s parameter:\n%s", tc.label, migrated)
				}
				r := newRoundTripRunner(t)
				doc := loadRoundTripDocument(t, r.compileSource("migrated", migrated))
				identifier := "is.workflow.actions.setters.calendarevents"
				if action == "editReminder" {
					identifier = "is.workflow.actions.setters.reminders"
				}
				params := findActionParams(doc, identifier)
				if params == nil || params["Mode"] != tc.mode {
					t.Fatalf("wrong setter mode: expected %q; got %v\n%s", tc.mode, params, migrated)
				}
				if tc.mode == "Remove" {
					for _, key := range []string{"ValueLabel", "RemoveSpecifiedValue", "WFCalendarEventContentItemURL", "WFReminderContentItemURL"} {
						if _, found := params[key]; found {
							t.Errorf("whole-property removal acquired %s", key)
						}
					}
					if err := p15SetterGraphError(p15SetterDocumentActions(t, doc), action, p15SetterCase{name: tc.name, mode: "Remove"}); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

func TestP15SetterModeGraphRejectsCorruption(t *testing.T) {
	for _, action := range []string{"editEvent", "editReminder"} {
		tc := p15SetterCases()[0]
		original := p15CompiledActions(t, p15SetterSource(action, tc))
		if err := p15SetterGraphError(original, action, tc); err != nil {
			t.Fatal(err)
		}
		for _, fault := range []string{"changed_mode", "injected_value", "missing_input", "wrong_property", "extra_action"} {
			t.Run(action+"/"+fault, func(t *testing.T) {
				clone := p15ContainerCloneNative(t, original)
				if err := p15SetterGraphError(clone, action, tc); err != nil {
					t.Fatal("unchanged clone rejected", err)
				}
				last := p15Param(clone[len(clone)-1])
				switch fault {
				case "changed_mode":
					last["Mode"] = "Set"
				case "injected_value":
					last["RemoveSpecifiedValue"] = "Remove"
				case "missing_input":
					delete(last, "WFInput")
				case "wrong_property":
					last["WFContentItemPropertyName"] = "Title"
				case "extra_action":
					clone = append(clone, clone[0])
				}
				if err := p15SetterGraphError(clone, action, tc); err == nil {
					t.Fatal("accepted corrupt native graph", fault)
				}
			})
		}
	}
}

// Resolve only the two representations this public Text specimen actually
// permits. Unknown item/attachment kinds never become a successful empty value.
func p15SetterLiteralText(actions []map[string]any, value any) (string, error) {
	if text, ok := value.(string); ok {
		return text, nil
	}
	attachment, ok := value.(map[string]any)
	if !ok || attachment["WFSerializationType"] != "WFTextTokenAttachment" {
		return "", fmt.Errorf("unexpected Text value %T", value)
	}
	ref, ok := attachment["Value"].(map[string]any)
	if !ok || ref["Type"] != "ActionOutput" {
		return "", fmt.Errorf("unexpected Text attachment")
	}
	for _, action := range actions {
		if p15Param(action)["UUID"] == ref["OutputUUID"] && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" {
			if text, ok := p15Param(action)["WFTextActionText"].(string); ok {
				return text, nil
			}
		}
	}
	return "", fmt.Errorf("Text attachment has no literal producer")
}

func TestP15SetterNamedModeWordsRemainData(t *testing.T) {
	for _, action := range []string{"editEvent", "editReminder"} {
		for index, value := range []string{"Remove", "Set", ""} {
			for _, authoredMode := range []string{"default", "Set", "Remove"} {
				t.Run(fmt.Sprintf("%s/value%d/%s", action, index, authoredMode), func(t *testing.T) {
					source := "let target = \"PUBLIC_SYNTHETIC_TARGET\"\n" + action + fmt.Sprintf("(target, detail: \"URL\", newValue: %q", value)
					mode := authoredMode
					if mode == "default" {
						mode = "Set"
					} else {
						source += fmt.Sprintf(", mode: %q", mode)
					}
					source += ")\n"
					actions := p15CompiledActions(t, source)
					if len(actions) < 2 {
						t.Fatal("missing literal input and setter")
					}
					last := actions[len(actions)-1]
					kind, property := "is.workflow.actions.setters.calendarevents", "WFCalendarEventContentItemURL"
					if action == "editReminder" {
						kind, property = "is.workflow.actions.setters.reminders", "WFReminderContentItemURL"
					}
					p := p15Param(last)
					if last["WFWorkflowActionIdentifier"] != kind || p["Mode"] != mode || p["WFContentItemPropertyName"] != "URL" {
						t.Fatalf("named data changed setter identity/mode/property: %v", last)
					}
					key, keys := property, 5
					if mode == "Remove" {
						key, keys = "RemoveSpecifiedValue", 4
						if _, present := p["ValueLabel"]; present {
							t.Fatal("Remove acquired ValueLabel")
						}
					} else if p["ValueLabel"] != true {
						t.Fatal("Set lost ValueLabel")
					}
					if len(p) != keys {
						t.Fatalf("setter parameter cardinality %d != %d: %v", len(p), keys, p)
					}
					raw, present := p[key]
					if !present {
						t.Fatalf("provided Text disappeared from %s", key)
					}
					text, err := p15SetterLiteralText(actions, raw)
					if err != nil || text != value {
						t.Fatalf("provided Text changed: %q %v; want %q", text, err, value)
					}
				})
			}
		}
	}
}

func TestP15SetterLegacyExplicitRemoveValue(t *testing.T) {
	for _, action := range []string{"editEvent", "editReminder"} {
		for _, explicit := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/explicit_%t", action, explicit), func(t *testing.T) {
				source := "#include 'actions/calendar'\nconst target = \"PUBLIC_SYNTHETIC_TARGET\"\n" + action + "(target, \"URL\", \"Remove\""
				if explicit {
					source += ", \"Remove\""
				}
				source += ")\n"
				r := newRoundTripRunner(t)
				actions := p15SetterDocumentActions(t, loadRoundTripDocument(t, r.compileSourceWithMode("legacy", source, true)))
				p := p15Param(actions[len(actions)-1])
				if p["Mode"] != "Remove" {
					t.Fatal("Legacy removal changed mode", p)
				}
				raw, present := p["RemoveSpecifiedValue"]
				if present != explicit {
					t.Fatalf("explicit fourth mode must distinguish value from shorthand: present=%t want=%t: %v", present, explicit, p)
				}
				if explicit {
					text, err := p15SetterLiteralText(actions, raw)
					if err != nil || text != "Remove" {
						t.Fatalf("Legacy explicit removal value changed: %q %v", text, err)
					}
				}
			})
		}
	}
}
