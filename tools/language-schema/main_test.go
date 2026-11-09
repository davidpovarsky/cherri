package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateFacets_DetectsInvalidAction(t *testing.T) {
	cat := &LegacyCatalog{
		Actions: []LegacyAction{
			{Name: "text", Parameters: []LegacyParameter{{Name: "text"}}},
		},
	}
	oldFacets := facets
	facets = map[string]ActionFacet{
		"nonexistentAction": {PrimaryParameterID: "foo"},
	}
	defer func() { facets = oldFacets }()

	err := validateFacets(cat)
	if err == nil || !strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("expected error on unknown action in facet, got %v", err)
	}
}

func TestValidateFacets_DetectsInvalidParameter(t *testing.T) {
	cat := &LegacyCatalog{
		Actions: []LegacyAction{
			{Name: "alert", Parameters: []LegacyParameter{{Name: "title"}}}, // missing "alert" param
		},
	}
	oldFacets := facets
	facets = map[string]ActionFacet{
		"alert": {PrimaryParameterID: "alert"},
	}
	defer func() { facets = oldFacets }()

	err := validateFacets(cat)
	if err == nil || !strings.Contains(err.Error(), "unknown primary parameter") {
		t.Fatalf("expected error on missing primary parameter in facet, got %v", err)
	}
}

func TestUpstreamPropagation_AddsParameterAndPreservesAllFields(t *testing.T) {
	reqInstall := true
	cat := &LegacyCatalog{
		Actions: []LegacyAction{
			{
				Name:               "customAction",
				ShortcutIdentifier: "is.workflow.actions.custom",
				Title:              "Custom Action",
				Description:        `Action with "quoted" description`,
				InsertionSnippet:   "customAction(\"arg\")",
				OutputType:         "text",
				EmittedKeys:        []string{"UUID", "WFCustomKey"},
				Builtin:            false,
				Custom:             true,
				MacOnly:            true,
				MinVersion:         17.0,
				AppIntent: &LegacyAppIntent{
					Name:                    "CustomIntent",
					BundleIdentifier:        "com.example.app",
					AppIntentIdentifier:     "CustomIntent",
					TeamIdentifier:          "0000000000",
					RequiresAppInstallation: &reqInstall,
				},
				Parameters: []LegacyParameter{
					{Name: "param1", Key: "Param1", Type: "text", Reference: true},
					{Name: "newOptionalParam", Key: "NewOptionalParam", Type: "number", Optional: true, Literal: true, Default: `"defaultVal"`},
				},
			},
		},
	}
	code := generateGoCode(cat)
	if !strings.Contains(code, "newOptionalParam") {
		t.Errorf("expected generated schema to contain propagated newOptionalParam")
	}
	if !strings.Contains(code, "types.Number") {
		t.Errorf("expected generated schema to map Number type for propagated parameter")
	}
	if !strings.Contains(code, `RawOutputType: "text"`) {
		t.Errorf("expected generated schema to preserve RawOutputType")
	}
	if !strings.Contains(code, `EmittedKeys: []string{"UUID", "WFCustomKey"}`) {
		t.Errorf("expected generated schema to preserve EmittedKeys")
	}
	if !strings.Contains(code, "Custom: true") {
		t.Errorf("expected generated schema to preserve Custom flag")
	}
	if !strings.Contains(code, "Reference: true") || !strings.Contains(code, "Literal: true") {
		t.Errorf("expected generated schema to preserve Reference and Literal parameter flags")
	}
	if !strings.Contains(code, "RequiresAppInstallation: boolPtr(true)") {
		t.Errorf("expected generated schema to preserve RequiresAppInstallation")
	}
	// Verify no double-escaping of quotes in Description or InsertionSnippet
	if strings.Contains(code, `\\\"`) {
		t.Errorf("detected double-escaped quotes in generated code:\n%s", code)
	}
	if !strings.Contains(code, `InsertionSnippet: "customAction(\"arg\")"`) {
		t.Errorf("expected single-escaped InsertionSnippet literal, got:\n%s", code)
	}
}

func TestLanguageSchema_FailsClosedWithoutSilentBaselineFallback(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "--verify-generated")
	cmd.Env = append(os.Environ(), "CHERRI_LIVE_CATALOG_CMD=go version-nonexistent-flag")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected tools/language-schema to fail closed when live --actions-json fails, but it succeeded:\n%s", string(out))
	}
	if !strings.Contains(string(out), "fail-closed, no baseline fallback") {
		t.Fatalf("expected fail-closed diagnostic message, got:\n%s", string(out))
	}
}

func TestLanguageSchema_VerifyGeneratedDetectsDrift(t *testing.T) {
	tmpDir := t.TempDir()
	catPath := filepath.Join(tmpDir, "catalog.json")
	outPath := filepath.Join(tmpDir, "generated_registry.go")

	// Include all facet-referenced actions minimal stubs so validateFacets succeeds
	var actions []LegacyAction
	for name, f := range facets {
		var params []LegacyParameter
		seen := map[string]bool{}
		if f.PrimaryParameterID != "" {
			params = append(params, LegacyParameter{Name: f.PrimaryParameterID, Key: "K", Type: "text"})
			seen[f.PrimaryParameterID] = true
		}
		for pName := range f.ParamTypeOverrides {
			if !seen[pName] {
				params = append(params, LegacyParameter{Name: pName, Key: "K", Type: "text"})
				seen[pName] = true
			}
		}
		actions = append(actions, LegacyAction{Name: name, ShortcutIdentifier: "is.workflow.actions." + name, Parameters: params})
	}
	raw, err := json.Marshal(LegacyCatalog{Actions: actions})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catPath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outPath, []byte("// stale content\npackage schema\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "run", ".", catPath, outPath, "--verify-generated")
	out, runErr := cmd.CombinedOutput()
	if runErr == nil {
		t.Fatalf("expected --verify-generated to fail on drifted generated_registry.go, but it passed:\n%s", string(out))
	}
	if !strings.Contains(string(out), "Drift detected!") {
		t.Fatalf("expected 'Drift detected!' message, got:\n%s", string(out))
	}
}

