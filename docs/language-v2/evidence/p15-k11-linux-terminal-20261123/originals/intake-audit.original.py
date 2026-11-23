from pathlib import Path
import subprocess,json,hashlib,re,collections,copy,zipfile,datetime
ROOT=Path('/mnt/development/projects/evidence/cherri-p10r15-k11-frozen-ci-20261123/linux-terminal-intake-01')
REPO=Path('/mnt/development/projects/worktrees/cherri-p10r15-505-refresh-20261123')
SHA='9b4135e705b08f765b7304e01212d3b9658806fb'
RUN='45108200176'
def digest(b):return hashlib.sha256(b).hexdigest()
def strict_pairs(pairs):
 d={}
 for k,v in pairs:
  if k in d:raise ValueError('duplicate JSON key: '+k)
  d[k]=v
 return d
def decode(b):return json.loads(b.decode('utf-8-sig'),object_pairs_hook=strict_pairs)
def read(n):return decode((ROOT/n).read_bytes())
def source(n):return subprocess.run(['git','--no-replace-objects','show',SHA+':'+n],cwd=REPO,capture_output=True,check=True).stdout
def write(n,o):
 p=ROOT/n
 assert not p.exists(),n
 p.write_text(json.dumps(o,indent=2,ensure_ascii=False)+'\n')
assert subprocess.run(['git','rev-parse','HEAD'],cwd=REPO,text=True,capture_output=True,check=True).stdout.strip()=='14a7440e839afdceca56a54c608f294e1d01073a'
assert not subprocess.run(['git','status','--porcelain'],cwd=REPO,text=True,capture_output=True,check=True).stdout
raw=(ROOT/'job-137474256318.original.log').read_bytes()
assert digest(raw)=='053d63d6166885c2d2904024bc3963aa0f4d270f3c022c213fdc534528ac7010'
lines=raw.decode('utf-8-sig').splitlines()
clean=[re.sub(r'^\d{4}-\d\d-\d\dT\S+Z ?','',x).lstrip('\ufeff') for x in lines]
starts=[i for i,x in enumerate(clean) if x.startswith('GO_SUITE_EXECUTE ')]
assert starts==[1157,25146]
end=next(i for i,x in enumerate(clean) if i>starts[-1] and '##[group]' in x)
assert end==37375
suite=read('artifacts/14790368224/cherri-full-go-suite.json')
assert suite['source_sha']==SHA and suite['source_status_before']==suite['source_status_after']==''
assert suite['success'] is True and suite['all_packages_executed_once'] is True
assert len(suite['all_packages'])==len(set(suite['all_packages']))==30
assert collections.Counter(x for g in suite['invocations'] for x in g['packages'])==collections.Counter(suite['all_packages'])
phases=[]
for name,a,z,inv in zip(['ordinary','acceptance'],starts,[starts[1],end],suite['invocations']):
 printed=json.loads(clean[a].split(' ',1)[1])
 assert printed=={k:inv[k] for k in ('packages','timeout','argv')}
 assert inv['exit_code']==0
 ev=[];pkgs=[]
 for i,line in enumerate(clean[a:z],a):
  m=re.match(r'^\s*\ufeff?--- (PASS|FAIL|SKIP): (\S+) \(([^)]*)\)',line)
  if m:ev.append({'line':i+1,'status':m[1],'test':m[2],'durationText':m[3]})
  m=re.match(r'^(ok|\?|FAIL)\s+(github\.com/\S+)\s+(.+)$',line)
  if m:pkgs.append({'line':i+1,'status':m[1],'package':m[2],'detail':m[3]})
 assert len(ev)==len({x['test'] for x in ev})
 assert not [x for x in ev if x['status']=='FAIL']
 assert collections.Counter(x['package'] for x in pkgs)==collections.Counter(inv['packages'])
 skips=[{**x,'originalReason':clean[x['line']-2]} for x in ev if x['status']=='SKIP']
 phases.append({'name':name,'logLinesInclusive':[a+1,z],'counts':dict(collections.Counter(x['status'] for x in ev)),'parentCounts':dict(collections.Counter(x['status'] for x in ev if '/' not in x['test'])),'subcaseCounts':dict(collections.Counter(x['status'] for x in ev if '/' in x['test'])),'uniqueNamedEvents':len(ev),'skipped':skips,'packageSummaries':pkgs,'originalInvocation':inv})
