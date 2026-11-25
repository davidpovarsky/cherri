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

const baselineSchema = "cherri-owned-source-baseline/1"
const baselineTestID = "tools/language-acceptance:B01OwnedSourceBaseline"
const baselineScope = "Recorded base ancestry, clean owned ordinary source clone, and actual publication preserving unrelated refs; no BRG01 takeover/backup, native, CI, or global closure credit"
const baselineTimeout = 120 * time.Second

var baselineSources = []string{
	"tools/language-acceptance/baseline_execution.go",
	"tools/language-acceptance/baseline_execution_test.go",
	"tools/language-acceptance/final_provenance_execution.go",
	"tools/language-acceptance/main.go", "tools/language-acceptance/evidence_map.go",
	"tools/language-acceptance/execution_audit.go", "tools/language-acceptance/assertion_scope.go",
	"tests/language-v2/acceptance-contract.json",
}

type baselineExpected struct {
	Schema         string                         `json:"schema"`
	ManifestSHA256 string                         `json:"manifestSha256"`
	BaseSHA        string                         `json:"baseSha"`
	Source         finalProvenanceSource          `json:"source"`
	Publication    finalProvenancePublication     `json:"publication"`
	Files          map[string]finalProvenanceFile `json:"files"`
}
type baselineManifest struct {
	Schema          string            `json:"schema"`
	SourceDirectory string            `json:"sourceDirectory"`
	Files           map[string]string `json:"files"`
}
type baselineObservation struct {
	Schema            string                      `json:"schema"`
	ImplementationSHA string                      `json:"implementationSha"`
	BaseSHA           string                      `json:"baseSha"`
	ExpectedSHA256    string                      `json:"expectedSha256"`
	ManifestSHA256    string                      `json:"manifestSha256"`
	PublicationSHA256 string                      `json:"publicationSha256"`
	StartedUTC        string                      `json:"startedUtc"`
	FinishedUTC       string                      `json:"finishedUtc"`
	ElapsedSeconds    float64                     `json:"elapsedSeconds"`
	Before            finalProvenanceSnapshot     `json:"sourceBefore"`
	After             finalProvenanceSnapshot     `json:"sourceAfter"`
	Commands          []finalProvenanceGitCommand `json:"commands"`
	Status            string                      `json:"status"`
	Scope             string                      `json:"scope"`
	ClosureComplete   bool                        `json:"closureComplete"`
	Failure           string                      `json:"failure,omitempty"`
}

func baselineEnvironment() (finalProvenanceConfig, bool, error) {
	c := finalProvenanceConfig{os.Getenv("CHERRI_BASELINE_EXPECTED"), os.Getenv("CHERRI_BASELINE_EXPECTED_SHA256"), os.Getenv("CHERRI_BASELINE_MANIFEST")}
	if c.ExpectedPath == "" && c.ExpectedSHA256 == "" && c.ManifestPath == "" {
		return c, false, nil
	}
	if !filepath.IsAbs(c.ExpectedPath) || !filepath.IsAbs(c.ManifestPath) ||
		!finalProvenanceHex(c.ExpectedSHA256, 64) || filepath.Clean(c.ExpectedPath) == filepath.Clean(c.ManifestPath) {
		return c, true, fmt.Errorf("B01_INCOMPLETE_INDEPENDENT_CONTEXT")
	}
	return c, true, nil
}

