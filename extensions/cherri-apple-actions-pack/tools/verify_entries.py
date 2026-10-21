#!/usr/bin/env python3
"""Compile real calls for every externally added or extended action with unmodified Cherri."""
import json,pathlib,subprocess,plistlib,collections,re,os,sys
R=pathlib.Path('/home/chatgpt-agent/workspaces/cherri-remote')
PURE=R/'pure-upstream'; PACK=PURE/'apple-actions-pack'
JOB=PURE/'jobs/apple-actions-pack-entry-tests'
JOB.mkdir(exist_ok=True,parents=True)
BIN=PURE/'bin/cherri'
CAT={a['name']:a for a in json.load(open(R/'jobs/cherri-original-cutover-audit/p10-actions.json'))['actions']}
MAN=json.load(open(PACK/'manifest.json'))
for module in (PACK/'modules').glob('*.cherri'):
  (JOB/module.name).write_bytes(module.read_bytes())
def sample(p):
  key=p.get('key','')
  typ=p.get('type','text')
  name=p.get('name','')
  if key=='WFContentItemFilter':return 'nil'
  if key in ('WFSSHHost','WFSSHUser','WFSSHPort'):return '"example.com"' if key!='WFSSHPort' else '"22"'
  if key in ('WFSSHAuthenticationType',):return '"Password"'
  if key=='WFAppIdentifier' or 'bundle' in name.lower() or 'appid' in name.lower():return '"com.apple.mobilesafari"'
  if key=='WFFileType':return '"public.text"'
  if typ=='qty':return 'qty(1, "sec")'
  if typ=='float':return '0.5'
  if typ=='color':return 'color(0.0,0.0,0.0,1.0)'
  if typ=='bool':return 'true'
  if typ in ('number','integer'):return '1'
  if typ=='dictionary':return '{"hello":"world"}'
  if typ=='array':return '["one"]'
  if typ in ('variable','image','date','file','content'):return 'Ask'
  if typ=='rawtext':return "'raw text'"
  opts=p.get('enumValues')
  if opts:
    x=opts[0]
    return json.dumps(x,ensure_ascii=False)
  return '"sample"'
results=[]
defs=[(m,entry) for m,entries in MAN['modules'].items() for entry in entries if entry['kind']=='action']
for module, entry in defs:
  name=entry['name'];alias=entry['alias'];target=alias or name
  meta=CAT.get(name,{})
  params=meta.get('parameters',[])
  for isFull in (False,True):
    if isFull:used=params
    else:
      ix=max((i for i,p in enumerate(params) if not p.get('optional')),default=-1)
      used=params[:ix+1]
    values=[sample(q) for q in used]
    title=re.sub('[^a-zA-Z0-9]','',target)+('Full' if isFull else 'Min')
    src=JOB/(title+'.cherri')
    expr=f'{target}({", ".join(values)})'
    v27=any('v26 v26<' in (PACK/'modules'/(module+'.cherri')).read_text() for _ in []) # default not altering minVersion
    src.write_text('#define name '+title+'\n'+"#include '"+module+".cherri'\n"+expr+'\n')
    out=JOB/(title+'_unsigned.shortcut')
    if out.exists():out.unlink()
    rr={'name':name,'alias':alias,'module':module,'variant':'full' if isFull else 'minimum','call':expr}
    try:
      p=subprocess.run([str(BIN),str(src),'--skip-sign','--no-ansi'],cwd=JOB,timeout=15,stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
      log=p.stdout.decode('utf8',errors='replace')
      rr['exit']=p.returncode
      if p.returncode==0 and out.exists():
        plist=plistlib.loads(out.read_bytes())
        actions=plist.get('WFWorkflowActions',[])
        rr['actions']=len(actions)
        if actions:
          main=actions[-1]
          rr['actualActionId']=main['WFWorkflowActionIdentifier']
          rr['expectedActionId']=meta.get('shortcutIdentifier')
          keys=set(main.get('WFWorkflowActionParameters',{}))
          exp={x.get('key') for x,val in zip(used,values) if x.get('key') and val!='nil'}
          rr['missingKeys']=sorted(exp-keys)
          rr['nativeKeys']=sorted(keys)
          rr['ok']=bool(rr['actualActionId']==rr['expectedActionId']) and not rr['missingKeys']
        else:rr['ok']=False;rr['error']='No native actions'
        out.unlink()
      else:rr['ok']=False;rr['error']=log[-550:]
    except Exception as e:rr['ok']=False;rr['error']=str(e)
    results.append(rr)
    print('PASS' if rr.get('ok') else 'FAIL',name,rr['variant'],'' if rr.get('ok') else str(rr.get('error') or rr.get('missingKeys')),flush=True)
(PACK/'entry-verification.json').write_text(json.dumps(results,indent=2,ensure_ascii=False))
summary=dict(entries=len(defs),tests=len(results),passed=sum(r.get('ok',False) for r in results),failed=sum(not r.get('ok',False) for r in results),pass_actions=len({r['name'] for r in results if r.get('ok')}),missingKeys=collections.Counter(k for r in results for k in r.get('missingKeys',[])))
(PACK/'entry-verification-summary.json').write_text(json.dumps(summary,indent=2,ensure_ascii=False))
print('SUMMARY',json.dumps(summary,ensure_ascii=False,default=dict),flush=True)
