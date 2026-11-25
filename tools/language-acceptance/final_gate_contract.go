package main

import (
	"fmt"
	"reflect"
)

const finalGateRP49Title = "Final SHA and artifact binding"
const finalGateRP49Scenario = "Collect all required event types/attempts and artifact metadata for final source F; inject earlier-SHA and missing artifacts."
const finalGateRP49Expected = "Only completed successful required gates and compatible F artifacts satisfy closure; pending/skipped/stale evidence does not."
const finalGateBRG31Title = "Actual encoding negative gate and exact-SHA CI"
const finalGateBRG31Scenario = "Intentionally violate required EXPECT in actual verifier, then run final real workflows and target computed assertions with artifact identity validation."
const finalGateBRG31Expected = "Negative subprocess nonzero; positive final jobs successful at delivered revision; no skipped/cancelled/stale execution accepted."

func finalGateValidateContracts(contracts *ContractsSet) error {
	if contracts == nil || contracts.Repair == nil || contracts.Gates == nil {
		return fmt.Errorf("FINAL_GATE_IMMUTABLE_CONTRACT")
	}
	repair, gate := 0, 0
	for _, row := range contracts.Repair.Cases {
		if row.ID != "RP49" {
			continue
		}
		repair++
		if row.Title != finalGateRP49Title || row.Scenario != finalGateRP49Scenario || row.ExpectedObservation != finalGateRP49Expected ||
			!reflect.DeepEqual(row.RequiredTestLevels, []string{"repository", "CI", "packaging"}) || !reflect.DeepEqual(row.OriginalRequirementIDs, []string{"CI03", "END01"}) {
			return fmt.Errorf("RP49_IMMUTABLE_CONTRACT")
		}
	}
	for _, row := range contracts.Gates.Gates {
		if row.ID != "BRG31" {
			continue
		}
		gate++
		if row.Title != finalGateBRG31Title || row.Scenario != finalGateBRG31Scenario || row.ExpectedObservation != finalGateBRG31Expected || !row.Mandatory || row.ProposedTestPrefix != "TestBackendRecovery31" ||
			!reflect.DeepEqual(row.MinimumEvidenceTiers, finalGateTiers("BRG31")) || !reflect.DeepEqual(row.RelatedOriginalIDs, []string{"CI01", "CI02", "CI03", "CI04", "END01"}) || !reflect.DeepEqual(row.RelatedRepairIDs, []string{"RP47", "RP48", "RP49"}) {
			return fmt.Errorf("BRG31_IMMUTABLE_CONTRACT")
		}
	}
	if repair != 1 || gate != 1 {
		return fmt.Errorf("FINAL_GATE_IMMUTABLE_INVENTORY")
	}
	return nil
}

// Both contexts may locate independent exact copies at different paths. Their
// complete file digests and all publication/run identities must still agree.
func finalGateSameBindings(a, b finalProvenanceExpected) bool {
	return reflect.DeepEqual(a.Sources, b.Sources) && reflect.DeepEqual(a.Files, b.Files) && reflect.DeepEqual(a.Publications, b.Publications) && reflect.DeepEqual(a.Runs, b.Runs) && reflect.DeepEqual(a.Artifacts, b.Artifacts)
}

func finalGateValidateIdentitySelection(in *finalProvenanceInputs, identity ciIdentityExpected, planFile string, selections map[string]finalGateSelection) error {
	plan, err := finalGateReadPlan(in, planFile)
	if err != nil {
		return err
	}
	for role := range ciIdentityRequiredWorkflows {
		matched := 0
		for _, intent := range plan.Required {
			if intent.Role == role && reflect.DeepEqual(identity.Checkouts[role], selections[intent.Key].Checkouts) {
				matched++
			}
		}
		if matched != 1 {
			return fmt.Errorf("FINAL_GATE_CI03_SELECTED_EXECUTION_JOIN")
		}
	}
	return nil
}

func finalGateSigningSelection(expected finalGateExpected, publication publicationSecurityExpected) error {
	if expected.Native == nil {
		return fmt.Errorf("BRG31_RUNTIME_SIGNING_SELECTION")
	}
	wanted := map[string]bool{}
	for name := range finalGateRuntimeSources {
		wanted[name] = true
	}
	seen := map[string]bool{}
	for _, intent := range publication.SigningIntents {
		if intent.Artifact != expected.Native.RuntimeArtifact {
			continue
		}
		if !wanted[intent.FixtureID] || seen[intent.FixtureID] || intent.Member != intent.FixtureID+".shortcut" || intent.FixtureSHA256 != finalGateRuntimeSources[intent.FixtureID] {
			return fmt.Errorf("BRG31_RUNTIME_SIGNING_SELECTION")
		}
		seen[intent.FixtureID] = true
	}
	if len(seen) != len(wanted) {
		return fmt.Errorf("BRG31_RUNTIME_SIGNING_SELECTION")
	}
	return nil
}
