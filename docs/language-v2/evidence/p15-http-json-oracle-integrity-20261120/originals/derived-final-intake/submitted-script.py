import pathlib,subprocess,json,hashlib,datetime,collections,re,math
r=pathlib.Path(r"C:\Users\DAVID\Code\cherri-p10r15-json-oracle-r5-20261120")
e=pathlib.Path(r"C:\Users\DAVID\Code\cherri-p10r15-continuation-evidence-20261119\r5-json-oracle")
source="3936ef41aa501b1dc5f6e201a5a1987888d0b7fd";base="89d0a327259bb97150c5dd410165e9097ff39ae9"
def git(*a):return subprocess.check_output(["git","--no-replace-objects",*a],cwd=r,timeout=30)
def text(*a):return git(*a).decode("utf-8").strip()
def sha(b):return hashlib.sha256(b).hexdigest()
def read(p):return json.loads(p.read_bytes())
assert text("rev-parse","HEAD")==source and not text("status","--porcelain")
changed=text("diff","--name-only",base,source).splitlines()
assert sorted(changed)==["p10_http_json_oracle_test.go","p10_http_json_test.go"]
specs=[
("original-nil-positive","baseline-go.jsonl","baseline-receipt.json","baseline-stderr.log",0,2,base),
("dirty-draft","draft01/go.jsonl","draft01/receipt.json","draft01/stderr.log",28,0,base),
("initial-clean-characterization","clean-ab791d18/go.jsonl","clean-ab791d18/receipt.json","clean-ab791d18/stderr.log",28,0,"ab791d1848b0c461908f4d4020ffd37f27c5e024"),
("counterexamples-with-harness-failure","counterexamples-red-79cffde0/go.jsonl","counterexamples-red-79cffde0/receipt.json","counterexamples-red-79cffde0/stderr.log",26,5,"79cffde04145685710da2a9b8687e5953d7e587c"),
("four-actual-counterexamples","counterexamples-red-49aa43be/go.jsonl","counterexamples-red-49aa43be/receipt.json","counterexamples-red-49aa43be/stderr.log",32,7,"49aa43beb1a5b77a84ddae58584205e70ef89760"),
("first-strict-clean","clean-2f14d418/go.jsonl","clean-2f14d418/receipt.json","clean-2f14d418/stderr.log",71,0,"2f14d4186d84679da356562327a6132610396852"),
("ordering-counterexample","ordering-red-ff6a75e6/go.jsonl","ordering-red-ff6a75e6/receipt.json","ordering-red-ff6a75e6/stderr.log",16,2,"ff6a75e60f41eb4e64be55104cfb10e785b3b5cf"),
("final-strict-clean","clean-3936ef41/go.jsonl","clean-3936ef41/receipt.json","clean-3936ef41/stderr.log",75,0,source)]
stages=[];final_pass=set()
for name,rawrel,recrel,errrel,expected_pass,expected_fail,commit in specs:
 raw=(e/rawrel).read_bytes();errors=(e/errrel).read_bytes();record=read(e/recrel)
 assert not errors and record["exit_code"]==(1 if expected_fail else 0)
 assert not record.get("timed_out",record.get("outer_timed_out",False))
 assert record.get("source",record.get("base"))==commit
 assert record.get("stdout_sha256",record.get("raw_sha256"))==sha(raw)
 clean=False if name=="dirty-draft" else (record.get("clean_before_after",record.get("source_clean_before_after",record.get("source_clean_before") and record.get("source_clean_after"))))
 if name!="dirty-draft":assert clean
 start=datetime.datetime.fromisoformat(record.get("started_at",record.get("started_utc")))
 elapsed=record.get("elapsed_seconds",record.get("seconds"))
 end=datetime.datetime.fromisoformat(record["finished_at"]) if record.get("finished_at") else start+datetime.timedelta(seconds=elapsed+2)
 events=[json.loads(line) for line in raw.decode("utf-8",errors="strict").splitlines()]
 runs={};done={};bytop=collections.defaultdict(collections.Counter);terminals=[];counts=collections.Counter()
 for i,event in enumerate(events):
  stamp=datetime.datetime.fromisoformat(event["Time"].replace("Z","+00:00"));assert start<=stamp<=end
  if "Elapsed" in event:assert type(event["Elapsed"]) in (int,float) and math.isfinite(event["Elapsed"]) and 0<=event["Elapsed"]<=elapsed+2
  test=event.get("Test");action=event.get("Action")
  if not test:
   if action in ("pass","fail","skip"):terminals.append(event)
   continue
  if action in ("run","pass","fail","skip"):counts[action]+=1;bytop[test.split("/",1)[0]][action]+=1
  if action=="run":assert test not in runs;runs[test]=i
  if action in ("pass","fail","skip"):assert test in runs and test not in done;done[test]=(i,action)
 assert counts["pass"]==expected_pass and counts["fail"]==expected_fail and counts["skip"]==0
 assert runs.keys()==done.keys() and len(runs)==expected_pass+expected_fail
 assert len(terminals)==1 and terminals[0]["Action"]==("fail" if expected_fail else "pass")
 for test,i in runs.items():
  parent=test
  while "/" in parent:
   parent=parent.rsplit("/",1)[0]
   if parent in runs:assert runs[parent]<i<done[test][0]<done[parent][0]
 if name=="final-strict-clean":final_pass=set(done)
 stages.append({"stage":name,"source_or_dirty_base":commit,"clean_source_execution":clean,"raw_path":rawrel,"raw_bytes":len(raw),"raw_sha256":sha(raw),"receipt_path":recrel,"receipt_sha256":sha((e/recrel).read_bytes()),"exit_code":record["exit_code"],"seconds":elapsed,"pass":counts["pass"],"fail":counts["fail"],"skip":counts["skip"],"by_top_test":bytop,"failed_names":[n for n,v in done.items() if v[1]=="fail"]})
