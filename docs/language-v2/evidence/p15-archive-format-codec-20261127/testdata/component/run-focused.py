import datetime,hashlib,json,os,pathlib,signal,subprocess,sys,time,shutil
root=pathlib.Path("/mnt/development/projects/agent-workspaces/cherri-macro-completion-20261127")
repo=root/"archive-format"
evidence=root/"archive-format-evidence"
phase,pattern=sys.argv[1:]
out=evidence/phase
out.mkdir(exist_ok=False)
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def git(*args):
 p=subprocess.run(["git","-C",str(repo),*args],text=True,capture_output=True,check=True)
 return p.stdout
pins=["action.go","decompile.go","archive_format_codec.go","archive_format_codec_test.go",
 "actions/documents.cherri","internal/language/schema/generated_registry.go",
 "internal/language/analysis/enums.go","docs/apple-runtime/macos-27/toolkit-registry.json",
 "tests/archive_format.cherri"]
def snapshot():
 files={}
 for name in pins:
  p=repo/name
  if p.is_file():
   b=p.read_bytes();files[name]={"bytes":len(b),"sha256":hashlib.sha256(b).hexdigest()}
 return {"head":git("rev-parse","HEAD").strip(),"status":git("status","--porcelain"),"files":files}
before=snapshot()
(out/"source-before.original.json").write_text(json.dumps(before,indent=2,sort_keys=True)+"\n")
(out/"tracked-diff.original.patch").write_text(git("diff","--binary"))
(out/"source").mkdir()
for name in ["archive_format_codec_test.go","archive_format_codec.go"]:
 p=repo/name
 if p.is_file():(out/"source"/name).write_bytes(p.read_bytes())
env=os.environ.copy()
overrides={"GOCACHE":"/mnt/development/tooling/caches/go-build/ubuntu",
 "GOMODCACHE":"/mnt/development/tooling/caches/go-mod/ubuntu",
 "GOPATH":"/mnt/development/tooling/go-workspace/ubuntu",
 "TMPDIR":str(evidence/"tmp"),"GOTMPDIR":str(evidence/"tmp"),
 "GOMAXPROCS":"2","CGO_ENABLED":"0"}
env.update(overrides)
command=["/usr/local/bin/go","test","-p=2","-count=1","-timeout=5m","-run",pattern,"-json","."]
receipt={"schema":"cherri-archive-format-focused-run/1","phase":phase,"startedUTC":now(),
 "cwd":str(repo),"command":command,"environmentOverrides":overrides,"sourceBefore":before,
 "diskFreeBefore":shutil.disk_usage("/mnt/development").free}
(out/"intent.original.json").write_text(json.dumps(receipt,indent=2,sort_keys=True)+"\n")
start=time.monotonic()
timed_out=False
with (out/"stdout.original.jsonl").open("wb") as stdout, (out/"stderr.original").open("wb") as stderr:
 p=subprocess.Popen(command,cwd=repo,env=env,stdout=stdout,stderr=stderr,start_new_session=True)
 receipt["pid"]=p.pid
 (out/"started.original.json").write_text(json.dumps({"startedUTC":now(),"pid":p.pid})+"\n")
 try:exit_code=p.wait(timeout=360)
 except subprocess.TimeoutExpired:
  timed_out=True
  os.killpg(p.pid,signal.SIGTERM)
  try:p.wait(timeout=5)
  except subprocess.TimeoutExpired:os.killpg(p.pid,signal.SIGKILL);p.wait()
  exit_code=p.returncode
after=snapshot()
results=[]
parse_errors=[]
for line in (out/"stdout.original.jsonl").read_text(errors="replace").splitlines():
 try:
  event=json.loads(line)
  if event.get("Action") in ["pass","fail","skip"]:
   results.append({k:event[k] for k in ["Action","Package","Test","Elapsed"] if k in event})
 except json.JSONDecodeError:parse_errors.append(line[:200])
logs={}
for name in ["stdout.original.jsonl","stderr.original"]:
 b=(out/name).read_bytes();logs[name]={"bytes":len(b),"sha256":hashlib.sha256(b).hexdigest()}
receipt.update({"finishedUTC":now(),"elapsedSeconds":round(time.monotonic()-start,3),
 "goExitCode":exit_code,"timedOut":timed_out,"sourceAfter":after,
 "sourceUnchangedDuringRun":before==after,"logs":logs,"testResults":results,
 "nonJSONStdoutLines":parse_errors,"diskFreeAfter":shutil.disk_usage("/mnt/development").free})
(out/"receipt.original.json").write_text(json.dumps(receipt,indent=2,sort_keys=True)+"\n")
print(json.dumps({"phase":phase,"goExitCode":exit_code,"timedOut":timed_out,
 "sourceUnchangedDuringRun":before==after,"elapsedSeconds":receipt["elapsedSeconds"],
 "resultCounts":{k:sum(x["Action"]==k for x in results) for k in ["pass","fail","skip"]},
 "receipt":str(out/"receipt.original.json"),"logs":logs}))
sys.exit(exit_code if exit_code>=0 else 1)
