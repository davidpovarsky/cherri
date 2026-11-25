"""Public causal observations of export rows and Clipboard boundaries only."""
import copy
import hashlib
from pathlib import Path

from native_comparison_graph import verify_equality_region

FIXTURE_ID = "qa-native-boundary-diagnostic-v2"
SAMPLE_ID = "qa-native-boundary-diagnostic"
SOURCE_SHA256 = '30f7ea3431a917b4b8f73b200ef113fd833cd0e522f9932b1a714e53bfd4e8dd'
PUBLIC_LITERAL = 'quote " backslash \\ newline\nHebrew שָׁלוֹם cherry 🍒 token SYNTHETIC_SECRET'
BINDING_BASE64 = 'Q0hFUlJJX0JJTkRJTkdfQ0FQVFVSRV9PSwpOVU1fQ0FQVFVSRUQ9MQpOVU1fQUdBSU49MQpDTElQX0NBUFRVUkVEPWNsaXBfaW5pdGlhbApDTElQX0FHQUlOPWNsaXBfaW5pdGlhbA=='
EXPECTED = 'CHERRI_QA_NATIVE_BOUNDARY_DIAG:ENTRIES=1:FIRST64=Y1hWdmRHVWdJaUJpWVdOcmMyeGhjMmdnWENCdVpYZHNhVzVsQ2tobFluSmxkeURYcWRhNDE0SFhuTmVWMXJuWG5TQmphR1Z5Y25rZzhKK05raUIwYjJ0bGJpQlRXVTVVU0VWVVNVTmZVMFZEVWtWVQ==:LAST64=Y1hWdmRHVWdJaUJpWVdOcmMyeGhjMmdnWENCdVpYZHNhVzVsQ2tobFluSmxkeURYcWRhNDE0SFhuTmVWMXJuWG5TQmphR1Z5Y25rZzhKK05raUIwYjJ0bGJpQlRXVTVVU0VWVVNVTmZVMFZEVWtWVQ==:TYPE_FILE=Y:TYPE_TEXT=N:TYPE_DICT=N:FILE_TEXT=N:FILE_DICT=N:COERCED_FILE=Y:COERCED_BYTES=Y:COERCED_NAME=Y:NUM_ONE=Y:CLIP_INITIAL=Y:ALIAS_INITIAL=Y:DIRECT_LEN=140:DIRECT_EQ=Y:GETVAR_LEN=140:GETVAR_EQ=Y:TOKEN_LEN=140:TOKEN_EQ=Y:ACTION_LEN=140:ACTION_EQ=Y'
FIELDS = [['ENTRIES', 'boundaryEntryCount', '1'], ['FIRST64', 'boundaryFirst64', 'Y1hWdmRHVWdJaUJpWVdOcmMyeGhjMmdnWENCdVpYZHNhVzVsQ2tobFluSmxkeURYcWRhNDE0SFhuTmVWMXJuWG5TQmphR1Z5Y25rZzhKK05raUIwYjJ0bGJpQlRXVTVVU0VWVVNVTmZVMFZEVWtWVQ=='], ['LAST64', 'boundaryLast64', 'Y1hWdmRHVWdJaUJpWVdOcmMyeGhjMmdnWENCdVpYZHNhVzVsQ2tobFluSmxkeURYcWRhNDE0SFhuTmVWMXJuWG5TQmphR1Z5Y25rZzhKK05raUIwYjJ0bGJpQlRXVTVVU0VWVVNVTmZVMFZEVWtWVQ=='], ['TYPE_FILE', 'boundaryArtifactIsFile', 'Y'], ['TYPE_TEXT', 'boundaryArtifactIsText', 'N'], ['TYPE_DICT', 'boundaryArtifactIsDictionary', 'N'], ['FILE_TEXT', 'boundaryFileIsText', 'N'], ['FILE_DICT', 'boundaryFileIsDictionary', 'N'], ['COERCED_FILE', 'boundaryCoercedIsFile', 'Y'], ['COERCED_BYTES', 'boundaryCoercedBytesEqual', 'Y'], ['COERCED_NAME', 'boundaryCoercedNameCorrect', 'Y'], ['NUM_ONE', 'boundaryNumIsOne', 'Y'], ['CLIP_INITIAL', 'boundaryClipInitial', 'Y'], ['ALIAS_INITIAL', 'boundaryAliasInitial', 'Y'], ['DIRECT_LEN', 'boundaryDirectLength', '140'], ['DIRECT_EQ', 'boundaryDirectMatches', 'Y'], ['GETVAR_LEN', 'boundaryGetVarLength', '140'], ['GETVAR_EQ', 'boundaryGetVarMatches', 'Y'], ['TOKEN_LEN', 'boundaryTokenLength', '140'], ['TOKEN_EQ', 'boundaryTokenMatches', 'Y'], ['ACTION_LEN', 'boundaryActionLength', '140'], ['ACTION_EQ', 'boundaryActionMatches', 'Y']]
TYPE_FLAGS = [['TYPE_FILE', 'boundaryArtifactIsFile', 'boundaryArtifactType', 'boundaryBinaryType', 'Y'], ['TYPE_TEXT', 'boundaryArtifactIsText', 'boundaryArtifactType', 'boundaryTextType', 'N'], ['TYPE_DICT', 'boundaryArtifactIsDictionary', 'boundaryArtifactType', 'boundaryDictionaryType', 'N'], ['FILE_TEXT', 'boundaryFileIsText', 'boundaryBinaryType', 'boundaryTextType', 'N'], ['FILE_DICT', 'boundaryFileIsDictionary', 'boundaryBinaryType', 'boundaryDictionaryType', 'N']]
ORIGINALS = {
    "tools/shortcuts-simulator-qa/fixtures/qa-export-unicode-diagnostic-v2.cherri":
        "4bbf88356f04bbf7ad2f07c1cc30874e611a0bcd7314a38c82a56b1ba5eeb3aa",
    "tests/runtime_poc/CherriBindingCapturePOC.cherri":
        "4c649f8d53ed164b73a0324b7941999e5c9e849d28670655fafc13c87cee5558",
}


