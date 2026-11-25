#!/usr/bin/env python3
"""Validate actual paginate graphs with a bounded loopback HTTP oracle.

This is a source/CLI/graph-model producer, NOT an Apple Shortcuts runtime. The
model reuses the existing native text/reference decoder and explicitly records
Content Graph, HTTP status, conversion and error assumptions still unverified.
"""
import argparse
import copy
import hashlib
import json
import os
import pathlib
import plistlib
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from verify_debug_export import GraphModel, Value, as_text

ROOT = pathlib.Path(__file__).resolve().parents[2]
PREFIX = "is.workflow.actions."
ALLOWED = {
    "list", "dictionary", "number", "gettext", "getitemtype", "getvariable",
    "setvariable", "appendvariable", "hash", "downloadurl", "detect.dictionary",
    "getvalueforkey", "count", "getitemfromlist", "repeat.count", "repeat.each",
    "conditional", "alert", "exit",
}


def extract_paginate_graph(actions, *, legacy, native_actions=ALLOWED):
    """Verify main completion separately from the macro's typed List result."""
    assert isinstance(actions, list) and actions, "empty pagination graph"
    if legacy:
        graph = actions
        boundary = "LEGACY_UNCHANGED"
    else:
        terminal = actions[-1]
        assert (isinstance(terminal, dict)
                and terminal.get("WFWorkflowActionIdentifier") == PREFIX + "nothing"
                and set(terminal) <= {"WFWorkflowActionIdentifier", "WFWorkflowActionParameters"}
                and terminal.get("WFWorkflowActionParameters", {}) == {}), \
            "V2 main requires exactly one parameter-free terminal Nothing"
        graph = actions[:-1]
        boundary = "V2_EXPLICIT_VOID_VERIFIED"

    # Nothing is not a macro effect. Reject it at every interior position and
    # keep the complete original actions for the independent lossless roundtrip.
    allowed = {PREFIX + name for name in native_actions}
    assert all(isinstance(action, dict)
               and action.get("WFWorkflowActionIdentifier") in allowed
               for action in graph), "unreviewed pagination action"
    assert len(graph) >= 3, "pagination result graph is incomplete"
    initial, store, result = graph[0], graph[1], graph[-1]
    ip = initial.get("WFWorkflowActionParameters", {})
    sp = store.get("WFWorkflowActionParameters", {})
    rp = result.get("WFWorkflowActionParameters", {})
    assert (initial["WFWorkflowActionIdentifier"] == PREFIX + "list"
            and ip.get("WFItems") == []
            and isinstance(ip.get("UUID"), str) and ip["UUID"]
            and isinstance(ip.get("CustomOutputName"), str)), \
        "pagination must initialize its actual empty List"
    name = sp.get("WFVariableName")
    assert (store["WFWorkflowActionIdentifier"] == PREFIX + "setvariable"
            and isinstance(name, str) and name.startswith("__cherri_paginate_")
            and name.endswith("_results")
            and sp.get("WFInput") == {
                "WFSerializationType": "WFTextTokenAttachment",
                "Value": {"Type": "ActionOutput", "OutputUUID": ip["UUID"],
                          "OutputName": ip["CustomOutputName"]}}), \
        "pagination results must bind the original List producer"
    assert sum(action["WFWorkflowActionIdentifier"] == PREFIX + "setvariable"
               and action.get("WFWorkflowActionParameters", {}).get("WFVariableName") == name
               for action in graph) == 1, "pagination List initializer was overwritten"
    assert (result["WFWorkflowActionIdentifier"] == PREFIX + "getvariable"
            and rp.get("WFVariable") == {
                "WFSerializationType": "WFTextTokenAttachment",
                "Value": {"Type": "Variable", "VariableName": name}}), \
        "pagination result must read the original results List"
    return graph, boundary


def extract_header_paginate_graph(actions, *, legacy, native_actions=ALLOWED):
    """Keep the actual authored Text producer while verifying the same macro body."""
    assert isinstance(actions, list) and len(actions) > 1
    producer = actions[0]
    params = producer.get("WFWorkflowActionParameters", {})
    assert producer.get("WFWorkflowActionIdentifier") == PREFIX + "gettext"
    assert params.get("WFTextActionText") == "Bearer PAGINATE_SYNTHETIC_ONLY"
    assert set(params) <= {"WFTextActionText", "UUID", "CustomOutputName"}
    identity = params.get("UUID")
    assert isinstance(identity, str) and identity
    assert sum(action.get("WFWorkflowActionParameters", {}).get("UUID") == identity
               for action in actions) == 1, "authored header producer identity changed"
    body, boundary = extract_paginate_graph(actions[1:], legacy=legacy, native_actions=native_actions)
    return [producer, *body], boundary


class GuardStop(Exception):
    pass


class BodyDecodeError(Exception):
    pass


def native_value(raw):
    if raw is None:
        return Value("Null", None)
    if isinstance(raw, bool):
        return Value("Bool", raw)
    if isinstance(raw, (int, float)):
        return Value("Number", raw)
    if isinstance(raw, str):
        return Value("Text", raw)
    if isinstance(raw, list):
        return Value("List", [native_value(v) for v in raw])
    if isinstance(raw, dict):
        return Value("Dictionary", {k: native_value(v) for k, v in raw.items()})
    raise ValueError("unsupported synthetic JSON value")


