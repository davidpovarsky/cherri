package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"time"
)

const takeoverSchema = "cherri-takeover-preservation/1"
const takeoverTestID = "tools/language-acceptance:BRG01TakeoverPreservation"
const takeoverScope = "Actual dual-repository takeover, private unpublished backups and create-only local recovery refs; no other acceptance or release credit"
const takeoverSource = "tools/p10r15/takeover_preservation.py"
const takeoverTimeout = 120 * time.Second

var takeoverSources = []string{
	takeoverSource, "tools/p10r15/test_takeover_preservation.py",
	"tools/language-acceptance/takeover_execution.go",
	"tools/language-acceptance/takeover_execution_test.go",
	"tools/language-acceptance/main.go", "tools/language-acceptance/evidence_map.go",
	"tools/language-acceptance/execution_audit.go",
	"tests/language-v2/backend-recovery-gates.json",
}

type takeoverConfig struct {
	ExpectedPath   string
	ExpectedSHA256 string
}
type takeoverVerification struct {
	Schema             string `json:"schema"`
	Status             string `json:"status"`
	ImplementationSHA  string `json:"implementationSHA"`
	ExpectedSHA256     string `json:"expectedSHA256"`
	ReceiptSHA256      string `json:"receiptSHA256"`
	Repositories       int    `json:"repositories"`
	Commands           int    `json:"commands"`
	UnpublishedBackups int    `json:"unpublishedBackups"`
	Scope              string `json:"scope"`
	ClosureComplete    bool   `json:"closureComplete"`
}
type takeoverObservation struct {
	Schema            string               `json:"schema"`
	ImplementationSHA string               `json:"implementationSHA"`
	ExpectedSHA256    string               `json:"expectedSHA256"`
	ProducerSHA256    string               `json:"producerSHA256"`
	StartedUTC        string               `json:"startedUtc"`
	FinishedUTC       string               `json:"finishedUtc"`
	ElapsedSeconds    float64              `json:"elapsedSeconds"`
	Command           []string             `json:"command"`
	ExitCode          *int                 `json:"exitCode"`
	TimedOut          *bool                `json:"timedOut"`
	StdoutSHA256      string               `json:"stdoutSHA256"`
	StderrSHA256      string               `json:"stderrSHA256"`
	Verification      takeoverVerification `json:"verification"`
	Status            string               `json:"status"`
	Scope             string               `json:"scope"`
	ClosureComplete   bool                 `json:"closureComplete"`
	Failure           string               `json:"failure,omitempty"`
}

func takeoverEnvironment() (takeoverConfig, bool, error) {
	c := takeoverConfig{os.Getenv("CHERRI_TAKEOVER_EXPECTED"), os.Getenv("CHERRI_TAKEOVER_EXPECTED_SHA256")}
	if c.ExpectedPath == "" && c.ExpectedSHA256 == "" {
		return c, false, nil
	}
	if !filepath.IsAbs(c.ExpectedPath) || !finalProvenanceHex(c.ExpectedSHA256, 64) {
		return c, true, fmt.Errorf("BRG01_INCOMPLETE_INDEPENDENT_CONTEXT")
	}
	return c, true, nil
}

func takeoverCanonicalModule(ctx context.Context, root, implementation string) ([]byte, error) {
	if !finalProvenanceHex(implementation, 40) {
		return nil, fmt.Errorf("BRG01_IMPLEMENTATION_IDENTITY")
	}
	command := exec.CommandContext(ctx, "git", "--no-replace-objects", "-C", root, "show", implementation+":"+takeoverSource)
	configureEditorProductionProcess(command)
	data, err := command.Output()
	if err != nil || len(data) == 0 || len(data) > 128<<10 {
		return nil, fmt.Errorf("BRG01_IMMUTABLE_PRODUCER_UNAVAILABLE")
	}
	return data, nil
}

func takeoverCheckVerification(v takeoverVerification, config takeoverConfig, implementation string) error {
	if v.Schema != takeoverSchema || v.Status != "PASSED" || v.ImplementationSHA != implementation ||
		v.ExpectedSHA256 != config.ExpectedSHA256 || !finalProvenanceHex(v.ReceiptSHA256, 64) ||
		v.Repositories != 2 || v.Commands != 58 || v.UnpublishedBackups < 0 ||
		v.Scope != takeoverScope || v.ClosureComplete {
		return fmt.Errorf("BRG01_VERIFIER_SCOPE_OR_RESULT")
	}
	return nil
}

