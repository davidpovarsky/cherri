from pathlib import Path
import subprocess,re,json,hashlib,collections,datetime,zipfile
P=Path('/mnt/development/projects/evidence/cherri-p10r15-k11-frozen-ci-20261123/macos-terminal-intake-01')
REPO=Path('/mnt/development/projects/worktrees/cherri-p10r15-505-refresh-20261123')
SHA='9b4135e705b08f765b7304e01212d3b9658806fb'
def h(b):return hashlib.sha256(b).hexdigest()
def source(path):return subprocess.run(['git','--no-replace-objects','show',SHA+':'+path],cwd=REPO,capture_output=True,check=True,timeout=10).stdout
def pairs(items):
 d={}
 for k,v in items:
  if k in d:raise ValueError('duplicate JSON key')
  d[k]=v
 return d
def decode(data):return json.loads(data.decode('utf-8-sig'),object_pairs_hook=pairs)
def write(name,obj):
 f=P/name;assert not f.exists();f.write_text(json.dumps(obj,indent=2,ensure_ascii=False)+'\n')
before={'head':subprocess.run(['git','rev-parse','HEAD'],cwd=REPO,capture_output=True,text=True,check=True).stdout.strip(),'status':subprocess.run(['git','status','--porcelain'],cwd=REPO,capture_output=True,text=True,check=True).stdout}
assert before=={'head':'14a7440e839afdceca56a54c608f294e1d01073a','status':''}
raw=(P/'job-137474256647.original.log').read_bytes()
assert len(raw)==10576046 and h(raw)=='3938ab765044742110bee1e509162401902a5cfbcd8d4a9ab9ab5ed2d9e8c1c3'
original=raw.decode('utf-8-sig').splitlines()
# Strip a possible boundary BOM before the timestamp, and after it.
lines=[re.sub(r'^\d{4}-\d\d-\d\dT\S+Z ?','',x.lstrip('\ufeff')).lstrip('\ufeff') for x in original]
starts=[i for i,l in enumerate(lines) if l.startswith('GO_SUITE_EXECUTE ')]
assert starts==[1247,25302]
end=next(i for i,l in enumerate(lines) if i>starts[-1] and '##[group]' in l);assert end==37183
job=decode((P/'job-137474256647.from-parent-watch.json').read_bytes())
run=decode((P/'parent-watch-run.original.json').read_bytes())
artifacts=decode((P/'parent-watch-artifacts.original.json').read_bytes())
assert job['id']==137474256647 and job['head_sha']==SHA and job['conclusion']=='failure'
assert run['id']==45108200176 and run['head_sha']==SHA and run['conclusion']=='failure'
meta=next(a for a in artifacts['artifacts'] if a['id']==14790748782)
archive=(P/'14790748782.original.zip').read_bytes()
assert h(archive)==meta['digest'].removeprefix('sha256:')=='2f6ba0b9333ca9f7c1f2be4f3fd07b2986d511aa7d27dbea8164011b8f0cc48f'
with zipfile.ZipFile(P/'14790748782.original.zip') as z:
 assert z.testzip() is None and z.namelist()==['cherri-full-go-suite.json']
 report_bytes=z.read('cherri-full-go-suite.json')
