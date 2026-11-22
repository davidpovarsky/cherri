#!/usr/bin/env python3
"""Run only the nine fixed synthetic Optional receipt controls.

The independent Go job owns actual compilation/native graph proof. Every Go
subprocess in these existing Python tests is mocked; an unmocked launch fails.
"""
import ast
import json
import os
from pathlib import Path
import sys

import run_function_envelope_focused as support


PACKAGE = "cherri.python.optional-forwarding-synthetic"
PROCESS_SECONDS = 60
STEP_SECONDS = 120
JOB_SECONDS = 240
# Fixed from actual source tables before execution; JSON kinds are significant.
EXPECTED_SUBTESTS = ({'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_absent_duplicate_and_duplicate_key_receipts_reject', 'ordinal': 1, 'params': {'output_length': 5}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_absent_duplicate_and_duplicate_key_receipts_reject', 'ordinal': 2, 'params': {'output_length': 1060}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_absent_duplicate_and_duplicate_key_receipts_reject', 'ordinal': 3, 'params': {'output_length': 556}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_absent_duplicate_and_duplicate_key_receipts_reject', 'ordinal': 4, 'params': {'output_length': 39}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_containment_regular_file_and_digest_fail_before_go', 'ordinal': 1, 'params': {'path': 'qa-optional-forwarding-diagnostic-v2.shortcut', 'digest': 'f0c23442'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_containment_regular_file_and_digest_fail_before_go', 'ordinal': 2, 'params': {'path': 'unrelated.shortcut', 'digest': 'f0c23442'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_containment_regular_file_and_digest_fail_before_go', 'ordinal': 3, 'params': {'path': 'qa-optional-forwarding-diagnostic-v2.shortcut', 'digest': '00000000'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_containment_regular_file_and_digest_fail_before_go', 'ordinal': 4, 'params': {'path': 'qa-optional-forwarding-diagnostic-v2.shortcut', 'digest': 'not-a-di'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_public_pins_and_exact_eleven_field_contract', 'ordinal': 1, 'params': {'source_bytes': 3486}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_public_pins_and_exact_eleven_field_contract', 'ordinal': 2, 'params': {'source_bytes': 3486}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 1, 'params': {'key': 'schemaVersion', 'value': 'wrong'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 2, 'params': {'key': 'fixtureId', 'value': 'wrong'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 3, 'params': {'key': 'status', 'value': 'wrong'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 4, 'params': {'key': 'sourceSha256', 'value': 'wrong'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 5, 'params': {'key': 'expectedContractSha256', 'value': 'wrong'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 6, 'params': {'key': 'unsignedPlistSha256', 'value': 'wrong'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 7, 'params': {'key': 'nativeActionCount', 'value': 'wrong'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 8, 'params': {'key': 'appleRuntime', 'value': 'wrong'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 9, 'params': {'key': 'formalAcceptanceGranted', 'value': 'wrong'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 10, 'params': {'key': 'nativeActionCount', 'value': 399}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 11, 'params': {'key': 'nativeActionCount', 'value': 400.0}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 12, 'params': {'key': 'nativeActionCount', 'value': 400}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 13, 'params': {'key': 'nativeActionCount', 'value': 402.0}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 14, 'params': {'key': 'formalAcceptanceGranted', 'value': 0}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 15, 'params': {'key': 'formalAcceptanceGranted', 'value': True}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 16, 'params': {'key': 'appleRuntime', 'value': 'PASS'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 17, 'params': {'key': 'unreviewedPayload', 'value': 'PRIVATE_CANARY'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 18, 'params': {'missing': 'schemaVersion'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 19, 'params': {'missing': 'fixtureId'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 20, 'params': {'missing': 'status'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 21, 'params': {'missing': 'sourceSha256'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 22, 'params': {'missing': 'expectedContractSha256'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 23, 'params': {'missing': 'unsignedPlistSha256'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 24, 'params': {'missing': 'nativeActionCount'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 25, 'params': {'missing': 'appleRuntime'}}, {'parent': 'test_optional_forwarding_diagnostic.OptionalArtifactGateTests.test_receipt_schema_truth_and_artifact_binding_mutations_reject', 'ordinal': 26, 'params': {'missing': 'formalAcceptanceGranted'}})

# This child is retained byte-for-byte outside both checkouts before execution.
# It observes actual unittest callbacks, preserving original assertion failures
# and tracebacks on stderr. No expected result is inferred from a test name.
CHILD_SOURCE = r'''import datetime
import importlib
import json
from pathlib import Path
import subprocess
import sys
import time
import traceback
import unittest
from unittest.mock import patch

PACKAGE = "cherri.python.optional-forwarding-synthetic"
EXPECTED_IDS = PYTHON_IDS_LITERAL

def emit(action, name="", **values):
    event = {"Time": datetime.datetime.now(datetime.timezone.utc).isoformat().replace("+00:00", "Z"),
             "Action": action, "Package": PACKAGE}
    if name:
        event["Test"] = name
    event.update(values)
    print(json.dumps(event, sort_keys=True, ensure_ascii=True, allow_nan=False), flush=True)

def forbid_external(*args, **kwargs):
    raise AssertionError("Synthetic receipt controls may not start an external process")

class ObservedResult(unittest.TextTestResult):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.started = {}
        self.outcomes = {}
        self.subcase_ordinals = {}

    def startTest(self, test):
        name = test.id()
        if name not in EXPECTED_IDS or name in self.started:
            raise AssertionError("Unexpected or duplicate actual Python parent")
        self.started[name] = time.monotonic()
        self.subcase_ordinals[name] = 0
        emit("run", name)
        super().startTest(test)

    def addSuccess(self, test):
        self.outcomes[test.id()] = "pass"
        super().addSuccess(test)

    def addFailure(self, test, err):
        self.outcomes[test.id()] = "fail"
        super().addFailure(test, err)

    def addError(self, test, err):
        self.outcomes[test.id()] = "fail"
        super().addError(test, err)

    def addSkip(self, test, reason):
        self.outcomes[test.id()] = "skip"
        super().addSkip(test, reason)

    def addExpectedFailure(self, test, err):
        self.outcomes[test.id()] = "fail"
        super().addExpectedFailure(test, err)

    def addUnexpectedSuccess(self, test):
        self.outcomes[test.id()] = "fail"
        super().addUnexpectedSuccess(test)

    def addSubTest(self, test, subtest, err):
        name = test.id()
        self.subcase_ordinals[name] += 1
        outcome = "pass" if err is None else ("fail" if issubclass(err[0], test.failureException) else "error")
        if err is not None:
            self.outcomes[name] = "fail"
        payload = {"kind": "unittest-subtest", "ordinal": self.subcase_ordinals[name],
                   "outcome": outcome, "params": dict(subtest.params)}
        emit("output", name, Output=json.dumps(payload, sort_keys=True, ensure_ascii=True, allow_nan=False))
        super().addSubTest(test, subtest, err)

    def stopTest(self, test):
        name = test.id()
        elapsed = time.monotonic() - self.started[name]
        emit(self.outcomes.get(name, "fail"), name, Elapsed=round(elapsed, 9))
        super().stopTest(test)

def main():
    began = time.monotonic()
    emit("start")
    passed = False
    try:
        if len(sys.argv) != 2:
            raise AssertionError("Exact source directory required")
        root = Path(sys.argv[1])
        if not root.is_absolute() or root.resolve(strict=True) != root or not root.is_dir():
            raise AssertionError("Source directory must be an exact existing physical path")
        module_root = root / "tools/shortcuts-simulator-qa"
        sys.path.insert(0, str(module_root))
        module = importlib.import_module("test_optional_forwarding_diagnostic")
        if Path(module.__file__).resolve(strict=True) != module_root / "test_optional_forwarding_diagnostic.py":
            raise AssertionError("Imported tests differ from exact subject")
        if Path(module.gate.__file__).resolve(strict=True) != module_root / "optional_forwarding_diagnostic.py":
            raise AssertionError("Imported receipt gate differs from exact subject")
        names = unittest.defaultTestLoader.getTestCaseNames(module.OptionalArtifactGateTests)
        actual_ids = tuple(module.OptionalArtifactGateTests(name).id() for name in names)
        if actual_ids != EXPECTED_IDS:
            raise AssertionError("Actual source Python parent inventory changed")
        suite = unittest.TestSuite(module.OptionalArtifactGateTests(name) for name in names)
        with patch.object(subprocess, "Popen", side_effect=forbid_external):
            result = unittest.TextTestRunner(stream=sys.stderr, verbosity=2, resultclass=ObservedResult).run(suite)
        passed = (result.wasSuccessful() and result.testsRun == 9 and
                  set(result.started) == set(EXPECTED_IDS) and
                  all(result.outcomes.get(name) == "pass" for name in EXPECTED_IDS) and
                  not result.skipped and not result.expectedFailures and not result.unexpectedSuccesses)
    except BaseException:
        traceback.print_exc(file=sys.stderr)
    emit("pass" if passed else "fail", Elapsed=round(time.monotonic() - began, 9))
    return 0 if passed else 1

if __name__ == "__main__":
    raise SystemExit(main())
'''.replace("PYTHON_IDS_LITERAL", repr(tuple(support.PYTHON_PARENTS)))


def verify_python_inventory(inventory):
    support.verify_fixed_inventory(inventory)
    expected = list(EXPECTED_SUBTESTS)
    support.require(len(expected) == 36 and len(support.PYTHON_PARENTS) == 9,
                    "Fixed synthetic source inventory is incomplete")
    # Canonical encoded bytes retain JSON scalar types, including int vs float.
    support.require(support.encoded(inventory.get("python_subtests")) == support.encoded(expected),
                    "Synthetic fixed subcase parameters changed")
    support.require(support.encoded(inventory.get("python_budgets_seconds")) == support.encoded(
        {"process": PROCESS_SECONDS, "workflow_step": STEP_SECONDS, "workflow_job": JOB_SECONDS,
         "cleanup_term": 5, "cleanup_kill": 10}), "Synthetic process budget changed")


def parse_python_events(raw, process):
    parsed = support.parse_go_events(raw, process["started_at"], process["finished_at"],
                                    expected_events=support.PYTHON_PARENTS, package=PACKAGE)
    observed, errors, ordinals = [], [], {}
    for number, line in enumerate(raw.splitlines(), 1):
        try:
            event = support.strict_json(line)
            if event.get("Action") != "output":
                continue
            support.require(type(event.get("Output")) is str, "Invalid synthetic callback payload")
            value = support.strict_json(event["Output"])
            support.require(isinstance(value, dict) and set(value) == {"kind", "ordinal", "outcome", "params"},
                            "Unexpected synthetic callback fields")
            parent = event.get("Test")
            support.require(parent in support.PYTHON_PARENTS and value["kind"] == "unittest-subtest",
                            "Unexpected synthetic callback parent/kind")
            ordinal = ordinals.get(parent, 0) + 1
            support.require(type(value["ordinal"]) is int and value["ordinal"] == ordinal,
                            "Unexpected synthetic callback order")
            support.require(value["outcome"] in ("pass", "fail", "error") and isinstance(value["params"], dict),
                            "Invalid synthetic callback result/parameters")
            ordinals[parent] = ordinal
            observed.append({"parent": parent, "ordinal": ordinal,
                             "params": value["params"], "outcome": value["outcome"]})
        except Exception as error:
            errors.append({"line": number, "error": type(error).__name__ + ": " + str(error)})
    without_result = [{key: row[key] for key in ("parent", "ordinal", "params")} for row in observed]
    complete = support.encoded(without_result) == support.encoded(list(EXPECTED_SUBTESTS))
    passed = complete and not errors and all(row["outcome"] == "pass" for row in observed)
    parsed.update(synthetic_subcases=observed, synthetic_subcase_errors=errors,
                  synthetic_subcase_inventory_complete=complete, synthetic_subcases_passed=passed)
    return parsed


def main():
    support.require(os.name == "posix", "Synthetic wrapper requires the reviewed POSIX process owner")
    root = Path(os.environ["CHERRI_COMPONENT_SOURCE"]).resolve(strict=True)
    dest = Path(os.environ["CHERRI_COMPONENT_OUT"]).resolve()
    script = Path(__file__).resolve(strict=True)
    workflow_root, workflow_sha = script.parents[2], os.environ["GITHUB_SHA"]
    support.require(script == workflow_root / support.HARNESS_PATHS[3], "Synthetic wrapper outside tracked path")
    for other in (root, workflow_root):
        support.require(not dest.is_relative_to(other) and not other.is_relative_to(dest),
                        "Output must be outside both exact checkouts")
    support.require(not root.is_relative_to(workflow_root) and not workflow_root.is_relative_to(root),
                    "Checkouts must be distinct")
    support.require(not dest.exists(), "Exclusive evidence destination already exists")
    dest.mkdir(parents=True)
    metadata_dir = dest / "metadata"
    metadata_dir.mkdir()
    temporary = dest / "synthetic-temporary"
    temporary.mkdir()
    environment = dict(os.environ, GIT_NO_REPLACE_OBJECTS="1", PYTHONDONTWRITEBYTECODE="1",
                       TMPDIR=str(temporary), TEMP=str(temporary), TMP=str(temporary))
    metadata = support.Metadata(metadata_dir, environment)
    child = dest / "python-child.original.py"
    child_raw = CHILD_SOURCE.encode("utf-8")
    command = [sys.executable, "-I", "-B", str(child), str(root)]
    summary = {"schema": "cherri.p15.function-envelope-python-synthetic/v1",
               "source": support.EXPECTED_SHA, "source_parent": support.EXPECTED_PARENT,
               "source_tree": support.EXPECTED_TREE, "workflow_source": workflow_sha,
               "execution_platform": "ubuntu-ci", "command": command,
               "actual_python_started": False, "actual_go_started": False, "passed_all": False,
               "expected_events": 9, "expected_subcases": 36,
               "started": 0, "passed": 0, "failed": 0, "skipped": 0,
               "unstarted": list(support.PYTHON_PARENTS), "unterminated": [], "errors": [],
               "formal_acceptance_granted": False, "apple_runtime_proved": False,
               "complete_envelope_proved": False, "native_execution_proved": False}
    before, after, process, inventory, names = {}, {}, None, None, None
    child_created = False
    with support.cancellation_signals():
        try:
            original = metadata.git(workflow_root, "inventory-blob", "show", workflow_sha + ":" + support.INVENTORY_PATH)
            working = support.read_original(workflow_root / support.INVENTORY_PATH, support.MAX_SOURCE)[0]
            support.require(working == original, "Inventory differs from exact workflow Git bytes")
            inventory = support.strict_json(original)
            verify_python_inventory(inventory)
            names = support.verify_fixed_inventory(inventory)
            before["subject"] = support.repository_state(metadata, "before", "subject", root, support.EXPECTED_SHA, names)
            before["workflow"] = support.repository_state(metadata, "before", "workflow", workflow_root, workflow_sha,
                                                         support.HARNESS_PATHS)
            support.verify_observed_bindings(before["subject"], inventory)
            ancestry = metadata.git(root, "source-parent", "rev-list", "--parents", "-n", "1", support.EXPECTED_SHA)
            support.require(ancestry.decode("ascii").strip() == support.EXPECTED_SHA + " " + support.EXPECTED_PARENT,
                            "Synthetic subject ancestry changed")
            tree = metadata.git(root, "source-tree", "rev-parse", support.EXPECTED_SHA + "^{tree}")
            support.require(tree.decode("ascii").strip() == support.EXPECTED_TREE, "Synthetic subject tree changed")
            ast.parse(CHILD_SOURCE)
            with child.open("xb") as stream:
                child_created = True
                stream.write(child_raw)
            os.chmod(child, 0o600)
            retained, child_identity = support.read_original(child, support.MAX_SOURCE)
            support.require(retained == child_raw, "Synthetic child source changed before execution")
            launch = dict(summary, root=str(root), created_at=support.utc(), process_timeout_seconds=PROCESS_SECONDS,
                          workflow_step_timeout_seconds=STEP_SECONDS, workflow_job_timeout_seconds=JOB_SECONDS,
                          cleanup_term_seconds=5, cleanup_kill_seconds=10,
                          policy="One exact nine-parent synthetic control process; no retry or external Go launch.",
                          expected_test_events=support.PYTHON_PARENTS, expected_subtest_completions=EXPECTED_SUBTESTS,
                          child_source=child_identity, inventory_sha256=support.sha(original),
                          source_before=before["subject"], workflow_before=before["workflow"])
            (dest / "launch.json").write_bytes(support.encoded(launch))
            process = support.run_owned_process(command, root, environment, dest, "python-synthetic", PROCESS_SECONDS)
        except BaseException as error:
            summary["errors"].append(type(error).__name__ + ": " + str(error))
        finally:
            if process is None and (dest / "python-synthetic.process.json").is_file():
                try:
                    process = support.strict_json(support.read_original(dest / "python-synthetic.process.json",
                                                                       support.MAX_SOURCE)[0])
                except Exception as error:
                    summary["errors"].append("Process receipt: " + str(error))
            summary["actual_python_started"] = process is not None and process.get("pid") is not None
            if child_created or summary["actual_python_started"]:
                try:
                    retained, summary["child_source_after"] = support.read_original(child, support.MAX_SOURCE)
                    support.require(retained == child_raw, "Synthetic child source changed after execution")
                except BaseException as error:
                    summary["errors"].append("Child source: " + type(error).__name__ + ": " + str(error))
            for role, repo, revision, files in (("subject", root, support.EXPECTED_SHA, names),
                                               ("workflow", workflow_root, workflow_sha, support.HARNESS_PATHS)):
                if files is None:
                    continue
                try:
                    after[role] = support.repository_state(metadata, "after", role, repo, revision, files)
                    if role == "subject":
                        support.verify_observed_bindings(after[role], inventory)
                except BaseException as error:
                    after[role] = {"error": type(error).__name__ + ": " + str(error)}
            summary.update(source_before=before.get("subject"), source_after=after.get("subject"),
                           workflow_before=before.get("workflow"), workflow_after=after.get("workflow"), process=process)
            summary["source_files_unchanged"] = "subject" in before and before["subject"] == after.get("subject")
            summary["workflow_files_unchanged"] = "workflow" in before and before["workflow"] == after.get("workflow")
            if process is not None:
                try:
                    raw, raw_receipt = support.read_original(dest / "python-synthetic.stdout.original", support.MAX_LOG)
                    _, err_receipt = support.read_original(dest / "python-synthetic.stderr.original", support.MAX_LOG)
                    support.require(raw_receipt == process.get("stdout") and err_receipt == process.get("stderr"),
                                    "Original synthetic streams changed after process receipt")
                    parsed = parse_python_events(raw, process)
                    summary.update(parsed, started_at=process["started_at"], finished_at=process["finished_at"],
                                   elapsed_seconds=process["elapsed_seconds"], exit_code=process["exit_code"],
                                   timed_out=process["timed_out"], error=process["error"],
                                   raw_sha256=raw_receipt["sha256"], raw_bytes=raw_receipt["bytes"],
                                   stderr_sha256=err_receipt["sha256"], stderr_bytes=err_receipt["bytes"])
                    summary["passed_all"] = (process.get("passed") is True and parsed["passed_all_events"] and
                        parsed["synthetic_subcases_passed"] and summary["source_files_unchanged"] and
                        summary["workflow_files_unchanged"] and not summary["errors"])
                except BaseException as error:
                    summary["errors"].append(type(error).__name__ + ": " + str(error))
            (dest / "summary.json").write_bytes(support.encoded(summary))
    print(json.dumps({key: summary[key] for key in ("actual_python_started", "actual_go_started", "passed_all",
                                                   "started", "passed", "failed", "skipped", "errors")}, ensure_ascii=True))
    return 0 if summary["passed_all"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
