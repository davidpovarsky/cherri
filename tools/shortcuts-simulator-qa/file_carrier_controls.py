"""Fixed public carrier experiments. Native observations are never synthesized."""
import base64
import copy
import hashlib
from pathlib import Path

from export_unicode_diagnostic import params, action_is, attachment, string_value

JSON_VALUE = '{"marker":"FILE_CARRIER_PUBLIC", "text":"שָׁלוֹם 🍒", "entries":[]}'
PREFIX = "CHERRI_QA_FILE_CARRIER"
MODES = {
    "qa-file-carrier-text-data-v2": ("text-data", "gettypeaction", "jsonText", "WFInput"),
    "qa-file-carrier-dictionary-data-v2": ("dictionary-data", "gettypeaction", "jsonDecoded", "WFInput"),
    "qa-file-carrier-file-coercion-v2": ("file-coercion", "getvariable", "jsonDecoded", "WFVariable"),
}
SOURCE_HASHES = {'qa-file-carrier-dictionary-data-v2': '5f73646410a54f3f0f15982e5e120df98d458304277e1decf87261c7fb33f016', 'qa-file-carrier-file-coercion-v2': '113f366c1c3f56ca035fd6aaaa749f16c7b2ed8818624331b80807c276eb57d4', 'qa-file-carrier-text-data-v2': '5a6ef2311370661943b2f23e8e6ec3396a7b8d0e9043b2087d9fd7e6e452866c'}


def b64(text):
    return base64.b64encode(text.encode("utf-8")).decode("ascii")


FIELDS = (
    ("TYPE64", "carrierType64", "carrierType", b64("File")),
    ("NAMED_TYPE64", "namedType64", "namedType", b64("File")),
    ("COUNT", "itemCount", "namedCarrier", "1"),
    ("BYTES64", "carrierBytes64", "namedCarrier", b64(JSON_VALUE)),
    ("NAME64", "name64", "fullName", b64("carrier.json")),
    ("JSON64", "jsonBytes64", "jsonText", b64(JSON_VALUE)),
    ("BINARY_TYPE64", "binaryType64", "binaryType", b64("File")),
    ("DECODE_TYPE64", "decodeType64", "decodedType", b64("Dictionary")),
)
EXPECTED = {fid: PREFIX + ":MODE=" + mode[0]
            + "".join(":" + label + "=" + expected for label, _, _, expected in FIELDS)
            for fid, mode in MODES.items()}


def require(condition, detail):
    if not condition:
        raise ValueError("FILE_CARRIER_CONTROL_MISMATCH: " + detail)


def verify_source(repo, fixture_id, source):
    if fixture_id not in MODES:
        return None
    path = Path(repo) / "tools/shortcuts-simulator-qa/fixtures" / (fixture_id + ".cherri")
    require(hashlib.sha256(source).hexdigest() == SOURCE_HASHES[fixture_id]
            and path.read_bytes() == source, "unreviewed fixture bytes")
    return {"sourceSha256": SOURCE_HASHES[fixture_id], "conversion": MODES[fixture_id][0],
            "syntheticOnly": True, "originalAcceptanceChanged": False,
            "appleRuntime": "NOT_RUN", "formalAcceptanceGranted": False}


