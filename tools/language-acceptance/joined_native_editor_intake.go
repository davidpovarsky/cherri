package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const joinedNativePrefix = "[CHERRI_JOINED_CONTRACT] "
const joinedNativeRejected = "[CHERRI_JOINED_REJECTED] "
const joinedNativeContractPath = "ios/TestFixtures/joined-editor-contract.json"
const joinedBRG25Bridge = "CherriJoinedBridgeTests/testBRG25ActualMobileRouteJoinedContract"
const joinedBRG27Bridge = "CherriJoinedBridgeTests/testBRG27ActualEditorBridgeJoinedContract"
const joinedBRG25UI = "CherriJoinedContractUITests/testBRG25ActualAppRejectedAndLiteralRoutesUI"
const joinedBRG27UI = "CherriJoinedContractUITests/testBRG27ActualEditorJoinedWorkflowUI"
const joinedNativeBridgePath = "ios/CherriTests/CherriJoinedBridgeTests.swift"
const joinedNativeUIPath = "ios/CherriUITests/CherriJoinedContractUITests.swift"

type joinedNativeProfile struct {
	Requirement, Producer, Tier, Checkpoint, Fixture, Scope string
	Assertions                                              map[string]string
}

func joinedNativeProfileFor(test string) (joinedNativeProfile, bool) {
	switch test {
	case joinedBRG25Bridge:
		return joinedNativeProfile{
			"BRG25", "ios-bridge", "ios-build", "BRG25_NATIVE_MOBILE_JOINED_VERIFIED", joinedNativeBridgePath,
			"Dedicated hosted Swift test executes actual unsigned and signed mobile entrypoints in explicit v2 and auto, preserves three exact literals and rejects both invalid fixtures with exact UTF16 diagnostics and no plist or signature. CLI, Legacy invocation counters and actual UI require separate records.",
			map[string]string{"actual_mobile_entrypoints": "true", "literal_v2_auto_parity": "true",
				"all_mobile_failure_routes_rejected": "true", "no_plist_or_signature_on_failure": "true",
				"exact_utf16_diagnostics": "true"},
		}, true
	case joinedBRG27Bridge:
		return joinedNativeProfile{"BRG27", "ios-bridge", "", "BRG27_NATIVE_EDITOR_BRIDGE_JOINED_VERIFIED", joinedNativeBridgePath,
			"Auxiliary native bridge proof only; it grants no service, integration, LSP or UI tier.", map[string]string{}}, true
	case joinedBRG25UI:
		return joinedNativeProfile{"BRG25", "ios-ui", "ios-ui", "BRG25_NATIVE_APP_ROUTES_JOINED_VERIFIED", joinedNativeUIPath,
			"Dedicated XCUITest opens and clears one real document, types three literal fixtures, uses actual Build and Sign controls, observes both exact diagnostic ranges and verifies invalid current source yields no compiled or signed artifact. Mobile and CLI route proofs remain separate.",
			map[string]string{"actual_app_build_sign_controls": "true", "literal_app_builds": "true",
				"current_invalid_build_sign_rejected": "true", "no_compiled_or_signed_artifact_on_failure": "true",
				"exact_rendered_utf16_diagnostics": "true"}}, true
	case joinedBRG27UI:
		return joinedNativeProfile{"BRG27", "ios-ui", "ios-ui", "BRG27_NATIVE_EDITOR_UI_JOINED_VERIFIED", joinedNativeUIPath,
			"Dedicated XCUITest drives current Unicode diagnostics, shared parameter and enum completion edits, rapid edits, Hover, Rename, Format and unsigned Build on an opened document; same-run dedicated native bridge proof supplies exact stale-version and request-identity controls. Actual LSP and performance tiers remain separate.",
			map[string]string{"actual_editor_joined_workflow_ui": "true", "shared_textedit_applied_ui": "true",
				"current_diagnostics_exact_utf16_ui": "true", "current_revision_only": "true", "hover_rename_format_ui": "true",
				"same_run_native_bridge_proof": "true", "comments_literals_preserved": "true"}}, true
	}
	return joinedNativeProfile{}, false
}

// These IDs enroll only the three dedicated records. The BRG27 bridge is
// companion evidence inside the UI proof, never an interchangeable LSP tier.
func joinedNativeEditorTestIDs(id string) []string {
	switch id {
	case "BRG25":
		return []string{joinedBRG25Bridge, joinedBRG25UI}
	case "BRG27":
		return []string{joinedBRG27UI}
	}
	return nil
}

