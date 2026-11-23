package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const aiMeasurementTask = "1220432417414104"
const aiMeasurementScope = "fresh exact instances of six existing public patterns"
const aiMeasurementTrustBoundary = "independent authenticated connector read supplied by operator"
const aiMeasurementProcessID = "AI01+RP50"
const aiMeasurementTimeout = 2 * time.Minute

var aiMeasurementTaskIDs = []string{
	"generate_unicode_literal", "generate_sum", "generate_list_selection",
	"edit_literal_only", "edit_binding_reference", "edit_arithmetic_operand",
}

var aiMeasurementEnvironment = []string{
	"CHERRI_AI_EVALUATION_BUNDLE", "CHERRI_AI_EVALUATION_SOURCE_ROOT",
	"CHERRI_AI_EVALUATION_ASANA_BEFORE", "CHERRI_AI_EVALUATION_ASANA_BEFORE_SHA256",
	"CHERRI_AI_EVALUATION_ASANA_BEFORE_OBSERVED_UTC",
	"CHERRI_AI_EVALUATION_ASANA_AFTER", "CHERRI_AI_EVALUATION_ASANA_AFTER_SHA256",
	"CHERRI_AI_EVALUATION_ASANA_AFTER_OBSERVED_UTC",
	"CHERRI_AI_EVALUATION_ANCHOR_STORY_ID", "CHERRI_AI_EVALUATION_ANCHOR_TEXT_SHA256",
}

var aiMeasurementSources = func() []string {
	var names = []string{
		"docs/agent-technical-reference.md", "docs/language-v2/language-guide.md",
		"go.mod", "go.sum",
		"tests/language-v2/acceptance-contract.json",
		"tests/language-v2/repair-regression-plan.json",
		"tests/language-v2/backend-recovery-gates.json",
		"tools/language-acceptance/assertion_scope.go",
		"tools/language-acceptance/ai_evaluation_execution.go",
		"tools/language-acceptance/ai_evaluation_execution_test.go",
		"tools/language-acceptance/ai_measurement_execution.go",
		"tools/language-acceptance/ai_measurement_execution_test.go",
		"tools/language-acceptance/documentation_intake.go",
		"tools/language-acceptance/editor_contract_execution.go",
		"tools/language-acceptance/editor_performance_execution.go",
		"tools/language-acceptance/editor_process_other.go",
		"tools/language-acceptance/editor_process_windows.go",
		"tools/language-acceptance/editor_source_binding.go",
		"tools/language-acceptance/evidence_map.go",
		"tools/language-acceptance/execution_audit.go",
		"tools/language-acceptance/local_gate_source_binding.go",
		"tools/language-acceptance/loop_execution.go",
		"tools/language-acceptance/main.go",
		"tools/language-acceptance/manifest_generator.go",
		"tools/language-acceptance/native_encoding_intake.go",
		"tools/language-acceptance/parser_execution.go",
		"tools/language-acceptance/python_interpreter.go",
		"tools/language-acceptance/state_acceptance_intake.go",
		"tools/language-acceptance/strict_go_events.go",
		"tools/language-acceptance/supplemental_execution.go",
		"tools/language-acceptance/verification_contract_test.go",
		"tools/language-ai-eval/README.md",
		"tools/language-ai-eval/anthropic-adapter.md",
		"tools/language-ai-eval/anthropic_adapter.py",
		"tools/language-ai-eval/evaluation_bundle.py",
		"tools/language-ai-eval/evaluation_intake.py",
		"tools/language-ai-eval/fresh-instance-protocol.md",
		"tools/language-ai-eval/fresh-instance-protocol-v2.md",
		"tools/language-ai-eval/graph_validator.py",
		"tools/language-ai-eval/runner.py",
		"tools/language-ai-eval/tasks.json",
		"tools/language-ai-eval/test_anthropic_adapter.py",
		"tools/language-ai-eval/test_evaluation_bundle.py",
		"tools/language-ai-eval/test_evaluation_intake.py",
		"tools/language-ai-eval/test_graph_validator.py",
		"tools/language-ai-eval/test_process_group_cleanup.py",
		"tools/language-ai-eval/test_runner.py",
		"tools/language-ai-eval/test_sealed_runner.py",
	}
	sort.Strings(names)
	return names
}()

