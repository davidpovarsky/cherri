#!/usr/bin/env python3
"""tools/p10r3/test_coverage_and_gates.py

Unit and negative mutation tests verifying P10R3 corrective audit invariants:
- F-01: 505 Catalog Coverage Matrix schema & anti-tautology fail-closed checks
- F-02: Denominator Crosswalk exact arithmetic (2396 -> 1527 -> 390 -> 505)
- F-03: 56 Release Gates (R01..R28, A01..A28), R22 platform-aware, R23 bcc0095..HEAD + working tree
- F-04: 15 Go-Only actions exact names and classification
- F-06: Device Acceptance Manifest 9 fixtures and BLOCKED_DEVICE state
- F-08: Authoring Patterns ZIP (BLOCKED_MISSING_ARTIFACT) and Owner Gate (OWNER_APPROVAL_REQUIRED)
"""

import copy
import json
import pathlib
import sys
import unittest

REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent.parent
sys.path.insert(0, str(REPO_ROOT / "tools" / "p10r3"))

import generate_catalog_and_pack_ledgers as gen
import run_release_gates as gates
import verify_or_generate_pack as pack_ver


class TestP10CoverageAndGates(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.p10_dir = REPO_ROOT / "docs" / "apple-runtime" / "p10"
        cls.matrix = json.loads((cls.p10_dir / "p10-505-catalog-coverage-matrix.json").read_text(encoding="utf-8"))
        cls.schema = json.loads((cls.p10_dir / "coverage-evidence.schema.json").read_text(encoding="utf-8"))
        cls.crosswalk = json.loads((cls.p10_dir / "p10-apple-cherri-crosswalk.json").read_text(encoding="utf-8"))
        cls.pack15 = json.loads((cls.p10_dir / "p10-pack-15-go-only-audit.json").read_text(encoding="utf-8"))
        cls.dev_manifest = json.loads((cls.p10_dir / "device-acceptance-manifest.json").read_text(encoding="utf-8"))
        cls.baseline = json.loads((cls.p10_dir / "baseline-state.json").read_text(encoding="utf-8"))

    def test_f01_matrix_passes_schema_and_has_real_execution_refs(self):
        gen.validate_against_schema(self.matrix, self.schema)
        self.assertEqual(len(self.matrix["entries"]), 505)
        for e in self.matrix["entries"]:
            self.assertGreaterEqual(e["sourceLine"], 1)
            self.assertIn("TestP10_VerifyAndRecord505CatalogExecution", e["unitTestRef"])

    def test_f01_negative_mutation_fake_unit_test_ref_fails_closed(self):
        mutated = copy.deepcopy(self.matrix)
        mutated["entries"][0]["unitTestRef"] = "actions_catalog_test.go"
        with self.assertRaises(ValueError):
            gen.validate_against_schema(mutated, self.schema)

    def test_f01_negative_mutation_invalid_source_line_fails_closed(self):
        mutated = copy.deepcopy(self.matrix)
        mutated["entries"][10]["sourceLine"] = 0
        with self.assertRaises(ValueError):
            gen.validate_against_schema(mutated, self.schema)

    def test_f01_negative_mutation_invalid_decompile_mode_fails_closed(self):
        mutated = copy.deepcopy(self.matrix)
        mutated["entries"][5]["decompileMode"] = "native_or_lossless_rawAction"
        with self.assertRaises(ValueError):
            gen.validate_against_schema(mutated, self.schema)

    def test_f02_denominator_crosswalk_arithmetic(self):
        d = self.crosswalk["denominators"]
        self.assertEqual(d["appleRuntimeDiscoveredTotal"], 2396)
        self.assertEqual(d["appleEligibleUserFacingTotal"], 1527)
        self.assertEqual(d["appleExcludedInternalOrNonRunnableTotal"], 869)
        self.assertEqual(
            d["appleEligibleUserFacingTotal"] + d["appleExcludedInternalOrNonRunnableTotal"],
            d["appleRuntimeDiscoveredTotal"],
        )
        self.assertEqual(d["appleCoveredUniqueIdentifiers"], 390)
        self.assertEqual(d["appleUncoveredEligibleIdentifiers"], 1137)
        self.assertEqual(
            d["appleCoveredUniqueIdentifiers"] + d["appleUncoveredEligibleIdentifiers"],
            d["appleEligibleUserFacingTotal"],
        )
        self.assertEqual(d["cherriCatalogTotalEntries"], 505)
        self.assertEqual(d["cherriCallableActions"] + d["cherriCompilerConstructs"], 505)

    def test_f03_gate_registry_and_negative_mutation(self):
        self.assertTrue(gates.validate_gate_registry(list(gates.GATE_DEFINITIONS.keys())))
        with self.assertRaises(ValueError):
            gates.validate_gate_registry([g for g in gates.ALL_REQUIRED_GATES if g != "R23"])

        # Verify R23 executes git diff 76d7225..HEAD and git status --porcelain
        r23_res = gates.execute_gate("R23", gates.GATE_DEFINITIONS["R23"])
        self.assertEqual(r23_res["status"], "PASS")
        self.assertIn("76d722502e20db977e84a095444d0172442655b8..HEAD", r23_res["command"])

    def test_f04_15_go_only_actions_exact_set_and_classification(self):
        items = self.pack15["goOnlyActionsAudit"]
        names = {x["actionName"] for x in items}
        self.assertEqual(names, set(pack_ver.EXPECTED_15_GO_ONLY.keys()))
        for item in items:
            self.assertEqual(
                item.get("classification"),
                pack_ver.EXPECTED_15_GO_ONLY[item["actionName"]],
            )

    def test_f06_device_acceptance_manifest_9_fixtures_blocked_device(self):
        self.assertEqual(self.dev_manifest["totalFixtures"], 9)
        self.assertEqual(self.dev_manifest["overallStatus"], "COMPILED_UNSIGNED_PENDING_DEVICE")
        for f in self.dev_manifest["fixtures"]:
            self.assertEqual(f["executionStatus"], "COMPILED_UNSIGNED_PENDING_DEVICE")
            self.assertEqual(f["deviceBlocker"], "BLOCKED_DEVICE")
            self.assertEqual(len(f["sourceSha256"]), 64)
            self.assertTrue(f["roundTripVerified"])

    def test_f08_external_blockers_explicit(self):
        eb = self.baseline["externalBlockers"]
        self.assertEqual(eb["authoringPatternsZip_1219604109827689"], "BLOCKED_MISSING_ARTIFACT")
        self.assertEqual(eb["ownerReleaseGate_1219611682706483"], "OWNER_APPROVAL_REQUIRED")
        self.assertEqual(eb["physicalIPadOS27"], "BLOCKED_DEVICE")
        self.assertEqual(eb["liveAppleServerSigning"], "BLOCKED_EXTERNAL_SIGNING")


if __name__ == "__main__":
    unittest.main()