// This operation only revalidates separately selected original takeover evidence.
// The acceptance runner never creates recovery branches or manufactures backups.
func executeTakeoverVerification(config takeoverConfig, implementation, root, directory string) (o takeoverObservation, outcome error) {
	start := time.Now()
	o = takeoverObservation{Schema: takeoverSchema, ImplementationSHA: implementation,
		ExpectedSHA256: config.ExpectedSHA256, StartedUTC: start.UTC().Format(time.RFC3339Nano),
		Status: "FAILED", Scope: takeoverScope}
	defer func() {
		o.FinishedUTC = time.Now().UTC().Format(time.RFC3339Nano)
		o.ElapsedSeconds = time.Since(start).Seconds()
		if outcome != nil {
			o.Failure = "BRG01_ORIGINALS_OR_CURRENT_SOURCE_REJECTED"
		}
	}()
	if !filepath.IsAbs(config.ExpectedPath) || !finalProvenanceHex(config.ExpectedSHA256, 64) {
		return o, fmt.Errorf("BRG01_INCOMPLETE_INDEPENDENT_CONTEXT")
	}
	ctx, cancel := context.WithTimeout(context.Background(), takeoverTimeout)
	defer cancel()
	module, err := takeoverCanonicalModule(ctx, root, implementation)
	if err != nil {
		return o, err
	}
	o.ProducerSHA256 = finalProvenanceDigest(module)
	python, err := acceptancePython3("CHERRI_TAKEOVER_PYTHON")
	if err != nil {
		return o, err
	}
	directory, err = filepath.Abs(directory)
	if err != nil {
		return o, err
	}
	modulePath := filepath.Join(directory, "takeover-verifier.py")
	if err = os.WriteFile(modulePath, module, 0600); err != nil {
		return o, err
	}
	o.Command = []string{python, "-I", "-B", modulePath, "verify", "--expected", config.ExpectedPath,
		"--expected-sha256", config.ExpectedSHA256, "--implementation-sha", implementation}
	command := exec.CommandContext(ctx, python, o.Command[1:]...)
	configureEditorProductionProcess(command)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	processErr := command.Run()
	code, timedOut := -1, ctx.Err() != nil
	if command.ProcessState != nil {
		code = command.ProcessState.ExitCode()
	}
	o.ExitCode, o.TimedOut = &code, &timedOut
	o.StdoutSHA256, o.StderrSHA256 = finalProvenanceDigest(stdout.Bytes()), finalProvenanceDigest(stderr.Bytes())
	for name, data := range map[string][]byte{"takeover-verifier.stdout": stdout.Bytes(), "takeover-verifier.stderr": stderr.Bytes()} {
		if len(data) > 128<<10 {
			return o, fmt.Errorf("BRG01_VERIFIER_OUTPUT_LIMIT")
		}
		if err = os.WriteFile(filepath.Join(directory, name), data, 0600); err != nil {
			return o, err
		}
	}
	if processErr != nil || timedOut || code != 0 || stderr.Len() != 0 {
		return o, fmt.Errorf("BRG01_ORIGINAL_VERIFICATION_FAILED")
	}
	if err = decodeEditorJSON(stdout.Bytes(), &o.Verification); err != nil {
		return o, err
	}
	if err = takeoverCheckVerification(o.Verification, config, implementation); err != nil {
		return o, err
	}
	o.Status = "PASSED"
	return o, nil
}

func (r *Runner) runTakeoverRequirement(id, title string) bool {
	if id != "BRG01" {
		return false
	}
	r.takeoverEvaluated = true
	result := CaseResult{ID: id, Group: "recovery_gate", Requirement: title, TestLevel: "repository",
		Status: "NOT_RUN", Detail: "BRG01 independently selected original two-repository takeover evidence is not configured"}
	config, configured, configErr := takeoverEnvironment()
	if !configured {
		r.results[id] = result
		return true
	}
	if configErr != nil {
		result.Status, result.Detail = "FAILED", "BRG01 independent context is malformed"
		r.results[id] = result
		return true
	}
	directory, err := stateEvidenceDirectory(r, "takeover-")
	if err != nil {
		result.Status, result.Detail = "FAILED", "BRG01 evidence directory unavailable"
		r.results[id] = result
		return true
	}
	o, verificationErr := executeTakeoverVerification(config, r.implSHA, resolveRepoRoot(), directory)
	data, err := json.MarshalIndent(o, "", "  ")
	observationPath := filepath.Join(directory, "takeover-observation.json")
	if err == nil {
		data = append(data, '\n')
		err = os.WriteFile(observationPath, data, 0600)
	}
	result.Status, result.Detail, result.ExecutedLevels = o.Status, takeoverScope, []string{"repository"}
	result.Passed = verificationErr == nil && err == nil && o.Status == "PASSED"
	if !result.Passed {
		result.Status, result.Detail = "FAILED", "BRG01 original takeover evidence or current source rejected"
	}
	r.results[id] = result
	if !result.Passed {
		return true
	}
	record := EvidenceRecord{EvidenceID: "ev-takeover-preservation-BRG01", Requirements: []string{id}, Tier: "repository",
		ImplementationSHA: r.implSHA, TestID: takeoverTestID, Command: []string{"internal", takeoverSchema}, ExitCode: 0, Result: "passed",
		Assertions: map[string]string{"scope": takeoverScope, "expected_context_sha256": config.ExpectedSHA256,
			"original_receipt_sha256": o.Verification.ReceiptSHA256, "closure_complete": "false"}, Detail: takeoverScope}
	for _, name := range []string{"takeover-observation.json", "takeover-verifier.py", "takeover-verifier.stdout", "takeover-verifier.stderr"} {
		p := filepath.Join(directory, name)
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			result.Passed, result.Status, result.Detail = false, "FAILED", "BRG01 verification artifact unavailable"
			r.results[id] = result
			return true
		}
		record.Artifacts = append(record.Artifacts, EvidenceArtifact{Path: p, SHA256: finalProvenanceDigest(b)})
	}
	r.records = append(r.records, record)
	return true
}

