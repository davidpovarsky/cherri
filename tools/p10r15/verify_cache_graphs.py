#!/usr/bin/env python3
"""Independent bounded interpreter for the actual emitted cache graph.

This checks compiler control/data-flow under explicitly stated action semantics.
It is not Apple runtime evidence and does not implement the cache producer.
Unknown native actions, invalid envelopes and unresolved references fail closed.
"""
from pathlib import Path
import base64,copy,datetime,hashlib,json,math,re,sys

class NativeAbort(Exception): pass
class Date(float): pass

def text(value):
 if value is None:return ""
 if isinstance(value,str):return value
 if isinstance(value,bytes):return value.decode("utf-8",errors="replace")
 if isinstance(value,Date):return datetime.datetime.fromtimestamp(float(value),datetime.timezone.utc).isoformat()
 if isinstance(value,(dict,list)):return json.dumps(value,ensure_ascii=False,separators=(",",":"),allow_nan=False)
 if isinstance(value,bool):return "true" if value else "false"
 if isinstance(value,(int,float)):
  if not math.isfinite(value):raise NativeAbort("nonfinite number")
  return str(int(value)) if value==int(value) else format(value,".17g")
 raise AssertionError(("unknown text coercion",type(value)))

def number(value):
 if isinstance(value,bool):raise NativeAbort("Bool is not an archived Number")
 try:result=float(value)
 except (TypeError,ValueError):raise NativeAbort("number conversion failed")
 if not math.isfinite(result):raise NativeAbort("nonfinite number")
 return result

def native_kind(value):
 if isinstance(value,Date):return "Date"
 if isinstance(value,bool):return "Boolean"
 if isinstance(value,(int,float)):return "Number"
 if isinstance(value,str):return "Text"
 if isinstance(value,dict):return "Dictionary"
 if isinstance(value,bytes):return "File"
 if value is None:return None
 if isinstance(value,list):return [native_kind(v) for v in value]
 raise AssertionError(type(value))

