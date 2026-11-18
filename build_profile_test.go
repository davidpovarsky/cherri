package main

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func compileBuildProfileFixture(t *testing.T, source string, legacy bool, profile BuildProfile) ([]ShortcutAction, error) {
	t.Helper()
	var root map[string]any
	var err error
	if legacy {
		root, err = compileLegacySourceToRawMapInMemoryWithProfile(source, profile)
	} else {
		var data []byte
		data, err = CompileSourceToPlistWithOptions("profile-fixture.cherri", source, CompileOptions{BuildProfile: profile})
		if err == nil {
			root, err = decodeShortcutRawMap(data)
		}
	}
	if err != nil {
		return nil, err
	}
	var raw, ok = root["WFWorkflowActions"].([]any)
	if !ok {
		return nil, fmt.Errorf("compiled action array missing")
	}
	var actionsOut []ShortcutAction
	for _, item := range raw {
		var object = item.(map[string]any)
		var params, _ = object["WFWorkflowActionParameters"].(map[string]any)
		actionsOut = append(actionsOut, ShortcutAction{
			WFWorkflowActionIdentifier: object["WFWorkflowActionIdentifier"].(string),
			WFWorkflowActionParameters: params,
		})
	}
	return actionsOut, nil
}

func TestBuildProfileReleasePreservesSourceAndAssertions(t *testing.T) {
	var src = "text(\"producer\")\ndebugLog()\ndebugTraceEvent(\"after\", \"diagnostic-secret-marker\")\ndebugStartTimer(\"work\")\ntext(\"kept\")\ndebugStopTimer(\"work\")\ndebugShowLogs()\ndebugAssert(false, \"must stop\")\ntext(\"tail\")\n"
	for _, legacy := range []bool{true, false} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			var debug, err = compileBuildProfileFixture(t, src, legacy, BuildProfileDebug)
			if err != nil {
				t.Fatal(err)
			}
			var release, releaseErr = compileBuildProfileFixture(t, src, legacy, BuildProfileRelease)
			if releaseErr != nil {
				t.Fatal(releaseErr)
			}
			var got = strings.Join(actionSuffixes(release), ",")
			var want = "gettext,gettext,alert,exit,gettext"
			if !legacy {
				want = "gettext,gettext,number,conditional,alert,exit,conditional,gettext"
			}
			if got != want {
				t.Fatalf("release deleted source/validation or retained diagnostics: %s", got)
			}
			if len(debug) <= len(release) {
				t.Fatal("debug build did not contain instrumentation")
			}
			for _, action := range release {
				if strings.Contains(fmt.Sprint(action.WFWorkflowActionParameters), "diagnostic-secret-marker") {
					t.Fatal("diagnostic literal leaked into release")
				}
			}
			var assertionFound bool
			for _, action := range release {
				if action.WFWorkflowActionIdentifier == "is.workflow.actions.alert" && action.WFWorkflowActionParameters["WFAlertActionMessage"] == "must stop" {
					assertionFound = true
				}
			}
			if !assertionFound {
				t.Fatal("semantic assertion message was removed")
			}
		})
	}
}

func TestBuildProfileReleasePreservesEffectAndTypedReference(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		var declaration = "let"
		if legacy {
			declaration = "const"
		}
		var src = declaration + " result = run(\"SyntheticTarget\")\ndebugLog()\ngetVariable(result)\n"
		var actionsOut, err = compileBuildProfileFixture(t, src, legacy, BuildProfileRelease)
		if err != nil {
			t.Fatalf("legacy=%t: %v", legacy, err)
		}
		if len(actionsOut) != 2 || actionsOut[0].WFWorkflowActionIdentifier != "is.workflow.actions.runworkflow" ||
			actionsOut[1].WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" {
			t.Fatalf("source effect lost/duplicated: %v", actionSuffixes(actionsOut))
		}
		var producer = actionsOut[0].WFWorkflowActionParameters["UUID"]
		var token = debugParamMap(actionsOut[1].WFWorkflowActionParameters["WFVariable"])
		var ref = debugParamMap(token["Value"])
		if ref["Type"] != "ActionOutput" || ref["OutputUUID"] != producer {
			t.Fatalf("typed producer-consumer reference changed: %v", ref)
		}
	}
}

func TestBuildProfileReleaseRejectsConsumedDiagnosticOutput(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		var declaration = "let"
		if legacy {
			declaration = "const"
		}
		var src = "debugStartTimer(\"work\")\n" + declaration + " elapsed = debugStopTimer(\"work\")\ngetVariable(elapsed)\n"
		if _, err := compileBuildProfileFixture(t, src, legacy, BuildProfileRelease); err == nil ||
			!strings.Contains(err.Error(), "RELEASE_DIAGNOSTIC_VALUE") {
			t.Fatalf("legacy=%t: expected causal diagnostic-output rejection, got %v", legacy, err)
		}
	}
}

