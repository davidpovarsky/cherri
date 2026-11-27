"""Complete original numeric/index/Unicode fixture collection.

All 37 original positive programs and all 691 original Go events are required.
Expected observations describe source contracts. Native type/cardinality,
intermediate effects and rendering remain unobserved until a genuine Apple
producer supplies them; this collector never grants runtime acceptance.
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
EXPORT_PARENT = "TestP15NativeNumericContractFixtureExport"
EXPORT_SCHEMA = "cherri-native-numeric-fixture/1"
# Thirty-seven full originals plus their original graph/roundtrip logs.
MAX_JSONL = 64 * 1024 * 1024
MAX_EXPORT = 2 * 1024 * 1024
MAX_NATIVE = 1024 * 1024
MAX_SOURCE = 16384
MARKER_TEXT = "P15_NATIVE_NUMERIC_FIXTURE"
MARKER = re.compile(
    r"^\s*p15_native_numeric_contract_export_test.go:[0-9]+: P15_NATIVE_NUMERIC_FIXTURE "
    r"sha256=([0-9a-f]{64}) bytes=([1-9][0-9]*) base64=([A-Za-z0-9+/]+={0,2})$"
)

CONTRACT = _json("{\"single\":[{\"id\":\"RP15\",\"unit\":\"TestP15RP15CompoundOnceUnit\",\"native\":\"TestP15RP15CompoundOnceNative\",\"path\":\"tests/language-v2/v2-rp15-compound-once.cherri\",\"filename\":\"combined-rp15.cherri\",\"oracle\":\"p15AMGraph\",\"expect\":{\"return\":{\"kind\":\"Number\",\"presence\":\"one-item\",\"value\":\"11\"},\"clipboard\":{\"kind\":\"NumericText\",\"value\":\"41234\",\"meaning\":\"four RHS evaluations in source order 1,2,3,4\",\"nativeTextRendering\":\"UNOBSERVED; retain exact clipboard text\"}},\"fileSHA\":\"56c9afc1083655ce5b7ca7545614b71e32addeb4dae6dd0f2c104d1c5042bb26\",\"fileBytes\":741},{\"id\":\"RP17\",\"unit\":\"TestP15RP17ComparisonsArithmeticUnit\",\"native\":\"TestP15RP17ComparisonsArithmeticNative\",\"path\":\"tests/language-v2/v2-rp17-comparisons-arithmetic.cherri\",\"filename\":\"combined-rp17.cherri\",\"oracle\":\"p15AMGraph\",\"expect\":{\"return\":{\"kind\":\"Text\",\"presence\":\"one-item\",\"template\":\"gt={greater};eq={equal};ne={unequal};div={divided};mod={remainder};mixed={mixed}\"},\"intermediate\":{\"greater\":{\"kind\":\"Bool\",\"value\":\"false\"},\"equal\":{\"kind\":\"Bool\",\"value\":\"true\"},\"unequal\":{\"kind\":\"Bool\",\"value\":\"false\"},\"divided\":{\"kind\":\"Number\",\"value\":\"2.5\"},\"remainder\":{\"kind\":\"Number\",\"value\":\"1\"},\"mixed\":{\"kind\":\"Number\",\"value\":\"6.5\"}},\"nativeTextRendering\":\"UNOBSERVED; retain exact returned text and authentic original producer values\"},\"fileSHA\":\"e5e43ba0823ea329304ecaf9e78f6a2f302acb9489bd0245f9616bd6bd6562ca\",\"fileBytes\":350},{\"id\":\"RP19\",\"unit\":\"TestP15RP19LiteralUnicodeUnit\",\"native\":\"TestP15RP19LiteralUnicodeNativeRoundTrip\",\"path\":\"tests/language-v2/v2-rp19-literal-unicode.cherri\",\"filename\":\"rp19-joined.cherri\",\"oracle\":\"p15UTGraph\",\"expect\":{\"return\":{\"kind\":\"Text\",\"presence\":\"one-item\",\"value\":\"שָׁלוֹם👩‍💻é|42falseαב|éα|🌍Ω\\n\"},\"textComparison\":\"exact UTF-8; no normalization\"},\"fileSHA\":\"57ecba5206a2418b595f5f5d023dc148fe96e4531ae80029ad7383d441913158\",\"fileBytes\":241},{\"id\":\"BRG08\",\"unit\":\"TestBackendRecovery08InlineCodecUnit\",\"native\":\"TestBackendRecovery08InlineCodecNativeRoundTrip\",\"path\":\"tests/language-v2/v2-brg08-inline-codec.cherri\",\"filename\":\"brg08-joined.cherri\",\"oracle\":\"p15UTGraph\",\"expect\":{\"return\":{\"kind\":\"Text\",\"presence\":\"one-item\",\"value\":\"שָׁלוֹם👩‍💻é|{literal}|ΔΔ|éΔ|🌍{seed}|JSON:{\\\"n\\\":42}|\\\\path\\\\d+{seed}\\\\n\\n\"},\"textComparison\":\"exact UTF-8; no normalization\"},\"fileSHA\":\"5b59664946c8e2ecf9841512839db6670df622538f065aa408b696fc0cb72f0d\",\"fileBytes\":658}],\"index\":[{\"id\":\"RP21\",\"unit\":\"TestP15RP21SafeIndexUnit\",\"native\":\"TestP15RP21SafeIndexNative\",\"path\":\"tests/language-v2/v2-rp21-safe-indexing.json\",\"fileSHA\":\"88a967f31f326b8e247e657964e4a06324b8d8f58f70d6b9a1437e0a009e17ac\",\"fileBytes\":3604,\"specs\":[{\"name\":\"static_first\",\"mode\":\"int\",\"literal\":\"0\",\"outcome\":\"Text:alpha\"},{\"name\":\"static_last\",\"mode\":\"int\",\"literal\":\"2\",\"outcome\":\"Text:gamma\"},{\"name\":\"static_empty\",\"mode\":\"int\",\"literal\":\"0\",\"outcome\":\"None\"},{\"name\":\"static_negative\",\"mode\":\"int\",\"literal\":\"-1\",\"outcome\":\"None\"},{\"name\":\"static_out_of_range\",\"mode\":\"int\",\"literal\":\"3\",\"outcome\":\"None\"},{\"name\":\"static_uint64_max\",\"mode\":\"int\",\"literal\":\"18446744073709551615\",\"outcome\":\"None\"},{\"name\":\"static_fraction\",\"mode\":\"real\",\"literal\":\"0.5\",\"outcome\":\"None\"},{\"name\":\"static_negative_fraction\",\"mode\":\"real\",\"literal\":\"-0.5\",\"outcome\":\"None\"},{\"name\":\"static_integral_real\",\"mode\":\"real\",\"literal\":\"2.0\",\"outcome\":\"Text:gamma\"},{\"name\":\"dynamic_first\",\"mode\":\"var\",\"literal\":\"0\",\"outcome\":\"Text:alpha\"},{\"name\":\"dynamic_last\",\"mode\":\"var\",\"literal\":\"2\",\"outcome\":\"Text:gamma\"},{\"name\":\"dynamic_empty\",\"mode\":\"var\",\"literal\":\"0\",\"outcome\":\"None\"},{\"name\":\"dynamic_negative\",\"mode\":\"var\",\"literal\":\"-1\",\"outcome\":\"None\"},{\"name\":\"dynamic_out_of_range\",\"mode\":\"var\",\"literal\":\"3\",\"outcome\":\"None\"},{\"name\":\"dynamic_fraction\",\"mode\":\"var\",\"literal\":\"0.5\",\"outcome\":\"None\"},{\"name\":\"dynamic_negative_fraction\",\"mode\":\"var\",\"literal\":\"-0.5\",\"outcome\":\"None\"},{\"name\":\"inline_fraction\",\"mode\":\"inline\",\"literal\":\"0.5\",\"outcome\":\"None\"},{\"name\":\"dynamic_empty_text\",\"mode\":\"var\",\"literal\":\"0\",\"outcome\":\"Text:\"}]},{\"id\":\"BRG16\",\"unit\":\"TestP15BRG16NumericBoundsUnit\",\"native\":\"TestP15BRG16NumericBoundsNative\",\"path\":\"tests/language-v2/v2-brg16-numeric-bounds.json\",\"fileSHA\":\"a00fa42daf2f373930f5a82adf737955e5c409fab1ab4936a6224ac38ca38bc3\",\"fileBytes\":7694,\"specs\":[{\"name\":\"static_first\",\"mode\":\"int\",\"literal\":\"0\",\"outcome\":\"Text:start\"},{\"name\":\"static_last\",\"mode\":\"int\",\"literal\":\"2\",\"outcome\":\"Text:end\"},{\"name\":\"static_empty\",\"mode\":\"int\",\"literal\":\"0\",\"outcome\":\"None\"},{\"name\":\"static_negative\",\"mode\":\"int\",\"literal\":\"-1\",\"outcome\":\"None\"},{\"name\":\"static_out_of_range\",\"mode\":\"int\",\"literal\":\"3\",\"outcome\":\"None\"},{\"name\":\"static_fraction\",\"mode\":\"real\",\"literal\":\"0.5\",\"outcome\":\"None\"},{\"name\":\"static_integral_real\",\"mode\":\"real\",\"literal\":\"2.0\",\"outcome\":\"Text:end\"},{\"name\":\"dynamic_first\",\"mode\":\"compound\",\"literal\":\"0\",\"outcome\":\"Text:start\"},{\"name\":\"dynamic_last\",\"mode\":\"compound\",\"literal\":\"2\",\"outcome\":\"Text:end\"},{\"name\":\"dynamic_empty\",\"mode\":\"compound\",\"literal\":\"0\",\"outcome\":\"None\"},{\"name\":\"dynamic_negative\",\"mode\":\"compound\",\"literal\":\"-1\",\"outcome\":\"None\"},{\"name\":\"dynamic_out_of_range\",\"mode\":\"compound\",\"literal\":\"3\",\"outcome\":\"None\"},{\"name\":\"dynamic_fraction\",\"mode\":\"compound\",\"literal\":\"0.5\",\"outcome\":\"None\"},{\"name\":\"dynamic_negative_fraction\",\"mode\":\"compound\",\"literal\":\"-0.5\",\"outcome\":\"None\"},{\"name\":\"dynamic_empty_text\",\"mode\":\"compound\",\"literal\":\"0\",\"outcome\":\"Text:\"}]}],\"inventories\":[{\"file\":\"combined_arithmetic_inventory.json\",\"sha\":\"a2e1c955ac449597fe4bebe2575a3c7beaa5f241dba9b9bcc7c2317c76239943\",\"counts\":{\"RP15\":80,\"RP17\":57}},{\"file\":\"combined_index_arithmetic_inventory.json\",\"sha\":\"c608d1f5b126a206c03f55daae03c97077e58aefb58f57488f90a7b412be95c9\",\"counts\":{\"RP21\":234,\"BRG16\":213}},{\"file\":\"combined_unicode_inventory.json\",\"sha\":\"2e66c5b40cdb66c26b7c9a7b6e07b0ba2ef072ec140e2aa528235a95ade93281\",\"counts\":{\"RP19\":46,\"BRG08\":61}}]}")


@dataclass(frozen=True)
class Case:
    requirement: str
    name: str
    unit_test: str
    native_test: str
    source_path: str
    source_file_sha256: str
    compile_filename: str
    graph_oracle: str
    source: bytes
    observations: dict[str, Any]

    @property
    def export_test(self) -> str:
        return f"{EXPORT_PARENT}/{self.requirement}/{self.name}"


def pinned_file(root: Path, spec: dict[str, Any]) -> bytes:
    raw = (root / spec["path"]).read_bytes()
    if len(raw) != spec["fileBytes"] or sha(raw) != spec["fileSHA"]:
        raise FixtureRejected("ORIGINAL_NUMERIC_SOURCE_FILE_CHANGED")
    return raw


def original_cases() -> tuple[Case, ...]:
    root = Path(__file__).resolve().parents[2]
    result = []
    for spec in CONTRACT["single"]:
        source = pinned_file(root, spec)
        result.append(Case(spec["id"], "full-original-scenario", spec["unit"], spec["native"],
            spec["path"], spec["fileSHA"], spec["filename"], spec["oracle"], source, spec["expect"]))
    for spec in CONTRACT["index"]:
        fixture = _json(pinned_file(root, spec))
        if fixture["schema"] != "cherri.p15.combined-index/v1" or fixture["requirement"] != spec["id"] or len(fixture["cases"]) != len(spec["specs"]) or len(fixture["invalid"]) != 2:
            raise FixtureRejected("ORIGINAL_INDEX_INVENTORY")
        for i, expected in enumerate(spec["specs"]):
            case = fixture["cases"][i]
            if case["name"] != expected["name"] or type(case["source"]) is not str or not case["source"]:
                raise FixtureRejected("ORIGINAL_INDEX_ORDER")
            outcome = expected["outcome"]
            if outcome == "None":
                returned = {"kind": "None", "presence": "absent", "items": "0"}
                lookup = "forbidden"
            elif outcome.startswith("Text:"):
                returned = {"kind": "Text", "presence": "one-item", "value": outcome[5:]}
                lookup = "required"
            else:
                raise FixtureRejected("ORIGINAL_INDEX_OUTCOME")
            observations = {"return": returned,
                "index": {"sourceLiteral": expected["literal"], "lookup": lookup},
                "absenceAndEmptyTextAreDistinct": True}
            if spec["id"] == "BRG16":
                observations["clipboard"] = {"kind": "NumericText", "value": expected["literal"].removesuffix(".0"),
                    "nativeTextRendering": "UNOBSERVED; retain exact clipboard text",
                    "meaning": "actual fractional/Modulus/compound result before indexing"}
            result.append(Case(spec["id"], case["name"], spec["unit"], spec["native"],
                f'{spec["path"]}#/cases/{i}/source', spec["fileSHA"],
                "combined-" + spec["id"].lower() + "-" + case["name"] + ".cherri",
                "p15IXGraph", case["source"].encode(), observations))
    if len(result) != 37 or len({case.export_test for case in result}) != 37:
        raise FixtureRejected("ORIGINAL_POSITIVE_INVENTORY")
    return tuple(result)


CASES = original_cases()
CASE_BY_TEST = {case.export_test: case for case in CASES}
ORIGINAL_PARENTS = tuple(dict.fromkeys(parent for case in CASES for parent in (case.unit_test, case.native_test)))


def original_events() -> dict[str, tuple[str, ...]]:
    root = Path(__file__).resolve().parents[2]
    result = {}
    for spec in CONTRACT["inventories"]:
        raw = (root / "tools/language-acceptance" / spec["file"]).read_bytes()
        if sha(raw) != spec["sha"]:
            raise FixtureRejected("ORIGINAL_NUMERIC_EVENT_INVENTORY_CHANGED")
        value = _json(raw)
        for requirement, count in spec["counts"].items():
            names = value["events"][requirement]
            case = next(case for case in CASES if case.requirement == requirement)
            if (type(names) is not list or len(names) != count or len(set(names)) != count
                    or any(type(name) is not str or name.split("/")[0] not in (case.unit_test, case.native_test) for name in names)
                    or case.unit_test not in names or case.native_test not in names):
                raise FixtureRejected("ORIGINAL_NUMERIC_EVENT_INVENTORY")
            result[requirement] = tuple(names)
    return result


ORIGINAL_EVENTS = original_events()
REQUIRED_EVENTS = frozenset(
    [EXPORT_PARENT] + list(CASE_BY_TEST)
    + [name for names in ORIGINAL_EVENTS.values() for name in names]
)
if len(REQUIRED_EVENTS) != 729 or sum(len(names) for names in ORIGINAL_EVENTS.values()) != 691:
    raise FixtureRejected("INCOMPLETE_ORIGINAL_EVENT_INVENTORY")

EXPORT_COMMAND = (
    "go", "test", "-json", "-count=1", "-timeout=3m",
    "-run", "^(" + "|".join((*ORIGINAL_PARENTS, EXPORT_PARENT)) + ")$", ".",
)
EXPORT_KEYS = frozenset((
    "schema", "requirement", "caseId", "originalUnitTest", "originalNativeTest",
    "originalGraphOracle", "sourcePath", "sourceFileSha256", "compileFilename",
    "sourceBase64", "sourceSha256", "sourceBytes", "unsignedBase64", "unsignedSha256",
    "unsignedBytes", "actionCount", "workflowName", "workflowNamePresent",
    "observationMode", "expectedObservations", "nativeExecuted", "formalAcceptanceGranted",
))


def same_json(left: Any, right: Any) -> bool:
    # Preserve JSON scalar kinds; Python False == 0 cannot weaken the contract.
    return json.dumps(left, sort_keys=True, ensure_ascii=False, separators=(",", ":"), allow_nan=False) == json.dumps(right, sort_keys=True, ensure_ascii=False, separators=(",", ":"), allow_nan=False)


def validate_fixture(value: Any, case: Case) -> dict[str, Any]:
    if type(value) is not dict or set(value) != EXPORT_KEYS:
        raise FixtureRejected("EXPORT_SCHEMA")
    fixed = {
        "schema": EXPORT_SCHEMA, "requirement": case.requirement, "caseId": case.name,
        "originalUnitTest": case.unit_test, "originalNativeTest": case.native_test,
        "originalGraphOracle": case.graph_oracle, "sourcePath": case.source_path,
        "sourceFileSha256": case.source_file_sha256, "compileFilename": case.compile_filename,
        "sourceSha256": sha(case.source), "sourceBytes": len(case.source),
        "observationMode": "original-typed-output-and-side-effects",
        "expectedObservations": case.observations,
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
    if any(type(action) is not dict or type(action.get("WFWorkflowActionIdentifier")) is not str for action in actions):
        raise FixtureRejected("WORKFLOW_ACTION_ENVELOPE")
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
