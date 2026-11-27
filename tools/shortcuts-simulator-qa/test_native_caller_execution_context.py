"""Causal environment and namespace controls for the explicit hosted FN04 profile."""
import copy
import hashlib
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest
from unittest import mock

import native_caller_execution_context as context
import native_caller_input_owner as owner
import native_signed_caller_input_owner as signed
import test_native_caller_input_owner as original_controls


class HostedCallerContextTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name).resolve()
        self.repo, self.temp = self.root / "checkout", self.root / "runner-temp"
        self.repo.mkdir()
        self.temp.mkdir()
        self.event_path = self.root / "create-event.json"
        self.event = {"ref_type": "branch", "ref": context.BRANCH,
                      "repository": {"full_name": context.REPOSITORY}}
        self.event_path.write_text(json.dumps(self.event))
        self.environment = {
            "GITHUB_ACTIONS": "true", "GITHUB_REPOSITORY": context.REPOSITORY,
            "GITHUB_WORKFLOW": context.WORKFLOW, "GITHUB_JOB": context.JOB,
            "GITHUB_EVENT_NAME": "create", "GITHUB_EVENT_PATH": str(self.event_path), "RUNNER_OS": "macOS",
            "GITHUB_SHA": "c"*40, "GITHUB_RUN_ID": "1234567", "GITHUB_RUN_ATTEMPT": "1",
            "GITHUB_REF": "refs/heads/" + context.BRANCH,
            "GITHUB_WORKFLOW_REF": context.REPOSITORY + "/" + context.WORKFLOW_FILE +
                                   "@refs/heads/" + context.BRANCH,
            "GITHUB_WORKFLOW_SHA": "c"*40,
            "GITHUB_WORKSPACE": str(self.repo), "RUNNER_TEMP": str(self.temp),
            "CHERRI_FN04_CASE": "nested_false",
        }

    def tearDown(self):
        self.temporary.cleanup()

    def model(self):
        value = context.context_from_environment()
        plan = original_controls.NativeCallerOwnerTests.plan()
        roots = context.paths(value)
        for key in ("repo", "root", "buildRoot", "bundleRoot"):
            plan[key] = str(roots[key])
        plan.update(controllerCommitSha=value["sourceCommitSha"],
                    builtHarnessCommitSha=value["sourceCommitSha"],
                    runId=value["runId"], caseId=value["caseId"],
                    pythonExecutableSHA256=hashlib.sha256(
                        Path(sys.executable).resolve().read_bytes()).hexdigest())
        plan["deviceRecord"].update(sourceCommit=value["sourceCommitSha"], runId=value["runId"])
        return value, plan

    def test_actual_context_rejects_other_job_source_ref_run_attempt_and_case(self):
        mutations = {
            "GITHUB_ACTIONS": "false", "GITHUB_REPOSITORY": "other/cherri",
            "GITHUB_WORKFLOW": "iOS 27 Shortcuts Runtime PoC", "GITHUB_JOB": "other",
            "GITHUB_EVENT_NAME": "push", "RUNNER_OS": "Linux", "GITHUB_SHA": "0",
            "GITHUB_RUN_ID": "0", "GITHUB_RUN_ATTEMPT": "2", "GITHUB_REF": "refs/heads/main",
            "GITHUB_WORKFLOW_REF": "unbound", "GITHUB_WORKFLOW_SHA": "d"*40,
            "CHERRI_FN04_CASE": "unknown",
        }
        self.assertEqual(context.context_from_environment(self.environment)["runId"], "1234567")
        for key, value in mutations.items():
            with self.subTest(field=key):
                changed = dict(self.environment, **{key: value})
                with self.assertRaises(ValueError):
                    context.context_from_environment(changed)

    def test_context_must_equal_current_environment_and_current_owner(self):
        with mock.patch.dict(os.environ, self.environment, clear=True):
            value, _ = self.model()
            self.assertEqual(context.validate_context(value), value)
            for key, alternate in (("uid", os.getuid()+1), ("runId", "7654321"),
                    ("sourceCommitSha", "d"*40), ("caseId", "nested_true"),
                    ("workspace", str(self.root)), ("namespace", str(self.root / "alias")),
                    ("extra", True)):
                with self.subTest(field=key):
                    with self.assertRaisesRegex(ValueError, "HOSTED_CONTEXT_CHANGED"):
                        context.validate_context(dict(value, **{key: alternate}))

    def test_source_and_temporary_namespace_aliases_are_rejected(self):
        for temporary in (self.repo, self.root, self.repo / "inside"):
            temporary.mkdir(exist_ok=True)
            with self.subTest(temporary=str(temporary)):
                with self.assertRaisesRegex(ValueError, "HOSTED_SOURCE_TEMP_ALIAS"):
                    context.context_from_environment(dict(self.environment, RUNNER_TEMP=str(temporary)))
        with mock.patch.object(context, "read_create_event",
                return_value=(self.event, self.event_path.read_bytes())), \
                mock.patch.object(os, "getuid", return_value=os.getuid()+1):
            with self.assertRaisesRegex(ValueError, "HOSTED_PATH_OWNER"):
                context.context_from_environment(self.environment)

    def test_native_plan_requires_exact_current_source_case_and_role_paths(self):
        with mock.patch.dict(os.environ, self.environment, clear=True):
            value, plan = self.model()
            self.assertEqual(owner.validate_plan(plan, value), plan)
            mutations = [(key, str(self.root / ("wrong-" + key))) for key in
                         ("repo", "root", "buildRoot", "bundleRoot")]
            mutations += [("controllerCommitSha", "d"*40), ("builtHarnessCommitSha", "b"*40),
                          ("runId", "7654321"), ("caseId", "nested_true")]
            for key, alternate in mutations:
                with self.subTest(field=key):
                    changed = copy.deepcopy(plan)
                    changed[key] = alternate
                    with self.assertRaises(ValueError):
                        owner.validate_plan(changed, value)
            with self.assertRaisesRegex(ValueError, "OWNED_LOCAL_PATH"):
                owner.validate_plan(plan)

    def test_local_default_still_rejects_actual_workflow_environment(self):
        plan = original_controls.NativeCallerOwnerTests.plan()
        plan["pythonExecutableSHA256"] = hashlib.sha256(
            Path(sys.executable).resolve().read_bytes()).hexdigest()
        with mock.patch.dict(os.environ, self.environment, clear=True), \
                mock.patch.object(owner.sys, "platform", "darwin"):
            with self.assertRaisesRegex(ValueError, "TRUTHFUL_LOCAL_PROVENANCE"):
                owner.NativeCallerObservation(plan)

    def test_explicit_constructor_retains_actual_context_without_environment_removal(self):
        with mock.patch.dict(os.environ, self.environment, clear=True), \
                mock.patch.object(owner.sys, "platform", "darwin"):
            value, plan = self.model()
            context.paths(value)["private"].mkdir(parents=True)
            instance = owner.NativeCallerObservation(plan, execution_context=value)
            self.assertEqual(instance.summary["executionLocation"], "GITHUB_ACTIONS_MACOS")
            self.assertEqual(instance.summary["hostedContext"], value)
            self.assertEqual(os.environ["GITHUB_RUN_ID"], value["runId"])
            self.assertFalse(instance.summary["runtimeObservationAuthenticated"])
            self.assertFalse(instance.summary["formalAcceptanceGranted"])
            self.assertTrue((Path(plan["root"]) / "execution-context.original.json").is_file())

    def test_signing_selection_cannot_reuse_a_local_or_other_run_bundle(self):
        with mock.patch.dict(os.environ, self.environment, clear=True):
            value, plan = self.model()
            selection = {"schema": signed.SELECTION_SCHEMA,
                "planSHA256": owner.sha(owner.library.canonical(plan)+b"\n"),
                "bundleRoot": str(context.paths(value)["signingRoot"]),
                "bundleIntakeSHA256": "e"*64}
            self.assertEqual(signed.validate_selection(selection, plan, value), selection)
            for path in ("/Users/m1/builds/old-signatures", str(self.root / "signatures"),
                         plan["bundleRoot"]):
                with self.subTest(path=path):
                    with self.assertRaises(ValueError):
                        signed.validate_selection(dict(selection, bundleRoot=path), plan, value)
            with self.assertRaisesRegex(ValueError, "OWNED_SIGNING_BUNDLE_PATH"):
                signed.validate_selection(selection, plan)

    def test_hosted_product_scope_is_exact_and_rejects_other_deriveddata(self):
        with mock.patch.dict(os.environ, self.environment, clear=True):
            value, _ = self.model()
            valid = context.paths(value)["derivedData"] / "Build/Products/current.xctestrun"
            self.assertTrue(context.product_scope(valid, value))
            for path in (self.root / "other/current.xctestrun",
                         Path("/Users/m1/caches/DerivedData/old/Build/Products/old.xctestrun"),
                         valid.parent / "nested/current.xctestrun"):
                with self.subTest(path=str(path)):
                    self.assertFalse(context.product_scope(path, value))


    def test_create_event_is_exact_bounded_and_revalidated(self):
        good = self.event_path.read_bytes()
        with mock.patch.dict(os.environ, self.environment, clear=True):
            value = context.context_from_environment()
            self.assertEqual(value["eventName"], "create")
            self.assertEqual(value["eventPayloadSHA256"], hashlib.sha256(good).hexdigest())
            mutations = [
                dict(self.event, ref_type="tag"), dict(self.event, ref="other"),
                dict(self.event, repository={"full_name": "other/cherri"}),
                dict(self.event, repository=None), [], None,
            ]
            for index, changed in enumerate(mutations):
                with self.subTest(payload=index):
                    self.event_path.write_text(json.dumps(changed))
                    with self.assertRaises(ValueError):
                        context.context_from_environment()
            for label, raw in (("duplicate", b'{"ref":"a","ref":"b"}'),
                               ("trailing", good+b"extra"),
                               ("oversized", b" " * 65537),
                               ("empty", b"")):
                with self.subTest(payload=label):
                    self.event_path.write_bytes(raw)
                    with self.assertRaises(ValueError):
                        context.context_from_environment()
            self.event_path.write_bytes(good)
            for key, changed in (("GITHUB_EVENT_NAME", "workflow_dispatch"),
                                 ("GITHUB_RUN_ATTEMPT", "2"),
                                 ("GITHUB_REF", "refs/heads/other"),
                                 ("CHERRI_FN04_CASE", "nested_true"),
                                 ("GITHUB_EVENT_PATH", "relative")):
                with self.subTest(environment=key):
                    with self.assertRaises(ValueError):
                        context.context_from_environment(dict(self.environment, **{key: changed}))
            with mock.patch.object(os, "getuid", return_value=os.getuid()+1):
                with self.assertRaisesRegex(ValueError, "BOUNDED_CREATE_EVENT_FILE"):
                    context.context_from_environment()
            self.event_path.write_bytes(good+b" ")
            with self.assertRaisesRegex(ValueError, "HOSTED_CONTEXT_CHANGED"):
                context.validate_context(value)


if __name__ == "__main__":
    unittest.main()
