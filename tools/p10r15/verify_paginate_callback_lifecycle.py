#!/usr/bin/env python3
"""Bounded callback graph evaluator. Explicit MODEL, never Apple runtime credit.

Inputs are the original compiler-emitted JSON products. No source is compiled,
no graph is rewritten, no native action executes, and no network is contacted.
The ordinary native control/value interpreter is extended from the existing
pagination model. Transport, content cardinality, Delay and Nothing are declared
model assumptions; native acceptance remains an independent requirement.
"""
import copy
import hashlib
import json
import pathlib
import sys
import urllib.parse

import verify_paginate as base
from paginate_marker_model import MarkerGraphModel, NATIVE_ACTIONS
from verify_debug_export import Value, as_text

PREFIX = base.PREFIX
content_items, native_value = base.content_items, base.native_value
GuardStop, BodyDecodeError = base.GuardStop, base.BodyDecodeError

def parameters(action):
    value=action.get("WFWorkflowActionParameters", {})
    if value is None:
        assert action["WFWorkflowActionIdentifier"] in {PREFIX+"nothing", PREFIX+"exit"}, "nil parameters on value/effect action"
        return {}
    assert isinstance(value,dict), "invalid native parameter map"
    return value

class RouteModel:
    origin = "https://example.invalid"
    def __init__(self, routes):
        self.routes, self.requests, self.request_headers = routes, [], []
    def request(self, url, headers):
        assert url.startswith(self.origin + "/"), "model forbids external route"
        path = url[len(self.origin):]
        assert path in self.routes, "unexpected route " + path
        self.requests.append(path)
        self.request_headers.append(headers)
        return self.routes[path]

