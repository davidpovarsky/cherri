"""Source-bound CLI graph/roundtrip evidence for four synthetic continuation probes.

The assertions inspect emitted native actions. They do not model storage, HTTP
conversion or Number comparison outcomes on Apple runtime.
"""
import argparse
import copy
import hashlib
import json
import os
from pathlib import Path
import plistlib
import re
import subprocess
import sys
import time

from verify_storage_probes import verify_build_identity

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "tools/shortcuts-simulator-qa"))
from native_comparison_graph import verify_equality_region
PUBLIC_URL = ("https://raw.githubusercontent.com/davidpovarsky/cherri/"
              "38c7098772f4b822d6a3555137c0c7cf8e5b1e5e/"
              "docs/apple-runtime/p10r15/paginate-pages/one.json")
PREFIX = "is.workflow.actions."


def require(value, reason):
    if not value:
        raise ValueError(reason)


def params(action):
    value = action.get("WFWorkflowActionParameters", {})
    require(isinstance(value, dict), "native parameters must be a dictionary")
    return value


def suffix(action):
    return action["WFWorkflowActionIdentifier"].removeprefix(PREFIX)


def walk(value):
    yield value
    if isinstance(value, dict):
        for nested in value.values():
            yield from walk(nested)
    elif isinstance(value, list):
        for nested in value:
            yield from walk(nested)


def refs(value):
    return [v["OutputUUID"] for v in walk(value)
            if isinstance(v, dict) and v.get("Type") == "ActionOutput" and "OutputUUID" in v]


def variables(value):
    return [v["VariableName"] for v in walk(value)
            if isinstance(v, dict) and v.get("Type") == "Variable" and "VariableName" in v]


def single_ref(value):
    found = refs(value)
    require(len(found) == 1, "exactly one native producer reference is required")
    return found[0]


def output_named(actions, name):
    found = [a for a in actions if params(a).get("CustomOutputName") == name]
    require(len(found) == 1 and params(found[0]).get("UUID"), "unique output identity missing: " + name)
    return found[0]


def text_parts(action):
    require(suffix(action) == "gettext", "Text witness must be a real Text action")
    value = params(action)["WFTextActionText"]
    if isinstance(value, str):
        return value, []
    require(value.get("WFSerializationType") == "WFTextTokenString", "unexpected Text codec")
    return value["Value"]["string"], refs(value)


def verify_debug_clear(actions):
    # These four reviewed sources each begin with one debugClearLogs call after
    # the public anchor. Nothing is permitted only as this exact zero-output
    # producer; it is not a general effect allowlist entry.
    require(len(actions) >= 3 and suffix(actions[0]) == "comment"
            and params(actions[0]).get("WFCommentActionText") == "ANCHOR_CARD_01_TOP_HEADER",
            "reviewed debug-clear anchor changed")
    clear, target = actions[1], actions[2]
    uid = params(clear).get("UUID")
    require(suffix(clear) == "nothing" and set(params(clear)) == {"UUID"}
            and isinstance(uid, str)
            and re.fullmatch(r"[0-9A-Fa-f]{8}(?:-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12}", uid),
            "debug clear requires one parameterless Nothing producer")
    require(suffix(target) == "setvariable"
            and params(target) == {"WFVariableName": "__cherri_debug_log"},
            "debug clear target must consume implicit zero output")
    require(sum(suffix(a) == "nothing" for a in actions) == 1,
            "extra Nothing action is unreviewed")
    require(all(uid not in refs(params(a)) for a in actions),
            "zero-output clear cannot be referenced as a value")
    require([i for i, a in enumerate(actions) if suffix(a) == "setvariable"
             and params(a).get("WFVariableName") == "__cherri_debug_log"] == [2],
            "unreviewed additional debug-log assignment")


def storage_type_pairs(actions):
    original = output_named(actions, "original")
    pairs = []
    require(sum(suffix(a) == "getitemtype" for a in actions) == 4,
            "exact four storage type witnesses required")
    for name in ("written", "restored"):
        read = output_named(actions, name)
        found = [(i, a) for i, a in enumerate(actions)
                 if suffix(a) == "getitemtype"
                 and refs(params(a).get("WFInput")) == [params(read)["UUID"]]]
        require(len(found) == 1, "readback native Get Type witness missing")
        index, actual = found[0]
        require(index > 0, "expected type witness missing")
        expected = actions[index - 1]
        require(suffix(expected) == "getitemtype"
                and refs(params(expected).get("WFInput")) == [params(original)["UUID"]],
                "readback expected type must inspect the original Text")
        pairs.append((name + "-type", actual, expected, 5))
    return pairs


def http_type_pairs(actions):
    return [
        ("raw-text-type", output_named(actions, "rawType"),
         output_named(actions, "textType"), 4),
        ("raw-dictionary-type", output_named(actions, "rawType"),
         output_named(actions, "dictionaryType"), 4),
        ("parsed-dictionary-type", output_named(actions, "parsedType"),
         output_named(actions, "dictionaryType"), 5),
    ]


