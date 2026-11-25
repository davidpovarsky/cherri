#!/usr/bin/env python3
"""Retain originals only for the fixed public tagged-carrier engineering probe.

The ordinary publication gate remains text-only. Reuse its existing bounded
transport primitives; this adapter grants no Apple execution, signature
cryptography, language acceptance or physical-device proof.
"""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

import tagged_carrier_graph as graph
import tagged_carrier_runtime as runtime
import prepare_public_artifacts as publication
from nested_list_public_originals import git_bytes, public_payloads, new_output, write_payloads
from runtime_poc_publication import identity, read_contained, read_text_data

FIXTURE = runtime.FIXTURE
RECEIPT = "tagged-carrier-public-originals.json"
COLLECTION = "tagged-carrier-collection.json"
POLICY = "EXACT_ONE_REVIEWED_PUBLIC_TAGGED_CARRIER_FIXTURE"
MAX_SOURCE = 64 * 1024


def require(ok, code):
    if not ok:
        raise publication.PublicationBlocked("TAGGED_PUBLIC_" + code)


def pin(name, raw):
    return {"path": name, "bytes": len(raw), "sha256": runtime.sha(raw)}


def verify_context(repo, fixture, commit, run_id, attempt):
    identity(commit, run_id, attempt)
    require(fixture == FIXTURE, "UNREVIEWED_FIXTURE")
    require(git_bytes(repo, "rev-parse", "HEAD").decode().strip() == commit, "CHECKOUT_SOURCE")
    inputs = (
        ("tools/shortcuts-simulator-qa/fixtures/" + fixture + ".cherri", graph.SOURCE_SHA256, MAX_SOURCE),
        ("tools/shortcuts-simulator-qa/fixtures/" + fixture + ".expected.json", graph.CONTRACT_SHA256, 8192),
        (runtime.ENROLLMENT, runtime.ENROLLMENT_SHA256, 16 * 1024),
    )
    for name, expected, maximum in inputs:
        raw = read_contained(repo, name)
        require(0 < len(raw) <= maximum and runtime.sha(raw) == expected, "FIXED_SOURCE_BYTES")
        require(git_bytes(repo, "show", commit + ":" + name) == raw, "COMMITTED_SOURCE_BYTES")
        read_text_data(raw, name)
    runtime.enrollment(repo)


def original_paths(fixture):
    require(fixture == FIXTURE, "UNREVIEWED_FIXTURE")
    return ("builds/" + fixture + ".cherri",
            "builds/" + fixture + ".shortcut",
            "builds/signed/" + fixture + ".shortcut")


def raw_payloads(root, fixture):
    names = original_paths(fixture)
    payloads = {}
    for name, maximum in zip(names, (MAX_SOURCE, runtime.MAX_PLIST, 10 * 1024 * 1024)):
        raw = read_contained(root, name)
        require(0 < len(raw) <= maximum, "ORIGINAL_SIZE")
        if name == names[2]:
            require(raw.startswith(b"AEA1") and len(raw) >= 16, "SIGNED_HEADER")
        else:
            read_text_data(raw, name)
        payloads[name] = raw
    return payloads


def receipt_for(bindings, payloads, public, fixture, commit, run_id, attempt):
    return {
        "schemaVersion": "1", "policy": POLICY, "syntheticOnly": True,
        "implementationSha": commit, "runId": run_id, "runAttempt": attempt,
        "fixtureId": fixture, "sourceSha256": graph.SOURCE_SHA256,
        "contractSha256": graph.CONTRACT_SHA256, "enrollmentSha256": runtime.ENROLLMENT_SHA256,
        "artifactName": "cherri-tagged-carrier-public-originals-" + run_id + "-" + attempt,
        "publicTextPublication": pin("publication-manifest.json", public["publication-manifest.json"]),
        "inputBindings": bindings,
        "originals": [pin(name, payloads[name]) for name in sorted(payloads)],
        "originalBytesUnchanged": True, "rasterPublicationAllowed": False,
        "runtimeAcceptanceGranted": False, "appleExecutionAuthenticated": False,
        "formalAcceptanceGranted": False,
        "scope": "Three exact public synthetic originals for independent engineering observation recomputation only.",
    }


