#!/usr/bin/env python3
"""Compile isolated typed-label representation probes; never claim native success.

Uses the existing compiler/import pipeline and named Apple actions. The resulting
unsigned files can be selected explicitly by the existing signed runtime harness.
No user shortcut, network endpoint, file provider or installed Skill is touched.
"""
import argparse
import collections
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import plistlib
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def command(args, required=True, timeout=90, env=None):
    process = subprocess.run([str(a) for a in args], cwd=ROOT, capture_output=True,
                             encoding="utf-8", errors="replace", timeout=timeout, env=env)
    if required and process.returncode:
        raise RuntimeError((process.stdout + process.stderr)[-2500:])
    return process


def input_fingerprint(manifest_path, cases):
    sys.dont_write_bytecode = True
    folder = ROOT / "tools/p10r3"
    sys.path.insert(0, str(folder))
    try:
        spec = importlib.util.spec_from_file_location("label_probe_source_scope", folder / "generate_action_contract_coverage.py")
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        baseline = module.source_fingerprint(ROOT)
    finally:
        sys.path.pop(0)
    paths = [Path(__file__).resolve(), manifest_path, ROOT / "docs/apple-runtime/macos-27/builtin-actions.json"]
    paths.extend(ROOT / item["path"] for item in cases)
    extra = {p.relative_to(ROOT).as_posix():hashlib.sha256(p.read_bytes().replace(b"\r\n",b"\n")).hexdigest() for p in paths}
    return hashlib.sha256(json.dumps({"compilerSource":baseline,"probeInputs":extra},sort_keys=True).encode()).hexdigest()


def refs(value):
    if isinstance(value, dict):
        if value.get("Type") == "ActionOutput":
            yield value.get("OutputUUID", "")
        for child in value.values():
            yield from refs(child)
    elif isinstance(value, list):
        for child in value:
            yield from refs(child)


def parameters(action):
    return action.get("WFWorkflowActionParameters", {})