artifact_meta=read('artifacts.original.json')
archives=read('artifact-download-intake.json')
for a in archives:
 data=(ROOT/(str(a['id'])+'.original.zip')).read_bytes()
 meta=next(x for x in artifact_meta['artifacts'] if x['id']==a['id'])
 assert digest(data)==a['archiveSha256']==meta['digest'].removeprefix('sha256:')
 assert meta['workflow_run']['id']==int(RUN) and meta['workflow_run']['head_sha']==SHA
 with zipfile.ZipFile(ROOT/(str(a['id'])+'.original.zip')) as z:
  assert z.testzip() is None
  assert set(z.namelist())=={x['path'] for x in a['members']}
  for m in a['members']:
   b=z.read(m['path']); p=ROOT/'artifacts'/str(a['id'])/m['path']
   assert b==p.read_bytes() and len(b)==m['bytes'] and digest(b)==m['sha256']
encdir=ROOT/'artifacts/14789316187'
gate=decode((encdir/'required-encoding-gate.json').read_bytes())
assert gate['implementationSha']==SHA and gate['runId']==RUN and gate['status']=='PASS'
assert gate['appleEngineExecuted'] is True and gate['shortcutRuntimeExecuted'] is False and gate['mockEngineAllowed'] is False
assert gate['inputBytesUnchanged'] is True and gate['requiredCases']==23 and gate['expectedRejections']==1
canon_path='tools/shortcut-runtime/required-encoding-expectations.json'
canon=decode(source(canon_path)); required=canon['REQUIRED'];assert len(required)==23
forms={'string','number','bool','attachment(variable)','attachment(output)','tokenString','wrappedSubject'}
negid='plainString.WFStringParameterState.string'
phase_checks=[]
def actual(v):
 if v=='roundtrip' or v.startswith('accepted, reserialized as ') and len(v)>len('accepted, reserialized as '):return 'accept'
 if v=='rejected':return 'reject'
 if v in ('class not present','threw: native parameter-state exception'):return 'unavailable'
 raise ValueError('unrecognized actual observation')
