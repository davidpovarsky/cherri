import subprocess,pathlib,json,datetime,time,hashlib,shutil,os
root=pathlib.Path(r"C:\Users\DAVID\Code\cherri-p10r15-local-gates-integration-20261120")
source="46d6a25daddb5101110f1376cc65dbd767407ec2"
def git(*a):return subprocess.check_output(["git","--no-replace-objects","-C",str(root),*a],timeout=20).decode().strip()
assert git("rev-parse","HEAD")==source and not git("status","--porcelain")
out=root/"dist/p15-local-gates-integration/baseline-46d6a25d";out.mkdir(exist_ok=False)
command=["go","test","-count=1","-json","-timeout=3m","-run","^TestP15RP52","."]
start=datetime.datetime.now(datetime.timezone.utc).isoformat();clock=time.monotonic();free=shutil.disk_usage("C:\\").free;timedout=False
(out/"submitted-script.py").write_bytes(pathlib.Path(__file__).read_bytes())
with (out/"go.jsonl").open("wb") as stdout,(out/"stderr.log").open("wb") as stderr:
 p=subprocess.Popen(command,cwd=root,stdout=stdout,stderr=stderr,env=os.environ.copy())
 try:p.wait(timeout=220)
 except subprocess.TimeoutExpired:
  timedout=True;subprocess.run(["taskkill","/PID",str(p.pid),"/T","/F"],capture_output=True,timeout=20);p.wait(timeout=20)
elapsed=time.monotonic()-clock
raw=(out/"go.jsonl").read_bytes();errors=(out/"stderr.log").read_bytes();rows=[]
for line in raw.splitlines():
 try:rows.append(json.loads(line))
 except Exception:pass
record={"source":source,"source_clean_before":True,"source_clean_after":git("rev-parse","HEAD")==source and not git("status","--porcelain"),"command":command,"started_at":start,"finished_at":datetime.datetime.now(datetime.timezone.utc).isoformat(),"elapsed_seconds":elapsed,"exit_code":p.returncode,"timed_out":timedout,"named_pass":sum(bool(e.get("Test")) and e.get("Action")=="pass" for e in rows),"named_fail":[e["Test"] for e in rows if e.get("Test") and e.get("Action")=="fail"],"named_skip":[e["Test"] for e in rows if e.get("Test") and e.get("Action")=="skip"],"stdout_bytes":len(raw),"stdout_sha256":hashlib.sha256(raw).hexdigest(),"stderr_bytes":len(errors),"stderr_sha256":hashlib.sha256(errors).hexdigest(),"free_bytes_before":free,"free_bytes_after":shutil.disk_usage("C:\\").free}
(out/"receipt.json").write_bytes((json.dumps(record,indent=2)+"\n").encode())
print(json.dumps({"receipt":str(out/"receipt.json"),**record,"stderr_tail":errors.decode("utf-8","replace")[-6000:]},ensure_ascii=True))
