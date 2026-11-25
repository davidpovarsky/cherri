package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

const finalGateSchema = "cherri-final-required-gates/1"
const finalGateTimeout = 360 * time.Second
const finalGateRP49Scope = "Dedicated RP49: independently predeclared final event/ref/attempt inventory; completed successful required jobs including post-main Build Linux/macOS/Nix, OpenMinis, full iOS, runtime PoC and Docs build/deploy; current K/S/D package and artifact bytes; all observed attempts retained; actual stale-SHA and missing-artifact candidate rejection. No other requirement or global179 closure is granted."
const finalGateBRG31Scope = "Dedicated BRG31: the full RP49 closure plus exact-source CI04 publication security, actual WorkflowKit positive/corrupt-EXPECT/restored process triplet and complete source-bound computed runtime originals. Native assertions are replayed from the selected final run's original bytes; generic PoC labels, aggregate XCTest counts and related requirement records do not grant this requirement. No global179 closure is granted."

var finalGateSources = []string{
	"tools/language-acceptance/main.go",
	"tools/language-acceptance/evidence_map.go",
	"tools/language-acceptance/execution_audit.go",
	"tools/language-acceptance/final_gate_canonical_integration_test.go",
	"tools/language-acceptance/ci_identity_execution_test.go",
	"tools/language-acceptance/final_gate_contract.go",
	"tools/language-acceptance/final_gate_closure.go",
	"tools/language-acceptance/final_gate_execution.go",
	"tools/language-acceptance/final_gate_native.go",
	"tools/language-acceptance/final_gate_execution_test.go",
	"tools/language-acceptance/final_gate_native_test.go",
	"tools/language-acceptance/ci_identity_execution.go",
	"tools/language-acceptance/publication_security_execution.go",
	"tools/language-acceptance/final_provenance_execution.go",
	"tools/language-acceptance/native_encoding_execution.go",
	"tools/language-acceptance/native_encoding_intake.go",
	"tools/shortcut-runtime/required-encoding-expectations.json",
	"tests/language-v2/repair-regression-plan.json",
	"tests/language-v2/backend-recovery-gates.json",
}

type finalGateConfig struct{ ExpectedPath, ExpectedSHA256, ManifestPath string }
type finalGateContextLocator struct {
	ExpectedPath string `json:"expectedPath"`
	ManifestPath string `json:"manifestPath"`
}
type finalGateExpected struct {
	Schema                    string                        `json:"schema"`
	ManifestSHA256            string                        `json:"manifestSha256"`
	ImplementationSHA         string                        `json:"implementationSha"`
	IdentityExpectedSHA256    string                        `json:"identityExpectedSha256"`
	PublicationExpectedSHA256 string                        `json:"publicationExpectedSha256,omitempty"`
	PlanOriginal              string                        `json:"planOriginal"`
	Selections                map[string]finalGateSelection `json:"selections"`
	Native                    *finalGateNativeExpected      `json:"native,omitempty"`
}
type finalGateManifest struct {
	Schema      string                   `json:"schema"`
	Identity    finalGateContextLocator  `json:"identity"`
	Publication *finalGateContextLocator `json:"publication,omitempty"`
}
type finalGateObservation struct {
	Schema                 string                          `json:"schema"`
	ImplementationSHA      string                          `json:"implementationSha"`
	ExpectedSHA256         string                          `json:"expectedSha256"`
	ManifestSHA256         string                          `json:"manifestSha256"`
	StartedUTC             string                          `json:"startedUtc"`
	FinishedUTC            string                          `json:"finishedUtc"`
	ElapsedSeconds         float64                         `json:"elapsedSeconds"`
	Identity               *ciIdentityObservation          `json:"identity,omitempty"`
	Publication            *publicationSecurityObservation `json:"publication,omitempty"`
	Closure                *finalGateClosure               `json:"closure,omitempty"`
	Negatives              []finalGateNegative             `json:"negatives,omitempty"`
	Native                 *finalGateNativeObservation     `json:"native,omitempty"`
	Statuses               map[string]string               `json:"statuses"`
	Failures               map[string]string               `json:"failures"`
	GlobalClosureComplete  bool                            `json:"globalClosureComplete"`
	AuthenticationBoundary string                          `json:"authenticationBoundary"`
}

