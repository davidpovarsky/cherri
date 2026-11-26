import os,sys,pathlib,json,hashlib,time,signal,datetime,re,ast,zipfile,stat,difflib
B=pathlib.Path('/var/tmp/cherri-xcresult-retention-20261126')
W=pathlib.Path('/var/tmp/cherri-xcresult-retention-source-20261126')
Q=B/'qualification-intake-01';Q.mkdir(mode=0o700,exist_ok=False)
os.umask(0o077);sys.dont_write_bytecode=True
pins=json.loads("[{\"bytes\":40602,\"path\":\"/var/cache/cherri-ai-darwin-hosted-targeted3-20261126/signal-control-fixture-02/fixed-capture/qa_terminal_receipts.py\",\"sha256\":\"cc7b8d3a6f3a8937d9603125f57800d195cac93ee0a50ef35c039c88215aa2be\"},{\"bytes\":21184,\"path\":\"/var/cache/cherri-ai-darwin-hosted-targeted3-20261126/signal-control-fixture-02/fixed-capture/prepare_public_artifacts.py\",\"sha256\":\"a1c575af90a111dd1828825fae049276650a38e5eac411e3e1b49b1662ba1043\"},{\"bytes\":20494,\"path\":\"/var/cache/cherri-ai-darwin-hosted-targeted3-20261126/signal-control-fixture-02/fixed-capture/storage_cross_run.py\",\"sha256\":\"6f527c5e22bcd23e220f5d21d09c217abff8859fad16fd743b68183e4ced20c9\"},{\"bytes\":22225,\"path\":\"/var/cache/cherri-ai-darwin-hosted-targeted3-20261126/signal-control-fixture-02/fixed-capture/xcode_destination_readiness.py\",\"sha256\":\"4b973bd979e3db7b24edb18599846f8a76661bbf5ed55fc758ad753ba7e98079\"},{\"bytes\":19880,\"path\":\"/var/cache/cherri-ai-darwin-hosted-targeted3-20261126/signal-control-fixture-02/fixed-capture/xcode_destination_diagnostic.py\",\"sha256\":\"adef5a99d7f7c41d297bbcbccabb12a229c07a7d326ecd4f97a76d52e81edf99\"},{\"bytes\":19628,\"path\":\"/var/cache/cherri-ai-darwin-hosted-targeted3-20261126/signal-control-fixture-02/fixed-capture/simulator_session_recovery.py\",\"sha256\":\"99f0cb92dc9ec32b2727f671f851053d53ae8e204b02d9405d0af85f188c230d\"},{\"bytes\":38742,\"path\":\"/var/cache/cherri-ai-darwin-hosted-targeted3-20261126/signal-control-fixture-02/fixed-capture/json_body_probe.py\",\"sha256\":\"afaf38f499bdcd4bc56be3a5b7958b96fb65f8eaa7ea22550248a57f678b1520\"},{\"bytes\":12641,\"path\":\"/var/cache/cherri-ai-darwin-hosted-targeted3-20261126/signal-control-fixture-02/fixed-capture/json_file_body_probe.py\",\"sha256\":\"6ecc10c7765b24957b94365e57bf81e0958136f3ba9629bb7866c98ab868c24f\"},{\"bytes\":36163,\"path\":\"/var/cache/cherri-ai-darwin-hosted-targeted3-20261126/signal-control-fixture-02/fixed-capture/retry_http_probe.py\",\"sha256\":\"8731f6b1fffcb4c3a84af21f6d60e0969026c67611e90bbf2789ecfb8ce03121\"},{\"bytes\":14611,\"path\":\"/var/cache/cherri-ai-darwin-hosted-targeted3-20261126/signal-control-fixture-02/fixed-capture/nested_list_runtime.py\",\"sha256\":\"1d78bc44abbd62b14f7af4a3272270736f37eb3a2c86bc43fa37f0d0ea179d8a\"},{\"bytes\":32947,\"path\":\"/var/cache/cherri-ai-darwin-hosted-targeted3-20261126/signal-control-fixture-02/fixed-capture/nested_list_probe.py\",\"sha256\":\"b21d24eecac940ab6decd9330015a49004ccdbbb7d21e5a3b42f1b15f878d925\"},{\"bytes\":15549,\"path\":\"/var/cache/cherri-ai-darwin-hosted-targeted3-20261126/signal-control-fixture-02/fixed-capture/tagged_carrier_runtime.py\",\"sha256\":\"34c13f516fe56ec1738860f6093a9af6772f309ec6938b68678aa1acbd3a0da3\"},{\"bytes\":6360,\"path\":\"/var/cache/cherri-ai-darwin-hosted-targeted3-20261126/signal-control-fixture-02/fixed-capture/tagged_carrier_observation.py\",\"sha256\":\"246225eb68cf80166e608186ff677afdd0ff80e3e4a39ddfc5777374852a6784\"},{\"bytes\":24075,\"path\":\"/var/cache/cherri-ai-darwin-hosted-targeted3-20261126/signal-control-fixture-02/fixed-capture/tagged_carrier_graph.py\",\"sha256\":\"69296dc7177b9ee8871410f9242fd5b50af56ef354193a47d247ae2d4cad58bf\"},{\"bytes\":9506,\"path\":\"/var/cache/cherri-ai-darwin-hosted-targeted3-20261126/signal-control-fixture-02/fixed-capture/native_comparison_graph.py\",\"sha256\":\"b25c055f7718f2b47a4f7615388dbc153f66ec788c8732200644a82a54bb860d\"},{\"bytes\":15837,\"path\":\"/var/cache/cherri-ai-darwin-hosted-targeted3-20261126/signal-control-fixture-02/fixed-capture/debug_label_preview.py\",\"sha256\":\"3ffe6e701c0cf6523ca8a0a3e32ea73777e3b3b3b25e31514162c085960b66cb\"},{\"bytes\":38850,\"path\":\"/var/cache/cherri-ai-darwin-hosted-targeted3-20261126/signal-control-fixture-02/fixed-capture/debug_label_acceptance.py\",\"sha256\":\"6cdc2685482131f2193f401aa7f99f63492506b82341eb8ec0ddb64830f422e5\"},{\"bytes\":4717,\"path\":\"/var/cache/cherri-ai-darwin-hosted-targeted3-20261126/signal-control-fixture-02/fixed-capture/project_prerequisite.py\",\"sha256\":\"6c0bed088dc53e73af138221ce72a6faff0dfa0bc0279cc254ae622dd54bd843\"},{\"bytes\":65318,\"path\":\"/var/cache/cherri-ai-darwin-hosted-targeted3-20261126/signal-control-fixture-02/fixed-capture/project_prerequisite_observation.py\",\"sha256\":\"49bcb0e3a0837cae1dc9b31dbac6f53bfa462e5608e6907cff3a1b11832b2cc5\"}]")
start=time.monotonic();first=None;checks=[]
result={'schema':'cherri-xctest-retention-original-intake/1','status':'FAILED','nativeInvocations':0,'testExecutions':0,'baselineCommitSha':'31d7b8d871618ff3eb82a65f0afd994d2c336d36','singleNineControlGreenRun':False}
def sha(b):return hashlib.sha256(b).hexdigest()
def save(p,x):
 b=x if isinstance(x,bytes) else (json.dumps(x,sort_keys=True,indent=2)+'\n').encode()
 with p.open('xb') as f:f.write(b);f.flush();os.fsync(f.fileno())
 return sha(b)