assert report_bytes==(P/'artifacts/14790748782/cherri-full-go-suite.json').read_bytes()
assert h(report_bytes)=='50fcc1ef0b65efbd62d22b41ff10fc1a19b097648173c6188a7e3d2f1ae75712'
report=decode(report_bytes)
assert report['source_sha']==SHA and report['source_status_before']==report['source_status_after']==''
assert report['all_packages_executed_once'] is True and report['success'] is False
assert len(report['all_packages'])==len(set(report['all_packages']))==30
assert collections.Counter(p for invocation in report['invocations'] for p in invocation['packages'])==collections.Counter(report['all_packages'])
phases=[];all_events=[]
for name,a,z,invocation in zip(['ordinary','acceptance'],starts,[starts[1],end],report['invocations']):
 printed=json.loads(lines[a].split(' ',1)[1]);assert printed=={k:invocation[k] for k in ['packages','timeout','argv']}
 events=[];packages=[]
 for i,line in enumerate(lines[a:z],a):
  m=re.match(r'^\s*\ufeff?--- (PASS|FAIL|SKIP): (\S+) \(([0-9.]+)s\)',line)
  if m:events.append({'line':i+1,'status':m[1],'test':m[2],'seconds':float(m[3])})
  m=re.match(r'^(ok|FAIL|\?)\s+(github\.com/\S+)\s+(.+)$',line)
  if m:packages.append({'line':i+1,'status':m[1],'package':m[2],'detail':m[3]})
 assert len(events)==len({e['test'] for e in events})
 assert collections.Counter(p['package'] for p in packages)==collections.Counter(invocation['packages'])
 all_events+=events
 phases.append({'name':name,'logLinesInclusive':[a+1,z],'counts':dict(collections.Counter(e['status'] for e in events)),'parentCounts':dict(collections.Counter(e['status'] for e in events if '/' not in e['test'])),'subcaseCounts':dict(collections.Counter(e['status'] for e in events if '/' in e['test'])),'uniqueNamedEvents':len(events),'skipped':[dict(e,originalReason=lines[e['line']-2]) for e in events if e['status']=='SKIP'],'packageSummaries':packages,'originalInvocation':invocation})
assert phases[0]['counts']=={'PASS':10813,'SKIP':11}
assert phases[1]['counts']=={'PASS':5852,'SKIP':3}
panic=next(i for i,l in enumerate(lines) if l=='panic: test timed out after 15m0s')
root_end=next(i for i,l in enumerate(lines) if l.startswith('FAIL\tgithub.com/electrikmilk/cherri\t'))
assert panic==23779 and root_end==23893
(root_stack:=P/'timeout-stack.original.log').write_text('\n'.join(original[panic:root_end+1])+'\n')
root_runs=[];paused=[];continued=[];parent_results=[]
for i,line in enumerate(lines[starts[0]:root_end],starts[0]):
 m=re.match(r'^=== (RUN|PAUSE|CONT)\s+(\S+)',line)
 if m and '/' not in m[2]:
  {'RUN':root_runs,'PAUSE':paused,'CONT':continued}[m[1]].append(m[2])
 m=re.match(r'^--- (PASS|SKIP|FAIL): (\S+) \(([0-9.]+)s\)',line)
 if m:parent_results.append({'test':m[2],'status':m[1],'seconds':float(m[3]),'line':i+1})
assert len(root_runs)==len(set(root_runs))==1180
assert len(paused)==len(set(paused))==29 and set(continued)==set(paused) and len(continued)==29
uncompleted=sorted(set(root_runs)-{e['test'] for e in parent_results})
assert uncompleted==['TestP15CanonicalBehaviorSourcePropagation']
serial=[e for e in parent_results if e['test'] not in paused]
parallel=[e for e in parent_results if e['test'] in paused]
assert len(parallel)==28 and all(e['status']=='PASS' for e in parallel)
serial_seconds=round(sum(e['seconds'] for e in serial),2)
parallel_seconds=round(sum(e['seconds'] for e in parallel),2)
assert serial_seconds==474.85 and parallel_seconds==781.14
critical=['TestP15K04OfflineOracleRejectsBypasses','TestP15K04OfflineCacheProduction','TestP15CanonicalBehaviorSourcePropagation']
tail=[]
for i,line in enumerate(lines[starts[0]:root_end],starts[0]):
 if any(n in line for n in critical) and (re.match(r'^(===|\s*---|\s*TestP15)',line)) and len(line)<1000:
  tail.append({'line':i+1,'text':line})
