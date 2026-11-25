#!/usr/bin/env python3
"""Causal mutations of actual cache action graphs, not producer-derived outputs."""
from pathlib import Path
import copy,hashlib,json,runpy,sys

def params(action):return action.get("WFWorkflowActionParameters") or {}
def suffix(action):return action["WFWorkflowActionIdentifier"].removeprefix("is.workflow.actions.")
def reference(value):
 if not isinstance(value,dict):return None
 if value.get("Type")=="ActionOutput":return value["OutputUUID"]
 if "Value" in value:return reference(value["Value"])
 if value.get("Type")=="Variable" and "Variable" in value:return reference(value["Variable"])
 if "attachmentsByRange" in value:
  refs=list(value["attachmentsByRange"].values())
  if len(refs)==1:return reference(refs[0])
 return None

def mutate(record,name):
 out=copy.deepcopy(record);a=out["actions"];byid={params(x).get("UUID"):x for x in a if params(x).get("UUID")}
 def producer(value):return byid.get(reference(value))
 def conditions():return [x for x in a if suffix(x)=="conditional" and params(x).get("WFControlFlowMode")==0]
 def source_key(condition):
  p=producer(params(condition)["WFInput"])
  return params(p).get("WFDictionaryKey") if p is not None and suffix(p)=="getvalueforkey" else None
 if name=="eager_body":
  index=next(i for i,x in enumerate(a) if suffix(x)=="gettext" and params(x).get("WFTextActionText")=="P15_CACHE_BODY_ONCE")
  a.insert(0,a.pop(index))
 elif name=="miss_predicate_inverted":
  c=next(x for x in conditions() if params(x).get("WFNumberValue")==0 and suffix(producer(params(x)["WFInput"]) or {})=="getvariable" and str(params(producer(params(x)["WFInput"]))["WFVariable"]["Value"].get("VariableName","")).endswith("_hit"))
  params(c)["WFCondition"]=5
 elif name=="expiry_boundary_not_inclusive":
  c=next(x for x in conditions() if params(x).get("WFCondition")==3 and params(producer(params(x).get("WFNumberValue")) or {}).get("WFDictionaryKey")=="expires")
  params(c)["WFCondition"]=2
 elif name=="wrong_created_clock":
  clock=next(params(x)["UUID"] for x in a if suffix(x)=="gettimebetweendates")
  field=next(row for x in a if suffix(x)=="dictionary" for row in params(x)["WFItems"]["Value"]["WFDictionaryFieldValueItems"] if row["WFKey"]["Value"]["string"]=="created")
  ref=next(iter(field["WFValue"]["Value"]["attachmentsByRange"].values()));ref["OutputUUID"]=clock
 elif name=="numeric_payload_became_text":
  field=next(row for x in a if suffix(x)=="dictionary" for row in params(x)["WFItems"]["Value"]["WFDictionaryFieldValueItems"] if row["WFKey"]["Value"]["string"]=="value")
  assert field["WFItemType"]==3;field["WFItemType"]=0
 elif name=="wrong_storage_scope":
  x=next(x for x in a if suffix(x)=="setstoredcontent");params(x)["WFStoredContentGlobalValue"]=not params(x)["WFStoredContentGlobalValue"]
 elif name=="text_roundtrip_bypassed":
  selected=None
  for c in conditions():
   left=producer(params(c)["WFInput"]);right=producer(params(c).get("WFConditionalActionString"))
   if left is None or right is None or suffix(left)!="hash" or suffix(right)!="hash":continue
   encoded=producer(params(left)["WFInput"])
   if encoded is not None and suffix(encoded)=="text.replace" and params(encoded).get("WFReplaceTextFind")=="^v:":selected=c;break
  assert selected is not None
  right=next(iter(params(selected)["WFConditionalActionString"]["Value"]["attachmentsByRange"].values()))
  params(selected)["WFInput"]["Variable"]["Value"]=copy.deepcopy(right)
 elif name=="version_guard_inverted":
  c=next(x for x in conditions() if source_key(x)=="version" and params(x).get("WFNumberValue")==1)
  params(c)["WFCondition"]=4
 elif name=="empty_text_changed":
  x=next(x for x in a if suffix(x)=="gettext" and params(x).get("WFTextActionText")=="" and x is not None)
  params(x)["WFTextActionText"]="changed empty value"
 elif name=="invalid_boolean_admitted":
  c=next(x for x in conditions() if source_key(x)=="value" and params(x).get("WFCondition")==5 and params(x).get("WFNumberValue")==1)
  params(c)["WFNumberValue"]=2
 elif name=="dangling_input":
  c=conditions()[0];params(c)["WFInput"]["Variable"]["Value"]["OutputUUID"]="00000000-0000-0000-0000-000000000000"
 elif name=="result_retargeted":
  x=next(x for x in reversed(a) if suffix(x)=="getvariable" and params(x).get("WFVariable",{}).get("Value",{}).get("VariableName","").endswith("_result"))
  ref=params(x)["WFVariable"]["Value"];ref["VariableName"]=ref["VariableName"][:-len("_result")]+"_hit"
 elif name=="wrong_storage_key":
  x=next(x for x in a if suffix(x)=="setstoredcontent");params(x)["WFStoredContentKey"]="foreign"
 elif name=="cache_read_removed":
  x=next(x for x in a if suffix(x)=="getstoredcontent");x["WFWorkflowActionIdentifier"]="is.workflow.actions.nothing"
 else:raise AssertionError(("unknown mutation",name))
 assert json.dumps(out,sort_keys=True)!=json.dumps(record,sort_keys=True)
 return out

