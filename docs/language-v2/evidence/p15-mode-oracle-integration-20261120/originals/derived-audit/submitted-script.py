import pathlib,subprocess,json,hashlib,re,collections,datetime
r=pathlib.Path(r"C:\Users\DAVID\Code\cherri-p10r15-local-gates-integration-20261120")
d=r/"dist/p15-mode-oracle-integration";source="4adf57a713e7386992aad7cb6dd39c3d8304c328";before="74e8a55dded1fbc3b4a13f26cee0e98e644d6599";baseline="8b8093eebf6544aad649a668c7afef147ff0cc32"
def git(*a):return subprocess.check_output(["git","--no-replace-objects",*a],cwd=r,timeout=30)
def text(*a):return git(*a).decode("utf-8").strip()
def sha(b):return hashlib.sha256(b).hexdigest()
def readjson(p):return json.loads(p.read_bytes())
assert text("rev-parse","HEAD")==source and not text("status","--porcelain")
changed=text("diff","--name-only",before,source).splitlines()
assert sorted(changed)==sorted(["p15_function_native_oracle_test.go","p15_value_control_function_discovery_test.go","p15_custom_output_evidence_test.go"])
actual=d/"clean-4adf57a7";receipt=readjson(actual/"receipt.json");raw=(actual/"go.jsonl").read_bytes();stderr=(actual/"stderr.log").read_bytes()
assert receipt["source"]==source and receipt["source_clean_before"] and receipt["source_clean_after"] and receipt["exit_code"]==0 and not receipt["timed_out"]
for kind,data in [("stdout",raw),("stderr",stderr)]:
 assert receipt[kind+"_bytes"]==len(data) and receipt[kind+"_sha256"]==sha(data)
assert not stderr
events=[json.loads(line) for line in raw.decode("utf-8",errors="strict").splitlines()]
start=datetime.datetime.fromisoformat(receipt["started_at"]);end=datetime.datetime.fromisoformat(receipt["finished_at"])
runs={};passes={};terminal=[];byp=collections.defaultdict(collections.Counter)
for i,e in enumerate(events):
 assert isinstance(e,dict)
 when=datetime.datetime.fromisoformat(e["Time"].replace("Z","+00:00"));assert start<=when<=end
 if "Elapsed" in e:assert isinstance(e["Elapsed"],(int,float)) and 0<=e["Elapsed"]<=receipt["elapsed_seconds"]+1
 assert e.get("Action") not in ("fail","skip")
 name=e.get("Test")
 if not name:
  if e.get("Action") in ("pass","fail","skip"):terminal.append(e)
  continue
 a=e["Action"]
 if a=="run": assert name not in runs;runs[name]=i
 if a=="pass":assert name in runs and name not in passes;passes[name]=i
 if a in ("run","pass","fail","skip"):byp[name.split("/",1)[0]][a]+=1
assert len(runs)==len(passes)==815 and runs.keys()==passes.keys()
assert len(terminal)==1 and terminal[0]["Action"]=="pass" and terminal[0]["Package"]=="github.com/electrikmilk/cherri"
for name,i in runs.items():
 parent=name
 while "/" in parent:
  parent=parent.rsplit("/",1)[0]
  if parent in runs:assert runs[parent]<i<passes[name]<passes[parent]
baseline_dir=d/"root-baseline-8b8093ee"
oldraw=(baseline_dir/"go-optional-discovery-opaque.stdout").read_bytes()
old=[json.loads(x) for x in oldraw.decode("utf-8",errors="strict").splitlines()]
oldprocess=readjson(baseline_dir/"go-optional-discovery-opaque-process.json")
assert oldprocess["source"]==baseline and oldprocess["source_clean_before_after"] and oldprocess["exit_code"]==1 and not oldprocess["timed_out"]
assert oldprocess["stdout_sha256"]==sha(oldraw) and oldprocess["stdout_bytes"]==len(oldraw)
oldpassed=[e["Test"] for e in old if e.get("Test") and e.get("Action")=="pass"]
oldfailed=[e["Test"] for e in old if e.get("Test") and e.get("Action")=="fail"]
assert len(oldpassed)==98 and len(oldfailed)==42 and not any(e.get("Action")=="skip" for e in old)
owned=[n for n in oldfailed if n.startswith("TestP15ValueControlFunctionDiscovery") or n.startswith("TestP15T04Opaque")]
unrelated=[n for n in oldfailed if n not in owned]
assert len(owned)==41 and unrelated==["TestP15OptionalNativeDiagnosticPreparedArtifactControls"] and all(n in passes for n in owned)
merge=readjson(d/"merge-and-baseline.json");originals=[]
for item in merge["original_artifacts"]:
 p=pathlib.Path(item["original_path"]);b=p.read_bytes();copy=(baseline_dir/item["name"]).read_bytes()
 assert b==copy and len(b)==item["bytes"] and sha(b)==item["sha256"]
 originals.append(item)
