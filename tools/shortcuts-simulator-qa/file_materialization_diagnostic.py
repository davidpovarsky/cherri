"""Source-bound public File characterization; no runtime result is synthesized."""
import base64
import copy
import hashlib
from pathlib import Path

from export_unicode_diagnostic import ORIGINAL, params, action_is, attachment, string_value

FIXTURE_ID = "qa-file-materialization-diagnostic-v2"
SAMPLE_ID = "qa-file-materialization-diagnostic"
SOURCE_SHA256 = "f008fe73c47c9d90695b965e585b748a62698f6e4bb8cad97b21ddf158717863"
JSON_VALUE = '{"boundary":"PUBLIC_JSON_WITNESS","entries":[]}'
PREFIX = "CHERRI_QA_FILE_MATERIALIZATION_DIAG"
TYPE_FIELDS = (
    ("JSON_TYPE64", "jsonType64", "jsonText", "Text"),
    ("DECODE_TYPE64", "decodeType64", "jsonDecoded", "File"),
    ("NAMED_TYPE64", "namedType64", "jsonNamed", "File"),
    ("EMPTY_TYPE64", "emptyType64", "emptyArtifact", "File"),
    ("ONE_TYPE64", "oneType64", "oneArtifact", "File"),
    ("BINARY_TYPE64", "binaryType64", "binaryWitness", "File"),
)
COUNT_FIELDS = (
    ("JSON_COUNT", "jsonCount", "jsonText", "1"),
    ("DECODE_COUNT", "decodedCount", "jsonDecoded", "1"),
    ("NAMED_COUNT", "namedCount", "jsonNamed", "1"),
    ("EMPTY_ENTRIES", "emptyCount", "emptyEntries", "0"),
    ("ONE_ENTRIES", "oneCount", "oneEntries", "1"),
)

def b64(value):
    return base64.b64encode(value.encode("utf-8")).decode("ascii")

BYTE_FIELDS = (
    ("JSON_BYTES64", "jsonBytes64", "jsonText", b64(JSON_VALUE)),
    ("DECODE_BYTES64", "decodeBytes64", "jsonDecoded", b64(JSON_VALUE)),
    ("NAMED_BYTES64", "namedBytes64", "jsonNamed", b64(JSON_VALUE)),
    ("PAYLOAD_TEXT64", "payloadText64", "payloadValue", b64(b64(ORIGINAL))),
    ("RESTORED64", "restored64", "restoredText", b64(ORIGINAL)),
    ("NAME64", "name64", "fullName", b64("boundary.json")),
    ("STATUS64", "status64", "oneStatus", b64("TEXT_PAYLOAD_ENCODED")),
)
# This is the desired File contract, not a claim about observed Apple type names.
# Real callbacks retain every different type/count value and fail this target.
EXPECTED_FIELDS = [(label, name, b64(expected)) for label, name, _, expected in TYPE_FIELDS]
EXPECTED_FIELDS += [(label, name, expected) for label, name, _, expected in COUNT_FIELDS + BYTE_FIELDS]
EXPECTED_OUTPUT = PREFIX + "".join(":" + label + "=" + value for label, _, value in EXPECTED_FIELDS)


def require(ok, detail):
    if not ok:
        raise ValueError("FILE_MATERIALIZATION_DIAGNOSTIC_GRAPH_MISMATCH: " + detail)


def verify_source(repo, fixture_id, source):
    if fixture_id != FIXTURE_ID:
        return None
    expected = Path(repo) / "tools/shortcuts-simulator-qa/fixtures" / (FIXTURE_ID + ".cherri")
    require(hashlib.sha256(source).hexdigest() == SOURCE_SHA256 and expected.read_bytes() == source,
            "unreviewed public diagnostic source")
    return {"sourceSha256": SOURCE_SHA256, "syntheticOnly": True,
            "originalAcceptanceChanged": False, "appleRuntime": "NOT_RUN"}