def verify_source(root, fid, source):
    if fid != FIXTURE_ID:
        return None
    if hashlib.sha256(source).hexdigest() != SOURCE_SHA256:
        raise ValueError("NATIVE_BOUNDARY_SOURCE_CONTRACT")
    for name, digest in ORIGINALS.items():
        original = (Path(root) / name).read_bytes()
        if hashlib.sha256(original).hexdigest() != digest:
            raise ValueError("NATIVE_BOUNDARY_ORIGINAL_CHANGED")
        if name.endswith("CherriBindingCapturePOC.cherri"):
            if source.count(original) != 1:
                raise ValueError("NATIVE_BOUNDARY_ORIGINAL_BODY_NOT_PRESERVED")
    return {"fixtureSha256": SOURCE_SHA256, "originalFixturesUnchanged": True,
            "bindingBodyByteIdentical": True, "formalAcceptance": False, "appleRuntime": "NOT_RUN"}


def verify_graph(fid, actions):
    if fid != FIXTURE_ID:
        return None

    def require(ok, why):
        if not ok:
            raise ValueError("NATIVE_BOUNDARY_GRAPH_CONTRACT: " + why)

    def p(action):
        return action.get("WFWorkflowActionParameters", {})

    def kind(action, suffix):
        return action.get("WFWorkflowActionIdentifier") == "is.workflow.actions." + suffix

    uuids = [p(action)["UUID"] for action in actions if "UUID" in p(action)]
    require(all(isinstance(value, str) and value.strip() for value in uuids)
            and len(uuids) == len(set(uuids)), "nonempty unique producer UUIDs")
    by_uuid = {p(action)["UUID"]: (i, action) for i, action in enumerate(actions) if "UUID" in p(action)}

    def named(name, suffix=None):
        rows = [(i, action) for i, action in enumerate(actions) if p(action).get("CustomOutputName") == name]
        require(len(rows) == 1 and isinstance(p(rows[0][1]).get("UUID"), str)
                and (suffix is None or kind(rows[0][1], suffix)), "named producer " + name)
        return rows[0]

    def att(value):
        require(isinstance(value, dict) and value.get("WFSerializationType") == "WFTextTokenAttachment"
                and isinstance(value.get("Value"), dict), "attachment")
        return value["Value"]

    def edge(value, source, numeric=False, file=False):
        token = att(value)
        expected_agg = ([{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass":
                          "WFNumberContentItem" if numeric else "WFGenericFileContentItem"}]
                        if numeric or file else None)
        require(bool(p(source).get("UUID")) and token.get("Type") == "ActionOutput"
                and token.get("OutputUUID") == p(source)["UUID"]
                and (token.get("Aggrandizements") == expected_agg if numeric or file else not token.get("Aggrandizements")),
                "exact source edge")

    def parent(value, suffix, before):
        token = att(value)
        require(token.get("Type") == "ActionOutput" and not token.get("Aggrandizements")
                and token.get("OutputUUID") in by_uuid, "known uncoerced parent")
        i, action = by_uuid[token["OutputUUID"]]
        require(i < before and kind(action, suffix), "preceding " + suffix + " parent")
        return i, action

    def string_edges(value, template, references):
        require(isinstance(value, dict) and value.get("WFSerializationType") == "WFTextTokenString",
                "dynamic string serialization")
        value = value.get("Value", {})
        require(value.get("string") == template, "exact observed string template")
        attachments = value.get("attachmentsByRange", {})
        positions = [i for i, ch in enumerate(template) if ch == "\ufffc"]
        require(len(attachments) == len(positions) == len(references), "one observation per attachment")
        for position, ref in zip(positions, references):
            token = attachments.get("{%d, 1}" % position, {})
            if isinstance(ref, str):
                require(token == {"Type": "Variable", "VariableName": ref}, "observed variable " + ref)
            else:
                require(token.get("Type") == "ActionOutput" and token.get("OutputUUID") == p(ref).get("UUID")
                        and bool(p(ref).get("UUID")) and not token.get("Aggrandizements"), "observed producer")

    oi, original = named("boundaryOriginal", "gettext")
    require(p(original).get("WFTextActionText") == PUBLIC_LITERAL, "exact public Unicode source")
    seeds = [(i, action) for i, action in enumerate(actions) if kind(action, "setvariable")
             and p(action).get("WFVariableName") == "__cherri_debug_log"]
    require(len(seeds) == 1 and seeds[0][0] < oi and seeds[0][0] > 0, "one original debug clear")
    si, seed = seeds[0]
    require(kind(actions[si - 1], "nothing") and "WFInput" not in p(actions[si - 1])
            and "WFInput" not in p(seed), "native zero-item debug clear")
    logs = [(i, action) for i, action in enumerate(actions) if kind(action, "appendvariable")
            and p(action).get("WFVariableName") == "__cherri_debug_log"]
    require(len(logs) == 1 and logs[0][0] == oi + 1, "one real logged input")
    edge(p(logs[0][1]).get("WFInput"), original)
    ai, artifact = named("boundaryArtifact", "setitemname")
    require(p(artifact).get("WFName") == "cherri-debug.json" and p(artifact).get("WFDontIncludeFileExtension") is False,
            "unchanged production export name")
    di, decoded = parent(p(artifact).get("WFInput"), "base64encode", ai)
    require(p(decoded).get("WFEncodeMode") == "Decode", "production outer Decode")
    ei, encoded = parent(p(decoded).get("WFInput"), "base64encode", di)
    require(p(encoded).get("WFEncodeMode") == "Encode" and p(encoded).get("WFBase64LineBreakMode") == "None",
            "production outer Encode")
    parent(p(encoded).get("WFInput"), "gettext", ei)

    values = {"boundaryArtifact": artifact}
    for name, suffix, key, upstream in (
            ("boundaryArtifactText", "detect.text", "WFInput", "boundaryArtifact"),
            ("boundaryParsed", "detect.dictionary", "WFInput", "boundaryArtifactText"),
            ("boundaryEntries", "getvalueforkey", "WFInput", "boundaryParsed"),
            ("boundaryEntryCount", "count", "Input", "boundaryEntries"),
            ("boundaryFirst", "getitemfromlist", "WFInput", "boundaryEntries"),
            ("boundaryLast", "getitemfromlist", "WFInput", "boundaryEntries"),
            ("boundaryFirstPayload", "getvalueforkey", "WFInput", "boundaryFirst"),
            ("boundaryLastPayload", "getvalueforkey", "WFInput", "boundaryLast"),
            ("boundaryFirstValue", "getvalueforkey", "WFInput", "boundaryFirstPayload"),
            ("boundaryLastValue", "getvalueforkey", "WFInput", "boundaryLastPayload")):
        _, action = named(name, suffix)
        edge(p(action).get(key), values[upstream])
        values[name] = action
    for name, key in (("boundaryEntries", "entries"), ("boundaryFirstPayload", "payload"),
                      ("boundaryLastPayload", "payload"), ("boundaryFirstValue", "value"),
                      ("boundaryLastValue", "value")):
        require(p(values[name]).get("WFDictionaryKey") == key, "actual export dictionary key")
    require(p(values["boundaryEntryCount"]).get("WFCountType", "Items") == "Items", "actual entry cardinality")
    require(p(values["boundaryFirst"]).get("WFItemSpecifier") == "First Item"
            and p(values["boundaryLast"]).get("WFItemSpecifier") == "Last Item", "independent first and last rows")

    bi, binary = named("boundaryBinary", "base64encode")
    require(p(binary).get("WFEncodeMode") == "Decode", "independent binary control")
    _, binary_input = parent(p(binary).get("WFInput"), "gettext", bi)
    require(p(binary_input).get("WFTextActionText") == "AQ==", "one-byte binary control")
    _, text = named("boundaryText", "gettext")
    require(p(text).get("WFTextActionText") == "NATIVE_BOUNDARY_TEXT_WITNESS", "independent Text control")
    _, dictionary = named("boundaryDictionary", "dictionary")
    expected_fields = [{"WFItemType": 0,
                        "WFKey": {"WFSerializationType": "WFTextTokenString", "Value": {"string": "probe"}},
                        "WFValue": {"WFSerializationType": "WFTextTokenString",
                                    "Value": {"string": "NATIVE_BOUNDARY_DICTIONARY_WITNESS"}}}]
    require(p(dictionary).get("WFItems") == {"WFSerializationType": "WFDictionaryFieldValue",
                                           "Value": {"WFDictionaryFieldValueItems": expected_fields}},
            "independent Dictionary control")
    for name, source in (("boundaryArtifactType", artifact), ("boundaryBinaryType", binary),
                         ("boundaryTextType", text), ("boundaryDictionaryType", dictionary)):
        _, action = named(name, "getitemtype")
        edge(p(action).get("WFInput"), source)
        values[name] = action

    def assignments(variable):
        rows = [(i, action) for i, action in enumerate(actions)
                if kind(action, "setvariable") and p(action).get("WFVariableName") == variable]
        require(len(rows) == 2, "exact conditional assignment pair " + variable)
        return rows

    def flag(variable, source, rhs=None, rhs_source=None, numeric=False, dynamic=False):
        (first, init), (second, changed) = assignments(variable)
        require(first > 0 and second >= first + 3 and second + 1 < len(actions),
                "single comparison controls flag " + variable)
        for i, action, literal in ((first, init, "N"), (second, changed, "Y")):
            require(kind(actions[i - 1], "gettext") and p(actions[i - 1]).get("WFTextActionText") == literal,
                    "fail-closed flag literals")
            edge(p(action).get("WFInput"), actions[i - 1])
        condition_index = second - 2
        condition, end = actions[condition_index], actions[second + 1]
        require(kind(condition, "conditional") and p(condition).get("WFControlFlowMode") == 0
                and kind(end, "conditional") and p(end).get("WFControlFlowMode") == 2
                and bool(p(condition).get("GroupingIdentifier"))
                and p(condition)["GroupingIdentifier"] == p(end).get("GroupingIdentifier"),
                "exact equal comparison region")
        require([i for i, action in enumerate(actions)
                 if p(action).get("GroupingIdentifier") == p(condition)["GroupingIdentifier"]]
                == [condition_index, second + 1], "flag has exactly one guarded arm")
        verify_equality_region(actions, first + 1, condition_index, source, require=require,
                               right=rhs, right_source=rhs_source, numeric=numeric, dynamic=dynamic)
    for _, variable, left, right, _ in TYPE_FLAGS:
        flag(variable, values[left], rhs_source=values[right])

    for side in ("First", "Last"):
        ri, raw = named("boundary" + side + "64Raw", "base64encode")
        edge(p(raw).get("WFInput"), values["boundary" + side + "Value"])
        require(p(raw).get("WFEncodeMode") == "Encode" and p(raw).get("WFBase64LineBreakMode") == "None",
                "bounded actual row bytes")
        li, length = named("boundary" + side + "64Length", "count")
        edge(p(length).get("Input"), raw)
        require(p(length).get("WFCountType") == "Characters" and li > ri, "actual byte witness length")
        (first, init), (second, changed) = assignments("boundary" + side + "64")
        require(first == li + 2 and second == first + 2 and kind(actions[first - 1], "gettext")
                and p(actions[first - 1]).get("WFTextActionText") == "OVERSIZE", "fail-closed 512 cap")
        edge(p(init).get("WFInput"), actions[first - 1]); edge(p(changed).get("WFInput"), raw)
        condition, end = actions[first + 1], actions[second + 1]
        require(kind(condition, "conditional") and p(condition).get("WFControlFlowMode") == 0
                and p(condition).get("WFCondition") == 0 and p(condition).get("WFNumberValue") == 513
                and kind(end, "conditional") and p(end).get("WFControlFlowMode") == 2
                and bool(p(condition).get("GroupingIdentifier"))
                and p(condition)["GroupingIdentifier"] == p(end).get("GroupingIdentifier"), "bounded numeric branch")
        edge(p(condition).get("WFInput", {}).get("Variable"), length, numeric=True)

    _, coerced_type = named("boundaryCoercedType", "getitemtype")
    edge(p(coerced_type).get("WFInput"), artifact, file=True)
    _, original_bytes = named("boundaryOriginalFile64", "base64encode")
    _, coerced_bytes = named("boundaryCoercedFile64", "base64encode")
    for action, coerced in ((original_bytes, False), (coerced_bytes, True)):
        edge(p(action).get("WFInput"), artifact, file=coerced)
        require(p(action).get("WFEncodeMode") == "Encode" and p(action).get("WFBase64LineBreakMode") == "None",
                "independent original/coerced byte route")
    _, coerced_stem = named("boundaryCoercedStem", "getitemname")
    _, coerced_extension = named("boundaryCoercedExtension", "properties.files")
    for action in (coerced_stem, coerced_extension):
        edge(p(action).get("WFInput"), artifact, file=True)
    require(p(coerced_extension).get("WFContentItemPropertyName") == "File Extension",
            "actual coerced extension")
    _, coerced_name = named("boundaryCoercedName", "gettext")
    string_edges(p(coerced_name).get("WFTextActionText"), "\ufffc.\ufffc",
                 [coerced_stem, coerced_extension])
    flag("boundaryCoercedIsFile", coerced_type, rhs_source=values["boundaryBinaryType"])
    flag("boundaryCoercedBytesEqual", coerced_bytes, rhs_source=original_bytes)
    flag("boundaryCoercedNameCorrect", coerced_name, rhs="cherri-debug.json")

    result_i, result = named("result", "gettext")
    _, number = named("numCaptured", "getvariable")
    _, clip = named("clipAlias", "getclipboard")
    require(att(p(number).get("WFVariable")) == {"Type": "Variable", "VariableName": "numCaptureSrc"}
            and set(p(clip)) == {"UUID", "CustomOutputName"}, "declaration-time original captures")
    string_edges(p(result).get("WFTextActionText"),
                 "CHERRI_BINDING_CAPTURE_OK\nNUM_CAPTURED=\ufffc\nNUM_AGAIN=\ufffc\nCLIP_CAPTURED=\ufffc\nCLIP_AGAIN=\ufffc",
                 [number, number, clip, clip])
    setters = [(i, action) for i, action in enumerate(actions) if kind(action, "setclipboard")]
    require(len(setters) == 3 and setters[-1][0] == result_i + 1, "original final Set Clipboard")
    edge(p(setters[-1][1]).get("WFInput"), result)
    gi, getvariable = named("boundaryGetVariable", "getvariable")
    require(att(p(getvariable).get("WFVariable")) == {"Type": "Clipboard"} and gi > result_i + 1,
            "separate Get Variable Clipboard observer")
    ci, clipboard_action = named("boundaryClipboardAction", "getclipboard")
    require(set(p(clipboard_action)) == {"UUID", "CustomOutputName"} and ci > gi,
            "canonical Get Clipboard action observer")
    for name, source in (("Direct", result), ("GetVar", getvariable), ("Token", None), ("Action", clipboard_action)):
        i, raw = named("boundary" + name + "64", "base64encode")
        require(i > result_i + 1 and p(raw).get("WFEncodeMode") == "Encode"
                and p(raw).get("WFBase64LineBreakMode") == "None", "independent byte route " + name)
        if source is None:
            require(att(p(raw).get("WFInput")) == {"Type": "Clipboard"}, "direct Clipboard token route")
        else:
            edge(p(raw).get("WFInput"), source)
        _, length = named("boundary" + name + "Length", "count")
        edge(p(length).get("Input"), raw)
        require(p(length).get("WFCountType") == "Characters", "actual Clipboard route length")
        values["boundary" + name + "Length"] = length
        flag("boundary" + name + "Matches", raw, rhs=BINDING_BASE64)
    flag("boundaryNumIsOne", number, rhs=1, numeric=True)
    flag("boundaryClipInitial", clip, rhs="clip_initial", dynamic=True)
    flag("boundaryAliasInitial", clip, rhs="clip_initial", dynamic=True)

    _, trace = named("boundaryTrace", "gettext")
    _, summary = named("boundarySummary", "gettext")
    template = "CHERRI_QA_NATIVE_BOUNDARY_DIAG"
    refs = []
    for label, variable, _ in FIELDS:
        template += ":" + label + "=\ufffc"
        refs.append(values.get(variable, variable))
    string_edges(p(summary).get("WFTextActionText"), template + "\n\ufffc", refs + [trace])
    require(kind(actions[-1], "output") and kind(actions[-2], "comment")
            and p(actions[-2]).get("WFCommentActionText") == "ANCHOR_CARD_28_BOTTOM_FOOTER", "actual final callback")
    string_edges(p(actions[-1]).get("WFOutput"), "\ufffc", [summary])
    return {"status": "PASS_LOCAL_CAUSAL_GRAPH_ONLY", "nativeActions": len(actions),
            "independentClipboardRoutes": 4, "boundedPublicExportByteWitnesses": 2,
            "clipboardContentPublished": False, "independentTypeWitnesses": 3,
            "supportedFileCoercionObservedSeparately": True,
            "formalAcceptance": False, "appleRuntime": "NOT_RUN"}