def verify_graph(fixture_id, actions):
    if fixture_id not in MODES:
        return None
    mode, conversion, source, key = MODES[fixture_id]
    named, by_uuid = {}, {}
    require(isinstance(actions, list) and actions, "missing graph")
    for index, action in enumerate(actions):
        p = params(action)
        uid, name = p.get("UUID"), p.get("CustomOutputName")
        if uid:
            require(uid not in by_uuid, "duplicate UUID")
            by_uuid[uid] = (index, action)
        if name:
            named.setdefault(name, []).append((index, action))

    def get(name, kind=None):
        rows = named.get(name, [])
        require(len(rows) == 1, "missing/duplicate producer " + name)
        index, action = rows[0]
        require(params(action).get("UUID") in by_uuid, "missing UUID " + name)
        require(kind is None or action_is(action, kind), "wrong producer " + name)
        return index, action

    def ref(value, name, coercion=None):
        token = attachment(value)
        require(token.get("Type") == "ActionOutput"
                and token.get("OutputUUID") == params(get(name)[1])["UUID"], "wrong edge " + name)
        expected = [] if coercion is None else [{"Type": "WFCoercionVariableAggrandizement",
                                                "CoercionItemClass": coercion}]
        require(token.get("Aggrandizements", []) == expected, "wrong coercion " + name)

    def linked(name, kind, producer, parameter="WFInput", coercion=None):
        p = params(get(name, kind)[1])
        ref(p.get(parameter), producer, coercion)
        return p

    def literal(value, expected):
        token = attachment(value)
        require(token.get("Type") == "ActionOutput" and not token.get("Aggrandizements")
                and token.get("OutputUUID") in by_uuid, "detached literal")
        _, action = by_uuid[token["OutputUUID"]]
        require(action_is(action, "gettext") and params(action).get("WFTextActionText") == expected,
                "different literal")

    clears = [(i, a) for i, a in enumerate(actions) if action_is(a, "nothing")]
    require(len(clears) == 1, "owned zero initializer cardinality")
    ci, clear = clears[0]
    require(set(params(clear)) == {"UUID"} and ci + 1 < len(actions)
            and action_is(actions[ci + 1], "setvariable")
            and params(actions[ci + 1]) == {"WFVariableName": "__cherri_debug_log"},
            "owned zero initializer inputs")
    require(params(get("jsonText", "gettext")[1]).get("WFTextActionText") == JSON_VALUE,
            "independent Unicode JSON source")
    p = linked("jsonEncoded", "base64encode", "jsonText")
    require(p.get("WFEncodeMode") == "Encode" and p.get("WFBase64LineBreakMode") == "None", "source encoding")
    require(linked("jsonDecoded", "base64encode", "jsonEncoded").get("WFEncodeMode") == "Decode", "baseline decode")
    p = linked("carrier", conversion, source, key,
               "WFGenericFileContentItem" if mode == "file-coercion" else None)
    if conversion == "gettypeaction":
        require(p.get("WFFileType") == "public.data", "explicit data UTI")
        require(sum(action_is(a, "gettypeaction") for a in actions) == 1, "one isolated conversion")
    else:
        require(not any(action_is(a, "gettypeaction") for a in actions), "different conversion experiment")
        require(set(p) == {"WFVariable", "UUID", "CustomOutputName"}, "coercion action parameters")
    p = linked("namedCarrier", "setitemname", "carrier")
    require(p.get("WFName") == "carrier.json" and p.get("WFDontIncludeFileExtension") is False, "actual carrier name")
    require(sum(action_is(a, "setitemname") for a in actions) == 1, "one isolated name operation")
    witness = params(get("binaryWitness", "base64encode")[1])
    require(witness.get("WFEncodeMode") == "Decode", "independent binary decode")
    literal(witness.get("WFInput"), "AQ==")
    for observer, producer in (("carrierType", "carrier"), ("namedType", "namedCarrier"),
                               ("binaryType", "binaryWitness"), ("decodedType", "jsonDecoded")):
        linked(observer, "getitemtype", producer)
    require(linked("itemCount", "count", "namedCarrier", "Input").get("WFCountType") == "Items", "count units")
    linked("namedStem", "getitemname", "namedCarrier")
    require(linked("namedExtension", "properties.files", "namedCarrier").get("WFContentItemPropertyName") == "File Extension",
            "extension observation")
    full = string_value(params(get("fullName", "gettext")[1]).get("WFTextActionText"))
    require(full.get("string") == "\ufffc.\ufffc" and set(full.get("attachmentsByRange", {})) == {"{0, 1}", "{2, 1}"}, "full name")
    for pos, name in (("{0, 1}", "namedStem"), ("{2, 1}", "namedExtension")):
        ref({"WFSerializationType": "WFTextTokenAttachment", "Value": full["attachmentsByRange"][pos]}, name)
    for label, name, producer, _ in FIELDS:
        if label == "COUNT":
            continue
        p = linked(name + "Raw", "base64encode", producer)
        require(p.get("WFEncodeMode") == "Encode" and p.get("WFBase64LineBreakMode") == "None", "observed bytes " + label)
        require(linked(name + "Length", "count", name + "Raw", "Input").get("WFCountType") == "Characters", "witness bound " + label)
        writes = [(i, a) for i, a in enumerate(actions) if action_is(a, "setvariable") and params(a).get("WFVariableName") == name]
        require(len(writes) == 2, "bounded writes " + label)
        first, second = (i for i, _ in writes)
        require(second == first + 2 and second + 1 < len(actions), "bounded branch " + label)
        literal(params(actions[first]).get("WFInput"), "OVERSIZE")
        ref(params(actions[second]).get("WFInput"), name + "Raw")
        cp, ep = params(actions[first + 1]), params(actions[second + 1])
        require(action_is(actions[first + 1], "conditional") and action_is(actions[second + 1], "conditional")
                and cp.get("WFControlFlowMode") == 0 and ep.get("WFControlFlowMode") == 2
                and cp.get("GroupingIdentifier") == ep.get("GroupingIdentifier")
                and cp.get("WFCondition") == 0 and cp.get("WFNumberValue") == 513, "512-character cap " + label)
        ref(cp.get("WFInput"), name + "Length", "WFNumberContentItem")
    value = string_value(params(get("finalSummary", "gettext")[1]).get("WFTextActionText"))
    summary, attachments = PREFIX + ":MODE=" + mode, {}
    for label, name, _, _ in FIELDS:
        summary += ":" + label + "="
        token = ({"Type": "ActionOutput", "OutputName": name, "OutputUUID": params(get(name)[1])["UUID"]}
                 if label == "COUNT" else {"Type": "Variable", "VariableName": name})
        attachments["{%d, 1}" % len(summary)] = token
        summary += "\ufffc"
    summary += "\n"
    attachments["{%d, 1}" % len(summary)] = {"Type": "ActionOutput", "OutputName": "traceEnvelope",
                                               "OutputUUID": params(get("traceEnvelope", "gettext")[1])["UUID"]}
    summary += "\ufffc"
    require(value.get("string") == summary and value.get("attachmentsByRange") == attachments, "actual independent result fields")
    require(action_is(actions[-1], "output"), "final output")
    out = string_value(params(actions[-1]).get("WFOutput"))
    require(out.get("string") == "\ufffc" and set(out.get("attachmentsByRange", {})) == {"{0, 1}"}, "final output token")
    ref({"WFSerializationType": "WFTextTokenAttachment", "Value": out["attachmentsByRange"]["{0, 1}"]}, "finalSummary")
    allowed = {"nothing", "comment", "gettext", "base64encode", "setitemname", "getitemname", "properties.files", "getitemtype",
               "count", "setvariable", "conditional", "appendvariable", "text.combine", "output"}
    allowed.add(conversion)
    require(all(a.get("WFWorkflowActionIdentifier") in {"is.workflow.actions." + x for x in allowed} for a in actions),
            "external/unreviewed effect")
    require(all(params(a).get("WFVariableName") == "__cherri_qa_trace" for a in actions if action_is(a, "appendvariable")),
            "unexpected payload buffer")
    return {"status": "PASS_LOCAL_CAUSAL_GRAPH_ONLY", "conversion": mode,
            "independentObservedFields": len(FIELDS), "maximumCharactersPerWitness": 512,
            "appleRuntime": "NOT_RUN", "formalAcceptanceGranted": False}


