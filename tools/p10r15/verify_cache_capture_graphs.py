#!/usr/bin/env python3
"""Qualify actual cache capture graphs under the declared native-action model.
This is a bounded graph interpreter check, never Apple runtime evidence.
"""
from pathlib import Path
import copy,hashlib,importlib.util,json,sys

def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()
def native(name):return "is.workflow.actions."+name
def params(action):return action.get("WFWorkflowActionParameters") or {}
class TraceVariables(dict):
 def __init__(self):super().__init__();self.writes=[]
 def __setitem__(self,key,value):
  self.writes.append((key,copy.deepcopy(value)));super().__setitem__(key,value)

def check_capture(model,record):
 test=record["test"];ttl=int(test.split("/")[1].split("=")[1])
 assert ttl in [0,300]
 key=model.native_key("number",False);slot=("shortcut-A",key);out=[]
 def run(name,storage,clock,expected_trace,effects,reads,writes,empty,scope="shortcut-A",created=None):
  before=copy.deepcopy(storage)
  graph=model.Graph(copy.deepcopy(record["actions"]),copy.deepcopy(storage),clock=clock,scope=scope,empty_items=empty)
  graph.variables=TraceVariables();graph.execute()
  observed=[v for n,v in graph.variables.writes if n=="counter"]
  assert observed==expected_trace,(test,name,"captured variable write sequence",observed,expected_trace)
  assert graph.last==expected_trace[-1],(test,name,"final counter",graph.last,expected_trace[-1])
  assert (graph.effects,graph.reads,graph.writes)==(effects,reads,writes),(test,name,"lazy effects/IO",graph.effects,graph.reads,graph.writes)
  if created is not None:
   current=(scope,key)
   assert current in graph.storage,(test,name,"missing precise local namespace",current)
   assert json.loads(graph.storage[current])==model.expected_envelope("number",9,key,created),(test,name,"shadow/result or creation-time changed")
  if writes==0:assert graph.storage==before,(test,name,"unrequested persistence mutation")
  if scope!="shortcut-A" and slot in before:assert graph.storage[slot]==before[slot],(test,name,"other shortcut scope overwritten")
  out.append({"scenario":name,"emptyTextItems":empty,"counterWrites":observed,"result":graph.last,"bodyExecutions":graph.effects,"reads":graph.reads,"writes":graph.writes})
  return graph
 for empty in [0,1]:
  if ttl==0:
   run("ttl0_empty_storage",{},(),[0,1,2,3],2,0,0,empty)
   run("ttl0_malformed_storage_ignored",{slot:"invalid JSON must not be read"},(),[0,1,2,3],2,0,0,empty)
   continue
  # The hit reuses the exact archive written by the first actual action graph.
  fresh=run("first_miss_second_hit",{},(1000,1010,1020,1030),[0,1,2],1,2,1,empty,created=1010)
  archive=copy.deepcopy(fresh.storage)
  run("both_calls_hit",archive,(1100,1110,1120,1130),[0,2],0,2,0,empty,created=1010)
  run("second_call_expires",archive,(1309,1310,1320,1330),[0,2,3],1,2,1,empty,created=1320)
  run("first_call_exact_expiry",archive,(1310,1320,1330,1340),[0,1,2],1,2,1,empty,created=1320)
  run("first_call_clock_rollback",archive,(1009,1019,1029,1039),[0,1,2],1,2,1,empty,created=1019)
  run("separate_shortcut_local_scope",archive,(1000,1010,1020,1030),[0,1,2],1,2,1,empty,scope="shortcut-B",created=1010)
 return out