def verify_causal_mutations(actions):
    def named(graph, name):
        return next(a["WFWorkflowActionParameters"] for a in graph
                    if a.get("WFWorkflowActionParameters", {}).get("CustomOutputName") == name)
    mutations = (
        "wrong-entry-count", "first-as-last", "wrong-payload-key", "wrong-dictionary-control",
        "wrong-binary-control", "type-witness-alias", "getvariable-not-clipboard",
        "token-route-collapsed", "canonical-action-collapsed", "direct-result-replaced",
        "hardcoded-result", "flag-always-yes", "cap-bypassed", "missing-uuid",
        "duplicate-uuid", "callback-replaced", "seed-changed", "unlogged-public-value",
        "file-coercion-removed", "file-coercion-wrong-class", "file-byte-route-collapsed", "file-name-hardcoded")
    rejected = []
    for mutation in mutations:
        graph = copy.deepcopy(actions)
        if mutation == "wrong-entry-count":
            named(graph, "boundaryEntryCount")["Input"]["Value"]["OutputUUID"] = named(graph, "boundaryArtifact")["UUID"]
        elif mutation == "first-as-last":
            named(graph, "boundaryLast")["WFItemSpecifier"] = "First Item"
        elif mutation == "wrong-payload-key":
            named(graph, "boundaryLastValue")["WFDictionaryKey"] = "missing"
        elif mutation == "wrong-dictionary-control":
            named(graph, "boundaryDictionary")["WFItems"]["Value"]["WFDictionaryFieldValueItems"] = []
        elif mutation == "wrong-binary-control":
            source = named(graph, "boundaryBinary")["WFInput"]["Value"]["OutputUUID"]
            next(a["WFWorkflowActionParameters"] for a in graph if a.get("WFWorkflowActionParameters", {}).get("UUID") == source)["WFTextActionText"] = "eA=="
        elif mutation == "type-witness-alias":
            named(graph, "boundaryBinaryType")["WFInput"]["Value"]["OutputUUID"] = named(graph, "boundaryArtifact")["UUID"]
        elif mutation == "getvariable-not-clipboard":
            named(graph, "boundaryGetVariable")["WFVariable"]["Value"] = {"Type": "Variable", "VariableName": "result"}
        elif mutation == "token-route-collapsed":
            named(graph, "boundaryToken64")["WFInput"] = copy.deepcopy(named(graph, "boundaryGetVar64")["WFInput"])
        elif mutation == "canonical-action-collapsed":
            next(a for a in graph if a.get("WFWorkflowActionParameters", {}).get("CustomOutputName") == "boundaryClipboardAction")["WFWorkflowActionIdentifier"] = "is.workflow.actions.getvariable"
        elif mutation == "direct-result-replaced":
            named(graph, "boundaryDirect64")["WFInput"] = copy.deepcopy(named(graph, "boundaryGetVar64")["WFInput"])
        elif mutation == "hardcoded-result":
            named(graph, "boundarySummary")["WFTextActionText"] = EXPECTED
        elif mutation == "flag-always-yes":
            action = next(a for a in graph if a.get("WFWorkflowActionParameters", {}).get("WFTextActionText") == "N")
            action["WFWorkflowActionParameters"]["WFTextActionText"] = "Y"
        elif mutation == "cap-bypassed":
            for action in graph:
                p = action.get("WFWorkflowActionParameters", {})
                if p.get("WFNumberValue") == 513:
                    p["WFNumberValue"] = 99999
                    break
        elif mutation == "missing-uuid":
            named(graph, "boundaryEntries").pop("UUID")
        elif mutation == "duplicate-uuid":
            named(graph, "boundaryEntries")["UUID"] = named(graph, "boundaryParsed")["UUID"]
        elif mutation == "callback-replaced":
            graph[-1]["WFWorkflowActionParameters"]["WFOutput"] = EXPECTED
        elif mutation == "seed-changed":
            clear = next(i for i, a in enumerate(graph)
                         if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable"
                         and a["WFWorkflowActionParameters"].get("WFVariableName") == "__cherri_debug_log")
            graph[clear - 1]["WFWorkflowActionIdentifier"] = "is.workflow.actions.gettext"
            graph[clear - 1]["WFWorkflowActionParameters"]["WFTextActionText"] = ""
        elif mutation == "file-coercion-removed":
            named(graph, "boundaryCoercedType")["WFInput"]["Value"].pop("Aggrandizements")
        elif mutation == "file-coercion-wrong-class":
            named(graph, "boundaryCoercedStem")["WFInput"]["Value"]["Aggrandizements"][0]["CoercionItemClass"] = "WFStringContentItem"
        elif mutation == "file-byte-route-collapsed":
            named(graph, "boundaryCoercedFile64")["WFInput"] = copy.deepcopy(named(graph, "boundaryOriginalFile64")["WFInput"])
        elif mutation == "file-name-hardcoded":
            named(graph, "boundaryCoercedName")["WFTextActionText"] = "cherri-debug.json"
        elif mutation == "unlogged-public-value":
            next(a["WFWorkflowActionParameters"] for a in graph
                 if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.appendvariable"
                 and a["WFWorkflowActionParameters"].get("WFVariableName") == "__cherri_debug_log")["WFInput"] = "hardcoded"
        try:
            verify_graph(FIXTURE_ID, graph)
        except ValueError as error:
            if "NATIVE_BOUNDARY_GRAPH_CONTRACT" not in str(error):
                raise
            rejected.append(mutation)
        else:
            raise AssertionError("damaged causal graph accepted: " + mutation)
    return rejected + verify_exact_comparison_mutations(actions)


