package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestP13_SimulatorQASyntheticFixturesAndInstrumentationParity(t *testing.T) {
	fixturesDir := filepath.Join("tools", "shortcuts-simulator-qa", "fixtures")
	specs := []struct {
		id         string
		file       string
		legacy     bool
		minActions int
	}{
		{"qa-functional-v2", "qa-functional-v2.cherri", false, 15},
		{"qa-functional-legacy", "qa-functional-legacy.cherri", true, 15},
		{"qa-scroll-editor-v2", "qa-scroll-editor-v2.cherri", false, 28},
		{"qa-scroll-editor-legacy", "qa-scroll-editor-legacy.cherri", true, 28},
		{"qa-rtl-hebrew-v2", "qa-rtl-hebrew-v2.cherri", false, 8},
		{"qa-functional-v2-uninstrumented", "qa-functional-v2-uninstrumented.cherri", false, 10},
		{"qa-functional-legacy-uninstrumented", "qa-functional-legacy-uninstrumented.cherri", true, 10},
	}

	compiled := make(map[string][]ShortcutAction)
	for _, sp := range specs {
		rawBytes, err := os.ReadFile(filepath.Join(fixturesDir, sp.file))
		if err != nil {
			t.Fatalf("read fixture %s: %v", sp.file, err)
		}
		acts := compileDebugFixture(t, string(rawBytes), sp.legacy)
		if len(acts) < sp.minActions {
			t.Fatalf("%s: expected >= %d native actions, got %d", sp.id, sp.minActions, len(acts))
		}
		compiled[sp.id] = acts
	}

	// Verify uninstrumented vs instrumented SOURCE_ACTION projection parity in both V2 and Legacy
	for _, pair := range []struct {
		instID   string
		uninstID string
	}{
		{"qa-functional-v2", "qa-functional-v2-uninstrumented"},
		{"qa-functional-legacy", "qa-functional-legacy-uninstrumented"},
	} {
		instMap := ClassifyShortcutTraceActions(compiled[pair.instID])
		uninstMap := ClassifyShortcutTraceActions(compiled[pair.uninstID])

		var instSource, uninstSource []string
		var inlineCount, collectorCount int
		for _, m := range instMap {
			switch m.Kind {
			case "SOURCE_ACTION":
				instSource = append(instSource, m.ActionIdentifier)
			case "INLINE_DEBUG_ACTION":
				inlineCount++
			case "COLLECTOR_EVENT":
				collectorCount++
			}
		}
		for _, m := range uninstMap {
			if m.Kind == "SOURCE_ACTION" {
				uninstSource = append(uninstSource, m.ActionIdentifier)
			}
		}
		if !reflect.DeepEqual(instSource, uninstSource) {
			t.Fatalf("%s vs %s: source action sequence mismatch:\n  inst:   %v\n  uninst: %v", pair.instID, pair.uninstID, instSource, uninstSource)
		}
		if inlineCount == 0 || collectorCount == 0 {
			t.Fatalf("%s: expected non-zero inline debug (%d) and collector (%d) actions", pair.instID, inlineCount, collectorCount)
		}
	}

	// Verify scroll fixture anchors at top and bottom
	scrollActs := compiled["qa-scroll-editor-v2"]
	if scrollActs[0].WFWorkflowActionIdentifier != "is.workflow.actions.comment" ||
		scrollActs[0].WFWorkflowActionParameters["WFCommentActionText"] != "ANCHOR_CARD_01_TOP_HEADER" {
		t.Fatalf("scroll fixture missing top anchor comment: %+v", scrollActs[0])
	}
	foundBottom := false
	for _, a := range scrollActs {
		if a.WFWorkflowActionIdentifier == "is.workflow.actions.comment" &&
			a.WFWorkflowActionParameters["WFCommentActionText"] == "ANCHOR_CARD_28_BOTTOM_FOOTER" {
			foundBottom = true
		}
	}
	if !foundBottom {
		t.Fatalf("scroll fixture missing ANCHOR_CARD_28_BOTTOM_FOOTER")
	}
}

func TestP13_SimulatorQAWorkflowAndHarnessContract(t *testing.T) {
	wfBytes, err := os.ReadFile(filepath.Join(".github", "workflows", "cherri-ios27-shortcuts-simulator-qa.yml"))
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	wf := string(wfBytes)
	for _, req := range []string{
		"workflow_dispatch:",
		"probe_level:",
		"device_profile:",
		"sample_id:",
		"capture_full_editor:",
		"max_editor_pages:",
		"upload_sanitized_artifacts:",
		"runs-on: xcode-27",
		"retention-days: 7",
	} {
		if !strings.Contains(wf, req) {
			t.Fatalf("workflow missing required token %q", req)
		}
	}

	shBytes, err := os.ReadFile(filepath.Join("scripts", "ios27_shortcuts_simulator_qa.sh"))
	if err != nil {
		t.Fatalf("read orchestrator script: %v", err)
	}
	sh := string(shBytes)
	for _, gate := range []string{
		"RUNNER_AVAILABLE",
		"SIMULATOR_BOOTED",
		"SHORTCUTS_APP_INSTALLED",
		"SHORTCUTS_LAUNCHED",
		"EDITOR_OPENED",
		"FILE_IMPORT_ATTEMPTED",
		"COMPILER_ARTIFACT_IMPORTED",
		"SHORTCUT_EXISTS_IN_LIBRARY",
		"ACTION_EDITOR_CAPTURED",
		"SHORTCUT_RUN_ATTEMPTED",
		"SHORTCUT_RUN_COMPLETED",
		"CALLBACK_CAPTURED",
		"TRACE_CAPTURED",
		"PHYSICAL_DEVICE_TESTED",
	} {
		if !strings.Contains(sh, gate) {
			t.Fatalf("orchestrator script missing progressive gate %q", gate)
		}
	}

	uiBytes, err := os.ReadFile(filepath.Join("tools", "shortcuts-simulator-qa", "XcodeProject", "UITests", "CherriSimulatorQAUITests.swift"))
	if err != nil {
		t.Fatalf("read UI tests: %v", err)
	}
	ui := string(uiBytes)
	if !strings.Contains(ui, "shortcuts.snapshot()") || !strings.Contains(ui, "snap.dictionaryRepresentation") {
		t.Fatalf("UI test must use public XCUIElementSnapshot APIs")
	}
	if strings.Contains(ui, "debugDescription") {
		t.Fatalf("UI test must not parse debugDescription")
	}
}