tops=sorted(n for n in runs if "/" not in n)
args=["grep","-l","-F"]
for name in tops:args.extend(["-e","func "+name+"("])
args.extend([source,"--","*.go"])
selected_paths=[p.split(":",1)[1] for p in text(*args).splitlines()]
inventory_src=git("show",source+":tools/language-acceptance/main_output_execution.go").decode()
inventory_block=re.search(r"var mainOutputProductionPaths = \[\]string\{(.*?)\n\}",inventory_src,re.S).group(1)
paths=sorted(set(re.findall(r'"([^"]+)"',inventory_block)+selected_paths+changed))
assert len(paths)<128
requests="".join(source+":"+p+"\n" for p in paths).encode()
batch=subprocess.run(["git","--no-replace-objects","cat-file","--batch"],cwd=r,input=requests,capture_output=True,check=True,timeout=30)
pos=0;witness=[]
for name in paths:
 edge=batch.stdout.index(b"\n",pos);header=batch.stdout[pos:edge].decode().split();assert len(header)==3 and header[1]=="blob"
 size=int(header[2]);pos=edge+1;b=batch.stdout[pos:pos+size];assert len(b)==size and batch.stdout[pos+size:pos+size+1]==b"\n";pos+=size+1
 witness.append({"path":name,"git_blob":header[0],"bytes":len(b),"sha256":sha(b)})
assert pos==len(batch.stdout)
baseline_witness=[]
for name in changed:
 b=git("show",baseline+":"+name);baseline_witness.append({"path":name,"git_blob":text("rev-parse",baseline+":"+name),"bytes":len(b),"sha256":sha(b)})
contracts=["tests/language-v2/acceptance-contract.json","tests/language-v2/repair-regression-plan.json","tests/language-v2/backend-recovery-gates.json"]
unchanged=[]
for p in contracts:
 a=git("show",baseline+":"+p);b=git("show",source+":"+p);assert a==b;unchanged.append({"path":p,"bytes":len(b),"sha256":sha(b)})
peer=pathlib.Path(r"C:\Users\DAVID\Code\cherri-p10r15-parity-evidence-20261119\r4-consumer-refresh-20261120\peer-local-gates-3dc23214\receipt.json")
peerb=peer.read_bytes();assert sha(peerb)=="c4adcd51955d4b3c7221b4b7f56304c0347730045cf4dd4cf6e95a549dd45fbd"
report={"schema_version":1,"kind":"independent-original-byte-and-source-intake","source":source,"merge_parent":before,"original_failed_source":baseline,"source_clean":not text("status","--porcelain"),"production_changed":False,"changed_paths":changed,"actual_receipt":receipt,"by_top_test":byp,"required_failed_names_now_pass":owned,"unrelated_original_failure_unclaimed":unrelated,"original_root_artifacts_verified":originals,"immutable_source_witnesses":witness,"baseline_test_source_witnesses":baseline_witness,"unchanged_contracts":unchanged,"selected_top_tests":tops,"selected_test_source_files":selected_paths,"peer_prior_component_audit":{"original_path":str(peer),"bytes":len(peerb),"sha256":sha(peerb),"scope":"Independent peer intake of prior718e/3dc; not execution at current4adf"},"limitations":["Only selected actual root tests executed at4adf; no full Go suite claim","No new Apple/iOS execution or final179 acceptance","M02 strict producer/local report remains source-bound to its own retained execution; no fabricated current record","Root Optional artifact and JSON fixes remain separate","BRG30 full scenario not rerun after this integration"]}
audit=d/"derived-audit";audit.mkdir(exist_ok=False)
(audit/"submitted-script.py").write_bytes(pathlib.Path(__file__).read_bytes())
(audit/"audit.json").write_bytes((json.dumps(report,indent=2,ensure_ascii=False)+"\n").encode("utf-8"))
(audit/"prior-component-peer-receipt.json").write_bytes(peerb)
print(json.dumps({"audit":str(audit/"audit.json"),"sha256":sha((audit/"audit.json").read_bytes()),"source":source,"actual_passes":len(passes),"fixed_original_failure_events":len(owned),"source_witnesses":len(witness),"baseline_witnesses":len(baseline_witness),"production_changed":False,"unchanged_contracts":len(unchanged),"all_original_root_copies_byte_equal":True,"clean":not text("status","--porcelain")}))
