"""Complete engineering graph for the public function-presence diagnostic.

The expected graph is authored from the fixed source, current legal sender and
original native action contracts. It does not read a compiler or observed plist.
This pins the existing dispatcher, including its known incomplete admission; it
does not certify complete RP26/BRG15 envelope dispatch.
"""
import argparse
import hashlib
import json
import pathlib
import plistlib
import re
import xml.etree.ElementTree as ET
from xml.parsers.expat import ExpatError

from native_comparison_graph import verify_equality_region
from tagged_carrier_graph import (
    ACTION_PREFIX, UUID_PATTERN, PLACEHOLDER, attachment, output, variable,
    text_state, dictionary_state, difference,
)
from function_envelope_observation import FIXTURE, PREFIX, FIELDS, CASES

SOURCE_SHA256 = "e3d9c1de1ef9ee78b8a11e73b3c296a746a30cf35b69f6a5fddf683f2318241a"
CONTRACT_SHA256 = "56822e337ae3742cc637f93921c11d9dbf2939990cce4b79dfb6ea6d43c7baac"
WORKFLOW = "CherriFunctionEnvelopePresenceProbe"


def require(ok, reason):
    if not ok:
        raise ValueError("FUNCTION_PRESENCE_GRAPH_" + reason)


class ExpectedProgram:
    def __init__(self):
        self.actions, self.roles, self.comparisons = [], {}, []

    def emit(self, kind, **parameters):
        index = len(self.actions)
        self.actions.append({"WFWorkflowActionIdentifier": ACTION_PREFIX + kind,
                             "WFWorkflowActionParameters": parameters})
        return index

    def empty(self, kind):
        index = len(self.actions)
        self.actions.append({"WFWorkflowActionIdentifier": ACTION_PREFIX + kind})
        return index

    def text(self, value):
        return self.emit("gettext", WFTextActionText=value)

    def number(self, value):
        return self.emit("number", WFNumberActionNumber=value)

    def set(self, name, value=None):
        parameters = {"WFVariableName": name}
        if value is not None:
            parameters["WFInput"] = value
        return self.emit("setvariable", **parameters)

    def set_text(self, name, value):
        return self.set(name, output(self.text(value)))

    def set_number(self, name, value):
        return self.set(name, output(self.number(value)))

    def begin(self, value, condition, right=None):
        index = len(self.actions)
        parameters = {
            "GroupingIdentifier": index, "WFControlFlowMode": 0,
            "WFConditionalLegacyComparisonBehavior": True,
            "WFCondition": condition,
            "WFInput": {"Type": "Variable", "Variable": value},
        }
        if right is not None:
            parameters["WFNumberValue" if type(right) in (int, float)
                       else "WFConditionalActionString"] = right
        self.emit("conditional", **parameters)
        return index

    def boundary(self, group, mode, kind="conditional"):
        return self.emit(kind, GroupingIdentifier=group, WFControlFlowMode=mode)

    def end(self, group, kind="conditional"):
        return self.boundary(group, 2, kind)

    def otherwise(self, group):
        return self.boundary(group, 1)

    def get(self, value, key, internal=False):
        parameters = {"WFInput": value, "WFDictionaryKey": key}
        if internal:
            parameters["WFGetDictionaryValueType"] = "Value"
        return self.emit("getvalueforkey", **parameters)

    def count(self, value):
        return self.emit("count", Input=value)

    def encode(self, value):
        return self.emit("base64encode", WFInput=value, WFBase64LineBreakMode="None",
                         WFEncodeMode="Encode", input="Encode")

    def typeof(self, value):
        return self.emit("getitemtype", WFInput=value)

    def exact_key(self, left, right):
        start = len(self.actions)
        left_snapshot = self.text(text_state("~" + PLACEHOLDER, [output(left)]))
        right_snapshot = self.text("~" + right)
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
        self.number(1.0)
        self.otherwise(compare)
        self.number(0.0)
        result = self.end(compare)
        guard = self.begin(output(result), 4, 1.0)
        self.comparisons.append({"start": start, "guard": guard, "left": left,
                                 "right": right, "quote": quote, "match": match,
                                 "count": count, "result": result})
        return guard

    def trace(self, stage):
        index = len(self.actions)
        payload = {
            "schemaVersion": "1.4", "kind": "INLINE_DEBUG_ACTION", "stage": stage,
            "label": "function-envelope-presence", "stepSequence": index,
            "actionIdentifier": self.actions[-1]["WFWorkflowActionIdentifier"],
            "timestampPrecision": "apple-currentdate",
            "valueType": "UNKNOWN_AT_TRACE_EVENT", "timestamp": PLACEHOLDER}
        event = self.text(text_state(
            json.dumps(payload, ensure_ascii=False, separators=(",", ":")),
            [attachment({"Type": "CurrentDate"})]))
        self.emit("appendvariable", WFInput=output(event), WFVariableName="__cherri_qa_trace")
        self.roles["trace_" + stage] = event

    def optional(self, present, value=None):
        rows = [("_cherri_optional_argument_v1", 0, text_state("1")),
                ("has_value", 0, text_state(present))]
        if value is not None:
            rows.append(("value", 0, text_state(value)))
        return self.emit("dictionary", WFItems=dictionary_state(rows))

    def return_text(self, value):
        return self.emit("output", WFOutput=text_state(PLACEHOLDER, [output(value)]))

    def header(self):
        self.set_text("_cherri_is_fn", "0")
        self.roles["callerSnapshot"] = self.set(
            "_cherri_caller_input", attachment({"Type": "ExtensionInput"}))
        has_input = self.begin(attachment({"Type": "ExtensionInput"}), 100)
        envelope = self.emit("detect.dictionary", WFInput=attachment({"Type": "ExtensionInput"}))
        marker = self.get(output(envelope), "cherri_functions", internal=True)
        magic = self.get(output(envelope), "_cherri_magic", internal=True)
        magic_guard = self.begin(output(magic), 4, "__cherri_fn_envelope_v2__")
        marker_guard = self.begin(output(marker), 4, "1")
        self.roles["existingActivation"] = self.set_text("_cherri_is_fn", "1")
        caller = self.get(output(envelope), "_cherri_caller_input", internal=True)
        self.roles["callerRestore"] = self.set("_cherri_caller_input", output(caller))
        self.end(marker_guard)
        self.end(magic_guard)
        self.end(has_input)
        main = self.begin(variable("_cherri_is_fn"), 4, "1")
        dictionary = self.emit("detect.dictionary", WFInput=attachment({"Type": "ExtensionInput"}))
        name = self.get(output(dictionary), "function", internal=True)
        self.get(output(dictionary), "arguments", internal=True)
        return main, dictionary, name

    def parameter(self, dictionary, optional):
        supplied = self.get(output(dictionary), "has_arg_0", internal=True)
        guard = self.begin(output(supplied), 4, "1")
        self.get(output(dictionary), "arg_0", internal=True)
        self.otherwise(guard)
        if optional:
            self.optional("1", "DEFAULT")
        else:
            self.text("DEFAULT")
        selected = self.end(guard)
        if not optional:
            return selected, None
        presence = self.get(output(selected), "has_value", internal=True)
        present_guard = self.begin(output(presence), 4, "1")
        self.get(output(selected), "value", internal=True)
        self.otherwise(present_guard)
        self.empty("nothing")
        value = self.end(present_guard)
        return value, presence

    def body(self, case, parameter=None, parameter_presence=None):
        envelope = self.emit("detect.dictionary", WFInput=attachment({"Type": "ExtensionInput"}))
        keys = self.emit("getvalueforkey", WFInput=output(envelope),
                         WFGetDictionaryValueType="All Keys")
        for name in ("callerKey", "argumentsKey", "flagKey", "valueKey"):
            self.set_number(name, 0)
        self.set_text("flag", "UNOBSERVED")
        loop = len(self.actions)
        self.emit("repeat.each", GroupingIdentifier=loop, WFControlFlowMode=0, WFInput=output(keys))
        self.set("key", variable("Repeat Item"))
        key_text = self.text(text_state(PLACEHOLDER, [variable("key")]))
        for key, target in (
                ("_cherri_caller_input", "callerKey"), ("arguments", "argumentsKey"),
                ("has_arg_0", "flagKey"), ("arg_0", "valueKey")):
            guard = self.exact_key(key_text, key)
            self.set_number(target, 1)
            if key == "has_arg_0":
                flag = self.get(output(envelope), key)
                self.set("flag", output(self.text(text_state(PLACEHOLDER, [output(flag)]))))
            self.end(guard)
        self.end(loop, "repeat.each")
        self.set_text("rawCount", "UNOBSERVED")
        self.set_text("rawType", "UNOBSERVED")
        caller_guard = self.begin(variable("callerKey"), 4, 1.0)
        raw = self.get(output(envelope), "_cherri_caller_input")
        raw_count = self.count(output(raw))
        self.set("rawCount", output(self.text(text_state(PLACEHOLDER, [output(raw_count)]))))
        raw_nonempty = self.begin(output(raw_count), 2, 0.0)
        raw_type = self.typeof(output(raw))
        self.set("rawType", output(self.encode(output(raw_type))))
        self.end(raw_nonempty)
        caller_guard_end = self.end(caller_guard)
        saved_count = self.count(variable("_cherri_caller_input"))
        self.set_text("savedType", "UNOBSERVED")
        saved_nonempty = self.begin(output(saved_count), 2, 0.0)
        saved_type = self.typeof(variable("_cherri_caller_input"))
        self.set("savedType", output(self.encode(output(saved_type))))
        self.end(saved_nonempty)
        if case == "probeZero":
            parameter_value = output(self.text("NO_PARAMETER"))
        elif case == "probeDefault":
            parameter_value = output(self.encode(output(parameter)))
        else:
            self.set_text("parameterObservation", "NONE")
            parameter_guard = self.begin(output(parameter_presence), 5, "0")
            self.set("parameterObservation", output(self.encode(output(parameter))))
            self.end(parameter_guard)
            parameter_value = variable("parameterObservation")
        key_count = self.count(output(keys))
        references = [
            output(key_count), variable("callerKey"), variable("argumentsKey"),
            variable("flagKey"), variable("valueKey"), variable("flag"),
            variable("rawCount"), output(saved_count), variable("rawType"),
            variable("savedType"), parameter_value]
        line = "CASE=" + case + ";" + ";".join(key + "=" + PLACEHOLDER for key in FIELDS[1:])
        semantic = self.text(text_state(line, references))
        result = self.return_text(semantic)
        self.roles[case] = {
            "envelope": envelope, "keys": keys, "loop": loop, "keyText": key_text,
            "callerGuard": caller_guard, "callerGuardEnd": caller_guard_end,
            "rawCaller": raw, "rawCount": raw_count, "rawType": raw_type,
            "savedCount": saved_count, "savedType": saved_type,
            "keyCount": key_count, "semantic": semantic, "return": result}

    def payload(self, name, flag=None, optional=None):
        rows = [
            ("_cherri_caller_input", 0, variable("_cherri_caller_input")),
            ("_cherri_magic", 0, text_state("__cherri_fn_envelope_v2__"))]
        if optional is not None:
            rows.append(("arg_0", 0, output(optional)))
        mirror = {"WFSerializationType": "WFArrayParameterState"}
        if flag is not None:
            mirror["Value"] = [{"WFItemType": 0,
                                "WFValue": output(optional) if optional is not None else text_state("")}]
        rows.extend([
            ("arguments", 2, mirror), ("cherri_functions", 0, text_state("1")),
            ("function", 0, text_state(name))])
        if flag is not None:
            rows.append(("has_arg_0", 0, text_state(flag)))
        payload = self.emit("dictionary", WFItems=dictionary_state(rows))
        called = self.emit("runworkflow", WFInput=output(payload),
                           WFWorkflow={"isSelf": True, "workflowName": WORKFLOW},
                           WFWorkflowName=WORKFLOW)
        self.roles["sender_" + name] = {"payload": payload, "call": called}
        return called


