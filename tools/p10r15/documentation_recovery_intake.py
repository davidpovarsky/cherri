#!/usr/bin/env python3
"""Seal and independently revalidate original documentation evidence, read-only.

Go stages this verifier and its reused canonical helpers from immutable Git K.
No compiler, wrapper, test, or generator is executed during verification.
"""
from __future__ import annotations
import argparse
import datetime
import hashlib
import io
import json
import math
from pathlib import Path, PurePosixPath
import plistlib
import re
import zipfile
import documentation_recovery_examples as examples
import documentation_recovery_wrappers as wrappers
import package_skill

MAX_TOTAL = 32 * 1024 * 1024
MAX_ARCHIVE = 4 * 1024 * 1024
GEN_TESTS = ("TestP15D01PrimaryParameterContract", "TestP15D01AllGeneratedSignaturesAndCategories",
             "TestP15D01CheckGateProducesAndRejectsFaults")
GEN_EVENTS = GEN_TESTS + tuple(GEN_TESTS[2]+"/"+name for name in (
    "optional_primary", "typed_default", "callable_identity", "wire_key",
    "return_type", "macro_identity", "missing_category", "extra_section"))
PARITY_EVENTS = ("TestP10_CatalogCLIAndMobileRegistryParity",)
CALLS = ("compiler-input-build-info", "compiler-build-info", "compiler-capabilities", "compiler-catalog",
    "direct-action-show", "wrapper-action-show", "direct-action-resizeimage", "wrapper-action-resizeimage",
    "direct-action-round", "wrapper-action-round", "wrapper-catalog", "wrapper-catalog-filter",
    "docs-active-0", "docs-active-1", "docs-active-2", "docs-legacy-explicit", "docs-no-match",
    "docs-invalid-regex", "missing-current-legacy-control", "docs-missing-current")

def need(value, why):
    if not value:
        raise ValueError(why)

def sha(raw):
    return hashlib.sha256(raw).hexdigest()

def canonical(value):
    return wrappers.canonical(value)

def read_archive(path):
    need(path.is_file() and not path.is_symlink() and path.stat().st_size <= MAX_ARCHIVE, "nested archive missing/linked/oversized")
    with zipfile.ZipFile(path) as archive:
        infos = archive.infolist()
        need(0 < len(infos) <= 4096, "nested archive entry budget")
        names = [row.filename for row in infos]
        need(len(names) == len(set(names)), "duplicate nested archive entry")
        total = 0
        files = {}
        for item in infos:
            pure = PurePosixPath(item.filename)
            need(not item.is_dir() and not item.flag_bits & 1 and not pure.is_absolute() and
                 ".." not in pure.parts and "\\" not in item.filename and pure.as_posix() == item.filename,
                 "unsafe nested archive path")
            need(0 <= item.file_size <= 4*1024*1024, "nested artifact byte budget")
            total += item.file_size
            need(total <= MAX_TOTAL, "nested aggregate byte budget")
            files[item.filename] = archive.read(item)
    need("manifest.json" in files, "missing nested manifest")
    manifest = wrappers.strict_json(files.pop("manifest.json"))
    expected = [{"path": name, "bytes": len(raw), "sha256": sha(raw)} for name,raw in sorted(files.items())]
    need(manifest.get("schemaVersion") == 1 and manifest.get("files") == expected, "nested file inventory/digest mismatch")
    return manifest, files

def file(files, path):
    need(path in files, "missing nested original: "+path)
    return files[path]

def obj(files, path):
    return wrappers.strict_json(file(files,path))

def bound_artifact(files, prefix, value, expected_path=None):
    need(isinstance(value,dict) and isinstance(value.get("path"),str), "missing original artifact identity")
    if expected_path is not None:
        need(value["path"] == expected_path, "changed original artifact role")
    path = value["path"]
    need(PurePosixPath(path).name == path and "\\" not in path, "unsafe original artifact role")
    data = file(files,prefix+path)
    need(type(value.get("bytes")) is int and len(data)==value["bytes"] and sha(data)==value.get("sha256"),
         "original artifact digest mismatch: "+prefix+path)
    return data