def stage(repo, root, public_root, output, fixture, commit, run_id, attempt):
    verify_context(repo, fixture, commit, run_id, attempt)
    public = public_payloads(repo, public_root, commit, run_id)
    payloads = raw_payloads(root, fixture)
    bindings = runtime.provenance(repo, root, fixture, commit)
    require(set(bindings) == {"compilerManifest", "compiledSource", "unsignedShortcut",
                              "signedShortcut", "signingProvenance"}, "PROVENANCE_KEYS")
    for role in ("compilerManifest", "signingProvenance"):
        name = bindings[role]["path"]
        require(runtime.same(bindings[role], pin(name, public[name]))
                and read_contained(root, name) == public[name], "PUBLIC_ORIGINAL_JOIN")
    for role in ("compiledSource", "unsignedShortcut", "signedShortcut"):
        name = bindings[role]["path"]
        require(runtime.same(bindings[role], pin(name, payloads[name])), "RAW_ORIGINAL_JOIN")
    receipt = receipt_for(bindings, payloads, public, fixture, commit, run_id, attempt)
    output = new_output(output, (root, public_root))
    temporary = Path(tempfile.mkdtemp(prefix=".tagged-public-", dir=output.parent))
    try:
        write_payloads(temporary, {**payloads, RECEIPT: runtime.canonical(receipt)})
        temporary.rename(output)
    finally:
        if temporary.exists():
            shutil.rmtree(temporary)
    return receipt


def collect(repo, bundle, public_root, output, fixture, commit, run_id, attempt):
    """Transport authentication belongs to the caller, before this exact join."""
    verify_context(repo, fixture, commit, run_id, attempt)
    public = public_payloads(repo, public_root, commit, run_id)
    payloads = raw_payloads(bundle, fixture)
    entries = list(Path(bundle).rglob("*"))
    require(not any(path.is_symlink() for path in entries)
            and {path.relative_to(bundle).as_posix() for path in entries if path.is_file()}
            == set(payloads) | {RECEIPT}, "BUNDLE_INVENTORY")
    raw_receipt = read_contained(bundle, RECEIPT)
    require(len(raw_receipt) <= 1024 * 1024, "RECEIPT_SIZE")
    receipt = read_text_data(raw_receipt, RECEIPT)
    output = new_output(output, (bundle, public_root))
    temporary = Path(tempfile.mkdtemp(prefix=".tagged-collect-", dir=output.parent))
    try:
        write_payloads(temporary, {**public, **payloads, RECEIPT: raw_receipt})
        bindings = runtime.provenance(repo, temporary, fixture, commit)
        expected = receipt_for(bindings, payloads, public, fixture, commit, run_id, attempt)
        require(runtime.same(receipt, expected), "BUNDLE_RECEIPT_JOIN")
        diagnostic_present = runtime.DIAGNOSTIC in public
        observed = False
        goals = False
        if diagnostic_present:
            diagnostic = publication.public_json(public[runtime.DIAGNOSTIC].decode("utf-8"))
            runtime.validate_public_diagnostic(diagnostic, temporary)
            require(diagnostic.get("targetFixtureId") == fixture
                    and diagnostic.get("compilerCommitSha") == commit, "DIAGNOSTIC_IDENTITY")
            observed = diagnostic["runtimeObservationVerified"]
            goals = diagnostic["nativeCarrierGoalsMatched"]
            if observed:
                assertions = publication.public_json(public["runtime/output-assertions.json"].decode("utf-8"))
                require(assertions["expectedRunId"].startswith(fixture + "-" + run_id + "-"),
                        "DIAGNOSTIC_RUN_NONCE")
        result = {
            "schemaVersion": "1", "policy": POLICY, "implementationSha": commit,
            "runId": run_id, "runAttempt": attempt, "fixtureId": fixture,
            "bundleReceipt": pin(RECEIPT, raw_receipt),
            "publicTextPublication": expected["publicTextPublication"],
            "originalBytesUnchanged": True, "originalsJoined": len(payloads),
            "diagnosticStatus": "VALIDATED_ORIGINAL" if diagnostic_present else "NOT_PRESENT",
            "runtimeObservationVerified": observed, "nativeCarrierGoalsMatched": goals,
            "semanticContractVerified": False, "appleExecutionAuthenticated": False,
            "formalAcceptanceGranted": False,
        }
        write_payloads(temporary, {COLLECTION: runtime.canonical(result)})
        temporary.rename(output)
    finally:
        if temporary.exists():
            shutil.rmtree(temporary)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("stage", "collect"))
    parser.add_argument("--repo", type=Path, required=True)
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--public-root", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--fixture", choices=(FIXTURE,), required=True)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--attempt", required=True)
    args = parser.parse_args()
    try:
        (stage if args.command == "stage" else collect)(
            args.repo, args.root, args.public_root, args.output,
            args.fixture, args.commit, args.run_id, args.attempt)
        if args.command == "stage" and os.environ.get("GITHUB_OUTPUT"):
            with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as stream:
                stream.write("ready=true\n")
        print(json.dumps({"status": "PUBLIC_ORIGINALS_STAGED" if args.command == "stage"
                          else "PUBLIC_ORIGINALS_COLLECTED",
                          "fixtureId": args.fixture, "runId": args.run_id,
                          "formalAcceptanceGranted": False}))
    except (OSError, ValueError, TypeError, KeyError, subprocess.SubprocessError):
        print("TAGGED_PUBLIC_ORIGINALS_BLOCKED", file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