def expected_program():
    p = ExpectedProgram()
    main, dictionary, name = p.header()
    for case in CASES:
        branch = p.begin(output(name), 4, case)
        parameter = presence = None
        if case != "probeZero":
            parameter, presence = p.parameter(dictionary, case == "probeOptional")
        p.body(case, parameter, presence)
        p.end(branch)
    p.empty("exit")
    p.otherwise(main)
    p.emit("comment", WFCommentActionText="ANCHOR_CARD_01_TOP_HEADER")
    p.emit("nothing")
    p.set("__cherri_debug_log")
    p.trace("before")
    zero = p.payload("probeZero")
    p.emit("comment", WFCommentActionText="ANCHOR_CARD_15_MID_CENTER")
    omitted = p.payload("probeDefault", "0")
    none = p.optional("0")
    supplied = p.payload("probeOptional", "1", none)
    p.roles["suppliedNone"] = none
    p.trace("after")
    p.emit("comment", WFCommentActionText="ANCHOR_CARD_28_BOTTOM_FOOTER")
    semantic = p.text(text_state(
        PREFIX + "\n" + PLACEHOLDER + "\n" + PLACEHOLDER + "\n" + PLACEHOLDER,
        [output(zero), output(omitted), output(supplied)]))
    trace = p.emit("text.combine", WFTextSeparator="New Lines", text=variable("__cherri_qa_trace"))

    def collector(stage):
        return json.dumps({
            "schemaVersion": "1.4", "kind": "COLLECTOR_EVENT", "stage": stage,
            "runId": FIXTURE, "traceTransport": "ndjson-stream",
            "collectorSentinel": "===CHERRI_QA_TRACE_NDJSON_V14==="}, separators=(",", ":"))

    trace_envelope = p.text(text_state(
        collector("collector_begin") + "\n" + PLACEHOLDER + "\n" + collector("collector_end"),
        [output(trace)]))
    final = p.text(text_state(PLACEHOLDER + "\n" + PLACEHOLDER,
                              [output(semantic), output(trace_envelope)]))
    p.return_text(final)
    p.end(main)
    p.roles.update(semantic=semantic, traceEnvelope=trace_envelope, final=final, main=main)
    require(len(p.actions) == 379 and len(p.comparisons) == 12, "EXPECTED_INVENTORY")
    return p