class CallbackGraphModel(MarkerGraphModel):
    native_actions = NATIVE_ACTIONS | {"delay", "nothing", "math"}
    def __init__(self, server):
        super().__init__(server)
        self.variables["__cherri_debug_log"] = Value("List", [])
        self.callbacks, self.effects, self.debug_values = [], [], []
        self.page_origins = {}
        self.last_read = None
    def value(self, raw):
        if isinstance(raw, dict) and raw.get("Type") == "ActionOutput":
            assert set(raw) <= {"Type", "OutputUUID", "OutputName"}, "unmodeled reference coercion"
        return super().value(raw)

    def http_headers(self, raw):
        # Canonical empty HTTP Dictionary omits its empty Items array.
        # Accept that exact native shape only; all nonempty/unknown forms use
        # the original strict decoder and causal guards below.
        if raw == {"WFSerializationType": "WFDictionaryFieldValue", "Value": {}}:
            return {}
        return super().http_headers(raw)

    def run(self, actions, start=0, end=None):
        end = len(actions) if end is None else end
        previous = Value("Null", None)
        index = start
        while index < end:
            a = actions[index]
            ident = a["WFWorkflowActionIdentifier"].removeprefix(PREFIX)
            p = parameters(a)
            assert ident in self.native_actions, "unexpected action in paginate graph: " + ident
            self.executed += 1
            assert self.executed <= 2_000_000, "model execution guard exceeded"
            if ident in {"conditional", "repeat.count", "repeat.each"} and p["WFControlFlowMode"] == 0:
                group = p["GroupingIdentifier"]
                closing = next(i for i in range(index + 1, end)
                               if parameters(actions[i]).get("GroupingIdentifier") == group
                               and actions[i]["WFWorkflowActionParameters"].get("WFControlFlowMode") == 2)
                if ident == "conditional":
                    alternate = next((i for i in range(index + 1, closing)
                                      if parameters(actions[i]).get("GroupingIdentifier") == group
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
                close = parameters(actions[closing])
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
                if previous.kind == "Text" and previous.data.startswith("P15_CALLBACK_DONE_"):
                    self.effects.append(previous.data)
            elif ident == "getvariable":
                previous = self.value(p["WFVariable"])
                self.last_read = previous
                if str(p.get("CustomOutputName", "")).endswith("_callback_input"):
                    assert previous.kind == "Dictionary" and id(previous) in self.page_origins, "callback source is not the actual decoded page"
                    self.callbacks.append((self.page_origins[id(previous)], previous))
                elif index and str(parameters(actions[index-1]).get("CustomOutputName", "")).endswith("_callback_input"):
                    assert self.callbacks and previous is self.callbacks[-1][1], "authored page read changed identity"
            elif ident == "setvariable":
                previous = self.value(p["WFInput"]) if "WFInput" in p else previous
                self.variables[p["WFVariableName"]] = previous
            elif ident == "appendvariable":
                incoming = self.value(p["WFInput"]) if "WFInput" in p else previous
                current = self.variables[p["WFVariableName"]]
                if p["WFVariableName"] == "__cherri_debug_log":
                    self.debug_values.append(incoming)
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
                status, body = self.server.request(url, headers)
                self.statuses.append(status)
                previous = Value("Text", body if isinstance(body, str) else json.dumps(body, ensure_ascii=False))
            elif ident == "detect.dictionary":
                incoming = self.value(p["WFInput"])
                if incoming.kind == "Text":
                    try:
                        previous = self.remember(native_value(json.loads(incoming.data)))
                    except json.JSONDecodeError as error:
                        raise BodyDecodeError("synthetic malformed JSON body") from error
                else:
                    previous = incoming
                if previous.kind == "Dictionary":
                    self.page_origins[id(previous)] = self.server.requests[-1]
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
            elif ident == "math":
                left, right = self.value(p["WFInput"]), self.value(p["WFMathOperand"])
                assert left.kind == right.kind == "Number" and p["WFMathOperation"] == "+"
                previous = Value("Number", left.data + right.data)
            elif ident == "delay":
                assert set(p) - {"UUID", "CustomOutputName"} == {"WFDelayTime"} and p["WFDelayTime"] == 0
                previous = Value("Null", None)
            elif ident == "nothing":
                assert not (set(p) - {"UUID", "CustomOutputName"})
                previous = Value("Null", None)
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

ITEM={"id":1,"flag":False,"zero":0,"empty":"","list":[],"map":{},"nested":[{"x":[False,0,""]},[]]}
FIRST="/pages/one.json"
INNER="/nested/one.json"

def scenarios(cursor=False):
    second=FIRST+"?cursor="+urllib.parse.quote("opaque +/שלום",safe="-._~") if cursor else "/pages/two.json"
    third=FIRST+"?cursor=third" if cursor else "/pages/three.json"
    fourth=FIRST+"?cursor=fourth" if cursor else "/pages/four.json"
    next2="opaque +/שלום" if cursor else "two.json"
    next3="third" if cursor else "three.json"
    next4="fourth" if cursor else "four.json"
    def page(items,next_value=None):
        body={"items":items}
        if next_value is not None:body["next"]=next_value
        return (200,body)
    def row(routes, expected, paths, valid, guard=None, malformed=False):
        return {"routes":routes,"expected":expected,"paths":paths,"valid":valid,"guard":guard,"malformed":malformed}
    return {
      "one":row({FIRST:page([ITEM])},[ITEM],[FIRST],[FIRST]),
      "empty_page":row({FIRST:page([])},[],[FIRST],[FIRST]),
      "three_middle_empty":row({FIRST:page([ITEM],next2),second:page([],next3),third:page([{"id":3}])},[ITEM,{"id":3}],[FIRST,second,third],[FIRST,second,third]),
      "cycle":row({FIRST:page([ITEM],next2),second:page([{"id":2}],next2 if cursor else "one.json")},[ITEM,{"id":2}],[FIRST,second],[FIRST,second]),
      "page_cap":row({FIRST:page([ITEM],next2),second:page([{"id":2}],next3),third:page([{"id":3}],next4),fourth:page([{"id":4}])},[ITEM,{"id":2},{"id":3}],[FIRST,second,third],[FIRST,second,third]),
      "item_cap":row({FIRST:page([ITEM]*6,next2),second:page([{"id":2}])},[ITEM]*6,[FIRST],[FIRST]),
      "failed_first":row({FIRST:page(["invalid"])},None,[FIRST],[],"PAGINATE_ITEM_DICTIONARY_REQUIRED"),
      "failed_second":row({FIRST:page([ITEM],next2),second:page(["invalid"])},None,[FIRST,second],[FIRST],"PAGINATE_ITEM_DICTIONARY_REQUIRED"),
      "missing_items":row({FIRST:(200,{"wrong":[]})},None,[FIRST],[],"PAGINATE_ITEMS_KEY_REQUIRED"),
      "wrong_page":row({FIRST:(200,[1,2])},None,[FIRST],[],"PAGINATE_PAGE_DICTIONARY_REQUIRED"),
      "malformed":row({FIRST:(200,"{malformed")},None,[FIRST],[],malformed=True),
      "status_unverified":row({FIRST:(404,{"items":[ITEM]})},[ITEM],[FIRST],[FIRST]),
    }

def evaluate(product,row,name):
    graph=product["actions"]
    nested=name=="nested_same_name"
    routes=copy.deepcopy(row["routes"])
    routes[INNER]=(200,{"items":[{"inner":True}]})
    server=RouteModel(routes)
    model=CallbackGraphModel(server)
    guard=None
    malformed=False
    try:model.run(graph)
    except GuardStop as error:guard=str(error)
    except BodyDecodeError:malformed=True
    assert guard==row["guard"] and malformed==row["malformed"],"failure/cancel boundary changed"
    valid=row["valid"]
    paths=[]
    callbacks=[]
    effects=[]
    for path in row["paths"]:
        paths.append(path)
        if path in valid:
            callbacks.append(path)
            if nested:
                paths.append(INNER);callbacks.append(INNER);effects.append("P15_CALLBACK_DONE_INNER")
            if name!="empty":effects.append("P15_CALLBACK_DONE_OUTER")
    assert server.requests==paths,("request/cap/cycle order",server.requests,paths)
    assert [path for path,value in model.callbacks]==callbacks,"callback did not run once for each validated page"
    for path,value in model.callbacks:
        assert base.plain(value)==routes[path][1],"page payload/shape/order changed"
    assert model.effects==effects,("body effects changed",model.effects,effects)
    if name=="captured_write":
        assert model.variables["total"].kind=="Number" and model.variables["total"].data==len(valid),"captured writes changed on zero/one/many/error path"
    if not guard and not malformed:
        result=model.last_read
        assert result is not None and result.kind=="List"
        assert base.plain(result)==row["expected"],"aggregate changed"
        assert all(id(item) in model.source_dictionaries for item in result.data),"copied/coerced native item"
    expected_debug=[]
    if product["profile"]=="debug":
        if name=="entry_previous":expected_debug=[value for path,value in model.callbacks]
        if name=="body_previous":
            assert len(model.debug_values)==len(valid)
            assert all(v.kind=="Text" and v.data=="P15_CALLBACK_LOCAL_INPUT" for v in model.debug_values),"local Previous did not replace entry"
        elif name=="entry_previous":
            assert len(model.debug_values)==len(expected_debug)
            assert all(v is e for v,e in zip(model.debug_values,expected_debug)),"Previous changed native page identity"
        else:assert not model.debug_values
    else:assert not model.debug_values,"Release kept diagnostics"
    expected_headers={"Authorization":"P15_CALLBACK_SYNTHETIC_AUTH"} if name=="runtime_headers" else {}
    assert all(h==expected_headers for h in server.request_headers),"headers not retained"
    return {"status":"PASS_MODEL","requests":paths,"callbackPages":callbacks,"bodyEffects":effects,"capturedTotal":base.plain(model.variables["total"]) if name=="captured_write" else None,"guard":guard,"malformed":malformed,"actions":model.executed,"nativeRuntimeExecuted":False}

def validate_products(directory):
    result=json.loads((directory.parent/"result.original.json").read_text())
    assert result["status"]=="PASS_CALLBACK_AND_EMPTY_INFERENCE_183_AND_80_PRODUCTS"
    assert result["counts"]=={"pass":183,"fail":0,"skip":0}
    index=json.loads((directory.parent/"products-index.original.json").read_text())
    products={}
    for item in index:
        path=directory/item["name"]
        assert hashlib.sha256(path.read_bytes()).hexdigest()==item["sha256"]
        products[item["name"]]=json.loads(path.read_text())
    assert len(products)==80
    return result,products,index

def main():
    import argparse
    parser=argparse.ArgumentParser()
    parser.add_argument("--products",required=True)
    parser.add_argument("--report",required=True)
    args=parser.parse_args()
    out=pathlib.Path(args.report)
    assert not out.exists()
    result,products,index=validate_products(pathlib.Path(args.products))
    checks=[]
    selected={}
    names={"reference","empty","entry_previous","body_previous","captured_write","outer_shadow","nested_same_name","cursor","runtime_headers"}
    for filename,product in sorted(products.items()):
        test=product["test"]
        if test.startswith("TestP15PaginatePageCallbackGraphs/"):
            name=test.split("/")[1]
            if name not in names:continue
            for scenario,row in scenarios(name=="cursor").items():
                try: observed=evaluate(product,row,name)
                except Exception as error: raise AssertionError(filename+"/"+scenario+": "+str(error)) from error
                checks.append({"product":filename,"scenario":scenario,**observed})
            selected[filename]=product
        elif test.startswith("TestP15PaginatePageCallbackZeroBudgets/"):
            zero={"routes":{},"expected":[],"paths":[],"valid":[],"guard":None,"malformed":False}
            checks.append({"product":filename,"scenario":"zero_budget",**evaluate(product,zero,"entry_previous")})
            selected[filename]=product
    assert len(selected)==44 and len(checks)==440
    # Faults alter the actual emitted graph after a positive interpretation.
    faults=[]
    def product(name):
        return copy.deepcopy(products["TestP15PaginatePageCallbackGraphs__"+name+"__legacy-false__debug.json"])
    for fault in ["wrong_page_source","dropped_effect","duplicate_effect","changed_page_cap","early_callback","captured_write_lost","local_previous_changed","reference_coercion","null_parameters","empty_header_extra","empty_header_null"]:
        name="reference"
        if fault=="captured_write_lost":name="captured_write"
        if fault=="local_previous_changed":name="body_previous"
        p=product(name);graph=p["actions"]
        entry=next(i for i,a in enumerate(graph) if str(parameters(a).get("CustomOutputName","")).endswith("_callback_input"))
        effect=next(i for i,a in enumerate(graph) if parameters(a).get("WFTextActionText")=="P15_CALLBACK_DONE_OUTER")
        scenario="one"
        if fault=="wrong_page_source":
            response=next(a for a in graph if a["WFWorkflowActionIdentifier"]==PREFIX+"downloadurl")
            graph[entry]["WFWorkflowActionParameters"]["WFVariable"]={"Type":"ActionOutput","OutputUUID":response["WFWorkflowActionParameters"]["UUID"]}
        elif fault=="dropped_effect":del graph[effect]
        elif fault=="duplicate_effect":graph.insert(effect,copy.deepcopy(graph[effect]))
        elif fault=="changed_page_cap":
            outer=next(a for a in graph if a["WFWorkflowActionIdentifier"]==PREFIX+"repeat.count" and a["WFWorkflowActionParameters"]["WFControlFlowMode"]==0)
            outer["WFWorkflowActionParameters"]["WFRepeatCount"]=1;scenario="three_middle_empty"
        elif fault=="early_callback":
            item_guard=next(i for i,a in enumerate(graph) if parameters(a).get("WFAlertActionMessage")=="PAGINATE_ITEMS_KEY_REQUIRED")
            dictionary=next(i for i,a in enumerate(graph) if a["WFWorkflowActionIdentifier"]==PREFIX+"detect.dictionary")
            assert dictionary<item_guard<entry
            action=graph.pop(entry);graph.insert(dictionary+1,action);scenario="failed_first"
        elif fault=="captured_write_lost":
            writes=[a for a in graph if a["WFWorkflowActionIdentifier"]==PREFIX+"setvariable" and a["WFWorkflowActionParameters"].get("WFVariableName")=="total"]
            assert len(writes)==2;writes[1]["WFWorkflowActionParameters"]["WFInput"]=0
        elif fault=="local_previous_changed":
            local=next(a for a in graph if parameters(a).get("WFTextActionText")=="P15_CALLBACK_LOCAL_INPUT")
            local["WFWorkflowActionParameters"]["WFTextActionText"]="wrong Previous"
        elif fault in {"empty_header_extra","empty_header_null"}:
            request=next(a for a in graph if a["WFWorkflowActionIdentifier"]==PREFIX+"downloadurl")
            headers=request["WFWorkflowActionParameters"]["WFHTTPHeaders"]
            assert headers == {"WFSerializationType":"WFDictionaryFieldValue","Value":{}}
            headers["Value"] = {"Unexpected":True} if fault=="empty_header_extra" else None
        elif fault=="null_parameters":
            action=next(a for a in graph if a["WFWorkflowActionIdentifier"]==PREFIX+"number")
            action["WFWorkflowActionParameters"]=None
        elif fault=="reference_coercion":
            ref=graph[entry]["WFWorkflowActionParameters"]["WFVariable"]
            ref["Value"]["Aggrandizements"]=[{"Type":"WFCoercionVariableAggrandizement","CoercionItemClass":"WFStringContentItem"}]
        try:evaluate(p,scenarios()[scenario],name)
        except (AssertionError,KeyError,ValueError,TypeError) as error:faults.append({"id":fault,"status":"REJECTED_MODEL","reason":str(error)})
        else:raise AssertionError("causal lifecycle fault accepted: "+fault)
    assert len(faults)==11
    dependencies=[pathlib.Path(__file__).resolve(),pathlib.Path(base.__file__).resolve(),pathlib.Path(sys.modules["paginate_marker_model"].__file__).resolve(),pathlib.Path(sys.modules["verify_debug_export"].__file__).resolve()]
    report={"schema":"cherri.page-callback-declared-model/1","status":"PASS_MODEL_ONLY","compilerSource":result["source"],"compilerTree":result["tree"],"qualificationResultSHA256":hashlib.sha256((pathlib.Path(args.products).parent/"result.original.json").read_bytes()).hexdigest(),"originalGraphs":len(selected),"checks":checks,"causalFaults":faults,"modelSources":[{"path":str(p),"sha256":hashlib.sha256(p.read_bytes()).hexdigest()} for p in dependencies],"artifactPins":[item for item in index if item["name"] in selected],"nativeRuntimeExecuted":False,"networkExecuted":False,"scopeExclusions":["measure clock runtime","typed cell payload runtime","Link/status/retry/cancellation","typed item selectors/returned values","final acceptance"]}
    out.write_text(json.dumps(report,ensure_ascii=False,indent=2)+"\n")
    print(json.dumps({"status":report["status"],"graphs":len(selected),"checks":len(checks),"faults":len(faults),"report":str(out),"sha256":hashlib.sha256(out.read_bytes()).hexdigest()}))

if __name__=="__main__":
    main()
