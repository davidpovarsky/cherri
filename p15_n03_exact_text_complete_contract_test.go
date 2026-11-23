package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/migrate"
)

const p15N03JoinedLegacySource = "const subject = text(\"AbC\")\n" +
	"if subject == \"abc\" {\n text(\"N03_OLD_ORDINARY\")\n}\n" +
	"if legacy subject != \"ABC\" {\n text(\"N03_OLD_LEGACY\")\n}\n"

var p15N03JoinedOriginalModes = []p15N03MigrationCondition{
	{code: 4, right: "abc"},
	{code: 5, right: "ABC", present: true, legacy: true},
}

func p15N03JoinedOldModes(actions []map[string]any) error {
	actual, err := p15N03MigrationConditions(actions)
	if err != nil || !reflect.DeepEqual(actual, p15N03JoinedOriginalModes) {
		return fmt.Errorf("joined original comparison modes differ: %v %v", actual, err)
	}
	// The complete compatibility prefix contains exactly one source Text and
	// the two original start/Text/end triples. Mode-only substring evidence
	// cannot hide an added action or a changed authored branch payload.
	identifiers := []string{"gettext", "conditional", "gettext", "conditional", "conditional", "gettext", "conditional"}
	if len(actions) != len(identifiers) {
		return fmt.Errorf("joined compatibility action inventory differs")
	}
	for index, identifier := range identifiers {
		if actions[index]["WFWorkflowActionIdentifier"] != "is.workflow.actions."+identifier {
			return fmt.Errorf("joined compatibility action order differs")
		}
	}
	if p15N03ExactParams(actions[0])["WFTextActionText"] != "AbC" ||
		p15N03ExactParams(actions[2])["WFTextActionText"] != "N03_OLD_ORDINARY" ||
		p15N03ExactParams(actions[5])["WFTextActionText"] != "N03_OLD_LEGACY" {
		return fmt.Errorf("joined compatibility source/branch payload differs")
	}
	var subject string
	count := 0
	for _, action := range actions {
		parameters := p15N03ExactParams(action)
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" &&
			parameters["WFTextActionText"] == "AbC" {
			subject, _ = parameters["UUID"].(string)
			count++
		}
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.text.match" {
			return fmt.Errorf("joined compatibility prefix acquired exact comparison helpers")
		}
	}
	if count != 1 || subject == "" {
		return fmt.Errorf("joined compatibility source subject is not unique")
	}
	for _, action := range actions {
		parameters := p15N03ExactParams(action)
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" ||
			p15Int(parameters["WFControlFlowMode"]) != 0 {
			continue
		}
		edge, err := p15N03ExactToken(parameters["WFInput"])
		if err != nil || edge != subject {
			return fmt.Errorf("joined compatibility condition detached from actual subject")
		}
	}
	return nil
}