def check_time(receipt, duration_key, budget_key):
    duration,budget = receipt.get(duration_key),receipt.get(budget_key)
    need(type(duration) in (int,float) and type(budget) in (int,float) and
         math.isfinite(duration) and math.isfinite(budget) and 0 <= duration <= budget and budget > 0,
         "nested process deadline missing/exceeded")
    start = datetime.datetime.fromisoformat(receipt["startedUTC"].replace("Z","+00:00"))
    end = datetime.datetime.fromisoformat(receipt["endedUTC"].replace("Z","+00:00"))
    need(start.tzinfo is not None and end.tzinfo is not None and 0 <= (end-start).total_seconds() <= budget+1
         and abs((end-start).total_seconds()-duration)<=1, "nested original process timing differs")
    return start,end

def process(files,prefix,name,expected=None,wrapper=False,exit_code=None,budget=None):
    receipt = obj(files,prefix+name+".process.json")
    if expected is not None:
        need(receipt == expected, "detached nested process receipt "+name)
    need(receipt.get("timedOut") is False and type(receipt.get("exitCode")) is int and
         not receipt.get("launchError"), "failed nested process "+name)
    if exit_code is not None:
        need(receipt["exitCode"]==exit_code, "wrong nested exit "+name)
    budget_key = "timeoutSeconds" if wrapper else "budgetSeconds"
    if budget is not None:
        need(receipt.get(budget_key)==budget, "changed original process budget "+name)
    check_time(receipt,"elapsedSeconds",budget_key)
    stdout=bound_artifact(files,prefix,receipt["stdout"],name+".stdout")
    stderr=bound_artifact(files,prefix,receipt["stderr"],name+".stderr")
    return receipt,stdout,stderr

def compiler_metadata(raw, expected):
    text=raw.decode("utf8",errors="strict")
    need(re.findall(r"^\s*path\s+(\S+)\s*$",text,re.M)==["github.com/electrikmilk/cherri"], "wrong original compiler role")
    settings={}
    for value in re.findall(r"^\s*build\s+(.+?)\s*$",text,re.M):
        key,sep,value=value.partition("=")
        need(sep and key not in settings,"ambiguous original build metadata")
        settings[key]=value
    need(settings.get("vcs")=="git" and settings.get("vcs.revision")==expected and
         settings.get("vcs.modified")=="false", "stale or dirty original compiler source")
    return settings

def identity(files,prefix,identity,pins):
    need(identity.get("commit")==pins["compilerCommit"] and identity.get("role")=="github.com/electrikmilk/cherri"
         and re.fullmatch(r"[0-9a-f]{64}",identity.get("sha256","")) and type(identity.get("bytes")) is int
         and identity["bytes"]>0, "original compiler identity mismatch")
    receipt,raw,err=process(files,prefix,"compiler-build-info",identity["buildInfo"],exit_code=0,budget=30)
    need(receipt["command"]==["go","version","-m",identity["path"]] and not err,"detached compiler metadata command")
    settings=compiler_metadata(raw,pins["compilerCommit"])
    need(identity.get("goos")==settings.get("GOOS") and identity.get("goarch")==settings.get("GOARCH"),"compiler platform mismatch")
    receipt,raw,err=process(files,prefix,"compiler-capabilities",identity["capabilitiesProcess"],exit_code=0,budget=30)
    need(receipt["command"]==[identity["path"],"--capabilities-json"] and not err,"detached compiler capabilities")
    caps=wrappers.strict_json(raw);wrappers.validate_capabilities(caps,pins)
    return caps

