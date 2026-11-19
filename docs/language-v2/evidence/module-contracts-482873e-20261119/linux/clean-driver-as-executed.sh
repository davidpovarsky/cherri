python3 - <<'PY'
from pathlib import Path
import subprocess,os,signal,time,json,hashlib,shutil
repo=Path('/home/chatgpt-agent/workspaces/cherri-p10r15-module-validation-20261119');out=Path('/home/chatgpt-agent/workspaces/cherri-p10r15-evidence-20261119/module-6439bd4');sha='482873e3b3776be7178c32977f6038559ce03d8d';go='/home/chatgpt-agent/workspaces/cherri-remote/tools/go/bin/go'
paths=['.gitattributes','action.go','internal/language/modules/graph.go','internal/language/modules/standard_library.go','internal/language/modules/stdfunc_source_generated.go','internal/language/service/p15_stdfunc_direct_test.go','tools/language-stdlib/main.go','language_module_sources.go','internal/language/analysis/analyzer.go','internal/language/analysis/module_imports.go','internal/language/lower/lower.go','p15_module_alias_case_test.go']
actual=set(subprocess.check_output(['git','diff','--cached','--name-only'],cwd=repo,text=True).splitlines());assert actual==set(paths),(actual,set(paths));assert subprocess.check_output(['git','diff','--name-only'],cwd=repo,text=True)=='';assert subprocess.check_output(['git','ls-files','--others','--exclude-standard'],cwd=repo,text=True)==''
subprocess.run(['git','fetch','origin','staging/cherri-p10r15-module-contracts-20261119'],cwd=repo,check=True,capture_output=True)
assert subprocess.check_output(['git','rev-parse','FETCH_HEAD'],cwd=repo,text=True).strip()==sha
tree=subprocess.check_output(['git','write-tree'],cwd=repo,text=True).strip();assert tree==subprocess.check_output(['git','rev-parse',sha+'^{tree}'],cwd=repo,text=True).strip(),tree
authorName=subprocess.check_output(['git','show','-s','--format=%an',sha],cwd=repo,text=True).strip();authorEmail=subprocess.check_output(['git','show','-s','--format=%ae',sha],cwd=repo,text=True).strip()
subprocess.run(['git','-c','user.name='+authorName,'-c','user.email='+authorEmail,'commit','-m','Checkpoint exact tested module follow-up tree'],cwd=repo,check=True,capture_output=True)
checkpoint=subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo,text=True).strip()
subprocess.run(['git','checkout','--detach',sha],cwd=repo,check=True,capture_output=True)
assert subprocess.check_output(['git','status','--porcelain=v1'],cwd=repo,text=True)==''
(out/'canonical-refresh.json').write_text(json.dumps({'exactSource':sha,'tree':tree,'testedWorkingTreeIdenticalToCanonical':True,'localPreservedCheckpoint':checkpoint,'clean':True},indent=2)+'\n')
env=os.environ.copy();env.update({'GOMAXPROCS':'1','GOFLAGS':'-p=1','CGO_ENABLED':'0'});env['PATH']=str(Path(go).parent)+os.pathsep+env['PATH'];records=[]
def run(name,cmd,limit=240):
 assert subprocess.check_output(['git','status','--porcelain=v1'],cwd=repo,text=True)==''
 start=time.monotonic();log=out/(name+'.log')
 with log.open('wb') as f:
  p=subprocess.Popen(cmd,cwd=repo,env=env,stdout=f,stderr=subprocess.STDOUT,start_new_session=True)
  try:code=p.wait(timeout=limit)
  except subprocess.TimeoutExpired:os.killpg(p.pid,signal.SIGKILL);p.wait();raise
 raw=log.read_bytes();r={'name':name,'source':sha,'sourceCleanBefore':True,'command':cmd,'exitCode':code,'seconds':round(time.monotonic()-start,3),'bytes':len(raw),'sha256':hashlib.sha256(raw).hexdigest(),'sourceCleanAfter':subprocess.check_output(['git','status','--porcelain=v1'],cwd=repo,text=True)==''}
 if '-json' in cmd:
  events=[json.loads(line) for line in raw.splitlines() if line.startswith(b'{')];r['named']={a:sum(x.get('Action')==a and bool(x.get('Test')) for x in events) for a in ['run','pass','fail','skip']};r['failures']=[x['Test'] for x in events if x.get('Action')=='fail' and x.get('Test')]
 records.append(r);(out/'module-clean-482873e-summary.json').write_text(json.dumps({'source':sha,'records':records},indent=2)+'\n');print(json.dumps(r),flush=True);assert code==0 and r['sourceCleanAfter'],name
run('clean-generation-482873e',[go,'generate','.'])
run('clean-generation-check-482873e',[go,'run','./tools/language-stdlib','-check'])
run('clean-focus-482873e',[go,'test','-p=1','-json','-count=1','-timeout=3m','-run','^(TestP15IM0[1345]|TestMODULES_|TestP15C01|TestP15C04|TestP15Enum|TestP15F0[134]|TestP15T06)','.'])
run('clean-packages-482873e',[go,'test','-p=1','-json','-count=1','-timeout=3m','./internal/language/analysis','./internal/language/lower','./internal/language/service','./internal/language/workspace'])
binary=out/'cherri-482873e-linux-arm64';run('clean-build-482873e',[go,'build','-p=1','-trimpath','-buildvcs=true','-o',str(binary),'.'],180)
info=subprocess.check_output([go,'version','-m',str(binary)],text=True);assert 'vcs.revision='+sha in info and 'vcs.modified=false' in info and 'CGO_ENABLED=0' in info
(out/'build-info-482873e.txt').write_text(info)
receipt={'source':sha,'sourceClean':True,'sourceTree':tree,'origin':'https://github.com/davidpovarsky/cherri.git','records':records,'binary':{'path':str(binary),'bytes':binary.stat().st_size,'sha256':hashlib.sha256(binary.read_bytes()).hexdigest(),'buildInfoSha256':hashlib.sha256(info.encode()).hexdigest()},'freeBytes':shutil.disk_usage(repo).free};(out/'module-clean-482873e-summary.json').write_text(json.dumps(receipt,indent=2)+'\n');print('COMPLETE '+json.dumps(receipt),flush=True)
PY