canonical_phases=[]
for i,line in enumerate(lines[starts[0]:root_end],starts[0]):
 m=re.search(r'phase=([^ ]+) actual child Go JSON SHA256=([0-9a-f]{64}) emitted native tree SHA256=([0-9a-f]{64})',line)
 if m and 'p15_canonical_source_propagation_test.go' in line:canonical_phases.append({'line':i+1,'phase':m[1],'childGoJSONSha256':m[2],'nativeTreeSha256':m[3]})
assert [x['phase'] for x in canonical_phases]==['baseline','static','static-restored','callback','callback-restored']
assert canonical_phases[0]['nativeTreeSha256']==canonical_phases[2]['nativeTreeSha256']==canonical_phases[4]['nativeTreeSha256']
workload={'rootParentsStarted':1180,'rootParentResults':dict(collections.Counter(e['status'] for e in parent_results)),'pausedParents':29,'continuedParents':29,'completedParallelParents':28,'remainingNamedParent':uncompleted,'remainingQueuedParallelParents':[],'serialRoundedParentSecondsSum':serial_seconds,'completedParallelRoundedParentSecondsSum':parallel_seconds,'activeCanonicalDurationText':'59s','activeCodecDurationText':'2s','approximateWorkloadLowerBoundBeforeRemainingTailSeconds':round(serial_seconds+(parallel_seconds+59)/2,2),'qualification':'Rounded parent durations are workload evidence, not precise scheduling timestamps. The arithmetic is a lower bound under ideal two-slot packing and excludes the missing canonical tail; it does not predict a guaranteed successful schedule.','parallelParentDurations':parallel,'largestSerialParentDurations':sorted(serial,key=lambda e:-e['seconds'])[:20],'criticalOriginalEvents':tail,'canonicalCompletedPhaseHashReceipts':canonical_phases}
write('scheduler-workload-analysis.json',workload)
paths=['.github/workflows/build-test.yml','tools/p10r15/run_full_go_suite.py','p15_install_actual_tools_test.go','p15_skill_offline_recovery_test.go','p15_canonical_source_propagation_test.go','p15_schema_production_test.go','p15_schema_generation_recovery_test.go','p15_schema_identity_recovery_test.go','p15_schema_optional_propagation_test.go']
bindings=[]
for path in paths:
 r=subprocess.run(['git','--no-replace-objects','show',SHA+':'+path],cwd=REPO,capture_output=True,timeout=10)
 if r.returncode:
  assert path=='p15_skill_offline_recovery_test.go'
  continue
 bindings.append({'path':path,'sourceCommit':SHA,'bytes':len(r.stdout),'sha256':h(r.stdout)})