func joinedNativeEditorUIAssertions(id string) map[string]string {
	test := map[string]string{"BRG25": joinedBRG25UI, "BRG27": joinedBRG27UI}[id]
	profile, ok := joinedNativeProfileFor(test)
	if !ok {
		return nil
	}
	return profile.Assertions
}

var joinedNativeSourcePaths = []string{
	".github/workflows/ios-build.yml",
	"ios/CherriApp/App/WorkspaceView.swift",
	"ios/CherriApp/Compiler/CherriCompiler.swift",
	"ios/CherriApp/Documents/CherriDocument.swift",
	"ios/CherriApp/Editor/CherriCompletionRequest.swift",
	"ios/CherriApp/Editor/CherriEditorActionRequest.swift",
	"ios/CherriApp/Editor/CherriEditorView.swift",
	joinedNativeBridgePath, joinedNativeUIPath,
	"ios/CherriUITests/EditorStartupDeadline.swift",
	joinedNativeContractPath, "ios/TestSupport/CherriJoinedContract.swift",
	"ios/project.yml", "ios/scripts/collect_ios_test_evidence.py",
	"ios/scripts/joined_editor_evidence.py", "ios_bridge.go",
	"language_cli_adapter.go", "mobile_compiler.go", "mobile_language_service.go",
}

type joinedNativeFixture struct {
	Source, SourceSha256, ExpectedSource, ExpectedSourceSha256 string
}

type joinedNativeStage struct{ ID, FixtureID, ObservedSourceField string }

type joinedNativeMethod struct {
	Requirement, Producer, Checkpoint string
	Stages                            []joinedNativeStage
}

type joinedNativeContract struct {
	Schema, EventSchema, EventPrefix, PositionEncoding               string
	GenericTestInheritanceAllowed, FormalAcceptanceGrantedByContract bool
	Fixtures                                                         map[string]joinedNativeFixture
	Methods                                                          map[string]joinedNativeMethod
}

func loadJoinedNativeContract(raw []byte) (*joinedNativeContract, error) {
	value, err := decodeNativeEncodingObject(raw)
	if err != nil || len(raw) > 128*1024 ||
		value["schema"] != "cherri-joined-editor-contract/1" ||
		value["eventSchema"] != "cherri-ios-joined-contract-event/1" ||
		value["eventPrefix"] != joinedNativePrefix ||
		value["positionEncoding"] != "zero-based-UTF16" ||
		value["genericTestInheritanceAllowed"] != false ||
		value["formalAcceptanceGrantedByContract"] != false {
		return nil, fmt.Errorf("JOINED_CONTRACT: exact source schema required")
	}
	var contract joinedNativeContract
	if err := json.Unmarshal(raw, &contract); err != nil {
		return nil, fmt.Errorf("JOINED_CONTRACT: %w", err)
	}
	if len(contract.Methods) != 4 || len(contract.Fixtures) == 0 || len(contract.Fixtures) > 64 {
		return nil, fmt.Errorf("JOINED_CONTRACT: exact method inventory required")
	}
	for _, fixture := range contract.Fixtures {
		if len(fixture.Source) > 16384 || len(fixture.ExpectedSource) > 16384 ||
			nativeEncodingDigest([]byte(fixture.Source)) != fixture.SourceSha256 ||
			nativeEncodingDigest([]byte(fixture.ExpectedSource)) != fixture.ExpectedSourceSha256 {
			return nil, fmt.Errorf("JOINED_CONTRACT: fixture bytes differ")
		}
	}
	for test, method := range contract.Methods {
		profile, ok := joinedNativeProfileFor(test)
		if !ok || profile.Requirement != method.Requirement || profile.Producer != method.Producer ||
			profile.Checkpoint != method.Checkpoint || len(method.Stages) == 0 || len(method.Stages) > 32 {
			return nil, fmt.Errorf("JOINED_CONTRACT: dedicated method identity differs")
		}
		seen := map[string]bool{}
		for _, stage := range method.Stages {
			_, exists := contract.Fixtures[stage.FixtureID]
			if !exists || !regexp.MustCompile("^[a-z0-9-]{1,96}$").MatchString(stage.ID) ||
				seen[stage.ID] || (stage.ObservedSourceField != "source" && stage.ObservedSourceField != "expectedSource") {
				return nil, fmt.Errorf("JOINED_CONTRACT: ordered stage is invalid")
			}
			seen[stage.ID] = true
		}
	}
	return &contract, nil
}