def verify_type_comparisons(actions, pairs):
    regions, occupied = [], set()
    for name, left, right, operator in pairs:
        require(suffix(left) == suffix(right) == "getitemtype",
                "exact type comparison requires actual Get Type producers")
        left_uuid, right_uuid = params(left).get("UUID"), params(right).get("UUID")
        require(left_uuid and right_uuid and left_uuid != right_uuid,
                "independent type producer identities required")
        starts = [i for i in range(len(actions) - 1)
                  if suffix(actions[i]) == suffix(actions[i + 1]) == "gettext"
                  and refs(params(actions[i]).get("WFTextActionText")) == [left_uuid]
                  and refs(params(actions[i + 1]).get("WFTextActionText")) == [right_uuid]]
        require(len(starts) == 1, "one exact type comparison region per source predicate")
        start = starts[0]
        # The shared verifier checks all ten actions, their producer graph and
        # the consuming conditional; this is not a general effect allowlist.
        condition = start + 10
        verify_equality_region(actions, start, condition, left, right_source=right,
                               operator=operator, require=require)
        indices = set(range(start, condition))
        require(not (occupied & indices), "overlapping exact comparison regions")
        occupied.update(indices)
        regions.append({"name": name, "start": start, "condition": condition,
                        "leftUUID": left_uuid, "rightUUID": right_uuid,
                        "operator": operator})
    return regions


def verify_failure_flag(actions, condition, flag, value):
    body = actions[condition + 1:condition + 4]
    require(len(body) == 3 and suffix(body[0]) == "gettext"
            and text_parts(body[0])[0] == value
            and suffix(body[1]) == "setvariable"
            and params(body[1]).get("WFVariableName") == flag
            and refs(params(body[1]).get("WFInput")) == [params(body[0]).get("UUID")]
            and suffix(body[2]) == "conditional"
            and params(body[2]).get("WFControlFlowMode") == 2
            and params(body[2]).get("GroupingIdentifier")
                == params(actions[condition]).get("GroupingIdentifier"),
            "source failure flag must remain in its own comparison branch")


def common_graph(actions, allowed, comparisons=()):
    require(isinstance(actions, list) and bool(actions), "empty action graph")
    verify_debug_clear(actions)
    regions = verify_type_comparisons(actions, comparisons)
    reviewed_indices = {i for r in regions for i in range(r["start"], r["condition"])}
    producers, frames, contexts, groups = {}, [], {}, {}
    for index, action in enumerate(actions):
        reviewed_clear = index == 1 and suffix(action) == "nothing"
        require(action.get("WFWorkflowActionIdentifier", "").startswith(PREFIX)
                and (suffix(action) in allowed or reviewed_clear or index in reviewed_indices), "unreviewed native effect")
        p = params(action)
        for ref in refs(p):
            require(ref in producers, "missing or nonpreceding native producer")
        if suffix(action) == "conditional":
            mode, group = p.get("WFControlFlowMode"), p.get("GroupingIdentifier")
            if mode == 0:
                require(group and group not in groups, "duplicate conditional group")
                groups[group] = index
                frames.append((group, p.get("WFConditionalActionString"), False))
            elif mode == 1:
                require(frames and frames[-1][0] == group and not frames[-1][2], "unbalanced Else")
                frames[-1] = (*frames[-1][:2], True)
            elif mode == 2:
                require(frames and frames[-1][0] == group, "unbalanced End If")
                frames.pop()
            else:
                raise ValueError("unknown conditional mode")
        contexts[index] = list(frames)
        uuid = p.get("UUID")
        if uuid:
            require(uuid not in producers, "duplicate native producer UUID")
            producers[uuid] = action
    require(not frames, "unterminated conditional")
    return producers, contexts, regions


BASE = {"comment", "gettext", "setvariable", "appendvariable", "text.combine",
        "output", "conditional", "count", "getitemtype"}


