import pathlib,json,hashlib,zipfile,io,subprocess,datetime,re,stat,collections
B=pathlib.Path('/mnt/development/projects/evidence/cherri-p10r15-k11-frozen-ci-20261123')
R=pathlib.Path('/dev/shm/cherri-p10r15-runtime-20261123/cherri-k11-source')
D=pathlib.Path('/mnt/development/projects/worktrees/cherrilang-p10r15-d11-20261123')
O=B/'d03-consumers/independent-intake-01'
K='9b4135e705b08f765b7304e01212d3b9658806fb'
S='ca316a3dee252456c4bec751594cf249c709b331'
DS='365b08c5b4c71c6d22b591babb8d1ab852e5a3b9'
sha=lambda b:hashlib.sha256(b).hexdigest()
checks=[]
def check(name,value):
 checks.append({'check':name,'passed':bool(value)})
 if not value:raise ValueError(name)
def unique(raw):
 def pairs(items):
  x={}
  for k,v in items:
   if k in x:raise ValueError('duplicate JSON key')
   x[k]=v
  return x
 return json.loads(raw,object_pairs_hook=pairs)
def blob(repo,commit,path):
 return subprocess.check_output(['git','--no-replace-objects','-C',str(repo),'show',commit+':'+path],timeout=15)
def utc(s):return datetime.datetime.fromisoformat(s.replace('Z','+00:00'))
def safe_zip(raw,expected=None):
 z=zipfile.ZipFile(io.BytesIO(raw));infos=z.infolist();names=[i.filename for i in infos]
 check('zip unique safe regular members '+str(expected),len(names)==len(set(names)) and all(not n.startswith('/') and '..' not in pathlib.PurePosixPath(n).parts and '\\' not in n and not i.is_dir() and not stat.S_ISLNK(i.external_attr>>16) for n,i in zip(names,infos)))
 check('zip original CRC '+str(expected),z.testzip() is None)
 if expected is not None:check('zip member count '+str(expected),len(names)==expected)
 return z
archive=(B/'d03-consumers/14788322055.original.zip').read_bytes()
check('original archive digest and size',len(archive)==248340 and sha(archive)=='77fc47a73668c407b12856f95ff18a3243c55d290e67a89383ab683965f839e8')
z=safe_zip(archive,50)
data={n:z.read(n) for n in z.namelist()}
manifest=unique((B/'d03-consumers/members.json').read_bytes())
check('50 independent member inventory exact',len(manifest)==50 and {x['path'] for x in manifest}==set(data))
for x in manifest:check('outer member '+x['path'],len(data[x['path']])==x['bytes'] and sha(data[x['path']])==x['sha256'])
run=unique((B/'watch/45108346953/run.original.json').read_bytes())
jobs=unique((B/'watch/45108346953/jobs.original.json').read_bytes())['jobs']
check('actual exact K11 workflow terminal success',run['id']==45108346953 and run['head_sha']==K and run['event']=='workflow_dispatch' and run['path']=='.github/workflows/p10r15-documentation-consumers.yml' and run['status']=='completed' and run['conclusion']=='success' and run['run_attempt']==1)
check('actual installed-public job terminal success',len(jobs)==1 and jobs[0]['id']==137474720360 and jobs[0]['head_sha']==K and jobs[0]['status']=='completed' and jobs[0]['conclusion']=='success')
job=jobs[0]
required_steps=['Package and install the exact committed Skill outside its source tree','Collect actual installed searches and exact live documentation bytes','Validate actual collector bytes with existing Go intake in this workspace','Verify original Pages ZIP and current deployment metadata']
for n in required_steps:check('actual job step '+n,sum(s['name']==n and s['status']=='completed' and s['conclusion']=='success' for s in job['steps'])==1)
download=unique((B/'d03-consumers/download-intake.json').read_bytes())
a=download['artifact']
check('independent artifact metadata binding',a['id']==14788322055 and a['size_in_bytes']==len(archive) and a['digest']=='sha256:'+sha(archive) and a['workflow_run']['id']==run['id'] and a['workflow_run']['head_sha']==K and not a['expired'])
pins=unique(data['consumer/documentation-compatibility-manifest.json'])
check('consumer manifest exact K11 Git blob',data['consumer/documentation-compatibility-manifest.json']==blob(R,K,'skills/cherri-shortcuts/compatibility-manifest.json'))
check('S D K pins',pins['compilerCommit']==S and pins['docsCommit']==DS)
report=unique(data['consumer/documentation-consumers.json'])
record=unique(data['consumer/documentation-consumer-evidence.json'])['records']
check('one distinct original docs record',len(record)==1)
record=record[0]
check('original record scope and actual exit',record['requirements']==['D03'] and record['tier']=='docs' and record['implementation_sha']==K and record['test_id']=='tools/p10r15:verify_documentation_consumers' and record['exit_code']==0 and record['result']=='passed')
check('actual installed consumer clean identity',report['mode']=='actual-installed-public-site' and report['implementationSha']==K and report['compilerCommit']==S and report['docsCommit']==DS and report['sourceClean'] and report['docsClean'] and report['installedSkill']!=report['sourceDirectory']+'/skills/cherri-shortcuts')
check('actual canonical consumer origins',report['sourceOrigin']==pins['compilerRepo']=='https://github.com/davidpovarsky/cherri.git' and report['docsOrigin']==pins['docsRepo']=='https://github.com/davidpovarsky/cherrilang.org.git')
for key in ['schemaFingerprint','codecFingerprint','macroFingerprint']:check('consumer '+key,report[key]==pins[key])
check('collector observation within original job',utc(job['started_at'])<=utc(report['observedAtUTC'])<=utc(job['completed_at']) and 0<report['elapsedSeconds']<30)
cmd=record['command'];flags=dict(zip(cmd[2::2],cmd[3::2]))
check('actual collector invocation exact',len(cmd)==18 and cmd[:2]==['python3','tools/p10r15/verify_documentation_consumers.py'] and len(flags)==8 and flags['--source']==report['sourceDirectory'] and flags['--skill']==report['installedSkill'] and flags['--docs']==report['docsDirectory'] and flags['--implementation-sha']==K)
check('34 original consumer references',len(record['artifacts'])==34 and len({x['path'] for x in record['artifacts']})==34)
for x in record['artifacts']:
 p=pathlib.PurePosixPath(x['path'])
 check('original absolute reference '+p.name,p.is_absolute() and str(p.parent)==flags['--output'] and 'consumer/'+p.name in data and sha(data['consumer/'+p.name])==x['sha256'])
