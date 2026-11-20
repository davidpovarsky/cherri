from pathlib import Path
import subprocess,json,hashlib,io,zipfile
R=Path(r"C:\Users\DAVID\Code\cherri-p10r15-completion-r4-20261120")
E=Path(r"C:\Users\DAVID\Code\cherri-p10r15-continuation-evidence-20261119\r5-openminis-independent")
E.mkdir(parents=True,exist_ok=False)
(E/"launcher.py").write_bytes(Path(__file__).read_bytes())
C="bf621464b46f51c655381b2b6c65312963cab053"
P="docs/language-v2/evidence/p15-openminis-source-selection-44731965480-20261120/"
def sha(b):return hashlib.sha256(b).hexdigest()
def g(*a):return subprocess.check_output(["git","--no-replace-objects",*a],cwd=R)
def blob(p):return g("show",C+":"+P+p)
report=json.loads(blob("report.json"))
manifest=json.loads(blob("artifact-manifest.json"))
witness=json.loads(blob("source-witnesses.json"))
external=json.loads(blob("retained-external-artifacts.json"))
zb=blob("originals.zip")
assert sha(zb)==report["archive"]["sha256"] and len(zb)==report["archive"]["bytes"]
with zipfile.ZipFile(io.BytesIO(zb)) as z:
 assert len(z.namelist())==len(set(z.namelist()))==len(manifest)==68
 assert set(z.namelist())=={a["archive_path"] for a in manifest}
 for row in manifest:
  b=z.read(row["archive_path"]); original=Path(row["original_path"]).read_bytes()
  assert b==original and len(b)==row["bytes"] and sha(b)==row["sha256"],row["archive_path"]
for row in external:
 b=Path(row["original_path"]).read_bytes()
 assert len(b)==row["bytes"] and sha(b)==row["sha256"],row["original_path"]
 with zipfile.ZipFile(io.BytesIO(b)) as z: assert len(z.namelist())==row["archiveMembers"]
queries=[x["commit"]+":"+x["path"] for x in witness]
raw=subprocess.check_output(["git","--no-replace-objects","cat-file","--batch"],input=("\n".join(queries)+"\n").encode(),cwd=R)
offset=0
for x in witness:
 stop=raw.index(b"\n",offset); parts=raw[offset:stop].decode().split()
 assert len(parts)==3 and parts[1]=="blob" and parts[0]==x["git_blob"]
 size=int(parts[2]);data=raw[stop+1:stop+1+size]
 assert len(data)==x["bytes"] and sha(data)==x["sha256"]
 assert raw[stop+1+size:stop+2+size]==b"\n"
 offset=stop+2+size
assert offset==len(raw) and len(witness)==25
receipt={"scope":"Original byte and Git intake; semantic row review pending","component":C,"originalsVerified":len(manifest),"externalArchivesVerified":len(external),"gitWitnessesVerified":len(witness),"archiveSHA256":sha(zb),"reportSHA256":sha(blob("report.json")),"formalAcceptanceGranted":False}
(E/"byte-intake.json").write_text(json.dumps(receipt,indent=2)+"\n",encoding="utf-8")
print(json.dumps(receipt))
for a in manifest:
 if a["archive_path"].endswith(("signing-protocol-exit-evidence.json","run.json","jobs.json","macro-compatibility-isolation.json")):
  x=json.loads(Path(a["original_path"]).read_bytes())
  print(json.dumps({"path":a["original_path"],"keys":list(x) if isinstance(x,dict) else None,"summary":{k:v for k,v in x.items() if not isinstance(v,(dict,list))} if isinstance(x,dict) else None,"containers":{k:{"type":type(v).__name__,"len":len(v),"first":v[:1] if isinstance(v,list) else list(v)[:20]} for k,v in x.items() if isinstance(v,(dict,list))} if isinstance(x,dict) else None}))
print(json.dumps({"buildInfoArtifacts":[a for a in manifest if any(s in a["archive_path"] for s in ["build-info","intake-receipt","terminal-state","jobs","run-api","run.json"])]}))
