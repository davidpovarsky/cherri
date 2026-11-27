"""Causal parser controls. All envelopes in this module are synthetic.

Actual compiler exports and their process/source provenance are qualified in a
separate owner run; these controls cannot become Apple/runtime evidence.
"""
from __future__ import annotations

import base64
import copy
import json
from pathlib import Path
import plistlib
import re
import unittest

import native_loop_contract_fixtures as fixtures


def encode(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":")).encode()


def packet(case):
    source_file = Path(__file__).resolve().parents[2] / "p15_loop_identity_acceptance_test.go"
    match = re.search(r"const p15" + case.requirement + r"FullLoopSource = " + chr(96) + r"([\s\S]*?)" + chr(96), source_file.read_text())
    if match is None:
        raise AssertionError("original source factory literal absent")
    source = match[1].encode()
    workflow = {
        "WFWorkflowName": "P15_SYNTHETIC_LOOP_CONTROL_" + case.requirement,
        "WFWorkflowActions": [{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"}],
    }
    raw = plistlib.dumps(workflow, fmt=plistlib.FMT_XML)
    return {
        "schema": fixtures.EXPORT_SCHEMA, "requirement": case.requirement,
        "caseId": "full-original-scenario", "originalUnitTest": case.unit_test,
        "originalNativeTest": case.native_test, "originalGraphOracle": "p15LoopNativeError",
        "compileFilename": "p15-short-circuit.cherri", "sourceBase64": base64.b64encode(source).decode(),
        "sourceSha256": fixtures.sha(source), "sourceBytes": len(source),
        "unsignedBase64": base64.b64encode(raw).decode(), "unsignedSha256": fixtures.sha(raw),
        "unsignedBytes": len(raw), "actionCount": 1, "workflowName": workflow["WFWorkflowName"],
        "workflowNamePresent": True, "observationMode": "ordered-original-show-result-sequence",
        "expectedDisplays": list(case.displays), "nativeExecuted": False,
        "formalAcceptanceGranted": False,
    }


def marker(value):
    raw = encode(value)
    return ("    p15_native_loop_contract_export_test.go:100: P15_NATIVE_LOOP_FIXTURE "
            f"sha256={fixtures.sha(raw)} bytes={len(raw)} base64={base64.b64encode(raw).decode()}\n")


def event(action, test="", **extra):
    row = {"Time": "2026-11-27T00:00:00Z", "Package": fixtures.PACKAGE, "Action": action}
    if test:
        row["Test"] = test
    row.update(extra)
    return row


def events():
    rows = [event("start")]
    for case in fixtures.CASES.values():
        names = fixtures.original_events(case)
        for parent in (case.unit_test, case.native_test):
            rows.append(event("run", parent))
            for name in names:
                if name.startswith(parent + "/"):
                    rows.extend([event("run", name), event("pass", name, Elapsed=0.001)])
            rows.append(event("pass", parent, Elapsed=0.01))
    rows.append(event("run", fixtures.EXPORT_PARENT))
    for case in fixtures.CASES.values():
        rows.extend([
            event("run", case.export_test),
            event("output", case.export_test, Output=marker(packet(case))),
            event("pass", case.export_test, Elapsed=0.01),
        ])
    rows.extend([event("pass", fixtures.EXPORT_PARENT, Elapsed=0.1), event("pass", Elapsed=0.2)])
    return rows


def stream(rows):
    return b"".join(encode(row) + b"\n" for row in rows)


class NativeLoopFixtureTests(unittest.TestCase):
    def parsed(self, rows):
        raw = stream(rows)
        return fixtures.parse_exports(raw, expected_sha256=fixtures.sha(raw))

    def rejected(self, rows, diagnostic=None):
        raw = stream(rows)
        with self.assertRaisesRegex(fixtures.FixtureRejected, diagnostic or "."):
            fixtures.parse_exports(raw, expected_sha256=fixtures.sha(raw))

    def changed_packet(self, mutate):
        rows = events()
        test = fixtures.CASES["F02"].export_test
        value = packet(fixtures.CASES["F02"])
        mutate(value)
        next(row for row in rows if row.get("Test") == test and row["Action"] == "output")["Output"] = marker(value)
        return rows

    def test_complete_original_inventory_and_exports(self):
        self.assertEqual(len(fixtures.REQUIRED_EVENTS), 86)
        self.assertEqual([len(fixtures.original_events(c)) for c in fixtures.CASES.values()], [26, 28, 28])
        result = self.parsed(events())
        self.assertEqual([row["requirement"] for row in result], ["F02", "RP22", "BRG12"])
        self.assertEqual([len(row["expectedDisplays"]) for row in result], [8, 21, 22])
        self.assertTrue(all(row["nativeExecuted"] is False and row["formalAcceptanceGranted"] is False for row in result))

    def test_original_scenario_cannot_be_omitted(self):
        for case in fixtures.CASES.values():
            for parent in (case.unit_test, case.native_test):
                with self.subTest(requirement=case.requirement, parent=parent):
                    rows = [row for row in events() if row.get("Test", "").split("/")[0] != parent]
                    self.rejected(rows, "PACKAGE_PASS_BEFORE_CASES")

    def test_required_semantic_and_causal_children_cannot_be_omitted(self):
        children = (
            fixtures.CASES["F02"].unit_test + "/scoped_types",
            fixtures.CASES["F02"].native_test + "/wrong_index_owner",
            fixtures.CASES["RP22"].native_test + "/shadow_leaks_after_scope",
            fixtures.CASES["BRG12"].native_test + "/post_inner_owner_lost",
        )
        for child in children:
            with self.subTest(child=child):
                self.rejected([row for row in events() if row.get("Test") != child], "PACKAGE_PASS_BEFORE_CASES")

    def test_missing_duplicate_or_wrongly_attributed_export(self):
        for case in fixtures.CASES.values():
            original = events()
            at = next(i for i, row in enumerate(original) if row.get("Test") == case.export_test and row["Action"] == "output")
            with self.subTest(requirement=case.requirement, fault="missing"):
                self.rejected(original[:at] + original[at + 1:], "MISSING_FIXTURE_EXPORT")
            with self.subTest(requirement=case.requirement, fault="duplicate"):
                self.rejected(original[:at] + [copy.deepcopy(original[at])] + original[at:], "EXPORT_MARKER_BINDING")
            with self.subTest(requirement=case.requirement, fault="wrong_case"):
                wrong = copy.deepcopy(original)
                replacement = packet(next(c for c in fixtures.CASES.values() if c != case))
                wrong[at]["Output"] = marker(replacement)
                self.rejected(wrong, "EXPORT_CASE_BINDING")

    def test_split_go_output_retains_one_full_marker(self):
        rows = events()
        at = next(i for i, row in enumerate(rows) if row["Action"] == "output")
        output = rows[at]["Output"]
        for cut in (1, 31, output.index("base64=") + 7, len(output) - 1):
            with self.subTest(cut=cut):
                parts = [dict(rows[at], Output=output[:cut]), dict(rows[at], Output=output[cut:])]
                self.assertEqual(len(self.parsed(rows[:at] + parts + rows[at + 1:])), 3)

    def test_fail_skip_extra_duplicate_and_late_events_reject(self):
        for action in ("fail", "skip"):
            with self.subTest(fault=action):
                rows = events()
                next(row for row in rows if row.get("Test", "").endswith("/scoped_types") and row["Action"] == "pass")["Action"] = action
                self.rejected(rows, "TEST_NOT_PASSED")
        rows = events()
        self.rejected(rows[:1] + [event("run", "TestUnrelatedPoC")] + rows[1:], "UNEXPECTED_TEST")
        self.rejected(rows[:2] + [copy.deepcopy(rows[1])] + rows[2:], "DUPLICATE_TEST")
        self.rejected(rows + [event("output", Output="late\n")], "EVENT_AFTER_PACKAGE_PASS")
        self.rejected([rows[0], rows[-1]], "PACKAGE_PASS_BEFORE_CASES")

    def test_parent_and_output_lifecycle_rejects(self):
        rows = events()
        self.rejected([rows[0]] + rows[2:], "CHILD_OUTSIDE_PARENT")
        case = fixtures.CASES["F02"]
        final_at = next(i for i, r in enumerate(rows) if r.get("Test") == case.unit_test and r["Action"] == "pass")
        self.rejected(rows[:final_at + 1] + [event("output", case.unit_test, Output="late\n")] + rows[final_at + 1:], "OUTPUT_OUTSIDE_TEST")
        self.rejected([event("start"), event("output", Output=marker(packet(case)))], "UNBOUND_PACKAGE_OUTPUT")

    def test_source_and_byte_identity_rejects_reconciled_changes(self):
        def changed_source(value):
            raw = base64.b64decode(value["sourceBase64"]) + b"\nshow(99)\n"
            value.update(sourceBase64=base64.b64encode(raw).decode(), sourceBytes=len(raw), sourceSha256=fixtures.sha(raw))
        self.rejected(self.changed_packet(changed_source), "EXPORT_CASE_BINDING")
        for key, value in (("unsignedSha256", "0" * 64), ("unsignedBytes", 1), ("unsignedBase64", "!"), ("sourceBytes", True)):
            with self.subTest(key=key):
                self.rejected(self.changed_packet(lambda packet: packet.__setitem__(key, value)))

    def test_display_sequence_and_runtime_overclaims_reject(self):
        for key, value in (
            ("nativeExecuted", True), ("formalAcceptanceGranted", True),
            ("nativeExecuted", 0), ("expectedDisplays", ["F02_REPEAT:0"]),
        ):
            with self.subTest(key=key, value=value):
                self.rejected(self.changed_packet(lambda packet: packet.__setitem__(key, value)))
        self.rejected(self.changed_packet(lambda p: p["expectedDisplays"].__setitem__(3, p["expectedDisplays"][4])), "EXPORT_CASE_BINDING")

    def test_native_envelope_name_and_void_boundary_reject(self):
        for fault in ("count", "name", "name_presence", "void", "actions", "plist"):
            with self.subTest(fault=fault):
                def mutate(value):
                    if fault == "count":
                        value["actionCount"] = 2
                        return
                    if fault == "name":
                        value["workflowName"] = "wrong"
                        return
                    if fault == "name_presence":
                        value["workflowNamePresent"] = False
                        return
                    doc = plistlib.loads(base64.b64decode(value["unsignedBase64"]))
                    if fault == "void":
                        doc["WFWorkflowActions"][-1]["WFWorkflowActionIdentifier"] = "is.workflow.actions.showresult"
                    if fault == "actions":
                        doc["WFWorkflowActions"] = "wrong"
                    raw = b"not a plist" if fault == "plist" else plistlib.dumps(doc)
                    value.update(unsignedBase64=base64.b64encode(raw).decode(), unsignedBytes=len(raw), unsignedSha256=fixtures.sha(raw))
                self.rejected(self.changed_packet(mutate))

    def test_framing_schema_and_json_reject(self):
        raw = stream(events())
        for changed, digest in ((raw[:-1], fixtures.sha(raw[:-1])), (raw, "0" * 64), (b"\n" + raw, fixtures.sha(b"\n" + raw))):
            with self.subTest(length=len(changed)):
                with self.assertRaises(fixtures.FixtureRejected):
                    fixtures.parse_exports(changed, expected_sha256=digest)
        for changed in (b'{"Package":"x","Package":"y"}\n', b'{"value":NaN}\n', b"\xff\n"):
            with self.subTest(raw=changed):
                with self.assertRaises(fixtures.FixtureRejected):
                    fixtures.parse_exports(changed, expected_sha256=fixtures.sha(changed))
        self.rejected(self.changed_packet(lambda p: p.__setitem__("extra", "unreviewed")), "EXPORT_SCHEMA")
        self.rejected(self.changed_packet(lambda p: p.__setitem__("requirement", "FN04")), "EXPORT_CASE_BINDING")


if __name__ == "__main__":
    unittest.main()
