from pathlib import Path
import ast,collections,datetime,hashlib,json,os,stat,subprocess,time
RUN=Path("/mnt/development/projects/evidence/cherri-p10r15-505-s57b6d919-20261123/raw-attempt-01")
OUT=RUN.parent/"independent-intake-01"
SROOT=Path("/mnt/development/projects/releases/cherri-p10r15-f01-compiler-source-20261123")
GITROOT=Path("/mnt/development/projects/worktrees/cherri-p10r15-k12-integration-20261123")
S="57b6d919757e1d0262993c6f11c6dc1cd17d6177"
STREE="370d85f75d8fb5bba5f9c8afe438208d97a525c8"
K="d67e6c0995ef12112b944b78347f481235506b9d"
PRODUCERS=["TestP10_VerifyAndRecord505CatalogExecution","TestP10_ParameterContractMatrix_AllCatalogActions"]
NAMES=["p10-505-execution-evidence.json","p10-505-parameter-contract-evidence.json"]
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def put(n,v):
 with (OUT/n).open("x",encoding="utf-8") as f:f.write(json.dumps(v,indent=2)+"\n")
def git(repo,*args):
 e={k:os.environ[k] for k in ("PATH","HOME","LANG","LC_ALL") if k in os.environ}
 e.update({"GIT_NO_REPLACE_OBJECTS":"1","GIT_OPTIONAL_LOCKS":"0","GIT_TERMINAL_PROMPT":"0"})
 return subprocess.run(["git","--no-replace-objects","-C",str(repo),*args],stdin=subprocess.DEVNULL,capture_output=True,check=True,timeout=20,env=e).stdout
def tree(ref):
 rows={}
 for raw in git(GITROOT,"ls-tree","-r","-z",ref).split(b"\0"):
  if not raw:continue
  bits,path=raw.split(b"\t",1);mode,kind,blob=bits.decode().split();path=path.decode()
  rows[path]={"path":path,"mode":mode,"kind":kind,"blob":blob}
 return rows
def raw_selected(p):
 return ("/" not in p and p.endswith(".go")) or p in ("go.mod","go.sum","stdfunc.cherri") or p.startswith(("internal/","actions/","testdata/","tools/language-schema/"))
begin=time.monotonic()
assert os.getuid()==1001
assert sha(RUN/"process.original.json")=="c577f775fac7bdf74fb3dabb71fd586c0e357f9af19df4069a0ba635fedb4877"
receipt=json.loads((RUN/"process.original.json").read_bytes())
intent=json.loads((RUN/"intent.original.json").read_bytes())
terminal=json.loads((RUN/"process-terminal.original.json").read_bytes())
goenv=json.loads((RUN/"go-env.original.json").read_bytes())
old_manifest=json.loads((RUN/"originals-manifest.json").read_bytes())
checks={}
checks["originalManifestMembers"]=all(sha(RUN/x["path"])==x["sha256"] and (RUN/x["path"]).stat().st_size==x["bytes"] for x in old_manifest["files"])
checks["submittedSupervisor"]=sha(RUN/"submitted-supervisor.py")=="8ed66b338062c7d31efc5fce6dd468a599be0557309fe29344c204a2add33467"
checks["exactSource"]=receipt["source"]["head"]==S and receipt["source"]["tree"]==STREE and receipt["source"]==receipt["source_after"] and receipt["source"]["status"]=="" and receipt["source"]["replace_refs"]==""
checks["actualTerminal"]=receipt["exit_code"]==terminal["exit_code"]==0 and receipt["timed_out"] is terminal["timed_out"] is False and receipt["elapsed_seconds"]==terminal["elapsed_seconds"]
checks["originalCommand"]=receipt["argv"][1:]==["test","-count=1","-json","-timeout=15m","-parallel=2","-run","^("+"|".join(PRODUCERS)+")$","."]
checks["originalBounds"]=intent["test_timeout_seconds"]==900 and intent["process_wall_limit_seconds"]==1200 and intent["termination_grace_seconds"]==5 and intent["kill_reap_wait_seconds"]==5
checks["actualToolchain"]=goenv["GOVERSION"]=="go1.24.13" and goenv["GOOS"]=="linux" and goenv["GOARCH"]=="arm64"
checks["freshAdmission"]=intent["minimum_free_bytes"]==2147483648 and intent["capacity_free_before"]>=2147483648
checks["streams"]=sha(RUN/"stdout.original.jsonl")==receipt["stdout_sha256"]==terminal["stdout_sha256"] and sha(RUN/"stderr.original")==receipt["stderr_sha256"]==terminal["stderr_sha256"]
events=[];parse_errors=0
for line in (RUN/"stdout.original.jsonl").read_bytes().splitlines():
 if not line.strip():continue
 try:
  row=json.loads(line)
  if type(row) is not dict:raise ValueError()
  events.append(row)
 except (ValueError,UnicodeError):parse_errors+=1
