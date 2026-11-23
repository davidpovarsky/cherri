package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

const appIntegrationUITestID = "CherriRP40UITests/testRP40ActualAppDocumentWorkflowUI"
const appIntegrationUICheckpoint = "[CHECKPOINT:RP40_APP_UI_JOINED_VERIFIED]"
const appIntegrationUIRejected = "[CHERRI_RP40_UI_REJECTED] "
const appIntegrationUIStagePrefix = "[CHERRI_RP40_UI_STAGE] "
const appIntegrationUIFixture = "ios/CherriUITests/CherriRP40UITests.swift"
const appIntegrationUIScope = "Genuine XCUITest drives one opened Cherri document through a verified empty reset, contextual completion, rejected unfilled input, unsigned Build, rendered diagnostics, exact semantic rename and renamed-source Build; the separate hosted-app integration tier is not supplied by this UI record."

var appIntegrationUIStages = []string{
	"opened-synthetic-document", "contextual-completion", "unfilled-rejected",
	"source-compiled", "diagnostics-rendered", "semantic-rename", "renamed-source-compiled",
}

func appIntegrationUIAssertions() map[string]string {
	return map[string]string{
		"app_document_ui":                    "true",
		"contextual_service_completion_ui":   "true",
		"unfilled_editor_hole_rejected_ui":   "true",
		"same_document_diagnostics_rendered": "true",
		"semantic_rename_exact_ui":           "true",
		"unsigned_build_requested_ui":        "true",
		"renamed_source_build_recovered":     "true",
		"synthetic_reset_verified":           "true",
	}
}

// validateRP40ScenarioLog accepts only the two independently owned RP40 cases.
// A legitimate companion case is retained in the same full-run log; its marker
// is never allowed to replace the requested case's own exact marker/stages.
func validateRP40ScenarioLog(raw []byte, expected, checkpoint string) error {
	active := ""
	started, passed := map[string]bool{}, map[string]bool{}
	checkpoints := map[string]int{}
	uiStage := 0
	uiExpected := "CherriUITests." + appIntegrationUITestID
	appExpected := "CherriTests." + appIntegrationTestID
	for _, line := range strings.Split(string(raw), "\n") {
		if match := appIntegrationCaseLine.FindStringSubmatch(line); match != nil {
			id := match[1] + "/" + match[2]
			if match[3] == "started." {
				if active != "" || started[id] {
					return fmt.Errorf("RP40_CASE: duplicated or interleaved XCTest")
				}
				active, started[id] = id, true
			} else {
				if active != id || !started[id] || passed[id] || match[4] != "passed" {
					return fmt.Errorf("RP40_CASE: failed, orphaned or duplicate terminal")
				}
				passed[id], active = true, ""
			}
		} else if strings.HasPrefix(line, "Test Case '") {
			return fmt.Errorf("RP40_CASE: unsupported XCTest boundary")
		}
		if strings.HasPrefix(line, appIntegrationRejected) ||
			strings.HasPrefix(line, appIntegrationUIRejected) {
			return fmt.Errorf("RP40_CHECKPOINT: rejected original marker")
		}
		if strings.HasPrefix(line, appIntegrationUIStagePrefix) {
			stage := strings.TrimPrefix(line, appIntegrationUIStagePrefix)
			if active != uiExpected || uiStage >= len(appIntegrationUIStages) ||
				stage != appIntegrationUIStages[uiStage] {
				return fmt.Errorf("RP40_UI_STAGE: absent, reordered or unowned UI phase")
			}
			uiStage++
		}
		if strings.HasPrefix(line, "[CHECKPOINT:RP40") {
			if !((line == appIntegrationCheckpoint && active == appExpected) ||
				(line == appIntegrationUICheckpoint && active == uiExpected && uiStage == len(appIntegrationUIStages))) {
				return fmt.Errorf("RP40_CHECKPOINT: outside exact owned app/UI case")
			}
			checkpoints[line]++
			if checkpoints[line] > 1 {
				return fmt.Errorf("RP40_CHECKPOINT: duplicate original checkpoint")
			}
		}
	}
	if active != "" || !started[expected] || !passed[expected] {
		return fmt.Errorf("RP40_CASE: required exact XCTest did not finish once")
	}
	if checkpoints[checkpoint] != 1 {
		return fmt.Errorf("RP40_CHECKPOINT: expected exactly one original checkpoint")
	}
	return nil
}