doc_artifacts=['docs-index.md','docs-guide.md','docs-actions.md','docs-migration.md']
skill_artifacts=['installed-search-docs.sh.txt','installed-common.sh.txt','documentation-compatibility-manifest.json']
bindings={}
for item,name in zip(report['sourceFiles'],doc_artifacts):
 raw=blob(D,DS,item['path'])
 check('D11 exact documentation '+item['path'],raw==data['consumer/'+name] and sha(raw)==item['sha256'])
 bindings['docs/'+item['path']]=item['sha256']
check('four docs inventory',len(report['sourceFiles'])==4)
for item,name in zip(report['skillFiles'],skill_artifacts):
 raw=blob(R,K,'skills/cherri-shortcuts/'+item['path'])
 check('K11 installed Skill '+item['path'],raw==data['consumer/'+name] and sha(raw)==item['sha256'])
 bindings['skill/'+item['path']]=item['sha256']
check('three installed Skill identities',len(report['skillFiles'])==3)
check('all seven original before after digests unchanged',report['before']==bindings==report['after'])
args={'active-empty':[],'active-show':['show'],'legacy-empty':['--legacy-reference'],'legacy-query':['--legacy-reference','action'],'absent-query':['CHERRI_D03_NO_MATCH_6ce5e096b6a7'],'invalid-regex':['['],'missing-current':['CHERRI_D03_HISTORICAL_SENTINEL_49CE'],'restored-active':['show']}
check('eight original distinct searches',len(report['searchCases'])==8 and [x['name'] for x in report['searchCases']]==list(args))
search_summary=[]
for c in report['searchCases']:
 n=c['name'];stdout=data['consumer/'+n+'.stdout.log'];stderr=data['consumer/'+n+'.stderr.log']
 check('search invocation '+n,c['command']==['/usr/bin/sh',report['installedSkill']+'/scripts/search-docs.sh']+args[n] and c['cwd']==report['installedSkill'] and c['timedOut'] is False)
 check('search original streams '+n,sha(stdout)==c['stdoutSHA256'] and sha(stderr)==c['stderrSHA256'])
 docdir=c['docsDirectory'];expected=1 if n=='missing-current' else 2 if n=='invalid-regex' else 0
 check('search original exit '+n,c['exitCode']==expected)
 if n=='missing-current':
  check('isolated missing current fail closed',docdir!=report['docsDirectory'] and docdir.endswith('/missing-current-docs') and not stdout and stderr==('Active V2 documentation directory is missing or linked: '+docdir+'/fork-language\n').encode())
 else:check('search pinned docs '+n,docdir==report['docsDirectory'])
 if n in ['active-empty','legacy-empty']:
  check('exact selected subtree '+n,not stderr and stdout==(docdir+('/language' if n=='legacy-empty' else '/fork-language')+'\n').encode())
 if n in ['active-show','restored-active','legacy-query']:
  prefix=docdir+('/language' if n=='legacy-query' else '/fork-language')
  lines=stdout.decode().splitlines()
  check('actual bounded subtree search '+n,not stderr and len(lines)>0 and all(re.match('^'+re.escape(prefix)+r'/[^:\n]+:[1-9][0-9]*:',l) for l in lines))
  if n!='legacy-query':check('current generated action matches '+n,any(l.startswith(prefix+'/actions.md:') and 'show' in l.lower() for l in lines))
 if n=='absent-query':check('no match empty success',not stdout and not stderr)
 if n=='invalid-regex':check('invalid regex error retained',not stdout and bool(stderr))
 search_summary.append({'case':n,'exit_code':c['exitCode'],'timed_out':c['timedOut'],'stdout_bytes':len(stdout),'stderr_bytes':len(stderr)})
