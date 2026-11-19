#!/usr/bin/env python3
"""Source-bound local debug export validation; the graph model is NOT Apple runtime."""
import argparse, base64, copy, hashlib, json, os, pathlib, re, subprocess, sys, tempfile, textwrap, time

ROOT = pathlib.Path(__file__).resolve().parents[2]
PREFIX = "is.workflow.actions."
ALLOWED = {"gettext", "getitemtype", "base64encode", "repeat.each", "conditional", "text.combine", "setitemname"}

class Value:
    def __init__(self, kind, data, name=None):
        self.kind, self.data, self.name = kind, data, name

def as_text(value):
    if not isinstance(value, Value) or value.kind != "Text":
        raise ValueError("model refuses an implicit non-Text conversion")
    return value.data

class GraphModel:
    """Independent bounded evaluator of emitted graph, with explicit native assumptions."""
    def __init__(self, items):
        self.variables = {"__cherri_debug_log": Value("List", items)}
        self.outputs, self.executed = {}, 0
    def value(self, raw):
        if isinstance(raw, str):
            return Value("Text", raw)
        if not isinstance(raw, dict):
            raise ValueError("unknown native value representation")
        if raw.get("WFSerializationType") == "WFTextTokenAttachment":
            return self.value(raw["Value"])
        if raw.get("WFSerializationType") == "WFTextTokenString":
            payload = raw["Value"]
            units = payload["string"].encode("utf-16-le")
            replacements = []
            for key, ref in payload["attachmentsByRange"].items():
                found = re.fullmatch(r"\{(\d+), 1\}", key)
                if not found:
                    raise ValueError("invalid attachment range")
                offset = int(found[1])*2
                if units[offset:offset+2] != "\ufffc".encode("utf-16-le"):
                    raise ValueError("attachment does not address a placeholder")
                replacements.append((offset, as_text(self.value(ref)).encode("utf-16-le")))
            for offset, data in sorted(replacements, reverse=True):
                units = units[:offset] + data + units[offset+2:]
            return Value("Text", units.decode("utf-16-le"))
        if raw.get("Type") == "ActionOutput":
            return self.outputs[raw["OutputUUID"]]
        if raw.get("Type") == "Variable":
            if "Variable" in raw:
                return self.value(raw["Variable"])
            return self.variables[raw["VariableName"]]
        raise ValueError("unknown native reference")
    def run(self, actions, start=0, end=None):
        end = len(actions) if end is None else end
        previous = None
        index = start
        while index < end:
            action = actions[index]
            ident = action["WFWorkflowActionIdentifier"].removeprefix(PREFIX)
            p = action["WFWorkflowActionParameters"]
            self.executed += 1
            if ident not in ALLOWED:
                raise ValueError("nonlocal effect or mutation in export graph: "+ident)
            if ident in {"repeat.each", "conditional"} and p["WFControlFlowMode"] == 0:
                group = p["GroupingIdentifier"]
                closing = next(i for i in range(index+1,end) if actions[i]["WFWorkflowActionParameters"].get("GroupingIdentifier") == group and actions[i]["WFWorkflowActionParameters"].get("WFControlFlowMode") == 2)
                if ident == "repeat.each":
                    incoming = self.value(p["WFInput"])
                    if incoming.kind != "List":
                        raise ValueError("model requires explicit list cardinality")
                    collected = []
                    for item in incoming.data:
                        self.variables["Repeat Item"] = item
                        collected.append(self.run(actions,index+1,closing))
                    previous = Value("List",collected)
                else:
                    alternate = next(i for i in range(index+1,closing) if actions[i]["WFWorkflowActionParameters"].get("GroupingIdentifier") == group and actions[i]["WFWorkflowActionParameters"].get("WFControlFlowMode") == 1)
                    if p.get("WFCondition") != 4:
                        raise ValueError("unsupported condition; expected exact Text equality")
                    matches = as_text(self.value(p["WFInput"])) == as_text(self.value(p["WFConditionalActionString"]))
                    previous = self.run(actions,index+1,alternate) if matches else self.run(actions,alternate+1,closing)
                close = actions[closing]["WFWorkflowActionParameters"]
                if "UUID" in close:
                    self.outputs[close["UUID"]] = previous
                index = closing+1
                continue
            if ident == "gettext":
                previous = self.value(p["WFTextActionText"])
            elif ident == "getitemtype":
                # Native names may be localized: the implementation compares two
                # runtime witnesses, never this model's spelling.
                previous = Value("Text","native:"+self.value(p["WFInput"]).kind)
            elif ident == "base64encode":
                text = as_text(self.value(p["WFInput"]))
                if p["WFEncodeMode"] == "Decode":
                    previous = Value("File", base64.b64decode(text, validate=True))
                elif p["WFEncodeMode"] == "Encode":
                    data = base64.b64encode(text.encode("utf-8")).decode("ascii")
                    if p.get("WFBase64LineBreakMode") != "None":
                        data = "\n".join(textwrap.wrap(data,76))
                    previous = Value("Text", data)
                else:
                    raise ValueError("unknown base64 direction")
            elif ident == "text.combine":
                items = self.value(p["text"])
                if items.kind != "List" or p.get("WFTextSeparator") != "Custom":
                    raise ValueError("unknown combine contract")
                previous = Value("Text",p["WFTextCustomSeparator"].join(as_text(v) for v in items.data))
            elif ident == "setitemname":
                source = self.value(p["WFInput"])
                if source.kind != "File" or p["WFName"] not in {"cherri-debug.json","cherri-debug.txt"}:
                    raise ValueError("unsafe file output")
                previous = Value("File",source.data,p["WFName"])
            else:
                raise ValueError("unexpected control boundary")
            if "UUID" in p:
                self.outputs[p["UUID"]] = previous
            index += 1
        return previous