func baselineLoad(config finalProvenanceConfig, output string) (*finalProvenanceInputs, baselineExpected, baselineManifest, error) {
	var expected baselineExpected
	var manifest baselineManifest
	if err := finalProvenanceReadContext(config.ExpectedPath, config.ExpectedSHA256, &expected); err != nil {
		return nil, expected, manifest, err
	}
	s := expected.Source
	if expected.Schema != "cherri-baseline-expected/1" || !finalProvenanceHex(expected.ManifestSHA256, 64) ||
		!finalProvenanceHex(expected.BaseSHA, 40) || expected.BaseSHA == s.SHA ||
		s.Repository != "davidpovarsky/cherri" || !finalProvenanceHex(s.SHA, 40) || !finalProvenanceHex(s.Tree, 40) ||
		!strings.HasPrefix(s.LocalRef, "refs/heads/agent/") ||
		!strings.HasPrefix(s.PublishedRef, "refs/heads/agent/") ||
		expected.Publication.Format != "oracle-atomic-create-only/1" ||
		expected.Publication.PublisherRef != s.LocalRef || len(expected.Files) != 64 {
		return nil, expected, manifest, fmt.Errorf("B01_EXPECTED_SOURCE_OR_PUBLICATION_SCOPE")
	}
	if err := finalProvenanceReadContext(config.ManifestPath, expected.ManifestSHA256, &manifest); err != nil {
		return nil, expected, manifest, err
	}
	if manifest.Schema != "cherri-baseline-locators/1" || !filepath.IsAbs(manifest.SourceDirectory) || len(manifest.Files) != len(expected.Files) {
		return nil, expected, manifest, fmt.Errorf("B01_LOCATOR_INVENTORY")
	}
	resolved, err := filepath.EvalSymlinks(manifest.SourceDirectory)
	if err != nil {
		return nil, expected, manifest, finalProvenancePending("B01_SOURCE_UNAVAILABLE")
	}
	if resolved != filepath.Clean(manifest.SourceDirectory) {
		return nil, expected, manifest, fmt.Errorf("B01_SOURCE_REDIRECTED")
	}
	in := &finalProvenanceInputs{
		Expected: finalProvenanceExpected{Sources: map[string]finalProvenanceSource{"implementation": s}, Publications: map[string]finalProvenancePublication{"implementation": expected.Publication}, Files: expected.Files},
		Manifest: finalProvenanceManifest{SourceDirectories: map[string]string{"implementation": manifest.SourceDirectory}, Files: manifest.Files},
		JSON:     map[string]map[string]any{}, Bytes: map[string][]byte{},
	}
	var total int64
	for id, pin := range expected.Files {
		path := manifest.Files[id]
		resolvedOriginal, err := filepath.EvalSymlinks(path)
		if os.IsNotExist(err) {
			return nil, expected, manifest, finalProvenancePending("B01_REQUIRED_ORIGINAL_UNAVAILABLE")
		}
		if err != nil || !filepath.IsAbs(path) || finalProvenanceWithin(resolved, resolvedOriginal) ||
			(output != "" && finalProvenanceWithin(output, resolvedOriginal)) {
			return nil, expected, manifest, fmt.Errorf("B01_ORIGINAL_LOCATION")
		}
		total += pin.Bytes
		if total > 64*1024*1024 {
			return nil, expected, manifest, fmt.Errorf("B01_ORIGINAL_TOTAL_LIMIT")
		}
		b, err := finalProvenanceRead(path, pin, true)
		if err != nil {
			return nil, expected, manifest, err
		}
		in.Bytes[id] = b
	}
	return in, expected, manifest, nil
}

