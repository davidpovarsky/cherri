import os,pathlib,json,subprocess,time,hashlib,datetime,collections,shutil
source=pathlib.Path(r"<WINDOWS_USER_HOME>\Code\cherri-p10r15-formal-20261119\source")
docs=source.parent/"cherrilang.org"
out=pathlib.Path(r"<WINDOWS_USER_HOME>\Code\cherri-p10r15-evidence-20261119\formal-1cf9ae7")
S="1cf9ae776a9560217d04288a1ecf4b5277053736";D="9fb6256a4549d93414e9b65318d4794ae40fbee7"
go=r"C:\Program Files\Go\bin\go.exe"
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def digest(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def git(root,*args):
 p=subprocess.run(["git",*args],cwd=root,capture_output=True,text=True,encoding="utf-8",errors="replace",timeout=30)
 if p.returncode:raise RuntimeError("git failed: "+p.stderr[-1000:])
 return p.stdout.strip()
def snapshot():
 return {"source":git(source,"rev-parse","HEAD"),"sourceStatus":git(source,"status","--porcelain","--untracked-files=all"),"docs":git(docs,"rev-parse","HEAD"),"docsStatus":git(docs,"status","--porcelain","--untracked-files=all"),"contracts":{n:digest(source/n) for n in ["tests/language-v2/acceptance-contract.json","tests/language-v2/repair-regression-plan.json","tests/language-v2/backend-recovery-gates.json"]},"manifest":digest(source/"skills/cherri-shortcuts/compatibility-manifest.json")}
def write(n,data):
 temp=out/(n+".tmp");temp.write_text(json.dumps(data,ensure_ascii=False,indent=2)+"\n",encoding="utf-8");os.replace(temp,out/n)
out.mkdir(exist_ok=True)
assert not (out/"run-summary.json").exists() and not (out/"run.log").exists(),"Baseline already launched; inspect instead of duplicate"
(out/"runner.py").write_bytes(pathlib.Path(__file__).read_bytes())
before=snapshot()
assert before["source"]==S and before["docs"]==D and not before["sourceStatus"] and not before["docsStatus"]
assert all((p/".git").is_dir() and not (p/".git/objects/info/alternates").exists() for p in [source,docs])
env=os.environ.copy()
for k in list(env):
 if k.startswith("CHERRI_"):env.pop(k)
env["CHERRI_DOCS_DIR"]=str(docs);env["GOMAXPROCS"]="1";env["GOFLAGS"]="-p=1"
env["PATH"]=str(pathlib.Path(go).parent)+os.pathsep+env.get("PATH","")
cmd=[go,"run","./tools/language-acceptance","--phase","local","--out",str(out)]
started=now();t0=time.monotonic()
write("before.json",{"snapshot":before,"started":started,"command":cmd,"env":{"GOMAXPROCS":"1","GOFLAGS":"-p=1","CHERRI_DOCS_DIR":str(docs)},"runnerSHA256":digest(out/"runner.py"),"scope":"ACTUAL_ALL_179_INTERMEDIATE_BASELINE_NOT_FINAL_S2"})
timed_out=False
with (out/"run.log").open("wb") as log:
 p=subprocess.Popen(cmd,cwd=source,env=env,stdin=subprocess.DEVNULL,stdout=log,stderr=subprocess.STDOUT,creationflags=subprocess.BELOW_NORMAL_PRIORITY_CLASS|subprocess.CREATE_NEW_PROCESS_GROUP)
 write("launch.json",{"started":started,"pid":p.pid,"command":cmd,"source":S,"docs":D,"outerDeadlineSeconds":3000})
 last=0
 while p.poll() is None:
  elapsed=time.monotonic()-t0
  if elapsed>=3000:
   timed_out=True
   killed=subprocess.run(["taskkill","/PID",str(p.pid),"/T","/F"],capture_output=True,text=True,encoding="utf-8",errors="replace",timeout=20)
   write("owned-tree-timeout-cleanup.json",{"pid":p.pid,"exit":killed.returncode,"stdout":killed.stdout,"stderr":killed.stderr,"time":now()})
   try:p.wait(timeout=15)
   except subprocess.TimeoutExpired:pass
   break
  if elapsed-last>=20:
   last=elapsed
   with (out/"run.log").open("rb") as f:
    f.seek(max(0,(out/"run.log").stat().st_size-1800));tail=f.read().decode("utf-8","replace")
   write("progress.json",{"source":S,"pid":p.pid,"time":now(),"seconds":round(elapsed,1),"logBytes":(out/"run.log").stat().st_size,"tail":tail})
  time.sleep(1)
 returncode=p.poll()
after=snapshot()
summary={"scope":"ACTUAL_ALL_179_INTERMEDIATE_BASELINE_NOT_FINAL_S2","source":S,"docs":D,"started":started,"finished":now(),"seconds":round(time.monotonic()-t0,3),"exit":returncode,"timedOut":timed_out,"unchanged":before==after,"before":before,"after":after,"status":"INCOMPLETE_OWN_DEADLINE" if timed_out else "COMPLETED_PROCESS","logSHA256":digest(out/"run.log"),"logBytes":(out/"run.log").stat().st_size}
report=out/"acceptance-results.json"
if report.is_file():
 d=json.loads(report.read_text(encoding="utf-8"))
 summary["report"]={k:d[k] for k in ["timestamp","phase","closure_complete","total_cases","passed_cases","failed_cases","skipped_cases"] if k in d}
 summary["statuses"]=dict(collections.Counter(r["status"] for r in d["results"]))
 summary["failedIDs"]=[r["id"] for r in d["results"] if r["status"] in ["FAILED","FAIL"]]
 summary["outputs"]={f.name:{"bytes":f.stat().st_size,"sha256":digest(f)} for f in out.glob("*.json") if f.name not in ["progress.json","run-summary.json","preparation.json"]}
else:summary["reportAbsent"]=True
write("run-summary.json",summary)
print(json.dumps({k:v for k,v in summary.items() if k not in ["before","after","outputs"]},ensure_ascii=False))
