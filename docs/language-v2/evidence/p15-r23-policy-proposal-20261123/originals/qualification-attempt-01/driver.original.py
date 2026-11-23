from pathlib import Path
import subprocess,hashlib,json,ast,datetime,os,time,sys
PROPOSAL=Path('/mnt/development/projects/evidence/cherri-p10r15-r23-policy-review-k11-20261123')
OWNED=Path('/mnt/development/projects/worktrees/cherri-p10r15-505-refresh-20261123')
BASE='9b4135e705b08f765b7304e01212d3b9658806fb'
HEAD='14a7440e839afdceca56a54c608f294e1d01073a'
ATTEMPT=PROPOSAL/'qualification-attempt-01'
PATCH_SHA='65b4faad9bfdafd5b0bea4383c38f010f8098845e4eafff1e92593f5c309fbca'
DRAFT_SHA='feeee0f607d170ccb9602ede2213871c4a0dfcf8e82547b5a381b828d52fa6c7'
def h(b):return hashlib.sha256(b).hexdigest()
def git(*args):return subprocess.run(['git','--no-replace-objects',*args],cwd=OWNED,capture_output=True,check=True,timeout=10).stdout
def guard():
 return {'head':git('rev-parse','HEAD').decode().strip(),'status':git('status','--porcelain').decode(),'tree':git('rev-parse','HEAD^{tree}').decode().strip()}
before=guard();assert before['head']==HEAD and before['status']==''
assert h((PROPOSAL/'R23-v2-proposal.patch.inactive').read_bytes())==PATCH_SHA
assert not ATTEMPT.exists();ATTEMPT.mkdir()
COPY=ATTEMPT/'source';TMP=ATTEMPT/'tmp'
COPY.mkdir();TMP.mkdir()
relative=[
 'tools/p10r3/test_coverage_and_gates.py',
 'tools/p10r3/run_release_gates.py',
 'tools/p10r3/generate_catalog_and_pack_ledgers.py',
 'tools/p10r3/verify_ledger_consistency.py',
 'tools/p10r3/verify_or_generate_pack.py',
 'docs/apple-runtime/p10/p10-505-catalog-coverage-matrix.json',
 'docs/apple-runtime/p10/coverage-evidence.schema.json',
 'docs/apple-runtime/p10/p10-apple-cherri-crosswalk.json',
 'docs/apple-runtime/p10/p10-pack-15-go-only-audit.json',
 'docs/apple-runtime/p10/device-acceptance-manifest.json',
 'docs/apple-runtime/p10/baseline-state.json',
 'docs/apple-runtime/p10/p10r2-acceptance-contract.json',
 'docs/apple-runtime/p10/r23-protected-state-policy-v2.json',
]
copied=[]
for path in relative:
 proposed=PROPOSAL/'proposed-files'/(path+'.inactive')
 if proposed.exists():
  data=proposed.read_bytes();origin={'kind':'INACTIVE_REVIEWED_PROPOSAL_REVISION_4','path':str(proposed),'patchSha256':PATCH_SHA}
 else:
  data=git('show',BASE+':'+path);assert data==(OWNED/path).read_bytes()
  origin={'kind':'FROZEN_GIT_BLOB','commit':BASE,'path':path}
 target=COPY/path;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(data)
 copied.append({'path':path,'bytes':len(data),'sha256':h(data),'origin':origin})
