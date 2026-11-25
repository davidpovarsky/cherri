package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

// The immutable requirement remains a joined local/native contract. Enrollment
// of fresh local processes never grants a native or whole-requirement pass.
//
//go:embed joined_public_inventory.json
var joinedPublicInventory []byte

type joinedPublicScenario struct {
	Title         string               `json:"title"`
	Scenario      string               `json:"scenario"`
	Expected      string               `json:"expected"`
	Prefix        string               `json:"prefix"`
	RequiredTiers []string             `json:"required_tiers"`
	LocalTiers    []string             `json:"local_tiers"`
	TierSteps     map[string]string    `json:"tier_steps"`
	Scope         string               `json:"scope"`
	Steps         []editorContractStep `json:"steps"`
}

var joinedPublicScenarios = func() map[string]joinedPublicScenario {
	var config struct {
		Schema    int                             `json:"schema"`
		Scenarios map[string]joinedPublicScenario `json:"scenarios"`
	}
	if err := json.Unmarshal(joinedPublicInventory, &config); err != nil || config.Schema != 1 || len(config.Scenarios) != 2 {
		panic("invalid joined public execution inventory")
	}
	return config.Scenarios
}()

func (s joinedPublicScenario) tests() []string {
	var names []string
	for _, step := range s.Steps {
		names = append(names, step.Tests...)
	}
	return names
}
func (s joinedPublicScenario) testIDs() []string {
	var ids []string
	for _, step := range s.Steps {
		for _, test := range step.Tests {
			ids = append(ids, step.PackageID+":"+test)
		}
	}
	return ids
}
func (s joinedPublicScenario) tierStep(tier string) (editorContractStep, bool) {
	name, okay := s.TierSteps[tier]
	if !okay {
		return editorContractStep{}, false
	}
	for _, step := range s.Steps {
		if step.Name == name {
			return step, true
		}
	}
	return editorContractStep{}, false
}
func (s joinedPublicScenario) binding() string { raw, _ := json.Marshal(s); return string(raw) }
func (s joinedPublicScenario) matches(g RecoveryGate) bool {
	return g.Mandatory && g.Title == s.Title && g.Scenario == s.Scenario &&
		g.ExpectedObservation == s.Expected && g.ProposedTestPrefix == s.Prefix &&
		reflect.DeepEqual(parseRequiredTiers(strings.Join(g.MinimumEvidenceTiers, "+")), s.RequiredTiers)
}
func joinedPublicPrefix(id, step string) string {
	return "joined-public-" + strings.ToLower(id) + "-" + step
}