type aiMeasurementTrust struct {
	Path        string
	SHA256      string
	ObservedUTC string
}
type aiMeasurementConfig struct {
	Root, Bundle, StoryID, TextSHA256 string
	Before, After                     aiMeasurementTrust
}
type aiMeasurementAnchor struct {
	StoryID    string `json:"story_id"`
	CreatedUTC string `json:"created_at"`
	TaskID     string `json:"task_id"`
	TextSHA256 string `json:"text_sha256"`
}
type aiMeasurementInterval struct {
	StartedUTC  string `json:"started_utc"`
	FinishedUTC string `json:"finished_utc"`
}
type aiMeasurementCompiler struct {
	SourceSHA string `json:"source_sha"`
	BinarySHA string `json:"binary_sha256"`
}
type aiMeasurementUsage struct {
	Input  *int `json:"input_tokens"`
	Output *int `json:"output_tokens"`
}
type aiMeasurementCheck struct {
	Name   string `json:"name"`
	Passed *bool  `json:"passed"`
}
type aiMeasurementOutcome struct {
	TaskID                  string               `json:"task_id"`
	Arm                     string               `json:"arm"`
	RequestID               string               `json:"request_id"`
	Status                  string               `json:"status"`
	ProviderReceiptSHA256   string               `json:"provider_receipt_sha256"`
	ValidatorReceiptSHA256  string               `json:"validator_receipt_sha256,omitempty"`
	ProviderRequestIDSHA256 string               `json:"provider_request_id_sha256,omitempty"`
	SourceSHA256            string               `json:"source_sha256,omitempty"`
	ProviderStatus          string               `json:"provider_status,omitempty"`
	Usage                   *aiMeasurementUsage  `json:"usage,omitempty"`
	Checks                  []aiMeasurementCheck `json:"checks,omitempty"`
	Passed                  *bool                `json:"passed,omitempty"`
}
type aiMeasurementScore struct {
	Passed int `json:"passed"`
	Total  int `json:"total"`
}
type aiMeasurementSummary struct {
	Schema                     string                        `json:"schema"`
	Status                     string                        `json:"status"`
	ManifestSHA256             string                        `json:"manifest_sha256"`
	ImplementationSHA          string                        `json:"implementation_sha"`
	Compiler                   aiMeasurementCompiler         `json:"compiler"`
	SuiteSHA256                string                        `json:"suite_sha256"`
	TrustedAsanaResponseSHA256 string                        `json:"trusted_asana_response_sha256"`
	Anchor                     aiMeasurementAnchor           `json:"anchor"`
	AuthenticationBoundary     string                        `json:"asana_authentication_boundary"`
	QualityMeasured            bool                          `json:"ai_quality_measured"`
	ExpectedOutcomes           int                           `json:"expected_outcomes"`
	Records                    []aiMeasurementOutcome        `json:"records"`
	ProvenanceScope            string                        `json:"provenance_scope"`
	UnseenPatternProved        *bool                         `json:"unseen_pattern_provenance_proved"`
	AppleRuntimeProved         *bool                         `json:"apple_runtime_proved"`
	FormalAcceptanceGranted    *bool                         `json:"formal_acceptance_granted"`
	MeasuredResults            map[string]aiMeasurementScore `json:"measured_results,omitempty"`
	ExecutionInterval          aiMeasurementInterval         `json:"execution_interval"`
}
type aiMeasurementExecution struct {
	Configured bool
	Config     aiMeasurementConfig
	Summary    aiMeasurementSummary
	Artifacts  []EvidenceArtifact
	Command    []string
	Err        error
}

func aiMeasurementTestID(id string) string {
	return "tools/language-ai-eval/evaluation_intake.py:" + id
}

func aiMeasurementTime(value string) (time.Time, error) {
	var parsed, err = time.Parse(time.RFC3339Nano, value)
	if err != nil || !strings.HasSuffix(value, "Z") {
		return time.Time{}, fmt.Errorf("AI measurement requires an explicit UTC timestamp")
	}
	return parsed, nil
}

