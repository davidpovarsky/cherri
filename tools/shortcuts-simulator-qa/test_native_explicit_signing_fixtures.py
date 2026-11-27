"""Synthetic BRG26 intake controls, never actual signing/native evidence."""
import base64
import copy
import json
import plistlib
import unittest
import native_explicit_signing_fixtures as f

def encode(v): return json.dumps(v,sort_keys=True,separators=(",",":")).encode()
def blob(raw): return {"base64":base64.b64encode(raw).decode(),"sha256":f.sha(raw),"bytes":len(raw)}
def workflow(mode,literal,name):
    actions=[{"WFWorkflowActionIdentifier":"is.workflow.actions.gettext",
              "WFWorkflowActionParameters":{"WFTextActionText":literal}}]
    if mode=="v2":
        actions.append({"WFWorkflowActionIdentifier":"is.workflow.actions.nothing"})
    raw=plistlib.dumps({"WFWorkflowName":name,"WFWorkflowActions":actions})
    return dict(blob(raw),actionCount=len(actions),workflowNamePresent=True,workflowName=name)
def packet(mode):
    return {
        "schema":f.EXPORT_SCHEMA,"requirement":"BRG26","caseId":mode,"dialect":mode,
        "originalHelper":"p15BRG26StaleState","originalGraphOracle":"p15BRG26NativeLiteral",
        "sourceAuthorityPath":f.AUTHORITY_PATH,"sourceAuthoritySha256":f.AUTHORITY_SHA,
        "originalProductionTests":list(f.ORIGINAL_PARENTS),"compileOrder":["A","B"],
        "earlierName":"BRG26 A earlier","currentName":f.CURRENT_NAME,
        "earlierSource":blob(f.SOURCE_A),"currentSource":blob(f.SOURCE_B),
        "earlierUnsigned":workflow(mode,f.EARLIER,"BRG26 A earlier"),
        "currentUnsigned":workflow(mode,f.CURRENT,f.CURRENT_NAME),
        "ambientUnsigned":workflow(mode,f.EARLIER,"BRG26 A earlier"),
        "ambientInputBasename":"A earlier.shortcut","ambientOutputBasename":"A stale output.shortcut",
        "ambientBasename":"BRG26_STALE_A_NAME","ambientWorkflowName":"BRG26_STALE_A_WORKFLOW",
        "explicitInputBasename":"B current unsigned.shortcut","explicitOutputBasename":f.CURRENT_NAME+".shortcut",
        "existingDestination":"BRG26_EXISTING_DESTINATION","expectedObservations":f.expected_observations(mode),
        "liveSigningExecuted":False,"nativeExecuted":False,"formalAcceptanceGranted":False,
    }
def marker(p):
    b=encode(p)
    return "    p15_native_explicit_signing_export_test.go:100: "+f.MARKER_TEXT+f" sha256={f.sha(b)} bytes={len(b)} base64={base64.b64encode(b).decode()}\n"
def event(action,test="",**kw):
    r={"Package":f.PACKAGE,"Action":action}
    if test:r["Test"]=test
    return dict(r,**kw)
def events():
    rows=[event("start")]
    def visit(name):
        rows.append(event("run",name))
        children=[n for n in f.REQUIRED_EVENTS if n.startswith(name+"/") and
                  not any(n.startswith(a+"/") and a.startswith(name+"/") for a in f.REQUIRED_EVENTS)]
        for child in sorted(children):visit(child)
        if name in f.CASE_BY_TEST:
            rows.append(event("output",name,Output=marker(packet(f.CASE_BY_TEST[name]))))
        rows.append(event("pass",name))
    for parent in (*f.ORIGINAL_PARENTS,f.EXPORT_PARENT):visit(parent)
    return rows+[event("pass")]
def stream(rows):return b"".join(encode(r)+b"\n" for r in rows)

