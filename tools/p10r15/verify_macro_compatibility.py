#!/usr/bin/env python3
"""Causally verify optional macro compatibility using real compiler binaries.

Positive controls use actual compiler capabilities and canonical generated docs.
Only isolated manifests/docs or one capabilities field are mutated. This is a
compatibility protocol check, not Apple runtime or release promotion evidence.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import subprocess
import tempfile
import time


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def capabilities(binary):
    output = subprocess.check_output([str(binary), "--capabilities-json"], text=True)
    return json.loads(output)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--compiler", type=Path, required=True)
    parser.add_argument("--compiler-commit", required=True)
    parser.add_argument("--skill", type=Path, required=True)
    parser.add_argument("--docs-reference", type=Path, required=True)
    parser.add_argument("--expected-macro-fingerprint", required=True)
    parser.add_argument("--legacy-compiler", type=Path)
    parser.add_argument("--legacy-compiler-commit")
    parser.add_argument("--legacy-docs-reference", type=Path)
    parser.add_argument("--report", type=Path, required=True)
    opts = parser.parse_args()
    opts.compiler = opts.compiler.resolve()
    opts.skill = opts.skill.resolve()
    opts.docs_reference = opts.docs_reference.resolve()
    if not re.fullmatch(r"[a-fA-F0-9]{40}", opts.compiler_commit):
        parser.error("compiler commit must be immutable")
    if not re.fullmatch(r"[a-fA-F0-9]{64}", opts.expected_macro_fingerprint):
        parser.error("expected macro fingerprint must be 64 hex characters")
    caps = capabilities(opts.compiler)
    if caps.get("macroFingerprint") != opts.expected_macro_fingerprint:
        parser.error("real compiler does not match independently supplied macro fingerprint")
    source_manifest = json.loads((opts.skill / "compatibility-manifest.json").read_text())
    for field in ("languageVersion", "catalogSchemaVersion", "schemaFingerprint", "codecFingerprint"):
        if source_manifest.get(field) != caps.get(field):
            parser.error("native manifest/capabilities baseline mismatch: " + field)

    rows = []
    with tempfile.TemporaryDirectory(prefix="cherri-macro-compat-") as tmp:
        root = Path(tmp)
        skill = root / "skill"
        shutil.copytree(opts.skill, skill, ignore=shutil.ignore_patterns("bin", "__pycache__"))
        manifest_path = skill / "compatibility-manifest.json"
        docs = root / "docs"
        docs.mkdir()
        (docs / "fork-language").mkdir()
        action_ref = docs / "fork-language/actions.md"
        valid_docs = opts.docs_reference.read_text()
        action_ref.write_text(valid_docs)
        subprocess.run(["git", "init", "-q", str(docs)], check=True)
        subprocess.run(["git", "-C", str(docs), "add", "."], check=True)
        subprocess.run(["git", "-C", str(docs), "-c", "user.name=Cherri QA",
                        "-c", "user.email=qa@example.invalid", "commit", "-qm",
                        "Canonical docs fixture"], check=True)
        docs_ref = subprocess.check_output(["git", "-C", str(docs), "rev-parse", "HEAD"], text=True).strip()
        manifest = dict(source_manifest, compilerCommit=opts.compiler_commit,
                        docsCommit=docs_ref, macroFingerprint=opts.expected_macro_fingerprint)
        def write_manifest(value):
            manifest_path.write_text(json.dumps(value, indent=2) + "\n")
        write_manifest(manifest)
        caps_path = root / "capabilities.json"
        caps_path.write_text(json.dumps(caps, indent=2) + "\n")
        proxy = root / "capabilities-only-proxy"
        proxy.write_text("#!/bin/sh\nif [ \"$" + "{1:-}\" = --capabilities-json ]; then\n"
                         "  cat " + shlex.quote(str(caps_path)) + "\nelse\n"
                         "  exec " + shlex.quote(str(opts.compiler)) + " \"$@\"\nfi\n")
        proxy.chmod(0o700)
        Path(str(proxy) + ".commit").write_text(opts.compiler_commit + "\n")
        env = os.environ.copy()
        for key in ("CHERRI_BIN", "CHERRI_REF", "CHERRI_DOCS_REF", "CHERRI_MANIFEST_FILE",
                    "CHERRI_HOME", "CHERRI_DOCS_DIR", "SKILL_DIR"):
            env.pop(key, None)
        env.update(CHERRI_HOME=str(root / "home"), CHERRI_DOCS_DIR=str(docs),
                   SKILL_DIR=str(skill), CHERRI_OFFLINE="1")

        def check(name, expected_code=None, binary=None, compiler_ref=None):
            run_env = dict(env, CHERRI_BIN=str(binary or opts.compiler),
                           CHERRI_REF=compiler_ref or opts.compiler_commit)
            start = time.monotonic()
            process = subprocess.run(["sh", str(skill / "scripts/doctor.sh")],
                env=run_env, text=True, capture_output=True, timeout=30)
            output = process.stdout + process.stderr
            ok = process.returncode == 0 if expected_code is None else (
                process.returncode != 0 and "[" + expected_code + "]" in output
                and "[CHERRI_COMPILER_NOT_FOUND]" not in output)
            row = {"case":name, "exitCode":process.returncode, "expectedDiagnostic":expected_code,
                   "verdict":"PASS" if ok else "FAIL", "seconds":round(time.monotonic()-start,3),
                   "diagnosticTail":output.replace(str(root), "<isolated-test>")[-1800:]}
            rows.append(row)
            print(json.dumps({k:row[k] for k in ("case","exitCode","verdict")}), flush=True)

        check("actual_healthy_tuple")
        check("healthy_capabilities_proxy", binary=proxy)
        for name, value, expected in (
            ("missing", None, "CHERRI_MACRO_FINGERPRINT_MISSING"),
            ("malformed", "not-a-fingerprint", "CHERRI_MACRO_FINGERPRINT_INVALID"),
            ("mismatch", "0"*64, "CHERRI_MACRO_FINGERPRINT_MISMATCH"),
        ):
            altered = dict(caps)
            if value is None:
                altered.pop("macroFingerprint")
            else:
                altered["macroFingerprint"] = value
            caps_path.write_text(json.dumps(altered, indent=2) + "\n")
            check("compiler_macro_" + name, expected, proxy)
            caps_path.write_text(json.dumps(caps, indent=2) + "\n")
            check("compiler_restore_after_" + name, binary=proxy)

        for name, value, expected in (
            ("empty", "", "CHERRI_MACRO_MANIFEST_INVALID"),
            ("malformed", "not-a-fingerprint", "CHERRI_MACRO_MANIFEST_INVALID"),
            ("mismatch", "0"*64, "CHERRI_MACRO_FINGERPRINT_MISMATCH"),
        ):
            write_manifest(dict(manifest, macroFingerprint=value))
            check("manifest_macro_" + name, expected)
            write_manifest(manifest)
            check("manifest_restore_after_" + name)

        pattern = r"(?m)^\*\*Compiler Macro Fingerprint:\*\*[^\n]*\n?"
        if len(re.findall(pattern, valid_docs)) != 1:
            parser.error("canonical docs must contain exactly one Compiler Macro Fingerprint header")
        for name, value, expected in (
            ("missing", None, "CHERRI_DOCS_MACRO_FINGERPRINT_MISSING"),
            ("malformed", "not-a-fingerprint", "CHERRI_DOCS_MACRO_FINGERPRINT_INVALID"),
            ("mismatch", "0"*64, "CHERRI_STALE_DOCS_MACROS"),
        ):
            replacement = "" if value is None else "**Compiler Macro Fingerprint:** " + chr(96) + value + chr(96) + "\n"
            action_ref.write_text(re.sub(pattern, replacement, valid_docs))
            check("docs_macro_" + name, expected)
            action_ref.write_text(valid_docs)
            check("docs_restore_after_" + name)

        # Old manifests do not require the new contract, even with a newer binary.
        compatible_manifest = dict(manifest)
        compatible_manifest.pop("macroFingerprint")
        write_manifest(compatible_manifest)
        action_ref.write_text(re.sub(pattern, "", valid_docs))
        check("old_manifest_without_macro_field")
        if opts.legacy_compiler:
            if not opts.legacy_compiler_commit or not opts.legacy_docs_reference:
                parser.error("legacy control needs its exact commit and canonical docs")
            legacy_caps = capabilities(opts.legacy_compiler)
            if legacy_caps.get("macroFingerprint"):
                parser.error("legacy control unexpectedly exposes a macro fingerprint")
            for field in ("languageVersion","catalogSchemaVersion","schemaFingerprint","codecFingerprint"):
                if legacy_caps.get(field) != source_manifest.get(field):
                    parser.error("legacy native baseline mismatch: " + field)
            write_manifest(dict(compatible_manifest, compilerCommit=opts.legacy_compiler_commit))
            action_ref.write_text(opts.legacy_docs_reference.read_text())
            check("actual_r1_legacy_contract", binary=opts.legacy_compiler,
                  compiler_ref=opts.legacy_compiler_commit)

    report = {"formatVersion":1, "observedAtUtc":time.strftime("%Y-%m-%dT%H:%M:%SZ",time.gmtime()),
        "scope":"ACTUAL_COMPILER_CANONICAL_DOCS_ISOLATED_SINGLE_FIELD_MUTATIONS",
        "compilerCommit":opts.compiler_commit,"compilerSHA256":digest(opts.compiler),
        "macroFingerprint":opts.expected_macro_fingerprint,
        "macroCount":len(caps.get("compilerMacros",[])),
        "schemaFingerprint":caps["schemaFingerprint"],
        "canonicalDocsSHA256":digest(opts.docs_reference),
        "skillCommonSHA256":digest(opts.skill / "scripts/common.sh"),
        "legacyCompilerCommit":opts.legacy_compiler_commit,
        "legacyCompilerSHA256":digest(opts.legacy_compiler) if opts.legacy_compiler else None,
        "livePromotion":False, "results":rows,
        "summary":{"checks":len(rows),"passed":sum(r["verdict"]=="PASS" for r in rows),
                   "failed":sum(r["verdict"]=="FAIL" for r in rows)}}
    opts.report.parent.mkdir(parents=True, exist_ok=True)
    opts.report.write_text(json.dumps(report,indent=2)+"\n")
    print(json.dumps(report["summary"]),flush=True)
    return 0 if report["summary"]["failed"]==0 else 1


if __name__ == "__main__":
    raise SystemExit(main())