def plain(value):
    if value.kind == "Dictionary":
        return {k: plain(v) for k, v in value.data.items()}
    if value.kind == "List":
        return [plain(v) for v in value.data]
    return value.data


def content_items(value):
    # Explicit MODEL assumption: an empty list/null has zero items, a scalar or
    # native Dictionary is one item, and a List retains its ordered members.
    return value.data if value.kind == "List" else ([] if value.kind == "Null" else [value])


class PageServer:
    def __init__(self):
        self.routes, self.requests, self.request_headers = {}, [], []
        owner = self

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                owner.requests.append(self.path)
                owner.request_headers.append({key.lower(): value for key, value in self.headers.items()})
                status, body = owner.routes.get(self.path, (404, {"error": "synthetic missing page"}))
                encoded = body.encode("utf-8") if isinstance(body, str) else json.dumps(body, ensure_ascii=False).encode("utf-8")
                self.send_response(status)
                self.send_header("Content-Type", "application/json; charset=utf-8")
                self.send_header("Content-Length", str(len(encoded)))
                self.end_headers()
                self.wfile.write(encoded)

            def log_message(self, *_):
                pass

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.origin = "http://127.0.0.1:" + str(self.server.server_port)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)

    def __enter__(self):
        self.thread.start()
        return self

    def __exit__(self, *_):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=3)


