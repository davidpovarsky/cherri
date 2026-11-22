package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

const functionTransportTitle = "Structured argument/result transport"
const functionTransportExpected = "List/map/optional shapes retained or exact limitation reported"
const functionTransportScope = "Independent FN05 typed source and complete native graphs for bound numeric List, empty List/Map, nested Map with reserved keys and ordered empty/false/zero values, Optional None versus present empty Text/List/Map, false, zero and nonempty structured values. Every argument carrier, child return, caller decoder and final consumer is checked with causal corruptions. Nested native Lists and Optional absence inside ordinary containers retain exact source-bound target limitations and emit no artifact. This supplies unit/native-structure only; it does not execute Apple or grant T05/L01/F04/RP20/RP24/RP25/RP26/BRG11/BRG15 credit."
const functionTransportPrefix = "function-transport-fn05"

var functionTransportTests = []string{"TestP15FN05CombinedStructuredTransportUnit", "TestP15FN05CombinedStructuredTransportNative"}
var functionTransportTiers = []string{"unit", "native-structure"}

// The clean whole commit fixes all dependencies. These additional Git witnesses
// bind the relevant transport implementation, independent oracles and intake.
var functionTransportProductionPaths = []string{
	"go.mod", "go.sum",
	"internal/language/syntax/ast.go", "internal/language/syntax/lexer.go", "internal/language/syntax/parser.go",
	"internal/language/analysis/analyzer.go", "internal/language/analysis/function_arguments.go", "internal/language/analysis/optional_presence.go", "internal/language/analysis/optional_transport.go",
	"internal/language/types/types.go", "internal/language/types/optional_presence.go", "internal/language/ir/ir.go",
	"internal/language/lower/lower.go", "internal/language/lower/call_arguments.go", "internal/language/lower/function_defaults.go", "internal/language/lower/function_optional_presence.go", "internal/language/lower/optional_presence_demand.go", "internal/language/lower/optional_transport.go",
	"internal/language/lower/function_argument_containers.go", "internal/language/lower/container_evaluation_order.go", "internal/language/lower/container_keys.go", "internal/language/lower/empty_list.go", "internal/language/lower/literal_list.go", "internal/language/lower/binding_capture.go", "internal/language/lower/scoped_blocks.go", "internal/language/lower/main_output.go",
	"internal/language/backend/contracts.go", "internal/language/schema/generated_registry.go",
	"language_backend_session.go", "language_backend_adapter.go", "language_emit_adapter.go", "language_value_adapter.go", "language_typed_literal_adapter.go", "shared_codec.go", "shortcutgen.go",
	"p15_fn05_combined_transport_test.go", "p15_function_transport_strict_oracle_test.go", "p15_combined_short_circuit_test.go", "p15_container_shape_acceptance_test.go", "p15_nested_list_analysis_boundary_test.go", "p15_optional_transport_boundary_test.go",
	"p15_function_acceptance_test.go", "p15_function_native_oracle_test.go", "p15_types_acceptance_test.go", "p15_short_circuit_test.go", "p15_call_order_test.go", "p15_literal_collections_acceptance_test.go",
	"tests/language-v2/acceptance-contract.json", "tests/language-v2/repair-regression-plan.json", "tests/language-v2/backend-recovery-gates.json",
	"tools/language-acceptance/function_transport_execution.go", "tools/language-acceptance/function_transport_execution_test.go", "tools/language-acceptance/strict_go_events.go",
	"tools/language-acceptance/editor_contract_execution.go", "tools/language-acceptance/loop_execution.go", "tools/language-acceptance/editor_source_binding.go", "tools/language-acceptance/editor_performance_execution.go", "tools/language-acceptance/editor_process_other.go", "tools/language-acceptance/editor_process_windows.go", "tools/language-acceptance/native_encoding_intake.go", "tools/language-acceptance/state_acceptance_intake.go", "tools/language-acceptance/state_acceptance_execution.go", "tools/language-acceptance/local_gate_source_binding.go",
	"tools/language-acceptance/main.go", "tools/language-acceptance/parser_execution.go", "tools/language-acceptance/evidence_map.go", "tools/language-acceptance/execution_audit.go", "tools/language-acceptance/assertion_scope.go",
}

