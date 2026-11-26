import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import types
import unittest
from unittest import mock
import zipfile

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("r22_observation_under_test", HERE / "observe_r22_native_gate.py")
observer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(observer)
REPO = Path(os.environ.get("CHERRI_R22_ORIGINAL_SOURCE_REPO", HERE.parents[1])).resolve()


class OriginalR22CaptureTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.gate = observer.load_gate(REPO)
        # Exercise the original gate's actual code on fixed synthetic children.
        # These controls run no simctl, Go, Shortcuts, Xcode or provider command.
        self.gate.sys = types.SimpleNamespace(platform="darwin")
        self.invocations = []

    def capture(self, outputs, *, result="PASS", group="ABSENT"):
        def invoke(argv, allowance, *, output, error_output, hard_deadline,
                   stream_limit, execution_deadline, journal):
            self.invocations.append((argv, allowance))
            stdout, stderr, code = outputs[len(self.invocations) - 1]
            details = {"pid": 900000 + len(self.invocations), "cleanupParentReaped": True,
                       "groupAfterCleanup": {"state": group},
                       "stdout": {"eof": True, "overflow": False, "bytes": len(stdout),
                                  "sha256": hashlib.sha256(stdout).hexdigest()},
                       "stderr": {"eof": True, "overflow": False, "bytes": len(stderr),
                                  "sha256": hashlib.sha256(stderr).hexdigest()}}
            journal("started", details)
            output.write(stdout)
            error_output.write(stderr)
            journal("finished", details)
            return {"result": result if code == 0 else "EXIT_NONZERO", "exitCode": code}, details
        return invoke

    def children(self, outputs, **options):
        return observer.OriginalGateChildren(REPO, self.root, self.capture(outputs, **options),
                                             time.monotonic() + observer.WHOLE_SECONDS)

    def healthy_outputs(self):
        return [(b"iOS 27.0 available\n", b"", 0),
                (b"five synthetic fixture observations\nIOS27_SHORTCUTS_RUNTIME_POC=PASS\n", b"", 0)]

    def test_actual_original_gate_invokes_each_child_once(self):
        children = self.children(self.healthy_outputs())
        dependency = self.gate.subprocess
        row = observer.invoke_original(self.gate, children)
        self.assertIs(self.gate.subprocess, dependency)
        observer.verify_gate_row(row, children, self.root)
        self.assertEqual(self.invocations, [(observer.SIMCTL, 15), (observer.NATIVE, 3600)])
        self.assertEqual(row["status"], "PASS")
        self.assertEqual(sum(r["spawned"] for r in children.rows), 2)
        for name in ("01-platform", "02-native-poc"):
            process = json.loads((self.root / (name + ".process.original.json")).read_bytes())
            self.assertTrue(process["spawned"])
            self.assertIsInstance(process["pid"], int)
            self.assertTrue((self.root / (name + ".stdout.original")).exists())
            self.assertTrue((self.root / (name + ".stderr.original")).exists())

    def test_original_platform_block_never_launches_native(self):
        children = self.children([(b"iOS 26.7 available\n", b"", 0)])
        row = observer.invoke_original(self.gate, children)
        self.assertEqual(row["status"], "BLOCKED_PLATFORM")
        self.assertEqual(self.invocations, [(observer.SIMCTL, 15)])
        with self.assertRaisesRegex(observer.Rejected, "ORIGINAL_GATE_NOT_PASS"):
            observer.verify_gate_row(row, children, self.root)

    def test_original_signing_block_stays_blocked(self):
        outputs = self.healthy_outputs()
        outputs[1] = (b"IOS27_SHORTCUTS_RUNTIME_POC=BLOCKED_SIGNING\n", b"", 2)
        children = self.children(outputs)
        row = observer.invoke_original(self.gate, children)
        self.assertEqual((row["status"], row["exitCode"]), ("BLOCKED_SIGNING", 2))
        with self.assertRaisesRegex(observer.Rejected, "ORIGINAL_GATE_NOT_PASS"):
            observer.verify_gate_row(row, children, self.root)
        self.assertEqual((self.root / "02-native-poc.stdout.original").read_bytes(), outputs[1][0])

    def test_zero_exit_without_native_marker_remains_failure(self):
        outputs = self.healthy_outputs()
        outputs[1] = (b"no runtime result\n", b"", 0)
        children = self.children(outputs)
        row = observer.invoke_original(self.gate, children)
        self.assertEqual(row["status"], "FAIL")
        with self.assertRaises(observer.Rejected):
            observer.verify_gate_row(row, children, self.root)

    def test_marker_cannot_override_nonzero_child(self):
        outputs = self.healthy_outputs()
        outputs[1] = (outputs[1][0], b"fixture failed\n", 1)
        children = self.children(outputs)
        row = observer.invoke_original(self.gate, children)
        self.assertEqual(row["status"], "FAIL")
        with self.assertRaises(observer.Rejected):
            observer.verify_gate_row(row, children, self.root)

    def test_exact_decoded_gate_hash_and_original_bytes_are_distinct(self):
        outputs = self.healthy_outputs()
        outputs[1] = (b"\xffIOS27_SHORTCUTS_RUNTIME_POC=PASS\n", b"\xfe", 0)
        children = self.children(outputs)
        row = observer.invoke_original(self.gate, children)
        observer.verify_gate_row(row, children, self.root)
        self.assertEqual((self.root / "02-native-poc.stdout.original").read_bytes(), outputs[1][0])
        self.assertEqual(row["outputSha256"], hashlib.sha256(
            (outputs[1][0].decode("utf-8", "replace") + outputs[1][1].decode("utf-8", "replace")).encode()).hexdigest())

    def test_owned_group_survival_cannot_authenticate_original_pass(self):
        children = self.children(self.healthy_outputs(), group="PRESENT")
        row = observer.invoke_original(self.gate, children)
        self.assertEqual(row["status"], "PASS")
        with self.assertRaisesRegex(observer.Rejected, "CHILD_PROCESS_NOT_COMPLETE"):
            observer.verify_gate_row(row, children, self.root)

    def test_changed_output_cannot_authenticate_gate_hash(self):
        children = self.children(self.healthy_outputs())
        row = observer.invoke_original(self.gate, children)
        (self.root / "02-native-poc.stdout.original").write_bytes(b"different\n")
        with self.assertRaisesRegex(observer.Rejected, "CHILD_ORIGINAL_HASH_CHANGED"):
            observer.verify_gate_row(row, children, self.root)

    def test_equal_decoded_text_cannot_hide_changed_original_bytes(self):
        outputs = self.healthy_outputs()
        outputs[1] = (b"\xffIOS27_SHORTCUTS_RUNTIME_POC=PASS\n", b"", 0)
        children = self.children(outputs)
        row = observer.invoke_original(self.gate, children)
        (self.root / "02-native-poc.stdout.original").write_bytes(outputs[1][0].replace(b"\xff", b"\xfe"))
        with self.assertRaisesRegex(observer.Rejected, "CHILD_ORIGINAL_HASH_CHANGED"):
            observer.verify_gate_row(row, children, self.root)

    def test_closed_router_rejects_arbitrary_command_before_spawn(self):
        children = self.children(self.healthy_outputs())
        with self.assertRaisesRegex(observer.Rejected, "GATE_CHILD_ROUTE"):
            children.run(["bash", "different.sh"])
        self.assertEqual(self.invocations, [])

    def test_changed_original_child_options_reject_before_spawn(self):
        children = self.children(self.healthy_outputs())
        with self.assertRaisesRegex(observer.Rejected, "ORIGINAL_CHILD_OPTIONS"):
            children.run(observer.SIMCTL, cwd=REPO, capture_output=True,
                         text=False, encoding="utf-8", errors="replace")
        self.assertEqual(self.invocations, [])

    def test_duplicate_invocation_is_rejected(self):
        children = self.children(self.healthy_outputs())
        observer.invoke_original(self.gate, children)
        with self.assertRaisesRegex(observer.Rejected, "GATE_CHILD_ROUTE"):
            children.run(observer.NATIVE)
        self.assertEqual(len(self.invocations), 2)

    def test_retention_collision_prevents_the_child(self):
        (self.root / "01-platform.intent.original.json").write_bytes(b"retained original")
        children = self.children(self.healthy_outputs())
        with self.assertRaises(FileExistsError):
            observer.invoke_original(self.gate, children)
        self.assertEqual(self.invocations, [])
        self.assertEqual((self.root / "01-platform.intent.original.json").read_bytes(), b"retained original")

    def test_interrupted_capture_is_originally_retained_and_not_retried(self):
        def stopped(argv, allowance, **kwargs):
            self.invocations.append(argv)
            kwargs["journal"]("started", {"pid": 999999})
            raise observer.OwnerInterrupted()
        children = observer.OriginalGateChildren(REPO, self.root, stopped,
                                                time.monotonic() + observer.WHOLE_SECONDS)
        dependency = self.gate.subprocess
        with self.assertRaises(observer.OwnerInterrupted):
            observer.invoke_original(self.gate, children)
        self.assertIs(self.gate.subprocess, dependency)
        process = json.loads((self.root / "01-platform.process.original.json").read_bytes())
        self.assertTrue(process["spawned"])
        self.assertEqual(process["result"], "NOT_RETURNED")
        self.assertEqual(len(self.invocations), 1)

    def test_whole_owner_budget_cannot_be_extended_for_another_child(self):
        children = self.children(self.healthy_outputs())
        children.deadline = time.monotonic() + observer.FINAL_RESERVE
        with self.assertRaisesRegex(observer.Rejected, "CHILD_BUDGET_UNAVAILABLE"):
            observer.invoke_original(self.gate, children)
        self.assertEqual(self.invocations, [])

    def test_source_pin_rejects_an_altered_original_gate(self):
        fake = self.root / "repo"
        (fake / observer.GATE_PATH).parent.mkdir(parents=True)
        (fake / observer.GATE_PATH).write_bytes(b"not the original source")
        with self.assertRaisesRegex(observer.Rejected, "ORIGINAL_GATE_OR_SCRIPT_CHANGED"):
            observer.load_gate(fake)

    def test_real_workflow_identity_is_mandatory(self):
        env = {"GITHUB_ACTIONS": "true", "GITHUB_REPOSITORY": "davidpovarsky/cherri",
               "GITHUB_WORKFLOW": "iOS 27 Shortcuts Runtime PoC", "GITHUB_SHA": "a" * 40,
               "GITHUB_RUN_ID": "12345", "GITHUB_RUN_ATTEMPT": "1"}
        self.assertEqual(observer.context_from_environment(env)["sourceCommitSha"], "a" * 40)
        for key, value in (("GITHUB_ACTIONS", "false"), ("GITHUB_SHA", "HEAD"),
                           ("GITHUB_RUN_ID", "old-run"), ("GITHUB_RUN_ATTEMPT", "0"),
                           ("GITHUB_REPOSITORY", "foreign/repository")):
            with self.subTest(field=key):
                changed = dict(env, **{key: value})
                with self.assertRaises(observer.Rejected):
                    observer.context_from_environment(changed)

    def test_private_original_archive_and_cipher_transport_are_separate(self):
        private, public = self.root / "private", self.root / "public"
        private.mkdir()
        public.mkdir()
        original = b"SYNTHETIC PRIVATE CHILD BYTES\xff\n"
        (private / "child.stdout.original").write_bytes(original)
        calls = []
        def encrypt(source, destination, archive, binding, recipient, deadline):
            calls.append(archive)
            raw = archive.read_bytes()
            self.assertEqual(binding, {"bytes": len(raw), "sha256": observer.digest(raw)})
            with zipfile.ZipFile(archive) as retained:
                self.assertEqual(set(retained.namelist()), {"child.stdout.original", "manifest.original.json"})
                self.assertEqual(retained.read("child.stdout.original"), original)
                manifest = json.loads(retained.read("manifest.original.json"))
                self.assertEqual(manifest["sourceCommitSha"], "a" * 40)
                self.assertEqual(manifest["members"], [{"name": "child.stdout.original",
                    "bytes": len(original), "sha256": observer.digest(original)}])
            # This is an explicit synthetic transport, not an encryption qualification.
            cipher = b"SYNTHETIC-CIPHER-TRANSPORT"
            (destination / "project-prerequisite.zip.p7m").write_bytes(cipher)
            return {"bytes": len(cipher), "sha256": observer.digest(cipher)}
        result = observer.seal_originals(private, public, types.SimpleNamespace(encrypt_archive=encrypt),
            self.root / "synthetic-recipient", {"sourceCommitSha": "a" * 40}, time.monotonic() + 60)
        self.assertEqual(len(calls), 1)
        self.assertEqual(result["members"], 1)
        self.assertEqual({p.name for p in public.iterdir()}, {"r22-originals.zip.p7m"})
        self.assertNotIn(original, (public / "r22-originals.zip.p7m").read_bytes())
        self.assertEqual((private / "child.stdout.original").read_bytes(), original)

    def test_archive_budget_failure_does_not_invoke_transport(self):
        private, public = self.root / "private", self.root / "public"
        private.mkdir()
        public.mkdir()
        (private / "child.stdout.original").write_bytes(b"retained")
        calls = []
        module = types.SimpleNamespace(encrypt_archive=lambda *args: calls.append(args))
        with self.assertRaisesRegex(observer.Rejected, "SEAL_BUDGET_UNAVAILABLE"):
            observer.seal_originals(private, public, module, self.root / "recipient", {}, time.monotonic())
        self.assertEqual(calls, [])
        self.assertEqual(list(public.iterdir()), [])

    def run_synthetic_main(self, *, source_failure=False, encryption_failure=False):
        private, public = self.root / "main-private", self.root / "main-public"
        terminal = b"IOS27_SHORTCUTS_RUNTIME_POC=PASS (All 5 fixtures verified pass)\n"
        outputs = [(b"iOS 27.0 available\n", b"", 0),
                   (b"PRIVATE_SYNTHETIC_STDOUT\xff\n" + terminal,
                    b"PRIVATE_SYNTHETIC_STDERR\xfe\n", 0)]
        def encrypt(source, destination, archive, binding, recipient, deadline):
            if encryption_failure:
                raise observer.Rejected("SYNTHETIC_ENCRYPTION_FAILURE")
            # The production ZIP/readback path runs; this is an explicit fake cipher.
            raw = b"SYNTHETIC-CIPHER-TRANSPORT"
            (destination / "project-prerequisite.zip.p7m").write_bytes(raw)
            return {"bytes": len(raw), "sha256": observer.digest(raw)}
        def snapshot(repo, private, capture, context, phase, deadline):
            return {"syntheticSourceControl": "changed" if source_failure and phase == "after" else "same"}
        recipient = REPO / observer.QA / "xcode_environment_inventory_recipient.pem"
        capture = types.SimpleNamespace(STREAM_LIMIT=observer.STREAM_LIMIT,
            ARCHIVE_LIMIT=observer.ARCHIVE_LIMIT, RECIPIENT_SHA256=observer.digest(recipient.read_bytes()),
            capture=self.capture(outputs), encrypt_archive=encrypt)
        env = {"GITHUB_ACTIONS": "true", "GITHUB_REPOSITORY": "davidpovarsky/cherri",
               "GITHUB_WORKFLOW": "iOS 27 Shortcuts Runtime PoC", "GITHUB_SHA": "a" * 40,
               "GITHUB_RUN_ID": "12345", "GITHUB_RUN_ATTEMPT": "1"}
        sink = io.BytesIO()
        stdout = io.TextIOWrapper(sink, encoding="utf-8", write_through=True)
        with mock.patch.dict(os.environ, env), \
             mock.patch.dict(sys.modules, {"project_prerequisite_observation": capture}), \
             mock.patch.object(observer.sys, "platform", "darwin"), \
             mock.patch.object(observer.sys, "argv", ["observe_r22_native_gate.py", "--repo", str(REPO),
                                                     "--private", str(private), "--public", str(public)]), \
             mock.patch.object(observer.sys, "stdout", stdout), \
             mock.patch.object(observer, "snapshot", snapshot), \
             mock.patch.object(observer, "load_gate", return_value=self.gate):
            code = observer.main()
        raw = sink.getvalue()
        stdout.detach()
        return code, raw, private, public, outputs

    def test_actual_main_retains_original_consumer_log_contract(self):
        consumer = REPO / "tools/language-acceptance/final_gate_native.go"
        source = consumer.read_bytes()
        self.assertEqual(observer.digest(source),
                         "56b47354a43140124a39737f0691b9cda5fcf572d9f57b6f3780f5ec09a53a0c")
        self.assertIn(b'native.RuntimeStep, "Run iOS 27 Shortcuts Runtime PoC")', source)
        self.assertIn(b'marker := "IOS27_SHORTCUTS_RUNTIME_POC=PASS (All 5 fixtures verified pass)"', source)
        self.assertIn(b'count != 1 || !bytes.Contains(log, []byte("scripts/ios27_runtime_poc.sh"))', source)
        code, raw, private, public, outputs = self.run_synthetic_main()
        self.assertEqual(code, 0)
        # These are exactly the original Go consumer's two log predicates.
        marker = b"IOS27_SHORTCUTS_RUNTIME_POC=PASS (All 5 fixtures verified pass)"
        self.assertEqual(raw.split(b"\n").count(marker), 1)
        self.assertIn(b"scripts/ios27_runtime_poc.sh", raw)
        self.assertNotIn(b"PRIVATE_SYNTHETIC", raw)
        lines = raw.splitlines()
        self.assertEqual(len(lines), 3)
        command = json.loads(lines[0].removeprefix(b"R22_NATIVE_PROCESS_ORIGINAL="))
        process = (private / "02-native-poc.process.original.json").read_bytes()
        self.assertEqual(command["argv"], json.loads(process)["argv"])
        self.assertEqual(command["pid"], json.loads(process)["pid"])
        self.assertEqual(command["processSHA256"], observer.digest(process))
        self.assertEqual(command["stdoutSHA256"], observer.digest(outputs[1][0]))
        self.assertEqual(command["stderrSHA256"], observer.digest(outputs[1][1]))
        self.assertEqual((private / "02-native-poc.stdout.original").read_bytes(), outputs[1][0])
        with zipfile.ZipFile(private / "r22-originals.zip") as archive:
            self.assertEqual(archive.read("native-public-projection.original"),
                             (private / "native-public-projection.original").read_bytes())
        self.assertEqual({p.name for p in public.iterdir()}, {"summary.json", "r22-originals.zip.p7m"})
        self.assertEqual(self.invocations, [(observer.SIMCTL, 15), (observer.NATIVE, 3600)])

    def test_public_projection_rejects_inexact_or_duplicate_terminal(self):
        marker = b"IOS27_SHORTCUTS_RUNTIME_POC=PASS (All 5 fixtures verified pass)\n"
        cases = {"prefix_only": (b"IOS27_SHORTCUTS_RUNTIME_POC=PASS\n", b""),
                 "duplicate": (marker + marker, b""), "stderr_only": (b"", marker),
                 "both_streams": (marker, marker), "crlf": (marker[:-1] + b"\r\n", b""),
                 "unterminated": (marker[:-1], b""), "padded": (b" " + marker, b""),
                 "embedded": (b"prefix:" + marker, b""), "cr_only_boundary": (b"prefix\r" + marker, b"")}
        for name, (out, err) in cases.items():
            with self.subTest(boundary=name):
                private = self.root / name
                private.mkdir()
                self.invocations = []
                children = observer.OriginalGateChildren(REPO, private,
                    self.capture([(b"iOS 27.0 available\n", b"", 0), (out, err, 0)]),
                    time.monotonic() + observer.WHOLE_SECONDS)
                row = observer.invoke_original(self.gate, children)
                self.assertEqual(row["status"], "PASS")  # Original prefix gate alone is insufficient.
                with self.assertRaisesRegex(observer.Rejected, "PUBLIC_NATIVE_TERMINAL_NOT_UNIQUE"):
                    observer.native_public_projection(row, children, private)

    def test_public_projection_binds_retained_actual_process(self):
        marker = b"IOS27_SHORTCUTS_RUNTIME_POC=PASS (All 5 fixtures verified pass)\n"
        for name in ("argv", "unspawned", "pid", "capture_pid", "process_bytes", "stream_bytes"):
            with self.subTest(boundary=name):
                private = self.root / name
                private.mkdir()
                self.invocations = []
                children = observer.OriginalGateChildren(REPO, private,
                    self.capture([(b"iOS 27.0 available\n", b"", 0), (marker, b"", 0)]),
                    time.monotonic() + observer.WHOLE_SECONDS)
                row = observer.invoke_original(self.gate, children)
                child = children.rows[1]
                if name == "argv":
                    child["argv"] = ["bash", "unrelated.sh"]
                elif name == "unspawned":
                    child["spawned"] = False
                elif name == "pid":
                    child["pid"] = 0
                elif name == "capture_pid":
                    child["capture"]["pid"] += 1
                elif name == "process_bytes":
                    (private / "02-native-poc.process.original.json").write_bytes(b"{}\n")
                else:
                    (private / "02-native-poc.stdout.original").write_bytes(marker + b"changed\n")
                with self.assertRaises(observer.Rejected):
                    observer.native_public_projection(row, children, private)

    def test_actual_main_source_or_encryption_failure_cannot_publish_terminal(self):
        original_root = self.root
        for name in ("source_failure", "encryption_failure"):
            with self.subTest(boundary=name):
                self.root = original_root / name
                self.root.mkdir()
                self.invocations = []
                try:
                    code, raw, private, public, outputs = self.run_synthetic_main(**{name: True})
                finally:
                    self.root = original_root
                self.assertEqual(code, 1)
                self.assertNotIn(b"IOS27_SHORTCUTS_RUNTIME_POC=PASS", raw)
                self.assertNotIn(b"R22_NATIVE_PROCESS_ORIGINAL", raw)
                self.assertNotIn(b"PRIVATE_SYNTHETIC", raw)
                self.assertEqual(json.loads(raw)["status"], "FAILED")
                self.assertEqual((private / "02-native-poc.stdout.original").read_bytes(), outputs[1][0])

    def test_final_public_projection_requires_all_observation_guards(self):
        healthy = {"status": "PASS", "sourceUnchanged": True, "privateOriginalsEncrypted": True,
                   "gateInvocations": 1, "nativeScriptInvocations": 1,
                   "originalGateStatus": "PASS", "errorCategories": []}
        cases = {"sourceUnchanged": False, "privateOriginalsEncrypted": False, "gateInvocations": 2,
                 "nativeScriptInvocations": 2, "originalGateStatus": "FAIL", "errorCategories": ["error"]}
        for field, value in cases.items():
            with self.subTest(boundary=field):
                sink = io.BytesIO()
                with self.assertRaisesRegex(observer.Rejected, "PUBLIC_NATIVE_FINAL_GUARDS"):
                    observer.emit_public_result(dict(healthy, **{field: value}), b"synthetic", sink)
                self.assertEqual(sink.getvalue(), b"")
        with self.subTest(boundary="missing_projection"):
            sink = io.BytesIO()
            with self.assertRaisesRegex(observer.Rejected, "PUBLIC_NATIVE_FINAL_GUARDS"):
                observer.emit_public_result(healthy, None, sink)
            self.assertEqual(sink.getvalue(), b"")

    def test_workflow_preserves_every_original_step_and_publication_rule(self):
        path = HERE.parents[1] / ".github/workflows/ios27-shortcuts-runtime-poc.yml"
        current = path.read_text()
        replacement = '''      - name: Run iOS 27 Shortcuts Runtime PoC
        id: r22_gate
        env:
          CHERRI_ALLOW_EXTERNAL_SIGNING: ${{ inputs.allow_public_fixture_hubsign == true && '1' || '0' }}
        run: python3 -B tools/p10r3/observe_r22_native_gate.py --repo . --private "$RUNNER_TEMP/cherri-r22-originals" --public r22-public
'''
        original = '''      - name: Run iOS 27 Shortcuts Runtime PoC
        env:
          CHERRI_ALLOW_EXTERNAL_SIGNING: ${{ inputs.allow_public_fixture_hubsign == true && '1' || '0' }}
        run: bash scripts/ios27_runtime_poc.sh
'''
        control = '''      - name: Verify original R22 observation controls
        run: python3 -B -m unittest discover -s tools/p10r3 -p 'test_observe_r22_native_gate.py' -v

'''
        publication = '''      - name: Upload encrypted original R22 evidence
        if: always() && steps.r22_gate.outcome != 'skipped'
        uses: actions/upload-artifact@v4
        with:
          name: original-r22-evidence
          path: |
            r22-public/summary.json
            r22-public/r22-originals.zip.p7m
          retention-days: 7
          if-no-files-found: error

'''
        for part in (replacement, control, publication):
            self.assertEqual(current.count(part), 1)
        reconstructed = current.replace(replacement, original).replace(control, "").replace(publication, "")
        self.assertEqual(hashlib.sha256(reconstructed.encode()).hexdigest(),
                         "6770bf59df007d66733c8c860b72ffab924d655d911723cad202934fded67f8f")


if __name__ == "__main__":
    unittest.main()