func finalGateTestID(id string) string {
	switch id {
	case "RP49":
		return "tools/language-acceptance:RP49FinalRequiredGateClosure"
	case "BRG31":
		return "tools/language-acceptance:BRG31ActualNativeFinalGateClosure"
	}
	return ""
}
func finalGateTiers(id string) []string {
	if id == "RP49" {
		return []string{"repository", "ci", "skill-package"}
	}
	if id == "BRG31" {
		return []string{"ci", "ios-runtime", "repository", "security"}
	}
	return nil
}
func finalGateScope(id string) string {
	if id == "BRG31" {
		return finalGateBRG31Scope
	}
	return finalGateRP49Scope
}

func finalGateEnvironment() (finalGateConfig, bool, error) {
	config := finalGateConfig{os.Getenv("CHERRI_FINAL_GATES_EXPECTED"), os.Getenv("CHERRI_FINAL_GATES_EXPECTED_SHA256"), os.Getenv("CHERRI_FINAL_GATES_MANIFEST")}
	if config.ExpectedPath == "" && config.ExpectedSHA256 == "" && config.ManifestPath == "" {
		return config, false, nil
	}
	if !filepath.IsAbs(config.ExpectedPath) || !filepath.IsAbs(config.ManifestPath) || !finalProvenanceHex(config.ExpectedSHA256, 64) || filepath.Clean(config.ExpectedPath) == filepath.Clean(config.ManifestPath) {
		return config, true, fmt.Errorf("FINAL_GATE_INCOMPLETE_INDEPENDENT_CONTEXT")
	}
	return config, true, nil
}

func finalGateLoad(config finalGateConfig, source, output string) (*finalProvenanceInputs, ciIdentityExpected, finalGateExpected, finalGateManifest, error) {
	var expected finalGateExpected
	var manifest finalGateManifest
	var identity ciIdentityExpected
	if err := finalProvenanceReadContext(config.ExpectedPath, config.ExpectedSHA256, &expected); err != nil {
		return nil, identity, expected, manifest, err
	}
	if expected.Schema != "cherri-final-required-gates-expected/1" || expected.ImplementationSHA != source || !finalProvenanceHex(source, 40) || !finalProvenanceHex(expected.ManifestSHA256, 64) || !finalProvenanceHex(expected.IdentityExpectedSHA256, 64) || expected.PlanOriginal == "" {
		return nil, identity, expected, manifest, fmt.Errorf("FINAL_GATE_EXPECTED_CONTEXT")
	}
	if err := finalProvenanceReadContext(config.ManifestPath, expected.ManifestSHA256, &manifest); err != nil {
		return nil, identity, expected, manifest, err
	}
	if manifest.Schema != "cherri-final-required-gates-locators/1" || !filepath.IsAbs(manifest.Identity.ExpectedPath) || !filepath.IsAbs(manifest.Identity.ManifestPath) {
		return nil, identity, expected, manifest, fmt.Errorf("FINAL_GATE_LOCATOR_SCHEMA")
	}
	if (manifest.Publication == nil) != (expected.PublicationExpectedSHA256 == "") || (expected.PublicationExpectedSHA256 != "" && !finalProvenanceHex(expected.PublicationExpectedSHA256, 64)) {
		return nil, identity, expected, manifest, fmt.Errorf("FINAL_GATE_PUBLICATION_CONTEXT")
	}
	in, identity, err := ciIdentityLoad(ciIdentityConfig{manifest.Identity.ExpectedPath, expected.IdentityExpectedSHA256, manifest.Identity.ManifestPath}, output)
	if err != nil {
		return nil, identity, expected, manifest, err
	}
	if in.Expected.Sources["implementation"].SHA != source {
		return nil, identity, expected, manifest, fmt.Errorf("FINAL_GATE_IDENTITY_SOURCE")
	}
	return in, identity, expected, manifest, nil
}

func finalGateFailureStatus(err error) string {
	var pending finalProvenancePending
	if errors.As(err, &pending) {
		return "NOT_RUN"
	}
	return "FAILED"
}

