"""Collect all 50 complete original RP12 programs and their original Go proof.

The 37 self-contained, eight custom-provider and five image-input instances
remain separate. Intermediate action/value observations are still required;
main fallthrough or successful compilation does not grant native acceptance.
"""
from __future__ import annotations

from dataclasses import dataclass
import json
from pathlib import Path
import plistlib
import re
from typing import Any

from native_go_fixture_stream import FixtureRejected, _decoded, _json, parse_complete_exports, sha

PACKAGE = "github.com/electrikmilk/cherri"
EXPORT_PARENT = "TestP15NativeRP12OriginalFixtureExport"
EXPORT_SCHEMA = "cherri-native-rp12-original-fixture/1"
ORIGINAL_MAP_SHA256 = "fc33704fd0eafc5e06c59f25b208f0078bfce01dbbc4149a386ed4db17c6cf80"
SHARED_SHA256 = "71b843d5272ff0bed52b2ad89674959c55e041d7b2c6b0e1ad72fe65071e02d2"
MAX_JSONL = 64 * 1024 * 1024
MAX_EXPORT = 2 * 1024 * 1024
MAX_NATIVE = 1024 * 1024
MAX_SOURCE = 16384
MARKER_TEXT = "P15_NATIVE_RP12_ORIGINAL_FIXTURE"
MARKER = re.compile(
    r"^\s*p15_native_rp12_original_export_test.go:[0-9]+: P15_NATIVE_RP12_ORIGINAL_FIXTURE "
    r"sha256=([0-9a-f]{64}) bytes=([1-9][0-9]*) base64=([A-Za-z0-9+/]+={0,2})$"
)

# Derived from the pinned complete original Go expressions, without source edits.
# Duplicate programs stay separate when their original oracle obligations differ.
CATALOGUE_PATH = "tests/language-v2/native-rp12-original-cases.json"
CATALOGUE_SHA256 = "3f91519429fc3d4d99724435df1bb0ba3a201da503231fc02911c991fa959ef7"
CATALOGUE_BYTES = 99962


def read_catalogue() -> dict[str, Any]:
    raw = (Path(__file__).resolve().parents[2] / CATALOGUE_PATH).read_bytes()
    if len(raw) != CATALOGUE_BYTES or sha(raw) != CATALOGUE_SHA256:
        raise FixtureRejected("ORIGINAL_RP12_CATALOGUE_CHANGED")
    return _json(raw)


CONTRACT = read_catalogue()


@dataclass(frozen=True)
class Case:
    name: str
    source: bytes
    original_case: dict[str, Any]

    @property
    def export_test(self) -> str:
        return f"{EXPORT_PARENT}/{self.name}"

    @property
    def partition(self) -> str:
        return self.original_case["prerequisitePartition"]


def original_cases() -> tuple[Case, ...]:
    root = Path(__file__).resolve().parents[2]
    if CONTRACT["schema"] != "cherri-native-rp12-original-catalogue/1" or CONTRACT["originalMapSha256"] != ORIGINAL_MAP_SHA256:
        raise FixtureRejected("ORIGINAL_RP12_CATALOGUE_AUTHORITY")
    if len(CONTRACT["originalFiles"]) != 9:
        raise FixtureRejected("ORIGINAL_RP12_SOURCE_INVENTORY")
    for pin in CONTRACT["originalFiles"]:
        raw = (root / pin["repositoryPath"]).read_bytes()
        if len(raw) != pin["bytes"] or sha(raw) != pin["sha256"]:
            raise FixtureRejected("ORIGINAL_RP12_SOURCE_CHANGED")
    if sha((Path(__file__).parent / "native_go_fixture_stream.py").read_bytes()) != SHARED_SHA256:
        raise FixtureRejected("ORIGINAL_SHARED_STREAM_CHANGED")
    result = tuple(Case(spec["mapId"], spec["source"].encode("utf-8"),
        {key: value for key, value in spec.items() if key not in ("source", "sourceExpression")})
        for spec in CONTRACT["cases"])
    if len(result) != 50 or len({case.name for case in result}) != 50:
        raise FixtureRejected("ORIGINAL_RP12_POSITIVE_INVENTORY")
    counts = {kind: sum(case.partition == kind for case in result) for kind in ("self-contained", "provider", "image")}
    if counts != {"self-contained": 37, "provider": 8, "image": 5}:
        raise FixtureRejected("ORIGINAL_RP12_PREREQUISITE_PARTITION")
    if any(case.original_case["runtimeEvidence"] != "UNOBSERVED" for case in result):
        raise FixtureRejected("ORIGINAL_RP12_RUNTIME_SCOPE")
    return result


CASES = original_cases()
CASE_BY_TEST = {case.export_test: case for case in CASES}
ORIGINAL_PARENTS = ("TestP15RP12NamedEffectsUnit", "TestP15RP12NamedEffectsNative")