class PaginateGraphModel(GraphModel):
    native_actions = ALLOWED

    def extension_action(self, ident, p):
        raise AssertionError("unexpected control delimiter reached: " + ident)

    def __init__(self, server):
        super().__init__([])
        self.variables = {}
        self.server = server
        self.statuses, self.source_dictionaries = [], set()
        self.alert = None

    def value(self, raw):
        if isinstance(raw, (int, float, bool)) or raw is None:
            return native_value(raw)
        if isinstance(raw, dict) and raw.get("WFSerializationType") == "WFTextTokenString":
            if not raw["Value"].get("attachmentsByRange"):
                return Value("Text", raw["Value"]["string"])
        return super().value(raw)

    def http_headers(self, raw):
        # Only the native Text-valued HTTP dictionary used by these fixtures is
        # modeled. Unknown wrappers/fields and unbound references fail closed.
        assert isinstance(raw, dict) and set(raw) == {"WFSerializationType", "Value"}
        assert raw["WFSerializationType"] == "WFDictionaryFieldValue"
        value = raw["Value"]
        assert isinstance(value, dict) and set(value) == {"WFDictionaryFieldValueItems"}
        rows = value["WFDictionaryFieldValueItems"]
        assert isinstance(rows, list)

        def reference(ref):
            assert isinstance(ref, dict) and set(ref) <= {"Type", "OutputUUID", "OutputName"}
            assert ref.get("Type") == "ActionOutput"
            assert isinstance(ref.get("OutputUUID"), str) and ref["OutputUUID"] in self.outputs
            if "OutputName" in ref:
                assert isinstance(ref["OutputName"], str) and ref["OutputName"]

        def text(field):
            if isinstance(field, str):
                return field
            assert isinstance(field, dict) and set(field) == {"WFSerializationType", "Value"}
            if field["WFSerializationType"] == "WFTextTokenAttachment":
                # V2 direct dynamic leaves use the existing native attachment
                # form; Legacy interpolation uses a Text token string below.
                # Both must resolve a real preceding Text action output.
                reference(field["Value"])
                return as_text(self.value(field))
            assert field["WFSerializationType"] == "WFTextTokenString"
            payload = field["Value"]
            assert isinstance(payload, dict) and set(payload) <= {"string", "attachmentsByRange"}
            assert isinstance(payload.get("string"), str)
            refs = payload.get("attachmentsByRange", {})
            assert isinstance(refs, dict)
            assert len(refs) == payload["string"].count("\ufffc")
            for ref in refs.values():
                reference(ref)
            return as_text(self.value(field))

        headers = {}
        for row in rows:
            assert isinstance(row, dict) and set(row) == {"WFKey", "WFItemType", "WFValue"}
            assert type(row["WFItemType"]) is int and row["WFItemType"] == 0
            key = text(row["WFKey"])
            assert key and key.lower() not in {prior.lower() for prior in headers}
            headers[key] = text(row["WFValue"])
        return headers

    def remember(self, value):
        if value.kind == "Dictionary":
            self.source_dictionaries.add(id(value))
            for v in value.data.values():
                self.remember(v)
        elif value.kind == "List":
            for v in value.data:
                self.remember(v)
        return value

    def condition(self, p):
        left = self.value(p["WFInput"])
        code = p["WFCondition"]
        if "WFNumberValue" in p:
            assert left.kind == "Number", "numeric comparison lost native Number"
            a, b = left.data, self.value(p["WFNumberValue"]).data
        else:
            # Deliberately case-fold Text equality/prefix/contains. The emitted
            # graph hashes URL/key bytes before equality; it does not rely on
            # the native comparator being case-sensitive.
            a = as_text(left).casefold()
            b = as_text(self.value(p["WFConditionalActionString"])).casefold()
        return {
            0: lambda: a < b, 2: lambda: a > b, 3: lambda: a >= b,
            4: lambda: a == b, 5: lambda: a != b,
            8: lambda: a.startswith(b), 99: lambda: b in a,
        }[code]()

    def run(self, actions, start=0, end=None):
        end = len(actions) if end is None else end
        previous = Value("Null", None)
        index = start
        while index < end:
            a = actions[index]
            ident = a["WFWorkflowActionIdentifier"].removeprefix(PREFIX)
            p = a.get("WFWorkflowActionParameters", {})
            assert ident in self.native_actions, "unexpected action in paginate graph: " + ident
            self.executed += 1
            assert self.executed <= 2_000_000, "model execution guard exceeded"
            if ident in {"conditional", "repeat.count", "repeat.each"} and p["WFControlFlowMode"] == 0:
                group = p["GroupingIdentifier"]
                closing = next(i for i in range(index + 1, end)
                               if actions[i].get("WFWorkflowActionParameters", {}).get("GroupingIdentifier") == group
                               and actions[i]["WFWorkflowActionParameters"].get("WFControlFlowMode") == 2)
                if ident == "conditional":
                    alternate = next((i for i in range(index + 1, closing)
                                      if actions[i].get("WFWorkflowActionParameters", {}).get("GroupingIdentifier") == group
                                      and actions[i]["WFWorkflowActionParameters"].get("WFControlFlowMode") == 1), closing)
                    if self.condition(p):
                        previous = self.run(actions, index + 1, alternate)
                    elif alternate != closing:
                        previous = self.run(actions, alternate + 1, closing)
                else:
                    if ident == "repeat.count":
                        count = self.value(p["WFRepeatCount"])
                        assert count.kind == "Number" and int(count.data) == count.data and 0 <= count.data <= 101
                        members = [Value("Number", i + 1) for i in range(int(count.data))]
                    else:
                        members = content_items(self.value(p["WFInput"]))
                    old_item = self.variables.get("Repeat Item")
                    for item in members:
                        self.variables["Repeat Item"] = item
                        previous = self.run(actions, index + 1, closing)
                    if old_item is None:
                        self.variables.pop("Repeat Item", None)
                    else:
                        self.variables["Repeat Item"] = old_item
                close = actions[closing].get("WFWorkflowActionParameters", {})
                if "UUID" in close:
                    self.outputs[close["UUID"]] = previous
                index = closing + 1
                continue
            if ident == "list":
                assert p["WFItems"] == [], "paginate initializes only empty lists"
                previous = Value("List", [])
            elif ident == "dictionary":
                fields = p["WFItems"]["Value"]["WFDictionaryFieldValueItems"]
                assert len(fields) == 1 and fields[0]["WFItemType"] == 4
                assert self.value(fields[0]["WFKey"]).data == "cherri_paginate_type_witness"
                assert fields[0]["WFValue"]["Value"] is True
                previous = Value("Dictionary", {"cherri_paginate_type_witness": Value("Bool", True)})
            elif ident == "number":
                previous = Value("Number", int(p["WFNumberActionNumber"]))
            elif ident == "gettext":
                previous = self.value(p["WFTextActionText"])
            elif ident == "getvariable":
                previous = self.value(p["WFVariable"])
            elif ident == "setvariable":
                previous = self.value(p["WFInput"])
                self.variables[p["WFVariableName"]] = previous
            elif ident == "appendvariable":
                incoming = self.value(p["WFInput"])
                current = self.variables[p["WFVariableName"]]
                assert current.kind == "List"
                current.data.extend(content_items(incoming))
                previous = current
            elif ident == "getitemtype":
                previous = Value("Text", "model-native:" + self.value(p["WFInput"]).kind)
            elif ident == "hash":
                assert p["WFHashType"] == "SHA256"
                previous = Value("Text", hashlib.sha256(as_text(self.value(p["WFInput"])).encode("utf-8")).hexdigest())
            elif ident == "downloadurl":
                assert p["WFHTTPMethod"] == "GET"
                assert set(p) <= {"WFURL", "WFHTTPMethod", "WFHTTPHeaders", "UUID", "CustomOutputName"}
                headers = self.http_headers(p["WFHTTPHeaders"]) if "WFHTTPHeaders" in p else {}
                url = as_text(self.value(p["WFURL"]))
                # The oracle cannot contact an external endpoint, even after a
                # causal mutation. This is a test boundary, not native behavior.
                assert url.startswith(self.server.origin + "/"), "oracle forbids external network"
                try:
                    request = urllib.request.Request(url, headers=headers, method="GET")
                    response = urllib.request.urlopen(request, timeout=5)
                except urllib.error.HTTPError as error:
                    response = error
                with response:
                    self.statuses.append(response.status)
                    previous = Value("Text", response.read().decode("utf-8"))
            elif ident == "detect.dictionary":
                incoming = self.value(p["WFInput"])
                if incoming.kind == "Text":
                    try:
                        previous = self.remember(native_value(json.loads(incoming.data)))
                    except json.JSONDecodeError as error:
                        raise BodyDecodeError("synthetic malformed JSON body") from error
                else:
                    previous = incoming
            elif ident == "getvalueforkey":
                incoming = self.value(p["WFInput"])
                assert incoming.kind == "Dictionary"
                mode = p["WFGetDictionaryValueType"]
                if mode == "All Keys":
                    previous = Value("List", [Value("Text", k) for k in incoming.data])
                else:
                    assert mode == "Value"
                    previous = incoming.data[p["WFDictionaryKey"]]
            elif ident == "count":
                incoming = self.value(p["Input"])
                if p["WFCountType"] == "Items":
                    previous = Value("Number", len(content_items(incoming)))
                else:
                    assert p["WFCountType"] == "Characters"
                    previous = Value("Number", len(as_text(incoming)))
            elif ident == "getitemfromlist":
                assert p["WFItemSpecifier"] == "Items in Range"
                first, last = self.value(p["WFItemRangeStart"]).data, self.value(p["WFItemRangeEnd"]).data
                assert first == 1 and last >= first
                previous = Value("List", content_items(self.value(p["WFInput"]))[int(first)-1:int(last)])
            elif ident == "alert":
                self.alert = p["WFAlertActionMessage"]
                assert self.alert.startswith("PAGINATE_") and "http" not in self.alert and "SYNTHETIC_SECRET" not in self.alert
                previous = Value("Null", None)
            elif ident == "exit":
                assert self.alert is not None, "semantic stop lacks its fixed diagnostic"
                raise GuardStop(self.alert)
            else:
                previous = self.extension_action(ident, p)
            if "UUID" in p:
                self.outputs[p["UUID"]] = previous
            index += 1
        return previous


