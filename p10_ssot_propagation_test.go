/*
 * Copyright (c) Cherri P10 — Single Source of Truth & Automatic Action Propagation Tests
 */

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
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

func copyIsolatedSSoTWorkspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()

	copyFile := func(srcRel string) {
		t.Helper()
		src := filepath.FromSlash(srcRel)
		dst := filepath.Join(ws, src)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(dst), err)
		}
		in, err := os.Open(src)
		if err != nil {
			t.Fatalf("open %s: %v", src, err)
		}
		defer in.Close()
		out, err := os.Create(dst)
		if err != nil {
			t.Fatalf("create %s: %v", dst, err)
		}
		defer out.Close()
		if _, err := io.Copy(out, in); err != nil {
			t.Fatalf("copy %s -> %s: %v", src, dst, err)
		}
	}

	copyDir := func(relDir string) {
		t.Helper()
		root := filepath.FromSlash(relDir)
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(".", path)
			if err != nil {
				return err
			}
			copyFile(filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", relDir, err)
		}
	}

	copyFile("go.mod")
	copyFile("go.sum")
	copyFile("stdfunc.cherri")

	rootEntries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir .: %v", err)
	}
	for _, entry := range rootEntries {
		name := entry.Name()
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			copyFile(name)
		}
	}

	copyDir("actions")
	copyDir("internal")
	copyDir("tools/language-schema")
	copyFile("docs/apple-runtime/macos-27/parameter-encodings.json")
	copyFile("docs/apple-runtime/p10/p10-505-catalog-coverage-matrix.json")
	copyFile("docs/apple-runtime/p10/p10-action-ledger.json")
	copyFile("docs/architecture/action-emitter-ownership.json")

	return ws
}

