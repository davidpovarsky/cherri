"""Synthetic process controls; native execution is never inferred from these tests."""
import copy
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import optional_forwarding_diagnostic as gate


class OptionalArtifactGateTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.repo = self.root / "repo"
        real = Path(__file__).resolve().parents[2]
        for name in (gate.SOURCE_PATH, gate.SOURCE_PATH.replace(".cherri", ".expected.json")):
            target = self.repo / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes((real / name).read_bytes())
        self.artifacts = self.root / "artifacts"
        self.builds = self.artifacts / "builds"
        self.builds.mkdir(parents=True)
        self.path = self.builds / (gate.FIXTURE_ID + ".shortcut")
        # Deliberately not a valid native plist. Mocked process controls exercise
        # orchestration only; the real Go tests separately decode actual plists.
        self.path.write_bytes(b"SYNTHETIC PROCESS CONTROL ONLY")
        self.digest = hashlib.sha256(self.path.read_bytes()).hexdigest()
        self.receipt = {
            "schemaVersion": "1.0", "fixtureId": gate.FIXTURE_ID,
            "status": "PASS_NATIVE_STRUCTURE_ONLY", "sourceSha256": gate.SOURCE_SHA256,
            "expectedContractSha256": gate.CONTRACT_SHA256,
            "unsignedPlistSha256": self.digest, "nativeActionCount": 399,
            "appleRuntime": "NOT_RUN", "formalAcceptanceGranted": False,
        }

    def output(self, receipt=None):
        return ("    source_test.go:1: " + gate.GRAPH_MARKER +
                json.dumps(self.receipt if receipt is None else receipt) + "\n").encode()

    def invoke(self, stdout=None, side_effect=None, code=0):
        result = subprocess.CompletedProcess([], code, self.output() if stdout is None else stdout, b"")
        with patch.object(gate.subprocess, "run", return_value=result, side_effect=side_effect) as run:
            receipt = gate.verify_prepared_unsigned(self.repo, self.artifacts, gate.FIXTURE_ID, self.path, self.digest)
            return receipt, run

    def test_public_pins_and_exact_eleven_field_contract(self):
        source = (self.repo / gate.SOURCE_PATH).read_bytes()
        self.assertEqual(gate.verify_source(self.repo, gate.FIXTURE_ID, source)["observedFields"], 11)
        self.assertEqual(len(gate.EXPECTED_OUTPUT.encode()), 233)
        self.assertEqual(hashlib.sha256(gate.EXPECTED_OUTPUT.encode()).hexdigest(),
                         "86c2f4ae634e41a462d7b273f76513bd667a5f667f726f510885f54310585a1b")
        for changed in (source + b"\n", source.replace(b"none", b"false", 1)):
            with self.subTest(source_bytes=len(changed)), self.assertRaises(ValueError):
                gate.verify_source(self.repo, gate.FIXTURE_ID, changed)
        contract = self.repo / gate.SOURCE_PATH.replace(".cherri", ".expected.json")
        contract.write_bytes(contract.read_bytes() + b"\n")
        with self.assertRaises(ValueError):
            gate.verify_source(self.repo, gate.FIXTURE_ID, source)

    def test_unrelated_fixture_cannot_invoke_gate(self):
        with patch.object(gate.subprocess, "run") as run:
            self.assertIsNone(gate.verify_source(self.repo, "qa-unrelated", b""))
            self.assertIsNone(gate.verify_prepared_unsigned(self.repo, self.artifacts, "qa-unrelated", None, None))
            run.assert_not_called()

    def test_bound_receipt_retains_command_and_raw_process_digest(self):
        proof, run = self.invoke()
        args, kwargs = run.call_args
        self.assertEqual(args[0], ["go", "test", "-count=1", "-timeout=3m", "-run", "^TestP15OptionalNativeDiagnostic", "-v", "."])
        self.assertEqual(kwargs["cwd"], self.repo)
        self.assertEqual(kwargs["timeout"], 210)
        self.assertEqual(kwargs["env"]["CHERRI_OPTIONAL_DIAGNOSTIC_UNSIGNED"], str(self.path))
        self.assertEqual(kwargs["env"]["CHERRI_OPTIONAL_DIAGNOSTIC_SHA256"], self.digest)
        self.assertEqual(proof["testStdoutSha256"], hashlib.sha256(self.output()).hexdigest())
        self.assertFalse(proof["formalAcceptanceGranted"])
        self.assertEqual((self.builds / (gate.FIXTURE_ID + ".graph-guard.stdout.txt")).read_bytes(), self.output())

    def test_receipt_schema_truth_and_artifact_binding_mutations_reject(self):
        changes = [(key, "wrong") for key in self.receipt]
        changes += [("nativeActionCount", 398), ("nativeActionCount", 399.0),
                    ("formalAcceptanceGranted", 0), ("formalAcceptanceGranted", True),
                    ("appleRuntime", "PASS"), ("unreviewedPayload", "PRIVATE_CANARY")]
        for key, value in changes:
            receipt = {**self.receipt, key: value}
            with self.subTest(key=key, value=value), self.assertRaises(ValueError):
                self.invoke(self.output(receipt))
        for key in self.receipt:
            receipt = copy.deepcopy(self.receipt)
            del receipt[key]
            with self.subTest(missing=key), self.assertRaises(ValueError):
                self.invoke(self.output(receipt))

    def test_absent_duplicate_and_duplicate_key_receipts_reject(self):
        duplicate_key = self.output().replace(b'"nativeActionCount": 399', b'"nativeActionCount": 399, "nativeActionCount": 399')
        for output in (b"PASS\n", self.output() * 2, duplicate_key, (gate.GRAPH_MARKER + "null\n").encode()):
            with self.subTest(output_length=len(output)), self.assertRaises(ValueError):
                self.invoke(output)

    def test_failed_process_and_timeout_do_not_make_a_proof(self):
        with self.assertRaises(ValueError):
            self.invoke(code=1)
        with self.assertRaisesRegex(ValueError, "timed out"):
            self.invoke(side_effect=subprocess.TimeoutExpired(["go"], 210, output=b"partial stdout", stderr=b"partial stderr"))
        self.assertEqual((self.builds / (gate.FIXTURE_ID + ".graph-guard.stdout.txt")).read_bytes(), b"partial stdout")
        self.assertEqual((self.builds / (gate.FIXTURE_ID + ".graph-guard.stderr.txt")).read_bytes(), b"partial stderr")

    def test_artifact_changed_during_gate_rejects_real_bytes(self):
        def mutate(*args, **kwargs):
            self.path.write_bytes(b"changed artifact")
            return subprocess.CompletedProcess([], 0, self.output(), b"")
        with self.assertRaises(ValueError):
            self.invoke(side_effect=mutate)

    def test_containment_regular_file_and_digest_fail_before_go(self):
        outside = self.root / self.path.name
        outside.write_bytes(self.path.read_bytes())
        wrong_name = self.builds / "unrelated.shortcut"
        wrong_name.write_bytes(self.path.read_bytes())
        for path, digest in ((outside, self.digest), (wrong_name, self.digest),
                             (self.path, "0" * 64), (self.path, "not-a-digest")):
            with self.subTest(path=path.name, digest=digest[:8]), patch.object(gate.subprocess, "run") as run:
                with self.assertRaises(ValueError):
                    gate.verify_prepared_unsigned(self.repo, self.artifacts, gate.FIXTURE_ID, path, digest)
                run.assert_not_called()
        self.path.unlink()
        self.path.mkdir()
        with patch.object(gate.subprocess, "run") as run, self.assertRaises(ValueError):
            gate.verify_prepared_unsigned(self.repo, self.artifacts, gate.FIXTURE_ID, self.path, self.digest)
        run.assert_not_called()

    def test_contained_link_is_rejected_without_reading_contents(self):
        # Exercised with an actual link on POSIX, and a platform-neutral link
        # observation on Windows where unprivileged symlink creation is disabled.
        original = Path.is_symlink
        if os.name == "posix":
            target = self.builds / "synthetic-target"
            self.path.rename(target)
            self.path.symlink_to(target)
        with patch.object(Path, "is_symlink", lambda path: path == self.path or original(path)):
            with patch.object(gate.subprocess, "run") as run, self.assertRaisesRegex(ValueError, "linked path"):
                gate.verify_prepared_unsigned(self.repo, self.artifacts, gate.FIXTURE_ID, self.path, self.digest)
            run.assert_not_called()


if __name__ == "__main__":
    unittest.main()
