"""Exercise the real storage selector/callback contracts; no storage is simulated."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import unittest

import test_runtime_callback_contract as callback_contract
import test_workflow_input_contract as workflow_contract

ROOT = Path(__file__).resolve().parents[2]


class StorageProbeContractTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        workflow_contract.WorkflowInputContractTests.setUpClass()
        callback_contract.CallbackContractTests.setUpClass()
        cls.workflow = workflow_contract.WorkflowInputContractTests()
        cls.callback = callback_contract.CallbackContractTests()
        cls.manifest = json.loads((ROOT/"docs/apple-runtime/p10r15/storage-probe-manifest.json").read_bytes())
        cls.cases = cls.manifest["cases"]

    def test_explicit_storage_choices_bind_owned_sources(self):
        script = (ROOT/"scripts/ios27_shortcuts_simulator_qa.sh").read_text(encoding="utf-8")
        selector = script.split("# Select primary target fixture from SAMPLE_ID\n", 1)[1].split("LEGACY_COUNTERPART_SOURCE=", 1)[0]
        selector += '\nprintf "%s\\n" "$TARGET_FIXTURE_ID" "$TARGET_SOURCE_FILE" "$EXPECTED_SHORTCUT_NAME" "$EXPECTED_SEMANTIC_OUTPUT"\n'
        workflow = (ROOT/".github/workflows/cherri-ios27-shortcuts-simulator-qa.yml").read_text(encoding="utf-8")
        options = workflow.split("      sample_id:", 1)[1].split("      capture_full_editor:", 1)[0]
        for case in self.cases:
            with self.subTest(fixture=case["id"]):
                self.assertIn("          - "+case["sampleId"]+"\n", options)
                self.assertEqual(self.workflow.validate(["full","ipad-default",case["sampleId"],"false","20","true"]).returncode, 0)
                result = subprocess.run([self.workflow.bash,"-s"], input=selector,
                    env={**os.environ,"SAMPLE_ID":case["sampleId"],"ROOT_DIR":"/reviewed",
                         "TARGET_SOURCE_RELATIVE":"../../private-input.cherri"},
                    capture_output=True,text=True,timeout=10)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout.splitlines(), [
                    case["id"], "/reviewed/"+case["path"], case["id"], case["expectedSummary"]])
        for sample in ("qa-storage-private","qa-storage-text-same-run-release",
                       "qa-storage-scope;exit 0","../qa-storage-delete"):
            self.assertEqual(self.workflow.validate(["full","ipad-default",sample,"false","20","true"]).returncode, 2)

    def test_storage_callbacks_require_each_exact_flag_and_normal_trace(self):
        for case in self.cases:
            fid, expected = case["id"], case["expectedSummary"]
            with self.subTest(fixture=fid):
                event = self.callback.event()
                event.update(fixtureId=fid, expectedShortcutName=fid, resultSummary=expected)
                event["tracePayload"] = event["tracePayload"].replace("qa-unit", fid)
                result = self.callback.verify([event],fid,expected)
                self.assertEqual((result["runGate"], result["traceGate"]), ("PASS","PASS"))
                summaries = {expected+"_WRONG"}
                for flag in expected.split(":")[1:]:
                    summaries.add(expected.replace(flag, flag+"_FAIL", 1))
                for summary in summaries:
                    self.assertEqual(self.callback.verify([{**event,"resultSummary":summary}],fid,expected)["runGate"], "FAIL_RUNTIME")
                for key, value in (("status","ERROR"),("status","CANCELLED"),("runId","stale"),
                                   ("fixtureId","qa-other"),("expectedShortcutName","qa-other"),
                                   ("receiverSelfTest",True),("eventSource","receiver-self-test")):
                    self.assertEqual(self.callback.verify([{**event,key:value}],fid,expected)["runGate"], "FAIL_RUNTIME")
                self.assertEqual(self.callback.verify([event,event],fid,expected)["runGate"], "FAIL_RUNTIME")
                for trace in ("", event["tracePayload"].replace(fid,"qa-other")):
                    self.assertEqual(self.callback.verify([{**event,"tracePayload":trace}],fid,expected)["traceGate"], "FAIL_RUNTIME")
                manifest = self.callback.manifest(fid)
                manifest["fixtures"][0].update(buildProfile="release",runtimeTraceExpectation="ABSENT_RELEASE",
                    releaseDiagnosticActionsAbsent=True,releaseBodyShapeVerified=True)
                self.assertEqual(self.callback.verify([event],fid,expected,manifest)["runGate"], "FAIL_RUNTIME")

    def test_compiler_identity_rejects_unproved_or_mismatched_binaries(self):
        import importlib.util
        spec = importlib.util.spec_from_file_location("storage_probe_producer", ROOT/"tools/p10r15/verify_storage_probes.py")
        producer = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(producer)
        sha = "a" * 40
        good = "\tvcs.revision=" + sha + "\n\tvcs.modified=false\n"
        self.assertEqual(producer.verify_build_identity(good, sha, False, False)["mode"], "COMPILER_VCS_METADATA")
        self.assertEqual(producer.verify_build_identity("", sha, True, True)["mode"], "CONTROLLED_BUILD_FROM_CLEAN_COMMIT")
        for info, rebuilt, worktree in [
            ("", False, True), ("", False, False), ("", True, False),
            (good.replace(sha, "b" * 40), True, True),
            (good.replace("modified=false", "modified=true"), True, True),
            (good.replace("\tvcs.modified=false\n", ""), True, True),
        ]:
            with self.subTest(info=info, rebuilt=rebuilt, worktree=worktree):
                with self.assertRaises(RuntimeError):
                    producer.verify_build_identity(info, sha, rebuilt, worktree)

    def test_manifest_and_unicode_preserve_the_unproven_boundary(self):
        self.assertTrue(self.manifest["syntheticOnly"])
        self.assertTrue(self.manifest["sameRunOnly"])
        self.assertFalse(self.manifest["cacheImplemented"])
        self.assertEqual(self.manifest["crossRunPersistence"], "NOT_PROVEN")
        self.assertEqual(self.manifest["typedArchive"], "NOT_PROVEN")
        self.assertEqual(len(self.cases), 3)
        for case in self.cases:
            path = ROOT/case["path"]
            data = path.read_bytes()
            self.assertEqual(hashlib.sha256(data).hexdigest(), case["sourceSha256"])
            self.assertEqual(case["nativeRuntimeStatus"], "NOT_EXECUTED")
            source = data.decode("utf-8")
            self.assertIn("cherri.qa.p15.storage.", source)
            self.assertNotIn("lumen.", source)
            self.assertNotIn("downloadURL(", source)
            if "text-same-run" in case["id"]:
                self.assertIn("\u05e9\u05b8\u05c1\u05dc\u05d5\u05b9\u05dd", source)
                self.assertIn("\U0001f352", source)
                self.assertNotIn("????", source)


if __name__ == "__main__":
    unittest.main()
