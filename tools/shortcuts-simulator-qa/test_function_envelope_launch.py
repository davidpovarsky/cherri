"""Synthetic filesystem/process controls. No test invokes simctl or Apple Shortcuts."""
import base64
import copy
import datetime
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest import mock

import function_envelope_graph as graph
import function_envelope_launch as launch
import function_envelope_runtime as runtime
import function_envelope_observation as observation
from test_function_envelope_observation import GOOD as ORIGINAL_THREE_CASE_SUMMARY

HERE = Path(__file__).resolve().parent


class SyntheticRun:
    def __init__(self):
        self.temp = tempfile.TemporaryDirectory()
        self.base = Path(self.temp.name).resolve(strict=True)
        self.repo, self.root, self.documents = (self.base / n for n in ("repo", "artifacts", "documents"))
        for path in (self.repo, self.root, self.documents):
            path.mkdir()
        (self.root / "runtime").mkdir()
        for name in launch.SOURCE_PATHS:
            dst = self.repo / name
            dst.parent.mkdir(parents=True, exist_ok=True)
            original = HERE.parents[1] / name
            # The missing Apple policy in this independent source branch is a
            # declared synthetic witness, never a claim of a Receiver build.
            dst.write_bytes(original.read_bytes() if original.is_file()
                            else b"synthetic source-witness fixture only\n")
        self.git("init", "-q")
        self.git("add", "--all")
        self.git("-c", "user.name=Synthetic Evidence Control", "-c", "user.email=control@example.invalid",
                 "-c", "commit.gpgsign=false", "commit", "-qm", "synthetic input witnesses")
        self.commit = self.git("rev-parse", "HEAD").decode().strip()
        self.compiler = runtime.COMPILER_SOURCE
        self.nonce = "function-control-17"
        self.udid = "11111111-2222-4333-8444-555555555555"
        self.source = (HERE / "fixtures" / (launch.FIXTURE + ".cherri")).read_bytes()
        self.unsigned = (HERE / "testdata/function-envelope-presence-compiled.original.plist").read_bytes()
        self.signed = b"AEA1SYNTHETIC_CONTROL_NOT_AN_APPLE_SIGNATURE"
        for folder in ("builds", "builds/signed"):
            (self.root / folder).mkdir(exist_ok=True)
        (self.root / "builds" / (launch.WORKFLOW + ".cherri")).write_bytes(self.source)
        (self.root / "builds" / (launch.WORKFLOW + ".shortcut")).write_bytes(self.unsigned)
        (self.root / "builds/signed" / (launch.WORKFLOW + ".shortcut")).write_bytes(self.signed)
        proof = runtime.prepared(self.repo, launch.FIXTURE, self.source, self.unsigned)
        row = {"fixtureId": launch.FIXTURE, "fileName": launch.WORKFLOW + ".cherri",
               "workflowName": launch.WORKFLOW, "dialect": "v2", "buildProfile": "debug",
               "runtimeTraceExpectation": "REQUIRED_NDJSON", "sourceSha256": graph.SOURCE_SHA256,
               "unsignedPlistSha256": launch.sha(self.unsigned), "nativeActionCount": 379,
               "functionEnvelopeProbe": proof}
        self.json_write(self.root / "compiler-manifest.json",
                        {"syntheticOnly": True, "compilerCommitSha": self.compiler,
                         "harnessCommitSha": self.commit, "compilerArtifactOrigin": "retained-original-preparation",
                         "fixtures": [row]})
        (self.root / "builds/function-envelope-compiler-export.original.json").write_bytes(
            (HERE / "testdata/function-envelope-compiler-export.original.json").read_bytes())
        target = {"compilerCommitSha": self.compiler, "sourceSha256": graph.SOURCE_SHA256,
                  "unsignedPlistSha256": launch.sha(self.unsigned), "unsignedByteSize": len(self.unsigned),
                  "nativeActionCount": 379, "signedArtifactSha256": launch.sha(self.signed),
                  "signedByteSize": len(self.signed), "headerMagic": "AEA1"}
        self.json_write(self.root / "runtime/signing-provenance.json",
                        {"compilerCommitSha": self.compiler, "signedArtifactValidAEA1": True,
                         "signedArtifactSha256": launch.sha(self.signed), "signedByteSize": len(self.signed),
                         "targetProvenance": target})
        self.context = None
        self.calls = []

    def git(self, *args):
        return subprocess.check_output(["git", "-C", str(self.repo), *args],
                                       stderr=subprocess.PIPE, timeout=10)

    def close(self):
        self.temp.cleanup()

    @staticmethod
    def json_write(path, value):
        path.write_bytes(launch.canonical(value) + b"\n")

    def prepare(self):
        self.context = launch.prepare(self.repo, self.root, self.commit, self.compiler, "synthetic-owner", 1,
                                      self.nonce, self.udid)
        return self.context

    def capture(self, command, allowance, *, output, error_output, hard_deadline,
                stream_limit, journal):
        assert (self.root / launch.RELATIVE / "attempt.claim.original.json").is_file()
        assert allowance == 25 and stream_limit == 65536
        self.calls.append(command)
        start = launch.now()
        stdout, stderr = b"org.cherrilang.qa.Receiver: 4242\n", b""
        details = {"pid": 3131, "startedAt": start, "finishedAt": launch.now(),
                   "cleanupParentReaped": True, "groupPresentAfterCleanup": False,
                   "stdout": {"bytes": len(stdout), "sha256": launch.sha(stdout),
                              "eof": True, "overflow": False},
                   "stderr": {"bytes": 0, "sha256": launch.sha(stderr), "eof": True, "overflow": False}}
        journal("started", details)
        output.write(stdout)
        error_output.write(stderr)
        journal("finished", details)
        return {"result": "PASS", "exitCode": 0, "elapsedMs": 1}, details

    def execute(self, capture=None):
        with mock.patch.object(launch.sys, "platform", "darwin"), \
             mock.patch.object(launch.processes, "capture", capture or self.capture):
            return launch.execute(self.repo, self.root)

    def triggers(self):
        target, context, raw, contract, _ = launch.load_prepared(self.root)
        directory = self.documents / "primary-no-input-launch" / launch.sha(raw)
        directory.mkdir(parents=True)
        launch.write_new(directory / "contract.original.json", raw)
        launch.record(directory / "owner.original.json", {
            "schema": "cherri-function-envelope-trigger-owner/1", "processID": 4242,
            "contractByteCount": len(raw), "contractSHA256": launch.sha(raw),
            "contractOriginalRetained": True})
        primary = contract["primaryURL"].encode("utf-8")
        stamp = datetime.datetime.now(datetime.timezone.utc).replace(microsecond=0).isoformat()
        for sequence, event in ((1, "intent"), (2, "completion")):
            row = {"schema": "cherri-function-envelope-trigger-receipt/1", "mode": launch.MODE,
                   "sequence": sequence, "event": event, "attemptOrdinal": 1, "processID": 4242,
                   "timestampISO8601": stamp, "contractSHA256": launch.sha(raw),
                   "contractByteCount": len(raw), "source": "env", "attemptConsumed": True,
                   "requestedURLUTF8Bytes": len(primary), "requestedURLSHA256": launch.sha(primary),
                   "requestedURLOriginalRetained": True, "contractOriginalRetained": True,
                   "retentionFailure": False, "requestedURL": contract["primaryURL"],
                   "requestedURLUTF8Base64": base64.b64encode(primary).decode(),
                   "fixtureId": launch.FIXTURE, "workflowName": launch.WORKFLOW,
                   "runNonce": self.nonce, "primaryURL": contract["primaryURL"],
                   "primaryURLUTF8Base64": base64.b64encode(primary).decode(),
                   "primaryURLUTF8Bytes": len(primary), "primaryURLSHA256": launch.sha(primary),
                   "invocationToken": "12345678-1234-4234-8234-1234567890AB"}
            if event == "completion":
                row["success"] = True
            launch.record(directory / ("%03d-%s.original.json" % (sequence, event)), row)
        launch.collect_receiver(self.root, self.documents)

    def refresh_trigger_inventory(self):
        p = self.root / "runtime/function-envelope-trigger-acquisition.original.json"
        value = json.loads(p.read_bytes())
        directory = self.root / launch.TRIGGERS
        value["files"] = [{"path": q.name, "bytes": len(q.read_bytes()),
                           "sha256": launch.sha(q.read_bytes())} for q in sorted(directory.iterdir())]
        self.json_write(p, value)

    def callback(self):
        records = []
        for stage in ("collector_begin", "before", "after", "collector_end"):
            if stage.startswith("collector"):
                row = {"schemaVersion": "1.4", "kind": "COLLECTOR_EVENT", "stage": stage,
                       "runId": launch.FIXTURE, "traceTransport": "ndjson-stream",
                       "collectorSentinel": "===CHERRI_QA_TRACE_NDJSON_V14==="}
            else:
                sequence, action = (360, "setvariable") if stage == "before" else (370, "runworkflow")
                row = {"schemaVersion": "1.4", "kind": "INLINE_DEBUG_ACTION", "stage": stage,
                       "label": "function-envelope-presence", "stepSequence": sequence,
                       "actionIdentifier": "is.workflow.actions." + action,
                       "timestampPrecision": "apple-currentdate", "valueType": "UNKNOWN_AT_TRACE_EVENT",
                       "timestamp": "synthetic native timestamp control"}
            records.append(row)
        event = {"runId": self.nonce, "receiverSelfTest": False, "eventSource": "shortcuts-x-callback",
                 "fixtureId": launch.FIXTURE, "expectedShortcutName": launch.WORKFLOW, "status": "SUCCESS",
                 "resultSummary": ORIGINAL_THREE_CASE_SUMMARY,
                 "tracePayload": "\n".join(launch.canonical(v).decode() for v in records)}
        self.json_write(self.root / "runtime/callback-events.json", [event])

    def ready(self):
        self.prepare()
        self.execute()
        self.triggers()
        self.callback()

    def evaluate(self):
        return runtime.evaluate(self.repo, self.root, launch.FIXTURE, self.compiler, self.commit, self.nonce, "")