func executeFinalRequiredGates(config finalGateConfig, source, output string, records []EvidenceRecord) (observed finalGateObservation, executionErr error) {
	started := time.Now()
	observed = finalGateObservation{Schema: finalGateSchema, ImplementationSHA: source, ExpectedSHA256: config.ExpectedSHA256,
		StartedUTC: started.UTC().Format(time.RFC3339Nano), Statuses: map[string]string{"RP49": "FAILED", "BRG31": "NOT_RUN"}, Failures: map[string]string{},
		AuthenticationBoundary: "Caller independently authenticates and pins the pre-dispatch plan, selected API originals and nested CI contexts. Hashes establish exact integrity; this collector does not turn candidate-controlled files into authentication."}
	ctx, cancel := context.WithTimeout(context.Background(), finalGateTimeout)
	defer cancel()
	defer func() {
		observed.FinishedUTC = time.Now().UTC().Format(time.RFC3339Nano)
		observed.ElapsedSeconds = time.Since(started).Seconds()
		if ctx.Err() != nil || observed.ElapsedSeconds >= finalGateTimeout.Seconds() {
			observed.Statuses["RP49"], observed.Statuses["BRG31"] = "FAILED", "FAILED"
			observed.Failures["RP49"], observed.Failures["BRG31"] = "FINAL_GATE_WHOLE_OBSERVATION_DEADLINE", "FINAL_GATE_WHOLE_OBSERVATION_DEADLINE"
			executionErr = fmt.Errorf("FINAL_GATE_WHOLE_OBSERVATION_DEADLINE")
		}
	}()
	in, identityExpected, expected, manifest, err := finalGateLoad(config, source, output)
	if err == nil {
		observed.ManifestSHA256 = expected.ManifestSHA256
		var closure finalGateClosure
		closure, err = finalGateValidateClosure(ctx, in, expected.PlanOriginal, expected.Selections)
		if err == nil {
			observed.Closure = &closure
		}
		if err == nil {
			err = finalGateValidateIdentitySelection(in, identityExpected, expected.PlanOriginal, expected.Selections)
		}
	}
	if err == nil {
		identity, identityErr := executeCIIdentity(ciIdentityConfig{manifest.Identity.ExpectedPath, expected.IdentityExpectedSHA256, manifest.Identity.ManifestPath}, source, output)
		observed.Identity, err = &identity, identityErr
		if err == nil && identity.Status != "PASSED" {
			err = fmt.Errorf("FINAL_GATE_IDENTITY_NOT_PASSED")
		}
	}
	if err == nil {
		observed.Negatives, err = finalGateNegativeControls(ctx, in, expected.PlanOriginal, expected.Selections)
	}
	if err == nil {
		err = ciIdentityNotBackdated(observed.StartedUTC, observed.Closure.InputsCompletedUTC)
	}
	if err != nil {
		observed.Statuses["RP49"], observed.Statuses["BRG31"] = finalGateFailureStatus(err), finalGateFailureStatus(err)
		observed.Failures["RP49"], observed.Failures["BRG31"] = "FINAL_GATE_CLOSURE_ORIGINALS_REJECTED_OR_INCOMPLETE", "FINAL_GATE_CLOSURE_ORIGINALS_REJECTED_OR_INCOMPLETE"
		return observed, err
	}
	observed.Statuses["RP49"] = "PASSED"
	if manifest.Publication == nil || expected.Native == nil {
		observed.Failures["BRG31"] = "BRG31_DEDICATED_PUBLICATION_OR_NATIVE_ORIGINALS_NOT_CONFIGURED"
		return observed, nil
	}
	pubConfig := publicationSecurityConfig{manifest.Publication.ExpectedPath, expected.PublicationExpectedSHA256, manifest.Publication.ManifestPath}
	publicationInputs, publicationExpected, pubErr := publicationSecurityLoad(pubConfig, output)
	if pubErr == nil && !finalGateSameBindings(publicationInputs.Expected, in.Expected) {
		pubErr = fmt.Errorf("BRG31_PUBLICATION_IDENTITY_CONTEXT_DISAGREES")
	}
	if pubErr == nil {
		pubErr = finalGateSigningSelection(expected, publicationExpected)
	}
	if pubErr == nil {
		publication, currentErr := executePublicationSecurity(pubConfig, source, output, records)
		observed.Publication, pubErr = &publication, currentErr
		if pubErr == nil && publication.Status != "PASSED" {
			pubErr = fmt.Errorf("BRG31_PUBLICATION_NOT_PASSED")
		}
	}
	if pubErr == nil {
		budget := &ciExpandedBudget{Limit: ciIdentityArchiveLimit}
		var archives map[string]map[string][]byte
		archives, pubErr = ciIdentityArchiveInputs(in, identityExpected, budget)
		if pubErr == nil {
			native, nativeErr := finalGateValidateNative(ctx, in, expected, archives)
			observed.Native, pubErr = &native, nativeErr
		}
	}
	if pubErr == nil {
		for _, role := range []string{"implementation", "compiler", "docs"} {
			if _, pubErr = finalProvenanceSnapshotSource(ctx, in.Manifest.SourceDirectories[role], in.Expected.Sources[role]); pubErr != nil {
				break
			}
		}
	}
	// The original BRG31 scenario requires target computed assertions and points
	// to CI02/RP48. CI02 requires reset per test and saved logs/xcresult; RP48
	// requires all required safe fixtures with reset channels and per-case
	// identities. The older five-fixture producer proves only the bounded scope
	// stated in Native.RuntimeCoverageBoundary. Until a complete dedicated cohort
	// is implemented and joined, neither labels nor a green workflow can upgrade it.
	if pubErr == nil && (observed.Native == nil || !observed.Native.FullRuntimeContractVerified) {
		pubErr = finalProvenancePending("BRG31_COMPLETE_COMPUTED_RUNTIME_COHORT_NOT_PRODUCED")
	}
	if pubErr != nil {
		observed.Statuses["BRG31"] = finalGateFailureStatus(pubErr)
		observed.Failures["BRG31"] = "BRG31_NATIVE_OR_SECURITY_ORIGINALS_REJECTED_OR_INCOMPLETE"
		return observed, pubErr
	}
	observed.Statuses["BRG31"] = "PASSED"
	return observed, nil
}

