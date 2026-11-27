"""Synthetic flow-parser controls; no compiler or Apple execution evidence."""
from __future__ import annotations

import base64
import copy
import json
from pathlib import Path
import plistlib
import unittest

import native_flow_contract_fixtures as fixtures


def encode(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":")).encode()


def packet(case):
    source = (Path(__file__).resolve().parents[2] / case.source_path).read_bytes()
    # Deliberately synthetic: actual graph proof belongs to the genuine Go owner.
    workflow = {"WFWorkflowActions": [{"WFWorkflowActionIdentifier": "is.workflow.actions.output"}]}
    raw = plistlib.dumps(workflow)
    return {
        "schema": fixtures.EXPORT_SCHEMA, "requirement": case.requirement,
        "caseId": "full-original-scenario", "originalUnitTest": case.unit_test,
        "originalNativeTest": case.native_test, "originalGraphOracle": case.graph_oracle,
        "sourcePath": case.source_path, "compileFilename": case.compile_filename,
        "sourceBase64": base64.b64encode(source).decode(), "sourceSha256": fixtures.sha(source),
        "sourceBytes": len(source), "unsignedBase64": base64.b64encode(raw).decode(),
        "unsignedSha256": fixtures.sha(raw), "unsignedBytes": len(raw), "actionCount": 1,
        "workflowName": "", "workflowNamePresent": False,
        "observationMode": "exact-original-returned-text", "expectedType": "Text",
        "expectedText": case.expected_text, "nativeExecuted": False,
        "formalAcceptanceGranted": False,
    }


def marker(value):
    raw = encode(value)
    return ("    p15_native_flow_contract_export_test.go:100: P15_NATIVE_FLOW_FIXTURE "
            f"sha256={fixtures.sha(raw)} bytes={len(raw)} base64={base64.b64encode(raw).decode()}\n")


def event(action, test="", **extra):
    row = {"Package": fixtures.PACKAGE, "Action": action}
    if test:
        row["Test"] = test
    return dict(row, **extra)


def events():
    rows = [event("start")]
    children = {}
    for name in fixtures.REQUIRED_EVENTS:
        ancestors = [p for p in fixtures.REQUIRED_EVENTS if name.startswith(p + "/")]
        if ancestors:
            children.setdefault(max(ancestors, key=len), []).append(name)
    def visit(name):
        rows.append(event("run", name))
        for child in sorted(children.get(name, [])):
            visit(child)
        if name in fixtures.CASE_BY_TEST:
            rows.append(event("output", name, Output=marker(packet(fixtures.CASE_BY_TEST[name]))))
        rows.append(event("pass", name))
    for parent in (*fixtures.ORIGINAL_PARENTS, fixtures.EXPORT_PARENT):
        visit(parent)
    return rows + [event("pass")]


def stream(rows):
    return b"".join(encode(row) + b"\n" for row in rows)