class NativeExplicitSigningFixtureTests(unittest.TestCase):
    def parse(self,rows):
        raw=stream(rows)
        return f.parse_exports(raw,expected_sha256=f.sha(raw))
    def rejected(self,rows,diagnostic="."):
        with self.assertRaisesRegex(f.FixtureRejected,diagnostic):self.parse(rows)
    def changed(self,mode,change):
        rows=events();p=packet(mode);change(p)
        next(r for r in rows if r.get("Test")==f.EXPORT_PARENT+"/"+mode and r["Action"]=="output")["Output"]=marker(p)
        return rows
    def test_two_complete_dialects_and_distinct_terminal_contracts(self):
        self.assertEqual(len(f.ORIGINAL_EVENTS),13)
        self.assertEqual(len(f.REQUIRED_EVENTS),16)
        result=self.parse(events())
        self.assertEqual([p["dialect"] for p in result],["v2","legacy"])
        self.assertEqual(result[0]["expectedObservations"]["finalReturn"]["kind"],"None")
        self.assertEqual(result[1]["expectedObservations"]["finalReturn"]["value"],f.CURRENT)
    def test_all_original_routes_and_protocol_children_are_required(self):
        for name in f.ORIGINAL_EVENTS:
            with self.subTest(event=name):
                self.rejected([r for r in events() if r.get("Test")!=name and not r.get("Test","").startswith(name+"/")])
    def test_current_artifact_cannot_be_replaced_with_stale_or_reconciled_bytes(self):
        for mode in f.CASES:
            with self.subTest(mode=mode,change="stale"):
                self.rejected(self.changed(mode,lambda p:p.__setitem__("currentUnsigned",copy.deepcopy(p["earlierUnsigned"]))),"ORIGINAL_LITERAL_VALUE")
            with self.subTest(mode=mode,change="source"):
                self.rejected(self.changed(mode,lambda p:p.__setitem__("currentSource",blob(f.SOURCE_B+b"\nreturn \"PASS\"\n"))),"ORIGINAL_SOURCE_BYTES")
            with self.subTest(mode=mode,change="ambient"):
                self.rejected(self.changed(mode,lambda p:p.__setitem__("ambientUnsigned",copy.deepcopy(p["currentUnsigned"]))),"ORIGINAL_LITERAL_VALUE")
            for field in ("currentName","explicitOutputBasename","ambientInputBasename","sourceAuthoritySha256"):
                with self.subTest(mode=mode,field=field):
                    self.rejected(self.changed(mode,lambda p:p.__setitem__(field,"stale")),"EXPORT_CASE_BINDING")
    def test_compile_order_fallthrough_and_native_claims_remain_exact(self):
        self.rejected(self.changed("v2",lambda p:p.__setitem__("compileOrder",["B","A"])),"EXPORT_CASE_BINDING")
        self.rejected(self.changed("v2",lambda p:p.__setitem__("expectedObservations",f.expected_observations("legacy"))),"ORIGINAL_NATIVE_EXPECTATIONS")
        for field in ("liveSigningExecuted","nativeExecuted","formalAcceptanceGranted"):
            for value in (True,0):
                with self.subTest(field=field,value=value):
                    self.rejected(self.changed("v2",lambda p:p.__setitem__(field,value)),"EXPORT_OVERCLAIM")
    def test_blob_tampering_and_missing_explicit_void_reject(self):
        for field,value in (("sha256","0"*64),("bytes",True),("base64","!"),("actionCount",True),("workflowNamePresent",1)):
            with self.subTest(field=field):
                self.rejected(self.changed("v2",lambda p:p["currentUnsigned"].__setitem__(field,value)))
        def remove_void(p):
            p["currentUnsigned"]=workflow("legacy",f.CURRENT,f.CURRENT_NAME)
        self.rejected(self.changed("v2",remove_void),"ORIGINAL_COMPLETE_ACTION_COUNT")
    def test_parameterless_nothing_preserved_and_text_parameters_required(self):
        self.parse(events())
        def replace_graph(p,change):
            old=p["currentUnsigned"]
            document=plistlib.loads(base64.b64decode(old["base64"]))
            change(document["WFWorkflowActions"])
            raw=plistlib.dumps(document)
            old.update(blob(raw))
        # Both omission and an explicitly empty dictionary represent the same
        # parameterless action; neither is a present native value.
        self.parse(self.changed("v2",lambda p:replace_graph(p,lambda a:a[1].__setitem__("WFWorkflowActionParameters",{}))))
        for mode in f.CASES:
            with self.subTest(mode=mode,problem="missing_text_parameters"):
                self.rejected(self.changed(mode,lambda p:replace_graph(p,lambda a:a[0].pop("WFWorkflowActionParameters"))),"ACTION_ENVELOPE")
        for value in ("bad",[],1,{"unexpected":"field"}):
            with self.subTest(problem="invalid_nothing_parameters",value=value):
                self.rejected(self.changed("v2",lambda p:replace_graph(p,lambda a:a[1].__setitem__("WFWorkflowActionParameters",value))),"ACTION_ENVELOPE")
    def test_exports_must_be_exactly_once_under_their_original_case(self):
        for mode in f.CASES:
            rows=events();name=f.EXPORT_PARENT+"/"+mode
            at=next(i for i,r in enumerate(rows) if r.get("Test")==name and r["Action"]=="output")
            with self.subTest(mode=mode,problem="absent"):self.rejected(rows[:at]+rows[at+1:],"MISSING_FIXTURE_EXPORT")
            with self.subTest(mode=mode,problem="duplicate"):self.rejected(rows[:at]+[copy.deepcopy(rows[at])]+rows[at:],"EXPORT_MARKER_BINDING")
            other="legacy" if mode=="v2" else "v2"
            rows[at]["Output"]=marker(packet(other))
            with self.subTest(mode=mode,problem="misbound"):self.rejected(rows,"EXPORT_CASE_BINDING")

if __name__=="__main__":unittest.main()