starts=collections.Counter((x.get("Package"),x["Test"]) for x in events if x.get("Action")=="run" and x.get("Test"))
ends=collections.Counter((x.get("Package"),x["Test"]) for x in events if x.get("Action") in ("pass","fail","skip") and x.get("Test"))
case_ends=[x for x in events if x.get("Action") in ("pass","fail","skip") and x.get("Test")]
packages=[x for x in events if x.get("Action") in ("pass","fail","skip") and not x.get("Test")]
checks["strictEvents"]=not parse_errors and len(starts)==2 and {x[1] for x in starts}==set(PRODUCERS) and set(starts)==set(ends) and all(v==1 for v in starts.values()) and all(v==1 for v in ends.values()) and len(case_ends)==2 and all(x["Action"]=="pass" for x in case_ends) and len(packages)==1 and packages[0]["Action"]=="pass"
a=json.loads((RUN/"artifacts"/NAMES[0]).read_bytes());b=json.loads((RUN/"artifacts"/NAMES[1]).read_bytes())
ae=a["entries"];be=b["entries"]
an=[x["name"] for x in ae];bn=[x["actionName"] for x in be]
checks["twoRawFiles"]=set(p.name for p in (RUN/"artifacts").iterdir())==set(NAMES) and all(stat.S_ISREG((RUN/"artifacts"/n).lstat().st_mode) for n in NAMES)
checks["two505Catalogs"]=len(ae)==len(be)==a["totalCatalogEntries"]==b["totalCatalog"]==505 and len(set(an))==len(set(bn))==505 and set(an)==set(bn)
checks["constructIdentities"]=all(type(x["compilerConstruct"]) is bool for x in ae+be) and {x["name"]:x["compilerConstruct"] for x in ae}=={x["actionName"]:x["compilerConstruct"] for x in be}
constructs=sum(x["compilerConstruct"] for x in ae)
checks["constructCounts"]=constructs==a["constructCount"]==b["constructCount"]==8 and a["callableCount"]==b["callableCount"]==505-constructs
mapping={"legacyPassCount":"legacySupported","v2PassCount":"v2Supported","plistIdentifierPassCount":"plistIdentifierVerified","parameterContractPassCount":"parameterContractVerified","decompilePassCount":"decompileSupported","roundTripPassCount":"roundTripVerified","structuralRoundTripCount":"structuralRoundTripPass"}
counts={k:sum(x[v] is True for x in ae) for k,v in mapping.items()}
checks["localExecutionCounts"]=all(all(type(x[v]) is bool for x in ae) and counts[k]==a[k]==505 for k,v in mapping.items())
native=dict(collections.Counter(x["nativeSpecimenParityStatus"] for x in ae));runtime=dict(collections.Counter(x["appleRuntimeExecutedStatus"] for x in ae))
checks["nativeRuntimeScope"]=native=={"NEEDS_EVIDENCE":505} and runtime=={"BLOCKED_DEVICE":505} and a["nativeSpecimenParityCount"]==a["appleRuntimeExecutedCount"]==0
checks["allParameterContracts"]=b["schemaVersion"]=="2.0.0" and b["allContractsPassed"] is True and b["passedCount"]==sum(x["contractPassed"] is True for x in be)==505 and b["failedCount"]==sum(x["contractPassed"] is False for x in be)==0 and all(type(x["contractPassed"]) is bool for x in be)
dimensions=["requiredParams","missingRequiredRejected","optionalParams","enumValidation","dynamicReference","platformGate"]
flags=["requiredParamsVerified","missingRequiredRejectedBool","optionalParamsVerified","enumValidationVerified","dynamicReferenceVerified","platformGateVerified"]
reconstructed=[]
for name,flag in zip(dimensions,flags):
 statuses=collections.Counter(x[name]["status"] for x in be)
 subkinds=collections.Counter(x[name]["subKind"] for x in be if x[name].get("subKind"))
 summary={"dimension":name,"totalCatalog":505,"constructsExcluded":constructs,
          "eligible":sum(statuses.get(k,0) for k in ("PASS","FAIL","NOT_TESTED","BLOCKED_MISSING_EVIDENCE")),
          "tested":statuses.get("PASS",0)+statuses.get("FAIL",0),"pass":statuses.get("PASS",0),"fail":statuses.get("FAIL",0),
          "notApplicable":statuses.get("NOT_APPLICABLE",0),"notTested":statuses.get("NOT_TESTED",0),
          "blocked":statuses.get("BLOCKED_MISSING_EVIDENCE",0),"subKindBreakdown":dict(subkinds)}
 reconstructed.append(summary)
 checks["dimension_"+name]=set(statuses)<= {"PASS","NOT_APPLICABLE"} and sum(statuses.values())==505 and all(type(x[flag]) is bool and x[flag]==(x[name]["status"]=="PASS") for x in be) and all(not x["compilerConstruct"] or (x[name]["status"]=="NOT_APPLICABLE" and x[name]["subKind"]=="COMPILER_CONSTRUCT") for x in be)