def check(condition,label):
 if not condition:raise AssertionError(label)
 checks.append(label)
class Stop(BaseException):pass
def stop(n,f):
 global first
 if first is None:first=n;signal.setitimer(signal.ITIMER_REAL,0);raise Stop(str(n))
for n in (signal.SIGALRM,signal.SIGTERM,signal.SIGINT):signal.signal(n,stop)
signal.setitimer(signal.ITIMER_REAL,70)
try:
 save(Q/'owner.original.py',pathlib.Path(__file__).read_bytes())
 for p in pins:
  b=pathlib.Path(p['path']).read_bytes();check(len(b)==p['bytes'] and sha(b)==p['sha256'],'helper:'+pathlib.Path(p['path']).name)
 sys.path.insert(0,str(pathlib.Path(pins[-1]['path']).parent))
 from project_prerequisite_observation import capture
 def command(name,argv,seconds=10):
  d=Q/name;d.mkdir(mode=0o700)
  save(d/'intent.original.json',{'argv':argv,'timeoutSeconds':seconds})
  def journal(e,x):save(d/(e+'.original.json'),x)
  with (d/'stdout.original').open('xb') as out,(d/'stderr.original').open('xb') as err:
   obs,details=capture(argv,seconds,output=out,error_output=err,hard_deadline=start+70,journal=journal)
  save(d/'process.original.json',{'observation':obs,'details':details})
  check(obs['exitCode']==0 and details['cleanupParentReaped'] and details['groupAfterCleanup']['state']=='ABSENT',name+':PASS_REAPED_ABSENT')
  return (d/'stdout.original').read_bytes()
 base=['/usr/sbin/runuser','-u','ubuntu','--','/usr/bin/git','-C',str(W)]
 check(command('git-head',base+['rev-parse','HEAD']).decode().strip()==result['baselineCommitSha'],'worktree baseline')
 shell=W/'scripts/ios27_shortcuts_simulator_qa.sh'
 test=W/'tools/shortcuts-simulator-qa/tests/test_xcresult_retention.py'
 check(sha(shell.read_bytes())=='b029c0bf1af37e38a8e47dfb1a7a3dab8fda0807260592c01b9fb13fd6dd231c','current production pin')
 check(sha(test.read_bytes())=='e474ff491d1b2d6db29daa921865c518b7efac4c9f45586e5a393e3088d03196','current test pin')
 command('bash-syntax',['/bin/bash','-n',str(shell)])
 changed=command('git-diff',base+['diff','--binary','--','scripts/ios27_shortcuts_simulator_qa.sh'])
 save(Q/'production.original.patch',changed)
 names=command('git-changed-paths',base+['diff','--name-only']).decode().splitlines()
 check(names==['scripts/ios27_shortcuts_simulator_qa.sh'],'one tracked production change')
 untracked=command('git-untracked',base+['ls-files','--others','--exclude-standard']).decode().splitlines()
 check(untracked==['tools/shortcuts-simulator-qa/tests/test_xcresult_retention.py'],'one additive control file')
 def methods(p):
  tree=ast.parse(p.read_text())
  c=next(n for n in tree.body if isinstance(n,ast.ClassDef) and n.name=='XCTestRetentionTests')
  return {n.name:ast.dump(n,include_attributes=False) for n in c.body if isinstance(n,ast.FunctionDef)}
 oldm=methods(B/'preparation-02/test_xcresult_retention.original.py');newm=methods(test)
 check(set(oldm)==set(newm),'same nine method identities and helpers')
 changed_methods=[n for n in oldm if oldm[n]!=newm[n]]
 check(changed_methods==['normal_cleanup'],'only extraction helper AST changed')
 tests=sorted(n for n in newm if n.startswith('test_'))
 check(len(tests)==9,'nine unique controls')
 unaffected=[
 'test_failed_exit_trap_preserves_partial_bundle_and_original_exit',
 'test_confirm_and_metadata_cleanup_retains_only_after_original_collection',
 'test_repeated_retention_keeps_old_objects_and_adds_new_confirmation',
 'test_original_closing_receipt_io_still_has_the_retention_deadline']
 for n in unaffected:
  check(oldm[n]==newm[n] and 'normal_cleanup' not in oldm[n],'unaffected control:'+n)
 records=[];allfiles=[]
 for qname in ['oracle-qualification-01','oracle-qualification-02','oracle-qualification-03']:
  q=B/qname
  for n in ['red-original','green-retention']:
   p=q/n;process=json.loads((p/'process.original.json').read_bytes());dt=process['details'];obs=process['observation']
   check(dt['cleanupParentReaped'] and dt['groupAfterCleanup']['state']=='ABSENT',qname+'/'+n+':cleanup')
   check(json.loads((p/'finished.original.json').read_bytes())==dt,qname+'/'+n+':finished')
   check(json.loads((p/'started.original.json').read_bytes())['pid']==dt['pid'],qname+'/'+n+':PID')
   for channel in ['stdout','stderr']:
    raw=(p/(channel+'.original')).read_bytes();r=dt[channel]
    check(r['bytes']==len(raw) and r['sha256']==sha(raw) and r['eof'] and not r['overflow'],qname+'/'+n+':'+channel)
   ev=json.loads((p/'events.original.json').read_bytes());term=[e for e in ev['events'] if e['event']!='start'];starts=[e['id'] for e in ev['events'] if e['event']=='start']
   check(len(starts)==ev['testsRun']==len(term) and starts==[e['id'] for e in term],qname+'/'+n+':event projection')
   records.append({'qualification':qname,'run':n,'events':ev,'process':obs,'processSHA256':sha((p/'process.original.json').read_bytes())})
 q2=json.loads((B/'oracle-qualification-02/green-retention/events.original.json').read_bytes())
 q3=json.loads((B/'oracle-qualification-03/green-retention/events.original.json').read_bytes())
 check(q2['testsRun']==9 and q2['failures']==1 and q2['errors']==q2['skips']==0,'retained 8PASS/1FAIL run')
 check(q3['testsRun']==5 and q3['failures']==q3['errors']==q3['skips']==0,'actual affected five PASS')
 join=[]
 for n in tests:
  qname='oracle-qualification-02' if n in unaffected else 'oracle-qualification-03'
  ev=q2 if n in unaffected else q3
  ident='retention_controls.XCTestRetentionTests.'+n
  check({'id':ident,'event':'PASS'} in ev['events'],'qualified named control:'+n)
  join.append({'id':ident,'original':qname+'/green-retention/events.original.json','sourceSHA256':'b029c0bf1af37e38a8e47dfb1a7a3dab8fda0807260592c01b9fb13fd6dd231c','methodASTSHA256':sha(newm[n].encode())})
 for qname in ['oracle-qualification-01','oracle-qualification-03']:
  check((B/qname/'sources.before.original.json').read_bytes()==(B/qname/'sources.after.original.json').read_bytes(),qname+':source closure')
 check(not (B/'oracle-qualification-02/sources.after.original.json').exists(),'failed owner closing source snapshot absent retained')
 before=(B/'preparation-01/shell.before.original.sh').read_bytes()
 check(sha(before)=='cd6abb28becfe3add4a380ff57ff26397e02b2f89dd2cb3db0de3c2d648c8fe1','original31d')
 cmds=re.findall(rb'run_cmd_timeout [0-9]+[^\n]*',before)
 check(len(cmds)==21 and cmds==re.findall(rb'run_cmd_timeout [0-9]+[^\n]*',shell.read_bytes()),'21 original native argv/bounds unchanged')
 for folder in ['preparation-01','preparation-02','preparation-03','oracle-qualification-01','oracle-qualification-02','oracle-qualification-03']:
  for p in sorted((B/folder).rglob('*')):
   if p.is_file():
    st=p.lstat();check(stat.S_ISREG(st.st_mode) and not p.is_symlink(),'regular original:'+str(p.relative_to(B)))
    b=p.read_bytes();allfiles.append({'path':str(p.relative_to(B)),'bytes':len(b),'sha256':sha(b),'mode':stat.S_IMODE(st.st_mode)})
 manifest={'schema':'cherri-retention-proof-members/1','members':allfiles}
 save(Q/'manifest.original.json',manifest)
 z=Q/'synthetic-originals.zip'
 with zipfile.ZipFile(z,'x',compression=zipfile.ZIP_DEFLATED,compresslevel=6) as archive:
  for row in allfiles:archive.write(B/row['path'],row['path'])
 with zipfile.ZipFile(z) as archive:
  check(sorted(archive.namelist())==sorted(x['path'] for x in allfiles),'ZIP member set')
  for row in allfiles:
   info=archive.getinfo(row['path']);raw=archive.read(info)
   check(len(raw)==row['bytes'] and sha(raw)==row['sha256'] and stat.S_IMODE(info.external_attr>>16)==row['mode'],'ZIP original:'+row['path'])
 save(Q/'qualified-identities.derived.json',join)
 result.update(status='NINE_CURRENT_CONTROLS_JOINED_FROM_ORIGINAL_RUNS',checks=len(checks),qualifiedControlCount=9,qualifiedIdentities=join,productionSHA256=sha(shell.read_bytes()),testSHA256=sha(test.read_bytes()),originalRuns=records,originalMemberCount=len(allfiles),originalMemberBytes=sum(x['bytes'] for x in allfiles),archive={'path':str(z),'bytes':z.stat().st_size,'sha256':sha(z.read_bytes())},failedOwner02Preserved=True,failedOwner02ClosingSourceSnapshotPresent=False,scope='Linux filesystem and actual shell control qualification; no XCTest or native authority',nativeCommandLinesUnchanged=21)
 save(Q/'checks.derived.json',checks)
except BaseException as e:result.update(errorType=type(e).__name__,error=str(e)[:1400],firstInterruption=first)
finally:
 signal.setitimer(signal.ITIMER_REAL,0);result['elapsedMs']=round((time.monotonic()-start)*1000);result['completedUTC']=datetime.datetime.now(datetime.timezone.utc).isoformat();h=save(Q/'result.original.json',result)
print(json.dumps({'path':str(Q/'result.original.json'),'sha256':h,'status':result['status'],'checks':result.get('checks'),'archive':result.get('archive'),'error':result.get('error')}))
