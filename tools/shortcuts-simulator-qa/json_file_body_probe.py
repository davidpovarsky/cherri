"""Causal graph for the fixed registered File-body UTF8 JSON diagnostic.

This observes transport, not compiler Optional/container semantics.
"""
import copy
import json
import re
from retry_http_probe import require, sha

FIXTURE = "qa-json-body-file-utf8-v2"
CAUSAL_CONTROLS = (
    "external-host", "changed-loopback-path", "borrowed-input", "wrong-url-range",
    "get-instead-of-post", "json-instead-of-file", "missing-file-body",
    "borrowed-body-producer", "body-text-encoded", "null-dropped",
    "null-empty-text", "null-false", "null-zero", "false-coerced-zero",
    "zero-coerced-false", "empty-text-null", "flattened-matrix",
    "missing-key-authored", "wrong-content-type", "extra-header",
    "continuation-post", "continuation-wrong-input", "borrowed-response-producer",
    "unreviewed-extra-network", "extra-nothing", "clear-borrows-input",
    "trace-current-date-lost", "trace-store-replaced", "response-edge-replaced",
    "trace-envelope-replaced", "final-output-replaced",
)


def validate_contract(value):
    require(value["fixture_id"] == FIXTURE and value["case"] == "file-utf8"
            and value["authored_api"] == "fileRequest"
            and value["authored_body_type"] == "File"
            and value["expected_content_type"] == "application/json", "FILE_BODY_CONTRACT")
    raw = value["expected_raw_utf8"].encode("utf-8")
    require(sha(raw) == value["expected_raw_utf8_sha256"], "FILE_BODY_LITERAL_HASH")
    from json_body_probe import read_json, same_json
    parsed = read_json(raw)
    require(same_json(parsed, value["expected_json"]) and set(parsed) == set(value["expected_control_json"]) | {"candidate_null"}
            and parsed["candidate_null"] is None and "missing_val" not in parsed,
            "FILE_BODY_LITERAL_FIELDS")
    require(type(parsed["stream"]) is bool and parsed["stream"] is False
            and type(parsed["zero"]) is int and parsed["zero"] == 0
            and parsed["empty_val"] == "" and parsed["nested"]["matrix"] == [[1, 2], [3, 4]],
            "FILE_BODY_LITERAL_CONTROLS")
    return raw


def graph_receipt(value):
    raw = validate_contract(value)
    return {"status": "PASS_LOCAL_CAUSAL_GRAPH_ONLY", "nativeActionCount": 17,
            "httpActionCount": 2, "primaryMethod": "POST", "continuationMethod": "GET",
            "case": "file-utf8", "bodySha256": sha(raw),
            "transport": "REGISTERED_FILE_REQUEST_TEXT_BODY",
            "candidateNullSemantics": "NOT_ASSUMED", "appleRuntime": "NOT_RUN",
            "formalAcceptanceGranted": False}


def mutation_receipt():
    return {"status": "PASS_LOCAL_NEGATIVE_CONTROLS", "rejected": list(CAUSAL_CONTROLS),
            "appleRuntime": "NOT_RUN", "formalAcceptanceGranted": False}


