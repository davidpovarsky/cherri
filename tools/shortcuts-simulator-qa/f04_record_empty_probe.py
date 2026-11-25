"""A public Dictionary-carrier feasibility probe; never F04 nested-list acceptance."""
import copy
import hashlib
from pathlib import Path

from native_comparison_graph import verify_equality_region

FIXTURE_ID = "qa-f04-record-empty-v2"
SAMPLE_ID = "qa-f04-record-empty"
EXPECTED = "CHERRI_QA_F04_RECORD_EMPTY_RESULT:outer=2:scalars=2:tags=LEFT,RIGHT:keys=2,2:items=0,0:empty=0:dictionary=Y"
SOURCE_SHA256 = "71e107ee20de044274bac55b15439fe86cdb06b6b793d29a7beec49b38d320f4"
ORIGINALS = {
    "qa-f04-yield-list-v2.cherri": "c0719b046c2ed724fccef9e5b09e313c7dd72c02dc2c72ce1abe8aab1592e091",
    "qa-f04-yield-empty-list-v2.cherri": "d1322f6c7c663d266b0e9a7e272a92ae331be0e411818dae0c7d5172587578c3",
}

def verify_source(root, fid, actual):
    if fid != FIXTURE_ID: return None
    if hashlib.sha256(actual).hexdigest() != SOURCE_SHA256:
        raise ValueError("F04_RECORD_EMPTY_SOURCE_CONTRACT")
    folder = Path(root) / "tools/shortcuts-simulator-qa/fixtures"
    for name, digest in ORIGINALS.items():
        if hashlib.sha256((folder/name).read_bytes()).hexdigest() != digest:
            raise ValueError("F04_ORIGINAL_ORACLE_CHANGED")
    return {"fixtureSha256": SOURCE_SHA256, "originalF04SourcesUnchanged": True,
            "carrier": "Dictionary", "formalF04Acceptance": False, "appleRuntime": "NOT_RUN"}

