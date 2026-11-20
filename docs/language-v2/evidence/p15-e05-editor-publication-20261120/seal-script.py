import pathlib,subprocess,json,hashlib,zipfile,datetime
w=pathlib.Path(r"C:\Users\DAVID\Code\cherri-p10r15-local-gates-integration-20261120")
source="47630bedef7dc986af78ff293cac4109703f0847";base="f03209a6b58b748add37aa09961cd21a8333e455"
slug="p15-e05-editor-publication-20261120"
def git(*a):return subprocess.check_output(["git","--no-replace-objects",*a],cwd=w)
def sha(b):return hashlib.sha256(b).hexdigest()
assert git("rev-parse","HEAD").decode().strip()==source and git("status","--porcelain")==b""
stages=[
 ("e05-swift-baseline-805d9b38","805d9b38e4ac13ec38ed3b497ab9dcf2492a1ecf",14,5),
 ("e05-swift-expanded-baseline-8a026dbe","8a026dbe95e3cef62ec437319136c5c3e604eb5c",24,13),
 ("e05-swift-owned-baseline-36a419df","36a419dfb32ba6b6f1872e6aaefa1ab5a6f9b87d",26,15),
 ("e05-swift-clean-47630bed",source,26,0),
]
changed=sorted(git("diff","--name-only",base,source).decode().splitlines())
expected=sorted(["ios/CherriApp/App/WorkspaceView.swift","ios/CherriApp/Editor/CherriEditorBuildRequest.swift","tools/p10r15/editor_publication_probe.py","tools/p10r15/editor_publication_probe.swift","tools/p10r15/test_editor_publication_probe.py"])
assert changed==expected,(changed,expected)
contracts=["tests/language-v2/acceptance-contract.json","tests/language-v2/repair-regression-plan.json","tests/language-v2/backend-recovery-gates.json"]
dependencies=["ios/CherriApp/Editor/CherriCompletionRequest.swift","ios/CherriApp/Editor/CherriEditorView.swift","ios/CherriApp/Compiler/CherriCompiler.swift","ios/project.yml"]
out=w/"docs/language-v2/evidence"/slug
out.mkdir(parents=True,exist_ok=False)
(out/"seal-script.py").write_bytes(pathlib.Path(__file__).read_bytes())
manifest={"schema_version":1,"source":source,"base":base,"scope":"actual portable Swift publication control flow with explicit controlled adapters; no SwiftUI, real compiler adapter, signing, Apple runtime or formal UI credit","formal_requirements_proved":[],"changed_paths":changed,"stages":[],"originals":[],"git_witnesses":[],"unchanged_contracts":[]}
for stage,pin,count,fail in stages:
 root=w/"dist/p15-editor-evidence-20261120"/stage
 receipt=json.loads((root/"receipt.json").read_bytes());probe=json.loads((root/"probe/receipt.json").read_bytes())
 assert receipt["source"]==receipt["head_after"]==pin and receipt["status_before"]==receipt["status_after"]==""
 assert all(not x["timed_out"] and 0<=x["duration_seconds"]<=x["timeout_seconds"] for x in receipt["commands"])
 assert receipt["commands"][0]["exit_code"]==0
 assert receipt["commands"][1]["exit_code"]==(1 if fail else 0)
 assert probe["compile"]["exit_code"]==0 and probe["compile"]["within_budget"] and probe["sources_unchanged"]
 assert probe["execution"]["exit_code"]==(1 if fail else 0) and probe["execution"]["within_budget"] and not probe["execution"]["timed_out"]
 raw=(root/"probe/execute.stdout").read_bytes()
 rows=[json.loads(x) for x in raw.decode("utf-8").splitlines()]
 assert len(rows)==count+1 and rows[-1]=={"event":"summary","controls":count,"failed":fail,"uiExecuted":False,"realCompilerExecuted":False}
 names=[x["name"] for x in rows[:-1]];assert len(set(names))==count
 assert sum(x["result"]=="FAIL" for x in rows[:-1])==fail
 assert all(x["scope"]=="controlled adapter; actual production method body" for x in rows[:-1])
 assert probe["summary"]==rows[-1] and probe["passed"]==(fail==0)
 for process,prefix in [(x,root)for x in receipt["commands"]]+[(probe["compile"],root/"probe"),(probe["execution"],root/"probe")]:
  for artifact in process["artifacts"]:
   b=(prefix/artifact["path"]).read_bytes()
   assert len(b)==artifact["bytes"] and sha(b)==artifact["sha256"]
 for x in probe["sources"]:
  b=git("show",pin+":"+x["path"]);assert len(b)==x["bytes"] and sha(b)==x["sha256"],(stage,x["path"])
 generated=(root/"probe"/probe["generated"]["path"]).read_bytes()
 assert len(generated)==probe["generated"]["bytes"] and sha(generated)==probe["generated"]["sha256"]
 manifest["stages"].append({"name":stage,"source":pin,"controls":count,"passed":count-fail,"failed":fail,"raw_bytes":len(raw),"raw_sha256":sha(raw),"python_test_methods":7,"commands":receipt["commands"]})
 for p in sorted(root.rglob("*")):
  if not p.is_file():continue
  rel=p.relative_to(root).as_posix();target=out/"originals"/stage/rel;target.parent.mkdir(parents=True,exist_ok=True);b=p.read_bytes();target.write_bytes(b)
  manifest["originals"].append({"stage":stage,"original_path":str(p),"artifact":target.relative_to(out).as_posix(),"bytes":len(b),"sha256":sha(b)})
