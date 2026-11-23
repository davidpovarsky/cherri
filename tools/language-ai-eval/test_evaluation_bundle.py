"""Offline provenance controls; no network, model request or compiler call."""
import copy
from contextlib import contextmanager
import json
from pathlib import Path
import stat
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
        eb._PUBLIC_KEY_MEMO.clear(); eb._SIGNATURE_MEMO.clear()
        self.addCleanup(eb._PUBLIC_KEY_MEMO.clear); self.addCleanup(eb._SIGNATURE_MEMO.clear)
        key = self.root / "key.pem"
        public = self.root / "public.pem"
        manifest = self.root / "manifest.json"
        signature = self.root / "signature-real.bin"
        eb.crypto("genpkey", "-algorithm", "ED25519", "-out", key)
        eb.crypto("pkey", "-in", key, "-pubout", "-out", public)
        raw = eb.canonical({"synthetic_control": True, "caps": [12000, 2000]})
        manifest.write_bytes(raw)
        eb.crypto("pkeyutl", "-sign", "-rawin", "-inkey", key, "-in", manifest, "-out", signature)
        eb.crypto("genpkey", "-algorithm", "ED25519", "-out", self.root / "other-key.pem")
        eb.crypto("pkey", "-in", self.root / "other-key.pem", "-pubout", "-out", self.root / "other-public.pem")
        eb.crypto("genpkey", "-algorithm", "X25519", "-out", self.root / "wrong-algorithm.pem")
        eb.crypto("pkey", "-in", self.root / "wrong-algorithm.pem", "-pubout", "-out", self.root / "wrong-algorithm-public.pem")
        public_raw, signature_raw = public.read_bytes(), signature.read_bytes()
        actual_counts = {}
        # Real OpenSSL integration: no cryptographic result is mocked. Wrapping
        # counts the actual subprocess path while every rejection still runs it.
        with mock.patch.object(eb, "crypto", wraps=eb.crypto) as calls:
            eb.verify_signature(raw, public_raw, signature_raw)
            self.assertEqual(calls.call_count, 2)
            before = calls.call_count
            eb.verify_signature(bytes(bytearray(raw)), bytes(bytearray(public_raw)), bytes(bytearray(signature_raw)))
            actual_counts["exact_repeat"] = calls.call_count - before
            self.assertEqual(actual_counts["exact_repeat"], 0)
            changed = raw.replace(b"12000", b"12001")
            for name, data, pub, sig, expected_calls, cause in (
                ("message", changed, public_raw, signature_raw, 1, "SIGNATURE_VERIFICATION_FAILED"),
                ("same_failed_message", changed, public_raw, signature_raw, 1, "SIGNATURE_VERIFICATION_FAILED"),
                ("signature", raw, public_raw, bytes(64), 1, "SIGNATURE_VERIFICATION_FAILED"),
                ("key", raw, (self.root / "other-public.pem").read_bytes(), signature_raw, 2, "SIGNATURE_VERIFICATION_FAILED"),
                ("algorithm", raw, (self.root / "wrong-algorithm-public.pem").read_bytes(), signature_raw, 1, "SIGNATURE_ALGORITHM"),
                ("same_failed_algorithm", raw, (self.root / "wrong-algorithm-public.pem").read_bytes(), signature_raw, 1, "SIGNATURE_ALGORITHM"),
            ):
                with self.subTest(crypto_input=name):
                    before = calls.call_count
                    with self.assertRaisesRegex(eb.BundleError, cause):
                        eb.verify_signature(data, pub, sig)
                    actual_counts[name] = calls.call_count - before
                    self.assertEqual(actual_counts[name], expected_calls)
            # Same mathematical key, different complete PEM bytes: both actual
            # algorithm inspection and signature verification are required.
            alternate_public = public_raw.replace(b"\n", b"\r\n")
            before = calls.call_count
            eb.verify_signature(raw, alternate_public, signature_raw)
            actual_counts["changed_key_bytes"] = calls.call_count - before
            self.assertEqual(actual_counts["changed_key_bytes"], 2)
            with self.assertRaisesRegex(eb.BundleError, "SIGNATURE_BYTES_TYPE"):
                eb.verify_signature(bytearray(raw), public_raw, signature_raw)
        # An entry larger than the fixed production memo cap still receives
        # real verification on every call; it is not rejected or remembered.
        oversized = b"PUBLIC_SYNTHETIC_OVERSIZED_MESSAGE:" + b"x" * (64 * 1024)
        manifest.write_bytes(oversized)
        eb.crypto("pkeyutl", "-sign", "-rawin", "-inkey", key, "-in", manifest, "-out", signature)
        big_signature = signature.read_bytes()
        with mock.patch.object(eb, "crypto", wraps=eb.crypto) as calls:
            eb.verify_signature(oversized, public_raw, big_signature)
            eb.verify_signature(oversized, public_raw, big_signature)
            actual_counts["oversized_two_actual_verifications"] = calls.call_count
            self.assertEqual(calls.call_count, 2)
            self.assertFalse(eb._SIGNATURE_MEMO.contains((oversized, public_raw, big_signature)))
        print(json.dumps({"real_openssl_memo_controls": actual_counts}, sort_keys=True))
        if hasattr(eb.os, "fork") and hasattr(eb.os, "register_at_fork"):
            import os
            import select
            import signal
            import time
            parent_counts = [len(eb._PUBLIC_KEY_MEMO.entries), len(eb._SIGNATURE_MEMO.entries)]
            self.assertTrue(all(parent_counts))
            reader, writer = os.pipe()
            child = os.fork()
            if child == 0:
                os.close(reader)
                observed = {"entries": [len(eb._PUBLIC_KEY_MEMO.entries), len(eb._SIGNATURE_MEMO.entries)],
                    "bytes": [eb._PUBLIC_KEY_MEMO.byte_count, eb._SIGNATURE_MEMO.byte_count],
                    "owned_by_child": all(m.owner_pid == os.getpid() for m in (eb._PUBLIC_KEY_MEMO, eb._SIGNATURE_MEMO))}
                os.write(writer, json.dumps(observed).encode())
                os.close(writer); os._exit(0)
            os.close(writer)
            reaped = False
            try:
                self.assertTrue(select.select([reader], [], [], 3)[0], "fork control did not return its receipt")
                observed = json.loads(os.read(reader, 4096))
                deadline = time.monotonic() + 3
                while time.monotonic() < deadline:
                    pid, status = os.waitpid(child, os.WNOHANG)
                    if pid == child:
                        reaped = True; self.assertEqual(os.waitstatus_to_exitcode(status), 0); break
                    time.sleep(0.01)
                self.assertTrue(reaped, "fork control did not exit within its bound")
                self.assertEqual(observed, {"entries": [0, 0], "bytes": [0, 0], "owned_by_child": True})
                self.assertEqual([len(eb._PUBLIC_KEY_MEMO.entries), len(eb._SIGNATURE_MEMO.entries)], parent_counts)
                print(json.dumps({"actual_posix_fork_empty_memos": observed}, sort_keys=True))
            finally:
                os.close(reader)
                if not reaped:
                    os.kill(child, signal.SIGKILL); os.waitpid(child, 0)
        # Bookkeeping controls below are separate from the real crypto proof.
        # They exercise simultaneous byte/entry bounds, eviction, duplicate
        # insertion, oversized bypass and a new process's empty fact set.
        memo = eb._SuccessMemo(2, 6, 4)
        memo.remember_success((b"aa",)); memo.remember_success((b"bbb",))
        self.assertEqual((len(memo.entries), memo.byte_count), (2, 5))
        memo.remember_success((b"cccc",))
        self.assertEqual((len(memo.entries), memo.byte_count), (1, 4))
        self.assertFalse(memo.contains((b"aa",))); self.assertFalse(memo.contains((b"bbb",)))
        memo.remember_success((b"cccc",))
        self.assertEqual((len(memo.entries), memo.byte_count), (1, 4))
        memo.remember_success((b"oversized",))
        self.assertEqual((len(memo.entries), memo.byte_count), (1, 4))
        memo.clear()
        for value in (b"a", b"b", b"c"):
            memo.remember_success((value,))
        self.assertEqual((len(memo.entries), memo.byte_count), (2, 2))
        self.assertFalse(memo.contains((b"a",)))
        with mock.patch.object(eb.os, "getpid", return_value=memo.owner_pid + 1):
            self.assertFalse(memo.contains((b"c",)))
            self.assertEqual((len(memo.entries), memo.byte_count), (0, 0))

    def test_duplicate_json_and_nonfinite_values_are_not_reconciled(self):
        for raw in ('{"model":"a","model":"b"}', '{"tokens":NaN}', '{"tokens":Infinity}'):
            with self.assertRaises(eb.BundleError):
                eb.unique_json(raw)

    def test_symlink_input_cannot_supply_an_original(self):
        actual = self.root / "real.json"
        actual.write_text("{}")
        original_lstat = Path.lstat
        visits = {}
        def counted(path):
            visits[path] = visits.get(path, 0) + 1
            return original_lstat(path)
        # Each phase observes every path anew, with one no-follow stat serving
        # both link predicates. No second pathname query supplies its tag.
        with mock.patch.object(Path, "lstat", counted), \
                mock.patch.object(Path, "is_symlink", side_effect=AssertionError("duplicate path query")), \
                mock.patch.object(Path, "is_junction", side_effect=AssertionError("duplicate path query"), create=True):
            self.assertEqual(eb.read_regular(actual, 8), b"{}")
            self.assertEqual(visits, {path: 2 for path in (actual, *actual.parents)})
            self.assertEqual(eb.read_regular(actual, 8), b"{}")
            self.assertEqual(visits, {path: 4 for path in (actual, *actual.parents)})
        # Model the exact stat fields returned for a Windows junction, even on
        # POSIX. Every late mutation is applied only to the fresh second scan.
        for location, kind, occurrence, error in (
            (actual, "junction", 1, "INPUT_NOT_REGULAR"),
            (actual, "junction", 2, "INPUT_PARENT_SYMLINK"),
            (self.root, "junction", 1, "INPUT_PARENT_SYMLINK"),
            (self.root, "junction", 2, "INPUT_PARENT_SYMLINK"),
            (self.root, "symlink", 1, "INPUT_PARENT_SYMLINK"),
            (self.root, "symlink", 2, "INPUT_PARENT_SYMLINK"),
        ):
            seen = {}
            def changed(path):
                observed = original_lstat(path)
                seen[path] = seen.get(path, 0) + 1
                if path == location and seen[path] == occurrence:
                    fields = {key: getattr(observed, key) for key in (
                        "st_dev", "st_ino", "st_mode", "st_size", "st_mtime_ns", "st_ctime_ns")}
                    if kind == "junction":
                        fields["st_reparse_tag"] = 0xA0000003
                    else:
                        fields["st_mode"] = stat.S_IFLNK | stat.S_IMODE(observed.st_mode)
                    return SimpleNamespace(**fields)
                return observed
            with self.subTest(location="file" if location == actual else "parent", kind=kind, phase=occurrence), \
                    mock.patch.object(Path, "lstat", changed):
                with self.assertRaisesRegex(eb.BundleError, error):
                    eb.read_regular(actual, 8)
            self.assertEqual(seen[location], occurrence)
        link = self.root / "link.json"
        try:
            link.symlink_to(actual)
        except OSError:
            self.skipTest("Host does not permit symlink creation")
        with self.assertRaisesRegex(eb.BundleError, "INPUT_NOT_REGULAR"):
            eb.read_regular(link)
        physical_parent = self.root / "physical-parent"
        physical_parent.mkdir()
        (physical_parent / "original.json").write_bytes(b"{}")
        alias_parent = self.root / "alias-parent"
        alias_parent.symlink_to(physical_parent, target_is_directory=True)
        # This real ancestor alias must fail even immediately after a valid
        # physical-path read; success on one path cannot be reused for another.
        self.assertEqual(eb.read_regular(physical_parent / "original.json", 8), b"{}")
        with self.assertRaisesRegex(eb.BundleError, "INPUT_PARENT_SYMLINK"):
            eb.read_regular(alias_parent / "original.json", 8)

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
                fields["st_mode"] = actual.lstat().st_mode ^ changed_bits
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

    @contextmanager
    def prepared_profile_control(self, profile):
        # Public deterministic unit material: no fresh seed or compiler executes.
        repository = Path(eb.__file__).resolve().parents[2]
        version = profile.rsplit("/", 1)[1]
        output = self.root / ("offline-profile-" + version)
        compiler = self.root / ("nonexecutable-compiler-control-" + version)
        compiler.write_bytes(b"synthetic compiler control; never execute\n")
        compiler_hash = eb.digest(compiler.read_bytes())
        model = {"id": eb.MODEL, "created_at": "2026-09-28T00:00:00Z", "lifecycle": "active"}
        model_raw = eb.canonical(model)
        response = self.root / ("synthetic-model-response-" + version)
        response.write_bytes(model_raw)
        receipt = self.root / ("synthetic-model-receipt-" + version)
        receipt.write_bytes(eb.canonical({
            "httpStatus": 200, "operation": "PROVIDER_MODEL_IDENTITY_ONLY",
            "requestedModelId": eb.MODEL, "transportErrorType": None, "parseErrorType": None,
            "canonicalIdentityVerified": True, "returnedModel": model,
            "finishedAtUtc": "2026-11-21T00:00:00Z",
            "responseSha256": eb.digest(model_raw), "responseBytes": len(model_raw)}))
        def git_control(command, **kwargs):
            arguments = command[2:]
            if arguments == ["status", "--porcelain"]:
                return b""
            if arguments == ["rev-parse", "HEAD"]:
                return b"cccccccccccccccccccccccccccccccccccccccc\n"
            if len(arguments) == 2 and arguments[0] == "show":
                return (repository / arguments[1].split(":", 1)[1]).read_bytes()
            raise AssertionError("unexpected synthetic source operation")
        with mock.patch.object(eb, "COMPILER_SHA256", compiler_hash), \
             mock.patch.object(eb, "V2_COMPILER_SHA256", compiler_hash), \
             mock.patch.object(eb.secrets, "token_bytes", return_value=bytes(range(32))) as seed, \
             mock.patch.object(eb.subprocess, "check_output", side_effect=git_control):
            eb.create_bundle(output, repository, compiler, receipt, response, profile=profile)
            seed.assert_called_once_with(32)
            yield eb.load_bundle(output, require_anchor=False)

    def test_profile_v1_constants_and_fresh_instance_derivation_are_preserved(self):
        self.assertEqual(eb.PROFILE, "fresh-six-exact-instances/v1")
        self.assertEqual(eb.COMPILER_SOURCE, "b791ce0616d1821cebfc57909a6bc50a195ca68c")
        self.assertEqual(eb.COMPILER_SHA256, "50e73c811c6829448f7406a6be4731b632cf27ef038615c83bdd5c96cb3b6ef3")
        original = eb.reviewed_profile(eb.PROFILE)
        self.assertEqual(original["provider"]["settings"], {"thinking": {"type": "disabled"}, "stream": False})
        original["provider"]["settings"]["thinking"]["type"] = "mutated"
        self.assertEqual(eb.PROVIDER["settings"]["thinking"], {"type": "disabled"})
        with self.assertRaisesRegex(eb.BundleError, "BUNDLE_PROFILE"):
            eb.reviewed_profile("unreviewed-profile")

    def test_v2_unreviewed_compiler_blocks_before_source_seed_or_output_creation(self):
        output = self.root / "must-not-exist"
        with mock.patch.object(eb, "V2_COMPILER_SHA256", None), \
             mock.patch.object(eb.secrets, "token_bytes") as seed, \
             mock.patch.object(eb.subprocess, "check_output") as git, \
             mock.patch.object(eb, "crypto") as crypto:
            with self.assertRaisesRegex(eb.BundleError, "UNREVIEWED_PROFILE_COMPILER"):
                eb.create_bundle(output, self.root, self.root/"absent", self.root/"absent",
                                 self.root/"absent", profile=eb.V2_PROFILE)
            seed.assert_not_called()
            git.assert_not_called()
            crypto.assert_not_called()
        self.assertFalse(output.exists())

    def test_both_profiles_seal_and_load_exact_budgets_and_original_six_public_patterns(self):
        suites = []
        for profile in (eb.PROFILE, eb.V2_PROFILE):
            with self.subTest(profile=profile), self.prepared_profile_control(profile) as bundle:
                manifest = bundle["manifest"]
                self.assertEqual(manifest["profile"], profile)
                self.assertEqual(manifest["provider"], eb.reviewed_profile(profile)["provider"])
                self.assertEqual(manifest["budget"], {"max_input_tokens": 12000, "max_output_tokens": 2000})
                self.assertEqual(manifest["expected_outcomes"], 12)
                self.assertEqual(manifest["retry_limit"], 0)
                self.assertEqual(manifest["task_ids"], list(eb.TASK_IDS))
                self.assertEqual(len(eb.request_sequence(bundle)), 12)
                self.assertEqual(manifest["compiler"], eb.reviewed_profile(profile)["compiler"])
                suites.append(eb.canonical(bundle["suite"]))
                mode = (bundle["root"] / "compiler.exe").stat().st_mode
                if eb.os.name == "posix":
                    self.assertEqual(stat.S_IMODE(mode) if profile == eb.V2_PROFILE else mode & 0o111,
                                     0o700 if profile == eb.V2_PROFILE else 0)
        self.assertEqual(suites[0], suites[1])

    def test_validly_signed_profile_budget_compiler_and_tool_mutations_still_reject(self):
        with self.prepared_profile_control(eb.V2_PROFILE) as bundle:
            original = bundle["manifest"]
            changes = [
                ("v1 label", lambda m: m.update(profile=eb.PROFILE), "PROVIDER_SETTINGS"),
                ("v1 settings", lambda m: m.update(provider=copy.deepcopy(eb.PROVIDER)), "PROVIDER_SETTINGS"),
                ("model drift", lambda m: m["provider"].update(model="other-model"), "PROVIDER_SETTINGS"),
                ("tools", lambda m: m["provider"]["settings"].update(tools=[]), "PROVIDER_SETTINGS"),
                ("effort", lambda m: m["provider"]["settings"]["output_config"].update(effort="max"), "PROVIDER_SETTINGS"),
                ("input cap", lambda m: m["budget"].update(max_input_tokens=12001), "EQUAL_BUDGET"),
                ("output cap", lambda m: m["budget"].update(max_output_tokens=2001), "EQUAL_BUDGET"),
                ("retries", lambda m: m.update(retry_limit=1), "OUTCOME_POLICY"),
                ("outcomes", lambda m: m.update(expected_outcomes=11), "OUTCOME_POLICY"),
                ("compiler source", lambda m: m["compiler"].update(source_sha=eb.COMPILER_SOURCE), "COMPILER_IDENTITY"),
                ("compiler binary", lambda m: m["compiler"].update(binary_sha256="0"*64), "COMPILER_IDENTITY"),
                ("environment", lambda m: m.update(compiler_environment_profile="isolated-defaults/v1"), "COMPILER_ENVIRONMENT_PROFILE")]
            for name, change, error in changes:
                with self.subTest(mutation=name):
                    modified = copy.deepcopy(original)
                    change(modified)
                    (bundle["root"]/"manifest.json").write_bytes(eb.canonical(modified))
                    eb.crypto("pkeyutl", "-sign", "-rawin", "-inkey", bundle["root"]/"private"/"signing-key.pem",
                              "-in", bundle["root"]/"manifest.json", "-out", bundle["root"]/"signature.bin")
                    with self.assertRaisesRegex(eb.BundleError, error):
                        eb.load_bundle(bundle["root"], require_anchor=False)

    def test_v1_manifest_relabel_does_not_reuse_its_original_signature(self):
        with self.prepared_profile_control(eb.PROFILE) as bundle:
            modified = copy.deepcopy(bundle["manifest"])
            selected = eb.reviewed_profile(eb.V2_PROFILE)
            modified.update(profile=eb.V2_PROFILE, provider=selected["provider"],
                            compiler=selected["compiler"],
                            compiler_environment_profile=selected["compiler_environment_profile"])
            (bundle["root"]/"manifest.json").write_bytes(eb.canonical(modified))
            with self.assertRaisesRegex(eb.BundleError, "SIGNATURE_VERIFICATION_FAILED"):
                eb.load_bundle(bundle["root"], require_anchor=False)

    def test_v2_reviewed_protocol_cannot_drift_or_leave_its_sealed_inventory(self):
        name = "fresh-instance-protocol-v2.md"
        self.assertNotIn(name, eb.reviewed_profile(eb.PROFILE)["producer_files"])
        with self.prepared_profile_control(eb.V2_PROFILE) as bundle:
            self.assertIn(name, bundle["manifest"]["producer_sha256"])
            actual_read = eb.read_regular
            protocol_path = Path(eb.__file__).with_name(name)
            def changed_protocol(path, *args, **kwargs):
                raw = actual_read(path, *args, **kwargs)
                return raw + b"synthetic document drift\n" if Path(path) == protocol_path else raw
            with mock.patch.object(eb, "read_regular", side_effect=changed_protocol):
                with self.assertRaisesRegex(eb.BundleError, "PRODUCER_CHANGED"):
                    eb.load_bundle(bundle["root"], require_anchor=False)
            modified = copy.deepcopy(bundle["manifest"])
            modified["producer_sha256"].pop(name)
            (bundle["root"]/"manifest.json").write_bytes(eb.canonical(modified))
            eb.crypto("pkeyutl", "-sign", "-rawin", "-inkey", bundle["root"]/"private"/"signing-key.pem",
                      "-in", bundle["root"]/"manifest.json", "-out", bundle["root"]/"signature.bin")
            with self.assertRaisesRegex(eb.BundleError, "PRODUCER_INVENTORY"):
                eb.load_bundle(bundle["root"], require_anchor=False)




if __name__ == "__main__":
    unittest.main()
