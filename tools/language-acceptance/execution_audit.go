package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var f01RequiredTests = []string{
	"TestP15F01StatementAndExpressionGuardRHS",
	"TestP15F01StaticSkipsMissingTarget",
	"TestP15F01BooleanStatementUsesValueNotPresence",
	"TestP15F01GuardOracleRejectsMutations",
	"TestP15F01RuntimeFixtureCompiles",
}

// The acceptance runner executes the production compiler regressions, and
// verifies discovery. A successful process with no matching tests is not proof.
func executeF01ProductionTests() (bool, string) {
	var ctx, cancel = context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	var cmd = exec.CommandContext(ctx, "go", "test", "-count=1", "-json", "-timeout", "3m", ".", "-run", "^TestP15F01")
	cmd.Dir = resolveRepoRoot()
	var output, err = cmd.CombinedOutput()
	var seen = make(map[string]bool)
	var scanner = bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		var event struct {
			Action string
			Test   string
		}
		if json.Unmarshal(scanner.Bytes(), &event) == nil && event.Action == "pass" {
			seen[event.Test] = true
		}
	}
	var missing []string
	for _, name := range f01RequiredTests {
		if !seen[name] {
			missing = append(missing, name)
		}
	}
	if err != nil || scanner.Err() != nil || len(missing) != 0 {
		return false, fmt.Sprintf("Production F01 tests failed or were not discovered: process=%v; missing=%v; scanner=%v", err, missing, scanner.Err())
	}
	return true, fmt.Sprintf("%d production compiler tests passed: decoded native guard placement, once-only RHS, false/true predicates, static missing-target elimination, negative wire mutations; signed runtime still required", len(f01RequiredTests))
}

type ExecutionAuditRow struct {
	ID                  string   `json:"id"`
	Requirement         string   `json:"requirement"`
	Status              string   `json:"status"`
	Tier                []string `json:"tier"`
	RequiredTiers       []string `json:"required_tiers"`
	ProductionPath      []string `json:"production_path"`
	TestNames           []string `json:"test_names"`
	PositiveAndNegative bool     `json:"positive_and_negative"`
	SourceSHA           string   `json:"source_sha"`
	Evidence            []string `json:"evidence"`
	Blocker             string   `json:"blocker,omitempty"`
	Detail              string   `json:"detail"`
	Scope               string   `json:"scope"`
}

func writeExecutionAudit(runner *Runner, contracts *ContractsSet, outPath string) error {
	var mapping, err = BuildRequirementsEvidenceMap(contracts)
	if err != nil {
		return err
	}
	var rows = make([]ExecutionAuditRow, 0, mapping.TotalRequirements)
	for _, declared := range mapping.Records {
		var res, exists = runner.results[declared.ID]
		var row = ExecutionAuditRow{
			ID: declared.ID, Requirement: declared.Title, Status: "UNVERIFIED",
			Tier: []string{}, RequiredTiers: declared.RequiredTiers,
			ProductionPath: []string{"tools/language-acceptance/main.go"},
			TestNames:      []string{}, SourceSHA: runner.implSHA, Evidence: []string{},
			Scope: "Local runner assertion scope only; not universal feature, installed consumer, or Apple runtime acceptance",
		}
		if exists {
			row.Detail = res.Detail
			row.Tier = res.ExecutedLevels
			if row.Tier == nil {
				row.Tier = []string{}
			}
			switch {
			case res.Passed && res.Status == "PASSED":
				row.Status = "PASS"
			case res.Status == "FAILED":
				row.Status = "FAIL"
			case res.Status == "PENDING_EXTERNAL":
				row.Status = "BLOCKED_EXTERNAL"
				row.Blocker = "Required external tier has no validated current-SHA evidence in this local run"
			case res.Status == "AI_EVAL_NOT_RUN":
				row.Status = "BLOCKED_EXTERNAL"
				row.Blocker = "External held-out model evaluation endpoint not configured"
			default:
				row.Blocker = "Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation"
			}
			for _, record := range runner.records {
				for _, id := range record.Requirements {
					if id == declared.ID {
						row.TestNames = appendUniqueAudit(row.TestNames, record.TestID)
						row.Evidence = appendUniqueAudit(row.Evidence, "artifacts/backend-recovery/evidence/local-runner-evidence.json#"+record.EvidenceID)
					}
				}
			}
		}
		if row.ID == "F01" {
			row.ProductionPath = []string{"internal/language/syntax/parser.go", "internal/language/analysis/analyzer.go", "internal/language/lower/lower.go", "internal/language/lower/short_circuit.go", "language_backend_adapter.go"}
			row.TestNames = append([]string{}, f01RequiredTests...)
			row.PositiveAndNegative = res.Status == "PENDING_EXTERNAL" || res.Passed
		}
		rows = append(rows, row)
	}
	var doc = map[string]any{
		"schema_version": "1", "source_sha": runner.implSHA, "generated_at": time.Now().UTC().Format(time.RFC3339),
		"acceptance_count": len(contracts.Acceptance.Cases), "repair_count": len(contracts.Repair.Cases),
		"recovery_gate_count": len(contracts.Gates.Gates), "closure_complete": false,
		"scope": "Executed local assertions and explicit unexecuted obligations; declaration mappings are not execution evidence",
		"rows":  rows,
	}
	var data, marshalErr = json.MarshalIndent(doc, "", "  ")
	if marshalErr != nil {
		return marshalErr
	}
	if err = os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(outPath, append(data, '\n'), 0644)
}

func appendUniqueAudit(items []string, item string) []string {
	for _, existing := range items {
		if strings.EqualFold(existing, item) {
			return items
		}
	}
	return append(items, item)
}
