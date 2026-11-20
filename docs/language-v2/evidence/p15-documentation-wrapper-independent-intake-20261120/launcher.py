from pathlib import Path,PurePosixPath
import subprocess,json,hashlib,zipfile,io,collections,re,math
R=Path(r"C:\Users\DAVID\Code\cherri-p10r15-completion-r4-20261120")
E=Path(r"C:\Users\DAVID\Code\cherri-p10r15-continuation-evidence-20261119\r5-documentation-wrapper-independent")
E.mkdir(exist_ok=False);(E/"launcher.py").write_bytes(Path(__file__).read_bytes())
proof="8407fd880b41efb9f7a4b644c1ae2be16c6e95c4";prefix="docs/language-v2/evidence/p15-documentation-wrappers-20261120/"
def sha(b):return hashlib.sha256(b).hexdigest()
def g(root,*a):return subprocess.check_output(["git","--no-replace-objects",*a],cwd=root)
def blob(n):return g(R,"show",proof+":"+prefix+n)
sr=blob("summary.json");s=json.loads(sr);m=json.loads(blob("artifact-manifest.json"));w=json.loads(blob("source-witnesses.json"))
assert sha(sr)=="38d550df6150b2416f040d0fc41beb00d27942183918fbf685ff2a8b7dad6eab"
zr=blob(m["archive"]["path"]);assert sha(zr)==m["archive"]["sha256"]==s["archive"]["sha256"] and len(zr)==m["archive"]["bytes"]
z=zipfile.ZipFile(io.BytesIO(zr));assert len(z.namelist())==len(set(z.namelist()))==len(m["members"])==160
assert set(z.namelist())=={x["path"] for x in m["members"]}
for x in m["members"]:
 b=z.read(x["path"]);assert b==Path(x["originalPath"]).read_bytes() and len(b)==x["bytes"] and sha(b)==x["sha256"],x["path"]
for x in m["externalBinaries"]:
 b=Path(x["path"]).read_bytes();assert len(b)==x["bytes"] and sha(b)==x["sha256"],x["path"]
objects={};groups=collections.defaultdict(list)
for x in w["sources"]:groups[x["repository"]].append(x)
for root,rows in groups.items():
 queries=[x["sourceCommit"]+":"+x["path"] for x in rows]
 raw=subprocess.check_output(["git","--no-replace-objects","cat-file","--batch"],cwd=root,input=("\n".join(queries)+"\n").encode())
 assert len(raw)<=16*1024*1024
 pos=0
 for x in rows:
  stop=raw.index(b"\n",pos);h=raw[pos:stop].decode().split()
  assert len(h)==3 and h[1]=="blob" and h[0]==x["gitBlob"]
  length=int(h[2]);assert length==x["bytes"] and length<=4*1024*1024
  b=raw[stop+1:stop+1+length];assert sha(b)==x["sha256"] and raw[stop+1+length:stop+2+length]==b"\n"
  objects[(x["sourceCommit"],x["path"])]=b;pos=stop+2+length
 assert pos==len(raw)
assert len(w["sources"])==82
pre="clean-06b61987/"
pr=z.read(pre+"process-result.json");p=json.loads(pr);hr=z.read(pre+"healthy/report.json");h=json.loads(hr)
assert sha(pr)==s["rawProcessResultSHA256"] and sha(hr)==s["healthyReportSHA256"]
assert p["source"]==h["sourceCommit"]==s["sourceCommit"] and p["sourceCleanBefore"] is True and p["sourceCleanAfter"] is True
assert h["inputsUnchangedAfter"] is True and h["formalAcceptanceGranted"] is False and h["requirementsClosed"]==[]
assert h["status"]=="PASS_PACKAGED_DOCUMENTATION_WRAPPERS" and h["tuple"]==s["tuple"]
calls={};out={};err={}
for row in h["calls"]:
 name=row["id"];assert name not in calls
 actual=json.loads(z.read(pre+"healthy/calls/"+name+".process.json"));assert actual==row
 assert row["timedOut"] is False and math.isfinite(row["elapsedSeconds"]) and 0<=row["elapsedSeconds"]<=row["timeoutSeconds"]
 for field,target in [("stdout",out),("stderr",err)]:
  a=row[field];b=z.read(pre+"healthy/calls/"+a["path"])
  assert len(b)==a["bytes"] and sha(b)==a["sha256"];target[name]=b
 calls[name]=row
assert list(calls)==s["actualWrapperProcessIDs"] and len(calls)==20
for name,row in calls.items():
 if name.startswith("direct-action-"):assert row["exitCode"] in [0,1]
 elif name=="docs-invalid-regex":assert row["exitCode"]==2 and err[name]
 elif name=="docs-missing-current":assert row["exitCode"]==1 and err[name]
 else:assert row["exitCode"]==0,name