class Graph:
 def __init__(self,actions,storage=None,clock=(1000,1010),scope="shortcut-A",empty_items=0):
  self.a=actions;self.storage={} if storage is None else storage;self.clock=iter(clock);self.scope=scope
  self.outputs={};self.variables={};self.last=None;self.reads=0;self.writes=0;self.effects=0
  self.empty_items=empty_items;self.steps=0;self.ends={};self.elses={}
  stack=[];seen={};groups=set()
  for i,a in enumerate(actions):
   p=a.get("WFWorkflowActionParameters") or {}
   uuid=p.get("UUID")
   if uuid:
    assert uuid not in seen,("duplicate producer",i);seen[uuid]=i
   def refs(v):
    if isinstance(v,dict):
     if v.get("Type")=="ActionOutput":assert seen.get(v.get("OutputUUID"),i)>=0 and seen.get(v.get("OutputUUID"),i)<i,("missing/forward reference",i,v)
     for child in v.values():refs(child)
    elif isinstance(v,list):
     for child in v:refs(child)
   refs({k:v for k,v in p.items() if k!="UUID"})
   if "GroupingIdentifier" not in p:continue
   group=p["GroupingIdentifier"];mode=p["WFControlFlowMode"]
   if mode==0:
    assert group not in groups;groups.add(group);stack.append((group,i))
    if a["WFWorkflowActionIdentifier"]=="is.workflow.actions.conditional":
     input=p.get("WFInput")
     assert isinstance(input,dict) and set(input)=={"Type","Variable"} and input["Type"]=="Variable","invalid native conditional envelope"
     token=input["Variable"];assert isinstance(token,dict) and set(token)=={"Value","WFSerializationType"} and token["WFSerializationType"]=="WFTextTokenAttachment","invalid native conditional attachment"
   elif mode==1:
    assert stack and stack[-1][0]==group;self.elses[stack[-1][1]]=i
   elif mode==2:
    assert stack and stack[-1][0]==group
    _,start=stack.pop();self.ends[start]=i
   else:raise AssertionError(("control mode",mode))
  assert not stack,"unclosed native groups"
 def resolve(self,v):
  if isinstance(v,list):return [self.resolve(x) for x in v]
  if not isinstance(v,dict):return v
  serial=v.get("WFSerializationType")
  if serial=="WFTextTokenAttachment":return self.resolve(v["Value"])
  if serial=="WFNumberSubstitutableState":return self.resolve(v["Value"])
  if serial=="WFTextTokenString":
   body=v["Value"];s=body.get("string",body.get("String",""))
   refs=body.get("attachmentsByRange",body.get("AttachmentsByRange",{}))
   if not refs:return s
   if s=="\ufffc" and set(refs)=={"{0, 1}"}:return self.resolve(refs["{0, 1}"])
   raw=s.encode("utf-16-le");edits=[]
   for position,ref in refs.items():
    m=re.fullmatch(r"\{(\d+), (\d+)\}",position);assert m
    start,length=map(int,m.groups());assert length==1 and raw[start*2:(start+1)*2]=="\ufffc".encode("utf-16-le")
    edits.append((start,self.resolve(ref)))
   for start,value in sorted(edits,reverse=True):raw=raw[:start*2]+text(value).encode("utf-16-le")+raw[(start+1)*2:]
   return raw.decode("utf-16-le")
  if serial=="WFDictionaryFieldValue":
   body=v["Value"];assert isinstance(body,dict) and set(body)=={"WFDictionaryFieldValueItems"}
   result={}
   for row in body["WFDictionaryFieldValueItems"]:
    key=text(self.resolve(row["WFKey"]));assert key not in result
    raw=self.resolve(row["WFValue"]);kind=row["WFItemType"]
    if kind==0:raw=text(raw)
    elif kind==3:raw=number(raw)
    elif kind==4:assert isinstance(raw,bool)
    else:raise AssertionError(("unsupported typed dictionary row",kind))
    result[key]=raw
   return result
  kind=v.get("Type")
  if kind=="ActionOutput":
   uuid=v["OutputUUID"];assert uuid in self.outputs,("reference not executed",uuid)
   return self.outputs[uuid]
  if kind=="Variable":
   if "VariableName" in v:
    name=v["VariableName"];assert name in self.variables,("undefined variable",name)
    return self.variables[name]
   if set(v)=={"Type","Variable"}:return self.resolve(v["Variable"])
  if serial is not None:raise AssertionError(("unsupported carrier",serial))
  return {k:self.resolve(value) for k,value in v.items()}
 def comparison(self,p):
  left=self.resolve(p["WFInput"])
  if "WFNumberValue" in p:left,right=number(left),number(self.resolve(p["WFNumberValue"]))
  elif "WFConditionalActionString" in p:left,right=text(left),text(self.resolve(p["WFConditionalActionString"]))
  else:raise AssertionError("condition lost typed operand")
  code=p["WFCondition"]
  if code==0:return left<right
  if code==1:return left<=right
  if code==2:return left>right
  if code==3:return left>=right
  if code==4:return left==right
  if code==5:return left!=right
  raise AssertionError(("unsupported comparison",code))
 def execute(self,start=0,end=None):
  if end is None:end=len(self.a)
  i=start
  while i<end:
   self.steps+=1;assert self.steps<=20000,"unbounded graph"
   a=self.a[i];p=a.get("WFWorkflowActionParameters") or {};name=a["WFWorkflowActionIdentifier"].removeprefix("is.workflow.actions.")
   if p.get("WFControlFlowMode")==0:
    close=self.ends[i]
    if name=="conditional":
     alternate=self.elses.get(i,close)
     if self.comparison(p):self.execute(i+1,alternate)
     elif alternate!=close:self.execute(alternate+1,close)
    elif name=="repeat.each":
     values=self.resolve(p["WFInput"]);assert isinstance(values,list) and len(values)<=1000
     old=self.variables.get("Repeat Item",None);had="Repeat Item" in self.variables
     for item in values:self.variables["Repeat Item"]=item;self.execute(i+1,close)
     if had:self.variables["Repeat Item"]=old
     else:self.variables.pop("Repeat Item",None)
    else:raise AssertionError(("unknown control",name))
    ending=self.a[close].get("WFWorkflowActionParameters") or {}
    if ending.get("UUID"):self.outputs[ending["UUID"]]=self.last
    i=close+1;continue
   r=None
   if name=="number":r=number(self.resolve(p["WFNumberActionNumber"]))
   elif name=="gettext":
    r=text(self.resolve(p["WFTextActionText"]))
    if r=="P15_CACHE_BODY_ONCE":self.effects+=1
   elif name=="getvariable":r=self.resolve(p["WFVariable"])
   elif name=="setvariable":
    # Native Set Variable uses implicit Previous only when WFInput is absent.
    # Preserve the complete value/no-content sequence; never coerce it to Text.
    r=self.resolve(p["WFInput"]) if "WFInput" in p else self.last
    self.variables[p["WFVariableName"]]=r
   elif name=="dictionary":r=self.resolve(p["WFItems"]);assert isinstance(r,dict)
   elif name=="getitemtype":r=native_kind(self.resolve(p["WFInput"]))
   elif name=="count":
    value=self.resolve(p["Input"])
    if p["WFCountType"]=="Characters":r=len(text(value))
    else:
     assert p["WFCountType"]=="Items"
     r=0 if value is None else len(value) if isinstance(value,list) else self.empty_items if value=="" else 1
   elif name=="date":
    if p["WFDateActionMode"]=="Current Date":r=Date(next(self.clock))
    else:assert p["WFDateActionMode"]=="Specified Date";r=Date(datetime.datetime.fromisoformat(p["WFDateActionDate"].replace("Z","+00:00")).timestamp())
   elif name=="gettimebetweendates":
    assert p["WFTimeUntilUnit"]=="Seconds"
    first,last=self.resolve(p["WFTimeUntilFromDate"]),self.resolve(p["WFInput"])
    assert isinstance(first,Date) and isinstance(last,Date)
    r=float(last)-float(first)
   elif name=="getstoredcontent":
    key=("global" if p["WFStoredContentGlobalValue"] else self.scope,p["WFStoredContentKey"])
    self.reads+=1;r=self.storage.get(key,None)
   elif name=="setstoredcontent":
    key=("global" if p["WFStoredContentGlobalValue"] else self.scope,p["WFStoredContentKey"])
    r=self.resolve(p["WFInput"]);assert isinstance(r,str)
    self.storage[key]=r;self.writes+=1
   elif name=="detect.dictionary":
    try:r=json.loads(text(self.resolve(p["WFInput"])))
    except (ValueError,TypeError):raise NativeAbort("native JSON parsing failed")
   elif name=="getvalueforkey":
    value=self.resolve(p["WFInput"]);assert isinstance(value,dict)
    if p["WFGetDictionaryValueType"]=="All Keys":r=list(value)
    else:assert p["WFGetDictionaryValueType"]=="Value";r=value.get(p["WFDictionaryKey"],None)
   elif name=="hash":
    assert p["WFHashType"]=="SHA256";r=hashlib.sha256(text(self.resolve(p["WFInput"])).encode("utf-8")).hexdigest()
   elif name=="math":
    left,right=number(self.resolve(p["WFInput"])),number(self.resolve(p["WFMathOperand"]))
    op=p["WFMathOperation"]
    if op=="+":r=left+right
    elif op=="-":r=left-right
    elif op in ["*","×"]:r=left*right
    elif op in ["/","÷"]:r=left/right
    else:raise AssertionError(("unknown math operation",op))
   elif name=="base64encode":
    value=self.resolve(p["WFInput"])
    if p["WFEncodeMode"]=="Encode":
     assert p["WFBase64LineBreakMode"]=="None"
     r=base64.b64encode(value if isinstance(value,bytes) else text(value).encode("utf-8")).decode("ascii")
    else:
     assert p["WFEncodeMode"]=="Decode"
     try:r=base64.b64decode(text(value),validate=True)
     except ValueError:raise NativeAbort("native base64 parsing failed")
   elif name=="text.replace":
    subject=text(self.resolve(p["WFInput"]));find=text(self.resolve(p["WFReplaceTextFind"]));replacement=text(self.resolve(p["WFReplaceTextReplace"]))
    assert p["WFReplaceTextCaseSensitive"] is True and p["WFReplaceTextRegularExpression"] is True
    r=re.sub(find,replacement,subject)
   elif name=="nothing":r=None
   elif name=="alert":r=None
   elif name=="exit":raise NativeAbort("native scalar contract failure")
   elif name=="output":r=self.resolve(p["WFOutput"])
   else:raise AssertionError(("unsupported actual native action",name))
   self.last=r
   if p.get("UUID"):self.outputs[p["UUID"]]=r
   if name=="output":return
   i+=1
  return self.last

