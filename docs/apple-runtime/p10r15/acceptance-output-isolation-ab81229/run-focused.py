from pathlib import Path
import subprocess, os, sys, json, hashlib, time, datetime, collections
root=Path(r"/home/chatgpt-agent/workspaces/cherri-p10r15-output-isolation-checks-20261119")
evidence=Path(r"/home/chatgpt-agent/workspaces/cherri-p10r15-output-isolation-evidence-20261119")
mode=sys.argv[1]
assert mode in ('baseline','fixed','clean')
main=root/'tools/language-acceptance/main.go'
patched=main.read_bytes()
if mode=='baseline': main.write_bytes((evidence/'baseline-main.go').read_bytes())
def git(*args):
    return subprocess.check_output(['git',*args],cwd=root).decode('utf-8').strip()
command=[r'/home/chatgpt-agent/workspaces/cherri-remote/tools/go/bin/go','test','-json','-count=1','-p=1','-timeout=3m','./tools/language-acceptance','-run',r'^(TestAcceptanceCLIOutputIsolationAndWriteFailure|TestAcceptanceCLIOutputNameCannotBypassFinalValidation|TestAcceptanceCLILocalRunnerOutputsFollowDestination)$']
env=os.environ.copy();env.update({'GOMAXPROCS':'2','GOFLAGS':'-p=1','PYTHONDONTWRITEBYTECODE':'1','CHERRI_P10R10_CORPUS':'0'});env.pop('CHERRI_P10R10_CORPUS_DIR',None)
env['PATH']='/home/chatgpt-agent/workspaces/cherri-remote/tools/go/bin:'+env.get('PATH','')
record={'mode':mode,'head':git('rev-parse','HEAD'),'status_before':git('status','--porcelain','--untracked-files=all'),'diff_sha256':hashlib.sha256(subprocess.check_output(['git','diff','--binary'],cwd=root)).hexdigest(),'started_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'command':command,'main_sha256':hashlib.sha256(main.read_bytes()).hexdigest(),'test_sha256':hashlib.sha256((root/'tools/language-acceptance/output_isolation_test.go').read_bytes()).hexdigest()}
if mode=='clean': assert record['status_before']=='',record['status_before']
start=time.monotonic();raw=evidence/(mode+'.jsonl');counts=collections.Counter();last={};failures={};tests=set()
try:
    with raw.open('w',encoding='utf-8',newline='\n') as output:
        proc=subprocess.Popen(command,cwd=root,env=env,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True,encoding='utf-8',errors='replace')
        for line in proc.stdout:
            output.write(line);output.flush()
            try: event=json.loads(line)
            except ValueError: continue
            name=event.get('Test','');action=event.get('Action')
            if name:
                if action=='run': tests.add(name)
                if action in ('run','pass','fail','skip'): counts[action]+=1
                if action=='output': last[name]=(last.get(name,'')+event.get('Output',''))[-9000:]
                if action in ('fail','skip'): failures[name]={'status':action,'detail':last.get(name,'')}
        code=proc.wait()
    record.update({'exit_code':code,'elapsed_seconds':round(time.monotonic()-start,3),'counts':dict(counts),'discovered_unique_tests':len(tests),'failures_or_skips':failures,'raw_jsonl_sha256':hashlib.sha256(raw.read_bytes()).hexdigest(),'raw_jsonl_bytes':raw.stat().st_size,'status_after':git('status','--porcelain','--untracked-files=all')})
    (evidence/(mode+'-receipt.json')).write_text(json.dumps(record,indent=2)+'\n',encoding='utf-8')
    print(json.dumps({k:v for k,v in record.items() if k!='failures_or_skips'},indent=2))
    print('Failure/skip test names:',list(failures))
finally:
    if mode=='baseline': main.write_bytes(patched)
required={'TestAcceptanceCLIOutputIsolationAndWriteFailure','TestAcceptanceCLIOutputNameCannotBypassFinalValidation','TestAcceptanceCLILocalRunnerOutputsFollowDestination'}
if not required.issubset(tests): raise SystemExit('Missing required actual test discovery: '+str(sorted(required-tests)))
if mode=='baseline' and 'TestAcceptanceCLILocalRunnerOutputsFollowDestination' not in failures: raise SystemExit('Missing expected causal output-routing failure')
if mode!='baseline' and code!=0: sys.exit(code)
if mode=='baseline' and code==0: raise SystemExit('Causal baseline unexpectedly passed')
