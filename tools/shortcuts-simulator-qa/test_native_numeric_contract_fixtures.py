"""Synthetic protocol controls, not compiled specimens or native observations."""
from __future__ import annotations

import base64
import copy
import json
import plistlib
import unicodedata
import unittest

import native_numeric_contract_fixtures as fixtures


def encode(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode()


def packet(case):
    raw = plistlib.dumps({"WFWorkflowActions": [{"WFWorkflowActionIdentifier": "is.workflow.actions.output"}]})
    return {
        "schema": fixtures.EXPORT_SCHEMA, "requirement": case.requirement, "caseId": case.name,
        "originalUnitTest": case.unit_test, "originalNativeTest": case.native_test,
        "originalGraphOracle": case.graph_oracle, "sourcePath": case.source_path,
        "sourceFileSha256": case.source_file_sha256, "compileFilename": case.compile_filename,
        "sourceBase64": base64.b64encode(case.source).decode(), "sourceSha256": fixtures.sha(case.source),
        "sourceBytes": len(case.source), "unsignedBase64": base64.b64encode(raw).decode(),
        "unsignedSha256": fixtures.sha(raw), "unsignedBytes": len(raw), "actionCount": 1,
        "workflowName": "", "workflowNamePresent": False,
        "observationMode": "original-typed-output-and-side-effects",
        "expectedObservations": copy.deepcopy(case.observations), "nativeExecuted": False,
        "formalAcceptanceGranted": False,
    }


def marker(value):
    raw = encode(value)
    return ("    p15_native_numeric_contract_export_test.go:100: P15_NATIVE_NUMERIC_FIXTURE "
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


def case(requirement, name=None):
    return next(c for c in fixtures.CASES if c.requirement == requirement and (name is None or c.name == name))


class NativeNumericFixtureTests(unittest.TestCase):
    def parsed(self, rows):
        raw = stream(rows)
        return fixtures.parse_exports(raw, expected_sha256=fixtures.sha(raw))

    def rejected(self, rows, diagnostic=None):
        with self.assertRaisesRegex(fixtures.FixtureRejected, diagnostic or "."):
            self.parsed(rows)

    def changed(self, chosen, mutate):
        rows = events()
        value = packet(chosen)
        mutate(value)
        next(r for r in rows if r.get("Test") == chosen.export_test and r["Action"] == "output")["Output"] = marker(value)
        return rows

    def test_all_original_sources_events_and_typed_boundaries(self):
        self.assertEqual({k: len(v) for k, v in fixtures.ORIGINAL_EVENTS.items()},
            {"RP15": 80, "RP17": 57, "RP21": 234, "BRG16": 213, "RP19": 46, "BRG08": 61})
        self.assertEqual(len(fixtures.REQUIRED_EVENTS), 729)
        result = self.parsed(events())
        self.assertEqual(len(result), 37)
        self.assertEqual([(r["requirement"], r["caseId"]) for r in result],
            [(c.requirement, c.name) for c in fixtures.CASES])
        self.assertEqual(case("RP21", "dynamic_empty") .observations["return"],
            {"kind": "None", "presence": "absent", "items": "0"})
        self.assertEqual(case("RP21", "dynamic_empty_text").observations["return"],
            {"kind": "Text", "presence": "one-item", "value": ""})
        self.assertEqual(case("BRG16", "static_integral_real").observations["clipboard"]["value"], "2")
        self.assertTrue(all(r["nativeExecuted"] is False and r["formalAcceptanceGranted"] is False for r in result))

    def test_original_families_cannot_be_omitted(self):
        for parent in fixtures.ORIGINAL_PARENTS:
            with self.subTest(parent=parent):
                self.rejected([r for r in events() if r.get("Test", "").split("/")[0] != parent], "PACKAGE_PASS_BEFORE_CASES")

    def test_reconciled_source_changes_and_wrong_locations_reject(self):
        for requirement in ("RP15", "RP17", "RP21", "BRG16", "RP19", "BRG08"):
            chosen = case(requirement)
            with self.subTest(requirement=requirement):
                def change(value):
                    raw = chosen.source + b"\n"
                    value.update(sourceBase64=base64.b64encode(raw).decode(), sourceSha256=fixtures.sha(raw), sourceBytes=len(raw))
                self.rejected(self.changed(chosen, change), "EXPORT_CASE_BINDING")
        chosen = case("RP21", "static_last")
        self.rejected(self.changed(chosen, lambda p: p.__setitem__("sourcePath", case("RP21", "static_first").source_path)), "EXPORT_CASE_BINDING")
        self.rejected(self.changed(chosen, lambda p: p.__setitem__("sourceFileSha256", "0" * 64)), "EXPORT_CASE_BINDING")

    def test_effects_arithmetic_absence_and_empty_text_contracts_cannot_change(self):
        controls = [
            (case("RP15"), lambda p: p["expectedObservations"]["clipboard"].__setitem__("value", "41243")),
            (case("RP17"), lambda p: p["expectedObservations"]["intermediate"]["divided"].__setitem__("value", "2")),
            (case("RP21", "dynamic_empty"), lambda p: p["expectedObservations"].__setitem__("return", {"kind": "Text", "presence": "one-item", "value": ""})),
            (case("RP21", "dynamic_empty_text"), lambda p: p["expectedObservations"].__setitem__("return", {"kind": "None", "presence": "absent", "items": "0"})),
            (case("BRG16", "dynamic_fraction"), lambda p: p["expectedObservations"]["index"].__setitem__("lookup", "required")),
            (case("BRG16", "dynamic_fraction"), lambda p: p["expectedObservations"].__setitem__("absenceAndEmptyTextAreDistinct", 1)),
        ]
        for chosen, mutate in controls:
            with self.subTest(requirement=chosen.requirement, case=chosen.name):
                self.rejected(self.changed(chosen, mutate), "EXPORT_CASE_BINDING")

    def test_exact_unicode_rendering_contract_rejects_normalization(self):
        for requirement in ("RP19", "BRG08"):
            chosen = case(requirement)
            original = chosen.observations["return"]["value"]
            normalized = unicodedata.normalize("NFC", original)
            self.assertNotEqual(original, normalized)
            with self.subTest(requirement=requirement):
                self.rejected(self.changed(chosen, lambda p: p["expectedObservations"]["return"].__setitem__("value", normalized)), "EXPORT_CASE_BINDING")

    def test_exports_cannot_be_omitted_duplicated_or_cross_bound(self):
        selected = [case("RP15"), case("RP19"), case("RP21", "dynamic_empty"), case("RP21", "dynamic_empty_text")]
        for chosen in selected:
            rows = events()
            at = next(i for i, r in enumerate(rows) if r.get("Test") == chosen.export_test and r["Action"] == "output")
            with self.subTest(test=chosen.export_test, fault="missing"):
                self.rejected(rows[:at] + rows[at + 1:], "MISSING_FIXTURE_EXPORT")
            with self.subTest(test=chosen.export_test, fault="duplicate"):
                self.rejected(rows[:at] + [copy.deepcopy(rows[at])] + rows[at:], "EXPORT_MARKER_BINDING")
            with self.subTest(test=chosen.export_test, fault="misbound"):
                other = case("RP21", "dynamic_empty_text") if chosen != case("RP21", "dynamic_empty_text") else case("RP21", "dynamic_empty")
                rows[at]["Output"] = marker(packet(other))
                self.rejected(rows, "EXPORT_CASE_BINDING")

    def test_byte_binding_native_overclaims_and_extra_schema_reject(self):
        chosen = case("RP17")
        for key, value in (
            ("unsignedSha256", "0" * 64), ("unsignedBytes", True), ("actionCount", 2),
            ("nativeExecuted", True), ("formalAcceptanceGranted", 0), ("workflowNamePresent", True),
        ):
            with self.subTest(key=key):
                self.rejected(self.changed(chosen, lambda p: p.__setitem__(key, value)))
        self.rejected(self.changed(chosen, lambda p: p.__setitem__("extra", "unreviewed")), "EXPORT_SCHEMA")


if __name__ == "__main__":
    unittest.main()