func executeOwnedBaseline(config finalProvenanceConfig, implementationSHA, output string) (observation baselineObservation, outcome error) {
	start := time.Now()
	observation = baselineObservation{Schema: baselineSchema, ImplementationSHA: implementationSHA, ExpectedSHA256: config.ExpectedSHA256,
		StartedUTC: start.UTC().Format(time.RFC3339Nano), Status: "FAILED", Scope: baselineScope}
	defer func() {
		observation.FinishedUTC = time.Now().UTC().Format(time.RFC3339Nano)
		observation.ElapsedSeconds = time.Since(start).Seconds()
		if outcome != nil {
			var pending finalProvenancePending
			if errors.As(outcome, &pending) {
				observation.Status = "NOT_RUN"
			}
			observation.Failure = "B01_ORIGINALS_OR_CURRENT_SOURCE_REJECTED"
		}
	}()
	in, expected, manifest, err := baselineLoad(config, output)
	if err != nil {
		return observation, err
	}
	observation.BaseSHA, observation.ManifestSHA256 = expected.BaseSHA, expected.ManifestSHA256
	observation.PublicationSHA256 = expected.Files[expected.Publication.Result].SHA256
	if expected.Source.SHA != implementationSHA {
		return observation, fmt.Errorf("B01_DIFFERENT_IMPLEMENTATION")
	}
	ctx, cancel := context.WithTimeout(context.Background(), baselineTimeout)
	defer cancel()
	root := manifest.SourceDirectory
	gitDirectory := filepath.Join(root, ".git")
	info, err := os.Lstat(gitDirectory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return observation, fmt.Errorf("B01_REQUIRES_OWNED_ORDINARY_CLONE")
	}
	for _, p := range []string{"objects/info/alternates", "info/grafts"} {
		_, err := os.Lstat(filepath.Join(gitDirectory, filepath.FromSlash(p)))
		if !os.IsNotExist(err) {
			return observation, fmt.Errorf("B01_ALTERNATES_OR_GRAFTS")
		}
	}
	if observation.Before, err = finalProvenanceSnapshotSource(ctx, root, expected.Source); err != nil {
		return observation, err
	}
	for _, query := range [][]string{{"worktree", "list", "--porcelain"}, {"merge-base", "--is-ancestor", expected.BaseSHA, implementationSHA}} {
		b, receipt, err := finalProvenanceGit(ctx, root, query...)
		observation.Commands = append(observation.Commands, receipt)
		if err != nil {
			return observation, fmt.Errorf("B01_OWNERSHIP_OR_BASE_ANCESTRY")
		}
		want := ""
		if query[0] == "worktree" {
			want = "worktree " + root + "\nHEAD " + implementationSHA + "\nbranch " + expected.Source.LocalRef + "\n\n"
		}
		if string(b) != want {
			return observation, fmt.Errorf("B01_EXTRA_OR_DIFFERENT_WORKTREE")
		}
	}
	if err = finalProvenanceValidatePublication(in, "implementation"); err != nil {
		return observation, err
	}
	publication, err := in.object(expected.Publication.Result)
	if err != nil {
		return observation, err
	}
	finished, err := finalProvenanceTime(finalProvenanceString(publication, "finishedAt"))
	if err != nil || finished.After(start) {
		return observation, fmt.Errorf("B01_PUBLICATION_NOT_TERMINAL_BEFORE_OBSERVATION")
	}
	if observation.After, err = finalProvenanceSnapshotSource(ctx, root, expected.Source); err != nil {
		return observation, err
	}
	if !reflect.DeepEqual(observation.Before, observation.After) {
		return observation, fmt.Errorf("B01_SOURCE_CHANGED_DURING_INTAKE")
	}
	if _, _, _, err = baselineLoad(config, output); err != nil {
		return observation, err
	}
	if ctx.Err() != nil {
		return observation, fmt.Errorf("B01_OBSERVATION_TIMEOUT")
	}
	observation.Status = "PASSED"
	return observation, nil
}

func (r *Runner) runOwnedBaselineRequirement() {
	r.baselineEvaluated = true
	config, configured, configErr := baselineEnvironment()
	result := CaseResult{ID: "B01", Group: "baseline", Requirement: "Verify isolated base/remote HEAD and keep unrelated agent work unchanged", TestLevel: "repository", Status: "NOT_RUN", Detail: "B01 independently selected base, owned source and original publication context is not configured"}
	if !configured {
		r.results["B01"] = result
		return
	}
	if configErr != nil {
		result.Status = "FAILED"
		result.Detail = "B01 independent context is malformed"
		r.results["B01"] = result
		return
	}
	directory, err := stateEvidenceDirectory(r, "baseline-")
	if err != nil {
		result.Status = "FAILED"
		result.Detail = "B01 evidence directory unavailable"
		r.results["B01"] = result
		return
	}
	observed, executionErr := executeOwnedBaseline(config, r.implSHA, directory)
	encoded, err := json.MarshalIndent(observed, "", "  ")
	name := filepath.Join(directory, "baseline-observation.json")
	if err == nil {
		err = os.WriteFile(name, append(encoded, '\n'), 0600)
	}
	if err != nil {
		observed.Status = "FAILED"
		executionErr = err
	}
	result.Status, result.Detail = observed.Status, baselineScope
	result.Passed = observed.Status == "PASSED" && executionErr == nil
	if observed.Status != "NOT_RUN" {
		result.ExecutedLevels = []string{"repository"}
	}
	r.results["B01"] = result
	if !result.Passed {
		return
	}
	r.records = append(r.records, EvidenceRecord{EvidenceID: "ev-owned-source-baseline-B01", Requirements: []string{"B01"}, Tier: "repository",
		ImplementationSHA: r.implSHA, TestID: baselineTestID, Command: []string{"internal", baselineSchema}, ExitCode: 0, Result: "passed",
		Assertions: map[string]string{"scope": baselineScope, "expected_context_sha256": config.ExpectedSHA256, "manifest_sha256": observed.ManifestSHA256,
			"recorded_base_sha": observed.BaseSHA, "publication_sha256": observed.PublicationSHA256, "closure_complete": "false"},
		Artifacts: []EvidenceArtifact{{Path: name, SHA256: finalProvenanceDigest(append(encoded, '\n'))}}, Detail: baselineScope})
}