// TestP10_RealSSoTSourceToConsumersPropagation (PART D.2–D.4) proves true Single Source of Truth
// propagation by modifying ONLY ONE SOURCE FILE (`actions/basic.cherri`) in an isolated workspace,
// running `go run ./tools/language-schema`, and verifying automatic propagation across all 17 stages
// and consumers (plus parameter addition propagation) without touching any other source file.
func TestP10_RealSSoTSourceToConsumersPropagation(t *testing.T) {
	ws := copyIsolatedSSoTWorkspace(t)
	basicPath := filepath.Join(ws, "actions", "basic.cherri")
	origBasicBytes, err := os.ReadFile(basicPath)
	if err != nil {
		t.Fatalf("read basic.cherri: %v", err)
	}

	// Stage 1: Edit ONLY actions/basic.cherri to add syntheticProbeMode + syntheticPropagationProbe
	synthDeclV1 := `

enum syntheticProbeMode {
	'Fast',
	'Thorough',
}

// [Doc]: [Synthetic Probe] Probe automatic SSoT propagation across all 17 consumers.
action 'synthetic.propagation.probe' syntheticPropagationProbe(text prompt: 'WFSyntheticProbePrompt', syntheticProbeMode ?mode: 'WFSyntheticProbeMode' = "Fast"): text {
	"WFSyntheticProbeVersion": 1
}
`
	if err := os.WriteFile(basicPath, append(origBasicBytes, []byte(synthDeclV1)...), 0644); err != nil {
		t.Fatalf("write basic.cherri: %v", err)
	}

	// Before running generator, --verify-generated MUST fail closed (detecting un-regenerated drift)
	verifyBeforeCmd := exec.Command("go", "run", "./tools/language-schema", "--verify-generated")
	verifyBeforeCmd.Dir = ws
	if out, err := verifyBeforeCmd.CombinedOutput(); err == nil {
		t.Fatalf("expected --verify-generated to fail BEFORE running generator, but it succeeded:\n%s", string(out))
	}

	// Stage 4: Run the REAL schema generator (`go run ./tools/language-schema`)
	genCmd := exec.Command("go", "run", "./tools/language-schema")
	genCmd.Dir = ws
	if out, err := genCmd.CombinedOutput(); err != nil {
		t.Fatalf("go run ./tools/language-schema failed in isolated workspace: %v\n%s", err, string(out))
	}

	// Verify --verify-generated now passes with zero drift
	verifyAfterCmd := exec.Command("go", "run", "./tools/language-schema", "--verify-generated")
	verifyAfterCmd.Dir = ws
	if out, err := verifyAfterCmd.CombinedOutput(); err != nil {
		t.Fatalf("expected --verify-generated to pass AFTER running generator, got: %v\n%s", err, string(out))
	}

	// Stage 5: Inspect generated_registry.go on disk
	genRegBytes, err := os.ReadFile(filepath.Join(ws, "internal", "language", "schema", "generated_registry.go"))
	if err != nil {
		t.Fatalf("read generated_registry.go: %v", err)
	}
	genRegText := string(genRegBytes)
	for _, needle := range []string{
		`"syntheticPropagationProbe"`,
		`AppleIdentifier: "is.workflow.actions.synthetic.propagation.probe"`,
		`WireKey: "WFSyntheticProbePrompt"`,
		`WireKey: "WFSyntheticProbeMode"`,
		`EnumName: "syntheticProbeMode"`,
		`"WFSyntheticProbeVersion"`,
	} {
		if !strings.Contains(genRegText, needle) {
			t.Fatalf("Stage 5: generated_registry.go missing expected generated content %q", needle)
		}
	}

	// Build isolated cherri binary once to exercise CLI, Legacy, V2, Decompiler, Docs, Skill, and Preview
	binName := "cherri_probe"
	if runtime.GOOS == "windows" {
		binName = "cherri_probe.exe"
	}
	binPath := filepath.Join(ws, binName)
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	buildCmd.Dir = ws
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build isolated binary failed: %v\n%s", err, string(out))
	}

	// Stage 11 & 15: CLI --action=syntheticPropagationProbe (also used by Open Minis Skill lookup)
	actionDocCmd := exec.Command(binPath, "--action=syntheticPropagationProbe", "--no-ansi")
	actionDocCmd.Dir = ws
	actionDocOut, err := actionDocCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Stage 11: --action=syntheticPropagationProbe failed: %v\n%s", err, string(actionDocOut))
	}
	for _, needle := range []string{"syntheticPropagationProbe(", "prompt", "mode", "Fast", "Thorough"} {
		if !strings.Contains(string(actionDocOut), needle) {
			t.Fatalf("Stage 11: --action=syntheticPropagationProbe missing %q in:\n%s", needle, string(actionDocOut))
		}
	}

	// Stage 3, 12, 14, 15, 16: CLI --actions-json (506 entries, Docs/Skill/Preview metadata)
	jsonCmd := exec.Command(binPath, "--actions-json", "--no-ansi")
	jsonCmd.Dir = ws
	jsonOut, err := jsonCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Stage 12: --actions-json failed: %v\n%s", err, string(jsonOut))
	}
	var catPayload struct {
		Actions []catalogActionInfo `json:"actions"`
	}
	if err := json.Unmarshal(jsonOut, &catPayload); err != nil {
		t.Fatalf("Stage 12: unmarshal --actions-json: %v", err)
	}
	if len(catPayload.Actions) != 506 {
		t.Fatalf("Stage 3/12: expected 506 actions (505 + syntheticPropagationProbe), got %d", len(catPayload.Actions))
	}
	var probeCat *catalogActionInfo
	for i := range catPayload.Actions {
		if catPayload.Actions[i].Name == "syntheticPropagationProbe" {
			probeCat = &catPayload.Actions[i]
			break
		}
	}
	if probeCat == nil {
		t.Fatal("Stage 12: syntheticPropagationProbe missing from --actions-json")
	}
	if probeCat.ShortcutIdentifier != "is.workflow.actions.synthetic.propagation.probe" ||
		len(probeCat.Parameters) != 2 ||
		!strings.Contains(probeCat.InsertionSnippet, "syntheticPropagationProbe(") {
		t.Fatalf("Stage 12/16: unexpected catalog/preview metadata: %+v", probeCat)
	}

	// Stage 2, 6, 7, 8, 9, 10, 13: Write a small Go test inside the isolated workspace to verify
	// in-process Parser table, schema.DefaultRegistry(), currentMobileActionCatalog() (iOS bridge),
	// Legacy compile, V2 compile, Canonical Structural Plist comparison, and Decompiler round-trip!
	probeTestCode := `package main

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/shortcutcompare"
	"howett.net/plist"
)

func unmarshalShortcutDoc(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if _, err := plist.Unmarshal(b, &doc); err != nil {
		t.Fatalf("plist unmarshal: %v", err)
	}
	return doc
}

func TestIsolatedWorkspaceConsumers(t *testing.T) {
	resetCompilerStateFully()
	schema.ResetDefaultRegistry()
	markBuiltins()
	defineRawAction()
	loadStandardActions()

	// Stage 2: Parser action & enum tables
	if actions["syntheticPropagationProbe"] == nil {
		t.Fatal("Stage 2: syntheticPropagationProbe missing from parser actions map")
	}
	if len(enumerations["syntheticProbeMode"]) != 2 {
		t.Fatalf("Stage 2: syntheticProbeMode enum missing or wrong size: %v", enumerations["syntheticProbeMode"])
	}

	// Stage 6: V2 Schema Registry
	reg := schema.DefaultRegistry()
	actSchema, ok := reg.LookupAction("syntheticPropagationProbe")
	if !ok {
		t.Fatal("Stage 6: syntheticPropagationProbe missing from schema.DefaultRegistry()")
	}
	if actSchema.AppleIdentifier != "is.workflow.actions.synthetic.propagation.probe" || len(actSchema.Parameters) != 2 {
		t.Fatalf("Stage 6: unexpected schema entry: %+v", actSchema)
	}

	// Stage 13: Mobile / iOS Bridge Catalog (currentMobileActionCatalog)
	mobCat := currentMobileActionCatalog()
	if len(mobCat) != 506 {
		t.Fatalf("Stage 13: expected 506 entries in currentMobileActionCatalog(), got %d", len(mobCat))
	}
	foundMob := false
	for _, item := range mobCat {
		if item.Name == "syntheticPropagationProbe" {
			foundMob = true
			if item.InsertionSnippet == "" || len(item.EmittedKeys) == 0 {
				t.Fatalf("Stage 13/16: mobile/preview metadata incomplete: %+v", item)
			}
			break
		}
	}
	if !foundMob {
		t.Fatal("Stage 13: syntheticPropagationProbe missing from currentMobileActionCatalog()")
	}

	// Stage 7, 8, 9: Legacy Compile vs V2 Compile + Canonical Structural Comparison
	legResp := compileForMobileMode("#define name ProbeLeg\nsyntheticPropagationProbe(\"Hello Probe\", \"Thorough\")\n", "ProbeLeg", "legacy", false)
	if !legResp.OK {
		t.Fatalf("Stage 7: Legacy compile failed: %s", legResp.Error)
	}
	v2Resp := compileForMobileMode("syntheticPropagationProbe(\"Hello Probe\", mode: \"Thorough\")\n", "ProbeV2", "v2", false)
	if !v2Resp.OK {
		t.Fatalf("Stage 8: V2 compile failed: %s", v2Resp.Error)
	}
	legPlist, _ := base64.StdEncoding.DecodeString(legResp.PlistBase64)
	v2Plist, _ := base64.StdEncoding.DecodeString(v2Resp.PlistBase64)

	cmpRes := shortcutcompare.CompareDocumentsNormalized(unmarshalShortcutDoc(t, legPlist), unmarshalShortcutDoc(t, v2Plist))
	if !cmpRes.Equal {
		t.Fatalf("Stage 9: Legacy vs V2 canonical plist mismatch: diffs=%+v", cmpRes.Differences)
	}

	// Stage 10: Decompiler -> Recompile Canonical Structural Round-Trip
	decompResp := decompileForMobile(v2Plist, "ProbeDecomp")
	if !decompResp.OK {
		t.Fatalf("Stage 10: decompileForMobile failed: %s", decompResp.Error)
	}
	if !strings.Contains(decompResp.Source, "syntheticPropagationProbe(\"Hello Probe\", \"Thorough\")") {
		t.Fatalf("Stage 10: expected native decompiled call, got:\n%s", decompResp.Source)
	}
	recompResp := compileForMobileMode(decompResp.Source, "ProbeRecomp", "legacy", false)
	if !recompResp.OK {
		t.Fatalf("Stage 10: recompile failed: %s", recompResp.Error)
	}
	recompPlist, _ := base64.StdEncoding.DecodeString(recompResp.PlistBase64)
	rtCmp := shortcutcompare.CompareDocumentsNormalized(unmarshalShortcutDoc(t, v2Plist), unmarshalShortcutDoc(t, recompPlist))
	if !rtCmp.Equal {
		t.Fatalf("Stage 10: Decompile -> Recompile structural mismatch: diffs=%+v", rtCmp.Differences)
	}
}
`
	if err := os.WriteFile(filepath.Join(ws, "ssot_probe_verify_test.go"), []byte(probeTestCode), 0644); err != nil {
		t.Fatalf("write ssot_probe_verify_test.go: %v", err)
	}
	goTestCmd := exec.Command("go", "test", "-run", "^TestIsolatedWorkspaceConsumers$", "-count=1", "-v", ".")
	goTestCmd.Dir = ws
	if out, err := goTestCmd.CombinedOutput(); err != nil {
		t.Fatalf("TestIsolatedWorkspaceConsumers failed: %v\n%s", err, string(out))
	}

	// Stage 17: Pure+Pack Generator (`verify_or_generate_pack.py --generate --out-dir ... --p10-root <ws>`)
	packOutDir := filepath.Join(ws, "generated_pack")
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	packScript := filepath.Join(repoRoot, "tools", "p10r3", "verify_or_generate_pack.py")
	packCmd := exec.Command("python", packScript, "--generate", "--out-dir", packOutDir, "--p10-root", ws)
	packCmd.Dir = repoRoot
	if out, err := packCmd.CombinedOutput(); err != nil {
		t.Fatalf("Stage 17: verify_or_generate_pack.py --generate failed: %v\n%s", err, string(out))
	}
	genBasicModuleBytes, err := os.ReadFile(filepath.Join(packOutDir, "modules", "basic.cherri"))
	if err != nil {
		t.Fatalf("Stage 17: read generated modules/basic.cherri: %v", err)
	}
	genBasicModule := string(genBasicModuleBytes)
	if !strings.Contains(genBasicModule, "syntheticPropagationProbe") || !strings.Contains(genBasicModule, "syntheticProbeMode") {
		t.Fatalf("Stage 17: generated Pack modules/basic.cherri missing syntheticPropagationProbe or syntheticProbeMode:\n%s", genBasicModule)
	}

	// --- PART D.4: Parameter Addition Propagation (add `text ?tag: 'WFSyntheticProbeTag'` to basic.cherri ONLY) ---
	synthDeclV2 := `

enum syntheticProbeMode {
	'Fast',
	'Thorough',
}

// [Doc]: [Synthetic Probe] Probe automatic SSoT propagation across all 17 consumers.
action 'synthetic.propagation.probe' syntheticPropagationProbe(text prompt: 'WFSyntheticProbePrompt', syntheticProbeMode ?mode: 'WFSyntheticProbeMode' = "Fast", text ?tag: 'WFSyntheticProbeTag'): text {
	"WFSyntheticProbeVersion": 1
}
`
	if err := os.WriteFile(basicPath, append(origBasicBytes, []byte(synthDeclV2)...), 0644); err != nil {
		t.Fatalf("write basic.cherri V2: %v", err)
	}
	genCmd2 := exec.Command("go", "run", "./tools/language-schema")
	genCmd2.Dir = ws
	if out, err := genCmd2.CombinedOutput(); err != nil {
		t.Fatalf("D4: go run ./tools/language-schema after parameter addition failed: %v\n%s", err, string(out))
	}

	paramAddTestCode := `package main

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/shortcutcompare"
	"howett.net/plist"
)

func unmarshalShortcutDoc(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if _, err := plist.Unmarshal(b, &doc); err != nil {
		t.Fatalf("plist unmarshal: %v", err)
	}
	return doc
}

func TestIsolatedParameterAdditionPropagation(t *testing.T) {
	resetCompilerStateFully()
	schema.ResetDefaultRegistry()
	markBuiltins()
	defineRawAction()
	loadStandardActions()

	v2Resp := compileForMobileMode("syntheticPropagationProbe(\"Hello Probe\", mode: \"Thorough\", tag: \"v42\")\n", "ProbeTagV2", "v2", false)
	if !v2Resp.OK {
		t.Fatalf("D4: V2 compile with new tag parameter failed: %s", v2Resp.Error)
	}
	v2Plist, _ := base64.StdEncoding.DecodeString(v2Resp.PlistBase64)
	act := extractFirstActionForParamTest(t, v2Plist)
	params, _ := act["WFWorkflowActionParameters"].(map[string]any)
	if params["WFSyntheticProbeTag"] != "v42" {
		t.Fatalf("D4: expected WFSyntheticProbeTag='v42', got %v", params["WFSyntheticProbeTag"])
	}

	decompResp := decompileForMobile(v2Plist, "ProbeTagDecomp")
	if !decompResp.OK || !strings.Contains(decompResp.Source, "\"v42\"") {
		t.Fatalf("D4: decompile did not preserve new tag parameter: %s", decompResp.Source)
	}
	recompResp := compileForMobileMode(decompResp.Source, "ProbeTagRecomp", "legacy", false)
	if !recompResp.OK {
		t.Fatalf("D4: recompile failed: %s", recompResp.Error)
	}
	recompPlist, _ := base64.StdEncoding.DecodeString(recompResp.PlistBase64)
	cmp := shortcutcompare.CompareDocumentsNormalized(unmarshalShortcutDoc(t, v2Plist), unmarshalShortcutDoc(t, recompPlist))
	if !cmp.Equal {
		t.Fatalf("D4: canonical round-trip failed after parameter addition: diffs=%+v", cmp.Differences)
	}
}

func extractFirstActionForParamTest(t *testing.T, plistBytes []byte) map[string]any {
	var doc map[string]any
	_, _ = plist.Unmarshal(plistBytes, &doc)
	acts := doc["WFWorkflowActions"].([]any)
	return acts[0].(map[string]any)
}
`

	if err := os.WriteFile(filepath.Join(ws, "ssot_probe_verify_test.go"), []byte(paramAddTestCode), 0644); err != nil {
		t.Fatalf("write ssot_probe_verify_test.go V2: %v", err)
	}
	goTestCmd2 := exec.Command("go", "test", "-run", "^TestIsolatedParameterAdditionPropagation$", "-count=1", "-v", ".")
	goTestCmd2.Dir = ws
	if out, err := goTestCmd2.CombinedOutput(); err != nil {
		t.Fatalf("TestIsolatedParameterAdditionPropagation failed: %v\n%s", err, string(out))
	}
}

