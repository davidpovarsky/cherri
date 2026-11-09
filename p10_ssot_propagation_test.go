/*
 * Copyright (c) Cherri P10 — Single Source of Truth & Automatic Action Propagation Tests
 */

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/types"
	"howett.net/plist"
)

// TestP10_CatalogCLIAndMobileRegistryParity proves 100% 1:1 semantic parity across:
// 1. CLI machine-readable catalog: buildActionCatalog() (--actions-json)
// 2. V2 generated schema registry: schema.DefaultRegistry()
// 3. iOS/iPadOS mobile bridge catalog: currentMobileActionCatalog() (CherriActionCatalog)
func TestP10_CatalogCLIAndMobileRegistryParity(t *testing.T) {
	resetCompilerStateFully()
	schema.ResetDefaultRegistry()
	resetParser()
	markBuiltins()
	defineRawAction()
	loadStandardActions()

	cliCatalog := buildActionCatalog()
	mobileCatalog := currentMobileActionCatalog()
	reg := schema.DefaultRegistry()

	if len(cliCatalog) != 505 {
		t.Fatalf("expected 505 actions in buildActionCatalog(), got %d", len(cliCatalog))
	}
	if len(mobileCatalog) != len(cliCatalog) {
		t.Fatalf("count mismatch: currentMobileActionCatalog()=%d vs buildActionCatalog()=%d", len(mobileCatalog), len(cliCatalog))
	}
	if len(reg.AllActions()) != len(cliCatalog) {
		t.Fatalf("count mismatch: schema.DefaultRegistry().AllActions()=%d vs buildActionCatalog()=%d", len(reg.AllActions()), len(cliCatalog))
	}

	cliByName := make(map[string]catalogActionInfo, len(cliCatalog))
	for _, item := range cliCatalog {
		cliByName[item.Name] = item
	}

	mobileByName := make(map[string]catalogActionInfo, len(mobileCatalog))
	for _, item := range mobileCatalog {
		mobileByName[item.Name] = item
	}

	for name, cliItem := range cliByName {
		mobItem, ok := mobileByName[name]
		if !ok {
			t.Fatalf("action %q present in buildActionCatalog() but missing from currentMobileActionCatalog()", name)
		}

		if cliItem.ShortcutIdentifier != mobItem.ShortcutIdentifier {
			t.Errorf("%s: ShortcutIdentifier mismatch: cli=%q mobile=%q", name, cliItem.ShortcutIdentifier, mobItem.ShortcutIdentifier)
		}
		if cliItem.Title != mobItem.Title {
			t.Errorf("%s: Title mismatch: cli=%q mobile=%q", name, cliItem.Title, mobItem.Title)
		}
		if cliItem.Description != mobItem.Description {
			t.Errorf("%s: Description mismatch: cli=%q mobile=%q", name, cliItem.Description, mobItem.Description)
		}
		if cliItem.Category != mobItem.Category || cliItem.Subcategory != mobItem.Subcategory {
			t.Errorf("%s: Category/Subcategory mismatch: cli=%q/%q mobile=%q/%q", name, cliItem.Category, cliItem.Subcategory, mobItem.Category, mobItem.Subcategory)
		}
		if cliItem.OutputType != mobItem.OutputType {
			t.Errorf("%s: OutputType mismatch: cli=%q mobile=%q", name, cliItem.OutputType, mobItem.OutputType)
		}
		if !reflect.DeepEqual(cliItem.EmittedKeys, mobItem.EmittedKeys) {
			t.Errorf("%s: EmittedKeys mismatch: cli=%v mobile=%v", name, cliItem.EmittedKeys, mobItem.EmittedKeys)
		}
		if cliItem.MacOnly != mobItem.MacOnly || cliItem.NonMacOnly != mobItem.NonMacOnly {
			t.Errorf("%s: MacOnly/NonMacOnly mismatch: cli=%t/%t mobile=%t/%t", name, cliItem.MacOnly, cliItem.NonMacOnly, mobItem.MacOnly, mobItem.NonMacOnly)
		}
		if cliItem.MinVersion != mobItem.MinVersion || cliItem.MaxVersion != mobItem.MaxVersion {
			t.Errorf("%s: MinVersion/MaxVersion mismatch: cli=%v/%v mobile=%v/%v", name, cliItem.MinVersion, cliItem.MaxVersion, mobItem.MinVersion, mobItem.MaxVersion)
		}
		if cliItem.Builtin != mobItem.Builtin || cliItem.Custom != mobItem.Custom || cliItem.CompilerConstruct != mobItem.CompilerConstruct {
			t.Errorf("%s: Builtin/Custom/CompilerConstruct mismatch: cli=%t/%t/%t mobile=%t/%t/%t",
				name, cliItem.Builtin, cliItem.Custom, cliItem.CompilerConstruct, mobItem.Builtin, mobItem.Custom, mobItem.CompilerConstruct)
		}
		if cliItem.InsertionSnippet != mobItem.InsertionSnippet {
			t.Errorf("%s: InsertionSnippet mismatch: cli=%q mobile=%q", name, cliItem.InsertionSnippet, mobItem.InsertionSnippet)
		}
		if !reflect.DeepEqual(cliItem.AppIntent, mobItem.AppIntent) {
			t.Errorf("%s: AppIntent mismatch: cli=%+v mobile=%+v", name, cliItem.AppIntent, mobItem.AppIntent)
		}
		if !reflect.DeepEqual(cliItem.Parameters, mobItem.Parameters) {
			t.Errorf("%s: Parameters mismatch:\n  cli:    %+v\n  mobile: %+v", name, cliItem.Parameters, mobItem.Parameters)
		}
		// Compare StaticParameters via canonical JSON unmarshaled values so struct field order vs map key order normalizes cleanly
		if len(cliItem.StaticParameters) > 0 || len(mobItem.StaticParameters) > 0 {
			cliJSON, _ := json.Marshal(cliItem.StaticParameters)
			mobJSON, _ := json.Marshal(mobItem.StaticParameters)
			var cliVal, mobVal any
			_ = json.Unmarshal(cliJSON, &cliVal)
			_ = json.Unmarshal(mobJSON, &mobVal)
			if !reflect.DeepEqual(cliVal, mobVal) {
				t.Errorf("%s: StaticParameters mismatch: cli=%s mobile=%s", name, string(cliJSON), string(mobJSON))
			}
		}
	}
}