class FunctionNoInputContractTests(unittest.TestCase):
    def test_exact_six_keys_and_no_input_callbacks(self):
        for nonce in ("A", "a-Z_19", "x" * 128):
            with self.subTest(nonce=nonce):
                raw = launch.make_contract(nonce)
                value = launch.validate_contract(raw)
                self.assertEqual(value["runNonce"], nonce)
                self.assertEqual(raw, launch.canonical(value))
                self.assertNotIn(b"input=", raw)
                self.assertNotIn(b"text=", raw)
                self.assertFalse(raw.endswith(b"\n"))

    def test_contract_causal_rejections(self):
        raw = launch.make_contract("case_K")
        value = json.loads(raw)
        cases = {
            "duplicate_key": b'{"mode":"primary-no-input",' + raw[1:],
            "newline": raw + b"\n",
            "reordered": json.dumps(dict(reversed(list(value.items()))), separators=(",", ":")).encode(),
            "whitespace": json.dumps(value).encode(),
            "escaped_slashes": raw.replace(b"/", b"\\/"),
            "invalid_json": b"{",
            "oversized": b"x" * 65537,
        }
        mutations = [
            ("extra_key", lambda v: v.update(extra="x")),
            ("wrong_schema", lambda v: v.update(schema="other")),
            ("wrong_mode", lambda v: v.update(mode="fallback")),
            ("wrong_fixture", lambda v: v.update(fixtureId="qa-nested-list-control-v2")),
            ("wrong_name", lambda v: v.update(workflowName=launch.FIXTURE)),
            ("bool_nonce", lambda v: v.update(runNonce=True)),
            ("unicode_nonce", lambda v: v.update(runNonce="case_\u212a")),
            ("large_nonce", lambda v: v.update(runNonce="n" * 129)),
            ("outer_input", lambda v: v.update(primaryURL=v["primaryURL"] + "&input=text")),
            ("outer_text", lambda v: v.update(primaryURL=v["primaryURL"] + "&text=")),
            ("duplicate_name", lambda v: v.update(primaryURL=v["primaryURL"] + "&%6eame=x")),
            ("fragment", lambda v: v.update(primaryURL=v["primaryURL"] + "#x")),
            ("username", lambda v: v.update(primaryURL=v["primaryURL"].replace("//x-callback-url", "//u@x-callback-url"))),
            ("bad_percent", lambda v: v.update(primaryURL=v["primaryURL"] + "%XZ")),
            ("url_newline", lambda v: v.update(primaryURL=v["primaryURL"] + "\n")),
            ("callback_nonce", lambda v: v.update(primaryURL=v["primaryURL"].replace("case_K", "case_Q", 1))),
            ("callback_kelvin", lambda v: v.update(primaryURL=v["primaryURL"].replace("case_K", "case_%25E2%2584%25AA", 1))),
            ("callback_input", lambda v: v.update(primaryURL=v["primaryURL"].replace("shortcuts-x-callback", "shortcuts-x-callback%26input%3Dtext", 1))),
            ("cancelled_host", lambda v: v.update(primaryURL=v["primaryURL"].replace("%2F%2Fcancel%3F", "%2F%2Fcancelled%3F"))),
        ]
        for name, mutate in mutations:
            bad = copy.deepcopy(value)
            mutate(bad)
            cases[name] = launch.canonical(bad)
        for name, bad in cases.items():
            with self.subTest(name=name), self.assertRaises((ValueError, TypeError)):
                launch.validate_contract(bad)


