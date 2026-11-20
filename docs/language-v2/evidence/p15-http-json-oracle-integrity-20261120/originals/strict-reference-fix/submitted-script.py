import pathlib,subprocess,json,hashlib
r=pathlib.Path(r"C:\Users\DAVID\Code\cherri-p10r15-json-oracle-r5-20261120");base="49aa43beb1a5b77a84ddae58584205e70ef89760"
def git(*a):return subprocess.check_output(["git","--no-replace-objects",*a],cwd=r,timeout=30)
def textgit(*a):return git(*a).decode().strip()
assert textgit("rev-parse","HEAD")==base and not textgit("status","--porcelain")
data=json.loads("{\"reference\":\"func p10DecodeJSONReference(ref map[string]any, resolved map[string]any) (any, error) {\\n\\tvar kind, ok = ref[\\\"Type\\\"].(string)\\n\\tif !ok {\\n\\t\\treturn nil, fmt.Errorf(\\\"invalid reference Type metadata\\\")\\n\\t}\\n\\tvar identityKey string\\n\\tvar allowed map[string]bool\\n\\tswitch kind {\\n\\tcase \\\"ActionOutput\\\":\\n\\t\\tidentityKey = \\\"OutputUUID\\\"\\n\\t\\tallowed = map[string]bool{\\\"Type\\\": true, \\\"OutputUUID\\\": true, \\\"OutputName\\\": true}\\n\\tcase \\\"Variable\\\":\\n\\t\\tidentityKey = \\\"VariableName\\\"\\n\\t\\tallowed = map[string]bool{\\\"Type\\\": true, \\\"VariableName\\\": true}\\n\\tdefault:\\n\\t\\treturn nil, fmt.Errorf(\\\"unsupported reference kind %q\\\", kind)\\n\\t}\\n\\t// This decoder only proves plain references. Ignoring a property, coercion\\n\\t// or unknown field would silently substitute the original producer value.\\n\\tfor name, raw := range ref {\\n\\t\\tif !allowed[name] {\\n\\t\\t\\treturn nil, fmt.Errorf(\\\"unsupported reference field %q\\\", name)\\n\\t\\t}\\n\\t\\tif _, valid := raw.(string); !valid {\\n\\t\\t\\treturn nil, fmt.Errorf(\\\"invalid reference %s metadata\\\", name)\\n\\t\\t}\\n\\t}\\n\\tvar identity, valid = ref[identityKey].(string)\\n\\tif !valid || identity == \\\"\\\" {\\n\\t\\treturn nil, fmt.Errorf(\\\"missing reference %s\\\", identityKey)\\n\\t}\\n\\tvar key = kind + \\\":\\\" + identity\\n\\tvar value, exists = resolved[key]\\n\\tif !exists || value == nil {\\n\\t\\treturn nil, fmt.Errorf(\\\"unresolved actual reference %q\\\", key)\\n\\t}\\n\\treturn value, nil\\n}\",\"binding\":\"// Bind the value to one actual producer and a unique native identity. Map\\n// cardinality alone cannot distinguish duplicated actions sharing one UUID.\\nfunc p10JSONModelBindings(actions []map[string]any) (map[string]any, error) {\\n\\tvar resolved = map[string]any{}\\n\\tvar identities = map[string]bool{}\\n\\tvar producers int\\n\\tfor _, action := range actions {\\n\\t\\tvar identifier, valid = action[\\\"WFWorkflowActionIdentifier\\\"].(string)\\n\\t\\tif !valid || identifier == \\\"\\\" {\\n\\t\\t\\treturn nil, fmt.Errorf(\\\"invalid actual action identifier\\\")\\n\\t\\t}\\n\\t\\t// The canonical main Nothing action can omit its parameter envelope.\\n\\t\\t// No other missing or malformed action shape supplies a binding.\\n\\t\\tif _, present := action[\\\"WFWorkflowActionParameters\\\"]; !present && identifier == \\\"is.workflow.actions.nothing\\\" {\\n\\t\\t\\tcontinue\\n\\t\\t}\\n\\t\\tvar fields, ok = action[\\\"WFWorkflowActionParameters\\\"].(map[string]any)\\n\\t\\tif !ok || fields == nil {\\n\\t\\t\\treturn nil, fmt.Errorf(\\\"invalid actual action parameter shape\\\")\\n\\t\\t}\\n\\t\\tvar uuid string\\n\\t\\tif raw, present := fields[\\\"UUID\\\"]; present {\\n\\t\\t\\tuuid, valid = raw.(string)\\n\\t\\t\\tif !valid || uuid == \\\"\\\" {\\n\\t\\t\\t\\treturn nil, fmt.Errorf(\\\"invalid actual action UUID metadata\\\")\\n\\t\\t\\t}\\n\\t\\t\\tif identities[uuid] {\\n\\t\\t\\t\\treturn nil, fmt.Errorf(\\\"duplicated actual action UUID\\\")\\n\\t\\t\\t}\\n\\t\\t\\tidentities[uuid] = true\\n\\t\\t}\\n\\t\\tvar outputName string\\n\\t\\tif raw, present := fields[\\\"CustomOutputName\\\"]; present {\\n\\t\\t\\toutputName, valid = raw.(string)\\n\\t\\t\\tif !valid {\\n\\t\\t\\t\\treturn nil, fmt.Errorf(\\\"invalid actual output name metadata\\\")\\n\\t\\t\\t}\\n\\t\\t}\\n\\t\\tif outputName != \\\"dynamicModel\\\" {\\n\\t\\t\\tcontinue\\n\\t\\t}\\n\\t\\tproducers++\\n\\t\\tvar text, literal = fields[\\\"WFTextActionText\\\"].(string)\\n\\t\\tif producers != 1 || uuid == \\\"\\\" || identifier != \\\"is.workflow.actions.gettext\\\" || !literal || text != \\\"gpt-4o-mini\\\" {\\n\\t\\t\\treturn nil, fmt.Errorf(\\\"actual dynamic model producer changed or duplicated\\\")\\n\\t\\t}\\n\\t\\tresolved[\\\"ActionOutput:\\\"+uuid] = text\\n\\t}\\n\\tif producers != 1 {\\n\\t\\treturn nil, fmt.Errorf(\\\"missing actual dynamic model producer\\\")\\n\\t}\\n\\treturn resolved, nil\\n}\",\"plain\":\"\\n// Decoder-only controls distinguish a bound present empty value from no binding;\\n// these synthetic values are not Apple or compiler runtime observations.\\nfunc TestP10JSONPlainReferenceShape(t *testing.T) {\\n\\tvar id = \\\"db740247-8a09-480c-b7ec-11ff01483f13\\\"\\n\\tfor _, kind := range []string{\\\"ActionOutput\\\", \\\"Variable\\\"} {\\n\\t\\tfor _, tc := range []struct {\\n\\t\\t\\tname  string\\n\\t\\t\\tvalue any\\n\\t\\t}{{\\\"text\\\", \\\"value\\\"}, {\\\"empty_text\\\", \\\"\\\"}, {\\\"false\\\", false}, {\\\"zero\\\", float64(0)}} {\\n\\t\\t\\tt.Run(kind+\\\"/\\\"+tc.name, func(t *testing.T) {\\n\\t\\t\\t\\tvar ref = map[string]any{\\\"Type\\\": kind}\\n\\t\\t\\t\\tvar identity = \\\"source\\\"\\n\\t\\t\\t\\tif kind == \\\"ActionOutput\\\" {\\n\\t\\t\\t\\t\\tidentity = id\\n\\t\\t\\t\\t\\tref[\\\"OutputUUID\\\"] = id\\n\\t\\t\\t\\t\\tref[\\\"OutputName\\\"] = \\\"display name\\\"\\n\\t\\t\\t\\t} else {\\n\\t\\t\\t\\t\\tref[\\\"VariableName\\\"] = identity\\n\\t\\t\\t\\t}\\n\\t\\t\\t\\tvar value, err = p10DecodeJSONReference(ref, map[string]any{kind + \\\":\\\" + identity: tc.value})\\n\\t\\t\\t\\tif err != nil || !reflect.DeepEqual(value, tc.value) {\\n\\t\\t\\t\\t\\tt.Fatalf(\\\"present plain value changed: %#v %v\\\", value, err)\\n\\t\\t\\t\\t}\\n\\t\\t\\t})\\n\\t\\t}\\n\\t}\\n\\tfor _, fault := range []string{\\\"name_type\\\", \\\"empty_name\\\", \\\"extra_output_uuid\\\", \\\"extra_output_name\\\", \\\"property\\\", \\\"unknown_field\\\", \\\"missing_binding\\\", \\\"nil_binding\\\"} {\\n\\t\\tt.Run(\\\"variable_reject/\\\"+fault, func(t *testing.T) {\\n\\t\\t\\tvar ref = map[string]any{\\\"Type\\\": \\\"Variable\\\", \\\"VariableName\\\": \\\"source\\\"}\\n\\t\\t\\tvar resolved = map[string]any{\\\"Variable:source\\\": \\\"\\\"}\\n\\t\\t\\tif value, err := p10DecodeJSONReference(ref, resolved); err != nil || value != \\\"\\\" {\\n\\t\\t\\t\\tt.Fatal(\\\"unchanged plain reference rejected\\\", value, err)\\n\\t\\t\\t}\\n\\t\\t\\tswitch fault {\\n\\t\\t\\tcase \\\"name_type\\\":\\n\\t\\t\\t\\tref[\\\"VariableName\\\"] = 7\\n\\t\\t\\tcase \\\"empty_name\\\":\\n\\t\\t\\t\\tref[\\\"VariableName\\\"] = \\\"\\\"\\n\\t\\t\\tcase \\\"extra_output_uuid\\\":\\n\\t\\t\\t\\tref[\\\"OutputUUID\\\"] = id\\n\\t\\t\\tcase \\\"extra_output_name\\\":\\n\\t\\t\\t\\tref[\\\"OutputName\\\"] = \\\"unreviewed\\\"\\n\\t\\t\\tcase \\\"property\\\":\\n\\t\\t\\t\\tref[\\\"Aggrandizements\\\"] = []any{map[string]any{\\\"Type\\\": \\\"WFPropertyVariableAggrandizement\\\", \\\"PropertyName\\\": \\\"Name\\\"}}\\n\\t\\t\\tcase \\\"unknown_field\\\":\\n\\t\\t\\t\\tref[\\\"UnknownMetadata\\\"] = \\\"unreviewed\\\"\\n\\t\\t\\tcase \\\"missing_binding\\\":\\n\\t\\t\\t\\tdelete(resolved, \\\"Variable:source\\\")\\n\\t\\t\\tcase \\\"nil_binding\\\":\\n\\t\\t\\t\\tresolved[\\\"Variable:source\\\"] = nil\\n\\t\\t\\t}\\n\\t\\t\\tif value, err := p10DecodeJSONReference(ref, resolved); err == nil {\\n\\t\\t\\t\\tt.Fatalf(\\\"unsupported variable reference accepted: %#v\\\", value)\\n\\t\\t\\t}\\n\\t\\t})\\n\\t}\\n}\\n\"}")
p=r/"p10_http_json_oracle_test.go";text=p.read_text(encoding="utf-8")
start=text.index("func p10DecodeJSONReference(");end=text.index("\nfunc p10DecodeJSONText(",start)
text=text[:start]+data["reference"]+"\n"+text[end:]
start=text.index("// Extracted without changing the original map-based counting behavior")
end=text.index("\nfunc TestP10JSONProducerBindingIntegrity(",start)
text=text[:start]+data["binding"]+"\n"+text[end:]
old='"reference_property", "reference_coercion", "missing_array"'
new='"reference_property", "reference_coercion", "reference_unknown_field", "reference_type_type", "reference_uuid_type", "reference_name_type", "reference_missing_uuid", "reference_extra_variable_name", "reference_empty_aggrandizements", "missing_array"'
assert text.count(old)==1;text=text.replace(old,new,1)
old='\t\t\tcase "reference_property", "reference_coercion":\n'
new='''\t\t\tcase "reference_unknown_field", "reference_type_type", "reference_uuid_type", "reference_name_type", "reference_missing_uuid", "reference_extra_variable_name", "reference_empty_aggrandizements":
\t\t\t\tmodel := p10JSONWireField(t, copy, "model")["WFValue"].(map[string]any)
\t\t\t\tref := model["Value"].(map[string]any)
\t\t\t\tswitch fault {
\t\t\t\tcase "reference_unknown_field":
\t\t\t\t\tref["UnknownMetadata"] = "unreviewed"
\t\t\t\tcase "reference_type_type":
\t\t\t\t\tref["Type"] = 7
\t\t\t\tcase "reference_uuid_type":
\t\t\t\t\tref["OutputUUID"] = 7
\t\t\t\tcase "reference_name_type":
\t\t\t\t\tref["OutputName"] = []any{"dynamicModel"}
\t\t\t\tcase "reference_missing_uuid":
\t\t\t\t\tdelete(ref, "OutputUUID")
\t\t\t\tcase "reference_extra_variable_name":
\t\t\t\t\tref["VariableName"] = "unreviewed"
\t\t\t\tcase "reference_empty_aggrandizements":
\t\t\t\t\tref["Aggrandizements"] = []any{}
\t\t\t\t}
\t\t\tcase "reference_property", "reference_coercion":
'''
assert text.count(old)==1;text=text.replace(old,new,1)
old='"wrong_name", "missing_uuid"}'
new='"wrong_name", "missing_uuid", "wrong_uuid_type", "empty_uuid", "consumer_uuid_type", "wrong_output_name_type", "missing_model_parameters", "missing_consumer_parameters", "malformed_action_identifier", "duplicate_nonmodel_uuid"}'
assert text.count(old)==1;text=text.replace(old,new,1)
old='''\t\t\tcase "missing_uuid":
\t\t\t\tdelete(fields, "UUID")
'''
new='''\t\t\tcase "missing_uuid":
\t\t\t\tdelete(fields, "UUID")
\t\t\tcase "wrong_uuid_type":
\t\t\t\tfields["UUID"] = 7
\t\t\tcase "empty_uuid":
\t\t\t\tfields["UUID"] = ""
\t\t\tcase "consumer_uuid_type":
\t\t\t\tp15C01Params(changed[consumer])["UUID"] = []any{"unreviewed"}
\t\t\tcase "wrong_output_name_type":
\t\t\t\tp15C01Params(changed[consumer])["CustomOutputName"] = []any{"unreviewed"}
\t\t\tcase "missing_model_parameters":
\t\t\t\tdelete(changed[model], "WFWorkflowActionParameters")
\t\t\tcase "missing_consumer_parameters":
\t\t\t\tdelete(changed[consumer], "WFWorkflowActionParameters")
\t\t\tcase "malformed_action_identifier":
\t\t\t\tchanged[consumer]["WFWorkflowActionIdentifier"] = 7
\t\t\tcase "duplicate_nonmodel_uuid":
\t\t\t\tchanged = append(changed, p10CloneJSONWire(changed[consumer]).(map[string]any))
'''
assert text.count(old)==1;text=text.replace(old,new,1)
assert "func TestP10JSONPlainReferenceShape(" not in text
text+=data["plain"]
p.write_bytes(text.encode("utf-8"))
subprocess.run(["gofmt","-w",str(p)],cwd=r,check=True)
git("diff","--check")
changed=textgit("diff","--name-only").splitlines();assert changed==["p10_http_json_oracle_test.go"]
assert git("show",base+":p10_http_json_test.go")==(r/"p10_http_json_test.go").read_bytes()
diff=git("diff","--","p10_http_json_oracle_test.go")
d=pathlib.Path(r"C:\Users\DAVID\Code\cherri-p10r15-continuation-evidence-20261119\r5-json-oracle\strict-reference-fix");d.mkdir(parents=True,exist_ok=False)
(d/"component.patch").write_bytes(diff);(d/"submitted-script.py").write_bytes(pathlib.Path(__file__).read_bytes())
git("add","--","p10_http_json_oracle_test.go")
result=textgit("commit","-m","test: reject transformed JSON references and duplicate producer identities")
head=textgit("rev-parse","HEAD");assert not textgit("status","--porcelain")
record={"baseline":base,"source":head,"changed_paths":changed,"patch_sha256":hashlib.sha256(diff).hexdigest(),"original_request_test_unchanged":True,"source_clean":True,"commit":result}
(d/"receipt.json").write_bytes((json.dumps(record,indent=2)+"\n").encode())
print(json.dumps(record))
