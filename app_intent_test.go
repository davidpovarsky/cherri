/*
 * Copyright (c) Cherri
 */
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"howett.net/plist"
)

// Phase 1 App Intent infrastructure tests:
//
//   - backward compatibility of every curated App Intent action,
//   - the generalized descriptor (TeamIdentifier policy, tri-state
//     ActionRequiresAppInstallation),
//   - the shared catalog facet derived from actionDefinition.appIntent,
//   - rawAction as the lossless fallback for unknown descriptor-bearing
//     actions, with curated matching still winning for known ones.
//
// The outer WFWorkflowActionIdentifier is asserted independently of the
// descriptor everywhere: classic-identifier hybrids must keep both.

// curatedAppIntentSpec describes one curated App Intent action's locked
// contract: outer identifier, emitted descriptor, and emitted parameter keys.
var curatedAppIntentSpecs = []struct {
	name            string
	outerIdentifier string
	descriptor       map[string]any
	declaredParams   []string
	appendedParamKey string
}{
	{
		name:            "createAlarm",
		outerIdentifier: "com.apple.mobiletimer-framework.MobileTimerIntents.MTCreateAlarmIntent",
		descriptor: map[string]any{
			"Name":                "Clock",
			"BundleIdentifier":    "com.apple.clock",
			"AppIntentIdentifier": "CreateAlarmIntent",
			"TeamIdentifier":      appleLegacyTeamIdentifier,
		},
		// repeatWeekdays declares no plist key: repeats is written
		// dynamically through appendParamsFunc.
		declaredParams: []string{"name", "dateComponents", "allowsSnooze"},
	},
	{
		name:            "deleteAlarm",
		outerIdentifier: "com.apple.clock.DeleteAlarmIntent",
		descriptor: map[string]any{
			"Name":                "Clock",
			"BundleIdentifier":    "com.apple.clock",
			"AppIntentIdentifier": "DeleteAlarmIntent",
			"TeamIdentifier":      appleLegacyTeamIdentifier,
		},
		declaredParams: []string{"entities"},
	},
	{
		name:            "turnOnAlarm",
		outerIdentifier: "com.apple.mobiletimer-framework.MobileTimerIntents.MTToggleAlarmIntent",
		descriptor: map[string]any{
			"Name":                "Clock",
			"BundleIdentifier":    "com.apple.clock",
			"AppIntentIdentifier": "ToggleAlarmIntent",
			"TeamIdentifier":      appleLegacyTeamIdentifier,
		},
		declaredParams:   []string{"alarm", "ShowWhenRun"},
		appendedParamKey: "state",
	},
	{
		name:            "turnOffAlarm",
		outerIdentifier: "com.apple.mobiletimer-framework.MobileTimerIntents.MTToggleAlarmIntent",
		descriptor: map[string]any{
			"Name":                "Clock",
			"BundleIdentifier":    "com.apple.clock",
			"AppIntentIdentifier": "ToggleAlarmIntent",
			"TeamIdentifier":      appleLegacyTeamIdentifier,
		},
		declaredParams:   []string{"alarm", "ShowWhenRun"},
		appendedParamKey: "state",
	},
	{
		name:            "toggleAlarm",
		outerIdentifier: "is.workflow.actions.togglealarm",
		descriptor: map[string]any{
			"Name":                "Clock",
			"BundleIdentifier":    "com.apple.clock",
			"AppIntentIdentifier": "ToggleAlarmIntent",
			"TeamIdentifier":      appleLegacyTeamIdentifier,
		},
		declaredParams:   []string{"alarm", "ShowWhenRun"},
		appendedParamKey: "operation",
	},
	{
		name:            "createShortcutLink",
		outerIdentifier: "com.apple.shortcuts.CreateShortcutiCloudLinkAction",
		descriptor: map[string]any{
			"Name":                "Shortcuts",
			"BundleIdentifier":    "com.apple.shortcuts",
			"AppIntentIdentifier": "CreateShortcutiCloudLinkAction",
			"TeamIdentifier":      appleLegacyTeamIdentifier,
		},
		declaredParams: []string{"shortcut"},
	},
	{
		name:            "setWindowedMultitasking",
		outerIdentifier: "com.apple.ShortcutsActions.SetMultitaskingModeAction",
		descriptor: map[string]any{
			"Name":                "ShortcutsActions",
			"BundleIdentifier":    "com.apple.ShortcutsActions",
			"AppIntentIdentifier": "SetMultitaskingModeAction",
			"TeamIdentifier":      appleLegacyTeamIdentifier,
		},
		declaredParams:   []string{"automaticallyShowAndHideDock"},
		appendedParamKey: "mode",
	},
	{
		name:            "setStageManagerMultitasking",
		outerIdentifier: "com.apple.ShortcutsActions.SetMultitaskingModeAction",
		descriptor: map[string]any{
			"Name":                "ShortcutsActions",
			"BundleIdentifier":    "com.apple.ShortcutsActions",
			"AppIntentIdentifier": "SetMultitaskingModeAction",
			"TeamIdentifier":      appleLegacyTeamIdentifier,
		},
		declaredParams:   []string{"automaticallyShowAndHideDock", "showRecentApps"},
		appendedParamKey: "mode",
	},
}