func joinedNativeExpectedEvent(contract *joinedNativeContract, contractSHA, test string, sequence int) map[string]any {
	method := contract.Methods[test]
	stage := method.Stages[sequence]
	fixture := contract.Fixtures[stage.FixtureID]
	output := fixture.SourceSha256
	if stage.ObservedSourceField == "expectedSource" {
		output = fixture.ExpectedSourceSha256
	}
	return map[string]any{"schema": contract.EventSchema, "testId": test, "requirement": method.Requirement,
		"producer": method.Producer, "stage": stage.ID, "sequence": json.Number(strconv.Itoa(sequence)),
		"fixtureId": stage.FixtureID, "inputSha256": fixture.SourceSha256,
		"observedSourceSha256": output, "contractSha256": contractSHA}
}

func parseJoinedNativeLog(raw []byte, contract *joinedNativeContract, contractSHA string) (map[string]any, error) {
	active := ""
	started, passed, checkpoint := map[string]bool{}, map[string]bool{}, map[string]bool{}
	events := map[string][]any{}
	for _, line := range strings.Split(string(raw), "\n") {
		if match := appIntegrationCaseLine.FindStringSubmatch(line); match != nil {
			parts := strings.SplitN(match[1], ".", 2)
			test := parts[1] + "/" + match[2]
			profile, known := joinedNativeProfileFor(test)
			if match[3] == "started." {
				if _, owned := joinedNativeProfileFor(active); owned {
					return nil, fmt.Errorf("JOINED_CASE: interleaved case")
				}
				active = test
				if known {
					target := "CherriUITests"
					if profile.Producer == "ios-bridge" {
						target = "CherriTests"
					}
					if parts[0] != target || started[test] {
						return nil, fmt.Errorf("JOINED_CASE: duplicated or foreign target")
					}
					started[test] = true
					events[test] = []any{}
				}
			} else {
				if known {
					if active != test || !started[test] || passed[test] || match[4] != "passed" {
						return nil, fmt.Errorf("JOINED_CASE: exact passing terminal required")
					}
					passed[test] = true
				}
				active = ""
			}
			continue
		}
		if strings.HasPrefix(line, "Test Case '") {
			if _, owned := joinedNativeProfileFor(active); owned || strings.Contains(line, "CherriJoined") {
				return nil, fmt.Errorf("JOINED_CASE: unsupported boundary")
			}
			active = ""
			continue
		}
		if strings.HasPrefix(line, joinedNativeRejected) {
			return nil, fmt.Errorf("JOINED_LOG: rejected original marker")
		}
		if strings.HasPrefix(line, joinedNativePrefix) {
			_, owned := joinedNativeProfileFor(active)
			method := contract.Methods[active]
			if !owned || checkpoint[active] || len(events[active]) >= len(method.Stages) {
				return nil, fmt.Errorf("JOINED_STAGE: event outside owned sequence")
			}
			event, err := decodeNativeEncodingObject([]byte(strings.TrimPrefix(line, joinedNativePrefix)))
			expected := joinedNativeExpectedEvent(contract, contractSHA, active, len(events[active]))
			if err != nil || !reflect.DeepEqual(event, expected) {
				return nil, fmt.Errorf("JOINED_STAGE: original order, hashes or identity differ")
			}
			events[active] = append(events[active], event)
			continue
		}
		if strings.HasPrefix(line, "[CHECKPOINT:BRG25_NATIVE_") || strings.HasPrefix(line, "[CHECKPOINT:BRG27_NATIVE_") {
			profile, owned := joinedNativeProfileFor(active)
			if !owned || checkpoint[active] || len(events[active]) != len(contract.Methods[active].Stages) ||
				line != "[CHECKPOINT:"+profile.Checkpoint+"]" {
				return nil, fmt.Errorf("JOINED_CHECKPOINT: exact complete owned marker required")
			}
			checkpoint[active] = true
		}
	}
	if _, owned := joinedNativeProfileFor(active); owned {
		return nil, fmt.Errorf("JOINED_CASE: incomplete case")
	}
	methods := map[string]any{}
	for test := range started {
		profile, _ := joinedNativeProfileFor(test)
		if !passed[test] || !checkpoint[test] || len(events[test]) != len(contract.Methods[test].Stages) {
			return nil, fmt.Errorf("JOINED_METHOD: required joined phases incomplete")
		}
		methods[test] = map[string]any{"testId": test, "producer": profile.Producer, "requirement": profile.Requirement,
			"status": "passed", "checkpoint": "[CHECKPOINT:" + profile.Checkpoint + "]", "events": events[test]}
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("JOINED_METHOD: no dedicated original method")
	}
	return methods, nil
}

