#!/usr/bin/env python3
"""Adversarial evidence tests; no compiler, network, or private corpus needed."""
import copy
import unittest
import generate_action_contract_coverage as contracts


class ContractEvidenceTests(unittest.TestCase):
    def setUp(self):
        self.catalog = {"count": 1, "actions": [
            {"name": "text", "shortcutIdentifier": "is.workflow.actions.gettext",
             "parameters": [{"name": "text", "key": "WFTextActionText", "type": "text"}]}
        ]}
        self.evidence = {"totalCatalogEntries": 1, "entries": [{
            "name": "text", "shortcutIdentifier": "is.workflow.actions.gettext",
            "compilerConstruct": False, "legacySupported": True,
            "v2Supported": True, "plistIdentifierVerified": True,
            "parameterContractVerified": True, "decompileSupported": True,
            "structuralRoundTripPass": True,
            "nativeSpecimenParityStatus": "NEEDS_EVIDENCE",
            "appleRuntimeExecutedStatus": "BLOCKED_DEVICE",
            "legacySource": 'text("שלום 😀")', "v2Source": 'text(text: "שלום 😀")',
        }]}
        self.receipt = {"sourceSha": "a" * 40, "sourceFingerprint": "b" * 64,
                        "executionEvidenceSha256": "c" * 64}
        self.sources = {"text": ("actions/basic.cherri", 1)}

    def build(self):
        return contracts.build_contracts(self.catalog, self.evidence, self.receipt,
                                         self.sources, {"entries": []})

    def test_complete_explicit_tiers_without_universal_or_runtime_claim(self):
        report = self.build()
        row = report["entries"][0]
        self.assertEqual(set(row["dimensions"]), set(contracts.DIMENSIONS))
        self.assertTrue(all(row["dimensions"].values()))
        self.assertEqual(row["status"], "PARTIALLY_VERIFIED")
        self.assertEqual(row["dimensions"]["signedRuntime"], "NOT_SAMPLED_PER_CALLABLE")
        self.assertEqual(row["platform"], {})
        self.assertEqual(row["platformEvidence"], "DECLARED_CONSTRAINTS_ONLY")
        self.assertEqual(report["summary"]["uniqueCallableNativeIdentifiers"], 1)

    def test_missing_evidence_cannot_be_green(self):
        self.evidence["entries"] = []
        self.assertRaisesRegex(ValueError, "MEMBERSHIP", self.build)

    def test_duplicate_evidence_cannot_overwrite(self):
        self.evidence["entries"].append(copy.deepcopy(self.evidence["entries"][0]))
        self.assertRaisesRegex(ValueError, "DUPLICATE_ENTRY", self.build)

    def test_wrong_native_identifier_is_rejected(self):
        self.evidence["entries"][0]["shortcutIdentifier"] = "is.workflow.actions.number"
        self.assertRaisesRegex(ValueError, "NATIVE_IDENTIFIER", self.build)

    def test_v2_failure_is_not_hidden_by_legacy_pass(self):
        self.evidence["entries"][0]["v2Supported"] = False
        row = self.build()["entries"][0]
        self.assertEqual(row["status"], "FAIL")
        self.assertEqual(row["dimensions"]["v2NamedCompile"], "FAIL")

    def test_runtime_status_string_is_not_proof(self):
        self.evidence["entries"][0]["appleRuntimeExecutedStatus"] = "APPLE_RUNTIME_EXECUTED"
        self.assertRaisesRegex(ValueError, "INDEPENDENT_RECEIPT", self.build)

    def test_construct_cannot_be_counted_as_callable(self):
        self.catalog["actions"][0]["compilerConstruct"] = True
        self.evidence["entries"][0]["compilerConstruct"] = True
        report = self.build()
        self.assertEqual(report["summary"]["uniqueCallableNativeIdentifiers"], 0)
        self.assertEqual(report["entries"][0]["status"], "COMPILER_CONSTRUCT")

    def test_construct_kind_mutation_is_rejected(self):
        self.evidence["entries"][0]["compilerConstruct"] = True
        self.assertRaisesRegex(ValueError, "CONSTRUCT_KIND", self.build)

    def test_default_presence_does_not_turn_empty_default_into_required(self):
        self.catalog["actions"][0]["parameters"][0]["default"] = ""
        row = self.build()["entries"][0]
        self.assertFalse(row["parameters"][0]["required"])

    def test_missing_source_location_is_rejected(self):
        self.sources = {}
        self.assertRaisesRegex(ValueError, "MISSING_CANONICAL_SOURCE", self.build)

    def test_receipt_requires_exact_inputs_and_command(self):
        receipt = {**self.receipt, "exitCode": 0, "command": contracts.TEST_COMMAND,
                   "testExecuted": "TestP10_VerifyAndRecord505CatalogExecution"}
        contracts.validate_receipt(receipt, "b" * 64, "c" * 64)
        for field, value, message in [
            ("sourceFingerprint", "d" * 64, "STALE_SOURCE"),
            ("executionEvidenceSha256", "d" * 64, "STALE_EXECUTION"),
            ("exitCode", 1, "UNVERIFIED_EXECUTION"),
            ("command", ["go", "version"], "UNVERIFIED_EXECUTION"),
            ("testExecuted", "DifferentTest", "NOT_EXECUTED"),
        ]:
            with self.subTest(field=field):
                broken = {**receipt, field: value}
                self.assertRaisesRegex(ValueError, message, contracts.validate_receipt,
                                       broken, "b" * 64, "c" * 64)

    def test_boolean_strings_do_not_count_as_pass(self):
        self.evidence["entries"][0]["legacySupported"] = "true"
        self.assertEqual(self.build()["entries"][0]["dimensions"]["legacyNamedCompile"], "FAIL")


if __name__ == "__main__":
    unittest.main(verbosity=2)
