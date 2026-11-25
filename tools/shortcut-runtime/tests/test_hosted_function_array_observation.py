"""Offline route/source controls; none executes JXA or establishes native results."""
import copy
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

HERE = Path(__file__).resolve()
ROOT = HERE.parents[3]
MODULE = ROOT / "tools/shortcut-runtime/hosted_function_array_observation.py"
if not MODULE.is_file():
    MODULE = HERE.with_name("hosted_function_array_observation.py")
spec = importlib.util.spec_from_file_location("hosted_function_array", MODULE)
hosted = importlib.util.module_from_spec(spec)
spec.loader.exec_module(hosted)
PLAN = MODULE.parent / "fixtures/function-array-hosted-plan.json"
if not PLAN.is_file():
    PLAN = HERE.with_name("function-array-hosted-plan.json")


class HostedFixedObservationTests(unittest.TestCase):
    def plan(self):
        return json.loads(PLAN.read_bytes())

    def test_workflow_preserves_every_existing_route_and_runs_only_fixed_probe(self):
        path = ROOT / hosted.WORKFLOW
        if not path.is_file():
            path = HERE.with_name("macos-shortcuts-deep-extract.yml")
        workflow = path.read_text()
        start = workflow.index("  function-dictionary-array:\n")
        finish = workflow.index("  preflight:\n", start)
        job = workflow[start:finish]
        self.assertIn("if: ${{ inputs.function_dictionary_array_only }}", job)
        self.assertIn("ref: " + hosted.PROBE_SHA, job)
        self.assertIn("ref: " + hosted.PRELUDE_COMMIT, job)
        self.assertIn("if: always()", job)
        self.assertNotIn("go test", job)
        self.assertNotIn("xcodebuild", job)
        self.assertNotIn("simctl", job)
        restored = workflow[:start] + workflow[finish:]
        flag_start = restored.index("      function_dictionary_array_only:\n")
        flag_end = restored.index("      literal_object_character_only:\n", flag_start)
        restored = restored[:flag_start] + restored[flag_end:]
        restored = restored.replace("inputs.literal_object_character_only && !inputs.function_dictionary_array_only", "inputs.literal_object_character_only")
        restored = restored.replace("!inputs.literal_object_character_only && !inputs.function_dictionary_array_only", "!inputs.literal_object_character_only")
        self.assertEqual(hosted.sha(restored.encode()), "65653147a60f66d4ba7fe42f53cb37791da39ed54180d69e6199773193d99094")

    def test_exact_plan_preserves_immutable_source_protocol_and_24_pins(self):
        plan = hosted.validate_plan(self.plan())
        self.assertEqual(plan["sourceSha"], "e0ee87ffa4eb289a8cc9927a86ec0b34a09e9c34")
        self.assertEqual((plan["wholeOwnerSeconds"], plan["perChildSeconds"]), (90, 8))
        self.assertEqual(len(plan["sourceFiles"]), 24)
        self.assertEqual(plan["sourceFiles"][hosted.PROBE]["sha256"], "007c093c60b30a3cb6963404786982c6f07733b826baa1a142167c3afbd0f156")
        actual = ROOT / hosted.PROBE
        if not actual.is_file():
            actual = HERE.parent.parent / "function-array-probe/observe_function_dictionary_array.py"
        self.assertEqual(hosted.sha(actual.read_bytes()), plan["sourceFiles"][hosted.PROBE]["sha256"])
        # Match the real importlib path. Import only; never call original main().
        original_spec = importlib.util.spec_from_file_location("cherri_original_probe_import_only", actual)
        original = importlib.util.module_from_spec(original_spec)
        original_spec.loader.exec_module(original)
        self.assertEqual((original.TOTAL_SECONDS, original.CHILD_SECONDS), (90, 8))
        self.assertEqual(tuple(original.CASE_IDS), hosted.CASE_IDS)

    def test_rejects_source_prelude_case_and_budget_changes(self):
        changes = {"sourceSha": "1" * 40, "sourceTree": "2" * 40, "preludeSha256": "3" * 64,
                   "caseIds": list(reversed(hosted.CASE_IDS)), "wholeOwnerSeconds": 91,
                   "perChildSeconds": 9, "schema": "other"}
        for key, value in changes.items():
            with self.subTest(key=key):
                plan = self.plan()
                plan[key] = value
                with self.assertRaisesRegex(ValueError, "FIXED_PLAN"):
                    hosted.validate_plan(plan)

    def test_rejects_missing_duplicate_or_unbounded_pin_metadata(self):
        for change in ("missing", "extra", "boolean", "digest"):
            with self.subTest(change=change):
                plan = self.plan()
                if change == "missing":
                    plan["sourceFiles"].pop(hosted.PROBE)
                elif change == "extra":
                    plan["sourceFiles"][hosted.PROBE]["arbitrary"] = True
                elif change == "boolean":
                    plan["sourceFiles"][hosted.PROBE]["bytes"] = True
                else:
                    plan["sourceFiles"][hosted.PROBE]["sha256"] = "not a digest"
                with self.assertRaises(ValueError):
                    hosted.validate_plan(plan)

    def test_create_only_receipts_never_overwrite_originals(self):
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp) / "original.json"
            hosted.write_new(p, {"first": True})
            before = p.read_bytes()
            with self.assertRaises(FileExistsError):
                hosted.write_new(p, {"first": False})
            self.assertEqual(p.read_bytes(), before)

    def test_rejects_symlink_and_parent_path_sources(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "file").write_text("x")
            (root / "linked").symlink_to(root / "file")
            for name in ("../file", str(root / "file"), "linked"):
                with self.subTest(name=name), self.assertRaises(ValueError):
                    hosted.regular_file(root, name)

    def repository(self, root):
        env = dict(os.environ, GIT_CONFIG_GLOBAL=os.devnull, GIT_CONFIG_SYSTEM=os.devnull)
        def command(argv):
            return subprocess.run(argv, capture_output=True, check=True, timeout=5, env=env).stdout
        command(["git", "init", "-q", str(root)])
        (root / "one.py").write_text("VALUE = 1\n")
        command(["git", "-C", str(root), "add", "one.py"])
        command(["git", "-C", str(root), "-c", "user.name=Offline fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "source"])
        head = command(["git", "-C", str(root), "rev-parse", "HEAD"]).decode().strip()
        return command, head

    def test_snapshot_binds_real_git_and_physical_bytes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            command, head = self.repository(root)
            raw = (root / "one.py").read_bytes()
            expected = {"one.py": {"bytes": len(raw), "sha256": hosted.sha(raw)}}
            good = hosted.snapshot(root, head, ["one.py"], command, expected)
            self.assertEqual(good["sha"], head)
            self.assertEqual(good["files"]["one.py"]["sha256"], expected["one.py"]["sha256"])
            (root / "one.py").write_text("VALUE = 2\n")
            with self.assertRaisesRegex(ValueError, "SOURCE_DIRTY"):
                hosted.snapshot(root, head, ["one.py"], command, expected)
            def blind_status(argv):
                return b"" if "status" in argv else command(argv)
            with self.assertRaisesRegex(ValueError, "SOURCE_PHYSICAL_GIT_BYTES"):
                hosted.snapshot(root, head, ["one.py"], blind_status, expected)

    def test_snapshot_rejects_wrong_head_and_fixed_digest(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            command, head = self.repository(root)
            with self.assertRaisesRegex(ValueError, "SOURCE_HEAD"):
                hosted.snapshot(root, "0" * 40, ["one.py"], command)
            with self.assertRaisesRegex(ValueError, "FIXED_SOURCE_BYTES"):
                hosted.snapshot(root, head, ["one.py"], command, {"one.py": {"bytes": 10, "sha256": "0" * 64}})

    def test_route_rejection_retains_failure_without_invoking_probe(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            output = root / "out"
            argv = ["controller", "--probe", str(root / "probe"), "--prelude", str(root / "prelude"), "--output", str(output)]
            with mock.patch.object(sys, "argv", argv), mock.patch.dict(os.environ, {}, clear=True), mock.patch.object(hosted, "invoke_original") as invoke:
                self.assertEqual(hosted.main(), 1)
                invoke.assert_not_called()
            actual = json.loads((output / "result.original.json").read_bytes())
            self.assertEqual(actual["error"]["message"], "FIXED_HOSTED_ROUTE")
            self.assertFalse(actual["nativeAcceptanceGranted"])
            self.assertFalse(actual["shortcutRuntime"])

    def test_fresh_output_collision_is_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            argv = ["controller", "--probe", str(root), "--prelude", str(root / "p"), "--output", str(root)]
            with mock.patch.object(sys, "argv", argv), self.assertRaisesRegex(SystemExit, "FRESH_OUTPUT_REQUIRED"):
                hosted.main()

    def fake_original(self, root, body):
        path = root / hosted.PROBE
        path.parent.mkdir(parents=True)
        path.write_text("TOTAL_SECONDS = 90\nCHILD_SECONDS = 8\nCASE_IDS = " + repr(list(hosted.CASE_IDS)) + "\nPRELUDE_SHA256 = " + repr(hosted.PRELUDE_SHA) + "\ndef main():\n" + body)

    def test_calls_original_once_with_unchanged_cli_arguments_and_restores_argv(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            out = root / "out"
            out.mkdir()
            self.fake_original(root, "    import sys\n    print(repr(sys.argv))\n    return 7\n")
            prior = sys.argv
            receipt = {}
            self.assertEqual(hosted.invoke_original(root, root / "prelude", out, receipt), 7)
            self.assertIs(sys.argv, prior)
            self.assertEqual(receipt["producerInvocation"]["exitCode"], 7)
            invoked = receipt["producerInvocation"]
            self.assertNotIn("argv", invoked)
            self.assertEqual(invoked["actualControllerProcess"]["originalArgv"], sys.orig_argv)
            self.assertEqual(invoked["actualControllerProcess"]["pid"], os.getpid())
            self.assertEqual(invoked["actualControllerProcess"]["pgid"], os.getpgrp())
            self.assertEqual(invoked["effectiveProducerSysArgv"][1:3], ["--source-sha", hosted.PROBE_SHA])
            self.assertNotEqual(invoked["effectiveProducerSysArgv"], invoked["actualControllerProcess"]["originalArgv"])
            self.assertEqual(len(list(out.glob("producer-intent.original.json"))), 1)
            self.assertIn(hosted.PROBE_SHA, (out / "producer.stdout.original").read_text())

    def test_original_exception_is_retained_and_cannot_become_success(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            out = root / "out"
            out.mkdir()
            self.fake_original(root, "    raise InterruptedError('original cancellation')\n")
            receipt = {}
            prior = sys.argv
            with self.assertRaisesRegex(InterruptedError, "original cancellation"):
                hosted.invoke_original(root, root / "prelude", out, receipt)
            self.assertIs(sys.argv, prior)
            returned = json.loads((out / "producer-return.original.json").read_bytes())
            self.assertNotIn("exitCode", returned)
            self.assertIn("finishedAt", returned)

    def test_original_protocol_drift_blocks_before_invocation(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            out = root / "out"
            out.mkdir()
            self.fake_original(root, "    raise AssertionError('must not execute')\n")
            path = root / hosted.PROBE
            path.write_text(path.read_text().replace("CHILD_SECONDS = 8", "CHILD_SECONDS = 9"))
            with self.assertRaisesRegex(ValueError, "ORIGINAL_PROTOCOL"):
                hosted.invoke_original(root, root / "prelude", out, {})
            self.assertFalse((out / "producer-intent.original.json").exists())


if __name__ == "__main__":
    unittest.main()
