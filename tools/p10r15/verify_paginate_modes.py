#!/usr/bin/env python3
"""Compile marker traversal modes and check their actual graphs over loopback HTTP.

The evaluator is a declared model. It does not grant Apple runtime acceptance.
"""
import argparse
import copy
import hashlib
import json
import pathlib
import plistlib
import time
import urllib.parse

import verify_paginate as base
from paginate_marker_model import NATIVE_ACTIONS, inspect_marker_graph

ROOT = pathlib.Path(__file__).resolve().parents[2]
PREFIX = base.PREFIX
AUTH = {"Authorization": "Bearer PAGINATE_SYNTHETIC_ONLY", "X-Cherri-Client": "Cherri-P15"}
ITEM = {"id": 1, "false": False, "zero": 0, "empty": "", "list": [], "map": {},
        "nested": [{"flag": False, "values": [0, "", [], {}]}, []]}


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def source_fingerprint():
    extra = [pathlib.Path(__file__).resolve(), ROOT / "tools/p10r15/paginate_marker_model.py"]
    return hashlib.sha256((base.fingerprint() + "".join(digest(p) for p in extra)).encode()).hexdigest()


def wire_edges(graph):
    owners, stack = {}, []
    def visit(value):
        if isinstance(value, dict):
            if value.get("Type") == "ActionOutput":
                ident = value.get("OutputUUID")
                assert ident in owners, "missing preceding producer"
                if "OutputName" in value:
                    assert value["OutputName"] == owners[ident], "reference output name changed"
            for child in value.values():
                visit(child)
        elif isinstance(value, list):
            for child in value:
                visit(child)
    for action in graph:
        p = action.get("WFWorkflowActionParameters", {})
        visit(p)
        if "UUID" in p:
            assert p["UUID"] and p["UUID"] not in owners, "duplicate producer identity"
            owners[p["UUID"]] = p.get("CustomOutputName")
        if "GroupingIdentifier" in p:
            group, mode = p["GroupingIdentifier"], p["WFControlFlowMode"]
            if mode == 0:
                assert group not in stack
                stack.append(group)
            else:
                assert stack and stack[-1] == group, "control flow group order changed"
                if mode == 2:
                    stack.pop()
                else:
                    assert mode == 1 and action["WFWorkflowActionIdentifier"] == PREFIX + "conditional"
    assert not stack, "unterminated native group"


def spec(mode, flavor="base"):
    result = {"mode": mode, "key": mode, "pages": 3, "items": 6, "dynamic": False,
              "first": "/items?keep=%2F+a&filter=z", "prefix": "/items?keep=%2F+a&filter=z&" + mode + "=", "suffix": "",
              "initial": None}
    if mode in {"page", "offset"}:
        initial = 1 if mode == "page" else 0
        result.update(first="/items?" + mode + "=" + str(initial) + "&keep=%2F+a&filter=z",
                      prefix="/items?" + mode + "=", suffix="&keep=%2F+a&filter=z", initial=initial)
    if flavor == "initial":
        assert mode == "cursor"
        result.update(first="/items?cursor=seed%2b%2f&keep=%2F+a&filter=z",
                      prefix="/items?cursor=", suffix="&keep=%2F+a&filter=z", initial="seed+/")
    if flavor == "custom":
        assert mode == "cursor"
        result.update(key="page[cursor]", prefix="/items?keep=%2F+a&filter=z&page%5Bcursor%5D=")
    if flavor == "headers":
        result["dynamic"] = True
    if flavor == "page-cap":
        result["pages"] = 1
    if flavor == "item-cap":
        result["items"] = 1
    if flavor == "zero-pages":
        result["pages"] = 0
    if flavor == "zero-items":
        result["items"] = 0
    return result


def marker_path(cfg, marker):
    text = urllib.parse.quote(marker, safe="-._~") if cfg["mode"] == "cursor" else str(marker)
    return cfg["prefix"] + text + cfg["suffix"]