def storage_graph(actions):
    producers, contexts, regions = common_graph(actions, BASE | {
        "detect.dictionary", "getvalueforkey", "setstoredcontent", "getstoredcontent", "deletestoredcontent"},
        storage_type_pairs(actions))
    phase = output_named(actions, "phase")
    run_token = output_named(actions, "runToken")
    request = output_named(actions, "request")
    require(suffix(request) == "detect.dictionary", "input request must use canonical Get Dictionary")
    for action, key in ((phase, "phase"), (run_token, "runToken")):
        require(suffix(action) == "getvalueforkey" and params(action)["WFDictionaryKey"] == key
                and single_ref(params(action)["WFInput"]) == params(request)["UUID"], "phase/token input binding changed")
    key, original = output_named(actions, "key"), output_named(actions, "original")
    key_text, key_refs = text_parts(key)
    value_text, value_refs = text_parts(original)
    require(key_text == "cherri.qa.p15.storage.cross-run.\ufffc"
            and key_refs == [params(run_token)["UUID"]], "owned fresh key binding changed")
    require(value_text.startswith("P15_STORAGE_CROSS_RUN:\ufffc:")
            and "\u05e9\u05b8\u05c1\u05dc\u05d5\u05b9\u05dd" in value_text and "\U0001f352" in value_text
            and value_refs == key_refs, "original Unicode Text/token witness changed")
    phase_guards = [a for a in actions if suffix(a) == "conditional"
                    and params(a).get("WFControlFlowMode") == 0
                    and params(a).get("WFConditionalActionString") in ("WRITE", "READ")]
    require(len(phase_guards) == 2 and all(params(a)["WFCondition"] == 4
            and single_ref(params(a)["WFInput"]) == params(phase)["UUID"] for a in phase_guards),
            "phase guard changed")
    observed, read_ids = [], []
    for index, action in enumerate(actions):
        if not suffix(action).endswith("storedcontent"):
            continue
        p = params(action)
        require(p.get("WFStoredContentGlobalValue") is True, "global flag changed")
        require(single_ref(p["WFStoredContentKey"]) == params(key)["UUID"], "key retargeted")
        context = [(label, is_else) for _, label, is_else in contexts[index] if isinstance(label, str)]
        if suffix(action) == "setstoredcontent":
            require(context == [("WRITE", False)], "store must occur only in writer branch")
            require(single_ref(p["WFInput"]) == params(original)["UUID"], "store original Text retargeted")
        elif suffix(action) == "deletestoredcontent":
            require(context == [("WRITE", True), ("READ", False)], "cleanup must occur only in reader branch")
        else:
            require(context in ([("WRITE", False)], [("WRITE", True), ("READ", False)]),
                    "read escaped reviewed phase")
            read_ids.append(p["UUID"])
        observed.append((suffix(action), context))
    require([s for s, _ in observed] == ["setstoredcontent", "getstoredcontent", "getstoredcontent", "deletestoredcontent"],
            "storage operations changed")
    positions = {id(a): i for i, a in enumerate(actions)}
    source_conditions = {positions[id(a)] for a in phase_guards}
    for read_uuid, region in zip(read_ids, regions):
        read = producers[read_uuid]
        read_index = positions[id(read)]
        require(contexts[region["start"]] == contexts[read_index]
                and contexts[region["condition"]][:-1] == contexts[read_index],
                "type comparison moved outside its storage phase")
        verify_failure_flag(actions, region["condition"], "typeFlag", "TYPE_FAIL")
        source_conditions.add(region["condition"])
        comparisons = [i for i, a in enumerate(actions) if suffix(a) == "conditional"
                       and params(a).get("WFControlFlowMode") == 0
                       and refs(params(a).get("WFInput")) == [read_uuid]]
        require(len(comparisons) == 1, "one stored-value predicate per readback")
        condition = comparisons[0]
        # Stored Content remains dynamic in the reviewed source. Preserve its
        # explicit String coercion and original operand; do not claim typed
        # exact Text semantics for this separate runtime observation.
        verify_equality_region(actions, condition, condition, read,
                               right_source=original, operator=5, dynamic=True,
                               coercion="WFStringContentItem", require=require)
        require(contexts[condition][:-1] == contexts[read_index],
                "value comparison moved outside its storage phase")
        verify_failure_flag(actions, condition, "valueFlag", "VALUE_FAIL")
        source_conditions.add(condition)
    internal = {i for r in regions for i in range(r["start"], r["condition"])}
    require({i for i, a in enumerate(actions) if suffix(a) == "conditional"
             and params(a).get("WFControlFlowMode") == 0 and i not in internal}
            == source_conditions, "storage source predicate inventory changed")
    return {"nativeActions": len(actions), "storageOperations": [
        {"native": native, "phaseContext": context} for native, context in observed],
        "sameKeyProducerEveryOperation": True, "originalUnicodeTextReferencePreserved": True,
        "exactTypeComparisons": regions, "nativeStorageExecuted": False}