def main():
 model=Path(sys.argv[1]);inputs=Path(sys.argv[2]);out=Path(sys.argv[3]);out.mkdir(exist_ok=False)
 module=runpy.run_path(str(model));check=module["check_record"];NativeAbort=module["NativeAbort"]
 source={}
 for name in ["unicode_text","zero_number","false_boolean"]:
  p=inputs/("TestP15CacheScalarLazyGraphs_"+name+"_legacy=false_release.json")
  source[name]=(p,json.loads(p.read_text()))
 cases={
 "eager_body":"unicode_text","miss_predicate_inverted":"unicode_text","expiry_boundary_not_inclusive":"unicode_text",
 "wrong_created_clock":"unicode_text","numeric_payload_became_text":"zero_number","wrong_storage_scope":"unicode_text",
 "text_roundtrip_bypassed":"unicode_text","version_guard_inverted":"unicode_text","empty_text_changed":"unicode_text",
 "invalid_boolean_admitted":"false_boolean","dangling_input":"unicode_text","result_retargeted":"unicode_text",
 "wrong_storage_key":"unicode_text","cache_read_removed":"unicode_text"}
 results=[]
 for name,which in cases.items():
  path,original=source[which];changed=mutate(original,name)
  target=out/(name+".actual-mutated-graph.json");target.write_text(json.dumps(changed,indent=2,ensure_ascii=False)+"\n")
  rejected=False;error=""
  try:check(changed)
  except (AssertionError,NativeAbort) as exc:rejected=True;error=str(exc)
  assert rejected,("causal mutation survived",name)
  results.append({"mutation":name,"source":str(path),"sourceSHA256":hashlib.sha256(path.read_bytes()).hexdigest(),
   "mutatedGraph":str(target),"mutatedSHA256":hashlib.sha256(target.read_bytes()).hexdigest(),"rejected":rejected,"error":error})
 result={"schema":"cherri.cache-causal-model-controls/1","status":"PASS_REJECTED_ALL_CAUSAL_MUTATIONS","count":len(results),"records":results,
  "modelSHA256":hashlib.sha256(model.read_bytes()).hexdigest(),"controllerSHA256":hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),"nativeRuntimeExecuted":False}
 (out/"result.original.json").write_text(json.dumps(result,indent=2,ensure_ascii=False)+"\n")
 print(json.dumps({k:v for k,v in result.items() if k!="records"}))
 print(json.dumps({"resultSHA256":hashlib.sha256((out/"result.original.json").read_bytes()).hexdigest()}))
if __name__=="__main__":main()
