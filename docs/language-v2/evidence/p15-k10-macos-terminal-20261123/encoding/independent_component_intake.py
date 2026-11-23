import pathlib,json,hashlib,subprocess,copy,re,datetime,zipfile,stat
ROOT=pathlib.Path('/mnt/development/projects/releases/cherri-p10r15-k10-source-20261123')
BASE=pathlib.Path('/mnt/development/projects/evidence/cherri-p10r15-k10-frozen-ci-20261123')
OUT=BASE/'required-encoding-gate-intake-01'
MEM=OUT/'original-members'
SHA='4feadab9db4d351628f78593304373631693869a'
RUN='45100706965'
PIN='ee52c32937053268e4a035eba4b372fe77ec6411'
TARGET='plainString.WFStringParameterState.string'
FORMS={'string','number','bool','attachment(variable)','attachment(output)','tokenString','wrappedSubject'}
failures=[];checks=[]
def check(ok,label):
 checks.append(label)
 if not ok: failures.append(label)
def digest(b):return hashlib.sha256(b).hexdigest()
def unique(pairs):
 o={}
 for k,v in pairs:
  if k in o: raise ValueError('duplicate JSON key: '+k)
  o[k]=v
 return o
def load(path):
 b=path.read_bytes()
 assert 0<len(b)<=4*1024*1024
 o=json.loads(b,object_pairs_hook=unique,parse_constant=lambda s: (_ for _ in ()).throw(ValueError(s)))
 def depth(x,n=0):
  assert n<=64
  if isinstance(x,dict):
   for v in x.values():depth(v,n+1)
  elif isinstance(x,list):
   for v in x:depth(v,n+1)
 depth(o)
 return o
def git(*args):return subprocess.check_output(['git','-C',str(ROOT),*args],timeout=30)
def actual(s):
 if s=='roundtrip' or isinstance(s,str) and s.startswith('accepted, reserialized as ') and len(s)>len('accepted, reserialized as '):return 'accept'
 if s=='rejected':return 'reject'
 if s in ('class not present','threw: native parameter-state exception'):return 'unavailable'
 raise ValueError('unknown outcome')
before=git('status','--porcelain=v1')
check(before==b'','source clean before')
check(git('rev-parse','HEAD').decode().strip()==SHA,'source exact K10 HEAD')
s=load(MEM/'required-encoding-gate.json')
env=load(MEM/'required-encoding-evidence.json')
expected=load(ROOT/'tools/shortcut-runtime/required-encoding-expectations.json')
required=expected['REQUIRED']
check(len(required)==23 and sum(x['expect']=='reject' for x in required)==1,'canonical original 23 and one expected rejection')
check(s['schemaVersion']==1 and s['status']=='PASS' and s['implementationSha']==SHA and s['runId']==RUN,'summary exact status/source/run')
check(s['appleEngineExecuted'] is True and s['mockEngineAllowed'] is False and s['shortcutRuntimeExecuted'] is False,'real WorkflowKit only')
check(s['inputBytesUnchanged'] is True and s['requiredCases']==23 and s['expectedRejections']==1 and s['negativeControlExit']==1 and s['failures']==[],'summary invariant counts and unchanged inputs')
check(s['host']['system']=='Darwin' and s['host']['machine']=='arm64','actual Darwin ARM64 host')
check(s['preludeCommit']==PIN,'exact prelude commit')
check(s['isolatedMutation']=={'id':TARGET,'field':'expect','before':'accept','after':'reject'},'exact sole mutation')
isolate=copy.deepcopy(expected)
next(x for x in isolate['REQUIRED'] if x['id']==TARGET)['expect']='reject'
check(load(MEM/'isolated-corrupt-expectations.json')==isolate,'original isolated file differs only in target EXPECT')
source_paths=[
'.github/workflows/build-test.yml',
'tools/shortcut-runtime/jxa/verify_encodings.js',
'tools/shortcut-runtime/required-encoding-expectations.json',
'tools/shortcut-runtime/required_encoding_gate.py',
'docs/apple-runtime/parameter-encodings.json',
'docs/apple-runtime/encoding-table.json',
'tools/language-acceptance/native_encoding_execution.go',
'tools/language-acceptance/native_encoding_intake.go',
'tools/language-acceptance/main.go',
'tests/language-v2/acceptance-contract.json',
'tests/language-v2/repair-regression-plan.json',
'tests/language-v2/backend-recovery-gates.json']
witness=[]
blobs={}
for rel in source_paths:
 b=git('show',SHA+':'+rel);blobs[rel]=b
 check((ROOT/rel).read_bytes()==b,'tracked exact '+rel)
 witness.append({'path':rel,'sha256':digest(b),'bytes':len(b),'gitBlob':git('rev-parse',SHA+':'+rel).decode().strip()})
