"""Synthetic policy controls only; these tests never execute Apple runtime."""
import json
from pathlib import Path
import re
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import runtime_poc_device as device
import runtime_poc_publication as publication
from prepare_public_artifacts import PublicationBlocked, sha

COMMIT = "a" * 40
RUN = "123456789"
ATTEMPT = "2"
UDID = "12345678-1234-1234-1234-1234567890ab"
REPO = Path(__file__).resolve().parents[2]


class OwnedSimulatorTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.calls = []

    def tearDown(self):
        self.temp.cleanup()

    def command(self, args, **kwargs):
        self.calls.append(args)
        if args == ["xcrun", "simctl", "list", "runtimes", "-j"]:
            return subprocess.CompletedProcess(args, 0, json.dumps({"runtimes": [
                {"identifier": "com.apple.CoreSimulator.SimRuntime.iOS-27-0", "isAvailable": True}
            ]}), "")
        if args[:3] == ["xcrun", "simctl", "create"]:
            return subprocess.CompletedProcess(args, 0, UDID + "\n", "")
        if args[:3] in (["xcrun", "simctl", "shutdown"], ["xcrun", "simctl", "delete"]):
            return subprocess.CompletedProcess(args, 0, "", "")
        raise AssertionError("unexpected simulator operation: " + repr(args))

    def test_creates_fresh_device_and_deletes_only_its_exact_udid(self):
        receipt = device.create(self.root, COMMIT, RUN, ATTEMPT, run=self.command)
        self.assertEqual(receipt["udid"], UDID)
        self.assertEqual(len(self.calls), 2)
        self.assertEqual(self.calls[1][:3], ["xcrun", "simctl", "create"])
        self.assertTrue(self.calls[1][3].startswith("Cherri-PoC-" + RUN + "-" + ATTEMPT + "-"))
        self.assertFalse(any("devices" in args for args in self.calls))
        receipt = device.cleanup(self.root, COMMIT, RUN, ATTEMPT, UDID, run=self.command)
        self.assertEqual(receipt["cleanupStatus"], "DELETED")
        self.assertEqual(self.calls[2:], [
            ["xcrun", "simctl", "shutdown", UDID],
            ["xcrun", "simctl", "delete", UDID],
        ])
        device.cleanup(self.root, COMMIT, RUN, ATTEMPT, UDID, run=self.command)
        self.assertEqual(len(self.calls), 4, "repeat cleanup must not issue another delete")

    def test_unknown_device_or_foreign_run_cannot_be_deleted(self):
        device.create(self.root, COMMIT, RUN, ATTEMPT, run=self.command)
        for commit, run_id, attempt, udid in [
            ("b" * 40, RUN, ATTEMPT, UDID), (COMMIT, RUN + "1", ATTEMPT, UDID),
            (COMMIT, RUN, "3", UDID), (COMMIT, RUN, ATTEMPT, "booted"),
            (COMMIT, RUN, ATTEMPT, "22345678-1234-1234-1234-1234567890ab"),
        ]:
            with self.subTest(run_id=run_id, udid=udid), self.assertRaises(PublicationBlocked):
                device.cleanup(self.root, commit, run_id, attempt, udid, run=self.command)
        self.assertEqual(len(self.calls), 2)

    def test_missing_runtime_never_falls_back_to_existing_device(self):
        def empty(args, **kwargs):
            self.calls.append(args)
            return subprocess.CompletedProcess(args, 0, '{"runtimes":[]}', "")
        with self.assertRaises(PublicationBlocked):
            device.create(self.root, COMMIT, RUN, ATTEMPT, run=empty)
        self.assertEqual(self.calls, [["xcrun", "simctl", "list", "runtimes", "-j"]])


class FormalPublicationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.base = Path(self.temp.name)
        self.repo, self.root = self.base / "repo", self.base / "raw"
        self.repo.mkdir()
        self.root.mkdir()
        self.original_sources = {}
        for name in publication.SOURCE_HASHES:
            relative = "tests/runtime_poc/" + name + ".cherri"
            data = (REPO / relative).read_bytes()
            self.original_sources[relative] = data
            target = self.repo / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(data)
            (self.root / (name + ".cherri")).write_bytes(data)
            (self.root / (name + ".shortcut")).write_bytes(b"AEA1SYNTHETIC_POLICY_UNIT_CONTROL_" + name.encode())
        self.git = patch.object(publication.subprocess, "check_output", side_effect=self.git_read)
        self.git.start()
        self.addCleanup(self.git.stop)
        publication.check_sources(self.repo, self.root, COMMIT, RUN, ATTEMPT)
        for name in publication.FORMAL_TEXT:
            data = b'{"scope":"SYNTHETIC_POLICY_UNIT_CONTROL"}\n' if name.endswith(".json") else b"SYNTHETIC_POLICY_UNIT_CONTROL\n"
            (self.root / name).write_bytes(data)
        (self.root / device.RECEIPT).write_text(json.dumps({
            "implementationSha": COMMIT, "runId": RUN, "runAttempt": ATTEMPT,
            "createdByRun": True, "cleanupStatus": "DELETED",
        }), encoding="utf-8")
        rows = [{"fixtureId": name, "sourceSha256": expected, "unsignedSha256": "d" * 64,
                 "signedSha256": sha((self.root / (name + ".shortcut")).read_bytes()),
                 "signedArtifact": name + ".shortcut", "signedMagicVerified": True,
                 "signingMethod": "native"} for name, expected in publication.SOURCE_HASHES.items()]
        (self.root / publication.SIGNING_RECEIPT).write_text(json.dumps({
            "implementationSha": COMMIT, "runId": RUN, "runAttempt": ATTEMPT, "fixtures": rows,
        }), encoding="utf-8")
        self.write_evidence()

    def tearDown(self):
        self.temp.cleanup()

    def git_read(self, args, **kwargs):
        if args[-2:] == ["rev-parse", "HEAD"]:
            return COMMIT + "\n"
        if len(args) >= 5 and args[-2] == "show":
            self.assertTrue(args[-1].startswith(COMMIT + ":"))
            return self.original_sources[args[-1].split(":", 1)[1]]
        raise AssertionError("unexpected Git operation: " + repr(args))

    def write_evidence(self):
        refs = [{"path": publication.CI_PREFIX + name, "run_id": RUN,
                 "sha256": sha((self.root / name).read_bytes())}
                for name in sorted(publication.FORMAL_FILES)]
        # Deliberately failed: publication is never native execution credit.
        self.records = [{"implementation_sha": COMMIT, "tier": "ios-runtime",
                         "test_id": "ios27-runtime-poc:smoke", "result": "failed",
                         "exit_code": 1, "runtime_pass": False, "artifacts": refs,
                         "detail": "SYNTHETIC_POLICY_UNIT_CONTROL_NOT_APPLE_EXECUTION"}]
        self.save_records()

    def save_records(self):
        (self.root / "runtime-evidence.json").write_text(json.dumps(self.records), encoding="utf-8")

    def stage(self):
        return publication.stage(self.repo, self.root, self.base / "public", COMMIT, RUN, ATTEMPT)

    def assert_blocked(self):
        with self.assertRaises(PublicationBlocked):
            self.stage()
        self.assertFalse((self.base / "public").exists(), "failed policy must publish nothing")

    def test_preserves_every_formal_byte_and_excludes_raster_and_system_logs(self):
        (self.root / "failure.png").write_bytes(b"\x89PNG\r\n\x1a\nSYNTHETIC")
        (self.root / "shortcuts-runtime.log").write_text("CHERRI_PRIVATE_CANARY", encoding="utf-8")
        before = {p.name: p.read_bytes() for p in self.root.iterdir() if p.is_file()}
        report = self.stage()
        self.assertIs(report["formalEvidenceBytesUnchanged"], True)
        self.assertIs(report["rasterPublicationAllowed"], False)
        self.assertIs(report["systemLogPublicationAllowed"], False)
        names = {row["path"] for row in report["published"]}
        self.assertEqual(names, publication.FORMAL_FILES | publication.SUPPORT | {"runtime-evidence.json"})
        for name in names:
            self.assertEqual((self.base / "public" / name).read_bytes(), before[name])
        self.assertFalse((self.base / "public" / "failure.png").exists())
        self.assertFalse((self.base / "public" / "shortcuts-runtime.log").exists())
        self.assertEqual(json.loads((self.base / "public" / "runtime-evidence.json").read_bytes())[0]["result"], "failed")

    def test_allowlist_matches_every_existing_formal_producer_reference(self):
        source = (REPO / "scripts/ios27_runtime_poc.sh").read_text(encoding="utf-8")
        refs = re.findall(r'file_art\("artifacts/([^"/]+)", "([^"\n]+)"\)', source)
        self.assertEqual(len(refs), len(publication.FORMAL_FILES))
        self.assertEqual({name for name, _ in refs}, publication.FORMAL_FILES)
        self.assertTrue(all(path == publication.CI_PREFIX + name for name, path in refs))

    def test_modified_source_blocks_before_publication(self):
        (self.repo / "tests/runtime_poc/CherriRuntimePOC.cherri").write_bytes(b"private input")
        self.assert_blocked()

    def test_changed_signed_bytes_are_rejected(self):
        (self.root / "CherriRuntimePOC.shortcut").write_bytes(b"AEA1WRONG_PUBLIC_BYTES")
        self.write_evidence()  # Even a refreshed evidence hash cannot replace signing provenance.
        self.assert_blocked()

    def test_foreign_record_or_artifact_run_is_rejected(self):
        self.records[0]["artifacts"][0]["run_id"] = "999"
        self.save_records()
        self.assert_blocked()

    def test_unreviewed_and_traversing_references_are_rejected(self):
        for path in (publication.CI_PREFIX + "../private.txt", publication.CI_PREFIX + "failure.png"):
            with self.subTest(path=path):
                self.records[0]["artifacts"][0]["path"] = path
                self.save_records()
                self.assert_blocked()

    def test_escaped_private_json_and_duplicate_keys_are_rejected(self):
        original = (self.root / "menu-alpha-selection-proof.json").read_bytes()
        for raw in (b'{"data":"CHERRI_PRIVATE_\\u0043ANARY"}', b'{"a":1,"a":2}'):
            with self.subTest(data=raw):
                (self.root / "menu-alpha-selection-proof.json").write_bytes(raw)
                self.write_evidence()
                self.assert_blocked()
        (self.root / "menu-alpha-selection-proof.json").write_bytes(original)

    def test_binary_disguised_as_text_is_rejected(self):
        (self.root / "clipboard-result.txt").write_bytes(b"\x89PNG\r\n\x1a\nSYNTHETIC")
        self.write_evidence()
        self.assert_blocked()

    def test_referenced_symlink_is_rejected_without_following_it(self):
        original = self.root / "clipboard-result.txt"
        outside = self.base / "outside.txt"
        outside.write_bytes(original.read_bytes())
        original.unlink()
        try:
            original.symlink_to(outside)
        except OSError:
            self.skipTest("host cannot create symlink; POSIX CI remains required")
        self.assert_blocked()

    def test_non_owned_or_uncleaned_simulator_blocks_publication(self):
        path = self.root / device.RECEIPT
        value = json.loads(path.read_bytes())
        value["cleanupStatus"] = "PENDING"
        path.write_text(json.dumps(value), encoding="utf-8")
        self.assert_blocked()


if __name__ == "__main__":
    unittest.main()