assert out["wrapper-catalog"]==out["compiler-catalog"]
cat=json.loads(out["compiler-catalog"])
for name in ["show","resizeimage","round"]:
 assert out["wrapper-action-"+name]==out["direct-action-"+name].rstrip(b"\r\n")+b"\n",name
assert calls["wrapper-catalog-filter"]["command"][-1]=="resizeImage"
expected=b"\n".join(line for line in out["compiler-catalog"].replace(b",",b"\n").splitlines() if b"resizeImage" in line)+b"\n"
assert out["wrapper-catalog-filter"]==expected and expected.strip()
S,D=h["tuple"]["S"],h["tuple"]["D"]
for name in ["compiler-input-build-info","compiler-build-info"]:
 info=out[name].decode()
 for field in ["vcs.revision="+S,"vcs.modified=false"]:
  assert re.search(r"^\s*build\s+"+re.escape(field)+r"\s*$",info,re.M)
 assert re.search(r"^\s*path\s+github\.com/electrikmilk/cherri\s*$",info,re.M)
caps=json.loads(out["compiler-capabilities"])
assert all(caps.get(k)==v for k,v in s["capabilities"].items())
docs={path:b for (rev,path),b in objects.items() if rev==D}
docsRoot=Path(h["selectedEnvironment"]["CHERRI_DOCS_DIR"]).as_posix()
retrieval=[]
for name,key in [("docs-active-0","docs-active-0"),("docs-active-1","docs-active-1"),("docs-active-2","docs-active-2"),("docs-legacy-explicit","explicit-legacy")]:
 check=s["checks"][key];query=check["query"];scope=check["scope"]
 actual=out[name].decode().splitlines()
 expected=[]
 for path,b in docs.items():
  if not path.startswith(scope+"/"):continue
  for number,line in enumerate(b.decode().splitlines(),1):
   if re.search(query,line,re.I):expected.append(docsRoot+"/"+path+":"+str(number)+":"+line)
 assert len(expected)==check["fullSourceMatches"]==check["matches"] and len(expected)<=check["headLimit"]
 assert collections.Counter(actual)==collections.Counter(expected),(name,len(actual),len(expected))
 retrieval.append({"id":name,"query":query,"exactBoundLines":len(expected)})
assert out["docs-no-match"]==b"" and out["missing-current-legacy-control"]
generated=s["generatedReference"];candidates=[b for (rev,path),b in objects.items() if rev==S and sha(b)==generated["sha256"]]
assert len(candidates)==1 and len(candidates[0])==generated["bytes"]
assert docs["fork-language/actions.md"].endswith(candidates[0])
for proc in p["processes"]:
 for field in ["stdout","stderr"]:
  b=z.read(pre+proc["id"]+"."+field);assert sha(b)==proc[field+"SHA256"]
controls=next(x for x in p["processes"] if x["id"]=="collector-controls")
assert controls["exit"]==0
controlerr=z.read(pre+"collector-controls.stderr").decode()
assert re.search(r"Ran 16 tests in ",controlerr) and re.search(r"^OK\s*$",controlerr,re.M)
negative=json.loads(z.read(pre+"current-m02-rejection/report.json"))
assert p["negativeCalls"]==1 and p["negativeStatus"]=="FAIL_PACKAGED_DOCUMENTATION_WRAPPERS"
assert "compiler revision mismatch" in p["negativeError"] and "f05a6221b44582cb04bb085d254df9ff25cf64a8" in p["negativeError"]
assert len(negative["calls"])==1 and negative["calls"][0]["id"]=="compiler-input-build-info"
for x in p["actualPackageFaults"]:
 n=json.loads(z.read(pre+x["id"]+"/report.json"))
 assert x["calls"]==0 and x["exit"]==1 and x["originalSHA256"]!=x["changedSHA256"] and n["calls"]==[]
assert s["formalAcceptanceGranted"] is False and not s["requirementsClosed"]
result={"schemaVersion":1,"proofCommit":proof,"sourceCommit":s["sourceCommit"],"tuple":s["tuple"],"originalArtifactsVerified":160,"externalBinaryHashesVerified":2,"gitWitnessesVerified":82,"actualProcessRowsVerified":20,"pythonControlTestsPassed":16,"exactDocumentationRetrieval":retrieval,"actionAndCatalogBytesMatch":True,"committedGeneratedReferenceEqualsDocs":True,"freshGeneratedReferenceExecuted":False,"actualM02StalePinRejectedBeforeWrappers":True,"actualPackageCorruptionsRejected":2,"summarySHA256":sha(sr),"archiveSHA256":sha(zr),"formalAcceptanceGranted":False,"currentReleaseTupleVerified":False}
b=(json.dumps(result,indent=2)+"\n").encode();(E/"receipt.json").write_bytes(b)
print(json.dumps({"intakeVerified":True,"originals":160,"gitWitnesses":82,"processes":20,"controls":16,"receiptSHA256":sha(b),"formalAcceptanceGranted":False}))