class FunctionNoInputLaunchTests(unittest.TestCase):
    def setUp(self):
        self.run = SyntheticRun()
        self.addCleanup(self.run.close)

    def test_prepare_is_create_only_and_context_exact(self):
        context = self.run.prepare()
        self.assertEqual(context["harnessCommitSha"], self.run.commit)
        self.assertFalse(context["nativeExecutionGranted"])
        with self.assertRaises(FileExistsError):
            self.run.prepare()
        self.assertEqual(self.run.calls, [])

    def test_source_change_hidden_flags_and_symlink_rejected(self):
        path = self.run.repo / launch.SOURCE_PATHS[0]
        original = path.read_bytes()
        path.write_bytes(original + b"\nchanged")
        with self.assertRaisesRegex(ValueError, "SOURCE_CLEAN"):
            self.run.prepare()
        path.write_bytes(original)
        self.run.git("update-index", "--assume-unchanged", launch.SOURCE_PATHS[0])
        with self.assertRaisesRegex(ValueError, "SOURCE_HIDDEN_INDEX_FLAGS"):
            self.run.prepare()
        self.run.git("update-index", "--no-assume-unchanged", launch.SOURCE_PATHS[0])
        alias = self.run.base / "alias"
        alias.symlink_to(self.run.root, target_is_directory=True)
        with self.assertRaisesRegex(ValueError, "ROOT"):
            launch.prepare(self.run.repo, alias, self.run.commit, self.run.compiler, "owner", 1, self.run.nonce, self.run.udid)

    def test_claim_precedes_fixed_launch_and_replay_is_rejected(self):
        self.run.prepare()
        result = self.run.execute()
        self.assertEqual(result["status"], "PASS")
        self.assertEqual(result["receiverPID"], 4242)
        self.assertFalse(result["nativeExecutionAuthenticated"])
        self.assertEqual(len(self.run.calls), 1)
        command = self.run.calls[0]
        self.assertEqual(command[3:], ["/usr/bin/xcrun", "simctl", "launch", self.run.udid, launch.RECEIVER])
        with self.assertRaises(FileExistsError):
            self.run.execute()
        self.assertEqual(len(self.run.calls), 1)

    def test_nonzero_originals_retained_without_retry(self):
        self.run.prepare()
        def fail(*args, **kwargs):
            observed, details = self.run.capture(*args, **kwargs)
            observed.update(result="EXIT_NONZERO", exitCode=7)
            return observed, details
        with self.assertRaisesRegex(ValueError, "LAUNCH_PROCESS"):
            self.run.execute(fail)
        p = self.run.root / launch.RELATIVE
        result = json.loads((p / "result.original.json").read_bytes())
        self.assertEqual(result["status"], "FAIL")
        self.assertTrue(result["sourceUnchanged"])
        self.assertTrue((p / "process.original.json").is_file())
        self.assertEqual(len(self.run.calls), 1)

    def test_source_drift_has_terminal_original_failure(self):
        self.run.prepare()
        def changed(*args, **kwargs):
            values = self.run.capture(*args, **kwargs)
            p = self.run.repo / launch.SOURCE_PATHS[0]
            p.write_bytes(p.read_bytes() + b"\nchanged after launch")
            return values
        result = self.run.execute(changed)
        self.assertEqual(result["status"], "FAIL")
        self.assertFalse(result["sourceUnchanged"])
        self.assertIn("SOURCE_CLEAN", result["sourceRecheckError"])
        self.assertTrue((self.run.root / launch.RELATIVE / "result.original.json").is_file())

    def test_collection_preserves_bytes_rejects_reuse_and_extra_files(self):
        self.run.prepare()
        self.run.execute()
        self.run.triggers()
        acquired = self.run.root / launch.TRIGGERS
        value = json.loads((self.run.root / "runtime/function-envelope-trigger-acquisition.original.json").read_bytes())
        source = Path(value["sourceDirectory"])
        self.assertEqual(len(value["files"]), 4)
        for row in value["files"]:
            self.assertEqual((source / row["path"]).read_bytes(), (acquired / row["path"]).read_bytes())
        with self.assertRaises(FileExistsError):
            launch.collect_receiver(self.run.root, self.run.documents)

    def test_darwin_is_required_before_any_semantic_claim(self):
        self.run.prepare()
        with mock.patch.object(launch.sys, "platform", "linux"), self.assertRaisesRegex(ValueError, "ACTUAL_DARWIN"):
            launch.execute(self.run.repo, self.run.root)
        self.assertFalse((self.run.root / launch.RELATIVE / "attempt.claim.original.json").exists())


if __name__ == "__main__":
    unittest.main()
