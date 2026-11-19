#!/usr/bin/env python3
"""Bounded loopback HTTP observations for native Shortcuts feasibility probes.

This is a test endpoint and evidence validator, not a retry implementation.
It never turns an ERROR/CANCELLED callback into a passing runtime/trace gate.
"""
import argparse
from datetime import datetime, timezone
import hashlib
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
from pathlib import Path
import re
import signal
import socket
from socketserver import TCPServer
import threading
import time

CASES = {"control": 200, "http500": 500, "http404": 404, "http429": 429, "transport": None}
FIXTURES = {"qa-retry-" + case + "-v2": case for case in CASES}
BODY = b"P15_HTTP_BODY_OK"
CONTINUED_BODY = b"P15_HTTP_CONTINUED_OK"
MAX_REQUESTS = 16
MAX_LIFETIME_SECONDS = 1800
LIMITATIONS = ["A missing continuation receipt does not identify the failing native action by itself.",
               "Apple transport or harness execution may repeat a GET; request counts do not prove compiler retries.",
               "ERROR/CANCELLED remain ordinary runtime/trace failures."]
SHA256 = re.compile(r"^[0-9a-f]{64}$")
COMMIT = re.compile(r"^[0-9a-f]{40}$")
UTC_STAMP = re.compile(r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z$")


class ProbeError(ValueError):
    pass


def sha(data):
    return hashlib.sha256(data).hexdigest()


def utc():
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%fZ")


def require(condition, code):
    if not condition:
        raise ProbeError(code)


def validate_identity(fixture, nonce, commit):
    require(fixture in FIXTURES, "UNREVIEWED_FIXTURE")
    require(isinstance(nonce, str) and len(nonce) <= 128
            and re.fullmatch(re.escape(fixture) + r"-[A-Za-z0-9_-]{1,50}-[0-9a-f]{32}", nonce),
            "INVALID_FRESH_NONCE")
    require(isinstance(commit, str) and COMMIT.fullmatch(commit), "INVALID_COMMIT")


def atomic_json(path, data):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")
    temporary.replace(path)


def expected_marker(fixture):
    return "CHERRI_QA_RETRY_" + FIXTURES[fixture].upper() + "_RESULT:" + BODY.decode()


def token_text(value):
    require(isinstance(value, dict) and value.get("WFSerializationType") == "WFTextTokenString",
            "EXPECTED_NATIVE_TOKEN_STRING")
    value = value.get("Value")
    require(isinstance(value, dict) and isinstance(value.get("string"), str)
            and isinstance(value.get("attachmentsByRange"), dict), "INVALID_NATIVE_TOKEN_STRING")
    return value["string"], list(value["attachmentsByRange"].values())


def validate_fixture_graph(fixture, actions):
    """Inspect actual native HTTP actions and producer edges, not source text."""
    if fixture not in FIXTURES:
        return None
    case = FIXTURES[fixture]
    allowed = {"comment", "nothing", "gettext", "setvariable", "appendvariable", "downloadurl", "text.combine", "output"}
    require(isinstance(actions, list) and actions
            and all(isinstance(a, dict) and a.get("WFWorkflowActionIdentifier") in
                    {"is.workflow.actions." + name for name in allowed} for a in actions),
            "UNEXPECTED_ACTION_OR_RETRY_CONTROL_FLOW")
    clears = [(i, a.get("WFWorkflowActionParameters", {})) for i, a in enumerate(actions)
              if a.get("WFWorkflowActionIdentifier") == "is.workflow.actions.setvariable"
              and a.get("WFWorkflowActionParameters", {}).get("WFVariableName") == "__cherri_debug_log"]
    require(len(clears) == 1 and clears[0][0] > 0, "DEBUG_CLEAR_COUNT")
    clear_index, clear = clears[0]
    before = actions[clear_index - 1]
    require(before.get("WFWorkflowActionIdentifier") == "is.workflow.actions.nothing"
            and "WFInput" not in before.get("WFWorkflowActionParameters", {})
            and "WFInput" not in clear, "DEBUG_CLEAR_MUST_HAVE_ZERO_ITEMS")
    require(sum(a.get("WFWorkflowActionIdentifier") == "is.workflow.actions.nothing" for a in actions) == 1,
            "UNEXPECTED_NOTHING_ACTION")
    http = [(i, a.get("WFWorkflowActionParameters", {})) for i, a in enumerate(actions)
            if a.get("WFWorkflowActionIdentifier") == "is.workflow.actions.downloadurl"]
    require(len(http) == 2, "HTTP_OPERATION_OR_OBSERVER_COUNT")
    for (_, params), route in zip(http, (case, "continued")):
        require(params.get("WFHTTPMethod") == "GET", "PROBE_MUST_BE_GET")
        require(set(params) <= {"UUID", "CustomOutputName", "WFHTTPMethod", "WFURL", "WFHTTPHeaders"},
                "UNEXPECTED_HTTP_PARAMETER")
        require(params.get("WFHTTPHeaders") in (None, {}), "UNEXPECTED_HTTP_HEADERS")
        require(not any(key in params for key in ("WFJSONValues", "WFRequestVariable", "WFFormValues")),
                "UNEXPECTED_HTTP_BODY")
        value, attachments = token_text(params.get("WFURL"))
        require(value == "http://127.0.0.1:\ufffc/" + route, "NON_LOOPBACK_OR_WRONG_ROUTE")
        require(len(attachments) == 1 and attachments[0] == {"Type": "ExtensionInput"}
                and params["WFURL"]["Value"]["attachmentsByRange"] == {"{17, 1}": {"Type": "ExtensionInput"}},
                "HTTP_INPUT_MUST_BE_SHORTCUT_INPUT")
    primary_uuid = http[0][1].get("UUID")
    observer_uuid = http[1][1].get("UUID")
    require(isinstance(primary_uuid, str) and primary_uuid and observer_uuid != primary_uuid,
            "HTTP_PRODUCER_IDENTITY")
    marker = expected_marker(fixture).split(":", 1)[0] + ":\ufffc"
    summaries = []
    for index, action in enumerate(actions):
        if action.get("WFWorkflowActionIdentifier") != "is.workflow.actions.gettext":
            continue
        try:
            value, attachments = token_text(action.get("WFWorkflowActionParameters", {}).get("WFTextActionText"))
        except ProbeError:
            continue
        if value == marker:
            summaries.append(index)
            require(index > http[1][0] and len(attachments) == 1
                    and attachments[0].get("Type") == "ActionOutput"
                    and attachments[0].get("OutputUUID") == primary_uuid,
                    "SUMMARY_MUST_USE_PRIMARY_HTTP_PRODUCER")
    require(len(summaries) == 1, "HTTP_SUMMARY_COUNT")
    return {"verified": True, "httpActionCount": 2, "method": "GET",
            "primaryIndex": http[0][0], "continuationIndex": http[1][0],
            "summaryIndex": summaries[0], "primaryUUIDDigest": sha(primary_uuid.encode()),
            "continuationUUIDDigest": sha(observer_uuid.encode()) if isinstance(observer_uuid, str) else None,
            "retryActionCount": 0, "nativeErrorCatchAdded": False}


class LoopbackHTTPServer(HTTPServer):
    def server_bind(self):
        # HTTPServer normally reverse-resolves its host name while binding.
        # The reviewed endpoint is always numeric loopback; no DNS is needed.
        TCPServer.server_bind(self)
        self.server_name = "127.0.0.1"
        self.server_port = self.server_address[1]


STARTUP_PHASES = {"BINDING_LOOPBACK", "BOUND_LOOPBACK", "SERVING", "STOPPED", "EXPIRED"}
STARTUP_ERROR_CODES = {"UNREVIEWED_FIXTURE", "INVALID_FRESH_NONCE", "INVALID_COMMIT",
                       "INVALID_SERVER_LIFETIME", "PRIOR_PROBE_EVIDENCE_EXISTS",
                       "SERVER_ALREADY_STARTED", "SERVER_BINDING_MISMATCH",
                       "INVALID_LOOPBACK_PORT", "SERVER_NOT_READY"}
STARTUP_FAILURE_FIELDS = {"schemaVersion", "command", "status", "errorCategory", "errorCode", "osErrorNumber"}


def startup_failure(command, error):
    category = ("CONTRACT" if isinstance(error, ProbeError) else
                "OS" if isinstance(error, OSError) else "STRUCTURE")
    number = error.errno if isinstance(error, OSError) else None
    if type(number) is not int or not -4096 <= number <= 65535:
        number = None
    code = str(error) if isinstance(error, ProbeError) and str(error) in STARTUP_ERROR_CODES else "REDACTED"
    return {"schemaVersion": "1.0", "command": command, "status": "FAILED",
            "errorCategory": category, "errorCode": code, "osErrorNumber": number}


def startup_report(root, fixture, nonce, commit):
    """Print only reviewed startup fields, never raw server stdout/errors."""
    validate_identity(fixture, nonce, commit)
    runtime = Path(root) / "runtime"
    result = {"schemaVersion": "1.0", "kind": "HTTP_PROBE_STARTUP", "fixtureId": fixture,
              "runNonce": nonce, "compilerSha": commit, "nativeOutcome": "NOT_OBSERVED",
              "lastObservedServerPhase": None, "readyFilePresent": (runtime / "http-probe-ready.json").exists(),
              "stopFilePresent": (runtime / "http-probe.stop").exists(),
              "serveFailure": None, "inputFailure": None}
    state_path = runtime / "http-probe-startup-state.json"
    if state_path.exists():
        state = json.loads(state_path.read_text(encoding="utf-8"))
        require(set(state) == {"schemaVersion", "fixtureId", "runNonce", "compilerSha", "phase"}
                and state["schemaVersion"] == "1.0" and state["fixtureId"] == fixture
                and state["runNonce"] == nonce and state["compilerSha"] == commit
                and state["phase"] in STARTUP_PHASES, "SERVER_BINDING_MISMATCH")
        result["lastObservedServerPhase"] = state["phase"]
    for command in ("serve", "input"):
        path = runtime / ("http-probe-startup-" + command + ".json")
        if not path.exists():
            continue
        failure = json.loads(path.read_text(encoding="utf-8"))
        require(set(failure) == STARTUP_FAILURE_FIELDS and failure["schemaVersion"] == "1.0"
                and failure["command"] == command and failure["status"] == "FAILED"
                and failure["errorCategory"] in {"CONTRACT", "OS", "STRUCTURE"}
                and failure["errorCode"] in STARTUP_ERROR_CODES | {"REDACTED"}
                and (failure["osErrorNumber"] is None or
                     type(failure["osErrorNumber"]) is int and -4096 <= failure["osErrorNumber"] <= 65535),
                "SERVER_BINDING_MISMATCH")
        result[command + "Failure"] = failure
    return result


class ProbeServer:
    def __init__(self, root, fixture, nonce, commit, lifetime=MAX_LIFETIME_SECONDS):
        validate_identity(fixture, nonce, commit)
        require(1 <= lifetime <= MAX_LIFETIME_SECONDS, "INVALID_SERVER_LIFETIME")
        self.root = Path(root)
        self.root.mkdir(parents=True, exist_ok=True)
        self.ready_path = self.root / "http-probe-ready.json"
        self.receipt_path = self.root / "http-probe-receipt.json"
        self.stop_path = self.root / "http-probe.stop"
        self.startup_path = self.root / "http-probe-startup-state.json"
        require(not any(p.exists() for p in (self.ready_path, self.receipt_path, self.stop_path, self.startup_path)),
                "PRIOR_PROBE_EVIDENCE_EXISTS")
        self.started = None
        self.lifetime = lifetime
        self.stop_event = threading.Event()
        owner = self

        class Handler(BaseHTTPRequestHandler):
            def setup(self):
                super().setup()
                self.connection.settimeout(2)

            def log_message(self, *_):
                pass

            def reject(self):
                owner.receipt["rejectedRequestCount"] += 1
                owner.persist()
                self.send_response_only(404)
                self.send_header("Content-Length", "0")
                self.send_header("Connection", "close")
                self.end_headers()

            def parse_request(self):
                if not super().parse_request():
                    return False
                # BaseHTTPRequestHandler's default 501 path otherwise bypasses
                # our receipt. Every parsed non-GET request invalidates the run,
                # including methods for which no do_* handler exists.
                if self.command != "GET":
                    self.reject()
                    return False
                return True

            def do_GET(self):
                routes = {"/" + nonce + "/" + FIXTURES[fixture]: "primary",
                          "/" + nonce + "/continued": "continued"}
                route = routes.get(self.path)
                if route is None or len(owner.receipt["events"]) >= MAX_REQUESTS:
                    self.reject()
                    return
                status = CASES[FIXTURES[fixture]] if route == "primary" else 200
                body = BODY if route == "primary" else CONTINUED_BODY
                event = {"sequence": len(owner.receipt["events"]) + 1, "route": route,
                         "method": "GET", "status": status,
                         "responseKind": "CONNECTION_CLOSED" if status is None else "HTTP_STATUS",
                         "bodySha256": None if status is None else sha(body),
                         "receivedAt": utc(),
                         "elapsedMilliseconds": round((time.monotonic() - owner.started) * 1000),
                         "responseWriteCompleted": False}
                owner.receipt["events"].append(event)
                owner.persist()
                try:
                    if status is None:
                        self.close_connection = True
                        self.connection.shutdown(socket.SHUT_RDWR)
                        self.connection.close()
                    else:
                        self.send_response_only(status)
                        self.send_header("Content-Type", "text/plain; charset=utf-8")
                        self.send_header("Content-Length", str(len(body)))
                        self.send_header("Connection", "close")
                        self.end_headers()
                        self.wfile.write(body)
                        self.wfile.flush()
                    event["responseWriteCompleted"] = True
                except OSError:
                    event["responseWriteCompleted"] = False
                owner.persist()

        self.startup_identity = {"schemaVersion": "1.0", "fixtureId": fixture,
                                 "runNonce": nonce, "compilerSha": commit}
        self.startup_phase("BINDING_LOOPBACK")
        self.server = LoopbackHTTPServer(("127.0.0.1", 0), Handler)
        self.startup_phase("BOUND_LOOPBACK")
        self.server.timeout = 0.2
        self.receipt = {"schemaVersion": "1.0", "fixtureId": fixture, "runNonce": nonce,
                        "compilerSha": commit, "case": FIXTURES[fixture], "boundHost": "127.0.0.1",
                        "port": self.server.server_port, "startedAt": utc(),
                        "lifetimeSeconds": lifetime, "serverState": "STARTING", "stoppedAt": None,
                        "events": [], "rejectedRequestCount": 0}
        self.persist()

    def startup_phase(self, phase):
        require(phase in STARTUP_PHASES, "SERVER_BINDING_MISMATCH")
        atomic_json(self.startup_path, {**self.startup_identity, "phase": phase})

    def persist(self):
        atomic_json(self.receipt_path, self.receipt)

    def serve(self):
        require(self.started is None, "SERVER_ALREADY_STARTED")
        self.started = time.monotonic()
        self.receipt["startedAt"] = utc()
        self.receipt["serverState"] = "SERVING"
        try:
            # Readiness belongs to the running service, not construction. A slow
            # bind/setup must not consume the finite active lifetime and leave
            # a stale ready file for a socket already closed by expiry.
            self.persist()
            self.startup_phase("SERVING")
            atomic_json(self.ready_path, {"fixtureId": self.receipt["fixtureId"],
                        "runNonce": self.receipt["runNonce"], "compilerSha": self.receipt["compilerSha"],
                        "host": "127.0.0.1", "port": self.server.server_port})
            while (not self.stop_event.is_set() and not self.stop_path.exists()
                   and time.monotonic() - self.started < self.lifetime):
                self.server.handle_request()
        finally:
            self.server.server_close()
            self.receipt["serverState"] = "STOPPED" if self.stop_event.is_set() or self.stop_path.exists() else "EXPIRED"
            self.receipt["stoppedAt"] = utc()
            self.persist()
            self.startup_phase(self.receipt["serverState"])

    def stop(self):
        self.stop_event.set()


def ready_input(path, fixture, nonce, commit, timeout=10):
    validate_identity(fixture, nonce, commit)
    deadline = time.monotonic() + timeout
    while True:
        try:
            ready_path = Path(path)
            ready = json.loads(ready_path.read_text(encoding="utf-8"))
            require(isinstance(ready, dict) and ready.get("fixtureId") == fixture
                    and ready.get("runNonce") == nonce and ready.get("compilerSha") == commit
                    and ready.get("host") == "127.0.0.1", "SERVER_BINDING_MISMATCH")
            port = ready.get("port")
            require(type(port) is int and 1 <= port <= 65535, "INVALID_LOOPBACK_PORT")
            # Keep the historical ready file, but do not treat it as current
            # service state after a recorded shutdown or expiry. These state
            # files are published before readiness by the serving loop itself.
            state = json.loads((ready_path.parent / "http-probe-startup-state.json").read_text(encoding="utf-8"))
            receipt = json.loads((ready_path.parent / "http-probe-receipt.json").read_text(encoding="utf-8"))
            for record in (state, receipt):
                require(isinstance(record, dict) and record.get("schemaVersion") == "1.0"
                        and record.get("fixtureId") == fixture and record.get("runNonce") == nonce
                        and record.get("compilerSha") == commit, "SERVER_BINDING_MISMATCH")
            require(state.get("phase") in STARTUP_PHASES
                    and receipt.get("case") == FIXTURES[fixture]
                    and receipt.get("boundHost") == "127.0.0.1"
                    and type(receipt.get("port")) is int and receipt["port"] == port,
                    "SERVER_BINDING_MISMATCH")
            require(state["phase"] == "SERVING" and receipt.get("serverState") == "SERVING"
                    and receipt.get("stoppedAt") is None, "SERVER_NOT_READY")
            return str(port) + "/" + nonce
        except FileNotFoundError:
            if time.monotonic() >= deadline:
                raise ProbeError("SERVER_NOT_READY")
            time.sleep(0.05)


def validate_receipt(receipt, fixture, nonce, commit):
    validate_identity(fixture, nonce, commit)
    require(receipt.get("schemaVersion") == "1.0" and receipt.get("fixtureId") == fixture
            and receipt.get("runNonce") == nonce and receipt.get("compilerSha") == commit
            and receipt.get("case") == FIXTURES[fixture], "SERVER_BINDING_MISMATCH")
    require(receipt.get("boundHost") == "127.0.0.1" and type(receipt.get("port")) is int
            and 1 <= receipt["port"] <= 65535, "INVALID_LOOPBACK_BINDING")
    require(receipt.get("serverState") == "STOPPED", "SERVER_NOT_STOPPED_CLEANLY")
    require(all(isinstance(receipt.get(key), str) and UTC_STAMP.fullmatch(receipt[key])
                for key in ("startedAt", "stoppedAt")), "INVALID_SERVER_TIMESTAMP")
    events = receipt.get("events")
    require(isinstance(events, list) and len(events) <= MAX_REQUESTS, "INVALID_REQUEST_COUNT")
    require(type(receipt.get("rejectedRequestCount")) is int and receipt["rejectedRequestCount"] == 0,
            "UNREVIEWED_REQUEST_OBSERVED")
    require(type(receipt.get("lifetimeSeconds")) is int and 1 <= receipt["lifetimeSeconds"] <= MAX_LIFETIME_SECONDS,
            "INVALID_SERVER_LIFETIME")
    require(receipt["startedAt"] <= receipt["stoppedAt"], "INVALID_SERVER_TIMESTAMP_ORDER")
    previous_time = -1
    sanitized = []
    for index, event in enumerate(events):
        require(isinstance(event, dict) and type(event.get("sequence")) is int and event["sequence"] == index + 1
                and event.get("method") == "GET" and event.get("route") in ("primary", "continued"),
                "INVALID_REQUEST_EVENT")
        status = CASES[FIXTURES[fixture]] if event["route"] == "primary" else 200
        body = BODY if event["route"] == "primary" else CONTINUED_BODY
        require(event.get("status") == status
                and event.get("bodySha256") == (None if status is None else sha(body))
                and event.get("responseKind") == ("CONNECTION_CLOSED" if status is None else "HTTP_STATUS"),
                "RESPONSE_CONTRACT_MISMATCH")
        elapsed = event.get("elapsedMilliseconds")
        require(type(elapsed) is int and 0 <= elapsed and previous_time <= elapsed <= (MAX_LIFETIME_SECONDS + 3) * 1000
                and isinstance(event.get("receivedAt"), str) and UTC_STAMP.fullmatch(event["receivedAt"])
                and receipt["startedAt"] <= event["receivedAt"] <= receipt["stoppedAt"]
                and type(event.get("responseWriteCompleted")) is bool, "INVALID_EVENT_TIMESTAMP_OR_WRITE")
        previous_time = elapsed
        sanitized.append({key: event[key] for key in ("sequence", "route", "method", "status", "responseKind",
                          "bodySha256", "receivedAt", "elapsedMilliseconds", "responseWriteCompleted")})
    return sanitized


def classify_observation(fixture, events, callback_status, runtime):
    primary = [e for e in events if e["route"] == "primary"]
    continued = [e for e in events if e["route"] == "continued"]
    status = "UNVERIFIED"
    if not primary:
        status = "NO_PRIMARY_REQUEST_OBSERVED"
    elif len(primary) != 1 or len(continued) > 1:
        status = "MULTIPLE_REQUESTS_OBSERVED"
    elif continued and [e["route"] for e in events] != ["primary", "continued"]:
        status = "CONTINUATION_ORDER_REJECTED"
    elif not all(e["responseWriteCompleted"] for e in events):
        status = "RESPONSE_WRITE_INCOMPLETE"
    elif continued and runtime:
        status = "CONTROL_VERIFIED" if FIXTURES[fixture] == "control" else "HTTP_RESPONSE_CONTINUED"
    elif continued:
        status = "CONTINUATION_OBSERVED_OUTPUT_UNVERIFIED"
    elif callback_status == "ERROR":
        status = "TERMINATED_WITHOUT_CONTINUATION_RECEIPT"
    elif callback_status == "CANCELLED":
        status = "CANCELLED_WITHOUT_CONTINUATION_RECEIPT"
    verified = status in ("CONTROL_VERIFIED", "HTTP_RESPONSE_CONTINUED",
                          "TERMINATED_WITHOUT_CONTINUATION_RECEIPT", "CANCELLED_WITHOUT_CONTINUATION_RECEIPT")
    if FIXTURES[fixture] == "control" and status != "CONTROL_VERIFIED":
        verified = False
    return status, verified


def diagnose(root, fixtures_dir, fixture, nonce, commit):
    root, fixtures_dir = Path(root), Path(fixtures_dir)
    validate_identity(fixture, nonce, commit)
    receipt_bytes = (root / "runtime/http-probe-receipt.json").read_bytes()
    receipt = json.loads(receipt_bytes)
    events = validate_receipt(receipt, fixture, nonce, commit)
    manifest = json.loads((root / "compiler-manifest.json").read_text(encoding="utf-8"))
    matches = [f for f in manifest.get("fixtures", []) if f.get("fixtureId") == fixture]
    require(manifest.get("syntheticOnly") is True and manifest.get("compilerCommitSha") == commit
            and len(matches) == 1, "COMPILER_MANIFEST_BINDING")
    row = matches[0]
    source = fixtures_dir / (fixture + ".cherri")
    graph = row.get("httpProbeGraph")
    require(isinstance(graph, dict) and graph.get("verified") is True
            and graph.get("httpActionCount") == 2 and graph.get("method") == "GET"
            and graph.get("retryActionCount") == 0 and graph.get("nativeErrorCatchAdded") is False,
            "HTTP_GRAPH_CONTRACT_MISSING")
    require(not source.is_symlink() and row.get("fileName") == source.name
            and row.get("sourceSha256") == sha(source.read_bytes())
            and SHA256.fullmatch(row.get("unsignedPlistSha256", ""))
            and row.get("buildProfile") == "debug" and row.get("runtimeTraceExpectation") == "REQUIRED_NDJSON"
            and graph.get("verified") is True, "COMPILED_SOURCE_OR_GRAPH_BINDING")
    assertions = json.loads((root / "runtime/output-assertions.json").read_text(encoding="utf-8"))
    require(assertions.get("targetFixtureId") == fixture and assertions.get("expectedRunId") == nonce
            and assertions.get("compilerCommitSha") == commit
            and assertions.get("compiledSourceSha256") == row["sourceSha256"]
            and assertions.get("unsignedPlistSha256") == row["unsignedPlistSha256"]
            and assertions.get("buildProfile") == "debug"
            and assertions.get("traceExpectation") == "REQUIRED_NDJSON"
            and assertions.get("expectedSemanticOutput") == expected_marker(fixture),
            "RUNTIME_ASSERTION_BINDING")
    callbacks = json.loads((root / "runtime/callback-events.json").read_text(encoding="utf-8"))
    require(isinstance(callbacks, list) and all(isinstance(c, dict) for c in callbacks),
            "INVALID_CALLBACK_STORE")
    callbacks = [c for c in callbacks if not c.get("receiverSelfTest") and c.get("runId") == nonce]
    require(len(callbacks) == 1, "CORRELATED_CALLBACK_MISSING_OR_DUPLICATE")
    callback = callbacks[0]
    require(callback.get("receiverSelfTest") is False
            and callback.get("eventSource") == "shortcuts-x-callback"
            and callback.get("fixtureId") == fixture and callback.get("expectedShortcutName") == fixture,
            "CALLBACK_IDENTITY_MISMATCH")
    require(callback.get("status") in ("SUCCESS", "ERROR", "CANCELLED"), "UNKNOWN_CALLBACK_STATUS")
    if callback["status"] != "SUCCESS":
        require(assertions.get("runtimeContractVerified") is False
                and assertions.get("shortcutRunCompletedGate") == "FAIL_RUNTIME"
                and assertions.get("traceCapturedGate") == "FAIL_RUNTIME",
                "ERROR_CALLBACK_CANNOT_PASS_ORDINARY_RUNTIME")
    primary = [e for e in events if e["route"] == "primary"]
    continued = [e for e in events if e["route"] == "continued"]
    runtime = (assertions.get("runtimeContractVerified") is True
               and assertions.get("shortcutRunCompletedGate") == "PASS"
               and assertions.get("traceCapturedGate") == "PASS"
               and assertions.get("compiledSourceSha256") == row["sourceSha256"]
               and assertions.get("unsignedPlistSha256") == row["unsignedPlistSha256"]
               and callback.get("status") == "SUCCESS"
               and callback.get("resultSummary") == expected_marker(fixture))
    status, verified = classify_observation(fixture, events, callback["status"], runtime)
    error_code = callback.get("errorCode")
    report = {"schemaVersion": "1.0", "diagnosticStatus": status, "diagnosticObservationComplete": verified,
              "fixtureId": fixture, "runNonce": nonce, "compilerSha": commit,
              "sourceSha256": row["sourceSha256"], "unsignedPlistSha256": row["unsignedPlistSha256"],
              "serverReceiptSha256": sha(receipt_bytes), "serverState": receipt["serverState"],
              "serverStartedAt": receipt["startedAt"], "serverStoppedAt": receipt["stoppedAt"],
              "lifetimeSeconds": receipt["lifetimeSeconds"],
              "loopbackPort": receipt["port"], "case": FIXTURES[fixture], "events": events,
              "primaryRequestCount": len(primary), "continuationRequestCount": len(continued),
              "observedCallbackStatus": callback["status"],
              "numericErrorCode": error_code if isinstance(error_code, str) and re.fullmatch(r"-?\d{1,12}", error_code) else None,
              "ordinaryRuntimeContractVerified": runtime,
              "nativeCatchability": "NOT_PROVEN", "sameRequestStatusValue": "NOT_PROVEN",
              "retryImplemented": False,
              "limitations": LIMITATIONS}
    validate_public_diagnostic(report, manifest, assertions)
    atomic_json(root / "runtime/http-probe-diagnostic.json", report)
    return report


def validate_public_diagnostic(report, manifest, assertions):
    """Reject arbitrary fields/text before this new JSON path can be published."""
    require(isinstance(report, dict), "INVALID_PUBLIC_HTTP_DIAGNOSTIC")
    failed_keys = {"schemaVersion", "diagnosticStatus", "diagnosticObservationComplete",
                   "ordinaryRuntimeContractVerified", "nativeCatchability", "retryImplemented"}
    if report.get("diagnosticStatus") == "PROBE_EVIDENCE_REJECTED":
        require(set(report) == failed_keys and report["schemaVersion"] == "1.0"
                and report["diagnosticObservationComplete"] is False
                and report["ordinaryRuntimeContractVerified"] is False
                and report["nativeCatchability"] == "NOT_PROVEN" and report["retryImplemented"] is False,
                "INVALID_REJECTED_HTTP_DIAGNOSTIC")
        return
    keys = failed_keys | {"fixtureId", "runNonce", "compilerSha", "sourceSha256", "unsignedPlistSha256",
             "serverReceiptSha256", "serverState", "serverStartedAt", "serverStoppedAt", "lifetimeSeconds",
             "loopbackPort", "case", "events", "primaryRequestCount", "continuationRequestCount",
             "observedCallbackStatus", "numericErrorCode", "sameRequestStatusValue", "limitations"}
    require(set(report) == keys, "UNREVIEWED_PUBLIC_HTTP_FIELD")
    fixture, nonce, commit = report["fixtureId"], report["runNonce"], report["compilerSha"]
    validate_identity(fixture, nonce, commit)
    require(report["schemaVersion"] == "1.0" and report["case"] == FIXTURES[fixture]
            and report["nativeCatchability"] == "NOT_PROVEN"
            and report["sameRequestStatusValue"] == "NOT_PROVEN"
            and report["retryImplemented"] is False and report["limitations"] == LIMITATIONS,
            "UNREVIEWED_PUBLIC_HTTP_CONTRACT")
    statuses = {"NO_PRIMARY_REQUEST_OBSERVED", "MULTIPLE_REQUESTS_OBSERVED",
                "CONTINUATION_ORDER_REJECTED", "RESPONSE_WRITE_INCOMPLETE", "CONTROL_VERIFIED",
                "HTTP_RESPONSE_CONTINUED", "CONTINUATION_OBSERVED_OUTPUT_UNVERIFIED",
                "TERMINATED_WITHOUT_CONTINUATION_RECEIPT", "CANCELLED_WITHOUT_CONTINUATION_RECEIPT", "UNVERIFIED"}
    require(report["diagnosticStatus"] in statuses and report["observedCallbackStatus"] in ("SUCCESS", "ERROR", "CANCELLED")
            and type(report["diagnosticObservationComplete"]) is bool
            and type(report["ordinaryRuntimeContractVerified"]) is bool,
            "INVALID_PUBLIC_HTTP_STATUS")
    require(all(isinstance(report[k], str) and SHA256.fullmatch(report[k])
                for k in ("sourceSha256", "unsignedPlistSha256", "serverReceiptSha256")),
            "INVALID_PUBLIC_HTTP_DIGEST")
    require(report["numericErrorCode"] is None or (isinstance(report["numericErrorCode"], str)
            and re.fullmatch(r"-?\d{1,12}", report["numericErrorCode"])), "INVALID_PUBLIC_HTTP_ERROR_CODE")
    rows = [r for r in manifest.get("fixtures", []) if isinstance(r, dict) and r.get("fixtureId") == fixture]
    require(manifest.get("syntheticOnly") is True and manifest.get("compilerCommitSha") == commit
            and len(rows) == 1 and rows[0].get("sourceSha256") == report["sourceSha256"]
            and rows[0].get("unsignedPlistSha256") == report["unsignedPlistSha256"]
            and rows[0].get("fileName") == fixture + ".cherri", "PUBLIC_HTTP_SOURCE_BINDING")
    require(assertions.get("targetFixtureId") == fixture and assertions.get("expectedRunId") == nonce
            and assertions.get("compilerCommitSha") == commit
            and assertions.get("compiledSourceSha256") == report["sourceSha256"]
            and assertions.get("unsignedPlistSha256") == report["unsignedPlistSha256"]
            and assertions.get("observedCorrelatedCallbackStatus") == report["observedCallbackStatus"],
            "PUBLIC_HTTP_CALLBACK_BINDING")
    events = validate_receipt({"schemaVersion": "1.0", "fixtureId": fixture, "runNonce": nonce, "compilerSha": commit,
              "case": report["case"], "boundHost": "127.0.0.1", "port": report["loopbackPort"],
              "serverState": report["serverState"], "startedAt": report["serverStartedAt"],
              "stoppedAt": report["serverStoppedAt"], "lifetimeSeconds": report["lifetimeSeconds"],
              "events": report["events"], "rejectedRequestCount": 0}, fixture, nonce, commit)
    require(events == report["events"] and all(type(report[k]) is int for k in ("primaryRequestCount", "continuationRequestCount"))
            and report["primaryRequestCount"] == sum(e["route"] == "primary" for e in events)
            and report["continuationRequestCount"] == sum(e["route"] == "continued" for e in events),
            "PUBLIC_HTTP_EVENT_COUNT_BINDING")
    expected_status, complete = classify_observation(fixture, events, report["observedCallbackStatus"],
                                                      report["ordinaryRuntimeContractVerified"])
    require(report["diagnosticStatus"] == expected_status and report["diagnosticObservationComplete"] is complete,
            "PUBLIC_HTTP_OBSERVATION_BINDING")
    if report["ordinaryRuntimeContractVerified"]:
        require(assertions.get("runtimeContractVerified") is True
                and assertions.get("shortcutRunCompletedGate") == "PASS"
                and assertions.get("traceCapturedGate") == "PASS"
                and report["observedCallbackStatus"] == "SUCCESS", "PUBLIC_HTTP_RUNTIME_BINDING")

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("serve", "input", "diagnose", "startup-report"))
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--fixture", required=True)
    parser.add_argument("--nonce", required=True)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--fixtures-dir", type=Path)
    args = parser.parse_args()
    try:
        if args.command == "serve":
            server = ProbeServer(args.root / "runtime", args.fixture, args.nonce, args.commit)
            signal.signal(signal.SIGTERM, lambda *_: server.stop())
            signal.signal(signal.SIGINT, lambda *_: server.stop())
            server.serve()
        elif args.command == "input":
            print(ready_input(args.root / "runtime/http-probe-ready.json", args.fixture, args.nonce, args.commit))
        elif args.command == "startup-report":
            print(json.dumps(startup_report(args.root, args.fixture, args.nonce, args.commit)))
        else:
            require(args.fixtures_dir is not None, "FIXTURE_DIRECTORY_REQUIRED")
            report = diagnose(args.root, args.fixtures_dir, args.fixture, args.nonce, args.commit)
            print(json.dumps({"diagnosticStatus": report["diagnosticStatus"],
                              "ordinaryRuntimeContractVerified": report["ordinaryRuntimeContractVerified"]}))
            return 0 if report["diagnosticObservationComplete"] else 2
        return 0
    except (OSError, ValueError, TypeError, KeyError) as error:
        # Do not serialize raw exceptions, URLs, paths, headers or payloads.
        report = {"schemaVersion": "1.0", "diagnosticStatus": "PROBE_EVIDENCE_REJECTED",
                  "diagnosticObservationComplete": False, "ordinaryRuntimeContractVerified": False,
                  "nativeCatchability": "NOT_PROVEN", "retryImplemented": False}
        if args.command == "diagnose":
            atomic_json(args.root / "runtime/http-probe-diagnostic.json", report)
        if args.command in {"serve", "input"}:
            failure = startup_failure(args.command, error)
            try:
                atomic_json(args.root / ("runtime/http-probe-startup-" + args.command + ".json"), failure)
            except OSError:
                pass
            print(json.dumps(failure))
        else:
            print(json.dumps(report))
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