def inspect_artifact(graph, items, fmt, redact, include_types):
    model = GraphModel(items)
    before = [(id(v),v.kind,copy.deepcopy(v.data)) for v in items]
    output = model.run(graph)
    if output.kind != "File":
        raise AssertionError("export does not return File")
    content = output.data.decode("utf-8")
    if fmt == "json":
        decoded = json.loads(content)
        assert decoded["schemaVersion"] == "cherri.debug-export/1"
        assert decoded["payloadPolicy"] == ("redacted" if redact else "text-only-base64")
        assert decoded["dates"] == decoded["labels"] == "not-collected"
        entries = decoded["entries"]
    else:
        header, body = content.split("\n\n",1)
        assert header.startswith("Cherri Debug Export 1\n")
        entries = [json.loads(line) for line in body.splitlines()]
    assert len(entries) == len(items), "entry count changed"
    for entry, item in zip(entries,items):
        if include_types:
            assert entry["nativeType"]["encoding"] == "base64-utf8"
            assert base64.b64decode(entry["nativeType"]["value"],validate=True).decode("utf-8") == "native:"+item.kind
        else:
            assert "nativeType" not in entry
        if redact:
            assert entry["status"] == "REDACTED" and entry["payload"] is None
        elif item.kind == "Text":
            assert entry["status"] == "TEXT_PAYLOAD_ENCODED"
            assert entry["payload"]["encoding"] == "base64-utf8"
            assert base64.b64decode(entry["payload"]["value"],validate=True).decode("utf-8") == item.data
        else:
            assert entry["status"] == "UNSUPPORTED_PAYLOAD_TYPE" and entry["payload"] is None
    assert before == [(id(v),v.kind,v.data) for v in items], "model raw log mutated"
    return {"entries":len(entries),"bytes":len(output.data),"modelExecutedActions":model.executed}

def run(command,cwd=ROOT,timeout=180):
    p = subprocess.run([str(v) for v in command],cwd=cwd,capture_output=True,text=True,encoding="utf-8",timeout=timeout)
    return p

def fingerprint():
    paths = [*ROOT.glob("*.go"),*ROOT.joinpath("internal/language").rglob("*.go"),*ROOT.joinpath("actions").glob("*.cherri"),*ROOT.joinpath("tools/shortcuts-simulator-qa/fixtures/p15-export-probes").glob("*.cherri"),ROOT/"tools/shortcuts-simulator-qa/fixtures/p15-export-probes/manifest.json",ROOT/"docs/apple-runtime/macos-27/builtin-actions.json",ROOT/"go.mod",ROOT/"go.sum",pathlib.Path(__file__).resolve()]
    digest = hashlib.sha256()
    for path in sorted(set(paths)):
        digest.update(path.relative_to(ROOT).as_posix().encode()+b"\0"+path.read_bytes()+b"\0")
    return digest.hexdigest()

