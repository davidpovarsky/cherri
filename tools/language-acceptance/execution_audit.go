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
	"regexp"
	"strings"
	"time"
)

var f01RequiredTests = []string{
	"TestP15F01StatementAndExpressionGuardRHS",
	"TestP15F01StaticSkipsMissingTarget",
	"TestP15F01BooleanStatementUsesValueNotPresence",
	"TestP15F01GuardOracleRejectsMutations",
	"TestP15F01RuntimeFixtureCompiles",
	"TestP15F01HybridEditedPredicateGuardsRHS",
}

var r06ProductionTests = []string{"TestP15R06ProductionImportFormats", "TestHYB18_SignedShortcutContainerRejected", "TestHYB20_TrailingGarbageAfterJSONRootRejected", "TestHYB21_MalformedActionEntriesRejectedWithExactIndex"}
var r07ProductionTests = []string{"TestSEM28_UserEditOnNaturalActionIsolated", "TestSEM29_UserEditInsideControlRegionIsolated", "TestSEM30_OnlyTargetActionChangesAndNoCollateralDelta", "TestSEM34_TamperDetectionAndStrictCompareAsymmetry"}

// Execute named production tests with explicit discovery checks. Exit status
// alone cannot prove that any required scenario was selected.
func executeNamedProductionTests(required []string) (bool, string) {
	var ctx, cancel = context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	var alternatives []string
	for _, name := range required {
		alternatives = append(alternatives, regexp.QuoteMeta(name))
	}
	var cmd = exec.CommandContext(ctx, "go", "test", "-count=1", "-json", "-timeout", "3m", ".", "-run", "^("+strings.Join(alternatives, "|")+")$")
	cmd.Dir = resolveRepoRoot()
	var output, err = cmd.CombinedOutput()
	var seen = make(map[string]bool)
	var scanner = bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 4096), 2*1024*1024)
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
	for _, name := range required {
		if !seen[name] {
			missing = append(missing, name)
		}
	}
	if err != nil || scanner.Err() != nil || len(missing) != 0 {
		return false, fmt.Sprintf("Production tests failed or were not discovered: process=%v; missing=%v; scanner=%v", err, missing, scanner.Err())
	}
	return true, "Executed production tests: " + strings.Join(required, ", ")
}

func executeF01ProductionTests() (bool, string) {
	ok, detail := executeNamedProductionTests(f01RequiredTests)
	return ok, detail + "; signed runtime still required"
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
	if err := writeLocalObservations(runner, filepath.Join(filepath.Dir(outPath), "acceptance-local-observations.json")); err != nil {
		return err
	}
	evidence, err := json.MarshalIndent(runner.records, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(filepath.Dir(outPath), "acceptance-local-evidence.json"), append(evidence, '\n'), 0644); err != nil {
		return err
	}
	mapping, err := BuildRequirementsEvidenceMap(contracts)
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
						row.Evidence = appendUniqueAudit(row.Evidence, "acceptance-local-evidence.json#"+record.EvidenceID)
					}
				}
			}
		}
		if gap := incompleteLocalAssertions[row.ID]; gap != "" {
			row.Blocker = gap
			row.Evidence = []string{"acceptance-local-observations.json#" + row.ID}
			row.Scope = "Observed smoke only; no formal acceptance tier proven"
		}
		if row.ID == "U03" {
			row.TestNames = []string{"TestUpstreamPropagationRehearsal"}
			row.ProductionPath = []string{"actions/web.cherri", "upstream_rehearsal_test.go", "internal/language/schema/generated_registry.go"}
			row.PositiveAndNegative = res.Passed
		}
		if row.ID == "N02" {
			row.TestNames = []string{"tools/language-acceptance:runNumbers"}
			row.ProductionPath = []string{"internal/language/analysis/analyzer.go"}
		}
		if row.ID == "M05" {
			row.TestNames = []string{"tools/language-acceptance:runMetadata"}
			row.ProductionPath = []string{"internal/language/migrate/migrate.go", "internal/language/lower/lower.go"}
		}
		if row.ID == "E07" {
			row.TestNames = []string{"tools/language-acceptance:runEditor"}
			row.ProductionPath = []string{"internal/language/service/service.go"}
			row.PositiveAndNegative = res.Status == "PENDING_EXTERNAL"
		}
		if row.ID == "R06" {
			row.TestNames = append([]string{}, r06ProductionTests...)
			row.ProductionPath = []string{"import_fidelity.go", "hybrid_import.go", "mobile_compiler.go"}
			row.PositiveAndNegative = res.Passed
		}
		if row.ID == "R07" {
			row.TestNames = append([]string{}, r07ProductionTests...)
			row.ProductionPath = []string{"semantic_import_planner.go", "hybrid_import.go", "import_provenance.go"}
			row.PositiveAndNegative = res.Passed
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