for idx,name in enumerate(['positive','corrupt','restored']):
 phase=gate['phases'][idx];proc=decode((encdir/(name+'.process.json')).read_bytes())
 assert proc==phase and proc['phase']==name and proc['exitCode']==(1 if idx==1 else 0)
 for suffix,key in [('.json','reportSha256'),('.stdout.log','stdoutSha256'),('.stderr.log','stderrSha256')]:
  assert digest((encdir/(name+suffix)).read_bytes())==proc[key]
 command=proc['command']; assert len(command)==(9 if idx==1 else 8) and command[:3]==['/usr/bin/osascript','-l','JavaScript']
 assert Path(command[3]).name=='verify.js' and Path(command[6]).name==name+'.json'
 assert [Path(command[k]).name for k in [4,5,7]]==['parameter-encodings.json','encoding-table.json','required-encoding-expectations.json']
 assert Path(command[3]).parent==Path(command[6]).parent
 if idx==1:assert Path(command[8]).name=='isolated-corrupt-expectations.json' and Path(command[8]).parent==Path(command[3]).parent
 if idx:
  assert all(command[k]==gate['phases'][0]['command'][k] for k in [0,1,2,3,4,5,7])
 stderr=(encdir/(name+'.stderr.log')).read_bytes()
 diagnostic=b'E_REQUIRED_ENCODING_EXPECTATION' in stderr and negid.encode() in stderr
 assert diagnostic==(idx==1)==proc['negativeDiagnosticObserved']
 report=decode((encdir/(name+'.json')).read_bytes())
 assert report['schemaVersion']==2 and report['engineLoaded'] is True and report['engine']=='WorkflowKit parameter-state API'
 assert len(report['forms'])==7 and set(report['forms'])==forms
 results=report['results'];discovery=report['discovery']
 assert discovery['parameterStateClasses']>0 and discovery['appIntentStateClasses']>0 and discovery['uniqueStateClasses']==len(results)
 for state,row in results.items():
  assert re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*',state)
  if row=='class not present':continue
  assert isinstance(row,dict) and set(row)==forms
  for v in row.values():assert v!='class not present' and actual(v) in ('accept','reject','unavailable')
 expected=copy.deepcopy(required)
 if idx==1:next(x for x in expected if x['id']==negid)['expect']='reject'
 assert report['requiredExpectations']==expected and len(report['requiredResults'])==23
 failure=[]
 for exp,obs in zip(expected,report['requiredResults']):
  assert len(exp)==5 and len(obs)==8
  assert all(obs[k]==exp[k] for k in ('id','kind','stateClass','form'))
  observation=results[exp['stateClass']][exp['form']]
  outcome=actual(observation);passed=outcome==exp['expect']
  assert obs['observation']==observation and obs['actual']==outcome and obs['expected']==exp['expect'] and obs['passed'] is passed
  if not passed:failure.append({'code':'E_REQUIRED_ENCODING_ENGINE' if outcome=='unavailable' else 'E_REQUIRED_ENCODING_EXPECTATION','id':exp['id'],'expected':exp['expect'],'actual':outcome})
 assert report['failures']==failure
 assert report['status']==('FAIL' if idx==1 else 'PASS')
 assert failure==([{'code':'E_REQUIRED_ENCODING_EXPECTATION','id':negid,'expected':'reject','actual':'accept'}] if idx==1 else [])
 phase_checks.append({'phase':name,'exitCode':proc['exitCode'],'requiredCases':23,'passedRequiredCases':sum(x['passed'] for x in report['requiredResults']),'exactFailures':failure,'matrixStateClasses':len(results),'matrixForms':7,'allProcessAndPerFormBindingsVerified':True})
assert (encdir/'positive.json').read_bytes()==(encdir/'restored.json').read_bytes()
corrupt=copy.deepcopy(canon);next(x for x in corrupt['REQUIRED'] if x['id']==negid)['expect']='reject'
assert decode((encdir/'isolated-corrupt-expectations.json').read_bytes())==corrupt
records=decode((encdir/'required-encoding-evidence.json').read_bytes())['records']
assert len(records)==2
for record,req in zip(records,['CI01','RP47']):
 assert record['requirements']==[req] and record['tier']=='negative-ci' and record['implementation_sha']==SHA
 assert record['test_id']=='tools/shortcut-runtime:required-encoding-'+req.lower()
 assert record['fixture_sha256']==digest(source(canon_path))
 assert len(record['artifacts'])==13 and len({Path(x['path']).name for x in record['artifacts']})==13
 for a in record['artifacts']:
  assert a['run_id']==RUN and digest((encdir/Path(a['path']).name).read_bytes())==a['sha256']
source_bindings=[]
for p in ['.github/workflows/build-test.yml','tools/p10r15/run_full_go_suite.py','tools/language-acceptance/main.go','tools/language-acceptance/native_encoding_execution.go','tools/language-acceptance/native_encoding_intake.go','tools/p10r3/run_release_gates.py','tools/p10r3/generate_catalog_and_pack_ledgers.py']:
 raw_source=source(p);source_bindings.append({'path':p,'commit':SHA,'bytes':len(raw_source),'sha256':digest(raw_source)})
external=Path('/mnt/development/projects/evidence/cherri-p10r15-k10-frozen-ci-20261123/required-encoding-gate-intake-01/pinned-jxa-prelude.original.js')
for path,h in gate['inputs'].items():
 if '/cherri/cherri/' in path:
  p=path.split('/cherri/cherri/',1)[1];raw_source=source(p);assert digest(raw_source)==h
  source_bindings.append({'path':p,'commit':SHA,'bytes':len(raw_source),'sha256':h,'gateInputMatches':True})
 else:
  assert path.endswith('/required-encoding-shortcutkit/tools/jxa-prelude.js')
  assert digest(external.read_bytes())==h
  source_bindings.append({'path':'tools/jxa-prelude.js','repository':'frontboat/shortcutkit','commit':gate['preludeCommit'],'bytes':external.stat().st_size,'sha256':h,'reusedOriginal':str(external),'gateInputMatches':True})
