"""Synthetic controls for exact preview receipts; never native observations."""
import copy
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import sys
import subprocess

import debug_label_acceptance as labels
import debug_label_preview as preview
import prepare_public_artifacts as publisher


class DebugLabelPreviewControls(unittest.TestCase):
    commit = "a" * 40
    run_id = "12345"

    def context(self, mode="typed"):
        fid = "qa-debug-label-" + mode + "-v2"
        return {"schemaVersion": "1", "fixtureId": fid,
                "sourceSha256": labels.SOURCE_HASHES[fid], "compilerCommitSha": self.commit,
                "runId": self.run_id, "runAttempt": 1,
                "runNonce": fid + "-" + self.run_id + "-" + "b"*32, "mode": mode}

    def compiler(self, context):
        fid = context["fixtureId"]
        return {"syntheticOnly": True, "compilerCommitSha": self.commit, "fixtures": [{
            "fixtureId": fid, "fileName": fid + ".cherri", "sourceSha256": labels.SOURCE_HASHES[fid],
            "buildProfile": "debug", "runtimeTraceExpectation": "REQUIRED_NDJSON",
            "debugLabelSource": {"sourceSha256": labels.SOURCE_HASHES[fid]},
            "debugLabelGraph": {"scopedToReviewedSource": True}}]}

    def receipt(self, mode="typed"):
        context = self.context(mode)
        dates = mode.startswith("dated-")
        return {"schemaVersion": "1", "context": context, "budgetMs": 22000, "elapsedMs": 30,
                "maximumSwipes": 6, "observations": [{"sequence": 0, "startedMs": 0,
                    "observedMs": 10, "foreground": True, "previewContainer": True,
                    "doneExists": True, "doneHittable": True, "visibleLabels": [True]*5,
                    "visibleDateSuffixes": [dates]*5, "action": "DISMISS", "actionCompletedMs": 20}],
                "visibleLabels": [True]*5, "visibleDateSuffixes": [dates]*5, "swipeCount": 0,
                "dismissed": True, "ownershipLost": False, "result": "PASS_VISIBLE_LABELS",
                "creationDateValueIndependentlyVerified": False, "physicalDeviceVerified": False}

    def verify(self, receipt, context=None):
        context = context or receipt["context"]
        return preview.validate_receipt(receipt, self.compiler(context), context["fixtureId"],
            self.commit, self.run_id, context["runNonce"], 1)

    def write_fixture(self, root, mode="typed", semantic=True, include_receipt=True):
        context = self.context(mode)
        (root/"runtime").mkdir(parents=True)
        (root/"editor").mkdir()
        assertions = {"targetFixtureId": context["fixtureId"], "compilerCommitSha": self.commit,
            "compiledSourceSha256": context["sourceSha256"], "expectedRunId": context["runNonce"],
            "expectedSemanticOutput": labels.expected_summary(context["fixtureId"]),
            "runtimeContractVerified": semantic, "compiledShortcutCallbackVerified": semantic,
            "compiledShortcutRuntimeExecuted": semantic}
        files = {"compiler-manifest.json": self.compiler(context),
                 "summary.json": {"runId": self.run_id, "privacyGatePassed": True},
                 "runtime/output-assertions.json": assertions}
        if include_receipt:
            files[preview.RECEIPT] = self.receipt(mode)
        for name, value in files.items():
            (root/name).write_text(json.dumps(value), encoding="utf-8")
        return context

    def test_four_display_modes_replay_real_receipt_schema(self):
        for mode in ("typed", "dated-typed", "text", "dated-text"):
            with self.subTest(mode=mode):
                checked = self.verify(self.receipt(mode))
                self.assertTrue(checked["visibleLabelsVerified"])
                self.assertFalse(checked["physicalDeviceVerified"])
                self.assertFalse(checked["creationDateValueIndependentlyVerified"])

    def test_changed_source_run_nonce_mode_and_profile_are_rejected(self):
        baseline = self.receipt()
        for key, value in (("fixtureId", "qa-private"), ("sourceSha256", "c"*64),
                           ("compilerCommitSha", "c"*40), ("runId", "12346"),
                           ("runNonce", "stale"), ("mode", "text"), ("runAttempt", 2), ("runAttempt", False)):
            with self.subTest(field=key):
                damaged = copy.deepcopy(baseline)
                damaged["context"][key] = value
                with self.assertRaises((ValueError, TypeError, KeyError)):
                    self.verify(damaged, baseline["context"])
        compiler = self.compiler(baseline["context"])
        compiler["fixtures"][0]["buildProfile"] = "release"
        with self.assertRaises(ValueError):
            preview.validate_receipt(baseline, compiler, baseline["context"]["fixtureId"],
                                     self.commit, self.run_id, baseline["context"]["runNonce"])

    def test_extra_private_fields_or_unproved_scope_cannot_be_published(self):
        for location in ((), ("context",), ("observations", 0)):
            receipt = self.receipt()
            target = receipt
            for key in location:
                target = target[key]
            target["rawTitle"] = "unreviewed"
            with self.subTest(location=location), self.assertRaises(ValueError):
                self.verify(receipt)
        for key in ("creationDateValueIndependentlyVerified", "physicalDeviceVerified"):
            receipt = self.receipt();receipt[key] = True
            with self.subTest(field=key), self.assertRaises(ValueError):
                self.verify(receipt)

    def test_claimed_visible_labels_must_have_actual_observation_slots(self):
        for index in range(5):
            receipt = self.receipt()
            receipt["observations"][0]["visibleLabels"][index] = False
            with self.subTest(label=index), self.assertRaises(ValueError):
                self.verify(receipt)
        receipt = self.receipt("dated-text")
        receipt["observations"][0]["visibleDateSuffixes"][0] = False
        with self.assertRaises(ValueError):
            self.verify(receipt)

    def test_timing_is_bound_to_the_whole_observed_attempt(self):
        for key, value in (("startedMs", -1), ("observedMs", 300000),
                           ("actionCompletedMs", 300000), ("sequence", True),
                           ("actionCompletedMs", 1)):
            receipt = self.receipt();receipt["observations"][0][key] = value
            with self.subTest(field=key, value=value), self.assertRaises(ValueError):
                self.verify(receipt)
        receipt = self.receipt();receipt["elapsedMs"] = 1
        with self.assertRaises(ValueError):
            self.verify(receipt)
        receipt = self.receipt();receipt["observations"][0]["actionCompletedMs"] = 22001
        receipt["elapsedMs"] = 22002;receipt["result"] = "BLOCKED_PREVIEW_DEADLINE"
        self.assertFalse(self.verify(receipt)["visibleLabelsVerified"])
        receipt["result"] = "PASS_VISIBLE_LABELS"
        with self.assertRaises(ValueError):
            self.verify(receipt)

    def test_expired_or_unowned_observation_cannot_authorize_a_tap(self):
        for key, value in (("foreground", False), ("previewContainer", False),
                           ("doneExists", False), ("doneHittable", False), ("observedMs", 22000)):
            receipt = self.receipt();receipt["observations"][0][key] = value
            if key == "observedMs":
                receipt["observations"][0]["actionCompletedMs"] = 22001;receipt["elapsedMs"] = 22002
            with self.subTest(field=key), self.assertRaises(ValueError):
                self.verify(receipt)

    def test_non_dated_modes_reject_unexpected_date_suffixes(self):
        receipt = self.receipt("text")
        receipt["observations"][0]["visibleDateSuffixes"] = [True]*5
        receipt["visibleDateSuffixes"] = [True]*5
        receipt["result"] = "BLOCKED_PREVIEW_UNEXPECTED_DATES"
        self.assertFalse(self.verify(receipt)["visibleLabelsVerified"])
        receipt["result"] = "PASS_VISIBLE_LABELS"
        with self.assertRaises(ValueError):
            self.verify(receipt)

    def test_more_than_one_dismissal_or_fake_swipe_count_is_rejected(self):
        receipt = self.receipt()
        second = copy.deepcopy(receipt["observations"][0])
        second.update(sequence=1, startedMs=20, observedMs=21, actionCompletedMs=22)
        receipt["observations"].append(second)
        with self.assertRaises(ValueError):
            self.verify(receipt)
        receipt = self.receipt();receipt["swipeCount"] = 7
        with self.assertRaises(ValueError):
            self.verify(receipt)

    def test_missing_preview_preserves_native_callback_but_blocks_combined_result(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory);context = self.write_fixture(root, include_receipt=False)
            result = preview.finalize(root, context["fixtureId"], self.commit, self.run_id)
            self.assertTrue(result["semanticContractVerified"])
            self.assertFalse(result["runtimeContractVerified"])
            self.assertEqual(result["preview"]["result"], "NOT_OBSERVED")
            assertions = json.loads((root/"runtime/output-assertions.json").read_text())
            self.assertTrue(assertions["compiledShortcutCallbackVerified"])
            self.assertFalse(assertions["runtimeContractVerified"])
            preview.validate_public_diagnostic(result, root)

    def test_preview_never_overrides_a_failed_exact_native_callback(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory);context = self.write_fixture(root, semantic=False)
            original = (root/preview.RECEIPT).read_bytes()
            result = preview.finalize(root, context["fixtureId"], self.commit, self.run_id)
            self.assertTrue(result["preview"]["visibleLabelsVerified"])
            self.assertFalse(result["runtimeContractVerified"])
            self.assertEqual((root/preview.RECEIPT).read_bytes(), original)
            self.assertEqual(result["previewReceiptSha256"], hashlib.sha256(original).hexdigest())
            preview.validate_public_diagnostic(result, root)

    def test_plain_modes_require_exact_native_result_without_inventing_preview(self):
        for mode in ("plain", "dated"):
            with tempfile.TemporaryDirectory() as directory:
                root = Path(directory);context = self.write_fixture(root, mode=mode, include_receipt=False)
                result = preview.finalize(root, context["fixtureId"], self.commit, self.run_id)
                self.assertTrue(result["runtimeContractVerified"])
                self.assertEqual(result["preview"]["result"], "NOT_REQUIRED")
                self.assertFalse(result["preview"]["visibleLabelsVerified"])

    def test_context_creation_is_one_shot_for_exact_current_run(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory);context = self.write_fixture(root, include_receipt=False)
            preview.prepare(root, context["fixtureId"], self.commit, self.run_id, 1, context["runNonce"])
            before = (root/preview.CONTEXT).read_bytes()
            with self.assertRaises(FileExistsError):
                preview.prepare(root, context["fixtureId"], self.commit, self.run_id, 1, context["runNonce"])
            self.assertEqual((root/preview.CONTEXT).read_bytes(), before)

    def test_actual_publisher_preserves_valid_receipt_bytes_and_quarantines_context(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory);root = base/"raw";context = self.write_fixture(root)
            preview.prepare(root, context["fixtureId"], self.commit, self.run_id, 1, context["runNonce"])
            preview.finalize(root, context["fixtureId"], self.commit, self.run_id)
            original = (root/preview.RECEIPT).read_bytes()
            publisher.stage(root, base/"public", {context["fixtureId"]+".cherri": context["sourceSha256"]})
            self.assertEqual((base/"public"/preview.RECEIPT).read_bytes(), original)
            self.assertFalse((base/"public"/preview.CONTEXT).exists())
            receipt = json.loads(original);receipt["observations"][0]["visibleLabels"][0] = False
            (root/preview.RECEIPT).write_text(json.dumps(receipt), encoding="utf-8")
            with self.assertRaisesRegex(publisher.PublicationBlocked, "INVALID_BOUNDED_LABEL_PREVIEW"):
                publisher.stage(root, base/"tampered", {context["fixtureId"]+".cherri": context["sourceSha256"]})

    def test_json_duplicate_keys_and_symlinks_are_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory);(root/"duplicate.json").write_text('{"key":1,"key":2}')
            with self.assertRaises(ValueError):
                preview.read(root, "duplicate.json")
            target = root/"outside.json";target.write_text("{}")
            link = root/"link.json"
            try:
                link.symlink_to(target)
            except OSError:
                self.skipTest("symlink creation unavailable on this host; duplicate-key check executed")
            with self.assertRaises(ValueError):
                preview.read(root, "link.json")

    def test_actual_audit_helper_cannot_promote_failed_native_semantics(self):
        repository = Path(__file__).resolve().parents[2]
        sys.path.insert(0, str(repository/"tools/p10r15"))
        try:
            import audit_native_continuation as audit
        finally:
            sys.path.pop(0)
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory);context = self.write_fixture(root)
            preview.finalize(root, context["fixtureId"], self.commit, self.run_id)
            with patch.object(audit, "source_bytes", side_effect=lambda commit, path: (repository/path).read_bytes()):
                result = audit.label_preview_replay(root, self.commit, context["fixtureId"], True, 1)
                self.assertTrue(result["nativeVisibleLabelsVerified"])
                with self.assertRaisesRegex(ValueError, "actual workflow attempt"):
                    audit.label_preview_replay(root, self.commit, context["fixtureId"], True, 2)
                self.assertEqual(set(result["immutableHelperSha256"]), {"debug_label_acceptance.py", "debug_label_preview.py"})
                with self.assertRaisesRegex(ValueError, "cannot replace exact callback replay"):
                    audit.label_preview_replay(root, self.commit, context["fixtureId"], False, 1)
                changed = self.receipt();changed["elapsedMs"] = 1
                (root/preview.RECEIPT).write_text(json.dumps(changed), encoding="utf-8")
                with self.assertRaises(subprocess.CalledProcessError):
                    audit.label_preview_replay(root, self.commit, context["fixtureId"], True, 1)


if __name__ == "__main__":
    unittest.main()