checks["sixDimensionsReconstructed"]=reconstructed==a["parameterDimensionSummaries"]==b["dimensionSummaries"] and len(reconstructed)==6
artifacts=[]
for n in NAMES:
 p=RUN/"artifacts"/n;historical=SROOT/"docs/apple-runtime/p10"/n
 artifacts.append({"path":n,"bytes":p.stat().st_size,"sha256":sha(p),"historicalTrackedSha256":sha(historical),"byteIdenticalToHistorical":p.read_bytes()==historical.read_bytes(),"newActualExecutionSource":S})
checks["originalArtifactBindings"]=all(any(x["path"]==v["path"] and x["bytes"]==v["bytes"] and x["sha256"]==v["sha256"] for v in receipt["artifact_files"]) for x in artifacts)
sg=tree(S);kg=tree(K);rawpaths=sorted(p for p in sg if raw_selected(p));krawpaths=sorted(p for p in kg if raw_selected(p))
checks["S_to_d67_rawGitEquivalence"]=rawpaths==krawpaths and len(rawpaths)==595 and all(sg[p]==kg[p] for p in rawpaths)
before=json.loads((RUN/"raw-input-witness-before.original.json").read_bytes());after=json.loads((RUN/"raw-input-witness-after.original.json").read_bytes())
fresh=[]
for path in rawpaths:
 p=SROOT/path;raw=p.read_bytes();info=p.lstat()
 row={**sg[path],"bytes":len(raw),"sha256":hashlib.sha256(raw).hexdigest()}
 assert stat.S_ISREG(info.st_mode) and sg[path]["kind"]=="blob" and sg[path]["mode"]==("100755" if info.st_mode&0o111 else "100644")
 assert hashlib.sha1(b"blob "+str(len(raw)).encode()+b"\0"+raw).hexdigest()==sg[path]["blob"]
 fresh.append(row)
checks["actualBroadInputsBeforeAfter"]=before==after and before["files"]==fresh and before["count"]==595
generator=SROOT/"tools/p10r3/generate_catalog_and_pack_ledgers.py"
parsed=ast.parse(generator.read_bytes())
patterns=next(ast.literal_eval(x.value) for x in parsed.body if isinstance(x,ast.Assign) and any(isinstance(t,ast.Name) and t.id=="DEPENDENCY_GLOBS" for t in x.targets))
depnames=sorted({p.relative_to(SROOT).as_posix() for pattern in patterns for p in SROOT.glob(pattern) if p.is_file()})
dep=[];h=hashlib.sha256()
for name in depnames:
 raw=(SROOT/name).read_bytes();norm=hashlib.sha256(raw.replace(b"\r\n",b"\n")).hexdigest()
 dep.append({"path":name,"bytes":len(raw),"sha256":hashlib.sha256(raw).hexdigest(),"normalized_sha256":norm})
 h.update((name+":"+norm+"\n").encode())
