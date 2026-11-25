#!/usr/bin/env python3
"""Retain the three original files of one fixed public storage diagnostic.

This transport adapter does not authenticate an Apple signature, import,
execution, persistence, cache TTL or formal acceptance. The ordinary text
publication boundary is unchanged.
"""
import argparse
import json
import os
from pathlib import Path
import plistlib
import shutil
import subprocess
import sys
import tempfile

import prepare_public_artifacts as publication
from nested_list_public_originals import git_bytes, new_output, public_payloads, write_payloads
from runtime_poc_publication import identity, read_contained, read_text_data

FIXTURE = "qa-storage-delete-v2"
SOURCE_PATH = "tools/shortcuts-simulator-qa/fixtures/" + FIXTURE + ".cherri"
SOURCE_SHA256 = "4361d16d59d97a81c93da7e8d3d81168f2c10caa25db67229a18bb848edd05fc"
KEY = "cherri.qa.p15.storage.delete.35a6b60"
EXPECTED_SUMMARY = "CHERRI_QA_STORAGE_DELETE_RESULT:BEFORE_OK:NO_VALUE_OK"
MANIFEST_PATH = "docs/apple-runtime/p10r15/storage-probe-manifest.json"
RECEIPT = "storage-delete-public-originals.json"
POLICY = "EXACT_ONE_REVIEWED_PUBLIC_STORAGE_DELETE_FIXTURE"
ALLOWED_ACTIONS = {
    "comment", "nothing", "gettext", "setvariable", "appendvariable",
    "text.combine", "output", "setstoredcontent", "getstoredcontent",
    "deletestoredcontent", "getitemtype", "conditional", "count",
}


def require(ok, code):
    if not ok:
        raise publication.PublicationBlocked("STORAGE_PUBLIC_" + code)


def sha(raw):
    return publication.sha(raw)


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"),
                      ensure_ascii=False, allow_nan=False).encode("utf-8")


def same(left, right):
    return canonical(left) == canonical(right)


def pin(path, raw):
    return {"path": path, "bytes": len(raw), "sha256": sha(raw)}


def verify_context(repo, fixture, commit, run_id, attempt):
    identity(commit, run_id, attempt)
    require(fixture == FIXTURE, "UNREVIEWED_FIXTURE")
    require(git_bytes(repo, "rev-parse", "HEAD").decode().strip() == commit,
            "CHECKOUT_SOURCE")
    raw = read_contained(repo, SOURCE_PATH)
    require(0 < len(raw) <= 65536 and sha(raw) == SOURCE_SHA256, "FIXED_SOURCE")
    require(git_bytes(repo, "show", commit + ":" + SOURCE_PATH) == raw,
            "COMMITTED_SOURCE")
    read_text_data(raw, SOURCE_PATH)
    manifest_raw = read_contained(repo, MANIFEST_PATH)
    require(git_bytes(repo, "show", commit + ":" + MANIFEST_PATH) == manifest_raw,
            "COMMITTED_CONTRACT")
    manifest = read_text_data(manifest_raw, MANIFEST_PATH)
    cases = manifest.get("cases")
    require(isinstance(cases, list) and all(isinstance(row, dict) for row in cases),
            "CONTRACT_ROWS")
    selected = [row for row in cases if row.get("id") == FIXTURE]
    require(len(selected) == 1 and selected[0].get("path") == SOURCE_PATH
            and selected[0].get("sourceSha256") == SOURCE_SHA256
            and selected[0].get("expectedSummary") == EXPECTED_SUMMARY,
            "CONTRACT_IDENTITY")
    return {"source": pin(SOURCE_PATH, raw), "contract": pin(MANIFEST_PATH, manifest_raw)}


def original_paths():
    return ("builds/" + FIXTURE + ".cherri",
            "builds/" + FIXTURE + ".shortcut",
            "builds/signed/" + FIXTURE + ".shortcut")


def bounded_storage_graph(unsigned):
    """Bound published effects, not a substitute for the Go semantic oracle."""
    require(0 < len(unsigned) <= 2 * 1024 * 1024, "PLIST_SIZE")
    read_text_data(unsigned, "storage-delete.shortcut")
    document = plistlib.loads(unsigned)
    require(isinstance(document, dict), "PLIST")
    actions = document.get("WFWorkflowActions")
    require(isinstance(actions, list) and 0 < len(actions) <= 512
            and all(isinstance(a, dict) for a in actions), "ACTIONS")
    operations = []
    for action in actions:
        identifier = action.get("WFWorkflowActionIdentifier")
        parameters = action.get("WFWorkflowActionParameters")
        require(isinstance(identifier, str)
                and identifier.startswith("is.workflow.actions.")
                and identifier[len("is.workflow.actions."):] in ALLOWED_ACTIONS
                and isinstance(parameters, dict), "UNREVIEWED_EFFECT")
        suffix = identifier[len("is.workflow.actions."):]
        if suffix.endswith("storedcontent"):
            require(parameters.get("WFStoredContentKey") == KEY
                    and parameters.get("WFStoredContentGlobalValue") is True,
                    "STORAGE_NAMESPACE_SCOPE")
            operations.append(suffix)
            if suffix == "setstoredcontent":
                require(parameters.get("WFInput") == "P15_STORAGE_DELETE_ME",
                        "STORED_PUBLIC_LITERAL")
    require(operations == ["setstoredcontent", "getstoredcontent",
                           "deletestoredcontent", "getstoredcontent"],
            "STORAGE_OPERATION_ORDER")
    return actions