func joinedNativeCurrentPins() ([]any, error) {
	paths := append([]string{}, joinedNativeSourcePaths...)
	sort.Strings(paths)
	pins := []any{}
	for _, name := range paths {
		path := resolveRepoPath(name)
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 2*1024*1024 {
			return nil, fmt.Errorf("JOINED_SOURCE: missing or invalid producer %s", name)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		hash := sha1.Sum(append([]byte("blob "+strconv.Itoa(len(raw))+"\x00"), raw...))
		mode := "100644"
		if info.Mode()&0100 != 0 {
			mode = "100755"
		}
		pins = append(pins, map[string]any{"path": name, "bytes": json.Number(strconv.Itoa(len(raw))),
			"sha256": nativeEncodingDigest(raw), "gitBlob": hex.EncodeToString(hash[:]), "gitMode": mode})
	}
	return pins, nil
}

func joinedNativeMethodInventory(methods map[string]any) ([]string, []string) {
	verified, records := []string{}, []string{}
	for test := range methods {
		verified = append(verified, test)
		profile, _ := joinedNativeProfileFor(test)
		if profile.Tier != "" && (test != joinedBRG27UI || methods[joinedBRG27Bridge] != nil) {
			records = append(records, test)
		}
	}
	sort.Strings(verified)
	sort.Strings(records)
	return verified, records
}

func joinedNativeStrings(values []string) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}

