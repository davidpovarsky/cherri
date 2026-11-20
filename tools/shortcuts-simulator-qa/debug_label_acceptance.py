"""Causal gates for the reviewed, fixed six-mode label fixtures.

These gates inspect real compiler output. They do not claim Apple execution and
never classify arbitrary user-authored lookalikes as compiler instrumentation.
"""
import argparse
import copy
import hashlib
import json
from pathlib import Path
import re
import plistlib
import subprocess
import tempfile

LOG = "__cherri_debug_log"
META = "__cherri_debug_log_labels"
LABELS = ("TEXT_שָׁלוֹם_🍒", "", "MAP_LABEL", "LIST_LABEL", "DATE_LABEL", "FILE_LABEL")
PRODUCERS = ("textInput", "numberInput", "mapInput", "listInput", "dateInput", "fileInput")
MODES = {"plain": (False, False, False), "dated": (True, False, False),
         "typed": (False, True, False), "dated-typed": (True, True, False),
         "text": (False, False, True), "dated-text": (True, False, True)}
# Literal fixture digests and golden values are inserted after actual source review.
SOURCE_HASHES = {'qa-debug-label-plain-v2': '70cfa286cb19e5342e62acd3fa41371df376a5fe119fe0a359c0a96f81e98579',
 'qa-debug-label-dated-v2': '9a5674392e26be3d2165a9514520f3d7054427147d721f3a2ae63280771ef9df',
 'qa-debug-label-typed-v2': 'd86699260fd6d852db8995acc23e50ff4dbf1e7833955da40252bbb3293d26ec',
 'qa-debug-label-dated-typed-v2': '4a63249fdec08bea68d46a6201ed470ff6e37eb7afe388b19b1c1930951f2bd6',
 'qa-debug-label-text-v2': 'd915cc0db3c33a97e7f4c9787d4ec997aec9b5beb8ed1993f6ecde9b4136fc18',
 'qa-debug-label-dated-text-v2': '701d7a5a171bcd3db337e3ec01b2112b65e61a60e2917855578c04e9f8ca6de3',
 'qa-debug-label-dated-text-legacy': '28711046a7812917a33b12a9a6829345fd6517fc602c3766172813da3b3abd4b',
 'qa-debug-label-dated-typed-legacy': '4037ea0567230276f388d0c904cb86c9ad8682881bb60b55bc86656420d8064b',
 'qa-debug-label-dated-legacy': '05e9143edd5da4204adcd63336aab75718559e4e761416377445dc1e427dcd33',
 'qa-debug-label-plain-legacy': '8d5c7723bb3698dca0c31702e1fc7ffadc2d422945984f078cfb089c52d53592',
 'qa-debug-label-text-legacy': '6c6527ea063a778fbaee12507b721cf96a2b859625ab201da1a602a5762fb7cc',
 'qa-debug-label-typed-legacy': '02c7d600615fd5fcfc97a5a3ef028d29694878923d8220e314e86b62529f9fe0'}
FIELDS = (('BUFFER_COUNT', 'bufferCount', '7'),
 ('TEXT_TYPE64', 'textType64', 'VGV4dA=='),
 ('TEXT64', 'text64', '16LXqNeaIPCfjZI='),
 ('NUMBER_TYPE64', 'numberType64', 'TnVtYmVy'),
 ('NUMBER', 'observedNumber', '7.25'),
 ('MAP_TYPE64', 'mapType64', 'RGljdGlvbmFyeQ=='),
 ('MAP_KEYS', 'mapKeysCount', '2'),
 ('MAP_KIND64', 'mapKind64', 'TEFCRUxfTUFQ'),
 ('MAP_VALUE', 'mapNumber', '7.25'),
 ('LIST_COUNT', 'listCount', '2'),
 ('LIST_FIRST64', 'listFirst64', 'bGVmdA=='),
 ('LIST_LAST64', 'listLast64', 'cmlnaHQ='),
 ('DATE_TYPE64', 'dateType64', 'RGF0ZQ=='),
 ('DATE_COUNT', 'dateCount', '1'),
 ('DATE_DELTA_SECONDS', 'dateDelta', '0'),
 ('DATE_SOURCE_DELTA_SECONDS', 'dateSourceDelta', '0'),
 ('FILE_TYPE64', 'fileType64', 'RmlsZQ=='),
 ('FILE_COUNT', 'fileCount', '1'),
 ('FILE_BYTES64', 'fileBytes64', 'AQIDBA=='),
 ('FILE_NAME64', 'fileName64', 'bGFiZWwtc291cmNlLmJpbg=='),
 ('SOURCE_FILE_TYPE64', 'sourceFileType64', 'RmlsZQ=='),
 ('SOURCE_FILE_BYTES64', 'sourceFileBytes64', 'AQIDBA=='),
 ('SOURCE_FILE_NAME64', 'sourceFileName64', 'bGFiZWwtc291cmNlLmJpbg=='),
 ('METADATA_COUNT', 'metadataCount', '6'),
 ('M1_KEYS', 'm1Keys', '3'),
 ('M1_FIRST', 'm1First', '1'),
 ('M1_LAST', 'm1Last', '1'),
 ('M1_LABEL64', 'm1Label64', 'VEVYVF/Xqda414HXnNeV1rnXnV/wn42S'),
 ('M2_KEYS', 'm2Keys', '3'),
 ('M2_FIRST', 'm2First', '2'),
 ('M2_LAST', 'm2Last', '2'),
 ('M2_LABEL64', 'm2Label64', ''),
 ('M3_KEYS', 'm3Keys', '3'),
 ('M3_FIRST', 'm3First', '3'),
 ('M3_LAST', 'm3Last', '3'),
 ('M3_LABEL64', 'm3Label64', 'TUFQX0xBQkVM'),
 ('M4_KEYS', 'm4Keys', '3'),
 ('M4_FIRST', 'm4First', '4'),
 ('M4_LAST', 'm4Last', '5'),
 ('M4_LABEL64', 'm4Label64', 'TElTVF9MQUJFTA=='),
 ('M5_KEYS', 'm5Keys', '3'),
 ('M5_FIRST', 'm5First', '6'),
 ('M5_LAST', 'm5Last', '6'),
 ('M5_LABEL64', 'm5Label64', 'REFURV9MQUJFTA=='),
 ('M6_KEYS', 'm6Keys', '3'),
 ('M6_FIRST', 'm6First', '7'),
 ('M6_LAST', 'm6Last', '7'),
 ('M6_LABEL64', 'm6Label64', 'RklMRV9MQUJFTA=='),
 ('RAW_CLEAR', 'rawClearedCount', '0'),
 ('METADATA_CLEAR', 'metadataClearedCount', '0'))