def verify_causal_mutations(fixture_id, actions):
    verify_graph(fixture_id, actions)
    mode, _, source, key = MODES[fixture_id]
    def named(graph, name):
        return next(a for a in graph if params(a).get("CustomOutputName") == name)
    def redirect(graph, name, other, parameter="WFInput"):
        attachment(params(named(graph, name))[parameter])["OutputUUID"] = params(named(graph, other))["UUID"]
    tests = [
        ("wrong-conversion-source", lambda g: redirect(g, "carrier", "binaryWitness", key)),
        ("type-inspects-original", lambda g: redirect(g, "carrierType", "jsonText")),
        ("named-type-inspects-prior", lambda g: redirect(g, "namedType", "carrier")),
        ("bytes-inspect-original", lambda g: redirect(g, "carrierBytes64Raw", "jsonText")),
        ("wrong-name-source", lambda g: redirect(g, "namedCarrier", "jsonDecoded")),
        ("count-original", lambda g: redirect(g, "itemCount", "jsonText", "Input")),
        ("count-unit", lambda g: params(named(g, "itemCount")).update(WFCountType="Characters")),
        ("filename", lambda g: params(named(g, "namedCarrier")).update(WFName="carrier.txt")),
        ("source-bytes", lambda g: params(named(g, "jsonText")).update(WFTextActionText='{"entries":[]}')),
        ("binary-witness", lambda g: params(named(g, "binaryWitness")).update(WFEncodeMode="Encode")),
        ("constant-summary", lambda g: params(named(g, "finalSummary")).update(WFTextActionText=EXPECTED[fixture_id])),
        ("constant-output", lambda g: params(g[-1]).update(WFOutput=EXPECTED[fixture_id])),
        ("external-action", lambda g: g.insert(1, {"WFWorkflowActionIdentifier": "is.workflow.actions.downloadurl", "WFWorkflowActionParameters": {}})),
    ]
    tests += [
        ("zero-clear-seed", lambda g: next(a for a in g if action_is(a, "nothing")).update(WFWorkflowActionIdentifier="is.workflow.actions.gettext")),
        ("zero-clear-input", lambda g: params(next(a for a in g if action_is(a, "setvariable") and params(a).get("WFVariableName") == "__cherri_debug_log")).update(WFInput="")),
    ]
    if mode == "file-coercion":
        tests.append(("file-coercion-lost", lambda g: attachment(params(named(g, "carrier"))[key]).pop("Aggrandizements")))
    else:
        tests.append(("data-uti", lambda g: params(named(g, "carrier")).update(WFFileType="public.rtf")))
    rejected = []
    for label, mutate in tests:
        changed = copy.deepcopy(actions)
        mutate(changed)
        try:
            verify_graph(fixture_id, changed)
        except ValueError:
            rejected.append(label)
        else:
            raise AssertionError("carrier corruption accepted: " + label)
    return {"status": "PASS_LOCAL_NEGATIVE_CONTROLS", "rejected": rejected, "appleRuntime": "NOT_RUN"}
