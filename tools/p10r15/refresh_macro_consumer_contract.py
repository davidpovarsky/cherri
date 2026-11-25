#!/usr/bin/env python3
"""Capture the original compiler/docs producers for the pinned 2026-11-25 refresh.

This adapter never edits a checkout or installed Skill. Its manifest is a
proposal, and retaining an old docsCommit does not certify that docs release.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import stat
import subprocess
import sys
import time

SOURCE = "12f07521b3e9612b97f4e1fc52ccbee40e3b867a"
TREE = "7e445fe46da2c26832b5532b3cb344bbb1843c1c"
MACROS = (
    "cache", "debugAssert", "debugClearLogs", "debugExport",
    "debugExportTrace", "debugFinish", "debugLog", "debugLogPrevious",
    "debugShowLogs", "debugStartTimer", "debugStopTimer", "debugTrace",
    "debugTraceEvent", "measure", "paginate", "require",
)
TOTAL_SECONDS = 360


def strict_json(raw):
    def unique(pairs):
        out = {}
        for key, value in pairs:
            if key in out:
                raise ValueError("duplicate JSON key: " + key)
            out[key] = value
        return out
    return json.loads(raw, object_pairs_hook=unique,
                      parse_constant=lambda value: (_ for _ in ()).throw(
                          ValueError("nonfinite JSON token: " + value)))


def digest(data):
    return hashlib.sha256(data).hexdigest()


def validate_products(capabilities, manifest, reference):
    if not isinstance(capabilities, dict) or not isinstance(manifest, dict):
        raise ValueError("capabilities and manifest must be objects")
    expected_versions = {"languageVersion": "2.0", "catalogSchemaVersion": "3.0"}
    for key, expected in expected_versions.items():
        if capabilities.get(key) != expected:
            raise ValueError("capability version differs: " + key)
    for key in ("schemaFingerprint", "codecFingerprint", "macroFingerprint"):
        value = capabilities.get(key)
        if not isinstance(value, str) or not re.fullmatch(r"[0-9a-f]{64}", value):
            raise ValueError("invalid capability identity: " + key)
    if type(capabilities.get("actionCount")) is not int or capabilities["actionCount"] != 505:
        raise ValueError("actual native catalog count is not 505")
    macros = capabilities.get("compilerMacros")
    if not isinstance(macros, list) or any(not isinstance(item, dict) for item in macros):
        raise ValueError("compiler macros are not an actual descriptor array")
    names = [item.get("callableName") for item in macros]
    if names != list(MACROS):
        raise ValueError("actual macro inventory differs from the original 16-name selection")
    for key in ("compilerCommit", "docsCommit"):
        if not isinstance(manifest.get(key), str) or not re.fullmatch(r"[0-9a-f]{40}", manifest[key]):
            raise ValueError("manifest source pin is not immutable: " + key)
    for title, key in (("Schema Fingerprint", "schemaFingerprint"),
                       ("Compiler Macro Fingerprint", "macroFingerprint")):
        found = re.findall(r"^\*\*" + re.escape(title) + r":\*\* \x60([0-9a-f]{64})\x60 *$",
                           reference, re.MULTILINE)
        if found != [capabilities[key]]:
            raise ValueError("generated docs identity differs: " + key)
    if re.findall(r"^\*\*Total Actions:\*\* (\d+) *$", reference, re.MULTILINE) != ["505"]:
        raise ValueError("generated docs native count differs")
    if re.findall(r"^\*\*Compiler Macros:\*\* (\d+) *$", reference, re.MULTILINE) != ["16"]:
        raise ValueError("generated docs macro count differs")
    marker = "## Compiler macros\n"
    if reference.count(marker) != 1:
        raise ValueError("generated docs must have one original macro section")
    macro_part = reference.split(marker, 1)[1]
    headings = re.findall(r"^### \x60([^\x60]+)\x60$", macro_part, re.MULTILINE)
    if headings != list(MACROS):
        raise ValueError("generated macro headings differ from actual capabilities")
    # Pins describe the immutable source actually used, never a newer digest
    # coupled to the former compiler commit. The docs pin remains historical.
    proposed = dict(manifest)
    for key in ("languageVersion", "catalogSchemaVersion", "schemaFingerprint",
                "codecFingerprint", "macroFingerprint"):
        proposed[key] = capabilities[key]
    proposed["compilerCommit"] = SOURCE
    return proposed


def write_json(path, value):
    raw = (json.dumps(value, ensure_ascii=False, sort_keys=True, indent=2) + "\n").encode()
    with path.open("xb") as stream:
        stream.write(raw)
    return {"path": path.name, "bytes": len(raw), "sha256": digest(raw)}


def git(source, *args):
    result = subprocess.run(["git", "--no-replace-objects", "-C", str(source), *args],
                            check=True, capture_output=True, timeout=30)
    return result.stdout


def source_inventory(source):
    if git(source, "rev-parse", "HEAD").decode().strip() != SOURCE:
        raise ValueError("source commit differs from fixed selection")
    if git(source, "rev-parse", "HEAD^{tree}").decode().strip() != TREE:
        raise ValueError("source tree differs from fixed selection")
    if git(source, "status", "--porcelain=v1", "--untracked-files=all"):
        raise ValueError("source checkout is not clean")
    items = []
    for entry in git(source, "ls-files", "--stage", "-z").split(b"\0"):
        if not entry:
            continue
        metadata, raw_name = entry.split(b"\t", 1)
        mode, oid, stage = metadata.decode().split()
        name = raw_name.decode("utf-8")
        if stage != "0" or mode not in ("100644", "100755", "120000"):
            raise ValueError("unsupported source entry: " + name)
        path = source / name
        if path.is_symlink():
            if mode != "120000":
                raise ValueError("unexpected source symlink: " + name)
            raw = os.readlink(path).encode()
        else:
            if mode == "120000" or not path.is_file():
                raise ValueError("source file not physically hydrated: " + name)
            raw = path.read_bytes()
            executable = bool(path.stat().st_mode & stat.S_IXUSR)
            if executable != (mode == "100755"):
                raise ValueError("source executable mode differs: " + name)
        actual_oid = hashlib.sha1(b"blob " + str(len(raw)).encode() + b"\0" + raw).hexdigest()
        if actual_oid != oid:
            raise ValueError("source bytes differ from indexed Git blob: " + name)
        items.append({"path": name, "mode": mode, "gitBlob": oid,
                      "bytes": len(raw), "sha256": digest(raw)})
    if not items:
        raise ValueError("source inventory is empty")
    return {"commit": SOURCE, "tree": TREE, "clean": True, "files": items}


def run(source, output, name, argv, timeout, deadline, steps, env):
    remaining = deadline - time.monotonic()
    if remaining <= 0:
        raise TimeoutError("producer total duration exhausted before " + name)
    stdout = output / (name + ".stdout.original.bin")
    stderr = output / (name + ".stderr.original.bin")
    started = time.time()
    timed_out = False
    with stdout.open("xb") as out, stderr.open("xb") as err:
        process = subprocess.Popen(argv, cwd=source, env=env, stdout=out, stderr=err,
                                   start_new_session=True)
        try:
            returncode = process.wait(timeout=min(timeout, remaining))
        except subprocess.TimeoutExpired:
            timed_out = True
            os.killpg(process.pid, signal.SIGTERM)
            try:
                returncode = process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                returncode = process.wait(timeout=5)
    step = {"name": name, "argv": argv, "cwd": str(source), "startedUnix": started,
            "elapsedSeconds": round(time.time() - started, 6),
            "exitCode": returncode, "timedOut": timed_out,
            "stdout": {"path": stdout.name, "bytes": stdout.stat().st_size,
                       "sha256": digest(stdout.read_bytes())},
            "stderr": {"path": stderr.name, "bytes": stderr.stat().st_size,
                       "sha256": digest(stderr.read_bytes())}}
    steps.append(step)
    if timed_out or returncode:
        raise RuntimeError("original command did not complete successfully: " + name)
    return stdout.read_bytes()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source-dir", required=True, type=Path)
    parser.add_argument("--output-dir", required=True, type=Path)
    args = parser.parse_args()
    source, output = args.source_dir.resolve(), args.output_dir.resolve()
    if output == source or source in output.parents:
        raise ValueError("producer outputs must be outside the source checkout")
    output.mkdir(parents=True, exist_ok=False)
    started, deadline = time.time(), time.monotonic() + TOTAL_SECONDS
    steps = []
    report = {"schema": "cherri-fixed-macro-consumer-refresh/v1",
              "sourceCommit": SOURCE, "sourceTree": TREE, "startedUnix": started,
              "totalBoundSeconds": TOTAL_SECONDS, "status": "FAILED",
              "steps": steps, "sourceMutated": False,
              "manifestStatus": "PROPOSAL_ONLY_DOCS_PIN_NOT_SYNCHRONIZED",
              "nativeRuntimeCredit": False, "releaseCredit": False}
    before = None
    try:
        report["adapter"] = {"path": str(Path(__file__).resolve()),
                             "sha256": digest(Path(__file__).read_bytes())}
        before = source_inventory(source)
        report["sourceBefore"] = write_json(output / "source-before.original.json", before)
        env = dict(os.environ)
        env.update({"GOENV": "off", "GOWORK": "off", "GOTOOLCHAIN": "local",
                    "GOTELEMETRY": "off", "GOPROXY": "off", "GOSUMDB": "off",
                    "GOMAXPROCS": "2", "GOFLAGS": "-p=2"})
        version = run(source, output, "01-go-version", ["go", "version"], 10, deadline, steps, env)
        if not re.search(rb"go1\.(?:2[4-9]|[3-9][0-9])\.", version):
            raise ValueError("actual Go version is not a recorded compatible toolchain")
        run(source, output, "02-go-env",
            ["go", "env", "-json", "GOVERSION", "GOOS", "GOARCH", "GOMOD", "GOWORK",
             "GOTOOLCHAIN", "GOCACHE", "GOMODCACHE", "GOFLAGS"], 10, deadline, steps, env)
        compiler = output / "cherri"
        run(source, output, "03-compiler-build",
            ["go", "build", "-trimpath", "-buildvcs=true", "-o", str(compiler), "."],
            150, deadline, steps, env)
        report["compiler"] = {"path": compiler.name, "bytes": compiler.stat().st_size,
                              "sha256": digest(compiler.read_bytes())}
        build_info = run(source, output, "04-compiler-build-info", ["go", "version", "-m", str(compiler)],
                         10, deadline, steps, env)
        if ("vcs.revision=" + SOURCE).encode() not in build_info or b"vcs.modified=false" not in build_info:
            raise ValueError("actual compiler build identity differs from clean pinned source")
        raw_caps = run(source, output, "05-capabilities",
                       [str(compiler), "--capabilities-json"], 30, deadline, steps, env)
        capabilities = strict_json(raw_caps)
        reference_path = output / "actions-reference.generated.md"
        run(source, output, "06-original-docs-generator",
            ["go", "run", "./tools/language-docs", str(reference_path)],
            120, deadline, steps, env)
        run(source, output, "07-original-docs-check",
            ["go", "run", "./tools/language-docs", "--check", str(reference_path)],
            30, deadline, steps, env)
        manifest_path = source / "skills/cherri-shortcuts/compatibility-manifest.json"
        manifest_raw = manifest_path.read_bytes()
        manifest = strict_json(manifest_raw)
        reference_raw = reference_path.read_bytes()
        proposed = validate_products(capabilities, manifest, reference_raw.decode("utf-8"))
        report["reference"] = {"path": reference_path.name, "bytes": len(reference_raw),
                               "sha256": digest(reference_raw)}
        report["manifestBefore"] = {"path": str(manifest_path), "sha256": digest(manifest_raw)}
        report["manifestProposal"] = write_json(output / "compatibility-manifest.proposed.json", proposed)
        report["changedManifestFields"] = sorted(key for key in proposed if proposed[key] != manifest.get(key))
        report["currentIdentities"] = {key: capabilities[key] for key in
                                       ("schemaFingerprint", "codecFingerprint", "macroFingerprint")}
        report["nativeActionCount"] = 505
        report["macroNames"] = list(MACROS)
        report["docsCommitRetained"] = manifest["docsCommit"]
        report["status"] = "PASS_ORIGINAL_PRODUCERS_AND_PROPOSAL"
    except Exception as error:
        report["errorType"] = type(error).__name__
        report["error"] = str(error)
    finally:
        try:
            after = source_inventory(source)
            report["sourceAfter"] = write_json(output / "source-after.original.json", after)
            report["sourceUnchanged"] = before is not None and after == before
            if not report["sourceUnchanged"]:
                report["status"] = "FAILED"
        except Exception as error:
            report["status"] = "FAILED"
            report["sourceAfterError"] = str(error)
        report["elapsedSeconds"] = round(time.time() - started, 6)
        receipt = write_json(output / "result.original.json", report)
        print(json.dumps({"status": report["status"], "receipt": receipt,
                          "outputDirectory": str(output)}, sort_keys=True))
    return 0 if report["status"] == "PASS_ORIGINAL_PRODUCERS_AND_PROPOSAL" else 1


if __name__ == "__main__":
    sys.exit(main())
