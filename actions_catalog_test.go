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