inputs=[]
for remote,h in s['inputs'].items():
 if '/required-encoding-shortcutkit/tools/jxa-prelude.js' in remote:
  b=(OUT/'pinned-jxa-prelude.original.js').read_bytes()
  api=load(OUT/'pinned-prelude-source-api.original.json')
  check(api['sha']==hashlib.sha1(b'blob '+str(len(b)).encode()+b'\0'+b).hexdigest(),'pinned external git blob SHA1')
  rel='frontboat/shortcutkit@'+PIN+':tools/jxa-prelude.js'
 else:
  assert '/cherri/cherri/' in remote
  rel=remote.split('/cherri/cherri/',1)[1]
  assert rel in blobs
  b=blobs[rel]
 check(digest(b)==h,'summary input bytes '+rel)
 inputs.append({'identity':rel,'bytes':len(b),'sha256':digest(b)})
check(len(inputs)==6,'six original input hashes independently bound')
gate=blobs['tools/shortcut-runtime/required_encoding_gate.py'].decode()
workflow=blobs['.github/workflows/build-test.yml'].decode()
check('timeout=180' in gate and 'timeout-minutes: 12' in workflow,'original phase and workflow bounds')
check(PIN in gate and PIN in workflow and 'pinned + b"\\n" + GATE.read_bytes()' in gate,'source-bound pinned runner assembly')
expected_names={'required-encoding-gate.json','required-encoding-evidence.json','isolated-corrupt-expectations.json'}
for phase in ('positive','corrupt','restored'):
 expected_names.update(phase+suffix for suffix in ('.json','.process.json','.stdout.log','.stderr.log'))
check({p.name for p in MEM.iterdir()}==expected_names,'exact 15-member original inventory')
members=load(OUT/'members.json')
check(len(members)==15 and len({m['path'] for m in members})==15,'unique member names')
for m in members:
 b=(MEM/m['path']).read_bytes()
 check(len(b)==m['bytes'] and digest(b)==m['sha256'],'member original hash '+m['path'])
