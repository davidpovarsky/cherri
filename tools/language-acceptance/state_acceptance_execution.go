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
	"runtime"
	"strings"
	"time"
)

type stateAcceptanceScenario struct {
	Group, Title string
	Tiers        []string
}

var stateAcceptanceScenarios = map[string]stateAcceptanceScenario{
	"BRG06": {Group: "recovery_gate", Title: "Backend state and error isolation", Tiers: []string{"unit", "integration", "security"}},
	"RP51":  {Group: "repair", Title: "Performance and race evidence", Tiers: []string{"benchmark", "race"}},
}

type stateRaceExecution struct {
	Available bool
	Artifacts []EvidenceArtifact
	Detail    string
	Err       error
}

func stateAcceptanceTests(id string) []string {
	if _, found := stateAcceptanceScenarios[id]; !found {
		return nil
	}
	tests := append([]string{}, stateRaceTests...)
	if id == "BRG06" {
		return append(tests, stateSecurityTest)
	}
	return append(tests, editorBenchmarkTest)
}

func stateAcceptanceTestIDs(id string) []string {
	var ids []string
	for _, name := range stateAcceptanceTests(id) {
		ids = append(ids, "github.com/electrikmilk/cherri:"+name)
	}
	return ids
}

func stateAcceptanceSourcePaths(id string) []string {
	paths := append([]string{}, stateRaceSourcePaths...)
	if id == "BRG06" {
		return stateSecuritySourcePaths()
	}
	for _, path := range editorProductionPathsForID("RP51") {
		paths = appendUniqueAudit(paths, path)
	}
	return paths
}

func stateEvidenceDirectory(r *Runner, prefix string) (string, error) {
	parent := r.artifactDir
	if parent != "" {
		var err error
		parent, err = filepath.Abs(parent)
		if err != nil {
			return "", err
		}
		if err = os.MkdirAll(parent, 0755); err != nil {
			return "", err
		}
	}
	return os.MkdirTemp(parent, prefix)
}

func stateRaceRecord(id, tier, source string, artifacts []EvidenceArtifact) EvidenceRecord {
	producer := stateRaceTests[0]
	if tier == "unit" {
		producer = stateRaceTests[1]
	}
	record := EvidenceRecord{EvidenceID: "ev-local-" + id + "-" + tier, Requirements: []string{id}, Tier: tier, ImplementationSHA: source,
		TestID: "github.com/electrikmilk/cherri:" + producer, Command: append([]string{}, stateRaceCommand...), ExitCode: 0, Result: "passed",
		Assertions: map[string]string{"required_production_tests": strings.Join(stateRaceTests, "\n"), "scope": stateRaceScope}, Artifacts: append([]EvidenceArtifact{}, artifacts...)}
	for _, artifact := range artifacts {
		if filepath.Base(artifact.Path) == "race-receipt.json" {
			raw, err := os.ReadFile(artifact.Path)
			if err == nil {
				var receipt stateRaceReceipt
				if json.Unmarshal(raw, &receipt) == nil {
					record.Assertions["race_run_id"] = receipt.RunID
					record.Assertions["race_run_attempt"] = receipt.RunAttempt
				}
			}
		}
	}
	return record
}

func (r *Runner) executeStateRace() *stateRaceExecution {
	if r.stateRace != nil {
		return r.stateRace
	}
	result := &stateRaceExecution{}
	r.stateRace = result
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		result.Available = true
		result.Err = err
		return result
	}
	supplied := os.Getenv("CHERRI_STATE_RACE_EVIDENCE_DIR")
	if supplied == "" && (runtime.GOOS != "linux" || os.Getenv("GITHUB_REPOSITORY") != "davidpovarsky/cherri" || os.Getenv("GITHUB_SHA") != r.implSHA) {
		result.Detail = "No current-source Linux CI race bundle is configured; a non-race run or historical archive cannot supply this scenario"
		return result
	}
	result.Available = true
	before, clean, err := editorSourceState(root)
	if err != nil || !clean || before != r.implSHA {
		result.Err = fmt.Errorf("state/race producer requires exact clean source: %s clean=%t error=%v", before, clean, err)
		return result
	}
	if supplied == "" {
		python, pythonErr := acceptancePython3("CHERRI_STATE_PYTHON")
		if pythonErr != nil {
			result.Err = pythonErr
			return result
		}
		temporary, tempErr := os.MkdirTemp("", "cherri-state-production-")
		if tempErr != nil {
			result.Err = tempErr
			return result
		}
		defer os.RemoveAll(temporary)
		supplied = filepath.Join(temporary, "race")
		ctx, cancel := context.WithTimeout(context.Background(), 11*time.Minute)
		defer cancel()
		command := exec.CommandContext(ctx, python, "-I", "-B", filepath.Join(root, "tools", "language-acceptance", "run_state_race.py"), "--output", supplied)
		command.Dir = root
		command.WaitDelay = 3 * time.Second
		var out, errors bytes.Buffer
		command.Stdout = &out
		command.Stderr = &errors
		processErr := command.Run()
		destination, destErr := stateEvidenceDirectory(r, "state-race-")
		if destErr != nil {
			result.Err = destErr
			return result
		}
		// Preserve the original producer bytes, including failure receipts, before
		// classifying success. The adapter never rewrites run/source identities.
		for _, name := range stateRaceArtifactNames {
			data, readErr := os.ReadFile(filepath.Join(supplied, name))
			if readErr != nil {
				continue
			}
			path := filepath.Join(destination, name)
			if writeErr := os.WriteFile(path, data, 0600); writeErr != nil {
				result.Err = writeErr
				return result
			}
		}
		if err = os.WriteFile(filepath.Join(destination, "runner.stdout.log"), out.Bytes(), 0600); err != nil {
			result.Err = err
			return result
		}
		if err = os.WriteFile(filepath.Join(destination, "runner.stderr.log"), errors.Bytes(), 0600); err != nil {
			result.Err = err
			return result
		}
		supplied = destination
		if processErr != nil {
			result.Err = fmt.Errorf("actual state/race runner failed: %v (context %v); original logs retained at %s", processErr, ctx.Err(), destination)
		}
	}
	for _, name := range stateRaceArtifactNames {
		path := filepath.Join(supplied, name)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			if result.Err == nil {
				result.Err = readErr
			}
			continue
		}
		result.Artifacts = append(result.Artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(data)})
	}
	if result.Err == nil {
		result.Err = validateStateRaceRecord("RP51", stateRaceRecord("RP51", "race", r.implSHA, result.Artifacts))
	}
	after, afterClean, stateErr := editorSourceState(root)
	if stateErr != nil || !afterClean || after != before {
		result.Err = fmt.Errorf("state/race intake source changed during execution")
	}
	result.Detail = "Validated actual Linux CGO race process with six exact named passes, 64 mixed and 32 cold-import cases, error/cancellation/panic recovery and immutable source files"
	return result
}

