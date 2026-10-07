/*
 * Copyright (c) Cherri Language v2.0
 * Action Parity Test Suite
 */

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/electrikmilk/cherri/internal/shortcutcompare"
	"howett.net/plist"
)

type ActionParityFixture struct {
	Name             string
	Category         string
	TargetIdentifier string
	LegacyCherri     string
	V2Cherri         string
}

func TestActionParityMatrix(t *testing.T) {
	bin := roundTripBinary(t)

	fixtures := []ActionParityFixture{
		{
			Name:             "static_variant_base64_encode",
			Category:         "static variant parameter",
			TargetIdentifier: "is.workflow.actions.base64encode",
			LegacyCherri: `#include 'actions/crypto'
const input = "hello"
const out = base64Encode(input)
`,
			V2Cherri: `let input = "hello"
let out = base64Encode(input)
`,
		},
		{
			Name:             "static_variant_base64_decode",
			Category:         "static variant parameter",
			TargetIdentifier: "is.workflow.actions.base64encode",
			LegacyCherri: `#include 'actions/crypto'
const input = "aGVsbG8="
const out = base64Decode(input)
`,
			V2Cherri: `let input = "aGVsbG8="
let out = base64Decode(input)
`,
		},
		{
			Name:             "enum_and_omitted_calendar_events",
			Category:         "enum and optional omitted parameter",
			TargetIdentifier: "is.workflow.actions.getupcomingevents",
			LegacyCherri: `#include 'actions/calendar'
const events = getUpcomingEvents(5)
`,
			V2Cherri: `let events = getUpcomingEvents(5)
`,
		},
		{
			Name:             "app_intent_alarm",
			Category:         "AppIntent descriptor",
			TargetIdentifier: "com.apple.mobiletimer-framework.MobileTimerIntents.MTToggleAlarmIntent",
			LegacyCherri: `#include 'actions/calendar'
const alarm = "TestAlarm"
const res = turnOnAlarm(alarm)
`,
			V2Cherri: `let alarm = "TestAlarm"
let res = turnOnAlarm(alarm)
`,
		},
		{
			Name:             "text_token_variable_input",
			Category:         "variable/token parameter",
			TargetIdentifier: "is.workflow.actions.showresult",
			LegacyCherri: `const val = "Hello World"
show(val)
`,
			V2Cherri: `let val = "Hello World"
show(val)
`,
		},
		{
			Name:             "custom_serializer_save_file",
			Category:         "custom parameter builder with static params",
			TargetIdentifier: "is.workflow.actions.documentpicker.save",
			LegacyCherri: `#include 'actions/documents'
const content = "evidence data"
const file = saveFile("output.txt", content, true)
`,
			V2Cherri: `let content = "evidence data"
let file = saveFile("output.txt", content, overwrite: true)
`,
		},
		{
			Name:             "nested_javascript_execution",
			Category:         "nested token / custom action",
			TargetIdentifier: "is.workflow.actions.runjavascriptonwebpage",
			LegacyCherri: `#include 'actions/web'
const res = runJavaScriptOnWebpage("document.title;")
`,
			V2Cherri: `let res = runJavaScriptOnWebpage("document.title;")
`,
		},
	}

	for _, fix := range fixtures {
		fix := fix
		t.Run(fix.Name, func(t *testing.T) {
			dir := t.TempDir()

			legacyFile := filepath.Join(dir, "legacy.cherri")
			v2File := filepath.Join(dir, "v2.cherri")

			if err := os.WriteFile(legacyFile, []byte(fix.LegacyCherri), 0644); err != nil {
				t.Fatalf("failed to write legacy fixture: %v", err)
			}
			if err := os.WriteFile(v2File, []byte(fix.V2Cherri), 0644); err != nil {
				t.Fatalf("failed to write v2 fixture: %v", err)
			}

			// Compile legacy using binary in legacy mode (with --skip-sign)
			legacyCmd := exec.Command(bin, legacyFile, "--legacy", "--skip-sign")
			if out, err := legacyCmd.CombinedOutput(); err != nil {
				t.Fatalf("legacy compile failed: %v\nOutput: %s", err, string(out))
			}

			// Compile v2 using binary in v2 mode (with --skip-sign)
			v2Cmd := exec.Command(bin, v2File, "--skip-sign")
			if out, err := v2Cmd.CombinedOutput(); err != nil {
				t.Fatalf("v2 compile failed: %v\nOutput: %s", err, string(out))
			}

			legacyPlistPath := filepath.Join(dir, "legacy_unsigned.shortcut")
			v2PlistPath := filepath.Join(dir, "v2_unsigned.shortcut")

			legacyBytes, err := os.ReadFile(legacyPlistPath)
			if err != nil {
				t.Fatalf("failed to read legacy plist: %v", err)
			}
			v2Bytes, err := os.ReadFile(v2PlistPath)
			if err != nil {
				t.Fatalf("failed to read v2 plist: %v", err)
			}

			var legacyDoc, v2Doc map[string]any
			if _, err := plist.Unmarshal(legacyBytes, &legacyDoc); err != nil {
				t.Fatalf("failed to unmarshal legacy plist: %v", err)
			}
			if _, err := plist.Unmarshal(v2Bytes, &v2Doc); err != nil {
				t.Fatalf("failed to unmarshal v2 plist: %v", err)
			}

			legacyActions, ok1 := legacyDoc["WFWorkflowActions"].([]any)
			v2Actions, ok2 := v2Doc["WFWorkflowActions"].([]any)
			if !ok1 || !ok2 {
				t.Fatalf("missing WFWorkflowActions array in plists")
			}

			var legacyTarget, v2Target map[string]any
			for _, act := range legacyActions {
				if aMap, ok := act.(map[string]any); ok {
					if aMap["WFWorkflowActionIdentifier"] == fix.TargetIdentifier {
						legacyTarget = aMap
						break
					}
				}
			}
			for _, act := range v2Actions {
				if aMap, ok := act.(map[string]any); ok {
					if aMap["WFWorkflowActionIdentifier"] == fix.TargetIdentifier {
						v2Target = aMap
						break
					}
				}
			}

			if legacyTarget == nil {
				t.Fatalf("target action %s not found in legacy workflow", fix.TargetIdentifier)
			}
			if v2Target == nil {
				t.Fatalf("target action %s not found in v2 workflow", fix.TargetIdentifier)
			}

			lParams, _ := legacyTarget["WFWorkflowActionParameters"].(map[string]any)
			vParams, _ := v2Target["WFWorkflowActionParameters"].(map[string]any)

			scope := shortcutcompare.BuildDocumentScope(legacyDoc, v2Doc)
			compareActionParameters(t, fix.Name, lParams, vParams, scope)
		})
	}
}

func compareActionParameters(t *testing.T, prefix string, legacyParams, v2Params map[string]any, scope *shortcutcompare.Scope) {
	t.Helper()

	res := shortcutcompare.CompareActionParameters(legacyParams, v2Params, scope)
	if !res.Equal {
		for _, diff := range res.Differences {
			t.Errorf("%s: parameter mismatch at %s (%s): legacy=%v, v2=%v", prefix, diff.Path, diff.Kind, diff.A, diff.B)
		}
	}
}

func semanticValueEqual(a, b any) bool {
	res := shortcutcompare.CompareActionParameters(map[string]any{"v": a}, map[string]any{"v": b}, nil)
	return res.Equal
}