def examples_evidence(files,report,pins,repo,docs_repo,source):
    prefix="consumers/examples/"
    need(obj(files,prefix+"documentation-examples.json")==report, "detached all88 example report")
    need(report.get("implementationSha")==source and report.get("compilerCommit")==pins["compilerCommit"]
         and report.get("docsCommit")==pins["docsCommit"] and report.get("formalAcceptanceGranted") is False
         and report.get("nativeExecution")=="NOT_RUN", "example tuple/scope mismatch")
    pin_raw=examples.git(repo,"show",source+":skills/cherri-shortcuts/compatibility-manifest.json")
    need(report.get("manifestSha256")==sha(pin_raw),"example manifest binding mismatch")
    caps=identity(files,prefix,report["compiler"],pins)
    cases=[];tokens=[];sources=[]
    for root,commit,role,paths in (
        (repo,pins["compilerCommit"],"compiler",examples.COMPILER_GUIDES),
        (docs_repo,pins["docsCommit"],"docs",examples.DOCS_GUIDES),
        (repo,source,"consumer",examples.SKILL_GUIDES)):
        blobs=wrappers.git_files(root,commit,paths)
        for path in paths:
            raw=blobs[path][0]
            sources.append({"repository":role,"commit":commit,"path":path,"bytes":len(raw),"sha256":sha(raw)})
            cases+=examples.fence_examples(raw.decode("utf8"),role,path)
            if role=="consumer":
                found,inventory=examples.skill_examples(raw.decode("utf8"),path);cases+=found;tokens+=inventory
    need(len(cases)==88 and report.get("sources")==sources and report.get("inlineInventory")==tokens,
         "all88 source-derived inventory changed")
    actual=report.get("examples")
    need(isinstance(actual,list) and len(actual)==len(cases),"missing/extra original example")
    for index,(case,row) in enumerate(zip(cases,actual)):
        base=prefix+"example-%04d/"%index
        need(obj(files,base+"example.json")==row and row.get("passed") is True and row.get("failures")==[],
             "detached/failed original example")
        need(all(row.get(key)==value for key,value in case.items()),"altered original example inventory")
        raw=file(files,base+"snippet.cherri")
        need(raw==case["source"].encode("utf8") and row.get("sourceSHA256")==sha(raw),"source snippet differs")
        binary=report["compiler"]["path"]
        if case["mode"]=="v2":
            checked,out,err=process(files,base,"check",row["check"],budget=30)
            need(checked["command"]==[binary,"check",str(Path(checked["cwd"])/"snippet.cherri"),"--json","--no-ansi"],"changed check command")
            diagnostic,errors=examples.diagnostic_errors(out)
            need(errors==case["expectedErrors"]==row.get("actualErrors"),"wrong exact intended diagnostics")
            need((bool(case["expectedErrors"]) and checked["exitCode"]!=0 and diagnostic["valid"] is False) or
                 (not case["expectedErrors"] and checked["exitCode"]==0 and diagnostic["valid"] is True),"wrong check outcome")
        compiled,out,err=process(files,base,"compile",row["compile"],budget=30)
        path=str(Path(compiled["cwd"])/"snippet.cherri")
        command=[binary,path,"--skip-sign","--no-ansi"]
        if case["mode"]=="legacy":
            command=[binary,"compile",path,"--mode=legacy","--load-all-actions","--skip-sign","--no-ansi"]
        need(compiled["command"]==command,"changed original compile command")
        if case["expectedErrors"]:
            codes=list(dict.fromkeys(re.findall(r"\bE_[A-Z0-9_]+\b",(out+b"\n"+err).decode("utf8"))))
            need(compiled["exitCode"]!=0 and base+"snippet_unsigned.shortcut" not in files and row.get("artifact") is None
                 and codes==case["expectedErrors"]==row.get("compileErrors"),"invalid example produced output/wrong diagnostics")
        else:
            need(compiled["exitCode"]==0,"valid example compilation failed")
            raw=bound_artifact(files,base,row["artifact"],"snippet_unsigned.shortcut")
            workflow=plistlib.loads(raw)
            need(isinstance(workflow,dict) and isinstance(workflow.get("WFWorkflowActions"),list)
                 and all(isinstance(a,dict) and isinstance(a.get("WFWorkflowActionIdentifier"),str) and a["WFWorkflowActionIdentifier"]
                         for a in workflow["WFWorkflowActions"]),"invalid native workflow")
            need(row["artifact"].get("actionCount")==len(workflow["WFWorkflowActions"]),"native action count differs")
    summary={"executed":88,"passed":88,"failed":0,"intendedErrorExamples":sum(bool(c["expectedErrors"]) for c in cases),
             "explicitLegacyExamples":sum(c["mode"]=="legacy" for c in cases),"contextualExamples":sum(c["classification"]=="contextual" for c in cases)}
    need(report.get("summary")==summary,"example count/type summary differs")
    return caps

