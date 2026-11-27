#!/usr/bin/env python3
"""Synthetic boundary controls; these never constitute compiler or Apple evidence."""
import base64
import copy
import contextlib
import json
import os
from pathlib import Path
import plistlib
import sys
import tempfile
import time
from types import SimpleNamespace
import unittest
from unittest import mock

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import native_contract_fixtures as fixtures
import native_caller_input_observation as subject

TOKEN = "1234567890abcdef1234567890abcdef"
NONCE = "fedcba0987654321fedcba0987654321"


def canonical_exports():
    rows = []
    for case in fixtures.CASES:
        source = b"// Synthetic parser control; not authored or executed native evidence.\n"
        native = plistlib.dumps({"WFWorkflowActions": []})
        rows.append({
            "schema": fixtures.EXPORT_SCHEMA, "requirement": case.requirement, "caseId": case.case_id,
            "originalTest": case.original_test, "originalGraphOracle": case.oracle,
            "compileFilename": case.filename, "observationMode": case.mode,
            "sourceBase64": base64.b64encode(source).decode(), "sourceBytes": len(source),
            "sourceSha256": fixtures.sha(source), "unsignedBase64": base64.b64encode(native).decode(),
            "unsignedSha256": fixtures.sha(native), "unsignedBytes": len(native), "actionCount": 0,
            "slots": [{"name": name, "authoredType": "AnyContent", "present": True, "expectedValue": None}
                      for name in case.slots],
            "requiredEffects": ["omitted:FIRST", "omitted:SECOND"] if case.case_id.startswith("effects/") else [],
            "nativeExecuted": False, "formalAcceptanceGranted": False,
        })
    return rows