func stateSecurityRecord(source string, artifacts []EvidenceArtifact) EvidenceRecord {
	return EvidenceRecord{EvidenceID: "ev-local-BRG06-security", Requirements: []string{"BRG06"}, Tier: "security", ImplementationSHA: source, TestID: "github.com/electrikmilk/cherri:" + stateSecurityTest,
		Command: productionTestCommand([]string{stateSecurityTest}), ExitCode: 0, Result: "passed", Artifacts: artifacts,
		Assertions: map[string]string{"required_production_tests": stateSecurityTest, "scope": stateSecurityScope}}
}

func (r *Runner) runBackendStateRequirement(id, title string, levels []string) bool {
	scenario, found := stateAcceptanceScenarios[id]
	if !found || title != scenario.Title || !reflect.DeepEqual(parseRequiredTiers(strings.Join(levels, "+")), scenario.Tiers) {
		return false
	}
	var executed, details []string
	failed := false
	appendRecord := func(record EvidenceRecord, detail string, err error) {
		executed = append(executed, record.Tier)
		record.Detail = detail
		if err != nil {
			failed = true
			record.ExitCode = 1
			record.Result = "failed"
			record.Detail = err.Error()
		}
		record.Assertions["assertion"] = record.Detail
		r.records = append(r.records, record)
		details = append(details, record.Detail)
	}
	if id == "RP51" {
		artifacts, detail, err := r.executeEditorProduction(id)
		record := EvidenceRecord{EvidenceID: "ev-local-RP51-benchmark", Requirements: []string{id}, Tier: "benchmark", ImplementationSHA: r.implSHA, TestID: "github.com/electrikmilk/cherri:" + editorBenchmarkTest,
			Command: productionTestCommand(editorProductionTests[id]), ExitCode: 0, Result: "passed", Artifacts: artifacts,
			Assertions: map[string]string{"required_production_tests": strings.Join(editorProductionTests[id], "\n"), "scope": editorProductionScope(id)}}
		if err == nil {
			err = validateEditorPerformanceRecord(id, record)
		}
		appendRecord(record, detail, err)
	}
	race := r.executeStateRace()
	if race.Available {
		tiers := []string{"race"}
		if id == "BRG06" {
			tiers = []string{"unit", "integration"}
		}
		for _, tier := range tiers {
			record := stateRaceRecord(id, tier, r.implSHA, race.Artifacts)
			err := race.Err
			if err == nil {
				err = validateStateRaceRecord(id, record)
			}
			appendRecord(record, race.Detail, err)
		}
	} else {
		details = append(details, race.Detail)
	}
	if id == "BRG06" {
		artifacts, detail, err := r.executeStateSecurity()
		record := stateSecurityRecord(r.implSHA, artifacts)
		if err == nil {
			err = validateStateSecurityRecord(record)
		}
		appendRecord(record, detail, err)
	}
	status := "PASSED"
	passed := !failed && len(executed) == len(scenario.Tiers)
	if failed {
		status = "FAILED"
	} else if !passed {
		status = "NOT_RUN"
	}
	r.results[id] = CaseResult{ID: id, Group: scenario.Group, Requirement: title, TestLevel: strings.Join(scenario.Tiers, "+"), ExecutedLevels: executed, Passed: passed, Status: status, Detail: strings.Join(details, "; ")}
	return true
}