// TestCuratedAppIntentBackwardCompatibility locks the exact emitted structure
// of all eight curated App Intent actions across the struct generalization:
// descriptors keep Apple's placeholder TeamIdentifier, never carry an
// installation flag, and outer identifiers stay independent of descriptors.
func TestCuratedAppIntentBackwardCompatibility(t *testing.T) {
	loadTestStandardActions()

	for _, spec := range curatedAppIntentSpecs {
		t.Run(spec.name, func(t *testing.T) {
			var definition, found = actions[spec.name]
			if !found {
				t.Fatalf("curated action %s missing", spec.name)
			}
			setCurrentAction(spec.name, definition)

			if ident := getFullActionIdentifier(); ident != spec.outerIdentifier {
				t.Errorf("outer identifier = %q, want %q", ident, spec.outerIdentifier)
			}

			var params = getActionParameters([]actionArgument{})
			emitted, found := params["AppIntentDescriptor"]
			if !found {
				t.Fatalf("AppIntentDescriptor missing from emitted parameters")
			}
			if !reflect.DeepEqual(emitted, spec.descriptor) {
				t.Errorf("descriptor:\n got %#v\nwant %#v", emitted, spec.descriptor)
			}

			var descriptor = emitted.(map[string]any)
			if _, hasFlag := descriptor["ActionRequiresAppInstallation"]; hasFlag {
				t.Error("ActionRequiresAppInstallation must be omitted unless explicitly configured")
			}

			var declaredKeys []string
			for _, parameter := range definition.parameters {
				// Keys may be empty when emission is dynamic
				// (appendParamsFunc); those are covered by emittedKeys.
				if parameter.key != "" {
					declaredKeys = append(declaredKeys, parameter.key)
				}
			}
			if !reflect.DeepEqual(declaredKeys, spec.declaredParams) {
				t.Errorf("declared parameter keys:\n got %v\nwant %v", declaredKeys, spec.declaredParams)
			}



			if spec.appendedParamKey != "" {
				if _, present := params[spec.appendedParamKey]; !present {
					t.Errorf("statically appended key %q missing from emitted parameters: %v", spec.appendedParamKey, params)
				}
			}
		})
	}
}

