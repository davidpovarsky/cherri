import pathlib,subprocess,json,datetime,time,hashlib,shutil,os
r=pathlib.Path(r"C:\Users\DAVID\Code\cherri-p10r15-json-oracle-r5-20261120");source="79cffde04145685710da2a9b8687e5953d7e587c"
def git(*a):return subprocess.check_output(["git","--no-replace-objects",*a],cwd=r,timeout=25).decode().strip()
assert git("rev-parse","HEAD")==source and not git("status","--porcelain")
free=shutil.disk_usage(r).free;assert free>=2*1024**3,("insufficient admission capacity",free)
out=pathlib.Path(r"C:\Users\DAVID\Code\cherri-p10r15-continuation-evidence-20261119\r5-json-oracle\counterexamples-red-79cffde0");out.mkdir(parents=True,exist_ok=False)
(out/"submitted-script.py").write_bytes(pathlib.Path(__file__).read_bytes())
command=["go","test","-count=1","-json","-timeout=4m","-run","^(TestP10_B1_B2_JsonRequest_DynamicAndLiteralContract_And_LocalEchoServer|TestP10JSONOriginalNilRequestFailsPrecisely|TestP10JSONProducerBindingIntegrity)$","."]
started=datetime.datetime.now(datetime.timezone.utc).isoformat();clock=time.monotonic();timeout=False
with (out/"go.jsonl").open("wb") as stdout,(out/"stderr.log").open("wb") as stderr:
 p=subprocess.Popen(command,cwd=r,stdout=stdout,stderr=stderr,env=os.environ.copy())
 try:p.wait(timeout=280)
 except subprocess.TimeoutExpired:
  timeout=True;subprocess.run(["taskkill","/PID",str(p.pid),"/T","/F"],capture_output=True,timeout=20);p.wait(timeout=20)
elapsed=time.monotonic()-clock;finished=datetime.datetime.now(datetime.timezone.utc).isoformat()
raw=(out/"go.jsonl").read_bytes();errors=(out/"stderr.log").read_bytes();events=[];bad=[]
for n,line in enumerate(raw.splitlines(),1):
 try:events.append(json.loads(line.decode("utf-8",errors="strict")))
 except Exception as e:bad.append({"line":n,"error":type(e).__name__})
record={"source":source,"source_clean_before":True,"source_clean_after":git("rev-parse","HEAD")==source and not git("status","--porcelain"),"kind":"actual-counterexample-baseline","command":command,"started_at":started,"finished_at":finished,"elapsed_seconds":elapsed,"exit_code":p.returncode,"outer_timed_out":timeout,"named_run":sum(e.get("Action")=="run" and bool(e.get("Test")) for e in events),"named_pass":sum(e.get("Action")=="pass" and bool(e.get("Test")) for e in events),"named_failed":[e["Test"] for e in events if e.get("Action")=="fail" and e.get("Test")],"named_skipped":[e["Test"] for e in events if e.get("Action")=="skip" and e.get("Test")],"raw_parse_errors":bad,"stdout_bytes":len(raw),"stdout_sha256":hashlib.sha256(raw).hexdigest(),"stderr_bytes":len(errors),"stderr_sha256":hashlib.sha256(errors).hexdigest(),"free_bytes_before":free,"free_bytes_after":shutil.disk_usage(r).free}
(out/"receipt.json").write_bytes((json.dumps(record,indent=2)+"\n").encode())
failed=set(record["named_failed"])
details=[e for e in events if e.get("Action")=="output" and e.get("Test") in failed and not e.get("Output","").startswith(("=== RUN","--- FAIL"))]
print(json.dumps({"receipt":str(out/"receipt.json"),**record,"failure_details":details,"stderr_tail":errors.decode("utf-8","replace")[-2500:]}))
