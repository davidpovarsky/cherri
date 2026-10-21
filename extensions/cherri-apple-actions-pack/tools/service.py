#!/usr/bin/env python3
"""Opt-in Apple Actions pack runner. Calls ONLY the unmodified upstream Cherri service."""
import argparse,hashlib,json,os,pathlib,re,shutil,subprocess,sys,tempfile
ROOT=pathlib.Path('/home/chatgpt-agent/workspaces/cherri-remote/pure-upstream')
PACK=ROOT/'apple-actions-pack'
JOB=ROOT/'jobs'
CORE=ROOT/'bin/pure-cherri-service.sh'
MAN=json.loads((PACK/'manifest.json').read_text())
def fail(s):
 print('ERROR:',s,file=sys.stderr);sys.exit(1)
def verify_pack():
 sha=MAN.get('module_sha256',{})
 actual={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in (PACK/'modules').glob('*.cherri')}
 if sha!=actual:fail('Pack files not matching signed manifest: run generator and re-verify')
 if MAN['pure_source_sha']!='bcc0095754e2cc0baaeb4d93dae172b8153b4716':fail('Unexpected upstream compiler pin')
 return sha
def core_call(*args):
 return subprocess.run([str(CORE),*args],check=False).returncode
def stage_enhanced(source,dest):
 if not source.exists() or source.suffix!='.cherri':fail('Use an existing .cherri source file')
 source=source.resolve()
 if JOB not in source.parents:fail('Job source must be inside '+str(JOB))
 if source.is_symlink():fail('Source must not be a symlink')
 # Copy the user's includes/assets into a temporary job, never edit their original file.
 if source.parent==JOB:
  for item in JOB.iterdir():
   if item.is_file() and not item.is_symlink():shutil.copy2(item,dest/item.name)
 else:
  shutil.copytree(source.parent,dest,dirs_exist_ok=True,symlinks=True)
 name=source.name
 content=(dest/name).read_text(encoding='utf8')
 mod_names=sorted(MAN['modules'])
 (dest/'apple-actions-pack').mkdir()
 for n in mod_names:shutil.copy2(PACK/'modules'/(n+'.cherri'),dest/'apple-actions-pack'/(n+'.cherri'))
 # Pack modules include the original action categories, so do not duplicate includes
 # from the source in the generated staging copy. Original source is left unchanged.
 patterns=[r"(?m)^[ \t]*#include[ \t]+'actions/"+re.escape(n)+r"'[ \t]*$" for n in mod_names if n!='basic']
 patterns.append(r"(?m)^[ \t]*#include[ \t]+'actions/basic'[ \t]*$")
 for pat in patterns:content=re.sub(pat,'// Managed by opt-in Apple Actions Pack',content)
 all_includes=''.join("#include 'apple-actions-pack/"+n+".cherri'\n" for n in mod_names)
 # Keep leading #define metadata in place and inject external modules before normal actions.
 lines=content.splitlines(keepends=True)
 i=0
 while i<len(lines) and (not lines[i].strip() or lines[i].lstrip().startswith('//') or lines[i].lstrip().startswith('#define ')):
  i+=1
 lines.insert(i,all_includes)
 generated=dest/(source.stem+'.enhanced.cherri')
 generated.write_text(''.join(lines),encoding='utf8')
 return generated
def main():
 parser=argparse.ArgumentParser(description='Original Cherri, optionally with Apple Actions extensions')
 parser.add_argument('operation',choices=['status','verify','docs','modules','compile','sign','test'])
 parser.add_argument('source',nargs='?')
 parser.add_argument('--mode',choices=['pure','enhanced'],default='pure')
 args=parser.parse_args()
 if args.operation in ('status','verify','docs','modules','test'):
  rc=core_call('status' if args.operation=='status' else ('verify' if args.operation in ('verify','modules','test') else 'docs'))
  if rc:return rc
  if args.operation in ('verify','status','modules','test'):
   sha=verify_pack()
   print('APPLE_ACTIONS_PACK=VERIFIED')
   print('PACK_STATUS='+MAN.get('status','UNKNOWN'))
   print('PACK_MODULES='+str(len(sha)))
   print('PACK_ACTIONS_NEW='+str(len([x for x in MAN['decisions'] if x['status']=='added' and x['kind']=='action'])))
   print('PACK_ACTIONS_ALIASED='+str(len([x for x in MAN['decisions'] if x['status']=='extended_alias'])))
   if args.operation=='modules':print('MODULE_NAMES='+','.join(MAN['modules']))
  if args.operation=='test':
   scripts=[PACK/'tools/verify_modules.py',PACK/'tools/verify_action_calls.py']
   for s in scripts:
    r=subprocess.run([sys.executable,str(s)],stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
    print(r.stdout.decode(errors='replace')[-1200:])
    if r.returncode:return r.returncode
  return 0
 if not args.source:fail('Operation '+args.operation+' requires .cherri source')
 path=pathlib.Path(args.source)
 verify_pack()
 rc=core_call('verify')
 if rc:return rc
 if args.mode=='pure':return core_call(args.operation,str(path))
 dest=pathlib.Path(tempfile.mkdtemp(prefix='enhanced-',dir=JOB))
 try:
  enhanced=stage_enhanced(path,dest)
  print('MODE=enhanced (pure compiler plus external package)')
  print('PACK_SHA256='+hashlib.sha256((PACK/'manifest.json').read_bytes()).hexdigest())
  return core_call(args.operation,str(enhanced))
 finally:
  shutil.rmtree(dest,ignore_errors=True)
if __name__=='__main__':sys.exit(main())
