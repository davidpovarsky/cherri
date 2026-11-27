"""Full original RP24/RP18/BRG13 compile fixtures; no native execution grant.

The Go owner must run every original semantic/graph event and all exports in
one real process. This collector validates its retained stream and byte
bindings. Source/process/toolchain/exit identity is established separately.
"""
from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
import plistlib
import re
from typing import Any

from native_go_fixture_stream import (
    FixtureRejected, _decoded, _json, parse_complete_exports, sha,
)

PACKAGE = "github.com/electrikmilk/cherri"
EXPORT_PARENT = "TestP15NativeFlowContractFixtureExport"
EXPORT_SCHEMA = "cherri-native-flow-fixture/1"
MAX_JSONL = 16 * 1024 * 1024
MAX_EXPORT = 2 * 1024 * 1024
MAX_NATIVE = 1024 * 1024
MAX_SOURCE = 16384
MARKER_TEXT = "P15_NATIVE_FLOW_FIXTURE"
MARKER = re.compile(
    r"^\s*p15_native_flow_contract_export_test.go:[0-9]+: P15_NATIVE_FLOW_FIXTURE "
    r"sha256=([0-9a-f]{64}) bytes=([1-9][0-9]*) base64=([A-Za-z0-9+/]+={0,2})$"
)


@dataclass(frozen=True)
class Case:
    requirement: str
    unit_test: str
    native_test: str
    source_path: str
    source_sha256: str
    source_bytes: int
    compile_filename: str
    graph_oracle: str
    expected_text: str

    @property
    def export_test(self) -> str:
        return f"{EXPORT_PARENT}/{self.requirement}"


ORDER = "|AND_TF|AND_TT|OR_FF|OR_FT|MIX_A|MIX_B|MIX_C|MIX_D|MIX_E|MIX_F|NEED_H|NEED_J"
CASES = {
    "RP24": Case(
        "RP24", "TestP15RP24CombinedReturnRecursionUnit", "TestP15RP24CombinedReturnRecursionNative",
        "tests/language-v2/v2-rp24-return-recursion.cherri",
        "ff0ab9a41e1cffeea4860ce185a176e8430d8b9dee4520216994191c7ab6cc04", 303,
        "p15-function-transport.cherri", "p15RP24Graph", "10|6|7",
    ),
    "RP18": Case(
        "RP18", "TestP15RP18ObservableShortCircuitUnit", "TestP15RP18ObservableShortCircuitNative",
        "tests/language-v2/v2-rp18-observable-short-circuit.cherri",
        "fd40f18e0a8711a2a21fa90e417b60acf057706f82698eeb3c4da774cee1504b", 4037,
        "p15-short-circuit.cherri", "p15SCGraph", "P15_RP18_LOCAL_OK:calls=12:order=" + ORDER,
    ),
    "BRG13": Case(
        "BRG13", "TestBackendRecovery13TypedShortCircuitUnit", "TestBackendRecovery13TypedShortCircuitNative",
        "tests/language-v2/v2-brg13-typed-short-circuit.cherri",
        "4d445f0548ee8a5432582582fcabf8a3f00b9f49bcdfe8182bccf2a27fa1e396", 5364,
        "p15-short-circuit.cherri", "p15SCGraph", "P15_BRG13_LOCAL_OK:calls=12:order=" + ORDER,
    ),
}
RP24_UNIT_CHILDREN = ("immutable_capture", "mutable_capture", "later_capture", "wrong_argument", "wrong_return")
RP24_GRAPH_FAULTS = (
    "missing_action", "extra_action", "zero_producer_reconciled", "malformed_producer_reconciled",
    "duplicate_producer", "zero_group_reconciled", "malformed_group_reconciled", "real_control_code",
    "named_right_slot", "add_operator", "add_return", "base_predicate", "base_threshold",
    "base_result", "base_return", "decrement_operator", "recursive_argument", "recursive_callee",
    "recursive_self_target", "recursive_cast", "recursive_product", "recursive_return",
    "main_first_argument", "main_named_argument", "main_argument_presence", "main_callee",
    "main_number_cast", "continuation_product", "initial_recursion_argument", "continuation_add",
    "final_observation", "final_return", "authored_name_reconciled",
)