def verify_native_metadata():
    catalog=json.loads((ROOT/"docs/apple-runtime/macos-27/builtin-actions.json").read_text(encoding="utf-8"))
    for ident,cls in [("getitemtype","WFGetItemTypeAction"),("base64encode","WFBase64EncodingAction"),("setitemname","WFSetItemNameAction")]:
        assert catalog[PREFIX+ident]["ActionClass"]==cls
    assert catalog[PREFIX+"base64encode"]["Output"]["Types"]==["WFStringContentItem","public.data"]
    modes=next(p for p in catalog[PREFIX+"base64encode"]["Parameters"] if p["Key"]=="WFEncodeMode")
    assert modes["Items"]==["Encode","Decode"]
    assert catalog[PREFIX+"getitemtype"]["Output"]["Types"]==["NSString"]
    return {"path":"docs/apple-runtime/macos-27/builtin-actions.json","sha256":hashlib.sha256((ROOT/"docs/apple-runtime/macos-27/builtin-actions.json").read_bytes()).hexdigest(),"platform":"macOS 27 metadata; does not establish iPad runtime"}

def main():
    parser=argparse.ArgumentParser()
    parser.add_argument("--report",default="docs/apple-runtime/p10r15/debug-export-validation.json")
    args=parser.parse_args()
    source_sha=run(["git","rev-parse","HEAD"]).stdout.strip()
    source_dirty_before=bool(run(["git","status","--porcelain"]).stdout.strip())
    report_path=ROOT/args.report;report_path.parent.mkdir(parents=True,exist_ok=True)
    report_path.write_bytes(b'{"status":"RUNNING_SOURCE_VALIDATION","nativeExecuted":0}\n')
    before=fingerprint()
    out=ROOT/"dist/p15-debug-export";out.mkdir(parents=True,exist_ok=True)
    exe=out/("cherri.exe" if os.name=="nt" else "cherri")
    started=time.monotonic()
    build=run(["go","build","-p=1","-trimpath","-o",exe,"."])
    if build.returncode:
        raise RuntimeError("compiler build failed: "+build.stderr[-3000:])
    capabilities=run([exe,"--capabilities-json"])
    if capabilities.returncode: raise RuntimeError("capabilities failed")
    caps=json.loads(capabilities.stdout)
    import plistlib
    checks=[]
    graphs={}
    for legacy in (False,True):
      for fmt in ("json","text"):
       for redact in (True,False):
        for include_types in (True,False):
         ident=f"{'legacy' if legacy else 'v2'}-{fmt}-{'redacted' if redact else 'text'}-types{include_types}"
         source=out/(ident+".cherri");target=out/(ident+".shortcut")
         options=f'"{fmt}", false, {str(include_types).lower()}, {str(redact).lower()}'
         source.write_bytes(('debugClearLogs()\ntext("synthetic input")\ndebugLog()\ndebugExport('+options+')\n').encode())
         cmd=[exe,"compile",source,"-o",target,"--skip-sign","--no-ansi"]+(["--legacy"] if legacy else [])
         result=run(cmd)
         if result.returncode: raise RuntimeError(ident+" compile: "+(result.stdout+result.stderr)[-1800:])
         actions=plistlib.loads(target.read_bytes())["WFWorkflowActions"]
         graph=actions[4:]
         assert all(a["WFWorkflowActionIdentifier"].removeprefix(PREFIX) in ALLOWED for a in graph)
         imported=out/(ident+"-import.cherri");recompiled=out/(ident+"-recompiled.shortcut")
         result=run([exe,"import",target,"--mode=v2","--lossless","-o",imported])
         if result.returncode:raise RuntimeError(ident+" import")
         result=run([exe,"compile",imported,"-o",recompiled,"--skip-sign","--no-ansi"])
         if result.returncode:raise RuntimeError(ident+" recompile")
         assert actions==plistlib.loads(recompiled.read_bytes())["WFWorkflowActions"],"lossless graph changed"
         tricky='"\\\\\n\r\t שלום שָׁלוֹם 🍒 \ufffc '+''.join(chr(i) for i in range(32))+" SYNTHETIC_AUTH_SECRET"
         samples={
           "zero":[],
           "one":[Value("Text",tricky)],
           "empty-text":[Value("Text","")],
           "thousand":[Value("Text",tricky+str(i)) for i in range(1000)],
           "native-type-matrix":[Value("Text",tricky),Value("Number",12.5),Value("Bool",False),Value("Null",None),Value("Dictionary",{"secret":"SYNTHETIC_SECRET"}),Value("List",["x",1]),Value("Date","2026-11-18"),Value("File",b"\x00\xff"),Value("Image",b"\x00\xff")],
         }
         models={name:inspect_artifact(graph,items,fmt,redact,include_types) for name,items in samples.items()}
         checks.append({"id":ident,"status":"PASS","sourceSha256":hashlib.sha256(source.read_bytes()).hexdigest(),"actionCount":len(actions),"exportActionCount":len(graph),"exactNativeActionRoundtrip":True,"models":models})
         graphs[(legacy,fmt,redact,include_types)]=graph
    negatives=[]
    for legacy in (False,True):
      for profile in ("debug","release"):
       for name,source in {
          "format":'debugExport("xml")\n',
          "path":'debugExport("../export.json")\n',
          "unproved-dates":'debugExport("json", true)\n',
          "dynamic-format":('const' if legacy else 'let')+' fmt = text("json")\ndebugExport(fmt)\n',
       }.items():
          source_path=out/f"negative-{legacy}-{profile}-{name}.cherri";target=source_path.with_suffix(".shortcut")
          source_path.write_bytes(source.encode())
          target.unlink(missing_ok=True)
          result=run([exe,"compile",source_path,"-o",target,"--build-profile",profile]+(["--legacy"] if legacy else []))
          assert result.returncode != 0 and not target.exists(),"invalid source produced success"
          negatives.append({"id":f"{legacy}-{profile}-{name}","exitCode":result.returncode,"noArtifact":True,"status":"PASS"})
    base=graphs[(False,"json",False,True)]
    default=graphs[(False,"json",True,True)]
    def mutate_linebreak(g):
        next(a for a in g if a["WFWorkflowActionIdentifier"]==PREFIX+"base64encode" and a["WFWorkflowActionParameters"]["WFInput"]["Value"].get("VariableName")=="Repeat Item")["WFWorkflowActionParameters"]["WFBase64LineBreakMode"]="Every 76 Characters"
    def mutate_redaction(g):
        next(a for a in g if a["WFWorkflowActionIdentifier"]==PREFIX+"base64encode")["WFWorkflowActionParameters"]["WFInput"]={"WFSerializationType":"WFTextTokenAttachment","Value":{"Type":"Variable","VariableName":"Repeat Item"}}
    def mutate_witness(g):
        p=next(a["WFWorkflowActionParameters"] for a in g if a["WFWorkflowActionIdentifier"]==PREFIX+"conditional")
        p["WFConditionalActionString"]={"WFSerializationType":"WFTextTokenString","Value":{"string":"\ufffc","attachmentsByRange":{"{0, 1}":p["WFInput"]["Variable"]["Value"]}}}
    def mutate_filename(g):
        g[-1]["WFWorkflowActionParameters"]["WFName"]="../private.json"
    def mutate_file_direction(g):
        g[-2]["WFWorkflowActionParameters"]["WFEncodeMode"]="Encode"
    def mutate_unproved_conversion(g):
        g[-2]["WFWorkflowActionIdentifier"]=PREFIX+"gettypeaction"
        g[-2]["WFWorkflowActionParameters"]["WFFileType"]="public.utf8-plain-text"
    def mutate_reference(g):
        g[-1]["WFWorkflowActionParameters"]["WFInput"]["Value"]["OutputUUID"]="MISSING"
    faults=[]
    for name,original,redact,mutation in [
        ("base64-line-breaks",base,False,mutate_linebreak),("default-payload-leak",default,True,mutate_redaction),
        ("type-witness-bypass",base,False,mutate_witness),("unsafe-filename",base,False,mutate_filename),
        ("wrong-file-direction",base,False,mutate_file_direction),("unproved-text-file-conversion",base,False,mutate_unproved_conversion),
        ("missing-output-reference",base,False,mutate_reference),
    ]:
        altered=copy.deepcopy(original);mutation(altered)
        items=[Value("Text",'"\\\n'+("שלום🍒"*100)),Value("File",b"\x01\xff")]
        try: inspect_artifact(altered,items,"json",redact,True)
        except (AssertionError,ValueError,KeyError,UnicodeError): pass
        else: raise AssertionError("fault was not detected: "+name)
        inspect_artifact(original,items,"json",redact,True)
        faults.append({"id":name,"negativeRejected":True,"restoredControl":"PASS"})
    native_probes=[]
    manifest=json.loads((ROOT/"tools/shortcuts-simulator-qa/fixtures/p15-export-probes/manifest.json").read_text(encoding="utf-8"))
    for case in manifest["cases"]:
        source=ROOT/case["path"]
        source_text=source.read_text(encoding="utf-8")
        unicode_required=case["scenario"] in {"unicode","redacted","text-file"}
        if unicode_required:
            assert "\u05e9\u05b8\u05c1\u05dc\u05d5\u05b9\u05dd" in source_text and "\U0001f352" in source_text and "????" not in source_text,"actual UTF-8 probe source bytes missing"
        target=out/(case["id"]+".shortcut")
        isolated_source=out/(case["id"]+".cherri");isolated_source.write_bytes(source.read_bytes())
        result=run([exe,"compile",isolated_source,"-o",target,"--load-all-actions","--no-ansi","--skip-sign"]+(["--legacy"] if case["dialect"]=="legacy" else []))
        if result.returncode:raise RuntimeError(case["id"]+" native probe compile: "+(result.stdout+result.stderr)[-2000:])
        actions=plistlib.loads(target.read_bytes())["WFWorkflowActions"]
        imported=out/(case["id"]+"-import.cherri");rebuilt=out/(case["id"]+"-rebuilt.shortcut")
        result=run([exe,"import",target,"--mode=v2","--lossless","-o",imported])
        if result.returncode:raise RuntimeError(case["id"]+" native probe import")
        result=run([exe,"compile",imported,"-o",rebuilt,"--skip-sign","--no-ansi"])
        if result.returncode:raise RuntimeError(case["id"]+" native probe rebuild")
        assert actions==plistlib.loads(rebuilt.read_bytes())["WFWorkflowActions"],"native probe exact graph changed"
        native_probes.append({**case,"compiledStatus":"PASS","sourceSha256":hashlib.sha256(source.read_bytes()).hexdigest(),"utf8CodepointGuard":"PASS" if unicode_required else "NOT_APPLICABLE","actionCount":len(actions),"unsignedSha256":hashlib.sha256(target.read_bytes()).hexdigest(),"exactNativeActionRoundtrip":True})
    after=fingerprint();assert before==after,"source drift during validation"
    report={
      "schemaVersion":1,"status":"PASS_SOURCE_AND_MODEL_ONLY","nativeRuntimeStatus":"NOT_EXECUTED",
      "featureStatus":"PARTIAL_SOURCE_IMPLEMENTED_PENDING_NATIVE",
      "sourceCommit":source_sha,
      "worktreeDirtyBeforeEvidence":source_dirty_before,
      "sourceFingerprint":before,"compilerSha256":hashlib.sha256(exe.read_bytes()).hexdigest(),
      "schemaFingerprint":caps.get("schemaFingerprint"),"macroFingerprint":caps.get("macroFingerprint"),
      "nativeMetadata":verify_native_metadata(),
      "scope":["one returned File; no provider save/share/network","default full payload redaction","opt-in Text payloads explicitly base64-utf8","non-Text payloads explicitly unsupported","dates/labels not collected"],
      "nativeAssumptionsStillRequiringSignedRuntime":["empty cleared log Repeat cardinality","native Text Base64 UTF-8 including empty Text and controls","Text envelope Base64 Encode/Decode byte conversion","Set Name on newly produced File","Repeat and If result collection for 0/1/1000 items"],
      "summary":{"cliCompiles":len(checks),"exactNativeActionRoundtrips":len(checks),"independentModelCases":sum(len(c["models"]) for c in checks),"cliNegatives":len(negatives),"causalFaults":len(faults),"nativeProbeCompiles":len(native_probes),"nativeExecuted":0,"elapsedSeconds":round(time.monotonic()-started,3)},
      "compiles":checks,"cliNegatives":negatives,"causalFaults":faults,"nativeProbes":native_probes,
    }
    report_path=ROOT/args.report;report_path.parent.mkdir(parents=True,exist_ok=True)
    report_path.write_bytes((json.dumps(report,ensure_ascii=False,indent=2)+"\n").encode())
    print(json.dumps({"report":args.report,"status":report["status"],**report["summary"]},sort_keys=True))

if __name__=="__main__":
    try:
        main()
    except Exception as error:
        destination="docs/apple-runtime/p10r15/debug-export-validation.json"
        if "--report" in sys.argv:
            destination=sys.argv[sys.argv.index("--report")+1]
        failure=ROOT/destination;failure.parent.mkdir(parents=True,exist_ok=True)
        failure.write_bytes((json.dumps({"status":"FAIL_SOURCE_VALIDATION","exceptionType":type(error).__name__,"nativeExecuted":0})+"\n").encode())
        raise
