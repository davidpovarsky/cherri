from pathlib import Path
import subprocess,json,hashlib,zipfile,io,collections
R=Path(r"C:\Users\DAVID\Code\cherri-p10r15-completion-r4-20261120")
E=Path(r"C:\Users\DAVID\Code\cherri-p10r15-continuation-evidence-20261119\r5-s01-independent")
E.mkdir(exist_ok=True);(E/"launcher-retry1.py").write_bytes(Path(__file__).read_bytes())
proof="a53c0d70a4556921a6ca19a969c8323237d8d3f5";prefix="docs/language-v2/evidence/p15-s01-s02-main-oracles-20261120/"
def g(*a):return subprocess.check_output(["git","--no-replace-objects",*a],cwd=R)
def sha(b):return hashlib.sha256(b).hexdigest()
def blob(n):return g("show",proof+":"+prefix+n)
sr=blob("summary.json");mr=blob("artifact-manifest.json");wr=blob("source-witnesses.json")
s=json.loads(sr);m=json.loads(mr);w=json.loads(wr)
assert sha(sr)=="af2879e6adba42c69a1688b0231ab5a2ba9d9f6758199ffbf62ad0c26acfa3b6"
assert sha(mr)==s["artifactManifestSHA256"] and sha(wr)==s["sourceWitnessesSHA256"]
assert s["formalAcceptanceGranted"] is False and s["nativeRuntimeExecuted"] is False and not s["productionChanged"] and not s["requirementsClosed"]
zraw=blob(m["archive"]["path"]);assert len(zraw)==m["archive"]["bytes"] and sha(zraw)==m["archive"]["sha256"]
z=zipfile.ZipFile(io.BytesIO(zraw));assert len(z.namelist())==len(set(z.namelist()))==len(m["files"])==61
assert set(z.namelist())=={x["path"] for x in m["files"]}
for x in m["files"]:
 b=z.read(x["path"]);assert b==Path(x["originalPath"]).read_bytes() and len(b)==x["bytes"] and sha(b)==x["sha256"],x["path"]
specs=[x["sourceCommit"]+":"+x["path"] for x in w["files"]]
raw=g("cat-file","--batch") if False else subprocess.check_output(["git","--no-replace-objects","cat-file","--batch"],cwd=R,input=("\n".join(specs)+"\n").encode())
assert len(raw)<=w["maxAggregateBytes"] and len(raw)==w["actualRawBatchBytes"]
pos=0;objects={}
for x in w["files"]:
 stop=raw.index(b"\n",pos);header=raw[pos:stop].decode().split();assert len(header)==3 and header[1]=="blob" and header[0]==x["gitBlob"],x["path"]
 size=int(header[2]);assert size<=w["maxObjectBytes"] and size==x["bytes"]
 b=raw[stop+1:stop+1+size];assert sha(b)==x["sha256"] and raw[stop+1+size:stop+2+size]==b"\n",x["path"]
 objects[(x["sourceCommit"],x["path"])]=b;pos=stop+2+size
assert pos==len(raw) and len(w["files"])==742
changed=g("diff","--name-only",s["baseCommit"],s["sourceCommit"]).decode().splitlines()
assert set(changed)==set(s["changedFiles"]) and len(changed)==5 and all(p.endswith("_test.go") for p in changed)
unchanged=[]
for x in w["files"]:
 if x["sourceCommit"]==s["baseCommit"] and x["path"] not in changed:
  assert g("show",s["sourceCommit"]+":"+x["path"])==objects[(x["sourceCommit"],x["path"])]
  unchanged.append(x["path"])
assert len(unchanged)==5
results=[]
for stage in s["runs"]:
 pre=stage["path"]+"/";stdout=z.read(pre+"stdout.jsonl");stderrName=[n for n in z.namelist() if n.startswith(pre) and Path(n).name.startswith("stderr")];assert len(stderrName)==1,(pre,stderrName);stderr=z.read(stderrName[0]);pr=z.read(pre+"process-result.json");p=json.loads(pr)
 assert sha(stdout)==stage["stdoutSHA256"]==p["stdoutSHA256"] and sha(stderr)==stage["stderrSHA256"]==p["stderrSHA256"] and sha(pr)==stage["processSHA256"]
 events=[json.loads(l) for l in stdout.splitlines() if l.strip()]
 counts={kind:sum(v.get("Action")==kind and bool(v.get("Test")) for v in events) for kind in ["run","pass","fail","skip"]}
 assert counts==stage["counts"]==p["counts"]
 runs=[(v.get("Package"),v["Test"]) for v in events if v.get("Action")=="run" and v.get("Test")]
 terms=[(v.get("Package"),v["Test"]) for v in events if v.get("Action") in ["pass","fail","skip"] and v.get("Test")]
 assert len(set(runs))==len(runs) and collections.Counter(runs)==collections.Counter(terms)
 parents={name for pkg,name in runs if "/" not in name}
 assert parents==set(p["expectedParentNames"]) and len(parents)==stage["selectedParentCount"]
 assert p["source"]==stage["source"] and p["exitCode"]==stage["exitCode"] and not p["timedOut"]
 assert p["sourceCleanBefore"]==stage["clean"] and p["sourceCleanAfter"]==stage["clean"]
 results.append({"path":stage["path"],"source":p["source"],"exitCode":p["exitCode"],"clean":stage["clean"],"counts":counts,"parents":len(parents),"stdoutSHA256":sha(stdout)})
assert results[-1]["source"]==s["sourceCommit"] and results[-1]["counts"]=={"run":601,"pass":601,"fail":0,"skip":0} and results[-1]["clean"]
result={"schemaVersion":1,"proofCommit":proof,"sourceCommit":s["sourceCommit"],"baseCommit":s["baseCommit"],"originalMembersVerified":61,"originalClassification":{"processAndSourceFiles":60,"explicitlyNormalizedReaderNotification":1},"gitWitnessesVerified":742,"rawGitBatchBytes":len(raw),"fiveUnchangedPublicOrSharedPaths":unchanged,"runs":results,"summarySHA256":sha(sr),"archiveSHA256":sha(zraw),"peerReview":"Acceptance exact source diff review plus root immutable byte/event intake","formalAcceptanceGranted":False,"nativeRuntimeExecuted":False}
b=(json.dumps(result,indent=2)+"\n").encode();(E/"receipt.json").write_bytes(b)
print(json.dumps({"source":s["sourceCommit"],"originalMembers":61,"gitWitnesses":742,"counts":results[-1]["counts"],"receiptSHA256":sha(b),"formalAcceptanceGranted":False}))
