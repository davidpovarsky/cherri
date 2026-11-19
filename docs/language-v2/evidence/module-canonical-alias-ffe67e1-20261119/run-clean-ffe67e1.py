from pathlib import Path
import os,subprocess,time,json,hashlib
repo=Path(r'C:\Users\DAVID\Code\cherri-p10r15-openminis-evidence-20261119')
out=Path(r'C:\Users\DAVID\Code\cherri-p10r15-evidence-20261119\module-canonical-alias-8cfedc')
env=os.environ.copy();env.update({'GOMAXPROCS':'1','GOFLAGS':'-p=1'})
cmd=['go','test','-p=1','-json','-count=1','-timeout=3m','-run','^(TestP15IM0[1345]|TestMODULES_|TestP15C01|TestP15C04|TestP15Enum|TestP15F0[134]|TestP15T06)','.']
head=subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo,text=True,encoding='utf-8').strip()
assert head=='ffe67e1cc7c2363c813d9a32a1865b655597708f'
assert not subprocess.check_output(['git','status','--porcelain'],cwd=repo,text=True,encoding='utf-8').strip()
start=time.monotonic()
with (out/'clean-ffe67e1.jsonl').open('wb') as stream:p=subprocess.run(cmd,cwd=repo,env=env,stdout=stream,stderr=subprocess.STDOUT,timeout=210)
raw=(out/'clean-ffe67e1.jsonl').read_bytes();events=[]
for line in raw.splitlines():
 try:events.append(json.loads(line))
 except Exception:pass
s={'sourceProduction':head,'sourceState':'exact clean commit','command':cmd,'exitCode':p.returncode,'seconds':round(time.monotonic()-start,3),'bytes':len(raw),'sha256':hashlib.sha256(raw).hexdigest(),'testSha256':hashlib.sha256((repo/'p15_module_canonical_alias_test.go').read_bytes()).hexdigest(),'named':{a:sum(e.get('Action')==a and bool(e.get('Test')) for e in events) for a in ['run','pass','fail','skip']},'failures':[e.get('Test') for e in events if e.get('Action')=='fail' and e.get('Test')],'diagnostics':[e.get('Output','') for e in events if 'p15_module_canonical_alias_test.go:' in e.get('Output','')]}
(out/'clean-ffe67e1-summary.json').write_bytes((json.dumps(s,indent=2)+'\n').encode())

assert not subprocess.check_output(['git','status','--porcelain'],cwd=repo,text=True,encoding='utf-8').strip()