// These pins are supplied independently by the authenticated root/operator
// channel. A bundle, record or JSON claim of authentication cannot supply them.
func aiMeasurementConfiguration() (aiMeasurementConfig, bool, error) {
	var config aiMeasurementConfig
	var configured bool
	for _, name := range aiMeasurementEnvironment {
		configured = configured || os.Getenv(name) != ""
	}
	if !configured {
		return config, false, nil
	}
	config.Root = os.Getenv("CHERRI_AI_EVALUATION_SOURCE_ROOT")
	if config.Root == "" {
		var err error
		config.Root, err = filepath.Abs(resolveRepoRoot())
		if err != nil {
			return config, true, err
		}
	}
	config.Bundle = os.Getenv("CHERRI_AI_EVALUATION_BUNDLE")
	config.StoryID = os.Getenv("CHERRI_AI_EVALUATION_ANCHOR_STORY_ID")
	config.TextSHA256 = os.Getenv("CHERRI_AI_EVALUATION_ANCHOR_TEXT_SHA256")
	config.Before = aiMeasurementTrust{
		Path:        os.Getenv("CHERRI_AI_EVALUATION_ASANA_BEFORE"),
		SHA256:      os.Getenv("CHERRI_AI_EVALUATION_ASANA_BEFORE_SHA256"),
		ObservedUTC: os.Getenv("CHERRI_AI_EVALUATION_ASANA_BEFORE_OBSERVED_UTC"),
	}
	config.After = aiMeasurementTrust{
		Path:        os.Getenv("CHERRI_AI_EVALUATION_ASANA_AFTER"),
		SHA256:      os.Getenv("CHERRI_AI_EVALUATION_ASANA_AFTER_SHA256"),
		ObservedUTC: os.Getenv("CHERRI_AI_EVALUATION_ASANA_AFTER_OBSERVED_UTC"),
	}
	if err := aiMeasurementConfigShape(config); err != nil {
		return config, true, err
	}
	for _, directory := range []string{config.Root, config.Bundle} {
		info, err := os.Lstat(directory)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return config, true, fmt.Errorf("AI measurement owned directory unavailable")
		}
	}
	var beforeInfo os.FileInfo
	for index, trust := range []aiMeasurementTrust{config.Before, config.After} {
		raw, err := aiMeasurementReadRegular(trust.Path, 4<<20)
		if err != nil || editorSHA(raw) != trust.SHA256 {
			return config, true, fmt.Errorf("AI measurement independent Asana input %d does not match the operator pin", index)
		}
		info, err := os.Stat(trust.Path)
		if err != nil {
			return config, true, err
		}
		if index == 0 {
			beforeInfo = info
		} else if os.SameFile(beforeInfo, info) {
			return config, true, fmt.Errorf("AI measurement requires two independently retained Asana reads")
		}
	}
	return config, true, nil
}

func aiMeasurementConfigShape(config aiMeasurementConfig) error {
	if !regexp.MustCompile("^[0-9]{10,30}$").MatchString(config.StoryID) || !documentationHash(config.TextSHA256, 64) {
		return fmt.Errorf("AI measurement requires independently pinned anchor story and body")
	}
	for _, path := range []string{config.Root, config.Bundle, config.Before.Path, config.After.Path} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return fmt.Errorf("AI measurement requires absolute clean input paths")
		}
	}
	for _, trust := range []aiMeasurementTrust{config.Before, config.After} {
		relative, err := filepath.Rel(config.Bundle, trust.Path)
		if err != nil || relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) ||
			!documentationHash(trust.SHA256, 64) {
			return fmt.Errorf("AI measurement Asana pin must be independent of the bundle")
		}
		if _, err := aiMeasurementTime(trust.ObservedUTC); err != nil {
			return err
		}
	}
	before, _ := aiMeasurementTime(config.Before.ObservedUTC)
	after, _ := aiMeasurementTime(config.After.ObservedUTC)
	if !before.Before(after) || config.Before.Path == config.After.Path {
		return fmt.Errorf("AI measurement Asana read order is invalid")
	}
	return nil
}

func aiMeasurementReadRegular(path string, limit int64) ([]byte, error) {
	var before, err = os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Size() < 0 || before.Size() > limit {
		return nil, fmt.Errorf("AI measurement input is not a bounded regular file")
	}
	var file, openErr = os.Open(path)
	if openErr != nil {
		return nil, openErr
	}
	defer file.Close()
	var opened, statErr = file.Stat()
	if statErr != nil || !os.SameFile(before, opened) {
		return nil, fmt.Errorf("AI measurement input identity changed before read")
	}
	var raw, readErr = io.ReadAll(io.LimitReader(file, limit+1))
	var after, afterErr = os.Lstat(path)
	var final, finalErr = file.Stat()
	if readErr != nil || afterErr != nil || finalErr != nil || int64(len(raw)) != before.Size() ||
		int64(len(raw)) > limit || !os.SameFile(before, after) || !os.SameFile(before, final) ||
		after.Size() != before.Size() || final.Size() != before.Size() ||
		!after.ModTime().Equal(before.ModTime()) || !final.ModTime().Equal(before.ModTime()) {
		return nil, fmt.Errorf("AI measurement input changed during bounded read")
	}
	return raw, nil
}

