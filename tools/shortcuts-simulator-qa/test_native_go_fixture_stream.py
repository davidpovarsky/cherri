"""Synthetic event-lifecycle controls; never compiler or native evidence."""
from __future__ import annotations

import base64
import json
import re
import unittest

from native_go_fixture_stream import FixtureRejected, parse_complete_exports, sha

PACKAGE = "synthetic.fixture.controls"
PARENT = "TestOriginal"
LEAF = PARENT + "/oracle/slash_label"
NESTED_PARENT = PARENT + "/nested"
NESTED_LEAF = NESTED_PARENT + "/fault"
EXPORT_PARENT = "TestExport"
EXPORT = EXPORT_PARENT + "/case"
REQUIRED = frozenset((PARENT, LEAF, NESTED_PARENT, NESTED_LEAF, EXPORT_PARENT, EXPORT))
MARKER_TEXT = "SYNTHETIC_FIXTURE"
MARKER = re.compile(r"^SYNTHETIC_FIXTURE sha256=([0-9a-f]{64}) bytes=([1-9][0-9]*) base64=([A-Za-z0-9+/]+={0,2})$")


def row(action, test="", **extra):
    result = {"Action": action, "Package": PACKAGE}
    if test:
        result["Test"] = test
    return dict(result, **extra)


def rows():
    payload = b'{"case":"original","nativeExecuted":false}'
    output = f"{MARKER_TEXT} sha256={sha(payload)} bytes={len(payload)} base64={base64.b64encode(payload).decode()}\n"
    return [
        row("start"), row("run", PARENT),
        row("run", LEAF), row("pass", LEAF),
        row("run", NESTED_PARENT), row("run", NESTED_LEAF),
        row("pass", NESTED_LEAF), row("pass", NESTED_PARENT), row("pass", PARENT),
        row("run", EXPORT_PARENT), row("run", EXPORT),
        row("output", EXPORT, Output=output), row("pass", EXPORT),
        row("pass", EXPORT_PARENT), row("pass"),
    ]


def validate(value, case):
    if value != {"case": case, "nativeExecuted": False}:
        raise FixtureRejected("SYNTHETIC_CASE")
    return value


def parse(events):
    raw = b"".join(json.dumps(event).encode() + b"\n" for event in events)
    return parse_complete_exports(
        raw, expected_sha256=sha(raw), package=PACKAGE,
        required_events=REQUIRED, case_by_test={EXPORT: "original"},
        marker_text=MARKER_TEXT, marker=MARKER, validate_fixture=validate,
        max_jsonl=65536, max_export=4096,
    )


class NativeGoFixtureStreamTests(unittest.TestCase):
    def test_slash_label_uses_real_ancestor_without_invented_event(self):
        result = parse(rows())
        self.assertEqual(result, {EXPORT: {"case": "original", "nativeExecuted": False}})
        self.assertNotIn(PARENT + "/oracle", REQUIRED)

    def test_required_nearest_parent_must_be_running(self):
        events = [e for e in rows() if e.get("Test") != NESTED_PARENT]
        with self.assertRaisesRegex(FixtureRejected, "CHILD_OUTSIDE_PARENT"):
            parse(events)
        events = rows()
        at = next(i for i, e in enumerate(events) if e.get("Test") == NESTED_PARENT and e["Action"] == "run")
        events.insert(at + 1, row("pass", NESTED_PARENT))
        with self.assertRaisesRegex(FixtureRejected, "CHILD_OUTSIDE_PARENT"):
            parse(events)

    def test_label_parent_cannot_be_invented_and_child_cannot_be_omitted(self):
        events = rows()
        events.insert(2, row("run", PARENT + "/oracle"))
        with self.assertRaisesRegex(FixtureRejected, "UNEXPECTED_TEST"):
            parse(events)
        with self.assertRaisesRegex(FixtureRejected, "PACKAGE_PASS_BEFORE_CASES"):
            parse([e for e in rows() if e.get("Test") != LEAF])

    def test_parent_cannot_finish_before_live_or_late_child(self):
        events = rows()
        at = next(i for i, e in enumerate(events) if e.get("Test") == LEAF and e["Action"] == "run")
        events.insert(at + 1, row("pass", PARENT))
        with self.assertRaisesRegex(FixtureRejected, "PARENT_BEFORE_CHILD"):
            parse(events)
        events = rows()
        leaf = [e for e in events if e.get("Test") == LEAF]
        events = [e for e in events if e.get("Test") != LEAF]
        at = next(i for i, e in enumerate(events) if e.get("Test") == PARENT and e["Action"] == "pass")
        events[at + 1:at + 1] = leaf
        with self.assertRaisesRegex(FixtureRejected, "CHILD_OUTSIDE_PARENT"):
            parse(events)

    def test_export_cannot_be_attributed_to_original_or_follow_its_pass(self):
        events = rows()
        packet = next(e["Output"] for e in events if e.get("Test") == EXPORT and e["Action"] == "output")
        events.insert(2, row("output", PARENT, Output=packet))
        with self.assertRaisesRegex(FixtureRejected, "EXPORT_MARKER_BINDING"):
            parse(events)
        events = rows()
        at = next(i for i, e in enumerate(events) if e.get("Test") == EXPORT and e["Action"] == "pass")
        events.insert(at + 1, row("output", EXPORT, Output=packet))
        with self.assertRaisesRegex(FixtureRejected, "OUTPUT_OUTSIDE_TEST"):
            parse(events)


if __name__ == "__main__":
    unittest.main()
