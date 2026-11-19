from pathlib import Path
import subprocess,time,json,hashlib,os
repo=Path(r"C:\Users\DAVID\Code\cherri-p10r15-binding-producers-20261119")
out=Path(r"C:\Users\DAVID\Code\cherri-p10r15-evidence-20261119\binding-producers-33f0699")
out.mkdir(parents=True,exist_ok=True)
env=os.environ.copy();env.update({"GOMAXPROCS":"1","GOFLAGS":"-p=1"})
cmd=["go","test","-p=1","-count=1","-json","-timeout=2m","-run","^TestP15BindingCaptureTerminalCausality$","."]
start=time.monotonic()
with (out/"terminal-baseline.jsonl").open("wb") as stream:
 p=subprocess.Popen(cmd,cwd=repo,env=env,stdout=stream,stderr=subprocess.STDOUT)
 timeout=False
 try:code=p.wait(timeout=150)
 except subprocess.TimeoutExpired:
  timeout=True;subprocess.run(["taskkill","/PID",str(p.pid),"/T","/F"],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL);code=p.wait()
raw=(out/"terminal-baseline.jsonl").read_bytes();events=[]
for line in raw.splitlines():
 try:events.append(json.loads(line))
 except Exception:pass
summary={"sourceBase":subprocess.check_output(["git","rev-parse","HEAD"],cwd=repo,text=True,encoding="utf-8").strip(),"sourceState":"unchanged capture oracle plus new causal mutation controls","command":cmd,"exitCode":code,"timedOut":timeout,"seconds":round(time.monotonic()-start,3),"bytes":len(raw),"sha256":hashlib.sha256(raw).hexdigest(),"testSha256":hashlib.sha256((repo/"p15_binding_capture_terminal_test.go").read_bytes()).hexdigest(),"named":{a:sum(e.get("Action")==a and bool(e.get("Test")) for e in events) for a in ["run","pass","fail","skip"]},"failures":[e.get("Test") for e in events if e.get("Action")=="fail" and e.get("Test")],"diagnostics":[e.get("Output","") for e in events if ".go:" in e.get("Output","")]}
(out/"terminal-baseline-summary.json").write_text(json.dumps(summary,indent=2)+"\n",encoding="utf-8")
