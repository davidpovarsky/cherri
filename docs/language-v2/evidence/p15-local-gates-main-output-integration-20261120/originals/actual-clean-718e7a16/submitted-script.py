import subprocess,pathlib,json,datetime,time,hashlib,shutil,os
root=pathlib.Path(r"C:\Users\DAVID\Code\cherri-p10r15-local-gates-integration-20261120")
source="718e7a16ce0bfb1d5b3d132df06ab96f6c9793e9"
def git(*a):return subprocess.check_output(["git","--no-replace-objects","-C",str(root),*a],timeout=20).decode().strip()
assert git("rev-parse","HEAD")==source and not git("status","--porcelain")
out=root/"dist/p15-local-gates-integration/actual-clean-718e7a16";out.mkdir(exist_ok=False)
command=["go","test","-count=1","-json","-timeout=15m","-run","^Test(NativePaths|PurePublic|PublicFrontend|MainOutput)ActualProductionEvidence$","./tools/language-acceptance"]
env=os.environ.copy()
retained={"CHERRI_NATIVE_PATHS_EVIDENCE_OUTPUT":"brg18","CHERRI_PURE_PUBLIC_EVIDENCE_OUTPUT":"brg21","CHERRI_PUBLIC_FRONTEND_EVIDENCE_OUTPUT":"rp52","CHERRI_MAIN_OUTPUT_EVIDENCE_OUTPUT":"m02"}
for k,v in retained.items():env[k]=str(out/v)
start=datetime.datetime.now(datetime.timezone.utc).isoformat();clock=time.monotonic();free=shutil.disk_usage("C:\\").free;timedout=False
(out/"submitted-script.py").write_bytes(pathlib.Path(__file__).read_bytes())
with (out/"go.jsonl").open("wb") as stdout,(out/"stderr.log").open("wb") as stderr:
 p=subprocess.Popen(command,cwd=root,stdout=stdout,stderr=stderr,env=env)
 (out/"launch.json").write_bytes((json.dumps({"source":source,"pid":p.pid,"command":command,"started_at":start,"artifact_roots":{k:env[k] for k in retained}},indent=2)+"\n").encode())
 try:p.wait(timeout=950)
 except subprocess.TimeoutExpired:
  timedout=True;subprocess.run(["taskkill","/PID",str(p.pid),"/T","/F"],capture_output=True,timeout=20);p.wait(timeout=20)
elapsed=time.monotonic()-clock
raw=(out/"go.jsonl").read_bytes();errors=(out/"stderr.log").read_bytes();rows=[]
for line in raw.splitlines():
 try:rows.append(json.loads(line))
 except Exception:pass
record={"source":source,"source_clean_before":True,"source_clean_after":git("rev-parse","HEAD")==source and not git("status","--porcelain"),"command":command,"artifact_roots":{k:env[k] for k in retained},"started_at":start,"finished_at":datetime.datetime.now(datetime.timezone.utc).isoformat(),"elapsed_seconds":elapsed,"exit_code":p.returncode,"timed_out":timedout,"named_pass":sum(bool(e.get("Test")) and e.get("Action")=="pass" for e in rows),"named_fail":[e["Test"] for e in rows if e.get("Test") and e.get("Action")=="fail"],"named_skip":[e["Test"] for e in rows if e.get("Test") and e.get("Action")=="skip"],"stdout_bytes":len(raw),"stdout_sha256":hashlib.sha256(raw).hexdigest(),"stderr_bytes":len(errors),"stderr_sha256":hashlib.sha256(errors).hexdigest(),"free_bytes_before":free,"free_bytes_after":shutil.disk_usage("C:\\").free}
(out/"receipt.json").write_bytes((json.dumps(record,indent=2)+"\n").encode())
print(json.dumps({"receipt":str(out/"receipt.json"),**record,"stderr_tail":errors.decode("utf-8","replace")[-3000:]},ensure_ascii=True))