func validateOwnedBaselineRecord(id string, record EvidenceRecord) error {
	if id != "B01" {
		if record.TestID == baselineTestID {
			return fmt.Errorf("B01_DOES_NOT_GRANT_ANOTHER_REQUIREMENT")
		}
		return nil
	}
	if record.TestID != baselineTestID || !reflect.DeepEqual(record.Requirements, []string{"B01"}) ||
		record.Tier != "repository" || record.Result != "passed" || record.ExitCode != 0 ||
		!reflect.DeepEqual(record.Command, []string{"internal", baselineSchema}) ||
		record.Assertions["scope"] != baselineScope || record.Assertions["closure_complete"] != "false" {
		return fmt.Errorf("B01_RECORD_NOT_DEDICATED_BASELINE")
	}
	config, configured, err := baselineEnvironment()
	if err != nil || !configured || record.Assertions["expected_context_sha256"] != config.ExpectedSHA256 {
		return fmt.Errorf("B01_INDEPENDENT_CONTEXT_UNAVAILABLE")
	}
	data, err := stateReadArtifacts(record.Artifacts, []string{"baseline-observation.json"})
	if err != nil {
		return err
	}
	var original baselineObservation
	if err = decodeEditorJSON(data["baseline-observation.json"], &original); err != nil {
		return err
	}
	if original.Schema != baselineSchema || original.Status != "PASSED" || original.ClosureComplete || original.Scope != baselineScope ||
		original.ImplementationSHA != record.ImplementationSHA || original.ExpectedSHA256 != config.ExpectedSHA256 ||
		original.ManifestSHA256 != record.Assertions["manifest_sha256"] || original.BaseSHA != record.Assertions["recorded_base_sha"] ||
		original.PublicationSHA256 != record.Assertions["publication_sha256"] || original.Failure != "" {
		return fmt.Errorf("B01_ORIGINAL_OBSERVATION")
	}
	if err = stateTiming(original.StartedUTC, original.FinishedUTC, original.ElapsedSeconds, baselineTimeout.Seconds()); err != nil {
		return err
	}
	revalidated, err := executeOwnedBaseline(config, record.ImplementationSHA, "")
	if err != nil || revalidated.Status != "PASSED" {
		return fmt.Errorf("B01_ORIGINAL_REVALIDATION")
	}
	original.StartedUTC, original.FinishedUTC, original.ElapsedSeconds = revalidated.StartedUTC, revalidated.FinishedUTC, revalidated.ElapsedSeconds
	if !reflect.DeepEqual(original, revalidated) {
		return fmt.Errorf("B01_ORIGINAL_OBSERVATION_CHANGED")
	}
	return nil
}

func applyOwnedBaselineAudit(row *ExecutionAuditRow, runner *Runner) {
	if row.ID != "B01" || !runner.baselineEvaluated {
		return
	}
	row.TestNames, row.ProductionPath = []string{baselineTestID}, append([]string{}, baselineSources...)
	row.Scope, row.PositiveAndNegative, row.Blocker = baselineScope, false, ""
	if result := runner.results["B01"]; result.Status != "PASSED" {
		row.Blocker = result.Detail
	}
}
