#!/usr/bin/env python3
"""Build opt-in external action aliases, without writing to upstream Cherri code."""
from pathlib import Path
import json,re,collections,hashlib
ROOT=Path('/home/chatgpt-agent/workspaces/cherri-remote')
ORIGINAL=ROOT/'pure-upstream/source'
FORK=ROOT/'source/p10'
BASE=ROOT/'pure-upstream/apple-actions-pack'
MODULES=BASE/'modules'
MODULES.mkdir(parents=True,exist_ok=True)
CAT={x['name']:x for x in json.load(open(ROOT/'jobs/cherri-original-cutover-audit/p10-actions.json'))['actions']}
KNOWN_UNSAFE={'statistic'}
def declarations(path):
 s=path.read_text(encoding='utf8')
 starts=list(re.finditer(r'(?m)^(action|enum)\b',s))
 d={}
 for n,m in enumerate(starts):
  block=s[m.start():starts[n+1].start() if n+1<len(starts) else len(s)].strip()
  nm=(re.search(r'^enum\s+(\w+)',block) if m.group(1)=='enum' else re.search(r'^action\b.*?\b(\w+)\s*\(',block,re.S))
  if nm:d[(m.group(1),nm.group(1))]=block
 return d
def keys(block):
 m=re.search(r'\b\w+\s*\(',block)
 if not m:return []
 pos=m.end();level=1;q=None;start=pos
 while pos<len(block) and level:
  c=block[pos]
  if q:
   if c=='\\':pos+=2;continue
   if c==q:q=None
  elif c in "'\"":q=c
  elif c=='(':level+=1
  elif c==')':level-=1
  pos+=1
 return re.findall(r":\s*'([^']+)'",block[start:pos-1])
def compatible_keys(a,b):
 it=iter(b)
 return all(any(k==v for v in it) for k in a)
def normalize(s):
 return re.sub(r'\s+',' ',re.sub(r'(?m)^\s*//[^\n]*','',s)).strip()
def gather(repo):
 merged={};files={}
 for file in sorted((repo/'actions').glob('*.cherri')):
  for k,block in declarations(file).items():
   merged[k]=block;files[k]=file.stem
 return merged,files
def alias_declaration(block,name,alias,identifier):
 header_end=block.index('(',block.index(name))
 prefix=block[:header_end]
 # Quoted action identifiers override the default generated identifier.
 if "'" not in prefix:
  ident=identifier
  if ident.startswith('is.workflow.actions.'):ident=ident[len('is.workflow.actions.'):]
  prefix=prefix.replace(name+"'","") if False else prefix
  prefix=prefix[:prefix.rfind(name)]+"'"+ident+"' "+prefix[prefix.rfind(name):]
 block=prefix+block[header_end:]
 return re.sub(r'\b'+re.escape(name)+r'(?=\s*\()',alias,block,count=1)
OLD,OLD_FILES=gather(ORIGINAL);NEW,NEW_FILES=gather(FORK)
counts=collections.Counter()
choices=[]
by_module=collections.defaultdict(list)
for key,body in sorted(NEW.items(),key=lambda x:(NEW_FILES[x[0]],x[0][0],x[0][1])):
 kind,name=key;reason='';alias=None;out=None
 if name in KNOWN_UNSAFE:
  status='original_preserved';reason='fork has demonstrably wrong native key'
 elif key not in OLD:
  status='added';out=body
 elif normalize(OLD[key])==normalize(body):
  status='unchanged'
 elif kind=='enum':
  status='original_preserved';reason='changed original enum requires manual audit'
 else:
  original_keys=keys(OLD[key]);new_keys=keys(body)
  if not compatible_keys(original_keys,new_keys):
   status='original_preserved';reason='replacement/removal/reordering of original native param keys'
  else:
   status='extended_alias'
   alias=name+'Advanced'
   identifier=CAT.get(name,{}).get('shortcutIdentifier','is.workflow.actions.'+name.lower())
   out=alias_declaration(body,name,alias,identifier)
 counts[status]+=1
 choices.append(dict(kind=kind,name=name,alias=alias,module=NEW_FILES[key],status=status,old_file=OLD_FILES.get(key),reason=reason,originalKeys=keys(OLD[key]) if key in OLD and kind=='action' else None,newKeys=keys(body) if kind=='action' else None))
 if out is not None:by_module[NEW_FILES[key]].append((kind,name,out,alias))
for old_file in MODULES.glob('*.cherri'):old_file.unlink()
for module,defs in sorted(by_module.items()):
 lines=[
 '// EXTERNAL opt-in Cherri Apple Actions Pack — no modifications to original compiler.',
 '// Existing functions stay original; additional optional parameters use Advanced aliases.',
 "// Source declarations: P10 action DSL. Load original category's enums and metadata.",
 ("#include 'actions/"+module+"'" if module!="basic" else "// basic action category is implicitly included by Cherri"),
 '']
 for kind,name,block,alias in sorted(defs,key=lambda x: 0 if x[0]=='enum' else 1):
  lines+=['// '+('New '+kind+' '+name if not alias else 'Advanced alias '+alias+' for '+name),block,'']
 (MODULES/(module+'.cherri')).write_text('\n'.join(lines)+'\n')
manifest={'name':'cherri-apple-actions-pack','version':'0.1.1',
 'pure_source_sha':'bcc0095754e2cc0baaeb4d93dae172b8153b4716',
 'pure_source_tree':'4fe9fbc8c742aaf01d6ec8193d9be708fb1de7b5',
 'definition_source_branch':'agent/cherri-p10-apple-actions-coverage',
 'definition_source_sha':'2456327cc941f6ae89c870833d3d5f70109d693c',
 'status':'EXTERNAL_DEFINITIONS_NOT_RUNTIME_VERIFIED',
 'module_sha256':{p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in MODULES.glob('*.cherri')},
 'summary':dict(counts),'modules':{name:[{'name':k,'alias':alias,'kind':kind} for kind,k,_,alias in definitions] for name,definitions in sorted(by_module.items())},
 'special_handlers':['WFContentItemFilter predicates require native conversion; simple nil filters do not prove complex filters','P10 Go-only app intents and system actions need native descriptors/UUID mapping and separate validation','Original Cherri builtin run()/statistic() are NOT replaced'],
 'decisions':choices}
(BASE/'manifest.json').write_text(json.dumps(manifest,indent=2,ensure_ascii=False))
print(json.dumps({'summary':dict(counts),'modules':len(by_module),'entries_by_module':{n:len(l) for n,l in by_module.items()},'rejected':[x for x in choices if x['status']=='original_preserved']},indent=2,ensure_ascii=False))
