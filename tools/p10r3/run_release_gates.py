#!/usr/bin/env python3
"""tools/p10r3/run_release_gates.py

Authoritative executable gate runner for Cherri P10R4.
Executes all 28 acceptance gates (R01-R28) and 28 adversarial gates (A01-A28).
Rejects zero executions, skipped tests, missing registrations, stale PASS assertions,
and input dependency fingerprint drift.
Outputs docs/apple-runtime/p10/p10r3-gate-results.json (or --out-results) and optional
--ci-attestation directly from real execution evidence.
"""

import argparse
import datetime
import hashlib
import json
import os
import pathlib
import platform
import subprocess
import sys
import time

REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent.parent
sys.path.insert(0, str(REPO_ROOT / "tools" / "p10r3"))

from generate_catalog_and_pack_ledgers import compute_input_dependency_fingerprint

# Complete required gate identifiers
REQUIRED_R_GATES = [f"R{i:02d}" for i in range(1, 29)]
REQUIRED_A_GATES = [f"A{i:02d}" for i in range(1, 29)]
ALL_REQUIRED_GATES = REQUIRED_R_GATES + REQUIRED_A_GATES

BLOCKED_STATUSES = {
    "BLOCKED_EXTERNAL",
    "BLOCKED_PLATFORM",
    "BLOCKED_DEVICE",
    "BLOCKED_SIGNING",
    "BLOCKED_PERMISSION",
    "NEEDS_EVIDENCE",
}

