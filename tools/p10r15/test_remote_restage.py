"""Real file controls for repeated immutable Remote Builder staging.

The input bytes are explicit synthetic fixtures. No compiler, signing endpoint
or installed Remote Builder is run by these component tests.
"""
import copy
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import remote_stage_baseline as baseline
from remote_release_gates import PROTECTED, TOOL_PATHS


class SuccessorStageBaseline(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="p15-successor-stage-")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.live = self.root / "live"
        self.tuple = {
            "label": "p15-synthetic-old",
            "compilerCommit": "1" * 40, "docsCommit": "2" * 40,
            "skillSourceHead": "3" * 40,
            "schemaFingerprint": "4" * 64, "codecFingerprint": "5" * 64,
            "macroFingerprint": "6" * 64, "compilerSHA256": "7" * 64,
            "migrationSHA256": "8" * 64, "sourceBranch": "agent/synthetic-old",
        }
        hashes = {}
        for name in sorted(set(PROTECTED) | TOOL_PATHS):
            path = self.live / name
            path.parent.mkdir(parents=True, exist_ok=True)
            raw = json.dumps(self.tuple).encode() if name == "active.json" else (
                "explicit synthetic original: " + name).encode()
            path.write_bytes(raw)
            hashes[name] = hashlib.sha256(raw).hexdigest()
        self.record = {
            "status": "STAGED_NOT_ACTIVATED", "livePromotion": False,
            "tuple": self.tuple, "candidateHashes": hashes,
            "externalToolchainDependencies": {name: hashes[name] for name in TOOL_PATHS},
        }
        self.path = self.root / "previous-stage.json"
        self.save()
        # Synthetic fixture content is intentionally not claimed to be the
        # production linter or production wrappers. Live K13 replay is separate.
        self.addCleanup(patch.stopall)
        patch.dict(baseline.ORIGINAL_LIVE_HASHES, {
            name: hashes[name] for name in baseline.ORIGINAL_LIVE_HASHES}).start()

    def save(self, record=None):
        self.path.write_text(json.dumps(self.record if record is None else record))
        self.sha = hashlib.sha256(self.path.read_bytes()).hexdigest()

    def load(self):
        return baseline.load_previous_stage(self.path, self.sha, self.live)

    def test_complete_current_release_is_read_only_and_binds_all_ten_files(self):
        before = {str(p): p.read_bytes() for p in self.root.rglob("*") if p.is_file()}
        hashes, witness = self.load()
        self.assertEqual(len(hashes), 10)
        self.assertEqual(hashes, self.record["candidateHashes"])
        self.assertEqual(witness["tuple"], self.tuple)
        self.assertTrue(witness["liveBytesMatch"])
        self.assertFalse(witness["acceptanceGranted"])
        self.assertEqual(before, {str(p): p.read_bytes() for p in self.root.rglob("*") if p.is_file()})

    def test_each_stale_live_file_is_rejected_without_restamping(self):
        for name in self.record["candidateHashes"]:
            with self.subTest(file=name):
                path = self.live / name
                original = path.read_bytes()
                path.write_bytes(original + b"\nchanged")
                with self.assertRaisesRegex(ValueError, "digest differs"):
                    self.load()
                self.assertEqual(path.read_bytes(), original + b"\nchanged")
                path.write_bytes(original)

    def test_changed_receipt_is_rejected_even_when_json_is_valid(self):
        self.path.write_text(self.path.read_text() + "\n")
        with self.assertRaisesRegex(ValueError, "digest differs"):
            self.load()

    def test_duplicate_receipt_key_is_rejected(self):
        self.path.write_text('{"status":"other",' + self.path.read_text()[1:])
        self.sha = hashlib.sha256(self.path.read_bytes()).hexdigest()
        with self.assertRaisesRegex(ValueError, "Duplicate JSON key"):
            self.load()

    def test_missing_and_extra_protected_paths_are_rejected(self):
        for mutate in ("missing", "extra"):
            with self.subTest(mutate=mutate):
                record = copy.deepcopy(self.record)
                if mutate == "missing":
                    record["candidateHashes"].pop("active.json")
                else:
                    record["candidateHashes"]["unrelated"] = "a" * 64
                self.save(record)
                with self.assertRaisesRegex(ValueError, "protected inventory"):
                    self.load()

    def test_active_tuple_mismatch_is_rejected_after_digest_matches(self):
        self.record["tuple"] = dict(self.tuple, compilerCommit="f" * 40)
        self.save()
        with self.assertRaisesRegex(ValueError, "active tuple differs"):
            self.load()

    def test_toolchain_binding_mismatch_is_rejected(self):
        self.record["externalToolchainDependencies"]["tools/go/VERSION"] = "a" * 64
        self.save()
        with self.assertRaisesRegex(ValueError, "toolchain binding"):
            self.load()

    def test_changed_validator_requires_template_review(self):
        baseline.ORIGINAL_LIVE_HASHES["bin/cherri-shortcut-lint.py"] = "a" * 64
        with self.assertRaisesRegex(ValueError, "validator requires"):
            self.load()

    def test_receipt_symlink_is_rejected(self):
        real = self.path.with_suffix(".original")
        self.path.rename(real)
        self.path.symlink_to(real)
        with self.assertRaisesRegex(ValueError, "regular original"):
            self.load()

    def test_live_file_symlink_is_rejected(self):
        path = self.live / "bin/cherri-remote-runner.sh"
        real = path.with_suffix(".original")
        path.rename(real)
        path.symlink_to(real)
        with self.assertRaisesRegex(ValueError, "regular original"):
            self.load()

    def test_predecessor_options_are_all_required(self):
        runner = self.live / "bin/cherri-remote-runner.sh"
        legacy = self.live / "bin/cherri-remote-legacy.sh"
        options = [self.path, self.sha, runner, legacy]
        for index in range(4):
            with self.subTest(missing=index):
                selected = list(options)
                selected[index] = None
                with self.assertRaisesRegex(ValueError, "requires its receipt"):
                    baseline.wrapper_templates(self.live, *selected)

    def test_initial_and_successor_routes_use_the_same_pinned_original_templates(self):
        runner = self.live / "bin/cherri-remote-runner.sh"
        legacy = self.live / "bin/cherri-remote-legacy.sh"
        initial = baseline.wrapper_templates(self.live, None, None, None, None)
        successor = baseline.wrapper_templates(self.live, self.path, self.sha, runner, legacy)
        self.assertIsNone(initial[1])
        self.assertEqual(initial[2], successor[2])
        self.assertEqual(initial[3], successor[3])
        self.assertEqual(successor[1]["tuple"], self.tuple)

    def test_changed_template_is_rejected_with_unchanged_live_files(self):
        runner = self.root / "runner.original"
        runner.write_bytes((self.live / "bin/cherri-remote-runner.sh").read_bytes() + b"changed")
        with self.assertRaisesRegex(ValueError, "digest differs"):
            baseline.wrapper_templates(
                self.live, self.path, self.sha, runner,
                self.live / "bin/cherri-remote-legacy.sh")


if __name__ == "__main__":
    unittest.main()