def wrapper_evidence(files,report,pins,repo,docs_repo,source,caps,compiler):
    prefix="consumers/wrappers/"
    need(obj(files,prefix+"report.json")==report and report.get("status")=="PASS_PACKAGED_DOCUMENTATION_WRAPPERS"
         and report.get("inputsUnchangedAfter") is True and report.get("formalAcceptanceGranted") is False
         and report.get("tuple")=={"S":pins["compilerCommit"],"D":pins["docsCommit"],"K":source},
         "wrapper tuple/scope/input mismatch")
    expected,package_receipt=package_skill.expected_package(repo,source,"source")
    package_skill.verify_archive(io.BytesIO(file(files,"consumers/cherri-shortcuts-source.zip")),expected)
    for name,(raw,mode) in expected.items():
        need(file(files,"consumers/unpacked/"+name)==raw,"unpacked source package differs")
    need(report.get("packageManifest")==package_receipt,"package source receipt mismatch")
    need(report["compiler"]["sha256"]==compiler["sha256"] and report["compiler"]["bytes"]==compiler["bytes"]
         and report["compiler"]["compilerCommit"]==pins["compilerCommit"],"different wrapper compiler")
    docs=wrappers.git_files(docs_repo,pins["docsCommit"],wrappers.DOC_SCOPES)
    calls=report.get("calls",[])
    need(tuple(row.get("id") for row in calls)==CALLS,"incomplete/foreign wrapper call inventory")
    outputs={}
    normalize=lambda value: value.replace("\\","/").rstrip("/")
    work=normalize(calls[0]["cwd"])
    binary=normalize(report["selectedEnvironment"]["CHERRI_BIN"])
    scripts=normalize(calls[5]["command"][1]).removesuffix("/action.sh")
    need(scripts.endswith("/unpacked/cherri-shortcuts/scripts"),"foreign original package path")
    shell=calls[5]["command"][0]
    expected_commands={
        "compiler-input-build-info":["go","version","-m",normalize(report["compiler"]["originalPath"])],
        "compiler-build-info":["go","version","-m",binary],
        "compiler-capabilities":[binary,"--capabilities-json"],
        "compiler-catalog":[binary,"--actions-json"],
        "wrapper-catalog":[shell,scripts+"/catalog.sh"],
        "wrapper-catalog-filter":[shell,scripts+"/catalog.sh","resizeImage"],
        "docs-legacy-explicit":[shell,scripts+"/search-docs.sh","--legacy-reference","repeat"],
        "docs-no-match":[shell,scripts+"/search-docs.sh",wrappers.NO_MATCH],
        "docs-invalid-regex":[shell,scripts+"/search-docs.sh","["],
        "missing-current-legacy-control":[shell,scripts+"/search-docs.sh","--legacy-reference","repeat"],
        "docs-missing-current":[shell,scripts+"/search-docs.sh","repeat"]}
    for name in wrappers.ACTION_QUERIES:
        expected_commands["direct-action-"+name.lower()]=[binary,"--action="+name,"--no-ansi"]
        expected_commands["wrapper-action-"+name.lower()]=[shell,scripts+"/action.sh",name]
    for index,query in enumerate(wrappers.DOC_QUERIES):
        expected_commands["docs-active-"+str(index)]=[shell,scripts+"/search-docs.sh",query]
    for row in calls:
        command=[normalize(value) for value in row["command"]]
        need(command==[normalize(value) for value in expected_commands[row["id"]]] and normalize(row["cwd"])==work,
             "detached original wrapper process command")
        code={"docs-invalid-regex":2,"docs-missing-current":1}.get(row["id"],0)
        original,out,err=process(files,prefix+"calls/",row["id"],row,True,code,60)
        outputs[row["id"]]=(out,err)
    for name in ("compiler-input-build-info","compiler-build-info"):
        compiler_metadata(outputs[name][0],pins["compilerCommit"])
        need(not outputs[name][1],"compiler metadata error")
    raw=outputs["compiler-capabilities"][0];wrappers.validate_capabilities(wrappers.strict_json(raw),pins)
    need(wrappers.strict_json(raw)==caps,"different wrapper capabilities")
    catalog_raw=outputs["compiler-catalog"][0];catalog=wrappers.validate_catalog(wrappers.strict_json(catalog_raw),caps)
    reference=wrappers.git_files(repo,pins["compilerCommit"],("docs/language-v2/actions-reference.md",))["docs/language-v2/actions-reference.md"][0]
    wrappers.verify_reference(docs["fork-language/actions.md"][0],reference,caps,catalog)
    for name in wrappers.ACTION_QUERIES:
        direct,error=outputs["direct-action-"+name.lower()];wrapped,wrapped_error=outputs["wrapper-action-"+name.lower()]
        need(not error and not wrapped_error and wrapped==direct.rstrip(b"\n")+b"\n" and (name+"(").encode() in wrapped,
             "action wrapper output differs")
        for parameter in catalog[name].get("parameters",[]):
            need(parameter["name"].encode() in wrapped,"missing original action parameter")
            for choice in parameter.get("enumValues",[]):
                need(("'"+choice+"'").encode() in wrapped,"missing original enum")
    need(outputs["wrapper-catalog"]==(catalog_raw,b""),"full original catalog wrapper differs")
    wrappers.verify_catalog_filter(outputs["wrapper-catalog-filter"][0],catalog_raw,"resizeImage")
    docs_path=Path(report["selectedEnvironment"]["CHERRI_DOCS_DIR"])
    for index,query in enumerate(wrappers.DOC_QUERIES):
        out,err=outputs["docs-active-"+str(index)]
        need(not err and wrappers.verify_search(out,docs_path,"fork-language",query,docs)["matches"]>0,"active retrieval mismatch")
    out,err=outputs["docs-legacy-explicit"]
    need(not err and wrappers.verify_search(out,docs_path,"language","repeat",docs)["matches"]>0,"explicit legacy retrieval mismatch")
    need(outputs["docs-no-match"]==(b"",b""),"no-match control changed")
    out,err=outputs["docs-invalid-regex"]
    need(not out and b"grep:" in err,"invalid-regex control failed for wrong reason")
    missing=Path(report["calls"][-1]["cwd"])/"missing-current"
    out,err=outputs["missing-current-legacy-control"]
    need(not err and wrappers.verify_search(out,missing,"language","repeat",docs)["matches"]>0,"missing-current control lacks original legacy")
    out,err=outputs["docs-missing-current"]
    need(not out and err.startswith(b"Active V2 documentation directory is missing or linked: "),"silent legacy fallback")
    return catalog