func validateAppIntegrationUIRecord(record EvidenceRecord, manifest *EvidenceManifest) error {
	if manifest == nil || record.ImplementationSHA != manifest.ImplementationSHA ||
		!reflect.DeepEqual(record.Requirements, []string{"RP40"}) ||
		record.Tier != "ios-ui" || record.TestID != appIntegrationUITestID ||
		record.ExitCode != 0 || record.Result != "passed" ||
		!reflect.DeepEqual(record.Command, []string{"xcodebuild", "test", "-scheme", "Cherri"}) ||
		!reflect.DeepEqual(record.Assertions, appIntegrationUIAssertions()) ||
		record.Detail != appIntegrationUIScope {
		return fmt.Errorf("RP40_UI_RECORD: requires the dedicated joined UI case; hosted-app tier remains separate")
	}
	fixture, err := os.ReadFile(resolveRepoPath(appIntegrationUIFixture))
	if err != nil || len(fixture) == 0 || len(fixture) > 1024*1024 ||
		record.FixtureSHA256 != nativeEncodingDigest(fixture) {
		return fmt.Errorf("RP40_UI_FIXTURE: current joined Swift test source is not bound")
	}
	if len(record.Artifacts) != 2 {
		return fmt.Errorf("RP40_UI_ARTIFACTS: exact log and diagnostic originals are required")
	}
	data := map[string][]byte{}
	identities := map[string]EvidenceArtifact{}
	runID := ""
	for _, artifact := range record.Artifacts {
		name := filepath.Base(artifact.Path)
		limit := 1200000
		if name == "ios-test-diagnostics.json" {
			limit = 4 * 1024 * 1024
		} else if name != "xcodetest.log" {
			return fmt.Errorf("RP40_UI_ARTIFACTS: unexpected original")
		}
		if _, duplicate := data[name]; duplicate || artifact.RunID == "" ||
			(runID != "" && runID != artifact.RunID) {
			return fmt.Errorf("RP40_UI_ARTIFACTS: duplicate or mixed-run originals")
		}
		info, statErr := os.Stat(artifact.Path)
		if statErr != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > int64(limit) {
			return fmt.Errorf("RP40_UI_ARTIFACTS: absent or oversized original")
		}
		raw, readErr := os.ReadFile(artifact.Path)
		if readErr != nil || len(raw) > limit || nativeEncodingDigest(raw) != artifact.SHA256 {
			return fmt.Errorf("RP40_UI_ARTIFACT_DIGEST: original bytes are not bound")
		}
		data[name], identities[name], runID = raw, artifact, artifact.RunID
	}
	matched := 0
	for _, run := range manifest.Runs {
		if run.RunID != runID {
			continue
		}
		matched++
		if run.Repository != "davidpovarsky/cherri" || run.WorkflowName != "iOS Build" ||
			run.HeadSHA != record.ImplementationSHA || run.CheckoutSHA != record.ImplementationSHA ||
			(run.Conclusion != "success" && run.Conclusion != "SUCCESS") ||
			run.Attempt < 1 || (run.Event != "push" && run.Event != "workflow_dispatch") {
			return fmt.Errorf("RP40_UI_RUN: successful original iOS Build identity is required")
		}
	}
	if matched != 1 {
		return fmt.Errorf("RP40_UI_RUN: original run missing or duplicated")
	}
	diagnostic, err := decodeNativeEncodingObject(data["ios-test-diagnostics.json"])
	if err != nil || diagnostic["schemaVersion"] != "1.0" ||
		diagnostic["implementationSha"] != record.ImplementationSHA || diagnostic["runId"] != runID ||
		!nativeEncodingInteger(diagnostic["xcodebuildExitCode"], 0) ||
		diagnostic["runStatus"] != "PASSED" || diagnostic["rawLogPresent"] != true ||
		diagnostic["diagnosticOnly"] != false ||
		diagnostic["sanitizedLogSha256"] != identities["xcodetest.log"].SHA256 {
		return fmt.Errorf("RP40_UI_DIAGNOSTIC: actual complete Xcode process is not bound")
	}
	summary, okay := diagnostic["appIntegrationUIEvidence"].(map[string]any)
	if !okay || summary["status"] != "VERIFIED_APP_UI" ||
		summary["testId"] != appIntegrationUITestID || summary["scope"] != appIntegrationUIScope ||
		summary["fixtureSHA256"] != record.FixtureSHA256 || summary["appIntegrationTierGranted"] != false {
		return fmt.Errorf("RP40_UI_DIAGNOSTIC: joined app scope or fixture differs")
	}
	results, okay := diagnostic["testResults"].([]any)
	seen := map[string]bool{}
	if !okay || len(results) == 0 {
		return fmt.Errorf("RP40_UI_DIAGNOSTIC: actual test outcomes missing")
	}
	for _, value := range results {
		result, valid := value.(map[string]any)
		name, named := result["testId"].(string)
		if !valid || !named || name == "" || seen[name] || result["observedStatus"] != "passed" {
			return fmt.Errorf("RP40_UI_DIAGNOSTIC: duplicated, failed or incomplete test outcome")
		}
		seen[name] = true
	}
	if !seen[appIntegrationUITestID] {
		return fmt.Errorf("RP40_UI_DIAGNOSTIC: joined app XCTest omitted")
	}
	return validateRP40ScenarioLog(data["xcodetest.log"], "CherriUITests."+appIntegrationUITestID, appIntegrationUICheckpoint)
}
