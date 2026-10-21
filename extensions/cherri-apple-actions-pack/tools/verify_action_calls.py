#!/usr/bin/env python3
"""Exercise each opt-in action/alias using stock upstream compiler; inspect native plist."""
from pathlib import Path
import json,subprocess,plistlib,hashlib,collections,sys
ROOT=Path('/home/chatgpt-agent/workspaces/cherri-remote/pure-upstream')
PACK=ROOT/'apple-actions-pack'
JOB=ROOT/'jobs/apple-actions-pack-checks'
BIN=ROOT/'bin/cherri'
CAT={x['name']:x for x in json.load(open('/home/chatgpt-agent/workspaces/cherri-remote/jobs/cherri-original-cutover-audit/p10-actions.json'))['actions']}
MAN=json.load(open(PACK/'manifest.json'))
JOB.mkdir(exist_ok=True,parents=True)
def val(p):
 typ=p.get('type','text')
 key=p.get('key')
 nam=p.get('name','')
 if key=='WFContentItemFilter':return 'nil'
 if typ in ('dictionary','dict'):return '{"X-Test": "one"}'
 if typ in ('array','list'):return '["a"]'
 if typ in ('float',):return '0.5'
 if typ in ('number','integer'):return '1'
 if typ in ('bool','boolean'):return 'false'
 if typ in ('qty','quantity'):return 'qty(1, "sec")'
 if typ in ('variable','date','image','file','color','contact'):return 'Ask'
 if p.get('enumValues'):
  return json.dumps(p['enumValues'][0],ensure_ascii=False)
 if key in ('WFCalendarItemStartDate','WFCalendarItemEndDate'):return '"2026-10-22"'
 if 'url' in nam.lower() or 'url' in str(key).lower():return '"https://example.com"'
 if 'bundle' in nam.lower():return '"com.apple.mobilephone"'
 if 'email' in nam.lower():return '"test@example.com"'
 if 'script' in nam.lower():return '"echo test"'
 if 'path' in nam.lower():return '"sample.txt"'
 return '"sample"'
def args_for(catalog,full):
 a=catalog.get('parameters',[])
 if full:return [val(p) for p in a]
 # Take args through last required param; nil for earlier optional.
 must=[i for i,p in enumerate(a) if not p.get('optional')]
 return [val(p) if not p.get('optional') else 'nil' for p in a[:max(must)+1]] if must else []
def record_call(module,nm,original,variant,case):
 spec=CAT.get(original,{})
 actionid=spec.get('shortcutIdentifier')
 av=args_for(spec,variant=='full')
 code='#define name '+case+'\n#include '+"'"+module+".cherri'"+'\n'+nm+'('+', '.join(av)+')\n'
 path=JOB/(case+'.cherri');path.write_text(code)
 output=JOB/(case+'_unsigned.shortcut')
 if output.exists():output.unlink()
 try:
  p=subprocess.run([str(BIN),str(path),'--skip-sign','--no-ansi'],cwd=JOB,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,timeout=18)
  report=dict(name=nm,original=original,module=module,variant=variant,source=code,compiled=p.returncode==0,exit=p.returncode)
  if p.returncode:report['error']=p.stdout.decode(errors='replace')[-800:]
  elif output.is_file():
   obj=plistlib.loads(output.read_bytes())
   actions=obj.get('WFWorkflowActions',[])
   report['actual_ids']=[a['WFWorkflowActionIdentifier'] for a in actions]
   report['expected_id']=actionid
   a=actions[0] if actions else {}
   native=a.get('WFWorkflowActionParameters',{})
   report['actual_keys']=sorted(native)
   report['id_matches']=bool(actions and (not actionid or a['WFWorkflowActionIdentifier']==actionid))
   exp=[p['key'] for p,v in zip(spec.get('parameters',[]),av) if p.get('key') and v!='nil']
   report['missing_keys']=sorted(set(exp)-set(native))
   report['native_valid']=report['id_matches'] and not report['missing_keys'] and len(actions)==1
   output.unlink()
  else:report['error']='Compiler returned zero but produced no expected unsigned file'
 except Exception as e:report=dict(name=nm,module=module,variant=variant,source=code,compiled=False,error=str(e))
 return report
records=[]
for module,entries in MAN['modules'].items():
 for a in entries:
  if a['kind']!='action':continue
  original=a['name'];actual=a.get('alias') or original
  for var in ('minimal','full'):
   result=record_call(module,actual,original,var,'pack-'+actual+'-'+var)
   records.append(result)
   print(('PASS' if result.get('native_valid') else 'FAIL'),actual,var,(result.get('error') or result.get('missing_keys') or '') if not result.get('native_valid') else '',flush=True)
(PACK/'action-verification.json').write_text(json.dumps(records,indent=2,ensure_ascii=False))
print('SUMMARY',json.dumps({'calls':len(records),'compiled':sum(bool(r.get('compiled')) for r in records),'native_valid':sum(bool(r.get('native_valid')) for r in records),'failed_calls':len([r for r in records if not r.get('native_valid')]),'failed_actions':sorted(set(r['original'] for r in records if not r.get('native_valid'))) },ensure_ascii=False),flush=True)