class NativeFlowFixtureTests(unittest.TestCase):
    def parsed(self, rows):
        raw = stream(rows)
        return fixtures.parse_exports(raw, expected_sha256=fixtures.sha(raw))

    def rejected(self, rows, diagnostic=None):
        with self.assertRaisesRegex(fixtures.FixtureRejected, diagnostic or "."):
            self.parsed(rows)

    def changed(self, case, mutate):
        rows = events()
        value = packet(case)
        mutate(value)
        next(row for row in rows if row.get("Test") == case.export_test and row["Action"] == "output")["Output"] = marker(value)
        return rows

    def test_full_original_inventory_and_three_return_contracts(self):
        self.assertEqual([len(fixtures.ORIGINAL_EVENTS[k]) for k in fixtures.CASES], [40, 96, 139])
        self.assertEqual(len(fixtures.REQUIRED_EVENTS), 279)
        result = self.parsed(events())
        self.assertEqual([r["requirement"] for r in result], ["RP24", "RP18", "BRG13"])
        self.assertEqual([r["expectedText"] for r in result], [c.expected_text for c in fixtures.CASES.values()])
        self.assertTrue(all(r["expectedType"] == "Text" and r["nativeExecuted"] is False and r["formalAcceptanceGranted"] is False for r in result))

    def test_no_original_family_or_causal_child_can_be_omitted(self):
        for parent in fixtures.ORIGINAL_PARENTS:
            with self.subTest(parent=parent):
                self.rejected([r for r in events() if r.get("Test", "").split("/")[0] != parent], "PACKAGE_PASS_BEFORE_CASES")
        for name in (
            fixtures.CASES["RP24"].native_test + "/oracle/recursive_self_target",
            fixtures.CASES["RP18"].native_test + "/mixedAnd/outer_guard",
            fixtures.CASES["BRG13"].native_test + "/textCaseEqual/right_producer",
        ):
            with self.subTest(child=name):
                self.rejected([r for r in events() if r.get("Test") != name], "PACKAGE_PASS_BEFORE_CASES")

    def test_original_source_and_contract_cannot_be_reconciled(self):
        for case in fixtures.CASES.values():
            for key, value in (
                ("compileFilename", "replacement.cherri"), ("originalGraphOracle", "partialGraph"),
                ("expectedText", "PASS"), ("expectedType", "Number"), ("sourcePath", "other.cherri"),
            ):
                with self.subTest(case=case.requirement, key=key):
                    self.rejected(self.changed(case, lambda p: p.__setitem__(key, value)), "EXPORT_CASE_BINDING")
            with self.subTest(case=case.requirement, key="reconciled_source"):
                def change_source(value):
                    source = base64.b64decode(value["sourceBase64"]) + b"\nreturn \"PASS\"\n"
                    value.update(sourceBase64=base64.b64encode(source).decode(), sourceSha256=fixtures.sha(source), sourceBytes=len(source))
                self.rejected(self.changed(case, change_source), "EXPORT_CASE_BINDING")

    def test_export_bytes_name_and_count_are_bound(self):
        case = fixtures.CASES["RP24"]
        for key, value in (
            ("unsignedSha256", "0" * 64), ("unsignedBytes", True), ("unsignedBase64", "!"),
            ("workflowNamePresent", True), ("workflowName", "invented"), ("actionCount", 2),
        ):
            with self.subTest(key=key):
                self.rejected(self.changed(case, lambda p: p.__setitem__(key, value)))
        def bad_plist(value):
            raw = b"not a workflow"
            value.update(unsignedBase64=base64.b64encode(raw).decode(), unsignedBytes=len(raw), unsignedSha256=fixtures.sha(raw))
        self.rejected(self.changed(case, bad_plist), "INVALID_ORIGINAL_WORKFLOW")

    def test_fixture_claims_remain_non_native_and_exact_schema(self):
        case = fixtures.CASES["BRG13"]
        for key, value in (
            ("nativeExecuted", True), ("nativeExecuted", 0),
            ("formalAcceptanceGranted", True), ("formalAcceptanceGranted", 0),
        ):
            with self.subTest(key=key, value=value):
                self.rejected(self.changed(case, lambda p: p.__setitem__(key, value)), "EXPORT_OVERCLAIM")
        self.rejected(self.changed(case, lambda p: p.__setitem__("unreviewed", True)), "EXPORT_SCHEMA")

    def test_each_fixture_must_occur_once_under_its_own_test(self):
        for case in fixtures.CASES.values():
            rows = events()
            at = next(i for i, row in enumerate(rows) if row.get("Test") == case.export_test and row["Action"] == "output")
            with self.subTest(case=case.requirement, fault="missing"):
                self.rejected(rows[:at] + rows[at + 1:], "MISSING_FIXTURE_EXPORT")
            with self.subTest(case=case.requirement, fault="duplicate"):
                self.rejected(rows[:at] + [copy.deepcopy(rows[at])] + rows[at:], "EXPORT_MARKER_BINDING")
            with self.subTest(case=case.requirement, fault="misbound"):
                other = next(c for c in fixtures.CASES.values() if c != case)
                rows[at]["Output"] = marker(packet(other))
                self.rejected(rows, "EXPORT_CASE_BINDING")


if __name__ == "__main__":
    unittest.main()
