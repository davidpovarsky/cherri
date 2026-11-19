from pathlib import Path
import subprocess,os,time,json,hashlib
root=Path(r"C:\Users\DAVID\Code\cherri-p10r15-menus-asset-fixture-20261119")
out=Path(r"C:\Users\DAVID\Code\cherri-p10r15-evidence-20261119\ci-s4-linux-diagnosis")
sha="b97ae752205f1e747b6c98ff7511f3484dd53bc2"
def git(*args):return subprocess.check_output(["git",*args],cwd=root,encoding="utf-8").strip()
assert git("rev-parse","HEAD")==sha and not git("status","--porcelain")
paths=["tests/menus.cherri","assets/cherri_icon.png"]
before={x:hashlib.sha256((root/x).read_bytes()).hexdigest() for x in paths}
cmd=["go","test","-p=1","-count=1","-json","-timeout=3m","-run","^(TestP15C03OriginalCLIParity|TestP15MenusMigrationCLIParity)$","."]
env=os.environ.copy();env.update(GOMAXPROCS="1",GOFLAGS="-p=1");start=time.monotonic()
with (out/"menus-clean-s4-baseline.jsonl").open("wb") as stream:
 p=subprocess.Popen(cmd,cwd=root,env=env,stdout=stream,stderr=subprocess.STDOUT)
 timedout=False
 try:code=p.wait(timeout=200)
 except subprocess.TimeoutExpired:
  timedout=True;subprocess.run(["taskkill","/PID",str(p.pid),"/T","/F"],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL);code=p.wait()
raw=(out/"menus-clean-s4-baseline.jsonl").read_bytes();events=[]
for line in raw.splitlines():
 try:events.append(json.loads(line))
 except Exception:pass
summary={"sourceSHA":sha,"sourceSHAAfter":git("rev-parse","HEAD"),"sourceCleanBefore":True,"sourceCleanAfter":not git("status","--porcelain"),"publicOriginalsBefore":before,"publicOriginalsAfter":{x:hashlib.sha256((root/x).read_bytes()).hexdigest() for x in paths},"command":cmd,"exitCode":code,"timedOut":timedout,"seconds":round(time.monotonic()-start,3),"bytes":len(raw),"sha256":hashlib.sha256(raw).hexdigest(),"named":{a:sum(e.get("Action")==a and bool(e.get("Test")) for e in events) for a in ["run","pass","fail","skip"]},"failures":[e["Test"] for e in events if e.get("Action")=="fail" and e.get("Test")],"diagnostics":[e.get("Output","") for e in events if "E_WORKSPACE_ACCESS" in e.get("Output","")]}
(out/"menus-clean-s4-baseline-summary.json").write_bytes((json.dumps(summary,indent=2)+"\n").encode())
