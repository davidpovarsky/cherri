"""Offline provenance controls; no network, model request or compiler call."""
import copy
import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock

import evaluation_bundle as eb


class ProvenanceControls(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        suite, parameters = eb.generate_suite(bytes(range(32)))
        self.bundle = {"root": self.root, "suite": suite, "parameters": parameters,
            "guides": {"legacy": "offline legacy control", "v2": "offline v2 control"},
            "manifest": {"provider": eb.PROVIDER, "budget": {"max_input_tokens": 12000, "max_output_tokens": 2000},
                "order_seed": 13, "suite_sha256": eb.digest(eb.canonical(suite)),
                "oracle_sha256": eb.digest(eb.canonical({t["id"]: t["oracle"] for t in suite["tasks"]})),
                "seed_commitment_sha256": eb.digest(bytes(range(32))), "public_key_sha256": "a"*64,
                "created_at": "2026-11-21T20:00:00Z"}, "anchor": {"story_id": "1220000000000000"}}
        (self.root / "signature.bin").write_bytes(b"s"*64)

    def test_generator_is_reproducible_and_retains_original_six_contracts(self):
        original = json.loads(Path(eb.__file__).with_name("tasks.json").read_text(encoding="utf-8"))
        first = eb.generate_suite(bytes(range(32)))
        self.assertEqual(first, eb.generate_suite(bytes(range(32))))
        second = eb.generate_suite(bytes(reversed(range(32))))
        self.assertNotEqual(first, second)
        self.assertEqual([t["id"] for t in first[0]["tasks"]], list(eb.TASK_IDS))
        for fresh, old in zip(first[0]["tasks"], original["tasks"]):
            self.assertEqual(fresh["kind"], old["kind"])
            self.assertEqual(set(fresh["oracle"]), set(old["oracle"]))
            self.assertEqual(set(fresh["starter"]), {"legacy", "v2"})
        self.assertEqual(eb.digest(Path(eb.__file__).with_name("tasks.json").read_bytes()), eb.ORIGINAL_SUITE_SHA256)

    def test_exactly_twelve_distinct_paired_requests_without_oracle(self):
        sequence = eb.request_sequence(self.bundle)
        self.assertEqual(len(sequence), 12)
        self.assertEqual(len({r["request_id"] for r in sequence}), 12)
        self.assertEqual(sequence, eb.request_sequence(self.bundle))
        for request in sequence:
            eb.validate_model_request(request, self.bundle)
            self.assertNotIn("oracle", request)
            self.assertNotIn("parameters", request)
            self.assertNotIn("seed_commitment_sha256", request)
        for task_id in eb.TASK_IDS:
            arms = [r for r in sequence if r["task_id"] == task_id]
            self.assertEqual({r["arm"] for r in arms}, {"legacy", "v2"})
            self.assertEqual(arms[0]["prompt"], arms[1]["prompt"])
            self.assertEqual(arms[0]["budget"], arms[1]["budget"])
            self.assertEqual(arms[0]["model"], arms[1]["model"])

    def test_extra_oracle_and_changed_model_guide_budget_or_source_identity_reject(self):
        base = eb.request_sequence(self.bundle)[0]
        mutations = [lambda r: r.update(oracle={"secret": 1}), lambda r: r.update(model="other"),
                     lambda r: r.update(guide="other"), lambda r: r["budget"].update(max_input_tokens=12001),
                     lambda r: r.update(suite_sha256="0"*64), lambda r: r.update(prompt=r["prompt"]+" changed")]
        for change in mutations:
            value = copy.deepcopy(base)
            change(value)
            with self.assertRaisesRegex(eb.BundleError, "MODEL_REQUEST_NOT_SEALED_PUBLIC_INPUT"):
                eb.validate_model_request(value, self.bundle)

    def test_atomic_attempt_claim_prevents_any_retry(self):
        request = eb.request_sequence(self.bundle)[0]
        directory = eb.claim_request(self.bundle, request, "provider")
        self.assertTrue((directory / "intent.json").is_file())
        with self.assertRaisesRegex(eb.BundleError, "ATTEMPT_ALREADY_CLAIMED_NO_RETRY"):
            eb.claim_request(self.bundle, request, "provider")
        eb.claim_request(self.bundle, request, "validator")
        with self.assertRaisesRegex(eb.BundleError, "ATTEMPT_NOT_IN_SEALED_SUITE"):
            eb.claim_request(self.bundle, {"request_id": "0"*64}, "provider")

    def test_asana_task_read_binds_exact_comment_and_server_time(self):
        story = {"gid": "1220000000000001", "resource_subtype": "comment_added",
                 "text": eb.anchor_text(self.bundle), "created_at": "2026-11-21T20:00:01.123Z"}
        response = {"data": {"gid": eb.ASANA_TASK, "comments": [story]}}
        self.assertEqual(eb.validate_anchor(response, self.bundle)["story_id"], story["gid"])
        for change in [lambda x: x["data"].update(gid="1220000000000009"),
                       lambda x: x["data"]["comments"][0].update(created_at="2026-11-21T19:59:59Z"),
                       lambda x: x["data"]["comments"][0].update(text="different commitment"),
                       lambda x: x["data"]["comments"].append(copy.deepcopy(story))]:
            changed = copy.deepcopy(response)
            change(changed)
            with self.assertRaises(eb.BundleError):
                eb.validate_anchor(changed, self.bundle)

    def test_ed25519_signatures_reject_changed_manifest_signature_and_key(self):
        key = self.root / "key.pem"
        public = self.root / "public.pem"
        manifest = self.root / "manifest.json"
        signature = self.root / "signature-real.bin"
        eb.crypto("genpkey", "-algorithm", "ED25519", "-out", key)
        eb.crypto("pkey", "-in", key, "-pubout", "-out", public)
        raw = eb.canonical({"synthetic_control": True, "caps": [12000, 2000]})
        manifest.write_bytes(raw)
        eb.crypto("pkeyutl", "-sign", "-rawin", "-inkey", key, "-in", manifest, "-out", signature)
        eb.verify_signature(raw, public.read_bytes(), signature.read_bytes())
        with self.assertRaises(eb.BundleError):
            eb.verify_signature(raw.replace(b"12000", b"12001"), public.read_bytes(), signature.read_bytes())
        with self.assertRaises(eb.BundleError):
            eb.verify_signature(raw, public.read_bytes(), bytes(64))
        eb.crypto("genpkey", "-algorithm", "ED25519", "-out", self.root / "other-key.pem")
        eb.crypto("pkey", "-in", self.root / "other-key.pem", "-pubout", "-out", self.root / "other-public.pem")
        with self.assertRaises(eb.BundleError):
            eb.verify_signature(raw, (self.root / "other-public.pem").read_bytes(), signature.read_bytes())

    def test_duplicate_json_and_nonfinite_values_are_not_reconciled(self):
        for raw in ('{"model":"a","model":"b"}', '{"tokens":NaN}', '{"tokens":Infinity}'):
            with self.assertRaises(eb.BundleError):
                eb.unique_json(raw)

    def test_symlink_input_cannot_supply_an_original(self):
        actual = self.root / "real.json"
        actual.write_text("{}")
        link = self.root / "link.json"
        try:
            link.symlink_to(actual)
        except OSError:
            self.skipTest("Host does not permit symlink creation")
        with self.assertRaisesRegex(eb.BundleError, "INPUT_NOT_REGULAR"):
            eb.read_regular(link)

    def test_growing_file_is_read_with_a_bound_and_rejected(self):
        actual = self.root / "growing.json"
        actual.write_bytes(b"{}")
        real_open = Path.open
        reads = []
        class GrowingStream:
            def __init__(self, stream):
                self.stream = stream
            def __enter__(self):
                return self
            def __exit__(self, *args):
                return self.stream.__exit__(*args)
            def fileno(self):
                return self.stream.fileno()
            def read(self, size):
                reads.append(size)
                with real_open(actual, "ab") as out:
                    out.write(b"x" * 32)
                return self.stream.read(size)
        def opened(path, *args, **kwargs):
            stream = real_open(path, *args, **kwargs)
            return GrowingStream(stream) if path == actual and args == ("rb",) else stream
        with mock.patch.object(Path, "open", opened):
            with self.assertRaisesRegex(eb.BundleError, "INPUT_SIZE_LIMIT"):
                eb.read_regular(actual, 8)
        self.assertEqual(reads, [9])

    def test_handle_and_path_time_semantics_do_not_confuse_file_identity(self):
        actual = self.root / "ctime-control.json"
        actual.write_bytes(b"{}")
        real_fstat = eb.os.fstat
        def handle_stat(fd):
            observed = real_fstat(fd)
            fields = {key: getattr(observed, key) for key in (
                "st_dev", "st_ino", "st_mode", "st_size", "st_mtime_ns", "st_ctime_ns")}
            fields["st_ctime_ns"] += 100
            return SimpleNamespace(**fields)
        with mock.patch.object(eb.os, "fstat", side_effect=handle_stat):
            self.assertEqual(eb.read_regular(actual, 8), b"{}")

    def test_windows_exe_execute_bits_do_not_confuse_file_identity(self):
        actual = self.root / "public-control.exe"
        actual.write_bytes(b"public synthetic executable bytes")
        actual.chmod(0o755)
        real_fstat = eb.os.fstat
        calls = []
        def handle_stat(fd):
            observed = real_fstat(fd)
            fields = {key: getattr(observed, key) for key in (
                "st_dev", "st_ino", "st_mode", "st_size", "st_mtime_ns", "st_ctime_ns")}
            fields["st_mode"] &= ~0o111
            calls.append(fields)
            return SimpleNamespace(**fields)
        with mock.patch.object(eb, "os", SimpleNamespace(name="nt", fstat=handle_stat)):
            self.assertEqual(eb.read_regular(actual, 64), b"public synthetic executable bytes")
        self.assertEqual(len(calls), 2)

    def test_windows_same_handle_mode_change_still_rejects_after_read(self):
        actual = self.root / "changing-mode.exe"
        actual.write_bytes(b"public synthetic executable bytes")
        actual.chmod(0o755)
        real_fstat = eb.os.fstat
        calls = []
        def handle_stat(fd):
            observed = real_fstat(fd)
            fields = {key: getattr(observed, key) for key in (
                "st_dev", "st_ino", "st_mode", "st_size", "st_mtime_ns", "st_ctime_ns")}
            fields["st_mode"] &= ~0o111
            if calls:
                fields["st_mode"] ^= 0o200
            calls.append(fields)
            return SimpleNamespace(**fields)
        with mock.patch.object(eb, "os", SimpleNamespace(name="nt", fstat=handle_stat)):
            with self.assertRaisesRegex(eb.BundleError, "INPUT_CHANGED_DURING_READ"):
                eb.read_regular(actual, 64)
        self.assertEqual(len(calls), 2)

    def test_other_cross_api_mode_changes_are_not_normalized(self):
        actual = self.root / "preserved-permissions.exe"
        actual.write_bytes(b"public synthetic executable bytes")
        actual.chmod(0o755)
        real_fstat = eb.os.fstat
        for platform, changed_bits in (("posix", 0o111), ("nt", 0o200)):
            calls = []
            def handle_stat(fd):
                observed = real_fstat(fd)
                fields = {key: getattr(observed, key) for key in (
                    "st_dev", "st_ino", "st_mode", "st_size", "st_mtime_ns", "st_ctime_ns")}
                fields["st_mode"] ^= changed_bits
                calls.append(fields)
                return SimpleNamespace(**fields)
            with self.subTest(platform=platform), \
                    mock.patch.object(eb, "os", SimpleNamespace(name=platform, fstat=handle_stat)):
                with self.assertRaisesRegex(eb.BundleError, "INPUT_CHANGED_DURING_READ"):
                    eb.read_regular(actual, 64)
            self.assertEqual(len(calls), 1)

    def test_replaced_path_is_rejected_before_reading_its_bytes(self):
        actual = self.root / "before.json"
        changed = self.root / "replacement.json"
        actual.write_bytes(b"{}")
        changed.write_bytes(b"[]")
        real_open = Path.open
        def opened(path, *args, **kwargs):
            if path == actual and args == ("rb",):
                changed.replace(actual)
            return real_open(path, *args, **kwargs)
        with mock.patch.object(Path, "open", opened):
            with self.assertRaisesRegex(eb.BundleError, "INPUT_CHANGED_DURING_READ"):
                eb.read_regular(actual, 8)


if __name__ == "__main__":
    unittest.main()
