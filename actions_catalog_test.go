/*
 * Copyright (c) Cherri
 */

package main

import (
	"slices"
	"testing"
)

// TestForkActionCatalogCompleteness locks the shared machine-readable catalog
// surfaces for every fork-added/modified action so downstream consumers
// (iOS palette, preview metadata, Skill, docs) cannot silently lose them.
func TestForkActionCatalogCompleteness(t *testing.T) {
	loadTestStandardActions()

	catalog := buildActionCatalog()
	byName := make(map[string]catalogActionInfo)
	for _, entry := range catalog {
		byName[entry.Name] = entry
	}

	t.Run("runJavaScriptOnWebpage", func(t *testing.T) {
		entry, found := byName["runJavaScriptOnWebpage"]
		if !found {
			t.Fatal("missing from catalog")
		}
		if entry.ShortcutIdentifier != "is.workflow.actions.runjavascriptonwebpage" {
			t.Errorf("identifier = %q", entry.ShortcutIdentifier)
		}
		input := findCatalogParameter(t, entry, "input")
		if input.Key != "WFInput" || !input.Optional || !input.Reference {
			t.Errorf("input parameter = %+v, want WFInput optional reference", input)
		}
	})

	t.Run("saveFile", func(t *testing.T) {
		entry, found := byName["saveFile"]
		if !found {
			t.Fatal("missing from catalog")
		}
		if entry.ShortcutIdentifier != "is.workflow.actions.documentpicker.save" {
			t.Errorf("identifier = %q", entry.ShortcutIdentifier)
		}
		folder := findCatalogParameter(t, entry, "folder")
		if folder.Key != "WFFolder" || !folder.Optional || !folder.Reference {
			t.Errorf("folder parameter = %+v, want WFFolder optional reference", folder)
		}
		if !slices.Contains(entry.EmittedKeys, "WFAskWhereToSave") {
			t.Errorf("emittedKeys %v missing WFAskWhereToSave", entry.EmittedKeys)
		}
	})

	t.Run("openApp slideOver", func(t *testing.T) {
		entry, found := byName["openApp"]
		if !found {
			t.Fatal("missing from catalog")
		}
		slideOver := findCatalogParameter(t, entry, "slideOver")
		if slideOver.Key != "WFOpenInSlideOver" || slideOver.Type != string(Bool) || !slideOver.Optional {
			t.Errorf("slideOver parameter = %+v, want WFOpenInSlideOver optional bool", slideOver)
		}
		if !slices.Contains(entry.EmittedKeys, "WFSelectedApp") {
			t.Errorf("emittedKeys %v missing WFSelectedApp", entry.EmittedKeys)
		}
	})

	t.Run("run", func(t *testing.T) {
		entry, found := byName["run"]
		if !found {
			t.Fatal("missing from catalog")
		}
		if entry.ShortcutIdentifier != "is.workflow.actions.runworkflow" {
			t.Errorf("identifier = %q", entry.ShortcutIdentifier)
		}
		nameParam := findCatalogParameter(t, entry, "shortcutName")
		if nameParam.Key != "WFWorkflowName" {
			t.Errorf("shortcutName key = %q", nameParam.Key)
		}
		input := findCatalogParameter(t, entry, "input")
		if input.Key != "WFInput" || !input.Optional {
			t.Errorf("input parameter = %+v", input)
		}
		if !slices.Contains(entry.EmittedKeys, "WFWorkflow") {
			t.Errorf("emittedKeys %v missing WFWorkflow", entry.EmittedKeys)
		}
	})

	t.Run("conditional legacy flag", func(t *testing.T) {
		entry, found := byName["conditional"]
		if !found {
			t.Fatal("compiler construct missing from catalog")
		}
		if !entry.CompilerConstruct || entry.ShortcutIdentifier != "is.workflow.actions.conditional" {
			t.Errorf("entry = %+v", entry)
		}
		if !slices.Contains(entry.EmittedKeys, "WFConditionalLegacyComparisonBehavior") {
			t.Errorf("emittedKeys %v missing WFConditionalLegacyComparisonBehavior", entry.EmittedKeys)
		}
	})

	t.Run("base64 lineBreakMode", func(t *testing.T) {
		for _, name := range []string{"base64Encode", "base64Decode"} {
			entry, found := byName[name]
			if !found {
				t.Fatalf("%s missing from catalog", name)
			}
			if entry.ShortcutIdentifier != "is.workflow.actions.base64encode" {
				t.Errorf("%s identifier = %q", name, entry.ShortcutIdentifier)
			}
			lineBreakMode := findCatalogParameter(t, entry, "lineBreakMode")
			if lineBreakMode.Key != "WFBase64LineBreakMode" || lineBreakMode.Type != string(String) || !lineBreakMode.Optional {
				t.Errorf("%s lineBreakMode = %+v", name, lineBreakMode)
			}
			if lineBreakMode.Enum != "" {
				t.Errorf("%s lineBreakMode must remain free text, got enum %q", name, lineBreakMode.Enum)
			}
		}
		encode := byName["base64Encode"]
		if !slices.Contains(encode.EmittedKeys, "input") {
			t.Errorf("base64Encode emittedKeys %v missing static 'input' marker", encode.EmittedKeys)
		}
	})

	t.Run("getUpcomingEvents", func(t *testing.T) {
		entry, found := byName["getUpcomingEvents"]
		if !found {
			t.Fatal("fork-added action missing from catalog")
		}
		if entry.ShortcutIdentifier != "is.workflow.actions.getupcomingevents" {
			t.Errorf("identifier = %q", entry.ShortcutIdentifier)
		}
		if entry.OutputType != string(Variable) {
			t.Errorf("outputType = %q, want variable", entry.OutputType)
		}
		expectedKeys := map[string]string{
			"count":         "WFGetUpcomingItemCount",
			"dateSpecifier": "WFDateSpecifier",
			"specifiedDate": "WFSpecifiedDate",
		}
		for paramName, key := range expectedKeys {
			param := findCatalogParameter(t, entry, paramName)
			if param.Key != key || !param.Optional {
				t.Errorf("%s parameter = %+v, want %s optional", paramName, param, key)
			}
		}
		if len(entry.Parameters) != 3 {
			t.Errorf("parameters = %+v, want exactly 3 optional parameters", entry.Parameters)
		}
	})

	// Apple first-party App Intent wrapper batch: every wrapper must expose
	// its outer identifier, derived appIntent facet (Apple legacy placeholder
	// TeamIdentifier, no installation flag), and evidence-backed parameters
	// through the shared catalog automatically.
	t.Run("apple intent wrappers", func(t *testing.T) {
		expectations := []struct {
			name            string
			outerIdentifier string
			intent          string
			bundle          string
			params          map[string]string
			emittedKeys     []string
		}{
			{
				name:            "setSilentMode",
				outerIdentifier: "com.apple.ShortcutsActions.SetSilentModeAction",
				intent:          "SetSilentModeAction",
				bundle:          "com.apple.ShortcutsActions",
				params:          map[string]string{"state": "state"},
				emittedKeys:     []string{"UUID", "operation"},
			},
			{
				name:            "searchSpotlight",
				outerIdentifier: "com.apple.Spotlight.SearchSpotlightIntent",
				intent:          "SearchSpotlightIntent",
				bundle:          "com.apple.Spotlight",
				params:          map[string]string{"criteria": "criteria"},
				emittedKeys:     []string{"UUID"},
			},
			{
				name:            "createRemindersList",
				outerIdentifier: "com.apple.reminders.TTRCreateListAppIntent",
				intent:          "TTRCreateListAppIntent",
				bundle:          "com.apple.reminders",
				emittedKeys:     []string{"UUID"},
			},
			{
				name:            "startStopwatch",
				outerIdentifier: "com.apple.clock.StartStopwatchIntent",
				intent:          "StartStopwatchIntent",
				bundle:          "com.apple.clock",
				emittedKeys:     []string{"UUID"},
			},
			{
				name:            "stopStopwatch",
				outerIdentifier: "com.apple.clock.StopStopwatchIntent",
				intent:          "StopStopwatchIntent",
				bundle:          "com.apple.clock",
				emittedKeys:     []string{"UUID"},
			},
			{
				name:            "playAudiobook",
				outerIdentifier: "com.apple.iBooksX.PlayAudiobookIntent",
				intent:          "PlayAudiobookIntent",
				bundle:          "com.apple.iBooksX",
				params:          map[string]string{"target": "target"},
				emittedKeys:     []string{"UUID"},
			},
			{
				name:            "openBook",
				outerIdentifier: "com.apple.iBooksX.OpenBookIntent",
				intent:          "OpenBookIntent",
				bundle:          "com.apple.iBooksX",
				params:          map[string]string{"target": "target"},
				emittedKeys:     []string{"UUID"},
			},
		}

		for _, want := range expectations {
			t.Run(want.name, func(t *testing.T) {
				entry, found := byName[want.name]
				if !found {
					t.Fatal("missing from catalog")
				}
				if entry.ShortcutIdentifier != want.outerIdentifier {
					t.Errorf("identifier = %q, want %q", entry.ShortcutIdentifier, want.outerIdentifier)
				}

				if entry.AppIntent == nil {
					t.Fatalf("appIntent facet missing")
				}
				facet := entry.AppIntent
				if facet.AppIntentIdentifier != want.intent || facet.BundleIdentifier != want.bundle {
					t.Errorf("appIntent facet = %+v, want %s/%s", facet, want.bundle, want.intent)
				}
				if facet.TeamIdentifier != appleLegacyTeamIdentifier {
					t.Errorf("TeamIdentifier = %q, want Apple legacy placeholder", facet.TeamIdentifier)
				}
				if facet.RequiresAppInstallation != nil {
					t.Errorf("RequiresAppInstallation = %+v, want omitted tri-state", facet.RequiresAppInstallation)
				}

				if len(entry.Parameters) != len(want.params) {
					t.Errorf("parameters = %+v, want exactly %d", entry.Parameters, len(want.params))
				}
				for paramName, key := range want.params {
					param := findCatalogParameter(t, entry, paramName)
					if param.Key != key {
						t.Errorf("%s parameter = %+v, want plist key %q", paramName, param, key)
					}
				}

				for _, emitted := range want.emittedKeys {
					if !slices.Contains(entry.EmittedKeys, emitted) {
						t.Errorf("emittedKeys %v missing %q", entry.EmittedKeys, emitted)
					}
				}
			})
		}
	})
}

func findCatalogParameter(t *testing.T, entry catalogActionInfo, name string) catalogParameter {
	t.Helper()
	for _, parameter := range entry.Parameters {
		if parameter.Name == name {
			return parameter
		}
	}
	t.Fatalf("%s: parameter %q not found in %+v", entry.Name, name, entry.Parameters)
	return catalogParameter{}
}