def normalized_actions(actions, expected):
    require(isinstance(actions, list) and len(actions) == len(expected.actions), "ACTION_COUNT")
    ids, groups, stack, alternate, ended = {}, {}, [], set(), set()
    for index, action in enumerate(actions):
        require(isinstance(action, dict) and set(action) == set(expected.actions[index]), "ACTION_SHAPE")
        params = action.get("WFWorkflowActionParameters", {})
        require(isinstance(params, dict), "PARAMETERS")
        identity = params.get("UUID")
        if identity is not None:
            require(isinstance(identity, str) and re.fullmatch(UUID_PATTERN, identity) is not None
                    and identity not in ids, "UNIQUE_PRODUCER")
            ids[identity] = index
        group = params.get("GroupingIdentifier")
        if group is not None:
            kind = action["WFWorkflowActionIdentifier"]
            require(kind in (ACTION_PREFIX + "conditional", ACTION_PREFIX + "repeat.each")
                    and isinstance(group, str) and re.fullmatch(UUID_PATTERN, group) is not None,
                    "CONTROL_IDENTITY")
            mode = params.get("WFControlFlowMode")
            require(type(mode) is int and mode in (0, 1, 2), "CONTROL_MODE")
            if mode == 0:
                require(group not in groups, "CONTROL_REUSE")
                groups[group] = index
                stack.append((group, kind))
            elif mode == 1:
                require(kind == ACTION_PREFIX + "conditional" and stack
                        and stack[-1] == (group, kind) and group not in alternate,
                        "CONTROL_ALTERNATIVE")
                alternate.add(group)
            else:
                require(stack and stack[-1] == (group, kind) and group not in ended, "CONTROL_END")
                stack.pop()
                ended.add(group)
    require(not stack and set(groups) == ended, "CLOSED_CONTROLS")

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
                    and value["OutputUUID"] in ids and ids[value["OutputUUID"]] < current,
                    "ORDERED_PRODUCER_EDGE")
        for key, item in value.items():
            if is_output and key == "OutputName":
                require(isinstance(item, str), "OUTPUT_DISPLAY_NAME")
            elif is_output and key == "OutputUUID":
                result[key] = ids[item]
            else:
                result[key] = normalize(item, current)
        return result

    result = []
    for index, action in enumerate(actions):
        parameters = dict(action.get("WFWorkflowActionParameters", {}))
        parameters.pop("UUID", None)
        display = parameters.pop("CustomOutputName", None)
        require(display is None or isinstance(display, str), "ACTION_DISPLAY_NAME")
        if "GroupingIdentifier" in parameters:
            parameters["GroupingIdentifier"] = groups[parameters["GroupingIdentifier"]]
        item = {"WFWorkflowActionIdentifier": action["WFWorkflowActionIdentifier"]}
        if "WFWorkflowActionParameters" in action:
            item["WFWorkflowActionParameters"] = normalize(parameters, index)
        result.append(item)
    return result


