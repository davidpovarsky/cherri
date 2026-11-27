#!/usr/bin/env python3
"""Execute actual typed paginate CLI graphs against the existing loopback model.

PASS_MODEL is source/wire evidence, never Apple runtime acceptance. No source is
rewritten into a favorable result and no native endpoint is contacted. The
shared model supplies explicit native cardinality/type assumptions; this file
adds source provenance observations and a separately labeled Boolean/Number
alias simulation for the pre-request capability guard.
"""
import argparse
import copy
import hashlib
import json
import pathlib
import plistlib
import subprocess
import time

from verify_paginate import (
    ROOT, PREFIX, PageServer, PaginateGraphModel, GuardStop, Value,
    checked, extract_paginate_graph, fingerprint, native_value,
)

KINDS = {"dictionary": "Dictionary", "text": "Text", "number": "Number", "bool": "Bool"}
SOURCE_TYPES = {"dictionary": "Map<Text, AnyContent>", "text": "Text", "number": "Number", "bool": "Bool"}


def typed(value):
    if value.kind == "List":
        return ["List", [typed(member) for member in value.data]]
    if value.kind == "Dictionary":
        return ["Dictionary", {key: typed(member) for key, member in value.data.items()}]
    return [value.kind, value.data]


class SelectionModel(PaginateGraphModel):
    def __init__(self, server, *, boolean_number_alias=False, consumer=None):
        super().__init__(server)
        self.boolean_number_alias = boolean_number_alias
        self.consumer = consumer
        self.consumed = []
        self.pages = []

    def remember(self, value):
        # Observe original body objects without changing their kind or identity.
        self.pages.append(value)
        def visit(member):
            if member.kind == "Dictionary":
                self.source_dictionaries.add(id(member))
                for child in member.data.values():
                    visit(child)
            elif member.kind == "List":
                for child in member.data:
                    visit(child)
        visit(value)
        return value

    def value(self, raw):
        value = super().value(raw)
        if self.boolean_number_alias and value.kind == "Bool":
            # Explicit unfavorable platform simulation; not native evidence.
            value = Value("Number", int(value.data))
        if self.consumer is not None and raw is self.consumer:
            self.consumed.append(value)
        return value


def inspect(graph, server, case, kind, key, *, alias=False, consumer=None, result_name=None):
    server.routes, server.requests, server.request_headers = case["routes"], [], []
    model = SelectionModel(server, boolean_number_alias=alias, consumer=consumer)
    output, observed = None, None
    try:
        output = model.run(graph)
    except GuardStop as stopped:
        observed = str(stopped)
    assert observed == case.get("guard"), f"fixed guard changed: {observed!r} != {case.get('guard')!r}"
    assert server.requests == case["requests"], "request count/order changed"
    if observed is None:
        if result_name is not None:
            output = model.variables[result_name]
        expected = case["expected"]
        assert output.kind == "List", "typed aggregate lost its native List"
        assert [typed(value) for value in output.data] == [typed(native_value(value)) for value in expected], "exact typed values/order changed"
        assert all(value.kind == KINDS[kind] for value in output.data), "declared element kind was not preserved"
        selected = []
        for page in model.pages:
            assert page.kind == "Dictionary" and page.data["items"].kind == "List"
            for item in page.data["items"].data:
                selected.append(item.data[key] if key else item)
        selected = selected[:len(expected)]
        assert len(selected) == len(expected)
        assert [id(value) for value in output.data] == [id(value) for value in selected], "copied, serialized, coerced or reordered a source reference"
        if consumer is not None:
            assert [typed(value) for value in model.consumed] == [typed(native_value(value)) for value in expected], "downstream typed loop did not consume every selected value"
            assert [id(value) for value in model.consumed] == [id(value) for value in selected], "downstream consumer detached from selected references"
    else:
        assert output is None, "guard returned a partial typed aggregate"
    return {
        "status": "PASS_MODEL", "fixedGuard": observed,
        "requests": list(server.requests), "returnedItems": None if output is None else len(output.data),
        "consumedItems": len(model.consumed), "originalReferences": observed is None,
        "booleanNumberAliasSimulation": alias, "modelExecutedActions": model.executed,
    }


def values(kind):
    return {
        "dictionary": [{"id": 1, "false": False, "zero": 0, "empty": "",
                        "nested": {"flags": [False, True], "emptyList": [], "emptyMap": {}}},
                       {}, {"id": 3, "list": [{"name": "A"}, {"name": "a"}]}],
        "text": ["A", 'שָׁלוֹם 🍒\n"\\', "a"],
        "number": [0, -7, 1.25],
        "bool": [False, True, False],
    }[kind]


