package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
)

const appIntegrationTestID = "CherriCoreIntegrationTests/testRP40ActualAppEntrypointsJoinedContract"
const appIntegrationCheckpoint = "[CHECKPOINT:RP40_APP_ENTRYPOINTS_VERIFIED]"
const appIntegrationRejected = "[CHERRI_RP40_REJECTED] "
const appIntegrationFixture = "ios/CherriTests/CherriCoreIntegrationTests.swift"
const appIntegrationScope = "Hosted Cherri app XCTest executes the production document, completion, analysis, rename and unsigned v2 compile APIs on one synthetic document; actual UI gestures, document-browser opening and rendered publication remain unproved."

var appIntegrationSourcePaths = []string{
	"ios/CherriApp/Documents/CherriDocument.swift",
	"ios/CherriApp/Compiler/CherriCompiler.swift",
	"ios/CherriApp/Editor/CherriCompletionRequest.swift",
	"ios/CherriApp/Editor/CherriEditorView.swift",
	"ios/CherriApp/App/WorkspaceView.swift",
	"ios_bridge.go",
	"mobile_language_service.go",
	"mobile_compiler.go",
	"internal/language/schema/generated_registry.go",
	appIntegrationFixture,
	appIntegrationUIFixture,
	"ios/CherriUITests/EditorStartupDeadline.swift",
	"ios/CherriUITests/CherriEditorUITests.swift",
	"tools/language-acceptance/app_integration_ui_intake.go",
	"tools/language-acceptance/app_integration_ui_intake_test.go",
	"ios/scripts/collect_ios_test_evidence.py",
	"ios/scripts/test_collect_ios_test_evidence.py",
	"tools/language-acceptance/app_integration_intake.go",
	"tools/language-acceptance/app_integration_intake_test.go",
	"tests/language-v2/repair-regression-plan.json",
	".github/workflows/ios-build.yml",
}

func appIntegrationAssertions() map[string]string {
	return map[string]string{
		"actual_app_entrypoints":           "true",
		"current_completion_snapshot":      "true",
		"same_document_diagnostics":        "true",
		"semantic_rename_exact":            "true",
		"unsigned_v2_compile":              "true",
		"catalog_completion_compile_agree": "true",
		"negative_controls_executed":       "true",
	}
}

var appIntegrationCaseLine = regexp.MustCompile("^Test Case '-\\[([A-Za-z0-9_]+\\.[A-Za-z0-9_]+) (test[A-Za-z0-9_]+)\\]' (started\\.|(passed|failed) \\([0-9.]+ seconds\\)\\.)$")

// The existing collector's bounded XCTest log is the original diagnostic
// artifact. A copied assertion map or successful unrelated test cannot replace
// the new joined hosted-app case. This grants no UI evidence.
func validateAppIntegrationLog(raw []byte) error {
	return validateRP40ScenarioLog(raw, "CherriTests."+appIntegrationTestID, appIntegrationCheckpoint)
}

func validateAppIntegrationRecord(id string, record EvidenceRecord, manifest *EvidenceManifest) error {
	if id != "RP40" {
		return nil
	}
	if record.Tier == "ios-ui" && record.TestID == appIntegrationUITestID {
		return validateAppIntegrationUIRecord(record, manifest)
	}
	if manifest == nil || record.ImplementationSHA != manifest.ImplementationSHA ||
		!reflect.DeepEqual(record.Requirements, []string{"RP40"}) ||
		record.Tier != "app-integration" || record.TestID != appIntegrationTestID ||
		record.ExitCode != 0 || record.Result != "passed" ||
		!reflect.DeepEqual(record.Command, []string{"xcodebuild", "test", "-scheme", "Cherri"}) ||
		!reflect.DeepEqual(record.Assertions, appIntegrationAssertions()) ||
		record.Detail != appIntegrationScope {
		return fmt.Errorf("RP40_RECORD: requires exact joined app integration; UI remains unproved")
	}
	fixture, err := os.ReadFile(resolveRepoPath(appIntegrationFixture))
	if err != nil || len(fixture) == 0 || len(fixture) > 1024*1024 ||
		record.FixtureSHA256 != nativeEncodingDigest(fixture) {
		return fmt.Errorf("RP40_FIXTURE: current joined Swift test source is not bound")
	}
	if len(record.Artifacts) != 2 {
		return fmt.Errorf("RP40_ARTIFACTS: exact log and diagnostic originals are required")
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
			return fmt.Errorf("RP40_ARTIFACTS: unexpected original")
		}
		if _, duplicate := data[name]; duplicate || artifact.RunID == "" ||
			(runID != "" && runID != artifact.RunID) {
			return fmt.Errorf("RP40_ARTIFACTS: duplicate or mixed-run originals")
		}
		info, statErr := os.Stat(artifact.Path)
		if statErr != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > int64(limit) {
			return fmt.Errorf("RP40_ARTIFACTS: absent or oversized original")
		}
		raw, readErr := os.ReadFile(artifact.Path)
		if readErr != nil || len(raw) > limit || nativeEncodingDigest(raw) != artifact.SHA256 {
			return fmt.Errorf("RP40_ARTIFACT_DIGEST: original bytes are not bound")
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
			return fmt.Errorf("RP40_RUN: successful original iOS Build identity is required")
		}
	}
	if matched != 1 {
		return fmt.Errorf("RP40_RUN: original run missing or duplicated")
	}
	diagnostic, err := decodeNativeEncodingObject(data["ios-test-diagnostics.json"])
	if err != nil || diagnostic["schemaVersion"] != "1.0" ||
		diagnostic["implementationSha"] != record.ImplementationSHA || diagnostic["runId"] != runID ||
		!nativeEncodingInteger(diagnostic["xcodebuildExitCode"], 0) ||
		diagnostic["runStatus"] != "PASSED" || diagnostic["rawLogPresent"] != true ||
		diagnostic["diagnosticOnly"] != false ||
		diagnostic["sanitizedLogSha256"] != identities["xcodetest.log"].SHA256 {
		return fmt.Errorf("RP40_DIAGNOSTIC: actual complete Xcode process is not bound")
	}
	summary, okay := diagnostic["appIntegrationEvidence"].(map[string]any)
	if !okay || summary["status"] != "VERIFIED_APP_INTEGRATION" ||
		summary["testId"] != appIntegrationTestID || summary["scope"] != appIntegrationScope ||
		summary["fixtureSHA256"] != record.FixtureSHA256 || summary["uiTierGranted"] != false {
		return fmt.Errorf("RP40_DIAGNOSTIC: joined app scope or fixture differs")
	}
	results, okay := diagnostic["testResults"].([]any)
	seen := map[string]bool{}
	if !okay || len(results) == 0 {
		return fmt.Errorf("RP40_DIAGNOSTIC: actual test outcomes missing")
	}
	for _, value := range results {
		result, valid := value.(map[string]any)
		name, named := result["testId"].(string)
		if !valid || !named || name == "" || seen[name] || result["observedStatus"] != "passed" {
			return fmt.Errorf("RP40_DIAGNOSTIC: duplicated, failed or incomplete test outcome")
		}
		seen[name] = true
	}
	if !seen[appIntegrationTestID] {
		return fmt.Errorf("RP40_DIAGNOSTIC: joined app XCTest omitted")
	}
	return validateAppIntegrationLog(data["xcodetest.log"])
}