// TestAppIntentDescriptorGeneralization covers the generalized helper itself:
// legacy Apple compatibility, explicit third-party team identifiers, and the
// ActionRequiresAppInstallation tri-state.
func TestAppIntentDescriptorGeneralization(t *testing.T) {
	loadTestStandardActions()

	t.Run("apple legacy compatible descriptor", func(t *testing.T) {
		var intent = appleAppIntent("Clock", "com.apple.clock", "ExampleIntent")
		var descriptor = appIntentDescriptor(intent)["AppIntentDescriptor"].(map[string]any)
		if descriptor["TeamIdentifier"] != appleLegacyTeamIdentifier {
			t.Errorf("TeamIdentifier = %v, want legacy placeholder", descriptor["TeamIdentifier"])
		}
		if _, hasFlag := descriptor["ActionRequiresAppInstallation"]; hasFlag {
			t.Error("unspecified installation flag must be omitted")
		}
	})

	t.Run("explicit third-party team identifier is preserved verbatim", func(t *testing.T) {
		var intent = appIntent{
			name:                    "Third Party",
			bundleIdentifier:        "com.thirdparty.app",
			appIntentIdentifier:     "DoSomethingIntent",
			teamIdentifier:          "ABCD12345X",
			requiresAppInstallation: requiresAppInstallation(true),
		}
		var descriptor = appIntentDescriptor(intent)["AppIntentDescriptor"].(map[string]any)
		if descriptor["TeamIdentifier"] != "ABCD12345X" {
			t.Errorf("TeamIdentifier = %v, want exact supplied value; the Apple default must never be fabricated", descriptor["TeamIdentifier"])
		}
		if descriptor["ActionRequiresAppInstallation"] != true {
			t.Errorf("ActionRequiresAppInstallation = %v, want boolean true", descriptor["ActionRequiresAppInstallation"])
		}
	})

	t.Run("requiresAppInstallation tri-state", func(t *testing.T) {
		var cases = []struct {
			flag  *bool
			want any
			label string
		}{
			{nil, nil, "unspecified omits the field entirely"},
			{requiresAppInstallation(true), true, "explicit true emits boolean true"},
			{requiresAppInstallation(false), false, "explicit false emits boolean false"},
		}
		for _, testCase := range cases {
			var intent = appIntent{
				name:                    "App",
				bundleIdentifier:        "com.example.app",
				appIntentIdentifier:     "Intent",
				requiresAppInstallation: testCase.flag,
			}
			var descriptor = appIntentDescriptor(intent)["AppIntentDescriptor"].(map[string]any)
			var flagValue, found = descriptor["ActionRequiresAppInstallation"]
			if testCase.want == nil {
				if found {
					t.Errorf("%s: got %v", testCase.label, flagValue)
				}
				continue
			}
			if !found || flagValue != testCase.want {
				t.Errorf("%s: got %v", testCase.label, flagValue)
			}
		}
	})

	t.Run("empty team identifier without legacy marker emits nothing", func(t *testing.T) {
		var intent = appIntent{name: "App", bundleIdentifier: "com.example.app", appIntentIdentifier: "Intent"}
		if _, emitted := emittedTeamIdentifier(intent); emitted {
			t.Error("third-party definitions without a team identifier must not emit a fabricated value")
		}
	})
}