def verify_graph(value, fixture, actions):
    require(fixture == FIXTURE, "FILE_BODY_FIXTURE")
    validate_contract(value)
    ids = ["comment", "nothing", "setvariable", "gettext", "appendvariable", "gettext",
           "downloadurl", "downloadurl", "gettext", "comment", "gettext", "appendvariable",
           "text.combine", "gettext", "gettext", "comment", "output"]
    require(isinstance(actions, list) and len(actions) == len(ids)
            and all(isinstance(a, dict) and set(a) == {"WFWorkflowActionIdentifier", "WFWorkflowActionParameters"}
                    and a["WFWorkflowActionIdentifier"] == "is.workflow.actions." + name
                    and isinstance(a["WFWorkflowActionParameters"], dict)
                    for a, name in zip(actions, ids)), "FILE_BODY_UNREVIEWED_EFFECT")
    uuids = [a["WFWorkflowActionParameters"].get("UUID") for a in actions]
    slots = {1, 3, 5, 6, 8, 9, 10, 12, 13, 14}
    present = [u for u in uuids if u is not None]
    require({i for i, u in enumerate(uuids) if u is not None} == slots
            and all(isinstance(u, str) and re.fullmatch(r"[0-9A-Fa-f]{8}(?:-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12}", u) for u in present)
            and len(set(present)) == len(present), "FILE_BODY_PRODUCER_IDENTITY")
    def ref(index, name):
        return {"OutputName": name, "OutputUUID": uuids[index], "Type": "ActionOutput"}
    def attached(item):
        return {"Value": item, "WFSerializationType": "WFTextTokenAttachment"}
    def token(string, refs):
        positions = [i for i, c in enumerate(string) if c == "\ufffc"]
        require(len(positions) == len(refs), "FILE_BODY_ORACLE_TOKEN_COUNT")
        return {"Value": {"string": string, "attachmentsByRange": {
            "{%d, 1}" % (len(string[:i].encode("utf-16-le")) // 2): item
            for i, item in zip(positions, refs)}}, "WFSerializationType": "WFTextTokenString"}
    def plain(string):
        return {"Value": {"string": string}, "WFSerializationType": "WFTextTokenString"}
    def trace(stage, step, action):
        value = json.dumps({"schemaVersion": "1.4", "kind": "INLINE_DEBUG_ACTION",
            "stage": stage, "label": "json-body-file-utf8", "stepSequence": step,
            "actionIdentifier": "is.workflow.actions." + action,
            "timestampPrecision": "apple-currentdate", "valueType": "UNKNOWN_AT_TRACE_EVENT",
            "timestamp": "\ufffc"}, ensure_ascii=False, separators=(",", ":"))
        return token(value, [{"Type": "CurrentDate"}])
    def collector(stage):
        return json.dumps({"schemaVersion": "1.4", "kind": "COLLECTOR_EVENT", "stage": stage,
            "runId": fixture, "traceTransport": "ndjson-stream",
            "collectorSentinel": "===CHERRI_QA_TRACE_NDJSON_V14==="}, separators=(",", ":"))
    headers = {"Value": {"WFDictionaryFieldValueItems": [{
        "WFItemType": 0, "WFKey": plain("Content-Type"), "WFValue": plain("application/json")
    }]}, "WFSerializationType": "WFDictionaryFieldValue"}
    expected = [
        {"WFCommentActionText": "ANCHOR_CARD_01_TOP_HEADER"},
        {"UUID": uuids[1]},
        {"WFVariableName": "__cherri_debug_log"},
        {"UUID": uuids[3], "WFTextActionText": trace("before", 3, "setvariable")},
        {"WFInput": attached(ref(3, "Text")), "WFVariableName": "__cherri_qa_trace"},
        {"UUID": uuids[5], "CustomOutputName": "bodyText", "WFTextActionText": value["expected_raw_utf8"]},
        {"UUID": uuids[6], "CustomOutputName": "response", "WFHTTPMethod": "POST",
         "WFHTTPBodyType": "File", "WFURL": token("http://127.0.0.1:\ufffc/file-utf8", [{"Type": "ExtensionInput"}]),
         "WFRequestVariable": attached(ref(5, "bodyText")), "WFHTTPHeaders": headers},
        {"WFHTTPMethod": "GET", "WFURL": token("http://127.0.0.1:\ufffc/continued", [{"Type": "ExtensionInput"}])},
        {"UUID": uuids[8], "CustomOutputName": "semanticResult",
         "WFTextActionText": token(value["expected_callback_prefix"] + "\ufffc", [ref(6, "response")])},
        {"UUID": uuids[9], "WFCommentActionText": "ANCHOR_CARD_15_MID_CENTER"},
        {"UUID": uuids[10], "WFTextActionText": trace("after", 10, "comment")},
        {"WFInput": attached(ref(10, "Text")), "WFVariableName": "__cherri_qa_trace"},
        {"UUID": uuids[12], "WFTextSeparator": "New Lines",
         "text": attached({"Type": "Variable", "VariableName": "__cherri_qa_trace"})},
        {"UUID": uuids[13], "CustomOutputName": "traceEnvelope", "WFTextActionText": token(
            collector("collector_begin") + "\n\ufffc\n" + collector("collector_end"), [ref(12, "Combined Text")])},
        {"UUID": uuids[14], "CustomOutputName": "finalSummary",
         "WFTextActionText": token("\ufffc\n\ufffc", [ref(8, "semanticResult"), ref(13, "traceEnvelope")])},
        {"WFCommentActionText": "ANCHOR_CARD_28_BOTTOM_FOOTER"},
        {"WFOutput": token("\ufffc", [ref(14, "finalSummary")])},
    ]
    for index, params in enumerate(expected):
        require(json.dumps(actions[index]["WFWorkflowActionParameters"], sort_keys=True) == json.dumps(params, sort_keys=True),
                "FILE_BODY_ACTION_%d_CHANGED" % index)
    return graph_receipt(value)


def verify_mutations(value, fixture, actions):
    verify_graph(value, fixture, actions)
    def params(a, index=6):
        return a[index]["WFWorkflowActionParameters"]
    def body(a, change):
        original = json.loads(params(a, 5)["WFTextActionText"])
        change(original)
        params(a, 5)["WFTextActionText"] = json.dumps(original, separators=(",", ":"))
    mutations = {
        "external-host": lambda a: params(a)["WFURL"]["Value"].update(string="https://example.invalid/\ufffc"),
        "changed-loopback-path": lambda a: params(a)["WFURL"]["Value"].update(string="http://127.0.0.1:\ufffc/wrong"),
        "borrowed-input": lambda a: params(a)["WFURL"]["Value"].update(attachmentsByRange={"{17, 1}": {"Type": "Clipboard"}}),
        "wrong-url-range": lambda a: params(a)["WFURL"]["Value"].update(attachmentsByRange={"{16, 1}": {"Type": "ExtensionInput"}}),
        "get-instead-of-post": lambda a: params(a).update(WFHTTPMethod="GET"),
        "json-instead-of-file": lambda a: params(a).update(WFHTTPBodyType="JSON"),
        "missing-file-body": lambda a: params(a).pop("WFRequestVariable"),
        "borrowed-body-producer": lambda a: params(a)["WFRequestVariable"]["Value"].update(OutputUUID=params(a, 3)["UUID"]),
        "body-text-encoded": lambda a: params(a, 5).update(WFTextActionText="BASE64_INSTEAD_OF_RAW_JSON"),
        "null-dropped": lambda a: body(a, lambda b: b.pop("candidate_null")),
        "null-empty-text": lambda a: body(a, lambda b: b.update(candidate_null="")),
        "null-false": lambda a: body(a, lambda b: b.update(candidate_null=False)),
        "null-zero": lambda a: body(a, lambda b: b.update(candidate_null=0)),
        "false-coerced-zero": lambda a: body(a, lambda b: b.update(stream=0)),
        "zero-coerced-false": lambda a: body(a, lambda b: b.update(zero=False)),
        "empty-text-null": lambda a: body(a, lambda b: b.update(empty_val=None)),
        "flattened-matrix": lambda a: body(a, lambda b: b["nested"].update(matrix=[1, 2, 3, 4])),
        "missing-key-authored": lambda a: body(a, lambda b: b.update(missing_val=None)),
        "wrong-content-type": lambda a: params(a)["WFHTTPHeaders"]["Value"]["WFDictionaryFieldValueItems"][0]["WFValue"]["Value"].update(string="text/plain"),
        "extra-header": lambda a: params(a)["WFHTTPHeaders"]["Value"]["WFDictionaryFieldValueItems"].append(copy.deepcopy(params(a)["WFHTTPHeaders"]["Value"]["WFDictionaryFieldValueItems"][0])),
        "continuation-post": lambda a: params(a, 7).update(WFHTTPMethod="POST"),
        "continuation-wrong-input": lambda a: params(a, 7)["WFURL"]["Value"].update(attachmentsByRange={}),
        "borrowed-response-producer": lambda a: params(a).update(UUID=params(a, 3)["UUID"]),
        "unreviewed-extra-network": lambda a: a.append(copy.deepcopy(a[6])),
        "extra-nothing": lambda a: a.append({"WFWorkflowActionIdentifier": "is.workflow.actions.nothing", "WFWorkflowActionParameters": {}}),
        "clear-borrows-input": lambda a: params(a, 2).update(WFInput=""),
        "trace-current-date-lost": lambda a: params(a, 3)["WFTextActionText"]["Value"].update(attachmentsByRange={}),
        "trace-store-replaced": lambda a: params(a, 4).update(WFVariableName="__cherri_debug_log"),
        "response-edge-replaced": lambda a: params(a, 8)["WFTextActionText"]["Value"].update(attachmentsByRange={}),
        "trace-envelope-replaced": lambda a: params(a, 13)["WFTextActionText"]["Value"].update(string="wrong"),
        "final-output-replaced": lambda a: params(a, 16).update(WFOutput="success"),
    }
    require(tuple(mutations) == CAUSAL_CONTROLS, "FILE_BODY_CAUSAL_CONTROL_INVENTORY")
    rejected = []
    for name, mutate in mutations.items():
        changed = copy.deepcopy(actions)
        mutate(changed)
        try:
            verify_graph(value, fixture, changed)
        except ValueError:
            rejected.append(name)
        else:
            raise ValueError("FILE_BODY_GRAPH_CORRUPTION_ACCEPTED:" + name)
    require(rejected == list(CAUSAL_CONTROLS), "FILE_BODY_CAUSAL_CONTROLS_INCOMPLETE")
    return mutation_receipt()


def raw_body_exact(report, value):
    """Derive exact-byte transport truth from preserved public request bytes."""
    events = report["receipt"]["events"]
    if not report["controlledBodyVerified"] or not report["runtimeContractVerified"] or not events:
        return False
    import base64
    encoded = events[0]["requestBody"]["publicBodyBase64"]
    return bool(encoded is not None and base64.b64decode(encoded, validate=True) == validate_contract(value)
                and events[0]["requestBody"]["candidateKind"] == "NULL"
                and events[0]["requestBody"]["candidateNullObserved"] is True)
