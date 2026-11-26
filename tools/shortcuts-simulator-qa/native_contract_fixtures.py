"""Source-bound original fixture exports for T01/FN04/M02.

This module retains compiler originals. It does not sign, import, execute or
grant a runtime tier. In particular, an empty JSON array is not native List
type evidence. The exporter runs the existing complete graph models before
emitting each full original workflow.
"""
from __future__ import annotations

import base64
import hashlib
import json
import math
import plistlib
import re
from dataclasses import dataclass
from typing import Any

EXPORT_PARENT = "TestP15NativeContractRuntimeFixtureExport"
EXPORT_SCHEMA = "cherri-native-contract-fixture/1"
PACKAGE = "github.com/electrikmilk/cherri"
EXPORT_COMMAND = (
    "go", "test", "-json", "-count=1", "-timeout=3m",
    "-run", "^" + EXPORT_PARENT + "$", ".",
)
MAX_JSONL = 32 * 1024 * 1024
MAX_EXPORT = 2 * 1024 * 1024
MAX_NATIVE = 1024 * 1024
MAX_SOURCE = 256 * 1024
MARKER = re.compile(
    r"^\s*[A-Za-z0-9_./-]+\.go:[0-9]+: P15_NATIVE_CONTRACT_FIXTURE "
    r"sha256=([0-9a-f]{64}) bytes=([1-9][0-9]*) base64=([A-Za-z0-9+/]+={0,2})$"
)
SHA = re.compile(r"[0-9a-f]{64}\Z")
COMMIT = re.compile(r"[0-9a-f]{40}\Z")
TYPED = (
    "false", "zero", "empty_text", "empty_list", "empty_map",
    "empty_list_default", "empty_map_default", "reserved_map_default",
    "nested_empty_default",
)
OPTIONAL = ("empty", "reserved", "nested_empty", "literal_braces")
M02_MAIN = (
    "empty_input_boundary", "unused_text", "unused_number_binding",
    "void_action_after_value", "partial_early_return", "both_branches_return",
    "zero_iteration_return", "child_return_parent_falls_through",
    "child_bare_return_parent_continues", "parent_explicit_return_after_child",
    "raw_nothing_then_resolved_text", "native_word_is_only_text",
    "legacy_mode_preserves_incidental_result", "authored_native_output",
    "authored_native_stop", "explicit_mode_clears_incidental_result",
    "return_text", "return_empty_text", "return_false", "return_zero",
    "return_empty_", "return_empty_none",
)


class FixtureRejected(ValueError):
    """Closed, non-payload error suitable for a public failure receipt."""


@dataclass(frozen=True)
class Case:
    requirement: str
    case_id: str
    original_test: str
    oracle: str
    filename: str
    mode: str
    slots: tuple[str, ...]

    @property
    def test_id(self) -> str:
        return f"{EXPORT_PARENT}/{self.requirement}/{self.case_id}"


def _catalog() -> tuple[Case, ...]:
    rows: list[Case] = []
    for name in TYPED:
        rows.append(Case(
            "T01", "typed/" + name, "TestP15T01TypedDefaultBoundaryGraphs/" + name,
            "p15TypedDefaultGraph", "p15-short-circuit.cherri",
            "caller-slots-before-original-void", ("supplied", "omitted", "named"),
        ))
    for name in OPTIONAL:
        for mode in ("omitted", "none", "empty"):
            rows.append(Case(
                "T01", f"optional/{name}/{mode}",
                f"TestP15T01OptionalMapDefaultGraphs/{name}/{mode}",
                "p15OptionalMapDefaultGraph", "p15-short-circuit.cherri",
                "optional-caller-slot-before-original-void", ("result",),
            ))
    for flag in ("false", "true"):
        rows.append(Case(
            "T01", "effects/optional_" + flag,
            "TestP15T01MapDefaultEffectOrderAndScope/optional_" + flag,
            "p15MapDefaultEffectGraph", "p15-short-circuit.cherri",
            "caller-slots-and-ordered-default-effects", ("supplied", "omitted"),
        ))
        rows.append(Case(
            "FN04", "nested_" + flag,
            "TestP15FN04CallerInputCompleteNativeGraphs/nested_" + flag,
            "p15CallerInputGraph", "p15-short-circuit.cherri",
            "fresh-caller-input-and-original-clipboard", ("callerInput",),
        ))
    rows.append(Case(
        "FN04", "without_functions", "TestP15FN04CallerInputWithoutFunctions",
        "original-two-ExtensionInput-reads-and-main-Void",
        "p15-short-circuit.cherri", "two-original-input-reads-before-void",
        ("system.input", "ShortcutInput"),
    ))
    for name in M02_MAIN:
        rows.append(Case(
            "M02", "main/" + name, "TestP15M02CompleteMainGraphs/" + name,
            "p15M02GraphError", "p15-none-clear.cherri",
            "unaltered-target-and-separate-caller", ("result",),
        ))
    for number in (0, 1):
        rows.append(Case(
            "M02", f"exact_native/actions_{number}",
            f"TestP15M02ExactNativeWorkflowBoundary/actions_{number}",
            "whole-original-native-document-equality", "p15-none-clear.cherri",
            "unaltered-target-and-separate-caller", ("result",),
        ))
    for size in (0, 20, 51):
        rows.append(Case(
            "M02", f"migration/source_{size}",
            f"TestP15M02MigratedEntryOutput/source_{size}",
            "original-migration-and-complete-output-graph", "p15-none-clear.cherri",
            "unaltered-target-and-separate-caller", ("result",),
        ))
    return tuple(sorted(rows, key=lambda row: row.test_id))


