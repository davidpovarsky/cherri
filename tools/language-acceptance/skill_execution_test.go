package main

import "testing"

func TestSkillOfflineProductionEvidence(t *testing.T) {
	testNamedProductionEvidence(t, "K04", skillProductionTests["K04"], nil, (*Runner).runSkillOfflineRequirement,
		"P15_SKILL_OFFLINE_RECEIPT:",
		"Only K04 actual compiler offline cache, isolated docs/schema/commit rejection, missing docs and observable network denial. No fresh-install, package, atomic-update, signing, installed-consumer or release closure.")
}
