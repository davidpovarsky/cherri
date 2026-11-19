from pathlib import Path
import subprocess,os,time,json,hashlib,collections
root=Path(r"C:\Users\DAVID\Code\cherri-p10r15-menus-asset-fixture-20261119")
out=Path(r"C:\Users\DAVID\Code\cherri-p10r15-evidence-20261119\ci-s4-linux-diagnosis")
sha="fd790278f40c2b1352eba2821863b33ddea29c06"
def git(*args):return subprocess.check_output(["git",*args],cwd=root,encoding="utf-8").strip()
assert git("rev-parse","HEAD")==sha and not git("status","--porcelain")
protected=["tests/menus.cherri","assets/cherri_icon.png","tests/language-v2/acceptance-contract.json","tests/language-v2/repair-regression-plan.json","tests/language-v2/backend-recovery-gates.json"]
before={x:hashlib.sha256((root/x).read_bytes()).hexdigest() for x in protected}
env=os.environ.copy();env.update(GOMAXPROCS="1",GOFLAGS="-p=1")
results=[];start_all=time.monotonic()
for label,package,tests,timeout in [
("menus-clean-fixed-root",".",["TestP15C03OriginalCLIParity","TestP15MenusMigrationCLIParity","TestP15MenusMigrationAssetWorkspace"],200),
("menus-clean-fixed-producer","./tools/language-acceptance",["TestVariadicMigrationProductionEvidence"],330)]:
 cmd=["go","test","-p=1","-count=1","-json","-timeout="+("3m" if timeout==200 else "5m"),"-run","^("+"|".join(tests)+")$",package]
 start=time.monotonic()
 with (out/(label+".jsonl")).open("wb") as stream:
  p=subprocess.Popen(cmd,cwd=root,env=env,stdout=stream,stderr=subprocess.STDOUT);timedout=False
  try:code=p.wait(timeout=timeout)
  except subprocess.TimeoutExpired:
   timedout=True;subprocess.run(["taskkill","/PID",str(p.pid),"/T","/F"],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL);code=p.wait()
 raw=(out/(label+".jsonl")).read_bytes();events=[];outputs=collections.defaultdict(str)
 for line in raw.splitlines():
  try:
   event=json.loads(line);events.append(event)
   if event.get("Action")=="output":outputs[event.get("Test","")]+=event.get("Output","")
  except Exception:pass
 summary={"label":label,"sourceSHA":sha,"sourceSHAAfter":git("rev-parse","HEAD"),"sourceCleanBefore":True,"sourceCleanAfter":not git("status","--porcelain"),"protectedBefore":before,"protectedAfter":{x:hashlib.sha256((root/x).read_bytes()).hexdigest() for x in protected},"command":cmd,"exitCode":code,"timedOut":timedout,"seconds":round(time.monotonic()-start,3),"bytes":len(raw),"sha256":hashlib.sha256(raw).hexdigest(),"named":{a:sum(e.get("Action")==a and bool(e.get("Test")) for e in events) for a in ["run","pass","fail","skip"]},"missingRequired":[t for t in tests if not any(e.get("Test")==t and e.get("Action")=="pass" for e in events)],"failures":[e["Test"] for e in events if e.get("Action")=="fail" and e.get("Test")],"diagnostics":[e.get("Output","") for e in events if ".go:" in e.get("Output","")]}
 (out/(label+"-summary.json")).write_bytes((json.dumps(summary,indent=2)+"\n").encode())
 (out/(label+"-test-output.json")).write_bytes((json.dumps(outputs,indent=2)+"\n").encode())
 results.append(summary)
summary={"sourceSHA":sha,"sourceCleanAfter":not git("status","--porcelain"),"seconds":round(time.monotonic()-start_all,3),"steps":[{k:v for k,v in x.items() if k not in ["diagnostics","protectedBefore","protectedAfter"]} for x in results]}
summary["passed"]=all(x["exitCode"]==0 and not x["timedOut"] and not x["missingRequired"] and x["named"]["fail"]==0 and x["named"]["skip"]==0 and x["sourceCleanAfter"] and x["protectedBefore"]==x["protectedAfter"] for x in results)
(out/"menus-clean-fixed-summary.json").write_bytes((json.dumps(summary,indent=2)+"\n").encode())