func validateAIMeasurementSummary(raw []byte, source string, config aiMeasurementConfig, trust aiMeasurementTrust, observedUTC string) (aiMeasurementSummary, error) {
	var report aiMeasurementSummary
	if err := decodeEditorJSON(raw, &report); err != nil {
		return report, fmt.Errorf("AI measurement summary JSON: %w", err)
	}
	if report.Schema != "cherri-ai-evaluation-intake/v1" || report.Status != "VERIFIED_MEASUREMENT" ||
		!report.QualityMeasured || report.ExpectedOutcomes != 12 || len(report.Records) != 12 {
		return report, fmt.Errorf("AI measurement requires all twelve measured outcomes; exit zero or incomplete outcomes do not measure quality")
	}
	if !documentationHash(source, 40) || report.ImplementationSHA != source ||
		!documentationHash(report.Compiler.SourceSHA, 40) || !documentationHash(report.Compiler.BinarySHA, 64) ||
		!documentationHash(report.ManifestSHA256, 64) || !documentationHash(report.SuiteSHA256, 64) ||
		report.TrustedAsanaResponseSHA256 != trust.SHA256 ||
		report.Anchor.TaskID != aiMeasurementTask || report.Anchor.StoryID != config.StoryID ||
		report.Anchor.TextSHA256 != config.TextSHA256 || report.AuthenticationBoundary != aiMeasurementTrustBoundary {
		return report, fmt.Errorf("AI measurement source, compiler or independent Asana binding mismatch")
	}
	if report.ProvenanceScope != aiMeasurementScope || report.UnseenPatternProved == nil || *report.UnseenPatternProved ||
		report.AppleRuntimeProved == nil || *report.AppleRuntimeProved ||
		report.FormalAcceptanceGranted == nil || *report.FormalAcceptanceGranted {
		return report, fmt.Errorf("AI measurement cannot grant unseen-pattern, Apple or global acceptance scope")
	}
	var anchor, anchorErr = aiMeasurementTime(report.Anchor.CreatedUTC)
	var started, startErr = aiMeasurementTime(report.ExecutionInterval.StartedUTC)
	var finished, finishErr = aiMeasurementTime(report.ExecutionInterval.FinishedUTC)
	var before, beforeErr = aiMeasurementTime(config.Before.ObservedUTC)
	var after, afterErr = aiMeasurementTime(config.After.ObservedUTC)
	var observed, observedErr = aiMeasurementTime(observedUTC)
	if anchorErr != nil || startErr != nil || finishErr != nil || beforeErr != nil || afterErr != nil || observedErr != nil ||
		anchor.After(before) || !before.Before(started) || finished.Before(started) ||
		!finished.Before(after) || after.After(observed) {
		return report, fmt.Errorf("AI measurement independent reads do not bracket the original provider and validator interval")
	}
	var expected = map[string]bool{}
	for _, task := range aiMeasurementTaskIDs {
		for _, arm := range []string{"legacy", "v2"} {
			expected[task+"/"+arm] = true
		}
	}
	var requests, providerRequests = map[string]bool{}, map[string]bool{}
	var scores = map[string]aiMeasurementScore{"legacy": {Total: 6}, "v2": {Total: 6}}
	for _, outcome := range report.Records {
		key := outcome.TaskID + "/" + outcome.Arm
		if !expected[key] || !documentationHash(outcome.RequestID, 64) || requests[outcome.RequestID] ||
			!documentationHash(outcome.ProviderRequestIDSHA256, 64) || providerRequests[outcome.ProviderRequestIDSHA256] ||
			!documentationHash(outcome.ProviderReceiptSHA256, 64) || outcome.Passed == nil ||
			outcome.Usage == nil || outcome.Usage.Input == nil || outcome.Usage.Output == nil ||
			*outcome.Usage.Input < 0 || *outcome.Usage.Input > 12000 || *outcome.Usage.Output < 0 || *outcome.Usage.Output > 2000 {
			return report, fmt.Errorf("AI measurement twelve-outcome identity or actual usage inventory is incomplete")
		}
		delete(expected, key)
		requests[outcome.RequestID], providerRequests[outcome.ProviderRequestIDSHA256] = true, true
		switch outcome.Status {
		case "MEASURED":
			if !documentationHash(outcome.ValidatorReceiptSHA256, 64) || !documentationHash(outcome.SourceSHA256, 64) ||
				outcome.ProviderStatus != "" || len(outcome.Checks) == 0 {
				return report, fmt.Errorf("AI measured outcome lacks original source or validator checks")
			}
			var passed = true
			var seen = map[string]bool{}
			for _, check := range outcome.Checks {
				if check.Name == "" || seen[check.Name] || check.Passed == nil {
					return report, fmt.Errorf("AI measured check inventory is malformed")
				}
				seen[check.Name] = true
				passed = passed && *check.Passed
			}
			if passed != *outcome.Passed {
				return report, fmt.Errorf("AI measured task result differs from its checks")
			}
		case "MEASURED_PROVIDER_FAILURE":
			if *outcome.Passed || outcome.ValidatorReceiptSHA256 != "" || len(outcome.Checks) != 0 ||
				(outcome.ProviderStatus != "REFUSED" && outcome.ProviderStatus != "TRUNCATED") ||
				(outcome.SourceSHA256 != "" && !documentationHash(outcome.SourceSHA256, 64)) {
				return report, fmt.Errorf("AI provider refusal or truncation must remain a measured task failure")
			}
		default:
			return report, fmt.Errorf("AI transport, protocol or unadjudicated outcome is incomplete, not a measured failure")
		}
		if *outcome.Passed {
			score := scores[outcome.Arm]
			score.Passed++
			scores[outcome.Arm] = score
		}
	}
	if len(expected) != 0 || !reflect.DeepEqual(report.MeasuredResults, scores) {
		return report, fmt.Errorf("AI measurement requires the actual six-per-arm denominator and recomputed scores")
	}
	return report, nil
}

