#!/usr/bin/env python3
"""Causal parser controls. Synthetic streams here are never native evidence."""
import base64
import copy
import importlib.util
import json
import pathlib
import plistlib
import sys
import unittest

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("native_contract_fixtures", HERE / "native_contract_fixtures.py")
subject = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = subject
spec.loader.exec_module(subject)


def fixture(case):
    source = b"// synthetic parser control, not a compiler or Apple observation\n"
    native = plistlib.dumps({"WFWorkflowActions": []}, fmt=plistlib.FMT_XML)
    return {
        "schema": subject.EXPORT_SCHEMA,
        "requirement": case.requirement, "caseId": case.case_id,
        "originalTest": case.original_test, "originalGraphOracle": case.oracle,
        "compileFilename": case.filename, "observationMode": case.mode,
        "sourceBase64": base64.b64encode(source).decode(),
        "sourceSha256": subject.sha(source), "sourceBytes": len(source),
        "unsignedBase64": base64.b64encode(native).decode(),
        "unsignedSha256": subject.sha(native), "unsignedBytes": len(native),
        "actionCount": 0,
        "slots": [{"name": name, "authoredType": "AnyContent", "present": True,
                   "expectedValue": None} for name in case.slots],
        "requiredEffects": ["omitted:FIRST", "omitted:SECOND"] if case.case_id.startswith("effects/") else [],
        "nativeExecuted": False, "formalAcceptanceGranted": False,
    }


def event(action, test="", **fields):
    return {"Action": action, "Package": subject.PACKAGE, **({"Test": test} if test else {}), **fields}


def export_line(value):
    raw = json.dumps(value, sort_keys=True, separators=(",", ":")).encode()
    return ("    p15_native_contract_runtime_export_test.go:101: P15_NATIVE_CONTRACT_FIXTURE "
            f"sha256={subject.sha(raw)} bytes={len(raw)} base64={base64.b64encode(raw).decode()}\n")


def events():
    result = [event("start"), event("run", subject.EXPORT_PARENT)]
    for requirement in ("T01", "FN04", "M02"):
        parent = subject.EXPORT_PARENT + "/" + requirement
        result.append(event("run", parent))
        for case in subject.CASES:
            if case.requirement != requirement:
                continue
            result.extend([
                event("run", case.test_id),
                event("output", case.test_id, Output=export_line(fixture(case))),
                event("pass", case.test_id, Elapsed=0.01),
            ])
        result.append(event("pass", parent, Elapsed=0.1))
    result.extend([event("pass", subject.EXPORT_PARENT, Elapsed=0.3), event("pass", Elapsed=0.4)])
    return result


def encoded(rows):
    return b"".join(json.dumps(row, separators=(",", ":")).encode() + b"\n" for row in rows)


