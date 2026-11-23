#!/usr/bin/env python3
"""Read-only, local-only source/prerequisite original verification.
No collector, command stream, private path, nonce, archive or key is emitted.
Caller supplies source-bound public expected context and keeps plaintext local.
"""
import hashlib, io, json, math, pathlib, stat, sys, zipfile
ALLOWED = {'45086924979': ('ca316a3dee252456c4bec751594cf249c709b331', 'qa-json-body-file-utf8-v2'), '45100894957': ('4feadab9db4d351628f78593304373631693869a', 'qa-functional-legacy'), '45100952614': ('4feadab9db4d351628f78593304373631693869a', 'qa-bool-list-v2'), '45101024573': ('4feadab9db4d351628f78593304373631693869a', 'CherriTypedDefaultsPOC'), '45101081393': ('4feadab9db4d351628f78593304373631693869a', 'qa-short-circuit-v2'), '45108412777': ('9b4135e705b08f765b7304e01212d3b9658806fb', 'qa-bool-list-v2')}
CONTEXT = ("diagnosticSourceSha","workflowRunId","workflowRunAttempt","prerequisiteOnly",
           "formalAcceptanceGranted","executionScope","qaContextSha256","qaContextMatchedBefore",
           "qaContextMatchedAfter")
SCRIPTS = (
 ("install",'if ! command -v xcodegen >/dev/null 2>&1; then HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_INSTALL_CLEANUP=1 brew install xcodegen; fi'),
 ("generate","xcodegen generate"),
)
BUDGETS={"install":180,"generate":60,"aggregate":240,"preflight":30,"encryption":20,"outer":310}
class Rejected(Exception):
 def __init__(self,number):self.number=number
def require(value,number):
 if not value:raise Rejected(number)
def sha(raw):return hashlib.sha256(raw).hexdigest()
def duplicate_free(pairs):
 result={}
 for key,value in pairs:
  require(key not in result,1);result[key]=value
 return result
def parse(raw):
 return json.loads(raw,object_pairs_hook=duplicate_free,parse_constant=lambda _:(_ for _ in ()).throw(Rejected(2)))
def canonical(v):return (json.dumps(v,sort_keys=True,separators=(",",":"),allow_nan=False)+"\n").encode()
def bound(raw,record,number):
 require(record["bytes"]==len(raw) and record["sha256"]==sha(raw),number)
