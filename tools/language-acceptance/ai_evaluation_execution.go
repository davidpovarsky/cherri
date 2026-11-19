package main

import (
	"fmt"
	"reflect"
	"strings"
)

var aiEvaluationHarnessScenario = supplementalProductionScenario{
	PackagePath: "./tools/language-acceptance",
	ModulePath:  "github.com/electrikmilk/cherri/tools/language-acceptance",
	TestNames:   []string{"TestP15RP50AIHarnessControlFlow"},
	Tiers:       []string{"harness-unit"},
	ProductionPaths: []string{
		"tools/language-ai-eval/runner.py", "tools/language-ai-eval/tasks.json",
		"tools/language-ai-eval/test_runner.py", "tools/language-acceptance/ai_evaluation_execution.go",
		"tools/language-acceptance/ai_evaluation_execution_test.go",
	},
}

var aiEvaluationPythonTests = []string{
	"test_runner.EvaluationHarnessTests.test_absent_external_configuration_is_not_model_quality",
	"test_runner.EvaluationHarnessTests.test_duplicate_task_and_missing_arm_fail_closed",
	"test_runner.EvaluationHarnessTests.test_external_configuration_requires_explicit_execution_flag",
	"test_runner.EvaluationHarnessTests.test_fake_subprocess_covers_both_arms_without_quality_score",
	"test_runner.EvaluationHarnessTests.test_hung_adapter_times_out",
	"test_runner.EvaluationHarnessTests.test_identity_model_and_actual_usage_are_enforced",
	"test_runner.EvaluationHarnessTests.test_malformed_process_json_and_oversized_stdout_fail_closed",
	"test_runner.EvaluationHarnessTests.test_malformed_suite_replaces_stale_success_with_failure",
	"test_runner.EvaluationHarnessTests.test_nonfinite_timeout_replaces_stale_receipt_without_starting",
	"test_runner.EvaluationHarnessTests.test_nonzero_secret_stderr_is_never_in_error",
	"test_runner.EvaluationHarnessTests.test_output_cannot_overwrite_a_task_suite",
	"test_runner.EvaluationHarnessTests.test_request_identity_binds_all_model_inputs_and_excludes_oracle",
	"test_runner.EvaluationHarnessTests.test_response_types_are_rejected_without_attribute_errors",
	"test_runner.EvaluationHarnessTests.test_timeout_stops_a_spawned_worker",
	"test_runner.EvaluationHarnessTests.test_validator_requires_exact_source_identity_and_oracle_coverage",
}

func (r *Runner) runAIEvaluationHarnessRequirement(id, title string, levels []string) bool {
	if id != "RP50" || !reflect.DeepEqual(parseRequiredTiers(strings.Join(levels, "+")), []string{"harness-unit", "external-eval"}) {
		return false
	}
	var scenario = aiEvaluationHarnessScenario
	var passed, detail = executeNamedProductionTestsInPackage(scenario.PackagePath, scenario.TestNames)
	detail += "; actual Python task runner control-flow suite only; fake adapters are not AI quality evidence; external evaluation remains AI_EVAL_NOT_RUN"
	var firstRecord = len(r.records)
	r.Record(id, "repair", title, strings.Join(levels, "+"), scenario.Tiers, passed, "PASSED", detail)
	for index := firstRecord; index < len(r.records); index++ {
		r.records[index].TestID = scenario.testIDs()[0]
		r.records[index].Command = productionTestCommandInPackage(scenario.PackagePath, scenario.TestNames)
		r.records[index].Assertions["required_production_tests"] = strings.Join(scenario.TestNames, "\n")
		r.records[index].Assertions["required_python_tests"] = strings.Join(aiEvaluationPythonTests, "\n")
		r.records[index].Assertions["harness_scope"] = "control-flow-only"
		r.records[index].Assertions["ai_quality_measured"] = "false"
		r.records[index].Assertions["external_evaluation"] = "AI_EVAL_NOT_RUN"
	}
	return true
}

func validateAIEvaluationHarnessRecord(id string, record EvidenceRecord) error {
	if id != "RP50" {
		return nil
	}
	var scenario = aiEvaluationHarnessScenario
	if record.Tier != "harness-unit" || record.TestID != scenario.testIDs()[0] ||
		!reflect.DeepEqual(record.Command, productionTestCommandInPackage(scenario.PackagePath, scenario.TestNames)) ||
		record.Assertions["required_production_tests"] != strings.Join(scenario.TestNames, "\n") ||
		record.Assertions["required_python_tests"] != strings.Join(aiEvaluationPythonTests, "\n") ||
		record.Assertions["harness_scope"] != "control-flow-only" ||
		record.Assertions["ai_quality_measured"] != "false" ||
		record.Assertions["external_evaluation"] != "AI_EVAL_NOT_RUN" {
		return fmt.Errorf("record %s is detached from the complete RP50 harness controls or claims an unexecuted evaluation tier", record.EvidenceID)
	}
	return nil
}
