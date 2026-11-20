"""Reviewed adapters for the unchanged public binding/default runtime bodies."""
import base64
import hashlib
from pathlib import Path

CASES = {
    "CherriBindingCapturePOC": {
        "sample": "qa-binding-capture", "prefix": "CHERRI_QA_BINDING_CAPTURE_RESULT:",
        "clipboard": "CHERRI_BINDING_CAPTURE_OK\nNUM_CAPTURED=1\nNUM_AGAIN=1\nCLIP_CAPTURED=clip_initial\nCLIP_AGAIN=clip_initial",
    },
    "CherriTypedDefaultsPOC": {
        "sample": "qa-typed-defaults", "prefix": "CHERRI_QA_TYPED_DEFAULTS_RESULT:",
        "clipboard": "CHERRI_TYPED_DEFAULTS_OK\nDEFAULTS_EXPLICIT=0:0::0:0\nDEFAULTS_OMITTED=1:9:def:1:1\nDEFAULTS_MIXED=0:9:def:0:1",
    },
}
FOLDER = "p15-binding-defaults"


def expected_summary(fid):
    case = CASES[fid]
    return case["prefix"] + base64.b64encode(case["clipboard"].encode("utf-8")).decode("ascii")


def render_adapter(fid, original):
    """The original body bytes, including its final Set Clipboard, stay intact."""
    if fid not in CASES or not original.endswith(b"\n"):
        raise ValueError("BINDING_DEFAULTS_SOURCE_CONTRACT")
    prefix = ('comment("ANCHOR_CARD_01_TOP_HEADER")\n'
              'debugClearLogs()\n'
              'debugTraceEvent("before", "binding-defaults")\n'
              '// BEGIN unchanged tests/runtime_poc/' + fid + '.cherri\n')
    suffix = ('// END unchanged runtime body\n'
              'comment("ANCHOR_CARD_15_MID_CENTER")\n'
              'let qaObservedClipboard = getClipboard()\n'
              'let qaClipboard64Raw = base64Encode(qaObservedClipboard, lineBreakMode: "None")\n'
              'let qaClipboard64Length = count(qaClipboard64Raw, "Characters")\n'
              'var qaClipboard64 = "OVERSIZE"\n'
              'if qaClipboard64Length.number < 513 { qaClipboard64 = qaClipboard64Raw }\n'
              'debugTraceEvent("after", "binding-defaults")\n'
              'let qaTraceEnvelope = debugExportTrace(runId: "' + fid + '")\n'
              'let qaFinalSummary = text(f"' + CASES[fid]["prefix"] + '{qaClipboard64}\\n{qaTraceEnvelope}")\n'
              'comment("ANCHOR_CARD_28_BOTTOM_FOOTER")\n'
              'output(qaFinalSummary)\n')
    return prefix.encode("utf-8") + original + suffix.encode("utf-8")


def verify_adapter_source(root, fid, actual):
    if fid not in CASES:
        return None
    path = "tests/runtime_poc/" + fid + ".cherri"
    original = (Path(root) / path).read_bytes()
    if actual != render_adapter(fid, original):
        raise ValueError("BINDING_DEFAULTS_SOURCE_CONTRACT: original body or observer changed")
    return {"originalSourcePath": path, "originalSourceSha256": hashlib.sha256(original).hexdigest(),
            "adapterSourceSha256": hashlib.sha256(actual).hexdigest(),
            "originalBodyByteIdentical": True, "expectedSummary": expected_summary(fid),
            "originalShortcutNamePreserved": True, "appleRuntime": "NOT_RUN"}


