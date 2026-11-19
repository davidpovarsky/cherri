import pathlib,subprocess,os,json,hashlib,time,datetime,sys,ctypes
base=pathlib.Path(r"<HOST_HOME>\Code")
parent=base/"cherri-p10r15-formal-k2-20261119";assert not parent.exists();parent.mkdir()
source=parent/"source";docs=parent/"cherrilang.org"
primary=base/"cherri-p10r15-project-closure-20261118";docssource=base/"cherrilang-p10r15-docs-s2-20261119"
k="28bc5fa29d0c8d3ca4dfa2d96b42ee7374d65b46";d="daa58850140bc1d739806633932d01e084463404"
out=base/"cherri-p10r15-evidence-20261119"/"formal-28bc5fa";assert not out.exists();out.mkdir()
def git(p,*args):return subprocess.check_output(["git",*args],cwd=p,text=True,encoding="utf-8",timeout=45).strip()
for origin,target,commit,canonical in [(primary,source,k,"https://github.com/davidpovarsky/cherri.git"),(docssource,docs,d,"https://github.com/davidpovarsky/cherrilang.org.git")]:
 subprocess.run(["git","clone","--no-checkout","--local",str(origin),str(target)],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.PIPE,timeout=90)
 git(target,"config","core.autocrlf","false");git(target,"remote","set-url","origin",canonical);git(target,"checkout","--detach",commit)
 assert (target/".git").is_dir() and not (target/".git/objects/info/alternates").exists()
 assert git(target,"rev-parse","HEAD")==commit and git(target,"status","--porcelain")=="" and git(target,"remote","get-url","origin")==canonical
contracts=["tests/language-v2/acceptance-contract.json","tests/language-v2/repair-regression-plan.json","tests/language-v2/backend-recovery-gates.json"]
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def snap():return {"source":git(source,"rev-parse","HEAD"),"sourceStatus":git(source,"status","--porcelain"),"docs":git(docs,"rev-parse","HEAD"),"docsStatus":git(docs,"status","--porcelain"),"contracts":{f:sha(source/f) for f in contracts},"manifest":sha(source/"skills/cherri-shortcuts/compatibility-manifest.json")}
before=snap();assert before["source"]==k and before["docs"]==d
assert list(before["contracts"].values())==["9fdfa8910b09824dc61395d8ddf4aa8fe8914619af0fdde59c63b67ab10b895e","eba070df0e1ff229a94e52d977a09c2a0c125a31b1a69caf32787d41600074f5","e7fbb3ad177c5142935f71a46ca7e89b84822061bfc0919eec59708811ab7e7c"]
manifest=json.loads((source/"skills/cherri-shortcuts/compatibility-manifest.json").read_text(encoding="utf-8"))
assert manifest["compilerCommit"]=="7b0100a2a9ce5991ca8ffe82c805278e5aa7754e" and manifest["docsCommit"]==d
(out/"before.json").write_bytes((json.dumps(before,indent=2)+"\n").encode())
(out/"runner.py").write_bytes(pathlib.Path(__file__).read_bytes())
(out/"preparation.json").write_bytes((json.dumps({"source":str(source),"docs":str(docs),"sourcePin":k,"docsPin":d,"ordinaryRepositories":True,"canonicalOrigins":True,"alternates":False,"scope":"ACTUAL_ALL_179_K2_INTERMEDIATE_NOT_S3_PROJECT_CLOSURE","liveSigning":"ENABLED_FOR_PUBLIC_SYNTHETIC_K05_ONLY"},indent=2)+"\n").encode())
env={key:value for key,value in os.environ.items() if not key.startswith("CHERRI")}
env.update({"GOMAXPROCS":"1","GOFLAGS":"-p=1","CHERRI_DOCS_DIR":str(docs),"CHERRI_K05_LIVE_SIGNING":"1","CHERRI_K05_EVIDENCE_DIR":str(out/"actual-live-signing"),"PYTHONDONTWRITEBYTECODE":"1"})
cmd=[r"C:\Program Files\Go\bin\go.exe","run","./tools/language-acceptance","--phase","local","--out",str(out)]
log=out/"run.log";start=time.monotonic();started=datetime.datetime.now(datetime.timezone.utc).isoformat();timed=False
with log.open("wb") as f:
 p=subprocess.Popen(cmd,cwd=source,env=env,stdout=f,stderr=subprocess.STDOUT,creationflags=subprocess.BELOW_NORMAL_PRIORITY_CLASS)
 (out/"launch.json").write_bytes((json.dumps({"command":cmd,"pid":p.pid,"source":k,"docs":d,"started":started,"deadlineSeconds":3000,"priority":"BELOW_NORMAL_PRIORITY_CLASS","GOMAXPROCS":"1","GOFLAGS":"-p=1","publicLiveSigningEnabled":True},indent=2)+"\n").encode())
 while True:
  try:code=p.wait(timeout=20);break
  except subprocess.TimeoutExpired:
   elapsed=time.monotonic()-start
   (out/"progress.json").write_bytes((json.dumps({"source":k,"pid":p.pid,"time":datetime.datetime.now(datetime.timezone.utc).isoformat(),"seconds":round(elapsed,1),"logBytes":log.stat().st_size,"tail":log.read_text(encoding="utf-8",errors="replace")[-1500:]},indent=2)+"\n").encode())
   if elapsed>=3000:
    timed=True;subprocess.run(["taskkill","/PID",str(p.pid),"/T","/F"],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=30);code=p.wait(timeout=30);break
after=snap();summary={"scope":"ACTUAL_ALL_179_K2_INTERMEDIATE_NOT_S3_PROJECT_CLOSURE","source":k,"docs":d,"compilerPin":manifest["compilerCommit"],"started":started,"finished":datetime.datetime.now(datetime.timezone.utc).isoformat(),"seconds":round(time.monotonic()-start,3),"exit":code,"timedOut":timed,"unchanged":before==after,"before":before,"after":after,"publicLiveSigningEnabled":True,"status":"INCOMPLETE_OWN_DEADLINE" if timed else "COMPLETED_PROCESS","logSHA256":sha(log),"logBytes":log.stat().st_size}
report=out/"acceptance-results.json"
if report.exists():
 data=json.loads(report.read_text(encoding="utf-8"));summary["report"]={key:data[key] for key in ["timestamp","phase","closure_complete","total_cases","passed_cases","failed_cases","skipped_cases"]}
 counts={}
 for row in data["results"]:counts[row["status"]]=counts.get(row["status"],0)+1
 summary["statuses"]=counts;summary["failedIDs"]=[x["id"] for x in data["results"] if x["status"]=="FAILED"]
summary["outputs"]={f.name:{"bytes":f.stat().st_size,"sha256":sha(f)} for f in sorted(out.iterdir()) if f.is_file() and f.suffix==".json" and f.name not in ["progress.json","run-summary.json"]}
(out/"run-summary.json").write_bytes((json.dumps(summary,indent=2)+"\n").encode())
print(json.dumps({key:value for key,value in summary.items() if key not in ["before","after","outputs"]}),flush=True)
assert not timed and before==after and code==0 and report.exists()