func aiMeasurementCommand(python string, config aiMeasurementConfig, trust aiMeasurementTrust, output string) []string {
	return []string{python, "-E", "-s", "-B", filepath.Join(config.Root, "tools", "language-ai-eval", "evaluation_intake.py"),
		"--bundle", config.Bundle, "--trusted-asana-response", trust.Path, "--output", output}
}

func aiMeasurementRun(source string, config aiMeasurementConfig, directory string) aiMeasurementExecution {
	var result = aiMeasurementExecution{Configured: true, Config: config}
	var python, err = acceptancePython3("CHERRI_ACCEPTANCE_PYTHON")
	if err != nil {
		result.Err = err
		return result
	}
	var first aiMeasurementSummary
	for index, trust := range []aiMeasurementTrust{config.Before, config.After} {
		label := []string{"before", "after"}[index]
		prefix := "ai-measurement-" + label
		output := filepath.Join(directory, prefix+"-intake.json")
		command := aiMeasurementCommand(python, config, trust, output)
		receipt, artifacts, processErr := runExactSourceProcess(config.Root, source, aiMeasurementProcessID, prefix,
			directory, command, aiMeasurementTimeout, nil, aiMeasurementSources)
		result.Artifacts = append(result.Artifacts, artifacts...)
		result.Command = command
		if processErr != nil {
			result.Err = fmt.Errorf("AI canonical intake %s process failed: %w", label, processErr)
			return result
		}
		raw, readErr := aiMeasurementReadRegular(output, 1<<20)
		if readErr != nil {
			result.Err = readErr
			return result
		}
		result.Artifacts = append(result.Artifacts, EvidenceArtifact{Path: output, SHA256: editorSHA(raw)})
		report, validationErr := validateAIMeasurementSummary(raw, source, config, trust, receipt.StartedUTC)
		if validationErr != nil {
			result.Err = validationErr
			return result
		}
		stdout, stdoutErr := aiMeasurementReadRegular(filepath.Join(directory, prefix+"-go.jsonl"), 64<<10)
		if stdoutErr != nil {
			result.Err = stdoutErr
			return result
		}
		if err := validateAIMeasurementAnnouncement(stdout, report); err != nil {
			result.Err = err
			return result
		}
		if index == 0 {
			first = report
		} else {
			first.TrustedAsanaResponseSHA256 = report.TrustedAsanaResponseSHA256
			if !reflect.DeepEqual(first, report) {
				result.Err = fmt.Errorf("AI original outcomes or exact anchor changed between independently pinned reads")
				return result
			}
			result.Summary = report
		}
	}
	return result
}

func (r *Runner) aiMeasurement() *aiMeasurementExecution {
	if r.aiMeasurementResult != nil {
		return r.aiMeasurementResult
	}
	var config, configured, err = aiMeasurementConfiguration()
	var result = aiMeasurementExecution{Configured: configured, Config: config, Err: err}
	if configured && err == nil {
		var parent = r.artifactDir
		if parent != "" {
			parent, err = filepath.Abs(parent)
			if err == nil {
				err = os.MkdirAll(parent, 0700)
			}
		}
		var directory string
		if err == nil {
			directory, err = os.MkdirTemp(parent, "ai-measurement-")
		}
		if err == nil {
			result = aiMeasurementRun(r.implSHA, config, directory)
		} else {
			result.Err = err
		}
	}
	r.aiMeasurementResult = &result
	return &result
}

