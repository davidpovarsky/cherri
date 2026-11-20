import pathlib,subprocess,json,hashlib
r=pathlib.Path(r"C:\Users\DAVID\Code\cherri-p10r15-json-oracle-r5-20261120");base="79cffde04145685710da2a9b8687e5953d7e587c"
def git(*a):return subprocess.check_output(["git","--no-replace-objects",*a],cwd=r,timeout=25).decode().strip()
assert git("rev-parse","HEAD")==base and not git("status","--porcelain")
p=r/"p10_http_json_oracle_test.go";original=p.read_bytes();text=original.decode()
old='''\tfor _, action := range actions {\n\t\tfields, ok := action["WFWorkflowActionParameters"].(map[string]any)\n'''
new='''\tfor _, action := range actions {\n\t\t// The canonical main Nothing action can omit its parameter envelope.\n\t\t// No other missing or malformed action shape supplies a binding.\n\t\tif _, present := action["WFWorkflowActionParameters"]; !present && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.nothing" {\n\t\t\tcontinue\n\t\t}\n\t\tfields, ok := action["WFWorkflowActionParameters"].(map[string]any)\n'''
assert text.count(old)==1
p.write_bytes(text.replace(old,new,1).encode())
subprocess.run(["gofmt","-w",str(p)],check=True,cwd=r)
git("diff","--check")
diff=subprocess.check_output(["git","--no-replace-objects","diff","--","p10_http_json_oracle_test.go"],cwd=r)
d=pathlib.Path(r"C:\Users\DAVID\Code\cherri-p10r15-continuation-evidence-20261119\r5-json-oracle\counterexamples-harness-envelope")
d.mkdir(parents=True,exist_ok=False);(d/"component.patch").write_bytes(diff);(d/"submitted-script.py").write_bytes(pathlib.Path(__file__).read_bytes())
git("add","--","p10_http_json_oracle_test.go");result=git("commit","-m","test: accept canonical omitted Nothing parameters in JSON binding probe")
head=git("rev-parse","HEAD");assert not git("status","--porcelain")
record={"baseline":base,"source":head,"scope":"Only canonical Nothing omitted parameter envelope; duplicate counting and reference decoder unchanged","patch_sha256":hashlib.sha256(diff).hexdigest(),"source_clean":True,"commit":result}
(d/"receipt.json").write_bytes((json.dumps(record,indent=2)+"\n").encode())
print(json.dumps(record))
