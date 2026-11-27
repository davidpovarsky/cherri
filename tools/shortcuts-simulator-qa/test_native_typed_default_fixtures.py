"""Synthetic collector controls; no actual Go or Apple execution."""
import base64
import copy
import json
import plistlib
import unittest
import native_typed_default_fixtures as f

def encode(value):return json.dumps(value,sort_keys=True,separators=(",",":")).encode()
def bind(raw,prefix):
    return {prefix+"Base64":base64.b64encode(raw).decode(),prefix+"Sha256":f.sha(raw),prefix+"Bytes":len(raw)}
def packet(case):
    actions=[{"WFWorkflowActionIdentifier":"is.workflow.actions.gettext",
              "WFWorkflowActionParameters":{"UUID":"00000000-0000-4000-8000-000000000001","WFTextActionText":"synthetic-control"}}]
    actions.extend({"WFWorkflowActionIdentifier":e["action"],"WFWorkflowActionParameters":{e["parameter"]:e["value"]}} for e in case["requiredEffects"])
    actions.append({"WFWorkflowActionIdentifier":"is.workflow.actions.output","WFWorkflowActionParameters":{}})
    def point(i):
        action=actions[i];params=action.get("WFWorkflowActionParameters",{})
        return {"index":i,"identifier":action["WFWorkflowActionIdentifier"],"uuidPresent":"UUID" in params,"uuid":params.get("UUID","")}
    p={k:copy.deepcopy(case[k]) for k in ("requirement","caseId","sourceAuthorityPath","originalUnitTest","originalNativeTest","originalSourceHelper","originalGraphOracle","compileFilename","sourceCase","observationMode","expectedLanguageSemantics","requiredEffects")}
    p.update(schema=f.EXPORT_SCHEMA,dialect="v2",catalogPath=f.CATALOG_PATH,catalogSha256=f.CATALOG_SHA,
        sourceAuthoritySha256=f.CATALOG["authorities"][case["sourceAuthorityPath"]],
        actionCount=len(actions),actionPoints={"observation":point(0),"final_return":point(len(actions)-1)},
        workflowName="",workflowNamePresent=False,nativeExecuted=False,rawNativeTypesObserved=False,
        rawNativeValuesObserved=False,formalAcceptanceGranted=False)
    p.update(bind(case["source"].encode(),"source"))
    p.update(bind(plistlib.dumps({"WFWorkflowActions":actions}),"unsigned"))
    return p
def marker(p):
    raw=encode(p)
    return "    p15_native_typed_default_export_test.go:99: "+f.MARKER_TEXT+f" sha256={f.sha(raw)} bytes={len(raw)} base64={base64.b64encode(raw).decode()}\n"
def event(action,test="",**kwargs):
    row={"Package":f.PACKAGE,"Action":action}
    if test:row["Test"]=test
    return dict(row,**kwargs)
def events():
    result=[event("start")]
    def visit(name):
        result.append(event("run",name))
        children=[n for n in f.REQUIRED_EVENTS if n.startswith(name+"/") and not any(n.startswith(a+"/") and a.startswith(name+"/") for a in f.REQUIRED_EVENTS)]
        for child in sorted(children):visit(child)
        if name in f.CASE_BY_TEST:result.append(event("output",name,Output=marker(packet(f.CASE_BY_TEST[name]))))
        result.append(event("pass",name))
    for parent in (*f.ORIGINAL_PARENTS,f.EXPORT_PARENT):visit(parent)
    return result+[event("pass")]
def stream(rows):return b"".join(encode(row)+b"\n" for row in rows)
def graph_change(p,change):
    document=plistlib.loads(base64.b64decode(p["unsignedBase64"]))
    change(document["WFWorkflowActions"])
    p.update(bind(plistlib.dumps(document),"unsigned"))

