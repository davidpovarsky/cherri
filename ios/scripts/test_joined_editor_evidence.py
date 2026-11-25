"""Synthetic parser/fault controls; these tests never constitute native evidence."""
import copy
import hashlib
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import collect_ios_test_evidence as collector
import joined_editor_evidence as joined

ROOT = Path(__file__).resolve().parents[2]
SOURCE = "a" * 40


def model_log(tests=None):
    raw, contract = joined.load_contract(ROOT)
    lines = []
    for test in tests or joined.PROFILES:
        profile = joined.PROFILES[test]
        target = "CherriTests" if profile["producer"] == "ios-bridge" else "CherriUITests"
        suite, method = test.split("/")
        boundary = f"Test Case '-[{target}.{suite} {method}]'"
        lines.append(boundary + " started.")
        for sequence in range(len(contract["methods"][test]["stages"])):
            event = joined.expected_event(contract, joined.digest(raw), test, sequence)
            lines.append(joined.PREFIX + json.dumps(event, sort_keys=True, separators=(",", ":")))
        lines.append("[CHECKPOINT:" + profile["checkpoint"] + "]")
        lines.append(boundary + " passed (0.123 seconds).")
    return "\n".join(lines) + "\n"


def model_pins():
    pins = []
    for name in joined.SOURCE_PATHS:
        path = ROOT / name
        raw = path.read_bytes()
        pins.append({"path": name, "bytes": len(raw), "sha256": joined.digest(raw),
                     "gitBlob": hashlib.sha1(b"blob " + str(len(raw)).encode() + b"\0" + raw).hexdigest(),
                     "gitMode": "100755" if path.stat().st_mode & 0o100 else "100644"})
    return pins