func functionTransportRequiredEvents() []string {
	var names = append([]string{}, functionTransportTests...)
	var cases = []string{"list_bound_numbers", "list_empty", "map_reserved_nested", "map_empty", "optional_none", "optional_empty_text", "optional_false", "optional_zero", "optional_empty_list", "optional_empty_map", "optional_list_numbers", "optional_map_reserved_nested"}
	for _, name := range cases {
		names = append(names, functionTransportTests[0]+"/"+name)
		var parent = functionTransportTests[1] + "/" + name
		names = append(names, parent)
		var faults = []string{"missing_action", "extra_action", "zero_producer_reconciled", "malformed_producer_reconciled", "duplicate_producer", "zero_group_reconciled", "malformed_group_reconciled", "real_control_code", "child_slot", "child_output", "payload_arg_removed", "payload_presence", "payload_mirror", "payload_field_kind", "self_target", "call_input", "caller_output", "result_name_reconciled"}
		if name == "list_bound_numbers" {
			faults = append(faults, "raw_number_row", "swapped_number_rows", "wrong_number_kind", "payload_uses_scalar")
		}
		if strings.HasSuffix(name, "map_reserved_nested") {
			faults = append(faults, "map_false_as_number", "map_empty_list_as_map", "map_array_order", "map_reserved_key_lost")
		}
		if strings.HasPrefix(name, "optional_") {
			faults = append(faults, "argument_marker", "argument_presence", "child_truthiness", "child_nothing", "result_presence", "result_value_edge", "result_absence_value", "caller_truthiness", "caller_nothing", "caller_presence_key")
		}
		for _, fault := range faults {
			names = append(names, parent+"/oracle/"+fault)
		}
	}
	for _, name := range []string{"list_nested_none", "map_nested_none", "nested_native_list_limitation"} {
		var parent = functionTransportTests[0] + "/" + name
		names = append(names, parent)
		for _, fault := range []string{"code", "start", "end", "context", "severity", "missing", "extra"} {
			names = append(names, parent+"/oracle/"+fault)
		}
	}
	sort.Strings(names)
	return names
}

func functionTransportTestIDs() []string {
	return []string{"github.com/electrikmilk/cherri:" + functionTransportTests[0], "github.com/electrikmilk/cherri:" + functionTransportTests[1]}
}

func functionTransportProducer(tier string) string {
	for index, allowed := range functionTransportTiers {
		if tier == allowed {
			return functionTransportTestIDs()[index]
		}
	}
	return ""
}

func parseFunctionTransportOutput(raw []byte) error {
	return parseFunctionScenarioOutput(functionTransportRequiredEvents(), productionTestTimeout(functionTransportTests)+time.Minute, raw)
}

func parseFunctionScenarioOutput(names []string, maximum time.Duration, raw []byte) error {
	if err := parseStrictNamedProductionOutput(names, maximum, raw); err != nil {
		return err
	}
	// A case parent owns its diagnostic or graph faults, as well as the outer
	// production test. A copied parent PASS must not hide detached child events.
	var selected, run, passed = map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, name := range names {
		selected[name] = true
	}
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
		var event struct{ Action, Test string }
		if err := json.Unmarshal(line, &event); err != nil {
			return err
		}
		if event.Test == "" {
			continue
		}
		switch event.Action {
		case "run":
			for parent := event.Test; strings.Contains(parent, "/"); {
				parent = parent[:strings.LastIndexByte(parent, '/')]
				if selected[parent] && (!run[parent] || passed[parent]) {
					return fmt.Errorf("function transport child outside selected ancestor %s", parent)
				}
			}
			run[event.Test] = true
		case "pass":
			for _, name := range names {
				if strings.HasPrefix(name, event.Test+"/") && !passed[name] {
					return fmt.Errorf("function transport parent passed before required child %s", name)
				}
			}
			passed[event.Test] = true
		}
	}
	return nil
}

