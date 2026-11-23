from pathlib import Path
import re,json,hashlib,collections,copy,datetime
root=Path('/mnt/development/projects/evidence/cherri-p10r15-k11-frozen-ci-20261123/linux-terminal-intake-01')
raw=(root/'job-137474256318.original.log').read_bytes()
assert hashlib.sha256(raw).hexdigest()=='053d63d6166885c2d2904024bc3963aa0f4d270f3c022c213fdc534528ac7010'
original_lines=raw.decode('utf-8-sig').splitlines()
# GitHub may insert a BOM before a timestamp, as well as before message text.
# Remove only boundary BOMs; preserve message contents and original raw log bytes.
lines=[re.sub(r'^\d{4}-\d\d-\d\dT\S+Z ?','',line.lstrip('\ufeff')).lstrip('\ufeff') for line in original_lines]
starts=[i for i,line in enumerate(lines) if line.startswith('GO_SUITE_EXECUTE ')]
assert starts==[1157,25146]
end=next(i for i,line in enumerate(lines) if i>starts[-1] and '##[group]' in line)
assert end==37375
initial=json.loads((root/'intake.json').read_text())
assert hashlib.sha256((root/'intake.json').read_bytes()).hexdigest()=='8478f0848c01a5a3f34e23621c48882d2f268702a6f0362dd377e769df978a3b'
corrected=copy.deepcopy(initial)
phases=[]
for label,a,z in zip(['ordinary','acceptance'],starts,[starts[1],end]):
 events=[]
 for i,line in enumerate(lines[a:z],a):
  match=re.match(r'^\s*\ufeff?--- (PASS|FAIL|SKIP): (\S+) \(([^)]*)\)',line)
  if match:events.append({'line':i+1,'status':match[1],'test':match[2],'durationText':match[3]})
 assert len(events)==len({e['test'] for e in events})
 counts=dict(collections.Counter(e['status'] for e in events))
 parent_counts=dict(collections.Counter(e['status'] for e in events if '/' not in e['test']))
 subcase_counts=dict(collections.Counter(e['status'] for e in events if '/' in e['test']))
 assert not any(e['status']=='FAIL' for e in events)
 phases.append({'name':label,'logLinesInclusive':[a+1,z],'counts':counts,'parentCounts':parent_counts,'subcaseCounts':subcase_counts,'uniqueNamedEvents':len(events)})
 target=next(x for x in corrected['fullGoSuite']['phases'] if x['name']==label)
 for key in ('counts','parentCounts','subcaseCounts','uniqueNamedEvents'):target[key]=phases[-1][key]
expected=[('ordinary',10837,8,1426,9411),('acceptance',6026,3,243,5783)]
for (name,passed,skipped,parents,subcases),phase in zip(expected,phases):
 assert phase['name']==name and phase['counts']=={'PASS':passed,'SKIP':skipped}
 assert phase['parentCounts']=={'PASS':parents,'SKIP':skipped} and phase['subcaseCounts']=={'PASS':subcases}
missed=[]
for i,line in enumerate(original_lines):
 if line.startswith('\ufeff'):
  clean=lines[i]
  if re.match(r'^--- PASS:',clean) and starts[0]<=i<end:
   missed.append({'line':i+1,'originalLine':line,'originalLineUtf8Sha256':hashlib.sha256(line.encode()).hexdigest(),'normalizedText':clean})
assert [x['line'] for x in missed]==[21089,25570]
receipt={'schemaVersion':1,'kind':'DERIVED_OBSERVER_COUNT_CORRECTION_NO_EXECUTION','observedUtc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'sourceCommit':initial['sourceCommit'],'runId':initial['runId'],'jobId':initial['linuxJobId'],'originalLogSha256':hashlib.sha256(raw).hexdigest(),'initialIntakeSha256':hashlib.sha256((root/'intake.json').read_bytes()).hexdigest(),'initialAuditCodeSha256':hashlib.sha256((root/'intake-audit.original.py').read_bytes()).hexdigest(),'cause':'Initial parser removed timestamp before stripping a boundary BOM, so two BOM-prefixed timestamp lines retained their timestamp and did not match the anchored PASS pattern. This is an intake observer defect, not an execution failure.','correction':'Strip a leading BOM before timestamp removal and again at the message boundary; retain the same anchored PASS/SKIP/FAIL regex and original section bounds.','omittedOriginalPassParents':missed,'phases':phases,'correctedTotals':{'PASS':16863,'SKIP':11,'FAIL':0,'parentPASS':1669,'parentSKIP':11,'subcasePASS':15194},'unchangedFindings':['All30 packages executed once','No actual Go failure in Linux suite','Exact11 skip identities and reasons','Original process durations/source identities','Actual job failed --verify-commit before --run-all/R23','WorkflowKit component evidence remains formally ineligible'],'originalExecutionOrIntakeFilesModified':False,'newProducerExecutions':0}
corrected['fullGoSuite'].update({'passEvents':16863,'parentPassEvents':1669})
corrected['derivedCountCorrection']={'receipt':'bom-count-correction.json','initialIntakePreserved':'intake.json','newProducerExecutions':0}
for name,obj in [('bom-count-correction.json',receipt),('intake.corrected.json',corrected)]:
 path=root/name;assert not path.exists();path.write_text(json.dumps(obj,indent=2,ensure_ascii=False)+'\n')
print(json.dumps({'receiptSha256':hashlib.sha256((root/'bom-count-correction.json').read_bytes()).hexdigest(),'correctedIntakeSha256':hashlib.sha256((root/'intake.corrected.json').read_bytes()).hexdigest(),'correctedCounts':receipt['correctedTotals'],'omittedOriginalLines':[x['line'] for x in missed]},indent=2))