def http_graph(fid, actions):
    producers, contexts, regions = common_graph(actions, BASE | {
        "dictionary", "getvalueforkey", "detect.dictionary", "downloadurl"}, http_type_pairs(actions))
    fetched = fid == "qa-http-json-fetch-v2"
    response = output_named(actions, "response")
    network = [a for a in actions if suffix(a) == "downloadurl"]
    require(len(network) == int(fetched), "unreviewed network operation")
    if fetched:
        p = params(response)
        require(suffix(response) == "downloadurl" and p.get("WFURL") == PUBLIC_URL
                and p.get("WFHTTPMethod") == "GET"
                and set(p) <= {"UUID", "CustomOutputName", "WFURL", "WFHTTPMethod"}, "GET endpoint/headers/method changed")
    else:
        require(text_parts(response)[0] == '{"items":[{"id":1}],"next":"two.json"}',
                "literal JSON control changed")
    parsed = output_named(actions, "parsed")
    require(suffix(parsed) == "detect.dictionary"
            and refs(params(parsed).get("WFInput")) == [params(response)["UUID"]],
            "actual response must reach canonical Get Dictionary")
    for name, value in (("rawType", response), ("parsedType", parsed),
                        ("dictionaryType", output_named(actions, "dictionaryWitness")),
                        ("textType", output_named(actions, "textWitness"))):
        action = output_named(actions, name)
        require(suffix(action) == "getitemtype"
                and refs(params(action).get("WFInput")) == [params(value)["UUID"]], "type witness retargeted")
    for name in ("next", "items"):
        action = output_named(actions, name)
        require(suffix(action) == "getvalueforkey" and params(action).get("WFDictionaryKey") == name
                and refs(params(action).get("WFInput")) == [params(parsed)["UUID"]], "parsed field retargeted")
    count = output_named(actions, "itemCount")
    require(suffix(count) == "count" and refs(params(count)) == [params(output_named(actions, "items"))["UUID"]],
            "parsed items count retargeted")
    labels = ("http-json-raw-type-text", "http-json-raw-type-dictionary", "http-json-raw-type-other")
    rendered = json.dumps(actions, ensure_ascii=False)
    require(all(label in rendered for label in labels), "raw type diagnostic trace branch missing")
    require(sum(suffix(a) == "getitemtype" for a in actions) == 4,
            "exact four HTTP type witnesses required")
    first, second, typed = regions
    first_context = contexts[first["condition"]]
    second_context = contexts[second["condition"]]
    require(contexts[first["start"]] == [] and len(first_context) == 1
            and contexts[second["start"]] == [(first_context[0][0], None, True)]
            and len(second_context) == 2
            and second_context[:-1] == contexts[second["start"]]
            and contexts[typed["start"]] == []
            and len(contexts[typed["condition"]]) == 1,
            "HTTP type predicates escaped their source branches")
    expected_contexts = {
        labels[0]: first_context,
        labels[1]: second_context,
        labels[2]: [*second_context[:-1], (second_context[-1][0], None, True)],
    }
    for label, expected_context in expected_contexts.items():
        found = []
        for index, action in enumerate(actions):
            if suffix(action) != "gettext":
                continue
            try:
                event = json.loads(text_parts(action)[0])
            except (ValueError, TypeError):
                continue
            if isinstance(event, dict) and event.get("label") == label:
                require(event.get("kind") == "INLINE_DEBUG_ACTION" and event.get("stage") == "before",
                        "HTTP diagnostic label is not the original trace event")
                found.append(index)
        require(len(found) == 1 and contexts[found[0]] == expected_context,
                "HTTP raw type trace moved to another comparison branch")
    verify_failure_flag(actions, typed["condition"], "typeFlag", "DICTIONARY_FAIL")
    source_conditions = {r["condition"] for r in regions}
    for name, flag, failure in (("next", "nextFlag", "NEXT_FAIL"),
                                ("itemCount", "itemsFlag", "ITEMS_FAIL")):
        producer = output_named(actions, name)
        found = [i for i, action in enumerate(actions) if suffix(action) == "conditional"
                 and params(action).get("WFControlFlowMode") == 0
                 and refs(params(action).get("WFInput")) == [params(producer)["UUID"]]]
        require(len(found) == 1, "one original HTTP result predicate required")
        condition = found[0]
        if name == "next":
            verify_equality_region(actions, condition, condition, producer,
                                   right="two.json", operator=5, dynamic=True,
                                   coercion="WFStringContentItem", require=require)
        else:
            p = params(actions[condition])
            require(p.get("WFCondition") == 5
                    and type(p.get("WFNumberValue")) in (int, float) and p["WFNumberValue"] == 1
                    and p.get("WFConditionalLegacyComparisonBehavior") is True
                    and "WFConditionalActionString" not in p
                    and not variables(p["WFInput"])
                    and not any(isinstance(v, dict) and "Aggrandizements" in v
                                for v in walk(p["WFInput"])),
                    "HTTP item-count direct numeric predicate changed")
        require(len(contexts[condition]) == 1,
                "HTTP result predicate moved into a different branch")
        verify_failure_flag(actions, condition, flag, failure)
        source_conditions.add(condition)
    internal = {i for r in regions for i in range(r["start"], r["condition"])}
    require({i for i, a in enumerate(actions) if suffix(a) == "conditional"
             and params(a).get("WFControlFlowMode") == 0 and i not in internal}
            == source_conditions, "HTTP source predicate inventory changed")
    return {"nativeActions": len(actions), "fixedGETs": len(network),
            "actualResponseToGetDictionary": True, "rawTypeDiagnostic": "TEXT_DICTIONARY_OTHER",
            "exactTypeComparisons": regions, "nativeHTTPConversionExecuted": False}