check('healthy restoration byte identical',data['consumer/active-show.stdout.log']==data['consumer/restored-active.stdout.log'])
build=unique(data['cherri-docs-site-evidence.json']);controls=unique(data['cherri-docs-site-controls.json']);deployment=unique(data['documentation-site-deployment.json'])
for outer,copy,key in [('cherri-docs-site-evidence.json','documentation-site-build.json','siteBuildSHA256'),('cherri-docs-site-controls.json','documentation-site-controls.json','siteControlsSHA256'),('documentation-site-deployment.json','documentation-site-deployment.json','deploymentSHA256')]:
 check('original site input '+outer,data[outer]==data['consumer/'+copy] and sha(data[outer])==report[key])
check('site build exact pins and scope',build['docsCommit']==DS and build['compilerCommit']==S and build['sourceTreeClean'] and build['sitePublished'] is False and build['historicalSearchEntries']==0 and build['sourceReferenceSha256']==bindings['docs/fork-language/actions.md'])
check('original six site controls all reject and restore',controls['docsCommit']==DS and controls['compilerCommit']==S and controls['passed']==6 and controls['failed']==controls['skipped']==0 and controls['artifactsRestoredExactly'] and len(controls['controls'])==6 and all(x['rejected'] for x in controls['controls']))
routes=[('', 'published-home.html'),('fork-language/','published-index.html'),('fork-language/guide','published-guide.html'),('fork-language/actions','published-actions.html'),('fork-language/migration','published-migration.html'),('assets/js/search-data.json','published-search-data.json'),('assets/preview.js','published-preview.js')]
check('seven exact actual public HTTP observations',len(report['httpObservations'])==7)
http_summary=[]
for c,(route,n) in zip(report['httpObservations'],routes):
 raw=data['consumer/'+n];mime=c['headers'].get('Content-Type','').split(';')[0].strip()
 expected='application/json' if route.endswith('.json') else 'application/javascript' if route.endswith('.js') else 'text/html'
 check('public HTTP original '+route,c['route']==route and c['url']=='https://davidpovarsky.github.io/cherrilang.org/'+route and c['artifact']==n and c['status']==200 and c['bytes']==len(raw)>0 and c['sha256']==sha(raw) and mime==expected)
 http_summary.append({'route':route,'status':c['status'],'bytes':len(raw),'sha256':sha(raw)})
check('actual live actions equals deployed original',sha(data['consumer/published-actions.html'])==build['renderedActionPageSha256'])
check('actual live search equals deployed original',sha(data['consumer/published-search-data.json'])==build['searchIndexSha256'])
search=unique(data['consumer/published-search-data.json']);allowed={'','/fork-language','/fork-language/actions','/fork-language/guide','/fork-language/migration'}
check('search contains exact50 built entries',len(search)==build['searchEntries']==50)
action_count=0
for k,v in search.items():
 route=v['relUrl'].split('#',1)[0].rstrip('/')
 check('public search route '+k,route in allowed and '?' not in v['relUrl'] and v['url']=='/cherrilang.org'+v['relUrl'] and all(isinstance(v[n],str) for n in ['doc','title','content','url','relUrl']))
 action_count+=route=='/fork-language/actions'