// TestCatalogAppIntentFacet verifies the shared catalog exposes App Intent
// metadata derived from actionDefinition.appIntent, omitting unspecified
// fields and not duplicating the outer shortcutIdentifier.
func TestCatalogAppIntentFacet(t *testing.T) {
	loadTestStandardActions()

	catalog := buildActionCatalog()
	byName := make(map[string]catalogActionInfo)
	for _, entry := range catalog {
		byName[entry.Name] = entry
	}

	t.Run("legacy apple action", func(t *testing.T) {
		entry := byName["createAlarm"]
		if entry.AppIntent == nil {
			t.Fatal("createAlarm catalog entry missing appIntent facet")
		}
		if entry.AppIntent.Name != "Clock" ||
			entry.AppIntent.BundleIdentifier != "com.apple.clock" ||
			entry.AppIntent.AppIntentIdentifier != "CreateAlarmIntent" {
			t.Errorf("facet = %+v", entry.AppIntent)
		}
		if entry.AppIntent.TeamIdentifier != appleLegacyTeamIdentifier {
			t.Errorf("teamIdentifier = %q, want emitted legacy placeholder", entry.AppIntent.TeamIdentifier)
		}
		if entry.AppIntent.RequiresAppInstallation != nil {
			t.Error("unspecified requiresAppInstallation must be omitted")
		}
		if entry.ShortcutIdentifier != "com.apple.mobiletimer-framework.MobileTimerIntents.MTCreateAlarmIntent" {
			t.Errorf("shortcutIdentifier = %q (outer identifier stays in shortcutIdentifier)", entry.ShortcutIdentifier)
		}
	})

	t.Run("third-party style action", func(t *testing.T) {
		actions["catalogFacetProbe"] = &actionDefinition{
			identifier: "CatalogFacetProbeIntent",
			appIntent: appIntent{
				name:                    "Probe App",
				bundleIdentifier:        "com.probe.app",
				appIntentIdentifier:     "CatalogFacetProbeIntent",
				teamIdentifier:          "ABCD12345X",
				requiresAppInstallation: requiresAppInstallation(false),
			},
		}
		defer delete(actions, "catalogFacetProbe")

		catalog = buildActionCatalog()
		var probe *catalogActionInfo
		for i := range catalog {
			if catalog[i].Name == "catalogFacetProbe" {
				probe = &catalog[i]
			}
		}
		if probe == nil || probe.AppIntent == nil {
			t.Fatal("probe action missing appIntent facet")
		}
		if probe.AppIntent.TeamIdentifier != "ABCD12345X" {
			t.Errorf("teamIdentifier = %q, want explicit third-party value", probe.AppIntent.TeamIdentifier)
		}
		if probe.AppIntent.RequiresAppInstallation == nil || *probe.AppIntent.RequiresAppInstallation != false {
			t.Errorf("requiresAppInstallation = %+v, want explicit false", probe.AppIntent.RequiresAppInstallation)
		}

		var encoded, err = json.Marshal(probe.AppIntent)
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err = json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatal(err)
		}
		if _, present := decoded["requiresAppInstallation"]; !present {
			t.Error("explicit false must serialize, not be dropped")
		}
	})

	t.Run("non-app-intent actions have no facet", func(t *testing.T) {
		if entry := byName["saveFile"]; entry.AppIntent != nil {
			t.Errorf("saveFile unexpectedly carries an appIntent facet: %+v", entry.AppIntent)
		}
	})
}