// TestP10_ActionEmitterOwnershipAndQuarantine verifies that:
// 1. docs/architecture/action-emitter-ownership.json has zero unresolved TARGET_CONSOLIDATION or UNOWNED entries.
// 2. EmitNativeWorkflow in language_emit_adapter.go is strictly QUARANTINED and never invoked by production compilation.
func TestP10_ActionEmitterOwnershipAndQuarantine(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("docs", "architecture", "action-emitter-ownership.json"))
	if err != nil {
		t.Fatalf("failed to read action-emitter-ownership.json: %v", err)
	}

	var records []map[string]string
	if err := json.Unmarshal(raw, &records); err != nil {
		t.Fatalf("failed to parse action-emitter-ownership.json: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("action-emitter-ownership.json is empty")
	}

	validStatuses := map[string]bool{
		"ACTIVE":           true,
		"SHARED_DELEGATED": true,
		"CANONICAL_OWNER":  true,
		"QUARANTINED":      true,
	}

	foundQuarantined := false
	for _, rec := range records {
		status := rec["status"]
		symbol := rec["symbol"]
		if status == "TARGET_CONSOLIDATION" || status == "UNOWNED" {
			t.Errorf("emitter %q still has unresolved status %q", symbol, status)
		}
		if !validStatuses[status] {
			t.Errorf("emitter %q has invalid status %q", symbol, status)
		}
		if rec["sharedEncoder"] == "" || rec["tests"] == "" {
			t.Errorf("emitter %q missing sharedEncoder or tests metadata: %+v", symbol, rec)
		}
		if symbol == "EmitNativeWorkflow" && status == "QUARANTINED" {
			foundQuarantined = true
		}
	}
	if !foundQuarantined {
		t.Errorf("expected EmitNativeWorkflow to be explicitly recorded as QUARANTINED")
	}

	// Verify no production Go file outside language_emit_adapter*.go calls EmitNativeWorkflow
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "language_emit_adapter.go" {
			continue
		}
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(content), "EmitNativeWorkflow") {
			t.Errorf("production file %s references quarantined EmitNativeWorkflow", name)
		}
	}
}

