import pathlib,subprocess,json,hashlib,collections,datetime
root=pathlib.Path(r"C:\Users\DAVID\Code\cherri-p10r15-local-gates-integration-20261120")
base=root/"dist/p15-local-gates-integration";actual=base/"actual-clean-718e7a16"
source="718e7a16ce0bfb1d5b3d132df06ab96f6c9793e9"
def git(*a,input=None):
 p=subprocess.run(["git","--no-replace-objects","-C",str(root),*a],input=input,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=30)
 if p.returncode:raise RuntimeError(p.stderr.decode("utf-8","replace"))
 return p.stdout
assert git("rev-parse","HEAD").decode().strip()==source and not git("status","--porcelain").strip()
sha=lambda d:hashlib.sha256(d).hexdigest()
raw=(actual/"go.jsonl").read_bytes();events=[json.loads(l) for l in raw.splitlines() if l.strip()]
outer=json.loads((actual/"receipt.json").read_bytes())
assert outer["source"]==source and outer["exit_code"]==0 and not outer["timed_out"] and outer["source_clean_after"]
assert outer["stdout_sha256"]==sha(raw) and outer["stdout_bytes"]==len(raw)
named=collections.Counter(e["Action"] for e in events if e.get("Test"));assert named["pass"]==702 and not named["fail"] and not named["skip"]
markers={"BRG18":"P15_NATIVE_PATHS_PRODUCTION_RECEIPT:","BRG21":"P15_PURE_PUBLIC_PRODUCTION_RECEIPT:","RP52":"P15_PUBLIC_FRONTEND_PRODUCTION_RECEIPT:"}
parents={"BRG18":"TestNativePathsActualProductionEvidence","BRG21":"TestPurePublicActualProductionEvidence","RP52":"TestPublicFrontendActualProductionEvidence"}
records={}
for id,mark in markers.items():
 stream="".join(e.get("Output","") for e in events if e.get("Test")==parents[id])
 assert stream.count(mark)==1,id
 value,end=json.JSONDecoder().raw_decode(stream.split(mark,1)[1])
 assert value["requirement"]==id and value["source_sha"]==source
 assert value["report"]["passed_cases"]==1 and not value["report"]["closure_complete"]
 records[id]=value
m02_manifest=json.loads((actual/"m02/main-output-manifest.json").read_bytes());m02_report=json.loads((actual/"m02/main-output-report.json").read_bytes())
records["M02"]={"requirement":"M02","source_sha":source,"records":m02_manifest["records"],"report":m02_report}
assert m02_report["passed_cases"]==0 and m02_report["skipped_cases"]==1 and not m02_report["closure_complete"]
paths=subprocess.check_output(["rg","--files","--no-ignore","-g","*-process.json",str(actual)],timeout=20).decode().splitlines()
summary=[];source_union={}
for p in sorted(map(pathlib.Path,paths)):
 rec=json.loads(p.read_bytes())
 required=rec["required_tests"];discovered=rec["discovered_tests"]
 rawpath=p.with_name(p.name.replace("-process.json","-go.jsonl"));errpath=p.with_name(p.name.replace("-process.json","-stderr.log"))
 stream=rawpath.read_bytes();stderr=errpath.read_bytes();es=[json.loads(l) for l in stream.splitlines() if l.strip()]
 passed=[e["Test"] for e in es if e.get("Test") and e.get("Action")=="pass"]
 failed=[e["Test"] for e in es if e.get("Test") and e.get("Action") in ("fail","skip")]
 assert not failed and len(passed)==len(set(passed)) and sorted(passed)==sorted(required)==sorted(discovered)
 assert rec["source_commit"]==source and rec["exit_code"]==0 and rec["timed_out"] is False and rec["source_clean_before"] and rec["source_clean_after"]
 assert rec["raw_sha256"]==sha(stream) and rec["raw_bytes"]==len(stream)
 assert rec["stderr_sha256"]==sha(stderr) and rec["stderr_bytes"]==len(stderr)
 id=rec["requirement"];assert id in records
 for f in rec["source_files"]:
  if f["path"] in source_union:assert source_union[f["path"]]==f["sha256"]
  source_union[f["path"]]=f["sha256"]
 for record in records[id]["records"]:
  assert record["requirements"]==[id] and record["implementation_sha"]==source and record["result"]=="passed" and record["exit_code"]==0
  for art in record["artifacts"]:
   path=pathlib.Path(art["path"]);assert path.is_relative_to(actual)
   data=path.read_bytes();assert sha(data)==art["sha256"] and len(data)==art["bytes"]
 summary.append({"id":id,"named_pass":len(passed),"source_files":len(rec["source_files"]),"elapsed_seconds":rec["elapsed_seconds"],"raw_bytes":len(stream),"raw_sha256":sha(stream),"process_path":str(p.relative_to(base)),"process_sha256":sha(p.read_bytes()),"tiers":[x["tier"] for x in records[id]["records"]],"report":{"passed_cases":records[id]["report"]["passed_cases"],"skipped_cases":records[id]["report"]["skipped_cases"],"closure_complete":records[id]["report"]["closure_complete"]}})
requests="".join(source+":"+p+"\n" for p in source_union)
batch=git("cat-file","--batch",input=requests.encode());pos=0;witness=[]
for path,expected in source_union.items():
 end=batch.index(b"\n",pos);hdr=batch[pos:end].split();pos=end+1
 assert hdr[1]==b"blob";n=int(hdr[2]);data=batch[pos:pos+n];pos+=n
 assert batch[pos:pos+1]==b"\n";pos+=1
 assert sha(data)==expected,path
 witness.append({"path":path,"git_blob":hdr[0].decode(),"bytes":n,"sha256":expected})
assert pos==len(batch) and len(summary)==4
out={"format_version":1,"audited_at":datetime.datetime.now(datetime.timezone.utc).isoformat(),"source":source,"outer":outer,"actual_producers":summary,"unique_source_blobs_verified":len(witness),"source_witness":witness,"observed_production_receipts":records,"scope":"Fresh actual BRG18/BRG21/RP52 local complete scenarios and M02 local native only; no BRG30 actual rerun, no Apple/global acceptance"}
data=(json.dumps(out,indent=2)+"\n").encode();(base/"independent-actual-audit.json").write_bytes(data)
print(json.dumps({"receipt":str(base/"independent-actual-audit.json"),"sha256":sha(data),"bytes":len(data),"source":source,"actual_producers":summary,"unique_source_blobs_verified":len(witness),"source_clean":not git("status","--porcelain").strip()},ensure_ascii=True))
