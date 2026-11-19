from pathlib import Path
import subprocess,os,json,hashlib,time,sys,datetime
root=Path('/home/chatgpt-agent/workspaces/cherri-p10r15-supplemental-next-checks-20261119')
evidence=Path('/home/chatgpt-agent/workspaces/cherri-p10r15-supplemental-next-evidence-20261119')
go='/home/chatgpt-agent/workspaces/cherri-remote/tools/go/bin/go'
mode=sys.argv[1]
assert mode in ['baseline','fixed','clean']
prod=['internal/language/backend/contracts.go','internal/language/lower/lower.go','language_backend_adapter.go','language_value_adapter.go','shared_codec.go','shortcut.go']
helper='native_aggrandizement_fields.go'
primary=['TestP15BRG07DirectReferenceKindsRoundTrip','TestP15BRG07ReferenceMutationControls','TestP15BRG07SharedReferencePaths']
saved={p:(root/p).read_bytes() for p in prod+[helper]}
env=os.environ.copy()
env.update(GOMAXPROCS='2',GOFLAGS='-p=1',CHERRI_P10R10_CORPUS='0',PYTHONDONTWRITEBYTECODE='1')
env['PATH']=str(Path(go).parent)+os.pathsep+env['PATH']
env.pop('CHERRI_P10R10_CORPUS_DIR',None)
def git(*args):return subprocess.check_output(['git',*args],cwd=root,text=True).strip()
head=git('rev-parse','HEAD')
if mode=='clean':assert not git('status','--porcelain')
runs=[]
try:
 if mode=='baseline':
  for p in prod:(root/p).write_bytes(subprocess.check_output(['git','show',head+'^:'+p],cwd=root))
  (root/helper).unlink()
 selections=[('.',primary)] if mode=='baseline' else [
  ('.',primary+['TestCODEC006_NestedOrderedDictListWithRefsAndPrimitives','TestCODEC008_ApproachD_DictionaryCoercion','TestCODEC010_CoercionAndTransformationsMatrix','TestP15R03OrderedAggrandizementsRoundTrip']),
  ('./tools/language-acceptance',['TestSupplementalReferenceAndVariadicProductionEvidence','TestSupplementalAcceptanceCannotChangeRequiredTiers'])]
 for i,(package,names) in enumerate(selections):
  command=[go,'test','-count=1','-json','-p=1','-timeout=5m',package,'-run','^('+'|'.join(names)+')$']
  report={'head':head,'mode':mode,'status_before':git('status','--porcelain'),'command':command,'started_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'source_files_sha256':{p:hashlib.sha256((root/p).read_bytes()).hexdigest() if (root/p).exists() else None for p in prod+[helper,'p15_direct_reference_recovery_test.go','tests/language-v2/v2-direct-reference-chains.cherri','tools/language-acceptance/supplemental_execution.go','tools/language-acceptance/supplemental_execution_test.go']}}
  log=evidence/(mode+'-'+str(i)+'.jsonl')
  start=time.monotonic()
  with log.open('wb') as f:
   result=subprocess.run(command,cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT,timeout=420)
  events=[]; nonjson=[]
  for line in log.read_text(errors='replace').splitlines():
   try:events.append(json.loads(line))
   except json.JSONDecodeError:nonjson.append(line)
  seen={x['Test'] for x in events if x.get('Action')=='run' and x.get('Test')}
  outcomes={x['Test']:x['Action'] for x in events if x.get('Action') in ['pass','fail','skip'] and x.get('Test')}
  bad={t:v for t,v in outcomes.items() if v!='pass'}
  counts={state:sum(v==state for v in outcomes.values()) for state in ['pass','fail','skip']}
  report.update(exit_code=result.returncode,elapsed_seconds=round(time.monotonic()-start,3),discovered_unique_tests=len(seen),counts=counts,missing_required_tests=sorted(set(names)-seen),unfinished_tests=sorted(seen-set(outcomes)),failures_or_skips={t:{'status':v,'detail':''.join(x.get('Output','') for x in events if x.get('Test')==t and x.get('Action')=='output')[:14000]} for t,v in bad.items()},non_json_output=nonjson[:50],raw_jsonl_sha256=hashlib.sha256(log.read_bytes()).hexdigest(),raw_jsonl_bytes=log.stat().st_size,status_after=git('status','--porcelain'))
  (evidence/(mode+'-'+str(i)+'-receipt.json')).write_text(json.dumps(report,indent=2)+'\n')
  runs.append(report)
  print(json.dumps({k:v for k,v in report.items() if k not in ['failures_or_skips','source_files_sha256']},indent=2),flush=True)
  print('Failures/skips:',json.dumps(bad),flush=True)
  assert not report['missing_required_tests'], 'Missing actual test discovery'
  assert not report['unfinished_tests'], 'Unfinished test execution'
  if mode=='baseline':
   assert result.returncode!=0
   assert outcomes[primary[0]]=='fail' and outcomes[primary[1]]=='pass' and outcomes[primary[2]]=='fail', 'Expected causal reference-loss baseline not observed'
  else:
   assert result.returncode==0 and counts['fail']==0 and counts['skip']==0
finally:
 if mode=='baseline':
  for p,data in saved.items():(root/p).write_bytes(data)
(evidence/(mode+'-receipt.json')).write_text(json.dumps({'head':head,'mode':mode,'runs':runs,'restored_status':git('status','--porcelain')},indent=2)+'\n')