// Independently replay the retained original log. A collector assertion map,
// aggregate XCTest count, old partial probe or source-equivalent run cannot
// replace this candidate's dedicated method and companion native proof.
func validateJoinedNativeEditorRecord(id string, record EvidenceRecord, manifest *EvidenceManifest) error {
	if id != "BRG25" && id != "BRG27" {
		return nil
	}
	profile, known := joinedNativeProfileFor(record.TestID)
	if manifest == nil || !known || profile.Tier == "" || profile.Requirement != id ||
		!regexp.MustCompile("^[0-9a-f]{40}$").MatchString(record.ImplementationSHA) ||
		record.ImplementationSHA != manifest.ImplementationSHA || record.Tier != profile.Tier ||
		!reflect.DeepEqual(record.Requirements, []string{id}) || record.ExitCode != 0 || record.Result != "passed" ||
		!reflect.DeepEqual(record.Command, []string{"xcodebuild", "test", "-scheme", "Cherri"}) ||
		!reflect.DeepEqual(record.Assertions, profile.Assertions) || record.Detail != profile.Scope {
		return fmt.Errorf("JOINED_RECORD: only exact dedicated native tiers are enrolled")
	}
	fixture, err := os.ReadFile(resolveRepoPath(profile.Fixture))
	if err != nil || len(fixture) == 0 || record.FixtureSHA256 != nativeEncodingDigest(fixture) {
		return fmt.Errorf("JOINED_FIXTURE: current dedicated Swift producer differs")
	}
	if len(record.Artifacts) != 4 {
		return fmt.Errorf("JOINED_ARTIFACTS: four exact originals required")
	}
	limits := map[string]int{"xcodetest.log": 1200000, "ios-test-diagnostics.json": 4 * 1024 * 1024,
		"joined-editor-contract.json": 128 * 1024, "joined-editor-proof.json": 1024 * 1024}
	data := map[string][]byte{}
	hashes := map[string]string{}
	runID := ""
	for _, artifact := range record.Artifacts {
		name := filepath.Base(artifact.Path)
		limit, allowed := limits[name]
		info, err := os.Lstat(artifact.Path)
		if !allowed || data[name] != nil || artifact.RunID == "" || (runID != "" && artifact.RunID != runID) ||
			err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > int64(limit) {
			return fmt.Errorf("JOINED_ARTIFACTS: absent, duplicate, mixed-run or invalid original")
		}
		raw, err := os.ReadFile(artifact.Path)
		if err != nil || len(raw) > limit || nativeEncodingDigest(raw) != artifact.SHA256 {
			return fmt.Errorf("JOINED_ARTIFACT_DIGEST: original bytes differ")
		}
		data[name], hashes[name], runID = raw, artifact.SHA256, artifact.RunID
	}
	matches, attempt := 0, 0
	for _, run := range manifest.Runs {
		if run.RunID != runID {
			continue
		}
		matches++
		attempt = run.Attempt
		if run.Repository != "davidpovarsky/cherri" || run.WorkflowName != "iOS Build" ||
			run.HeadSHA != record.ImplementationSHA || run.CheckoutSHA != record.ImplementationSHA ||
			(run.Conclusion != "success" && run.Conclusion != "SUCCESS") || run.Attempt < 1 ||
			(run.Event != "push" && run.Event != "workflow_dispatch") {
			return fmt.Errorf("JOINED_RUN: exact successful candidate checkout and attempt required")
		}
	}
	if matches != 1 {
		return fmt.Errorf("JOINED_RUN: absent or duplicated original run")
	}
	currentContract, err := os.ReadFile(resolveRepoPath(joinedNativeContractPath))
	if err != nil || !bytes.Equal(currentContract, data["joined-editor-contract.json"]) {
		return fmt.Errorf("JOINED_CONTRACT: bundled contract differs from current source")
	}
	contract, err := loadJoinedNativeContract(currentContract)
	if err != nil {
		return err
	}
	methods, err := parseJoinedNativeLog(data["xcodetest.log"], contract, hashes["joined-editor-contract.json"])
	if err != nil {
		return err
	}
	if methods[record.TestID] == nil || (record.TestID == joinedBRG27UI && methods[joinedBRG27Bridge] == nil) {
		return fmt.Errorf("JOINED_COMPANION: dedicated case and same-run native companion required")
	}
	proof, err := decodeNativeEncodingObject(data["joined-editor-proof.json"])
	pins, pinErr := joinedNativeCurrentPins()
	if err != nil || pinErr != nil || len(proof) != 10 ||
		proof["schema"] != "cherri-joined-native-editor-proof/1" ||
		proof["implementationSha"] != record.ImplementationSHA || proof["runId"] != runID ||
		!nativeEncodingInteger(proof["runAttempt"], attempt) || !nativeEncodingInteger(proof["xcodebuildExitCode"], 0) ||
		proof["logSha256"] != hashes["xcodetest.log"] || proof["contractSha256"] != hashes["joined-editor-contract.json"] ||
		proof["noCrossTierInheritance"] != true || !reflect.DeepEqual(proof["sourcePins"], pins) ||
		!reflect.DeepEqual(proof["methods"], methods) {
		return fmt.Errorf("JOINED_PROOF: original source, process, attempt or ordered method proof differs")
	}
	diagnostic, err := decodeNativeEncodingObject(data["ios-test-diagnostics.json"])
	if err != nil || diagnostic["schemaVersion"] != "1.0" || diagnostic["implementationSha"] != record.ImplementationSHA ||
		diagnostic["runId"] != runID || !nativeEncodingInteger(diagnostic["xcodebuildExitCode"], 0) ||
		diagnostic["runStatus"] != "PASSED" || diagnostic["rawLogPresent"] != true || diagnostic["diagnosticOnly"] != false ||
		diagnostic["sanitizedLogSha256"] != hashes["xcodetest.log"] {
		return fmt.Errorf("JOINED_DIAGNOSTIC: actual complete Xcode process differs")
	}
	verified, records := joinedNativeMethodInventory(methods)
	summary, ok := diagnostic["joinedEditorEvidence"].(map[string]any)
	expectedSummary := map[string]any{"status": "VERIFIED_JOINED_NATIVE", "errorCodes": []any{}, "noCrossTierInheritance": true,
		"runAttempt": json.Number(strconv.Itoa(attempt)), "contractSha256": hashes["joined-editor-contract.json"],
		"proofSha256": hashes["joined-editor-proof.json"], "verifiedTests": joinedNativeStrings(verified),
		"recordTestIds": joinedNativeStrings(records)}
	if !ok || !reflect.DeepEqual(summary, expectedSummary) {
		return fmt.Errorf("JOINED_DIAGNOSTIC: dedicated scope differs")
	}
	results, ok := diagnostic["testResults"].([]any)
	seen := map[string]bool{}
	if !ok || len(results) == 0 {
		return fmt.Errorf("JOINED_DIAGNOSTIC: test outcomes absent")
	}
	for _, row := range results {
		result, valid := row.(map[string]any)
		if !valid {
			return fmt.Errorf("JOINED_DIAGNOSTIC: malformed outcome")
		}
		test, named := result["testId"].(string)
		if !named || test == "" || seen[test] || result["observedStatus"] != "passed" {
			return fmt.Errorf("JOINED_DIAGNOSTIC: duplicated, failed or incomplete outcome")
		}
		seen[test] = true
	}
	for test := range methods {
		if !seen[test] {
			return fmt.Errorf("JOINED_DIAGNOSTIC: dedicated method outcome missing")
		}
	}
	return nil
}
