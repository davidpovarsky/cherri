#!/usr/bin/env python3
"""Observe fixed public JSON request bodies on one run-owned loopback endpoint.

WFItemType 5 is a wire candidate, not a supported compiler null representation.
An observed request is kept separate from its field semantics and from acceptance.
"""
import argparse
import base64
import copy
import hashlib
from http.server import BaseHTTPRequestHandler
import json
import math
from pathlib import Path
import re
import signal
import threading
import time

from retry_http_probe import LoopbackHTTPServer, atomic_json, require, sha, utc

FIXTURES = {
    "qa-json-body-control-v2": ("control", "292d75762a0bc9f6924bc3ec5f9f2649795043768f45405ceec4c6015ac25a6c", "e893d69244f75d85b4582fbcfb3216296d942850109795ff6b263a9548f05122"),
    "qa-json-body-null-candidate-v2": ("null-candidate", "6803a3efe34a1f374a255c948d9f401a752b0eec7bf55b6b492c4748b4bf50f0", "fb984a98169609c93962f86098b91f986169bdf31895b45e787e6c6cf8e20fc3"),
}
BODY = b"P15_JSON_BODY_OBSERVED"
CONTINUED = b"P15_JSON_BODY_CONTINUED"
MAX_BODY = 8192
MAX_REQUESTS = 4
MAX_LIFETIME = 1800
HASH = re.compile(r"[0-9a-f]{64}")
STAMP = re.compile(r"\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{6}Z")
CAUSAL_CONTROLS = (
    "external-host", "changed-loopback-path", "borrowed-input", "wrong-url-range",
    "get-instead-of-post", "form-instead-of-json", "missing-json-body", "extra-header",
    "dropped-first-field", "wrong-item-type", "lost-zero", "false-coerced-to-text",
    "extra-candidate-value", "continuation-post", "continuation-wrong-input",
    "borrowed-response-producer", "unreviewed-extra-network", "extra-nothing",
    "clear-borrows-input", "trace-current-date-lost", "trace-store-replaced",
    "response-edge-replaced", "trace-envelope-replaced", "final-output-replaced",
)


def read_json(data):
    def pairs(values):
        result = {}
        for key, value in values:
            require(key not in result, "DUPLICATE_JSON_KEY")
            result[key] = value
        return result
    return json.loads(data, object_pairs_hook=pairs,
                      parse_constant=lambda _: require(False, "NONFINITE_JSON_NUMBER"))


def identity(fixture, nonce, commit):
    require(fixture in FIXTURES, "UNREVIEWED_JSON_FIXTURE")
    require(isinstance(nonce, str) and len(nonce) <= 128 and
            re.fullmatch(re.escape(fixture) + r"-[A-Za-z0-9_-]{1,50}-[0-9a-f]{32}", nonce),
            "INVALID_FRESH_NONCE")
    require(isinstance(commit, str) and re.fullmatch(r"[0-9a-f]{40}", commit), "INVALID_SOURCE")
    return {"schemaVersion": "1", "fixtureId": fixture, "runNonce": nonce, "compilerSha": commit}


def contract(repo, fixture, source=None):
    if fixture not in FIXTURES:
        return None
    root = Path(repo) / "tools/shortcuts-simulator-qa/fixtures"
    source_path, expected_path = root / (fixture + ".cherri"), root / (fixture + ".expected.json")
    require(root.is_dir() and not root.is_symlink() and source_path.is_file() and
            expected_path.is_file() and not source_path.is_symlink() and not expected_path.is_symlink(),
            "FIXTURE_PATH")
    actual = source_path.read_bytes()
    require(source is None or source == actual, "SOURCE_BYTES_DIFFER")
    case, source_hash, expected_hash = FIXTURES[fixture]
    data = expected_path.read_bytes()
    require(sha(actual) == source_hash and sha(data) == expected_hash, "REVIEWED_FIXTURE_HASH")
    value = read_json(data)
    require(value["schema_version"] == "1" and value["fixture_id"] == fixture and
            value["case"] == case and value["source_sha256"] == source_hash and
            value["runtime_status"] == "NOT_RUN" and value["formal_requirements_proved"] == [] and
            value["candidate_null_semantics"] == "NOT_ASSUMED", "FIXTURE_CONTRACT")
    return value


def verify_source(repo, fixture, source):
    value = contract(repo, fixture, source)
    if value is None:
        return None
    return {"sourceSha256": FIXTURES[fixture][1], "expectedContractSha256": FIXTURES[fixture][2],
            "case": value["case"], "syntheticOnly": True, "candidateNullSemantics": "NOT_ASSUMED",
            "appleRuntime": "NOT_RUN", "formalAcceptanceGranted": False}


