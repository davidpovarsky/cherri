"""Collect original typed defaults without inventing native runtime values."""
from __future__ import annotations
from pathlib import Path
import plistlib
import re
from typing import Any
from native_go_fixture_stream import FixtureRejected, _decoded, _json, parse_complete_exports, sha

ROOT=Path(__file__).resolve().parents[2]
CATALOG_PATH="tests/native-runtime/typed-default-original-cases.json"
CATALOG_SHA="89312716103525e1744de81da556d92f7b14b393ffa0ad686e41954b6e4f8786"
_catalog_raw=(ROOT/CATALOG_PATH).read_bytes()
if sha(_catalog_raw)!=CATALOG_SHA:
    raise FixtureRejected("ORIGINAL_TYPED_CATALOG_CHANGED")
CATALOG=_json(_catalog_raw)
if CATALOG.get("schema")!="cherri-native-typed-default-catalog/1":
    raise FixtureRejected("ORIGINAL_TYPED_CATALOG_SCHEMA")
for _path,_digest in CATALOG["authorities"].items():
    if sha((ROOT/_path).read_bytes())!=_digest:
        raise FixtureRejected("ORIGINAL_TYPED_AUTHORITY_CHANGED")

PACKAGE="github.com/electrikmilk/cherri"
EXPORT_SCHEMA="cherri-native-typed-default-fixture/1"
EXPORT_PARENT="TestP15NativeTypedDefaultFixtureExport"
CASES=tuple(CATALOG["cases"])
if len(CASES)!=14 or len({c["id"] for c in CASES})!=14:
    raise FixtureRejected("ORIGINAL_TYPED_CASE_INVENTORY")
ORIGINAL_PARENTS=tuple(CATALOG["originalParents"])
ORIGINAL_EVENTS=frozenset(CATALOG["originalEvents"])
if len(ORIGINAL_PARENTS)!=4 or len(ORIGINAL_EVENTS)!=511:
    raise FixtureRejected("ORIGINAL_TYPED_EVENT_INVENTORY")
CASE_BY_TEST={EXPORT_PARENT+"/"+case["id"]:case for case in CASES}
REQUIRED_EVENTS=ORIGINAL_EVENTS | frozenset([EXPORT_PARENT,*CASE_BY_TEST])
EXPORT_COMMAND=("go","test","-json","-count=1","-timeout=3m","-run",
                "^("+"|".join((*ORIGINAL_PARENTS,EXPORT_PARENT))+")$",".")
MARKER_TEXT="P15_NATIVE_TYPED_DEFAULT_FIXTURE"
MARKER=re.compile(r"^\s*p15_native_typed_default_export_test.go:[0-9]+: "
                  r"P15_NATIVE_TYPED_DEFAULT_FIXTURE sha256=([0-9a-f]{64}) "
                  r"bytes=([1-9][0-9]*) base64=([A-Za-z0-9+/]+={0,2})$")
KEYS=frozenset((
    "schema","dialect","requirement","caseId","catalogPath","catalogSha256",
    "sourceAuthorityPath","sourceAuthoritySha256","originalUnitTest","originalNativeTest",
    "originalSourceHelper","originalGraphOracle","compileFilename","sourceCase",
    "sourceBase64","sourceSha256","sourceBytes","unsignedBase64","unsignedSha256","unsignedBytes",
    "actionCount","actionPoints","workflowName","workflowNamePresent",
    "observationMode","expectedLanguageSemantics","requiredEffects",
    "nativeExecuted","rawNativeTypesObserved","rawNativeValuesObserved","formalAcceptanceGranted",
))

def exact(value: Any,expected: Any) -> bool:
    if type(value) is not type(expected):
        return False
    if type(value) is dict:
        return set(value)==set(expected) and all(exact(value[k],v) for k,v in expected.items())
    if type(value) is list:
        return len(value)==len(expected) and all(exact(a,b) for a,b in zip(value,expected))
    return value==expected