def require(ok, message):
    if not ok:
        raise ValueError("DEBUG_LABEL_CONTRACT: " + message)


def params(action):
    require(isinstance(action, dict), "action object")
    value = action.get("WFWorkflowActionParameters", {})
    require(isinstance(value, dict), "action parameters")
    return value


def kind(action):
    return action.get("WFWorkflowActionIdentifier", "").removeprefix("is.workflow.actions.")


def token(value):
    require(isinstance(value, dict), "attachment object")
    if value.get("WFSerializationType") == "WFTextTokenString":
        v = value.get("Value", {})
        require(v.get("string") == "\ufffc" and set(v.get("attachmentsByRange", {})) == {"{0, 1}"}, "single attachment text")
        return v["attachmentsByRange"]["{0, 1}"]
    require(value.get("WFSerializationType") == "WFTextTokenAttachment" and isinstance(value.get("Value"), dict), "native attachment")
    return value["Value"]


def text_value(value):
    if isinstance(value, str):
        return {"string": value}
    require(isinstance(value, dict) and value.get("WFSerializationType") == "WFTextTokenString"
            and isinstance(value.get("Value"), dict), "native token text")
    return value["Value"]


def literal(value, expected):
    require(text_value(value) == {"string": expected}, "literal bytes changed")


def variable(value, expected):
    require(token(value) == {"Type": "Variable", "VariableName": expected}, "variable edge " + expected)


def fixture_mode(fid):
    require(fid in SOURCE_HASHES, "unreviewed fixture")
    for mode in MODES:
        if fid in (f"qa-debug-label-{mode}-v2", f"qa-debug-label-{mode}-legacy"):
            return mode
    raise ValueError("DEBUG_LABEL_CONTRACT: unknown mode")


def expected_summary(fid):
    return "CHERRI_QA_DEBUG_LABEL:MODE=" + fixture_mode(fid) + "".join(":" + key + "=" + expected for key, _, expected in FIELDS)


def verify_source(repo, fid, source):
    if fid not in SOURCE_HASHES:
        return None
    path = Path(repo) / "tools/shortcuts-simulator-qa/fixtures" / (fid + ".cherri")
    require(source == path.read_bytes() and hashlib.sha256(source).hexdigest() == SOURCE_HASHES[fid], "reviewed source bytes")
    return {"sourceSha256": SOURCE_HASHES[fid], "mode": fixture_mode(fid),
            "syntheticOnly": True, "nativeExecution": "NOT_RUN", "formalAcceptanceGranted": False}