def go_events(raw, package, required, process_receipt):
    rows=[wrappers.strict_json(line) for line in raw.splitlines()]
    required=set(required);run=set();passed=set();started=0;finished=0;previous=None
    begin,end=check_time(process_receipt,"elapsedSeconds","budgetSeconds")
    for row in rows:
        need(not finished and row.get("Package")==package,"foreign or post-completion generation event")
        moment=datetime.datetime.fromisoformat(row["Time"].replace("Z","+00:00"))
        need(moment.tzinfo is not None and begin-datetime.timedelta(seconds=1)<=moment<=end+datetime.timedelta(seconds=1)
             and (previous is None or moment>=previous),"generation event time mismatch")
        previous=moment
        test=row.get("Test","");action=row.get("Action")
        need(not test or test in required,"unselected generation event")
        if action=="start":
            need(not test and not started and not run,"duplicate package start");started=1
        elif action=="run":
            need(started==1 and test and test not in run,"duplicate/omitted run")
            if "/" in test:need(test.split("/")[0] in run and test.split("/")[0] not in passed,"child outside parent")
            run.add(test)
        elif action=="pass":
            elapsed=row.get("Elapsed")
            need(type(elapsed) in (int,float) and math.isfinite(elapsed) and 0<=elapsed<=process_receipt["budgetSeconds"],"generation event elapsed")
            if not test:
                need(started==1 and passed==required,"incomplete generation package");finished=1
            else:
                need(test in run and test not in passed and all(x in passed for x in required if x.startswith(test+"/")),"orphan/reordered pass")
                passed.add(test)
        elif action=="output":
            need(started==1 and (not test or test in run and test not in passed),"output outside execution")
        else:raise ValueError("failed/skipped/unknown generation event")
    need(started==finished==1 and run==passed==required,"missing generation tests")

