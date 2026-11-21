import pathlib,subprocess,json,hashlib,collections,zipfile,datetime
config=json.loads("{\"source\":\"46246596dee9032fb41ce9bf35107183fda19bab\",\"base\":\"1720ccd8f9272cfcc379ad5c5bea0e4720a55aa8\",\"slug\":\"p15-e05-atomic-analysis-snapshot-20261120\",\"scope\":\"Actual production shared-mobile Go analysis response under concurrent/versioned edits; atomic diagnostic/version ownership. C/iOS ABI, UI cancellation/publication and full E05 remain unexecuted.\",\"stages\":[\"e05-mobile-baseline-a1b83438\",\"e05-mobile-baseline-d22ec46a\",\"e05-mobile-baseline-05a45465\",\"e05-mobile-clean-46246596\",\"e05-packages-clean-46246596\"],\"expectedPass\":235,\"changed\":[\"ios_bridge.go\",\"mobile_language_service.go\",\"p15_editor_version_snapshot_test.go\",\"internal/language/service/service.go\"],\"dependencies\":[\"go.mod\",\"go.sum\",\"mobile_compiler.go\",\"internal/language/service/diagnostics.go\",\"internal/language/service/module_analysis.go\",\"internal/language/analysis/analyzer.go\",\"internal/language/analysis/diagnostics.go\",\"internal/language/protocol/protocol.go\",\"internal/language/schema/registry.go\",\"internal/language/source/span.go\",\"internal/language/syntax/parser.go\"],\"report\":\"# E05 atomic versioned analysis snapshot — actual local component\\n\\nTested source:46246596dee9032fb41ce9bf35107183fda19bab. Base:1720ccd8f9272cfcc379ad5c5bea0e4720a55aa8.\\n\\nThis component repairs a real shared-mobile service response race. It does not establish E05's required UI, cancellation/publication in SwiftUI, Apple execution, full179 or final consumer identity.\\n\\n## Actual causal failure\\n\\nThe existing CherriAnalyzeVersioned export updated the shared document, read diagnostics and then read active version through three separately locked Service operations. Behavior-preserving extraction a1b83438 moves that exact response producer into mobile_language_service.go, with the C export retaining exact delegation and response encoding. The portable tests execute the actual production Go response function; the C wrapper is checked structurally, not represented as an executed iOS ABI.\\n\\nThe corrected clean05a45465 replay executes128 actual calls across eight concurrent workers.107 returned a newer response.Version with diagnostics from an older source marker, including Version12 with missingVersion0011. The expected diagnostic identity/range is independently authored from each returned version. Sequential stale input, independent documents, returned-copy isolation, healthy recovery, close/reopen, C delegation and11 mutations of actual response fields pass. This isolates one genuine concurrency failure.\\n\\n## Repair\\n\\nService.AnalyzeVersioned updates the document and obtains diagnostics, schema identity and active version under one existing service mutex. OpenDocument and Analyze delegate to private locked helpers so their public signatures, stale lower-version policy, module/asset refresh and exact diagnostic dedup remain unchanged. The actual mobile response producer calls the new atomic API; no wire schema, compiler mode, semantic type, codec or179 contract is altered.\\n\\n## Retained test issues\\n\\nThe first a1b83438 attempt executed no tests: the new test imported go/token without an alias, colliding with the root package's existing token declaration. The separate d22ec46a correction fixes only that test import and C delegation guard. Its first executed oracle then wrongly expected lowercase error, while the existing protocol deliberately emits ERROR. That failed original is retained;05a45465 corrects only the expected spelling and leaves the actual107 cross-version mismatches visible. No production output is changed to satisfy either test mistake.\\n\\n## Actual execution\\n\\nSTAGE_TABLE\\n\\nThe clean repaired root command go test -count=1 -json -timeout=3m -run ^TestP15E05 . completed20 named PASS,0FAIL/0SKIP,49.922s, clean before/after. All128 actual production responses matched their versions, with zero mismatches. Original17,717B raw SHA256 de45acd65d4bfb68f0bdbb27997242d340b10d29fd9c150f2f9112e5e26f7480.\\n\\nThe same immutable source then passed the entire service/analysis/LSP packages:235 named PASS,0FAIL/0SKIP,23.547s, original217,437B raw SHA256 e1091d512e68351d46f20dbccddf744b389894cec0233a783bdeab2bf1e162a6. Both commands used GOMAXPROCS=2 and retained their finite original budgets.\\n\\n## Evidence scope\\n\\nAll original logs, receipts and submitted scripts are byte-preserved. provenance.json records exact source commits, event counts, immutable Git witnesses and three unchanged acceptance contracts. The ZIP is verified against every original; proof bytes are reread from the separate committed object. These20 production controls are local evidence only; no synthetic process events, inherited requirement IDs, native UI pass or full E05 closure are generated.\\n\\nSwift publication/cancellation behavior and its real UI execution remain separate follow-ups. The shared versioned service is now coherent, but a UI can still mishandle results unless it independently preserves current request ownership.\\n\"}")
root=pathlib.Path.cwd()
def git(*args):return subprocess.run(["git","--no-replace-objects",*args],cwd=root,capture_output=True,check=True).stdout
source,base=config["source"],config["base"]
assert git("rev-parse","HEAD").decode().strip()==source and git("status","--porcelain")==b""
dist=root/"dist/p15-editor-evidence-20261120";prefix="docs/language-v2/evidence/"+config["slug"];evidence=root/prefix
assert not evidence.exists();evidence.mkdir(parents=True)
def digest(raw):return hashlib.sha256(raw).hexdigest()
artifacts=[];audits=[]
for stage in config["stages"]:
 folder=dist/stage;receipt=json.loads((folder/"receipt.json").read_text(encoding="utf-8"))
 events=[json.loads(line)for line in(folder/"stdout.jsonl").read_bytes().splitlines()if line]
 counts=dict(collections.Counter(x["Action"]for x in events if x.get("Test")and x.get("Action")in["pass","fail","skip"]))
 assert counts==receipt["named_counts"],(stage,counts)
 for row in receipt["artifacts"]:
  raw=(folder/row["path"]).read_bytes();assert len(raw)==row["bytes"] and digest(raw)==row["sha256"]
 if not receipt["status_before"]:assert receipt["status_after"]=="" and receipt["head_after"]==receipt["source"]
 audits.append({"stage":stage,"source":receipt["source"],"dirty":bool(receipt["status_before"]),"exit_code":receipt["exit_code"],"timed_out":receipt["timed_out"],"duration_seconds":receipt["duration_seconds"],"named_counts":counts,"raw_sha256":digest((folder/"stdout.jsonl").read_bytes()),"terminal_events":[{"package":x["Package"],"test":x["Test"],"action":x["Action"]}for x in events if x.get("Test")and x.get("Action")in["pass","fail","skip"]]})
 for p in sorted(folder.rglob("*")):
  if p.is_file():
   raw=p.read_bytes();rel="originals/"+stage+"/"+p.relative_to(folder).as_posix()
   target=evidence/rel;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(raw);assert target.read_bytes()==raw
   artifacts.append({"path":rel,"original_path":str(p),"bytes":len(raw),"sha256":digest(raw)})
