package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type RequirementEvidenceMapping struct {
	ID                   string            `json:"id"`
	Type                 string            `json:"type"` // "acceptance", "repair", "recovery_gate"
	Title                string            `json:"title"`
	RequiredTiers        []string          `json:"required_tiers"`
	TestIDs              []string          `json:"test_ids"`
	RequiredAssertions   map[string]string `json:"required_assertions,omitempty"`
	RequiredUIAssertions map[string]string `json:"required_ui_assertions,omitempty"`
	FixtureIdentities    []string          `json:"fixture_identities"`
	Assertions           []string          `json:"assertions"`
	ResultReferences     []string          `json:"result_references"`
}

type RequirementsEvidenceMapFile struct {
	SchemaVersion     string                                `json:"schema_version"`
	GeneratedAt       string                                `json:"generated_at"`
	Authority         string                                `json:"authority"`
	TotalRequirements int                                   `json:"total_requirements"`
	AcceptanceCount   int                                   `json:"acceptance_count"`
	RepairCount       int                                   `json:"repair_count"`
	GatesCount        int                                   `json:"gates_count"`
	Mappings          map[string]RequirementEvidenceMapping `json:"mappings"`
	Records           []RequirementEvidenceMapping          `json:"records"`
}

func applyRuntimeAndUIMappings(id string, tiers []string, testIDs *[]string, reqAssertions map[string]string, reqUIAssertions map[string]string) {
	switch id {
	case "CI02", "RP48", "BRG26", "BRG31":
		reqAssertions["status"] = "CHERRI_IOS27_RUNTIME_OK"
		*testIDs = append(*testIDs, "ios27-runtime-poc:smoke")
	case "V01", "V03", "RP12", "RP14", "BRG10":
		reqAssertions["variable_result"] = "updated"
		reqAssertions["captured_before"] = "initial"
		reqAssertions["num_captured"] = "1"
		reqAssertions["clip_captured"] = "clip_initial"
		*testIDs = append(*testIDs, "ios27-runtime-poc:variables")
	case "S01", "RP19", "BRG08":
		reqAssertions["unicode_output"] = "שָׁלוֹם:🌍:1"
		*testIDs = append(*testIDs, "ios27-runtime-poc:unicode")
	case "L02", "L04", "RP21":
		reqAssertions["index_trace"] = "alpha:gamma:::::beta"
		*testIDs = append(*testIDs, "ios27-runtime-poc:indexing")
	case "N01", "RP15", "RP17", "BRG16":
		reqAssertions["math_trace"] = "2.5:1:11:0:1:0"
		reqAssertions["index_trace"] = "alpha:gamma:::::beta"
		*testIDs = append(*testIDs, "ios27-runtime-poc:numbers")
	case "F01", "RP18", "BRG13":
		reqAssertions["conditional_result"] = "true"
		reqAssertions["short_circuit_trace"] = "START|RAN_RHS:0:1:1"
		*testIDs = append(*testIDs, "ios27-runtime-poc:conditionals")
	case "F02", "F04", "RP22", "BRG12", "BRG14":
		reqAssertions["loop_trace"] = "0:A:0:1|0:A:1:2|1:B:0:1|1:B:1:2|"
		reqAssertions["value_blocks"] = "then_val:<X><Y>"
		*testIDs = append(*testIDs, "ios27-runtime-poc:nested-loops")
	case "FN01", "FN02", "FN03", "FN04", "T01", "M02", "RP24", "RP25", "RP26", "BRG11", "BRG15":
		reqAssertions["function_result"] = "21"
		reqAssertions["caller_continuation"] = "22"
		reqAssertions["defaults_explicit"] = "0:0::"
		reqAssertions["defaults_omitted"] = "1:9:def:"
		reqAssertions["recursion_fact3"] = "6"
		reqAssertions["void_output"] = ""
		reqAssertions["partial_envelope_safe"] = "multiply"
		*testIDs = append(*testIDs, "ios27-runtime-poc:functions")
	}

	switch id {
	case "E01", "BRG27":
		reqUIAssertions["multi_diagnostics"] = "true"
		*testIDs = append(*testIDs, "CherriCoreIntegrationTests/testE01MultiDiagnosticAnalysis")
	case "E02", "RP40":
		reqUIAssertions["completion_hover_sig_def"] = "true"
		*testIDs = append(*testIDs, "CherriCoreIntegrationTests/testE02CompletionHoverSignatureDefinition")
	case "E03":
		reqUIAssertions["scoped_rename"] = "true"
		*testIDs = append(*testIDs, "CherriCoreIntegrationTests/testE03ScopedRenameIgnoresStringsCommentsAndShadowing")
	case "E04", "RP39":
		reqUIAssertions["formatter_idempotent_comments"] = "true"
		*testIDs = append(*testIDs, "CherriCoreIntegrationTests/testE04FormatterPreservesCommentsAndSemantics")
	case "E05":
		reqUIAssertions["versioned_edits_isolated"] = "true"
		*testIDs = append(*testIDs, "CherriCoreIntegrationTests/testE05OutOfOrderVersionedEdits")
	case "E06":
		reqUIAssertions["utf16_spans_exact"] = "true"
		*testIDs = append(*testIDs, "CherriCoreIntegrationTests/testE06UTF16DiagnosticSpansEmojiAndHebrew")
	case "E07", "RP41", "BRG28":
		reqUIAssertions["visual_source_edit_bounded"] = "true"
		*testIDs = append(*testIDs, "CherriCoreIntegrationTests/testE07BoundedVisualSourceEditAndRejection")
	case "BRG25":
		reqUIAssertions["shared_analysis_bridge"] = "true"
		*testIDs = append(*testIDs, "CherriCoreIntegrationTests/testBRG25SharedAnalysisBridgeContract")
	case "P05":
		reqUIAssertions["unicode_identifiers"] = "true"
		*testIDs = append(*testIDs, "CherriCoreIntegrationTests/testUnicodeIdentifierCompilationAndAnalysis")
	}

	if hasLevel(tiers, "ci") {
		*testIDs = append(*testIDs, "tools/language-acceptance:runCI", "github-actions:Build & Test", "github-actions:OpenMinis Skill")
	}
	if hasLevel(tiers, "skill-package") {
		*testIDs = append(*testIDs, "skills/cherri-shortcuts/scripts/self-test.sh")
	}
}