def numeric_graph(actions):
    producers, _, _ = common_graph(actions, BASE | {"number", "getvariable", "alert", "exit"})
    direct, required, one = [output_named(actions, n) for n in ("direct", "required", "one")]
    require(suffix(direct) == suffix(one) == "number", "independent native Number controls missing")
    require(str(params(direct).get("WFNumberActionNumber")) in ("0", "0.0")
            and str(params(one).get("WFNumberActionNumber")) in ("1", "1.0"), "Number control values changed")
    require(suffix(required) == "getvariable", "require result must retain its actual native value")
    required_source = producers[single_ref(params(required)["WFVariable"])]
    require(suffix(required_source) == "number" and params(required_source).get("WFNumberActionNumber") == 0,
            "require original zero producer changed")
    assignments = [a for a in actions if suffix(a) == "setvariable" and params(a).get("WFVariableName") == "bound"]
    require(len(assignments) == 1 and refs(params(assignments[0]).get("WFInput")) == [params(required)["UUID"]],
            "bound variable no longer receives exact require result")
    expected = [
        ("directEq", direct, 4, 0), ("directNe", direct, 5, 0),
        ("requiredEq", required, 4, 0), ("requiredNe", required, 5, 0),
        ("boundEq", None, 4, 0), ("boundNe", None, 5, 0),
        ("oneNeZero", one, 5, 0), ("zeroNeOne", direct, 5, 1), ("oneEqOne", one, 4, 1),
    ]
    selected = []
    for index, action in enumerate(actions):
        p = params(action)
        if suffix(action) != "conditional" or p.get("WFControlFlowMode") != 0:
            continue
        if (set(refs(p.get("WFInput"))) & {params(a)["UUID"] for a in (direct, required, one)}
                or variables(p.get("WFInput")) == ["bound"]):
            selected.append((index, p))
    require(len(selected) == len(expected), "numeric source condition count changed")
    matrix = []
    for (index, p), (flag, producer, op, literal) in zip(selected, expected):
        require(p.get("WFCondition") == op and type(p.get("WFNumberValue")) in (int, float)
                and p["WFNumberValue"] == literal and "WFConditionalActionString" not in p,
                "numeric condition operator/literal wire changed")
        require(not any(isinstance(v, dict) and "Aggrandizements" in v for v in walk(p["WFInput"])),
                "numeric input gained an unreviewed coercion")
        require(refs(p["WFInput"]) == ([params(producer)["UUID"]] if producer else [])
                and variables(p["WFInput"]) == ([] if producer else ["bound"]), "numeric producer retargeted")
        flag_sets = [(i, a) for i, a in enumerate(actions) if suffix(a) == "setvariable"
                     and params(a).get("WFVariableName") == flag]
        require(len(flag_sets) == 2 and flag_sets[0][0] < index
                and text_parts(producers[single_ref(params(flag_sets[0][1])["WFInput"])])[0] == "N",
                "numeric flag must start false before its own condition")
        body = actions[index + 1:index + 4]
        require(len(body) == 3 and suffix(body[0]) == "gettext" and text_parts(body[0])[0] == "Y"
                and suffix(body[1]) == "setvariable" and params(body[1]).get("WFVariableName") == flag
                and refs(params(body[1]).get("WFInput")) == [params(body[0])["UUID"]]
                and suffix(body[2]) == "conditional" and params(body[2]).get("WFControlFlowMode") == 2
                and params(body[2]).get("GroupingIdentifier") == p["GroupingIdentifier"], "numeric outcome flag body changed")
        matrix.append({"flag": flag, "WFCondition": p["WFCondition"],
                       "WFNumberValue": p["WFNumberValue"], "WFInput": p["WFInput"]})
    for left, right in ((0, 1), (2, 3), (4, 5)):
        require(selected[left][1]["WFInput"] == selected[right][1]["WFInput"],
                "paired operators do not consume identical native input")
    verdict = output_named(actions, "verdict")
    value = params(verdict)["WFTextActionText"]["Value"]
    attachment_items = list(value["attachmentsByRange"].items())
    require(all(re.fullmatch(r"\{\d+, 1\}", location) for location, _ in attachment_items),
            "summary attachment range changed")
    attachment_items.sort(key=lambda item: int(item[0].split(",")[0][1:]))
    require([token.get("VariableName") for _, token in attachment_items] == [row[0] for row in expected]
            and value["string"] == "DEQ=\ufffc:DNE=\ufffc:REQ=\ufffc:RNE=\ufffc:BEQ=\ufffc:BNE=\ufffc:ONE_NE_ZERO=\ufffc:ZERO_NE_ONE=\ufffc:ONE_EQ_ONE=\ufffc",
            "summary omitted, reordered or substituted a result flag")
    return {"nativeActions": len(actions), "pairedSameProducerComparisons": 3,
            "positiveOperatorControls": 3, "conditions": matrix, "nativeNumericComparisonExecuted": False}


def validate_graph(fid, actions):
    if fid == "qa-storage-text-cross-run-v2":
        return storage_graph(actions)
    if fid in ("qa-http-json-control-v2", "qa-http-json-fetch-v2"):
        return http_graph(fid, actions)
    if fid == "qa-number-comparison-diag-v2":
        return numeric_graph(actions)
    raise ValueError("unreviewed fixture")


