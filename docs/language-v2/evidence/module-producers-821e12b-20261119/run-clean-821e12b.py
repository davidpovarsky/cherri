from pathlib import Path
import os,subprocess,time,json,hashlib
repo=Path(r"C:\Users\DAVID\Code\cherri-p10r15-openminis-evidence-20261119")
out=Path(r"C:\Users\DAVID\Code\cherri-p10r15-evidence-20261119\module-formal-7476d80")
out.mkdir(parents=True,exist_ok=True)
env=os.environ.copy();env.update({"GOMAXPROCS":"1","GOFLAGS":"-p=1"})
required={"public-complete-working":["TestP15IM01CyclePathAndDuplicateAliasDiagnostics","TestP15IM02RejectsModuleRuntimeInitialization","TestP15IM02LegacyMigrationDoesNotDropInitialization","TestP15IM03DirectServiceWithoutMain","TestP15IM05UnresolvedLocalDependencyNoInstallation"],"module-clean-821e12b":["TestModuleProductionEvidence","TestModuleProductionPathsExist","TestModuleEvidenceRejectsDetachedAndCrossTierRecords","TestModuleStandardLibraryUnitEvidence","TestWorkspaceProductionPathsExist","TestWorkspaceResourceProductionEvidence","TestWorkspaceEvidenceRejectsUnrelatedCommands"]}
head=subprocess.check_output(["git","rev-parse","HEAD"],cwd=repo,text=True,encoding="utf-8").strip()
assert head=="821e12bf037939cd30075d06b11fd35fc5375a77"
assert not subprocess.check_output(["git","status","--porcelain"],cwd=repo,text=True,encoding="utf-8").strip()
runs=[["module-clean-821e12b", ["go", "test", "-p=1", "-json", "-count=1", "-timeout=5m", "-run", "^(TestModuleProductionEvidence|TestModuleProductionPathsExist|TestModuleEvidenceRejectsDetachedAndCrossTierRecords|TestModuleStandardLibraryUnitEvidence|TestWorkspaceProductionPathsExist|TestWorkspaceResourceProductionEvidence|TestWorkspaceEvidenceRejectsUnrelatedCommands)$", "./tools/language-acceptance"]]]
for name,cmd in runs:
 start=time.monotonic()
 with (out/(name+".jsonl")).open("wb") as stream:
  process=subprocess.Popen(cmd,cwd=repo,env=env,stdout=stream,stderr=subprocess.STDOUT)
  timed_out=False
  try:exit_code=process.wait(timeout=330)
  except subprocess.TimeoutExpired:
   timed_out=True
   subprocess.run(["taskkill","/PID",str(process.pid),"/T","/F"],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
   exit_code=process.wait()
 raw=(out/(name+".jsonl")).read_bytes();events=[]
 for line in raw.splitlines():
  try:events.append(json.loads(line))
  except Exception:pass
 summary={"sourceSHA":subprocess.check_output(["git","rev-parse","HEAD"],cwd=repo,text=True,encoding="utf-8").strip(),"sourceState":"exact clean module producer commit","command":cmd,"exitCode":exit_code,"timedOut":timed_out,"seconds":round(time.monotonic()-start,3),"bytes":len(raw),"sha256":hashlib.sha256(raw).hexdigest(),"named":{action:sum(e.get("Action")==action and bool(e.get("Test")) for e in events) for action in ["run","pass","fail","skip"]},"failures":[e.get("Test") for e in events if e.get("Action")=="fail" and e.get("Test")],"diagnostics":[e.get("Output","") for e in events if ".go:" in e.get("Output","")]}
 summary["missingRequiredTests"]=[test for test in required[name] if not any(e.get("Action")=="pass" and e.get("Test")==test for e in events)]
 (out/(name+"-summary.json")).write_text(json.dumps(summary,indent=2)+"\n",encoding="utf-8")

assert not subprocess.check_output(["git","status","--porcelain"],cwd=repo,text=True,encoding="utf-8").strip()
mapping=repo/"tests/language-v2/requirements-evidence-map.json"
before=mapping.read_bytes()
if summary["exitCode"]==0 and not summary["missingRequiredTests"]:
 command=["go","run","./tools/language-acceptance","--build-evidence-map"]
 start=time.monotonic()
 result=subprocess.run(command,cwd=repo,env=env,capture_output=True,timeout=120)
 after=mapping.read_bytes()
 record={"sourceSHA":head,"command":command,"exitCode":result.returncode,"seconds":round(time.monotonic()-start,3),"beforeSha256":hashlib.sha256(before).hexdigest(),"afterSha256":hashlib.sha256(after).hexdigest(),"identical":before==after,"stdout":result.stdout.decode("utf-8","replace"),"stderr":result.stderr.decode("utf-8","replace")}
 (out/"mapping-clean-replay.json").write_text(json.dumps(record,indent=2)+"\n",encoding="utf-8")
 assert result.returncode==0 and before==after
 assert not subprocess.check_output(["git","status","--porcelain"],cwd=repo,text=True,encoding="utf-8").strip()