class Graph:
    def __init__(self, actions):
        require(isinstance(actions, list) and actions, "missing graph")
        self.actions = actions
        self.named = {}
        self.uuids = {}
        self.inline = set()
        self.collector = set()
        self.authored_nothing = set()
        for i, a in enumerate(actions):
            p = params(a)
            uid, name = p.get("UUID"), p.get("CustomOutputName")
            if uid:
                require(isinstance(uid, str) and uid not in self.uuids, "UUID missing or duplicated")
                self.uuids[uid] = i
            if name:
                self.named.setdefault(name, []).append(i)

    def get(self, name, expected_kind=None):
        rows = self.named.get(name, [])
        require(len(rows) == 1, "one named producer " + name)
        i = rows[0]
        require(params(self.actions[i]).get("UUID") in self.uuids, "named producer UUID " + name)
        if expected_kind:
            require(kind(self.actions[i]) == expected_kind, "producer kind " + name)
        return i

    def at(self, i, expected_kind):
        require(0 <= i < len(self.actions) and kind(self.actions[i]) == expected_kind, "ordered action " + expected_kind)
        return params(self.actions[i])

    def ref(self, value, producer, name=None):
        i = self.get(producer) if isinstance(producer, str) else producer
        p = params(self.actions[i]); t = token(value)
        require(t.get("Type") == "ActionOutput" and t.get("OutputUUID") == p.get("UUID")
                and set(t) <= {"Type", "OutputUUID", "OutputName"}, "raw source UUID edge " + str(producer))
        if name is not None:
            require(t.get("OutputName") == name, "original OutputName " + str(name))

    def linked(self, name, expected_kind, source, parameter="WFInput"):
        p = self.at(self.get(name), expected_kind)
        self.ref(p.get(parameter), source)
        return p

    def count(self, name, source, unit="Items"):
        p = self.linked(name, "count", source, "Input")
        require(p.get("WFCountType") == unit, "count units " + name)

    def encode(self, name, source):
        p = self.linked(name, "base64encode", source)
        require(p.get("WFEncodeMode") == "Encode" and p.get("WFBase64LineBreakMode") == "None", "actual byte encoding " + name)

    def clear(self, index, name):
        a, b = self.at(index, "nothing"), self.at(index+1, "setvariable")
        require(set(a) <= {"UUID", "CustomOutputName"} and set(b) <= {"UUID", "CustomOutputName", "WFVariableName"}
                and b.get("WFVariableName") == name, "exact zero-output clear")
        self.inline.update((index, index+1))

    def conditional(self, i, source, code, rhs):
        p = self.at(i, "conditional")
        require(p.get("WFControlFlowMode") == 0 and p.get("WFCondition") == code
                and p.get("WFConditionalLegacyComparisonBehavior") is True, "numeric range condition")
        inp = p.get("WFInput", {})
        require(inp.get("Type") == "Variable" and set(inp) == {"Type", "Variable"}, "range condition input")
        self.ref(inp["Variable"], source)
        if isinstance(rhs, tuple): self.ref(p.get("WFNumberValue"), rhs[0])
        else: require(p.get("WFNumberValue") == rhs, "range condition literal")
        return p["GroupingIdentifier"]

    def control(self, i, expected_kind, mode, group):
        p = self.at(i, expected_kind)
        require(p.get("WFControlFlowMode") == mode and p.get("GroupingIdentifier") == group, "control owner and mode")
        require(set(p) <= {"WFControlFlowMode", "GroupingIdentifier", "UUID", "CustomOutputName"}, "control end/else has no extra input")