assert audits[-1]["named_counts"]=={"pass":config["expectedPass"]} and audits[-1]["exit_code"]==0 and not audits[-1]["dirty"] and not audits[-1]["timed_out"]
changed=git("diff","--name-only",base,source).decode().splitlines()
assert set(changed)==set(config["changed"]),(changed,config["changed"])
contracts=["tests/language-v2/acceptance-contract.json","tests/language-v2/repair-regression-plan.json","tests/language-v2/backend-recovery-gates.json"]
witnesses=[]
commits=list(dict.fromkeys([base,source]+[x["source"]for x in audits]))
for sha in commits:
 paths=sorted(set(changed+config["dependencies"]+contracts)) if sha in[base,source] else changed
 for rel in paths:
  result=subprocess.run(["git","--no-replace-objects","show",sha+":"+rel],cwd=root,capture_output=True)
  if result.returncode:
   assert rel in config["changed"];witnesses.append({"commit":sha,"path":rel,"absent":True});continue
  raw=result.stdout;dest="source/"+sha+"/"+rel;p=evidence/dest;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(raw)
  witnesses.append({"commit":sha,"path":rel,"artifact":dest,"bytes":len(raw),"sha256":digest(raw)})
unchanged=[]
for rel in contracts:
 a=git("show",base+":"+rel);b=git("show",source+":"+rel);assert a==b
 unchanged.append({"path":rel,"bytes":len(b),"sha256":digest(b)})
archive=evidence/"original-evidence.zip"
with zipfile.ZipFile(archive,"w",compression=zipfile.ZIP_DEFLATED,compresslevel=6)as z:
 for row in artifacts:z.write(evidence/row["path"],row["path"])
with zipfile.ZipFile(archive)as z:
 for row in artifacts:assert digest(z.read(row["path"]))==row["sha256"]
provenance={"schema_version":1,"source_commit":source,"base_commit":base,"created_at":datetime.datetime.now(datetime.timezone.utc).isoformat(),"scope":config["scope"],"formal_requirements_proved":[],"apple_runtime":"NOT_RUN","full179":"NOT_RUN","changed_paths":changed,"artifacts":artifacts,"source_witnesses":witnesses,"unchanged_contracts":unchanged,"stages":audits,"archive":{"path":archive.name,"bytes":archive.stat().st_size,"sha256":digest(archive.read_bytes())}}
(evidence/"provenance.json").write_text(json.dumps(provenance,indent=2)+"\n",encoding="utf-8",newline="\n")
table="| Attempt | Source state | Exit | Named PASS | Named FAIL | Named SKIP | Seconds |\n| --- | --- | ---: | ---: | ---: | ---: | ---: |\n"
table+="\n".join("| "+x["stage"]+" | "+x["source"][:8]+(" dirty"if x["dirty"]else" clean")+" | "+str(x["exit_code"])+" | "+str(x["named_counts"].get("pass",0))+" | "+str(x["named_counts"].get("fail",0))+" | "+str(x["named_counts"].get("skip",0))+" | "+("%.3f"%x["duration_seconds"])+" |"for x in audits)
report_path="docs/language-v2/"+config["slug"]+".md"
(root/report_path).write_text(config["report"].replace("STAGE_TABLE",table),encoding="utf-8",newline="\n")
assert b"\r"not in(root/report_path).read_bytes()
subprocess.run(["git","add","-f","--",prefix,report_path],cwd=root,check=True,capture_output=True)
subprocess.run(["git","commit","-m","docs(evidence): seal "+config["slug"]+" source and actual replay"],cwd=root,check=True,capture_output=True)
head=git("rev-parse","HEAD").decode().strip()
for row in artifacts+[w for w in witnesses if not w.get("absent")]:
 rel=prefix+"/"+row.get("artifact",row.get("path"));raw=git("show",head+":"+rel)
 assert len(raw)==row["bytes"] and digest(raw)==row["sha256"],rel
assert git("status","--porcelain")==b""
print(json.dumps({"proof_commit":head,"source_commit":source,"verified_originals":len(artifacts),"verified_git_witnesses":len([w for w in witnesses if not w.get("absent")]),"archive":provenance["archive"],"provenance_sha256":digest((evidence/"provenance.json").read_bytes())}))