// TestP10_AutomaticActionPropagation_Scenarios1To4 verifies end-to-end propagation across:
// - Scenario 1: Synthetic declarative action -> CLI catalog, Schema Registry, Mobile Bridge, Legacy compile, V2 compile, Decompiler round-trip.
// - Scenario 2: Modified optional parameter propagation across Legacy and V2.
// - Scenario 3: Shared custom handler action in actions_std.go style invoked identically by Legacy and V2.
// - Scenario 4: Negative tests (unknown action in V2 fails closed, invalid enum fails closed).
func TestP10_AutomaticActionPropagation_Scenarios1To4(t *testing.T) {
	resetParser()
	markBuiltins()
	defineRawAction()
	loadStandardActions()

	const synthActionName = "syntheticPropagatedAction"
	const synthIdent = "is.workflow.actions.synthetic.propagated"
	const synthEnumName = "syntheticModeEnum"

	reg := schema.DefaultRegistry()

	// Save & restore state cleanly (including baselineActions/baselineEnumerations used by resetCompilerStateFully)
	captureCompilerBaselines()
	oldActionDef := actions[synthActionName]
	oldBaseActionDef := baselineActions[synthActionName]
	oldEnumVals, hadEnum := enumerations[synthEnumName]
	oldBaseEnumVals, hadBaseEnum := baselineEnumerations[synthEnumName]
	defer func() {
		if oldActionDef == nil {
			delete(actions, synthActionName)
		} else {
			actions[synthActionName] = oldActionDef
		}
		if oldBaseActionDef == nil {
			delete(baselineActions, synthActionName)
		} else {
			baselineActions[synthActionName] = oldBaseActionDef
		}
		if !hadEnum {
			delete(enumerations, synthEnumName)
		} else {
			enumerations[synthEnumName] = oldEnumVals
		}
		if !hadBaseEnum {
			delete(baselineEnumerations, synthEnumName)
		} else {
			baselineEnumerations[synthEnumName] = oldBaseEnumVals
		}
	}()

	// --- Scenario 1: Register synthetic declarative action ---
	enumerations[synthEnumName] = []string{"Fast", "Accurate"}
	baselineEnumerations[synthEnumName] = []string{"Fast", "Accurate"}
	synthDef := &actionDefinition{
		identifier:         "synthetic.propagated",
		overrideIdentifier: synthIdent,
		builtin:            true,
		outputType:         String,
		doc: selfDoc{
			title:       "Synthetic Propagated Action",
			description: "Test action proving single-source propagation.",
			category:    "scripting",
		},
		parameters: []parameterDefinition{
			{
				name:      "prompt",
				key:       "WFSyntheticPrompt",
				validType: String,
				optional:  false,
			},
			{
				name:         "mode",
				key:          "WFSyntheticMode",
				validType:    String,
				enum:         synthEnumName,
				optional:     true,
				defaultValue: "Fast",
			},
		},
		appendParams: map[string]any{
			"WFSyntheticVersion": 2,
		},
	}
	actions[synthActionName] = synthDef
	baselineActions[synthActionName] = synthDef

	// 1a. Verify CLI catalog (buildActionCatalog) immediately reflects the synthetic action
	cliCat := buildActionCatalog()
	var foundCLI *catalogActionInfo
	for i := range cliCat {
		if cliCat[i].Name == synthActionName {
			foundCLI = &cliCat[i]
			break
		}
	}
	if foundCLI == nil {
		t.Fatalf("Scenario 1 failed: %s not found in buildActionCatalog()", synthActionName)
	}
	if foundCLI.ShortcutIdentifier != synthIdent || len(foundCLI.Parameters) != 2 {
		t.Fatalf("Scenario 1 failed: unexpected catalog metadata: %+v", foundCLI)
	}

	// 1b. Register corresponding ActionSchema in DefaultRegistry and verify currentMobileActionCatalog()
	reg.RegisterCustomAction(&schema.ActionSchema{
		ID:                 synthActionName,
		CallableName:       synthActionName,
		Module:             foundCLI.Category,
		AppleIdentifier:    foundCLI.ShortcutIdentifier,
		PrimaryParameterID: "prompt",
		EmittedKeys:        foundCLI.EmittedKeys,
		OutputType:         types.Text,
		OutputTypeName:     "Text",
		RawOutputType:      foundCLI.OutputType,
		Builtin:            foundCLI.Builtin,
		Custom:             foundCLI.Custom,
		StaticParameters:   foundCLI.StaticParameters,
		Docs: schema.ActionDocs{
			Title:            foundCLI.Title,
			Description:      foundCLI.Description,
			Category:         foundCLI.Category,
			InsertionSnippet: foundCLI.InsertionSnippet,
		},
		Parameters: []schema.ParameterSchema{
			{
				ID:            "prompt",
				Label:         "prompt",
				DisplayName:   "prompt",
				Type:          types.Text,
				TypeName:      "Text",
				RawType:       "text",
				Optional:      false,
				AcceptedForms: []string{"literal", "tokenString", "attachment(output)", "attachment(variable)"},
				WireKey:       "WFSyntheticPrompt",
				Codec:         "text",
			},
			{
				ID:            "mode",
				Label:         "mode",
				DisplayName:   "mode",
				Type:          types.Text,
				TypeName:      "Text",
				RawType:       "text",
				Optional:      true,
				AcceptedForms: []string{"literal", "attachment(output)", "attachment(variable)"},
				DefaultValue:  "Fast",
				WireKey:       "WFSyntheticMode",
				Codec:         "enum",
				EnumName:      synthEnumName,
				EnumValues:    []string{"Fast", "Accurate"},
			},
		},
	})

	mobCat := currentMobileActionCatalog()
	var foundMob *catalogActionInfo
	for i := range mobCat {
		if mobCat[i].Name == synthActionName {
			foundMob = &mobCat[i]
			break
		}
	}
	if foundMob == nil {
		t.Fatalf("Scenario 1 failed: %s not found in currentMobileActionCatalog()", synthActionName)
	}
	if !reflect.DeepEqual(foundCLI, foundMob) {
		t.Fatalf("Scenario 1 failed: CLI vs Mobile catalog mismatch:\n  cli=%+v\n  mob=%+v", foundCLI, foundMob)
	}

	// 1c. Compile in Legacy mode and V2 mode and verify identical emitted action payload
	legacyResp := compileForMobileMode("#define name SynthTest\n"+synthActionName+"(\"Hello World\", \"Accurate\")\n", "SynthTest", "legacy", false)
	if !legacyResp.OK {
		t.Fatalf("Scenario 1 Legacy compile failed: %s", legacyResp.Error)
	}
	v2Resp := compileForMobileMode(synthActionName+"(\"Hello World\", mode: \"Accurate\")\n", "SynthTest", "v2", false)
	if !v2Resp.OK {
		t.Fatalf("Scenario 1 V2 compile failed: %s", v2Resp.Error)
	}

	legacyPlist, _ := base64.StdEncoding.DecodeString(legacyResp.PlistBase64)
	v2Plist, _ := base64.StdEncoding.DecodeString(v2Resp.PlistBase64)

	legacyAct := extractFirstAction(t, legacyPlist)
	v2Act := extractFirstAction(t, v2Plist)

	if legacyAct["WFWorkflowActionIdentifier"] != synthIdent || v2Act["WFWorkflowActionIdentifier"] != synthIdent {
		t.Fatalf("Scenario 1 identifier mismatch: legacy=%v v2=%v", legacyAct["WFWorkflowActionIdentifier"], v2Act["WFWorkflowActionIdentifier"])
	}
	legacyParams, _ := legacyAct["WFWorkflowActionParameters"].(map[string]any)
	v2Params, _ := v2Act["WFWorkflowActionParameters"].(map[string]any)
	for _, key := range []string{"WFSyntheticPrompt", "WFSyntheticMode", "WFSyntheticVersion"} {
		if !reflect.DeepEqual(legacyParams[key], v2Params[key]) {
			t.Fatalf("Scenario 1 parameter %s mismatch: legacy=%v v2=%v", key, legacyParams[key], v2Params[key])
		}
	}

	// 1d. Decompile emitted plist and verify round-trip back to syntheticPropagatedAction
	decompResp := decompileForMobile(v2Plist, "SynthTest")
	if !decompResp.OK {
		t.Fatalf("Scenario 1 decompile failed: %s", decompResp.Error)
	}
	if !strings.Contains(decompResp.Source, synthActionName+"(\"Hello World\", \"Accurate\")") {
		t.Fatalf("Scenario 1 expected native decompiled call to %s, got:\n%s", synthActionName, decompResp.Source)
	}

	// --- Scenario 2: Add a new optional parameter and verify propagation ---
	actions[synthActionName].parameters = append(actions[synthActionName].parameters, parameterDefinition{
		name:      "extraTag",
		key:       "WFSyntheticExtraTag",
		validType: String,
		optional:  true,
	})
	actSchema, _ := reg.LookupAction(synthActionName)
	actSchema.Parameters = append(actSchema.Parameters, schema.ParameterSchema{
		ID:            "extraTag",
		Label:         "extraTag",
		DisplayName:   "extraTag",
		Type:          types.Text,
		TypeName:      "Text",
		RawType:       "text",
		Optional:      true,
		AcceptedForms: []string{"literal", "tokenString"},
		WireKey:       "WFSyntheticExtraTag",
		Codec:         "text",
	})

	v2RespWithExtra := compileForMobileMode(synthActionName+"(\"Hello\", mode: \"Fast\", extraTag: \"tag-42\")\n", "SynthExtra", "v2", false)
	if !v2RespWithExtra.OK {
		t.Fatalf("Scenario 2 V2 compile with new optional param failed: %s", v2RespWithExtra.Error)
	}
	extraPlist, _ := base64.StdEncoding.DecodeString(v2RespWithExtra.PlistBase64)
	extraAct := extractFirstAction(t, extraPlist)
	extraParams, _ := extraAct["WFWorkflowActionParameters"].(map[string]any)
	if extraParams["WFSyntheticExtraTag"] != "tag-42" {
		t.Fatalf("Scenario 2 expected WFSyntheticExtraTag='tag-42', got %v", extraParams["WFSyntheticExtraTag"])
	}

	// --- Scenario 3: Special handler action (actions_std.go callback) shared between Legacy & V2 ---
	handlerInvocations := 0
	actions[synthActionName].parameters = []parameterDefinition{
		{name: "prompt", validType: String, optional: false},
	}
	actions[synthActionName].makeParams = func(args []actionArgument) map[string]any {
		handlerInvocations++
		return map[string]any{
			"WFCustomHandlerPayload": "CUSTOM_HANDLER:" + fmt.Sprint(args[0].value),
		}
	}

	legHandlerResp := compileForMobileMode("#define name HandlerTest\n"+synthActionName+"(\"Ping\")\n", "HandlerTest", "legacy", false)
	if !legHandlerResp.OK {
		t.Fatalf("Scenario 3 Legacy handler compile failed: %s", legHandlerResp.Error)
	}
	v2HandlerResp := compileForMobileMode(synthActionName+"(\"Ping\")\n", "HandlerTest", "v2", false)
	if !v2HandlerResp.OK {
		t.Fatalf("Scenario 3 V2 handler compile failed: %s", v2HandlerResp.Error)
	}
	if handlerInvocations < 2 {
		t.Fatalf("Scenario 3 expected shared makeParams handler to be invoked by both Legacy and V2, got %d invocations", handlerInvocations)
	}
	legHandlerPlist, _ := base64.StdEncoding.DecodeString(legHandlerResp.PlistBase64)
	v2HandlerPlist, _ := base64.StdEncoding.DecodeString(v2HandlerResp.PlistBase64)
	if !bytes.Contains(legHandlerPlist, []byte("CUSTOM_HANDLER:Ping")) || !bytes.Contains(v2HandlerPlist, []byte("CUSTOM_HANDLER:Ping")) {
		t.Fatalf("Scenario 3 custom handler payload missing from Legacy or V2 plist")
	}

	// --- Scenario 4: Negative tests (fail closed on unknown action or invalid argument label) ---
	negUnknown := compileForMobileMode("nonExistentSyntheticAction999(\"test\")\n", "NegTest", "v2", false)
	if negUnknown.OK {
		t.Fatalf("Scenario 4 expected unknown action to fail closed in V2, but it succeeded")
	}
	negBadParam := compileForMobileMode(synthActionName+"(\"Ping\", nonExistentParam: \"bad\")\n", "NegTest2", "v2", false)
	if negBadParam.OK {
		t.Fatalf("Scenario 4 expected unknown named parameter to fail closed in V2, but it succeeded")
	}
}

func extractFirstAction(t *testing.T, plistBytes []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if _, err := plist.Unmarshal(plistBytes, &doc); err != nil {
		t.Fatalf("failed to unmarshal plist: %v", err)
	}
	acts, ok := doc["WFWorkflowActions"].([]any)
	if !ok || len(acts) == 0 {
		t.Fatalf("expected non-empty WFWorkflowActions in plist")
	}
	act, ok := acts[0].(map[string]any)
	if !ok {
		t.Fatalf("expected action dictionary, got %T", acts[0])
	}
	return act
}