def verify_preview(g, start, end, dates, as_text):
    # Inspect both the metadata branch and its real empty-metadata fallback.
    expected_len = 35 if dates else (33 if as_text else 32)
    require(end-start == expected_len, "bounded preview action count")
    a = g.actions
    p = g.at(start, "count"); variable(p.get("Input"), META); require(p.get("WFCountType") == "Items", "metadata count")
    outer_if = g.conditional(start+1, start, 2, 0)
    outer = g.at(start+2, "repeat.each"); require(outer.get("WFControlFlowMode") == 0, "outer repeat start"); variable(outer.get("WFInput"), LOG)
    variable(g.at(start+3, "getvariable").get("WFVariable"), "Repeat Item")
    variable(g.at(start+4, "getvariable").get("WFVariable"), "Repeat Index")
    literal(g.at(start+5, "gettext").get("WFTextActionText"), "")
    p = g.at(start+6, "setvariable"); display = p.get("WFVariableName")
    require(display == "__cherri_debug_label_display_" + outer["GroupingIdentifier"].replace("-", ""), "owned display variable")
    g.ref(p.get("WFInput"), start+5)
    inner = g.at(start+7, "repeat.each"); require(inner.get("WFControlFlowMode") == 0, "inner repeat start"); variable(inner.get("WFInput"), META)
    for offset, key in ((8,"first"),(9,"last"),(16,"label")):
        p = g.at(start+offset, "getvalueforkey")
        require(p.get("WFDictionaryKey") == key and p.get("WFGetDictionaryValueType") == "Value", "metadata field selector")
        variable(p.get("WFInput"), "Repeat Item")
    before = g.conditional(start+10, start+4, 0, (start+8,))
    after = g.conditional(start+13, start+4, 2, (start+9,))
    for offset in (11,14): require(set(g.at(start+offset,"nothing")) <= {"UUID"}, "skipped range has no fake item")
    g.control(start+12,"conditional",1,before);g.control(start+15,"conditional",1,after)
    p=g.at(start+17,"setvariable");require(p.get("WFVariableName")==display,"label belongs to outer snapshot");g.ref(p.get("WFInput"),start+16)
    g.control(start+18,"conditional",2,after);g.control(start+19,"conditional",2,before)
    g.control(start+20,"repeat.each",2,inner["GroupingIdentifier"])
    p=g.at(start+21,"count");variable(p.get("Input"),display);require(p.get("WFCountType")=="Characters","empty label presence")
    has_label=g.conditional(start+22,start+21,2,0)
    def item(offset,labeled):
        p=g.at(start+offset,"gettext" if as_text else ("setitemname" if labeled or dates else "getvariable"))
        if not as_text: g.ref(p.get("WFInput" if labeled or dates else "WFVariable"),start+3)
        if not as_text and not labeled and not dates: return
        value=text_value(p.get("WFTextActionText" if as_text else "WFName"))
        parts=[]
        if labeled: parts += [("variable",display)] + ([": "] if as_text else [])
        if as_text:parts += [("raw",start+3)]
        if dates:parts += ([" - "] if parts else [])+[("date",start+3)]
        expected="";attachments={}
        for part in parts:
            if isinstance(part,str):expected+=part;continue
            key="{%d, 1}" % (len(expected.encode("utf-16-le"))//2)
            expected+="\ufffc";attachments[key]=part
        require(value.get("string")==expected and set(value.get("attachmentsByRange",{}))==set(attachments),"preview literal/range text")
        for key,(role,target) in attachments.items():
            t=value["attachmentsByRange"][key]
            if role=="variable":require(t=={"Type":"Variable","VariableName":target},"per-entry visible label")
            else:
                base={k:v for k,v in t.items() if k!="Aggrandizements"}
                g.ref({"WFSerializationType":"WFTextTokenAttachment","Value":base},target)
                if role=="date":require(t.get("Aggrandizements")==[{"Type":"WFPropertyVariableAggrandizement","PropertyName":"Creation Date","PropertyUserInfo":"WFFileCreationDate"},{"Type":"WFDateFormatVariableAggrandizement","WFDateFormatStyle":"Medium","WFTimeFormatStyle":"Long","WFISO8601IncludeTime":False}],"creation date display")
                else:require("Aggrandizements" not in t,"typed text is explicit only")
    item(23,True);g.control(start+24,"conditional",1,has_label);item(25,False);g.control(start+26,"conditional",2,has_label)
    g.control(start+27,"repeat.each",2,outer["GroupingIdentifier"])
    g.ref(g.at(start+28,"previewdocument").get("WFInput"),start+27)
    g.control(start+29,"conditional",1,outer_if)
    fallback_start=start+30
    if dates:
        p=g.at(fallback_start,"repeat.each");require(p.get("WFControlFlowMode")==0,"dated fallback loop");variable(p.get("WFInput"),LOG)
        body=g.at(fallback_start+1,"gettext" if as_text else "setitemname")
        if not as_text:variable(body.get("WFInput"),"Repeat Item")
        value=text_value(body.get("WFTextActionText" if as_text else "WFName"))
        refs=value.get("attachmentsByRange",{});require(refs,"dated fallback references")
        require(all(t.get("Type")=="Variable" and t.get("VariableName")=="Repeat Item" for t in refs.values()),"fallback raw item")
        require(any(any(x.get("PropertyUserInfo")=="WFFileCreationDate" for x in t.get("Aggrandizements",[])) for t in refs.values()),"fallback creation date")
        g.control(fallback_start+2,"repeat.each",2,p["GroupingIdentifier"])
        g.ref(g.at(fallback_start+3,"previewdocument").get("WFInput"),fallback_start+2)
    elif as_text:
        p=g.at(fallback_start,"text.combine");variable(p.get("text"),LOG);require(p.get("WFTextSeparator")=="New Lines","text fallback separator")
        g.ref(g.at(fallback_start+1,"previewdocument").get("WFInput"),fallback_start)
    else:variable(g.at(fallback_start,"previewdocument").get("WFInput"),LOG)
    g.control(end-1,"conditional",2,outer_if)
    g.inline.update(range(start,end))


def verify_graph(fid, actions):
    if fid not in SOURCE_HASHES:
        return None
    mode=fixture_mode(fid);dates,typed,as_text=MODES[mode];legacy=fid.endswith("-legacy")
    g=Graph(actions)
    # Independent, fixed native values; metadata may never carry these objects.
    literal(g.at(g.get("textInput"),"gettext").get("WFTextActionText"),"ערך 🍒")
    require(g.at(g.get("numberInput"),"number").get("WFNumberActionNumber")==7.25,"Number source")
    for name in ("dateInput","referenceDate"):
        p=g.at(g.get(name),"date");require(p.get("WFDateActionMode")=="Specified Date","fixed Date source");literal(p.get("WFDateActionDate"),"2001-02-03T04:05:06Z")
    if legacy:
        variable(g.at(g.get("mapInput"),"getvariable").get("WFVariable"),"mapSeed")
        stores=[i for i,a in enumerate(actions) if kind(a)=="setvariable" and params(a).get("WFVariableName")=="mapSeed"]
        require(len(stores)==1 and stores[0]+1==g.get("mapInput"),"explicit Legacy Map producer")
        t=token(params(actions[stores[0]]).get("WFInput"));source=g.uuids.get(t.get("OutputUUID"))
        require(source is not None and source+1==stores[0] and kind(actions[source])=="dictionary","Legacy Map source kind")
        variable(g.at(g.get("listInput"),"getvariable").get("WFVariable"),"listSeed")
        start=g.get("listInput")-6
        require(g.at(start,"nothing")=={} and g.at(start+1,"setvariable")=={"WFVariableName":"listSeed"},"exact authored Legacy List clear")
        stores=[i for i,a in enumerate(actions) if kind(a)=="setvariable" and params(a).get("WFVariableName")=="listSeed"]
        appends=[i for i,a in enumerate(actions) if kind(a)=="appendvariable" and params(a).get("WFVariableName")=="listSeed"]
        require(stores==[start+1] and appends==[start+3,start+5],"two authored List elements")
        for offset,value in ((2,"left"),(4,"right")):
            literal(g.at(start+offset,"gettext").get("WFTextActionText"),value)
            p=g.at(start+offset+1,"appendvariable");g.ref(p.get("WFInput"),start+offset,"Text")
        # This clear belongs to the explicit Legacy source List, not debugLog.
        g.authored_nothing.add(start)
    else:
        g.get("mapInput","dictionary");g.get("listInput","list")
    def source_container(name, seed, expected_kind):
        if not legacy:return params(actions[g.get(name)])
        store=next(a for a in actions if kind(a)=="setvariable" and params(a).get("WFVariableName")==seed)
        return params(actions[g.uuids[token(params(store)["WFInput"])["OutputUUID"]]])
    v=source_container("mapInput","mapSeed","dictionary").get("WFItems",{})
    require(v.get("WFSerializationType")=="WFDictionaryFieldValue","Map source encoding")
    items=v.get("Value",{}).get("WFDictionaryFieldValueItems",[])
    require(len(items)==2,"Map source key cardinality")
    for item,key,value,tpe in zip(items,("kind","value"),("LABEL_MAP","7.25"),(0,3)):
        require(item.get("WFItemType")==tpe,"Map source value type");literal(item.get("WFKey"),key);literal(item.get("WFValue"),value)
    if not legacy:
        require(source_container("listInput","listSeed","list").get("WFItems")==[{"WFItemType":0,"WFValue":"left"},{"WFItemType":0,"WFValue":"right"}],"List source elements and order")
    p=g.at(g.get("fileSeed"),"base64encode");require(p.get("WFEncodeMode")=="Decode","binary File producer")
    try: literal(p.get("WFInput"),"AQIDBA==")
    except ValueError:
        t=token(p.get("WFInput"));i=g.uuids.get(t.get("OutputUUID"));require(i is not None,"binary source witness")
        literal(g.at(i,"gettext").get("WFTextActionText"),"AQIDBA==")
    p=g.linked("fileInput","setitemname","fileSeed");literal(p.get("WFName"),"label-source.bin");require(p.get("WFDontIncludeFileExtension") is False,"File extension contract")
    clears=[i-1 for i,a in enumerate(actions) if kind(a)=="setvariable" and params(a).get("WFVariableName")==LOG]
    require(len(clears)==2 and clears[0]==1,"two owned raw-buffer clears")
    for i in clears:g.clear(i,LOG);g.clear(i+2,META)
    raw_appends=[i for i,a in enumerate(actions) if kind(a)=="appendvariable" and params(a).get("WFVariableName")==LOG]
    meta_appends=[i for i,a in enumerate(actions) if kind(a)=="appendvariable" and params(a).get("WFVariableName")==META]
    require(len(raw_appends)==len(meta_appends)==6,"one raw and one scalar-metadata append per call")
    mid=[i for i,a in enumerate(actions) if kind(a)=="comment" and params(a).get("WFCommentActionText")=="ANCHOR_CARD_15_MID_CENTER"]
    require(len(mid)==1,"unique observer boundary")
    for n,(name,label) in enumerate(zip(PRODUCERS,LABELS)):
        producer=g.get(name);index=raw_appends[n]
        require(index==producer+1+int(dates),"log captures immediate original producer")
        original=params(actions[index]).get("WFInput");g.ref(original,name,name)
        if dates:
            p=g.at(producer+1,"detect.date");t=token(p.get("WFInput"))
            require(t.get("Aggrandizements")==[{"Type":"WFPropertyVariableAggrandizement","PropertyName":"Creation Date","PropertyUserInfo":"WFFileCreationDate"}],"immediate Creation Date capture")
            g.ref({"WFSerializationType":"WFTextTokenAttachment","Value":{k:v for k,v in t.items() if k!="Aggrandizements"}},name,name)
            g.inline.add(producer+1)
        last,size,before,first,metadata=index+1,index+2,index+3,index+4,index+5
        p=g.at(last,"count");variable(p.get("Input"),LOG);require(p.get("WFCountType")=="Items","buffer native cardinality")
        p=g.at(size,"count");require(p.get("Input")==original and p.get("WFCountType")=="Items","unboxed original native cardinality")
        p=g.at(before,"math");g.ref(p.get("WFInput"),last);g.ref(p.get("WFMathOperand"),size);require(p.get("WFMathOperation")=="-","start offset subtraction")
        p=g.at(first,"math");g.ref(p.get("WFInput"),before);require(p.get("WFMathOperation")=="+" and p.get("WFMathOperand")==1,"inclusive one-based metadata start")
        v=g.at(metadata,"dictionary").get("WFItems",{});require(v.get("WFSerializationType")=="WFDictionaryFieldValue","scalar Dictionary metadata")
        fields=v.get("Value",{}).get("WFDictionaryFieldValueItems",[])
        require(len(fields)==3,"exactly three metadata fields; raw objects cannot be boxed")
        for item,key,tpe in zip(fields,("first","label","last"),(3,0,3)):
            require(set(item)=={"WFItemType","WFKey","WFValue"} and item.get("WFItemType")==tpe,"metadata scalar type")
            literal(item.get("WFKey"),key)
            if key=="label":literal(item.get("WFValue"),label)
            else:g.ref(item.get("WFValue"),first if key=="first" else last)
        require(meta_appends[n]==index+6,"adjacent scalar metadata append")
        g.ref(g.at(index+6,"appendvariable").get("WFInput"),metadata)
        restore=index+7
        if n==5 and (typed or as_text):
            restore=mid[0]-1;verify_preview(g,index+7,restore,dates,as_text)
        p=g.at(restore,"getvariable");require(p.get("WFVariable")==original,"restore exact previous raw output")
        g.inline.update(range(index,index+7));g.inline.add(restore)
    require(g.get("buffered")==raw_appends[-1],"macro alias is raw buffer append, not preview or metadata")
    g.count("bufferCount","buffered")
    for i,name in enumerate(("observedText","observedNumber","observedMap","observedListFirst","observedListLast","observedDate","observedFile"),1):
        p=g.linked(name,"getitemfromlist","buffered");require(p.get("WFItemSpecifier")=="Item At Index" and p.get("WFItemIndex")==i,"independent raw item observer")
    for prefix,source in (("textType","observedText"),("numberType","observedNumber"),("mapType","observedMap"),("dateType","observedDate"),("fileType","observedFile"),("sourceFileType","fileInput")):
        g.linked(prefix,"getitemtype",source);g.encode(prefix+"64",prefix)
    for name,source in (("text64","observedText"),("mapKind64","mapKind"),("listFirst64","observedListFirst"),("listLast64","observedListLast"),("fileBytes64","observedFile"),("sourceFileBytes64","fileInput")):
        g.encode(name,source)
    def key(name,source,which):
        p=g.linked(name,"getvalueforkey",source);literal(p.get("WFDictionaryKey"),which)
        require(p.get("WFGetDictionaryValueType","Value")=="Value","real Dictionary value")
    def keys(name,source):
        p=g.linked(name,"getvalueforkey",source);require(p.get("WFGetDictionaryValueType")=="All Keys" and "WFDictionaryKey" not in p,"All Keys observation")
    keys("mapKeys","observedMap");g.count("mapKeysCount","mapKeys");key("mapKind","observedMap","kind");key("mapNumber","observedMap","value")
    p=g.linked("observedList","getitemfromlist","buffered");require(p.get("WFItemSpecifier")=="Items in Range" and p.get("WFItemRangeStart")==4 and p.get("WFItemRangeEnd")==5,"native List range")
    g.count("listCount","observedList");g.count("dateCount","observedDate");g.count("fileCount","observedFile")
    for name,target in (("dateDelta","observedDate"),("dateSourceDelta","dateInput")):
        p=g.linked(name,"gettimebetweendates",target);g.ref(p.get("WFTimeUntilFromDate"),"referenceDate");require(p.get("WFTimeUntilUnit")=="Seconds","explicit Date precision")
    for pre,source in (("file","observedFile"),("sourceFile","fileInput")):
        g.linked(pre+"Stem","getitemname",source)
        p=g.linked(pre+"Extension","properties.files",source);require(p.get("WFContentItemPropertyName")=="File Extension","File extension observation")
        p=g.at(g.get(pre+"FullName"),"gettext");v=text_value(p.get("WFTextActionText"))
        require(v.get("string")=="\ufffc.\ufffc" and set(v.get("attachmentsByRange",{}))=={"{0, 1}","{2, 1}"},"independent exact File name")
        for offset,name in ((0,pre+"Stem"),(2,pre+"Extension")):
            g.ref({"WFSerializationType":"WFTextTokenAttachment","Value":v["attachmentsByRange"]["{%d, 1}"%offset]},name)
        g.encode(pre+"Name64",pre+"FullName")
    for observed,alias,var in (("labelMetadataObserved","labelMetadata",META),("rawClearedObserved","rawCleared",LOG),("metadataClearedObserved","metadataCleared",META)):
        variable(g.at(g.get(observed),"getvariable").get("WFVariable"),var)
        g.linked(alias,"getvariable",observed,"WFVariable")
        require(g.get(observed) not in g.inline and g.get(alias) not in g.inline,"explicit raw observer is source, not instrumentation")
    require(mid[0]<g.get("labelMetadataObserved")<clears[1]<g.get("rawClearedObserved")<g.get("metadataClearedObserved"),"capture and clear observation order")
    g.count("metadataCount","labelMetadata");g.count("rawClearedCount","rawCleared");g.count("metadataClearedCount","metadataCleared")
    for i in range(1,7):
        name="m"+str(i);p=g.linked(name,"getitemfromlist","labelMetadata")
        require(p.get("WFItemSpecifier")=="Item At Index" and p.get("WFItemIndex")==i,"metadata record index")
        keys(name+"KeyList",name);g.count(name+"Keys",name+"KeyList")
        for suffix,which in (("First","first"),("Last","last"),("Label","label")):key(name+suffix,name,which)
        g.encode(name+"Label64",name+"Label")
    # Trace collection is inspected separately from label instrumentation.
    trace=[i for i,a in enumerate(actions) if kind(a)=="appendvariable" and params(a).get("WFVariableName")=="__cherri_qa_trace"]
    require(len(trace)==2,"bounded before/after trace")
    for i,stage in zip(trace,("before","after")):
        v=text_value(g.at(i-1,"gettext").get("WFTextActionText"));g.ref(params(actions[i]).get("WFInput"),i-1)
        attachments=v.get("attachmentsByRange",{});require(len(attachments)==1 and list(attachments.values())==[{"Type":"CurrentDate"}],"fresh trace timestamp")
        data=json.loads(v["string"].replace("\ufffc","OBSERVED_TIME"))
        require(data.get("stage")==stage and data.get("label")=="label-"+mode and data.get("kind")=="INLINE_DEBUG_ACTION","trace stage")
        g.inline.update((i-1,i))
    envelope=g.get("traceEnvelope","gettext");combine=envelope-1;p=g.at(combine,"text.combine");variable(p.get("text"),"__cherri_qa_trace")
    v=text_value(params(actions[envelope]).get("WFTextActionText"));require(fid in v.get("string","") and "===CHERRI_QA_TRACE_NDJSON_V14===" in v.get("string","") and len(v.get("attachmentsByRange",{}))==1,"source-bound trace envelope")
    g.ref({"WFSerializationType":"WFTextTokenAttachment","Value":next(iter(v["attachmentsByRange"].values()))},combine)
    g.collector.update((combine,envelope))
    final=g.get("finalSummary","gettext");value=text_value(params(actions[final]).get("WFTextActionText"))
    summary="CHERRI_QA_DEBUG_LABEL:MODE="+mode;expected_refs={}
    for key,name,_ in FIELDS:
        summary+=":"+key+"=";expected_refs["{%d, 1}" % len(summary)]=name;summary+="\ufffc"
    summary+="\n";expected_refs["{%d, 1}"%len(summary)]="traceEnvelope";summary+="\ufffc"
    require(value.get("string")==summary and set(value.get("attachmentsByRange",{}))==set(expected_refs),"exact independent result fields")
    for pos,name in expected_refs.items():g.ref({"WFSerializationType":"WFTextTokenAttachment","Value":value["attachmentsByRange"][pos]},name)
    require(kind(actions[-1])=="output","last action outputs result")
    g.ref(params(actions[-1]).get("WFOutput"),"finalSummary")
    allowed={"comment","nothing","setvariable","appendvariable","gettext","number","dictionary","list","getvariable","count","math","date","detect.date","base64encode","setitemname","getitemfromlist","getitemtype","getvalueforkey","gettimebetweendates","getitemname","properties.files","text.combine","output","conditional","repeat.each","previewdocument"}
    require(all(kind(a) in allowed for a in actions),"unreviewed effect")
    require(not g.inline.intersection(g.collector),"disjoint instrumentation classes")
    require(all(i in g.inline|g.authored_nothing for i,a in enumerate(actions) if kind(a)=="nothing"),"unowned Nothing action")
    expected_count={"plain":171,"dated":177,"typed":203,"dated-typed":212,"text":204,"dated-text":212}[mode]
    require(len(actions)==expected_count+(7 if legacy else 0),"unexpected action count")
    return {"mode":mode,"sourceSha256":SOURCE_HASHES[fid],"labels":6,"rawBufferExpectedItems":7,
            "metadataContainsRawObjects":False,"originalUUIDAndOutputNamesPreserved":True,
            "nativeValueAndCardinality":"NOT_RUN","datePrecision":"Seconds",
            "instrumentation":{"inlineIndices":sorted(g.inline),"collectorIndices":sorted(g.collector),
                "sourceIndices":[i for i in range(len(actions)) if i not in g.inline|g.collector]},
            "scopedToReviewedSource":True,"arbitraryUserActionClassification":False}


def verify_causal_mutations(fid, actions):
    """Reject corruption of real compiled graphs without changing source/goldens."""
    baseline=verify_graph(fid,actions)
    require(baseline is not None,"mutation fixture scope")
    dates,typed,as_text=MODES[fixture_mode(fid)]
    g=Graph(actions)
    raw=next(i for i,a in enumerate(actions) if kind(a)=="appendvariable" and params(a).get("WFVariableName")==LOG)
    metadata=raw+5
    clears=[i for i,a in enumerate(actions) if kind(a)=="setvariable" and params(a).get("WFVariableName")==LOG]
    mutations=[]
    def param(i,key,value):return lambda a:params(a[i]).__setitem__(key,copy.deepcopy(value))
    other={"WFSerializationType":"WFTextTokenAttachment","Value":{"Type":"ActionOutput","OutputUUID":params(actions[g.get("numberInput")])["UUID"],"OutputName":"numberInput"}}
    mutations += [("raw_object_redirected",param(raw,"WFInput",other)),
                  ("original_output_name_lost",lambda a:params(a[raw])["WFInput"]["Value"].__setitem__("OutputName","wrong")),
                  ("boxed_raw_metadata",lambda a:params(a[metadata])["WFItems"]["Value"]["WFDictionaryFieldValueItems"].append({"WFItemType":1,"WFKey":{"WFSerializationType":"WFTextTokenString","Value":{"string":"payload"}},"WFValue":copy.deepcopy(other)})),
                  ("metadata_label_replaced",lambda a:params(a[metadata])["WFItems"]["Value"]["WFDictionaryFieldValueItems"][1]["WFValue"]["Value"].__setitem__("string","WRONG_LABEL")),
                  ("cardinality_from_other_object",param(raw+2,"Input",other)),
                  ("range_subtraction_reversed",param(raw+3,"WFMathOperation","+")),
                  ("range_start_zero",param(raw+4,"WFMathOperand",0)),
                  ("metadata_append_uses_raw_object",param(raw+6,"WFInput",other)),
                  ("restore_previous_output_redirected",param(raw+7,"WFVariable",other)),
                  ("clear_has_input",param(clears[0],"WFInput",other)),
                  ("metadata_clear_has_input",param(clears[0]+2,"WFInput",other)),
                  ("constant_success_text",param(g.get("finalSummary"),"WFTextActionText",expected_summary(fid))),
                  ("raw_list_item_shifted",param(g.get("observedListFirst"),"WFItemIndex",5)),
                  ("file_bytes_borrow_source",param(g.get("fileBytes64"),"WFInput",params(actions[g.get("sourceFileBytes64")])["WFInput"])),
                  ("file_name_borrow_source",param(g.get("fileStem"),"WFInput",params(actions[g.get("sourceFileStem")])["WFInput"])),
                  ("empty_label_key_observation_changed",param(g.get("m2KeyList"),"WFGetDictionaryValueType","All Values")),
                  ("wrong_fixed_date",param(g.get("dateInput"),"WFDateActionDate","2002-02-03T04:05:06Z")),
                  ("arbitrary_unowned_nothing",lambda a:a.append({"WFWorkflowActionIdentifier":"is.workflow.actions.nothing","WFWorkflowActionParameters":{}})),
                  ("network_effect",lambda a:a.append({"WFWorkflowActionIdentifier":"is.workflow.actions.downloadurl","WFWorkflowActionParameters":{}}))]
    if dates:
        mutations += [("late_creation_date_capture",lambda a:a.__setitem__(slice(raw-1,raw+1),[a[raw],a[raw-1]])),
                      ("different_date_property",lambda a:params(a[raw-1])["WFInput"]["Value"]["Aggrandizements"][0].__setitem__("PropertyUserInfo","WFFileModificationDate"))]
    if typed or as_text:
        start=g.get("buffered")+7
        mutations += [("outer_item_not_snapshotted",param(start+3,"WFVariable",{"WFSerializationType":"WFTextTokenAttachment","Value":{"Type":"Variable","VariableName":"Repeat Index"}})),
                      ("outer_index_not_snapshotted",param(start+4,"WFVariable",{"WFSerializationType":"WFTextTokenAttachment","Value":{"Type":"Variable","VariableName":"Repeat Item"}})),
                      ("range_first_reads_last",param(start+8,"WFDictionaryKey","last")),
                      ("range_comparison_inverted",param(start+10,"WFCondition",2)),
                      ("metadata_loop_owner_changed",param(start+20,"GroupingIdentifier","00000000-0000-0000-0000-000000000000")),
                      ("preview_row_name_is_constant",param(start+23,"WFTextActionText" if as_text else "WFName","WRONG_LABEL")),
                      ("preview_reads_raw_buffer_directly",param(start+28,"WFInput",{"WFSerializationType":"WFTextTokenAttachment","Value":{"Type":"Variable","VariableName":LOG}}))]
    if fid.endswith("-legacy"):
        start=g.get("listInput")-6
        mutations += [("authored_legacy_list_clear_has_input",param(start+1,"WFInput",other)),
                      ("authored_legacy_list_order_changed",param(start+2,"WFTextActionText","right")),
                      ("authored_legacy_list_append_redirected",param(start+3,"WFInput",other))]
    rejected=[]
    for name,mutate in mutations:
        damaged=copy.deepcopy(actions);mutate(damaged)
        try:verify_graph(fid,damaged)
        except ValueError:rejected.append(name)
        else:raise ValueError("DEBUG_LABEL_MUTATION_ACCEPTED: "+name)
    return {"rejected":rejected,"count":len(rejected),"nativeExecution":"NOT_RUN"}


def compile_controls(repo, compiler):
    """Compile all reviewed sources and mutate their actual native graphs."""
    repo, compiler = Path(repo).resolve(), Path(compiler).resolve()
    commit = subprocess.run(["git", "rev-parse", "HEAD"], cwd=repo, check=True,
                            capture_output=True, text=True, timeout=15).stdout.strip()
    clean = not subprocess.run(["git", "status", "--porcelain", "--untracked-files=normal"],
        cwd=repo, check=True, capture_output=True, text=True, timeout=15).stdout.strip()
    require(re.fullmatch(r"[0-9a-f]{40}", commit), "immutable checkout identity")
    rows = []
    with tempfile.TemporaryDirectory(prefix="cherri-debug-label-graph-") as directory:
        for fid in SOURCE_HASHES:
            source = repo / "tools/shortcuts-simulator-qa/fixtures" / (fid + ".cherri")
            source_receipt = verify_source(repo, fid, source.read_bytes())
            output = Path(directory) / (fid + ".shortcut")
            command = [str(compiler), str(source), "--skip-sign", "--build-profile=debug",
                       "--output=" + str(output)]
            if fid.endswith("-legacy"):
                command.append("--legacy")
            result = subprocess.run(command, cwd=repo, capture_output=True, timeout=60)
            require(result.returncode == 0 and output.is_file(), "actual fixture compilation " + fid)
            raw = output.read_bytes()
            actions = plistlib.loads(raw).get("WFWorkflowActions")
            graph = verify_graph(fid, actions)
            controls = verify_causal_mutations(fid, actions)
            rows.append({"fixtureId": fid, "source": source_receipt,
                         "unsignedPlistSha256": hashlib.sha256(raw).hexdigest(),
                         "nativeActionCount": len(actions), "graph": graph,
                         "causalMutations": controls})
    return {"schemaVersion": "1", "implementationSha": commit, "cleanCheckout": clean,
            "compilerSha256": hashlib.sha256(compiler.read_bytes()).hexdigest(),
            "helperSha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
            "fixtures": rows, "actualCompileCount": len(rows),
            "rejectedMutationCount": sum(r["causalMutations"]["count"] for r in rows),
            "nativeExecution": "NOT_RUN", "formalAcceptanceGranted": False}


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", required=True)
    parser.add_argument("--compiler", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()
    receipt = compile_controls(args.repo, args.compiler)
    with Path(args.out).open("x", encoding="utf-8", newline="\n") as stream:
        json.dump(receipt, stream, indent=2, ensure_ascii=False)
        stream.write("\n")
    print(json.dumps({key: receipt[key] for key in
        ("implementationSha", "cleanCheckout", "actualCompileCount", "rejectedMutationCount", "nativeExecution")}))
