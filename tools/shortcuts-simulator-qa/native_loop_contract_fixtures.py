"""Retain all three original F02/RP22/BRG12 compiler fixtures.

This is an adapter around the original loop source factories and native graph
oracles. It grants no Apple runtime tier. The caller authenticates the actual
process, exit, source before/after, toolchain and original JSONL hash separately.
All six original test families, including their causal children, must pass in
the same exact process as the three full workflow exports.
"""
from __future__ import annotations

import plistlib
import re
from dataclasses import dataclass
from typing import Any

from native_go_fixture_stream import (
    SHA, FixtureRejected, _decoded, _json, _pairs, parse_complete_exports, sha,
)

PACKAGE = "github.com/electrikmilk/cherri"
EXPORT_PARENT = "TestP15NativeLoopContractFixtureExport"
EXPORT_SCHEMA = "cherri-native-loop-fixture/1"
MAX_JSONL = 16 * 1024 * 1024
MAX_EXPORT = 2 * 1024 * 1024
MAX_NATIVE = 1024 * 1024
MAX_SOURCE = 4096
MARKER_TEXT = "P15_NATIVE_LOOP_FIXTURE"
MARKER = re.compile(
    r"^\s*p15_native_loop_contract_export_test.go:[0-9]+: P15_NATIVE_LOOP_FIXTURE "
    r"sha256=([0-9a-f]{64}) bytes=([1-9][0-9]*) base64=([A-Za-z0-9+/]+={0,2})$"
)


@dataclass(frozen=True)
class Case:
    requirement: str
    unit_test: str
    native_test: str
    source_sha256: str
    source_bytes: int
    displays: tuple[str, ...]

    @property
    def export_test(self) -> str:
        return f"{EXPORT_PARENT}/{self.requirement}"


CASES = {
    "F02": Case("F02", "TestP15F02LoopScopeUnit", "TestP15F02LoopNativeGraph", "1246d05cfdf2b9da82ecb8e29a5a0b44404e15cd1079fe63077962728e88fd39", 282,
        ("F02_PAIR:A:1:0:0", "F02_PAIR:A:2:0:1", "F02_AFTER:A:0", "F02_PAIR:B:1:1:0", "F02_PAIR:B:2:1:1", "F02_AFTER:B:1", "F02_REPEAT:0", "F02_REPEAT:1",)),
    "RP22": Case("RP22", "TestP15RP22LoopBindingsUnit", "TestP15RP22LoopNativeGraph", "951c2c2db1b72bb97ca0a64a5e14a87b2529e5c786794272f1e54df6d0425e8b", 393,
        ("RP22_OUTER:0:A", "RP22_REPEAT:0:A", "RP22_INNER:0:1", "RP22_INNER:1:2", "RP22_REPEAT_AFTER:0:A", "RP22_REPEAT:1:A", "RP22_INNER:0:1", "RP22_INNER:1:2", "RP22_REPEAT_AFTER:1:A", "RP22_OUTER_AFTER:0:A", "RP22_OUTER:1:B", "RP22_REPEAT:0:B", "RP22_INNER:0:1", "RP22_INNER:1:2", "RP22_REPEAT_AFTER:0:B", "RP22_REPEAT:1:B", "RP22_INNER:0:1", "RP22_INNER:1:2", "RP22_REPEAT_AFTER:1:B", "RP22_OUTER_AFTER:1:B", "RP22_RESTORED:99:outside",)),
    "BRG12": Case("BRG12", "TestBackendRecovery12LoopIdentityUnit", "TestBackendRecovery12LoopNativeGraph", "4239cc938ac02c736c1fd19aeb3d9e1c0a1d74f9f417be2236111b5a1d506d53", 828,
        ("BRG12_PAIR:A:1:0:0", "BRG12_PAIR:A:2:0:1", "BRG12_MIXED:A:0:0", "BRG12_MIXED:A:0:1", "BRG12_AFTER:A:0", "BRG12_PAIR:B:1:1:0", "BRG12_PAIR:B:2:1:1", "BRG12_MIXED:B:1:0", "BRG12_MIXED:B:1:1", "BRG12_AFTER:B:1", "BRG12_SIBLING:C:0", "BRG12_SIBLING:D:1", "BRG12_REVERSE:0:0:X", "BRG12_REVERSE:0:1:Y", "BRG12_REVERSE_AFTER:0", "BRG12_REVERSE:1:0:X", "BRG12_REVERSE:1:1:Y", "BRG12_REVERSE_AFTER:1", "BRG12_SHADOW_INNER:0:S", "BRG12_SHADOW_INNER:1:S", "BRG12_SHADOW_OUTER:0:S", "BRG12_RESTORED:77:outside",)),
}
UNIT_CHILDREN = (
    "scoped_types", "tuple_index_does_not_escape", "tuple_item_does_not_escape",
    "named_repeat_index_does_not_escape", "healthy_after_rejection",
)
GRAPH_FAULTS = (
    "one_based_capture", "wrong_index_owner", "wrong_item_owner", "wrong_offset",
    "wrong_direction", "string_offset", "dangling_index_edge",
    "duplicate_producer_identity", "reused_loop_owner", "wrong_end_owner",
    "iteration_order", "iteration_cardinality", "repeat_count",
    "consumer_wrong_owner", "post_inner_owner_lost", "capture_escaped_loop",
    "missing_action", "extra_action",
)


