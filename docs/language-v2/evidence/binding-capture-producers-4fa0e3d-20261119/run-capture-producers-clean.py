from pathlib import Path
import subprocess,time,json,hashlib,os,collections
repo=Path(r"C:\Users\DAVID\Code\cherri-p10r15-binding-producers-20261119")
out=Path(r"C:\Users\DAVID\Code\cherri-p10r15-evidence-20261119\binding-producers-33f0699")
expected="4fa0e3d07fbe18a1b910979b390765f840f6d152"
def git(*args):return subprocess.check_output(["git",*args],cwd=repo,encoding="utf-8").strip()
assert git("rev-parse","HEAD")==expected
assert not git("status","--porcelain","--untracked-files=normal")
protected=["tests/language-v2/acceptance-contract.json","tests/language-v2/repair-regression-plan.json","tests/language-v2/backend-recovery-gates.json","tests/runtime_poc/CherriBindingCapturePOC.cherri"]
before={p:hashlib.sha256((repo/p).read_bytes()).hexdigest() for p in protected}
env=os.environ.copy();env.update({"GOMAXPROCS":"1","GOFLAGS":"-p=1"})
cmd=["go","test","-p=1","-count=1","-json","-timeout=5m","-run","^(TestBindingCaptureProductionEvidence|TestBindingCaptureRecordRejectsDetachedCommandsAndTiers)$","./tools/language-acceptance"]
start=time.monotonic()
with (out/"capture-producers-clean.jsonl").open("wb") as stream:
 p=subprocess.Popen(cmd,cwd=repo,env=env,stdout=stream,stderr=subprocess.STDOUT)
 timeout=False
 try:code=p.wait(timeout=330)
 except subprocess.TimeoutExpired:
  timeout=True;subprocess.run(["taskkill","/PID",str(p.pid),"/T","/F"],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL);code=p.wait()
raw=(out/"capture-producers-clean.jsonl").read_bytes();events=[];outputs=collections.defaultdict(str)
for line in raw.splitlines():
 try:
  event=json.loads(line);events.append(event)
  if event.get("Action")=="output":outputs[event.get("Test","")]+=event.get("Output","")
 except Exception:pass
required=["TestBindingCaptureProductionEvidence","TestBindingCaptureRecordRejectsDetachedCommandsAndTiers"]
missing=[t for t in required if not any(e.get("Test")==t and e.get("Action")=="pass" for e in events)]
after={p:hashlib.sha256((repo/p).read_bytes()).hexdigest() for p in protected}
summary={"sourceSHA":expected,"sourceSHAAfter":git("rev-parse","HEAD"),"sourceCleanBefore":True,"sourceCleanAfter":not git("status","--porcelain","--untracked-files=normal"),"protectedBefore":before,"protectedAfter":after,"command":cmd,"exitCode":code,"timedOut":timeout,"seconds":round(time.monotonic()-start,3),"bytes":len(raw),"sha256":hashlib.sha256(raw).hexdigest(),"named":{a:sum(e.get("Action")==a and bool(e.get("Test")) for e in events) for a in ["run","pass","fail","skip"]},"requiredMissing":missing,"failures":[e.get("Test") for e in events if e.get("Action")=="fail" and e.get("Test")]}
receipt=None
for line in outputs["TestBindingCaptureProductionEvidence"].splitlines():
 marker="P15_BINDING_CAPTURE_PRODUCTION_RECEIPT:"
 if marker in line:receipt=json.loads(line.split(marker,1)[1])
if receipt:
 (out/"capture-producers-clean-receipt.json").write_text(json.dumps(receipt,indent=2)+"\n",encoding="utf-8")
summary["receiptObserved"]=bool(receipt)
summary["receiptSourceClean"]=bool(receipt and receipt.get("source_tree_clean"))
summary["receiptRecords"]=len(receipt.get("records",[])) if receipt else 0
(out/"capture-producers-clean-summary.json").write_text(json.dumps(summary,indent=2)+"\n",encoding="utf-8")
assert code==0 and not timeout and not missing and summary["named"]["fail"]==0 and summary["named"]["skip"]==0,summary
assert summary["sourceCleanAfter"] and summary["sourceSHAAfter"]==expected and before==after
assert receipt and receipt["source_tree_clean"] and receipt["source_sha"]==expected and len(receipt["records"])==4
