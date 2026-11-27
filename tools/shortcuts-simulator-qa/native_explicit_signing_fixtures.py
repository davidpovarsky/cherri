"""Original BRG26 A-then-B fixtures, without a live signing/native claim."""
from __future__ import annotations
from pathlib import Path
import plistlib
import re
from typing import Any
from native_go_fixture_stream import FixtureRejected, _decoded, parse_complete_exports, sha

PACKAGE = "github.com/electrikmilk/cherri"
EXPORT_PARENT = "TestP15NativeExplicitSigningFixtureExport"
EXPORT_SCHEMA = "cherri-native-explicit-signing-fixture/1"
ORIGINAL_PARENTS = (
    "TestBackendRecovery26ExplicitBytesAfterSequentialCompilation",
    "TestBackendRecovery26ExplicitFileAfterSequentialCompilation",
    "TestBackendRecovery26MobileAfterSequentialCompilation",
)
ORIGINAL_EVENTS = frozenset(
    list(ORIGINAL_PARENTS)
    + [p+"/"+protocol+"/"+mode for p in ORIGINAL_PARENTS[:2]
       for protocol in ("multipart", "json") for mode in ("v2", "legacy")]
    + [ORIGINAL_PARENTS[2]+"/"+mode for mode in ("v2", "legacy")]
)
CASES = ("v2", "legacy")
CASE_BY_TEST = {EXPORT_PARENT+"/"+mode: mode for mode in CASES}
REQUIRED_EVENTS = ORIGINAL_EVENTS | frozenset([EXPORT_PARENT, *CASE_BY_TEST])
EXPORT_COMMAND = ("go", "test", "-json", "-count=1", "-timeout=3m",
                  "-run", "^("+"|".join((*ORIGINAL_PARENTS,EXPORT_PARENT))+")$", ".")
MARKER_TEXT = "P15_NATIVE_EXPLICIT_SIGNING_FIXTURE"
MARKER = re.compile(r"^\s*p15_native_explicit_signing_export_test.go:[0-9]+: "
                    r"P15_NATIVE_EXPLICIT_SIGNING_FIXTURE sha256=([0-9a-f]{64}) "
                    r"bytes=([1-9][0-9]*) base64=([A-Za-z0-9+/]+={0,2})$")
AUTHORITY_PATH = "p15_signing_explicit_joined_test.go"
AUTHORITY_SHA = "ec54d81c51bc51c2a7b96b48437f2362f8bb4f88e5f25fb113a55bfc952e8b01"
EARLIER = "BRG26_A_EARLIER"
CURRENT = "BRG26_B_CURRENT_שלום"
CURRENT_NAME = "BRG26 B שלום"
SOURCE_A = ('text("'+EARLIER+'")\n').encode()
SOURCE_B = ('text("'+CURRENT+'")\n').encode()
KEYS = frozenset((
    "schema", "requirement", "caseId", "dialect", "originalHelper", "originalGraphOracle",
    "sourceAuthorityPath", "sourceAuthoritySha256", "originalProductionTests", "compileOrder",
    "earlierName", "currentName", "earlierSource", "currentSource", "earlierUnsigned",
    "currentUnsigned", "ambientUnsigned", "ambientInputBasename", "ambientOutputBasename",
    "ambientBasename", "ambientWorkflowName", "explicitInputBasename", "explicitOutputBasename",
    "existingDestination", "expectedObservations", "liveSigningExecuted", "nativeExecuted",
    "formalAcceptanceGranted",
))
BLOB_KEYS = frozenset(("base64", "sha256", "bytes"))
PLIST_KEYS = BLOB_KEYS | frozenset(("actionCount", "workflowNamePresent", "workflowName"))

def original_authorities() -> None:
    root = Path(__file__).resolve().parents[2]
    for path, digest in (
        (AUTHORITY_PATH, AUTHORITY_SHA),
        ("tools/language-acceptance/explicit_signing_execution.go",
         "57ed2d42872a880520b29a9f3c5fc14d34c1a9eb2c7d2c768c2bf9d68d0771fa"),
    ):
        if sha((root/path).read_bytes()) != digest:
            raise FixtureRejected("ORIGINAL_BRG26_AUTHORITY_CHANGED")

original_authorities()

def expected_observations(mode: str) -> dict[str, Any]:
    final = ({"kind":"None", "presence":"absent", "items":0} if mode=="v2"
             else {"kind":"Text", "presence":"one-item", "value":CURRENT})
    return {"intermediateText":CURRENT, "finalReturn":final,
            "earlierTextForbidden":EARLIER, "originalV2FallthroughPreserved":mode=="v2"}

def source_blob(value: Any, expected: bytes) -> bytes:
    if type(value) is not dict or set(value) != BLOB_KEYS:
        raise FixtureRejected("SOURCE_BLOB_SCHEMA")
    raw = _decoded(value["base64"],value["bytes"],value["sha256"],1024)
    if raw != expected:
        raise FixtureRejected("ORIGINAL_SOURCE_BYTES")
    return raw

