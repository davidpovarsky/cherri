python3 - <<'PY'
from pathlib import Path
import subprocess,json,time,hashlib,plistlib
out=Path('/home/chatgpt-agent/workspaces/cherri-p10r15-evidence-20261119/module-6439bd4');binary=out/'cherri-482873e-linux-arm64';old=out.parent/'module-audit-b34-20261119';dest=out/'cli-cases-482873e';dest.mkdir(exist_ok=True)
for suffix in ['stdout','stderr']:
 p=dest/'missing'/('check.'+suffix)
 if p.exists():p.rename(p.with_name('initial-incorrect-flag-syntax.'+suffix))
sha='482873e3b3776be7178c32977f6038559ce03d8d';assert hashlib.sha256(binary.read_bytes()).hexdigest()=='da57d10e9e8de7799eda8dd6c12e24c7b04bea3a7920d7767a2523f261ecfd82'
cases=[('missing',False,'cannot resolve module',[]),('runtime_init',False,'module top-level runtime initialization rejected',[]),('cycle',False,'import cycle detected',[]),('same_spelling_distinct_paths',True,'',['LEAF_OK']),('distinct_path_control',True,'',['LEAF_OK']),('namespace_collision',True,'',['MODULE_A','MODULE_B']),('distinct_name_control',True,'',['MODULE_A','MODULE_B'])]
for name,valid,message,markers in cases:
 for p in (old/name).rglob('*.cherri'):
  target=dest/name/p.relative_to(old/name);target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(p.read_bytes())
extra={'uppercase_namespace':{'main.cherri':'import "./a.cherri" as First\nimport "./b.cherri" as Second\nlet firstValue = First.value()\nlet secondValue = Second.value()\n','a.cherri':'function value() -> Text { return "MODULE_A" }\n','b.cherri':'function value() -> Text { return "MODULE_B" }\n'},'uppercase_standard_library':{'main.cherri':'import stdfunc as Std\nlet js = Std.runJS("document.title")\nlet selected = Std.chooseFromVCard(["One", "Two"], prompt: "Choose")\n'},'uppercase_unexported':{'main.cherri':'import stdfunc as Std\nlet value = Std.missing()\n'}}
for name,files in extra.items():
 for rel,text in files.items():
  p=dest/name/rel;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(text)
cases += [('uppercase_namespace',True,'',['MODULE_A','MODULE_B']),('uppercase_standard_library',True,'',['is.workflow.actions.text.replace','is.workflow.actions.setitemname','is.workflow.actions.choosefromlist']),('uppercase_unexported',False,'does not export callable',[])]
inputs=[{'path':str(p.relative_to(dest)),'bytes':p.stat().st_size,'sha256':hashlib.sha256(p.read_bytes()).hexdigest()} for p in sorted(dest.rglob('*.cherri'))]
rows=[]
def strings(value):
 if isinstance(value,str):yield value
 elif isinstance(value,dict):
  for item in value.values():yield from strings(item)
 elif isinstance(value,list):
  for item in value:yield from strings(item)
for name,valid,message,markers in cases:
 case=dest/name;entry=case/'main.cherri';artifact=case/'result.shortcut';assert not artifact.exists()
 for operation in ['check','compile']:
  cmd=[str(binary),'--check='+str(entry),'--json','--no-ansi'] if operation=='check' else [str(binary),'compile',str(entry),'--output='+str(artifact),'--skip-sign','--no-ansi']
  start=time.monotonic();r=subprocess.run(cmd,cwd=case,capture_output=True,timeout=15);seconds=round(time.monotonic()-start,3)
  (case/(operation+'.stdout')).write_bytes(r.stdout);(case/(operation+'.stderr')).write_bytes(r.stderr)
  row={'case':name,'operation':operation,'command':cmd,'exitCode':r.returncode,'expectedExitCode':0 if valid else 1,'seconds':seconds,'stdoutBytes':len(r.stdout),'stdoutSha256':hashlib.sha256(r.stdout).hexdigest(),'stderrBytes':len(r.stderr),'stderrSha256':hashlib.sha256(r.stderr).hexdigest()}
  assert r.returncode==(0 if valid else 1),(name,operation,r.returncode,r.stderr.decode()[:500])
  if operation=='check':
   data=json.loads(r.stdout);assert data['valid'] is valid;diags=data.get('diagnostics') or [];row['valid']=data['valid'];row['diagnosticCodes']=[d.get('code') for d in diags];row['artifactAbsent']=not artifact.exists();assert row['artifactAbsent']
   if not valid:assert any(message in d.get('message','') for d in diags),(name,diags)
  elif valid:
   raw=artifact.read_bytes();native=plistlib.loads(raw);actions=native['WFWorkflowActions'];values=list(strings(actions));row['artifact']={'bytes':len(raw),'sha256':hashlib.sha256(raw).hexdigest(),'actions':len(actions),'expectedNativeMarkers':markers,'allExpectedMarkersPresent':all(m in values for m in markers)};assert row['artifact']['allExpectedMarkersPresent'],(name,markers)
  else:
   row['artifactAbsent']=not artifact.exists();assert row['artifactAbsent'] and message in r.stderr.decode(),name
  row['passed']=True;rows.append(row)
summary={'source':sha,'binarySha256':hashlib.sha256(binary.read_bytes()).hexdigest(),'scope':'Actual local CLI check/compile and native plist structure; no Apple execution claim','operations':len(rows),'passed':sum(r['passed'] for r in rows),'failed':0,'fixtureFiles':inputs,'results':rows}
(out/'cli-module-482873e-summary.json').write_text(json.dumps(summary,indent=2)+'\n')
print(json.dumps({'source':sha,'operations':len(rows),'passed':summary['passed'],'failed':0,'fixtureFiles':len(inputs),'cases':[{'case':n,'expectedValid':v} for n,v,_,_ in cases],'summarySha256':hashlib.sha256((out/'cli-module-482873e-summary.json').read_bytes()).hexdigest()}))
PY