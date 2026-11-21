from pathlib import Path
import subprocess,json,hashlib,collections
E=Path(r"C:\Users\DAVID\Code\cherri-p10r15-continuation-evidence-20261119\r5-openminis-independent")
B=Path(r"C:\Users\DAVID\Code\cherri-p10r15-parity-evidence-20261119\r4-consumer-refresh-20261120\openminis-44731965480-original")
R=Path(r"C:\Users\DAVID\Code\cherri-p10r15-completion-r4-20261120")
def sha(b):return hashlib.sha256(b).hexdigest()
byte=json.loads((E/"byte-intake.json").read_bytes())
assert byte["component"]=="bf621464b46f51c655381b2b6c65312963cab053" and byte["originalsVerified"]==68 and byte["externalArchivesVerified"]==8 and byte["gitWitnessesVerified"]==25
(E/"semantic-launcher.py").write_bytes(Path(__file__).read_bytes())
signpath=B/"artifact-members/14626738722/signing-protocol-exit-evidence.json"
macroPath=B/"artifact-members/14627981157/macro-compatibility-isolation.json"
sign=json.loads(signpath.read_bytes());macro=json.loads(macroPath.read_bytes())
assert sign["summary"]=={"checks":34,"controlsPassed":3,"negativesRejected":31,"failures":0}
rows=sign["results"];assert len(rows)==34 and len({(x["surface"],x["case"]) for x in rows})==34
counts=dict(collections.Counter(x["verdict"] for x in rows))
assert counts=={"CONTROL_PASS":3,"REJECTED_NONZERO_NO_ARTIFACT":31},counts
for row in rows:
 assert row["exactPreflightBytesObserved"] is None
 if row["verdict"]=="CONTROL_PASS":assert row["exitCode"]==0 and len(row["artifacts"])==1
 else:assert isinstance(row["exitCode"],int) and row["exitCode"]!=0 and row["artifacts"]==[]
assert sign["appleCryptographicValidation"]=="NOT_TESTED" and sign["privateCorpusUsed"] is False
assert macro["summary"]=={"checks":23,"passed":23,"failed":0} and len(macro["results"])==23
assert len({x["case"] for x in macro["results"]})==23 and all(x["verdict"]=="PASS" for x in macro["results"])
assert macro["livePromotion"] is False and macro["compilerCommit"]=="2585983e9d0cb1a9097ec4f183dbf9b57036f75f"
assert macro["macroCount"]==15
run=json.loads((B/"run.json").read_bytes());jobs=json.loads((B/"jobs.json").read_bytes())
assert run["id"]==44731965480 and run["run_attempt"]==1 and run["head_sha"]==macro["compilerCommit"] and run["status"]=="completed" and run["conclusion"]=="success"
assert jobs["total_count"]==1 and len(jobs["jobs"])==1
job=jobs["jobs"][0]
assert job["id"]==136273762871 and job["run_id"]==run["id"] and job["head_sha"]==run["head_sha"] and job["conclusion"]=="success"
assert all(x["status"]=="completed" and x["conclusion"]=="success" for x in job["steps"])
binaryRows=[]
for name,expected,rev,goarch,role in [
("current-linux-amd64","d4e3f75470479c35771f59429aa12537ddc0d53615f76b5d4da5b568a431c437","2585983e9d0cb1a9097ec4f183dbf9b57036f75f","amd64","github.com/electrikmilk/cherri"),
("current-linux-arm64","9f4ef21f3a8f2b06bc6d444c4e80f504862f0fc87e4b1b3154014bbdfac0e0ca","2585983e9d0cb1a9097ec4f183dbf9b57036f75f","arm64","github.com/electrikmilk/cherri"),
("pinned-linux-arm64","725b781d17f4478da1749266299013a26b283b530fd8f07f8abb31b5fb374e8b","fd691b96fb661697a1cbaddab92476d457636ba9","arm64","github.com/electrikmilk/cherri"),
("pinned-migration-linux-arm64","2455aa26144bc23b245a5824943f81fa753922e844b195db7773a7037f81f76a","fd691b96fb661697a1cbaddab92476d457636ba9","arm64","github.com/electrikmilk/cherri/cmd/cherri-migrate")]:
 p=B/"intake-v1"/name
 b=p.read_bytes();assert sha(b)==expected,name
 q=subprocess.run(["go","version","-m",str(p)],cwd=R,capture_output=True,timeout=30)
 (E/(name+".go-version.stdout")).write_bytes(q.stdout);(E/(name+".go-version.stderr")).write_bytes(q.stderr)
 assert q.returncode==0,q.stderr.decode(errors="replace")
 info=q.stdout.decode();assert "\tpath\t"+role+"\n" in info
 for field,value in [("vcs.revision",rev),("vcs.modified","false"),("GOOS","linux"),("GOARCH",goarch)]:
  assert "\tbuild\t"+field+"="+value+"\n" in info,(name,field,info)
 binaryRows.append({"name":name,"bytes":len(b),"sha256":expected,"revision":rev,"GOOS":"linux","GOARCH":goarch,"role":role,"modified":False,"metadataSHA256":sha(q.stdout),"executedByThisIntake":False})
log=(B/"job-log-original.txt").read_bytes()
assert len(log)==151494 and sha(log)=="93997219d799d290e1b5bd8e845a6c3b135e6ee72e7dfcb1345693147cdb186e"
result={"schemaVersion":1,"proofCommit":byte["component"],"source":run["head_sha"],"runID":run["id"],"runAttempt":1,"jobID":job["id"],"runConclusion":"success","allPublishedStepsSucceeded":True,"originalByteIntake":byte,"signing":{"checks":34,"controls":3,"negatives":31,"artifactSHA256":sha(signpath.read_bytes()),"exactPreflightBytesObserved":None,"appleCryptographicValidation":"NOT_TESTED"},"macroCompatibility":{"checks":23,"passed":23,"artifactSHA256":sha(macroPath.read_bytes()),"scope":macro["scope"],"livePromotion":False},"verifiedBinaries":binaryRows,"rawJobLogSHA256":sha(log),"historicalTupleOnly":True,"currentM02ConsumerReleaseVerified":False,"formalAcceptanceGranted":False,"packageAcceptanceChildRawPublished":False}
b=(json.dumps(result,indent=2)+"\n").encode();(E/"semantic-intake.json").write_bytes(b)
print(json.dumps({"semanticVerified":True,"receiptSHA256":sha(b),"run":run["id"],"signing":result["signing"],"macroChecks":23,"realBinariesVerified":len(binaryRows),"currentM02ConsumerReleaseVerified":False}))
