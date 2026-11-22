#!/usr/bin/env python3
"""tools/p10r3/test_coverage_and_gates.py

Unit and negative mutation tests verifying P10R4 corrective audit invariants:
- PART A & B (F-01, F-02): 505 Catalog Coverage Matrix schema, 13 statuses, 7 coverage dimensions,
  evidenceTier anti-inflation, and 11-way ledger/crosswalk/report consistency.
- PART C (F-03): 56 Release Gates (R01..R28, A01..A28), R22 granular blocker classes,
  R23 76d7225..HEAD + working tree, inputDependencyFingerprint verification, and 10 negative gate tests.
- PART E (F-04): Pure+Pack deterministic generator (`--generate`, `--verify-generated`),
  7 classification buckets, and 15 Go-only actions consistency.
- F-06 & F-08: Device Acceptance Manifest 9 fixtures (`BLOCKED_DEVICE`) and external blockers.
"""

import copy
import hashlib
import json
import os
import pathlib
import sys
import subprocess
import tempfile
import unittest
from unittest import mock

REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent.parent
sys.path.insert(0, str(REPO_ROOT / "tools" / "p10r3"))

import generate_catalog_and_pack_ledgers as gen
import run_release_gates as gates
import verify_ledger_consistency as consistency
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

    # --- PART A & B: Matrix Schema & Anti-Tautology ---
    def test_f01_matrix_passes_schema_and_has_real_execution_refs(self):
        gen.validate_against_schema(self.matrix, self.schema)
        self.assertEqual(len(self.matrix["entries"]), 505)
        self.assertEqual(self.matrix["summary"]["structuralRoundTripVerifiedCount"], 505)
        self.assertEqual(self.matrix["summary"]["parameterContractVerifiedCount"], 505)
        for e in self.matrix["entries"]:
            self.assertGreaterEqual(e["sourceLine"], 1)
            self.assertIn("TestP10_VerifyAndRecord505CatalogExecution", e["unitTestRef"])
            self.assertTrue(e["structuralRoundTripPass"])
            self.assertIn("STRUCTURAL_ROUNDTRIP_PASS", e["verificationStatuses"])
            self.assertIn("PARAMETER_CONTRACT_PASS", e["verificationStatuses"])
            # Ensure macOS static extraction is never inflated to APPLE_RUNTIME_VERIFIED without APPLE_RUNTIME_EXECUTED
            if e["appleRuntimeExecutedStatus"] != "APPLE_RUNTIME_EXECUTED":
                self.assertNotEqual(e["evidenceTier"], "APPLE_RUNTIME_VERIFIED")
            if e["nativeSpecimenParityStatus"] != "NATIVE_SPECIMEN_PARITY_PASS":
                self.assertNotEqual(e["evidenceTier"], "NATIVE_SPECIMEN_VERIFIED")
            self.assertIn(
                e["evidenceTier"],
                ("COMPILER_VERIFIED", "STRUCTURAL_ROUNDTRIP_VERIFIED", "NATIVE_SPECIMEN_VERIFIED", "APPLE_RUNTIME_VERIFIED"),
            )

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

    def test_f01_negative_mutation_inflated_confirmed_runtime_fails_closed(self):
        mutated = copy.deepcopy(self.matrix)
        mutated["entries"][0]["evidenceTier"] = "APPLE_RUNTIME_VERIFIED"
        mutated["entries"][0]["appleRuntimeExecutedStatus"] = "NEEDS_EVIDENCE"
        with self.assertRaises(ValueError):
            gen.validate_against_schema(mutated, self.schema)

        mutated_specimen = copy.deepcopy(self.matrix)
        mutated_specimen["entries"][0]["evidenceTier"] = "NATIVE_SPECIMEN_VERIFIED"
        mutated_specimen["entries"][0]["nativeSpecimenParityStatus"] = "NEEDS_EVIDENCE"
        with self.assertRaises(ValueError):
            gen.validate_against_schema(mutated_specimen, self.schema)

    # --- PART B: Ledger, Crosswalk & 11-Surface Consistency ---
    def test_f02_denominator_crosswalk_arithmetic_and_ledger_truth(self):
        d = self.crosswalk["denominators"]
        self.assertEqual(d["appleRuntimeDiscoveredTotal"], 2396)
        self.assertEqual(d["appleEligibleUserFacingTotal"], 1527)
        self.assertEqual(d["appleEligibleBuiltinTotal"], 487)
        self.assertEqual(d["appleEligibleAppIntentTotal"], 1040)
        self.assertEqual(d["appleExcludedInternalOrNonRunnableTotal"], 869)
        self.assertEqual(
            d["appleEligibleUserFacingTotal"] + d["appleExcludedInternalOrNonRunnableTotal"],
            d["appleRuntimeDiscoveredTotal"],
        )
        self.assertEqual(d["appleCoveredUniqueIdentifiers"], 390)
        self.assertEqual(d["appleCoveredVerifiedIdentifiers"], 0)
        self.assertEqual(d["appleCoveredPartialIdentifiers"], 390)
        self.assertEqual(d["appleUncoveredEligibleIdentifiers"], 1137)
        self.assertEqual(d["appleNewNativeActionIdentifiers"], 97)
        self.assertEqual(d["appleMissingAppIntentDescriptorIdentifiers"], 1040)
        self.assertEqual(
            d["appleNewNativeActionIdentifiers"] + d["appleMissingAppIntentDescriptorIdentifiers"],
            d["appleUncoveredEligibleIdentifiers"],
        )
        self.assertEqual(d["cherriCatalogTotalEntries"], 505)
        self.assertEqual(d["cherriCallableActions"] + d["cherriCompilerConstructs"], 505)
        self.assertEqual(d["uniqueIdentifiersInCherriCatalog"], 390)
        self.assertEqual(d["uniqueCallableIdentifiersInCherriCatalog"], 382)
        self.assertEqual(d["uniqueConstructIdentifiersInCherriCatalog"], 8)
        self.assertEqual(
            d["uniqueCallableIdentifiersInCherriCatalog"] + d["aliasCallableActionsOverSharedIdentifiers"],
            d["cherriCallableActions"],
        )

        ok, errs = consistency.check_consistency(p10_dir=self.p10_dir)
        self.assertTrue(ok, f"Expected 11-way consistency check to pass, got: {errs}")

    def test_f02_negative_mutations_across_crosswalk_and_reports_fail_closed(self):
        for mode in (
            "c5_1_ledger_vs_crosswalk",
            "c5_2_crosswalk_vs_report",
            "c5_3_deferred_double_count",
            "c5_4_unique_identifiers_mismatch",
            "c5_5_evidence_tier_inflated",
            "c5_6_markdown_stale_number",
            "c5_7_parameter_matrix_vs_report",
            "report_verified",
            "crosswalk_verified",
            "gap_md_missing_high",
            "matrix_fake_runtime_tier",
        ):
            ok, errs = consistency.check_consistency(p10_dir=self.p10_dir, mutate_mode=mode)
            self.assertFalse(ok, f"Expected mutation mode {mode} to fail closed")
            self.assertGreater(len(errs), 0)

        # Mutate ledger row status directly via overrides
        ledger_copy = json.loads((self.p10_dir / "p10-action-ledger.json").read_text(encoding="utf-8"))
        ledger_copy[0]["status"] = "COVERED_VERIFIED"
        ok, errs = consistency.check_consistency(p10_dir=self.p10_dir, overrides={"p10-action-ledger.json": ledger_copy})
        self.assertFalse(ok)

    # --- PART C: 10 Negative Release Gate & Fingerprint Tests ---
    def test_c5_01_missing_gate_in_registry_fails_closed(self):
        self.assertTrue(gates.validate_gate_registry(list(gates.GATE_DEFINITIONS.keys())))
        with self.assertRaises(ValueError):
            gates.validate_gate_registry([g for g in gates.ALL_REQUIRED_GATES if g != "R23"])

    def test_c5_02_skipped_go_test_fails_closed(self):
        defn = gates.GATE_DEFINITIONS["R08"]
        res = gates.classify_command_output(
            "R08",
            defn,
            returncode=0,
            combined_output="=== RUN TestP10R_R08\n--- SKIP: TestP10R_R08 (0.00s)\nPASS\n",
            duration_seconds=0.01,
        )
        self.assertEqual(res["status"], "FAIL")
        self.assertIn("skipped", res["error"])

    def test_c5_03_zero_tests_run_fails_closed(self):
        defn = gates.GATE_DEFINITIONS["R09"]
        res = gates.classify_command_output(
            "R09",
            defn,
            returncode=0,
            combined_output="testing: warning: no tests to run\nPASS\n",
            duration_seconds=0.01,
        )
        self.assertEqual(res["status"], "FAIL")

    def test_c5_04_missing_expected_marker_fails_closed(self):
        defn = gates.GATE_DEFINITIONS["R10"]
        res = gates.classify_command_output(
            "R10",
            defn,
            returncode=0,
            combined_output="=== RUN SomeOtherTest\n--- PASS: SomeOtherTest (0.00s)\nPASS\n",
            duration_seconds=0.01,
        )
        self.assertEqual(res["status"], "FAIL")
        self.assertIn("Missing expected output marker", res["error"])

    def test_c5_05_non_zero_exit_code_fails_closed(self):
        defn = gates.GATE_DEFINITIONS["R11"]
        res = gates.classify_command_output(
            "R11",
            defn,
            returncode=1,
            combined_output="FAIL\tgithub.com/electrikmilk/cherri\n",
            duration_seconds=0.01,
        )
        self.assertEqual(res["status"], "FAIL")
        self.assertEqual(res["exitCode"], 1)

    def test_c5_06_r23_protected_ios_file_modified_fails_closed(self):
        # A clean current checkout may contain committed protected changes.
        # Exercise the actual Git gate against an owned, explicit baseline.
        production_definition = copy.deepcopy(gates.GATE_DEFINITIONS["R23"])
        protected = production_definition["checkProtectedIOSFiles"]
        self.assertEqual(production_definition["upstreamBase"],
                         "76d722502e20db977e84a095444d0172442655b8")
        self.assertEqual(protected, [
            "ios/CherriUITests/CherriEditorUITests.swift",
            "ios/CherriApp/Views/CodeEditorView.swift",
        ])
        environment = {key: value for key, value in os.environ.items()
                       if not key.startswith("GIT_")}
        environment.update({
            "GIT_CONFIG_NOSYSTEM": "1",
            "GIT_CONFIG_GLOBAL": os.devnull,
            "GIT_AUTHOR_NAME": "R23 isolated fixture",
            "GIT_AUTHOR_EMAIL": "r23-fixture@example.invalid",
            "GIT_COMMITTER_NAME": "R23 isolated fixture",
            "GIT_COMMITTER_EMAIL": "r23-fixture@example.invalid",
            "GIT_AUTHOR_DATE": "2026-01-01T00:00:00+0000",
            "GIT_COMMITTER_DATE": "2026-01-01T00:00:00+0000",
        })
        with tempfile.TemporaryDirectory(prefix="cherri-r23-git-") as directory:
            fixture = pathlib.Path(directory)
            root = fixture / "repository"
            empty_template = fixture / "empty-template"
            root.mkdir()
            empty_template.mkdir()
            with mock.patch.dict(os.environ, environment, clear=True), \
                    mock.patch.object(gates, "REPO_ROOT", root):
                def git(*arguments):
                    result = subprocess.run(
                        ["git", *arguments], cwd=root, capture_output=True,
                        text=True, encoding="utf-8", timeout=10,
                    )
                    self.assertEqual(result.returncode, 0,
                                     (arguments, result.stdout, result.stderr))
                    return result.stdout

                git("init", "--initial-branch=r23-fixture",
                    "--template=" + str(empty_template))
                git("config", "--local", "core.autocrlf", "false")
                git("config", "--local", "commit.gpgsign", "false")
                git("config", "--local", "core.hooksPath", str(empty_template))
                originals = {}
                for index, relative in enumerate(protected):
                    path = root / relative
                    path.parent.mkdir(parents=True, exist_ok=True)
                    originals[relative] = f"// protected fixture {index}\n".encode()
                    path.write_bytes(originals[relative])
                git("add", "--", *protected)
                git("commit", "-m", "R23 explicit fixture baseline")
                baseline = git("rev-parse", "HEAD").strip()
                self.assertEqual(len(baseline), 40)
                fixture_definition = copy.deepcopy(production_definition)
                fixture_definition["upstreamBase"] = baseline

                def observe(expected, *, historical_changed, working_changed,
                            changed_path=None):
                    historical = git("diff", baseline + "..HEAD", "--", *protected)
                    working = git("status", "--porcelain", "--", *protected)
                    self.assertEqual(bool(historical.strip()), historical_changed)
                    self.assertEqual(bool(working.strip()), working_changed)
                    if changed_path is not None:
                        self.assertIn(changed_path, historical + working)
                    result = gates.execute_gate("R23", fixture_definition)
                    self.assertEqual(result["status"], expected, result)
                    self.assertIn(baseline + "..HEAD", result["command"])
                    self.assertEqual(result["outputSha256"], hashlib.sha256(
                        f"DIFF:\n{historical}\nSTATUS:\n{working}".encode("utf-8")
                    ).hexdigest())

                with self.subTest(state="explicit_baseline_clean"):
                    observe("PASS", historical_changed=False, working_changed=False)
                for relative in protected:
                    path = root / relative
                    with self.subTest(path=relative, state="unstaged_change"):
                        path.write_bytes(originals[relative] + b"// mutation\n")
                        observe("FAIL", historical_changed=False, working_changed=True,
                                changed_path=relative)
                    with self.subTest(path=relative, state="staged_change"):
                        git("add", "--", relative)
                        observe("FAIL", historical_changed=False, working_changed=True,
                                changed_path=relative)
                    with self.subTest(path=relative, state="committed_change_clean_tree"):
                        git("commit", "-m", "Retain protected fixture change")
                        self.assertEqual(git("status", "--porcelain"), "")
                        observe("FAIL", historical_changed=True, working_changed=False,
                                changed_path=relative)
                    with self.subTest(path=relative, state="committed_exact_restoration"):
                        path.write_bytes(originals[relative])
                        git("add", "--", relative)
                        git("commit", "-m", "Restore exact protected fixture bytes")
                        self.assertEqual(path.read_bytes(), originals[relative])
                        observe("PASS", historical_changed=False, working_changed=False)
                with self.subTest(state="unprotected_committed_change"):
                    (root / "outside-protected-scope.txt").write_bytes(b"unrelated\n")
                    git("add", "--", "outside-protected-scope.txt")
                    git("commit", "-m", "Unrelated fixture history")
                    self.assertNotEqual(git("rev-parse", "HEAD").strip(), baseline)
                    self.assertEqual(git("status", "--porcelain"), "")
                    observe("PASS", historical_changed=False, working_changed=False)
                with self.subTest(state="unavailable_baseline_fails_closed"):
                    missing = copy.deepcopy(fixture_definition)
                    missing["upstreamBase"] = "0" * 40
                    self.assertEqual(gates.execute_gate("R23", missing)["status"], "FAIL")
        self.assertEqual(gates.GATE_DEFINITIONS["R23"], production_definition)

    def test_c5_07_non_ancestor_source_commit_fails_closed(self):
        head_sha = gates.get_current_git_sha()
        dep_fp = gen.compute_input_dependency_fingerprint(REPO_ROOT)
        fake_artifact = {
            "sourceCommit": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
            "inputDependencyFingerprint": dep_fp,
        }
        ok, errs = gates.verify_commit_and_fingerprints(
            repo_root=REPO_ROOT,
            head_sha=head_sha,
            artifact_rel_paths=("docs/apple-runtime/p10/baseline-state.json",),
            current_fingerprint=dep_fp,
            artifact_overrides={"docs/apple-runtime/p10/baseline-state.json": fake_artifact},
        )
        self.assertFalse(ok)
        self.assertTrue(any("not an ancestor" in e for e in errs))

    def test_c5_08_drifted_input_dependency_fingerprint_fails_closed(self):
        head_sha = gates.get_current_git_sha()
        dep_fp = gen.compute_input_dependency_fingerprint(REPO_ROOT)
        drifted_artifact = {
            "sourceCommit": head_sha,
            "inputDependencyFingerprint": "0" * 64,
        }
        ok, errs = gates.verify_commit_and_fingerprints(
            repo_root=REPO_ROOT,
            head_sha=head_sha,
            artifact_rel_paths=("docs/apple-runtime/p10/baseline-state.json",),
            current_fingerprint=dep_fp,
            artifact_overrides={"docs/apple-runtime/p10/baseline-state.json": drifted_artifact},
        )
        self.assertFalse(ok)
        self.assertTrue(any("inputDependencyFingerprint drift" in e for e in errs))

    def test_c5_fingerprint_covers_new_compiler_and_lowering_files(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            before = gen.compute_input_dependency_fingerprint(root)
            for relative in ("build_profile.go", "internal/language/lower/short_circuit.go",
                             "internal/language/analysis/new_guard.go", "go.sum",
                             "tools/p10r3/run_release_gates.py"):
                source = root / relative
                source.parent.mkdir(parents=True, exist_ok=True)
                source.write_bytes(b"original compiler input\n")
                added = gen.compute_input_dependency_fingerprint(root)
                self.assertNotEqual(before, added, relative + " was absent from fingerprint")
                source.write_bytes(b"changed compiler input\n")
                changed = gen.compute_input_dependency_fingerprint(root)
                self.assertNotEqual(added, changed, relative + " mutation was ignored")
                source.unlink()
                self.assertEqual(before, gen.compute_input_dependency_fingerprint(root))

    def test_c5_fingerprint_normalizes_line_endings_and_excludes_generated_receipts(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            (root / "build_profile.go").write_bytes(b"first\nsecond\n")
            before = gen.compute_input_dependency_fingerprint(root)
            (root / "build_profile.go").write_bytes(b"first\r\nsecond\r\n")
            self.assertEqual(before, gen.compute_input_dependency_fingerprint(root))
            receipt = root / "docs/apple-runtime/p10/p10r3-gate-results.json"
            receipt.parent.mkdir(parents=True)
            receipt.write_text('{"sourceCommit":"receipt-only"}', encoding="utf-8")
            self.assertEqual(before, gen.compute_input_dependency_fingerprint(root),
                             "generated receipt created a circular dependency")

    def test_c5_09_release_verdict_never_ready_when_blocked_or_failing(self):
        all_pass = [{"id": gid, "status": "PASS"} for gid in gates.ALL_REQUIRED_GATES]
        self.assertEqual(gates.compute_release_verdict(all_pass), "READY")

        with_blocked = copy.deepcopy(all_pass)
        with_blocked[21]["status"] = "BLOCKED_PLATFORM"
        self.assertEqual(gates.compute_release_verdict(with_blocked), "PARTIAL_WITH_EXTERNAL_BLOCKERS")

        with_signing_blocked = copy.deepcopy(all_pass)
        with_signing_blocked[21]["status"] = "BLOCKED_SIGNING"
        self.assertEqual(gates.compute_release_verdict(with_signing_blocked), "PARTIAL_WITH_EXTERNAL_BLOCKERS")

        with_fail = copy.deepcopy(with_blocked)
        with_fail[0]["status"] = "FAIL"
        self.assertEqual(gates.compute_release_verdict(with_fail), "NOT_READY")

    def test_c5_10_r22_granular_blocker_classification(self):
        defn = gates.GATE_DEFINITIONS["R22"]
        # 1. Linux/Windows host -> BLOCKED_PLATFORM
        res_linux = gates.evaluate_r22_simulator_gate("R22", defn, host_platform="linux")
        self.assertEqual(res_linux["status"], "BLOCKED_PLATFORM")

        # 2. macOS host without iOS 27 Simulator runtime -> BLOCKED_PLATFORM
        res_mac_no_sim = gates.evaluate_r22_simulator_gate(
            "R22", defn, host_platform="darwin", simctl_returncode=0, simctl_stdout="== Runtimes ==\niOS 18.2\n"
        )
        self.assertEqual(res_mac_no_sim["status"], "BLOCKED_PLATFORM")

        # 3. macOS host with iOS 27 Simulator runtime but native signing unavailable -> BLOCKED_SIGNING
        res_mac_sign_blocked = gates.evaluate_r22_simulator_gate(
            "R22",
            defn,
            host_platform="darwin",
            simctl_returncode=0,
            simctl_stdout="== Runtimes ==\niOS 27.0\n",
            poc_returncode=2,
            poc_output="IOS27_SHORTCUTS_RUNTIME_POC=BLOCKED_SIGNING BLOCKER=NATIVE_SIGNING_UNAVAILABLE",
        )
        self.assertEqual(res_mac_sign_blocked["status"], "BLOCKED_SIGNING")

    # --- PART E: Pure+Pack Real Generator & Negative Drift Detection ---
    def test_f04_15_go_only_actions_and_pack_generator_verification(self):
        items = self.pack15["goOnlyActionsAudit"]
        names = {x["actionName"] for x in items}
        self.assertEqual(names, set(pack_ver.EXPECTED_15_GO_ONLY.keys()))
        for item in items:
            self.assertEqual(
                item.get("classification"),
                pack_ver.EXPECTED_15_GO_ONLY[item["actionName"]],
            )

        # Verify clean generation matches 141b9cc byte-for-byte
        errs = pack_ver.verify_generated_pack(p10_root=REPO_ROOT)
        self.assertEqual(errs, [])

        # Verify mutated generated module fails closed
        drift_errs = pack_ver.verify_generated_pack(
            p10_root=REPO_ROOT,
            mutated_modules={"math.cherri": "// Tampered module\n"},
        )
        self.assertGreater(len(drift_errs), 0)
        self.assertTrue(any("math.cherri" in e for e in drift_errs))

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
        self.assertEqual(eb["liveAppleServerSigning"], "BLOCKED_SIGNING")


if __name__ == "__main__":
    unittest.main()
