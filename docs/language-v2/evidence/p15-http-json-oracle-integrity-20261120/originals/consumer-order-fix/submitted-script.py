import pathlib,subprocess,json,hashlib
r=pathlib.Path(r"C:\Users\DAVID\Code\cherri-p10r15-json-oracle-r5-20261120");base="ff6a75e60f41eb4e64be55104cfb10e785b3b5cf"
def git(*a):return subprocess.check_output(["git","--no-replace-objects",*a],cwd=r,timeout=25)
def textgit(*a):return git(*a).decode().strip()
assert textgit("rev-parse","HEAD")==base and not textgit("status","--porcelain")
p=r/"p10_http_json_oracle_test.go";s=p.read_text(encoding="utf-8")
old='''\tvar identities = map[string]bool{}
\tvar producers int
\tfor _, action := range actions {
'''
new='''\tvar identities = map[string]bool{}
\tvar producers int
\tvar producerPosition = -1
\tvar consumerPosition = -1
\tfor index, action := range actions {
'''
assert s.count(old)==1;s=s.replace(old,new,1)
old='''\t\tif outputName != "dynamicModel" {
\t\t\tcontinue
\t\t}
\t\tproducers++
'''
new='''\t\tif identifier == "is.workflow.actions.downloadurl" && fields["WFHTTPBodyType"] == "JSON" {
\t\t\tif consumerPosition >= 0 {
\t\t\t\treturn nil, fmt.Errorf("duplicated actual JSON consumer")
\t\t\t}
\t\t\tconsumerPosition = index
\t\t}
\t\tif outputName != "dynamicModel" {
\t\t\tcontinue
\t\t}
\t\tproducers++
\t\tproducerPosition = index
'''
assert s.count(old)==1;s=s.replace(old,new,1)
old='''\tif producers != 1 {
\t\treturn nil, fmt.Errorf("missing actual dynamic model producer")
\t}
\treturn resolved, nil
'''
new='''\tif producers != 1 {
\t\treturn nil, fmt.Errorf("missing actual dynamic model producer")
\t}
\t// This fixed source has one JSON consumer. A future producer cannot supply
\t// its already executed request merely because the UUID exists somewhere.
\tif consumerPosition < 0 || producerPosition >= consumerPosition {
\t\treturn nil, fmt.Errorf("actual model producer must precede its single JSON consumer")
\t}
\treturn resolved, nil
'''
assert s.count(old)==1;s=s.replace(old,new,1)
old='"duplicate_nonmodel_uuid", "producer_after_consumer"}'
new='"duplicate_nonmodel_uuid", "producer_after_consumer", "missing_json_consumer", "duplicate_json_consumer", "non_json_consumer"}'
assert s.count(old)==1;s=s.replace(old,new,1)
old='''\t\t\tcase "producer_after_consumer":
'''
new='''\t\t\tcase "missing_json_consumer":
\t\t\t\tchanged = append(changed[:consumer], changed[consumer+1:]...)
\t\t\tcase "duplicate_json_consumer":
\t\t\t\tvar duplicate = p10CloneJSONWire(changed[consumer]).(map[string]any)
\t\t\t\tp15C01Params(duplicate)["UUID"] = "adebc120-169d-4cc9-9eab-91166f2b3275"
\t\t\t\tchanged = append(changed, duplicate)
\t\t\tcase "non_json_consumer":
\t\t\t\tp15C01Params(changed[consumer])["WFHTTPBodyType"] = "File"
\t\t\tcase "producer_after_consumer":
'''
assert s.count(old)==1;s=s.replace(old,new,1)
p.write_bytes(s.encode());subprocess.run(["gofmt","-w",str(p)],cwd=r,check=True);git("diff","--check")
d=pathlib.Path(r"C:\Users\DAVID\Code\cherri-p10r15-continuation-evidence-20261119\r5-json-oracle\consumer-order-fix");d.mkdir(parents=True,exist_ok=False)
patch=git("diff","--","p10_http_json_oracle_test.go");(d/"component.patch").write_bytes(patch);(d/"submitted-script.py").write_bytes(pathlib.Path(__file__).read_bytes())
git("add","--","p10_http_json_oracle_test.go");message=textgit("commit","-m","test: require JSON producer availability before its consumer")
head=textgit("rev-parse","HEAD");assert not textgit("status","--porcelain")
record={"parent":base,"source":head,"scope":"One actual JSON consumer must follow the actual unique model producer","patch_sha256":hashlib.sha256(patch).hexdigest(),"source_clean":True,"commit":message}
(d/"receipt.json").write_bytes((json.dumps(record,indent=2)+"\n").encode());print(json.dumps(record))
