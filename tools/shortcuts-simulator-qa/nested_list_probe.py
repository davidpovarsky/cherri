"""Fixed public nested-List observation; no compiler encoding or acceptance claim.

Only action UUIDs and control-group UUIDs vary in the independently authored
complete graph. The experimental row is deliberately confined to this fixture.
"""
import argparse
import base64
import binascii
import copy
import hashlib
import json
from pathlib import Path
import plistlib
import re

CONTROL = "qa-nested-list-control-v2"
CANDIDATE = "qa-nested-list-candidate-v2"
FIXTURES = (CONTROL, CANDIDATE)
SOURCE_HASHES = {
    CONTROL: "5452ff03c5c09f8fa7c9c511e988f398059ea5a901f00da63314cfeef7ede121",
    CANDIDATE: "b13ca17d1eb425e0b54854c922df46fc50cbcf1aff491f988191c774d2760299",
}
CONTRACT_HASHES = {'qa-nested-list-control-v2': 'c825bfff307008d032c08143de019a6125f825b138ed213c03ecae2e9c8f9feb', 'qa-nested-list-candidate-v2': '5aa826dce959a4667a3f498a5b6c9fae4c91a3f71eeb6209c3772e405ff5c9a8'}
PREFIXES = {CONTROL: "CHERRI_NESTED_LIST_CONTROL_V1:", CANDIDATE: "CHERRI_NESTED_LIST_CANDIDATE_V1:"}
FIELDS = {
    CONTROL: ("FLAT_COUNT", "FLAT_FIRST64", "FLAT_LAST64", "EMPTY_COUNT", "BOX_COUNT",
              "FIRST_TAG64", "LAST_TAG64", "FIRST_KEYS", "LAST_KEYS", "FIRST_ITEMS",
              "FIRST_VALUE64", "LAST_VALUE64", "LAST_ITEMS", "TEXT_TYPE64", "DICT_TYPE64",
              "FIRST_TYPE64", "LAST_TYPE64"),
    CANDIDATE: ("OUTER_COUNT", "OBSERVED", "FIRST_COUNT", "LAST_COUNT", "FIRST_FIRST64",
                "FIRST_LAST64", "LAST_FIRST64", "LAST_LAST64", "TEXT_TYPE64", "DICT_TYPE64",
                "FIRST_TYPE64", "LAST_TYPE64"),
}
GOALS = {
    CONTROL: {"FLAT_COUNT": "2", "FLAT_FIRST64": "TEVGVA==", "FLAT_LAST64": "UklHSFQ=",
              "EMPTY_COUNT": "0", "BOX_COUNT": "2", "FIRST_TAG64": "TEVGVA==",
              "LAST_TAG64": "RU1QVFk=", "FIRST_KEYS": "2", "LAST_KEYS": "2",
              "FIRST_ITEMS": "2", "FIRST_VALUE64": "SU5ORVJfQQ==", "LAST_VALUE64": "SU5ORVJfQg==",
              "LAST_ITEMS": "0"},
    CANDIDATE: {"OUTER_COUNT": "2", "OBSERVED": "Y", "FIRST_COUNT": "2", "LAST_COUNT": "0",
                "FIRST_FIRST64": "SU5ORVJfQQ==", "FIRST_LAST64": "SU5ORVJfQg==",
                "LAST_FIRST64": "UNOBSERVED", "LAST_LAST64": "UNOBSERVED"},
}
COUNTS = {CONTROL: 56, CANDIDATE: 78}
UUID_RE = re.compile(r"[0-9A-Fa-f]{8}(?:-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12}")
COMMON_MUTATIONS = (
    "action-removed", "extra-effect", "duplicate-producer", "missing-producer",
    "clear-borrows-input", "before-trace-time-lost", "trace-store-replaced",
    "after-trace-time-lost", "summary-field-static", "summary-range-shift",
    "trace-envelope-replaced", "final-output-replaced", "extra-terminal-nothing",
)
CONTROL_MUTATIONS = (
    "flat-order-swapped", "empty-control-singleton", "empty-clear-borrows-input",
    "first-array-flattened-text", "empty-array-value-omitted", "empty-field-removed",
    "dictionary-carriers-swapped", "raw-map-in-list-row", "outer-count-borrows-flat",
    "first-tag-wrong-key", "first-items-read-last-box", "first-count-borrows-last",
    "first-value-uses-last", "dictionary-type-borrows-text", "last-type-borrows-first-producer",
)
CANDIDATE_MUTATIONS = (
    "candidate-row-type-zero", "candidate-rows-flattened", "empty-row-removed",
    "empty-row-becomes-text", "inner-order-swapped", "first-row-cleared",
    "candidate-fixed-uuid-changed", "outer-count-borrows-text", "outer-guard-inverted",
    "outer-guard-threshold-one", "first-selector-last", "first-count-borrows-outer",
    "first-type-borrows-witness", "first-guard-uses-last-count", "inner-order-reads-last",
    "last-guard-uses-first-count", "last-inner-reads-first", "control-group-reused",
    "observed-flag-never-set", "summary-borrows-initializer",
)