def run(command, timeout=180):
    return subprocess.run([str(v) for v in command], cwd=ROOT, capture_output=True,
                          text=True, encoding="utf-8", timeout=timeout)


def checked(command, label):
    result = run(command)
    if result.returncode:
        raise RuntimeError(label + " failed: " + (result.stdout + result.stderr)[-2000:])
    return result


def fingerprint():
    paths = [*ROOT.glob("*.go"), *ROOT.joinpath("internal/language").rglob("*.go"),
             *ROOT.joinpath("actions").glob("*.cherri"),
             *ROOT.joinpath("docs/apple-runtime/p10r15/paginate-pages").glob("*.json"),
             ROOT/"docs/apple-runtime/macos-27/builtin-actions.json",
             ROOT/"go.mod", ROOT/"go.sum", pathlib.Path(__file__).resolve(),
             ROOT/"tools/p10r15/verify_debug_export.py"]
    digest = hashlib.sha256()
    for path in sorted(set(paths)):
        digest.update(path.relative_to(ROOT).as_posix().encode() + b"\0" + path.read_bytes() + b"\0")
    return digest.hexdigest()


def inspect_graph(graph, server, routes, expected=None, requests=None, guard=None, body_error=False, *, headers=None):
    server.routes, server.requests, server.request_headers = routes, [], []
    model = PaginateGraphModel(server)
    output = None
    observed_guard = None
    try:
        output = model.run(graph)
    except GuardStop as error:
        observed_guard = str(error)
        assert guard == observed_guard, "unexpected semantic guard: " + observed_guard
    except BodyDecodeError:
        assert body_error, "unexpected synthetic body decoder failure"
    else:
        assert guard is None and not body_error, "expected rejection was bypassed"
        assert output.kind == "List"
        assert all(v.kind == "Dictionary" for v in output.data), "returned a coerced non-Dictionary item"
        assert all(id(v) in model.source_dictionaries for v in output.data), "copied/coerced the original item reference"
        assert plain(output) == expected, "ordered typed values or cap changed"
    assert server.requests == requests, "unexpected native GET invocation sequence"
    for observed in server.request_headers:
        expected_headers = {} if headers is None else {key.lower(): value for key, value in headers.items()}
        assert {key: observed[key] for key in ("authorization", "x-cherri-client") if key in observed} == expected_headers, "actual loopback request headers changed"
    return {"status": "PASS_MODEL", "requests": len(server.requests), "httpStatusesObservedByOracle": model.statuses,
            "returnedItems": None if output is None else len(output.data), "fixedGuard": observed_guard,
            "syntheticBodyDecodeFailure": body_error, "modelExecutedActions": model.executed}