// BuildRequirementsEvidenceMap builds the complete requirements-to-evidence map covering all 94 acceptance, 52 repair, and 33 recovery gate requirements.
func BuildRequirementsEvidenceMap(contracts *ContractsSet) (*RequirementsEvidenceMapFile, error) {
	if contracts == nil || contracts.Acceptance == nil || contracts.Repair == nil || contracts.Gates == nil {
		return nil, fmt.Errorf("all contracts must be loaded before building requirements-evidence map")
	}

	doc := &RequirementsEvidenceMapFile{
		SchemaVersion:     "1",
		GeneratedAt:       time.Now().UTC().Format(time.RFC3339),
		Authority:         "CHERRI-CANONICAL-BACKEND-RECOVERY.md Appendix A & Section 4.4",
		AcceptanceCount:   len(contracts.Acceptance.Cases),
		RepairCount:       len(contracts.Repair.Cases),
		GatesCount:        len(contracts.Gates.Gates),
		TotalRequirements: len(contracts.Acceptance.Cases) + len(contracts.Repair.Cases) + len(contracts.Gates.Gates),
		Mappings:          make(map[string]RequirementEvidenceMapping),
		Records:           make([]RequirementEvidenceMapping, 0, 180),
	}

	// 1. Acceptance Contract Cases (94)
	for _, c := range contracts.Acceptance.Cases {
		tiers := parseRequiredTiers(c.MinimumTestLevel)
		testIDs := deriveAcceptanceTestIDs(c.ID, c.Group)
		fixtures := deriveAcceptanceFixtures(c.ID, c.Group)
		reqAssertions := make(map[string]string)
		reqUIAssertions := make(map[string]string)
		applyRuntimeAndUIMappings(c.ID, tiers, &testIDs, reqAssertions, reqUIAssertions)

		assertions := []string{
			fmt.Sprintf("Requirement satisfied: %s", c.Requirement),
			fmt.Sprintf("Expected behavior verified: %s", c.Expected),
		}
		resultRefs := []string{
			fmt.Sprintf("artifacts/backend-recovery/local/acceptance-results.json#%s", c.ID),
		}

		mapping := RequirementEvidenceMapping{
			ID:                   c.ID,
			Type:                 "acceptance",
			Title:                c.Requirement,
			RequiredTiers:        tiers,
			TestIDs:              testIDs,
			RequiredAssertions:   reqAssertions,
			RequiredUIAssertions: reqUIAssertions,
			FixtureIdentities:    fixtures,
			Assertions:           assertions,
			ResultReferences:     resultRefs,
		}
		doc.Mappings[c.ID] = mapping
		doc.Records = append(doc.Records, mapping)
	}

	// 2. Repair Regression Cases (52)
	for _, c := range contracts.Repair.Cases {
		tiers := make([]string, 0, len(c.RequiredTestLevels))
		for _, l := range c.RequiredTestLevels {
			tiers = append(tiers, parseRequiredTiers(l)...)
		}
		testIDs := deriveRepairTestIDs(c.ID, c.Title)
		fixtures := deriveRepairFixtures(c.ID)
		reqAssertions := make(map[string]string)
		reqUIAssertions := make(map[string]string)
		applyRuntimeAndUIMappings(c.ID, tiers, &testIDs, reqAssertions, reqUIAssertions)

		assertions := []string{
			fmt.Sprintf("Scenario verified: %s", c.Scenario),
			fmt.Sprintf("Expected observation: %s", c.ExpectedObservation),
		}
		resultRefs := []string{
			fmt.Sprintf("docs/language-v2/repair-regression-plan.json#%s", c.ID),
		}

		mapping := RequirementEvidenceMapping{
			ID:                   c.ID,
			Type:                 "repair",
			Title:                c.Title,
			RequiredTiers:        tiers,
			TestIDs:              testIDs,
			RequiredAssertions:   reqAssertions,
			RequiredUIAssertions: reqUIAssertions,
			FixtureIdentities:    fixtures,
			Assertions:           assertions,
			ResultReferences:     resultRefs,
		}
		doc.Mappings[c.ID] = mapping
		doc.Records = append(doc.Records, mapping)
	}

	// 3. Recovery Gates (33)
	for _, g := range contracts.Gates.Gates {
		tiers := make([]string, 0, len(g.MinimumEvidenceTiers))
		for _, l := range g.MinimumEvidenceTiers {
			tiers = append(tiers, parseRequiredTiers(l)...)
		}
		testIDs := deriveGateTestIDs(g.ID, g.ProposedTestPrefix)
		fixtures := deriveGateFixtures(g.ID)
		reqAssertions := make(map[string]string)
		reqUIAssertions := make(map[string]string)
		applyRuntimeAndUIMappings(g.ID, tiers, &testIDs, reqAssertions, reqUIAssertions)

		assertions := []string{
			fmt.Sprintf("Scenario: %s", g.Scenario),
			fmt.Sprintf("Expected observation: %s", g.ExpectedObservation),
		}
		resultRefs := []string{
			fmt.Sprintf("tests/language-v2/backend-recovery-gates.json#%s", g.ID),
		}

		mapping := RequirementEvidenceMapping{
			ID:                   g.ID,
			Type:                 "recovery_gate",
			Title:                g.Title,
			RequiredTiers:        tiers,
			TestIDs:              testIDs,
			RequiredAssertions:   reqAssertions,
			RequiredUIAssertions: reqUIAssertions,
			FixtureIdentities:    fixtures,
			Assertions:           assertions,
			ResultReferences:     resultRefs,
		}
		doc.Mappings[g.ID] = mapping
		doc.Records = append(doc.Records, mapping)
	}

	return doc, nil
}