def verify_graph(fixture_id, actions):
    if fixture_id != FIXTURE_ID:
        return None
    require(isinstance(actions, list) and actions, "missing actions")
    named, by_uuid = {}, {}
    for index, action in enumerate(actions):
        p = params(action)
        name, uid = p.get("CustomOutputName"), p.get("UUID")
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
        if kind:
            require(action_is(action, kind), "wrong producer " + name)
        return index, action

    def ref(value, name, coercion=None):
        token = attachment(value)
        require(token.get("Type") == "ActionOutput"
                and token.get("OutputUUID") == params(get(name)[1])["UUID"], "wrong edge " + name)
        expected = [] if coercion is None else [{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": coercion}]
        require(token.get("Aggrandizements", []) == expected, "wrong coercion " + name)

    def linked(name, kind, source, key="WFInput"):
        _, action = get(name, kind)
        ref(params(action).get(key), source)
        return params(action)

    def producer(value):
        token = attachment(value)
        require(token.get("Type") == "ActionOutput" and not token.get("Aggrandizements")
                and token.get("OutputUUID") in by_uuid, "detached input")
        return by_uuid[token["OutputUUID"]]

    def literal(value, text):
        _, action = producer(value)
        require(action_is(action, "gettext") and params(action).get("WFTextActionText") == text,
                "literal input mismatch")

    # Explicit zero-output initializer. No stale input or empty Text seed.
    clears = [(i, a) for i, a in enumerate(actions) if action_is(a, "nothing")]
    require(len(clears) == 1, "zero initializer cardinality")
    ci, clear = clears[0]
    require(set(params(clear)) == {"UUID"} and ci + 1 < len(actions)
            and action_is(actions[ci + 1], "setvariable")
            and params(actions[ci + 1]) == {"WFVariableName": "__cherri_debug_log"},
            "zero initializer inputs")
    require(params(get("jsonText", "gettext")[1]).get("WFTextActionText") == JSON_VALUE,
            "independent JSON text")
    require(params(get("original", "gettext")[1]).get("WFTextActionText") == ORIGINAL,
            "independent Unicode input")
    oi, _ = get("original")
    require(action_is(actions[oi + 1], "appendvariable")
            and params(actions[oi + 1]).get("WFVariableName") == "__cherri_debug_log",
            "original log append")
    ref(params(actions[oi + 1]).get("WFInput"), "original")
    require(linked("jsonEncoded", "base64encode", "jsonText").get("WFEncodeMode") == "Encode"
            and params(get("jsonEncoded")[1]).get("WFBase64LineBreakMode") == "None", "JSON encoding")
    require(linked("jsonDecoded", "base64encode", "jsonEncoded").get("WFEncodeMode") == "Decode", "JSON decoding")
    require(linked("jsonNamed", "setitemname", "jsonDecoded").get("WFName") == "boundary.json"
            and params(get("jsonNamed")[1]).get("WFDontIncludeFileExtension") is False, "JSON naming")
    witness = params(get("binaryWitness", "base64encode")[1])
    require(witness.get("WFEncodeMode") == "Decode", "independent binary witness")
    literal(witness.get("WFInput"), "AQ==")

    for prefix in ("empty", "one"):
        index, action = get(prefix + "Artifact", "setitemname")
        p = params(action)
        require(p.get("WFName") == "cherri-debug.json" and p.get("WFDontIncludeFileExtension") is False,
                "production export naming " + prefix)
        di, decode = producer(p.get("WFInput"))
        ei, encode = producer(params(decode).get("WFInput"))
        ti, text = producer(params(encode).get("WFInput"))
        require((ti, ei, di) == (index - 3, index - 2, index - 1)
                and action_is(decode, "base64encode") and params(decode).get("WFEncodeMode") == "Decode"
                and action_is(encode, "base64encode") and params(encode).get("WFEncodeMode") == "Encode"
                and params(encode).get("WFBase64LineBreakMode") == "None" and action_is(text, "gettext"),
                "production export materialization " + prefix)
        envelope = string_value(params(text).get("WFTextActionText"))
        require(envelope.get("string", "").startswith('{"schemaVersion":"cherri.debug-export/1"')
                and len(envelope.get("attachmentsByRange", {})) == 1, "production envelope " + prefix)
        linked(prefix + "Text", "detect.text", prefix + "Artifact")
        linked(prefix + "Parsed", "detect.dictionary", prefix + "Text")
        require(linked(prefix + "Entries", "getvalueforkey", prefix + "Parsed").get("WFDictionaryKey") == "entries",
                "production entries " + prefix)
    require(get("emptyArtifact")[0] < oi < get("oneArtifact")[0], "empty and one-entry order")
    require(linked("firstEntry", "getitemfromlist", "oneEntries").get("WFItemSpecifier") == "First Item", "first entry")
    for name, source, key in (("oneStatus", "firstEntry", "status"), ("payload", "firstEntry", "payload"), ("payloadValue", "payload", "value")):
        require(linked(name, "getvalueforkey", source).get("WFDictionaryKey") == key, "entry key " + name)
    require(linked("restoredFile", "base64encode", "payloadValue").get("WFEncodeMode") == "Decode", "actual payload decode")
    linked("restoredText", "detect.text", "restoredFile")
    linked("namedStem", "getitemname", "jsonNamed")
    require(linked("namedExtension", "properties.files", "jsonNamed").get("WFContentItemPropertyName") == "File Extension", "actual name extension")
    full = string_value(params(get("fullName", "gettext")[1]).get("WFTextActionText"))
    require(full.get("string") == "\ufffc.\ufffc" and set(full.get("attachmentsByRange", {})) == {"{0, 1}", "{2, 1}"}, "actual full name")
    for pos, name in (("{0, 1}", "namedStem"), ("{2, 1}", "namedExtension")):
        ref({"WFSerializationType": "WFTextTokenAttachment", "Value": full["attachmentsByRange"][pos]}, name)

    for _, name, source, _ in COUNT_FIELDS:
        require(linked(name, "count", source, "Input").get("WFCountType") == "Items", "count unit " + name)
    for _, name, source, _ in TYPE_FIELDS:
        linked(name + "Type", "getitemtype", source)
    for label, name, source, _ in TYPE_FIELDS + BYTE_FIELDS:
        if label.endswith("TYPE64"):
            source = name + "Type"
        p = linked(name + "Raw", "base64encode", source)
        require(p.get("WFEncodeMode") == "Encode" and p.get("WFBase64LineBreakMode") == "None", "actual observed encoding " + label)
        require(linked(name + "Length", "count", name + "Raw", "Input").get("WFCountType") == "Characters", "witness bound " + label)
        writes = [(i, a) for i, a in enumerate(actions) if action_is(a, "setvariable") and params(a).get("WFVariableName") == name]
        require(len(writes) == 2, "bounded assignments " + label)
        first, second = (i for i, _ in writes)
        require(second == first + 2, "bounded branch " + label)
        literal(params(actions[first]).get("WFInput"), "OVERSIZE")
        ref(params(actions[second]).get("WFInput"), name + "Raw")
        cp, ep = params(actions[first + 1]), params(actions[second + 1])
        require(action_is(actions[first + 1], "conditional") and action_is(actions[second + 1], "conditional")
                and cp.get("WFControlFlowMode") == 0 and ep.get("WFControlFlowMode") == 2
                and cp.get("GroupingIdentifier") == ep.get("GroupingIdentifier")
                and cp.get("WFCondition") == 0 and cp.get("WFNumberValue") == 513,
                "512-character cap " + label)
        ref(cp.get("WFInput"), name + "Length", "WFNumberContentItem")

    value = string_value(params(get("finalSummary", "gettext")[1]).get("WFTextActionText"))
    summary, attachments = PREFIX, {}
    bounded = {name for _, name, _, _ in TYPE_FIELDS + BYTE_FIELDS}
    for label, name, _ in EXPECTED_FIELDS:
        summary += ":" + label + "="
        token = {"Type": "Variable", "VariableName": name} if name in bounded else {
            "Type": "ActionOutput", "OutputName": name, "OutputUUID": params(get(name)[1])["UUID"]}
        attachments["{%d, 1}" % len(summary)] = token
        summary += "\ufffc"
    summary += "\n"
    attachments["{%d, 1}" % len(summary)] = {"Type": "ActionOutput", "OutputName": "traceEnvelope", "OutputUUID": params(get("traceEnvelope", "gettext")[1])["UUID"]}
    summary += "\ufffc"
    require(value.get("string") == summary and value.get("attachmentsByRange") == attachments,
            "independent actual result fields")
    require(action_is(actions[-1], "output"), "final output")
    out = string_value(params(actions[-1]).get("WFOutput"))
    require(out.get("string") == "\ufffc" and set(out.get("attachmentsByRange", {})) == {"{0, 1}"}, "final output token")
    ref({"WFSerializationType": "WFTextTokenAttachment", "Value": out["attachmentsByRange"]["{0, 1}"]}, "finalSummary")
    forbidden = {"runworkflow", "geturl", "downloadurl", "gettypeaction", "file", "getfile", "savefile", "getclipboard", "setclipboard"}
    require(not any(action_is(a, kind) for a in actions for kind in forbidden), "external or unreviewed conversion action")
    return {"status": "PASS_LOCAL_CAUSAL_GRAPH_ONLY", "actualTypeNames": 6, "independentCounts": 5,
            "boundedByteWitnesses": 7, "maximumCharactersPerWitness": 512,
            "appleRuntime": "NOT_RUN", "formalAcceptanceGranted": False}


def verify_causal_mutations(actions):
    baseline = verify_graph(FIXTURE_ID, actions)
    require(baseline is not None, "baseline absent")
    def named(graph, name):
        return next(a for a in graph if params(a).get("CustomOutputName") == name)
    tests = []
    for name, source, key in (("decodeType64Type", "jsonText", "WFInput"), ("namedType64Type", "jsonDecoded", "WFInput"),
                              ("emptyCount", "oneEntries", "Input"), ("oneCount", "emptyEntries", "Input"),
                              ("jsonDecoded", "binaryWitness", "WFInput"), ("restoredFile", "jsonEncoded", "WFInput")):
        def redirect(g, n=name, s=source, k=key):
            attachment(params(named(g, n))[k])["OutputUUID"] = params(named(g, s))["UUID"]
        tests.append(("redirect-" + name, redirect))
    tests += [
        ("decode-mode", lambda g: params(named(g, "jsonDecoded")).update(WFEncodeMode="Encode")),
        ("name-extension", lambda g: params(named(g, "jsonNamed")).update(WFName="boundary.txt")),
        ("independent-binary-lost", lambda g: params(named(g, "binaryWitness")).update(WFInput=params(named(g, "jsonDecoded"))["WFInput"])),
        ("count-unit", lambda g: params(named(g, "emptyCount")).update(WFCountType="Characters")),
        ("wrong-entry-key", lambda g: params(named(g, "oneEntries")).update(WFDictionaryKey="payload")),
        ("constant-summary", lambda g: params(named(g, "finalSummary")).update(WFTextActionText=EXPECTED_OUTPUT)),
        ("constant-output", lambda g: params(g[-1]).update(WFOutput=EXPECTED_OUTPUT)),
        ("clear-seed-text", lambda g: next(a for a in g if action_is(a, "nothing")).update(WFWorkflowActionIdentifier="is.workflow.actions.gettext")),
        ("clear-explicit-input", lambda g: params(next(a for a in g if action_is(a, "setvariable") and params(a).get("WFVariableName") == "__cherri_debug_log")).update(WFInput="")),
        ("external-action", lambda g: g.insert(1, {"WFWorkflowActionIdentifier": "is.workflow.actions.geturl", "WFWorkflowActionParameters": {}})),
    ]
    rejected = []
    for label, mutate in tests:
        changed = copy.deepcopy(actions)
        mutate(changed)
        try:
            verify_graph(FIXTURE_ID, changed)
        except ValueError:
            rejected.append(label)
        else:
            raise AssertionError("causal diagnostic corruption accepted: " + label)
    return {"status": "PASS_LOCAL_NEGATIVE_CONTROLS", "rejected": rejected,
            "appleRuntime": "NOT_RUN"}