def graph_receipt(value):
    return {"status": "PASS_LOCAL_CAUSAL_GRAPH_ONLY", "nativeActionCount": 16, "httpActionCount": 2,
            "primaryMethod": "POST", "continuationMethod": "GET", "case": value["case"],
            "bodySha256": sha(json.dumps(value["expected_native_parameters"]["WFJSONValues"], sort_keys=True, separators=(",", ":")).encode()),
            "candidateNullSemantics": "NOT_ASSUMED", "appleRuntime": "NOT_RUN", "formalAcceptanceGranted": False}


def mutation_receipt():
    return {"status": "PASS_LOCAL_NEGATIVE_CONTROLS", "rejected": list(CAUSAL_CONTROLS),
            "appleRuntime": "NOT_RUN", "formalAcceptanceGranted": False}


def verify_graph(repo, fixture, actions):
    value = contract(repo, fixture)
    if value is None:
        return None
    ids = ["comment", "nothing", "setvariable", "gettext", "appendvariable", "downloadurl",
           "downloadurl", "gettext", "comment", "gettext", "appendvariable", "text.combine",
           "gettext", "gettext", "comment", "output"]
    require(isinstance(actions, list) and len(actions) == len(ids) and all(isinstance(a, dict) and
            set(a) == {"WFWorkflowActionIdentifier", "WFWorkflowActionParameters"} and
            a["WFWorkflowActionIdentifier"] == "is.workflow.actions." + name and
            isinstance(a["WFWorkflowActionParameters"], dict)
            for a, name in zip(actions, ids)), "UNREVIEWED_JSON_EFFECT")
    http = [(i, a.get("WFWorkflowActionParameters", {})) for i, a in enumerate(actions)
            if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.downloadurl"]
    require(len(http) == 2, "JSON_HTTP_ACTION_COUNT")
    first, second = http[0][1], http[1][1]
    actual = first
    require(actual == value["expected_native_parameters"], "JSON_NATIVE_PARAMETERS_CHANGED")
    expected_url = {"Value": {"string": "http://127.0.0.1:\ufffc/continued",
                    "attachmentsByRange": {"{17, 1}": {"Type": "ExtensionInput"}}},
                    "WFSerializationType": "WFTextTokenString"}
    require(second == {"WFHTTPMethod": "GET", "WFURL": expected_url}, "JSON_CONTINUATION_CHANGED")
    uuids = [a.get("WFWorkflowActionParameters", {}).get("UUID") for a in actions]
    present = [x for x in uuids if x is not None]
    require(all(isinstance(x, str) and re.fullmatch(r"[0-9A-Fa-f-]{36}", x) for x in present) and
            len(set(present)) == len(present), "JSON_PRODUCER_IDENTITY")
    require(first.get("UUID"), "JSON_HTTP_PRODUCERS")
    clears = [(i, a.get("WFWorkflowActionParameters", {})) for i, a in enumerate(actions)
              if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" and
              a.get("WFWorkflowActionParameters", {}).get("WFVariableName") == "__cherri_debug_log"]
    require(len(clears) == 1 and 0 < clears[0][0] < http[0][0], "JSON_CLEAR_COUNT")
    index, clear = clears[0]
    seed = actions[index - 1]
    require(seed["WFWorkflowActionIdentifier"] == "is.workflow.actions.nothing" and
            "WFInput" not in seed.get("WFWorkflowActionParameters", {}) and "WFInput" not in clear and
            sum(a["WFWorkflowActionIdentifier"] == "is.workflow.actions.nothing" for a in actions) == 1,
            "JSON_CLEAR_NOT_ZERO_OUTPUT")
    # The unused GET intentionally has no output UUID. Require its actual
    # canonical parameters and every remaining action/edge independently.
    uuid_slots = {1, 3, 5, 7, 8, 9, 11, 12, 13}
    require({i for i, item in enumerate(uuids) if item is not None} == uuid_slots,
            "JSON_UUID_SLOTS")
    def ref(index, name):
        return {"OutputName": name, "OutputUUID": uuids[index], "Type": "ActionOutput"}
    def attached(item):
        return {"Value": item, "WFSerializationType": "WFTextTokenAttachment"}
    def token(string, refs):
        positions = [i for i, c in enumerate(string) if c == "\ufffc"]
        require(len(positions) == len(refs), "ORACLE_TOKEN_COUNT")
        return {"Value": {"string": string, "attachmentsByRange": {
            "{%d, 1}" % (len(string[:i].encode("utf-16-le")) // 2): item
            for i, item in zip(positions, refs)}}, "WFSerializationType": "WFTextTokenString"}
    def trace(stage, step, action):
        text = json.dumps({"schemaVersion": "1.4", "kind": "INLINE_DEBUG_ACTION", "stage": stage,
            "label": "json-body-" + value["case"], "stepSequence": step,
            "actionIdentifier": "is.workflow.actions." + action, "timestampPrecision": "apple-currentdate",
            "valueType": "UNKNOWN_AT_TRACE_EVENT", "timestamp": "\ufffc"}, ensure_ascii=False, separators=(",", ":"))
        return token(text, [{"Type": "CurrentDate"}])
    def collector(stage):
        return json.dumps({"schemaVersion": "1.4", "kind": "COLLECTOR_EVENT", "stage": stage,
            "runId": fixture, "traceTransport": "ndjson-stream",
            "collectorSentinel": "===CHERRI_QA_TRACE_NDJSON_V14==="}, separators=(",", ":"))
    expected = [
        {"WFCommentActionText": "ANCHOR_CARD_01_TOP_HEADER"},
        {"UUID": uuids[1]},
        {"WFVariableName": "__cherri_debug_log"},
        {"UUID": uuids[3], "WFTextActionText": trace("before", 3, "setvariable")},
        {"WFInput": attached(ref(3, "Text")), "WFVariableName": "__cherri_qa_trace"},
        value["expected_native_parameters"],
        {"WFHTTPMethod": "GET", "WFURL": expected_url},
        {"UUID": uuids[7], "CustomOutputName": "semanticResult", "WFTextActionText": token(
            value["expected_callback_prefix"] + "\ufffc", [ref(5, "response")])},
        {"UUID": uuids[8], "WFCommentActionText": "ANCHOR_CARD_15_MID_CENTER"},
        {"UUID": uuids[9], "WFTextActionText": trace("after", 9, "comment")},
        {"WFInput": attached(ref(9, "Text")), "WFVariableName": "__cherri_qa_trace"},
        {"UUID": uuids[11], "WFTextSeparator": "New Lines", "text": attached({
            "Type": "Variable", "VariableName": "__cherri_qa_trace"})},
        {"UUID": uuids[12], "CustomOutputName": "traceEnvelope", "WFTextActionText": token(
            collector("collector_begin") + "\n\ufffc\n" + collector("collector_end"), [ref(11, "Combined Text")])},
        {"UUID": uuids[13], "CustomOutputName": "finalSummary", "WFTextActionText": token(
            "\ufffc\n\ufffc", [ref(7, "semanticResult"), ref(12, "traceEnvelope")])},
        {"WFCommentActionText": "ANCHOR_CARD_28_BOTTOM_FOOTER"},
        {"WFOutput": token("\ufffc", [ref(13, "finalSummary")])},
    ]
    for i, params in enumerate(expected):
        require(actions[i]["WFWorkflowActionParameters"] == params, "JSON_ACTION_%d_CHANGED" % i)
    return graph_receipt(value)


def verify_mutations(repo, fixture, actions):
    if fixture not in FIXTURES:
        return None
    verify_graph(repo, fixture, actions)
    indices = [i for i, a in enumerate(actions) if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.downloadurl"]
    first, second = indices
    def params(value, index=first):
        return value[index]["WFWorkflowActionParameters"]
    def field(value, key):
        return next(x for x in params(value)["WFJSONValues"]["Value"]["WFDictionaryFieldValueItems"]
                    if x["WFKey"]["Value"]["string"] == key)
    mutations = {
        "external-host": lambda x: params(x)["WFURL"]["Value"].update(string="https://example.invalid/\ufffc"),
        "changed-loopback-path": lambda x: params(x)["WFURL"]["Value"].update(string="http://127.0.0.1:\ufffc/wrong"),
        "borrowed-input": lambda x: params(x)["WFURL"]["Value"].update(attachmentsByRange={"{17, 1}": {"Type": "Clipboard"}}),
        "wrong-url-range": lambda x: params(x)["WFURL"]["Value"].update(attachmentsByRange={"{16, 1}": {"Type": "ExtensionInput"}}),
        "get-instead-of-post": lambda x: params(x).update(WFHTTPMethod="GET"),
        "form-instead-of-json": lambda x: params(x).update(WFHTTPBodyType="Form"),
        "missing-json-body": lambda x: params(x).pop("WFJSONValues"),
        "extra-header": lambda x: params(x).update(WFHTTPHeaders={"unexpected": "value"}),
        "dropped-first-field": lambda x: params(x)["WFJSONValues"]["Value"]["WFDictionaryFieldValueItems"].pop(0),
        "wrong-item-type": lambda x: params(x)["WFJSONValues"]["Value"]["WFDictionaryFieldValueItems"][0].update(WFItemType=5),
        "lost-zero": lambda x: field(x, "zero").update(WFValue={}),
        "false-coerced-to-text": lambda x: field(x, "stream").update(WFItemType=0, WFValue={"Value": {"string": "false"}, "WFSerializationType": "WFTextTokenString"}),
        "extra-candidate-value": lambda x: params(x)["WFJSONValues"]["Value"]["WFDictionaryFieldValueItems"][-1].update(WFValue={"Value": {"string": "wrong"}, "WFSerializationType": "WFTextTokenString"}),
        "continuation-post": lambda x: params(x, second).update(WFHTTPMethod="POST"),
        "continuation-wrong-input": lambda x: params(x, second)["WFURL"]["Value"].update(attachmentsByRange={}),
        "borrowed-response-producer": lambda x: params(x).update(UUID=x[3]["WFWorkflowActionParameters"]["UUID"]),
        "unreviewed-extra-network": lambda x: x.append(copy.deepcopy(x[first])),
        "extra-nothing": lambda x: x.append({"WFWorkflowActionIdentifier": "is.workflow.actions.nothing", "WFWorkflowActionParameters": {}}),
        "clear-borrows-input": lambda x: x[2]["WFWorkflowActionParameters"].update(WFInput=""),
        "trace-current-date-lost": lambda x: x[3]["WFWorkflowActionParameters"]["WFTextActionText"]["Value"].update(attachmentsByRange={}),
        "trace-store-replaced": lambda x: x[4]["WFWorkflowActionParameters"].update(WFVariableName="__cherri_debug_log"),
        "response-edge-replaced": lambda x: x[7]["WFWorkflowActionParameters"]["WFTextActionText"]["Value"].update(attachmentsByRange={}),
        "trace-envelope-replaced": lambda x: x[12]["WFWorkflowActionParameters"]["WFTextActionText"]["Value"].update(string="wrong"),
        "final-output-replaced": lambda x: x[15]["WFWorkflowActionParameters"].update(WFOutput="success"),
    }
    require(tuple(mutations) == CAUSAL_CONTROLS, "JSON_CAUSAL_CONTROL_INVENTORY")
    rejected = []
    for name, mutation in mutations.items():
        changed = copy.deepcopy(actions)
        mutation(changed)
        try:
            verify_graph(repo, fixture, changed)
        except ValueError:
            rejected.append(name)
        else:
            raise ValueError("JSON_GRAPH_CORRUPTION_ACCEPTED:" + name)
    require(rejected == list(CAUSAL_CONTROLS), "JSON_CAUSAL_CONTROLS_INCOMPLETE")
    return mutation_receipt()


def same_json(actual, expected):
    if isinstance(expected, bool) or expected is None:
        return type(actual) is type(expected) and actual == expected
    if isinstance(expected, (int, float)):
        return type(actual) in (int, float) and math.isfinite(actual) and actual == expected
    if isinstance(expected, str):
        return type(actual) is str and actual == expected
    if isinstance(expected, list):
        return isinstance(actual, list) and len(actual) == len(expected) and all(same_json(a, b) for a, b in zip(actual, expected))
    if isinstance(expected, dict):
        return isinstance(actual, dict) and set(actual) == set(expected) and all(same_json(actual[k], v) for k, v in expected.items())
    return False


def inspect_body(data, value):
    """Publish raw bytes only when every value is the fixed public control data."""
    require(isinstance(data, bytes) and len(data) <= MAX_BODY, "BODY_SIZE")
    result = {"bytes": len(data), "sha256": sha(data), "jsonParsed": False,
              "controlsExact": False, "missingKeyAbsent": False, "candidateKind": "UNOBSERVED",
              "candidateNullObserved": False, "publicBodyBase64": None}
    try:
        parsed = read_json(data.decode("utf-8", errors="strict"))
    except (UnicodeError, ValueError):
        return result
    result["jsonParsed"] = True
    if not isinstance(parsed, dict):
        return result
    expected = value["expected_control_json"]
    candidate = value["candidate_null_field"]
    result["missingKeyAbsent"] = value["required_absent_key"] not in parsed
    actual_controls = {k: v for k, v in parsed.items() if k != candidate}
    result["controlsExact"] = same_json(actual_controls, expected)
    public_candidate = True
    if candidate is None:
        result["candidateKind"] = "NOT_AUTHORED"
    elif candidate not in parsed:
        result["candidateKind"] = "ABSENT"
    else:
        observed = parsed[candidate]
        if observed is None:
            result["candidateKind"], result["candidateNullObserved"] = "NULL", True
        elif type(observed) is str and observed == "":
            result["candidateKind"] = "EMPTY_TEXT"
        elif type(observed) is bool and observed is False:
            result["candidateKind"] = "BOOLEAN_FALSE"
        elif type(observed) in (int, float) and math.isfinite(observed) and observed == 0:
            result["candidateKind"] = "NUMBER_ZERO"
        else:
            result["candidateKind"], public_candidate = "OTHER_REDACTED", False
    if result["controlsExact"] and result["missingKeyAbsent"] and public_candidate:
        result["publicBodyBase64"] = base64.b64encode(data).decode()
    else:
        # A redacted body cannot support a positive null claim at intake. Its
        # original byte count/hash and failed control observations are retained.
        result["candidateNullObserved"] = False
        if result["candidateKind"] != "OTHER_REDACTED":
            result["candidateKind"] = "UNOBSERVED"
    return result


class BodyServer:
    def __init__(self, root, repo, fixture, nonce, commit, lifetime=MAX_LIFETIME):
        self.identity = identity(fixture, nonce, commit)
        self.contract = contract(repo, fixture)
        require(type(lifetime) is int and 1 <= lifetime <= MAX_LIFETIME, "SERVER_LIFETIME")
        self.root = Path(root) / "runtime"
        self.root.mkdir(parents=True, exist_ok=True)
        require(not self.root.is_symlink(), "SERVER_ROOT")
        self.ready = self.root / "http-probe-ready.json"
        self.path = self.root / "http-probe-receipt.json"
        self.stop_path = self.root / "http-probe.stop"
        require(not any(p.exists() for p in (self.ready, self.path, self.stop_path)), "PRIOR_PROBE_EVIDENCE_EXISTS")
        self.stop_event, self.started = threading.Event(), None
        owner = self

        class Handler(BaseHTTPRequestHandler):
            def setup(self):
                super().setup()
                self.connection.settimeout(2)

            def log_message(self, *_):
                pass

            def reply(self, status, body):
                self.send_response_only(status)
                self.send_header("Content-Type", "text/plain; charset=utf-8")
                self.send_header("Content-Length", str(len(body)))
                self.send_header("Connection", "close")
                self.end_headers()
                self.wfile.write(body)
                self.wfile.flush()

            def reject(self):
                owner.receipt["rejectedRequestCount"] += 1
                owner.persist()
                self.reply(404, b"")

            def parse_request(self):
                if not super().parse_request():
                    return False
                if self.command not in {"GET", "POST"}:
                    self.reject()
                    return False
                return True

            def do_POST(self):
                expected_path = "/" + nonce + "/" + owner.contract["case"]
                lengths = self.headers.get_all("Content-Length", [])
                content_types = self.headers.get_all("Content-Type", [])
                if (self.path != expected_path or len(owner.receipt["events"]) >= MAX_REQUESTS or
                        self.headers.get("Transfer-Encoding") is not None or len(lengths) != 1 or
                        not re.fullmatch(r"[0-9]{1,5}", lengths[0]) or int(lengths[0]) > MAX_BODY or
                        len(content_types) != 1 or not re.fullmatch(r"application/json(?:;\s*charset=utf-8)?", content_types[0], re.I)):
                    self.reject()
                    return
                try:
                    data = self.rfile.read(int(lengths[0]))
                except OSError:
                    self.reject()
                    return
                if len(data) != int(lengths[0]):
                    self.reject()
                    return
                owner.event("primary", "POST", BODY, inspect_body(data, owner.contract), self)

            def do_GET(self):
                if (self.path != "/" + nonce + "/continued" or self.headers.get("Content-Length") not in (None, "0") or
                        self.headers.get("Transfer-Encoding") is not None or len(owner.receipt["events"]) >= MAX_REQUESTS):
                    self.reject()
                    return
                owner.event("continued", "GET", CONTINUED, None, self)

        self.server = LoopbackHTTPServer(("127.0.0.1", 0), Handler)
        self.server.timeout = .2
        self.receipt = {**self.identity, "case": self.contract["case"], "boundHost": "127.0.0.1",
                        "port": self.server.server_port, "lifetimeSeconds": lifetime, "startedAt": utc(),
                        "stoppedAt": None, "serverState": "STARTING", "events": [], "rejectedRequestCount": 0}
        self.persist()

    def persist(self):
        atomic_json(self.path, self.receipt)

    def event(self, route, method, body, request, handler):
        event = {"sequence": len(self.receipt["events"]) + 1, "route": route, "method": method,
                 "receivedAt": utc(), "elapsedMilliseconds": round((time.monotonic() - self.started) * 1000),
                 "responseStatus": 200, "responseSha256": sha(body), "responseWriteCompleted": False,
                 "requestBody": request}
        self.receipt["events"].append(event)
        self.persist()
        try:
            handler.reply(200, body)
            event["responseWriteCompleted"] = True
        except OSError:
            pass
        self.persist()

    def serve(self):
        require(self.started is None, "SERVER_ALREADY_STARTED")
        self.started = time.monotonic()
        self.receipt.update(startedAt=utc(), serverState="SERVING")
        self.persist()
        atomic_json(self.ready, {**self.identity, "host": "127.0.0.1", "port": self.server.server_port})
        try:
            while not self.stop_event.is_set() and not self.stop_path.exists() and time.monotonic() - self.started < self.receipt["lifetimeSeconds"]:
                self.server.handle_request()
        finally:
            self.server.server_close()
            self.receipt.update(stoppedAt=utc(), serverState="STOPPED" if self.stop_event.is_set() or self.stop_path.exists() else "EXPIRED")
            self.persist()

    def stop(self):
        self.stop_event.set()


def ready_input(root, fixture, nonce, commit, timeout=10):
    expected = identity(fixture, nonce, commit)
    end = time.monotonic() + timeout
    runtime = Path(root) / "runtime"
    while True:
        try:
            ready = read_json((runtime / "http-probe-ready.json").read_bytes())
            state = read_json((runtime / "http-probe-receipt.json").read_bytes())
            require(all(all(x.get(k) == v for k, v in expected.items()) for x in (ready, state)), "SERVER_IDENTITY")
            require(ready.get("host") == state.get("boundHost") == "127.0.0.1" and
                    type(ready.get("port")) is int and 1 <= ready["port"] <= 65535 and ready["port"] == state.get("port"), "SERVER_BINDING")
            require(state.get("serverState") == "SERVING" and state.get("stoppedAt") is None, "SERVER_NOT_READY")
            return str(ready["port"]) + "/" + nonce
        except FileNotFoundError:
            if time.monotonic() >= end:
                raise ValueError("SERVER_NOT_READY")
            time.sleep(.05)


def validate_receipt(receipt, value, fixture, nonce, commit):
    expected = identity(fixture, nonce, commit)
    require(isinstance(receipt, dict) and set(receipt) == set(expected) | {
        "case", "boundHost", "port", "lifetimeSeconds", "startedAt", "stoppedAt", "serverState", "events", "rejectedRequestCount"}, "BODY_RECEIPT_FIELDS")
    require(all(receipt[k] == v for k, v in expected.items()) and receipt["case"] == value["case"], "BODY_RECEIPT_IDENTITY")
    require(receipt["boundHost"] == "127.0.0.1" and type(receipt["port"]) is int and 1 <= receipt["port"] <= 65535, "BODY_RECEIPT_BINDING")
    require(type(receipt["lifetimeSeconds"]) is int and 1 <= receipt["lifetimeSeconds"] <= MAX_LIFETIME and
            receipt["serverState"] in {"STOPPED", "EXPIRED"} and all(isinstance(receipt[k], str) and STAMP.fullmatch(receipt[k]) for k in ("startedAt", "stoppedAt")) and
            receipt["startedAt"] <= receipt["stoppedAt"], "BODY_RECEIPT_LIFETIME")
    require(type(receipt["rejectedRequestCount"]) is int and 0 <= receipt["rejectedRequestCount"] <= 1000 and
            isinstance(receipt["events"], list) and len(receipt["events"]) <= MAX_REQUESTS, "BODY_RECEIPT_COUNT")
    previous = -1
    for index, event in enumerate(receipt["events"], 1):
        require(isinstance(event, dict) and set(event) == {"sequence", "route", "method", "receivedAt", "elapsedMilliseconds", "responseStatus", "responseSha256", "responseWriteCompleted", "requestBody"}, "BODY_EVENT_FIELDS")
        require(type(event["sequence"]) is int and event["sequence"] == index and event["route"] in {"primary", "continued"}, "BODY_EVENT_SEQUENCE")
        primary = event["route"] == "primary"
        require(event["method"] == ("POST" if primary else "GET") and event["responseStatus"] == 200 and type(event["responseStatus"]) is int and
                event["responseSha256"] == sha(BODY if primary else CONTINUED) and type(event["responseWriteCompleted"]) is bool, "BODY_EVENT_RESPONSE")
        elapsed = event["elapsedMilliseconds"]
        require(type(elapsed) is int and 0 <= elapsed and previous <= elapsed <= (receipt["lifetimeSeconds"] + 3) * 1000 and
                isinstance(event["receivedAt"], str) and STAMP.fullmatch(event["receivedAt"]) and receipt["startedAt"] <= event["receivedAt"] <= receipt["stoppedAt"], "BODY_EVENT_TIME")
        previous = elapsed
        body = event["requestBody"]
        if not primary:
            require(body is None, "BODY_ON_CONTINUATION")
            continue
        require(isinstance(body, dict) and set(body) == {"bytes", "sha256", "jsonParsed", "controlsExact", "missingKeyAbsent", "candidateKind", "candidateNullObserved", "publicBodyBase64"}, "BODY_FIELDS")
        require(type(body["bytes"]) is int and 0 <= body["bytes"] <= MAX_BODY and isinstance(body["sha256"], str) and HASH.fullmatch(body["sha256"]) and
                all(type(body[k]) is bool for k in ("jsonParsed", "controlsExact", "missingKeyAbsent", "candidateNullObserved")), "BODY_VALUES")
        require(body["candidateKind"] in {"UNOBSERVED", "NOT_AUTHORED", "ABSENT", "NULL", "EMPTY_TEXT", "BOOLEAN_FALSE", "NUMBER_ZERO", "OTHER_REDACTED"}, "BODY_CANDIDATE_KIND")
        if body["publicBodyBase64"] is not None:
            require(isinstance(body["publicBodyBase64"], str) and len(body["publicBodyBase64"]) <= MAX_BODY * 2, "BODY_BASE64_SIZE")
            try:
                raw = base64.b64decode(body["publicBodyBase64"], validate=True)
            except ValueError:
                raise ValueError("BODY_BASE64_INVALID")
            require(inspect_body(raw, value) == body, "BODY_OBSERVATION_CHANGED")
        else:
            require(not body["candidateNullObserved"] and not (body["controlsExact"] and body["missingKeyAbsent"] and body["candidateKind"] != "OTHER_REDACTED"), "BODY_PROOF_BYTES_MISSING")
    return receipt


def observation_status(receipt, runtime_ok):
    events = receipt["events"]
    exact_order = [x["route"] for x in events] == ["primary", "continued"]
    controlled = bool(exact_order and receipt["serverState"] == "STOPPED" and receipt["rejectedRequestCount"] == 0 and
                      all(x["responseWriteCompleted"] and x["elapsedMilliseconds"] <= receipt["lifetimeSeconds"] * 1000 for x in events) and events[0]["requestBody"]["controlsExact"] and
                      events[0]["requestBody"]["missingKeyAbsent"] and events[0]["requestBody"]["publicBodyBase64"] is not None)
    if controlled and runtime_ok:
        return "CONTROLLED_BODY_OBSERVED", True
    if not events:
        return "NO_REQUEST_OBSERVED", False
    return "OBSERVATION_INCOMPLETE", False


def validate_public_diagnostic(report, manifest, assertions, callback_bytes, repo=None):
    repo = repo or Path(__file__).resolve().parents[2]
    require(isinstance(report, dict) and set(report) == {"schemaVersion", "fixtureId", "runNonce", "compilerSha", "sourceSha256", "expectedContractSha256", "unsignedPlistSha256", "callbackEventsSha256", "observedCallbackStatus", "runtimeContractVerified", "receipt", "observationStatus", "controlledBodyVerified", "candidateNullObserved", "formalAcceptanceGranted", "scope"}, "JSON_REPORT_FIELDS")
    fixture, nonce, commit = report["fixtureId"], report["runNonce"], report["compilerSha"]
    identity(fixture, nonce, commit)
    value = contract(repo, fixture)
    require(report["schemaVersion"] == "1" and report["sourceSha256"] == FIXTURES[fixture][1] and
            report["expectedContractSha256"] == FIXTURES[fixture][2] and report["formalAcceptanceGranted"] is False and
            report["scope"] == "FIXED_PUBLIC_HTTP_BODY_OBSERVATION_ONLY", "JSON_REPORT_SCOPE")
    require(manifest.get("syntheticOnly") is True and manifest.get("compilerCommitSha") == commit, "JSON_REPORT_MANIFEST")
    rows = [x for x in manifest.get("fixtures", []) if x.get("fixtureId") == fixture]
    require(len(rows) == 1 and rows[0].get("sourceSha256") == report["sourceSha256"] and
            rows[0].get("unsignedPlistSha256") == report["unsignedPlistSha256"] and isinstance(report["unsignedPlistSha256"], str) and
            HASH.fullmatch(report["unsignedPlistSha256"]) and rows[0].get("jsonBodySource") == verify_source(repo, fixture, (Path(repo) / "tools/shortcuts-simulator-qa/fixtures" / (fixture + ".cherri")).read_bytes()), "JSON_REPORT_SOURCE_BINDING")
    require(rows[0].get("jsonBodyGraph") == graph_receipt(value) and
            rows[0].get("jsonBodyCausalMutations") == mutation_receipt(), "JSON_REPORT_GRAPH_BINDING")
    require(assertions.get("targetFixtureId") == fixture and assertions.get("expectedRunId") == nonce and
            assertions.get("compilerCommitSha") == commit and assertions.get("compiledSourceSha256") == report["sourceSha256"] and
            assertions.get("unsignedPlistSha256") == report["unsignedPlistSha256"] and
            assertions.get("expectedSemanticOutput") == value["expected_callback_prefix"] + BODY.decode() and
            type(assertions.get("runtimeContractVerified")) is bool and type(report["runtimeContractVerified"]) is bool and
            report["runtimeContractVerified"] == assertions["runtimeContractVerified"], "JSON_REPORT_ASSERTIONS")
    require(isinstance(callback_bytes, bytes) and report["callbackEventsSha256"] == sha(callback_bytes), "JSON_CALLBACK_BYTES")
    callbacks = read_json(callback_bytes)
    require(isinstance(callbacks, list) and all(isinstance(x, dict) for x in callbacks), "JSON_CALLBACK_SHAPE")
    observed = [x for x in callbacks if not x.get("receiverSelfTest") and x.get("runId") == nonce and x.get("fixtureId") == fixture]
    require(len(observed) <= 1, "JSON_MULTIPLE_CALLBACKS")
    status = observed[0].get("status") if observed else "MISSING"
    require(status in {"SUCCESS", "ERROR", "CANCELLED", "MISSING"} and report["observedCallbackStatus"] == status, "JSON_CALLBACK_STATUS")
    if observed:
        require(status in {"SUCCESS", "ERROR", "CANCELLED"} and
                observed[0].get("expectedShortcutName") == fixture and observed[0].get("eventSource") == "shortcuts-x-callback" and
                observed[0].get("host") == {"SUCCESS": "success", "ERROR": "error", "CANCELLED": "cancel"}[status], "JSON_CALLBACK_ORIGIN")
    if report["runtimeContractVerified"]:
        require(status == "SUCCESS" and assertions.get("compiledShortcutCallbackVerified") is True and
                isinstance(observed[0].get("resultSummary"), str) and
                observed[0]["resultSummary"].splitlines()[0] == value["expected_callback_prefix"] + BODY.decode(), "JSON_RUNTIME_FALSE_PASS")
    receipt = validate_receipt(report["receipt"], value, fixture, nonce, commit)
    result, controlled = observation_status(receipt, report["runtimeContractVerified"])
    candidate = bool(controlled and value["candidate_null_field"] is not None and receipt["events"][0]["requestBody"]["candidateNullObserved"])
    require(report["observationStatus"] == result and type(report["controlledBodyVerified"]) is bool and report["controlledBodyVerified"] == controlled and
            type(report["candidateNullObserved"]) is bool and report["candidateNullObserved"] == candidate, "JSON_REPORT_FALSE_OBSERVATION")
    return report


def diagnose(root, repo, fixture, nonce, commit):
    root = Path(root)
    value = contract(repo, fixture)
    receipt = read_json((root / "runtime/http-probe-receipt.json").read_bytes())
    validate_receipt(receipt, value, fixture, nonce, commit)
    manifest = read_json((root / "compiler-manifest.json").read_bytes())
    assertions = read_json((root / "runtime/output-assertions.json").read_bytes())
    callbacks = (root / "runtime/callback-events.json").read_bytes()
    parsed_callbacks = read_json(callbacks)
    require(isinstance(parsed_callbacks, list) and all(isinstance(x, dict) for x in parsed_callbacks), "JSON_CALLBACK_SHAPE")
    entries = [x for x in parsed_callbacks if not x.get("receiverSelfTest") and x.get("runId") == nonce and x.get("fixtureId") == fixture]
    require(len(entries) <= 1, "JSON_MULTIPLE_CALLBACKS")
    runtime = assertions.get("runtimeContractVerified") is True
    result, controlled = observation_status(receipt, runtime)
    report = {**identity(fixture, nonce, commit), "sourceSha256": FIXTURES[fixture][1], "expectedContractSha256": FIXTURES[fixture][2],
              "unsignedPlistSha256": assertions.get("unsignedPlistSha256"), "callbackEventsSha256": sha(callbacks),
              "observedCallbackStatus": entries[0].get("status") if entries else "MISSING", "runtimeContractVerified": runtime,
              "receipt": receipt, "observationStatus": result, "controlledBodyVerified": controlled,
              "candidateNullObserved": bool(controlled and value["candidate_null_field"] is not None and receipt["events"][0]["requestBody"]["candidateNullObserved"]),
              "formalAcceptanceGranted": False, "scope": "FIXED_PUBLIC_HTTP_BODY_OBSERVATION_ONLY"}
    validate_public_diagnostic(report, manifest, assertions, callbacks, repo)
    atomic_json(root / "runtime/json-body-diagnostic.json", report)
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("operation", choices=("serve", "input", "diagnose", "startup-report"))
    parser.add_argument("--root", required=True, type=Path)
    parser.add_argument("--fixture", required=True)
    parser.add_argument("--nonce", required=True)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--fixtures-dir", type=Path)
    args = parser.parse_args()
    repo = Path(__file__).resolve().parents[2]
    identity(args.fixture, args.nonce, args.commit)
    if args.operation == "serve":
        server = BodyServer(args.root, repo, args.fixture, args.nonce, args.commit)
        for sig in (signal.SIGTERM, signal.SIGINT):
            signal.signal(sig, lambda *_: server.stop())
        server.serve()
    elif args.operation == "input":
        print(ready_input(args.root, args.fixture, args.nonce, args.commit))
    elif args.operation == "startup-report":
        print(json.dumps({**identity(args.fixture, args.nonce, args.commit), "status": "SERVER_NOT_READY",
                          "nativeOutcome": "NOT_OBSERVED", "formalAcceptanceGranted": False}))
    else:
        report = diagnose(args.root, repo, args.fixture, args.nonce, args.commit)
        print(json.dumps({"jsonBodyObservation": report["observationStatus"], "controlledBodyVerified": report["controlledBodyVerified"],
                          "candidateNullObserved": report["candidateNullObserved"], "formalAcceptanceGranted": False}))
        return 0 if report["controlledBodyVerified"] else 2
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, ValueError, KeyError, TypeError):
        print("JSON_BODY_PROBE_FAILED")
        raise SystemExit(2)