def fault_cases(model,record):
 actions=record["actions"]
 outer=[i for i,a in enumerate(actions) if a["WFWorkflowActionIdentifier"]==native("setvariable") and params(a).get("WFVariableName")=="counter"]
 markers=[i for i,a in enumerate(actions) if a["WFWorkflowActionIdentifier"]==native("gettext") and params(a).get("WFTextActionText")=="P15_CACHE_BODY_ONCE"]
 assert len(outer)==4 and len(markers)==2,(outer,markers)
 graph=model.Graph(actions)
 miss=max(i for i,end in graph.ends.items() if i<markers[1]<end and actions[i]["WFWorkflowActionIdentifier"]==native("conditional"))
 nine=next(i for i,a in enumerate(actions) if a["WFWorkflowActionIdentifier"]==native("number") and float(params(a)["WFNumberActionNumber"])==9)
 value_read=next(i for i,a in enumerate(actions) if i>markers[0] and a["WFWorkflowActionIdentifier"]==native("getvariable"))
 wire=next(i for i,a in enumerate(actions) if a["WFWorkflowActionIdentifier"]==native("conditional") and params(a).get("WFControlFlowMode")==0)
 def set_target(a,index,value):params(a[index])["WFVariableName"]=value
 def replace_reference(a):
  params(a[value_read])["WFVariable"]={"WFSerializationType":"WFTextTokenAttachment","Value":{"Type":"Variable","VariableName":"counter"}}
 return {
  "captured_first_write_redirected":lambda a:set_target(a,outer[1],"UNRELATED_CAPTURE"),
  "intercall_write_redirected":lambda a:set_target(a,outer[2],"UNRELATED_INTERCALL"),
  "captured_second_write_redirected":lambda a:set_target(a,outer[3],"UNRELATED_CAPTURE"),
  "shadow_result_changed":lambda a:params(a[nine]).update({"WFNumberActionNumber":8}),
  "body_effect_removed":lambda a:params(a[markers[0]]).update({"WFTextActionText":"REMOVED_BODY_EFFECT"}),
  "second_hit_executes_body":lambda a:params(a[miss]).update({"WFCondition":5}),
  "scoped_result_retargeted_to_outer":replace_reference,
  "conditional_loses_native_envelope":lambda a:params(a[wire]).update({"WFInput":params(a[wire])["WFInput"]["Variable"]}),
 }

def main():
 module=Path(sys.argv[1]);directory=Path(sys.argv[2]);output=Path(sys.argv[3])
 output.mkdir(exist_ok=False)
 spec=importlib.util.spec_from_file_location("cache_graph_model",module)
 model=importlib.util.module_from_spec(spec);spec.loader.exec_module(model)
 selected=[]
 for path in sorted(directory.glob("*.json")):
  record=json.loads(path.read_text())
  if record["test"].startswith("TestP15CacheCapturedWritesAndShadowing/"):
   assert not record.get("error"),(record["test"],record.get("error"))
   selected.append((path,record))
 expected={f"TestP15CacheCapturedWritesAndShadowing/ttl={ttl}/legacy={legacy}/{profile}" for ttl in [0,300] for legacy in ["false","true"] for profile in ["debug","release"]}
 assert {r["test"] for _,r in selected}==expected
 records=[]
 for path,record in selected:
  records.append({"test":record["test"],"path":str(path),"sha256":sha(path),"scenarios":check_capture(model,record)})
 faults=[];faultdir=output/"faults";faultdir.mkdir()
 for path,record in selected:
  if "/ttl=300/" not in record["test"] or not record["test"].endswith("/debug"):continue
  for name,mutate in fault_cases(model,record).items():
   altered=copy.deepcopy(record);mutate(altered["actions"]);assert altered!=record
   leaf=record["test"].replace("/","_")+"_"+name+".json";target=faultdir/leaf
   target.write_text(json.dumps(altered,ensure_ascii=False,indent=2)+"\n")
   rejected=False;error=""
   try:check_capture(model,altered)
   except AssertionError as exc:rejected=True;error=str(exc)
   assert rejected,(record["test"],name,"semantic mutation accepted")
   faults.append({"test":record["test"],"fault":name,"originalSHA256":sha(path),"mutantPath":str(target),"mutantSHA256":sha(target),"rejected":True,"reason":error[:1200]})
 assert len(faults)==16
 result={"schema":"cherri.actual-cache-capture-model/1","status":"PASS_LAZY_CAPTURE_AND_SHADOW_GRAPH_MODEL","scope":"Independent interpretation of actual compiler graphs under declared action semantics; NOT Apple runtime, native persistence or concurrency acceptance.","nativeRuntimeExecuted":False,"actualGraphs":len(records),"scenarios":sum(len(r["scenarios"]) for r in records),"causalFaults":len(faults),"sourceModelPath":str(module),"sourceModelSHA256":sha(module),"controllerSHA256":sha(Path(__file__)),"records":records,"faults":faults}
 (output/"result.original.json").write_text(json.dumps(result,ensure_ascii=False,indent=2)+"\n")
 print(json.dumps({k:v for k,v in result.items() if k not in ["records","faults"]}));print(json.dumps({"resultSHA256":sha(output/"result.original.json")}))
if __name__=="__main__":main()