// Root calls this once after CI03/CI04 and the original RP43 producer. It only
// observes retained inputs; no workflow, compiler or native producer is launched.
func (r *Runner) runFinalRequiredGateRequirements(contracts *ContractsSet) {
	config, configured, configErr := finalGateEnvironment()
	set := func(id, status, detail string, levels []string) {
		group, title := "repair", finalGateRP49Title
		if id == "BRG31" {
			group, title = "recovery_gate", finalGateBRG31Title
		}
		r.results[id] = CaseResult{ID: id, Group: group, Requirement: title, TestLevel: strings.Join(finalGateTiers(id), "+"), ExecutedLevels: levels, Passed: status == "PASSED", Status: status, Detail: detail}
	}
	if err := finalGateValidateContracts(contracts); err != nil {
		for _, id := range []string{"RP49", "BRG31"} {
			set(id, "FAILED", "Dedicated final gate intake differs from immutable requirement contracts", nil)
		}
		return
	}
	if !configured || configErr != nil {
		status, detail := "NOT_RUN", "Dedicated pre-dispatch final gate context is not configured"
		if configErr != nil {
			status, detail = "FAILED", "Dedicated final gate caller context is malformed"
		}
		for _, id := range []string{"RP49", "BRG31"} {
			set(id, status, detail, nil)
		}
		return
	}
	directory, err := stateEvidenceDirectory(r, "final-required-gates-")
	if err != nil {
		for _, id := range []string{"RP49", "BRG31"} {
			set(id, "FAILED", "Dedicated final gate evidence directory is unavailable", nil)
		}
		return
	}
	observed, _ := executeFinalRequiredGates(config, r.implSHA, directory, r.records)
	raw, err := json.MarshalIndent(observed, "", "  ")
	name := filepath.Join(directory, "final-required-gates.original.json")
	if err == nil {
		raw = append(raw, '\n')
		err = finalProvenanceWriteNew(name, observed)
	}
	if err != nil {
		for _, id := range []string{"RP49", "BRG31"} {
			set(id, "FAILED", "Dedicated original observation could not be retained", nil)
		}
		return
	}
	for _, id := range []string{"RP49", "BRG31"} {
		status := observed.Statuses[id]
		levels := []string(nil)
		if status == "PASSED" {
			levels = finalGateTiers(id)
		}
		detail := finalGateScope(id)
		if observed.Failures[id] != "" {
			detail = observed.Failures[id]
		}
		set(id, status, detail, levels)
		if status != "PASSED" {
			continue
		}
		for _, tier := range levels {
			r.records = append(r.records, EvidenceRecord{EvidenceID: "ev-final-required-" + id + "-" + tier, Requirements: []string{id}, Tier: tier, ImplementationSHA: r.implSHA, TestID: finalGateTestID(id), Command: []string{"internal", finalGateSchema, id}, ExitCode: 0, Result: "passed",
				Assertions: map[string]string{"scope": finalGateScope(id), "expected_context_sha256": config.ExpectedSHA256, "manifest_sha256": observed.ManifestSHA256, "global_closure_complete": "false", "selected_required_gates_successful": "true", "predeclared_selection": "true", "stale_and_missing_candidates_rejected": "true"},
				Artifacts:  []EvidenceArtifact{{Path: name, SHA256: ciIdentityDigest(raw)}}, Detail: finalGateScope(id)})
		}
	}
}