// One actual V2 document contains the migrated old conditions and separately
// authored exact comparisons. The original Legacy graph supplies its own
// independent differential; neither side borrows another requirement's proof.
func p15N03JoinedGraph(actions []map[string]any) error {
	if err := p15N03ExactControlGroupsError(actions); err != nil {
		return err
	}
	marker := -1
	for index, action := range actions {
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.comment" &&
			p15N03ExactParams(action)["WFCommentActionText"] == "N03_AUTHORED_BOUNDARY" {
			if marker >= 0 {
				return fmt.Errorf("duplicate joined source boundary")
			}
			marker = index
		}
	}
	if marker < 0 {
		return fmt.Errorf("missing joined source boundary")
	}
	if err := p15N03JoinedOldModes(actions[:marker]); err != nil {
		return err
	}
	left, li, err := p15N03ExactNamedReference(actions, "subject")
	if err != nil || li >= marker {
		return fmt.Errorf("joined migrated subject differs: %v", err)
	}
	right, ri, err := p15N03ExactNamedReference(actions, "right")
	if err != nil || ri != marker+1 {
		return fmt.Errorf("joined authored RHS source position differs: %v", err)
	}
	cursor := p15N03Cursor{actions: p15DiagnosticGraph(actions), index: marker + 1, groups: map[string]bool{}}
	observedRight := cursor.take("gettext", "right", map[string]any{"WFTextActionText": "abc"}, true)
	if cursor.err != nil {
		return cursor.err
	}
	if !reflect.DeepEqual(observedRight, right) {
		return fmt.Errorf("joined authored RHS identity differs")
	}
	for _, comparison := range []struct {
		name string
		code float64
	}{
		{"authoredEqual", 4}, {"authoredNotEqual", 5},
	} {
		result := cursor.exactValueComparison(comparison.name, left, comparison.code, right)
		if cursor.err != nil {
			return cursor.err
		}
		if err := p15N03ExactResultConsumer(actions, cursor.index, result); err != nil {
			return err
		}
	}
	for _, folded := range []struct {
		name  string
		value uint64
	}{
		{"foldedSame", 1}, {"foldedCase", 0},
	} {
		if cursor.index >= len(actions) {
			return fmt.Errorf("joined folded source number missing")
		}
		// Only these two authored integral result fields admit the exact
		// native unsigned/retained-JSON representations of independent 1/0.
		rawNumber := p15N03ExactParams(actions[cursor.index])["WFNumberActionNumber"]
		value, okay := p15N03ControlInteger(rawNumber)
		if !okay || value != folded.value {
			return fmt.Errorf("joined folded source number differs")
		}
		result := cursor.take("number", folded.name, map[string]any{"WFNumberActionNumber": rawNumber}, true)
		if cursor.err != nil {
			return cursor.err
		}
		if err := p15N03ExactResultConsumer(actions, cursor.index, result); err != nil {
			return err
		}
	}
	if err := p15N03ExactPrimitiveError(actions, 2); err != nil {
		return err
	}
	// Every consumer below is an authored getVariable; no helper can replace
	// a result with a constant or a compatibility-prefix output.
	for _, name := range []string{"authoredEqual", "authoredNotEqual", "foldedSame", "foldedCase"} {
		ref, _, err := p15N03ExactNamedReference(actions, name)
		if err != nil {
			return err
		}
		cursor.take("getvariable", "", map[string]any{"WFVariable": p15N03Attachment(ref)}, false)
		if cursor.err != nil {
			return cursor.err
		}
	}
	if cursor.index != len(actions) {
		return fmt.Errorf("joined graph has extra or missing final source actions")
	}
	return nil
}