def require(ok, code):
    if not ok:
        raise ValueError("NESTED_LIST_" + code)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def _json(raw):
    def unique(pairs):
        out = {}
        for key, value in pairs:
            require(key not in out, "DUPLICATE_JSON_KEY")
            out[key] = value
        return out
    def invalid(_):
        raise ValueError("NESTED_LIST_NONFINITE_JSON")
    return json.loads(raw, object_pairs_hook=unique, parse_constant=invalid)


def _same(left, right):
    # JSON equality must not equate Bool false and Number zero.
    return json.dumps(left, sort_keys=True, allow_nan=False, separators=(",", ":")) == \
        json.dumps(right, sort_keys=True, allow_nan=False, separators=(",", ":"))


def validate_contract(value):
    require(isinstance(value, dict) and value.get("fixture_id") in FIXTURES, "CONTRACT_FIXTURE")
    fid = value["fixture_id"]
    expected_keys = {"schema_version", "fixture_id", "source_path", "source_sha256",
                     "diagnostic_prefix", "field_order", "goal_fields", "trace_run_id", "trace_label",
                     "native_list_type_proved", "formal_requirements_proved", "runtime_status", "scope"}
    require(set(value) == expected_keys and type(value["schema_version"]) is int
            and value["schema_version"] == 1, "CONTRACT_SHAPE")
    require(value["source_path"] == "tools/shortcuts-simulator-qa/fixtures/" + fid + ".cherri"
            and value["source_sha256"] == SOURCE_HASHES[fid], "CONTRACT_SOURCE")
    require(value["diagnostic_prefix"] == PREFIXES[fid]
            and value["field_order"] == list(FIELDS[fid])
            and _same(value["goal_fields"], GOALS[fid]), "CONTRACT_GOALS")
    require(value["trace_run_id"] == fid and value["trace_label"] == fid[3:-3], "CONTRACT_TRACE")
    require(value["native_list_type_proved"] is False and value["formal_requirements_proved"] == []
            and value["runtime_status"] == "NOT_RUN" and isinstance(value["scope"], str)
            and 1 <= len(value["scope"]) <= 2000, "CONTRACT_NO_PROMOTION")
    return value


def contract(root, fixture_id):
    require(fixture_id in FIXTURES, "FIXTURE")
    folder = Path(root) / "tools/shortcuts-simulator-qa/fixtures"
    raw = (folder / (fixture_id + ".expected.json")).read_bytes()
    require(len(raw) <= 8192 and sha(raw) == CONTRACT_HASHES[fixture_id], "CONTRACT_BYTES")
    return validate_contract(_json(raw))


def verify_source(root, fixture_id, actual):
    value = contract(root, fixture_id)
    require(isinstance(actual, bytes) and sha(actual) == SOURCE_HASHES[fixture_id], "SOURCE_BYTES_DIFFER")
    require(actual.endswith(b"return finalSummary\n"), "EXPLICIT_MAIN_RETURN")
    return {"fixtureSha256": value["source_sha256"], "contractSha256": CONTRACT_HASHES[fixture_id],
            "scope": "PUBLIC_NESTED_LIST_OBSERVATION_ONLY", "appleRuntime": "NOT_RUN",
            "nativeListTypeProved": False, "formalAcceptanceGranted": False}