def validate_fixture(value: Any,case: dict[str,Any]) -> dict[str,Any]:
    if type(value) is not dict or set(value)!=KEYS:
        raise FixtureRejected("EXPORT_SCHEMA")
    fixed={"schema":EXPORT_SCHEMA,"dialect":"v2","requirement":case["requirement"],"caseId":case["caseId"],
           "catalogPath":CATALOG_PATH,"catalogSha256":CATALOG_SHA,
           "sourceAuthorityPath":case["sourceAuthorityPath"],
           "sourceAuthoritySha256":CATALOG["authorities"][case["sourceAuthorityPath"]]}
    for name in ("originalUnitTest","originalNativeTest","originalSourceHelper","originalGraphOracle",
                 "compileFilename","sourceCase","observationMode","expectedLanguageSemantics","requiredEffects"):
        fixed[name]=case[name]
    if any(not exact(value[k],v) for k,v in fixed.items()):
        raise FixtureRejected("ORIGINAL_TYPED_CONTRACT")
    if any(value[k] is not False for k in ("nativeExecuted","rawNativeTypesObserved","rawNativeValuesObserved","formalAcceptanceGranted")):
        raise FixtureRejected("EXPORT_OVERCLAIM")
    source=_decoded(value["sourceBase64"],value["sourceBytes"],value["sourceSha256"],1024*1024)
    if source!=case["source"].encode():
        raise FixtureRejected("ORIGINAL_SOURCE_BYTES")
    native=_decoded(value["unsignedBase64"],value["unsignedBytes"],value["unsignedSha256"],8*1024*1024)
    try:
        workflow=plistlib.loads(native)
    except (ValueError,TypeError,plistlib.InvalidFileException,OverflowError,RecursionError) as error:
        raise FixtureRejected("INVALID_ORIGINAL_PLIST") from error
    if type(workflow) is not dict or type(workflow.get("WFWorkflowActions")) is not list:
        raise FixtureRejected("ORIGINAL_WORKFLOW_ACTIONS")
    actions=workflow["WFWorkflowActions"]
    if type(value["actionCount"]) is not int or not 0<value["actionCount"]==len(actions)<=5000:
        raise FixtureRejected("ORIGINAL_ACTION_COUNT")
    for action in actions:
        if type(action) is not dict or type(action.get("WFWorkflowActionIdentifier")) is not str or not action["WFWorkflowActionIdentifier"]:
            raise FixtureRejected("ORIGINAL_ACTION_ENVELOPE")
        if "WFWorkflowActionParameters" in action and type(action["WFWorkflowActionParameters"]) is not dict:
            raise FixtureRejected("ORIGINAL_ACTION_PARAMETERS")
    present="WFWorkflowName" in workflow
    if type(value["workflowNamePresent"]) is not bool or value["workflowNamePresent"]!=present or type(value["workflowName"]) is not str:
        raise FixtureRejected("WORKFLOW_NAME_PRESENCE")
    if value["workflowName"]!=workflow.get("WFWorkflowName",""):
        raise FixtureRejected("WORKFLOW_NAME_BINDING")
    points=value["actionPoints"]
    if type(points) is not dict or not {"observation","final_return"}.issubset(points):
        raise FixtureRejected("ORIGINAL_GRAPH_POINTS")
    for name,point in points.items():
        if type(name) is not str or not name or type(point) is not dict or set(point)!={"index","identifier","uuidPresent","uuid"}:
            raise FixtureRejected("GRAPH_POINT_SCHEMA")
        index=point["index"]
        if type(index) is not int or not 0<=index<len(actions):
            raise FixtureRejected("GRAPH_POINT_INDEX")
        action=actions[index];parameters=action.get("WFWorkflowActionParameters",{})
        if type(point["identifier"]) is not str or point["identifier"]!=action["WFWorkflowActionIdentifier"]:
            raise FixtureRejected("GRAPH_POINT_ACTION")
        if type(point["uuidPresent"]) is not bool or point["uuidPresent"]!=("UUID" in parameters) or type(point["uuid"]) is not str or point["uuid"]!=parameters.get("UUID",""):
            raise FixtureRejected("GRAPH_POINT_UUID")
    if actions[points["final_return"]["index"]]["WFWorkflowActionIdentifier"]!="is.workflow.actions.output":
        raise FixtureRejected("ORIGINAL_FINAL_RETURN")
    for effect in case["requiredEffects"]:
        if type(effect) is not dict or set(effect)!={"action","parameter","value"}:
            raise FixtureRejected("ORIGINAL_EFFECT_CONTRACT")
    if case["requiredEffects"]:
        identifiers={e["action"] for e in case["requiredEffects"]}
        actual=[a for a in actions if a["WFWorkflowActionIdentifier"] in identifiers]
        expected=[{"WFWorkflowActionIdentifier":e["action"],"WFWorkflowActionParameters":{e["parameter"]:e["value"]}} for e in case["requiredEffects"]]
        if not exact(actual,expected):
            raise FixtureRejected("ORIGINAL_SIDE_EFFECTS")
    return value

def parse_exports(raw: bytes, *, expected_sha256: str) -> tuple[dict[str,Any],...]:
    exports=parse_complete_exports(raw,expected_sha256=expected_sha256,package=PACKAGE,
        required_events=REQUIRED_EVENTS,case_by_test=CASE_BY_TEST,marker_text=MARKER_TEXT,marker=MARKER,
        validate_fixture=validate_fixture,max_jsonl=64*1024*1024,max_export=16*1024*1024)
    return tuple(exports[EXPORT_PARENT+"/"+case["id"]] for case in CASES)