func TestP15N03ExactTextCompleteContract(t *testing.T) {
	legacy := p15N03MigrationCompile(t, p15N03JoinedLegacySource, "legacy")
	if err := p15N03JoinedOldModes(legacy); err != nil {
		t.Fatal(err)
	}
	converted, diagnostics, err := migrate.MigrateSourceWithDiagnostics(p15N03JoinedLegacySource)
	if err != nil || len(diagnostics) != 0 {
		t.Fatal("actual joined migration failed", err, diagnostics)
	}
	if !strings.Contains(converted, "if modern:") || !strings.Contains(converted, "if legacy:") {
		t.Fatal("joined migrated comparison intent is not explicit")
	}
	source := converted + "\ncomment(\"N03_AUTHORED_BOUNDARY\")\n" +
		"let right=text(\"abc\")\n" +
		"let authoredEqual=subject==right\n" +
		"let authoredNotEqual=subject!=right\n" +
		"let foldedSame=\"AbC\"==\"AbC\"\n" +
		"let foldedCase=\"AbC\"==\"abc\"\n" +
		"getVariable(authoredEqual)\ngetVariable(authoredNotEqual)\n" +
		"getVariable(foldedSame)\ngetVariable(foldedCase)\n"
	good := p15N03ExactCompile(t, source)
	if err := p15N03JoinedGraph(good); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{
		"ordinary_mode", "legacy_mode", "legacy_subject", "legacy_branch", "compatibility_extra_action", "extra_action_after_boundary", "authored_operator",
		"authored_rhs", "exact_case_flag", "folded_case", "folded_type", "final_consumer",
	} {
		t.Run(fault, func(t *testing.T) {
			raw, err := json.Marshal(good)
			if err != nil {
				t.Fatal(err)
			}
			var candidate []map[string]any
			if err = json.Unmarshal(raw, &candidate); err != nil {
				t.Fatal(err)
			}
			if err := p15N03JoinedGraph(candidate); err != nil {
				t.Fatal("unchanged retained-JSON graph failed before the causal mutation", err)
			}
			changed := false
			if fault == "compatibility_extra_action" {
				candidate = append([]map[string]any{{
					"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
					"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "N03_UNAUTHORED_EXTRA"},
				}}, candidate...)
				changed = true
			}
			if fault == "extra_action_after_boundary" {
				boundary := -1
				for index, action := range candidate {
					if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.comment" &&
						p15N03ExactParams(action)["WFCommentActionText"] == "N03_AUTHORED_BOUNDARY" {
						boundary = index
					}
				}
				if boundary < 0 {
					t.Fatal("actual joined boundary missing")
				}
				expanded := make([]map[string]any, 0, len(candidate)+1)
				expanded = append(expanded, candidate[:boundary+1]...)
				expanded = append(expanded, map[string]any{
					"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
					"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "N03_UNAUTHORED_GAP"},
				})
				expanded = append(expanded, candidate[boundary+1:]...)
				candidate = expanded
				changed = true
			}
			for index, action := range candidate {
				p := p15N03ExactParams(action)
				switch fault {
				case "ordinary_mode":
					if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" && p15Int(p["WFControlFlowMode"]) == 0 && p["WFConditionalActionString"] == "abc" {
						p["WFConditionalLegacyComparisonBehavior"] = true
						changed = true
					}
				case "legacy_mode":
					if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" && p15Int(p["WFControlFlowMode"]) == 0 && p["WFConditionalActionString"] == "ABC" {
						delete(p, "WFConditionalLegacyComparisonBehavior")
						changed = true
					}
				case "legacy_subject":
					if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" && p15Int(p["WFControlFlowMode"]) == 0 && p["WFConditionalActionString"] == "ABC" {
						ref, _, e := p15N03ExactNamedReference(candidate, "right")
						if e != nil {
							t.Fatal(e)
						}
						p["WFInput"] = map[string]any{"Type": "Variable", "Variable": p15N03Attachment(ref)}
						changed = true
					}
				case "legacy_branch":
					if p["WFTextActionText"] == "N03_OLD_LEGACY" {
						p["WFTextActionText"] = "N03_CHANGED_BRANCH"
						changed = true
					}
				case "authored_operator":
					if p["CustomOutputName"] == "authoredNotEqual" {
						p15N03ExactParams(candidate[index-4])["WFCondition"] = float64(4)
						changed = true
					}
				case "authored_rhs":
					if p["CustomOutputName"] == "right" {
						p["WFTextActionText"] = "AbC"
						changed = true
					}
				case "exact_case_flag":
					if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.text.match" {
						p["WFMatchTextCaseSensitive"] = false
						changed = true
					}
				case "folded_case":
					if p["CustomOutputName"] == "foldedCase" {
						p["WFNumberActionNumber"] = float64(1)
						changed = true
					}
				case "folded_type":
					if p["CustomOutputName"] == "foldedCase" {
						p["WFNumberActionNumber"] = "0"
						changed = true
					}
				case "final_consumer":
					if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvariable" {
						ref, _, e := p15N03ExactNamedReference(candidate, "subject")
						if e != nil {
							t.Fatal(e)
						}
						p["WFVariable"] = p15N03Attachment(ref)
						changed = true
					}
				}
			}
			if !changed {
				t.Fatal("joined causal control did not change its target", fault)
			}
			wantError := map[string]string{
				"ordinary_mode":               "joined original comparison modes differ",
				"legacy_mode":                 "joined original comparison modes differ",
				"legacy_subject":              "joined compatibility condition detached",
				"legacy_branch":               "joined compatibility source/branch payload differs",
				"compatibility_extra_action":  "joined compatibility action inventory differs",
				"extra_action_after_boundary": "joined authored RHS source position differs",
				"authored_operator":           "N03 action",
				"authored_rhs":                "N03 action",
				"exact_case_flag":             "N03 action",
				"folded_case":                 "joined folded source number differs",
				"folded_type":                 "joined folded source number differs",
				"final_consumer":              "actual source getVariable did not consume",
			}[fault]
			if err := p15N03JoinedGraph(candidate); err == nil || !strings.Contains(err.Error(), wantError) {
				t.Fatal("joined corruption did not reach its intended rejection", fault, wantError, err)
			}
		})
	}
	t.Log("N03 complete local unit/native-structure scenario: actual Legacy migration and new typed Text graphs coexist; no Apple runtime claim")
}