check('21 generated action search entries',action_count==build['actionSearchEntries']==21)
pub=unique(data['publication-input-receipt.json'])
check('publication receipt S D K exact',pub['status']=='PASS_PUBLICATION_INPUT_BINDING_ONLY' and pub['implementationSha']==K and pub['docsCommit']==DS and pub['consumerCompilerPin']==S)
pubraw=data['original-publication-evidence.zip']
check('original Pages ZIP exact',len(pubraw)==pub['originalArchiveBytes']==1535 and sha(pubraw)==pub['originalArchiveSha256'])
pz=safe_zip(pubraw,2)
check('exact original publication ZIP members',set(pz.namelist())==set(pub['members']))
for n,h in pub['members'].items():check('original published receipt '+n,pz.read(n)==data[n] and sha(data[n])==h)
for n,h in pub['metadata'].items():check('original GitHub deployment metadata '+n,sha(data[n])==h)
dr=unique(data['github-deployment-run.json']);drc=unique(data['github-deployment-run-confirmed.json'])
dj=unique(data['github-deployment-jobs.json'])['jobs'];da=unique(data['github-deployment-artifacts.json'])['artifacts']
check('original confirmed D11 deployment identity',dr==drc and dr['id']==45091966838 and dr['head_sha']==DS and dr['path']=='.github/workflows/pages.yml' and dr['status']=='completed' and dr['conclusion']=='success' and dr['run_attempt']==1 and utc(dr['updated_at'])<utc(report['observedAtUTC']))
check('original build deploy jobs exact',len(dj)==2 and {x['name'] for x in dj}=={'build','deploy'} and all(x['head_sha']==DS and x['run_id']==dr['id'] and x['status']=='completed' and x['conclusion']=='success' for x in dj))
pa=[x for x in da if x['id']==14782045537]
check('actual publication artifact original hash',len(pa)==1 and pa[0]['digest']=='sha256:'+sha(pubraw) and pa[0]['size_in_bytes']==len(pubraw) and pa[0]['workflow_run']['id']==dr['id'] and pa[0]['workflow_run']['head_sha']==DS)
check('minimal deployment projection exact',deployment['headSha']==DS and deployment['status']=='completed' and deployment['conclusion']=='success' and deployment['url']==dr['html_url'] and deployment['jobs']==[{k:x[k] for k in ['name','conclusion','status']} for x in dj])
spraw=data['skill-source.zip'];sp=safe_zip(spraw,23)
pm=unique(sp.read('cherri-shortcuts/package-manifest.json'))
check('source package S D K exact',pm['kind']=='source' and pm['runtime'] is None and pm['skillCommit']==K and pm['compilerCommit']==S and pm['docsCommit']==DS)
check('source package exact22 payload inventory',len(pm['files'])==22 and {x['path'] for x in pm['files']}==set(sp.namelist())-{'cherri-shortcuts/package-manifest.json'})
package_bindings=[]
for x in pm['files']:
 raw=sp.read(x['path']);rel=x['path'].removeprefix('cherri-shortcuts/');p='LICENSE' if rel=='LICENSE.cherri' else 'skills/cherri-shortcuts/'+rel
 canonical=blob(R,K,p)
 mode=int(subprocess.check_output(['git','--no-replace-objects','-C',str(R),'ls-tree',K,'--',p],text=True,timeout=15).split()[0],8)
 check('source package original Git bytes '+p,raw==canonical and sha(raw)==x['sha256'] and len(raw)==x['bytes'] and int(x['mode'],8)==mode and (sp.getinfo(x['path']).external_attr>>16)==mode)
 package_bindings.append({'path':p,'sha256':sha(raw),'bytes':len(raw)})
for n,unpacked in [('skill-source-create.json',False),('skill-source-verify.json',True)]:
 x=unique(data[n]);check('actual package receipt '+n,x['archiveSha256']==sha(spraw) and x['skillCommit']==K and x['compilerCommit']==S and x['docsCommit']==DS and x['status']=='PASS_PACKAGE_INTEGRITY' and x['verifiedFiles']==23 and x['unpacked'] is unpacked)