def verify_adapter_graph(fid, actions):
    if fid not in CASES:
        return None
    def require(ok, detail):
        if not ok: raise ValueError("BINDING_DEFAULTS_GRAPH_CONTRACT: " + detail)
    def p(a): return a.get("WFWorkflowActionParameters", {})
    def kind(a, suffix): return a.get("WFWorkflowActionIdentifier") == "is.workflow.actions." + suffix
    def named(name, suffix):
        found = [(i,a) for i,a in enumerate(actions) if p(a).get("CustomOutputName") == name]
        require(len(found) == 1 and kind(found[0][1], suffix), "named producer " + name)
        return found[0]
    def token(v):
        require(isinstance(v,dict) and v.get("WFSerializationType") == "WFTextTokenAttachment", "attachment")
        return v.get("Value", {})
    def edge(v, source):
        t = token(v)
        require(t.get("Type") == "ActionOutput" and t.get("OutputUUID") == p(source).get("UUID")
                and not t.get("Aggrandizements"), "exact producer edge")
    # Native run 44466065158 independently observed Get Variable(Clipboard)
    # as empty while canonical Get Clipboard returned the actual public value.
    oi, observed = named("qaObservedClipboard", "getclipboard")
    require(set(p(observed)) <= {"UUID", "CustomOutputName"}, "canonical clipboard observation")
    setters = [i for i,a in enumerate(actions) if kind(a,"setclipboard")]
    require(bool(setters) and max(setters) < oi, "observe only after original final Set Clipboard")
    ri, raw = named("qaClipboard64Raw", "base64encode")
    edge(p(raw).get("WFInput"), observed)
    require(p(raw).get("WFEncodeMode") == "Encode" and p(raw).get("WFBase64LineBreakMode") == "None", "exact byte encoding")
    li, length = named("qaClipboard64Length", "count")
    edge(p(length).get("Input"), raw)
    require(p(length).get("WFCountType") == "Characters" and oi < ri < li, "byte witness count")
    writes = [(i,a) for i,a in enumerate(actions) if kind(a,"setvariable") and p(a).get("WFVariableName") == "qaClipboard64"]
    require(len(writes)==2 and writes[1][0]==writes[0][0]+2, "bounded witness assignments")
    first, second = writes[0][0], writes[1][0]
    require(first==li+2 and kind(actions[first-1],"gettext") and p(actions[first-1]).get("WFTextActionText")=="OVERSIZE",
            "fail-closed oversize initializer")
    edge(p(actions[first]).get("WFInput"), actions[first-1])
    cond, end = actions[first+1], actions[second+1]
    require(kind(cond,"conditional") and kind(end,"conditional") and p(cond).get("WFControlFlowMode")==0
            and p(end).get("WFControlFlowMode")==2 and p(cond).get("GroupingIdentifier")==p(end).get("GroupingIdentifier")
            and p(cond).get("WFCondition")==0 and p(cond).get("WFNumberValue")==513, "512-character cap")
    ct = token(p(cond).get("WFInput",{}).get("Variable"))
    require(ct.get("Type")=="ActionOutput" and ct.get("OutputUUID")==p(length).get("UUID") and ct.get("Aggrandizements")==[
        {"Type":"WFCoercionVariableAggrandizement","CoercionItemClass":"WFNumberContentItem"}], "cap numeric source")
    edge(p(actions[second]).get("WFInput"), raw)
    _, trace = named("qaTraceEnvelope", "gettext")
    si, summary = named("qaFinalSummary", "gettext")
    serialized = p(summary).get("WFTextActionText")
    require(isinstance(serialized,dict) and serialized.get("WFSerializationType")=="WFTextTokenString", "dynamic summary serialization")
    text = serialized.get("Value",{})
    prefix = CASES[fid]["prefix"]
    require(text.get("string")==prefix+"\ufffc\n\ufffc", "dynamic summary text")
    atts = text.get("attachmentsByRange",{})
    require(atts.get("{%d, 1}" % len(prefix))=={"Type":"Variable","VariableName":"qaClipboard64"}
            and len(atts)==2, "bounded summary witness")
    trace_att=atts.get("{%d, 1}" % (len(prefix)+2),{})
    require(trace_att.get("Type")=="ActionOutput" and trace_att.get("OutputUUID")==p(trace).get("UUID"), "real trace output")
    output_index = si + 2
    require(output_index < len(actions) and kind(actions[si+1],"comment")
            and p(actions[si+1]).get("WFCommentActionText")=="ANCHOR_CARD_28_BOTTOM_FOOTER"
            and kind(actions[output_index],"output"), "actual callback output")
    serialized_output = p(actions[output_index]).get("WFOutput",{})
    require(serialized_output.get("WFSerializationType")=="WFTextTokenString", "output serialization")
    ov = serialized_output.get("Value",{})
    oa = ov.get("attachmentsByRange",{}).get("{0, 1}",{})
    require(ov.get("string")=="\ufffc" and len(ov.get("attachmentsByRange",{}))==1
            and oa.get("Type")=="ActionOutput" and oa.get("OutputUUID")==p(summary).get("UUID")
            and not oa.get("Aggrandizements"), "final summary edge")
    if fid=="CherriTypedDefaultsPOC":
        # The function dispatcher closes after main-body output. Default V2
        # main fallthrough must then end with one parameter-free Nothing.
        require(output_index==len(actions)-3 and kind(actions[-2],"conditional")
                and p(actions[-2]).get("WFControlFlowMode")==2, "single dispatcher closure")
        require(kind(actions[-1],"nothing")
                and set(actions[-1]) <= {"WFWorkflowActionIdentifier", "WFWorkflowActionParameters"}
                and p(actions[-1]) == {}, "exact parameter-free main completion")
        group = p(actions[-2]).get("GroupingIdentifier")
        members = [(i,a) for i,a in enumerate(actions) if p(a).get("GroupingIdentifier")==group]
        require(bool(group) and len(members)==3 and all(kind(a,"conditional") for _,a in members)
                and [p(a).get("WFControlFlowMode") for _,a in members]==[0,1,2]
                and members[1][0] < oi, "balanced main dispatcher branch")
        dispatch = p(members[0][1])
        require(dispatch.get("WFCondition")==4 and dispatch.get("WFConditionalActionString")=="1"
                and token(dispatch.get("WFInput",{}).get("Variable"))=={
                    "Type":"Variable","VariableName":"_cherri_is_fn"}, "canonical function dispatcher")
        calls=[a for a in actions if kind(a,"runworkflow")]
        require(len(calls)==3 and all(p(a).get("WFWorkflowName")==fid
                and p(a).get("WFWorkflow")=={"isSelf":True,"workflowName":fid} for a in calls),
                "three exact original-name function self-calls")
    else:
        require(output_index==len(actions)-1, "no actions after binding callback")
    return {"status":"PASS_LOCAL_GRAPH_ONLY","actualClipboardObservation":True,
            "maximumEncodedClipboardCharacters":512,"originalShortcutName":fid,"appleRuntime":"NOT_RUN"}