counterexamples=[
"TestP10JSONProducerBindingIntegrity/duplicate_model_same_uuid",
"TestP10JSONProducerBindingIntegrity/duplicate_consumer_uuid",
"TestP10_B1_B2_JsonRequest_DynamicAndLiteralContract_And_LocalEchoServer/present-literal-dict-with-dynamic-fields-and-local-http-echo/strict-wire/reference_property",
"TestP10_B1_B2_JsonRequest_DynamicAndLiteralContract_And_LocalEchoServer/present-literal-dict-with-dynamic-fields-and-local-http-echo/strict-wire/reference_coercion",
"TestP10JSONProducerBindingIntegrity/producer_after_consumer"]
for name in counterexamples:
 assert name in final_pass and any(name in x["failed_names"] for x in stages)
# Compare actual immutable literal byte slices, without line-ending normalization.
tick=bytes([96]);before=git("show",base+":p10_http_json_test.go");after=git("show",source+":p10_http_json_test.go")
marker=b"v2Src := fmt.Sprintf("+tick
a=before.index(marker)+len(marker);b=before.index(tick+b", srv.URL)",a);old_literal=before[a:b]
marker=b"const p10OriginalJSONLiteralSource = "+tick
a=after.index(marker)+len(marker);b=after.index(tick,a);new_literal=after[a:b]
assert old_literal==new_literal and sha(old_literal)=="1f22bb15f3ba1fe956b5f35d81f1c843329521466faf966656f3776a6aedff02"
convenience=(e/"original-literal-request-template.cherri.txt").read_bytes()
assert convenience!=old_literal and convenience==old_literal.replace(b"\n",b"\r\n")
assert read(e/"failed-commit-guard-original.json")["job"]["exit_code"]==1
# Source witnesses cover the changed oracles, actual helper definitions and
# compiler packages they invoke. The exact clean commit binds the whole tree.
tracked=text("ls-tree","-r","--name-only",source).splitlines()
prefixes=("internal/language/analysis/","internal/language/lower/","internal/language/types/","internal/language/syntax/","internal/language/backend/","internal/language/ir/","internal/language/schema/","internal/language/service/","internal/language/nativecodec/")
paths={p for p in tracked if p.endswith(".go") and not p.endswith("_test.go") and p.startswith(prefixes)}
roots=["go.mod","go.sum","main.go","action.go","actions_std.go","shortcut.go","shortcutgen.go","mobile_compiler.go","language_backend_session.go","language_backend_adapter.go","language_cli_adapter.go","language_emit_adapter.go","language_value_adapter.go","shared_codec.go","actions/web.cherri","p10_http_json_test.go","p10_http_json_oracle_test.go","tests/language-v2/acceptance-contract.json","tests/language-v2/repair-regression-plan.json","tests/language-v2/backend-recovery-gates.json"]
for p in roots:assert p in tracked;paths.add(p)
helpers=["newRoundTripRunner","loadRoundTripDocument","p15CompiledActions","p15C01Params","p15ContainerCloneNative","p15TypeDiagnostics","p15OptionalTransportDiagnostic","CompileSourceToPlist"]
args=["grep","-l","-F"]
for h in helpers:args.extend(["-e","func "+h+"("])
args.extend([source,"--","*.go"])
helper_paths=sorted({x.split(":",1)[1] for x in text(*args).splitlines()});paths.update(helper_paths)
paths=sorted(paths);requests="".join(source+":"+p+"\n" for p in paths).encode()
result=subprocess.run(["git","--no-replace-objects","cat-file","--batch"],cwd=r,input=requests,capture_output=True,check=True,timeout=30)
assert len(result.stdout)<=16*1024*1024
pos=0;witness=[]
for path in paths:
 edge=result.stdout.index(b"\n",pos);header=result.stdout[pos:edge].decode().split();assert len(header)==3 and header[1]=="blob"
 size=int(header[2]);assert 0<=size<=4*1024*1024;pos=edge+1;blob=result.stdout[pos:pos+size];assert result.stdout[pos+size:pos+size+1]==b"\n";pos+=size+1
 witness.append({"path":path,"git_blob":header[0],"bytes":size,"sha256":sha(blob)})