func TestBuildProfileInvalidAndSequentialIsolation(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		if _, err := compileBuildProfileFixture(t, "text(\"x\")", legacy, BuildProfile("dynamic")); err == nil ||
			!strings.Contains(err.Error(), "BUILD_PROFILE_INVALID") {
			t.Fatalf("invalid profile was accepted: %v", err)
		}
		for _, profile := range []BuildProfile{BuildProfileRelease, BuildProfileDebug, BuildProfileRelease, ""} {
			var actionsOut, err = compileBuildProfileFixture(t, "text(\"x\")\ndebugLog()\n", legacy, profile)
			if err != nil {
				t.Fatal(err)
			}
			var expected = 2
			if profile == BuildProfileRelease {
				expected = 1
			}
			if len(actionsOut) != expected {
				t.Fatalf("profile state leaked across compile: %s emitted %d actions", profile, len(actionsOut))
			}
		}
	}
}

func TestBuildProfileNestedDiagnosticReferenceMutation(t *testing.T) {
	var source = []any{map[string]any{
		"WFTextActionText": map[string]any{"attachmentsByRange": map[string]any{
			"{0, 1}": map[string]any{"Type": "ActionOutput", "OutputUUID": "STRIPPED-OUTPUT"},
		}},
	}}
	if err := inspectReleaseMacroReferences(source, map[string]string{"STRIPPED-OUTPUT": "debugStopTimer"}); err == nil {
		t.Fatal("nested missing producer mutation falsely passed")
	}
	source[0].(map[string]any)["WFTextActionText"] = "STRIPPED-OUTPUT"
	if err := inspectReleaseMacroReferences(source, map[string]string{"STRIPPED-OUTPUT": "debugStopTimer"}); err != nil {
		t.Fatal("ordinary literal was mistaken for an output edge")
	}
}

func TestBuildProfileReleaseRejectsInvalidOptions(t *testing.T) {
	var cases = []struct {
		name       string
		params     map[string]any
		diagnostic string
	}{
		{"debugLog", map[string]any{"WFDebugWithDates": "true"}, "literal Boolean"},
		{"debugLog", map[string]any{"WFDebugShowLogs": true, "WFDebugShowLogsAsText": true}, "alternative display modes"},
		{"debugLogPrevious", map[string]any{"WFInput": "not a previous output"}, "captures the previous native output"},
		{"debugTraceEvent", map[string]any{"WFDebugTraceStage": "unknown"}, "stage must be one of"},
		{"debugStartTimer", map[string]any{"WFDebugTimerLabel": ""}, "non-empty literal"},
		{"debugTrace", map[string]any{}, "requires an input"},
	}
	for _, tc := range cases {
		t.Run(tc.name+tc.diagnostic, func(t *testing.T) {
			resetDebugInstrumentationManifest()
			if _, err := resolveCompilerMacroCall(tc.name, tc.params, nil, "", "", BuildProfileRelease); err == nil || !strings.Contains(err.Error(), tc.diagnostic) {
				t.Fatalf("invalid diagnostic option was ignored in release: %v", err)
			}
		})
	}
}

func TestBuildProfileMobileBridgeIsolation(t *testing.T) {
	var src = "text(\"source\")\ndebugLog()\n"
	for _, mode := range []string{"legacy", "v2", "auto"} {
		for _, profile := range []BuildProfile{BuildProfileRelease, BuildProfileDebug, ""} {
			var response = compileForMobileModeWithOptions(src, "Profile Bridge", mode, false, CompileOptions{BuildProfile: profile})
			if !response.OK || response.SignedBase64 != "" {
				t.Fatalf("mode=%s profile=%s: %+v", mode, profile, response)
			}
			var data, err = base64.StdEncoding.DecodeString(response.PlistBase64)
			if err != nil {
				t.Fatal(err)
			}
			var root, decodeErr = decodeShortcutRawMap(data)
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}
			var actionsOut = root["WFWorkflowActions"].([]any)
			var expected = 2
			if profile == BuildProfileRelease {
				expected = 1
			}
			if len(actionsOut) != expected {
				t.Fatalf("mobile profile lost/isolation failed: mode=%s profile=%s actions=%d", mode, profile, len(actionsOut))
			}
		}
		var bad = compileForMobileModeWithOptions(src, "Profile Bridge", mode, false, CompileOptions{BuildProfile: "invalid"})
		if bad.OK || bad.PlistBase64 != "" || !strings.Contains(bad.Error, "BUILD_PROFILE_INVALID") {
			t.Fatalf("invalid mobile profile accepted: %+v", bad)
		}
		var oldAPI = compileForMobileMode(src, "Original Bridge", mode, false)
		if !oldAPI.OK {
			t.Fatalf("existing bridge ABI changed: %+v", oldAPI)
		}
	}
	var badMode = compileForMobileModeWithOptions(src, "Profile Bridge", "misspelled", false, CompileOptions{})
	if badMode.OK || !strings.Contains(badMode.Error, "COMPILE_MODE_INVALID") {
		t.Fatalf("invalid mode fell back to Legacy: %+v", badMode)
	}
}