func validateTakeoverRecord(id string, record EvidenceRecord) error {
	return validateTakeoverRecordAt(id, record, resolveRepoRoot())
}

func validateTakeoverRecordAt(id string, record EvidenceRecord, root string) error {
	if id != "BRG01" {
		if record.TestID == takeoverTestID {
			return fmt.Errorf("BRG01_DOES_NOT_GRANT_ANOTHER_REQUIREMENT")
		}
		return nil
	}
	if record.TestID != takeoverTestID || !reflect.DeepEqual(record.Requirements, []string{"BRG01"}) ||
		record.Tier != "repository" || record.Result != "passed" || record.ExitCode != 0 ||
		!reflect.DeepEqual(record.Command, []string{"internal", takeoverSchema}) ||
		record.Assertions["scope"] != takeoverScope || record.Assertions["closure_complete"] != "false" {
		return fmt.Errorf("BRG01_RECORD_NOT_DEDICATED_TAKEOVER")
	}
	config, configured, err := takeoverEnvironment()
	if err != nil || !configured || record.Assertions["expected_context_sha256"] != config.ExpectedSHA256 {
		return fmt.Errorf("BRG01_INDEPENDENT_CONTEXT_UNAVAILABLE")
	}
	data, err := stateReadArtifacts(record.Artifacts, []string{"takeover-observation.json", "takeover-verifier.py", "takeover-verifier.stdout", "takeover-verifier.stderr"})
	if err != nil {
		return err
	}
	var o takeoverObservation
	if err = decodeEditorJSON(data["takeover-observation.json"], &o); err != nil {
		return err
	}
	modulePath := ""
	for _, artifact := range record.Artifacts {
		if filepath.Base(artifact.Path) == "takeover-verifier.py" {
			modulePath, err = filepath.Abs(artifact.Path)
			if err != nil {
				return err
			}
		}
	}
	python, err := acceptancePython3("CHERRI_TAKEOVER_PYTHON")
	if err != nil {
		return err
	}
	wantCommand := []string{python, "-I", "-B", modulePath, "verify", "--expected", config.ExpectedPath,
		"--expected-sha256", config.ExpectedSHA256, "--implementation-sha", record.ImplementationSHA}
	if o.Schema != takeoverSchema || o.Status != "PASSED" || o.ClosureComplete || o.Scope != takeoverScope ||
		o.ImplementationSHA != record.ImplementationSHA || o.ExpectedSHA256 != config.ExpectedSHA256 ||
		o.Failure != "" || o.ExitCode == nil || *o.ExitCode != 0 || o.TimedOut == nil || *o.TimedOut ||
		!reflect.DeepEqual(o.Command, wantCommand) ||
		o.ProducerSHA256 != finalProvenanceDigest(data["takeover-verifier.py"]) ||
		o.StdoutSHA256 != finalProvenanceDigest(data["takeover-verifier.stdout"]) ||
		o.StderrSHA256 != finalProvenanceDigest(data["takeover-verifier.stderr"]) || len(data["takeover-verifier.stderr"]) != 0 ||
		o.Verification.ReceiptSHA256 != record.Assertions["original_receipt_sha256"] {
		return fmt.Errorf("BRG01_ORIGINAL_OBSERVATION")
	}
	if err = stateTiming(o.StartedUTC, o.FinishedUTC, o.ElapsedSeconds, takeoverTimeout.Seconds()); err != nil {
		return err
	}
	var raw takeoverVerification
	if err = decodeEditorJSON(data["takeover-verifier.stdout"], &raw); err != nil || !reflect.DeepEqual(raw, o.Verification) {
		return fmt.Errorf("BRG01_ORIGINAL_OUTPUT")
	}
	if err = takeoverCheckVerification(raw, config, record.ImplementationSHA); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "cherri-takeover-revalidation-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	replayed, err := executeTakeoverVerification(config, record.ImplementationSHA, root, dir)
	if err != nil || replayed.Status != "PASSED" || replayed.ProducerSHA256 != o.ProducerSHA256 ||
		!reflect.DeepEqual(replayed.Verification, o.Verification) {
		return fmt.Errorf("BRG01_ORIGINAL_REVALIDATION")
	}
	return nil
}

func applyTakeoverAudit(row *ExecutionAuditRow, runner *Runner) {
	if row.ID != "BRG01" || !runner.takeoverEvaluated {
		return
	}
	row.TestNames, row.ProductionPath = []string{takeoverTestID}, append([]string{}, takeoverSources...)
	row.Scope, row.PositiveAndNegative, row.Blocker = takeoverScope, false, ""
	if result := runner.results["BRG01"]; result.Status != "PASSED" {
		row.Blocker = result.Detail
	}
}