def causal_faults(fid, actions):
    faults = []
    def mutate(name, change):
        altered = copy.deepcopy(actions)
        change(altered)
        try:
            validate_graph(fid, altered)
        except (ValueError, KeyError, TypeError, IndexError):
            faults.append(name)
        else:
            raise RuntimeError("causal graph fault accepted: " + fid + "/" + name)
    def first(items, native):
        return next(a for a in items if suffix(a) == native)
    mutate("clear-empty-text", lambda a: a[1].update(
        WFWorkflowActionIdentifier=PREFIX+"gettext",
        WFWorkflowActionParameters={"UUID": params(a[1])["UUID"], "WFTextActionText": ""}))
    mutate("clear-extra-parameter", lambda a: params(a[1]).update(WFInput="unexpected"))
    mutate("clear-explicit-input", lambda a: params(a[2]).update(WFInput={
        "WFSerializationType": "WFTextTokenAttachment",
        "Value": {"Type": "ActionOutput", "OutputUUID": params(a[1])["UUID"]}}))
    mutate("clear-missing-uuid", lambda a: params(a[1]).pop("UUID"))
    mutate("clear-foreign-variable", lambda a: params(a[2]).update(WFVariableName="foreign"))
    mutate("clear-reordered", lambda a: a.__setitem__(slice(1, 3), list(reversed(a[1:3]))))
    mutate("clear-duplicate-pair", lambda a: a.extend(copy.deepcopy(a[1:3])))
    mutate("extra-nothing", lambda a: a.append({"WFWorkflowActionIdentifier": PREFIX+"nothing",
        "WFWorkflowActionParameters": {"UUID": "00000000-0000-0000-0000-000000000000"}}))
    mutate("clear-late-write", lambda a: a.append({"WFWorkflowActionIdentifier": PREFIX+"setvariable",
        "WFWorkflowActionParameters": {"WFVariableName": "__cherri_debug_log"}}))
    mutate("clear-value-consumed", lambda a: a.append({"WFWorkflowActionIdentifier": PREFIX+"gettext",
        "WFWorkflowActionParameters": {"WFTextActionText": {"WFSerializationType": "WFTextTokenAttachment",
        "Value": {"Type": "ActionOutput", "OutputUUID": params(a[1])["UUID"]}}}}))
    mutate("extra-network", lambda a: a.append({"WFWorkflowActionIdentifier": PREFIX + "downloadurl",
          "WFWorkflowActionParameters": {"WFURL": "https://unreviewed.invalid/", "WFHTTPMethod": "GET"}}))
    if "storage" in fid:
        mutate("foreign-key", lambda a: params(first(a, "setstoredcontent")).update(WFStoredContentKey="foreign"))
        mutate("wrong-scope", lambda a: params(first(a, "setstoredcontent")).update(WFStoredContentGlobalValue=False))
        mutate("delete-before-reader", lambda a: a.insert(0, a.pop(next(i for i,x in enumerate(a) if suffix(x)=="deletestoredcontent"))))
        mutate("read-replaced-by-text", lambda a: first(a, "getstoredcontent").update(WFWorkflowActionIdentifier=PREFIX+"gettext"))
        mutate("wrong-phase", lambda a: params(next(x for x in a if suffix(x)=="conditional" and params(x).get("WFConditionalActionString")=="WRITE")).update(WFCondition=5))
    elif "http-json" in fid:
        mutate("parsed-response-retarget", lambda a: params(output_named(a, "parsed")).update(
            WFInput=copy.deepcopy(params(output_named(a, "dictionaryType"))["WFInput"])))
        mutate("raw-type-retarget", lambda a: params(output_named(a, "rawType")).update(
            WFInput=copy.deepcopy(params(output_named(a, "dictionaryType"))["WFInput"])))
        mutate("next-selector", lambda a: params(output_named(a, "next")).update(WFDictionaryKey="wrong"))
        if "fetch" in fid:
            mutate("foreign-url", lambda a: params(first(a, "downloadurl")).update(WFURL="https://unreviewed.invalid/"))
            mutate("post-method", lambda a: params(first(a, "downloadurl")).update(WFHTTPMethod="POST"))
            mutate("extra-header", lambda a: params(first(a, "downloadurl")).update(WFHTTPHeaders={"X-Unreviewed":"synthetic"}))
    else:
        def condition(items, name):
            uid = params(output_named(items, name))["UUID"]
            return next(x for x in items if suffix(x)=="conditional" and params(x).get("WFControlFlowMode")==0
                        and refs(params(x).get("WFInput"))==[uid] and params(x).get("WFCondition")==5)
        mutate("paired-operator", lambda a: params(condition(a,"direct")).update(WFCondition=4))
        mutate("number-as-string", lambda a: params(condition(a,"direct")).update(WFNumberValue="0"))
        mutate("required-retarget", lambda a: params(condition(a,"required")).update(WFInput=copy.deepcopy(params(condition(a,"direct"))["WFInput"])))
        mutate("positive-control-value", lambda a: params(output_named(a,"one")).update(WFNumberActionNumber="0"))
        mutate("bound-retarget", lambda a: params(next(x for x in a if suffix(x)=="setvariable" and params(x).get("WFVariableName")=="bound")).update(
            WFInput=copy.deepcopy(params(next(x for x in a if suffix(x)=="conditional" and refs(params(x).get("WFInput"))==[params(output_named(a,"direct"))["UUID"]]))["WFInput"])))
    regions = validate_graph(fid, actions).get("exactTypeComparisons", [])
    def attachment(items, index, field):
        values = [v for v in walk(params(items[index])[field])
                  if isinstance(v, dict) and v.get("Type") == "ActionOutput"]
        require(len(values) == 1, "causal control must select one actual dependency")
        return values[0]
    for region in regions:
        start, condition = region["start"], region["condition"]
        prefix = "exact-" + region["name"] + "-"
        mutate(prefix + "left-source", lambda a: attachment(a, start, "WFTextActionText").update(
            OutputUUID=region["rightUUID"]))
        mutate(prefix + "right-source", lambda a: attachment(a, start + 1, "WFTextActionText").update(
            OutputUUID=region["leftUUID"]))
        mutate(prefix + "left-sentinel", lambda a: params(a[start])["WFTextActionText"]["Value"].update(
            string="\ufffc"))
        mutate(prefix + "quote-escape", lambda a: params(a[start + 2]).update(WFReplaceTextFind="\\Q"))
        mutate(prefix + "quote-regex-mode", lambda a: params(a[start + 2]).update(
            WFReplaceTextRegularExpression=True))
        def unanchor(a):
            value = params(a[start + 3])["WFMatchTextPattern"]["Value"]
            token = next(iter(value["attachmentsByRange"].values()))
            value.update(string="\\Q\ufffc\\E", attachmentsByRange={"{2, 1}": token})
        mutate(prefix + "unanchored-pattern", unanchor)
        mutate(prefix + "case-folding", lambda a: params(a[start + 3]).update(
            WFMatchTextCaseSensitive=False))
        mutate(prefix + "count-source", lambda a: attachment(a, start + 4, "Input").update(
            OutputUUID=params(a[start])["UUID"]))
        mutate(prefix + "count-kind", lambda a: params(a[start + 4]).update(WFCountType="Characters"))
        mutate(prefix + "count-threshold", lambda a: params(a[start + 5]).update(WFNumberValue=0))
        mutate(prefix + "comparison-operator", lambda a: params(a[start + 5]).update(
            WFCondition=5 if region["operator"] == 4 else 4))
        mutate(prefix + "true-polarity", lambda a: params(a[start + 6]).update(WFNumberActionNumber=0))
        mutate(prefix + "false-polarity", lambda a: params(a[start + 8]).update(WFNumberActionNumber=1))
        mutate(prefix + "consumer-polarity", lambda a: params(a[condition]).update(WFNumberValue=0))
        mutate(prefix + "consumer-source", lambda a: attachment(a, condition, "WFInput").update(
            OutputUUID=params(a[start + 4])["UUID"]))
        mutate(prefix + "group-boundary", lambda a: params(a[start + 9]).update(
            GroupingIdentifier=params(a[condition])["GroupingIdentifier"]))
        mutate(prefix + "extra-region-action", lambda a: a.insert(start + 3, {
            "WFWorkflowActionIdentifier": PREFIX + "number",
            "WFWorkflowActionParameters": {"WFNumberActionNumber": 1}}))
        mutate(prefix + "extra-match-outside-region", lambda a: a.append(copy.deepcopy(a[start + 3])))
        def type_source(a):
            left = next(x for x in a if params(x).get("UUID") == region["leftUUID"])
            right = next(x for x in a if params(x).get("UUID") == region["rightUUID"])
            params(left)["WFInput"] = copy.deepcopy(params(right)["WFInput"])
        mutate(prefix + "type-producer-source", type_source)
        if "storage" in fid or region["name"] == "parsed-dictionary-type":
            mutate(prefix + "failure-flag", lambda a: params(a[condition + 2]).update(
                WFVariableName="unreviewed_flag"))
    if "storage" in fid:
        def move_type_to_else(a):
            region = regions[0]
            block = a[region["start"] - 2:region["condition"] + 4]
            del a[region["start"] - 2:region["condition"] + 4]
            phase = next(x for x in a if suffix(x) == "conditional"
                         and params(x).get("WFConditionalActionString") == "WRITE")
            group = params(phase)["GroupingIdentifier"]
            otherwise = next(i for i, x in enumerate(a) if params(x).get("GroupingIdentifier") == group
                             and params(x).get("WFControlFlowMode") == 1)
            a[otherwise + 1:otherwise + 1] = block
        mutate("exact-storage-type-moved-across-phase", move_type_to_else)
    elif "http-json" in fid:
        def move_else_after_dictionary(a):
            group = params(a[regions[0]["condition"]])["GroupingIdentifier"]
            otherwise = next(i for i, x in enumerate(a) if params(x).get("GroupingIdentifier") == group
                             and params(x).get("WFControlFlowMode") == 1)
            marker = a.pop(otherwise)
            end = next(i for i, x in enumerate(a) if params(x).get("GroupingIdentifier") == group
                       and params(x).get("WFControlFlowMode") == 2)
            a.insert(end, marker)
        mutate("exact-http-dictionary-outside-else", move_else_after_dictionary)
    else:
        mutate("numeric-boolean-is-not-number", lambda a: params(condition(a, "direct")).update(
            WFNumberValue=False))
    return faults


