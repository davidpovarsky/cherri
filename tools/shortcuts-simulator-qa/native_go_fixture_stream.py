"""Strict collector for complete source-bound Go fixture-export streams.

This module validates event lifecycle and byte identity only. It neither runs
native shortcuts nor grants an Apple tier or a formal acceptance result.
"""
from __future__ import annotations

import base64
import hashlib
import json
import re
from typing import Any

SHA = re.compile(r"[0-9a-f]{64}\Z")


class FixtureRejected(ValueError):
    """A bounded non-payload diagnosis, never a native execution result."""


def sha(raw: bytes) -> str:
    return hashlib.sha256(raw).hexdigest()


def _pairs(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in pairs:
        if key in result:
            raise FixtureRejected("DUPLICATE_JSON_KEY")
        result[key] = value
    return result


def _json(raw: bytes | str) -> Any:
    try:
        return json.loads(
            raw, object_pairs_hook=_pairs,
            parse_constant=lambda _: (_ for _ in ()).throw(FixtureRejected("NONFINITE_JSON")),
        )
    except (UnicodeError, json.JSONDecodeError, RecursionError) as exc:
        raise FixtureRejected("INVALID_JSON") from exc


def _decoded(value: Any, size: Any, digest: Any, limit: int) -> bytes:
    if type(value) is not str or type(size) is not int or not 0 < size <= limit or type(digest) is not str or not SHA.fullmatch(digest):
        raise FixtureRejected("INVALID_BYTE_BINDING")
    if len(value) > 4 * ((limit + 2) // 3):
        raise FixtureRejected("OVERSIZE_BASE64")
    try:
        raw = base64.b64decode(value, validate=True)
    except (ValueError, TypeError) as exc:
        raise FixtureRejected("INVALID_BASE64") from exc
    if base64.b64encode(raw).decode("ascii") != value or len(raw) != size or sha(raw) != digest:
        raise FixtureRejected("BYTE_BINDING_MISMATCH")
    return raw


def parse_complete_exports(
    raw: bytes, *, expected_sha256: str, package: str,
    required_events: frozenset[str], case_by_test: dict[str, Any],
    marker_text: str, marker: re.Pattern[str], validate_fixture: Any,
    max_jsonl: int, max_export: int,
) -> dict[str, dict[str, Any]]:
    """Validate one complete fixed Go inventory and its source-bound exports.

    All inputs other than the original stream are reviewed protocol constants.
    A slash in a Go subtest label does not create an intermediate run event.
    Parent ownership therefore uses the nearest actually inventoried ancestor.
    Source/process/toolchain/exit provenance remains the caller's obligation.
    """
    if type(raw) is not bytes or not raw or len(raw) > max_jsonl or not raw.endswith(b"\n"):
        raise FixtureRejected("GO_STREAM_SIZE_OR_TERMINATION")
    if type(expected_sha256) is not str or not SHA.fullmatch(expected_sha256) or sha(raw) != expected_sha256:
        raise FixtureRejected("GO_STREAM_HASH")
    states: dict[str, str] = {}
    buffers: dict[str, str] = {}
    exports: dict[str, dict[str, Any]] = {}
    package_started = package_passed = False
    for line in raw.splitlines():
        if not line:
            raise FixtureRejected("EMPTY_GO_EVENT")
        event = _json(line)
        if type(event) is not dict or event.get("Package") != package:
            raise FixtureRejected("GO_PACKAGE")
        action, test = event.get("Action"), event.get("Test", "")
        if type(action) is not str or type(test) is not str:
            raise FixtureRejected("GO_EVENT_SCHEMA")
        if package_passed:
            raise FixtureRejected("EVENT_AFTER_PACKAGE_PASS")
        if not test:
            if action == "start":
                if package_started:
                    raise FixtureRejected("DUPLICATE_PACKAGE_START")
                package_started = True
            elif action == "pass":
                if not package_started or set(states) != required_events or any(state != "pass" for state in states.values()):
                    raise FixtureRejected("PACKAGE_PASS_BEFORE_CASES")
                package_passed = True
            elif action == "output":
                if not package_started or type(event.get("Output")) is not str or marker_text in event["Output"]:
                    raise FixtureRejected("UNBOUND_PACKAGE_OUTPUT")
            else:
                raise FixtureRejected("PACKAGE_NOT_PASSED")
            continue
        if not package_started or test not in required_events:
            raise FixtureRejected("UNEXPECTED_TEST")
        if action == "run":
            if test in states:
                raise FixtureRejected("DUPLICATE_TEST")
            ancestors = [name for name in required_events if test.startswith(name + "/")]
            if ancestors and states.get(max(ancestors, key=len)) != "run":
                raise FixtureRejected("CHILD_OUTSIDE_PARENT")
            states[test] = "run"
        elif action == "output":
            if states.get(test) != "run" or type(event.get("Output")) is not str:
                raise FixtureRejected("OUTPUT_OUTSIDE_TEST")
            text = buffers.get(test, "") + event["Output"]
            if len(text.encode("utf-8")) > 3 * max_export:
                raise FixtureRejected("TEST_OUTPUT_BOUND")
            while "\n" in text:
                output, text = text.split("\n", 1)
                if marker_text not in output:
                    continue
                match = marker.fullmatch(output)
                if match is None or test not in case_by_test or test in exports:
                    raise FixtureRejected("EXPORT_MARKER_BINDING")
                encoded = _decoded(match[3], int(match[2]), match[1], max_export)
                exports[test] = validate_fixture(_json(encoded), case_by_test[test])
            buffers[test] = text
        elif action == "pass":
            if states.get(test) != "run" or buffers.get(test):
                raise FixtureRejected("INVALID_TEST_PASS")
            if any(state != "pass" for child, state in states.items() if child.startswith(test + "/")):
                raise FixtureRejected("PARENT_BEFORE_CHILD")
            if test in case_by_test and test not in exports:
                raise FixtureRejected("MISSING_FIXTURE_EXPORT")
            states[test] = "pass"
        else:
            raise FixtureRejected("TEST_NOT_PASSED")
    if not package_passed or set(exports) != set(case_by_test):
        raise FixtureRejected("INCOMPLETE_EXPORT")
    return exports
