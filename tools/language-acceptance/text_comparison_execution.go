package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

const textComparisonRequirement = "N03"
const textComparisonGroup = "numbers"
const textComparisonTitle = "Exact case-sensitive text equality and legacy comparison migration"
const textComparisonLevel = "unit+native"
const textComparisonPrefix = "n03-text-comparison"
const textComparisonUnitTest = "TestP15N03ExactTextQuotedTransport"
const textComparisonNativeTest = "TestP15N03ExactTextCompleteContract"
const textComparisonScope = "Complete local N03 unit/native-structure evidence: actual Legacy compilation and migration preserve ordinary/explicit comparison modes, including nested and lexical controls; one migrated V2 document also executes the authored exact-Text lowering path. Dynamic/literal Text operands, exact case/code-point value pairs, ==/!= polarity, direct/negated/lazy conditions, one-time evaluation, source-owned implicit input and explicit native Text input are joined to actual unsigned compiler graphs with causal corruption rejection. Number/AnyContent and explicit compatibility routes retain their native contract. No Apple runtime, signing, device or related-requirement acceptance is claimed."

// These are reviewed source declarations, never a selection inferred from a
// process that happened to pass. Native means native-structure in immutable N03.
//
//go:embed text_comparison_inventory.json
var textComparisonInventory []byte

type textComparisonInventoryData struct {
	Schema      int      `json:"schema"`
	Tests       []string `json:"tests"`
	Events      []string `json:"events"`
	SourcePaths []string `json:"source_paths"`
}

var textComparisonEvidence = func() textComparisonInventoryData {
	var value textComparisonInventoryData
	if err := json.Unmarshal(textComparisonInventory, &value); err != nil {
		panic(err)
	}
	return value
}()

var textComparisonTiers = []string{"unit", "native-structure"}

const textComparisonTestsSHA256 = "784519226c21b45756b5adc239284894ada209cda1c18cdb1e6e5ab09af59909"
const textComparisonEventsSHA256 = "c26cd4c07766d2458db8a8414a2610e30627550cb7464035cb0e586780a3e74e"
const textComparisonSourceWitnessSHA256 = "57af67bff1d0eb351c152a9950136a4b4c990f00875bfc24a566edb8f6896df3"

func textComparisonListHash(values []string) string {
	return editorSHA([]byte(strings.Join(values, "\n") + "\n"))
}

func validateTextComparisonSourceInventory(names []string) error {
	// Preserve the shared whole-set 128-file, 4MiB-per-blob and 16MiB-total
	// limits. Immutable source Git calls retain their existing 15-second bounds.
	if len(names) == 0 || len(names) > 128 {
		return fmt.Errorf("invalid bounded N03 source inventory")
	}
	if !sort.StringsAreSorted(names) {
		return fmt.Errorf("unordered N03 source inventory")
	}
	for index := 1; index < len(names); index++ {
		if names[index] == names[index-1] {
			return fmt.Errorf("duplicate N03 source witness")
		}
	}
	if textComparisonListHash(names) != textComparisonSourceWitnessSHA256 {
		return fmt.Errorf("N03 inventory omits or changes reviewed source witnesses")
	}
	return nil
}

func validateTextComparisonInventory(value textComparisonInventoryData) error {
	if value.Schema != 1 || len(value.Tests) != 18 || len(value.Events) != 222 ||
		!sort.StringsAreSorted(value.Tests) || !sort.StringsAreSorted(value.Events) ||
		textComparisonListHash(value.Tests) != textComparisonTestsSHA256 ||
		textComparisonListHash(value.Events) != textComparisonEventsSHA256 {
		return fmt.Errorf("N03 complete parent/child selection differs from the reviewed inventory")
	}
	parents := map[string]bool{}
	for _, name := range value.Tests {
		if parents[name] || !strings.HasPrefix(name, "TestP15N03") || strings.Contains(name, "/") {
			return fmt.Errorf("N03 parent inventory is duplicated or foreign")
		}
		parents[name] = true
	}
	seen := map[string]bool{}
	for _, event := range value.Events {
		parent := strings.SplitN(event, "/", 2)[0]
		if seen[event] || !parents[parent] {
			return fmt.Errorf("N03 named-event inventory is duplicated or foreign")
		}
		seen[event] = true
	}
	for parent := range parents {
		if !seen[parent] {
			return fmt.Errorf("N03 parent event is missing")
		}
	}
	return validateTextComparisonSourceInventory(value.SourcePaths)
}

func textComparisonTestIDs() []string {
	ids := make([]string, len(textComparisonEvidence.Tests))
	for index, name := range textComparisonEvidence.Tests {
		ids[index] = "github.com/electrikmilk/cherri:" + name
	}
	return ids
}