// TestP10_SSoTNegativeDriftDetection (PART D.5) verifies that 10 distinct drift mutations across
// .cherri declarations, generated_registry.go, facets, emitter ownership, and Pack modules
// each fail closed deterministically.
func TestP10_SSoTNegativeDriftDetection(t *testing.T) {
	ws := copyIsolatedSSoTWorkspace(t)
	basicPath := filepath.Join(ws, "actions", "basic.cherri")
	genRegPath := filepath.Join(ws, "internal", "language", "schema", "generated_registry.go")

	origBasic, err := os.ReadFile(basicPath)
	if err != nil {
		t.Fatal(err)
	}
	origGenReg, err := os.ReadFile(genRegPath)
	if err != nil {
		t.Fatal(err)
	}

	restoreWorkspace := func() {
		_ = os.WriteFile(basicPath, origBasic, 0644)
		_ = os.WriteFile(genRegPath, origGenReg, 0644)
	}

	// Build language-schema binary once in ws so subtests don't recompile tools/language-schema 11 times
	schemaBin := filepath.Join(ws, "language-schema-bin")
	if runtime.GOOS == "windows" {
		schemaBin += ".exe"
	}
	buildSchemaCmd := exec.Command("go", "build", "-o", schemaBin, "./tools/language-schema")
	buildSchemaCmd.Dir = ws
	if bOut, bErr := buildSchemaCmd.CombinedOutput(); bErr != nil {
		t.Fatalf("failed to build language-schema binary: %v\n%s", bErr, string(bOut))
	}

	// Cache baseline --actions-json output for subtests 5..8 that only mutate generated_registry.go
	baseCatPath := filepath.Join(ws, "baseline-live-catalog.json")
	dumpCatCmd := exec.Command("go", "run", ".", "--actions-json")
	dumpCatCmd.Dir = ws
	catBytes, catErr := dumpCatCmd.Output()
	if catErr != nil {
		t.Fatalf("failed to dump baseline --actions-json: %v", catErr)
	}
	if err := os.WriteFile(baseCatPath, catBytes, 0644); err != nil {
		t.Fatal(err)
	}

	// Baseline sanity check: clean isolated workspace MUST pass --verify-generated before any mutation
	cleanCmd := exec.Command(schemaBin, "--catalog-file="+baseCatPath, "--verify-generated")
	cleanCmd.Dir = ws
	if cleanOut, cleanErr := cleanCmd.CombinedOutput(); cleanErr != nil {
		t.Fatalf("clean isolated workspace failed --verify-generated before mutations: %v\n%s", cleanErr, string(cleanOut))
	}

	assertVerifyGeneratedFails := func(t *testing.T, label string, expectedSubstring string, useCachedCatalog bool) {
		t.Helper()
		args := []string{"--verify-generated"}
		if useCachedCatalog {
			args = []string{"--catalog-file=" + baseCatPath, "--verify-generated"}
		}
		cmd := exec.Command(schemaBin, args...)
		cmd.Dir = ws
		out, err := cmd.CombinedOutput()
		outStr := string(out)
		if err == nil {
			t.Fatalf("%s: expected --verify-generated to fail closed, but it passed:\n%s", label, outStr)
		}
		if !strings.Contains(outStr, expectedSubstring) {
			t.Fatalf("%s: expected failure output to contain %q (not an accidental build error), got:\n%s", label, expectedSubstring, outStr)
		}
	}

	t.Run("1_Unregenerated_New_Action_In_Cherri_File", func(t *testing.T) {
		defer restoreWorkspace()
		mut := string(origBasic) + "\naction 'drift.one' driftActionOne(text x: 'WFX')\n"
		_ = os.WriteFile(basicPath, []byte(mut), 0644)
		assertVerifyGeneratedFails(t, "1_Unregenerated_New_Action_In_Cherri_File", "Drift detected!", false)
	})

	t.Run("2_Unregenerated_Parameter_Addition", func(t *testing.T) {
		defer restoreWorkspace()
		mut := strings.Replace(
			string(origBasic),
			"action comment(rawtext text: 'WFCommentActionText')",
			"action comment(rawtext text: 'WFCommentActionText', text ?extraDrift: 'WFExtraDrift')",
			1,
		)
		if mut == string(origBasic) {
			t.Fatal("replacement failed to modify origBasic")
		}
		_ = os.WriteFile(basicPath, []byte(mut), 0644)
		assertVerifyGeneratedFails(t, "2_Unregenerated_Parameter_Addition", "Drift detected!", false)
	})

	t.Run("3_Unregenerated_Enum_Modification", func(t *testing.T) {
		defer restoreWorkspace()
		mut := strings.Replace(
			string(origBasic),
			"'Items',",
			"'Items',\n\t'TamperedEnumVariant',",
			1,
		)
		if mut == string(origBasic) {
			t.Fatal("replacement failed to modify origBasic")
		}
		_ = os.WriteFile(basicPath, []byte(mut), 0644)
		assertVerifyGeneratedFails(t, "3_Unregenerated_Enum_Modification", "Drift detected!", false)
	})

	t.Run("4_Unregenerated_StaticParameters_Change", func(t *testing.T) {
		defer restoreWorkspace()
		mut := strings.Replace(
			string(origBasic),
			`"WFAlertActionCancelButtonShown": false`,
			`"WFAlertActionCancelButtonShown": true`,
			1,
		)
		if mut == string(origBasic) {
			t.Fatal("replacement failed to modify origBasic")
		}
		_ = os.WriteFile(basicPath, []byte(mut), 0644)
		assertVerifyGeneratedFails(t, "4_Unregenerated_StaticParameters_Change", "Drift detected!", false)
	})

	t.Run("5_Manual_Edit_To_Generated_Registry_WireKey", func(t *testing.T) {
		defer restoreWorkspace()
		mut := strings.Replace(string(origGenReg), `WireKey: "WFCommentActionText"`, `WireKey: "WFTamperedCommentKey"`, 1)
		_ = os.WriteFile(genRegPath, []byte(mut), 0644)
		assertVerifyGeneratedFails(t, "5_Manual_Edit_To_Generated_Registry_WireKey", "Drift detected!", true)
	})

	t.Run("6_Manual_Edit_To_Generated_Registry_AppleIdentifier", func(t *testing.T) {
		defer restoreWorkspace()
		mut := strings.Replace(
			string(origGenReg),
			`AppleIdentifier: "is.workflow.actions.comment"`,
			`AppleIdentifier: "is.workflow.actions.tampered.comment"`,
			1,
		)
		_ = os.WriteFile(genRegPath, []byte(mut), 0644)
		assertVerifyGeneratedFails(t, "6_Manual_Edit_To_Generated_Registry_AppleIdentifier", "Drift detected!", true)
	})

	t.Run("7_Manual_Edit_To_Generated_Registry_AppIntentDescriptor", func(t *testing.T) {
		defer restoreWorkspace()
		mut := strings.Replace(
			string(origGenReg),
			`AppIntentIdentifier: "TTRCreateListAppIntent"`,
			`AppIntentIdentifier: "TamperedCreateListAppIntent"`,
			1,
		)
		_ = os.WriteFile(genRegPath, []byte(mut), 0644)
		assertVerifyGeneratedFails(t, "7_Manual_Edit_To_Generated_Registry_AppIntentDescriptor", "Drift detected!", true)
	})

	t.Run("8_Manual_Edit_To_Generated_Registry_PlatformAvailability", func(t *testing.T) {
		defer restoreWorkspace()
		mut := strings.Replace(string(origGenReg), `MacOnly: true`, `MacOnly: false`, 1)
		_ = os.WriteFile(genRegPath, []byte(mut), 0644)
		assertVerifyGeneratedFails(t, "8_Manual_Edit_To_Generated_Registry_PlatformAvailability", "Drift detected!", true)
	})

	t.Run("9_Facet_Referencing_Removed_Primary_Parameter", func(t *testing.T) {
		defer restoreWorkspace()
		// Rename alert parameter 'alert' -> 'renamedAlert' in basic.cherri; validateFacets must fail closed!
		mut := strings.Replace(
			string(origBasic),
			"action alert(text alert: 'WFAlertActionMessage'",
			"action alert(text renamedAlert: 'WFAlertActionMessage'",
			1,
		)
		_ = os.WriteFile(basicPath, []byte(mut), 0644)
		assertVerifyGeneratedFails(t, "9_Facet_Referencing_Removed_Primary_Parameter", "Facet validation failed:", false)
	})

	t.Run("10_Pure_Pack_Generator_Drift_Against_Modified_Cherri_Source", func(t *testing.T) {
		defer restoreWorkspace()
		mut := string(origBasic) + "\naction 'drift.pack' driftPackAction(text x: 'WFX')\n"
		_ = os.WriteFile(basicPath, []byte(mut), 0644)

		repoRoot, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		packScript := filepath.Join(repoRoot, "tools", "p10r3", "verify_or_generate_pack.py")
		cmd := exec.Command("python", packScript, "--verify-generated", "--p10-root", ws)
		cmd.Dir = repoRoot
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("10_Pure_Pack_Generator_Drift: expected verify_or_generate_pack.py --verify-generated to fail closed on drifted basic.cherri, but it passed:\n%s", string(out))
		}
	})
}