func aiMeasurementAssertions(report aiMeasurementSummary, config aiMeasurementConfig) map[string]string {
	return map[string]string{
		"ai_quality_measured": "true", "measurement_status": "VERIFIED_MEASUREMENT",
		"expected_outcomes": "12", "provenance_scope": aiMeasurementScope,
		"unseen_pattern_provenance_proved": "false", "apple_runtime_proved": "false",
		"formal_acceptance_granted": "false", "asana_authentication_boundary": aiMeasurementTrustBoundary,
		"asana_task_id": aiMeasurementTask, "asana_story_id": config.StoryID, "asana_text_sha256": config.TextSHA256,
		"asana_before_sha256": config.Before.SHA256, "asana_after_sha256": config.After.SHA256,
		"asana_before_observed_utc": config.Before.ObservedUTC, "asana_after_observed_utc": config.After.ObservedUTC,
		"manifest_sha256": report.ManifestSHA256, "suite_sha256": report.SuiteSHA256,
		"compiler_source_sha": report.Compiler.SourceSHA, "compiler_binary_sha256": report.Compiler.BinarySHA,
		"legacy_passed": strconv.Itoa(report.MeasuredResults["legacy"].Passed), "legacy_total": "6",
		"v2_passed": strconv.Itoa(report.MeasuredResults["v2"].Passed), "v2_total": "6",
	}
}

func (r *Runner) appendAIMeasurementRecord(id string, result *aiMeasurementExecution) {
	var record = EvidenceRecord{
		EvidenceID: "ev-ai-measurement-" + id, Requirements: []string{id}, Tier: "external-eval",
		ImplementationSHA: r.implSHA, TestID: aiMeasurementTestID(id), Command: result.Command,
		Result: "passed", Artifacts: result.Artifacts,
	}
	if result.Err != nil {
		// -1 is an aggregate preflight/intake rejection, never a fabricated
		// provider or Python exit code. Original process receipts retain theirs.
		record.Result, record.ExitCode = "rejected", -1
		record.Assertions = map[string]string{"ai_quality_measured": "false", "intake_rejected": "true"}
		record.Detail = "Configured evaluation intake rejected: " + result.Err.Error()
	} else {
		record.FixtureSHA256 = result.Summary.SuiteSHA256
		record.Assertions = aiMeasurementAssertions(result.Summary, result.Config)
		record.Detail = "Twelve original outcomes revalidated against both independently pinned Asana reads; " + aiMeasurementScope +
			"; valid refusals, truncations and semantic failures stay in the six-per-arm denominator; no Apple or unseen-pattern proof"
	}
	r.records = append(r.records, record)
}

func (r *Runner) runMeasuredAIEvaluation() {
	var result = r.aiMeasurement()
	if !result.Configured {
		r.Record("AI01", "evaluation", "Held-out equal-budget old/new model evaluation harness", "evaluation", nil, false,
			"AI_EVAL_NOT_RUN", "No independently pinned real evaluation bundle is configured; RP50 fake harness controls do not measure quality")
		return
	}
	r.appendAIMeasurementRecord("AI01", result)
	var record = r.records[len(r.records)-1]
	r.results["AI01"] = CaseResult{ID: "AI01", Group: "evaluation", Requirement: "Held-out equal-budget old/new model evaluation harness",
		TestLevel: "evaluation", Passed: result.Err == nil, Status: "PASSED", Detail: record.Detail}
	if result.Err != nil {
		var rejected = r.results["AI01"]
		rejected.Status = "FAILED"
		r.results["AI01"] = rejected
	} else {
		var measured = r.results["AI01"]
		measured.ExecutedLevels = []string{"external-eval"}
		r.results["AI01"] = measured
	}
}