def provenance(root, public, commit):
    names = original_paths()
    limits = (65536, 2 * 1024 * 1024, 10 * 1024 * 1024)
    payloads = {}
    for name, maximum in zip(names, limits):
        raw = read_contained(root, name)
        require(0 < len(raw) <= maximum, "ORIGINAL_SIZE")
        payloads[name] = raw
    source, unsigned, signed = (payloads[name] for name in names)
    require(sha(source) == SOURCE_SHA256, "ORIGINAL_SOURCE")
    read_text_data(source, names[0])
    actions = bounded_storage_graph(unsigned)
    require(signed.startswith(b"AEA1") and len(signed) >= 16, "SIGNED_HEADER")
    bindings = {role: pin(name, payloads[name]) for role, name in zip(
        ("compiledSource", "unsignedShortcut", "signedShortcut"), names)}
    parsed = {}
    for role, name in (("compilerManifest", "compiler-manifest.json"),
                       ("signingProvenance", "runtime/signing-provenance.json")):
        raw = read_contained(root, name)
        require(name in public and raw == public[name], "PUBLIC_ORIGINAL_JOIN")
        parsed[role] = read_text_data(raw, name)
        bindings[role] = pin(name, raw)
    manifest = parsed["compilerManifest"]
    require(isinstance(manifest, dict) and manifest.get("syntheticOnly") is True
            and manifest.get("compilerCommitSha") == commit, "COMPILER_IDENTITY")
    rows = manifest.get("fixtures")
    require(isinstance(rows, list) and all(isinstance(row, dict) for row in rows),
            "COMPILER_ROWS")
    ids = [row.get("fixtureId") for row in rows]
    require(all(isinstance(fid, str) for fid in ids) and len(ids) == len(set(ids)),
            "COMPILER_DUPLICATE")
    selected = [row for row in rows if row.get("fixtureId") == FIXTURE]
    require(len(selected) == 1, "COMPILER_FIXTURE")
    for key, expected in {
        "fileName": FIXTURE + ".cherri", "dialect": "v2", "buildProfile": "debug",
        "runtimeTraceExpectation": "REQUIRED_NDJSON", "sourceSha256": SOURCE_SHA256,
        "unsignedPlistSha256": sha(unsigned), "nativeActionCount": len(actions),
        "actionIdentifiers": [a["WFWorkflowActionIdentifier"] for a in actions],
    }.items():
        require(same(selected[0].get(key), expected), "COMPILER_" + key)
    signing = parsed["signingProvenance"]
    require(isinstance(signing, dict) and signing.get("compilerCommitSha") == commit
            and signing.get("signedArtifactValidAEA1") is True
            and signing.get("signedArtifactSha256") == sha(signed)
            and type(signing.get("signedByteSize")) is int
            and signing["signedByteSize"] == len(signed), "SIGNING_SUMMARY")
    target = signing.get("targetProvenance")
    require(isinstance(target, dict), "SIGNING_TARGET")
    for key, expected in {
        "compilerCommitSha": commit, "sourceSha256": SOURCE_SHA256,
        "unsignedPlistSha256": sha(unsigned), "unsignedByteSize": len(unsigned),
        "nativeActionCount": len(actions), "signedArtifactSha256": sha(signed),
        "signedByteSize": len(signed), "headerMagic": "AEA1",
    }.items():
        require(same(target.get(key), expected), "SIGNING_" + key)
    return payloads, bindings


def stage(repo, root, public_root, output, fixture, commit, run_id, attempt):
    context = verify_context(repo, fixture, commit, run_id, attempt)
    public = public_payloads(repo, public_root, commit, run_id)
    payloads, bindings = provenance(root, public, commit)
    receipt = {
        "schemaVersion": "1", "policy": POLICY, "syntheticOnly": True,
        "implementationSha": commit, "runId": run_id, "runAttempt": attempt,
        "fixtureId": FIXTURE, "sourceSha256": SOURCE_SHA256,
        "sourceContext": context, "inputBindings": bindings,
        "publicTextPublication": pin("publication-manifest.json",
                                     public["publication-manifest.json"]),
        "originals": [pin(name, payloads[name]) for name in sorted(payloads)],
        "originalBytesUnchanged": True, "rasterPublicationAllowed": False,
        "appleSignatureAuthenticated": False, "appleExecutionAuthenticated": False,
        "storageSemanticsVerified": False, "cacheAcceptanceGranted": False,
        "formalAcceptanceGranted": False,
        "scope": "Exact original public storage-delete transport only; native results require independently authenticated workflow, import, callback and trace originals.",
    }
    output = new_output(output, (repo, root, public_root))
    temporary = Path(tempfile.mkdtemp(prefix=".storage-public-", dir=output.parent))
    try:
        write_payloads(temporary, {**payloads, RECEIPT: canonical(receipt)})
        temporary.rename(output)
    finally:
        if temporary.exists():
            shutil.rmtree(temporary)
    return receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("repo", "root", "public-root", "output"):
        parser.add_argument("--" + name, type=Path, required=True)
    parser.add_argument("--fixture", choices=(FIXTURE,), required=True)
    for name in ("commit", "run-id", "attempt"):
        parser.add_argument("--" + name, required=True)
    args = parser.parse_args()
    try:
        stage(args.repo, args.root, args.public_root, args.output,
              args.fixture, args.commit, args.run_id, args.attempt)
        if os.environ.get("GITHUB_OUTPUT"):
            with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as stream:
                stream.write("ready=true\n")
        print(json.dumps({"status": "PUBLIC_ORIGINALS_STAGED", "fixtureId": FIXTURE,
                          "runId": args.run_id, "formalAcceptanceGranted": False}))
    except (OSError, ValueError, TypeError, KeyError, subprocess.SubprocessError,
            plistlib.InvalidFileException):
        print("STORAGE_PUBLIC_ORIGINALS_BLOCKED", file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