class JoinedEvidenceControls(unittest.TestCase):
    def collect(self, raw=None, *, exit_code="0", attempt="7", diagnostic_only=False):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        directory = Path(temp.name).resolve(strict=True)
        log, code, output = directory / "raw.log", directory / "exit", directory / "output"
        log.write_text(model_log() if raw is None else raw)
        code.write_text(exit_code)
        # The logical collector control supplies explicit synthetic pins. The
        # real Git binding is checked independently below and in the Go intake.
        with mock.patch.object(joined, "source_pins", return_value=model_pins()), \
                mock.patch.dict(os.environ, {"GITHUB_RUN_ATTEMPT": attempt}):
            diagnostic, records = collector.collect(
                log, output, SOURCE, "1234567", code, diagnostic_only=diagnostic_only)
        return diagnostic, records, output

    def test_exact_three_native_tiers_and_no_lsp_or_benchmark_inheritance(self):
        diagnostic, records, output = self.collect()
        self.assertEqual({(row["requirements"][0], row["tier"], row["test_id"]) for row in records},
                         {("BRG25", "ios-build", joined.BRG25_BRIDGE),
                          ("BRG25", "ios-ui", joined.BRG25_UI),
                          ("BRG27", "ios-ui", joined.BRG27_UI)})
        self.assertEqual(diagnostic["joinedEditorEvidence"]["status"], "VERIFIED_JOINED_NATIVE")
        self.assertEqual((output / "xcodetest.log").read_text(), model_log())
        proof = json.loads((output / "joined-editor-proof.json").read_bytes())
        self.assertEqual(set(proof["methods"]), set(joined.PROFILES))
        self.assertEqual(proof["runAttempt"], 7)
        self.assertTrue(proof["noCrossTierInheritance"])
        for row in records:
            self.assertEqual(len(row["artifacts"]), 4)
            for artifact in row["artifacts"]:
                payload = (output / Path(artifact["path"]).name).read_bytes()
                self.assertEqual(joined.digest(payload), artifact["sha256"])

    def test_brg27_ui_needs_its_own_same_run_bridge_companion(self):
        raw = model_log([joined.BRG25_BRIDGE, joined.BRG25_UI, joined.BRG27_UI])
        _, records, _ = self.collect(raw)
        self.assertEqual({row["test_id"] for row in records}, {joined.BRG25_BRIDGE, joined.BRG25_UI})
        _, auxiliary, _ = self.collect(model_log([joined.BRG27_BRIDGE]))
        self.assertEqual(auxiliary, [])

    def test_stage_and_checkpoint_corruptions_fail_closed_without_private_text(self):
        original = model_log()
        lines = original.splitlines()
        event_index = next(i for i, line in enumerate(lines) if line.startswith(joined.PREFIX))
        event = json.loads(lines[event_index][len(joined.PREFIX):])
        cases = {}
        for key, value in [
            ("testId", joined.BRG27_BRIDGE), ("requirement", "BRG28"), ("producer", "ios-ui"),
            ("sequence", 0.0), ("sequence", True), ("sequence", 1),
            ("stage", "unexpected"), ("fixtureId", "rejected-type"),
            ("inputSha256", "0" * 64), ("observedSourceSha256", "0" * 64),
            ("contractSha256", "0" * 64), ("schema", "unknown"),
        ]:
            changed = dict(event); changed[key] = value
            copy_lines = list(lines)
            copy_lines[event_index] = joined.PREFIX + json.dumps(changed)
            cases[f"{key}-{value}"] = "\n".join(copy_lines) + "\n"
        extra = dict(event, privateText="CHERRI_PRIVATE_CANARY")
        cases["unexpected-private-field"] = original.replace(lines[event_index], joined.PREFIX + json.dumps(extra), 1)
        cases["duplicate-json-key"] = original.replace(lines[event_index], lines[event_index][:-1] + ',"sequence":0}', 1)
        cases["missing-stage"] = original.replace(lines[event_index] + "\n", "", 1)
        cases["duplicate-stage"] = original.replace(lines[event_index], lines[event_index] + "\n" + lines[event_index], 1)
        cases["event-outside"] = lines[event_index] + "\n" + original
        checkpoint = "[CHECKPOINT:" + joined.PROFILES[joined.BRG25_BRIDGE]["checkpoint"] + "]"
        cases["checkpoint-trailing-text"] = original.replace(checkpoint, checkpoint + " CHERRI_PRIVATE_CANARY", 1)
        cases["checkpoint-outside"] = checkpoint + "\n" + original
        cases["duplicate-checkpoint"] = original.replace(checkpoint, checkpoint + "\n" + checkpoint, 1)
        cases["missing-checkpoint"] = original.replace(checkpoint + "\n", "", 1)
        cases["event-after-checkpoint"] = original.replace(checkpoint, checkpoint + "\n" + lines[event_index], 1)
        cases["rejected-marker"] = original + joined.REJECTED + "CHERRI_PRIVATE_CANARY\n"
        for name, raw in cases.items():
            with self.subTest(name=name):
                diagnostic, records, output = self.collect(raw)
                self.assertEqual(records, [])
                self.assertNotEqual(diagnostic["joinedEditorEvidence"]["status"], "VERIFIED_JOINED_NATIVE")
                self.assertNotIn("CHERRI_PRIVATE_CANARY", (output / "xcodetest.log").read_text())

    def test_case_ownership_and_actual_run_controls(self):
        original = model_log()
        first_end = original.index(" passed (0.123 seconds).") + len(" passed (0.123 seconds).")
        first_case = original[:first_end] + "\n"
        cases = {
            "duplicate-case": original + first_case,
            "failed-case": original.replace("passed (", "failed (", 1),
            "missing-terminal": original[:original.rfind("Test Case '")],
            "wrong-target": original.replace("CherriTests.CherriJoinedBridgeTests", "CherriUITests.CherriJoinedBridgeTests", 1),
            "unsupported-boundary": original.replace(" started.", " skipped.", 1),
            "foreign-start": original.replace(joined.PREFIX, "Test Case '-[Foreign.Unknown testOther]' started.\n" + joined.PREFIX, 1),
        }
        for name, raw in cases.items():
            with self.subTest(name=name):
                _, records, _ = self.collect(raw)
                self.assertEqual(records, [])
        for kwargs in ({"exit_code": "65"}, {"exit_code": ""}, {"attempt": ""},
                       {"attempt": "0"}, {"attempt": "true"}, {"diagnostic_only": True}):
            with self.subTest(kwargs=kwargs):
                _, records, _ = self.collect(**kwargs)
                self.assertEqual(records, [])

    def test_failed_native_terminal_remains_failed_and_does_not_poison_next_case(self):
        original = model_log([joined.BRG25_UI, joined.BRG27_UI])
        raw = original.replace(" passed (", " failed (", 1)
        diagnostic, records, output = self.collect(raw, exit_code="65")
        self.assertEqual(records, [])
        self.assertEqual(diagnostic["testResults"], [
            {"testId": joined.BRG25_UI, "observedStatus": "failed"},
            {"testId": joined.BRG27_UI, "observedStatus": "passed"},
        ])
        retained = (output / "xcodetest.log").read_text()
        self.assertIn(" failed (0.123 seconds).", retained)
        self.assertIn(joined.REJECTED + "CASE_TERMINAL", retained)
        self.assertNotIn(joined.REJECTED + "INTERLEAVED_CASE", retained)
        self.assertIn('[CHECKPOINT:' + joined.PROFILES[joined.BRG27_UI]["checkpoint"] + ']', retained)
        self.assertEqual(diagnostic["joinedEditorEvidence"]["status"], "NOT_VERIFIED")
        guard = joined.LogGuard(ROOT)
        for line in raw.splitlines():
            guard.observe(line)
        self.assertIsNone(guard.active)
        self.assertEqual(guard.rejections, ["CASE_TERMINAL"])
        with self.assertRaisesRegex(joined.Rejected, "REJECTED_LOG"):
            guard.verified()

    def test_real_source_binding_rejects_false_head_and_changed_physical_bytes(self):
        with self.assertRaisesRegex(joined.Rejected, "SOURCE_HEAD"):
            joined.source_pins(ROOT, "0" * 40)
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp).resolve(strict=True)
            path = root / joined.SOURCE_PATHS[0]
            path.parent.mkdir(parents=True)
            path.write_bytes(b"changed physical bytes")
            old = b"expected committed bytes"
            blob = hashlib.sha1(b"blob " + str(len(old)).encode() + b"\0" + old).hexdigest()
            head = mock.Mock(returncode=0, stdout=(SOURCE + "\n").encode())
            tree = mock.Mock(returncode=0, stdout=f"100644 blob {blob}\t{joined.SOURCE_PATHS[0]}\0".encode())
            with mock.patch.object(joined.subprocess, "run", side_effect=[head, tree]):
                with self.assertRaisesRegex(joined.Rejected, "SOURCE_BYTES"):
                    joined.source_pins(root, SOURCE)

    def test_source_contract_and_swift_emitters_have_exact_fixed_methods(self):
        raw, contract = joined.load_contract(ROOT)
        self.assertEqual(set(contract["methods"]), set(joined.PROFILES))
        for test, profile in joined.PROFILES.items():
            source = (ROOT / profile["sourcePath"]).read_text()
            self.assertIn("func " + test.split("/")[1] + "(", source)
            self.assertIn('testId: "' + test + '"', source)
        helper = (ROOT / "ios/TestSupport/CherriJoinedContract.swift").read_text()
        self.assertIn('print("[CHECKPOINT:\\(method.checkpoint)]")', helper)
        changed = copy.deepcopy(contract)
        changed["fixtures"]["empty"]["sourceSha256"] = "0" * 64
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp);path = root / joined.CONTRACT_PATH
            path.parent.mkdir(parents=True);path.write_text(json.dumps(changed))
            with self.assertRaisesRegex(joined.Rejected, "FIXTURE_HASH"):
                joined.load_contract(root)
        self.assertEqual(joined.digest(raw), "8d28cbde2ad7ae1c967cb4c4e177a10cc12b10223e070b78ee44977725224474")


if __name__ == "__main__":
    unittest.main()