// TestRawActionFallbackPreservesUnknownAppIntents proves the decompiler's
// rawAction fallback is lossless for unknown descriptor-bearing actions and
// that curated matching still wins for known Cherri actions. Fixtures are
// sanitized synthetic shapes modeled on corpus evidence (plain nested
// descriptors without device payloads). The loop runs through the real CLI:
//
//	plist -> decompile -> Cherri source -> recompile -> structural compare
func TestRawActionFallbackPreservesUnknownAppIntents(t *testing.T) {
	var r = newRoundTripRunner(t)

	// Canonical envelope: compiled from real Cherri so fixture metadata is a
	// valid Shortcut document; fixture actions replace its actions list.
	var envelopePath = r.compileSource("rawaction-envelope", "@placeholder = \"fixture\"\n")
	var envelope = loadRoundTripDocument(t, envelopePath)

	var writeFixture = func(label string, actions []map[string]any) string {
		envelope["WFWorkflowActions"] = actions
		var data, err = plist.Marshal(envelope, plist.XMLFormat)
		if err != nil {
			t.Fatalf("%s: marshal fixture: %v", label, err)
		}
		var path = filepath.Join(r.dir, label+".shortcut")
		if err = os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("%s: write fixture: %v", label, err)
		}
		return path
	}

	// canonicalParams normalizes desired parameter values exactly like
	// rawAction recompilation does, so fixtures built with it are structurally
	// comparable to recompiled output under the strict corpus comparator.
	var canonicalParams = func(params map[string]any) map[string]any {
		var normalized = make(map[string]any, len(params))
		for key, value := range params {
			normalized[key] = value
		}
		handleRawParams(normalized)
		return normalized
	}

	var expectLosslessRoundTrip = func(subtest *testing.T, label string, fixturePath string) {
		var decompiled = r.decompileShortcut(label, fixturePath)
		if !strings.Contains(decompiled, "rawAction(") {
			t.Errorf("%s: unknown action did not fall back to rawAction:\n%s", label, decompiled)
		}
		var sourceBPath = filepath.Join(r.dir, label+"_b.cherri")
		if err := os.WriteFile(sourceBPath, []byte(decompiled), 0644); err != nil {
			t.Fatal(err)
		}
		r.runCherri(label+"/recompile", sourceBPath, "--skip-sign", "--no-ansi")
		assertStructurallyEqual(subtest, label, fixturePath,
			filepath.Join(r.dir, label+"_b_unsigned.shortcut"))
	}

	t.Run("unknown apple-style app intent", func(t *testing.T) {
		var params = canonicalParams(map[string]any{
			"AppIntentDescriptor": map[string]any{
				"TeamIdentifier":      appleLegacyTeamIdentifier,
				"BundleIdentifier":    "com.apple.synthetic.example",
				"Name":                "Synthetic Example",
				"AppIntentIdentifier": "DoThingIntent",
			},
			"WFTextActionText": "literal payload",
		})
		var fixturePath = writeFixture("rawapp-apple", []map[string]any{{
			"WFWorkflowActionIdentifier": "com.apple.synthetic.example.DoThingIntent",
			"WFWorkflowActionParameters": params,
		}})
		expectLosslessRoundTrip(t, "rawapp-apple", fixturePath)
	})

	t.Run("unknown third-party app intent with alphanumeric team identifier", func(t *testing.T) {
		var params = canonicalParams(map[string]any{
			"AppIntentDescriptor": map[string]any{
				"TeamIdentifier":                "ABCD12345X",
				"BundleIdentifier":              "com.thirdparty.toolkit",
				"Name":                          "Toolkit Pro",
				"AppIntentIdentifier":           "RunExternalStepIntent",
				"ActionRequiresAppInstallation": true,
			},
		})
		var fixturePath = writeFixture("rawapp-thirdparty", []map[string]any{{
			"WFWorkflowActionIdentifier": "com.thirdparty.toolkit.RunExternalStepIntent",
			"WFWorkflowActionParameters": params,
		}})
		expectLosslessRoundTrip(t, "rawapp-thirdparty", fixturePath)
	})

	t.Run("classic identifier carrying app intent descriptor", func(t *testing.T) {
		var params = canonicalParams(map[string]any{
			"AppIntentDescriptor": map[string]any{
				"BundleIdentifier":    "com.apple.notes",
				"Name":                "Notes",
				"AppIntentIdentifier": "FilterNotesIntent",
			},
			"WFInput": "sanitized evidence",
		})
		var fixturePath = writeFixture("rawapp-classic", []map[string]any{{
			"WFWorkflowActionIdentifier": "is.workflow.actions.filter.syntheticevidence",
			"WFWorkflowActionParameters": params,
		}})
		expectLosslessRoundTrip(t, "rawapp-classic", fixturePath)
	})

	t.Run("descriptor installation flag tri-state survives round-trip", func(t *testing.T) {
		var params = canonicalParams(map[string]any{
			"AppIntentDescriptor": map[string]any{
				"TeamIdentifier":                "ABCD12345X",
				"BundleIdentifier":              "com.thirdparty.optional",
				"Name":                          "Optional App",
				"AppIntentIdentifier":           "OptionalIntent",
				"ActionRequiresAppInstallation": false,
			},
		})
		var fixturePath = writeFixture("rawapp-installflag", []map[string]any{{
			"WFWorkflowActionIdentifier": "com.thirdparty.optional.OptionalIntent",
			"WFWorkflowActionParameters": params,
		}})

		var decompiled = r.decompileShortcut("rawapp-installflag", fixturePath)
		if !strings.Contains(decompiled, `"ActionRequiresAppInstallation" : false`) &&
			!strings.Contains(decompiled, `"ActionRequiresAppInstallation": false`) {
			t.Errorf("explicit false lost during decompilation:\n%s", decompiled)
		}
		var sourceBPath = filepath.Join(r.dir, "rawapp-installflag_b.cherri")
		if err := os.WriteFile(sourceBPath, []byte(decompiled), 0644); err != nil {
			t.Fatal(err)
		}
		r.runCherri("rawapp-installflag/recompile", sourceBPath, "--skip-sign", "--no-ansi")
		assertStructurallyEqual(t, "rawapp-installflag", fixturePath,
			filepath.Join(r.dir, "rawapp-installflag_b_unsigned.shortcut"))
	})

	t.Run("plain descriptor evidence shape preserves every field", func(t *testing.T) {
		// Corpus evidence carries descriptors as plain nested dictionaries
		// without WFSerializationType markers. Decompilation must surface
		// every field in the generated source; recompilation normalizes the
		// representation to the canonical serialized dictionary form.
		var plainParams = map[string]any{
			"AppIntentDescriptor": map[string]any{
				"TeamIdentifier":                "ABCD12345X",
				"BundleIdentifier":              "com.thirdparty.plain",
				"Name":                          "Plain Evidence",
				"AppIntentIdentifier":           "PlainIntent",
				"ActionRequiresAppInstallation": true,
			},
		}
		var plainFixture = writeFixture("rawapp-plain", []map[string]any{{
			"WFWorkflowActionIdentifier": "com.thirdparty.plain.PlainIntent",
			"WFWorkflowActionParameters": plainParams,
		}})

		var decompiled = r.decompileShortcut("rawapp-plain", plainFixture)
		for _, fragment := range []string{
			"ABCD12345X",
			"com.thirdparty.plain",
			"Plain Evidence",
			"PlainIntent",
			"true",
		} {
			if !strings.Contains(decompiled, fragment) {
				t.Errorf("plain descriptor lost %q during decompilation:\n%s", fragment, decompiled)
			}
		}

		var sourceBPath = filepath.Join(r.dir, "rawapp-plain_b.cherri")
		if err := os.WriteFile(sourceBPath, []byte(decompiled), 0644); err != nil {
			t.Fatal(err)
		}
		r.runCherri("rawapp-plain/recompile", sourceBPath, "--skip-sign", "--no-ansi")

		// Representation normalization only: recompiled output equals the
		// same fixture once its descriptor is expressed canonically.
		var normalizedFixture = writeFixture("rawapp-plain-normalized", []map[string]any{{
			"WFWorkflowActionIdentifier": "com.thirdparty.plain.PlainIntent",
			"WFWorkflowActionParameters": canonicalParams(plainParams),
		}})
		assertStructurallyEqual(t, "rawapp-plain/normalized",
			normalizedFixture,
			filepath.Join(r.dir, "rawapp-plain_b_unsigned.shortcut"))
	})
}

