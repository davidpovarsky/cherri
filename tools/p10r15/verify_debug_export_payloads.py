#!/usr/bin/env python3
"""Source-bound payload semantics. Does not execute Apple or the File boundary."""
import argparse, base64, copy, hashlib, json, sys
from pathlib import Path
import verify_debug_export_metadata as oracle

N, text, number = oracle.Native, oracle.text, oracle.number

def samples():
    date = N("Date", "2026-11-27T12:34:56Z")
    values = [
        text('שלום 🍒 "quoted"\\slash\nsecond line'),
        number(0), number(-2.5), date, N("Bool", True), N("Bool", False),
        N("File", b"\x00SECRET_BINARY\xff"), N("Image", b"SECRET_IMAGE"),
    ]
    values[0].creation = (date,)
    dictionary = N("Dictionary", {
        "public": [text("PUBLIC_TEXT_SENTINEL")],
        "amount": [number(0)], "date": [date],
        "bool": [N("Bool", False)], "empty": [],
        "multiple": [number(1), number(2)],
        "nested": [N("Dictionary", {"token":[text("NESTED_SECRET")]})],
        "file": [N("File", b"SECRET_FILE")],
        "authorization": [text("CREDENTIAL_SENTINEL")],
        "HEADERS": [N("Dictionary", {"x-api-key":[text("HEADER_SENTINEL")]})],
        "URL": [text("https://example.invalid/?token=QUERY_SENTINEL")],
        "My.Secret": [number(901)],
        "סוד": [text("CUSTOM_SENTINEL")],
        "a.b": [text("DOTTED_SENTINEL")],
        "literal[0]": [text("BRACKET_TEXT")],
    })
    dictionary.creation = (date,)
    return {
        "empty": [],
        "scalars": values,
        "structured": [dictionary, N("Dictionary", {}), text("")],
        "combined": values + [dictionary],
    }

def check(record, items):
    actions = record["actions"]
    labels = [oracle.label_range(1, len(items), "LABEL שלום 🍒")] if items and record["metadata"] else []
    before = copy.deepcopy((items, labels))
    ids = [id(x) for x in items]
    model = oracle.GraphModel(items, labels)
    boundary = oracle.content_boundary(actions)
    completed = oracle.scalar(model.run(actions, end=boundary+1), "Text").value
    if record["format"] == "json":
        document = json.loads(completed)
        assert document["scope"] == "run-local-native-items"
        assert document["payloadPolicy"] == oracle.payload_policy(record["redact"], record["keys"])
        assert document["sourceTypePolicy"] == "source-types-not-reconstructed"
        assert document["collectionPolicy"] == "native-item-stream"
        assert document["nullPolicy"] == "absence-not-reconstructed"
        rows = document["entries"]
    else:
        header, body = completed.split("\n\n", 1)
        assert "source-types-not-reconstructed" in header
        rows = [json.loads(line) for line in body.splitlines()]
    assert len(rows) == len(items)
    expected_formats = 0
    for row, item in zip(rows, items):
        expected_formats += oracle.inspect_payload(row, item, True, record["redact"], record["keys"])
        assert oracle.decode(row["nativeType"]) == "native:"+item.kind
        if record["metadata"]:
            date = row["date"]
            assert date["source"] == "source-creation-date"
            if item.creation:
                assert date["status"] == "PRESENT" and oracle.decode(date) == item.creation[0].value
                expected_formats += 1
            else:
                assert date == {"source":"source-creation-date","status":"ABSENT","value":None}
            label = row["label"]
            if record["redact"]:
                assert label == {"status":"REDACTED","value":None}
            else:
                assert label["status"] == "PRESENT" and oracle.decode(label) == "LABEL שלום 🍒"
        else:
            assert "date" not in row and "label" not in row
    assert model.date_formats == expected_formats
    if record["redact"]:
        assert model.label_reads == model.label_value_reads == 0
    protected = oracle.PROTECTED_KEYS.union(key.casefold() for key in record["keys"])
    for item in items:
        if item.kind == "Dictionary":
            reads = [key for source, key in model.dictionary_field_reads if source == id(item)]
            if record["keys"]:
                assert not protected.intersection(key.casefold() for key in reads), "sensitive field read before redaction"
            assert not any("." in key for key in reads), "dotted field was interpreted as key path"
    assert before == (items, labels) and ids == [id(x) for x in items]
    field_count = sum(len(item.value) for item in items if item.kind == "Dictionary")
    assert model.executed <= 240*(len(items)+field_count+len(labels))+1000
    return {"items":len(items),"fields":field_count,"executed":model.executed,"textBytes":len(completed.encode()),
            "sha256":hashlib.sha256(completed.encode()).hexdigest()}

