#!/usr/bin/env python3
"""One authorized offline protocol suite; no retry or native/provider invocation."""
from pathlib import Path
import ast, datetime, hashlib, json, os, pwd, re, subprocess, sys, time
ROOT=Path("/mnt/development/projects/worktrees/cherri-p10r15-505-refresh-20261123")
OUT=Path(__file__).resolve().parent
SOURCE="adf78c827fa7d9c5676a38258d2a6f423991ea1e"
MANIFEST=OUT.parent/"qualification-03-input-manifest.json"
def digest(raw): return {"bytes":len(raw),"sha256":hashlib.sha256(raw).hexdigest()}
def save(name,value):
    raw=(json.dumps(value,indent=2)+"\n").encode()
    with (OUT/name).open("xb") as f:f.write(raw)
    return digest(raw)
def git(*args):
    run=subprocess.run(["git","-C",str(ROOT),*args],stdin=subprocess.DEVNULL,capture_output=True,text=True,timeout=10)
    if run.returncode:raise RuntimeError("SOURCE_GUARD_GIT_FAILED")
    return run.stdout.strip()
manifest=json.loads(MANIFEST.read_text())
def guard():
    if git("rev-parse","HEAD")!=SOURCE or git("status","--porcelain=v1"):
        raise RuntimeError("SOURCE_GUARD_IDENTITY_FAILED")
    rows=[]
    for row in manifest["files"]+manifest["preservedExistingInputs"]:
        current=digest((ROOT/row["path"]).read_bytes())
        if current!={k:row[k] for k in ("bytes","sha256")}:
            raise RuntimeError("SOURCE_GUARD_BYTES_FAILED")
        rows.append({"path":row["path"],**current})
    absent=ROOT/manifest["preservedAbsentPath"]
    if absent.exists() or absent.is_symlink():raise RuntimeError("SOURCE_GUARD_ABSENCE_FAILED")
    return {"sourceCommit":SOURCE,"tree":git("rev-parse","HEAD^{tree}"),"branch":git("branch","--show-current"),"status":"","files":rows,"absent":manifest["preservedAbsentPath"]}
before=guard()
save("source-before.json",before)
temporary=OUT/"tmp";temporary.mkdir(mode=0o700)
env={"PATH":"/usr/local/bin:/usr/bin:/bin","HOME":pwd.getpwuid(os.getuid()).pw_dir,
     "LANG":"C.UTF-8","LC_ALL":"C.UTF-8","TMPDIR":str(temporary),
     "GIT_CONFIG_NOSYSTEM":"1","GIT_CONFIG_GLOBAL":"/dev/null",
     "GIT_TERMINAL_PROMPT":"0","GIT_OPTIONAL_LOCKS":"0"}
argv=["/usr/bin/timeout","--signal=TERM","--kill-after=5s","120s",
      sys.executable,"-I","-B","-m","unittest","discover","-s","ios/scripts",
      "-p","test_ios_xcresult_summary.py","-v"]
tree=ast.parse((ROOT/"ios/scripts/test_ios_xcresult_summary.py").read_bytes())
expected=sorted(n.name for n in ast.walk(tree) if isinstance(n,ast.FunctionDef) and n.name.startswith("test_"))
claim={"schema":"cherri-ios-xcresult-offline-controls/1","attempt":3,"sourceCommit":SOURCE,
       "cwd":str(ROOT),"argv":argv,"suiteBudgetSeconds":120,"killGraceSeconds":5,
       "outerSupervisorSeconds":135,"expectedParents":expected,
       "environmentNames":sorted(env),"tmpdir":str(temporary),"python":sys.version,
       "driver":digest(Path(__file__).read_bytes()),"noRetry":True,
       "scope":"synthetic local process fixtures; one existing-public-recipient encryption; no private-key operation"}
save("claim.original.json",claim)
start_utc=datetime.datetime.now(datetime.timezone.utc).isoformat()
start=time.monotonic();outer_expired=False;exit_code=None
with (OUT/"stdout.original.log").open("xb") as stdout,(OUT/"stderr.original.log").open("xb") as stderr:
    try:
        run=subprocess.run(argv,cwd=ROOT,env=env,stdin=subprocess.DEVNULL,
                           stdout=stdout,stderr=stderr,timeout=135)
        exit_code=run.returncode
    except subprocess.TimeoutExpired:
        outer_expired=True
elapsed=time.monotonic()-start
after_error=None
try:
    after=guard();save("source-after.json",after);unchanged=before==after
except Exception:
    after_error="SOURCE_GUARD_FAILED";unchanged=False
stderr=(OUT/"stderr.original.log").read_text(errors="replace")
stdout=(OUT/"stdout.original.log").read_bytes()
parents=re.findall(r"^(test_[A-Za-z0-9_]+) \(",stderr,re.M)
match=re.search(r"^Ran ([0-9]+) tests? in ([0-9.]+)s$",stderr,re.M)
reported=int(match.group(1)) if match else None
suite_elapsed=float(match.group(2)) if match else None
passed=(exit_code==0 and not outer_expired and unchanged and
        sorted(parents)==expected and reported==len(expected) and
        re.search(r"^OK$",stderr,re.M) is not None)
result={"schema":claim["schema"],"sourceCommit":SOURCE,"attempt":3,
        "startedAt":start_utc,"finishedAt":datetime.datetime.now(datetime.timezone.utc).isoformat(),
        "durationSeconds":elapsed,"suiteReportedSeconds":suite_elapsed,
        "exitCode":exit_code,"outerSupervisorExpired":outer_expired,
        "reportedParents":reported,"observedParents":parents,"expectedParents":expected,
        "sourceUnchanged":unchanged,"sourceGuardError":after_error,
        "outcome":"PASS" if passed else "FAIL_OR_INCOMPLETE",
        "stdout":digest(stdout),"stderr":digest((OUT/"stderr.original.log").read_bytes()),
        "nativeExecutionCredit":False,"formalAcceptanceGranted":False}
binding=save("result.original.json",result)
print(json.dumps({"result":result,"receipt":str(OUT/"result.original.json"),"receiptBinding":binding},indent=2))
raise SystemExit(0 if passed else 1)