def original_events(case: Case) -> tuple[str, ...]:
    native_children = ["consistent_uuid_renaming"]
    if case.requirement != "F02":
        native_children.append("consistent_shadow_renaming")
    native_children.extend(GRAPH_FAULTS)
    if case.requirement != "F02":
        native_children.append("shadow_leaks_after_scope")
    return tuple(
        [case.unit_test] + [case.unit_test + "/" + child for child in UNIT_CHILDREN]
        + [case.native_test] + [case.native_test + "/" + child for child in native_children]
    )


ORIGINAL_PARENTS = tuple(parent for case in CASES.values() for parent in (case.unit_test, case.native_test))
REQUIRED_EVENTS = frozenset(
    [EXPORT_PARENT] + [case.export_test for case in CASES.values()]
    + [event for case in CASES.values() for event in original_events(case)]
)
CASE_BY_TEST = {case.export_test: case for case in CASES.values()}
EXPORT_COMMAND = (
    "go", "test", "-json", "-count=1", "-timeout=3m",
    "-run", "^(" + "|".join((*ORIGINAL_PARENTS, EXPORT_PARENT)) + ")$", ".",
)
EXPORT_KEYS = frozenset((
    "schema", "requirement", "caseId", "originalUnitTest", "originalNativeTest",
    "originalGraphOracle", "compileFilename", "sourceBase64", "sourceSha256",
    "sourceBytes", "unsignedBase64", "unsignedSha256", "unsignedBytes",
    "actionCount", "workflowName", "workflowNamePresent", "observationMode",
    "expectedDisplays", "nativeExecuted", "formalAcceptanceGranted",
))


def validate_fixture(value: Any, case: Case) -> dict[str, Any]:
    if type(value) is not dict or set(value) != EXPORT_KEYS:
        raise FixtureRejected("EXPORT_SCHEMA")
    fixed = {
        "schema": EXPORT_SCHEMA, "requirement": case.requirement,
        "caseId": "full-original-scenario", "originalUnitTest": case.unit_test,
        "originalNativeTest": case.native_test, "originalGraphOracle": "p15LoopNativeError",
        "compileFilename": "p15-short-circuit.cherri",
        "sourceSha256": case.source_sha256, "sourceBytes": case.source_bytes,
        "observationMode": "ordered-original-show-result-sequence",
        "expectedDisplays": list(case.displays),
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
    if type(value["actionCount"]) is not int or not 1 <= value["actionCount"] <= 10000 or value["actionCount"] != len(actions):
        raise FixtureRejected("WORKFLOW_ACTION_COUNT")
    if any(type(action) is not dict or type(action.get("WFWorkflowActionIdentifier")) is not str for action in actions):
        raise FixtureRejected("WORKFLOW_ACTION_ENVELOPE")
    if actions[-1] != {"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"}:
        raise FixtureRejected("ORIGINAL_MAIN_VOID_BOUNDARY")
    if type(value["workflowNamePresent"]) is not bool or type(value["workflowName"]) is not str:
        raise FixtureRejected("WORKFLOW_NAME")
    if value["workflowNamePresent"] != ("WFWorkflowName" in workflow) or value["workflowName"] != workflow.get("WFWorkflowName", ""):
        raise FixtureRejected("WORKFLOW_NAME_BINDING")
    return value


def parse_exports(raw: bytes, *, expected_sha256: str) -> tuple[dict[str, Any], ...]:
    """Require the complete original 82 events and four additive export events.

    This parses one authentic stream only after its caller has established
    process/source/toolchain/command/exit/timeout and SHA provenance. Synthetic
    parser controls do not supply those facts or Apple observations.
    """
    exports = parse_complete_exports(
        raw, expected_sha256=expected_sha256, package=PACKAGE,
        required_events=REQUIRED_EVENTS, case_by_test=CASE_BY_TEST,
        marker_text=MARKER_TEXT, marker=MARKER, validate_fixture=validate_fixture,
        max_jsonl=MAX_JSONL, max_export=MAX_EXPORT,
    )
    return tuple(exports[case.export_test] for case in CASES.values())
