package main

import "strings"

// Every declared obligation has its own result. A repair/gate linkage is not
// execution evidence, but it is also not an unknown acceptance identifier.
func allLocalContracts(contracts *ContractsSet) *ContractFile {
	var combined = *contracts.Acceptance
	combined.Cases = append([]CaseSpec{}, contracts.Acceptance.Cases...)
	for _, c := range contracts.Repair.Cases {
		combined.Cases = append(combined.Cases, CaseSpec{
			ID: c.ID, Group: "repair", Requirement: c.Title,
			MinimumTestLevel: strings.Join(c.RequiredTestLevels, "+"), Expected: c.ExpectedObservation,
		})
	}
	for _, g := range contracts.Gates.Gates {
		combined.Cases = append(combined.Cases, CaseSpec{
			ID: g.ID, Group: "backend-recovery", Requirement: g.Title,
			MinimumTestLevel: strings.Join(g.MinimumEvidenceTiers, "+"), Expected: g.ExpectedObservation,
		})
	}
	combined.CaseCount = len(combined.Cases)
	return &combined
}
