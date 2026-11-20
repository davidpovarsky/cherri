import pathlib,subprocess,json,hashlib,time,datetime,shutil,os,collections
root=pathlib.Path(r"C:\Users\DAVID\Code\cherri-p10r15-local-gates-integration-20261120")
expected="6809ba0645211e2cb4086eed4dc6b1593a497ba7"
dest=root/"dist/p15-editor-evidence-20261120/e01-corrected-baseline-6809ba06"
def git(*args):return subprocess.run(["git","--no-replace-objects",*args],cwd=root,capture_output=True,check=True).stdout
assert git("rev-parse","HEAD").decode().strip()==expected
assert git("status","--porcelain")==b""
assert shutil.disk_usage(root).free>=2_000_000_000
dest.mkdir(parents=True,exist_ok=False)
(dest/"submitted-script.py").write_bytes(pathlib.Path(__file__).read_bytes())
command=["go","test","-count=1","-json","-timeout=3m","-run","^TestP15E01","./internal/language/service"]
receipt={"source":expected,"cwd":str(root),"command":command,"status_before":"","free_before":shutil.disk_usage(root).free,"started_at":datetime.datetime.now(datetime.timezone.utc).isoformat(),"environment_overrides":{"GOMAXPROCS":"2"}}
t=time.monotonic();env=dict(os.environ);env["GOMAXPROCS"]="2"
with (dest/"stdout.jsonl").open("wb")as out,(dest/"stderr.txt").open("wb")as err:
 try:
  p=subprocess.run(command,cwd=root,env=env,stdout=out,stderr=err,timeout=240)
  receipt.update(exit_code=p.returncode,timed_out=False)
 except subprocess.TimeoutExpired:
  receipt.update(exit_code=None,timed_out=True)
receipt.update(duration_seconds=time.monotonic()-t,completed_at=datetime.datetime.now(datetime.timezone.utc).isoformat(),status_after=git("status","--porcelain").decode(),head_after=git("rev-parse","HEAD").decode().strip(),free_after=shutil.disk_usage(root).free)
events=[]
for line in (dest/"stdout.jsonl").read_bytes().splitlines():
 try:events.append(json.loads(line))
 except:pass
receipt["named_counts"]=dict(collections.Counter(e.get("Action")for e in events if e.get("Test") and e.get("Action")in["pass","fail","skip"]))
receipt["failed_names"]=[e["Test"]for e in events if e.get("Test")and e.get("Action")=="fail"]
receipt["artifacts"]=[{"path":p.name,"bytes":p.stat().st_size,"sha256":hashlib.sha256(p.read_bytes()).hexdigest()}for p in [dest/"stdout.jsonl",dest/"stderr.txt"]]
(dest/"receipt.json").write_text(json.dumps(receipt,indent=2)+"\n",encoding="utf-8")
print(json.dumps(receipt))
print("CAUSAL_OUTPUT")
for e in events:
 if e.get("Action")=="output" and any(x in e.get("Output","")for x in["actual contextual","offered candidate","FAIL","fatal error"]):print(e.get("Output","").rstrip())
