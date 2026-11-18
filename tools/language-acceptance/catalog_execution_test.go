package main

import "testing"

func TestCatalogBaselineProductionEvidence(t *testing.T) {
	testNamedProductionEvidence(t, "B02", catalogProductionTests["B02"], nil,
		(*Runner).runCatalogBaselineRequirement, "P15_CATALOG_BASELINE_RECEIPT",
		"Archived definition identity reconciliation and current representative native output capture only; no historical plist parity or runtime claim")
}

func TestCatalogAccountingProductionEvidence(t *testing.T) {
	testNamedProductionEvidence(t, "AC01", catalogProductionTests["AC01"], nil,
		(*Runner).runCatalogAccountingRequirement, "P15_CATALOG_ACCOUNTING_RECEIPT",
		"Complete canonical definition/variant membership, real construct grammar and actual regenerated schema; no Apple runtime or release closure")
}