def cases(cfg):
    first, mode = cfg["first"], cfg["mode"]
    second = "Ab+/= שלום" if mode == "cursor" else (2 if mode == "page" else 10)
    third = "Next%2F" if mode == "cursor" else (3 if mode == "page" else 20)
    p2, p3 = marker_path(cfg, second), marker_path(cfg, third)
    def case(routes, expected, requests, guard=None, **options):
        return {"routes": routes, "expected": expected, "requests": requests, "guard": guard, **options}
    rows = {
        "single-absent": case({first: (200, {"items": [ITEM]})}, [ITEM], [first]),
        "null-stop": case({first: (200, {"items": [ITEM], "next": None})}, [ITEM], [first]),
        "empty-container-stop": case({first: (200, {"items": [ITEM], "next": []})}, [ITEM], [first]),
        "key-case-exact": case({first: (200, {"items": [ITEM], "Next": second})}, [ITEM], [first]),
        "three-pages-middle-empty": case({
            first: (200, {"items": [ITEM], "next": second}),
            p2: (200, {"items": [], "next": third}),
            p3: (200, {"items": [{"id": 3}]})}, [ITEM, {"id": 3}], [first, p2, p3]),
        "three-request-cap": case({
            first: (200, {"items": [ITEM], "next": second}),
            p2: (200, {"items": [{"id": 2}], "next": third}),
            p3: (200, {"items": [{"id": 3}], "next": "four" if mode == "cursor" else 40}),
            marker_path(cfg, "four" if mode == "cursor" else 40): (200, {"items": [{"id": 4}]})},
            [ITEM, {"id": 2}, {"id": 3}], [first, p2, p3]),
        "full-items-no-next-evaluation": case({first: (200, {"items": [ITEM] * 6, "next": {"invalid": True}})}, [ITEM] * 6, [first]),
        "http404-body-only-limitation": case({first: (404, {"items": [ITEM]})}, [ITEM], [first]),
    }
    required = "PAGINATE_NEXT_TEXT_REQUIRED" if mode == "cursor" else "PAGINATE_NEXT_NUMBER_REQUIRED"
    for name, value in [("wrong-bool", False), ("wrong-map", {"a": 1}), ("wrong-many", [1, 2]),
                        ("wrong-scalar", 2 if mode == "cursor" else "2")]:
        rows[name] = case({first: (200, {"items": [ITEM], "next": value})}, None, [first], required)
    cycle_marker = cfg["initial"] if cfg["initial"] is not None else second
    if cfg["initial"] is not None:
        rows["initial-cycle"] = case({first: (200, {"items": [ITEM], "next": cycle_marker})}, [ITEM], [first])
    else:
        rows["repeated-marker-cycle"] = case({
            first: (200, {"items": [ITEM], "next": second}),
            p2: (200, {"items": [{"id": 2}], "next": second})}, [ITEM, {"id": 2}], [first, p2])
    if mode == "cursor":
        rows["empty-text-stop"] = case({first: (200, {"items": [ITEM], "next": ""})}, [ITEM], [first])
        for name, token in [("reserved-cursor", "https://other.invalid/a?x=1&y=2#f"),
                            ("percent-literal", "%2F"), ("unicode-and-controls", "א🍒+\n"),
                            ("case-A", "A"), ("case-a", "a")]:
            rows[name] = case({first: (200, {"items": [ITEM], "next": token}),
                              marker_path(cfg, token): (200, {"items": [{"id": 2}]})},
                             [ITEM, {"id": 2}], [first, marker_path(cfg, token)])
        rows["case-distinct-cycle-keys"] = case({
            first: (200, {"items": [ITEM], "next": "A"}),
            marker_path(cfg, "A"): (200, {"items": [{"id": 2}], "next": "a"}),
            marker_path(cfg, "a"): (200, {"items": [{"id": 3}]})},
            [ITEM, {"id": 2}, {"id": 3}], [first, marker_path(cfg, "A"), marker_path(cfg, "a")])
        rows["unsafe-native-encoder-guard"] = case({first: (200, {"items": [ITEM], "next": "A&B"})},
                                                 None, [first], "PAGINATE_CURSOR_ENCODING_INVALID",
                                                 cursor_encoding="unsafe-reserved")
        rows["lossy-native-encoder-guard"] = case({first: (200, {"items": [ITEM], "next": "שלום"})},
                                                None, [first], "PAGINATE_CURSOR_ENCODING_CHANGED",
                                                cursor_encoding="lossy-unicode")
    else:
        for name, value, guard in [
            ("negative", -1, "PAGINATE_NEXT_INTEGER_RANGE"),
            ("overflow", 9007199254740992, "PAGINATE_NEXT_INTEGER_RANGE"),
            ("fraction", 1.5, "PAGINATE_NEXT_INTEGER_REQUIRED"),
        ]:
            rows[name] = case({first: (200, {"items": [ITEM], "next": value})}, None, [first], guard)
        rows["maximum-exact-integer"] = case({
            first: (200, {"items": [ITEM], "next": 9007199254740991}),
            marker_path(cfg, 9007199254740991): (200, {"items": [{"id": 2}]})},
            [ITEM, {"id": 2}], [first, marker_path(cfg, 9007199254740991)])
        rows["integer-valued-number"] = case({
            first: (200, {"items": [ITEM], "next": 2.0}),
            marker_path(cfg, 2): (200, {"items": [{"id": 2}]})},
            [ITEM, {"id": 2}], [first, marker_path(cfg, 2)])
        if mode == "page":
            rows["zero-page-rejected"] = case({first: (200, {"items": [ITEM], "next": 0})},
                                              None, [first], "PAGINATE_NEXT_INTEGER_RANGE")
        rows["noncanonical-number-text"] = case({first: (200, {"items": [ITEM], "next": 1000})},
                                                None, [first], "PAGINATE_NEXT_INTEGER_TEXT_REQUIRED",
                                                number_format="grouped")
        rows["exponent-number-text"] = case({first: (200, {"items": [ITEM], "next": 2})},
                                            None, [first], "PAGINATE_NEXT_INTEGER_TEXT_REQUIRED",
                                            number_format="exponent")
    return rows