def validate_probe_unicode(cases):
    verified = []
    for spec in cases:
        if "carrier" not in spec["id"] and "setname" not in spec["id"]:
            continue
        path = ROOT / spec["path"]
        source = path.read_text(encoding="utf-8")
        expected = ("\u05ea\u05d5\u05d5\u05d9\u05ea" if "carrier" in spec["id"] else "\u05e9\u05dc\u05d5\u05dd")
        if source.count(expected) != 2 or source.count("\U0001f352") != 2 or "????" in source:
            raise RuntimeError("UNICODE_PROBE_SOURCE_BYTES_INVALID:" + spec["id"])
        verified.append({"id":spec["id"], "sha256":sha(path),
                         "hebrewOccurrences":source.count(expected),
                         "cherryOccurrences":source.count("\U0001f352"),
                         "utf8Roundtrip":source.encode("utf-8")==path.read_bytes()})
    if len(verified) != 6:
        raise RuntimeError("UNICODE_PROBE_DENOMINATOR_CHANGED")
    return verified


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output-dir", type=Path, default=ROOT / "dist/p15-label-probes")
    parser.add_argument("--report", type=Path, default=ROOT / "docs/apple-runtime/p10r15/debug-label-probes.json")
    opts = parser.parse_args()
    output = opts.output_dir.resolve()
    output.mkdir(parents=True, exist_ok=True)
    manifest_path = ROOT / "tools/shortcuts-simulator-qa/fixtures/p15-label-probes/manifest.json"
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    unicode_sources = validate_probe_unicode(manifest["cases"])
    source_fingerprint = input_fingerprint(manifest_path, manifest["cases"])
    source_sha = command(["git","rev-parse","HEAD"]).stdout.strip()
    source_dirty = bool(command(["git","status","--porcelain"]).stdout.strip())
    binary = output / ("compiler.exe" if os.name == "nt" else "compiler")
    build_args = ["go","build","-trimpath","-o",binary,"."]
    build_env = os.environ.copy()
    build_env["CGO_ENABLED"] = "0"
    command(build_args, timeout=300, env=build_env)
    caps = json.loads(command([binary, "--capabilities-json"]).stdout)
    debug = next(x for x in caps["compilerMacros"] if x["callableName"] == "debugLog")
    label_declared = any(p["id"] == "label" for p in debug.get("parameters", []))
    native_path = ROOT / "docs/apple-runtime/macos-27/builtin-actions.json"
    native = json.loads(native_path.read_text(encoding="utf-8"))
    setter = native["is.workflow.actions.setvalueforkey"]
    getter = native["is.workflow.actions.getvalueforkey"]
    namer = native["is.workflow.actions.setitemname"]
    value_param = next(p for p in setter["Parameters"] if p["Key"] == "WFDictionaryValue")
    metadata = {"source":native_path.relative_to(ROOT).as_posix(), "sha256":sha(native_path),
                "dictionaryValueParameterClass":value_param["Class"],
                "dictionaryGetterOutputClasses":getter["Output"]["Types"],
                "setNameInputMultiple":namer["Input"]["Multiple"],
                "setNameOutputMultiple":namer["Output"]["Multiple"],
                "setNameInputDeterminesOutputType":namer["Input"]["InputTypeDeterminesOutputType"],
                "copyIsolationGuaranteedByMetadata":False,
                "universalNativeObjectCarrierEstablished":False}
    rows = []
    prohibited = {"is.workflow.actions.downloadurl","is.workflow.actions.documentpicker.save",
                  "is.workflow.actions.setstoredcontent","is.workflow.actions.deletestoredcontent",
                  "is.workflow.actions.runworkflow","is.workflow.actions.runself","is.workflow.actions.share"}
    for spec in manifest["cases"]:
        source = ROOT / spec["path"]
        target = output / (spec["id"] + ".shortcut")
        target.unlink(missing_ok=True)
        args = [binary,"compile",source,"--output="+str(target),"--mode="+spec["dialect"],
                "--build-profile=debug","--skip-sign","--load-all-actions","--no-ansi"]
        process = command(args,required=False)
        row = dict(spec, sourceSHA256=sha(source), compilerExitCode=process.returncode,
                   artifactPresent=target.exists(), nativeStatus="NOT_EXECUTED",
                   diagnosticTail=(process.stdout+process.stderr)[-1600:])
        if process.returncode or not target.exists():
            row["structuralStatus"]="COMPILER_REJECTED_PROBE"
            rows.append(row)
            print(json.dumps({"id":spec["id"],"status":row["structuralStatus"],"diagnostic":row["diagnosticTail"][:700]}),flush=True)
            continue
        actions = plistlib.loads(target.read_bytes())["WFWorkflowActions"]
        ids = collections.Counter(a["WFWorkflowActionIdentifier"] for a in actions)
        seen, dangling, references = set(), [], 0
        for index, action in enumerate(actions):
            params = parameters(action)
            for reference in refs(params):
                references += 1
                if reference not in seen:
                    dangling.append({"index":index,"uuid":reference})
            if params.get("UUID"):
                seen.add(params["UUID"])
        setter_wires = []
        for action in actions:
            if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvalueforkey":
                value = parameters(action).get("WFDictionaryValue")
                setter_wires.append({"key":parameters(action).get("WFDictionaryKey"),
                    "serializationType":value.get("WFSerializationType") if isinstance(value,dict) else type(value).__name__,
                    "outputReferenceCount":len(list(refs(value)))})
        imported = output / (spec["id"] + ".expanded.cherri")
        rebuilt = output / (spec["id"] + ".rebuilt.shortcut")
        imported.unlink(missing_ok=True)
        rebuilt.unlink(missing_ok=True)
        decompile = command([binary,"import",target,"--output="+str(imported),"--mode=v2","--fidelity=lossless","--no-ansi"],required=False)
        recomp = None
        exact = False
        if decompile.returncode == 0 and imported.exists():
            recomp = command([binary,"compile",imported,"--output="+str(rebuilt),"--mode=v2","--skip-sign","--no-ansi"],required=False)
            if recomp.returncode == 0 and rebuilt.exists():
                exact = plistlib.loads(rebuilt.read_bytes())["WFWorkflowActions"] == actions
        forbidden = sorted(set(ids) & prohibited)
        row.update(unsignedSHA256=sha(target), nativeActionCount=len(actions), nativeActionIDs=dict(sorted(ids.items())),
                   exactPrecedingReferenceCount=references, danglingReferenceCount=len(dangling),
                   payloadSetterWire=setter_wires, prohibitedExternalEffects=forbidden,
                   importExitCode=decompile.returncode,recompileExitCode=recomp.returncode if recomp else None,
                   exactLosslessNativeActionRoundtrip=exact)
        valid = not dangling and not forbidden and exact and references > 0
        row["structuralStatus"]="COMPILED_UNSIGNED_READY_FOR_NATIVE_PROBE" if valid else "STRUCTURAL_FAILURE"
        rows.append(row)
        print(json.dumps({"id":spec["id"],"status":row["structuralStatus"],"actions":len(actions)}),flush=True)
    good = sum(r["structuralStatus"]=="COMPILED_UNSIGNED_READY_FOR_NATIVE_PROBE" for r in rows)
    if input_fingerprint(manifest_path, manifest["cases"]) != source_fingerprint:
        raise RuntimeError("SOURCE_CHANGED_DURING_LABEL_PROBES")
    report = {"formatVersion":1,"compilerSourceSha":source_sha, "sourceFingerprint":source_fingerprint,
        "worktreeDirtyBeforeBuild":source_dirty, "buildCommand":["go","build","-trimpath","-o","<output-dir>/compiler","."],
        "goVersion":command(["go","version"]).stdout.strip(),
        "compilerSHA256":sha(binary),"manifestSHA256":sha(manifest_path),"schemaFingerprint":caps["schemaFingerprint"],
        "macroFingerprint":caps["macroFingerprint"],"debugLogLabelDeclared":label_declared,
        "metadataBoundary":metadata,"unicodeSourceBytes":unicode_sources,"cases":rows,"summary":{"probes":len(rows),"compiledReady":good,
        "nativeExecuted":0,"labelFeatureStatus":"OPEN_REPRESENTATION_UNPROVEN"},
        "privacy":{"privateCorpusRead":False,"signerUsed":False,"networkExecuted":False,"installedConsumerChanged":False}}
    opts.report.parent.mkdir(parents=True,exist_ok=True)
    opts.report.write_bytes((json.dumps(report,ensure_ascii=False,indent=2)+"\n").encode("utf-8"))
    print(json.dumps(report["summary"]),flush=True)
    return 0 if good == len(rows) and not label_declared else 1


if __name__ == "__main__":
    raise SystemExit(main())