def cases(kind, key):
    items = values(kind)
    def row(value):
        return {key: value, "other": "SYNTHETIC_OTHER"} if key else value
    def page(entries, after=None):
        body = {"items": [row(value) for value in entries]}
        if after is not None:
            body["next"] = after
        return (200, body)
    def one(entries, expected=None, guard=None):
        return {"routes": {"/entry.json": page(entries)}, "expected": entries if expected is None else expected,
                "requests": ["/entry.json"], "guard": guard}
    wrong = {"dictionary": "text", "text": 0, "number": "0", "bool": 0}[kind]
    diagnostic = "PAGINATE_ITEM_" + kind.upper() + "_REQUIRED"
    result = {
        "ordered-three-pages-middle-empty": {
            "routes": {"/entry.json": page(items[:2], "two.json"), "/two.json": page([], "three.json"), "/three.json": page(items[2:])},
            "expected": items, "requests": ["/entry.json", "/two.json", "/three.json"]},
        "empty-items": one([]),
        "duplicates-retain-order": one([items[0], items[1], items[0]]),
        "wrong-type": one([wrong], guard=diagnostic),
        "null-rejected": one([None], guard=diagnostic),
        "empty-list-rejected": one([[]], guard=diagnostic),
        "singleton-list-not-a-scalar": one([[items[0]]], guard=diagnostic),
        "multiple-list-values-rejected": one([[items[0], items[1]]], guard=diagnostic),
        "partial-page-rejected": one([items[0], wrong], guard=diagnostic),
        "item-cap-no-extra-request": {
            "routes": {"/entry.json": page((items * 3)[:7], "two.json"), "/two.json": page(items)},
            "expected": (items * 3)[:6], "requests": ["/entry.json"]},
    }
    if kind == "bool":
        result["number-one-is-not-bool"] = one([1], guard=diagnostic)
        result["text-false-is-not-bool"] = one(["false"], guard=diagnostic)
    if kind == "text":
        # This is the shared model's explicit empty-Text assumption. It is
        # reported separately, not promoted to an Apple cardinality guarantee.
        result["empty-text-model-assumption"] = one(["", "A", ""])
    if kind == "number":
        result["bool-false-is-not-number"] = one([False], guard=diagnostic)
        result["bool-true-is-not-number"] = one([True], guard=diagnostic)
    if key:
        for name, item in [
            ("missing-selected-key", {"other": items[0]}),
            ("case-wrong-selected-key", {"Payload": items[0]}),
        ]:
            result[name] = {"routes": {"/entry.json": (200, {"items": [item]})},
                            "requests": ["/entry.json"], "guard": "PAGINATE_ITEM_KEY_REQUIRED"}
        result["selected-source-not-dictionary"] = {
            "routes": {"/entry.json": (200, {"items": ["not-a-dictionary"]})},
            "requests": ["/entry.json"], "guard": "PAGINATE_ITEM_DICTIONARY_REQUIRED"}
    return result


def compile_graph(exe, output, identity, code, legacy, profile):
    source, target = output / (identity + ".cherri"), output / (identity + ".shortcut")
    source.write_text(code, encoding="utf-8")
    checked([exe, "compile", source, "-o", target, "--skip-sign", "--no-ansi", "--build-profile=" + profile] + (["--legacy"] if legacy else []), identity)
    graph = plistlib.loads(target.read_bytes())["WFWorkflowActions"]
    imported, rebuilt = output / (identity + "-import.cherri"), output / (identity + "-rebuilt.shortcut")
    checked([exe, "import", target, "--mode=v2", "--lossless", "-o", imported], identity + " import")
    checked([exe, "compile", imported, "-o", rebuilt, "--skip-sign", "--no-ansi"], identity + " rebuild")
    assert graph == plistlib.loads(rebuilt.read_bytes())["WFWorkflowActions"], "actual lossless roundtrip changed the typed graph"
    return graph, {
        "id": identity, "status": "PASS", "sourceSha256": hashlib.sha256(source.read_bytes()).hexdigest(),
        "artifactSha256": hashlib.sha256(target.read_bytes()).hexdigest(),
        "actionCount": len(graph), "exactNativeActionRoundtrip": True,
    }


def call(origin, kind, key, pages=3, items=6):
    return f'paginate("{origin}/entry.json", "items", "next", {pages}, {items}, nil, "url", nil, {json.dumps(key) if key else "nil"}, "{kind}")'


def params(graph, suffix):
    return [action.setdefault("WFWorkflowActionParameters", {}) for action in graph if action["WFWorkflowActionIdentifier"] == PREFIX + suffix]


def output_ref(p):
    return {"WFSerializationType": "WFTextTokenAttachment",
            "Value": {"Type": "ActionOutput", "OutputUUID": p["UUID"], "OutputName": p["CustomOutputName"]}}