def _decode(value, limit, field):
    require(isinstance(value, str) and len(value) <= ((limit + 2) // 3) * 4, "BASE64_SIZE_" + field)
    try:
        raw = base64.b64decode(value, validate=True)
        decoded = raw.decode("utf-8", errors="strict")
    except (ValueError, binascii.Error, UnicodeError) as exc:
        raise ValueError("NESTED_LIST_BASE64_ENCODING_" + field) from exc
    require(len(raw) <= limit and base64.b64encode(raw).decode("ascii") == value,
            "BASE64_CANONICAL_" + field)
    return decoded


def validate_summary(summary, fixture_id):
    """Validate one semantic line. The caller separately authenticates its trace.

    A goal mismatch is a retained observation, not malformed transport. Even a
    shape match cannot establish the runtime type of a nested native List.
    """
    require(fixture_id in FIXTURES and isinstance(summary, str), "SUMMARY_FIXTURE")
    require(len(summary.encode("utf-8")) <= 8192 and summary.isascii()
            and "\n" not in summary and "\r" not in summary, "SUMMARY_BYTES")
    require(summary.startswith(PREFIXES[fixture_id]), "SUMMARY_PREFIX")
    parts = summary[len(PREFIXES[fixture_id]):].split(":")
    fields = {}
    require(len(parts) == len(FIELDS[fixture_id]), "SUMMARY_FIELD_COUNT")
    for part, key in zip(parts, FIELDS[fixture_id]):
        require(part.startswith(key + "=") and key not in fields, "SUMMARY_FIELD_ORDER")
        fields[key] = part[len(key) + 1:]
    decoded = {}
    for key, value in fields.items():
        if key == "OBSERVED":
            require(value in ("N", "Y"), "OBSERVED_FLAG")
        elif key.endswith("64"):
            if value == "UNOBSERVED":
                require(fixture_id == CANDIDATE and key not in ("TEXT_TYPE64", "DICT_TYPE64"),
                        "UNOBSERVED_FIELD")
            else:
                decoded[key] = _decode(value, 128 if key.endswith("TYPE64") else 256, key)
        else:
            require(re.fullmatch(r"0|[1-9][0-9]{0,3}", value) is not None
                    or fixture_id == CANDIDATE and key in ("FIRST_COUNT", "LAST_COUNT") and value == "-1",
                    "COUNT_FIELD_" + key)
    if fixture_id == CANDIDATE:
        if fields["OBSERVED"] == "N":
            require(fields["OUTER_COUNT"] == "0"
                    and fields["FIRST_COUNT"] == fields["LAST_COUNT"] == "-1"
                    and all(fields[k] == "UNOBSERVED" for k in
                            ("FIRST_FIRST64", "FIRST_LAST64", "LAST_FIRST64", "LAST_LAST64",
                             "FIRST_TYPE64", "LAST_TYPE64")), "UNOBSERVED_BRANCH")
        else:
            require(int(fields["OUTER_COUNT"]) > 0, "OBSERVED_BRANCH")
            for side in ("FIRST", "LAST"):
                require(int(fields[side + "_COUNT"]) >= 0 and fields[side + "_TYPE64"] != "UNOBSERVED",
                        "OBSERVED_SIDE")
                if fields[side + "_COUNT"] == "0":
                    require(fields[side + "_FIRST64"] == fields[side + "_LAST64"] == "UNOBSERVED",
                            "EMPTY_INNER_BRANCH")
                else:
                    require(fields[side + "_FIRST64"] != "UNOBSERVED"
                            and fields[side + "_LAST64"] != "UNOBSERVED", "NONEMPTY_INNER_BRANCH")
    witnesses = bool(decoded["TEXT_TYPE64"] and decoded["DICT_TYPE64"]
                     and decoded["TEXT_TYPE64"] != decoded["DICT_TYPE64"])
    goals = {key: fields[key] == expected for key, expected in GOALS[fixture_id].items()}
    carrier_types = None
    if fixture_id == CONTROL:
        carrier_types = bool(witnesses and decoded["FIRST_TYPE64"] == decoded["DICT_TYPE64"]
                             and decoded["LAST_TYPE64"] == decoded["DICT_TYPE64"])
    return {"status": "OBSERVATION_PARSED", "fixtureId": fixture_id, "actualFields": fields,
            "decodedBase64": decoded, "goalChecks": goals, "typeWitnessesDistinct": witnesses,
            "dictionaryCarrierTypesMatched": carrier_types,
            "controlGoalsMatched": all(goals.values()) and carrier_types is True if fixture_id == CONTROL else None,
            "candidateShapeMatched": all(goals.values()) if fixture_id == CANDIDATE else None,
            "nativeListTypeProved": False, "formalAcceptanceGranted": False}


def _plain(text):
    return {"Value": {"string": text}, "WFSerializationType": "WFTextTokenString"}


def _attached(ref):
    return {"Value": ref, "WFSerializationType": "WFTextTokenAttachment"}


def _variable(name):
    return {"Type": "Variable", "VariableName": name}


def _token(text, refs):
    positions = [i for i, c in enumerate(text) if c == "\ufffc"]
    require(len(positions) == len(refs), "ORACLE_TOKEN_COUNT")
    return {"Value": {"string": text, "attachmentsByRange": {
        "{%d, 1}" % (len(text[:i].encode("utf-16-le")) // 2): ref
        for i, ref in zip(positions, refs)}}, "WFSerializationType": "WFTextTokenString"}


def _array(values):
    return {"Value": [{"WFItemType": 0, "WFValue": _plain(x)} for x in values],
            "WFSerializationType": "WFArrayParameterState"}


def _dictionary(fields):
    rows = []
    for key, value in sorted(fields.items()):
        if isinstance(value, list):
            item_type, encoded = 2, _array(value)
        else:
            item_type, encoded = 0, _plain(value)
        rows.append({"WFItemType": item_type, "WFKey": _plain(key), "WFValue": encoded})
    return {"Value": {"WFDictionaryFieldValueItems": rows}, "WFSerializationType": "WFDictionaryFieldValue"}


class _Graph:
    def __init__(self, actual):
        self.actual = actual
        self.expected = []
        self.outputs = {}

    def uuid(self, index):
        value = self.actual[index].get("WFWorkflowActionParameters", {}).get("UUID")
        require(isinstance(value, str) and UUID_RE.fullmatch(value), "PRODUCER_UUID")
        return value

    def emit(self, kind, params=None, name=None, produced=False, reference_name=None, omitted=False):
        index = len(self.expected)
        p = copy.deepcopy(params or {})
        if name is not None:
            p["CustomOutputName"] = name
        if produced or name is not None:
            p["UUID"] = self.uuid(index)
        action = {"WFWorkflowActionIdentifier": "is.workflow.actions." + kind}
        if not omitted:
            action["WFWorkflowActionParameters"] = p
        self.expected.append(action)
        ref = None
        if produced or name is not None:
            ref = {"Type": "ActionOutput", "OutputUUID": p["UUID"],
                   "OutputName": reference_name or name}
            if name is not None:
                self.outputs[name] = ref
        return ref

    def read(self, kind, name, source, key="WFInput", extra=None, reference_name=None):
        params = dict(extra or {})
        params[key] = _attached(source)
        return self.emit(kind, params, name=name, produced=True, reference_name=reference_name)

    def count(self, name, source):
        return self.read("count", name, source, key="Input")

    def item(self, name, source, which):
        return self.read("getitemfromlist", name, source, extra={"WFItemSpecifier": which + " Item"})

    def base64(self, name, source):
        return self.read("base64encode", name, source, extra={
            "WFBase64LineBreakMode": "None", "WFEncodeMode": "Encode", "input": "Encode"},
            reference_name=name or "Base 64 Encode")

    def store(self, name, source):
        self.emit("setvariable", {"WFInput": _attached(source), "WFVariableName": name})

    def initialize(self, name, text):
        source = self.emit("gettext", {"WFTextActionText": text}, name="Text")
        self.store(name, source)

    def begin(self, source):
        index = len(self.expected)
        group = self.actual[index].get("WFWorkflowActionParameters", {}).get("GroupingIdentifier")
        require(isinstance(group, str) and UUID_RE.fullmatch(group), "CONTROL_GROUP")
        ref = {**source, "Aggrandizements": [
            {"CoercionItemClass": "WFNumberContentItem", "Type": "WFCoercionVariableAggrandizement"}]}
        self.emit("conditional", {"GroupingIdentifier": group, "WFCondition": 2,
            "WFConditionalLegacyComparisonBehavior": True, "WFControlFlowMode": 0,
            "WFInput": {"Type": "Variable", "Variable": _attached(ref)}, "WFNumberValue": 0.0})
        return group

    def end(self, group):
        self.emit("conditional", {"GroupingIdentifier": group, "WFControlFlowMode": 2})


def _trace(label, stage, sequence, action):
    value = json.dumps({"schemaVersion": "1.4", "kind": "INLINE_DEBUG_ACTION",
        "stage": stage, "label": label, "stepSequence": sequence,
        "actionIdentifier": "is.workflow.actions." + action,
        "timestampPrecision": "apple-currentdate", "valueType": "UNKNOWN_AT_TRACE_EVENT",
        "timestamp": "\ufffc"}, ensure_ascii=False, separators=(",", ":"))
    return _token(value, [{"Type": "CurrentDate"}])


def _collector(fixture, stage):
    return json.dumps({"schemaVersion": "1.4", "kind": "COLLECTOR_EVENT", "stage": stage,
        "runId": fixture, "traceTransport": "ndjson-stream",
        "collectorSentinel": "===CHERRI_QA_TRACE_NDJSON_V14==="}, separators=(",", ":"))


def _prefix(g, fixture):
    g.emit("comment", {"WFCommentActionText": "ANCHOR_CARD_01_TOP_HEADER"})
    g.emit("nothing", produced=True)
    g.emit("setvariable", {"WFVariableName": "__cherri_debug_log"})
    before = g.emit("gettext", {"WFTextActionText": _trace(fixture[3:-3], "before", 3, "setvariable")},
                    produced=True, reference_name="Text")
    g.emit("appendvariable", {"WFInput": _attached(before), "WFVariableName": "__cherri_qa_trace"})


def _suffix(g, fixture, refs):
    summary = PREFIXES[fixture] + ":".join(key + "=\ufffc" for key in FIELDS[fixture])
    semantic = g.emit("gettext", {"WFTextActionText": _token(summary, refs)}, name="semanticResult")
    g.emit("comment", {"WFCommentActionText": "ANCHOR_CARD_15_MID_CENTER"}, produced=True)
    sequence = len(g.expected)
    after = g.emit("gettext", {"WFTextActionText": _trace(fixture[3:-3], "after", sequence, "comment")},
                   produced=True, reference_name="Text")
    g.emit("appendvariable", {"WFInput": _attached(after), "WFVariableName": "__cherri_qa_trace"})
    combined = g.emit("text.combine", {"WFTextSeparator": "New Lines",
        "text": _attached(_variable("__cherri_qa_trace"))}, produced=True, reference_name="Combined Text")
    envelope = g.emit("gettext", {"WFTextActionText": _token(
        _collector(fixture, "collector_begin") + "\n\ufffc\n" + _collector(fixture, "collector_end"),
        [combined])}, name="traceEnvelope")
    final = g.emit("gettext", {"WFTextActionText": _token("\ufffc\n\ufffc", [semantic, envelope])},
                   name="finalSummary")
    g.emit("comment", {"WFCommentActionText": "ANCHOR_CARD_28_BOTTOM_FOOTER"})
    g.emit("output", {"WFOutput": _token("\ufffc", [final])})


def _control_graph(g):
    flat = g.emit("list", {"WFItems": [{"WFItemType": 0, "WFValue": x} for x in ("LEFT", "RIGHT")]}, name="flat")
    g.count("flatCount", flat)
    first = g.item("flatFirst", flat, "First")
    last = g.item("flatLast", flat, "Last")
    g.base64("flatFirst64", first)
    g.base64("flatLast64", last)
    g.emit("nothing", omitted=True)
    private = "__cherri_scope_1_emptyList_" + g.uuid(13)
    g.emit("setvariable", {"WFVariableName": private})
    empty = g.emit("getvariable", {"WFVariable": _attached(_variable(private))}, name="empty")
    g.count("emptyCount", empty)
    first_box = g.emit("dictionary", {"WFItems": _dictionary({"tag": "LEFT", "items": ["INNER_A", "INNER_B"]})}, name="firstBox")
    last_box = g.emit("dictionary", {"WFItems": _dictionary({"tag": "EMPTY", "items": []})}, name="lastBox")
    boxes = g.emit("list", {"WFItems": [{"WFItemType": 0, "WFValue": _attached(x)}
                                      for x in (first_box, last_box)]}, name="boxes")
    g.count("boxCount", boxes)
    first = g.item("observedFirstBox", boxes, "First")
    last = g.item("observedLastBox", boxes, "Last")
    first_tag = g.read("getvalueforkey", "firstTag", first, extra={"WFDictionaryKey": "tag"})
    last_tag = g.read("getvalueforkey", "lastTag", last, extra={"WFDictionaryKey": "tag"})
    g.base64("firstTag64", first_tag)
    g.base64("lastTag64", last_tag)
    first_keys = g.read("getvalueforkey", "firstKeys", first, extra={"WFGetDictionaryValueType": "All Keys"})
    last_keys = g.read("getvalueforkey", "lastKeys", last, extra={"WFGetDictionaryValueType": "All Keys"})
    g.count("firstKeyCount", first_keys)
    g.count("lastKeyCount", last_keys)
    first_items = g.read("getvalueforkey", "firstItems", first, extra={"WFDictionaryKey": "items"})
    last_items = g.read("getvalueforkey", "lastItems", last, extra={"WFDictionaryKey": "items"})
    g.count("firstItemCount", first_items)
    g.count("lastItemCount", last_items)
    first_value = g.item("firstValue", first_items, "First")
    last_value = g.item("lastValue", first_items, "Last")
    g.base64("firstValue64", first_value)
    g.base64("lastValue64", last_value)
    text = g.emit("gettext", {"WFTextActionText": "TYPE_WITNESS"}, name="textWitness")
    dictionary = g.emit("dictionary", {"WFItems": _dictionary({"kind": "CONTROL"})}, name="dictionaryWitness")
    for name, ref in (("textType", text), ("dictionaryType", dictionary), ("firstType", first), ("lastType", last)):
        g.read("getitemtype", name, ref)
    for name in ("textType", "dictionaryType", "firstType", "lastType"):
        g.base64(name + "64", g.outputs[name])
    names = ("flatCount", "flatFirst64", "flatLast64", "emptyCount", "boxCount",
             "firstTag64", "lastTag64", "firstKeyCount", "lastKeyCount", "firstItemCount",
             "firstValue64", "lastValue64", "lastItemCount", "textType64", "dictionaryType64",
             "firstType64", "lastType64")
    _suffix(g, CONTROL, [g.outputs[x] for x in names])


def _candidate_graph(g):
    require(g.uuid(5) == "24DF1735-6B49-435A-8D0A-A2FCE1D7E901", "EXPERIMENTAL_UUID")
    candidate = g.emit("list", {"WFItems": [
        {"WFItemType": 2, "WFValue": _array(["INNER_A", "INNER_B"])},
        {"WFItemType": 2, "WFValue": _array([])}]}, name="candidateRows")
    outer = g.count("outerCount", candidate)
    text = g.emit("gettext", {"WFTextActionText": "TYPE_WITNESS"}, name="textWitness")
    dictionary = g.emit("dictionary", {"WFItems": _dictionary({"kind": "CONTROL"})}, name="dictionaryWitness")
    g.read("getitemtype", "textType", text)
    g.read("getitemtype", "dictionaryType", dictionary)
    g.base64("textType64", g.outputs["textType"])
    g.base64("dictionaryType64", g.outputs["dictionaryType"])
    for name, value in (("observed", "N"), ("firstCountText", "-1"), ("lastCountText", "-1"),
                        ("firstFirst64", "UNOBSERVED"), ("firstLast64", "UNOBSERVED"),
                        ("lastFirst64", "UNOBSERVED"), ("lastLast64", "UNOBSERVED"),
                        ("firstType64", "UNOBSERVED"), ("lastType64", "UNOBSERVED")):
        g.initialize(name, value)
    outer_group = g.begin(outer)
    g.initialize("observed", "Y")
    first = g.item("first", candidate, "First")
    last = g.item("last", candidate, "Last")
    first_count = g.count("firstCount", first)
    last_count = g.count("lastCount", last)
    for name, ref in (("firstCountText", first_count), ("lastCountText", last_count)):
        value = g.read("detect.text", None, ref, reference_name="Get Text")
        g.store(name, value)
    g.read("getitemtype", "firstType", first)
    g.read("getitemtype", "lastType", last)
    for name in ("firstType", "lastType"):
        value = g.base64(None, g.outputs[name])
        g.store(name + "64", value)
    first_group = g.begin(first_count)
    for side, ref, count, group in (("first", first, first_count, first_group), ("last", last, last_count, None)):
        if group is None:
            group = g.begin(count)
        first_item = g.item(side + "First", ref, "First")
        last_item = g.item(side + "Last", ref, "Last")
        first_text = g.read("detect.text", side + "FirstText", first_item)
        last_text = g.read("detect.text", side + "LastText", last_item)
        g.store(side + "First64", g.base64(None, first_text))
        g.store(side + "Last64", g.base64(None, last_text))
        g.end(group)
    g.end(outer_group)
    refs = [outer, _variable("observed"), _variable("firstCountText"), _variable("lastCountText"),
            _variable("firstFirst64"), _variable("firstLast64"), _variable("lastFirst64"),
            _variable("lastLast64"), g.outputs["textType64"], g.outputs["dictionaryType64"],
            _variable("firstType64"), _variable("lastType64")]
    _suffix(g, CANDIDATE, refs)


def graph_receipt(fixture_id):
    require(fixture_id in FIXTURES, "FIXTURE")
    return {"status": "PASS_LOCAL_COMPLETE_GRAPH_ONLY", "fixtureId": fixture_id,
            "nativeActionCount": COUNTS[fixture_id], "appleRuntime": "NOT_RUN",
            "nativeListTypeProved": False, "formalAcceptanceGranted": False}


def mutation_receipt(fixture_id):
    require(fixture_id in FIXTURES, "FIXTURE")
    names = COMMON_MUTATIONS + (CONTROL_MUTATIONS if fixture_id == CONTROL else CANDIDATE_MUTATIONS)
    return {"status": "PASS_LOCAL_CAUSAL_CORRUPTIONS_ONLY", "fixtureId": fixture_id,
            "rejected": list(names), "appleRuntime": "NOT_RUN",
            "nativeListTypeProved": False, "formalAcceptanceGranted": False}


def verify_graph(value, fixture_id, actions):
    validate_contract(value)
    require(value["fixture_id"] == fixture_id and fixture_id in FIXTURES, "GRAPH_FIXTURE")
    require(isinstance(actions, list) and len(actions) == COUNTS[fixture_id]
            and all(isinstance(a, dict) for a in actions), "ACTION_COUNT")
    uuids = []
    groups = []
    for action in actions:
        params = action.get("WFWorkflowActionParameters", {})
        require(isinstance(params, dict), "PARAMETERS")
        if "UUID" in params:
            require(isinstance(params["UUID"], str) and UUID_RE.fullmatch(params["UUID"]), "PRODUCER_UUID")
            uuids.append(params["UUID"])
        if "GroupingIdentifier" in params:
            require(isinstance(params["GroupingIdentifier"], str)
                    and UUID_RE.fullmatch(params["GroupingIdentifier"]), "CONTROL_GROUP")
            groups.append(params["GroupingIdentifier"])
    require(len(uuids) == len(set(uuids)), "DUPLICATE_PRODUCER")
    require(not set(uuids).intersection(groups), "GROUP_PRODUCER_COLLISION")
    require(len(set(groups)) == (0 if fixture_id == CONTROL else 3), "GROUP_INVENTORY")
    graph = _Graph(actions)
    _prefix(graph, fixture_id)
    (_control_graph if fixture_id == CONTROL else _candidate_graph)(graph)
    require(len(graph.expected) == COUNTS[fixture_id], "ORACLE_INVENTORY")
    for i, (actual, expected) in enumerate(zip(actions, graph.expected)):
        require(_same(actual, expected), "ACTION_%d_CHANGED" % i)
    return graph_receipt(fixture_id)


def verify_mutations(value, fixture_id, actions):
    verify_graph(value, fixture_id, actions)
    def p(a, i): return a[i]["WFWorkflowActionParameters"]
    summary, after, envelope, output = (47, 49, 52, 55) if fixture_id == CONTROL else (69, 71, 74, 77)
    def shift(a):
        attachments = p(a, summary)["WFTextActionText"]["Value"]["attachmentsByRange"]
        old = next(iter(attachments))
        attachments["{999, 1}"] = attachments.pop(old)
    mutations = {
        "action-removed": lambda a: a.pop(6),
        "extra-effect": lambda a: a.append({"WFWorkflowActionIdentifier": "is.workflow.actions.getclipboard",
                                          "WFWorkflowActionParameters": {}}),
        "duplicate-producer": lambda a: p(a, 6).update(UUID=p(a, 5)["UUID"]),
        "missing-producer": lambda a: p(a, 6).pop("UUID"),
        "clear-borrows-input": lambda a: p(a, 2).update(WFInput=""),
        "before-trace-time-lost": lambda a: p(a, 3)["WFTextActionText"]["Value"].update(attachmentsByRange={}),
        "trace-store-replaced": lambda a: p(a, 4).update(WFVariableName="__cherri_debug_log"),
        "after-trace-time-lost": lambda a: p(a, after)["WFTextActionText"]["Value"].update(attachmentsByRange={}),
        "summary-field-static": lambda a: p(a, summary).update(WFTextActionText="SUCCESS"),
        "summary-range-shift": shift,
        "trace-envelope-replaced": lambda a: p(a, envelope).update(WFTextActionText="TRACE"),
        "final-output-replaced": lambda a: p(a, output).update(WFOutput="SUCCESS"),
        "extra-terminal-nothing": lambda a: a.append({"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"}),
    }
    if fixture_id == CONTROL:
        def fields(a, index): return p(a, index)["WFItems"]["Value"]["WFDictionaryFieldValueItems"]
        mutations.update({
            "flat-order-swapped": lambda a: p(a, 5)["WFItems"].reverse(),
            "empty-control-singleton": lambda a: a.__setitem__(11, {"WFWorkflowActionIdentifier": "is.workflow.actions.list",
                "WFWorkflowActionParameters": {"WFItems": [{"WFItemType": 0, "WFValue": ""}]}}),
            "empty-clear-borrows-input": lambda a: p(a, 12).update(WFInput=_attached(_variable("flat"))),
            "first-array-flattened-text": lambda a: fields(a, 15)[0].update(WFItemType=0, WFValue=_plain("INNER_A,INNER_B")),
            "empty-array-value-omitted": lambda a: fields(a, 16)[0]["WFValue"].pop("Value"),
            "empty-field-removed": lambda a: fields(a, 16).pop(0),
            "dictionary-carriers-swapped": lambda a: p(a, 17)["WFItems"].reverse(),
            "raw-map-in-list-row": lambda a: p(a, 17)["WFItems"][0].update(WFValue={"tag": "LEFT", "items": ["INNER_A", "INNER_B"]}),
            "outer-count-borrows-flat": lambda a: p(a, 18).update(Input=p(a, 6)["Input"]),
            "first-tag-wrong-key": lambda a: p(a, 21).update(WFDictionaryKey="items"),
            "first-items-read-last-box": lambda a: p(a, 29).update(WFInput=p(a, 30)["WFInput"]),
            "first-count-borrows-last": lambda a: p(a, 31).update(Input=p(a, 32)["Input"]),
            "first-value-uses-last": lambda a: p(a, 33).update(WFItemSpecifier="Last Item"),
            "dictionary-type-borrows-text": lambda a: p(a, 40).update(WFInput=p(a, 39)["WFInput"]),
            "last-type-borrows-first-producer": lambda a: p(a, 42).update(WFInput=p(a, 41)["WFInput"]),
        })
    else:
        def rows(a): return p(a, 5)["WFItems"]
        mutations.update({
            "candidate-row-type-zero": lambda a: rows(a)[0].update(WFItemType=0),
            "candidate-rows-flattened": lambda a: p(a, 5).update(WFItems=copy.deepcopy(rows(a)[0]["WFValue"]["Value"])),
            "empty-row-removed": lambda a: rows(a).pop(1),
            "empty-row-becomes-text": lambda a: rows(a)[1].update(WFItemType=0, WFValue=""),
            "inner-order-swapped": lambda a: rows(a)[0]["WFValue"]["Value"].reverse(),
            "first-row-cleared": lambda a: rows(a)[0]["WFValue"].update(Value=[]),
            "candidate-fixed-uuid-changed": lambda a: p(a, 5).update(UUID="00000000-0000-0000-0000-000000000001"),
            "outer-count-borrows-text": lambda a: p(a, 6)["Input"]["Value"].update(OutputUUID=p(a, 7)["UUID"]),
            "outer-guard-inverted": lambda a: p(a, 31).update(WFCondition=3),
            "outer-guard-threshold-one": lambda a: p(a, 31).update(WFNumberValue=1.0),
            "first-selector-last": lambda a: p(a, 34).update(WFItemSpecifier="Last Item"),
            "first-count-borrows-outer": lambda a: p(a, 36).update(Input=p(a, 6)["Input"]),
            "first-type-borrows-witness": lambda a: p(a, 42).update(WFInput=p(a, 9)["WFInput"]),
            "first-guard-uses-last-count": lambda a: p(a, 48).update(WFInput=p(a, 58)["WFInput"]),
            "inner-order-reads-last": lambda a: p(a, 49).update(WFItemSpecifier="Last Item"),
            "last-guard-uses-first-count": lambda a: p(a, 58).update(WFInput=p(a, 48)["WFInput"]),
            "last-inner-reads-first": lambda a: p(a, 59).update(WFInput=p(a, 49)["WFInput"]),
            "control-group-reused": lambda a: p(a, 58).update(GroupingIdentifier=p(a, 48)["GroupingIdentifier"]),
            "observed-flag-never-set": lambda a: p(a, 32).update(WFTextActionText="N"),
            "summary-borrows-initializer": lambda a: p(a, summary)["WFTextActionText"]["Value"].update(
                attachmentsByRange={"{44, 1}": {"Type": "ActionOutput", "OutputName": "Text", "OutputUUID": p(a, 13)["UUID"]}}),
        })
    wanted = mutation_receipt(fixture_id)
    require(list(mutations) == wanted["rejected"], "MUTATION_INVENTORY")
    for name, mutate in mutations.items():
        changed = copy.deepcopy(actions)
        mutate(changed)
        try:
            verify_graph(value, fixture_id, changed)
        except ValueError:
            continue
        raise ValueError("NESTED_LIST_CORRUPTION_ACCEPTED:" + name)
    return wanted


def main(argv=None):
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--fixture", choices=FIXTURES, required=True)
    parser.add_argument("--unsigned", type=Path, required=True)
    args = parser.parse_args(argv)
    value = contract(args.root, args.fixture)
    raw_source = (args.root / value["source_path"]).read_bytes()
    source = verify_source(args.root, args.fixture, raw_source)
    raw = args.unsigned.read_bytes()
    require(0 < len(raw) <= 2 * 1024 * 1024, "UNSIGNED_SIZE")
    workflow = plistlib.loads(raw)
    require(isinstance(workflow, dict) and isinstance(workflow.get("WFWorkflowActions"), list), "WORKFLOW")
    actions = workflow["WFWorkflowActions"]
    result = {"fixtureId": args.fixture, "source": source, "unsignedPlistSha256": sha(raw),
              "graph": verify_graph(value, args.fixture, actions),
              "causalMutations": verify_mutations(value, args.fixture, actions),
              "nativeListTypeProved": False, "formalAcceptanceGranted": False}
    print("CHERRI_NESTED_LIST_UNSIGNED_GRAPH_V1=" + json.dumps(result, sort_keys=True, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