def validate_actions(actions):
    expected = expected_program()
    actual = normalized_actions(actions, expected)
    mismatch = difference(actual, expected.actions)
    require(mismatch is None, "FULL_ACTION_CONTRACT:" + str(mismatch))
    for comparison in expected.comparisons:
        verify_equality_region(
            actions, comparison["start"], comparison["guard"],
            actions[comparison["left"]], right=comparison["right"], require=require)
    return {
        "status": "ENGINEERING_SOURCE_GRAPH_VERIFIED", "fixtureId": FIXTURE,
        "actionCount": len(actions), "exactKeyComparisons": 12, "functionCases": 3,
        "normalization": ["producer/control UUID alpha identity", "action/output display names"],
        "roles": expected.roles,
        "knownExistingIncompleteDispatcherPreserved": True,
        "completeEnvelopeAdmissionVerified": False, "noInputLaunchVerified": False,
        "appleExecutionAuthenticated": False, "formalAcceptanceGranted": False,
    }


def validate_compiled(source, contract, native):
    require(isinstance(source, bytes) and hashlib.sha256(source).hexdigest() == SOURCE_SHA256,
            "SOURCE_SHA256")
    require(isinstance(contract, bytes) and hashlib.sha256(contract).hexdigest() == CONTRACT_SHA256,
            "CONTRACT_SHA256")
    require(isinstance(native, bytes) and 0 < len(native) <= 1024 * 1024, "NATIVE_BYTES")
    try:
        # This diagnostic's compiler export is XML. Reject duplicate Dictionary
        # keys before plistlib can overwrite them, and retain every value type.
        xml = ET.fromstring(native)
        require(xml.tag == "plist" and xml.attrib == {"version": "1.0"}, "XML_PLIST")
        for dictionary in xml.iter("dict"):
            children = list(dictionary)
            require(len(children) % 2 == 0, "XML_DICTIONARY")
            keys = []
            for i in range(0, len(children), 2):
                require(children[i].tag == "key", "XML_KEY")
                keys.append(children[i].text or "")
            require(len(keys) == len(set(keys)), "XML_DUPLICATE_KEY")
        document = plistlib.loads(native)
    except (ValueError, TypeError, plistlib.InvalidFileException, ExpatError, ET.ParseError) as exc:
        raise ValueError("FUNCTION_PRESENCE_GRAPH_NATIVE_PLIST:" + str(exc)) from exc
    require(isinstance(document, dict), "WORKFLOW")
    result = validate_actions(document.get("WFWorkflowActions"))
    result.update(sourceAuthenticated=True, sourceSha256=SOURCE_SHA256,
                  expectedContractSha256=CONTRACT_SHA256,
                  unsignedSha256=hashlib.sha256(native).hexdigest(), unsignedBytes=len(native))
    return result


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", required=True)
    parser.add_argument("--contract", required=True)
    parser.add_argument("--native", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()
    result = validate_compiled(
        pathlib.Path(args.source).read_bytes(), pathlib.Path(args.contract).read_bytes(),
        pathlib.Path(args.native).read_bytes())
    pathlib.Path(args.out).write_text(json.dumps(result, indent=2) + "\n")


if __name__ == "__main__":
    main()