def original_events() -> tuple[str, ...]:
    root = Path(__file__).resolve().parents[2]
    raw = (root / "tools/language-acceptance/capture_order_inventory.json").read_bytes()
    if sha(raw) != "1e673b16b6931a9db55e38a5312cf721f78540181caa3d536346374acaabeae0":
        raise FixtureRejected("ORIGINAL_RP12_EVENT_SOURCE")
    names = _json(raw)["events"]["RP12"]
    if (type(names) is not list or names != CONTRACT["originalEvents"] or len(names) != 65 or len(set(names)) != 65
            or any(type(name) is not str or name.split("/")[0] not in ORIGINAL_PARENTS for name in names)
            or any(parent not in names for parent in ORIGINAL_PARENTS)):
        raise FixtureRejected("ORIGINAL_RP12_EVENTS")
    return tuple(names)


ORIGINAL_EVENTS = original_events()
REQUIRED_EVENTS = frozenset((*ORIGINAL_EVENTS, EXPORT_PARENT, *CASE_BY_TEST))
if len(REQUIRED_EVENTS) != 116:
    raise FixtureRejected("COMPLETE_RP12_EVENT_INVENTORY")
EXPORT_COMMAND = (
    "go", "test", "-json", "-count=1", "-timeout=3m", "-run",
    "^(" + "|".join((*ORIGINAL_PARENTS, EXPORT_PARENT)) + ")$", ".",
)
EXPORT_KEYS = frozenset((
    "schema", "requirement", "caseId", "originalMapSha256", "originalCase",
    "sourceBase64", "sourceSha256", "sourceBytes", "unsignedBase64", "unsignedSha256", "unsignedBytes",
    "actionCount", "workflowName", "workflowNamePresent", "nativeExecuted", "formalAcceptanceGranted",
))


def same_json(left: Any, right: Any) -> bool:
    # JSON kinds are exact: False may not substitute for numeric zero.
    return json.dumps(left, sort_keys=True, ensure_ascii=False, separators=(",", ":"), allow_nan=False) == json.dumps(right, sort_keys=True, ensure_ascii=False, separators=(",", ":"), allow_nan=False)


def validate_fixture(value: Any, case: Case) -> dict[str, Any]:
    if type(value) is not dict or set(value) != EXPORT_KEYS:
        raise FixtureRejected("EXPORT_SCHEMA")
    fixed = {
        "schema": EXPORT_SCHEMA, "requirement": "RP12", "caseId": case.name,
        "originalMapSha256": ORIGINAL_MAP_SHA256, "originalCase": case.original_case,
        "sourceSha256": sha(case.source), "sourceBytes": len(case.source),
    }
    if any(not same_json(value[key], expected) for key, expected in fixed.items()):
        raise FixtureRejected("EXPORT_CASE_BINDING")
    if value["nativeExecuted"] is not False or value["formalAcceptanceGranted"] is not False:
        raise FixtureRejected("EXPORT_OVERCLAIM")
    source = _decoded(value["sourceBase64"], value["sourceBytes"], value["sourceSha256"], MAX_SOURCE)
    raw = _decoded(value["unsignedBase64"], value["unsignedBytes"], value["unsignedSha256"], MAX_NATIVE)
    if source != case.source:
        raise FixtureRejected("ORIGINAL_SOURCE_BYTES")
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
    for action in actions:
        if type(action) is not dict or type(action.get("WFWorkflowActionIdentifier")) is not str or not action["WFWorkflowActionIdentifier"]:
            raise FixtureRejected("WORKFLOW_ACTION_ENVELOPE")
        # Do not normalize a legitimate omitted envelope into an empty mapping.
        # A present envelope must remain a dictionary; Nothing has no parameters.
        if "WFWorkflowActionParameters" in action:
            parameters = action["WFWorkflowActionParameters"]
            if type(parameters) is not dict:
                raise FixtureRejected("WORKFLOW_ACTION_PARAMETERS")
            if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.nothing" and parameters:
                raise FixtureRejected("NOTHING_HAS_PARAMETERS")
    if type(value["workflowNamePresent"]) is not bool or type(value["workflowName"]) is not str:
        raise FixtureRejected("WORKFLOW_NAME")
    if value["workflowNamePresent"] != ("WFWorkflowName" in workflow) or value["workflowName"] != workflow.get("WFWorkflowName", ""):
        raise FixtureRejected("WORKFLOW_NAME_BINDING")
    return value


def parse_exports(raw: bytes, *, expected_sha256: str) -> tuple[dict[str, Any], ...]:
    exports = parse_complete_exports(
        raw, expected_sha256=expected_sha256, package=PACKAGE,
        required_events=REQUIRED_EVENTS, case_by_test=CASE_BY_TEST,
        marker_text=MARKER_TEXT, marker=MARKER, validate_fixture=validate_fixture,
        max_jsonl=MAX_JSONL, max_export=MAX_EXPORT,
    )
    return tuple(exports[case.export_test] for case in CASES)
