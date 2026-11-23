from pathlib import Path
import subprocess,json,hashlib,datetime,time,os
root=Path('/mnt/development/projects/diagnostics/cherri-private-s4-20261123')
proposal=root/'intake-s57b6d919-proposal-01'
adapter=proposal/'audit_new_source_s4.py'
run=root/'s57b6d919-replay-01'
evidence=root/'intake-s57b6d919-supervisor-01'
S='57b6d919757e1d0262993c6f11c6dc1cd17d6177'
EXPECTED={
'00-p12-par':'4b7b80d68dc180af104700dcb68c4e3d882ed42071332e17a697aaa60d0f0890',
'01-p13':'9b42f6e89ff1d39587abccd5b1a9c282e1f16911b5975f75f3bcca768d504db9',
'02-p10-measure':'bb5505197094f176c2d8f9eeb3671cdf4b827789b9df5497a51aeec313eac8c3',
'03-p11-fresh38x2':'7b16fa02e4b4eda8239244a21b8815715b85561da055b9b00cdeb44ca693f7a0',
'04-p12-lifecycle':'60bd69e3c764098dceb94f9e03c62d13333d2e4b8af7ca0aeb642ddab1e55aa6'}
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
assert os.getuid()==1001
assert sha(adapter)=='f186bc74fb355aa0334ca60696e387a2a55deea0ab5ffb0230a58285bc66ce77'
assert sha(proposal/'root-reviewed-handoff.original.json')=='8b9d1389e6a51bc6bdc9a55e32ec138eaa72b3660a70ba8897433d3153de2178'
for mode,digest in EXPECTED.items():assert sha(run/mode/'receipt.json')==digest
assert not (run/'independent-intake-01').exists()
evidence.mkdir(mode=0o700,exist_ok=False)
def put(n,v):
 with (evidence/n).open('x',encoding='utf-8') as f:f.write(json.dumps(v,indent=2)+'\n')
argv=['timeout','--signal=TERM','--kill-after=5s','60s','python3','-B',str(adapter),'--execute-reviewed-intake']
env={k:os.environ[k] for k in ('PATH','HOME','USER','LOGNAME','LANG','LC_ALL','LC_CTYPE','TZ') if k in os.environ}
env.update({'PYTHONDONTWRITEBYTECODE':'1','GIT_NO_REPLACE_OBJECTS':'1','GIT_OPTIONAL_LOCKS':'0','GIT_TERMINAL_PROMPT':'0'})
start=datetime.datetime.now(datetime.timezone.utc).isoformat();mono=time.monotonic()
put('intent.original.json',{'schema':'cherri-s4-independent-intake-supervisor-intent/2','source':S,'argv':argv,'uid':os.getuid(),'startedUTC':start,'adapterSHA256':sha(adapter),'rootHandoffSHA256':sha(proposal/'root-reviewed-handoff.original.json'),'completedProducerReceiptSHA256':EXPECTED,'privateHashPassAuthorized':1,'producerExecutionsAuthorized':0,'outerSeconds':60,'termKillGraceSeconds':5})
with (evidence/'stdout.original.bin').open('xb') as so,(evidence/'stderr.original.bin').open('xb') as se:
 p=subprocess.run(argv,cwd=proposal,stdin=subprocess.DEVNULL,stdout=so,stderr=se,env=env,start_new_session=True)
elapsed=time.monotonic()-mono
finished=datetime.datetime.now(datetime.timezone.utc).isoformat()
resultpath=run/'independent-intake-01/result.original.json'
j=json.loads(resultpath.read_text()) if resultpath.exists() else None
after={mode:sha(run/mode/'receipt.json') for mode in EXPECTED}
receipt={'schema':'cherri-s4-independent-intake-supervisor-result/2','source':S,'argv':argv,'startedUTC':start,'finishedUTC':finished,'elapsedSeconds':elapsed,'exitCode':p.returncode,'timedOut':p.returncode in (124,137),'adapterSHA256':sha(adapter),'adapterUnchanged':sha(adapter)=='f186bc74fb355aa0334ca60696e387a2a55deea0ab5ffb0230a58285bc66ce77','stdoutSHA256':sha(evidence/'stdout.original.bin'),'stderrSHA256':sha(evidence/'stderr.original.bin'),'intakeResultExists':resultpath.exists(),'intakeResultSHA256':sha(resultpath) if resultpath.exists() else None,'allFiveNewSourceParentsAccepted':j.get('allFiveNewSourceParentsAccepted') if j else False,'producerReceiptsUnchanged':after==EXPECTED,'producerExecutions':0,'automaticRetry':False}
put('result.original.json',receipt)
files=[{'path':q.name,'bytes':q.stat().st_size,'sha256':sha(q)} for q in sorted(evidence.iterdir()) if q.is_file()]
put('manifest.json',{'schema':'cherri-s4-independent-intake-supervisor-manifest/2','files':files})
print(json.dumps({'supervisor':receipt,'supervisorResultSHA256':sha(evidence/'result.original.json'),'manifestSHA256':sha(evidence/'manifest.json'),'fixedStdout':(evidence/'stdout.original.bin').read_text()[:3000],'stderrBytes':(evidence/'stderr.original.bin').stat().st_size},indent=2))
raise SystemExit(0 if p.returncode==0 and j and j.get('allFiveNewSourceParentsAccepted') else 1)
