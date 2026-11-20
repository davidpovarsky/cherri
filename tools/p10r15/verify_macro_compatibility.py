#!/usr/bin/env python3
"""Causally verify optional macro compatibility using real compiler binaries.

Installed positive controls use a real same-source compiler/migration pair and
canonical generated docs. Capability-field faults exercise the existing public
validator with explicitly controlled responses; they are not installed binaries.
This is a compatibility protocol check, not Apple runtime or release evidence.
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
    output = subprocess.check_output([str(binary), "--capabilities-json"], text=True,
                                     encoding="utf-8", stdin=subprocess.DEVNULL, timeout=30)
    return json.loads(output)


def inspect_pair(compiler, migrator, revision):
    """Authenticate the supplied byte receipts before either installed tool runs."""
    expected = ((compiler, "cherri", "github.com/electrikmilk/cherri"),
                (migrator, "cherri-migrate", "github.com/electrikmilk/cherri/cmd/cherri-migrate"))
    hashes = {}
    for binary, role, _ in expected:
        recorded_source = Path(str(binary) + ".commit").read_text(encoding="utf-8").strip()
        recorded_hash = Path(str(binary) + ".sha256").read_text(encoding="utf-8").strip()
        if recorded_source != revision or recorded_hash != digest(binary):
            raise ValueError("supplied " + role + " source/hash receipt does not match")
        hashes[role] = recorded_hash
    identities = {}
    for binary, role, module in expected:
        identity = json.loads(subprocess.check_output(
            [str(migrator), "--inspect-build-json", str(binary)], text=True,
            encoding="utf-8", stdin=subprocess.DEVNULL, timeout=30))
        if (identity.get("tool") != role or identity.get("modulePath") != module
                or identity.get("sourceCommit") != revision or identity.get("buildState") != "clean"
                or identity.get("sha256") != hashes[role]):
            raise ValueError("supplied " + role + " is not the exact clean role/source/bytes")
        identities[role] = identity
    for key in ("goos", "goarch"):
        if not identities["cherri"].get(key) or identities["cherri"][key] != identities["cherri-migrate"].get(key):
            raise ValueError("supplied compiler/migration platform differs: " + key)
    return identities


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--compiler", type=Path, required=True)
    parser.add_argument("--migrator", type=Path, required=True)
    parser.add_argument("--compiler-commit", required=True)
    parser.add_argument("--skill", type=Path, required=True)
    parser.add_argument("--docs-reference", type=Path, required=True)
    parser.add_argument("--expected-macro-fingerprint", required=True)
    parser.add_argument("--legacy-compiler", type=Path)
    parser.add_argument("--legacy-migrator", type=Path)
    parser.add_argument("--legacy-compiler-commit")
    parser.add_argument("--legacy-docs-reference", type=Path)
    parser.add_argument("--report", type=Path, required=True)
    opts = parser.parse_args()
    opts.compiler = opts.compiler.resolve()
    opts.migrator = opts.migrator.resolve()
    opts.skill = opts.skill.resolve()
    opts.docs_reference = opts.docs_reference.resolve()
    if not re.fullmatch(r"[a-fA-F0-9]{40}", opts.compiler_commit):
        parser.error("compiler commit must be immutable")
    if not re.fullmatch(r"[a-fA-F0-9]{64}", opts.expected_macro_fingerprint):
        parser.error("expected macro fingerprint must be 64 hex characters")
    identities = inspect_pair(opts.compiler, opts.migrator, opts.compiler_commit)
    caps = capabilities(opts.compiler)
    migration_caps = capabilities(opts.migrator)
    for field in ("languageVersion", "catalogSchemaVersion", "protocolVersion", "schemaFingerprint",
                  "codecFingerprint", "macroFingerprint"):
        if migration_caps.get(field) != caps.get(field):
            parser.error("supplied compiler/migration capabilities differ: " + field)
    if caps.get("macroFingerprint") != opts.expected_macro_fingerprint:
        parser.error("real compiler does not match independently supplied macro fingerprint")
    source_manifest = json.loads((opts.skill / "compatibility-manifest.json").read_text(encoding="utf-8"))
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
        valid_docs = opts.docs_reference.read_text(encoding="utf-8")
        action_ref.write_text(valid_docs, encoding="utf-8")
        subprocess.run(["git", "--no-replace-objects", "init", "-q", str(docs)],
                       check=True, stdin=subprocess.DEVNULL, timeout=30)
        def commit_docs(message):
            subprocess.run(["git", "--no-replace-objects", "-C", str(docs), "add", "."],
                           check=True, stdin=subprocess.DEVNULL, timeout=30)
            subprocess.run(["git", "--no-replace-objects", "-C", str(docs), "-c", "user.name=Cherri QA",
                            "-c", "user.email=qa@example.invalid", "commit", "-qm", message],
                           check=True, stdin=subprocess.DEVNULL, timeout=30)
            return subprocess.check_output(["git", "--no-replace-objects", "-C", str(docs),
                                            "rev-parse", "HEAD"], text=True,
                                           stdin=subprocess.DEVNULL, timeout=30).strip()
        docs_ref = commit_docs("Canonical docs fixture")
        manifest = dict(source_manifest, compilerCommit=opts.compiler_commit,
                        docsCommit=docs_ref, macroFingerprint=opts.expected_macro_fingerprint)
        def write_manifest(value):
            manifest_path.write_text(json.dumps(value, indent=2) + "\n", encoding="utf-8")
        write_manifest(manifest)
        caps_path = root / "capabilities.json"
        caps_path.write_text(json.dumps(caps, indent=2) + "\n", encoding="utf-8")
        proxy = root / "capabilities-only-proxy"
        observed = root / "controlled-response-observed.txt"
        proxy.write_text("#!/bin/sh\nif [ \"$" + "{1:-}\" = --capabilities-json ]; then\n"
                         "  printf '%s\\n' capabilities >> " + shlex.quote(str(observed)) + "\n"
                         "  cat " + shlex.quote(str(caps_path)) + "\nelse\n"
                         "  exit 64\nfi\n", encoding="utf-8")
        proxy.chmod(0o700)
        # These self-asserted proxy receipts are deliberately insufficient for
        # the installed-tool control below: only a real Go tool has that role.
        Path(str(proxy) + ".commit").write_text(opts.compiler_commit + "\n", encoding="utf-8")
        Path(str(proxy) + ".sha256").write_text(digest(proxy) + "\n", encoding="utf-8")
        env = os.environ.copy()
        for key in ("CHERRI_BIN", "CHERRI_MIGRATE_BIN", "CHERRI_REF", "CHERRI_DOCS_REF", "CHERRI_MANIFEST_FILE",
                    "CHERRI_HOME", "CHERRI_DOCS_DIR", "SKILL_DIR", "CHERRI_REPO", "CHERRI_DOCS_REPO"):
            env.pop(key, None)
        env.update(CHERRI_HOME=str(root / "home"), CHERRI_DOCS_DIR=str(docs),
                   SKILL_DIR=str(skill), CHERRI_OFFLINE="1", GIT_NO_REPLACE_OBJECTS="1")

        def check(name, expected_code=None, binary=None, compiler_ref=None, migrator=None,
                  controlled_response=False):
            run_env = dict(env, CHERRI_BIN=str(binary or opts.compiler),
                           CHERRI_MIGRATE_BIN=str(migrator or opts.migrator),
                           CHERRI_REF=compiler_ref or opts.compiler_commit)
            observed.unlink(missing_ok=True)
            command = ["sh", str(skill / "scripts/doctor.sh")]
            scope = "actual-installed-pair"
            if controlled_response:
                command = ["sh", "-c", '. "$SKILL_DIR/scripts/common.sh"; validate_cherri_bin "$CHERRI_BIN"',
                           str(skill / "scripts/doctor.sh")]
                scope = "controlled-capability-field-response"
            elif binary == proxy:
                scope = "controlled-proxy-installed-rejection"
            start = time.monotonic()
            process = subprocess.run(command, env=run_env, text=True, encoding="utf-8",
                stdin=subprocess.DEVNULL, capture_output=True, timeout=30)
            output = process.stdout + process.stderr
            ok = process.returncode == 0 if expected_code is None else (
                process.returncode != 0 and "[" + expected_code + "]" in output
                and "[CHERRI_COMPILER_NOT_FOUND]" not in output)
            response_observed = observed.exists() and observed.read_bytes() == b"capabilities\n"
            if binary == proxy:
                ok = ok and response_observed
            row = {"case":name, "scope":scope, "exitCode":process.returncode, "expectedDiagnostic":expected_code,
                   "verdict":"PASS" if ok else "FAIL", "seconds":round(time.monotonic()-start,3),
                   "diagnosticTail":output.replace(str(root), "<isolated-test>")[-1800:]}
            if binary == proxy:
                row.update(controlledResponseObserved=response_observed,
                           controlledResponseSHA256=digest(caps_path))
            rows.append(row)
            print(json.dumps({k:row[k] for k in ("case","exitCode","verdict")}), flush=True)

        check("actual_healthy_tuple")
        check("controlled_healthy_capabilities_response", binary=proxy, controlled_response=True)
        check("proxy_cannot_impersonate_installed_compiler", "CHERRI_TOOL_IDENTITY_INVALID", binary=proxy)
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
            caps_path.write_text(json.dumps(altered, indent=2) + "\n", encoding="utf-8")
            check("compiler_macro_" + name, expected, proxy, controlled_response=True)
            caps_path.write_text(json.dumps(caps, indent=2) + "\n", encoding="utf-8")
            check("compiler_restore_after_" + name, binary=proxy, controlled_response=True)

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
            action_ref.write_text(re.sub(pattern, replacement, valid_docs), encoding="utf-8")
            check("docs_macro_" + name, expected)
            action_ref.write_text(valid_docs, encoding="utf-8")
            check("docs_restore_after_" + name)

        # Old manifests do not require the new contract, even with a newer binary.
        compatible_manifest = dict(manifest)
        compatible_manifest.pop("macroFingerprint")
        action_ref.write_text(re.sub(pattern, "", valid_docs), encoding="utf-8")
        compatible_manifest["docsCommit"] = commit_docs("Canonical old macro-contract docs fixture")
        write_manifest(compatible_manifest)
        check("old_manifest_without_macro_field")
        if opts.legacy_compiler:
            if not opts.legacy_compiler_commit or not opts.legacy_docs_reference or not opts.legacy_migrator:
                parser.error("legacy installed control needs its exact commit, companion and canonical docs")
            opts.legacy_compiler = opts.legacy_compiler.resolve()
            opts.legacy_migrator = opts.legacy_migrator.resolve()
            inspect_pair(opts.legacy_compiler, opts.legacy_migrator, opts.legacy_compiler_commit)
            legacy_caps = capabilities(opts.legacy_compiler)
            if legacy_caps.get("macroFingerprint"):
                parser.error("legacy control unexpectedly exposes a macro fingerprint")
            for field in ("languageVersion","catalogSchemaVersion","schemaFingerprint","codecFingerprint"):
                if legacy_caps.get(field) != source_manifest.get(field):
                    parser.error("legacy native baseline mismatch: " + field)
            action_ref.write_text(opts.legacy_docs_reference.read_text(encoding="utf-8"), encoding="utf-8")
            legacy_docs_ref = commit_docs("Canonical supplied legacy compiler docs fixture")
            write_manifest(dict(compatible_manifest, compilerCommit=opts.legacy_compiler_commit,
                                docsCommit=legacy_docs_ref))
            check("actual_r1_legacy_contract", binary=opts.legacy_compiler,
                  compiler_ref=opts.legacy_compiler_commit, migrator=opts.legacy_migrator)
        subprocess.run(["git", "--no-replace-objects", "-C", str(docs), "checkout", "--quiet", "--detach", docs_ref],
                       check=True, stdin=subprocess.DEVNULL, timeout=30)
        write_manifest(manifest)
        check("actual_current_tuple_restored")

    if digest(opts.compiler) != identities["cherri"]["sha256"] or digest(opts.migrator) != identities["cherri-migrate"]["sha256"]:
        raise ValueError("supplied tool bytes changed during compatibility verification")
    report = {"formatVersion":2, "observedAtUtc":time.strftime("%Y-%m-%dT%H:%M:%SZ",time.gmtime()),
        "scope":"ACTUAL_INSTALLED_PAIR_AND_EXPLICIT_CONTROLLED_CAPABILITY_FIELD_RESPONSES",
        "compilerCommit":opts.compiler_commit,"compilerSHA256":digest(opts.compiler),
        "migratorSHA256":digest(opts.migrator), "toolIdentities":identities,
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