def verify(repo,source,docs_repo,archive,requirement):
    need(requirement in {"RP46","BRG29"} and re.fullmatch(r"[0-9a-f]{40}",source),"foreign requirement/source")
    manifest,files=read_archive(archive)
    pins=wrappers.strict_json(examples.git(repo,"show",source+":skills/cherri-shortcuts/compatibility-manifest.json"))
    tuple_value={"S":pins["compilerCommit"],"D":pins["docsCommit"],"K":source}
    modes=["consumers"]+(["generation"] if requirement=="BRG29" else [])
    need(manifest.get("requirement")==requirement and manifest.get("tuple")==tuple_value and manifest.get("modes")==modes,
         "nested requirement/S/D/K tuple changed")
    report_hashes={}
    for mode in modes:
        raw=file(files,mode+"/report.json");report=wrappers.strict_json(raw)
        report_hashes[mode]=sha(raw)
        need(report.get("status")=="PASS_DOCUMENTATION_RECOVERY" and report.get("tuple")==tuple_value
             and report.get("implementationSHA")==source and report.get("mode")==mode and report.get("inputsUnchangedAfter") is True
             and report.get("formalAcceptanceGranted") is False and report.get("nativeStatus")=="NOT_RUN",
             "nested production report tuple/scope mismatch")
        if mode=="consumers":
            caps=examples_evidence(files,report["examples"],pins,repo,docs_repo,source)
            wrapper_evidence(files,report["wrappers"],pins,repo,docs_repo,source,caps,report["examples"]["compiler"])
        else:
            generated=file(files,"generation/actions-reference.md")
            expected=examples.git(repo,"show",pins["compilerCommit"]+":docs/language-v2/actions-reference.md")
            docs_raw=examples.git(docs_repo,"show",pins["docsCommit"]+":fork-language/actions.md")
            need(generated==expected==wrappers.reference_body(docs_raw),"fresh full generated reference drift")
            gen=report["generation"]
            need(gen.get("bytes")==len(generated) and gen.get("sha256")==sha(generated)
                 and gen.get("matchesCompilerBlob") is True and gen.get("matchesPinnedDocsBody") is True,"generation byte identity")
            identity(files,"generation/",gen["compilerIdentity"],pins)
            commands={
              "generate-reference":["go","run","./tools/language-docs"],
              "check-reference":["go","run","./tools/language-docs","--check"]}
            for name,prefix in commands.items():
                rec,out,err=process(files,"generation/",name,exit_code=0,budget=60)
                need(rec["command"][:-1]==prefix and Path(rec["command"][-1]).name=="actions-reference.md" and not err,
                     "wrong fresh generation command")
            selections=(
              ("canonical-contract","./tools/language-docs","github.com/electrikmilk/cherri/tools/language-docs",GEN_TESTS,GEN_EVENTS),
              ("snippet-parity",".","github.com/electrikmilk/cherri",PARITY_EVENTS,PARITY_EVENTS))
            for name,package,package_id,parents,events in selections:
                rec,out,err=process(files,"generation/",name,exit_code=0,budget=90)
                expected_cmd=["go","test",package,"-count=1","-json","-timeout=90s","-run","^("+"|".join(parents)+")$"]
                need(rec["command"]==expected_cmd and not err,"wrong canonical contract command")
                go_events(out,package_id,events,rec)
    need(manifest.get("reportSha256")==report_hashes,"nested report bytes changed")
    return {"schemaVersion":1,"requirement":requirement,"tuple":tuple_value,"modes":modes,
            "reportSha256":report_hashes,"archiveSha256":sha(archive.read_bytes()),
            "originalFiles":len(files),"status":"VERIFIED_NESTED_DOCUMENTATION_EVIDENCE"}