write('source-bindings.json',bindings)
full_step=next(s for s in job['steps'] if s['name']=='Full Go Test Suite')
assert full_step['conclusion']=='failure'
fail_steps=[s['name'] for s in job['steps'] if s['conclusion']=='failure'];assert fail_steps==['Full Go Test Suite']
intake={'schemaVersion':1,'kind':'READ_ONLY_ORIGINAL_TERMINAL_INTAKE','observedUtc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'sourceCommit':SHA,'runId':str(run['id']),'runAttempt':run['run_attempt'],'jobId':str(job['id']),'runConclusion':'failure','jobConclusion':'failure','originalLog':{'path':'job-137474256647.original.log','bytes':len(raw),'sha256':h(raw)},'actualFailure':{'kind':'ROOT_PACKAGE_CUMULATIVE_TIMEOUT','originalTimeout':'15m0s','rootPackageElapsedSeconds':900.153,'namedAssertionFailEvents':0,'activeParent':'TestP15CanonicalBehaviorSourcePropagation','activeParentDurationText':'59s','activeSubcase':'TestP15CanonicalBehaviorSourcePropagation/codec','activeSubcaseDurationText':'2s','stackPath':'p15_canonical_source_propagation_test.go:187 → p15CanonicalRebuildRead:22 → p15SchemaCommand:88 → exec.Cmd.CombinedOutput/Wait','specificChildTimeoutObserved':False,'deadlockEstablished':False,'originalChildLimits':{'goTestTimeoutSeconds':90,'p15SchemaCommandContextSeconds':120},'stackOriginal':'timeout-stack.original.log','originalWorkflowFailedStep':full_step},
 'fullSuite':{'success':False,'all30PackagesInvokedOnce':True,'sourceCleanBeforeAndAfter':True,'namedPassEvents':16665,'namedSkipEvents':14,'namedFailEvents':0,'parentPassEvents':1666,'parentSkipEvents':13,'subcasePassEvents':14999,'subcaseSkipEvents':1,'warning':'Zero named --- FAIL events is not suite success: the root package terminated by timeout and its canonical parent did not finish.','phases':phases,'originalReportObservedUtc':report['observed_utc'],'originalInvocationTotalSeconds':round(sum(x['elapsed_seconds'] for x in report['invocations']),3),'githubStepWallSeconds':2957,'clockQualification':'Original invocation monotonic times918.709s and1711.823s sum2630.532s; report observed_utc04:52:52.376203Z precedes late console delivery/step completion04:58:17Z. Cause of clock/delivery difference is unestablished. Each original time remains separately preserved.'},
 'k04Scheduling':{'originalTwoHooksObserved':True,'sameRootTwoSlotLimit':2,'parents':[e for e in parallel if e['test'] in critical[:2]],'allOriginalOracleSubcasesPassed':True,'claim':'Both K04 parents actually PAUSE/CONT and complete under the existing scheduler. Global15m qualification still fails. No controlled comparative speedup is claimed.'},
 'sourceLocalDiagnosis':{'canonicalAlreadyScheduled':True,'canonicalMutationState':'Private detached Git worktree; sequential static/callback/codec mutation and restoration; selected child go test only; five completed probe hash receipts, codec and final restoration incomplete at root timeout.','sevenActualProbesPreserved':'baseline + three changed + three restored; no safe omission identified','candidateOnly':'After independent isolation review, consider exactly two first-line existing hook calls for RP08 schema identity and optional declaration parents, whose measured serial durations are30.62s and21.07s. No edit, runtime validation or deadline gain is claimed here.','readOnlyWorkloadFile':'scheduler-workload-analysis.json'},
 'artifacts':{'fullSuiteArchiveId':'14790748782','archiveDigestVerified':True,'allMembersCRCVerified':True,'reportSha256':h(report_bytes),'encodingReuse':'../linux-terminal-intake-01/artifacts/14789316187; already intaken once, actual component PASS, formally ineligible because run45108200176 failed','editorCaptureArtifactId':'14790952964','editorCaptureDownloaded':False,'editorCaptureReason':'Not needed to diagnose this timeout; advertised original metadata retained. No editor/native acceptance is inferred.','releaseGateReportPresent':False,'actualReleaseGatesExecuted':False},
 'originalsPreserved':True,'newTestExecutions':0,'newGoExecutions':0,'newProviderCalls':0,'newWorkflowDispatches':0,'newRemotePollingLoops':0,'sourceBefore':before}
after={'head':subprocess.run(['git','rev-parse','HEAD'],cwd=REPO,capture_output=True,text=True,check=True).stdout.strip(),'status':subprocess.run(['git','status','--porcelain'],cwd=REPO,capture_output=True,text=True,check=True).stdout}
assert after==before;intake['sourceAfter']=after
write('intake.json',intake)
print(json.dumps({'intakeSha256':h((P/'intake.json').read_bytes()),'workloadSha256':h((P/'scheduler-workload-analysis.json').read_bytes()),'sourceBindingsSha256':h((P/'source-bindings.json').read_bytes()),'serialRoundedSeconds':serial_seconds,'parallelCompletedSeconds':parallel_seconds,'rootParentResults':workload['rootParentResults'],'remainingParents':uncompleted,'namedCounts':{'PASS':16665,'SKIP':14,'FAIL':0},'sourceUnchanged':after==before},indent=2))