EXPECTED={
 "unicode_text":("text","שלום 🍒\nquoted \"value\"",False),
 "empty_text":("text","",False),
 "zero_number":("number",0.0,False),
 "fraction_number":("number",-2.5,False),
 "false_boolean":("bool",0.0,False),
 "true_boolean":("bool",1.0,False),
 "global_text":("text","shared value",True),
 "sensitive_opt_in":("text","explicitly allowed",False),
}
def native_key(kind,global_value):
 return "cherri.cache.v1."+("global" if global_value else "local")+"."+kind+"."+hashlib.sha256(b"P15_SCALAR_KEY").hexdigest()

def expected_envelope(kind,value,key,created=1010):
 return {"version":1,"typeTag":kind,"created":created,"expires":created+300,"ttl":300,"namespace":key,
 "value":"v:"+base64.b64encode(value.encode("utf-8")).decode("ascii") if kind=="text" else value}

def check_record(record):
 test=record["test"]
 if test.startswith("TestP15CacheZeroTTLBypassesPersistence"):
  out=[]
  for empty in [0,1]:
   graph=Graph(record["actions"],empty_items=empty);graph.execute()
   assert graph.last==EXPECTED["unicode_text"][1] and graph.effects==1 and graph.reads==graph.writes==0
   out.append({"scenario":"ttl0","emptyTextItems":empty,"body":1,"reads":0,"writes":0})
  return out
 name=test.split("/")[1];kind,body,global_value=EXPECTED[name];key=native_key(kind,global_value)
 slot=("global" if global_value else "shortcut-A",key);out=[]
 def run_scenario(label,payload,now,expected,miss=True,abort=False,empty=0,scope="shortcut-A"):
  storage={} if payload is None else {slot:payload if isinstance(payload,(str,list)) else json.dumps(payload,ensure_ascii=False,separators=(",",":"))}
  graph=Graph(record["actions"],storage,(now,now+10),scope=scope,empty_items=empty)
  failed=False
  try:graph.execute()
  except NativeAbort:failed=True
  assert failed==abort,(label,"abort",failed,abort)
  if not abort:
   assert graph.last==expected,(label,"result",graph.last,expected)
   assert graph.effects==(1 if miss else 0),(label,"body",graph.effects,miss)
   assert graph.reads==1 and graph.writes==(1 if miss else 0),(label,"io",graph.reads,graph.writes)
  out.append({"scenario":label,"emptyTextItems":empty,"body":graph.effects,"reads":graph.reads,"writes":graph.writes,"nativeAbortModeled":failed})
  return graph
 for empty in [0,1]:
  # No supplied archive is trusted as a fake success: the miss writes the
  # actual graph's dictionary/JSON product, then a separate invocation reads it.
  fresh=run_scenario("fresh_miss",None,1000,body,empty=empty)
  assert slot in fresh.storage,(test,"write missed the exact authorized storage scope/key",slot)
  stored=fresh.storage[slot];parsed=json.loads(stored)
  assert parsed==expected_envelope(kind,body,key),(test,"post-body envelope",parsed)
  hit=run_scenario("actual_cross_invocation_hit",stored,1200,body,False,empty=empty)
  reference=expected_envelope(kind,body,key)
  run_scenario("just_before_expiry",reference,1309.999,body,False,empty=empty)
  run_scenario("exact_expiry",reference,1310,body,True,empty=empty)
  run_scenario("after_expiry",reference,1311,body,True,empty=empty)
  run_scenario("clock_rollback",reference,1009,body,True,empty=empty)
  alterations={
   "wrong_version":{"version":2},"changed_ttl":{"ttl":301},"wrong_type":{"typeTag":"foreign"},
   "wrong_namespace":{"namespace":"foreign"},"typed_created_text":{"created":"1010"},
   "typed_ttl_bool":{"ttl":True},"inconsistent_expires":{"expires":999999},
   "extra_field":{"extra":1},"missing_value":None,
  }
  for label,changes in alterations.items():
   payload=copy.deepcopy(reference)
   if changes is None:del payload["value"]
   else:payload.update(changes)
   run_scenario(label,payload,1200,body,True,empty=empty)
  run_scenario("multiple_storage_items",["first","second"],1200,body,True,empty=empty)
  run_scenario("malformed_json","not JSON",1200,None,False,True,empty)
  if kind=="text":
   for label,value in [("invalid_utf8","v:////"),("noncanonical_pad_bits","v:Zh=="),("invalid_alphabet","v:!!!!"),("wrong_prefix","x:Zg==")]:
    payload=copy.deepcopy(reference);payload["value"]=value
    run_scenario(label,payload,1200,body,True,empty=empty)
   for label,value in [("seeded_empty_text",""),("seeded_unicode","🌍\nשונה"),("seeded_false_text","false")]:
    payload=expected_envelope(kind,value,key)
    run_scenario(label,payload,1200,value,False,empty=empty)
  if kind=="bool":
   for label,value,miss in [("invalid_boolean_two",2,True),("present_false",0,False),("present_true",1,False)]:
    payload=expected_envelope(kind,value,key)
    run_scenario(label,payload,1200,body if miss else value,miss,empty=empty)
  # Apple's global/local scope is a stated native dependency. This proves the
  # emitted flag/key use, not native device isolation or atomicity.
  run_scenario("different_shortcut_scope",reference,1200,body,not global_value,empty=empty,scope="shortcut-B")
 return out