phases=s['phases'];check(len(phases)==3,'three actual phases')
phase_results=[]
for index,name in enumerate(('positive','corrupt','restored')):
 p=phases[index];n=index==1
 check(p==load(MEM/(name+'.process.json')),'summary equals original process '+name)
 check(p['phase']==name and p['exitCode']==(1 if n else 0),'exact phase identity and exit '+name)
 cmd=p['command']
 check(len(cmd)==(9 if n else 8) and cmd[:3]==['/usr/bin/osascript','-l','JavaScript'],'exact real command '+name)
 for i,bn in ((3,'verify.js'),(4,'parameter-encodings.json'),(5,'encoding-table.json'),(6,name+'.json'),(7,'required-encoding-expectations.json')):
  check(pathlib.PurePosixPath(cmd[i]).name==bn,'argv basename '+name+':'+str(i))
 for i in (0,1,2,3,4,5,7):
  check(cmd[i]==phases[0]['command'][i],'identical source/discovery argv '+name+':'+str(i))
 check(pathlib.PurePosixPath(cmd[6]).parent==pathlib.PurePosixPath(cmd[3]).parent,'phase output same directory '+name)
 if n:
  check(pathlib.PurePosixPath(cmd[8]).name=='isolated-corrupt-expectations.json' and pathlib.PurePosixPath(cmd[8]).parent==pathlib.PurePosixPath(cmd[3]).parent,'isolated corruption command')
 for suffix,key in (('.json','reportSha256'),('.stdout.log','stdoutSha256'),('.stderr.log','stderrSha256')):
  check(digest((MEM/(name+suffix)).read_bytes())==p[key],'phase stream/report hash '+name+suffix)
 stderr=(MEM/(name+'.stderr.log')).read_bytes()
 diagnostic=b'E_REQUIRED_ENCODING_EXPECTATION' in stderr and TARGET.encode() in stderr
 check(diagnostic==n and p['negativeDiagnosticObserved']==diagnostic,'exact stderr negative diagnostic '+name)
 r=load(MEM/(name+'.json'))
 check(r['schemaVersion']==2 and r['engineLoaded'] is True and r['engine']=='WorkflowKit parameter-state API','actual engine identity '+name)
 check(len(r['forms'])==7 and set(r['forms'])==FORMS,'all seven forms '+name)
 matrix=r['results'];d=r['discovery']
 check(type(d['parameterStateClasses']) is int and d['parameterStateClasses']>0 and type(d['appIntentStateClasses']) is int and d['appIntentStateClasses']>0 and d['uniqueStateClasses']==len(matrix)>0,'complete discovery '+name)
 observed_cells=0
 for state,row in matrix.items():
  check(re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*',state) is not None,'state identity '+name+':'+state)
  if row=='class not present':continue
  check(isinstance(row,dict) and set(row)==FORMS,'complete matrix row '+name+':'+state)
  for form,value in row.items():
   check(value!='class not present' and actual(value) in ('accept','reject','unavailable'),'recognized matrix observation '+name+':'+state+':'+form)
   observed_cells+=1
 applied=(isolate if n else expected)['REQUIRED']
 check(r['requiredExpectations']==applied,'all original applied EXPECT '+name)
 check(len(r['requiredResults'])==23,'all required observations '+name)
 derived=[]
 for e,o in zip(applied,r['requiredResults']):
  row=matrix[e['stateClass']]
  raw=row[e['form']] if isinstance(row,dict) else row
  a=actual(raw);passed=a==e['expect']
  wanted={k:e[k] for k in ('id','kind','stateClass','form')}
  wanted.update(expected=e['expect'],actual=a,observation=raw,passed=passed)
  check(o==wanted,'required result derives from raw matrix '+name+':'+e['id'])
  if not passed:derived.append({'code':'E_REQUIRED_ENCODING_ENGINE' if a=='unavailable' else 'E_REQUIRED_ENCODING_EXPECTATION','id':e['id'],'expected':e['expect'],'actual':a})
 wanted_failures=[{'code':'E_REQUIRED_ENCODING_EXPECTATION','id':TARGET,'expected':'reject','actual':'accept'}] if n else []
 check(derived==r['failures']==wanted_failures and r['status']==('FAIL' if n else 'PASS'),'exact phase verdict '+name)
 phase_results.append({'phase':name,'exitCode':p['exitCode'],'reportStatus':r['status'],'discovery':d,'matrixObservations':observed_cells,'requiredObservations':23,'requiredPassed':sum(x['passed'] for x in r['requiredResults']),'failures':r['failures'],'reportSha256':p['reportSha256']})
check((MEM/'positive.json').read_bytes()==(MEM/'restored.json').read_bytes(),'positive and restored report bytes identical')
check(load(MEM/'positive.json')['results']==load(MEM/'corrupt.json')['results'],'negative control preserves actual per-form matrix')
records=env['records']
check(len(records)==2,'exact two dedicated evidence records')
artifact_names=expected_names-{'required-encoding-evidence.json','isolated-corrupt-expectations.json'}
for r,req in zip(records,('CI01','RP47')):
 check(r['evidence_id']=='ev-required-encoding-'+req.lower() and r['requirements']==[req] and r['tier']=='negative-ci' and r['test_id']=='tools/shortcut-runtime:required-encoding-'+req.lower(),'dedicated mapping '+req)
 check(r['implementation_sha']==SHA and r['exit_code']==0 and r['result']=='passed' and r['fixture_sha256']==digest(blobs['tools/shortcut-runtime/required-encoding-expectations.json']),'source/result/fixture binding '+req)
 check(r['command']==['python3','tools/shortcut-runtime/required_encoding_gate.py','--source-sha',SHA,'--run-id',RUN],'formal command '+req)
 check(r['assertions']=={k:'true' for k in ('required_expect_nonzero','exact_corruption_rejected','expected_rejection_passed','per_form_outcomes_preserved','restored_required_set_passed','apple_engine_executed')},'exact assertions '+req)
 check(len(r['artifacts'])==13 and {pathlib.PurePosixPath(a['path']).name for a in r['artifacts']}==artifact_names,'complete distinct 13 artifact references '+req)
 for a in r['artifacts']:
  p=pathlib.PurePosixPath(a['path'])
  check(str(p.parent)=='artifacts/backend-recovery/ci-artifacts/required-encoding-gate' and a['run_id']==RUN and a['sha256']==digest((MEM/p.name).read_bytes()),'original artifact/run binding '+req+':'+p.name)
watch=(BASE/'watch'/'snapshot.json').read_bytes()
jobs=(BASE/'watch'/'45100706965'/'jobs.latest.json').read_bytes()
(OUT/'watch-component-intake.original.json').write_bytes(watch)
(OUT/'jobs-component-intake.original.json').write_bytes(jobs)
wo=json.loads(watch);build=next(x for x in wo['runs'] if str(x['run'])==RUN)
check(build['source']==SHA,'watch exact implementation')
source_after=git('status','--porcelain=v1')
check(source_after==before==b'','source remains clean after read-only intake')
(OUT/'independent-source-witness.json').write_text(json.dumps({'implementationSha':SHA,'files':witness,'originalInputBindings':inputs},indent=2)+'\n')
result={
'schemaVersion':1,'kind':'Independent read-only component intake; no producer or Go execution',
'observedUTC':datetime.datetime.now(datetime.timezone.utc).isoformat(),
'implementationSha':SHA,'runId':RUN,'jobId':137450662143,'artifactId':14785316588,
'status':'PASS_COMPONENT_ONLY' if not failures else 'FAIL_COMPONENT_INTAKE',
'checks':len(checks),'failures':failures,'originalZipSha256':digest((OUT/'14785316588.original.zip').read_bytes()),
'originalMembers':15,'originalBytes':sum(m['bytes'] for m in members),'phaseResults':phase_results,
'requiredCases':23,'originalExpectedRejections':1,'independentlyBoundInputs':inputs,'artifactReferencesReconciled':26,
'emittedRecordRequirements':['CI01','RP47'],'emittedTier':'negative-ci',
'appleEngineExecuted':True,'shortcutRuntimeExecuted':False,'nativeShortcutRuntimeCredit':False,
'formalAggregateAdmissionExecuted':False,'formalEvidenceCredit':False,'closureAllowed':False,
'formalEligibilityAtObservation':{'buildStatus':build['status'],'buildConclusion':build['conclusion'],'knownFailedJobs':[x['id'] for x in build['jobs'] if x['conclusion']=='failure'],'reason':'Existing canonical aggregator rejects all non-success manifest runs and their artifact references. This component receipt does not waive that rule or rebind K10 evidence to K11.'},
'limitations':['No canonical Go intake or producer was rerun. This is independent mechanical reconciliation against retained original bytes and the exact committed validator source.','The existing upload glob omits assembled verify.js; its committed assembly recipe and both exact constituent source blobs are independently verified.','Phase receipts contain no monotonic duration; GitHub step wall times do not establish individual phase runtimes.','WorkflowKit parameter-state execution is not Shortcut runtime execution. No BRG31 encoding record exists.'],
'sourceCleanBeforeAfter':True,'newTests':0,'newWorkflowDispatches':0,'newNativeExecutions':0,'providerGenerationCalls':0}
target=OUT/'independent-component-intake.json'
assert not target.exists(),'preserve original derived receipt'
target.write_text(json.dumps(result,indent=2)+'\n')
(OUT/'independent-component-checks.json').write_text(json.dumps(checks,indent=2)+'\n')
print(json.dumps({'status':result['status'],'checks':len(checks),'failures':failures,'receipt':str(target),'receiptSha256':digest(target.read_bytes()),'phaseResults':phase_results,'watch':result['formalEligibilityAtObservation']},separators=(',',':')))