def finite(v):return type(v) in (int,float) and math.isfinite(v)
def verify(plain_path,summary_path,protocol_path,context_path,cipher_path):
 encoded=pathlib.Path(plain_path).read_bytes()
 summary_raw=pathlib.Path(summary_path).read_bytes();summary=parse(summary_raw)
 protocol_raw=pathlib.Path(protocol_path).read_bytes();expected=parse(protocol_raw)
 expected_context=parse(pathlib.Path(context_path).read_bytes())
 run=summary["workflowRunId"];source=summary["diagnosticSourceSha"]
 require(run in ALLOWED and ALLOWED[run]==(source,expected_context["targetFixtureId"]),3)
 require(expected["sourceSha"]==source and summary["workflowRunAttempt"]=="1",4)
 require(expected_context["compilerCommitSha"]==source and expected_context["runId"]==run,5)
 require(summary["prerequisiteOnly"] is True and summary["formalAcceptanceGranted"] is False,6)
 require(all(summary[k] is True for k in ("sourceMatchedBefore","sourceMatchedAfter","environmentMatched","captureComplete","qaContextMatchedBefore","qaContextMatchedAfter")),7)
 require(summary["prerequisiteStatus"]=="PREPARED" and summary["reason"]=="NONE",8)
 require(summary["budgetsSeconds"]==BUDGETS and type(summary["elapsedMs"]) is int and 0<=summary["elapsedMs"]<310000,9)
 require(summary["recipientSha256"]==expected["recipientSha256"]=="55956e825597a2e60acca4045e4ea8040ae27f17caa7964e9b4493824fa0c1f1",10)
 require(summary["collectorActionsStarted"]=={"compiler":False,"simulator":False,"nativeTest":False,"modelRequest":False} and summary["priorWorkflowPhasesAssessed"] is False,11)
 require(len(encoded)<=9*1024*1024,12);bound(encoded,summary["plaintextArchive"],13)
 cipher=pathlib.Path(cipher_path).read_bytes()
 require(cipher.startswith(b"\x30") and len(cipher)<=9*1024*1024+65536,14);bound(cipher,summary["ciphertext"],15)
 with zipfile.ZipFile(io.BytesIO(encoded)) as z:
  entries=z.infolist();names=z.namelist()
  require(len(names)==len(set(names)) and 4<=len(names)<=24,16)
  require(all(not pathlib.PurePosixPath(n).is_absolute() and ".." not in pathlib.PurePosixPath(n).parts and "\\" not in n for n in names),17)
  require(all(not e.is_dir() and e.file_size<=2*1024*1024+1 and not stat.S_ISLNK(e.external_attr>>16) and not e.flag_bits&1 for e in entries),18)
  original={n:z.read(n) for n in names}
 manifest=parse(original["manifest.json"])
 require(manifest["schema"]==summary["schema"]+"/originals" and all(manifest[k]==summary[k] for k in CONTEXT),19)
 require(manifest["members"]==summary["members"],20)
 members=manifest["members"]
 require(names==[r["name"] for r in members]+["manifest.json"] and len(members)==23,21)
 for member in members:
  require(member["complete"] is True,22);bound(original[member["name"]],member,23)
 source_record=parse(original["source.json"]);before=source_record["before"];after=source_record["after"]
 require(source_record["sourceMatchedBefore"] is True and source_record["sourceMatchedAfter"] is True,24)
 require(before["sourceSha"]==after["sourceSha"]==source and before["cleanRequiredBeforeOnly"] is True and after["cleanRequiredBeforeOnly"] is False,25)
 require(before["files"]==after["files"] and len(before["files"])==len(expected["files"])==21,26)
 require([{k:v for k,v in r.items() if k!="identity"} for r in before["files"]]==expected["files"],27)
 require([r["path"] for r in before["files"]]==expected["sourcePaths"],28)
 journal=parse(original["source-preflight.json"])
 require(journal["schema"]==summary["schema"]+"/source-preflight" and journal["sourceSha"]==source and journal["sourcePaths"]==expected["sourcePaths"],29)
 require(journal["preflightSeconds"]==30 and journal["checkpoint"]=="MATCHED" and journal["fileIndex"] is None and journal["rejectionCode"] is None and journal["sourceMatched"] is True,30)
 rows=journal["commands"]
 require(len(rows)==3 and [r["phase"] for r in rows]==expected["expectedSourcePhases"]==["head","status","tree"],31)
 require(before["commands"]==[r["commandReceipt"] for r in rows],32)
 commands=[["git","--no-replace-objects","rev-parse","HEAD"],["git","--no-replace-objects","status","--porcelain=v1"],["git","--no-replace-objects","ls-tree","-rz",source,"--",*expected["sourcePaths"]]]
 for index,row in enumerate(rows):
  phase=row["phase"];command=row["commandReceipt"]
  require(row["argv"]==command["argv"]==commands[index] and row["maximumSeconds"]==(3,3,5)[index] and row["stdin"]=="DEVNULL" and row["captureReturned"] is True,33)
  require(original["source-"+phase+".process.json"]==canonical(row),34)
  require(finite(row["hardDeadlineMonotonic"]) and finite(row["allowanceSampleMonotonic"]) and row["hardDeadlineMonotonic"]==journal["hardDeadlineMonotonic"] and row["allowanceSeconds"]==max(0,min(row["maximumSeconds"],row["hardDeadlineMonotonic"]-row["allowanceSampleMonotonic"])),35)
  require(command["result"]=="PASS" and command["exitCode"]==0 and 0<=command["elapsedMs"]<=math.ceil(row["allowanceSeconds"]*1000),36)
  cap=command["capture"]
  require(cap["cleanupParentReaped"] is True and cap["cleanupGroupSignal"]["state"] in {"SENT","ABSENT"} and cap["groupPresentAfterCleanup"] is False,37)
  for stream in ("stdout","stderr"):
   raw=original["source-"+phase+"."+stream]
   bound(raw,row["streams"][stream],38);bound(raw,command[stream],39);bound(raw,cap[stream],40)
   require(row["streams"][stream]["complete"] is True and cap[stream]["eof"] is True and cap[stream]["overflow"] is False,41)
  require(original["source-"+phase+".stderr"]==b"",42)
 require(original["source-head.stdout"].decode("ascii").strip()==source and original["source-status.stdout"]==b"",43)
 tree=original["source-tree.stdout"];require(len(tree)==expected["sourceTreeBytes"] and sha(tree)==expected["sourceTreeSha256"],44)
 tree_rows={}
 for line in tree.split(b"\0"):
  if not line:continue
  meta,name=line.split(b"\t",1);mode,kind,blob=meta.decode("ascii").split();name=name.decode("utf-8")
  require(mode in {"100644","100755"} and kind=="blob" and name not in tree_rows,45);tree_rows[name]=blob
 require(tree_rows=={r["path"]:r["gitBlob"] for r in expected["files"]},46)
 identity=parse(original["identity.json"])
 require(identity["matched"] is True and identity["expected"]==identity["actual"]==expected["expectedEnvironment"],47)
 require(identity["ci"]=={"GITHUB_SHA":source,"GITHUB_RUN_ID":run,"GITHUB_RUN_ATTEMPT":"1","GITHUB_REPOSITORY":"davidpovarsky/cherri","GITHUB_EVENT_NAME":"workflow_dispatch"},48)
 qa=source_record["qaContext"]
 require(qa["matchedBefore"] is True and qa["matchedAfter"] is True and qa["before"]==qa["after"] and qa["before"]["context"]==expected_context,49)
 require(sha(canonical(expected_context))==summary["qaContextSha256"],50)
 manifest_raw=original["qa-compiler-manifest.original.json"]
 require(sha(manifest_raw)==expected_context["compilerManifestSha256"],51)
 bound(manifest_raw,qa["before"]["compilerManifest"],52)
 compiler_manifest=parse(manifest_raw)
 require(compiler_manifest["compilerCommitSha"]==source and compiler_manifest["syntheticOnly"] is True,53)
 fixture=[f for f in compiler_manifest["fixtures"] if f["fixtureId"]==expected_context["targetFixtureId"]]
 require(len(fixture)==1,54);fixture=fixture[0]
 require(fixture["sourceSha256"]==expected_context["compiledSourceSha256"] and fixture["unsignedPlistSha256"]==expected_context["unsignedPlistSha256"] and fixture["buildProfile"]==expected_context["buildProfile"] and fixture["runtimeTraceExpectation"]==expected_context["traceExpectation"],55)
 diagnostic=parse(original["diagnostic.json"])
 require(all(diagnostic[k]==summary[k] for k in CONTEXT) and diagnostic["prerequisiteReturnCode"]==0 and diagnostic["reason"]=="NONE" and diagnostic["captureComplete"] is True and diagnostic["prerequisiteStatus"]=="PREPARED" and diagnostic["budgets"]==BUDGETS,56)
 receipt=parse(original["builds/project-prerequisite/receipt.json"])
 require(all(receipt[k]==v for k,v in expected_context.items()) and receipt["formalAcceptanceGranted"] is False and receipt["status"]=="PREPARED" and receipt["budgetMs"]==240000 and 0<=receipt["elapsedMs"]<240000,57)
 require(receipt["projectSha256"]==diagnostic["generatedProject"]["sha256"] and len(receipt["commands"])==2 and len(summary["stages"])==2,58)
 for index,(stage,script) in enumerate(SCRIPTS):
  intent=parse(original[stage+".intent.json"]);process=parse(original[stage+".process.json"]);brief=summary["stages"][index];record=receipt["commands"][index]
  require(intent["stage"]==process["stage"]==brief["stage"]==record["stage"]==stage and all(process[k]==v for k,v in intent.items()),59)
  require(intent["argv"]==[identity["bash"]["supplied"],"-c",script] and intent["stdin"]=="DEVNULL",60)
  require(process["result"]==brief["status"]==record["result"]=="PASS" and process["exitCode"]==brief["exitCode"]==record["exitCode"]==0,61)
  require(process["elapsedMs"]==brief["elapsedMs"]==record["elapsedMs"] and brief["allowanceMs"]==record["allowanceMs"]==round(intent["allowanceSeconds"]*1000) and 0<=intent["allowanceSeconds"]<=BUDGETS[stage],62)
  for stream in ("stdout","stderr"):
   raw=original["builds/project-prerequisite/"+stage+"."+stream]
   bound(raw,brief["streams"][stream],63);bound(raw,record[stream],64);bound(raw,process["capture"][stream],65)
   require(brief["streams"][stream]["complete"] is True and process["capture"][stream]["eof"] is True and process["capture"][stream]["overflow"] is False,66)
 return {"localVerificationPassed":True,"plaintextOrKeyTransferred":False,
  "archiveSha256":sha(encoded),"ciphertextSha256":sha(cipher),"expectedProtocolSha256":sha(protocol_raw),
  "expectedContextSha256":sha(canonical(expected_context)),"originalMembersVerified":len(members),
  "allMembersComplete":True,"manifestAndContextMatched":True,"sourceJournalVerified":True,
  "sourceCommandCount":len(rows),"sourceCommandPrefixMatched":True,"sourceFileCount":len(before["files"]),
  "sourceFilesMatchExpectedCommit":True,"sourceTreeSha256":sha(tree),"sourceCleanBefore":True,
  "sourceMatchedAfter":True,"environmentMatched":True,"prerequisitePrepared":True,
  "prerequisiteOnly":True,"nativeRuntimeAcceptanceClaimed":False,"formalAcceptanceGranted":False}
if __name__=="__main__":
 try:print(json.dumps(verify(*sys.argv[1:]),sort_keys=True))
 except Rejected as exc:
  print(json.dumps({"localVerificationPassed":False,"failedCheckNumber":exc.number,"plaintextOrKeyTransferred":False}));sys.exit(1)
 except Exception:
  print(json.dumps({"localVerificationPassed":False,"failedCheckNumber":0,"plaintextOrKeyTransferred":False}));sys.exit(1)