def qualified_report(directory):
 records=[]
 for path in sorted(Path(directory).glob("*.json")):
  record=json.loads(path.read_text())
  if not record["test"].startswith(("TestP15CacheScalarLazyGraphs/","TestP15CacheZeroTTLBypassesPersistence/")):continue
  if record.get("error"):raise AssertionError(("compile failed",record["test"]))
  records.append((path,record))
 assert len(records)==36,("missing original graphs",len(records))
 outputs=[]
 for path,record in records:
  outcomes=check_record(record)
  outputs.append({"test":record["test"],"path":str(path),"sha256":hashlib.sha256(path.read_bytes()).hexdigest(),"scenarios":outcomes})
 return outputs,records

def main():
 directory=Path(sys.argv[1]);output=Path(sys.argv[2]);output.mkdir(exist_ok=False)
 outputs,records=qualified_report(directory)
 result={"schema":"cherri.actual-cache-graph-model/1","status":"PASS_DECLARED_NATIVE_ACTION_MODEL",
 "scope":"Independent bounded semantic interpretation of original emitted action graphs; NOT Apple runtime, storage persistence, atomicity or simulator acceptance.",
 "originalGraphs":len(records),"scenarios":sum(len(r["scenarios"]) for r in outputs),"records":outputs,
 "interpreterSHA256":hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),"nativeRuntimeExecuted":False}
 (output/"result.original.json").write_text(json.dumps(result,indent=2,ensure_ascii=False)+"\n")
 print(json.dumps({k:v for k,v in result.items() if k!="records"},ensure_ascii=False))
 print(json.dumps({"resultSHA256":hashlib.sha256((output/"result.original.json").read_bytes()).hexdigest()}))
if __name__=="__main__":main()