def scenarios(origin):
    a = {"id": 1, "text": '\"\\\n\t \u05e9\u05b8\u05c1\u05dc\u05d5\u05b9\u05dd \U0001f352', "active": False,
         "zero": 0, "null": None, "nested": {"list": ["", 1, True, None, {"key": "SYNTHETIC_SECRET"}]}}
    b, c, d = {"id": 2}, {"id": 3}, {"id": 4}
    def page(items, next_value=None, include_next=False):
        out = {"items": items}
        if include_next:
            out["next"] = next_value
        return (200, out)
    cases = {
        "three-pages-middle-empty": ({"/entry.json": page([a, b], "two.json", True), "/two.json": page([], "three.json", True), "/three.json": page([c])}, [a,b,c], ["/entry.json","/two.json","/three.json"], None, False),
        "empty": ({"/entry.json": page([])}, [], ["/entry.json"], None, False),
        "single": ({"/entry.json": page([a])}, [a], ["/entry.json"], None, False),
        "empty-next": ({"/entry.json": page([a], "", True)}, [a], ["/entry.json"], None, False),
        "null-next": ({"/entry.json": page([a], None, True)}, [a], ["/entry.json"], None, False),
        "null-items-model-boundary": ({"/entry.json": page(None)}, [], ["/entry.json"], None, False),
        "scalar-dictionary-model-boundary": ({"/entry.json": page(a)}, [a], ["/entry.json"], None, False),
        "same-origin-absolute": ({"/entry.json": page([a], origin+"/two.json", True), "/two.json": page([b])}, [a,b], ["/entry.json","/two.json"], None, False),
        "cycle": ({"/entry.json": page([a], "two.json", True), "/two.json": page([b], "entry.json", True)}, [a,b], ["/entry.json","/two.json"], None, False),
        "case-distinct-paths": ({"/entry.json": page([a], "ENTRY.json", True), "/ENTRY.json": page([b])}, [a,b], ["/entry.json","/ENTRY.json"], None, False),
        "max-pages": ({"/entry.json": page([a], "two.json", True), "/two.json": page([b], "three.json", True), "/three.json": page([c], "four.json", True), "/four.json": page([d])}, [a,b,c], ["/entry.json","/two.json","/three.json"], None, False),
        "max-items": ({"/entry.json": page([a,b,c,d], "two.json", True), "/two.json": page([a,b,c,d], "three.json", True), "/three.json": page([d])}, [a,b,c,d,a,b], ["/entry.json","/two.json"], None, False),
        "full-no-extra-request": ({"/entry.json": page([a,b,c,d,a,b], "two.json", True), "/two.json": page([d])}, [a,b,c,d,a,b], ["/entry.json"], None, False),
        "missing-items": ({"/entry.json": (200, {"next": ""})}, None, ["/entry.json"], "PAGINATE_ITEMS_KEY_REQUIRED", False),
        "case-wrong-selector": ({"/entry.json": (200, {"Items": [a]})}, None, ["/entry.json"], "PAGINATE_ITEMS_KEY_REQUIRED", False),
        "non-dictionary-page": ({"/entry.json": (200, [1,2])}, None, ["/entry.json"], "PAGINATE_PAGE_DICTIONARY_REQUIRED", False),
        "non-dictionary-item": ({"/entry.json": page(["SYNTHETIC_SECRET"])}, None, ["/entry.json"], "PAGINATE_ITEM_DICTIONARY_REQUIRED", False),
        "partial-page-rejected": ({"/entry.json": page([a, "SYNTHETIC_SECRET"])}, None, ["/entry.json"], "PAGINATE_ITEM_DICTIONARY_REQUIRED", False),
        "nontext-next": ({"/entry.json": page([a], 0, True)}, None, ["/entry.json"], "PAGINATE_NEXT_TEXT_REQUIRED", False),
        "multiple-next": ({"/entry.json": page([a], ["two.json","three.json"], True)}, None, ["/entry.json"], "PAGINATE_NEXT_TEXT_REQUIRED", False),
        "cross-origin-next": ({"/entry.json": page([a], "https://example.invalid/secret", True)}, None, ["/entry.json"], "PAGINATE_NEXT_URL_UNSUPPORTED", False),
        "sibling-escape": ({"/entry.json": page([a], "../two.json", True)}, None, ["/entry.json"], "PAGINATE_NEXT_URL_UNSUPPORTED", False),
        "sibling-query": ({"/entry.json": page([a], "two.json?token=SYNTHETIC_SECRET", True)}, None, ["/entry.json"], "PAGINATE_NEXT_URL_UNSUPPORTED", False),
        "whitespace-next": ({"/entry.json": page([a], "two .json", True)}, None, ["/entry.json"], "PAGINATE_NEXT_URL_INVALID", False),
        "absolute-fragment": ({"/entry.json": page([a], origin+"/two.json#synthetic", True)}, None, ["/entry.json"], "PAGINATE_NEXT_URL_INVALID", False),
        "malformed-json": ({"/entry.json": (200, "{malformed SYNTHETIC_SECRET")}, None, ["/entry.json"], None, True),
        "404-error-body": ({"/entry.json": (404, {"error":"synthetic"})}, None, ["/entry.json"], "PAGINATE_ITEMS_KEY_REQUIRED", False),
        "429-error-body-no-retry": ({"/entry.json": (429, {"error":"synthetic"})}, None, ["/entry.json"], "PAGINATE_ITEMS_KEY_REQUIRED", False),
        "404-valid-body-status-unverified": ({"/entry.json": (404, {"items":[a]})}, [a], ["/entry.json"], None, False),
    }
    return cases, a