func deriveAcceptanceTestIDs(id, group string) []string {
	prefix := "tools/language-acceptance:"
	switch group {
	case "baseline":
		return []string{prefix + "runBaseline"}
	case "upstream":
		return []string{prefix + "runUpstream"}
	case "parser":
		return []string{prefix + "runParser"}
	case "binding":
		return []string{prefix + "runBinding"}
	case "calls":
		return []string{prefix + "runCalls"}
	case "types":
		return []string{prefix + "runTypes"}
	case "text":
		return []string{prefix + "runText"}
	case "collections":
		return []string{prefix + "runCollections"}
	case "numbers":
		return []string{prefix + "runNumbers"}
	case "control":
		return []string{prefix + "runControl"}
	case "functions":
		return []string{prefix + "runFunctions"}
	case "metadata":
		return []string{prefix + "runMetadata"}
	case "modules":
		return []string{prefix + "runModules"}
	case "schema":
		return []string{prefix + "runSchema"}
	case "native":
		return []string{prefix + "runNative", "github.com/electrikmilk/cherri:TestActionParityMatrix"}
	case "editor":
		return []string{prefix + "runEditor"}
	case "cli":
		return []string{prefix + "runCLI"}
	case "docs":
		return []string{prefix + "runDocs", "skills/cherri-shortcuts/scripts/self-test.sh"}
	case "skill":
		return []string{prefix + "runSkill", "skills/cherri-shortcuts/scripts/self-test.sh"}
	case "verification":
		return []string{prefix + "runVerification"}
	case "evaluation":
		return []string{prefix + "runEvaluation"}
	case "delivery":
		return []string{prefix + "runDelivery"}
	default:
		return []string{prefix + "run" + group}
	}
}