def mutate_parameter(record, suffix, key, old, new):
    matches = [a for a in record["actions"] if a["WFWorkflowActionIdentifier"] == oracle.PREFIX+suffix
               and a["WFWorkflowActionParameters"].get(key) == old]
    assert matches, (suffix,key,old)
    matches[0]["WFWorkflowActionParameters"][key] = new

def faults(record):
    def sensitive(r):
        actions=[a for a in r["actions"] if a["WFWorkflowActionIdentifier"]==oracle.PREFIX+"text.match"
                 and "authorization" in a["WFWorkflowActionParameters"]["WFMatchTextPattern"]]
        assert len(actions)==1
        actions[0]["WFWorkflowActionParameters"]["WFMatchTextPattern"]="^NEVER_A_FIELD$"
    def dynamic_key(r):
        actions=[a for a in r["actions"] if a["WFWorkflowActionIdentifier"]==oracle.PREFIX+"getvalueforkey"
                 and isinstance(a["WFWorkflowActionParameters"].get("WFDictionaryKey"),dict)]
        assert len(actions)==1
        actions[0]["WFWorkflowActionParameters"]["WFDictionaryKey"]="amount"
    def date_source(r):
        actions=[a for a in r["actions"] if a["WFWorkflowActionIdentifier"]==oracle.PREFIX+"format.date"]
        assert actions
        actions[0]["WFWorkflowActionParameters"]["WFDate"]={"WFSerializationType":"WFTextTokenAttachment","Value":{"Type":"CurrentDate"}}
    def duplicate(r):
        outputs=[a["WFWorkflowActionParameters"] for a in r["actions"] if "UUID" in a["WFWorkflowActionParameters"]]
        outputs[1]["UUID"]=outputs[0]["UUID"]
    def raw_state(r):
        r["actions"].insert(0,{"WFWorkflowActionIdentifier":oracle.PREFIX+"setvariable",
                              "WFWorkflowActionParameters":{"WFVariableName":oracle.RAW}})
    def network(r):
        r["actions"].insert(0,{"WFWorkflowActionIdentifier":oracle.PREFIX+"downloadurl",
                              "WFWorkflowActionParameters":{"WFURL":"https://example.invalid"}})
    changes = [
        ("sensitive-field-gate-removed",sensitive),
        ("dynamic-key-replaced",dynamic_key),
        ("key-path-guard-removed",lambda r: mutate_parameter(r,"text.match","WFMatchTextPattern","\\.","^NEVER$")),
        ("date-source-replaced",date_source),
        ("date-format-weakened",lambda r: mutate_parameter(r,"format.date","WFISO8601IncludeTime",True,False)),
        ("base64-line-breaks",lambda r: mutate_parameter(r,"base64encode","WFBase64LineBreakMode","None","Every 76 Characters")),
        ("duplicate-producer",duplicate),("raw-buffer-write",raw_state),("network-inserted",network),
    ]
    results=[]
    for name, mutate in changes:
        changed=copy.deepcopy(record)
        mutate(changed)
        try:
            check(changed,samples()["combined"])
        except (AssertionError, ValueError, KeyError, TypeError, IndexError) as error:
            results.append({"name":name,"status":"REJECTED","error":type(error).__name__})
        else:
            raise AssertionError("causal fault was accepted: "+name)
    return results

def main():
    parser=argparse.ArgumentParser()
    parser.add_argument("--graphs",type=Path,required=True)
    parser.add_argument("--report",type=Path,required=True)
    args=parser.parse_args()
    sources=sorted(args.graphs.glob("*.json"))
    assert len(sources)==16, "all original two-frontend graph configurations are required"
    records=[]
    for file in sources:
        content=file.read_bytes();record=json.loads(content)
        records.append((file,record,hashlib.sha256(content).hexdigest()))
    cases=[]
    for file,record,digest in records:
        for name,items in samples().items():
            cases.append({"graph":file.name,"graphSHA256":digest,"case":name,**check(record,items)})
    stress=[]
    for file,record,digest in records:
        if record["metadata"] and not record["legacy"] and record["keys"]:
            items=(samples()["combined"]*112)[:1000]
            stress.append({"graph":file.name,"graphSHA256":digest,**check(record,items)})
    chosen=next(record for _,record,_ in records if record["keys"] and not record["metadata"] and not record["redact"])
    controls=faults(chosen)
    result={"schema":"cherri.debug-export-payload-model/1","status":"PASS_TEXT_ENVELOPE_MODEL_ONLY",
            "graphs":16,"cases":cases,"stress":stress,"causalFaults":controls,
            "nativeRuntimeExecuted":False,"fileClassAccepted":False}
    args.report.write_text(json.dumps(result,ensure_ascii=False,indent=2)+"\n")
    print(json.dumps({"status":result["status"],"graphs":16,"cases":len(cases),"stress":len(stress),"faults":len(controls),
                      "nativeRuntimeExecuted":False,"fileClassAccepted":False}))
if __name__=="__main__":main()