func validateAIMeasurementRecord(id string, record EvidenceRecord) error {
	if id != "AI01" && id != "RP50" {
		if strings.HasPrefix(record.TestID, "tools/language-ai-eval/evaluation_intake.py:") {
			return fmt.Errorf("AI measurement cannot confer evidence on unrelated requirement %s", id)
		}
		return nil
	}
	if id == "RP50" && record.Tier == "harness-unit" {
		return nil
	}
	if !reflect.DeepEqual(record.Requirements, []string{id}) || record.TestID != aiMeasurementTestID(id) ||
		record.Tier != "external-eval" || record.Result != "passed" || record.ExitCode != 0 ||
		record.EvidenceID != "ev-ai-measurement-"+id || len(record.Artifacts) != 8 {
		return fmt.Errorf("record %s is not an actual complete source-bound AI measurement for %s", record.EvidenceID, id)
	}
	var config, configured, err = aiMeasurementConfiguration()
	if err != nil || !configured {
		return fmt.Errorf("AI measurement requires the independent operator configuration: %v", err)
	}
	var data = map[string][]byte{}
	var directory string
	for _, artifact := range record.Artifacts {
		name := filepath.Base(artifact.Path)
		if data[name] != nil || artifact.RunID != "" || !filepath.IsAbs(artifact.Path) {
			return fmt.Errorf("AI measurement artifact identity is ambiguous")
		}
		if directory == "" {
			directory = filepath.Dir(artifact.Path)
		} else if directory != filepath.Dir(artifact.Path) {
			return fmt.Errorf("AI measurement original artifacts are detached")
		}
		raw, readErr := aiMeasurementReadRegular(artifact.Path, 1<<20)
		if readErr != nil || editorSHA(raw) != artifact.SHA256 {
			return fmt.Errorf("AI measurement original artifact hash or size mismatch")
		}
		data[name] = raw
	}
	var python, pythonErr = acceptancePython3("CHERRI_ACCEPTANCE_PYTHON")
	if pythonErr != nil {
		return pythonErr
	}
	var afterSummary aiMeasurementSummary
	for index, trust := range []aiMeasurementTrust{config.Before, config.After} {
		prefix := "ai-measurement-" + []string{"before", "after"}[index]
		var receipt editorProcessReceipt
		if err := decodeEditorJSON(data[prefix+"-process.json"], &receipt); err != nil {
			return fmt.Errorf("AI original process receipt: %w", err)
		}
		command := aiMeasurementCommand(python, config, trust, filepath.Join(directory, prefix+"-intake.json"))
		if err := validateExactSourceProcess(aiMeasurementProcessID, record.ImplementationSHA, receipt, command, nil,
			data[prefix+"-go.jsonl"], data[prefix+"-stderr.log"], aiMeasurementTimeout, aiMeasurementSources); err != nil {
			return fmt.Errorf("AI original exact-source process: %w", err)
		}
		report, reportErr := validateAIMeasurementSummary(data[prefix+"-intake.json"], record.ImplementationSHA, config, trust, receipt.StartedUTC)
		if reportErr != nil {
			return reportErr
		}
		if err := validateAIMeasurementAnnouncement(data[prefix+"-go.jsonl"], report); err != nil {
			return err
		}
		if index == 1 {
			afterSummary = report
			if !reflect.DeepEqual(record.Command, command) {
				return fmt.Errorf("AI record command differs from original canonical intake")
			}
		}
	}
	if record.FixtureSHA256 != afterSummary.SuiteSHA256 || !reflect.DeepEqual(record.Assertions, aiMeasurementAssertions(afterSummary, config)) {
		return fmt.Errorf("AI record assertions differ from actual original measurement")
	}
	// A standalone summary/process JSON is insufficient. Re-run only the frozen
	// read-only verifier against operator-selected originals, never the provider.
	var temporary, tempErr = os.MkdirTemp("", "ai-intake-revalidation-")
	if tempErr != nil {
		return tempErr
	}
	defer os.RemoveAll(temporary)
	var replay = aiMeasurementRun(record.ImplementationSHA, config, temporary)
	if replay.Err != nil {
		return fmt.Errorf("AI original outcome revalidation: %w", replay.Err)
	}
	for _, label := range []string{"before", "after"} {
		name := "ai-measurement-" + label + "-intake.json"
		raw, readErr := aiMeasurementReadRegular(filepath.Join(temporary, name), 1<<20)
		if readErr != nil || !bytes.Equal(raw, data[name]) {
			return fmt.Errorf("AI standalone summary differs from revalidated original outcomes")
		}
	}
	return nil
}

func (r *Runner) executeAIBoundHarness() ([]EvidenceArtifact, string, error) {
	var root, err = filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", err
	}
	var parent = r.artifactDir
	if parent != "" {
		parent, err = filepath.Abs(parent)
		if err == nil {
			err = os.MkdirAll(parent, 0700)
		}
		if err != nil {
			return nil, "", err
		}
	}
	var directory, directoryErr = os.MkdirTemp(parent, "ai-harness-")
	if directoryErr != nil {
		return nil, "", directoryErr
	}
	var scenario = aiEvaluationHarnessScenario
	var command = productionTestCommandInPackage(scenario.PackagePath, scenario.TestNames)
	var _, artifacts, processErr = runExactSourceProcess(root, r.implSHA, "RP50-harness", "ai-harness",
		directory, command, productionTestTimeout(scenario.TestNames)+time.Minute, scenario.TestNames, aiMeasurementSources)
	if processErr != nil {
		return artifacts, "", processErr
	}
	var raw, rawErr = aiMeasurementReadRegular(filepath.Join(directory, "ai-harness-go.jsonl"), 4<<20)
	if rawErr != nil {
		return artifacts, "", rawErr
	}
	var detail, detailErr = aiBoundHarnessExecutionDetail(raw)
	return artifacts, detail, detailErr
}

func aiBoundHarnessExecutionDetail(raw []byte) (string, error) {
	if err := validateAIBoundHarnessOutput(raw); err != nil {
		return "", err
	}
	var observed []string
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var event struct{ Action, Test string }
		if err := json.Unmarshal(line, &event); err != nil {
			return "", err
		}
		if event.Action == "pass" && event.Test != "" {
			observed = append(observed, event.Test)
		}
	}
	sort.Strings(observed)
	return "Actual fixed Go child " + strings.Join(observed, ", ") + " executed all fifteen Python harness controls; originals and immutable source witnesses retained", nil
}

