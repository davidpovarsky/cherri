"""Inspect the actual public Unicode diagnostic graph before signing.

This proves probe wiring and bounded output, never Apple byte preservation.
The independent native result remains the exact callback oracle.
"""
import base64

from native_comparison_graph import verify_equality_region

FIXTURE_ID = "qa-export-unicode-diagnostic-v2"
ORIGINAL = 'quote " backslash \\ newline\nHebrew שָׁלוֹם cherry 🍒 token SYNTHETIC_SECRET'
ORIGINAL_BASE64 = "cXVvdGUgIiBiYWNrc2xhc2ggXCBuZXdsaW5lCkhlYnJldyDXqda414HXnNeV1rnXnSBjaGVycnkg8J+NkiB0b2tlbiBTWU5USEVUSUNfU0VDUkVU"
FLAGS = (
    ("TYPE", "typeFlag", "artifactType", 4, "ref", "expectedFileType"),
    ("NAME", "nameFlag", "artifactName", 4, "text", "cherri-debug.json"),
    ("COUNT", "countFlag", "entryCount", 4, "number", 1),
    ("STATUS", "statusFlag", "status", 4, "text", "TEXT_PAYLOAD_ENCODED"),
    ("PAYLOAD", "payloadFlag", "payloadValue", 4, "text", ORIGINAL_BASE64),
    ("RESTORED_EQ", "restoredEq", "restoredText", 4, "ref", "original"),
    ("RESTORED_NE", "restoredNe", "restoredText", 5, "ref", "original"),
    ("LITERAL_EQ", "literalEq", "literalText", 4, "ref", "original"),
    ("LITERAL_NE", "literalNe", "literalText", 5, "ref", "original"),
    ("UNCHANGED", "unchangedFlag", "originalAfter", 4, "ref", "original"),
)
WITNESSES = (
    ("ORIGINAL64", "original64", "original"),
    ("PAYLOAD_TEXT64", "payloadText64", "payloadValue"),
    ("DECODED_FILE64", "decodedFile64", "restoredFile"),
    ("RESTORED64", "restored64", "restoredText"),
    ("LITERAL64", "literal64", "literalText"),
    ("AFTER64", "after64", "originalAfter"),
    ("NAME64", "name64", "artifactName"),
    ("STATUS64", "status64", "status"),
)


def require(ok, detail):
    if not ok:
        raise ValueError("EXPORT_UNICODE_DIAGNOSTIC_GRAPH_MISMATCH: " + detail)


def params(action):
    return action.get("WFWorkflowActionParameters", {})


def action_is(action, short_id):
    return action.get("WFWorkflowActionIdentifier") == "is.workflow.actions." + short_id


def attachment(value):
    require(isinstance(value, dict), "missing reference")
    if value.get("Type") == "Variable" and "Variable" in value:
        value = value["Variable"]
    require(value.get("WFSerializationType") == "WFTextTokenAttachment", "reference serialization")
    require(isinstance(value.get("Value"), dict), "reference value")
    return value["Value"]


def string_value(value):
    require(isinstance(value, dict) and value.get("WFSerializationType") == "WFTextTokenString",
            "dynamic text serialization")
    result = value.get("Value")
    require(isinstance(result, dict), "dynamic text value")
    return result


