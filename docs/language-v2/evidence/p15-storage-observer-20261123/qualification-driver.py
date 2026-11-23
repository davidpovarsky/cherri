import datetime,hashlib,json,os,pathlib,subprocess,sys,time,traceback
ROOT=pathlib.Path('/dev/shm/cherri-p10r15-runtime-20261123/cherri-storage-observer-source')
OUT=pathlib.Path('/mnt/development/projects/evidence/cherri-p10r15-storage-observer-20261123')
SOURCE='db3b01672e0e09f7898bf11bef7b11be83b51a21'
COMPILER=ROOT/'artifacts/p15-storage-qualification/cherri'
def utc(): return datetime.datetime.now(datetime.timezone.utc).isoformat()
def digest(p): return hashlib.sha256(p.read_bytes()).hexdigest()
def git(*args): return subprocess.check_output(['git',*args],cwd=ROOT).decode().strip()
def save(p,d): p.write_text(json.dumps(d,indent=2,ensure_ascii=True)+'\n')
def state():
 return {'sourceCommit':git('rev-parse','HEAD'),'sourceTree':git('rev-parse','HEAD^{tree}'),'statusPorcelain':git('status','--porcelain')}
ENV={k:os.environ[k] for k in ('PATH','HOME','USER','LOGNAME','LANG','LC_ALL','SHELL') if k in os.environ}
ENV.update({'GOTOOLCHAIN':'go1.24.13','GOMODCACHE':'/mnt/development/tooling/cherri-go/mod','GOCACHE':'/dev/shm/cherri-p10r15-runtime-20261123/go-build-cache','GOTMPDIR':'/dev/shm/cherri-p10r15-runtime-20261123/go-tmp-ubuntu','TMPDIR':'/dev/shm/cherri-p10r15-runtime-20261123/go-tmp-ubuntu','PYTHONDONTWRITEBYTECODE':'1'})
before=state(); assert before['sourceCommit']==SOURCE and not before['statusPorcelain']
save(OUT/'source-before.json',before)
began=utc(); start=time.monotonic()
stages=[
 ('storage-attempt-01',[sys.executable,'tools/p10r15/verify_storage_probes.py',str(COMPILER),'--build','--output',str(OUT/'storage-attempt-01/validation.json')],None),
 ('binding-control-attempt-01',[sys.executable,'-B','-m','unittest','-v','test_binding_defaults_probe.BindingDefaultsProbeTests.test_retained_actual_compiled_adapters_require_exact_dispatcher_completion'],60),
 ('continuation-attempt-01',[sys.executable,'tools/p10r15/verify_native_continuation_probes.py',str(COMPILER),'--output',str(OUT/'continuation-attempt-01/validation.json')],None)]
records=[]; exitcode=1; failure=None
try:
 for name,argv,stage_bound in stages:
  d=OUT/name;d.mkdir(exist_ok=False)
  env=dict(ENV)
  if name.startswith('binding-'):env['PYTHONPATH']=str(ROOT/'tools/shortcuts-simulator-qa')
  rec={'kind':'ORIGINAL_PRODUCER_PROCESS','stage':name,'argv':argv,'cwd':str(ROOT),'sourceCommit':SOURCE,'startedUtc':utc(),'additionalOuterBoundSeconds':stage_bound,'state':'RUNNING','noLiveProviderEnvironment':True,'environmentNames':sorted(env)}
  save(d/'process-start.json',rec)
  t=time.monotonic()
  try:
   with (d/'stdout.original.log').open('wb') as out,(d/'stderr.original.log').open('wb') as err:
    p=subprocess.run(argv,cwd=ROOT,env=env,stdout=out,stderr=err,timeout=stage_bound)
   rec.update({'state':'COMPLETED','exitCode':p.returncode})
  except BaseException as exc:
   rec.update({'state':'EXCEPTION','exceptionType':type(exc).__name__,'exception':str(exc)})
   raise
  finally:
   rec.update({'completedUtc':utc(),'durationSeconds':time.monotonic()-t})
   for stream in ['stdout.original.log','stderr.original.log']:
    path=d/stream
    if path.exists():rec[stream]={'bytes':path.stat().st_size,'sha256':digest(path)}
   save(d/'process-result.json',rec);records.append(rec)
  if rec['exitCode']!=0: raise RuntimeError(name+' returned '+str(rec['exitCode']))
  if name.startswith('storage-'):
   report=json.loads((d/'validation.json').read_bytes())
   assert report['sourceCommit']==SOURCE and not report['workingTreeDirty']
   assert report['compilerBuildIdentity']['mode']=='COMPILER_VCS_METADATA'
   assert report['compilerBuildIdentity']['vcsRevision']==SOURCE
   assert report['compilerBuildIdentity']['vcsModified'] is False
   assert report['nativeExecuted']==0 and report['signing']=='NOT_RUN'
   assert report['compilerBinarySha256']==digest(COMPILER)
   version=subprocess.run(['go','version','-m',str(COMPILER)],env=env,cwd=ROOT,capture_output=True,check=True)
   (OUT/'compiler-go-version-m.original.log').write_bytes(version.stdout)
   (OUT/'compiler-go-version-m.stderr.original.log').write_bytes(version.stderr)
  if name.startswith('continuation-'):
   report=json.loads((d/'validation.json').read_bytes())
   assert report['sourceCommit']==SOURCE and report['sourceTreeClean']
   assert report['compilerBinarySha256']==digest(COMPILER)
   assert report['nativeExecuted']==0 and len(report['actualCLIFixtures'])==4
 exitcode=0
except BaseException as exc:
 failure={'type':type(exc).__name__,'message':str(exc)}
 (OUT/'qualification.exception.original.log').write_text(traceback.format_exc())
finally:
 after=state();save(OUT/'source-after.json',after)
 clean=after==before
 if not clean:exitcode=1
 result={'kind':'STORAGE_OBSERVER_QUALIFICATION','sourceCommit':SOURCE,'startedUtc':began,'completedUtc':utc(),'elapsedSeconds':time.monotonic()-start,'status':'PASS' if exitcode==0 else 'FAIL','sourceStateUnchanged':clean,'stages':records,'failure':failure,'nativeExecuted':0,'signing':'NOT_RUN','providerCalls':0,'aggregateCeilingSeconds':2700,'aggregateTerminationGraceSeconds':30}
 if COMPILER.exists():result['compiler']={'path':str(COMPILER),'bytes':COMPILER.stat().st_size,'sha256':digest(COMPILER)}
 save(OUT/'qualification-result.json',result)
 print(json.dumps({'status':result['status'],'sourceCommit':SOURCE,'elapsedSeconds':result['elapsedSeconds'],'stages':[(r['stage'],r.get('exitCode'),r['durationSeconds']) for r in records],'failure':failure}))
sys.exit(exitcode)