def invoke_case(graph, server, case):
    routes, expected, requests, guard, body_error = case
    return inspect_graph(graph, server, routes, expected, requests, guard, body_error)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--report", default="docs/apple-runtime/p10r15/paginate-validation.json")
    parser.add_argument("--output-dir", default="dist/p15-paginate", help="Isolated directory for original sources, compiler and unsigned roundtrip artifacts")
    args = parser.parse_args()
    destination = ROOT / args.report
    destination.parent.mkdir(parents=True, exist_ok=True)
    source_sha = checked(["git","rev-parse","HEAD"],"source identity").stdout.strip()
    dirty = bool(checked(["git","status","--porcelain"],"source state").stdout.strip())
    before = fingerprint()
    destination.write_text('{"status":"RUNNING_SOURCE_VALIDATION","nativeExecuted":0}\n', encoding="utf-8")
    output = ROOT/args.output_dir
    output.mkdir(parents=True, exist_ok=True)
    exe = output/("cherri.exe" if os.name=="nt" else "cherri")
    started = time.monotonic()
    checked(["go","build","-trimpath","-o",exe,"."],"compiler build")
    caps = json.loads(checked([exe,"--capabilities-json"],"capabilities").stdout)
    assert caps["actionCount"] == 505
    descriptor = next(item for item in caps["compilerMacros"] if item.get("id", item.get("ID")) == "paginate")
    checks, graphs, faults, negatives = [], {}, [], []
    flavors = {
        "base": (3,6,"items","next"), "page-cap": (1,6,"items","next"),
        "item-cap": (3,2,"items","next"), "zero-pages": (0,6,"items","next"),
        "zero-items": (3,0,"items","next"), "custom": (3,6,"rows","after"),
        "thousand": (3,1000,"items","next"), "upper-limits": (100,10000,"items","next"),
        "headers-literal": (3,6,"items","next"), "headers-dynamic": (3,6,"items","next"),
        "headers-zero": (0,6,"items","next"),
    }
    with PageServer() as server:
        cases, item = scenarios(server.origin)
        expected_headers = {"Authorization": "Bearer PAGINATE_SYNTHETIC_ONLY", "X-Cherri-Client": "Cherri-P15"}
        for legacy in (False,True):
            for profile in ("debug","release"):
                for flavor, (pages,items,items_key,next_key) in flavors.items():
                    ident = f"{'legacy' if legacy else 'v2'}-{profile}-{flavor}"
                    source, target = output/(ident+".cherri"), output/(ident+".shortcut")
                    code = f'paginate("{server.origin}/entry.json", "{items_key}", "{next_key}", {pages}, {items})\n'
                    if flavor.startswith("headers-"):
                        auth = json.dumps(expected_headers["Authorization"])
                        prelude = ""
                        if flavor == "headers-dynamic":
                            decl = "const" if legacy else "let"
                            prelude = f'{decl} authorization = text({auth})\n'
                            auth = '"{authorization}"' if legacy else "authorization"
                        fields = '{"Authorization": ' + auth + ', "X-Cherri-Client": "Cherri-P15"}'
                        code = prelude + f'paginate("{server.origin}/entry.json", "{items_key}", "{next_key}", {pages}, {items}, {fields})\n'
                    source.write_bytes(code.encode())
                    command = [exe,"compile",source,"-o",target,"--skip-sign","--no-ansi","--build-profile",profile]+(["--legacy"] if legacy else [])
                    checked(command,ident)
                    graph = plistlib.loads(target.read_bytes())["WFWorkflowActions"]
                    extractor = extract_header_paginate_graph if flavor == "headers-dynamic" else extract_paginate_graph
                    macro_graph, main_boundary = extractor(graph, legacy=legacy)
                    imported, rebuilt = output/(ident+"-import.cherri"), output/(ident+"-rebuilt.shortcut")
                    checked([exe,"import",target,"--mode=v2","--lossless","-o",imported], ident+" lossless import")
                    checked([exe,"compile",imported,"-o",rebuilt,"--skip-sign","--no-ansi"], ident+" rebuild")
                    assert graph == plistlib.loads(rebuilt.read_bytes())["WFWorkflowActions"], "lossless graph changed"
                    models = {}
                    if flavor == "base":
                        for case_name, case in cases.items():
                            models[case_name] = invoke_case(macro_graph,server,case)
                    elif flavor in {"zero-pages","zero-items","headers-zero"}:
                        models["zero-network"] = inspect_graph(macro_graph,server,{"/entry.json":(200,{"items":[item]})},[],[])
                        assert len(macro_graph)==3 and len(graph)==(3 if legacy else 4)
                    elif flavor in {"headers-literal", "headers-dynamic"}:
                        route, expected, paths, _, _ = cases["three-pages-middle-empty"]
                        models["exact-request-headers-three-pages"] = inspect_graph(macro_graph, server, route, expected, paths, headers=expected_headers)
                    elif flavor == "page-cap":
                        route = cases["three-pages-middle-empty"][0]
                        models["one-request"] = inspect_graph(macro_graph,server,route,[item,{"id":2}],["/entry.json"])
                    elif flavor == "item-cap":
                        route = cases["three-pages-middle-empty"][0]
                        models["stop-before-next"] = inspect_graph(macro_graph,server,route,[item,{"id":2}],["/entry.json"])
                    elif flavor == "upper-limits":
                        models["inclusive-limit-boundaries"] = invoke_case(macro_graph,server,cases["single"])
                    elif flavor == "custom":
                        route = {"/entry.json":(200,{"rows":[item],"after":"two.json"}),"/two.json":(200,{"rows":[{"id":2}]})}
                        models["custom-selectors"] = inspect_graph(macro_graph,server,route,[item,{"id":2}],["/entry.json","/two.json"])
                    else:
                        values = [{"id":i,"typed":item} for i in range(1000)]
                        route = {"/entry.json":(200,{"items":values,"next":"two.json"})}
                        models["thousand-no-extra-request"] = inspect_graph(macro_graph,server,route,values,["/entry.json"])
                    checks.append({"id":ident,"status":"PASS","sourceSha256":hashlib.sha256(source.read_bytes()).hexdigest(),
                                   "actionCount":len(graph),"getActionNodes":sum(a["WFWorkflowActionIdentifier"]==PREFIX+"downloadurl" for a in graph),
                                   "exactNativeActionRoundtrip":True,"models":models,
                                    "macroActionCount":len(macro_graph),"mainOutputBoundary":main_boundary})
                    graphs[(legacy,profile,flavor)] = macro_graph
        base = graphs[(False,"debug","base")]
        def params(graph, suffix):
            return [a.setdefault("WFWorkflowActionParameters",{}) for a in graph if a["WFWorkflowActionIdentifier"]==PREFIX+suffix]
        def page_budget(g):
            next(p for p in params(g,"repeat.count") if p.get("WFControlFlowMode")==0)["WFRepeatCount"]=4
        # Identify the results room comparison by its producing count/getvariable
        # chain instead of relying on generated UUIDs or action indices.
        def item_budget(g):
            by_uuid={a.get("WFWorkflowActionParameters",{}).get("UUID"):a for a in g}
            for p in params(g,"conditional"):
                if p.get("WFCondition")!=0 or p.get("WFNumberValue")!=6:
                    continue
                count=by_uuid[p["WFInput"]["Variable"]["Value"]["OutputUUID"]]["WFWorkflowActionParameters"]
                if "Input" not in count:
                    continue
                getter=by_uuid[count["Input"]["Value"]["OutputUUID"]]["WFWorkflowActionParameters"]
                if getter.get("WFVariable",{}).get("Value",{}).get("VariableName","").endswith("_results"):
                    p["WFNumberValue"]=999
                    return
            raise AssertionError("results cap not found")
        def cycle(g):
            next(p for p in params(g,"conditional") if p.get("WFCondition")==4 and "WFConditionalActionString" in p)["WFCondition"]=5
        def typed_append(g):
            getter=next(p for p in params(g,"gettext") if isinstance(p.get("WFTextActionText"),str) and p["WFTextActionText"].startswith(server.origin))
            next(p for p in params(g,"appendvariable") if p["WFVariableName"].endswith("_results"))["WFInput"]={
                "WFSerializationType":"WFTextTokenAttachment","Value":{"Type":"ActionOutput","OutputUUID":getter["UUID"],"OutputName":getter["CustomOutputName"]}}
        def witness(g):
            for index,a in enumerate(g):
                if a.get("WFWorkflowActionParameters",{}).get("WFAlertActionMessage")=="PAGINATE_ITEM_DICTIONARY_REQUIRED":
                    p=g[index-1]["WFWorkflowActionParameters"]
                    if "WFConditionalActionString" in p:
                        p["WFCondition"]=4
                        return
            raise AssertionError("type guard not found")
        def method(g):
            params(g,"downloadurl")[0]["WFHTTPMethod"]="POST"
        def full_stop(g):
            one=next(p for p in params(g,"number") if p["WFNumberActionNumber"]=="1")
            for index,a in enumerate(g):
                p=a.get("WFWorkflowActionParameters",{})
                if p.get("WFCondition")==3 and p.get("WFNumberValue")==6:
                    g[index+1]["WFWorkflowActionParameters"]["WFInput"]={
                        "WFSerializationType":"WFTextTokenAttachment","Value":{"Type":"ActionOutput","OutputUUID":one["UUID"],"OutputName":one["CustomOutputName"]}}
                    return
            raise AssertionError("full stop not found")
        for name,case_name,mutator in [
            ("page-budget-drift","max-pages",page_budget),("item-budget-drift","max-items",item_budget),
            ("cycle-predicate-inverted","cycle",cycle),("typed-item-reference-replaced","single",typed_append),
            ("dictionary-type-guard-bypassed","non-dictionary-item",witness),
            ("unsafe-http-method","single",method),("full-cap-fetches-extra-page","full-no-extra-request",full_stop),
        ]:
            altered=copy.deepcopy(base);mutator(altered)
            try:
                invoke_case(altered,server,cases[case_name])
            except (AssertionError,ValueError,KeyError):
                pass
            else:
                raise AssertionError("causal fault not rejected: "+name)
            invoke_case(base,server,cases[case_name])
            faults.append({"id":name,"negativeRejected":True,"restoredControl":"PASS"})
        header_graph = graphs[(False, "debug", "headers-dynamic")]
        route, expected, paths, _, _ = cases["three-pages-middle-empty"]
        for fault in ("drop-headers", "wrong-wrapper", "drop-field", "duplicate-field", "changed-authored-value", "orphan-reference", "request-body"):
            broken = copy.deepcopy(header_graph)
            request = params(broken, "downloadurl")[0]
            if fault == "drop-headers":
                del request["WFHTTPHeaders"]
            elif fault == "wrong-wrapper":
                request["WFHTTPHeaders"]["WFSerializationType"] = "WFTextTokenAttachment"
            elif fault in {"drop-field", "duplicate-field"}:
                fields = request["WFHTTPHeaders"]["Value"]["WFDictionaryFieldValueItems"]
                if fault == "drop-field":
                    fields.pop()
                else:
                    fields.append(copy.deepcopy(fields[0]))
            elif fault == "changed-authored-value":
                broken[0]["WFWorkflowActionParameters"]["WFTextActionText"] = "OTHER_SYNTHETIC_VALUE"
            elif fault == "orphan-reference":
                def corrupt(value):
                    if isinstance(value, dict):
                        if value.get("Type") == "ActionOutput":
                            value["OutputUUID"] = "MISSING_HEADER_PRODUCER"
                        for child in value.values():
                            corrupt(child)
                    elif isinstance(value, list):
                        for child in value:
                            corrupt(child)
                corrupt(request["WFHTTPHeaders"])
            else:
                request["WFRequestVariable"] = "UNRELATED_BODY"
            try:
                inspect_graph(broken, server, route, expected, paths, headers=expected_headers)
            except (AssertionError, KeyError, TypeError, ValueError):
                pass
            else:
                raise AssertionError("header causal fault not rejected: " + fault)
            inspect_graph(header_graph, server, route, expected, paths, headers=expected_headers)
            faults.append({"id": "headers-" + fault, "negativeRejected": True, "restoredControl": "PASS"})
    bad_sources={
        "file-scheme":'paginate("file:///private/example")',
        "userinfo":'paginate("https://user:pass@example.invalid/one")',
        "fragment":'paginate("https://example.invalid/one#token")',
        "selector-path":'paginate("https://example.invalid/one", "data.items")',
        "same-selectors":'paginate("https://example.invalid/one", "items", "items")',
        "too-many-pages":'paginate("https://example.invalid/one", "items", "next", 101)',
        "too-many-items":'paginate("https://example.invalid/one", "items", "next", 3, 10001)',
        "fractional-pages":'paginate("https://example.invalid/one", "items", "next", 1.5)',
        "negative-pages":'paginate("https://example.invalid/one", "items", "next", -1)',
        "dynamic-url":'DECL url = text("https://example.invalid/one")\npaginate(url)',
        "dynamic-cap":'DECL cap = number(3)\npaginate("https://example.invalid/one", "items", "next", cap)',
    }
    for legacy in (False,True):
        for profile in ("debug","release"):
            for name,code in bad_sources.items():
                ident=f"negative-{legacy}-{profile}-{name}"
                source,target=output/(ident+".cherri"),output/(ident+".shortcut")
                source.write_bytes((code.replace("DECL","const" if legacy else "let")+"\n").encode())
                target.unlink(missing_ok=True)
                result=run([exe,"compile",source,"-o",target,"--skip-sign","--no-ansi","--build-profile",profile]+(["--legacy"] if legacy else []))
                assert result.returncode!=0 and not target.exists(), ident+" accepted an invalid contract"
                negatives.append({"id":ident,"exitCode":result.returncode,"noArtifact":True,"status":"PASS"})
    after=fingerprint()
    assert before==after,"source drift during validation"
    report={
        "schemaVersion":1,"status":"PASS_SOURCE_AND_MODEL_ONLY","featureStatus":"PARTIAL_SOURCE_IMPLEMENTED_PENDING_NATIVE",
        "nativeRuntimeStatus":"NOT_EXECUTED","sourceCommit":source_sha,"worktreeDirtyBeforeEvidence":dirty,
        "sourceFingerprint":before,"compilerSha256":hashlib.sha256(exe.read_bytes()).hexdigest(),
        "schemaFingerprint":caps.get("schemaFingerprint"),"macroFingerprint":caps.get("macroFingerprint"),
        "macroDescriptor":descriptor,
        "nativeMetadata":{"path":"docs/apple-runtime/macos-27/builtin-actions.json",
                          "sha256":hashlib.sha256((ROOT/"docs/apple-runtime/macos-27/builtin-actions.json").read_bytes()).hexdigest(),
                          "platform":"macOS 27 metadata; does not establish iPad runtime"},
        "scope":["literal bounded GET body-next traversal","typed original Dictionary item references in order",
                 "maxPages native GET invocations; zero budgets emit no GET","maxItems bounded returned prefix",
                 "exact resolved URL spelling SHA256 cycle prevention","fixed diagnostics omit URLs/body/header data",
                 "same-origin absolute next URL or simple sibling filename",
                  "optional native HTTP header dictionary, literal and authored dynamic Text leaves; actual loopback request observation"],
        "unsupported":["HTTP status success verification","Link headers/page/offset/cursor expression/trailing block API",
                       "retry/backoff/catch/cancellation/error classification","transport timeout/redirect cap",
                       "strict external JSON array-vs-scalar validation","unbounded/general authenticated pagination"],
        "nativeAssumptionsStillRequiringSignedRuntime":[
            "Content Graph Count Items, empty/null and singleton collection behavior",
            "Get Contents of URL to Dictionary conversion, real HTTP non-2xx/transport errors and cancellation",
            "runtime Get Type witnesses, native Text hashing UTF-8 and key lookup semantics",
            "Repeat magic variable scoping and native Dictionary identity/nested types across Append",
            "native GET may follow redirects; maxPages does not limit redirect hops or wall-clock duration",
            "404 with a valid page body passes the body-only model; this is an exposed limitation, not HTTP success",
            "Alert plus Stop is a semantic stop, not a catchable exception or proven ERROR callback"
        ],
        "summary":{"cliCompiles":len(checks),"exactNativeActionRoundtrips":len(checks),
                   "independentModelCases":sum(len(c["models"]) for c in checks),"cliNegatives":len(negatives),
                   "causalFaults":len(faults),"nativeExecuted":0,"elapsedSeconds":round(time.monotonic()-started,3)},
        "compiles":checks,"cliNegatives":negatives,"causalFaults":faults,
    }
    destination.write_bytes((json.dumps(report,ensure_ascii=False,indent=2)+"\n").encode("utf-8"))
    print(json.dumps({"report":args.report,"status":report["status"],**report["summary"]},sort_keys=True))


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        location="docs/apple-runtime/p10r15/paginate-validation.json"
        if "--report" in sys.argv:
            location=sys.argv[sys.argv.index("--report")+1]
        target=ROOT/location;target.parent.mkdir(parents=True,exist_ok=True)
        target.write_text(json.dumps({"status":"FAIL_SOURCE_VALIDATION","exceptionType":type(error).__name__,"nativeExecuted":0})+"\n",encoding="utf-8")
        raise