draft=COPY/'docs/apple-runtime/p10/r23-protected-state-policy-v2.json'
assert h(draft.read_bytes())==DRAFT_SHA
record=json.loads(draft.read_bytes())
assert record['proposalOnly'] is True and record['approval']['status']=='DRAFT_NOT_APPROVED'
assert all(record['approval'][x] is None for x in ('sourceReference','sourceText','sourceTextSha256','approvedAtUtc'))
assert not (COPY/'.git').exists()
test=(COPY/'tools/p10r3/test_coverage_and_gates.py').read_text()
original=git('show',BASE+':tools/p10r3/test_coverage_and_gates.py').decode()
def parents(t):return [n.name for n in ast.walk(ast.parse(t)) if isinstance(n,ast.FunctionDef) and n.name.startswith('test_')]
assert parents(test)==parents(original) and len(parents(test))==22
a=test.index('                def observe(expected, *, historical_changed, working_changed,\n')
z=test.index('\n                # New controls use the actual production shape:',a)
block=test[a:z].encode()
assert len(block)==3473 and h(block)=='63f878bd2aec9f75e1926d6e5a7d67bb1cbcba81ded5c8ffa22afb6791bcb64b'
command=['timeout','--signal=TERM','--kill-after=5s','60s','python3','-B','tools/p10r3/test_coverage_and_gates.py','TestP10CoverageAndGates.test_c5_06_r23_protected_ios_file_modified_fails_closed','-v']
environment={k:os.environ[k] for k in ('PATH','HOME','LANG','LC_ALL') if k in os.environ}
environment.update({'TMPDIR':str(TMP),'PYTHONDONTWRITEBYTECODE':'1','PYTHONUNBUFFERED':'1'})
plan={'schemaVersion':1,'scope':'ONE_SYNTHETIC_R23_UNITTEST_PARENT_ONLY','sourceCommit':BASE,'ownedHead':HEAD,'patchSha256':PATCH_SHA,'command':command,'cwd':str(COPY),'environmentNames':sorted(environment),'temporaryRoot':str(TMP),'timeoutSeconds':60,'termKillGraceSeconds':5,'copiedFiles':copied,'sourceBefore':before,'allParentIds':parents(test),'originalElevenBlockBytes':len(block),'originalElevenBlockSha256':h(block),'draftPolicySha256':DRAFT_SHA,'activePatchApplied':False,'productionR23Executed':False,'ownerApprovalCreated':False,'providerCalls':0}
(ATTEMPT/'launch-plan.original.json').write_text(json.dumps(plan,indent=2)+'\n')
started=datetime.datetime.now(datetime.timezone.utc).isoformat();monotonic=time.monotonic()
with (ATTEMPT/'stdout.original.log').open('xb') as out,(ATTEMPT/'stderr.original.log').open('xb') as err:
 proc=subprocess.run(command,cwd=COPY,env=environment,stdout=out,stderr=err,check=False,timeout=70)
duration=time.monotonic()-monotonic;finished=datetime.datetime.now(datetime.timezone.utc).isoformat()
after=guard()
copies_after=[{'path':x['path'],'bytes':(COPY/x['path']).stat().st_size,'sha256':h((COPY/x['path']).read_bytes())} for x in copied]
unchanged=all(y['bytes']==x['bytes'] and y['sha256']==x['sha256'] for x,y in zip(copied,copies_after))
result={'schemaVersion':1,'scope':'SYNTHETIC_FIXTURE_ONLY_NO_POLICY_AUTHORIZATION','startedUtc':started,'finishedUtc':finished,'durationSeconds':duration,'exitCode':proc.returncode,'timedOut':proc.returncode in (124,137),'command':command,'stdout':{'bytes':(ATTEMPT/'stdout.original.log').stat().st_size,'sha256':h((ATTEMPT/'stdout.original.log').read_bytes())},'stderr':{'bytes':(ATTEMPT/'stderr.original.log').stat().st_size,'sha256':h((ATTEMPT/'stderr.original.log').read_bytes())},'sourceBefore':before,'sourceAfter':after,'sourceUnchanged':before==after,'copyFilesUnchanged':unchanged,'productionDraftUnchanged':h(draft.read_bytes())==DRAFT_SHA,'copiedFilesAfter':copies_after,'remainingTemporaryEntries':[str(x.relative_to(TMP)) for x in sorted(TMP.iterdir())],'activePatchApplied':False,'productionR23Executed':False,'ownerApprovalCreated':False}
(ATTEMPT/'result.original.json').write_text(json.dumps(result,indent=2)+'\n')
manifest={'files':[{'path':str(x.relative_to(ATTEMPT)),'bytes':x.stat().st_size,'sha256':h(x.read_bytes())} for x in sorted(ATTEMPT.rglob('*')) if x.is_file()]}
(ATTEMPT/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
print(json.dumps({'exitCode':proc.returncode,'durationSeconds':duration,'sourceUnchanged':before==after,'copyFilesUnchanged':unchanged,'productionDraftUnchanged':result['productionDraftUnchanged'],'resultSha256':h((ATTEMPT/'result.original.json').read_bytes()),'manifestSha256':h((ATTEMPT/'manifest.json').read_bytes()),'artifactRoot':str(ATTEMPT)},indent=2))
print('STDOUT\n'+(ATTEMPT/'stdout.original.log').read_text(errors='replace')[:8000])
print('STDERR\n'+(ATTEMPT/'stderr.original.log').read_text(errors='replace')[:14000])