job=read('job-137474256318.original.json')
assert job['id']==137474256318 and job['run_id']==int(RUN) and job['head_sha']==SHA and job['conclusion']=='failure'
failed_steps=[x for x in job['steps'] if x['conclusion']=='failure'];assert len(failed_steps)==1 and failed_steps[0]['name']=='P10 SSoT, Coverage Ledgers, Pack & Release Gates'
go_step=next(x for x in job['steps'] if x['name']=='Full Go Test Suite');assert go_step['conclusion']=='success'
drift=[{'line':i+1,'text':x} for i,x in enumerate(clean) if x.startswith('ERROR: inputDependencyFingerprint drift')]
assert len(drift)==4
missing=[{'line':i+1,'text':x} for i,x in enumerate(clean) if i>=38790 and ('No files were found' in x or 'No artifacts will be uploaded' in x)]
intake={
 'schemaVersion':1,'kind':'READ_ONLY_ORIGINAL_INTAKE','observedUtc':datetime.datetime.now(datetime.timezone.utc).isoformat(),
 'sourceCommit':SHA,'runId':RUN,'linuxJobId':str(job['id']),'newProducerExecutions':0,'newWorkflowDispatches':0,'newRemotePollers':0,
 'linuxJobConclusion':'failure','actualFailure':{'step':failed_steps[0],'stage':'--verify-commit','fingerprintDrift':drift,'actualRunAllExecuted':False,'actualR23ExecutedInThisJob':False,'gateArtifactsMissing':missing,'registeredGatesVerified':56,'coverageAndGateFixtureParentTestsPassed':22,'coverageAndGateFixtureElapsedSeconds':0.848},
 'fullGoSuite':{'originalReport':'artifacts/14790368224/cherri-full-go-suite.json','success':True,'sourceCleanBeforeAndAfter':True,'exactPackagesOnce':30,'packagesWithTests':25,'packagesWithoutTestFiles':5,'passEvents':16861,'skipEvents':11,'failEvents':0,'parentPassEvents':1667,'parentSkipEvents':11,'subcasePassEvents':15194,'originalStep':go_step,'phases':phases,'timingQualification':'Original monotonic invocation times674.641s and1346.726s (sum2021.367s) differ from GitHub step wall time3479s and log delivery timestamps. Original report observed_utc04:23:56.228983Z precedes late log deliveries. Cause is not established; timings are kept separate without restamping. Go package elapsed root659.094s and acceptance1342.317s are independently retained.'},
 'requiredEncoding':{'artifactId':'14789316187','componentStatus':'PASS','componentPlatform':gate['host'],'sourceCommit':SHA,'runId':RUN,'phases':phase_checks,'positiveAndRestoredBytesIdentical':True,'all26FormalArtifactReferencesVerified':True,'allSixSourceInputHashesVerified':True,'formalRecordsProduced':['CI01:negative-ci','RP47:negative-ci'],'formalAcceptanceGranted':False,'formalIneligibility':'The same Build & Test workflow contains a terminal failed Linux job; it cannot conclude success. Existing main.go842-846 rejects non-success run evidence. This is genuine WorkflowKit parameter-state evidence only, not Shortcut runtime, BRG31, successful CI or later-source credit. No canonical Go intake was executed by this audit.'},
 'sourceBindings':'source-bindings.json','archiveBindings':'artifact-download-intake.json','originalsPreserved':True
}
write('source-bindings.json',source_bindings);write('intake.json',intake)
md='''# K11 Linux terminal and WorkflowKit component intake

Frozen source: `9b4135e705b08f765b7304e01212d3b9658806fb`. Build & Test run `45108200176`; Linux job `137474256318`.

## Actual Linux outcome

The original complete Go suite passed. All30 discovered packages ran exactly once:29 ordinary packages under the original15m per-package bound and the acceptance package under its original35m bound. The original receipt has clean source before and after both invocations.

BOM-aware, timestamp-stripped, section-bounded parsing of the original console log finds **16,861 PASS,11 SKIP,0 FAIL** named events. This consists of1,667 parent PASS,11 parent SKIP and15,194 subcase PASS. Ordinary:10,836 PASS/8 SKIP; acceptance:6,025 PASS/3 SKIP. There are25 tested package PASS summaries and5 packages with no test files. Exact skip identities and original reasons remain in `intake.json`; these are not acceptance credit.

Original monotonic invocation durations are674.641s ordinary and1346.726s acceptance. Root package reports659.094s; acceptance package reports1342.317s. GitHub records the overall Full Go step03:50:14–04:48:13Z (3479s). The receipt timestamp04:23:56.228983Z and late console delivery timestamps differ. This intake preserves these independent clocks and does not invent a cause.

The sole failed job step is **P10 SSoT, Coverage Ledgers, Pack & Release Gates**. Its22 fixture tests passed in0.848s and all56 registry entries verified. Then **`--verify-commit` failed on four stale dependency fingerprints**, recorded `e29b07449bd5f3758128bbef51466196a14f2179246b48ba1b3df58c75e784f0` versus actual `0c704ae9a25933ba3ece1e4017af0afe922e6b3899fa7886a57ad965b4ca3c97`. The shell stopped before `--run-all`. Thus **this job did not execute actual R23**. The separate known fixed-base R23 blocker is unchanged. The upload warning confirms no release-gate report or attestation was created.

## Actual WorkflowKit component

The source-bound `required-encoding-gate` archive14789316187 contains15 CRC-verified members. All advertised archive/member digests, six input hashes (including the previously retained pinned shortcutkit prelude), three process receipts/argv/streams, the complete per-form matrices, and26 formal-record artifact references were reconciled mechanically with original bytes.

Actual Darwin arm64 WorkflowKit results:23/23 positive required cases; one exact isolated EXPECT corruption correctly exits1 with its precise diagnostic;23/23 restored cases. Positive/restored reports are byte-identical. Canonical expected rejection remains present. This is actual parameter-state engine execution; Shortcut runtime execution is false.

Only CI01 andRP47 negative-ci records are produced. **No formal acceptance is granted**: the same Build & Test workflow contains a terminal failed Linux job and cannot satisfy the existing successful-run requirement (`tools/language-acceptance/main.go:842–846`). No BRG31, general native execution, later-candidate credit, or canonical Go intake is claimed.

## Preservation and method

Exactly one failed Linux job-log download and one download for each relevant original artifact were used. No test, compiler, gate, provider request or workflow was run. No additional remote polling was started. The intake reads local retained originals and exact frozen Git blobs; it does not modify those originals or frozen source. `intake.json`, `source-bindings.json`, and `artifact-download-intake.json` carry the detailed identities and qualifications.
'''
assert not (ROOT/'INTAKE.md').exists();(ROOT/'INTAKE.md').write_text(md)
manifest={'schemaVersion':1,'files':[{'path':str(p.relative_to(ROOT)),'bytes':p.stat().st_size,'sha256':digest(p.read_bytes())} for p in sorted(ROOT.rglob('*')) if p.is_file() and p.name!='manifest.json']}
write('manifest.json',manifest)
print(json.dumps({'intakeSha256':digest((ROOT/'intake.json').read_bytes()),'sourceBindingsSha256':digest((ROOT/'source-bindings.json').read_bytes()),'manifestSha256':digest((ROOT/'manifest.json').read_bytes()),'files':len(manifest['files']),'pass':16861,'skip':11,'fail':0,'actualFailure':'verify-commit fingerprint drift; run-all and R23 not reached','encoding':'component PASS only; formal acceptance not granted','ownedStatus':subprocess.run(['git','status','--porcelain'],cwd=REPO,text=True,capture_output=True,check=True).stdout},indent=2))