fingerprint=h.hexdigest()
dw=json.loads((RUN/"input-witness-before.original.json").read_bytes())
checks["actual597DependencyFingerprint"]=len(dep)==597 and dw["files"]==dep and dw==json.loads((RUN/"input-witness-after.original.json").read_bytes()) and fingerprint==receipt["input_fingerprint_before"]==receipt["input_fingerprint_after"]=="995e46487692e4d9cbccc59a5ddb02b41455bd1c57b4383521800cb87434b98e"
checks["S_to_d67_dependencyGitEquivalence"]=all(sg[p]==kg[p] for p in depnames)
checks["S_ancestorOf_d67"]=subprocess.run(["git","--no-replace-objects","-C",str(GITROOT),"merge-base","--is-ancestor",S,K],stdin=subprocess.DEVNULL,capture_output=True,timeout=20).returncode==0
checks["sourceStillClean"]=git(SROOT,"rev-parse","HEAD").decode().strip()==S and git(SROOT,"rev-parse","HEAD^{tree}").decode().strip()==STREE and not git(SROOT,"status","--porcelain=v1","--untracked-files=all").strip()
equivalence={"source":S,"sourceTree":STREE,"consumerObservation":K,"consumerTree":git(GITROOT,"rev-parse",K+"^{tree}").decode().strip(),"finalConsumerSource":None,"broadRawInputs":595,"rawGitEntriesEqual":checks["S_to_d67_rawGitEquivalence"],"currentActualFileWitnessSha256":hashlib.sha256((json.dumps(fresh,indent=2)+"\n").encode()).hexdigest(),"dependencyCount":597,"dependencyFingerprint":fingerprint,"dependencyEntriesEqual":checks["S_to_d67_dependencyGitEquivalence"],"ancestor":checks["S_ancestorOf_d67"],"compatibilityManifestOutsideBuiltInFingerprint":True,"finalSourceRecheckRequired":True}
put("input-equivalence.original.json",equivalence)
put("actual-raw-input-witness.original.json",fresh)
result={"schema":"cherri-raw505-independent-intake/2","auditedUTC":datetime.datetime.now(datetime.timezone.utc).isoformat(),"source":S,"tree":STREE,"sourceProcessReceiptSHA256":sha(RUN/"process.original.json"),"sourceSupervisorSHA256":sha(RUN/"submitted-supervisor.py"),"auditorSHA256":sha(Path(__file__)),"actualProcessSeconds":receipt["elapsed_seconds"],"actualExitCode":receipt["exit_code"],"timedOut":receipt["timed_out"],"eventCount":len(events),"parseErrorCount":parse_errors,"parentCount":len(starts),"parents":[{"test":x["Test"],"action":x["Action"],"elapsed":x.get("Elapsed")} for x in case_ends],"packageCount":len(packages),"packageElapsed":packages[0].get("Elapsed") if packages else None,"localCounts":counts,"constructCount":constructs,"callableCount":505-constructs,"parameterPassed":b["passedCount"],"parameterFailed":b["failedCount"],"parameterDimensions":reconstructed,"nativeStatusCounts":native,"runtimeStatusCounts":runtime,"nativeRuntimeAcceptanceGranted":False,"artifacts":artifacts,"checks":checks,"intakeAccepted":all(checks.values()),"sourceEquivalence":equivalence,"historicalOutputBytesReusedAsMeasurement":False,"newMeasuredBytesEqualHistorical":all(x["byteIdenticalToHistorical"] for x in artifacts),"newSourceMeasurementAtS":True,"dependencyFingerprintActuallyChanged":False,"producerCommandsExecutedByIntake":0,"privateInputReads":0,"intakeDurationSeconds":round(time.monotonic()-begin,6)}
put("result.original.json",result)
print(json.dumps({"intakeAccepted":result["intakeAccepted"],"resultSHA256":sha(OUT/"result.original.json"),"eventCount":len(events),"parentCount":len(starts),"counts":counts,"parameterPassFail":[b["passedCount"],b["failedCount"]],"dimensionPassCounts":[x["pass"] for x in reconstructed],"raw595Equal":checks["S_to_d67_rawGitEquivalence"],"dependency597Equal":checks["S_to_d67_dependencyGitEquivalence"],"fingerprint":fingerprint,"newSourceMeasurementAtS":True,"equalHistoricalBytes":result["newMeasuredBytesEqualHistorical"],"failedChecks":[k for k,v in checks.items() if not v]},sort_keys=True))
raise SystemExit(0 if result["intakeAccepted"] else 1)