def original_inventory() -> dict[str, tuple[str, ...]]:
    root = Path(__file__).resolve().parents[2]
    raw = (root / "tools/language-acceptance/short_circuit_inventory.json").read_bytes()
    if sha(raw) != "524452f66c5eff6b0fe4cc60dee1da918d943559e7d8e9e3ed3fe8da759da2b4":
        raise FixtureRejected("ORIGINAL_SHORT_CIRCUIT_INVENTORY_CHANGED")
    value = _json(raw)
    result = {}
    for requirement, count in (("RP18", 96), ("BRG13", 139)):
        names = value["events"][requirement]
        case = CASES[requirement]
        if (type(names) is not list or len(names) != count or len(set(names)) != count
                or any(type(name) is not str or name.split("/")[0] not in (case.unit_test, case.native_test) for name in names)
                or case.unit_test not in names or case.native_test not in names):
            raise FixtureRejected("ORIGINAL_SHORT_CIRCUIT_INVENTORY")
        result[requirement] = tuple(names)
    authority = (root / "tools/language-acceptance/function_return_execution.go").read_bytes()
    if sha(authority) != "0fb9ce8751a56229f83ac061d833814f34ee5993e8aacb6ae73c425761469954":
        raise FixtureRejected("ORIGINAL_RP24_INVENTORY_CHANGED")
    case = CASES["RP24"]
    result["RP24"] = tuple(
        [case.unit_test, case.native_test]
        + [case.unit_test + "/" + name for name in RP24_UNIT_CHILDREN]
        + [case.native_test + "/oracle/" + name for name in RP24_GRAPH_FAULTS]
    )
    return result


ORIGINAL_EVENTS = original_inventory()
ORIGINAL_PARENTS = tuple(parent for case in CASES.values() for parent in (case.unit_test, case.native_test))
CASE_BY_TEST = {case.export_test: case for case in CASES.values()}
REQUIRED_EVENTS = frozenset(
    [EXPORT_PARENT] + list(CASE_BY_TEST)
    + [name for case in CASES.values() for name in ORIGINAL_EVENTS[case.requirement]]
)
EXPORT_COMMAND = (
    "go", "test", "-json", "-count=1", "-timeout=3m",
    "-run", "^(" + "|".join((*ORIGINAL_PARENTS, EXPORT_PARENT)) + ")$", ".",
)
EXPORT_KEYS = frozenset((
    "schema", "requirement", "caseId", "originalUnitTest", "originalNativeTest",
    "originalGraphOracle", "sourcePath", "compileFilename", "sourceBase64",
    "sourceSha256", "sourceBytes", "unsignedBase64", "unsignedSha256", "unsignedBytes",
    "actionCount", "workflowName", "workflowNamePresent", "observationMode",
    "expectedType", "expectedText", "nativeExecuted", "formalAcceptanceGranted",
))


def validate_fixture(value: Any, case: Case) -> dict[str, Any]:
    if type(value) is not dict or set(value) != EXPORT_KEYS:
        raise FixtureRejected("EXPORT_SCHEMA")
    fixed = {
        "schema": EXPORT_SCHEMA, "requirement": case.requirement,
        "caseId": "full-original-scenario", "originalUnitTest": case.unit_test,
        "originalNativeTest": case.native_test, "originalGraphOracle": case.graph_oracle,
        "sourcePath": case.source_path, "compileFilename": case.compile_filename,
        "sourceSha256": case.source_sha256, "sourceBytes": case.source_bytes,
        "observationMode": "exact-original-returned-text",
        "expectedType": "Text", "expectedText": case.expected_text,
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
    if type(value["workflowNamePresent"]) is not bool or type(value["workflowName"]) is not str:
        raise FixtureRejected("WORKFLOW_NAME")
    if value["workflowNamePresent"] != ("WFWorkflowName" in workflow) or value["workflowName"] != workflow.get("WFWorkflowName", ""):
        raise FixtureRejected("WORKFLOW_NAME_BINDING")
    return value


def parse_exports(raw: bytes, *, expected_sha256: str) -> tuple[dict[str, Any], ...]:
    """Require all 275 original events plus four additive export events."""
    exports = parse_complete_exports(
        raw, expected_sha256=expected_sha256, package=PACKAGE,
        required_events=REQUIRED_EVENTS, case_by_test=CASE_BY_TEST,
        marker_text=MARKER_TEXT, marker=MARKER, validate_fixture=validate_fixture,
        max_jsonl=MAX_JSONL, max_export=MAX_EXPORT,
    )
    return tuple(exports[case.export_test] for case in CASES.values())