def validate_export_unicode_graph(fid, actions):
    if fid != FIXTURE_ID:
        return None
    require(isinstance(actions, list) and bool(actions), "no actions")
    by_uuid = {}
    by_name = {}
    for index, action in enumerate(actions):
        p = params(action)
        uid = p.get("UUID")
        if uid:
            require(uid not in by_uuid, "duplicate producer UUID")
            by_uuid[uid] = (index, action)
        name = p.get("CustomOutputName")
        if name:
            by_name.setdefault(name, []).append((index, action))

    def named(name, kind=None):
        found = by_name.get(name, [])
        require(len(found) == 1, "missing/ambiguous named producer " + name)
        i, a = found[0]
        if kind:
            require(action_is(a, kind), "producer kind " + name)
        require(params(a).get("UUID") in by_uuid, "producer UUID " + name)
        return i, a

    def ref(value, name, coercion=None):
        token = attachment(value)
        require(token.get("Type") == "ActionOutput"
                and token.get("OutputUUID") == params(named(name)[1])["UUID"],
                "wrong producer reference " + name)
        expected = [] if coercion is None else [{
            "CoercionItemClass": coercion, "Type": "WFCoercionVariableAggrandizement"}]
        require(token.get("Aggrandizements", []) == expected, "unexpected coercion " + name)

    def linked(name, kind, source, key="WFInput"):
        _, a = named(name, kind)
        ref(params(a).get(key), source)
        return params(a)

    def producer(value):
        token = attachment(value)
        require(token.get("Type") == "ActionOutput"
                and token.get("OutputUUID") in by_uuid
                and not token.get("Aggrandizements"), "unbound producer")
        return by_uuid[token["OutputUUID"]]

    def literal_input(value, text):
        _, a = producer(value)
        require(action_is(a, "gettext") and params(a).get("WFTextActionText") == text,
                "independent literal input")

    oi, original = named("original", "gettext")
    require(params(original).get("WFTextActionText") == ORIGINAL, "public UTF-8 literal changed")
    require(len(ORIGINAL.encode("utf-8")) == 84
            and base64.b64encode(ORIGINAL.encode("utf-8")).decode("ascii") == ORIGINAL_BASE64,
            "independent literal bytes")
    require(oi + 1 < len(actions) and action_is(actions[oi + 1], "appendvariable")
            and params(actions[oi + 1]).get("WFVariableName") == "__cherri_debug_log",
            "original logging edge")
    ref(params(actions[oi + 1]).get("WFInput"), "original")

    # Actual production envelope -> Encode(None) -> Decode(File) -> Set Name.
    ai, artifact = named("artifact", "setitemname")
    ap = params(artifact)
    require(ap.get("WFName") == "cherri-debug.json"
            and ap.get("WFDontIncludeFileExtension") is False, "production file naming")
    di, decode = producer(ap.get("WFInput"))
    ei, encode = producer(params(decode).get("WFInput"))
    ti, envelope = producer(params(encode).get("WFInput"))
    require((ti, ei, di) == (ai - 3, ai - 2, ai - 1)
            and action_is(envelope, "gettext")
            and action_is(encode, "base64encode") and params(encode).get("WFEncodeMode") == "Encode"
            and params(encode).get("WFBase64LineBreakMode") == "None"
            and action_is(decode, "base64encode") and params(decode).get("WFEncodeMode") == "Decode",
            "production envelope materialization")
    ev = string_value(params(envelope).get("WFTextActionText"))
    require(ev.get("string", "").startswith('{"schemaVersion":"cherri.debug-export/1"')
            and len(ev.get("attachmentsByRange", {})) == 1, "production envelope contents")

    linked("artifactStem", "getitemname", "artifact")
    require(linked("artifactExtension", "properties.files", "artifact").get("WFContentItemPropertyName")
            == "File Extension", "extension boundary")
    nv = string_value(params(named("artifactName", "gettext")[1]).get("WFTextActionText"))
    require(nv.get("string") == "\ufffc.\ufffc" and set(nv.get("attachmentsByRange", {})) == {"{0, 1}", "{2, 1}"},
            "independent name and extension")
    for pos, source in (("{0, 1}", "artifactStem"), ("{2, 1}", "artifactExtension")):
        ref({"WFSerializationType": "WFTextTokenAttachment", "Value": nv["attachmentsByRange"][pos]}, source)
    fp = params(named("fileWitness", "base64encode")[1])
    require(fp.get("WFEncodeMode") == "Decode", "File witness decode")
    literal_input(fp.get("WFInput"), "AQ==")
    linked("expectedFileType", "getitemtype", "fileWitness")
    linked("artifactType", "getitemtype", "artifact")
    linked("artifactText", "detect.text", "artifact")
    linked("parsed", "detect.dictionary", "artifactText")
    for name, source, key in (("entries", "parsed", "entries"), ("status", "firstEntry", "status"),
                              ("payload", "firstEntry", "payload"), ("payloadValue", "payload", "value")):
        require(linked(name, "getvalueforkey", source).get("WFDictionaryKey") == key, "dictionary key " + key)
    linked("entryCount", "count", "entries", "Input")
    require(linked("firstEntry", "getitemfromlist", "entries").get("WFItemSpecifier") == "First Item",
            "first exported entry")
    require(linked("restoredFile", "base64encode", "payloadValue").get("WFEncodeMode") == "Decode",
            "dynamic payload decode")
    linked("restoredText", "detect.text", "restoredFile")
    linked("originalAfter", "getvariable", "original", "WFVariable")
    lp = params(named("literalFile", "base64encode")[1])
    require(lp.get("WFEncodeMode") == "Decode", "independent literal decode")
    literal_input(lp.get("WFInput"), ORIGINAL_BASE64)
    linked("literalText", "detect.text", "literalFile")

    def writes(variable):
        return [(i, a) for i, a in enumerate(actions)
                if action_is(a, "setvariable") and params(a).get("WFVariableName") == variable]

    def closed_condition(start, end):
        require(action_is(actions[start], "conditional") and action_is(actions[end], "conditional"),
                "missing conditional boundaries")
        a, b = params(actions[start]), params(actions[end])
        require(a.get("WFControlFlowMode") == 0 and b.get("WFControlFlowMode") == 2
                and a.get("GroupingIdentifier") == b.get("GroupingIdentifier")
                and bool(a.get("GroupingIdentifier")), "unmatched conditional boundaries")
        require([i for i, action in enumerate(actions)
                 if params(action).get("GroupingIdentifier") == a["GroupingIdentifier"]] == [start, end],
                "conditional has exactly one guarded arm")
        return a

    for label, variable, source, condition, kind, expected in FLAGS:
        assignments = writes(variable)
        require(len(assignments) == 2, "independent flag assignments " + label)
        first, second = (i for i, _ in assignments)
        require(first > 0 and second >= first + 3 and second + 1 < len(actions),
                "independent flag control flow " + label)
        literal_input(params(actions[first]).get("WFInput"), "N")
        literal_input(params(actions[second]).get("WFInput"), "Y")
        closed_condition(second - 2, second + 1)
        coercion = "WFNumberContentItem" if kind == "number" else (
            "WFStringContentItem" if source in ("status", "payloadValue") else None)
        verify_equality_region(actions, first + 1, second - 2, named(source)[1],
                               require=require, right=expected if kind != "ref" else None,
                               right_source=named(expected)[1] if kind == "ref" else None,
                               numeric=kind == "number", coercion=coercion, operator=condition,
                               dynamic=source in ("status", "payloadValue", "originalAfter"))

    for label, variable, source in WITNESSES:
        rp = linked(variable + "Raw", "base64encode", source)
        require(rp.get("WFEncodeMode") == "Encode" and rp.get("WFBase64LineBreakMode") == "None",
                "byte witness encoding " + label)
        length = linked(variable + "Length", "count", variable + "Raw", "Input")
        require(length.get("WFCountType") == "Characters", "byte witness length unit " + label)
        assignments = writes(variable)
        require(len(assignments) == 2, "bounded witness assignments " + label)
        first, second = (i for i, _ in assignments)
        require(second == first + 2, "bounded witness control flow " + label)
        literal_input(params(actions[first]).get("WFInput"), "OVERSIZE")
        cp = closed_condition(first + 1, second + 1)
        require(cp.get("WFCondition") == 0 and cp.get("WFNumberValue") == 513,
                "512-character witness cap " + label)
        ref(cp.get("WFInput"), variable + "Length", "WFNumberContentItem")
        ref(params(actions[second]).get("WFInput"), variable + "Raw")

    # Every label must read its own flag or bounded witness, not an authored PASS.
    sv = string_value(params(named("finalSummary", "gettext")[1]).get("WFTextActionText"))
    summary = "CHERRI_QA_EXPORT_UNICODE_DIAG"
    expected_attachments = {}
    for label, variable in [(x[0], x[1]) for x in FLAGS + WITNESSES]:
        summary += ":" + label + "="
        expected_attachments["{%d, 1}" % len(summary)] = {"Type": "Variable", "VariableName": variable}
        summary += "\ufffc"
    summary += "\n"
    expected_attachments["{%d, 1}" % len(summary)] = {
        "Type": "ActionOutput", "OutputName": "traceEnvelope",
        "OutputUUID": params(named("traceEnvelope", "gettext")[1])["UUID"]}
    summary += "\ufffc"
    require(sv.get("string") == summary and sv.get("attachmentsByRange") == expected_attachments,
            "dynamic summary field mapping")
    require(action_is(actions[-1], "output"), "final native output")
    ov = string_value(params(actions[-1]).get("WFOutput"))
    require(ov.get("string") == "\ufffc" and set(ov.get("attachmentsByRange", {})) == {"{0, 1}"},
            "final output token")
    ref({"WFSerializationType": "WFTextTokenAttachment", "Value": ov["attachmentsByRange"]["{0, 1}"]},
        "finalSummary")
    return {"status": "PASS_LOCAL_GRAPH_ONLY", "independentFlags": len(FLAGS),
            "boundedByteWitnesses": len(WITNESSES), "maximumCharactersPerWitness": 512,
            "publicLiteralUTF8Bytes": 84, "appleRuntime": "NOT_RUN", "formalAcceptanceGranted": False}
