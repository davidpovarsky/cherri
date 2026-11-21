import pathlib,subprocess,json,hashlib,time,datetime,shutil,sys
w=pathlib.Path(r"C:\Users\DAVID\Code\cherri-p10r15-local-gates-integration-20261120")
expected="8a026dbe95e3cef62ec437319136c5c3e604eb5c"
dest=w/"dist/p15-editor-evidence-20261120/e05-swift-expanded-baseline-8a026dbe"
def git(*a):return subprocess.check_output(["git","--no-replace-objects",*a],cwd=w).decode("utf-8")
assert git("rev-parse","HEAD").strip()==expected and git("status","--porcelain")==""
assert shutil.disk_usage(w).free>=2_000_000_000
dest.mkdir(parents=True,exist_ok=False);(dest/"submitted-script.py").write_bytes(pathlib.Path(__file__).read_bytes())
receipt={"source":expected,"status_before":"","free_before":shutil.disk_usage(w).free,"started_at":datetime.datetime.now(datetime.timezone.utc).isoformat(),"commands":[]}
for name,command,budget in [
 ("python-controls",[sys.executable,"-m","unittest","discover","-s","tools/p10r15","-p","test_editor_publication_probe.py","-v"],60),
 ("swift-probe",[sys.executable,"tools/p10r15/editor_publication_probe.py","--repo",str(w),"--output",str(dest/"probe")],180)
]:
 began=time.monotonic()
 with (dest/(name+".stdout")).open("wb")as out,(dest/(name+".stderr")).open("wb")as err:
  try:
   p=subprocess.run(command,cwd=w,stdout=out,stderr=err,timeout=budget);code=p.returncode;timeout=False
  except subprocess.TimeoutExpired:code=None;timeout=True
 row={"name":name,"command":command,"timeout_seconds":budget,"exit_code":code,"timed_out":timeout,"duration_seconds":time.monotonic()-began}
 row["artifacts"]=[{"path":name+x,"bytes":(dest/(name+x)).stat().st_size,"sha256":hashlib.sha256((dest/(name+x)).read_bytes()).hexdigest()}for x in[".stdout",".stderr"]]
 receipt["commands"].append(row)
 if timeout:break
receipt.update(status_after=git("status","--porcelain"),head_after=git("rev-parse","HEAD").strip(),free_after=shutil.disk_usage(w).free,completed_at=datetime.datetime.now(datetime.timezone.utc).isoformat())
(dest/"receipt.json").write_text(json.dumps(receipt,indent=2)+"\n",encoding="utf-8",newline="\n")
print(json.dumps(receipt))
for p in [dest/"python-controls.stderr",dest/"swift-probe.stdout",dest/"swift-probe.stderr",dest/"probe"/"compile.stderr",dest/"probe"/"execute.stdout"]:
 if p.exists():print(p.name+"\n"+p.read_text(encoding="utf-8",errors="replace")[:18000])