def source(cfg, origin, legacy):
    prefix, headers = "", "{}"
    if cfg["dynamic"]:
        decl = "const" if legacy else "let"
        prefix = decl + " authorization = text(" + json.dumps(AUTH["Authorization"]) + ")\n"
        auth = '"{authorization}"' if legacy else "authorization"
        headers = '{"Authorization": ' + auth + ', "X-Cherri-Client": ' + json.dumps(AUTH["X-Cherri-Client"]) + '}'
    args = [json.dumps(origin + cfg["first"]), '"items"', '"next"', str(cfg["pages"]),
            str(cfg["items"]), headers, json.dumps(cfg["mode"]), json.dumps(cfg["key"])]
    return prefix + "paginate(" + ", ".join(args) + ")\n"


def run_case(graph, server, cfg, row):
    return inspect_marker_graph(graph, server, **row, headers=AUTH if cfg["dynamic"] else None)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--report", required=True)
    parser.add_argument("--compiler", help="An already-built compiler for this exact source, when explicitly supplied by its owner")
    args = parser.parse_args()
    out, report_path = pathlib.Path(args.output_dir), pathlib.Path(args.report)
    out.mkdir(parents=True, exist_ok=False)
    report_path.parent.mkdir(parents=True, exist_ok=True)
    assert not report_path.exists()
    before = source_fingerprint()
    head = base.checked(["git", "rev-parse", "HEAD"], "source").stdout.strip()
    assert not base.checked(["git", "status", "--porcelain"], "clean source").stdout.strip()
    exe = pathlib.Path(args.compiler) if args.compiler else out / "cherri"
    started = time.monotonic()
    if not args.compiler:
        base.checked(["go", "build", "-trimpath", "-o", exe, "."], "compiler build")
    caps = json.loads(base.checked([exe, "--capabilities-json"], "capabilities").stdout)
    assert caps["actionCount"] == 505
    checks, graphs, faults, negatives = [], {}, [], []
    with base.PageServer() as server:
        fixtures = []
        for legacy in (False, True):
            for profile in ("debug", "release"):
                fixtures.extend((legacy, profile, mode, "base") for mode in ("cursor", "page", "offset"))
                fixtures.extend((legacy, profile, "cursor", flavor) for flavor in ("initial", "custom"))
                fixtures.extend((legacy, profile, mode, "headers") for mode in ("cursor", "page", "offset"))
        fixtures.extend((False, "debug", mode, flavor) for mode in ("cursor", "page", "offset")
                        for flavor in ("page-cap", "item-cap", "zero-pages", "zero-items"))
        assert len(fixtures) == 44
        for legacy, profile, mode, flavor in fixtures:
            cfg = spec(mode, flavor)
            ident = f"{'legacy' if legacy else 'v2'}-{profile}-{mode}-{flavor}"
            src, target = out / (ident + ".cherri"), out / (ident + ".shortcut")
            src.write_text(source(cfg, server.origin, legacy), encoding="utf-8")
            command = [exe, "compile", src, "-o", target, "--skip-sign", "--no-ansi", "--build-profile", profile]
            if legacy:
                command.append("--legacy")
            base.checked(command, ident)
            graph = plistlib.loads(target.read_bytes())["WFWorkflowActions"]
            wire_edges(graph)
            extractor = base.extract_header_paginate_graph if cfg["dynamic"] else base.extract_paginate_graph
            macro, boundary = extractor(graph, legacy=legacy, native_actions=NATIVE_ACTIONS)
            imported, rebuilt = out / (ident + "-import.cherri"), out / (ident + "-rebuilt.shortcut")
            base.checked([exe, "import", target, "--mode=v2", "--lossless", "-o", imported], ident + " import")
            base.checked([exe, "compile", imported, "-o", rebuilt, "--skip-sign", "--no-ansi"], ident + " rebuild")
            assert graph == plistlib.loads(rebuilt.read_bytes())["WFWorkflowActions"], "native roundtrip changed"
            available, results = cases(cfg), {}
            if flavor in {"zero-pages", "zero-items"}:
                results["no-request"] = inspect_marker_graph(macro, server, {}, [], [])
                assert len(macro) == 3
            elif flavor == "page-cap":
                row = copy.deepcopy(available["three-pages-middle-empty"])
                row.update(expected=[ITEM], requests=[cfg["first"]])
                results["one-request-cap"] = run_case(macro, server, cfg, row)
            elif flavor == "item-cap":
                row = copy.deepcopy(available["three-pages-middle-empty"])
                row.update(expected=[ITEM], requests=[cfg["first"]])
                results["one-item-cap"] = run_case(macro, server, cfg, row)
            elif flavor == "headers":
                results["three-pages-actual-headers"] = run_case(macro, server, cfg, available["three-pages-middle-empty"])
            else:
                for name, row in available.items():
                    results[name] = run_case(macro, server, cfg, row)
            (out / (ident + "-actions.json")).write_text(json.dumps(graph, ensure_ascii=False, indent=2) + "\n")
            graphs[(legacy, profile, mode, flavor)] = (macro, cfg)
            checks.append({"id": ident, "status": "PASS", "sourceSHA256": digest(src), "shortcutSHA256": digest(target),
                           "roundtripSHA256": digest(rebuilt), "exactNativeRoundtrip": True, "mainBoundary": boundary,
                           "models": results, "actionCount": len(graph)})

        def params(g, suffix):
            return [a["WFWorkflowActionParameters"] for a in g if a["WFWorkflowActionIdentifier"] == PREFIX + suffix]
        def guard_condition(g, diagnostic):
            indexes = [i for i, a in enumerate(g) if a.get("WFWorkflowActionParameters", {}).get("WFAlertActionMessage") == diagnostic]
            assert len(indexes) == 1
            condition = g[indexes[0]-1]["WFWorkflowActionParameters"]
            assert condition.get("WFControlFlowMode") == 0
            return condition
        def mutate(g, name):
            if name == "encode-as-decode":
                next(p for p in params(g,"urlencode") if p["WFEncodeMode"] == "Encode")["WFEncodeMode"] = "Decode"
            elif name == "decoded-hash-self-comparison":
                p = guard_condition(g, "PAGINATE_CURSOR_ENCODING_CHANGED")
                p["WFConditionalActionString"]["Value"]["attachmentsByRange"]["{0, 1}"] = copy.deepcopy(p["WFInput"]["Variable"]["Value"])
            elif name == "cursor-safe-component-guard-inverted":
                guard_condition(g,"PAGINATE_CURSOR_ENCODING_INVALID")["WFCondition"] = 4
            elif name == "initial-marker-not-recorded":
                p = next(p for p in params(g,"appendvariable") if p["WFVariableName"].endswith("_seenMarkers"))
                p["WFVariableName"] = p["WFVariableName"].removesuffix("seenMarkers") + "seen"
            elif name == "wrong-round-input":
                one = next(p for p in params(g,"number") if p["WFNumberActionNumber"] == "1")
                params(g,"round")[0]["WFInput"] = {"WFSerializationType":"WFTextTokenAttachment",
                    "Value":{"Type":"ActionOutput","OutputUUID":one["UUID"],"OutputName":one["CustomOutputName"]}}
            elif name == "numeric-max-guard-bypassed":
                for i,a in enumerate(g):
                    p=a.get("WFWorkflowActionParameters",{})
                    if p.get("WFAlertActionMessage") == "PAGINATE_NEXT_INTEGER_RANGE" and g[i-1]["WFWorkflowActionParameters"].get("WFCondition") == 2:
                        g[i-1]["WFWorkflowActionParameters"]["WFCondition"] = 0
                        break
                else: raise AssertionError("upper bound missing")
            elif name == "page-zero-guard-bypassed":
                for i,a in enumerate(g):
                    p=a.get("WFWorkflowActionParameters",{})
                    if p.get("WFAlertActionMessage") == "PAGINATE_NEXT_INTEGER_RANGE" and g[i-1]["WFWorkflowActionParameters"].get("WFCondition") == 0:
                        g[i-1]["WFWorkflowActionParameters"]["WFNumberValue"] = 0
                        break
                else: raise AssertionError("page lower bound missing")
            elif name == "integer-equality-inverted":
                guard_condition(g,"PAGINATE_NEXT_INTEGER_REQUIRED")["WFCondition"] = 4
            elif name in {"query-key-changed","query-suffix-removed"}:
                for p in params(g,"gettext"):
                    value = p.get("WFTextActionText",{})
                    if isinstance(value,dict) and isinstance(value.get("Value"),dict):
                        text=value["Value"].get("string","")
                        if "/items?" in text and "\ufffc" in text:
                            value["Value"]["string"] = text.replace("cursor=", "otherx=",1) if name=="query-key-changed" else text.split("\ufffc")[0]+"\ufffc"
                            break
                else: raise AssertionError("query composition not found")
            elif name == "page-budget-raised":
                next(p for p in params(g,"repeat.count") if p.get("WFControlFlowMode")==0)["WFRepeatCount"] = 4
            elif name == "headers-dropped":
                del params(g,"downloadurl")[0]["WFHTTPHeaders"]
            else:
                raise AssertionError(name)

        for name, mode, flavor, case_name in [
            ("encode-as-decode","cursor","base","reserved-cursor"),
            ("decoded-hash-self-comparison","cursor","base","lossy-native-encoder-guard"),
            ("cursor-safe-component-guard-inverted","cursor","base","reserved-cursor"),
            ("initial-marker-not-recorded","cursor","initial","initial-cycle"),
            ("wrong-round-input","page","base","three-pages-middle-empty"),
            ("numeric-max-guard-bypassed","offset","base","overflow"),
            ("page-zero-guard-bypassed","page","base","zero-page-rejected"),
            ("integer-equality-inverted","page","base","three-pages-middle-empty"),
            ("query-key-changed","cursor","base","three-pages-middle-empty"),
            ("query-suffix-removed","page","base","three-pages-middle-empty"),
            ("page-budget-raised","cursor","base","three-request-cap"),
            ("headers-dropped","cursor","headers","three-pages-middle-empty"),
        ]:
            graph,cfg=graphs[(False,"debug",mode,flavor)]
            row=cases(cfg)[case_name]
            original=json.dumps(graph,sort_keys=True)
            broken=copy.deepcopy(graph);mutate(broken,name)
            assert json.dumps(broken,sort_keys=True)!=original,"fault was a no-op"
            try:
                run_case(broken,server,cfg,row)
            except (AssertionError, ValueError, KeyError, TypeError):
                pass
            else:
                raise AssertionError("causal graph fault was not rejected: "+name)
            restored=run_case(graph,server,cfg,row)
            fault_path=out/("fault-"+name+".json")
            fault_path.write_text(json.dumps(broken,ensure_ascii=False,indent=2)+"\n")
            faults.append({"id":name,"rejected":True,"mutantSHA256":digest(fault_path),"restoredControl":restored})

    invalid = {
        "unknown-mode": ('https://example.invalid/p','"link"'),
        "missing-page-marker": ('https://example.invalid/p','"page"'),
        "zero-page-marker": ('https://example.invalid/p?page=0','"page"'),
        "negative-offset": ('https://example.invalid/p?offset=-1','"offset"'),
        "ambiguous-cursor-key": ('https://example.invalid/p?cursor=a&%63ursor=b','"cursor"'),
        "invalid-utf8-cursor": ('https://example.invalid/p?cursor=%FF','"cursor"'),
        "fractional-page": ('https://example.invalid/p?page=1.5','"page"'),
        "overflow-offset": ('https://example.invalid/p?offset=9007199254740992','"offset"'),
    }
    for legacy in (False,True):
        for profile in ("debug","release"):
            for name,(url,mode) in invalid.items():
                ident=f"reject-{legacy}-{profile}-{name}"
                src,target=out/(ident+".cherri"),out/(ident+".shortcut")
                src.write_text("paginate("+json.dumps(url)+', "items", "next", 3, 6, {}, '+mode+")\n")
                result=base.run([exe,"compile",src,"-o",target,"--skip-sign","--no-ansi","--build-profile",profile]+(["--legacy"] if legacy else []))
                assert result.returncode!=0 and not target.exists(),"invalid traversal source emitted artifact"
                (out/(ident+".stdout")).write_text(result.stdout)
                (out/(ident+".stderr")).write_text(result.stderr)
                negatives.append({"id":ident,"exitCode":result.returncode,"sourceSHA256":digest(src),"noArtifact":True})
    after=source_fingerprint()
    assert before==after and not base.checked(["git","status","--porcelain"],"source after").stdout.strip()
    result={
        "schema":"cherri.paginate-marker-source-model/1","status":"PASS_SOURCE_AND_DECLARED_MODEL_ONLY",
        "source":head,"sourceFingerprint":before,"compilerSHA256":digest(exe),
        "nativeRuntimeExecuted":False,"fullAcceptanceCredit":False,
        "summary":{"cliCompiles":len(checks),"exactNativeRoundtrips":len(checks),"modelCases":sum(len(c["models"]) for c in checks),
                   "causalFaults":len(faults),"cliNegatives":len(negatives),"durationSeconds":time.monotonic()-started},
        "compiles":checks,"causalFaults":faults,"cliNegatives":negatives,
        "nativeAssumptions":["Count Items/Get Type and null/empty/singleton Content Graph semantics",
            "URL Encode/Decode UTF-8 and Number round/text behavior; explicit noncanonical/lossy model variants exercise emitted guards",
            "Dictionary item identity/nested typed values across native Repeat/Append",
            "GET native errors/redirect/cancellation and response conversion remain external; the 404 body-only case exposes absence of status validation"],
        "remainingContracts":["same-GET Link/status response capability","typed trailing block","catch/retry/cancellation","real signed Apple runtime"]
    }
    report_path.write_text(json.dumps(result,ensure_ascii=False,indent=2)+"\n")
    print(json.dumps({"report":str(report_path),"sha256":digest(report_path),"status":result["status"],**result["summary"]},sort_keys=True))


if __name__=="__main__":
    main()