func finalGateCompareObservations(original, current finalGateObservation) error {
	if original.Closure == nil || current.Closure == nil || ciIdentityNotBackdated(original.StartedUTC, current.Closure.InputsCompletedUTC) != nil {
		return fmt.Errorf("FINAL_GATE_ORIGINAL_CHRONOLOGY")
	}
	original.StartedUTC, original.FinishedUTC, original.ElapsedSeconds = current.StartedUTC, current.FinishedUTC, current.ElapsedSeconds
	if original.Identity == nil || current.Identity == nil || ciIdentityCompareObservation(*original.Identity, *current.Identity) != nil {
		return fmt.Errorf("FINAL_GATE_IDENTITY_REVALIDATION")
	}
	original.Identity = current.Identity
	if (original.Publication == nil) != (current.Publication == nil) {
		return fmt.Errorf("FINAL_GATE_PUBLICATION_REVALIDATION")
	}
	if original.Publication != nil {
		if publicationCompareObservation(*original.Publication, *current.Publication) != nil {
			return fmt.Errorf("FINAL_GATE_PUBLICATION_REVALIDATION")
		}
		original.Publication = current.Publication
	}
	if !reflect.DeepEqual(original, current) {
		return fmt.Errorf("FINAL_GATE_OBSERVATION_REVALIDATION")
	}
	return nil
}

func validateFinalRequiredGateRecord(id string, record EvidenceRecord) error {
	if finalGateTestID(id) == "" {
		if record.TestID == finalGateTestID("RP49") || record.TestID == finalGateTestID("BRG31") {
			return fmt.Errorf("FINAL_GATE_RELATED_REQUIREMENT_NOT_AUTHORIZED")
		}
		return nil
	}
	allowedTier := false
	for _, tier := range finalGateTiers(id) {
		if record.Tier == tier {
			allowedTier = true
		}
	}
	if !allowedTier || record.TestID != finalGateTestID(id) || !reflect.DeepEqual(record.Requirements, []string{id}) || record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Command, []string{"internal", finalGateSchema, id}) || record.Assertions["scope"] != finalGateScope(id) || record.Assertions["global_closure_complete"] != "false" || record.Assertions["selected_required_gates_successful"] != "true" || record.Assertions["predeclared_selection"] != "true" || record.Assertions["stale_and_missing_candidates_rejected"] != "true" {
		return fmt.Errorf("FINAL_GATE_RECORD_NOT_DEDICATED")
	}
	contracts, contractErr := LoadAllContracts("", "", "")
	if contractErr != nil || finalGateValidateContracts(contracts) != nil {
		return fmt.Errorf("FINAL_GATE_IMMUTABLE_CONTRACT")
	}
	config, configured, err := finalGateEnvironment()
	if err != nil || !configured || config.ExpectedSHA256 != record.Assertions["expected_context_sha256"] {
		return fmt.Errorf("FINAL_GATE_INDEPENDENT_CONTEXT_UNAVAILABLE")
	}
	data, err := stateReadArtifacts(record.Artifacts, []string{"final-required-gates.original.json"})
	if err != nil {
		return err
	}
	var original finalGateObservation
	if decodeEditorJSON(data["final-required-gates.original.json"], &original) != nil || original.Schema != finalGateSchema || original.ImplementationSHA != record.ImplementationSHA || original.ExpectedSHA256 != config.ExpectedSHA256 || original.ManifestSHA256 != record.Assertions["manifest_sha256"] || original.GlobalClosureComplete || original.Statuses[id] != "PASSED" || original.Failures[id] != "" {
		return fmt.Errorf("FINAL_GATE_ORIGINAL_OBSERVATION")
	}
	if err = stateTiming(original.StartedUTC, original.FinishedUTC, original.ElapsedSeconds, finalGateTimeout.Seconds()); err != nil {
		return err
	}
	var components []EvidenceRecord
	if original.Publication != nil && original.Publication.RP43 != nil {
		components = append(components, *original.Publication.RP43)
	}
	current, _ := executeFinalRequiredGates(config, record.ImplementationSHA, "", components)
	if current.Statuses[id] != "PASSED" {
		return fmt.Errorf("FINAL_GATE_CURRENT_ORIGINALS_REJECTED")
	}
	return finalGateCompareObservations(original, current)
}

func applyFinalRequiredGateAudit(row *ExecutionAuditRow, runner *Runner) {
	if finalGateTestID(row.ID) == "" {
		return
	}
	row.TestNames = []string{finalGateTestID(row.ID)}
	row.ProductionPath = append([]string{}, finalGateSources...)
	row.Scope = finalGateScope(row.ID)
	row.PositiveAndNegative = runner.results[row.ID].Status == "PASSED"
	row.Blocker = ""
	if runner.results[row.ID].Status != "PASSED" {
		row.Blocker = runner.results[row.ID].Detail
	}
}