def plist_blob(value: Any, mode: str, literal: str, name: str) -> bytes:
    if type(value) is not dict or set(value) != PLIST_KEYS:
        raise FixtureRejected("PLIST_BLOB_SCHEMA")
    raw = _decoded(value["base64"],value["bytes"],value["sha256"],1024*1024)
    try:
        workflow = plistlib.loads(raw)
    except (ValueError, TypeError, plistlib.InvalidFileException, OverflowError, RecursionError) as exc:
        raise FixtureRejected("INVALID_PLIST") from exc
    if type(workflow) is not dict or type(workflow.get("WFWorkflowActions")) is not list:
        raise FixtureRejected("WORKFLOW_ACTIONS")
    actions = workflow["WFWorkflowActions"]
    count = 2 if mode=="v2" else 1
    if type(value["actionCount"]) is not int or value["actionCount"] != count or len(actions) != count:
        raise FixtureRejected("ORIGINAL_COMPLETE_ACTION_COUNT")
    if any(type(a) is not dict for a in actions):
        raise FixtureRejected("ACTION_ENVELOPE")
    if type(actions[0].get("WFWorkflowActionParameters")) is not dict:
        raise FixtureRejected("ACTION_ENVELOPE")
    # The original bare Nothing action omits its optional parameter dictionary.
    # Preserve that absence; a present dictionary must remain empty.
    if mode == "v2" and "WFWorkflowActionParameters" in actions[1]:
        parameters = actions[1]["WFWorkflowActionParameters"]
        if type(parameters) is not dict or parameters:
            raise FixtureRejected("ACTION_ENVELOPE")
    if actions[0].get("WFWorkflowActionIdentifier") != "is.workflow.actions.gettext":
        raise FixtureRejected("ORIGINAL_TEXT_PRODUCER")
    text = actions[0]["WFWorkflowActionParameters"].get("WFTextActionText")
    if type(text) is dict:
        if text.get("WFSerializationType") != "WFTextTokenString" or type(text.get("Value")) is not dict:
            raise FixtureRejected("ORIGINAL_LITERAL_ENCODING")
        token = text["Value"]
        if "attachmentsByRange" in token and token["attachmentsByRange"] != {}:
            raise FixtureRejected("STALE_LITERAL_REFERENCES")
        text = token.get("string")
    if type(text) is not str or text != literal:
        raise FixtureRejected("ORIGINAL_LITERAL_VALUE")
    if mode=="v2" and actions[1].get("WFWorkflowActionIdentifier") != "is.workflow.actions.nothing":
        raise FixtureRejected("ORIGINAL_V2_FALLTHROUGH")
    if type(value["workflowNamePresent"]) is not bool or type(value["workflowName"]) is not str:
        raise FixtureRejected("WORKFLOW_NAME_SCHEMA")
    present = "WFWorkflowName" in workflow
    if value["workflowNamePresent"] != present or value["workflowName"] != workflow.get("WFWorkflowName",""):
        raise FixtureRejected("WORKFLOW_NAME_BINDING")
    if present and workflow["WFWorkflowName"] != name:
        raise FixtureRejected("WORKFLOW_CURRENT_NAME")
    return raw

def validate_fixture(value: Any, mode: str) -> dict[str, Any]:
    if type(value) is not dict or set(value) != KEYS:
        raise FixtureRejected("EXPORT_SCHEMA")
    fixed = {
        "schema":EXPORT_SCHEMA, "requirement":"BRG26", "caseId":mode, "dialect":mode,
        "originalHelper":"p15BRG26StaleState", "originalGraphOracle":"p15BRG26NativeLiteral",
        "sourceAuthorityPath":AUTHORITY_PATH, "sourceAuthoritySha256":AUTHORITY_SHA,
        "originalProductionTests":list(ORIGINAL_PARENTS), "compileOrder":["A","B"],
        "earlierName":"BRG26 A earlier", "currentName":CURRENT_NAME,
        "ambientInputBasename":"A earlier.shortcut", "ambientOutputBasename":"A stale output.shortcut",
        "ambientBasename":"BRG26_STALE_A_NAME", "ambientWorkflowName":"BRG26_STALE_A_WORKFLOW",
        "explicitInputBasename":"B current unsigned.shortcut", "explicitOutputBasename":CURRENT_NAME+".shortcut",
        "existingDestination":"BRG26_EXISTING_DESTINATION",
    }
    if any(type(value[k]) is not type(v) or value[k] != v for k,v in fixed.items()):
        raise FixtureRejected("EXPORT_CASE_BINDING")
    # Encode the independently authored expectation to distinguish bool/int too.
    import json
    if json.dumps(value["expectedObservations"],sort_keys=True) != json.dumps(expected_observations(mode),sort_keys=True):
        raise FixtureRejected("ORIGINAL_NATIVE_EXPECTATIONS")
    if any(value[k] is not False for k in ("liveSigningExecuted","nativeExecuted","formalAcceptanceGranted")):
        raise FixtureRejected("EXPORT_OVERCLAIM")
    source_blob(value["earlierSource"],SOURCE_A)
    source_blob(value["currentSource"],SOURCE_B)
    a = plist_blob(value["earlierUnsigned"],mode,EARLIER,"BRG26 A earlier")
    b = plist_blob(value["currentUnsigned"],mode,CURRENT,CURRENT_NAME)
    plist_blob(value["ambientUnsigned"],mode,EARLIER,"BRG26 A earlier")
    if a == b:
        raise FixtureRejected("A_AND_B_MUST_DIFFER")
    return value

def parse_exports(raw: bytes, *, expected_sha256: str) -> tuple[dict[str,Any],...]:
    exports = parse_complete_exports(
        raw,expected_sha256=expected_sha256,package=PACKAGE,
        required_events=REQUIRED_EVENTS,case_by_test=CASE_BY_TEST,
        marker_text=MARKER_TEXT,marker=MARKER,validate_fixture=validate_fixture,
        max_jsonl=16*1024*1024,max_export=4*1024*1024,
    )
    return tuple(exports[EXPORT_PARENT+"/"+mode] for mode in CASES)