for pin in dict.fromkeys([base]+[x[1] for x in stages]):
 for rel in sorted(set(changed+dependencies+contracts)):
  result=subprocess.run(["git","--no-replace-objects","show",pin+":"+rel],cwd=w,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
  if result.returncode:
   assert rel in changed and pin!=source
   manifest["git_witnesses"].append({"commit":pin,"path":rel,"absent":True});continue
  b=result.stdout;target=out/"git"/pin/rel;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(b)
  manifest["git_witnesses"].append({"commit":pin,"path":rel,"artifact":target.relative_to(out).as_posix(),"bytes":len(b),"sha256":sha(b)})
for rel in contracts:
 a=git("show",base+":"+rel);b=git("show",source+":"+rel);assert a==b
 manifest["unchanged_contracts"].append({"path":rel,"bytes":len(b),"sha256":sha(b)})
zip_path=out/"original-evidence.zip"
with zipfile.ZipFile(zip_path,"x",compression=zipfile.ZIP_DEFLATED)as z:
 for item in manifest["originals"]:z.write(out/item["artifact"],item["artifact"])
with zipfile.ZipFile(zip_path)as z:
 assert len(z.namelist())==len(manifest["originals"])
 for item in manifest["originals"]:assert sha(z.read(item["artifact"]))==item["sha256"]
zb=zip_path.read_bytes();manifest["archive"]={"path":zip_path.name,"bytes":len(zb),"sha256":sha(zb)}
manifest["created_at"]=datetime.datetime.now(datetime.timezone.utc).isoformat()
(out/"provenance.json").write_text(json.dumps(manifest,indent=2,ensure_ascii=False)+"\n",encoding="utf-8",newline="\n")
report=f"""# E05 editor publication: exact request ownership

Tested source: {source}. Base: {base}.

## Actual execution

The portable probe compiled and executed the verbatim production WorkspaceView build method and CherriEditorView completion method. Only method visibility changed for extraction. Backend, signing and file-publication adapters were explicitly controlled; SwiftUI, the real compiler bridge, native Shortcuts and real signing were not executed.

The first clean14-control baseline had9PASS/5FAIL. The expanded24-control baseline had11PASS/13FAIL. The independent26-control baseline at36a419df had11PASS/15FAIL. The clean repair executed all26 controls with0 failures, plus7 actual Python intake/process test methods. All original raw outputs, generated Swift, compile/runtime receipts and binaries are preserved, including every failed baseline.

## Repair

A shared exact UTF-16 source/mode/name snapshot now owns live-task identity and guards publication. Each invocation owns a monotonically increasing generation, including same-source and A-to-B-to-A replacement. Cancellation is checked before dispatch and again after each await. Only the current generation clears busy state. The diagnostic result is committed after its analysis await and its renewed guard. A successful unsigned build clears the previous signed share URL; a stale result cannot publish a signed file after catalog refresh. The completion method is unchanged and its existing cancellation, source and caret guards pass.

The new helper is compiled by the real Swift compiler in the portable probe. The SwiftUI task modifier itself is source-reviewed but has no UI execution claim. Current success/error/signed-adapter positives prevent an implementation that suppresses every result from passing.

## Boundaries

This proves bounded local production control flow with controlled adapters. It does not establish a complete UI session, C bridge execution, Apple runtime, actual signer behavior, deployment or final179 acceptance. E05's required UI tier remains missing. All three immutable179 contracts are unchanged.

## Evidence

{len(manifest["originals"])} exact original files; {sum(not x.get("absent",False) for x in manifest["git_witnesses"])} present Git witnesses and {sum(x.get("absent",False) for x in manifest["git_witnesses"])} explicit absence witnesses. Every actual probe input source digest matches its immutable Git blob. Original archive: {len(zb)} bytes, SHA256 {sha(zb)}.

See evidence/{slug}/provenance.json for exact source, process, event, byte and archive bindings. No historical failed event is relabeled.
"""
rp=w/"docs/language-v2"/(slug+".md");rp.write_text(report,encoding="utf-8",newline="\n")
assert b"\r" not in rp.read_bytes()
subprocess.run(["git","diff","--check"],cwd=w,check=True)
subprocess.run(["git","add","-f","--",str(out.relative_to(w)),str(rp.relative_to(w))],cwd=w,check=True)
subprocess.run(["git","commit","-m","docs(evidence): seal actual editor publication red and green controls"],cwd=w,check=True)
proof=git("rev-parse","HEAD").decode().strip()
for item in manifest["originals"]+[x for x in manifest["git_witnesses"] if "artifact" in x]:
 b=git("show",proof+":"+(out/item["artifact"]).relative_to(w).as_posix())
 assert len(b)==item["bytes"] and sha(b)==item["sha256"]
assert git("status","--porcelain")==b""
print(json.dumps({"source":source,"proof":proof,"originals":len(manifest["originals"]),"present_witnesses":sum("artifact"in x for x in manifest["git_witnesses"]),"archive":manifest["archive"],"provenance_sha256":sha((out/"provenance.json").read_bytes()),"stages":[{k:x[k]for k in ["name","controls","passed","failed","raw_sha256"]}for x in manifest["stages"]],"status":""}))