func validateAIBoundHarnessOutput(raw []byte) error {
	var scenario = aiEvaluationHarnessScenario
	if err := parseStrictNamedProductionOutputInPackage(scenario.ModulePath, scenario.TestNames, 3*time.Minute, raw); err != nil {
		return fmt.Errorf("RP50 actual harness Go events: %w", err)
	}
	var output strings.Builder
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var event struct{ Test, Output string }
		if err := json.Unmarshal(line, &event); err != nil {
			return err
		}
		if event.Test == scenario.TestNames[0] {
			output.WriteString(event.Output)
		}
	}
	const marker = "P15_RP50_HARNESS_CONTROL_DISCOVERY:"
	var reports [][]byte
	for _, line := range strings.Split(output.String(), "\n") {
		if _, report, found := strings.Cut(line, marker); found {
			reports = append(reports, []byte(strings.TrimSpace(report)))
		}
	}
	if len(reports) != 1 {
		return fmt.Errorf("RP50 actual Python discovery receipt is missing or duplicated")
	}
	var receipt struct {
		SelectionComplete   bool     `json:"selection_complete"`
		Found               []string `json:"found"`
		Passed              []string `json:"passed"`
		TestsRun            int      `json:"tests_run"`
		Failures            int      `json:"failures"`
		Errors              int      `json:"errors"`
		Skipped             int      `json:"skipped"`
		ExpectedFailures    int      `json:"expected_failures"`
		UnexpectedSuccesses int      `json:"unexpected_successes"`
	}
	var required = append([]string{}, aiEvaluationPythonTests...)
	sort.Strings(required)
	if err := decodeEditorJSON(reports[0], &receipt); err != nil {
		return err
	}
	if !receipt.SelectionComplete || receipt.TestsRun != len(required) || !reflect.DeepEqual(receipt.Found, required) ||
		!reflect.DeepEqual(receipt.Passed, required) || receipt.Failures != 0 || receipt.Errors != 0 ||
		receipt.Skipped != 0 || receipt.ExpectedFailures != 0 || receipt.UnexpectedSuccesses != 0 {
		return fmt.Errorf("RP50 did not execute the complete fixed Python harness inventory")
	}
	return nil
}

func validateAIBoundHarnessArtifacts(record EvidenceRecord) error {
	if !reflect.DeepEqual(record.Requirements, []string{"RP50"}) || len(record.Artifacts) != 3 {
		return fmt.Errorf("RP50 harness metadata is not source-bound actual execution")
	}
	var data = map[string][]byte{}
	var directory string
	for _, artifact := range record.Artifacts {
		name := filepath.Base(artifact.Path)
		if data[name] != nil || artifact.RunID != "" {
			return fmt.Errorf("RP50 harness artifact identity is ambiguous")
		}
		if directory == "" {
			directory = filepath.Dir(artifact.Path)
		} else if directory != filepath.Dir(artifact.Path) {
			return fmt.Errorf("RP50 harness artifacts are detached")
		}
		raw, err := aiMeasurementReadRegular(artifact.Path, 4<<20)
		if err != nil || editorSHA(raw) != artifact.SHA256 {
			return fmt.Errorf("RP50 harness original artifact mismatch")
		}
		data[name] = raw
	}
	var receipt editorProcessReceipt
	if err := decodeEditorJSON(data["ai-harness-process.json"], &receipt); err != nil {
		return err
	}
	var scenario = aiEvaluationHarnessScenario
	var command = productionTestCommandInPackage(scenario.PackagePath, scenario.TestNames)
	if err := validateExactSourceProcess("RP50-harness", record.ImplementationSHA, receipt, command, scenario.TestNames,
		data["ai-harness-go.jsonl"], data["ai-harness-stderr.log"], productionTestTimeout(scenario.TestNames)+time.Minute, aiMeasurementSources); err != nil {
		return fmt.Errorf("RP50 harness actual source/process binding: %w", err)
	}
	return validateAIBoundHarnessOutput(data["ai-harness-go.jsonl"])
}

func validateAIMeasurementAnnouncement(raw []byte, report aiMeasurementSummary) error {
	var announced struct {
		Status          string `json:"status"`
		QualityMeasured bool   `json:"ai_quality_measured"`
	}
	if err := decodeEditorJSON(raw, &announced); err != nil {
		return fmt.Errorf("AI canonical intake announcement JSON: %w", err)
	}
	if announced.Status != report.Status || announced.Status != "VERIFIED_MEASUREMENT" || !announced.QualityMeasured {
		return fmt.Errorf("AI canonical intake announcement contradicts the measured report")
	}
	return nil
}
