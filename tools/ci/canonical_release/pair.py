"""Actual Q compiler/migrator pair using the original setup and pair protocol.

The original two600s builds,1380s whole owner and120s stage inspector are
preserved. Hosted source paths/account are fixed separately; this phase grants
no final tuple, native or179 acceptance.
"""
from pathlib import Path
import hashlib,importlib.util,json,os,re,stat,sys,time
P=Path(__file__).resolve().parent
PINS={"hosted_source.py":"4f0f26c7e3cd5be0d5104c9c4a8b1e695a315507a2c08175c2349cf9de19c21d",
"owned_source_process.py":"4672474ddb270a5857914041ebb279f20431f87e9da173e6cd8bf83b3fb11c85",
"observe_go_exclusive.py":"97e699223bc7438e6d4f9de643d9ebc27497e28e9c3a31715f93652c35248ba8"}
for name,pin in PINS.items():
 if hashlib.sha256((P/name).read_bytes()).hexdigest()!=pin:raise SystemExit("PAIR_OWNER_DEPENDENCY_CHANGED:"+name)
_spec=importlib.util.spec_from_file_location("cherri_hosted_release_source",P/"hosted_source.py")
fixed=importlib.util.module_from_spec(_spec)
sys.modules[_spec.name]=fixed
_spec.loader.exec_module(fixed)
HEAD="44707350fb6e52dd0d947256bced105ac762a1ac"
TREE="07db998051c5950be6326546b7e743d21beb8390"
PAIR=fixed.BASE/"pair"
W=PAIR/"src"
E=fixed.EVIDENCE/"pair"
PREPARATION=fixed.EVIDENCE/"pair-preparation/result.original.json"
PREPARATION_SHA=os.environ.get("CHERRI_PAIR_PREPARATION_SHA","")
fixed.require(re.fullmatch(r"[0-9a-f]{64}",PREPARATION_SHA) is not None,"PAIR_BOUND_PREPARATION_OUTPUT")
ADMISSION={"root":939524096,"development":1073741824,"shm":402653184,"memoryAvailable":3221225472}
COMMON_FIELDS=("languageVersion","catalogSchemaVersion","protocolVersion","schemaFingerprint","codecFingerprint","macroFingerprint")
INSPECT_CODE=f"""from pathlib import Path
import sys,json
sys.path.insert(0,{str(W/"tools/p10r15")!r})
from stage_remote_release import inspect_runtime_pair
compiler,migration,identity=inspect_runtime_pair(
Path({str(PAIR/"cherri")!r}),
Path({str(PAIR/"cherri-migrate")!r}),
{HEAD!r},
{str(fixed.GO)!r})
print(json.dumps(identity,sort_keys=True))
"""
def main():
 fixed.require(len(sys.argv)==1,"PAIR_FIXED_NO_ARGUMENT_ROUTE")
 with fixed.owned.WholeBudget(1380,cleanup_seconds=8,final_reserve=20) as budget:
  fixed.require(E.parent.resolve(strict=True)==E.parent and not E.exists(),"PAIR_FRESH_EVIDENCE")
  E.mkdir(mode=0o750);os.chown(E,os.getuid(),os.getgid())
  owner=fixed.SourceOwner({"worktree":str(W),"sourceRole":"compiler-pair"},E,budget)
  owner.env["CGO_ENABLED"]="0"
  before=after=None;binary_pins={};caps={};identities={};provenance=None;processes=[];failures=[];cache_receipt=None
  fixed.owned.original(E/"owner.original.py",Path(__file__).read_bytes())
  for name in PINS:fixed.owned.original(E/name,(P/name).read_bytes())
  builds=[("compiler-build",[str(fixed.GO),"build","-buildvcs=true","-trimpath","-ldflags=-s -w","-o",str(PAIR/"cherri"),"."],600),
          ("migrator-build",[str(fixed.GO),"build","-buildvcs=true","-trimpath","-ldflags=-s -w","-o",str(PAIR/"cherri-migrate"),"./cmd/cherri-migrate"],600)]
  fixed.owned.original(E/"selection.original.json",{"schema":"cherri-real-source-pair-selection/1","source":HEAD,"tree":TREE,"worktree":str(W),"pair":str(PAIR),
    "builds":[{"name":n,"argv":a,"seconds":s} for n,a,s in builds],"wholeOwnerSeconds":1380,"outerSeconds":1440,
    "originalFlagSource":"skills/cherri-shortcuts/scripts/setup.sh","ordinaryCheckout":True,"shallow":True,
    "ancestryQualification":False,"liveMutation":False,"finalImplementationPair":False})
  def command(label,argv,seconds=120):
   row,out,err=owner.command(label,argv,seconds,require_zero=False);processes.append({"stage":label,**row})
   fixed.owned.require_success(row)
   return owner.read(out),owner.read(err)
  try:
   raw=owner.read(PREPARATION);fixed.require(fixed.sha(raw)==PREPARATION_SHA,"PAIR_EXACT_PREPARATION")
   prep=json.loads(raw);fixed.require(prep["source"]==HEAD and prep["tree"]==TREE and prep["status"]=="PREPARED_EXACT_ORDINARY_SOURCE","PAIR_PREPARATION_IDENTITY")
   fixed.owned.original(E/"source-preparation.original.json",raw)
   for name in ("cherri","cherri-migrate","cherri.commit","cherri.sha256","cherri-migrate.commit","cherri-migrate.sha256","compiler-receipt.json"):
    fixed.require(not (PAIR/name).exists() and not (PAIR/name).is_symlink(),"PAIR_OUTPUT_MUST_BE_ABSENT:"+name)
   before=owner.snapshot();fixed.owned.original(E/"source-before.original.json",before)
   fixed.require((before["head"],before["tree"],before["branch"],before["status"])==(HEAD,TREE,"",""),"PAIR_EXACT_CLEAN_DETACHED_SOURCE")
   fixed.require(owner.read(W/".git/shallow")==(HEAD+"\n").encode(),"PAIR_SHALLOW_BOUNDARY")
   origin=owner.git("remote","get-url","origin").decode().strip()
   fixed.require(origin=="https://github.com/davidpovarsky/cherri.git","PAIR_CANONICAL_ORIGIN")
   fixed.require(fixed.sha(owner.read(fixed.GO))==fixed.GO_SHA256,"PAIR_PINNED_GO")
   fixed.require(fixed.sha(owner.read(W/"docs/language-v2/actions-reference.md"))=="370ccff0d1d2708117522d847ebd306124751d7cfb4968fc9c4dcadb38333a20","PAIR_CANONICAL_DOCS_SOURCE")
   setup=owner.read(W/"skills/cherri-shortcuts/scripts/setup.sh")
   fixed.require(b"go build -buildvcs=true -trimpath -ldflags='-s -w' -o \"$TMP_BIN\" ." in setup and
                 b"go build -buildvcs=true -trimpath -ldflags='-s -w' -o \"$TMP_MIGRATION\" ./cmd/cherri-migrate" in setup,"PAIR_ORIGINAL_BUILD_FLAGS")
   fixed.owned.original(E/"setup.original.sh",setup)
   fixed.owned.original(E/"environment.original.json",owner.env)
   _,out,err=owner.observe_concurrent_owner();decision=json.loads(owner.read(out))
   fixed.require(not owner.read(err) and decision.get("admitted") is True and decision.get("blockedPids")==[],"PAIR_EXCLUSIVE_GO_ADMISSION")
   available=fixed.free();admitted=all(available[k]>=v for k,v in ADMISSION.items())
   fixed.owned.original(E/"preflight.original.json",{"free":available,"admission":ADMISSION,"liveFloor":fixed.LIVE_FLOORS,"admitted":admitted})
   fixed.require(admitted,"PAIR_CAPACITY_ADMISSION")
   for label,argv,seconds in builds:command(label,argv,seconds)
   for name,package in (("cherri","github.com/electrikmilk/cherri"),("cherri-migrate","github.com/electrikmilk/cherri/cmd/cherri-migrate")):
    tool=PAIR/name
    fixed.require(tool.is_file() and not tool.is_symlink(),"PAIR_REGULAR_BINARY")
    info=tool.stat()
    fixed.require(stat.S_ISREG(info.st_mode) and info.st_uid==os.getuid() and info.st_mode&0o555==0o555,"PAIR_READABLE_EXECUTABLE")
    raw=owner.read(tool)
    binary_pins[name]={"path":str(tool),"bytes":len(raw),"sha256":fixed.sha(raw),"uid":info.st_uid,"gid":info.st_gid,"mode":oct(stat.S_IMODE(info.st_mode)),"source":HEAD,"modulePath":package}
    output,stderr=command(name+"-go-buildinfo",[str(fixed.GO),"version","-m",str(tool)],120)
    fixed.require(not stderr and re.search(rb"\bpath\s+"+re.escape(package.encode())+rb"\s*$",output,re.M) is not None,"PAIR_MAIN_PACKAGE")
    fixed.require(re.search(rb"\bbuild\s+vcs.revision="+HEAD.encode()+rb"\s*$",output,re.M) is not None and
                  re.search(rb"\bbuild\s+vcs.modified=false\s*$",output,re.M) is not None,"PAIR_EMBEDDED_CLEAN_SOURCE")
    output,stderr=command(name+"-installed-inspection",[str(PAIR/"cherri-migrate"),"--inspect-build-json",str(tool)],30)
    fixed.require(not stderr,"PAIR_INSTALLED_INSPECTION_STDERR")
    identity=json.loads(output);identities[name]=identity
    fixed.require(identity["tool"]==name and identity["sourceCommit"]==HEAD and identity["buildState"]=="clean" and identity["modulePath"]==package and identity["sha256"]==binary_pins[name]["sha256"] and identity["goos"]=="linux" and identity["goarch"]=="arm64","PAIR_INSTALLED_IDENTITY")
    output,stderr=command(name+"-capabilities",[str(tool),"--capabilities-json"],30)
    fixed.require(not stderr,"PAIR_CAPABILITIES_STDERR")
    caps[name]=json.loads(output)
    fixed.owned.original(E/(name+"-capabilities.original.json"),output)
   for field in COMMON_FIELDS:
    fixed.require(type(caps["cherri"].get(field)) is str and caps["cherri"][field]==caps["cherri-migrate"].get(field),"PAIR_CAPABILITIES_CONSISTENCY:"+field)
   for field in ("schemaFingerprint","codecFingerprint","macroFingerprint"):
    fixed.require(re.fullmatch(r"[0-9a-f]{64}",caps["cherri"][field]) is not None,"PAIR_FINGERPRINT:"+field)
   fixed.require(caps["cherri-migrate"]["sourceCommit"]==HEAD and caps["cherri-migrate"]["buildState"]=="clean" and caps["cherri-migrate"]["tool"]=="cherri-migrate","PAIR_MIGRATOR_RUNNING_IDENTITY")
   output,stderr=command("original-stage-pair-inspector",["python3","-B","-c",INSPECT_CODE],120)
   fixed.require(not stderr,"PAIR_STAGE_INSPECTION_STDERR")
   provenance=json.loads(output)
   fixed.require(provenance["compilerCommit"]==HEAD and provenance["sha256"]==binary_pins["cherri"]["sha256"] and provenance["goos"]=="linux" and provenance["goarch"]=="arm64" and provenance["vcsModified"] is False,"PAIR_STAGE_COMPILER_IDENTITY")
   migration=provenance["migration"]
   fixed.require(migration["compilerCommit"]==HEAD and migration["sha256"]==binary_pins["cherri-migrate"]["sha256"] and migration["goos"]=="linux" and migration["goarch"]=="arm64" and migration["vcsModified"] is False,"PAIR_STAGE_MIGRATION_IDENTITY")
   fixed.owned.original(E/"runtime-pair-provenance.original.json",provenance)
  except BaseException as error:
   failures.append({"stage":"execution","type":type(error).__name__,"message":str(error)})
  finally:
   with budget.finalizing():
    try:
     if before is not None:
      after=owner.snapshot(finalizing=True);fixed.owned.original(E/"source-after.original.json",after)
      fixed.require(before==after,"PAIR_SOURCE_CHANGED")
      fixed.require(owner.read(W/".git/shallow")==(HEAD+"\n").encode(),"PAIR_SHALLOW_BOUNDARY_CHANGED")
     for name,pin in binary_pins.items():
      tool=Path(pin["path"]);info=tool.stat()
      fixed.require(fixed.sha(owner.read(tool))==pin["sha256"] and info.st_uid==pin["uid"] and oct(stat.S_IMODE(info.st_mode))==pin["mode"],"PAIR_BINARY_CHANGED:"+name)
    except BaseException as error:failures.append({"stage":"post-source","type":type(error).__name__,"message":str(error)})
    if budget.cancel_signal is not None or time.monotonic()>=budget.deadline:
     failures.append({"stage":"owner","type":"CANCELLED_OR_DEADLINE","signal":budget.cancel_signal})
    passed=not failures and before is not None and before==after and set(binary_pins)=={"cherri","cherri-migrate"} and provenance is not None and len(caps)==2
    if passed:
     try:
      cache_receipt={"formatVersion":1,"compilerRepository":"https://github.com/davidpovarsky/cherri.git","compilerCommit":HEAD,"binarySha256":binary_pins["cherri"]["sha256"],"vcsModified":False}
      for name,pin in binary_pins.items():
       for suffix,data in ((".commit",(HEAD+"\n").encode()),(".sha256",(pin["sha256"]+"\n").encode())):
        fixed.owned.original(PAIR/(name+suffix),data);os.chown(PAIR/(name+suffix),os.getuid(),os.getgid());os.chmod(PAIR/(name+suffix),0o644)
      fixed.owned.original(PAIR/"compiler-receipt.json",cache_receipt);os.chown(PAIR/"compiler-receipt.json",os.getuid(),os.getgid());os.chmod(PAIR/"compiler-receipt.json",0o644)
      fixed.owned.original(E/"compiler-cache-receipt.original.json",(PAIR/"compiler-receipt.json").read_bytes())
     except BaseException as error:
      passed=False;failures.append({"stage":"artifact-receipt","type":type(error).__name__,"message":str(error)})
    report={"schema":"cherri-real-compiler-pair-owned-build/1","source":HEAD,"tree":TREE,"sourceDirectory":str(W),"pairDirectory":str(PAIR),
      "status":"PASS_CANDIDATE_COMPILER_PAIR" if passed else "FAIL" if processes else "NOT_RUN","failures":failures,"processes":processes,
      "sourceUnchanged":before is not None and before==after,"ordinaryCheckout":True,"shallow":True,"ancestryQualification":False,
      "binaries":binary_pins,"runtimeProvenance":provenance,"installedIdentities":identities,
      "capabilities":{name:{k:c.get(k) for k in COMMON_FIELDS} for name,c in caps.items()},"cacheReceipt":cache_receipt,
      "minimumFree":owner.minimum,"wholeOwnerSeconds":1380,"elapsedSeconds":time.monotonic()-budget.started,
      "nativeAcceptance":False,"finalTupleAcceptance":False,"finalImplementationPair":False,"liveWrites":False,
      "ownerSHA256":fixed.sha(Path(__file__).read_bytes()),"dependencySHA256":PINS}
    with budget.defer():pin=fixed.owned.original(E/"result.original.json",report)
 print(json.dumps({"result":str(E/"result.original.json"),"sha256":pin["sha256"],"status":report["status"],"binaries":binary_pins,
  "fingerprints":report["capabilities"],"failures":failures,"elapsedSeconds":report["elapsedSeconds"]}),flush=True)
 return 0 if passed else 1
if __name__=="__main__":raise SystemExit(main())