def verify_graph(fid, actions):
    if fid != FIXTURE_ID: return None
    def require(ok, why):
        if not ok: raise ValueError("F04_RECORD_EMPTY_GRAPH_CONTRACT: " + why)
    def p(a): return a.get("WFWorkflowActionParameters", {})
    def kind(a, suffix): return a.get("WFWorkflowActionIdentifier") == "is.workflow.actions." + suffix
    # A source and reference both missing UUID must never compare equal.
    # GroupingIdentifier is deliberately separate: control groups repeat it.
    uuids=[p(a)["UUID"] for a in actions if "UUID" in p(a)]
    require(all(isinstance(value,str) and bool(value.strip()) for value in uuids),
            "nonempty producer UUID")
    require(len(uuids)==len(set(uuids)), "unique producer UUID")
    require(all(isinstance(p(a).get("UUID"),str) and bool(p(a)["UUID"].strip())
                for a in actions if p(a).get("CustomOutputName")), "named producer UUID")
    def named(name, suffix):
        found = [(i,a) for i,a in enumerate(actions) if p(a).get("CustomOutputName")==name]
        require(len(found)==1 and kind(found[0][1],suffix), "named producer " + name)
        return found[0]
    def att(value):
        require(isinstance(value,dict) and value.get("WFSerializationType")=="WFTextTokenAttachment", "attachment")
        return value.get("Value",{})
    def edge(value, source):
        t=att(value)
        producer_uuid=p(source).get("UUID")
        require(isinstance(producer_uuid,str) and bool(producer_uuid.strip()), "edge producer UUID")
        require(t.get("Type")=="ActionOutput" and t.get("OutputUUID")==producer_uuid
                and not t.get("Aggrandizements"), "producer edge")
    def dictionary(a, tag):
        require(kind(a,"dictionary"), "Dictionary carrier")
        serialized=p(a).get("WFItems",{})
        require(serialized.get("WFSerializationType")=="WFDictionaryFieldValue", "Dictionary serialization")
        fields=serialized.get("Value",{}).get("WFDictionaryFieldValueItems",[])
        require(len(fields)==2, "two preserved keys")
        by_key={x.get("WFKey",{}).get("Value",{}).get("string"):x for x in fields}
        require(set(by_key)=={"tag","items"}, "tag and items keys")
        empty=by_key["items"]
        require(empty.get("WFItemType")==2 and empty.get("WFValue")=={
            "WFSerializationType":"WFArrayParameterState","Value":[]}, "explicit empty Array field")
        field=by_key["tag"]
        require(field.get("WFItemType")==0, "Text tag field")
        if tag=="item": require(att(field.get("WFValue"))=={"Type":"Variable","VariableName":"item"}, "iteration tag")
        else: require(field.get("WFValue")=={"WFSerializationType":"WFTextTokenString","Value":{"string":"CONTROL"}}, "independent control tag")
    _,inputs=named("inputs","list")
    require(p(inputs).get("WFItems")==[{"WFItemType":0,"WFValue":"LEFT"},{"WFItemType":0,"WFValue":"RIGHT"}], "two distinct inputs")
    loops={}
    for name in ("boxes","scalars"):
        end_i,end=named(name,"repeat.each");group=p(end).get("GroupingIdentifier")
        members=[(i,a) for i,a in enumerate(actions) if p(a).get("GroupingIdentifier")==group]
        require(bool(group) and len(members)==2 and [p(a).get("WFControlFlowMode") for _,a in members]==[0,2]
                and members[1][0]==end_i and members[0][0]+3==end_i, "one result per iteration")
        start_i,start=members[0];edge(p(start).get("WFInput"),inputs)
        bind=actions[start_i+1]
        require(kind(bind,"setvariable") and p(bind).get("WFVariableName")=="item"
                and att(p(bind).get("WFInput"))=={"Type":"Variable","VariableName":"Repeat Item"}, "iteration binding")
        yielded=actions[end_i-1]
        if name=="boxes": dictionary(yielded,"item")
        else: require(kind(yielded,"getvariable") and att(p(yielded).get("WFVariable"))=={"Type":"Variable","VariableName":"item"}, "scalar control")
        loops[name]=end
    producers=dict(loops)
    for name,which in (("firstBox","First Item"),("lastBox","Last Item")):
        _,a=named(name,"getitemfromlist");edge(p(a).get("WFInput"),loops["boxes"])
        require(p(a).get("WFItemSpecifier")==which, "first and last record identity")
        producers[name]=a
    for side in ("first","last"):
        for suffix,key in (("Tag","tag"),("Items","items"),("Keys",None)):
            name=side+suffix;_,a=named(name,"getvalueforkey");edge(p(a).get("WFInput"),producers[side+"Box"])
            if key is None: require(p(a).get("WFGetDictionaryValueType")=="All Keys" and "WFDictionaryKey" not in p(a), "actual key enumeration")
            else: require(p(a).get("WFDictionaryKey")==key and p(a).get("WFGetDictionaryValueType","Value")=="Value", "actual field read")
            producers[name]=a
    empty_i,empty=named("directEmpty","getvariable")
    empty_name=att(p(empty).get("WFVariable")).get("VariableName")
    require(empty_i>=2 and kind(actions[empty_i-2],"nothing") and kind(actions[empty_i-1],"setvariable")
            and p(actions[empty_i-1])=={"WFVariableName":empty_name}
            and bool(empty_name) and empty_name.startswith("__cherri_scope_"), "independent empty control")
    producers["directEmpty"]=empty
    count_inputs={"outerCount":"boxes","scalarCount":"scalars","firstKeyCount":"firstKeys","lastKeyCount":"lastKeys",
                  "firstItemCount":"firstItems","lastItemCount":"lastItems","directEmptyCount":"directEmpty"}
    for name,input_name in count_inputs.items():
        _,a=named(name,"count");edge(p(a).get("Input"),producers[input_name])
        require(p(a).get("WFCountType","Items")=="Items", "item cardinality")
        producers[name]=a
    _,witness=named("dictionaryWitness","dictionary");dictionary(witness,"CONTROL")
    for name,src in (("witnessType",witness),("firstType",producers["firstBox"]),("lastType",producers["lastBox"])):
        _,a=named(name,"getitemtype");edge(p(a).get("WFInput"),src);producers[name]=a
    writes=[(i,a) for i,a in enumerate(actions) if kind(a,"setvariable") and p(a).get("WFVariableName")=="recordsAreDictionaries"]
    require(len(writes)==2, "two type-parity writes")
    init_i,_=writes[0];pass_i,_=writes[1]
    require(init_i > 0 and pass_i > init_i + 3 and pass_i + 2 < len(actions)
            and kind(actions[init_i-1],"gettext") and p(actions[init_i-1]).get("WFTextActionText")=="N"
            and kind(actions[pass_i-1],"gettext") and p(actions[pass_i-1]).get("WFTextActionText")=="Y", "fail-closed type parity")
    for i,a in writes: edge(p(a).get("WFInput"),actions[i-1])
    comparison_start = init_i + 1
    for name, close_i in (("firstType",pass_i+2),("lastType",pass_i+1)):
        closing=actions[close_i]
        group=p(closing).get("GroupingIdentifier")
        members=[(i,a) for i,a in enumerate(actions) if p(a).get("GroupingIdentifier")==group]
        require(bool(group) and len(members)==2 and all(kind(a,"conditional") for _,a in members)
                and [p(a).get("WFControlFlowMode") for _,a in members]==[0,2]
                and members[-1][0]==close_i, "nested type comparison")
        condition_i,_=members[0]
        verify_equality_region(actions, comparison_start, condition_i, producers[name],
                               require=require, right_source=producers["witnessType"])
        comparison_start=condition_i+1
    require(comparison_start==pass_i-1, "type parity assignments remain inside both comparisons")
    def text_edges(value, template, names):
        require(isinstance(value,dict) and value.get("WFSerializationType")=="WFTextTokenString", "dynamic text")
        v=value.get("Value",{});require(v.get("string")==template, "exact result template")
        atts=v.get("attachmentsByRange",{});positions=[i for i,c in enumerate(template) if c=="\ufffc"]
        require(len(atts)==len(names)==len(positions), "one attachment per observation")
        for pos,name in zip(positions,names):
            t=atts.get("{%d, 1}"%pos,{})
            if name=="recordsAreDictionaries":require(t=={"Type":"Variable","VariableName":name}, "observed type parity")
            else:require(t.get("Type")=="ActionOutput" and t.get("OutputUUID")==p(producers[name]).get("UUID")
                         and not t.get("Aggrandizements"), "observed result " + name)
    _,semantic=named("semanticResult","gettext");producers["semanticResult"]=semantic
    text_edges(p(semantic).get("WFTextActionText"),"outer=\ufffc:scalars=\ufffc:tags=\ufffc,\ufffc:keys=\ufffc,\ufffc:items=\ufffc,\ufffc:empty=\ufffc:dictionary=\ufffc",
               ["outerCount","scalarCount","firstTag","lastTag","firstKeyCount","lastKeyCount","firstItemCount","lastItemCount","directEmptyCount","recordsAreDictionaries"])
    _,trace=named("traceEnvelope","gettext");producers["traceEnvelope"]=trace
    _,summary=named("finalSummary","gettext");producers["finalSummary"]=summary
    text_edges(p(summary).get("WFTextActionText"),"CHERRI_QA_F04_RECORD_EMPTY_RESULT:\ufffc\n\ufffc",["semanticResult","traceEnvelope"])
    require(kind(actions[-1],"output"), "actual callback")
    text_edges(p(actions[-1]).get("WFOutput"),"\ufffc",["finalSummary"])
    return {"status":"PASS_LOCAL_DICTIONARY_GRAPH_ONLY","nativeActions":len(actions),"carrier":"Dictionary",
            "twoExplicitEmptyArrayFields":True,"actualFieldReads":True,"formalF04Acceptance":False,"appleRuntime":"NOT_RUN"}