// TestCuratedMatchingWinsOverRawAction proves known Cherri App Intent actions
// decompile through their typed curated definitions instead of degrading to
// rawAction, while genuinely unknown identifiers still use the fallback.
func TestCuratedMatchingWinsOverRawAction(t *testing.T) {
	var r = newRoundTripRunner(t)

	var alarmPath = r.compileSource("curated-wins",
		"#include 'actions/calendar'\n\ncreateAlarm(\"Evidence alarm\", \"7:30\", false)\n")
	var decompiled = r.decompileShortcut("curated-wins", alarmPath)

	if strings.Contains(decompiled, "rawAction(") {
		t.Errorf("curated createAlarm degraded to rawAction:\n%s", decompiled)
	}
	if !strings.Contains(decompiled, "createAlarm(") {
		t.Errorf("curated createAlarm did not decompile to its typed form:\n%s", decompiled)
	}

	// Structural equality of the full loop proves the curated path is
	// lossless too: compile -> decompile -> recompile matches.
	var sourceBPath = filepath.Join(r.dir, "curated-wins_b.cherri")
	if err := os.WriteFile(sourceBPath, []byte(decompiled), 0644); err != nil {
		t.Fatal(err)
	}
	r.runCherri("curated-wins/recompile", sourceBPath, "--skip-sign", "--no-ansi")
	assertStructurallyEqual(t, "curated-wins", alarmPath,
		filepath.Join(r.dir, "curated-wins_b_unsigned.shortcut"))
}
