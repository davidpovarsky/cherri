#!/usr/bin/env python3
import pathlib, subprocess, json, plistlib, collections
ROOT=pathlib.Path('/home/chatgpt-agent/workspaces/cherri-remote/pure-upstream')
PACK=ROOT/'apple-actions-pack'
JOB=ROOT/'jobs/apple-actions-pack-checks'
BIN=ROOT/'bin/cherri'
MOD=PACK/'modules'
JOB.mkdir(exist_ok=True, parents=True)
for f in MOD.glob('*.cherri'):
 (JOB/f.name).write_bytes(f.read_bytes())
def compile_case(label,names,code):
 src=JOB/(label+'.cherri')
 src.write_text('#define name '+label+'\n'+''.join("#include '"+n+".cherri'\n" for n in names)+code+'\n',encoding='utf8')
 output=JOB/(label+'_unsigned.shortcut')
 if output.exists(): output.unlink()
 try:
  cmd=[str(BIN),str(src),'--skip-sign','--no-ansi']
  p=subprocess.run(cmd,cwd=JOB,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,timeout=20)
  text=p.stdout.decode(errors='replace')
  ret={'name':label,'modules':names,'ok':p.returncode==0,'return':p.returncode,'error':text[-600:]}
  if output.is_file():
   v=plistlib.loads(output.read_bytes())
   ret['actions']=[q['WFWorkflowActionIdentifier'] for q in v['WFWorkflowActions']]
   ret['native_keys']=[sorted(q.get('WFWorkflowActionParameters',{})) for q in v['WFWorkflowActions']]
   output.unlink()
  else:ret['ok']=False
  return ret
 except Exception as e:return dict(name=label,modules=names,ok=False,error=str(e))
result=[]
for f in sorted(MOD.glob('*.cherri')):
 result.append(compile_case('module-'+f.stem,[f.stem],'show("probe")'))
result.append(compile_case('pack-all-modules',[f.stem for f in sorted(MOD.glob('*.cherri'))],'show("probe")'))
for v in result:
 print('PASS' if v['ok'] else 'FAIL',v['name'], v['error'][:350] if not v['ok'] else ','.join(v['actions']),flush=True)
(PACK/'module-verification.json').write_text(json.dumps(result,ensure_ascii=False,indent=2))
print('SUMMARY',json.dumps(dict(total=len(result),passed=sum(x['ok'] for x in result),failed=[r['name'] for r in result if not r['ok']]),ensure_ascii=False),flush=True)