# Execution commands and metadata for every gate
GATE_DEFINITIONS = {
    "R01": {
        "name": "Missing macOS 27 directory fails closed",
        "command": ["python", "-m", "unittest", "-k", "test_r01_missing_runtime_dir_fails_closed", "tools/shortcut-runtime/tests/test_runtime_extraction.py"],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "OK",
        "sourceFile": "tools/shortcut-runtime/tests/test_runtime_extraction.py",
    },
    "R02": {
        "name": "Corrupt snapshot bytes fail checksum verification",
        "command": ["python", "-m", "unittest", "-k", "test_r02_corrupt_snapshot_checksum_failure", "tools/shortcut-runtime/tests/test_runtime_extraction.py"],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "OK",
        "sourceFile": "tools/shortcut-runtime/tests/test_runtime_extraction.py",
    },
    "R03": {
        "name": "macOS 26 manifest in macOS 27 dir fails closed",
        "command": ["python", "-m", "unittest", "-k", "test_r03_macos26_manifest_in_macos27_dir_fails", "tools/shortcut-runtime/tests/test_runtime_extraction.py"],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "OK",
        "sourceFile": "tools/shortcut-runtime/tests/test_runtime_extraction.py",
    },
    "R04": {
        "name": "Dropped or faked shard invariant failure",
        "command": ["python", "-m", "unittest", "-k", "test_r04_dropped_or_faked_shard_invariant_failure", "tools/shortcut-runtime/tests/test_runtime_extraction.py"],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "OK",
        "sourceFile": "tools/shortcut-runtime/tests/test_runtime_extraction.py",
    },
    "R05": {
        "name": "Corrupt or missing Cherri actions catalog fails",
        "command": ["python", "-m", "unittest", "-k", "test_r05_corrupt_or_missing_cherri_catalog_fails", "tools/shortcut-runtime/tests/test_runtime_extraction.py"],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "OK",
        "sourceFile": "tools/shortcut-runtime/tests/test_runtime_extraction.py",
    },
    "R06": {
        "name": "Disposition builder producers exist in repo",
        "command": ["python", "-m", "unittest", "-k", "test_r06_disposition_builder_producer_exists", "tools/shortcut-runtime/tests/test_runtime_extraction.py"],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "OK",
        "sourceFile": "tools/shortcut-runtime/tests/test_runtime_extraction.py",
    },
    "R07": {
        "name": "Filter without predicate is not verified",
        "command": ["python", "-m", "unittest", "-k", "test_r07_filter_signature_without_predicate_not_verified", "tools/shortcut-runtime/tests/test_runtime_extraction.py"],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "OK",
        "sourceFile": "tools/shortcut-runtime/tests/test_runtime_extraction.py",
    },
    "R08": {
        "name": "Compound filter round-trip and nested rejection",
        "command": ["go", "test", "-run", "TestP10R_R08_FilterTwoLevelAndOrRoundTrip", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R_R08_FilterTwoLevelAndOrRoundTrip",
        "sourceFile": "p10r_acceptance_test.go",
    },
    "R09": {
        "name": "Filter operator parsing fails closed on invalid operators",
        "command": ["go", "test", "-run", "TestP10R_R09_FilterOperatorParsing", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R_R09_FilterOperatorParsing",
        "sourceFile": "p10r_acceptance_test.go",
    },
    "R10": {
        "name": "Sorting without limit does not emit LimitEnabled",
        "command": ["go", "test", "-run", "TestP10R_R10_LimitAbsentSortOnly", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R_R10_LimitAbsentSortOnly",
        "sourceFile": "p10r_acceptance_test.go",
    },
    "R11": {
        "name": "editReminder invalid detail fails closed under Set and Remove",
        "command": ["go", "test", "-run", "TestP10R_R11_EditReminderInvalidDetail", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R_R11_EditReminderInvalidDetail",
        "sourceFile": "p10r_acceptance_test.go",
    },
    "R12": {
        "name": "editEvent invalid detail fails closed under Set and Remove",
        "command": ["go", "test", "-run", "TestP10R_R12_EditEventInvalidDetail", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R_R12_EditEventInvalidDetail",
        "sourceFile": "p10r_acceptance_test.go",
    },
    "R13": {
        "name": "Setter modes Set vs Remove and RemoveSpecifiedValue emission",
        "command": ["go", "test", "-run", "TestP10R_R13_SetterModesAndDecompile", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R_R13_SetterModesAndDecompile",
        "sourceFile": "p10r_acceptance_test.go",
    },
    "R14": {
        "name": "Residual gap keys adjudication integrity",
        "command": ["python", "-m", "unittest", "-k", "test_r14_residual_gaps_adjudication_integrity", "tools/shortcut-runtime/tests/test_runtime_extraction.py"],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "OK",
        "sourceFile": "tools/shortcut-runtime/tests/test_runtime_extraction.py",
    },
    "R15": {
        "name": "Calendar picker requires runtime evidence",
        "command": ["python", "-m", "unittest", "-k", "test_r15_calendar_picker_requires_runtime_evidence", "tools/shortcut-runtime/tests/test_runtime_extraction.py"],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "OK",
        "sourceFile": "tools/shortcut-runtime/tests/test_runtime_extraction.py",
    },
    "R16": {
        "name": "Output type mismatch yields partial status",
        "command": ["python", "-m", "unittest", "-k", "test_r16_output_type_mismatch_yields_partial", "tools/shortcut-runtime/tests/test_runtime_extraction.py"],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "OK",
        "sourceFile": "tools/shortcut-runtime/tests/test_runtime_extraction.py",
    },
    "R17": {
        "name": "Fingerprint drift on EvidenceStatus change",
        "command": ["go", "test", "-run", "TestP10R_R17_FingerprintDriftOnEvidenceStatus", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R_R17_FingerprintDriftOnEvidenceStatus",
        "sourceFile": "p10r_acceptance_test.go",
    },
    "R18": {
        "name": "Semantic version comparison and availability check",
        "command": ["go", "test", "-run", "TestP10R_R18_SemanticVersionComparison", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R_R18_SemanticVersionComparison",
        "sourceFile": "p10r_acceptance_test.go",
    },
    "R19": {
        "name": "OS availability not assumed universal",
        "command": ["python", "-m", "unittest", "-k", "test_r19_os_availability_not_assumed_universal", "tools/shortcut-runtime/tests/test_runtime_extraction.py"],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "OK",
        "sourceFile": "tools/shortcut-runtime/tests/test_runtime_extraction.py",
    },
    "R20": {
        "name": "Unrepresentable custom filter value rawAction fallback",
        "command": ["go", "test", "-run", "TestP10R_R20_UnrepresentableValueRawActionFallback", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R_R20_UnrepresentableValueRawActionFallback",
        "sourceFile": "p10r_acceptance_test.go",
    },
    "R21": {
        "name": "Exact UTF-16 attachment offset calculation",
        "command": ["go", "test", "-run", "TestP10R_R21_HebrewEmojiUtf16Tokens", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R_R21_HebrewEmojiUtf16Tokens",
        "sourceFile": "p10r_acceptance_test.go",
    },
    "R22": {
        "name": "iOS 27 Simulator runtime execution (platform-aware vs Physical iPad BLOCKED_DEVICE)",
        "isPlatformSimulatorCheck": True,
        "tier": "BLOCKED_EXTERNAL",
        "blockerReason": "iOS 27 Simulator execution requires macOS + Xcode iOS 27 Simulator runtime (BLOCKED_PLATFORM when absent). Physical iPadOS 27 execution is tracked separately as BLOCKED_DEVICE in docs/apple-runtime/p10/device-acceptance-manifest.json.",
        "sourceFile": "scripts/ios27_runtime_poc.sh",
    },
    "R23": {
        "name": "Protected parallel iOS files remain untouched across P10 baseline 76d7225..HEAD and working tree",
        "checkProtectedIOSFiles": [
            "ios/CherriUITests/CherriEditorUITests.swift",
            "ios/CherriApp/Views/CodeEditorView.swift",
        ],
        "upstreamBase": "76d722502e20db977e84a095444d0172442655b8",
        "tier": "PRODUCTION_PATH",
        "sourceFile": "git",
    },
    "R24": {
        "name": "ToolKit config-only and hidden entities excluded from user denominator",
        "command": ["python", "-m", "unittest", "-k", "test_r24_toolkit_entities_and_config_excluded", "tools/shortcut-runtime/tests/test_runtime_extraction.py"],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "OK",
        "sourceFile": "tools/shortcut-runtime/tests/test_runtime_extraction.py",
    },
    "R25": {
        "name": "Action catalog, schema registry, and consumer export contract",
        "command": ["go", "test", "-run", "TestP10R_R25_ActionCatalogAndGoBridgeExport", "-count=1", "-v", "."],
        "tier": "STATIC_CONTRACT",
        "expectedMarker": "PASS: TestP10R_R25_ActionCatalogAndGoBridgeExport",
        "sourceFile": "p10r_acceptance_test.go",
    },
    "R26": {
        "name": "Provenance registry ancestry and commit validation",
        "command": ["go", "test", "-run", "TestP10R_R26_ProvenanceRegistryIntegrity", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R_R26_ProvenanceRegistryIntegrity",
        "sourceFile": "p10r_acceptance_test.go",
    },
    "R27": {
        "name": "Missing serializer parameter disposition classification",
        "command": ["python", "-m", "unittest", "-k", "test_r27_missing_serializer_classification", "tools/shortcut-runtime/tests/test_runtime_extraction.py"],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "OK",
        "sourceFile": "tools/shortcut-runtime/tests/test_runtime_extraction.py",
    },
    "R28": {
        "name": "Acceptance suite completeness contract verification",
        "command": ["go", "test", "-run", "TestP10R_R28_AcceptanceSuiteCompleteness", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R_R28_AcceptanceSuiteCompleteness",
        "sourceFile": "p10r_acceptance_test.go",
    },

    # Adversarial battery A01-A28
    "A01": {
        "name": "Manifest compiler SHA non-existent in git fails closed",
        "command": ["go", "test", "-run", "TestP10R2_A01_ManifestCompilerShaNonExistentInGitFails", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A01_ManifestCompilerShaNonExistentInGitFails",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A02": {
        "name": "Stale docs fingerprint rejected by verifier",
        "command": ["go", "test", "-run", "TestP10R2_A02_StaleDocsFingerprintRejected", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A02_StaleDocsFingerprintRejected",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A03": {
        "name": "Source selector fails closed on missing 27 evidence",
        "command": ["go", "test", "-run", "TestP10R2_A03_SourceSelectorFailsClosedOnMissing27", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A03_SourceSelectorFailsClosedOnMissing27",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A04": {
        "name": "Legitimate empty 27 gaps preserved without 26 fallback",
        "command": ["go", "test", "-run", "TestP10R2_A04_LegitimateEmpty27GapsPreserved", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A04_LegitimateEmpty27GapsPreserved",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A05": {
        "name": "Manifest version check rejects version mismatch",
        "command": ["go", "test", "-run", "TestP10R2_A05_ManifestVersionCheckRejectsMismatch", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A05_ManifestVersionCheckRejectsMismatch",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A06": {
        "name": "Shard summary integrity validation",
        "command": ["go", "test", "-run", "TestP10R2_A06_ShardSummaryIntegrity", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A06_ShardSummaryIntegrity",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A07": {
        "name": "Unassessed output downgrades verified status",
        "command": ["go", "test", "-run", "TestP10R2_A07_UnassessedOutputDowngradesVerifiedStatus", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A07_UnassessedOutputDowngradesVerifiedStatus",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A08": {
        "name": "Disposition builder wire key emission verification",
        "command": ["go", "test", "-run", "TestP10R2_A08_DispositionCodePathVerification", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A08_DispositionCodePathVerification",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A09": {
        "name": "Unknown predicate operator fails closed",
        "command": ["go", "test", "-run", "TestP10R2_A09_UnknownPredicateOperatorFailsClosed", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A09_UnknownPredicateOperatorFailsClosed",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A10": {
        "name": "Malformed filter predicates fail closed",
        "command": ["go", "test", "-run", "TestP10R2_A10_MalformedFilterPredicatesFailsClosed", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A10_MalformedFilterPredicatesFailsClosed",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A11": {
        "name": "Invalid reminder detail fails under Set and Remove",
        "command": ["go", "test", "-run", "TestP10R2_A11_InvalidReminderDetailFailsUnderRemoveAndSet", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A11_InvalidReminderDetailFailsUnderRemoveAndSet",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A12": {
        "name": "Invalid event detail fails under Set and Remove",
        "command": ["go", "test", "-run", "TestP10R2_A12_InvalidEventDetailFailsUnderRemoveAndSet", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A12_InvalidEventDetailFailsUnderRemoveAndSet",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A13": {
        "name": "Unknown setter mode fails closed",
        "command": ["go", "test", "-run", "TestP10R2_A13_UnknownSetterModeFailsClosed", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A13_UnknownSetterModeFailsClosed",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A14": {
        "name": "RemoveSpecifiedValue emission and type verification",
        "command": ["go", "test", "-run", "TestP10R2_A14_RemoveSpecifiedValueEmission", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A14_RemoveSpecifiedValueEmission",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A15": {
        "name": "Semantic version compiler check 27.10 vs 27.2",
        "command": ["go", "test", "-run", "TestP10R2_A15_SemanticVersionCompilerCheck", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A15_SemanticVersionCompilerCheck",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A16": {
        "name": "Hebrew and emoji UTF-16 token interpolation offsets",
        "command": ["go", "test", "-run", "TestP10R2_A16_HebrewEmojiUtf16Offset", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A16_HebrewEmojiUtf16Offset",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A17": {
        "name": "RawAction fallback lossless structural recompile",
        "command": ["go", "test", "-run", "TestP10R2_A17_RawActionFallbackLossless", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A17_RawActionFallbackLossless",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A18": {
        "name": "Fake firstForkCommit detected by ancestry validator",
        "command": ["go", "test", "-run", "TestP10R2_A18_FakeFirstForkCommitDetected", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A18_FakeFirstForkCommitDetected",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A19": {
        "name": "Aggregate release gate detects missing/skipped test",
        "command": ["go", "test", "-run", "TestP10R2_A19_AggregateGateFailsOnMissingTest", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A19_AggregateGateFailsOnMissingTest",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A20": {
        "name": "iOS palette consumer action coverage",
        "command": ["go", "test", "-run", "TestP10R2_A20_IOSPaletteActionCoverage", "-count=1", "-v", "."],
        "tier": "STATIC_CONTRACT",
        "expectedMarker": "PASS: TestP10R2_A20_IOSPaletteActionCoverage",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A21": {
        "name": "Derived artifact tampering detected by checksum verifier",
        "command": ["go", "test", "-run", "TestP10R2_A21_DerivedArtifactTamperingFails", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A21_DerivedArtifactTamperingFails",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A22": {
        "name": "Empty shard summary in manifest rejected",
        "command": ["go", "test", "-run", "TestP10R2_A22_EmptyShardSummaryRejection", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A22_EmptyShardSummaryRejection",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A23": {
        "name": "MacOnly flag preserved on platform-specific actions",
        "command": ["go", "test", "-run", "TestP10R2_A23_MacOnlyFlagPreserved", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A23_MacOnlyFlagPreserved",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A24": {
        "name": "ToolKit internal config excluded from callable user actions",
        "command": ["go", "test", "-run", "TestP10R2_A24_ToolKitConfigExcludedFromUserActions", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A24_ToolKitConfigExcludedFromUserActions",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A25": {
        "name": "Schema fingerprint drift detection",
        "command": ["go", "test", "-run", "TestP10R2_A25_SchemaFingerprintDriftDetection", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A25_SchemaFingerprintDriftDetection",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A26": {
        "name": "Third-party App Intent conditional eligibility",
        "command": ["go", "test", "-run", "TestP10R2_A26_ThirdPartyAppIntentConditional", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A26_ThirdPartyAppIntentConditional",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A27": {
        "name": "Regression suite unbroken",
        "command": ["go", "test", "-run", "TestP10R2_A27_ExistingFixturesUnbroken", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A27_ExistingFixturesUnbroken",
        "sourceFile": "p10r2_adversarial_test.go",
    },
    "A28": {
        "name": "Full release tuple git object ancestry and fingerprint check",
        "command": ["go", "test", "-run", "TestP10R2_A28_FullReleaseTupleStructure", "-count=1", "-v", "."],
        "tier": "PRODUCTION_PATH",
        "expectedMarker": "PASS: TestP10R2_A28_FullReleaseTupleStructure",
        "sourceFile": "p10r2_adversarial_test.go",
    },
}


def get_current_git_sha():
    try:
        res = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            cwd=REPO_ROOT,
            capture_output=True,
            text=True,
            check=True,
            encoding="utf-8",
            errors="replace",
        )
        return res.stdout.strip()
    except Exception:
        return "unknown"


def validate_gate_registry(active_gates):
    """Validates that all required gates are present in the provided list. Fails closed."""
    missing = [g for g in ALL_REQUIRED_GATES if g not in active_gates]
    if missing:
        raise ValueError(f"Gate registry validation failed: missing required gate(s): {missing}")
    return True


def classify_command_output(gate_id, defn, returncode, combined_output, duration_seconds):
    """Evaluates command return code, skipped markers, and expected markers. Fails closed."""
    out_hash = hashlib.sha256(combined_output.encode("utf-8")).hexdigest()
    cmd_str = " ".join(defn.get("command", []))

    # Reject skipped tests or zero tests run
    if "--- SKIP:" in combined_output or "testing: warning: no tests to run" in combined_output or "Ran 0 tests" in combined_output:
        return {
            "id": gate_id,
            "name": defn["name"],
            "status": "FAIL",
            "tier": defn["tier"],
            "command": cmd_str,
            "exitCode": returncode if returncode != 0 else 1,
            "durationSeconds": duration_seconds,
            "outputSha256": out_hash,
            "sourceFile": defn["sourceFile"],
            "error": "Gate test was skipped or ran 0 tests",
        }

    if returncode != 0:
        return {
            "id": gate_id,
            "name": defn["name"],
            "status": "FAIL",
            "tier": defn["tier"],
            "command": cmd_str,
            "exitCode": returncode,
            "durationSeconds": duration_seconds,
            "outputSha256": out_hash,
            "sourceFile": defn["sourceFile"],
            "error": combined_output[-500:],
        }

    marker = defn.get("expectedMarker")
    if marker and marker not in combined_output:
        return {
            "id": gate_id,
            "name": defn["name"],
            "status": "FAIL",
            "tier": defn["tier"],
            "command": cmd_str,
            "exitCode": 1,
            "durationSeconds": duration_seconds,
            "outputSha256": out_hash,
            "sourceFile": defn["sourceFile"],
            "error": f"Missing expected output marker '{marker}' in test output",
        }

    return {
        "id": gate_id,
        "name": defn["name"],
        "status": "PASS",
        "tier": defn["tier"],
        "command": cmd_str,
        "exitCode": 0,
        "durationSeconds": duration_seconds,
        "outputSha256": out_hash,
        "sourceFile": defn["sourceFile"],
    }


def evaluate_r22_simulator_gate(
    gate_id,
    defn,
    host_platform=None,
    simctl_returncode=None,
    simctl_stdout=None,
    poc_returncode=None,
    poc_output=None,
):
    """Evaluates R22 with granular blocker classes (BLOCKED_PLATFORM, BLOCKED_SIGNING, BLOCKED_PERMISSION, BLOCKED_DEVICE)."""
    t0 = time.time()
    plat = host_platform if host_platform is not None else sys.platform
    if plat == "darwin":
        if simctl_returncode is None:
            sim_check = subprocess.run(
                ["xcrun", "simctl", "list", "runtimes"],
                cwd=REPO_ROOT,
                capture_output=True,
                text=True,
                encoding="utf-8",
                errors="replace",
            )
            simctl_returncode = sim_check.returncode
            simctl_stdout = sim_check.stdout or ""
        if simctl_returncode == 0 and "iOS 27" in (simctl_stdout or ""):
            if poc_returncode is None:
                proc = subprocess.run(
                    ["bash", "scripts/ios27_runtime_poc.sh"],
                    cwd=REPO_ROOT,
                    capture_output=True,
                    text=True,
                    encoding="utf-8",
                    errors="replace",
                )
                poc_returncode = proc.returncode
                poc_output = (proc.stdout or "") + (proc.stderr or "")
            dur = round(time.time() - t0, 3)
            out_hash = hashlib.sha256((poc_output or "").encode("utf-8")).hexdigest()
            if poc_returncode == 0 and "IOS27_SHORTCUTS_RUNTIME_POC=PASS" in (poc_output or ""):
                status = "PASS"
            elif "BLOCKED_SIGNING" in (poc_output or "") or poc_returncode == 2:
                status = "BLOCKED_SIGNING"
            elif "BLOCKED_PERMISSION" in (poc_output or ""):
                status = "BLOCKED_PERMISSION"
            else:
                status = "FAIL"
            return {
                "id": gate_id,
                "name": defn["name"],
                "status": status,
                "blockerClass": status if status in BLOCKED_STATUSES else None,
                "physicalDeviceBlockerClass": "BLOCKED_DEVICE",
                "tier": "PRODUCTION_PATH" if status in ("PASS", "FAIL") else "BLOCKED_EXTERNAL",
                "command": "bash scripts/ios27_runtime_poc.sh",
                "exitCode": poc_returncode,
                "durationSeconds": dur,
                "outputSha256": out_hash,
                "sourceFile": defn["sourceFile"],
            }
        return {
            "id": gate_id,
            "name": defn["name"],
            "status": "BLOCKED_PLATFORM",
            "blockerCategory": "BLOCKED_EXTERNAL",
            "blockerClass": "BLOCKED_PLATFORM",
            "physicalDeviceBlockerClass": "BLOCKED_DEVICE",
            "tier": defn["tier"],
            "command": "xcrun simctl list runtimes (darwin host without iOS 27 Simulator runtime installed)",
            "exitCode": 0,
            "durationSeconds": round(time.time() - t0, 3),
            "outputSha256": hashlib.sha256(b"DARWIN_MISSING_IOS27_SIMULATOR_RUNTIME").hexdigest(),
            "sourceFile": defn["sourceFile"],
            "rationale": "macOS host detected, but Xcode iOS 27 Simulator runtime is not installed on this runner (BLOCKED_PLATFORM). Physical iPadOS 27 hardware is tracked separately as BLOCKED_DEVICE.",
        }

    return {
        "id": gate_id,
        "name": defn["name"],
        "status": "BLOCKED_PLATFORM",
        "blockerCategory": "BLOCKED_EXTERNAL",
        "blockerClass": "BLOCKED_PLATFORM",
        "physicalDeviceBlockerClass": "BLOCKED_DEVICE",
        "tier": defn["tier"],
        "command": f"platform={plat} (requires darwin + iOS 27 Simulator runtime; physical iPad tracked as BLOCKED_DEVICE)",
        "exitCode": 0,
        "durationSeconds": round(time.time() - t0, 3),
        "outputSha256": hashlib.sha256(b"BLOCKED_PLATFORM_AND_BLOCKED_DEVICE").hexdigest(),
        "sourceFile": defn["sourceFile"],
        "rationale": defn.get("blockerReason", ""),
    }


def evaluate_r23_protected_files(gate_id, defn, diff_stdout="", status_stdout="", returncodes=(0, 0)):
    files = defn["checkProtectedIOSFiles"]
    base = defn.get("upstreamBase", "76d722502e20db977e84a095444d0172442655b8")
    cmd_diff = ["git", "diff", f"{base}..HEAD", "--"] + files
    cmd_status = ["git", "status", "--porcelain", "--"] + files
    combined = f"DIFF:\n{diff_stdout}\nSTATUS:\n{status_stdout}"
    out_hash = hashlib.sha256(combined.encode("utf-8")).hexdigest()
    cmd_str = f"{' '.join(cmd_diff)} && {' '.join(cmd_status)}"
    if returncodes == (0, 0) and len(diff_stdout.strip()) == 0 and len(status_stdout.strip()) == 0:
        return {
            "id": gate_id,
            "name": defn["name"],
            "status": "PASS",
            "tier": defn["tier"],
            "command": cmd_str,
            "exitCode": 0,
            "durationSeconds": 0.01,
            "outputSha256": out_hash,
            "sourceFile": defn["sourceFile"],
        }
    return {
        "id": gate_id,
        "name": defn["name"],
        "status": "FAIL",
        "tier": defn["tier"],
        "command": cmd_str,
        "exitCode": 1,
        "durationSeconds": 0.01,
        "outputSha256": out_hash,
        "sourceFile": defn["sourceFile"],
        "error": f"Protected iOS files modified since {base} or in working tree:\n{combined[:300]}",
    }


def execute_gate(gate_id, defn):
    """Executes a single gate command, returns result dictionary."""
    t0 = time.time()

    if defn.get("isPlatformSimulatorCheck"):
        return evaluate_r22_simulator_gate(gate_id, defn)

    if defn.get("checkProtectedIOSFiles"):
        files = defn["checkProtectedIOSFiles"]
        base = defn.get("upstreamBase", "76d722502e20db977e84a095444d0172442655b8")
        cmd_diff = ["git", "diff", f"{base}..HEAD", "--"] + files
        cmd_status = ["git", "status", "--porcelain", "--"] + files
        p_diff = subprocess.run(cmd_diff, cwd=REPO_ROOT, capture_output=True, text=True, encoding="utf-8", errors="replace")
        p_stat = subprocess.run(cmd_status, cwd=REPO_ROOT, capture_output=True, text=True, encoding="utf-8", errors="replace")
        res = evaluate_r23_protected_files(
            gate_id,
            defn,
            diff_stdout=p_diff.stdout or "",
            status_stdout=p_stat.stdout or "",
            returncodes=(p_diff.returncode, p_stat.returncode),
        )
        res["durationSeconds"] = round(time.time() - t0, 3)
        return res

    if defn.get("isBlocked"):
        b_class = defn.get("blockerClass", "BLOCKED_EXTERNAL")
        return {
            "id": gate_id,
            "name": defn["name"],
            "status": b_class,
            "blockerCategory": "BLOCKED_EXTERNAL",
            "blockerClass": b_class,
            "tier": defn["tier"],
            "command": "N/A (external environment)",
            "exitCode": 0,
            "durationSeconds": 0.0,
            "outputSha256": hashlib.sha256(b_class.encode("utf-8")).hexdigest(),
            "sourceFile": defn["sourceFile"],
            "rationale": defn.get("blockerReason", ""),
        }

    cmd = defn["command"]
    try:
        proc = subprocess.run(cmd, cwd=REPO_ROOT, capture_output=True, text=True, encoding="utf-8", errors="replace")
        dur = round(time.time() - t0, 3)
        combined_output = (proc.stdout or "") + (proc.stderr or "")
        return classify_command_output(gate_id, defn, proc.returncode, combined_output, dur)
    except Exception as e:
        dur = round(time.time() - t0, 3)
        return {
            "id": gate_id,
            "name": defn["name"],
            "status": "FAIL",
            "tier": defn["tier"],
            "command": " ".join(cmd),
            "exitCode": 1,
            "durationSeconds": dur,
            "outputSha256": hashlib.sha256(str(e).encode("utf-8")).hexdigest(),
            "sourceFile": defn["sourceFile"],
            "error": str(e),
        }


def compute_release_verdict(results):
    """Computes overall release verdict from gate results. Never returns READY if any gate is blocked or failing."""
    if len(results) != len(ALL_REQUIRED_GATES):
        return "NOT_READY"
    passing = sum(1 for r in results if r["status"] == "PASS")
    blocked = sum(1 for r in results if r["status"] in BLOCKED_STATUSES)
    failing = len(results) - passing - blocked
    if failing > 0:
        return "NOT_READY"
    if blocked > 0:
        return "PARTIAL_WITH_EXTERNAL_BLOCKERS"
    return "READY"


def verify_commit_and_fingerprints(
    repo_root: pathlib.Path = REPO_ROOT,
    head_sha: str = None,
    artifact_rel_paths: tuple = (
        "docs/apple-runtime/p10/p10r3-gate-results.json",
        "docs/apple-runtime/p10/p10-505-catalog-coverage-matrix.json",
        "docs/apple-runtime/p10/p10-apple-cherri-crosswalk.json",
        "docs/apple-runtime/p10/baseline-state.json",
    ),
    current_fingerprint: str = None,
    artifact_overrides: dict = None,
):
    """Verifies that every tracked P10 JSON artifact has:
    1. A valid 40-hex sourceCommit that is an ancestor of (or equal to) HEAD.
    2. An inputDependencyFingerprint that matches the current SHA-256 fingerprint of all compiler/action/schema/ledger inputs.
    """
    if head_sha is None:
        head_sha = get_current_git_sha()
    if current_fingerprint is None:
        current_fingerprint = compute_input_dependency_fingerprint(repo_root)
    artifact_overrides = artifact_overrides or {}
    errors = []

    for rel in artifact_rel_paths:
        if rel in artifact_overrides:
            data = artifact_overrides[rel]
        else:
            p = repo_root / rel
            if not p.exists():
                errors.append(f"Missing artifact for commit/fingerprint check: {rel}")
                continue
            data = json.loads(p.read_text(encoding="utf-8"))

        sc = data.get("sourceCommit", "")
        if not sc or len(sc) != 40:
            errors.append(f"Invalid sourceCommit '{sc}' in {rel}")
        else:
            anc = subprocess.run(["git", "merge-base", "--is-ancestor", sc, head_sha], cwd=repo_root)
            if anc.returncode != 0:
                errors.append(f"sourceCommit {sc} in {rel} is not an ancestor of HEAD {head_sha}")

        fp = data.get("inputDependencyFingerprint", "")
        if not fp or len(fp) != 64:
            errors.append(f"Missing or invalid inputDependencyFingerprint '{fp}' in {rel}")
        elif fp != current_fingerprint:
            errors.append(
                f"inputDependencyFingerprint drift in {rel}: recorded {fp} != current {current_fingerprint}"
            )

    return len(errors) == 0, errors


def get_tool_versions():
    versions = {"python": sys.version.split()[0], "platform": platform.platform()}
    try:
        go_ver = subprocess.run(["go", "version"], cwd=REPO_ROOT, capture_output=True, text=True, check=False)
        versions["go"] = (go_ver.stdout or "").strip()
    except Exception:
        versions["go"] = "unknown"
    return versions


def main():
    parser = argparse.ArgumentParser(description="Cherri P10R4 Executable Release Gate Runner")
    parser.add_argument("--run-all", action="store_true", help="Execute all 56 gates sequentially")
    parser.add_argument("--gate", type=str, help="Execute a single gate by ID")
    parser.add_argument("--verify-registry", action="store_true", help="Verify completeness of gate registry")
    parser.add_argument(
        "--verify-commit",
        action="store_true",
        help="Verify recorded sourceCommit ancestry and exact inputDependencyFingerprint across all P10 artifacts",
    )
    parser.add_argument("--mutate-missing", type=str, help="Simulate missing a gate to verify fail-closed behavior")
    parser.add_argument(
        "--out-results",
        type=str,
        default="docs/apple-runtime/p10/p10r3-gate-results.json",
        help="Path to write JSON results",
    )
    parser.add_argument(
        "--ci-attestation",
        type=str,
        default="",
        help="Optional path to write machine-readable CI attestation JSON",
    )
    args = parser.parse_args()

    if args.verify_commit:
        head_sha = get_current_git_sha()
        dep_fp = compute_input_dependency_fingerprint(REPO_ROOT)
        ok, errors = verify_commit_and_fingerprints(
            repo_root=REPO_ROOT,
            head_sha=head_sha,
            artifact_rel_paths=(
                args.out_results,
                "docs/apple-runtime/p10/p10-505-catalog-coverage-matrix.json",
                "docs/apple-runtime/p10/p10-apple-cherri-crosswalk.json",
                "docs/apple-runtime/p10/baseline-state.json",
            ),
            current_fingerprint=dep_fp,
        )
        if not ok:
            for e in errors:
                print(f"ERROR: {e}", file=sys.stderr)
            sys.exit(1)
        print(
            f"SUCCESS: All P10 artifact sourceCommit values verified as ancestors of HEAD ({head_sha}) "
            f"and inputDependencyFingerprint matched ({dep_fp})."
        )
        sys.exit(0)

    # Mutation test verification
    if args.mutate_missing:
        target = args.mutate_missing.strip().upper()
        active = [g for g in ALL_REQUIRED_GATES if g != target]
        try:
            validate_gate_registry(active)
            print(f"ERROR: Mutated registry missing {target} unexpectedly passed validation!", file=sys.stderr)
            sys.exit(0)
        except ValueError as e:
            print(f"SUCCESS: Mutated registry missing {target} failed closed as required: {e}")
            sys.exit(1)

    if args.verify_registry:
        validate_gate_registry(list(GATE_DEFINITIONS.keys()))
        print(f"SUCCESS: All {len(ALL_REQUIRED_GATES)} required gates registered correctly.")
        sys.exit(0)

    if args.gate:
        gid = args.gate.strip().upper()
        if gid not in GATE_DEFINITIONS:
            print(f"ERROR: Unknown gate ID {gid}", file=sys.stderr)
            sys.exit(1)
        res = execute_gate(gid, GATE_DEFINITIONS[gid])
        print(json.dumps(res, indent=2))
        sys.exit(0 if res["status"] == "PASS" or res["status"] in BLOCKED_STATUSES else 1)

    if args.run_all:
        validate_gate_registry(list(GATE_DEFINITIONS.keys()))
        print(f"=== Starting P10R4 Full Gate Execution ({len(ALL_REQUIRED_GATES)} gates) ===")
        results = []
        passing = 0
        blocked = 0
        failing = 0

        for gid in ALL_REQUIRED_GATES:
            defn = GATE_DEFINITIONS[gid]
            print(f"[{gid}] {defn['name']} ... ", end="", flush=True)
            res = execute_gate(gid, defn)
            results.append(res)
            st = res["status"]
            print(f"{st} ({res['durationSeconds']}s)")
            if st == "PASS":
                passing += 1
            elif st in BLOCKED_STATUSES:
                blocked += 1
            else:
                failing += 1
                if "error" in res:
                    print(f"    ERROR: {res['error'][:200]}", file=sys.stderr)

        print("\n=== Gate Execution Summary ===")
        print(f"Total Gates:       {len(ALL_REQUIRED_GATES)}")
        print(f"Passing:           {passing}")
        print(f"Blocked External:  {blocked}")
        print(f"Failing:           {failing}")

        head_sha = get_current_git_sha()
        dep_fp = compute_input_dependency_fingerprint(REPO_ROOT)
        verdict = compute_release_verdict(results)

        payload = {
            "milestone": "P10R4",
            "timestamp": datetime.datetime.now(datetime.timezone.utc).isoformat(),
            "sourceCommit": head_sha,
            "inputDependencyFingerprint": dep_fp,
            "verdict": verdict,
            "legacyVerdict": "P10R3_CORRECTIVE_PARTIAL_VERIFIED" if failing == 0 else "P10R3_GATE_FAILURE",
            "summary": {
                "totalGates": len(ALL_REQUIRED_GATES),
                "passingGates": passing,
                "blockedExternalGates": blocked,
                "failingGates": failing,
                "rGatesTotal": len(REQUIRED_R_GATES),
                "rGatesPassing": sum(1 for r in results if r["id"].startswith("R") and r["status"] == "PASS"),
                "rGatesBlockedExternal": sum(
                    1 for r in results if r["id"].startswith("R") and r["status"] in BLOCKED_STATUSES
                ),
                "aGatesTotal": len(REQUIRED_A_GATES),
                "aGatesPassing": sum(1 for r in results if r["id"].startswith("A") and r["status"] == "PASS"),
                "aGatesFailing": sum(1 for r in results if r["id"].startswith("A") and r["status"] == "FAIL"),
            },
            "gates": results,
        }

        out_path = pathlib.Path(args.out_results)
        if not out_path.is_absolute():
            out_path = REPO_ROOT / out_path
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
        print(f"Results written to: {out_path}")

        if args.ci_attestation:
            att_path = pathlib.Path(args.ci_attestation)
            if not att_path.is_absolute():
                att_path = REPO_ROOT / att_path
            att_path.parent.mkdir(parents=True, exist_ok=True)
            attestation = {
                "schemaVersion": "1.0",
                "headCommit": head_sha,
                "inputDependencyFingerprint": dep_fp,
                "runId": os.environ.get("GITHUB_RUN_ID", "local"),
                "workflow": os.environ.get("GITHUB_WORKFLOW", "local"),
                "runnerOs": os.environ.get("RUNNER_OS", sys.platform),
                "timestamp": payload["timestamp"],
                "command": " ".join(sys.argv),
                "verdict": verdict,
                "summary": payload["summary"],
                "toolVersions": get_tool_versions(),
                "gates": [
                    {
                        "id": r["id"],
                        "status": r["status"],
                        "exitCode": r["exitCode"],
                        "durationSeconds": r["durationSeconds"],
                        "outputSha256": r["outputSha256"],
                    }
                    for r in results
                ],
            }
            att_path.write_text(json.dumps(attestation, indent=2) + "\n", encoding="utf-8")
            print(f"CI attestation written to: {att_path}")

        sys.exit(0 if failing == 0 else 1)


if __name__ == "__main__":
    main()