assert pos==len(result.stdout)
contracts=[]
for path in roots[-3:]:
 old=git("show",base+":"+path);current=git("show",source+":"+path);assert old==current
 contracts.append({"path":path,"bytes":len(current),"sha256":sha(current)})
historical=[]
for commit,ps in [(base,["p10_http_json_test.go"]),("ab791d1848b0c461908f4d4020ffd37f27c5e024",["p10_http_json_test.go","p10_http_json_oracle_test.go"]),("79cffde04145685710da2a9b8687e5953d7e587c",["p10_http_json_test.go","p10_http_json_oracle_test.go"]),("49aa43beb1a5b77a84ddae58584205e70ef89760",["p10_http_json_oracle_test.go"]),("2f14d4186d84679da356562327a6132610396852",["p10_http_json_oracle_test.go"]),("ff6a75e60f41eb4e64be55104cfb10e785b3b5cf",["p10_http_json_oracle_test.go"])]:
 for p in ps:
  blob=git("show",commit+":"+p);historical.append({"source":commit,"path":p,"git_blob":text("rev-parse",commit+":"+p),"bytes":len(blob),"sha256":sha(blob)})
out=e/"derived-final-intake";out.mkdir(exist_ok=False)
(out/"original-immutable-template.cherri.txt").write_bytes(old_literal)
(out/"submitted-script.py").write_bytes(pathlib.Path(__file__).read_bytes())
report={"schema_version":1,"source":source,"base":base,"changed_paths":changed,"production_sources_changed":False,"stages":stages,"reproduced_and_rejected_counterexamples":counterexamples,"original_template":{"bytes":len(old_literal),"sha256":sha(old_literal),"byte_equal_to_original_git_literal":True,"derived_convenience_copy_sha256":sha(convenience),"derived_convenience_crlf_count":convenience.count(b"\r\n"),"failed_original_hashguard_preserved":True},"immutable_source_witnesses":witness,"historical_test_witnesses":historical,"helper_source_paths":helper_paths,"unchanged_contracts":contracts,"final_source_clean":not text("status","--porcelain"),"scope":["Source-bound local compiler wire observations and actual Go loopback echo assertions","Synthetic decoder metadata/present-value controls are separately named","No Apple HTTP execution, general dynamic Dictionary, compiler JSON-null, full T05/179 or deployment claim"]}
(out/"audit.json").write_bytes((json.dumps(report,indent=2,ensure_ascii=False)+"\n").encode())
print(json.dumps({"source":source,"audit":str(out/"audit.json"),"audit_sha256":sha((out/"audit.json").read_bytes()),"stages":[{k:x[k] for k in ("stage","pass","fail","seconds","clean_source_execution")} for x in stages],"current_source_witnesses":len(witness),"historical_test_witnesses":len(historical),"template":report["original_template"],"all_five_actual_red_names_pass_final":True,"changed_paths":changed,"production_changed":False,"source_clean":report["final_source_clean"]}))