func joinedPublicSourcePaths(id string) []string {
	var paths []string
	switch id {
	case "BRG25":
		paths = append(paths, publicFrontendProductionPaths...)
		paths = append(paths, "parser.go", "signing.go", "p15_joined_frontend_routes_test.go")
	case "BRG27":
		paths = append(paths, combinedEditorProductionPaths...)
		paths = append(paths,
			"mobile_compiler.go", "mobile_language_service.go", "ios_bridge.go",
			"internal/language/service/editor_snapshot.go",
			"internal/language/service/completion_edits.go",
			"internal/language/protocol/protocol.go",
			"internal/language/lsp/server.go",
			"internal/language/lsp/position_wire.go",
			"internal/language/lsp/p15_joined_editor_contract_test.go",
			"p15_joined_editor_mobile_test.go",
		)
	default:
		return nil
	}
	paths = append(paths,
		"tools/language-acceptance/joined_public_execution.go",
		"tools/language-acceptance/joined_public_inventory.json",
		"tools/language-acceptance/joined_public_execution_test.go",
		"tools/language-acceptance/partial_apple_gate_scope_test.go",
		"tools/language-acceptance/editor_contract_execution.go",
	)
	seen := map[string]bool{}
	var unique []string
	for _, path := range paths {
		if !seen[path] {
			seen[path] = true
			unique = append(unique, path)
		}
	}
	sort.Strings(unique)
	return unique
}
func joinedPublicValidateOriginals(id, source string, artifacts []EvidenceArtifact) error {
	spec, okay := joinedPublicScenarios[id]
	if !okay {
		return fmt.Errorf("unknown joined public scenario")
	}
	var names []string
	for _, step := range spec.Steps {
		names = append(names, editorContractArtifacts(joinedPublicPrefix(id, step.Name))...)
	}
	data, err := stateReadArtifacts(artifacts, names)
	if err != nil {
		return err
	}
	for _, step := range spec.Steps {
		prefix := joinedPublicPrefix(id, step.Name)
		var receipt editorProcessReceipt
		if err = decodeEditorJSON(data[prefix+"-process.json"], &receipt); err != nil {
			return err
		}
		raw, stderr := data[prefix+"-go.jsonl"], data[prefix+"-stderr.log"]
		budget := productionTestTimeout(step.Tests) + time.Minute
		if err = validateExactSourceProcess(id, source, receipt, editorContractCommand(step), step.Events, raw, stderr, budget, joinedPublicSourcePaths(id)); err != nil {
			return err
		}
		if err = parseStrictNamedProductionOutputInPackage(step.PackageID, step.Events, budget, raw); err != nil {
			return err
		}
		if err = loopOutputWithinProcess(raw, receipt); err != nil {
			return err
		}
	}
	return nil
}
func (r *Runner) runJoinedPublicRequirement(g RecoveryGate) bool {
	spec, okay := joinedPublicScenarios[g.ID]
	if !okay {
		return false
	}
	level := strings.Join(g.MinimumEvidenceTiers, "+")
	if !spec.matches(g) {
		r.results[g.ID] = CaseResult{ID: g.ID, Group: "recovery_gate", Requirement: g.Title, TestLevel: level, Status: "FAILED", Detail: "joined public producer differs from immutable requirement"}
		return true
	}
	root, err := filepath.Abs(resolveRepoRoot())
	var directory string
	if err == nil {
		directory, err = stateEvidenceDirectory(r, "joined-public-"+strings.ToLower(g.ID)+"-")
	}
	if err != nil {
		r.results[g.ID] = CaseResult{ID: g.ID, Group: "recovery_gate", Requirement: g.Title, TestLevel: level, Status: "NOT_RUN", Detail: err.Error()}
		return true
	}
	var artifacts []EvidenceArtifact
	for _, step := range spec.Steps {
		_, files, runErr := runExactSourceProcess(root, r.implSHA, g.ID, joinedPublicPrefix(g.ID, step.Name), directory,
			editorContractCommand(step), productionTestTimeout(step.Tests)+time.Minute, step.Events, joinedPublicSourcePaths(g.ID))
		artifacts = append(artifacts, files...)
		if runErr != nil {
			err = fmt.Errorf("%s: %w", step.Name, runErr)
			break
		}
	}
	if len(artifacts) == 0 {
		r.results[g.ID] = CaseResult{ID: g.ID, Group: "recovery_gate", Requirement: g.Title, TestLevel: level, Status: "NOT_RUN", Detail: fmt.Sprint(err)}
		return true
	}
	if err == nil {
		err = joinedPublicValidateOriginals(g.ID, r.implSHA, artifacts)
	}
	detail := spec.Scope
	if err != nil {
		detail = err.Error()
	}
	first := len(r.records)
	r.Record(g.ID, "recovery_gate", g.Title, level, spec.LocalTiers, err == nil, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		record := &r.records[i]
		step, found := spec.tierStep(record.Tier)
		if !found {
			panic("joined local tier has no process producer")
		}
		record.TestID = step.PackageID + ":" + step.Tests[0]
		record.Command = editorContractCommand(step)
		record.Artifacts = append([]EvidenceArtifact{}, artifacts...)
		record.Assertions["joined_public_contract"] = spec.binding()
		record.Assertions["scope"] = spec.Scope
	}
	return true
}
func validateJoinedPublicRecord(id string, record EvidenceRecord, manifest *EvidenceManifest) error {
	spec, okay := joinedPublicScenarios[id]
	if !okay {
		return nil
	}
	if record.Tier == "ios-build" || record.Tier == "ios-ui" {
		// Replaced only when the dedicated native original validator is integrated.
		return fmt.Errorf("%s dedicated native producer is not yet enrolled", id)
	}
	step, found := spec.tierStep(record.Tier)
	if !found || record.Result != "passed" || record.ExitCode != 0 ||
		!reflect.DeepEqual(record.Requirements, []string{id}) ||
		record.TestID != step.PackageID+":"+step.Tests[0] ||
		!reflect.DeepEqual(record.Command, editorContractCommand(step)) ||
		record.Assertions["joined_public_contract"] != spec.binding() ||
		record.Assertions["scope"] != spec.Scope {
		return fmt.Errorf("%s record detached from complete joined public process bundle", id)
	}
	return joinedPublicValidateOriginals(id, record.ImplementationSHA, record.Artifacts)
}