class NativeTypedDefaultFixtureTests(unittest.TestCase):
    def parse(self,rows):
        raw=stream(rows);return f.parse_exports(raw,expected_sha256=f.sha(raw))
    def rejected_packet(self,p,case,code="."):
        with self.assertRaisesRegex(f.FixtureRejected,code):f.validate_fixture(p,case)
    def test_complete_14_case_526_event_stream(self):
        self.assertEqual(len(f.ORIGINAL_EVENTS),511)
        self.assertEqual(len(f.REQUIRED_EVENTS),526)
        parsed=self.parse(events())
        self.assertEqual([p["caseId"] for p in parsed],[c["caseId"] for c in f.CASES])
        self.assertEqual(len(parsed),14)
    def test_missing_original_parent_positive_and_negative_events_reject(self):
        chosen=list(f.ORIGINAL_PARENTS)+[
            f.ORIGINAL_PARENTS[0]+"/false",f.ORIGINAL_PARENTS[0]+"/wrong_argument/list",
            f.ORIGINAL_PARENTS[1]+"/optional_none_default/oracle/missing_present",
            f.ORIGINAL_PARENTS[1]+"/empty_list_default/oracle/empty_inherits_input",
            f.ORIGINAL_PARENTS[2]+"/literal_binding",f.ORIGINAL_PARENTS[3]+"/defaults/mapping/truthiness",
            f.ORIGINAL_PARENTS[3]+"/forms/variadic_order"]
        base=events()
        for name in chosen:
            with self.subTest(event=name):
                with self.assertRaises(f.FixtureRejected):
                    self.parse([r for r in base if r.get("Test")!=name and not r.get("Test","").startswith(name+"/")])
    def test_all_original_sources_authorities_and_cases_are_bound(self):
        for case in f.CASES:
            for change in ("source","authority","case"):
                with self.subTest(case=case["id"],change=change):
                    p=packet(case)
                    if change=="source":p.update(bind(case["source"].encode()+b'\nreturn "FAKE"\n',"source"))
                    elif change=="authority":p["sourceAuthoritySha256"]="0"*64
                    else:p["caseId"]="wrong"
                    self.rejected_packet(p,case)
    def test_presence_types_empty_values_and_runtime_claims_cannot_be_reconciled(self):
        for name,change in (
            ("false",lambda p:p["expectedLanguageSemantics"]["calls"]["positional"].__setitem__("value",0)),
            ("zero",lambda p:p["expectedLanguageSemantics"]["calls"]["positional"].__setitem__("value",False)),
            ("empty_text",lambda p:p["expectedLanguageSemantics"]["calls"]["positional"].__setitem__("present",False)),
            ("empty_list",lambda p:p["expectedLanguageSemantics"]["calls"]["positional"].__setitem__("value",None)),
            ("empty_map",lambda p:p["expectedLanguageSemantics"]["calls"]["positional"].__setitem__("value",[])),
            ("optional_none_default",lambda p:p["expectedLanguageSemantics"]["calls"]["omitted"].__setitem__("value","")),
        ):
            with self.subTest(case=name):
                case=next(c for c in f.CASES if c["caseId"]==name);p=packet(case);change(p)
                self.rejected_packet(p,case,"ORIGINAL_TYPED_CONTRACT")
        case=f.CASES[0]
        for field in ("nativeExecuted","rawNativeTypesObserved","rawNativeValuesObserved","formalAcceptanceGranted"):
            for value in (True,0):
                with self.subTest(claim=field,value=value):
                    p=packet(case);p[field]=value;self.rejected_packet(p,case,"EXPORT_OVERCLAIM")
    def test_original_action_points_reject_wrong_indices_identity_and_return(self):
        case=f.CASES[0]
        for field,value,code in (("index",True,"GRAPH_POINT_INDEX"),("index",9999,"GRAPH_POINT_INDEX"),("identifier","wrong","GRAPH_POINT_ACTION"),("uuidPresent",1,"GRAPH_POINT_UUID"),("uuid","wrong","GRAPH_POINT_UUID")):
            with self.subTest(field=field,value=value):
                p=packet(case);p["actionPoints"]["observation"][field]=value;self.rejected_packet(p,case,code)
        p=packet(case);del p["actionPoints"]["observation"];self.rejected_packet(p,case,"ORIGINAL_GRAPH_POINTS")
        p=packet(case);p["actionPoints"]["final_return"]=copy.deepcopy(p["actionPoints"]["observation"]);self.rejected_packet(p,case,"ORIGINAL_FINAL_RETURN")
    def test_both_original_background_sound_effects_remain_in_order(self):
        case=f.CASES[-1]
        changes=[
            lambda a:a[1]["WFWorkflowActionParameters"].__setitem__("backgroundSound","Rain"),
            lambda a:a.__setitem__(slice(1,3),[a[2],a[1]]),
            lambda a:a[1].__setitem__("WFWorkflowActionIdentifier","is.workflow.actions.nothing"),
        ]
        for index,change in enumerate(changes):
            with self.subTest(change=index):
                p=packet(case);graph_change(p,change);self.rejected_packet(p,case,"ORIGINAL_SIDE_EFFECTS")
    def test_stream_and_envelope_corruption_reject(self):
        case=f.CASES[0]
        for field,value in (("unsignedSha256","0"*64),("unsignedBytes",True),("actionCount",True),("workflowNamePresent",1),("unsignedBase64","!"),("dialect","legacy")):
            with self.subTest(field=field):
                p=packet(case);p[field]=value;self.rejected_packet(p,case)
        p=packet(case);graph_change(p,lambda a:a[0].__setitem__("WFWorkflowActionParameters",[]));self.rejected_packet(p,case,"ORIGINAL_ACTION_PARAMETERS")
        rows=events();rows[-1]["Action"]="fail"
        with self.assertRaises(f.FixtureRejected):self.parse(rows)
        raw=stream(events())
        with self.assertRaises(f.FixtureRejected):f.parse_exports(raw[:-1],expected_sha256=f.sha(raw[:-1]))
    def test_missing_duplicate_and_misbound_exports_reject(self):
        base=events()
        for case in (f.CASES[0],f.CASES[-1]):
            name=f.EXPORT_PARENT+"/"+case["id"];at=next(i for i,r in enumerate(base) if r.get("Test")==name and r["Action"]=="output")
            for defect in ("missing","duplicate","misbound"):
                with self.subTest(case=case["id"],defect=defect):
                    rows=copy.deepcopy(base)
                    if defect=="missing":rows.pop(at)
                    elif defect=="duplicate":rows.insert(at,copy.deepcopy(rows[at]))
                    else:rows[at]["Output"]=marker(packet(f.CASES[-1] if case==f.CASES[0] else f.CASES[0]))
                    with self.assertRaises(f.FixtureRejected):self.parse(rows)

if __name__=="__main__":unittest.main()
