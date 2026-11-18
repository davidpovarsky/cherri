package main

import "testing"

func TestSkillOfflineProductionEvidence(t *testing.T) {
	testNamedProductionEvidence(t, "K04", skillProductionTests["K04"], nil, (*Runner).runSkillOfflineRequirement,
		"P15_SKILL_OFFLINE_RECEIPT:",
		"Only K04 actual compiler offline cache, isolated docs/schema/commit rejection, missing docs and observable network denial. No fresh-install, package, atomic-update, signing, installed-consumer or release closure.")
}

func TestSkillInstallProductionEvidence(t *testing.T) {
	testNamedProductionEvidence(t, "K02", skillProductionTests["K02"], nil, (*Runner).runSkillInstallRequirement,
		"P15_SKILL_INSTALL_RECEIPT:",
		"Only actual fresh source install and verified cache/bundle/explicit binary selection with isolated failures. No package, signing, native or installed-remote release closure.")
}

func TestSkillUpdateProductionEvidence(t *testing.T) {
	testNamedProductionEvidence(t, "K03", skillProductionTests["K03"], nil, (*Runner).runSkillUpdateRequirement,
		"P15_SKILL_UPDATE_RECEIPT:",
		"Only exact clean pinned source installation, pre-execution download checksum, controlled late validation/promotion rollback and successful update. No power-loss, concurrent-reader, packaging, signing, or live-consumer claim.")
}

func TestSkillPackageProductionEvidence(t *testing.T) {
	testNamedProductionEvidence(t, "K01", skillProductionTests["K01"], nil, (*Runner).runSkillPackageRequirement,
		"P15_SKILL_PACKAGE_RECEIPT:",
		"Only committed source/runtime ZIP inventory, actual clean pinned compiler provenance, unpacked real CLI self-test and rejected archive/source corruption. No remote installation, native runtime, signing or broad repair/release closure.")
}
