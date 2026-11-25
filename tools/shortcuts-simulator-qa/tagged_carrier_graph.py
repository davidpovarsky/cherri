"""Source-pinned engineering graph contract for the tagged carrier fixture.

This is an independent description of the authored program's complete action
graph. It authenticates source and edges only. Apple execution, signed import
and callback provenance must be supplied by the native owner separately.
"""
import argparse
import hashlib
import json
import pathlib
import plistlib
import re

from native_comparison_graph import verify_equality_region
from tagged_carrier_observation import FIELDS, PREFIX, FIXTURE

SOURCE_SHA256 = "8abc203ba48ba97e6c3c28240dd861e3a6b8b4a0b5ac91f7e82334ca528a46ec"
CONTRACT_SHA256 = "d1d4ce31aefae34870ccf4316d1cde11af08d2751597df8876f6811d0290fb4c"
PLACEHOLDER = "\ufffc"
ACTION_PREFIX = "is.workflow.actions."
UUID_PATTERN = r"[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}"


def require(condition, reason):
    if not condition:
        raise ValueError("TAGGED_CARRIER_GRAPH_" + reason)


def attachment(value):
    return {"WFSerializationType": "WFTextTokenAttachment", "Value": value}


def output(index, cast=None):
    value = {"Type": "ActionOutput", "OutputUUID": index}
    if cast is not None:
        value["Aggrandizements"] = [
            {"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": cast}]
    return attachment(value)


def variable(name):
    return attachment({"Type": "Variable", "VariableName": name})