def pack(repo,source,docs_repo,directory,archive,requirement):
    need(not archive.exists(),"nested archive output already exists")
    modes=["consumers"]+(["generation"] if requirement=="BRG29" else [])
    files={};hashes={}
    for mode in modes:
        matches=list(directory.glob(mode+"-*/collection"))
        need(len(matches)==1,"missing/duplicate executed documentation collection")
        for p in sorted(matches[0].rglob("*")):
            if not p.is_file():continue
            need(not p.is_symlink() and not getattr(p,"is_junction",lambda:False)(),"linked nested original")
            relative=p.relative_to(matches[0]).as_posix()
            # Runtime binary identity and original build metadata are retained;
            # executable copies are reproducible inputs, never evidence replay.
            if relative in {"wrappers/work/cherri.exe","wrappers/work/cherri"}:
                continue
            raw=p.read_bytes()
            need(len(raw)<=4*1024*1024,"nested original exceeds object budget")
            files[mode+"/"+relative]=raw
        hashes[mode]=sha(file(files,mode+"/report.json"))
    pins=wrappers.strict_json(examples.git(repo,"show",source+":skills/cherri-shortcuts/compatibility-manifest.json"))
    manifest={"schemaVersion":1,"requirement":requirement,"tuple":{"S":pins["compilerCommit"],"D":pins["docsCommit"],"K":source},
              "modes":modes,"reportSha256":hashes,
              "files":[{"path":name,"bytes":len(raw),"sha256":sha(raw)}for name,raw in sorted(files.items())]}
    with zipfile.ZipFile(archive,"x",compression=zipfile.ZIP_DEFLATED,compresslevel=9) as z:
        for name,raw in sorted(files.items()):z.writestr(name,raw)
        z.writestr("manifest.json",canonical(manifest))
    return verify(repo,source,docs_repo,archive,requirement)

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument("operation",choices=("pack","verify"))
    parser.add_argument("--repo",type=Path,required=True)
    parser.add_argument("--source",required=True)
    parser.add_argument("--docs-repo",type=Path,required=True)
    parser.add_argument("--archive",type=Path,required=True)
    parser.add_argument("--requirement",required=True)
    parser.add_argument("--directory",type=Path)
    args=parser.parse_args()
    if args.operation=="pack":
        result=pack(args.repo,args.source,args.docs_repo,args.directory,args.archive,args.requirement)
    else:
        result=verify(args.repo,args.source,args.docs_repo,args.archive,args.requirement)
    print(json.dumps(result,sort_keys=True))
    return 0

if __name__=="__main__":
    raise SystemExit(main())