func deriveAcceptanceFixtures(id, group string) []string {
	switch group {
	case "baseline":
		return []string{"git:rev-parse:HEAD", "docs/language-v2/baseline-catalog.json"}
	case "upstream":
		return []string{"actions/*.cherri", "internal/language/syntax"}
	case "schema":
		return []string{"actions/*.cherri", "catalog.json"}
	case "native":
		return []string{"action_parity_test.go:fixtures", "tests/corpus-batch001.cherri"}
	case "skill":
		return []string{"skills/cherri-shortcuts/SKILL.md"}
	case "docs":
		return []string{"docs/language-v2"}
	default:
		return []string{"tests/language-v2/acceptance-contract.json#" + id}
	}
}

func deriveRepairTestIDs(id, title string) []string {
	base := []string{"tools/language-acceptance:runRepair"}
	switch id {
	case "RP01", "RP02", "RP03":
		return append(base,
			"github.com/electrikmilk/cherri/tools/language-acceptance:TestMeta_ContradictoryPassedStatus",
			"github.com/electrikmilk/cherri/tools/language-acceptance:TestMeta_WeakTestLevelRejection",
			"github.com/electrikmilk/cherri/tools/language-acceptance:TestMeta_FailedOrPendingCIRejection",
		)
	case "RP04":
		return append(base,
			"github.com/electrikmilk/cherri:TestDecompileStateIsolation",
			"github.com/electrikmilk/cherri:TestDecompIncludeStateIsolation",
		)
	case "RP07", "RP08":
		return append(base,
			"github.com/electrikmilk/cherri:TestBackendRecoveryInitial_TransformsSurviveEmission",
			"github.com/electrikmilk/cherri:TestActionParityMatrix/text_token_variable_input",
		)
	case "RP09", "RP10", "RP11", "RP12":
		return append(base,
			"github.com/electrikmilk/cherri:TestActionParityMatrix",
		)
	case "RP37":
		return append(base,
			"github.com/electrikmilk/cherri/internal/shortcutcompare:TestMutation01_SwapProducerUUID",
			"github.com/electrikmilk/cherri/internal/shortcutcompare:TestMutation02_NonexistentProducer",
		)
	case "RP42", "RP44", "RP45", "RP46":
		return append(base, "skills/cherri-shortcuts/scripts/self-test.sh")
	case "RP49":
		return append(base,
			"scripts/verify-backend-recovery.sh:step1",
			"skills/cherri-shortcuts/scripts/self-test.sh",
		)
	default:
		return append(base, fmt.Sprintf("tests/language-v2/repair:%s", id))
	}
}

func deriveRepairFixtures(id string) []string {
	return []string{"tests/language-v2/repair-regression-plan.json#" + id}
}