def fault_control(base, server, kind, key, case, mutate, *, alias=False):
    altered = copy.deepcopy(base)
    mutate(altered)
    assert altered != base, "causal control did not mutate the actual graph"
    try:
        inspect(altered, server, case, kind, key, alias=alias)
    except (AssertionError, KeyError, ValueError):
        pass
    else:
        raise AssertionError("causal corruption was accepted")
    inspect(base, server, case, kind, key, alias=alias)
    return {"negativeRejected": True, "restoredControl": "PASS_MODEL"}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--compiler", type=pathlib.Path, required=True, help="An actual compiler built from this worktree")
    parser.add_argument("--output-dir", type=pathlib.Path, required=True)
    parser.add_argument("--report", type=pathlib.Path, required=True)
    args = parser.parse_args()
    exe, output, report = args.compiler.resolve(), args.output_dir.resolve(), args.report.resolve()
    output.mkdir(parents=True, exist_ok=True)
    report.parent.mkdir(parents=True, exist_ok=True)
    before = fingerprint()
    this_hash = hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest()
    receipt = {
        "status": "RUNNING_SOURCE_VALIDATION", "nativeExecuted": 0,
        "sourceCommit": checked(["git", "rev-parse", "HEAD"], "source identity").stdout.strip(),
        "sourceDirty": bool(checked(["git", "status", "--porcelain"], "source state").stdout.strip()),
        "compilerSha256": hashlib.sha256(exe.read_bytes()).hexdigest(),
        "inputFingerprint": before, "verifierSha256": this_hash,
        "checks": [], "causalControls": [],
        "nativeLimits": [
            "Apple native type/category and Content Graph singleton/null/empty behavior remain unverified.",
            "The shared model preserves empty Text as one item; no corresponding native guarantee is asserted.",
            "Boolean and Number modes require native Boolean and Number type witnesses to be distinguishable; alias simulation must stop before GET.",
            "Nested List/Optional/JsonNull and mixed result types have no public selection codec and are rejected.",
            "No Apple HTTP status/error/retry/cancellation/timeout or physical iPad acceptance is provided.",
        ],
    }
    report.write_text(json.dumps(receipt, indent=2) + "\n")
    started = time.monotonic()
    graphs = {}
    with PageServer() as server:
        for legacy in (False, True):
            for profile in ("debug", "release"):
                for kind in KINDS:
                    for key in ("", "payload"):
                        identity = f'{"legacy" if legacy else "v2"}-{profile}-{kind}-{"field" if key else "item"}'
                        actions, entry = compile_graph(exe, output, identity, call(server.origin, kind, key) + "\n", legacy, profile)
                        graph, boundary = extract_paginate_graph(actions, legacy=legacy)
                        entry["mainOutputBoundary"] = boundary
                        entry["models"] = {name: inspect(graph, server, case, kind, key) for name, case in cases(kind, key).items()}
                        if kind in {"bool", "number"}:
                            alias = {"routes": {}, "requests": [], "guard": "PAGINATE_" + kind.upper() + "_TYPE_UNAVAILABLE"}
                            entry["models"]["boolean-number-alias-before-get"] = inspect(graph, server, alias, kind, key, alias=True)
                        receipt["checks"].append(entry)
                        graphs[(legacy, profile, kind, key)] = graph
        for legacy in (False, True):
            for kind in KINDS:
                for pages, items, profile in ((0, 6, "debug"), (3, 0, "release")):
                    identity = f'{"legacy" if legacy else "v2"}-{kind}-zero-{"pages" if pages == 0 else "items"}'
                    actions, entry = compile_graph(exe, output, identity, call(server.origin, kind, "payload", pages, items) + "\n", legacy, profile)
                    graph, boundary = extract_paginate_graph(actions, legacy=legacy)
                    assert len(graph) == 3, "zero cap must emit no runtime type guard, callback or GET"
                    entry["mainOutputBoundary"] = boundary
                    entry["models"] = {"zero-work": inspect(graph, server, {"routes": {}, "requests": [], "expected": []}, kind, "payload", alias=True)}
                    receipt["checks"].append(entry)
        for profile in ("debug", "release"):
            for kind in KINDS:
                identity = f"v2-{profile}-{kind}-downstream-typed-loop"
                code = f'let rows: List<{SOURCE_TYPES[kind]}> = {call(server.origin, kind, "payload")}\nfor row in rows {{ let checked: {SOURCE_TYPES[kind]} = row\ngetVariable(checked)\n}}\n'
                actions, entry = compile_graph(exe, output, identity, code, False, profile)
                assert actions[-1]["WFWorkflowActionIdentifier"] == PREFIX + "nothing" and actions[-1].get("WFWorkflowActionParameters", {}) == {}
                graph = actions[:-1]
                tail = graph[-5:]
                assert [a["WFWorkflowActionIdentifier"].removeprefix(PREFIX) for a in tail] == ["repeat.each", "setvariable", "getvariable", "getvariable", "repeat.each"]
                prefix, _ = extract_paginate_graph(graph[:-5], legacy=True)
                result = prefix[-1]["WFWorkflowActionParameters"]
                assert tail[0]["WFWorkflowActionParameters"]["WFInput"] == output_ref(result), "downstream loop lost actual paginate result"
                assert tail[0]["WFWorkflowActionParameters"]["GroupingIdentifier"] == tail[-1]["WFWorkflowActionParameters"]["GroupingIdentifier"]
                assert tail[0]["WFWorkflowActionParameters"]["WFControlFlowMode"] == 0 and tail[-1]["WFWorkflowActionParameters"]["WFControlFlowMode"] == 2
                consumer = tail[-2]["WFWorkflowActionParameters"]["WFVariable"]
                result_name = result["WFVariable"]["Value"]["VariableName"]
                entry["models"] = {"actual-typed-loop": inspect(graph, server, cases(kind, "payload")["ordered-three-pages-middle-empty"], kind, "payload", consumer=consumer, result_name=result_name)}
                receipt["checks"].append(entry)
        text = graphs[(False, "release", "text", "payload")]
        text_cases = cases("text", "payload")
        def replace_append(g):
            start = next(p for p in params(g, "gettext") if p.get("WFTextActionText") == server.origin + "/entry.json")
            next(p for p in params(g, "appendvariable") if p["WFVariableName"].endswith("_results"))["WFInput"] = output_ref(start)
        def change_key(g):
            next(p for p in params(g, "getvalueforkey") if p.get("WFDictionaryKey") == "payload")["WFDictionaryKey"] = "other"
        def reverse_guard(g, diagnostic, field):
            for index, action in enumerate(g):
                if action.get("WFWorkflowActionParameters", {}).get("WFAlertActionMessage") == diagnostic:
                    p = g[index - 1]["WFWorkflowActionParameters"]
                    if field in p:
                        p["WFCondition"] = 4 if p["WFCondition"] == 5 else 5
                        return
            raise AssertionError("guard not found")
        def drop_type_stop(g):
            for index, action in enumerate(g):
                if action.get("WFWorkflowActionParameters", {}).get("WFAlertActionMessage") == "PAGINATE_ITEM_TEXT_REQUIRED" and "WFConditionalActionString" in g[index - 1]["WFWorkflowActionParameters"]:
                    name = next(p["WFVariableName"] for p in params(g, "setvariable") if p["WFVariableName"].endswith("_results"))
                    g[index + 1] = {"WFWorkflowActionIdentifier": PREFIX + "getvariable", "WFWorkflowActionParameters": {
                        "WFVariable": {"WFSerializationType": "WFTextTokenAttachment", "Value": {"Type": "Variable", "VariableName": name}}}}
                    return
            raise AssertionError("type stop not found")
        for name, scenario, mutate in [
            ("original-reference-replaced", "ordered-three-pages-middle-empty", replace_append),
            ("required-key-changed", "ordered-three-pages-middle-empty", change_key),
            ("selected-key-presence-bypassed", "missing-selected-key", lambda g: reverse_guard(g, "PAGINATE_ITEM_KEY_REQUIRED", "WFNumberValue")),
            ("selected-type-predicate-reversed", "wrong-type", lambda g: reverse_guard(g, "PAGINATE_ITEM_TEXT_REQUIRED", "WFConditionalActionString")),
            ("selected-type-stop-removed", "wrong-type", drop_type_stop),
        ]:
            receipt["causalControls"].append({"id": name, **fault_control(text, server, "text", "payload", text_cases[scenario], mutate)})
        for kind in ("bool", "number"):
            graph = graphs[(False, "release", kind, "payload")]
            diagnostic = "PAGINATE_" + kind.upper() + "_TYPE_UNAVAILABLE"
            alias = {"routes": {"/entry.json": (200, {"items": [{"payload": False}]})}, "requests": [], "guard": diagnostic}
            receipt["causalControls"].append({"id": kind + "-number-alias-guard-bypassed", **fault_control(
                graph, server, kind, "payload", alias,
                lambda g: reverse_guard(g, diagnostic, "WFConditionalActionString"), alias=True)})
    assert fingerprint() == before, "source changed while producing evidence"
    assert hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest() == this_hash, "verifier changed during execution"
    receipt.update(status="PASS_SOURCE_AND_MODEL_NATIVE_PENDING", nativeExecuted=0, sourceStable=True,
                   elapsedSeconds=time.monotonic() - started,
                   compiledGraphs=len(receipt["checks"]),
                   modelChecks=sum(len(entry["models"]) for entry in receipt["checks"]))
    report.write_text(json.dumps(receipt, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(json.dumps({key: receipt[key] for key in ("status", "nativeExecuted", "compiledGraphs", "modelChecks", "elapsedSeconds")}, indent=2))


if __name__ == "__main__":
    main()
