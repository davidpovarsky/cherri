package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type LocalSmokeObservation struct {
	ID            string   `json:"id"`
	Passed        bool     `json:"passed"`
	ClaimedLevels []string `json:"historically_claimed_levels"`
	Detail        string   `json:"observed_smoke_detail"`
	CoverageGap   string   `json:"coverage_gap"`
}

// Historical probes are retained as observations, never converted into proof
// of a larger contract. Remove an entry only with a reviewed scenario producer.
var incompleteLocalAssertions = map[string]string{
	"B01": "A historical ledger substring does not verify the current base, remote HEAD, or unrelated worktrees.",
	"T05": "Actual presence/false/zero/empty native-field controls and authored-null rejection pass locally; supported lossless native JSON null and all optional-container semantics remain unproved.",
	"L01": "Nonempty lowering does not assert nested shape, cardinality, or order.",
	"N03": "Nonempty IR does not prove case-sensitive equality or comparison migration.",
	"F04": "A repeat-end UUID does not prove one semantic collected element per iteration.",

	"CI01":  "Local process fixtures do not execute WorkflowKit; source-bound real macOS correct/corrupt/restored REQUIRED encoding evidence is required.",
	"CI02":  "Nonempty IR for a runtime fixture is not an iOS runtime execution.",
	"CI03":  "A 40-character local SHA does not prove final workflow head/checkout provenance.",
	"CI04":  "Absence of private_key.pem is not a private-corpus, credentials, or signing audit.",
	"END01": "Successful git status does not establish a clean tree, pushed branches, artifacts, or delivery provenance.",
}

func (r *Runner) recordIncompleteScope(id, group, req, level string, levels []string, passed bool, detail string) bool {
	gap, incomplete := incompleteLocalAssertions[id]
	if !incomplete {
		return false
	}
	r.observations = append(r.observations, LocalSmokeObservation{ID: id, Passed: passed, ClaimedLevels: append([]string{}, levels...), Detail: detail, CoverageGap: gap})
	status := "NOT_RUN"
	if !passed {
		status = "FAILED"
	}
	r.results[id] = CaseResult{
		ID: id, Group: group, Requirement: req, TestLevel: level,
		Passed: false, Status: status, ExecutedLevels: nil,
		Detail: fmt.Sprintf("Formal scope UNVERIFIED: %s Observed smoke passed=%t; see acceptance-local-observations.json.", gap, passed),
	}
	return true
}

func writeLocalObservations(runner *Runner, outPath string) error {
	data, err := json.MarshalIndent(map[string]any{
		"schema_version": "1", "source_sha": runner.implSHA,
		"scope":        "Historical smoke assertions only; not formal acceptance tier evidence",
		"observations": runner.observations,
	}, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(outPath, append(data, '\n'), 0644)
}
