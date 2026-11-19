package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestP15RP50AIHarnessControlFlow(t *testing.T) {
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		t.Fatal(err)
	}
	p15RunAIHarnessControlSuite(t, filepath.Join(root, "tools", "language-ai-eval"))
}

func p15RunAIHarnessControlSuite(t *testing.T, installedRoot string) {
	t.Helper()
	var python, pythonErr = acceptancePython3("CHERRI_ACCEPTANCE_PYTHON")
	if pythonErr != nil {
		t.Fatal(pythonErr)
	}
	var expected, marshalErr = json.Marshal(aiEvaluationPythonTests)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	const bridge = `import json, sys, unittest
root, required = sys.argv[1], json.loads(sys.argv[2])
suite = unittest.defaultTestLoader.discover(root, pattern="test_runner.py")
def flatten(tests):
    for test in tests:
        if isinstance(test, unittest.TestSuite):
            yield from flatten(test)
        else:
            yield test.id()
found = sorted(flatten(suite))
if found != sorted(required):
    print(json.dumps({"found": found, "selection_complete": False}))
    raise SystemExit(2)
class ObservedResult(unittest.TextTestResult):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.passed_names = []
    def addSuccess(self, test):
        self.passed_names.append(test.id())
        super().addSuccess(test)
result = unittest.TextTestRunner(stream=sys.stderr, verbosity=2, resultclass=ObservedResult).run(suite)
receipt = {"selection_complete": True, "found": found, "passed": sorted(result.passed_names),
           "tests_run": result.testsRun, "failures": len(result.failures), "errors": len(result.errors),
           "skipped": len(result.skipped), "expected_failures": len(result.expectedFailures),
           "unexpected_successes": len(result.unexpectedSuccesses)}
print(json.dumps(receipt))
raise SystemExit(0 if result.wasSuccessful() else 1)
`
	var ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var command = exec.CommandContext(ctx, python, "-I", "-B", "-c", bridge,
		installedRoot, string(expected))
	command.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	var stderr strings.Builder
	command.Stderr = &stderr
	var output, processErr = command.Output()
	if processErr != nil {
		t.Fatalf("actual AI harness control suite failed: %v context=%v\n%s\n%s", processErr, ctx.Err(), output, stderr.String())
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
	if err := json.Unmarshal(output, &receipt); err != nil {
		t.Fatal("actual Python suite did not return a discovery receipt", err, string(output))
	}
	var required = append([]string{}, aiEvaluationPythonTests...)
	sort.Strings(required)
	if !receipt.SelectionComplete || receipt.TestsRun != len(required) || !reflect.DeepEqual(receipt.Found, required) ||
		!reflect.DeepEqual(receipt.Passed, required) || receipt.Failures != 0 || receipt.Errors != 0 ||
		receipt.Skipped != 0 || receipt.ExpectedFailures != 0 || receipt.UnexpectedSuccesses != 0 {
		t.Fatalf("incomplete Python control execution: %+v", receipt)
	}
	t.Log("P15_RP50_HARNESS_CONTROL_DISCOVERY:" + string(output))
	t.Log("Only actual harness control flow was exercised; fake adapters supply no AI quality evidence")
}

func TestAIEvaluationHarnessEvidenceRetainsExternalRequirement(t *testing.T) {
	var all, err = LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var selected = &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, scenario := range all.Acceptance.Cases {
		if scenario.ID == "AI01" {
			selected.Acceptance.Cases = append(selected.Acceptance.Cases, scenario)
		}
	}
	for _, scenario := range all.Repair.Cases {
		if scenario.ID == "RP50" {
			selected.Repair.Cases = append(selected.Repair.Cases, scenario)
		}
	}
	selected.Acceptance.CaseCount, selected.Repair.CaseCount = len(selected.Acceptance.Cases), len(selected.Repair.Cases)
	if selected.Acceptance.CaseCount != 1 || selected.Repair.CaseCount != 1 {
		t.Fatal("immutable AI obligations missing")
	}
	var runner = NewRunner()
	runner.runEvaluation()
	runner.runRepairCases(selected.Repair)
	var result = runner.results["RP50"]
	if len(runner.records) != 1 || len(runner.results) != 2 || result.Passed || result.Status != "PENDING_EXTERNAL" ||
		!reflect.DeepEqual(result.ExecutedLevels, []string{"harness-unit"}) || runner.results["AI01"].Status != "AI_EVAL_NOT_RUN" {
		t.Fatalf("harness controls were missing or promoted to actual evaluation: %+v records=%d AI01=%+v", result, len(runner.records), runner.results["AI01"])
	}
	var record = runner.records[0]
	if record.Result != "passed" || record.ExitCode != 0 || record.ImplementationSHA != runner.implSHA ||
		!reflect.DeepEqual(record.Requirements, []string{"RP50"}) {
		t.Fatalf("actual local control execution produced invalid evidence: %+v", record)
	}
	if err := validateAIEvaluationHarnessRecord("RP50", record); err != nil {
		t.Fatal(err)
	}
	var manifest = &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	var report, validationErr = ValidateEvidenceManifest(manifest, selected, "local")
	if validationErr != nil || report.PassedCases != 0 || report.SkippedCases != 2 || report.FailedCases != 0 || report.ClosureComplete {
		t.Fatalf("control-only evidence erased external requirements: %+v %v", report, validationErr)
	}
	var encoded, marshalErr = json.Marshal(manifest)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	for _, fault := range []string{"invented_evaluation", "borrowed_AI01", "wrong_command", "missing_python_tests", "quality_claim", "stale_sha"} {
		t.Run(fault, func(t *testing.T) {
			var changed EvidenceManifest
			if err := json.Unmarshal(encoded, &changed); err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "invented_evaluation":
				changed.Records[0].Tier = "external-eval"
			case "borrowed_AI01":
				changed.Records[0].Requirements = []string{"AI01"}
			case "wrong_command":
				changed.Records[0].Command = []string{"python3", "--version"}
			case "missing_python_tests":
				delete(changed.Records[0].Assertions, "required_python_tests")
			case "quality_claim":
				changed.Records[0].Assertions["ai_quality_measured"] = "true"
			case "stale_sha":
				changed.Records[0].ImplementationSHA = strings.Repeat("f", 40)
			}
			var rejected, err = ValidateEvidenceManifest(&changed, selected, "local")
			if err == nil || rejected.ClosureComplete || rejected.PassedCases != 0 {
				t.Fatal("detached or falsely promoted harness evidence was accepted", err)
			}
		})
	}
	var final, finalErr = ValidateEvidenceManifest(manifest, selected, "final")
	if finalErr == nil || final.ClosureComplete || p15VerificationResult(t, final, "RP50").Passed {
		t.Fatal("fake controls satisfied actual model evaluation in final phase")
	}
	var auditPath = filepath.Join(t.TempDir(), "execution-audit.json")
	if err := writeExecutionAudit(runner, all, auditPath); err != nil {
		t.Fatal(err)
	}
	var auditBytes, readErr = os.ReadFile(auditPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	var audit struct {
		Rows            []ExecutionAuditRow `json:"rows"`
		ClosureComplete bool                `json:"closure_complete"`
	}
	if err := json.Unmarshal(auditBytes, &audit); err != nil || len(audit.Rows) != 179 || audit.ClosureComplete {
		t.Fatal("partial harness evidence changed immutable179 accounting", err)
	}
	for _, row := range audit.Rows {
		if row.Status == "PASS" || row.PositiveAndNegative {
			t.Fatalf("partial harness controls promoted a complete obligation: %+v", row)
		}
		if row.ID == "RP50" && (row.Status != "BLOCKED_EXTERNAL" ||
			!reflect.DeepEqual(row.Tier, []string{"harness-unit"}) ||
			!reflect.DeepEqual(row.RequiredTiers, []string{"harness-unit", "external-eval"})) {
			t.Fatalf("audit lost actual controls or pending external tier: %+v", row)
		}
	}
}

// Partial execution is accounted independently of the complete supplemental
// producer set. Adding harness controls cannot shrink RP50's immutable tiers.
func p15AppendAIEvaluationPartialSelection(t *testing.T, all, selected *ContractsSet) int {
	t.Helper()
	if _, complete := supplementalProductionScenarios["RP50"]; complete {
		t.Fatal("control-only RP50 producer entered the complete supplemental inventory")
	}
	if !reflect.DeepEqual(aiEvaluationHarnessScenario.Tiers, []string{"harness-unit"}) {
		t.Fatal("AI control producer expanded into an unexecuted evaluation tier")
	}
	for _, scenario := range selected.Repair.Cases {
		if scenario.ID == "RP50" {
			t.Fatal("partial AI requirement was selected twice")
		}
	}
	for _, scenario := range all.Repair.Cases {
		if scenario.ID == "RP50" {
			if !reflect.DeepEqual(parseRequiredTiers(strings.Join(scenario.RequiredTestLevels, "+")), []string{"harness-unit", "external-eval"}) {
				t.Fatal("immutable RP50 contract changed")
			}
			selected.Repair.Cases = append(selected.Repair.Cases, scenario)
			selected.Repair.CaseCount = len(selected.Repair.Cases)
			return len(aiEvaluationHarnessScenario.Tiers)
		}
	}
	t.Fatal("immutable partial AI requirement is absent")
	return 0
}

func TestAIEvaluationHarnessCannotChangeRequiredTiers(t *testing.T) {
	for _, levels := range [][]string{nil, {"harness-unit"}, {"external-eval"}, {"harness-unit", "ios-runtime"}} {
		var runner = NewRunner()
		if runner.runAIEvaluationHarnessRequirement("RP50", "altered requirement", levels) || len(runner.records) != 0 || len(runner.results) != 0 {
			t.Fatal("weakened or unrelated AI contract minted evidence", levels)
		}
	}
	var runner = NewRunner()
	if runner.runAIEvaluationHarnessRequirement("AI01", "borrowed requirement", []string{"harness-unit", "external-eval"}) || len(runner.records) != 0 || len(runner.results) != 0 {
		t.Fatal("AI01 inherited RP50 control evidence")
	}
}