class NativeContractFixtureTests(unittest.TestCase):
    def parse(self, rows):
        raw = encoded(rows)
        return subject.parse_exports(raw, expected_sha256=subject.sha(raw))

    def rejects(self, rows, reason):
        with self.assertRaisesRegex(subject.FixtureRejected, "^" + reason + "$"):
            self.parse(rows)

    def test_exact_declared_cases_and_actual_parent_inventory(self):
        result = self.parse(events())
        self.assertEqual(len(result), 53)
        self.assertEqual(len(subject.REQUIRED_EVENTS), 57)
        self.assertEqual({r: sum(x["requirement"] == r for x in result) for r in ("T01", "FN04", "M02")},
                         {"T01": 23, "FN04": 3, "M02": 27})
        self.assertTrue(all(x["nativeExecuted"] is False and x["formalAcceptanceGranted"] is False for x in result))

    def test_test2json_fragmented_export_is_reassembled_only_in_same_test(self):
        rows = events()
        index = next(i for i, x in enumerate(rows) if x["Action"] == "output")
        old = rows[index]
        value = old["Output"]
        rows[index:index + 1] = [
            event("output", old["Test"], Output=value[:17]),
            event("output", old["Test"], Output=value[17:301]),
            event("output", old["Test"], Output=value[301:]),
        ]
        self.assertEqual(len(self.parse(rows)), 53)

    def test_omitted_duplicate_failed_and_skipped_cases_reject(self):
        for operation, reason in (
            ("omit", "PACKAGE_PASS_BEFORE_CASES"),
            ("duplicate", "DUPLICATE_TEST"),
            ("fail", "TEST_NOT_PASSED"),
            ("skip", "TEST_NOT_PASSED"),
        ):
            with self.subTest(operation=operation):
                rows = events()
                first = subject.CASES[0].test_id
                positions = [i for i, e in enumerate(rows) if e.get("Test") == first]
                if operation == "omit":
                    rows = [e for e in rows if e.get("Test") != first]
                elif operation == "duplicate":
                    rows.insert(positions[-1] + 1, event("run", first))
                else:
                    rows[positions[-1]]["Action"] = operation
                self.rejects(rows, reason)

    def test_late_export_parent_pass_and_wrong_package_reject(self):
        rows = events()
        first = next(i for i, x in enumerate(rows) if x["Action"] == "output")
        rows[first], rows[first + 1] = rows[first + 1], rows[first]
        self.rejects(rows, "MISSING_FIXTURE_EXPORT")
        rows = events()
        parent = subject.EXPORT_PARENT + "/T01"
        index = next(i for i, x in enumerate(rows) if x["Action"] == "run" and x.get("Test") == parent)
        rows.insert(index + 1, event("pass", parent))
        self.rejects(rows, "CHILD_OUTSIDE_PARENT")
        rows = events()
        rows[0]["Package"] = "unrelated"
        self.rejects(rows, "GO_PACKAGE")

    def test_marker_source_native_and_scope_corruptions_reject(self):
        for field, replacement, reason in (
            ("sourceSha256", "0" * 64, "BYTE_BINDING_MISMATCH"),
            ("unsignedBytes", 1, "BYTE_BINDING_MISMATCH"),
            ("actionCount", True, "WORKFLOW_ACTION_COUNT"),
            ("caseId", "related-case", "EXPORT_CASE_BINDING"),
            ("nativeExecuted", True, "EXPORT_OVERCLAIM"),
            ("formalAcceptanceGranted", True, "EXPORT_OVERCLAIM"),
            ("requiredEffects", ["SECOND", "FIRST"], "EFFECT_INVENTORY"),
        ):
            with self.subTest(field=field):
                rows = events()
                index = next(i for i, x in enumerate(rows) if x["Action"] == "output")
                case = subject.CASE_BY_TEST[rows[index]["Test"]]
                value = fixture(case)
                value[field] = replacement
                rows[index]["Output"] = export_line(value)
                self.rejects(rows, reason)

    def test_duplicate_export_and_slot_presence_reject(self):
        rows = events()
        index = next(i for i, x in enumerate(rows) if x["Action"] == "output")
        rows.insert(index, copy.deepcopy(rows[index]))
        self.rejects(rows, "EXPORT_MARKER_BINDING")
        for value, reason in ((1, "SLOT_TYPE_PRESENCE"), (False, "ABSENT_SLOT_HAS_VALUE")):
            with self.subTest(value=value):
                rows = events()
                index = next(i for i, x in enumerate(rows) if x["Action"] == "output")
                case = subject.CASE_BY_TEST[rows[index]["Test"]]
                obj = fixture(case)
                obj["slots"][0]["present"] = value
                obj["slots"][0]["expectedValue"] = "stale"
                rows[index]["Output"] = export_line(obj)
                self.rejects(rows, reason)

    def test_hash_and_truncated_stream_cannot_be_repaired_from_summary(self):
        raw = encoded(events())
        with self.assertRaisesRegex(subject.FixtureRejected, "GO_STREAM_HASH"):
            subject.parse_exports(raw, expected_sha256="0" * 64)
        with self.assertRaisesRegex(subject.FixtureRejected, "GO_STREAM_SIZE_OR_TERMINATION"):
            subject.parse_exports(raw[:-1], expected_sha256=subject.sha(raw[:-1]))
        rows = events()
        rows.pop()
        self.rejects(rows, "INCOMPLETE_EXPORT")

    def test_duplicate_json_fields_and_nonfinite_values_reject(self):
        raw = encoded(events())
        malformed = raw.replace(b'"Action":"start"', b'"Action":"start","Action":"start"', 1)
        with self.assertRaisesRegex(subject.FixtureRejected, "DUPLICATE_JSON_KEY"):
            subject.parse_exports(malformed, expected_sha256=subject.sha(malformed))
        with self.assertRaisesRegex(subject.FixtureRejected, "NONFINITE_JSON"):
            subject._json(b'{"x":NaN}')

    def test_original_caller_input_aliases_and_nested_continuation(self):
        nonce = "CHERRI-CALLER-" + "a" * 32
        self.assertEqual(subject.caller_input_expected("nested_false", nonce),
                         "|".join([nonce, nonce, nonce, nonce, "VALUE", nonce, nonce]))
        self.assertEqual(subject.caller_input_expected("nested_true", nonce),
                         "|".join([nonce, nonce, nonce, nonce, nonce, "VALUE", nonce, nonce, nonce]))
        with self.assertRaisesRegex(subject.FixtureRejected, "CALLER_INPUT_NONCE"):
            subject.caller_input_expected("nested_false", "stale")
        with self.assertRaisesRegex(subject.FixtureRejected, "CALLER_CASE_HAS_NO_ORIGINAL_CLIPBOARD"):
            subject.caller_input_expected("without_functions", nonce)


if __name__ == "__main__":
    unittest.main(verbosity=2)