CASES = _catalog()
CASE_BY_TEST = {case.test_id: case for case in CASES}
REQUIRED_EVENTS = frozenset(
    test[:position]
    for case in CASES
    for test in (case.test_id,)
    for position in [len(test)] + [index for index, char in enumerate(test) if char == "/"]
)
# Go's t.Run("a/b") is one named child; it does not emit a run for "a".
# Only the explicit requirement parents and leaf calls exist in this exporter.
REQUIRED_EVENTS = frozenset(
    [EXPORT_PARENT] + [f"{EXPORT_PARENT}/{name}" for name in ("T01", "FN04", "M02")]
    + list(CASE_BY_TEST)
)
EXPORT_KEYS = frozenset((
    "schema", "requirement", "caseId", "originalTest", "originalGraphOracle",
    "compileFilename", "sourceBase64", "sourceSha256", "sourceBytes",
    "unsignedBase64", "unsignedSha256", "unsignedBytes", "actionCount",
    "observationMode", "slots", "requiredEffects", "nativeExecuted",
    "formalAcceptanceGranted",
))
SLOT_KEYS = frozenset(("name", "authoredType", "present", "expectedValue"))


def _pairs(items: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in items:
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


def sha(raw: bytes) -> str:
    return hashlib.sha256(raw).hexdigest()


def _integer(value: Any, low: int, high: int) -> bool:
    return type(value) is int and low <= value <= high


def _decoded(value: Any, size: Any, digest: Any, limit: int) -> bytes:
    if not isinstance(value, str) or not _integer(size, 0, limit) or not isinstance(digest, str) or not SHA.fullmatch(digest):
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


def validate_fixture(value: Any, case: Case) -> dict[str, Any]:
    if type(value) is not dict or set(value) != EXPORT_KEYS:
        raise FixtureRejected("EXPORT_SCHEMA")
    fixed = {
        "schema": EXPORT_SCHEMA, "requirement": case.requirement, "caseId": case.case_id,
        "originalTest": case.original_test, "originalGraphOracle": case.oracle,
        "compileFilename": case.filename, "observationMode": case.mode,
    }
    if any(value[key] != expected for key, expected in fixed.items()):
        raise FixtureRejected("EXPORT_CASE_BINDING")
    if value["nativeExecuted"] is not False or value["formalAcceptanceGranted"] is not False:
        raise FixtureRejected("EXPORT_OVERCLAIM")
    source = _decoded(value["sourceBase64"], value["sourceBytes"], value["sourceSha256"], MAX_SOURCE)
    raw = _decoded(value["unsignedBase64"], value["unsignedBytes"], value["unsignedSha256"], MAX_NATIVE)
    try:
        source.decode("utf-8", errors="strict")
        workflow = plistlib.loads(raw)
    except (UnicodeError, plistlib.InvalidFileException, ValueError, TypeError, OverflowError, RecursionError) as exc:
        raise FixtureRejected("INVALID_ORIGINAL_WORKFLOW") from exc
    if type(workflow) is not dict or type(workflow.get("WFWorkflowActions")) is not list:
        raise FixtureRejected("WORKFLOW_ACTIONS")
    actions = workflow["WFWorkflowActions"]
    if not _integer(value["actionCount"], 0, 10000) or len(actions) != value["actionCount"]:
        raise FixtureRejected("WORKFLOW_ACTION_COUNT")
    if any(type(action) is not dict or type(action.get("WFWorkflowActionIdentifier")) is not str for action in actions):
        raise FixtureRejected("WORKFLOW_ACTION_ENVELOPE")
    slots = value["slots"]
    if type(slots) is not list or len(slots) != len(case.slots):
        raise FixtureRejected("SLOT_INVENTORY")
    for slot, name in zip(slots, case.slots):
        if type(slot) is not dict or set(slot) != SLOT_KEYS or slot["name"] != name:
            raise FixtureRejected("SLOT_SCHEMA")
        if not isinstance(slot["authoredType"], str) or not 1 <= len(slot["authoredType"]) <= 80 or type(slot["present"]) is not bool:
            raise FixtureRejected("SLOT_TYPE_PRESENCE")
        if slot["present"] is False and slot["expectedValue"] is not None:
            raise FixtureRejected("ABSENT_SLOT_HAS_VALUE")
    effects = ["omitted:FIRST", "omitted:SECOND"] if case.case_id.startswith("effects/") else []
    if value["requiredEffects"] != effects:
        raise FixtureRejected("EFFECT_INVENTORY")
    return value


def parse_exports(raw: bytes, *, expected_sha256: str) -> tuple[dict[str, Any], ...]:
    """Parse an already process/source-bound original Go JSONL, never a summary.

    The caller must authenticate the real exit, timeout, streams, source before/
    after, toolchain, exact EXPORT_COMMAND and expected_sha256 independently.
    This function rejects incomplete/extra/duplicate case events and late output.
    It makes no native or formal acceptance decision.
    """
    if type(raw) is not bytes or not raw or len(raw) > MAX_JSONL or not raw.endswith(b"\n"):
        raise FixtureRejected("GO_STREAM_SIZE_OR_TERMINATION")
    if not isinstance(expected_sha256, str) or not SHA.fullmatch(expected_sha256) or sha(raw) != expected_sha256:
        raise FixtureRejected("GO_STREAM_HASH")
    states: dict[str, str] = {}
    buffers: dict[str, str] = {}
    exports: dict[str, dict[str, Any]] = {}
    package_started = package_passed = False
    for line in raw.splitlines():
        if not line:
            raise FixtureRejected("EMPTY_GO_EVENT")
        event = _json(line)
        if type(event) is not dict or event.get("Package") != PACKAGE:
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
                if not package_started or set(states) != REQUIRED_EVENTS or any(state != "pass" for state in states.values()):
                    raise FixtureRejected("PACKAGE_PASS_BEFORE_CASES")
                package_passed = True
            elif action != "output":
                raise FixtureRejected("PACKAGE_NOT_PASSED")
            continue
        if not package_started or test not in REQUIRED_EVENTS:
            raise FixtureRejected("UNEXPECTED_TEST")
        if action == "run":
            if test in states:
                raise FixtureRejected("DUPLICATE_TEST")
            for ancestor in (EXPORT_PARENT, f"{EXPORT_PARENT}/{test.split('/')[1]}" if "/" in test else ""):
                if ancestor and ancestor != test and states.get(ancestor) != "run":
                    raise FixtureRejected("CHILD_OUTSIDE_PARENT")
            states[test] = "run"
        elif action == "output":
            if states.get(test) != "run" or type(event.get("Output")) is not str:
                raise FixtureRejected("OUTPUT_OUTSIDE_TEST")
            text = buffers.get(test, "") + event["Output"]
            if len(text.encode("utf-8")) > 3 * MAX_EXPORT:
                raise FixtureRejected("TEST_OUTPUT_BOUND")
            while "\n" in text:
                output, text = text.split("\n", 1)
                if "P15_NATIVE_CONTRACT_FIXTURE" not in output:
                    continue
                match = MARKER.fullmatch(output)
                if match is None or test not in CASE_BY_TEST or test in exports:
                    raise FixtureRejected("EXPORT_MARKER_BINDING")
                encoded = _decoded(match[3], int(match[2]), match[1], MAX_EXPORT)
                exports[test] = validate_fixture(_json(encoded), CASE_BY_TEST[test])
            buffers[test] = text
        elif action == "pass":
            if states.get(test) != "run" or buffers.get(test):
                raise FixtureRejected("INVALID_TEST_PASS")
            if any(state != "pass" for child, state in states.items() if child.startswith(test + "/")):
                raise FixtureRejected("PARENT_BEFORE_CHILD")
            if test in CASE_BY_TEST and test not in exports:
                raise FixtureRejected("MISSING_FIXTURE_EXPORT")
            states[test] = "pass"
        else:
            raise FixtureRejected("TEST_NOT_PASSED")
    if not package_passed or set(exports) != set(CASE_BY_TEST):
        raise FixtureRejected("INCOMPLETE_EXPORT")
    return tuple(exports[case.test_id] for case in CASES)


def caller_input_expected(case_id: str, nonce_input: str) -> str:
    """Original FN04 clipboard value, using a fresh actual input sentinel."""
    if not re.fullmatch(r"CHERRI-CALLER-[0-9a-f]{32}", nonce_input):
        raise FixtureRejected("CALLER_INPUT_NONCE")
    if case_id == "nested_false":
        fields = [nonce_input] * 4 + ["VALUE"] + [nonce_input] * 2
    elif case_id == "nested_true":
        fields = [nonce_input] * 5 + ["VALUE"] + [nonce_input] * 3
    else:
        raise FixtureRejected("CALLER_CASE_HAS_NO_ORIGINAL_CLIPBOARD")
    return "|".join(fields)
