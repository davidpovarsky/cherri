package main

import "testing"

func TestTypePlatformVersionProductionEvidence(t *testing.T) {
	testTypeProductionEvidence(t, "T07", (*Runner).runTypePlatformVersionRequirement,
		"P15_TYPE_PLATFORM_VERSION_RECEIPT:",
		"Only T07 production unit contract: independent synthetic per-platform component bounds, original canonical emission, malformed metadata rejection, missing facts kept unverified, service isolation and exact schema fingerprint/clone behavior. No inferred Apple availability facts or runtime/release guarantee.")
}