def text_state(text, references=()):
    positions = [i for i, char in enumerate(text) if char == PLACEHOLDER]
    require(len(positions) == len(references), "EXPECTED_TEXT_ARITY")
    value = {"string": text}
    if positions:
        value["attachmentsByRange"] = {
            "{%d, 1}" % (len(text[:i].encode("utf-16-le")) // 2): ref["Value"]
            for i, ref in zip(positions, references)}
    return {"WFSerializationType": "WFTextTokenString", "Value": value}


def dictionary_state(rows):
    return {"WFSerializationType": "WFDictionaryFieldValue",
            "Value": {"WFDictionaryFieldValueItems": [
                {"WFItemType": kind, "WFKey": text_state(key), "WFValue": value}
                for key, kind, value in rows]}}


class ExpectedProgram:
    """Describe the source directly; no native compiler or actual plist is read."""

    def __init__(self):
        self.actions = []
        self.slots = []
        self.roles = {}

    def emit(self, kind, **parameters):
        index = len(self.actions)
        self.actions.append({"WFWorkflowActionIdentifier": ACTION_PREFIX + kind,
                             "WFWorkflowActionParameters": parameters})
        return index

    def empty_native_action(self):
        # The typed lowerer emits a parameterless Nothing action. DebugClearLogs'
        # registered native action carries an output identity instead.
        index = len(self.actions)
        self.actions.append({"WFWorkflowActionIdentifier": ACTION_PREFIX + "nothing"})
        return index

    def text(self, value):
        return self.emit("gettext", WFTextActionText=value)

    def set(self, name, value=None):
        parameters = {"WFVariableName": name}
        if value is not None:
            parameters["WFInput"] = value
        return self.emit("setvariable", **parameters)

    def set_text(self, name, value):
        return self.set(name, output(self.text(value)))

    def begin(self, value, operator, right):
        index = len(self.actions)
        parameters = {
            "GroupingIdentifier": index, "WFControlFlowMode": 0,
            "WFConditionalLegacyComparisonBehavior": True,
            "WFCondition": operator,
            "WFInput": {"Type": "Variable", "Variable": value}}
        if type(right) in (int, float):
            parameters["WFNumberValue"] = right
        else:
            parameters["WFConditionalActionString"] = right
        self.emit("conditional", **parameters)
        return index

    def otherwise(self, group):
        return self.emit("conditional", GroupingIdentifier=group, WFControlFlowMode=1)

    def end(self, group):
        return self.emit("conditional", GroupingIdentifier=group, WFControlFlowMode=2)

    def count(self, value, explicit=False):
        parameters = {"Input": value}
        if explicit:
            parameters["WFCountType"] = "Items"
        return self.emit("count", **parameters)

    def detect_text(self, value):
        return self.emit("detect.text", WFInput=value)

    def encode(self, value):
        return self.emit("base64encode", WFInput=value, WFBase64LineBreakMode="None",
                         WFEncodeMode="Encode", input="Encode")

    def typeof(self, value):
        return self.emit("getitemtype", WFInput=value)

    def list(self, values):
        return self.emit("list", WFItems=[{"WFItemType": 0, "WFValue": value}
                                         for value in values])

    def cell(self, kind, presence, payload=None, payload_kind=0):
        rows = [
            ("_cherri_value_cell_v1", 0, text_state("1")),
            ("has_value", 0, text_state(presence)),
            ("kind", 0, text_state(kind))]
        if payload is not None:
            rows.append(("value", payload_kind, payload))
        return self.emit("dictionary", WFItems=dictionary_state(rows))

    def checked_index(self, sequence, ordinal, role):
        number = len(self.slots) + 1
        value_name, presence_name = "@value%d" % number, "@presence%d" % number
        self.empty_native_action()
        clear = self.set(value_name)
        initial_presence = self.set_text(presence_name, "0")
        one_based = self.emit("math", WFInput=ordinal, WFMathOperation="+", WFMathOperand=1.0)
        count = self.count(output(sequence), explicit=True)
        difference = self.emit("math", WFInput=output(count),
                               WFMathOperation="-", WFMathOperand=output(one_based))
        lower = self.begin(output(one_based), 3, 1.0)
        upper = self.begin(output(difference), 3, 0.0)
        item = self.emit("getitemfromlist", WFInput=output(sequence),
                         WFItemSpecifier="Item At Index", WFItemIndex=output(one_based))
        has = self.emit("getvalueforkey", WFInput=output(item),
                        WFDictionaryKey="has_value", WFGetDictionaryValueType="Value")
        present = self.begin(output(has), 4, "1")
        payload = self.emit("getvalueforkey", WFInput=output(item),
                            WFDictionaryKey="value", WFGetDictionaryValueType="Value")
        self.otherwise(present)
        self.empty_native_action()
        decoded = self.end(present)
        self.set(presence_name, output(has))
        self.set(value_name, output(decoded))
        self.otherwise(upper)
        self.empty_native_action()
        self.end(upper)
        self.otherwise(lower)
        self.empty_native_action()
        self.end(lower)
        result = self.emit("getvariable", WFVariable=variable(value_name))
        result_presence = self.emit("getvariable", WFVariable=variable(presence_name))
        self.slots.append({"number": number, "valueSet": clear,
                           "presenceSet": initial_presence,
                           "valueName": value_name, "presenceName": presence_name})
        self.roles[role] = {
            "sequence": sequence, "oneBased": one_based, "count": count,
            "difference": difference, "lower": lower, "upper": upper,
            "item": item, "has": has, "present": present, "payload": payload,
            "decoded": decoded, "result": result, "presenceResult": result_presence}
        return result, result_presence

    def exact_type_equality(self, left, right):
        start = len(self.actions)
        left_snapshot = self.text(text_state("~" + PLACEHOLDER, [output(left)]))
        right_snapshot = self.text(text_state("~" + PLACEHOLDER, [output(right)]))
        quote = self.emit(
            "text.replace", WFInput=text_state(PLACEHOLDER, [output(right_snapshot)]),
            WFReplaceTextFind=r"\E", WFReplaceTextReplace=r"\E\\E\Q",
            WFReplaceTextCaseSensitive=True, WFReplaceTextRegularExpression=False)
        match = self.emit(
            "text.match", text=text_state(PLACEHOLDER, [output(left_snapshot)]),
            WFMatchTextPattern=text_state(r"\A\Q" + PLACEHOLDER + r"\E\z", [output(quote)]),
            WFMatchTextCaseSensitive=True)
        count = self.count(output(match))
        compare = self.begin(output(count), 4, 1.0)
        self.emit("number", WFNumberActionNumber=1.0)
        self.otherwise(compare)
        self.emit("number", WFNumberActionNumber=0.0)
        result = self.end(compare)
        guard = self.begin(output(result), 4, 1.0)
        self.roles["typeEquality"] = {
            "start": start, "left": left, "right": right, "quote": quote,
            "match": match, "count": count, "compare": compare,
            "result": result, "guard": guard}
        return guard

    def trace(self, stage):
        index = len(self.actions)
        previous_kind = self.actions[-1]["WFWorkflowActionIdentifier"]
        payload = {
            "schemaVersion": "1.4", "kind": "INLINE_DEBUG_ACTION", "stage": stage,
            "label": "tagged-value-carrier", "stepSequence": index,
            "actionIdentifier": previous_kind, "timestampPrecision": "apple-currentdate",
            "valueType": "UNKNOWN_AT_TRACE_EVENT", "timestamp": PLACEHOLDER}
        text = json.dumps(payload, ensure_ascii=False, separators=(",", ":"))
        event = self.text(text_state(text, [attachment({"Type": "CurrentDate"})]))
        self.emit("appendvariable", WFInput=output(event), WFVariableName="__cherri_qa_trace")
        self.roles["trace_" + stage] = event


def expected_program():
    p = ExpectedProgram()
    p.emit("comment", WFCommentActionText="ANCHOR_CARD_01_TOP_HEADER")
    p.emit("nothing")
    p.set("__cherri_debug_log")
    p.trace("before")
    first_number = p.emit("number", WFNumberActionNumber=11)
    last_number = p.emit("number", WFNumberActionNumber=22)
    original = p.list([output(first_number), output(last_number)])
    original_type = p.typeof(output(original))
    original_type64 = p.encode(output(original_type))
    text_witness = p.text("TYPE_WITNESS")
    text_type = p.typeof(output(text_witness))
    text_type64 = p.encode(output(text_type))
    first_cell = p.cell("list", "1", output(original))
    empty_cell = p.cell("list", "1",
                        {"WFSerializationType": "WFArrayParameterState", "Value": []}, 2)
    cells = p.list([output(first_cell), output(empty_cell)])
    outer_count = p.count(output(cells))
    for name, initial in (
            ("firstPresent", "N"), ("firstCountText", "-1"),
            ("firstFirst64", "UNOBSERVED"), ("firstLast64", "UNOBSERVED"),
            ("decodedType64", "UNOBSERVED")):
        p.set_text(name, initial)
    first, first_presence = p.checked_index(cells, 0, "first")
    first_guard = p.begin(output(first_presence), 5, "0")
    p.set_text("firstPresent", "Y")
    first_count = p.count(output(first))
    p.set("firstCountText", output(p.detect_text(output(first_count))))
    decoded_type = p.typeof(output(first))
    p.set("decodedType64", output(p.encode(output(decoded_type))))
    positive_count = p.begin(output(first_count, "WFNumberContentItem"), 2, 0.0)
    first_value = p.emit("getitemfromlist", WFInput=output(first), WFItemSpecifier="First Item")
    last_value = p.emit("getitemfromlist", WFInput=output(first), WFItemSpecifier="Last Item")
    first_text = p.detect_text(output(first_value))
    last_text = p.detect_text(output(last_value))
    p.set("firstFirst64", output(p.encode(output(first_text))))
    p.set("firstLast64", output(p.encode(output(last_text))))
    p.end(positive_count)
    p.end(first_guard)
    p.set_text("emptyPresent", "N")
    p.set_text("emptyCountText", "-1")
    empty, empty_presence = p.checked_index(cells, 1, "empty")
    empty_guard = p.begin(output(empty_presence), 5, "0")
    p.set_text("emptyPresent", "Y")
    empty_count = p.count(output(empty))
    p.set("emptyCountText", output(p.detect_text(output(empty_count))))
    p.end(empty_guard)
    original_map = p.emit("dictionary", WFItems=dictionary_state([
        ("alpha", 0, text_state("LEFT")), ("beta", 0, text_state("RIGHT"))]))
    original_map_type = p.typeof(output(original_map))
    original_map_type64 = p.encode(output(original_map_type))
    map_cell = p.cell("map", "1", output(original_map))
    absent_cell = p.cell("none", "0")
    map_cells = p.list([output(map_cell), output(absent_cell)])
    map_outer_count = p.count(output(map_cells))
    for name, initial in (
            ("mapPresent", "N"), ("mapAbsent", "N"),
            ("decodedMapType64", "UNOBSERVED"), ("mapKeysText", "-1"),
            ("mapAlpha64", "UNOBSERVED"), ("mapBeta64", "UNOBSERVED")):
        p.set_text(name, initial)
    decoded_map, map_presence = p.checked_index(map_cells, 0, "map")
    map_guard = p.begin(output(map_presence), 5, "0")
    p.set_text("mapPresent", "Y")
    decoded_map_type = p.typeof(output(decoded_map))
    p.set("decodedMapType64", output(p.encode(output(decoded_map_type))))
    type_guard = p.exact_type_equality(decoded_map_type, original_map_type)
    keys = p.emit("getvalueforkey", WFInput=output(decoded_map),
                  WFGetDictionaryValueType="All Keys")
    key_count = p.count(output(keys))
    p.set("mapKeysText", output(p.detect_text(output(key_count))))
    alpha = p.emit("getvalueforkey", WFInput=output(decoded_map), WFDictionaryKey="alpha")
    beta = p.emit("getvalueforkey", WFInput=output(decoded_map), WFDictionaryKey="beta")
    alpha_text = p.detect_text(output(alpha))
    beta_text = p.detect_text(output(beta))
    p.set("mapAlpha64", output(p.encode(output(alpha_text))))
    p.set("mapBeta64", output(p.encode(output(beta_text))))
    type_guard_end = p.end(type_guard)
    p.end(map_guard)
    _, absent_presence = p.checked_index(map_cells, 1, "absent")
    absent_guard = p.begin(output(absent_presence), 4, "0")
    p.set_text("mapAbsent", "Y")
    p.end(absent_guard)
    references = [
        output(outer_count), variable("firstPresent"), variable("firstCountText"),
        variable("firstFirst64"), variable("firstLast64"), variable("emptyPresent"),
        variable("emptyCountText"), output(original_type64), variable("decodedType64"),
        output(text_type64), output(map_outer_count), variable("mapPresent"),
        variable("mapAbsent"), output(original_map_type64), variable("decodedMapType64"),
        variable("mapKeysText"), variable("mapAlpha64"), variable("mapBeta64")]
    semantic = p.text(text_state(PREFIX + ":".join(key + "=" + PLACEHOLDER for key in FIELDS),
                                 references))
    p.emit("comment", WFCommentActionText="ANCHOR_CARD_15_MID_CENTER")
    p.trace("after")
    trace = p.emit("text.combine", WFTextSeparator="New Lines",
                   text=variable("__cherri_qa_trace"))
    def envelope(stage):
        return json.dumps({
            "schemaVersion": "1.4", "kind": "COLLECTOR_EVENT", "stage": stage,
            "runId": FIXTURE, "traceTransport": "ndjson-stream",
            "collectorSentinel": "===CHERRI_QA_TRACE_NDJSON_V14==="},
            separators=(",", ":"))
    trace_envelope = p.text(text_state(
        envelope("collector_begin") + "\n" + PLACEHOLDER + "\n" + envelope("collector_end"),
        [output(trace)]))
    final = p.text(text_state(PLACEHOLDER + "\n" + PLACEHOLDER,
                              [output(semantic), output(trace_envelope)]))
    p.emit("comment", WFCommentActionText="ANCHOR_CARD_28_BOTTOM_FOOTER")
    p.emit("output", WFOutput=text_state(PLACEHOLDER, [output(final)]))
    p.roles.update(
        originalList=original, originalListType=original_type,
        decodedListType=decoded_type, textType=text_type,
        originalMap=original_map, originalMapType=original_map_type,
        decodedMapType=decoded_map_type, firstCell=first_cell, emptyCell=empty_cell,
        mapCell=map_cell, absentCell=absent_cell, cells=cells, mapCells=map_cells,
        mapKeys=keys, mapAlpha=alpha, mapBeta=beta, mapTypeGuardEnd=type_guard_end,
        semantic=semantic, traceEnvelope=trace_envelope, final=final)
    require(len(p.actions) == 225, "EXPECTED_ACTION_INVENTORY")
    return p


def parameters(action):
    return action.get("WFWorkflowActionParameters", {})


def normalized_actions(actions, expected):
    require(isinstance(actions, list) and len(actions) == len(expected.actions), "ACTION_COUNT")
    ids, groups, stack, ended, alternate = {}, {}, [], set(), set()
    for index, action in enumerate(actions):
        require(isinstance(action, dict) and set(action) == set(expected.actions[index]),
                "ACTION_SHAPE")
        params = parameters(action)
        require(isinstance(params, dict), "PARAMETERS")
        identity = params.get("UUID")
        if identity is not None:
            require(isinstance(identity, str) and re.fullmatch(UUID_PATTERN, identity) is not None
                    and identity not in ids, "UNIQUE_PRODUCER_ID")
            ids[identity] = index
        group = params.get("GroupingIdentifier")
        if group is not None:
            require(action["WFWorkflowActionIdentifier"] == ACTION_PREFIX + "conditional"
                    and isinstance(group, str) and re.fullmatch(UUID_PATTERN, group) is not None,
                    "CONTROL_IDENTITY")
            mode = params.get("WFControlFlowMode")
            require(type(mode) is int and mode in (0, 1, 2), "CONTROL_MODE")
            if mode == 0:
                require(group not in groups, "CONTROL_REUSE")
                groups[group] = index
                stack.append(group)
            elif mode == 1:
                require(stack and stack[-1] == group and group not in alternate,
                        "CONTROL_ALTERNATIVE")
                alternate.add(group)
            else:
                require(stack and stack[-1] == group and group not in ended, "CONTROL_END")
                stack.pop()
                ended.add(group)
    require(not stack and set(groups) == ended, "CLOSED_CONTROLS")
    slots = {}
    for slot in expected.slots:
        for key, suffix, scope in (
                ("value", "taggedIndexValue", 2 * slot["number"] - 1),
                ("presence", "taggedIndexPresence", 2 * slot["number"])):
            name = parameters(actions[slot[key + "Set"]]).get("WFVariableName")
            require(isinstance(name, str) and re.fullmatch(
                "__cherri_scope_%d_%s_%s" % (scope, suffix, UUID_PATTERN), name) is not None
                and name not in slots, "OWNED_SLOT_IDENTITY")
            slots[name] = slot[key + "Name"]

    def normalize(value, current):
        if isinstance(value, list):
            return [normalize(item, current) for item in value]
        if not isinstance(value, dict):
            return value
        result = {}
        is_output = value.get("Type") == "ActionOutput"
        if is_output:
            require(set(value) <= {"Type", "OutputUUID", "OutputName", "Aggrandizements"}
                    and isinstance(value.get("OutputUUID"), str)
                    and value["OutputUUID"] in ids and ids[value["OutputUUID"]] < current, "ORDERED_PRODUCER_EDGE")
        for key, item in value.items():
            if is_output and key == "OutputName":
                require(isinstance(item, str), "OUTPUT_DISPLAY_NAME")
            elif is_output and key == "OutputUUID":
                result[key] = ids[item]
            elif key in ("WFVariableName", "VariableName") and isinstance(item, str):
                result[key] = slots.get(item, item)
            else:
                result[key] = normalize(item, current)
        return result

    normalized = []
    for index, action in enumerate(actions):
        params = dict(parameters(action))
        params.pop("UUID", None)
        display = params.pop("CustomOutputName", None)
        require(display is None or isinstance(display, str), "ACTION_DISPLAY_NAME")
        if "GroupingIdentifier" in params:
            params["GroupingIdentifier"] = groups[params["GroupingIdentifier"]]
        normalized_action = {"WFWorkflowActionIdentifier": action["WFWorkflowActionIdentifier"]}
        if "WFWorkflowActionParameters" in action:
            normalized_action["WFWorkflowActionParameters"] = normalize(params, index)
        normalized.append(normalized_action)
    return normalized


def difference(actual, expected, path=""):
    if type(actual) is not type(expected):
        return path
    if isinstance(expected, dict):
        if set(actual) != set(expected):
            return path + "/keys"
        for key in expected:
            result = difference(actual[key], expected[key], path + "/" + str(key))
            if result is not None:
                return result
        return None
    if isinstance(expected, list):
        if len(actual) != len(expected):
            return path + "/length"
        for index, (left, right) in enumerate(zip(actual, expected)):
            result = difference(left, right, path + "/" + str(index))
            if result is not None:
                return result
        return None
    return None if actual == expected else path


def validate_actions(actions):
    expected = expected_program()
    normalized = normalized_actions(actions, expected)
    mismatch = difference(normalized, expected.actions)
    require(mismatch is None, "FULL_ACTION_CONTRACT:" + str(mismatch))
    region = expected.roles["typeEquality"]
    verify_equality_region(
        actions, region["start"], region["guard"], actions[region["left"]],
        right_source=actions[region["right"]], require=require)
    return {
        "status": "ENGINEERING_SOURCE_GRAPH_VERIFIED", "actionCount": len(actions),
        "checkedCellIndices": 4, "typeWitnesses": 5,
        "normalization": ["producer/control UUID alpha identity",
                          "eight source-owned generated slot names",
                          "action/output display names"],
        "decodedListTypeProducer": expected.roles["decodedListType"],
        "decodedMapTypeProducer": expected.roles["decodedMapType"],
        "mapTypeEqualityRegion": [region["start"], region["guard"]],
        "mapDictionaryReads": [expected.roles[k] for k in ("mapKeys", "mapAlpha", "mapBeta")],
        "mapTypeGuardEnd": expected.roles["mapTypeGuardEnd"],
        "nativeCarrierPreservationVerified": False, "appleExecutionAuthenticated": False,
        "formalAcceptanceGranted": False}


def validate_compiled(source, contract, native):
    require(isinstance(source, bytes) and hashlib.sha256(source).hexdigest() == SOURCE_SHA256,
            "SOURCE_SHA256")
    require(isinstance(contract, bytes) and hashlib.sha256(contract).hexdigest() == CONTRACT_SHA256,
            "CONTRACT_SHA256")
    require(isinstance(native, bytes) and 0 < len(native) <= 2 * 1024**2, "NATIVE_BYTES")
    try:
        document = plistlib.loads(native)
    except (ValueError, TypeError, plistlib.InvalidFileException) as error:
        raise ValueError("TAGGED_CARRIER_GRAPH_NATIVE_PLIST") from error
    require(isinstance(document, dict), "WORKFLOW")
    result = validate_actions(document.get("WFWorkflowActions"))
    result.update(
        sourceAuthenticated=True, sourceSha256=SOURCE_SHA256,
        expectedContractSha256=CONTRACT_SHA256,
        unsignedSha256=hashlib.sha256(native).hexdigest(), unsignedBytes=len(native))
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=pathlib.Path, required=True)
    parser.add_argument("--expected", type=pathlib.Path, required=True)
    parser.add_argument("--native", type=pathlib.Path, required=True)
    args = parser.parse_args()
    result = validate_compiled(args.source.read_bytes(), args.expected.read_bytes(),
                               args.native.read_bytes())
    print(json.dumps(result, sort_keys=True))


if __name__ == "__main__":
    main()
