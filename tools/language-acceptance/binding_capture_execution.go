package main

import (
	"fmt"
	"reflect"
	"strings"
)

var bindingCaptureIDs = []string{"V01", "V03"}

func init() {
	bindingProductionTests["V01"] = []string{
		"TestP15V01DeclarationSnapshotProduction",
		"TestP15BindingCaptureTerminalCausality",
		"TestP15BindingDefaultsPublicRoutes",
	}
	bindingProductionTests["V03"] = []string{
		"TestP15V03ClipboardCapturedOnceProduction",
		"TestP15BindingCaptureTerminalCausality",
		"TestP15BindingDefaultsPublicRoutes",
	}
	for _, id := range bindingCaptureIDs {
		bindingProductionPaths[id] = []string{
			"internal/language/lower/lower.go", "internal/language/lower/binding_capture.go",
			"language_backend_adapter.go", "language_emit_adapter.go", "mobile_compiler.go",
			"p15_binding_defaults_acceptance_test.go", "p15_binding_capture_terminal_test.go",
			"tests/runtime_poc/CherriBindingCapturePOC.cherri",
		}
	}
}

func bindingCaptureProductionTestForTier(id, tier string) string {
	if id != "V01" && id != "V03" {
		return ""
	}
	switch tier {
	case "unit":
		return "github.com/electrikmilk/cherri:" + bindingProductionTests[id][0]
	case "native-structure":
		return "github.com/electrikmilk/cherri:" + bindingProductionTests[id][1]
	case "ios-runtime":
		return "ios27-runtime-poc:variables"
	}
	return ""
}

func (r *Runner) runBindingCaptureRequirement(id string) {
	title := "let captures current var value before later assignment"
	if id == "V03" {
		title = "System clipboard read captured once"
	}
	tests := bindingProductionTests[id]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	r.Record(id, "binding", title, "unit+native+iOS", []string{"unit", "native-structure"}, passed, "PASSED",
		detail+"; exact serialized capture/order/consumer graph and CLI/mobile routes verified; actual iOS values remain required")
	for index := first; index < len(r.records); index++ {
		r.records[index].TestID = bindingCaptureProductionTestForTier(id, r.records[index].Tier)
		r.records[index].Command = productionTestCommand(tests)
		r.records[index].Assertions["required_production_tests"] = strings.Join(tests, "\n")
	}
}

func validateBindingCaptureRecord(id string, record EvidenceRecord) error {
	if id != "V01" && id != "V03" {
		return nil
	}
	producer := bindingCaptureProductionTestForTier(id, record.Tier)
	if producer == "" || record.TestID != producer {
		return fmt.Errorf("binding capture record %s substitutes an unexecuted producer or tier for %s", record.EvidenceID, id)
	}
	if record.Tier == "ios-runtime" {
		// The independent existing runtime verifier checks its actual artifacts,
		// current source, device assertions and exact required observations.
		return nil
	}
	tests := bindingProductionTests[id]
	if !reflect.DeepEqual(record.Command, productionTestCommand(tests)) ||
		record.Assertions["required_production_tests"] != strings.Join(tests, "\n") {
		return fmt.Errorf("binding capture record %s is detached from its complete executed %s scenario", record.EvidenceID, id)
	}
	return nil
}