intake=unique(data['intake-receipt.json'])
check('strict intake source bound original consumer record',intake['status']=='PASS_D03_DOCS_PRODUCER_INTAKE_ONLY' and intake['implementation_sha']==K and intake['collector_evidence_sha256']==sha(data['consumer/documentation-consumer-evidence.json']) and intake['documentation_record_accepted'] and intake['original_references_unchanged'])
check('app tier and closure explicitly not granted',intake['app_integration_executed'] is False and intake['d03_complete'] is False and intake['closure_complete'] is False and intake['report']['passed_cases']==0 and intake['report']['failed_cases']==1 and intake['report']['results'][0]['executed_levels']==['docs'] and intake['report']['validation_errors']==['requirement D03 has unsatisfied local tier'])
events=[unique(x) for x in data['intake-test.jsonl'].splitlines() if x.strip()]
test='TestDocumentationConsumerCIActualIntake';pkg='github.com/electrikmilk/cherri/tools/language-acceptance'
check('actual original named Go run pass and package pass',sum(x.get('Action')=='run' and x.get('Test')==test and x.get('Package')==pkg for x in events)==1 and sum(x.get('Action')=='pass' and x.get('Test')==test and x.get('Package')==pkg for x in events)==1 and sum(x.get('Action')=='pass' and not x.get('Test') and x.get('Package')==pkg for x in events)==1 and not any(x.get('Action') in ['fail','skip'] for x in events))
check('all original Go events within actual job',all(utc(job['started_at'])<=utc(x['Time'])<=utc(job['completed_at']) for x in events))
logged=''.join(x.get('Output','') for x in events)
tail=logged.split('P15_D03_CONSUMER_CI_INTAKE:',1)[1].lstrip()
logged_receipt=json.JSONDecoder().raw_decode(tail)[0]
check('receipt actually emitted by original Go producer',logged_receipt==intake)
sourcepaths=['.github/workflows/p10r15-documentation-consumers.yml','tools/p10r15/package_skill.py','tools/p10r15/prepare_documentation_consumer_ci.py','tools/p10r15/verify_documentation_consumers.py','tools/language-acceptance/documentation_consumer_ci_test.go','tools/language-acceptance/documentation_consumers_intake.go','tools/language-acceptance/documentation_intake.go','tools/language-acceptance/main.go','tests/language-v2/acceptance-contract.json','tests/language-v2/repair-regression-plan.json','tests/language-v2/backend-recovery-gates.json']
witness=[{'path':p,'sha256':sha(blob(R,K,p)),'bytes':len(blob(R,K,p))} for p in sourcepaths]
outcome={'schema':'cherri-d03-independent-original-intake/v1','observed_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'status':'PASS_DOCS_PRODUCER_SCOPE_ONLY','implementation_sha':K,'compiler_sha':S,'documentation_sha':DS,'run':45108346953,'job':137474720360,'artifact':14788322055,'original_archive_path':str(B/'d03-consumers/14788322055.original.zip'),'original_archive_sha256':sha(archive),'outer_members':50,'original_reference_count':34,'source_package_members':23,'source_package_payload_blobs':22,'installed_source_and_docs_bindings':7,'installed_searches':search_summary,'public_http_observations':http_summary,'public_search_entries':50,'public_action_search_entries':21,'site_original_causal_controls':6,'collector_elapsed_seconds':report['elapsedSeconds'],'go_intake_parent_seconds':next(x['Elapsed'] for x in events if x.get('Action')=='pass' and x.get('Test')==test),'go_intake_package_seconds':next(x['Elapsed'] for x in events if x.get('Action')=='pass' and not x.get('Test')),'docs_record_accepted':True,'original_D03_report_status':intake['report']['results'][0]['status'],'app_integration_accepted':False,'d03_complete':False,'closure_complete':False,'new_tests_or_producers_executed':0,'new_network_requests':0,'originals_modified':False,'checks':checks,'source_witness':witness,'package_source_witness':package_bindings}
(O/'independent-intake.json').write_text(json.dumps(outcome,indent=2)+'\n')
print(json.dumps({k:v for k,v in outcome.items() if k not in ['checks','source_witness','package_source_witness','installed_searches','public_http_observations']},indent=2))
print('checks',len(checks),'intake_sha256',sha((O/'independent-intake.json').read_bytes()))
