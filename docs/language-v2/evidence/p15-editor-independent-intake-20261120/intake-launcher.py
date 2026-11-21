from pathlib import Path,PurePosixPath
import subprocess,json,hashlib,zipfile,io,collections,datetime
R=Path(r"C:\Users\DAVID\Code\cherri-p10r15-completion-r4-20261120")
E=Path(r"C:\Users\DAVID\Code\cherri-p10r15-continuation-evidence-20261119\r5-editor-independent")
E.mkdir(exist_ok=True)
(E/"intake-launcher.py").write_bytes(Path(__file__).read_bytes())
root="f05a6221b44582cb04bb085d254df9ff25cf64a8"
def g(*args):return subprocess.check_output(["git","--no-replace-objects",*args],cwd=R)
assert g("rev-parse","HEAD").decode().strip()==root
assert not g("status","--porcelain").strip()
def digest(b):return hashlib.sha256(b).hexdigest()
def batch(specs):
 specs=list(dict.fromkeys(specs))
 p=subprocess.run(["git","--no-replace-objects","cat-file","--batch"],cwd=R,input=("\n".join(specs)+"\n").encode(),capture_output=True,check=True)
 b=p.stdout;pos=0;out={}
 for spec in specs:
  stop=b.index(b"\n",pos);header=b[pos:stop].decode();pos=stop+1
  if header==spec+" missing":out[spec]=None;continue
  parts=header.split();assert len(parts)==3 and parts[1]=="blob",(spec,header)
  length=int(parts[2]);content=b[pos:pos+length];pos+=length
  assert len(content)==length and b[pos:pos+1]==b"\n";pos+=1
  out[spec]=content
 assert pos==len(b),(pos,len(b))
 return out
rows=[]
for label,proof,prefix in [
("E01","4049876b88f5be9f02625f5745917c3f7b797ac1","docs/language-v2/evidence/p15-e01-editor-context-20261120/"),
("E02","90ec31a82efdc2542fee6567a0cebcc30b510fb8","docs/language-v2/evidence/p15-e02-editor-diagnostics-20261120/"),
("E03","1720ccd8f9272cfcc379ad5c5bea0e4720a55aa8","docs/language-v2/evidence/p15-e03-semantic-editor-symbols-20261120/")]:
 raw=g("show",proof+":"+prefix+"provenance.json");p=json.loads(raw)
 assert p["formal_requirements_proved"]==[] and p["apple_runtime"]=="NOT_RUN" and p["full179"]=="NOT_RUN"
 archive=g("show",proof+":"+prefix+p["archive"]["path"])
 assert len(archive)==p["archive"]["bytes"] and digest(archive)==p["archive"]["sha256"]
 z=zipfile.ZipFile(io.BytesIO(archive))
 names=[v["path"] for v in p["artifacts"]]
 assert len(set(names))==len(names) and len(z.namelist())==len(names) and set(z.namelist())==set(names)
 for a in p["artifacts"]:
  n=PurePosixPath(a["path"]);assert not n.is_absolute() and ".." not in n.parts
  b=z.read(a["path"]);original=Path(a["original_path"]).read_bytes()
  assert b==original and len(b)==a["bytes"] and digest(b)==a["sha256"],a["path"]
 specs=[]
 for w in p["source_witnesses"]:
  specs.append(w["commit"]+":"+w["path"])
  if not w.get("absent"):specs.append(proof+":"+prefix+w["artifact"])
 for c in p["unchanged_contracts"]:
  specs.extend(rev+":"+c["path"] for rev in [p["base_commit"],p["source_commit"],proof,root])
 objects=batch(specs)
 present=absent=0
 for w in p["source_witnesses"]:
  b=objects[w["commit"]+":"+w["path"]]
  if w.get("absent"):
   assert b is None,w
   absent+=1
  else:
   assert b is not None and len(b)==w["bytes"] and digest(b)==w["sha256"],w
   assert b==objects[proof+":"+prefix+w["artifact"]],w
   present+=1
 for c in p["unchanged_contracts"]:
  for rev in [p["base_commit"],p["source_commit"],proof,root]:
   b=objects[rev+":"+c["path"]]
   assert b is not None and len(b)==c["bytes"] and digest(b)==c["sha256"],(rev,c)
 stages=[]
 for s in p["stages"]:
  pre="originals/"+s["stage"]+"/"
  stdout=z.read(pre+"stdout.jsonl");receipt=json.loads(z.read(pre+"receipt.json"))
  assert digest(stdout)==s["raw_sha256"]
  events=[json.loads(l) for l in stdout.splitlines() if l.strip()]
  terminals=[{"package":v.get("Package"),"test":v["Test"],"action":v["Action"]} for v in events if v.get("Test") and v.get("Action") in ["pass","fail","skip"]]
  counts=dict(collections.Counter(v["action"] for v in terminals))
  assert terminals==s["terminal_events"] and counts==s["named_counts"]==receipt["named_counts"],s["stage"]
  assert len({(v["package"],v["test"]) for v in terminals})==len(terminals),s["stage"]
  assert receipt["source"]==s["source"] and receipt["exit_code"]==s["exit_code"] and receipt["timed_out"]==s["timed_out"]
  assert bool(receipt["status_before"])==s["dirty"]
  assert receipt["head_after"]==s["source"]
  for a in receipt["artifacts"]:
   b=z.read(pre+a["path"]);assert len(b)==a["bytes"] and digest(b)==a["sha256"]
  stages.append({"stage":s["stage"],"source":s["source"],"dirty":s["dirty"],"exitCode":s["exit_code"],"timedOut":s["timed_out"],"counts":counts,"stdoutSHA256":digest(stdout),"command":receipt["command"]})
 final=p["stages"][-1];fr=json.loads(z.read("originals/"+final["stage"]+"/receipt.json"))
 assert final["source"]==p["source_commit"] and final["dirty"] is False and final["exit_code"]==0 and not final["timed_out"]
 assert not fr["status_before"] and not fr["status_after"] and fr["head_after"]==final["source"]
 assert set(final["named_counts"])=={"pass"} and final["named_counts"]["pass"]>0
 row={"label":label,"proofCommit":proof,"sourceCommit":p["source_commit"],"baseCommit":p["base_commit"],"provenanceSHA256":digest(raw),"archiveSHA256":digest(archive),"archiveBytes":len(archive),"originalArtifactsVerified":len(names),"gitPresentWitnessesVerified":present,"gitAbsenceWitnessesVerified":absent,"unchangedContractsVerified":len(p["unchanged_contracts"]),"stages":stages,"finalClean":True,"formalRequirementsGranted":[],"appleRuntime":"NOT_RUN","full179":"NOT_RUN"}
 (E/(label.lower()+"-receipt.json")).write_text(json.dumps(row,ensure_ascii=False,indent=2)+"\n",encoding="utf-8")
 rows.append(row)
 print(json.dumps({k:v for k,v in row.items() if k!="stages"}))
summary={"schemaVersion":1,"createdAt":datetime.datetime.now(datetime.timezone.utc).isoformat(),"rootSource":root,"components":rows,"productionDiffReview":"E01/E02 root reviewed; E03 independent parity review; no concrete defect outstanding in those changes","formalAcceptanceGranted":False,"full179":"NOT_RUN"}
b=(json.dumps(summary,ensure_ascii=False,indent=2)+"\n").encode()
(E/"receipt.json").write_bytes(b)
print(json.dumps({"allComponentsVerified":len(rows),"receiptSHA256":digest(b),"path":str(E/"receipt.json")}))