func (r *Runner) runFunctionTransportRequirement(id string) bool {
	if id != "FN05" {
		return false
	}
	var root, err = filepath.Abs(resolveRepoRoot())
	if err != nil {
		r.results[id] = CaseResult{ID: id, Group: "functions", Requirement: functionTransportTitle, TestLevel: "unit+native", Status: "NOT_RUN", Detail: err.Error()}
		return true
	}
	var directory string
	directory, err = stateEvidenceDirectory(r, functionTransportPrefix+"-")
	if err != nil {
		r.results[id] = CaseResult{ID: id, Group: "functions", Requirement: functionTransportTitle, TestLevel: "unit+native", Status: "NOT_RUN", Detail: err.Error()}
		return true
	}
	var command = productionTestCommand(functionTransportTests)
	var receipt, artifacts, runErr = runExactSourceProcess(root, r.implSHA, id, functionTransportPrefix, directory, command, productionTestTimeout(functionTransportTests)+time.Minute, functionTransportRequiredEvents(), functionTransportProductionPaths)
	if len(artifacts) == 0 {
		r.results[id] = CaseResult{ID: id, Group: "functions", Requirement: functionTransportTitle, TestLevel: "unit+native", Status: "NOT_RUN", Detail: fmt.Sprint(runErr)}
		return true
	}
	var raw, readErr = os.ReadFile(filepath.Join(directory, functionTransportPrefix+"-go.jsonl"))
	if runErr == nil {
		runErr = readErr
	}
	if runErr == nil {
		runErr = parseFunctionTransportOutput(raw)
	}
	if runErr == nil {
		runErr = loopOutputWithinProcess(raw, receipt)
	}
	var detail = functionTransportScope
	if runErr != nil {
		detail = runErr.Error()
	}
	var first = len(r.records)
	r.Record(id, "functions", functionTransportTitle, "unit+native", functionTransportTiers, runErr == nil, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		var record = &r.records[index]
		record.TestID = functionTransportProducer(record.Tier)
		record.Command = command
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(functionTransportTests, "\n")
		record.Assertions["required_go_events"] = strings.Join(functionTransportRequiredEvents(), "\n")
		record.Assertions["scope"] = functionTransportScope
	}
	return true
}

func validateFunctionTransportRecord(id string, record EvidenceRecord) error {
	if id != "FN05" {
		return nil
	}
	var producer = functionTransportProducer(record.Tier)
	if producer == "" || record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{id}) || record.TestID != producer ||
		!reflect.DeepEqual(record.Command, productionTestCommand(functionTransportTests)) || record.Assertions["required_production_tests"] != strings.Join(functionTransportTests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(functionTransportRequiredEvents(), "\n") || record.Assertions["scope"] != functionTransportScope {
		return fmt.Errorf("FN05 record is detached from complete structured transport production; no Apple or related-ID credit")
	}
	var data, err = stateReadArtifacts(record.Artifacts, editorContractArtifacts(functionTransportPrefix))
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data[functionTransportPrefix+"-process.json"], &receipt); err != nil {
		return err
	}
	var raw, stderr = data[functionTransportPrefix+"-go.jsonl"], data[functionTransportPrefix+"-stderr.log"]
	if err = validateExactSourceProcess(id, record.ImplementationSHA, receipt, record.Command, functionTransportRequiredEvents(), raw, stderr, productionTestTimeout(functionTransportTests)+time.Minute, functionTransportProductionPaths); err != nil {
		return err
	}
	if err = parseFunctionTransportOutput(raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