def verify_causal_mutations(actions):
    mutations=("list-carrier","missing-items-key","wrong-item-type","nonempty-inner","wrong-iteration-tag",
               "wrong-outer-count","wrong-last-record","wrong-items-read","wrong-keys-mode","wrong-empty-control",
               "wrong-type-witness","unconditional-type-pass","hardcoded-summary","wrong-output-edge",
               "missing-producer-uuid","empty-producer-uuid","duplicate-producer-uuid")
    rejected=[]
    def named(graph,name):
        return next(a["WFWorkflowActionParameters"] for a in graph if a.get("WFWorkflowActionParameters",{}).get("CustomOutputName")==name)
    def change_references(value, old, new):
        if isinstance(value,dict):
            if value.get("OutputUUID")==old:
                if new is None: value.pop("OutputUUID")
                else: value["OutputUUID"]=new
            for item in list(value.values()): change_references(item,old,new)
        elif isinstance(value,list):
            for item in value: change_references(item,old,new)
    for mutation in mutations:
        damaged=copy.deepcopy(actions)
        dictionary=next(a for a in damaged if a["WFWorkflowActionIdentifier"]=="is.workflow.actions.dictionary")
        fields=dictionary["WFWorkflowActionParameters"]["WFItems"]["Value"]["WFDictionaryFieldValueItems"]
        items=next(f for f in fields if f["WFKey"]["Value"]["string"]=="items")
        tag=next(f for f in fields if f["WFKey"]["Value"]["string"]=="tag")
        if mutation=="list-carrier":dictionary["WFWorkflowActionIdentifier"]="is.workflow.actions.list"
        elif mutation=="missing-items-key":fields.remove(items)
        elif mutation=="wrong-item-type":items["WFItemType"]=0
        elif mutation=="nonempty-inner":items["WFValue"]["Value"]=[{"WFItemType":0,"WFValue":"wrong"}]
        elif mutation=="wrong-iteration-tag":tag["WFValue"]["Value"]["VariableName"]="stale"
        elif mutation=="wrong-outer-count":named(damaged,"outerCount")["Input"]["Value"]["OutputUUID"]=named(damaged,"scalars")["UUID"]
        elif mutation=="wrong-last-record":named(damaged,"lastBox")["WFItemSpecifier"]="First Item"
        elif mutation=="wrong-items-read":named(damaged,"firstItems")["WFDictionaryKey"]="missing"
        elif mutation=="wrong-keys-mode":named(damaged,"firstKeys")["WFGetDictionaryValueType"]="All Values"
        elif mutation=="wrong-empty-control":named(damaged,"directEmpty")["WFVariable"]["Value"]["VariableName"]="wrong"
        elif mutation=="wrong-type-witness":named(damaged,"witnessType")["WFInput"]["Value"]["OutputUUID"]=named(damaged,"inputs")["UUID"]
        elif mutation=="unconditional-type-pass":
            for a in damaged:
                p=a.get("WFWorkflowActionParameters",{})
                if p.get("WFTextActionText")=="N":p["WFTextActionText"]="Y"
        elif mutation=="hardcoded-summary":named(damaged,"semanticResult")["WFTextActionText"]=EXPECTED
        elif mutation=="wrong-output-edge":damaged[-1]["WFWorkflowActionParameters"]["WFOutput"]["Value"]["attachmentsByRange"]["{0, 1}"]["OutputUUID"]="wrong"
        elif mutation in ("missing-producer-uuid","empty-producer-uuid","duplicate-producer-uuid"):
            source=named(damaged,"boxes");old=source["UUID"]
            new=None if mutation=="missing-producer-uuid" else ("" if mutation=="empty-producer-uuid" else named(damaged,"scalars")["UUID"])
            if new is None: source.pop("UUID")
            else: source["UUID"]=new
            # Mutate both ends, so the control tests identity integrity rather
            # than merely detecting a dangling edge to an unchanged producer.
            change_references(damaged,old,new)
        try:verify_graph(FIXTURE_ID,damaged)
        except ValueError as exc:
            if "F04_RECORD_EMPTY_GRAPH_CONTRACT" not in str(exc):raise
            rejected.append(mutation)
        else:raise AssertionError("damaged feasibility graph accepted: "+mutation)
    return rejected
