import datetime,hashlib,json,pathlib,subprocess,time
OUT=pathlib.Path('/mnt/development/projects/evidence/cherri-p10r15-storage-observer-20261123')
args=['timeout','--signal=TERM','--kill-after=30s','2700s','python3','-B',str(OUT/'qualification-driver.py')]
record={'argv':args,'startedUtc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'timeoutSeconds':2700,'terminationGraceSeconds':30,'hostWallCeilingSeconds':2800}
(OUT/'supervisor-start.json').write_text(json.dumps(record,indent=2)+'\n')
t=time.monotonic()
with (OUT/'supervisor.stdout.original.log').open('wb') as out,(OUT/'supervisor.stderr.original.log').open('wb') as err:
 p=subprocess.run(args,stdout=out,stderr=err)
record.update({'exitCode':p.returncode,'completedUtc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'durationSeconds':time.monotonic()-t})
for n in ['supervisor.stdout.original.log','supervisor.stderr.original.log']:
 f=OUT/n;record[n]={'bytes':f.stat().st_size,'sha256':hashlib.sha256(f.read_bytes()).hexdigest()}
(OUT/'supervisor-result.json').write_text(json.dumps(record,indent=2)+'\n')
print(json.dumps(record))
raise SystemExit(p.returncode)