func textComparisonProducer(tier string) string {
	switch tier {
	case "unit":
		return "github.com/electrikmilk/cherri:" + textComparisonUnitTest
	case "native-structure":
		return "github.com/electrikmilk/cherri:" + textComparisonNativeTest
	}
	return ""
}

func (r *Runner) executeTextComparison() ([]EvidenceArtifact, string, bool, error) {
	if err := validateTextComparisonInventory(textComparisonEvidence); err != nil {
		return nil, "", false, err
	}
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	source, clean, stateErr := editorSourceState(root)
	if stateErr != nil {
		return nil, "", false, fmt.Errorf("N03 source verification failed: %w", stateErr)
	}
	if !clean || source != r.implSHA {
		return nil, "", false, fmt.Errorf("N03 requires exact clean source: expected_head=%s observed_head=%s clean=%t", r.implSHA, source, clean)
	}
	directory, err := stateEvidenceDirectory(r, textComparisonPrefix+"-")
	if err != nil {
		return nil, "", false, err
	}
	command := productionTestCommand(textComparisonEvidence.Tests)
	budget := productionTestTimeout(textComparisonEvidence.Tests) + time.Minute
	receipt, artifacts, err := runExactSourceProcess(root, source, textComparisonRequirement, textComparisonPrefix, directory, command, budget, textComparisonEvidence.Events, textComparisonEvidence.SourcePaths)
	if err != nil {
		return artifacts, "", len(artifacts) > 0, err
	}
	raw, err := os.ReadFile(filepath.Join(directory, textComparisonPrefix+"-go.jsonl"))
	if err != nil {
		return artifacts, "", true, err
	}
	if err = parseStrictNamedProductionOutput(textComparisonEvidence.Events, budget, raw); err != nil {
		return artifacts, "", true, err
	}
	if err = loopOutputWithinProcess(raw, receipt); err != nil {
		return artifacts, "", true, err
	}
	return artifacts, textComparisonScope, true, nil
}

func (r *Runner) runTextComparisonRequirement(id, group, title, level string) bool {
	if id != textComparisonRequirement {
		return false
	}
	if group != textComparisonGroup || title != textComparisonTitle || level != textComparisonLevel ||
		!reflect.DeepEqual(parseRequiredTiers(level), textComparisonTiers) {
		r.results[id] = CaseResult{ID: id, Group: group, Requirement: title, TestLevel: level, Status: "FAILED", Detail: "N03 producer differs from the immutable contract"}
		return true
	}
	artifacts, detail, executed, err := r.executeTextComparison()
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[id] = CaseResult{ID: id, Group: group, Requirement: title, TestLevel: level, Status: "NOT_RUN", Detail: detail}
		return true
	}
	first := len(r.records)
	r.Record(id, group, title, level, textComparisonTiers, err == nil, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		record := &r.records[index]
		record.TestID = textComparisonProducer(record.Tier)
		record.Command = productionTestCommand(textComparisonEvidence.Tests)
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(textComparisonEvidence.Tests, "\n")
		record.Assertions["required_go_events"] = strings.Join(textComparisonEvidence.Events, "\n")
		record.Assertions["scope"] = textComparisonScope
		record.Assertions["apple_runtime_executed"] = "false"
	}
	return true
}

func validateTextComparisonRecord(id string, record EvidenceRecord) error {
	if id != textComparisonRequirement {
		return nil
	}
	if err := validateTextComparisonInventory(textComparisonEvidence); err != nil {
		return err
	}
	producer := textComparisonProducer(record.Tier)
	if producer == "" {
		return fmt.Errorf("N03 complete local producer does not authorize tier %q", record.Tier)
	}
	if record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{textComparisonRequirement}) ||
		record.TestID != producer || !reflect.DeepEqual(record.Command, productionTestCommand(textComparisonEvidence.Tests)) ||
		record.Assertions["required_production_tests"] != strings.Join(textComparisonEvidence.Tests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(textComparisonEvidence.Events, "\n") ||
		record.Assertions["scope"] != textComparisonScope || record.Assertions["apple_runtime_executed"] != "false" {
		return fmt.Errorf("detached or incomplete N03 local scenario")
	}
	data, err := stateReadArtifacts(record.Artifacts, editorContractArtifacts(textComparisonPrefix))
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data[textComparisonPrefix+"-process.json"], &receipt); err != nil {
		return err
	}
	raw, stderr := data[textComparisonPrefix+"-go.jsonl"], data[textComparisonPrefix+"-stderr.log"]
	budget := productionTestTimeout(textComparisonEvidence.Tests) + time.Minute
	if err = validateExactSourceProcess(textComparisonRequirement, record.ImplementationSHA, receipt, record.Command, textComparisonEvidence.Events, raw, stderr, budget, textComparisonEvidence.SourcePaths); err != nil {
		return err
	}
	if err = parseStrictNamedProductionOutput(textComparisonEvidence.Events, budget, raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
