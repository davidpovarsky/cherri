import pathlib,subprocess,json,hashlib
r=pathlib.Path(r"C:\Users\DAVID\Code\cherri-p10r15-json-oracle-r5-20261120");base="2f14d4186d84679da356562327a6132610396852"
def git(*a):return subprocess.check_output(["git","--no-replace-objects",*a],cwd=r,timeout=25)
def textgit(*a):return git(*a).decode().strip()
assert textgit("rev-parse","HEAD")==base and not textgit("status","--porcelain")
p=r/"p10_http_json_oracle_test.go";s=p.read_text(encoding="utf-8")
old='"malformed_action_identifier", "duplicate_nonmodel_uuid"}'
new='"malformed_action_identifier", "duplicate_nonmodel_uuid", "producer_after_consumer"}'
assert s.count(old)==1;s=s.replace(old,new,1)
old='''\t\t\tcase "duplicate_nonmodel_uuid":
\t\t\t\tchanged = append(changed, p10CloneJSONWire(changed[consumer]).(map[string]any))
'''
new=old+'''\t\t\tcase "producer_after_consumer":
\t\t\t\tvar producer = changed[model]
\t\t\t\tchanged = append(changed[:model], changed[model+1:]...)
\t\t\t\tvar movedConsumer = consumer
\t\t\t\tif model < consumer {
\t\t\t\t\tmovedConsumer--
\t\t\t\t}
\t\t\t\tvar insertion = movedConsumer + 1
\t\t\t\tchanged = append(changed, nil)
\t\t\t\tcopy(changed[insertion+1:], changed[insertion:])
\t\t\t\tchanged[insertion] = producer
'''
assert s.count(old)==1;s=s.replace(old,new,1)
p.write_bytes(s.encode());subprocess.run(["gofmt","-w",str(p)],cwd=r,check=True);git("diff","--check")
d=pathlib.Path(r"C:\Users\DAVID\Code\cherri-p10r15-continuation-evidence-20261119\r5-json-oracle\ordering-counterexample-source");d.mkdir(parents=True,exist_ok=False)
patch=git("diff","--","p10_http_json_oracle_test.go");(d/"component.patch").write_bytes(patch);(d/"submitted-script.py").write_bytes(pathlib.Path(__file__).read_bytes())
git("add","--","p10_http_json_oracle_test.go");message=textgit("commit","-m","test: expose JSON producer ordering counterexample")
head=textgit("rev-parse","HEAD");assert not textgit("status","--porcelain")
record={"parent":base,"source":head,"scope":"Tests-only reordered original native actions; decoder/binding behavior unchanged","patch_sha256":hashlib.sha256(patch).hexdigest(),"source_clean":True,"commit":message}
(d/"receipt.json").write_bytes((json.dumps(record,indent=2)+"\n").encode());print(json.dumps(record))