def packet(case_id):
    canonical = next(row for row in canonical_exports() if row["requirement"] == "FN04" and row["caseId"] == case_id)
    name = subject.PREFIXES[case_id] + TOKEN
    count = 0 if case_id == "without_functions" else 2 if case_id == "nested_true" else 1
    actions = [{"WFWorkflowActionIdentifier": "is.workflow.actions.runworkflow",
                "WFWorkflowActionParameters": {"WFWorkflowName": name,
                    "WFWorkflow": {"isSelf": True, "workflowName": name}}} for _ in range(count)]
    actions.append({"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
    native = plistlib.dumps({"WFWorkflowActions": actions})
    source = subject.PLAIN_SOURCE if case_id == "without_functions" else base64.b64decode(canonical["sourceBase64"])
    return {
        "schema": subject.PACKET_SCHEMA, "caseId": case_id, "packetToken": TOKEN,
        "workflowName": name, "compileFilename": name + ".cherri", "canonicalTest": canonical["originalTest"],
        "canonicalSourceBase64": canonical["sourceBase64"], "canonicalSourceSha256": canonical["sourceSha256"],
        "sourceBase64": base64.b64encode(source).decode(), "sourceSha256": fixtures.sha(source), "sourceBytes": len(source),
        "unsignedBase64": base64.b64encode(native).decode(), "unsignedSha256": fixtures.sha(native),
        "unsignedBytes": len(native), "actionCount": len(actions),
        "originalGraphOracle": "p15NativeCallerPlainGraph" if case_id == "without_functions" else "p15CallerInputGraphForWorkflow",
        "companionKind": "CAPTURE_ORIGINAL_INPUT_READ_RESULTS" if case_id == "without_functions" else "ORIGINAL_SOURCE_COMPILER_NAME",
        "nativeExecuted": False, "formalAcceptanceGranted": False,
    }


def event(action, test="", **extra):
    return {"Package": fixtures.PACKAGE, "Action": action, **({"Test": test} if test else {}), **extra}


def marker(value):
    raw = json.dumps(value, sort_keys=True, separators=(",", ":")).encode()
    return ("    p15_native_contract_caller_packets_test.go:99: P15_NATIVE_CALLER_PACKET "
            f"sha256={fixtures.sha(raw)} bytes={len(raw)} base64={base64.b64encode(raw).decode()}\n")


def events():
    result = [event("start"), event("run", subject.PACKET_PARENT)]
    for case_id in subject.PREFIXES:
        name = subject.PACKET_PARENT + "/" + case_id
        result.extend([event("run", name), event("output", name, Output=marker(packet(case_id))),
                       event("pass", name, Elapsed=0.01)])
    result.extend([event("pass", subject.PACKET_PARENT, Elapsed=0.1), event("pass", Elapsed=0.2)])
    return result


def encoded(rows):
    return b"".join(json.dumps(row, separators=(",", ":")).encode() + b"\n" for row in rows)


def callback(case_id):
    return {"receiverSelfTest": False, "eventSource": "shortcuts-x-callback", "status": "SUCCESS",
            "runId": NONCE, "fixtureId": "fixture-" + case_id, "expectedShortcutName": "Workflow"}


@contextlib.contextmanager
def control_directory(label):
    retained = os.environ.get("CHERRI_NATIVE_CONTROL_ROOT")
    if retained:
        root = Path(retained)
        if not root.is_absolute() or root.is_symlink() or root.resolve(strict=True) != root:
            raise ValueError("CONTROL_RETENTION_ROOT")
        folder = root / label
        folder.mkdir(mode=0o700)
        yield str(folder)
    else:
        with tempfile.TemporaryDirectory() as name:
            yield name


class NativeCallerInputTests(unittest.TestCase):
    def parse(self, rows):
        raw = encoded(rows)
        return subject.parse_packets(raw, expected_sha256=fixtures.sha(raw),
                                     canonical_exports=canonical_exports(), packet_token=TOKEN)

    def reject(self, rows, code):
        with self.assertRaisesRegex(fixtures.FixtureRejected, "^" + code + "$"):
            self.parse(rows)

    def test_three_named_packets_and_four_events_never_claim_native_types(self):
        values = self.parse(events())
        self.assertEqual([row["caseId"] for row in values], ["nested_false", "nested_true", "without_functions"])
        self.assertEqual(len(subject.PACKET_EVENTS), 4)
        self.assertTrue(all(row["nativeExecuted"] is False and row["formalAcceptanceGranted"] is False for row in values))
        self.assertEqual(subject.PACKET_COMMAND,
            ("go", "test", "-json", "-count=1", "-timeout=3m", "-run", "^TestP15NativeCallerInputPackets$", "."))
        raw = encoded(events())
        with self.assertRaisesRegex(fixtures.FixtureRejected, "CALLER_GO_STREAM_HASH"):
            subject.parse_packets(raw, expected_sha256="0" * 64,
                                  canonical_exports=canonical_exports(), packet_token=TOKEN)

    def test_packet_fragments_cannot_cross_case_or_terminal_boundaries(self):
        rows = events()
        index = next(i for i, row in enumerate(rows) if row["Action"] == "output")
        old = rows[index]
        rows[index:index + 1] = [event("output", old["Test"], Output=old["Output"][:100]),
                                  event("output", old["Test"], Output=old["Output"][100:])]
        self.assertEqual(len(self.parse(rows)), 3)
        rows[index + 1]["Test"] = subject.PACKET_PARENT
        self.reject(rows, "CALLER_INVALID_TEST_PASS")
        rows = events()
        index = next(i for i, row in enumerate(rows) if row["Action"] == "output")
        rows[index], rows[index + 1] = rows[index + 1], rows[index]
        self.reject(rows, "CALLER_MISSING_PACKET")
        rows = events() + [event("output", Output="late\n")]
        self.reject(rows, "CALLER_EVENT_AFTER_PACKAGE_PASS")

    def test_missing_duplicate_failed_skipped_and_early_parent_events_reject(self):
        first = subject.PACKET_PARENT + "/nested_false"
        for kind, code in (("missing", "CALLER_PACKAGE_BEFORE_CASES"),
                           ("duplicate", "CALLER_DUPLICATE_TEST"),
                           ("fail", "CALLER_TEST_NOT_PASSED"), ("skip", "CALLER_TEST_NOT_PASSED"),
                           ("early", "CALLER_CHILD_OUTSIDE_PARENT"), ("package", "CALLER_GO_PACKAGE")):
            with self.subTest(kind=kind):
                rows = events()
                if kind == "missing":
                    rows = [row for row in rows if row.get("Test") != first]
                elif kind == "duplicate":
                    rows.insert(3, event("run", first))
                elif kind in ("fail", "skip"):
                    next(row for row in rows if row.get("Test") == first and row["Action"] == "pass")["Action"] = kind
                elif kind == "early":
                    rows.insert(2, event("pass", subject.PACKET_PARENT))
                else:
                    rows[0]["Package"] = "other/package"
                self.reject(rows, code)

    def test_packet_name_token_canonical_graph_and_scope_bindings_reject(self):
        base = packet("nested_false")
        canonical = next(row for row in canonical_exports() if row["requirement"] == "FN04" and row["caseId"] == "nested_false")
        mutations = (
            ("caseId", "other"), ("packetToken", "0" * 32), ("workflowName", "AnotherWorkflow"),
            ("compileFilename", "other.cherri"), ("canonicalSourceSha256", "0" * 64),
            ("originalGraphOracle", "weaker"), ("companionKind", "relabelled"),
            ("nativeExecuted", True), ("formalAcceptanceGranted", True),
            ("sourceBytes", 1), ("unsignedSha256", "0" * 64),
        )
        for field, value in mutations:
            with self.subTest(field=field):
                changed = copy.deepcopy(base); changed[field] = value
                with self.assertRaises(fixtures.FixtureRejected):
                    subject.validate_packet(changed, canonical)
        raw = encoded(events())
        with self.assertRaisesRegex(fixtures.FixtureRejected, "CALLER_EXPORT_CASE_TOKEN_BINDING"):
            subject.parse_packets(raw, expected_sha256=fixtures.sha(raw),
                                  canonical_exports=canonical_exports(), packet_token="1" * 32)
        with self.assertRaisesRegex(fixtures.FixtureRejected, "ORIGINAL_EXPORT_INVENTORY"):
            subject.parse_packets(raw, expected_sha256=fixtures.sha(raw),
                                  canonical_exports=canonical_exports()[:-1], packet_token=TOKEN)

    def test_whole_unsigned_bytes_and_actual_self_calls_are_bound(self):
        for case_id in ("nested_false", "nested_true"):
            canonical = next(row for row in canonical_exports() if row["requirement"] == "FN04" and row["caseId"] == case_id)
            for fault in ("self_name", "self_flag", "removed", "invalid_document"):
                with self.subTest(case_id=case_id, fault=fault):
                    value = packet(case_id)
                    document = plistlib.loads(base64.b64decode(value["unsignedBase64"]))
                    if fault == "self_name":
                        document["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["WFWorkflowName"] = "Other"
                    elif fault == "self_flag":
                        document["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["WFWorkflow"]["isSelf"] = False
                    elif fault == "removed":
                        document["WFWorkflowActions"].pop(0); value["actionCount"] -= 1
                    else:
                        document = []
                    raw = plistlib.dumps(document)
                    value.update(unsignedBase64=base64.b64encode(raw).decode(), unsignedSha256=fixtures.sha(raw), unsignedBytes=len(raw))
                    with self.assertRaises(fixtures.FixtureRejected):
                        subject.validate_packet(value, canonical)

    def test_exact_original_alias_values_direct_nested_and_plain(self):
        text = "CHERRI-CALLER-" + NONCE
        expected = {
            "nested_false": "|".join([text, text, text, text, "VALUE", text, text]).encode(),
            "nested_true": "|".join([text, text, text, text, text, "VALUE", text, text, text]).encode(),
            "without_functions": (text + "|" + text).encode(),
        }
        for case_id, raw in expected.items():
            with self.subTest(case_id=case_id):
                result = subject.validate_runtime_bytes(callback(case_id), case_id, "fixture-" + case_id,
                                                       "Workflow", NONCE, raw, text)
                self.assertEqual(result["fieldCount"], {"nested_false": 7, "nested_true": 9, "without_functions": 2}[case_id])
                self.assertIs(result["actualCallerInputMatched"], True)
                self.assertIs(result["nativeRawTypeObserved"], False)
                self.assertIs(result["formalAcceptanceGranted"], False)
                self.assertEqual(result["originalCallbackClassification"], "OBSERVED_ONLY")

    def test_stale_callback_changed_alias_and_formatted_output_cannot_pass(self):
        case_id = "without_functions"; text = "CHERRI-CALLER-" + NONCE; raw = (text + "|" + text).encode()
        for field, value in (("runId", "0" * 32), ("fixtureId", "different"),
                             ("expectedShortcutName", "Other"), ("receiverSelfTest", True),
                             ("eventSource", "test-double"), ("status", "ERROR")):
            with self.subTest(field=field):
                event_value = callback(case_id); event_value[field] = value
                with self.assertRaises(fixtures.FixtureRejected):
                    subject.validate_runtime_bytes(event_value, case_id, "fixture-" + case_id, "Workflow", NONCE, raw, text)
        for bad in (raw + b"\n", raw.decode(), raw.replace(b"|", b"\r"), text.encode(), b"[]", b""):
            with self.subTest(raw_type=type(bad).__name__, value=repr(bad)[:30]):
                with self.assertRaises(fixtures.FixtureRejected):
                    subject.validate_runtime_bytes(callback(case_id), case_id, "fixture-" + case_id, "Workflow", NONCE, bad, text)
        with self.assertRaisesRegex(fixtures.FixtureRejected, "CALLER_INPUT_RUN_BINDING"):
            subject.validate_runtime_bytes(callback(case_id), case_id, "fixture-" + case_id, "Workflow", NONCE,
                                           raw, "CHERRI-CALLER-" + "0" * 32)

    def library_rows(self):
        dispatch = {"completedUTC": "2026-11-27T00:00:00+00:00"}
        ui = {"importTiming": {"libraryMatchedAfterCaptureUTC": "2026-11-27T00:00:03+00:00"}}
        rows = [{"schema": "cherri-current-library-selection-observation/1", "phase": "import-test",
                 "expectedName": "Workflow", "foreground": True, "dispatchConfirmed": True,
                 "canConfirm": True, "currentNamedCellCount": 1, "ordinal": i,
                 "observedUTC": "2026-11-27T00:00:0" + str(i) + "+00:00",
                 "deadlineUTC": "2026-11-27T00:00:30+00:00"} for i in (1, 2)]
        return ui, dispatch, rows

    def test_two_fresh_library_rows_reject_stale_ambiguous_and_cross_context(self):
        ui, dispatch, rows = self.library_rows()
        result = subject.validate_library_rows(ui, dispatch, rows, phase="import-test", name="Workflow")
        self.assertEqual(result["ordinals"], [1, 2])
        for field, value in (("ordinal", 1), ("expectedName", "Other"), ("phase", "old"),
                             ("currentNamedCellCount", 2), ("canConfirm", False),
                             ("foreground", False), ("dispatchConfirmed", False),
                             ("observedUTC", "2026-11-26T23:59:59+00:00")):
            with self.subTest(field=field):
                bad = copy.deepcopy(rows); bad[1][field] = value
                with self.assertRaises((fixtures.FixtureRejected, ValueError)):
                    subject.validate_library_rows(ui, dispatch, bad, phase="import-test", name="Workflow")

    def test_actual_owned_dispatch_retains_success_and_nonzero_process_originals(self):
        for executable, passes in (("/bin/true", True), ("/bin/false", False)):
            with self.subTest(executable=executable), control_directory("dispatch-" + Path(executable).name) as name:
                owner = SimpleNamespace(folder=Path(name), deadline=time.monotonic() + 70)
                if passes:
                    result = subject.dispatch_ui(owner, "phase", "runtime", "Workflow", [executable])
                    self.assertEqual(result["outcome"], "EXITED")
                    self.assertIs(result["parentReaped"], True)
                else:
                    with self.assertRaisesRegex(fixtures.FixtureRejected, "OWNED_NATIVE_DISPATCH_PROCESS"):
                        subject.dispatch_ui(owner, "phase", "runtime", "Workflow", [executable])
                observed = json.loads((owner.folder / "dispatch-phase/result.original.json").read_bytes())
                self.assertIs(observed["details"]["cleanupParentReaped"], True)
                self.assertIs(observed["details"]["groupPresentAfterCleanup"], False)
                self.assertEqual(observed["observation"]["exitCode"], 0 if passes else 1)
                self.assertTrue((owner.folder / "dispatch-phase/started.original.json").is_file())
                self.assertTrue((owner.folder / "dispatch-phase/finished.original.json").is_file())

    def test_interruption_after_command_publication_keeps_owned_cleanup_and_original_failure(self):
        class InjectedCancellation(KeyboardInterrupt):
            pass
        with control_directory("publication-cancellation") as name:
            owner = SimpleNamespace(folder=Path(name), deadline=time.monotonic() + 70)
            real = subject.differential.atomic_json
            def publish(path, value):
                real(path, value)
                if path.name == "command.json":
                    raise InjectedCancellation("synthetic post-publication cancellation")
            with mock.patch.object(subject.differential, "atomic_json", side_effect=publish):
                with self.assertRaises(InjectedCancellation):
                    subject.dispatch_ui(owner, "phase", "runtime", "Workflow", ["/bin/sleep", "30"])
            result = json.loads((owner.folder / "dispatch-phase/result.original.json").read_bytes())
            self.assertEqual(result["interruptionType"], "InjectedCancellation")
            self.assertIsNone(result["observation"])
            self.assertIs(result["details"]["cleanupParentReaped"], True)
            self.assertIs(result["details"]["groupPresentAfterCleanup"], False)
            self.assertEqual(result["details"]["returnCodeObserved"], -9)
            self.assertEqual(json.loads((owner.folder / "dispatch-phase.json").read_bytes())["outcome"], "INTERRUPTED")

    def test_incomplete_execution_and_cleanup_budget_cannot_spawn(self):
        with control_directory("budget-prevents-spawn") as name:
            owner = SimpleNamespace(folder=Path(name), deadline=time.monotonic() + 1)
            with mock.patch.object(subject.process_capture, "capture") as capture:
                with self.assertRaisesRegex(fixtures.FixtureRejected, "DISPATCH_EXECUTION_AND_CLEANUP_BUDGET"):
                    subject.dispatch_ui(owner, "phase", "runtime", "Workflow", ["/bin/sleep", "30"])
                capture.assert_not_called()
            self.assertEqual(list(owner.folder.iterdir()), [])

    def test_non_darwin_cannot_enter_any_import_or_runtime(self):
        with mock.patch.object(subject.sys, "platform", "linux"), mock.patch.object(subject, "imported_unique") as imported:
            with self.assertRaisesRegex(fixtures.FixtureRejected, "DARWIN_SHARED_MONOTONIC_CLOCK"):
                subject.observe_caller_input(None, None, None, None, None, None)
            imported.assert_not_called()


if __name__ == "__main__":
    unittest.main(verbosity=2)