def verify_exact_comparison_mutations(actions):
    """Exercise each causal dependency on the real compiler-emitted graph."""
    cases = ("exact-left-source", "exact-right-source", "exact-prefix",
             "exact-escape-find", "exact-escape-replacement", "exact-escape-regex",
             "exact-escape-case", "exact-match-case", "exact-anchors",
             "exact-match-source", "exact-quote-bypass", "exact-count-unit",
             "exact-count-source", "exact-predicate", "exact-cardinality",
             "exact-true-arm", "exact-false-arm", "exact-result-bypass",
             "exact-outer-predicate", "exact-unreviewed-action", "exact-native-downgrade")
    rejected = []
    for case in cases:
        graph = copy.deepcopy(actions)
        def p(action):
            return action.get("WFWorkflowActionParameters", {})
        def named(name):
            return next(p(action) for action in graph if p(action).get("CustomOutputName") == name)
        def single_ref(value):
            return next(iter(value["Value"]["attachmentsByRange"].values()))
        first, second = [i for i, action in enumerate(graph)
                         if p(action).get("WFVariableName") == "boundaryArtifactIsFile"]
        left = named("_n03_exact_left")
        right = named("_n03_exact_right")
        quote = named("_n03_exact_quote")
        match = named("_n03_exact_match")
        count = named("_n03_exact_count")
        outer = p(graph[second - 2])
        test_i = next(i for i, action in enumerate(graph)
                      if p(action).get("WFInput", {}).get("Variable", {}).get("Value", {}).get("OutputUUID")
                      == count["UUID"])
        test = p(graph[test_i])
        if case == "exact-left-source":
            single_ref(left["WFTextActionText"])["OutputUUID"] = named("boundaryTextType")["UUID"]
        elif case == "exact-right-source":
            single_ref(right["WFTextActionText"])["OutputUUID"] = named("boundaryArtifactType")["UUID"]
        elif case == "exact-prefix":
            left["WFTextActionText"]["Value"]["string"] = "!\ufffc"
        elif case == "exact-escape-find":
            quote["WFReplaceTextFind"] = "E"
        elif case == "exact-escape-replacement":
            quote["WFReplaceTextReplace"] = ""
        elif case == "exact-escape-regex":
            quote["WFReplaceTextRegularExpression"] = True
        elif case == "exact-escape-case":
            quote["WFReplaceTextCaseSensitive"] = False
        elif case == "exact-match-case":
            match["WFMatchTextCaseSensitive"] = False
        elif case == "exact-anchors":
            match["WFMatchTextPattern"]["Value"]["string"] = "\\Q\ufffc\\E"
        elif case == "exact-match-source":
            single_ref(match["text"])["OutputUUID"] = right["UUID"]
        elif case == "exact-quote-bypass":
            single_ref(match["WFMatchTextPattern"])["OutputUUID"] = right["UUID"]
        elif case == "exact-count-unit":
            count["WFCountType"] = "Characters"
        elif case == "exact-count-source":
            count["Input"]["Value"]["OutputUUID"] = left["UUID"]
        elif case == "exact-predicate":
            test["WFCondition"] = 5
        elif case == "exact-cardinality":
            test["WFNumberValue"] = 2
        elif case == "exact-true-arm":
            p(graph[test_i + 1])["WFNumberActionNumber"] = 0
        elif case == "exact-false-arm":
            p(graph[test_i + 3])["WFNumberActionNumber"] = 1
        elif case == "exact-result-bypass":
            outer["WFInput"]["Variable"]["Value"]["OutputUUID"] = count["UUID"]
        elif case == "exact-outer-predicate":
            outer["WFCondition"] = 5
        elif case == "exact-unreviewed-action":
            graph.insert(first + 1, {"WFWorkflowActionIdentifier": "is.workflow.actions.nothing",
                                     "WFWorkflowActionParameters": {}})
        elif case == "exact-native-downgrade":
            outer.pop("WFNumberValue")
            outer["WFConditionalActionString"] = {
                "WFSerializationType": "WFTextTokenString",
                "Value": {"string": "\ufffc", "attachmentsByRange": {
                    "{0, 1}": {"Type": "ActionOutput", "OutputUUID": named("boundaryBinaryType")["UUID"]}}}}
            outer["WFInput"]["Variable"]["Value"] = {
                "Type": "ActionOutput", "OutputUUID": named("boundaryArtifactType")["UUID"]}
            del graph[first + 1:second - 2]
        try:
            verify_graph(FIXTURE_ID, graph)
        except ValueError as error:
            if "NATIVE_BOUNDARY_GRAPH_CONTRACT" not in str(error):
                raise
            rejected.append(case)
        else:
            raise AssertionError("damaged exact comparison accepted: " + case)
    return rejected