def checked(argv, timeout=180):
    result = subprocess.run([str(x) for x in argv], cwd=ROOT, capture_output=True,
                            text=True, encoding="utf-8", errors="strict", timeout=timeout)
    if result.returncode:
        raise RuntimeError("command failed: " + str(argv[:3]) + ": " + (result.stdout + result.stderr)[-1500:])
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("compiler", type=Path)
    parser.add_argument("--build", action="store_true")
    parser.add_argument("--output", type=Path, default=ROOT/"artifacts/p15-continuation-probes/validation.json")
    args = parser.parse_args()
    started = time.monotonic()
    sha = checked(["git", "rev-parse", "HEAD"]).stdout.strip()
    require(not checked(["git", "status", "--porcelain"]).stdout.strip(), "clean committed source required")
    compiler, output = args.compiler.resolve(), args.output.resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    if args.build:
        checked(["git", "check-ignore", "--", compiler.relative_to(ROOT)])
        checked(["go", "build", "-buildvcs=true", "-o", compiler, "."])
    identity = verify_build_identity(checked(["go", "version", "-m", compiler]).stdout,
                                    sha, args.build, (ROOT/".git").is_file())
    manifest_path = ROOT/"docs/apple-runtime/p10r15/native-continuation-probe-manifest.json"
    manifest = json.loads(manifest_path.read_bytes())
    cases = manifest["cases"]
    require(len(cases)==4 and len({c["id"] for c in cases})==4, "four explicit cases required")
    paths = [manifest_path.relative_to(ROOT).as_posix(), "tools/p10r15/verify_native_continuation_probes.py",
        "tools/shortcuts-simulator-qa/storage_cross_run.py", "tools/shortcuts-simulator-qa/test_storage_cross_run.py",
        "tools/shortcuts-simulator-qa/native_comparison_graph.py",
        "tools/shortcuts-simulator-qa/prepare_public_artifacts.py", "scripts/ios27_shortcuts_simulator_qa.sh",
        ".github/workflows/cherri-ios27-shortcuts-simulator-qa.yml", ".github/workflows/openminis-skill.yml",
        "tools/shortcuts-simulator-qa/validate_macro_fixtures.py",
        "tools/shortcuts-simulator-qa/XcodeProject/UITests/CherriSimulatorQAUITests.swift",
        "docs/apple-runtime/p10r15/native-continuation-probes.md", *[c["path"] for c in cases]]
    bound = {}
    for relative in paths:
        data = (ROOT/relative).read_bytes()
        require(data == subprocess.check_output(["git","show",sha+":"+relative], cwd=ROOT), "HEAD byte drift")
        bound[relative] = hashlib.sha256(data).hexdigest()
    results = []
    for case in cases:
        fid = case["id"]
        require(bound[case["path"]] == case["sourceSha256"], "fixture source manifest drift")
        compiled = output.parent/(fid+".shortcut")
        imported = output.parent/(fid+"-import.cherri")
        rebuilt = output.parent/(fid+"-rebuilt.shortcut")
        checked([compiler,"compile",ROOT/case["path"],"-o",compiled,"--skip-sign","--no-ansi"])
        actions = plistlib.loads(compiled.read_bytes())["WFWorkflowActions"]
        observed = validate_graph(fid, actions)
        faults = causal_faults(fid, actions)
        checked([compiler,"import",compiled,"--mode=v2","--lossless","-o",imported])
        checked([compiler,"compile",imported,"-o",rebuilt,"--skip-sign","--no-ansi"])
        require(plistlib.loads(rebuilt.read_bytes())["WFWorkflowActions"] == actions, "exact native action roundtrip drift")
        results.append({"fixtureId":fid,"sourceSha256":case["sourceSha256"],
            "unsignedPlistSha256":hashlib.sha256(compiled.read_bytes()).hexdigest(),
            "graph":observed,"rejectedGraphFaults":faults,"exactNativeActionRoundtrip":True})
    env={**os.environ,"PYTHONPATH":str(ROOT/"tools/shortcuts-simulator-qa")}
    tested=subprocess.run([sys.executable,"-X","utf8","-m","unittest","-v","test_storage_cross_run"],
        cwd=ROOT,env=env,capture_output=True,text=True,encoding="utf-8",timeout=120)
    transcript=tested.stdout+tested.stderr
    (output.parent/"continuation-python.log").write_bytes(transcript.encode("utf-8"))
    require(tested.returncode==0,"continuation policy/oracle tests failed")
    match=re.search(r"Ran (\d+) tests in ([0-9.]+)s",transcript)
    require(match and re.search(r"\nOK\s*$",transcript),"test completion receipt missing")
    require(not checked(["git","status","--porcelain"]).stdout.strip(),"source changed during execution")
    report={"schemaVersion":1,"status":"PASS_CLEAN_SOURCE_AND_CLI_ONLY","sourceCommit":sha,"sourceTreeClean":True,
        "compilerBinarySha256":hashlib.sha256(compiler.read_bytes()).hexdigest(),"compilerBuildIdentity":identity,
        "sourceFileSha256":bound,"actualCLIFixtures":results,"pythonTests":int(match.group(1)),
        "pythonTranscriptSha256":hashlib.sha256(transcript.encode()).hexdigest(),
        "nativeExecuted":0,"cacheImplemented":False,"productionCompilerChanged":False,
        "scope":"Two-invocation orchestration, native graph identity and source roundtrip only; Apple outcomes require fresh native callbacks.",
        "elapsedSeconds":round(time.monotonic()-started,3)}
    output.write_text(json.dumps(report,indent=2,ensure_ascii=True)+"\n",encoding="utf-8")
    print(json.dumps({"status":report["status"],"sourceCommit":sha,"actualCLIFixtures":len(results),
        "exactNativeActionRoundtrips":len(results),"rejectedGraphFaults":sum(len(r["rejectedGraphFaults"])for r in results),
        "pythonTests":report["pythonTests"],"nativeExecuted":0}))


if __name__ == "__main__":
    main()