func deriveGateTestIDs(id, proposedPrefix string) []string {
	base := []string{"tools/language-acceptance:runGates"}
	switch id {
	case "BRG01":
		return append(base, "scripts/verify-backend-recovery.sh")
	case "BRG02":
		return append(base,
			"github.com/electrikmilk/cherri/internal/shortcutcompare:TestAlphaRenamingPasses",
			"github.com/electrikmilk/cherri/internal/shortcutcompare:TestMutation01_SwapProducerUUID",
			"github.com/electrikmilk/cherri/internal/shortcutcompare:TestMutation02_NonexistentProducer",
			"github.com/electrikmilk/cherri/internal/shortcutcompare:TestMutation03_ChangeVariableName",
		)
	case "BRG03":
		return append(base,
			"github.com/electrikmilk/cherri/internal/shortcutcompare:TestMutation04_ReplaceTokenWithString",
			"github.com/electrikmilk/cherri/internal/shortcutcompare:TestMutation05_Transformations",
			"github.com/electrikmilk/cherri/internal/shortcutcompare:TestMutation06_ShiftAttachmentRange",
			"github.com/electrikmilk/cherri/internal/shortcutcompare:TestMutation07_NestedKeyAsymmetry",
			"github.com/electrikmilk/cherri/internal/shortcutcompare:TestMutation08_ScalarTypesAndOmission",
			"github.com/electrikmilk/cherri/internal/shortcutcompare:TestMutation09_ReorderActionsAndGroupMarkers",
			"github.com/electrikmilk/cherri/internal/shortcutcompare:TestMutation10_RebindLoopReference",
			"github.com/electrikmilk/cherri/internal/shortcutcompare:TestMutation11_StaticVariantOrAppIntent",
			"github.com/electrikmilk/cherri/internal/shortcutcompare:TestMutation12_RetargetImportQuestion",
			"github.com/electrikmilk/cherri/internal/shortcutcompare:TestMutation13_ChangeOrdinaryUUIDLiteral",
		)
	case "BRG04":
		return append(base,
			"github.com/electrikmilk/cherri/tools/language-acceptance:TestMeta_AssertionReturnsFalse",
			"github.com/electrikmilk/cherri/tools/language-acceptance:TestMeta_ContradictoryPassedStatus",
			"github.com/electrikmilk/cherri/tools/language-acceptance:TestMeta_MissingRequiredID",
			"github.com/electrikmilk/cherri/tools/language-acceptance:TestMeta_UnknownIDInjected",
			"github.com/electrikmilk/cherri/tools/language-acceptance:TestMeta_ContractHashMismatch",
			"github.com/electrikmilk/cherri/tools/language-acceptance:TestMeta_ZeroMatchingTestsDetected",
			"github.com/electrikmilk/cherri/tools/language-acceptance:TestMeta_WeakTestLevelRejection",
			"github.com/electrikmilk/cherri/tools/language-acceptance:TestMeta_MismatchedEvidenceDigest",
			"github.com/electrikmilk/cherri/tools/language-acceptance:TestMeta_MissingReferencedArtifact",
			"github.com/electrikmilk/cherri/tools/language-acceptance:TestMeta_ArtifactHashMismatch",
			"github.com/electrikmilk/cherri/tools/language-acceptance:TestMeta_FailedOrPendingCIRejection",
			"github.com/electrikmilk/cherri/tools/language-acceptance:TestMeta_DuplicateEvidenceID",
		)
	case "BRG05", "BRG07", "BRG08", "BRG09", "BRG10":
		return append(base,
			"github.com/electrikmilk/cherri:TestActionParityMatrix",
			"github.com/electrikmilk/cherri:TestBackendRecoveryInitial_TransformsSurviveEmission",
		)
	case "BRG06":
		return append(base,
			"github.com/electrikmilk/cherri:TestDecompileStateIsolation",
			"github.com/electrikmilk/cherri:TestDecompIncludeStateIsolation",
		)
	case "BRG14", "BRG15", "BRG28":
		return append(base,
			"github.com/electrikmilk/cherri:TestDecomp",
			"github.com/electrikmilk/cherri:TestDecompileTextPartsKeepsEmptyCustomSeparator",
			"github.com/electrikmilk/cherri:TestDecompIncludeReconstructionMultiCategory",
		)
	case "BRG19", "BRG20", "BRG25", "BRG26":
		return append(base,
			"github.com/electrikmilk/cherri:ios_bridge:compileForMobile",
			"github.com/electrikmilk/cherri:signing:SignShortcutBytes",
		)
	case "BRG30":
		return append(base, "skills/cherri-shortcuts/scripts/self-test.sh")
	default:
		if proposedPrefix != "" {
			return append(base, fmt.Sprintf("tests/language-v2/recovery:%s", proposedPrefix))
		}
		return append(base, fmt.Sprintf("tests/language-v2/recovery:%s", id))
	}
}

func deriveGateFixtures(id string) []string {
	return []string{"tests/language-v2/backend-recovery-gates.json#" + id}
}

// WriteRequirementsEvidenceMap writes the generated requirements-evidence map to disk.
func WriteRequirementsEvidenceMap(doc *RequirementsEvidenceMapFile, targetPath string) error {
	bytes, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(targetPath, bytes, 0644)
}
