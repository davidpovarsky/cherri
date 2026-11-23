#!/usr/bin/env python3
"""Retain exact originals for two reviewed public nested-List fixtures only.

The ordinary QA publisher remains text-only. This separate bundle supplies
missing bytes for the existing native validator; it grants no runtime,
nested-List semantic, signing-cryptography or formal acceptance by itself.
"""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

import nested_list_probe as probe
import nested_list_runtime as runtime
import prepare_public_artifacts as publication
from runtime_poc_publication import identity, read_contained, read_text_data

FIXTURES = {
    "qa-nested-list-control-v2": (
        "5452ff03c5c09f8fa7c9c511e988f398059ea5a901f00da63314cfeef7ede121",
        "c825bfff307008d032c08143de019a6125f825b138ed213c03ecae2e9c8f9feb"),
    "qa-nested-list-candidate-v2": (
        "b13ca17d1eb425e0b54854c922df46fc50cbcf1aff491f988191c774d2760299",
        "5aa826dce959a4667a3f498a5b6c9fae4c91a3f71eeb6209c3772e405ff5c9a8"),
}
RECEIPT = "nested-list-public-originals.json"
COLLECTION = "nested-list-collection.json"
POLICY = "EXACT_TWO_REVIEWED_PUBLIC_NESTED_LIST_FIXTURES"
MAX_SOURCE = 64 * 1024
MAX_PUBLIC_TOTAL = 64 * 1024 * 1024


def require(ok, code):
    if not ok:
        raise publication.PublicationBlocked("NESTED_PUBLIC_" + code)


def pin(name, raw):
    return {"path": name, "bytes": len(raw), "sha256": probe.sha(raw)}


def git_bytes(repo, *args):
    # Local metadata only; no prompt, lazy fetch, replacement object or network.
    environment = {key: os.environ[key] for key in ("PATH", "HOME", "USER", "LOGNAME")
                   if key in os.environ}
    environment.update(GIT_NO_REPLACE_OBJECTS="1", GIT_NO_LAZY_FETCH="1",
                       GIT_OPTIONAL_LOCKS="0", GIT_TERMINAL_PROMPT="0",
                       LANG="C.UTF-8", LC_ALL="C.UTF-8")
    process = subprocess.run(
        ["git", "--no-replace-objects", "-C", str(repo), *args],
        stdin=subprocess.DEVNULL, capture_output=True, timeout=15, env=environment)
    require(process.returncode == 0 and len(process.stdout) <= 1024 * 1024
            and len(process.stderr) <= 1024 * 1024, "CHECKOUT_METADATA")
    return process.stdout


def verify_context(repo, fixture, commit, run_id, attempt):
    identity(commit, run_id, attempt)
    require(fixture in FIXTURES, "UNREVIEWED_FIXTURE")
    source_hash, contract_hash = FIXTURES[fixture]
    require(probe.SOURCE_HASHES.get(fixture) == source_hash
            and probe.CONTRACT_HASHES.get(fixture) == contract_hash,
            "FIXED_AUTHORITY_CHANGED")
    require(git_bytes(repo, "rev-parse", "HEAD").decode().strip() == commit,
            "CHECKOUT_SOURCE")
    for suffix, expected, limit in ((".cherri", source_hash, MAX_SOURCE),
                                    (".expected.json", contract_hash, 8192)):
        name = "tools/shortcuts-simulator-qa/fixtures/" + fixture + suffix
        raw = read_contained(repo, name)
        require(0 < len(raw) <= limit and probe.sha(raw) == expected,
                "FIXED_SOURCE_BYTES")
        require(git_bytes(repo, "show", commit + ":" + name) == raw,
                "COMMITTED_SOURCE_BYTES")
        read_text_data(raw, name)


def original_paths(fixture):
    require(fixture in FIXTURES, "UNREVIEWED_FIXTURE")
    return ("builds/" + fixture + ".cherri",
            "builds/" + fixture + ".shortcut",
            "builds/signed/" + fixture + ".shortcut")


def raw_payloads(root, fixture):
    names = original_paths(fixture)
    result = {}
    for name, limit in zip(names, (MAX_SOURCE, runtime.MAX_PLIST, 10 * 1024 * 1024)):
        raw = read_contained(root, name)
        require(0 < len(raw) <= limit, "ORIGINAL_SIZE")
        if name == names[2]:
            require(raw.startswith(b"AEA1") and len(raw) >= 16, "SIGNED_HEADER")
        else:
            # Reuse the PoC text/secret boundary. The existing native validator
            # owns all source, plist, graph and signing interpretation below.
            read_text_data(raw, name)
        result[name] = raw
    return result


def public_payloads(repo, root, commit, run_id):
    raw_manifest = read_contained(root, "publication-manifest.json")
    manifest = read_text_data(raw_manifest, "publication-manifest.json")
    require(isinstance(manifest, dict) and manifest.get("schemaVersion") == "1.5"
            and manifest.get("publicationGatePassed") is True
            and manifest.get("syntheticOnly") is True
            and manifest.get("policy") == "EXPLICIT_SYNTHETIC_TEXT_PATHS_ONLY"
            and manifest.get("rasterPublicationAllowed") is False,
            "PUBLIC_TEXT_GATE")
    rows = manifest.get("published")
    require(isinstance(rows, list) and 0 < len(rows) <= len(publication.ALLOWED),
            "PUBLIC_INVENTORY")
    payloads = {"publication-manifest.json": raw_manifest}
    for row in rows:
        require(isinstance(row, dict) and set(row) == {"path", "bytes", "sha256"},
                "PUBLIC_PIN_SHAPE")
        name = row["path"]
        require(isinstance(name, str) and name in publication.ALLOWED
                and name not in payloads and type(row["bytes"]) is int,
                "PUBLIC_INVENTORY")
        raw = read_contained(root, name)
        require(probe._same(row, pin(name, raw)), "PUBLIC_PIN_BYTES")
        read_text_data(raw, name)
        payloads[name] = raw
    require(sum(map(len, payloads.values())) <= MAX_PUBLIC_TOTAL, "PUBLIC_TOTAL_SIZE")
    entries = list(Path(root).rglob("*"))
    require(not any(path.is_symlink() for path in entries)
            and {path.relative_to(root).as_posix() for path in entries if path.is_file()}
            == set(payloads), "PUBLIC_UNDECLARED_FILE")
    require(publication.scan_public_privacy(root)["passed"] is True, "PUBLIC_PRIVACY")
    publication.verify_synthetic_manifest(
        Path(root), publication.trusted_fixture_sources(
            Path(repo) / "tools/shortcuts-simulator-qa/fixtures"))
    require("compiler-manifest.json" in payloads
            and "runtime/signing-provenance.json" in payloads
            and "summary.json" in payloads, "PUBLIC_CONTEXT_MISSING")
    summary = read_text_data(payloads["summary.json"], "summary.json")
    require(isinstance(summary, dict) and summary.get("runId") == run_id
            and summary.get("compilerCommitSha") == commit, "PUBLIC_RUN_SOURCE")
    return payloads


def receipt_for(bindings, payloads, public, fixture, commit, run_id, attempt):
    return {
        "schemaVersion": "1", "policy": POLICY, "syntheticOnly": True,
        "implementationSha": commit, "runId": run_id, "runAttempt": attempt,
        "fixtureId": fixture, "sourceSha256": FIXTURES[fixture][0],
        "contractSha256": FIXTURES[fixture][1],
        "artifactName": "cherri-nested-list-public-originals-" + run_id + "-" + attempt,
        "publicTextPublication": pin("publication-manifest.json",
                                     public["publication-manifest.json"]),
        "inputBindings": bindings,
        "originals": [pin(name, payloads[name]) for name in sorted(payloads)],
        "originalBytesUnchanged": True, "rasterPublicationAllowed": False,
        "runtimeAcceptanceGranted": False, "nativeListTypeProved": False,
        "formalAcceptanceGranted": False,
        "scope": "Three exact public synthetic originals for independent native diagnostic recomputation only.",
    }


def new_output(output, inputs):
    output = Path(output).absolute()
    require(not output.exists() and not output.is_symlink(), "OUTPUT_EXISTS")
    resolved = output.resolve()
    for root in inputs:
        original = Path(root).resolve()
        require(resolved != original and original not in resolved.parents,
                "OUTPUT_INSIDE_INPUT")
    output.parent.mkdir(parents=True, exist_ok=True)
    return output


def write_payloads(root, payloads):
    for name, raw in payloads.items():
        path = root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        with path.open("xb") as stream:
            stream.write(raw)
        require(path.read_bytes() == raw, "OUTPUT_BYTES")


def stage(repo, root, public_root, output, fixture, commit, run_id, attempt):
    verify_context(repo, fixture, commit, run_id, attempt)
    public = public_payloads(repo, public_root, commit, run_id)
    payloads = raw_payloads(root, fixture)
    # This is the existing authority, including its complete native graph and
    # causal checks. Hashes alone never authorize these three raw files.
    bindings = runtime.provenance(repo, root, fixture, commit)
    require(set(bindings) == {"compilerManifest", "compiledSource", "unsignedShortcut",
                              "signedShortcut", "signingProvenance"}, "PROVENANCE_KEYS")
    for role in ("compilerManifest", "signingProvenance"):
        name = bindings[role]["path"]
        require(probe._same(bindings[role], pin(name, public[name]))
                and read_contained(root, name) == public[name], "PUBLIC_ORIGINAL_JOIN")
    for role in ("compiledSource", "unsignedShortcut", "signedShortcut"):
        name = bindings[role]["path"]
        require(probe._same(bindings[role], pin(name, payloads[name])), "RAW_ORIGINAL_JOIN")
    receipt = receipt_for(bindings, payloads, public, fixture, commit, run_id, attempt)
    output = new_output(output, (root, public_root))
    temporary = Path(tempfile.mkdtemp(prefix=".nested-public-", dir=output.parent))
    try:
        write_payloads(temporary, {**payloads, RECEIPT: runtime.canonical(receipt)})
        temporary.rename(output)
    finally:
        if temporary.exists():
            shutil.rmtree(temporary)
    return receipt


def collect(repo, bundle, public_root, output, fixture, commit, run_id, attempt):
    """Join authenticated transport originals without changing any input bytes.

    Callers must first authenticate both artifact archives to the selected
    workflow run/attempt/checkout. This collector cannot establish that trust
    from a self-authored manifest and does not grant formal requirement credit.
    """
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
    temporary = Path(tempfile.mkdtemp(prefix=".nested-collect-", dir=output.parent))
    try:
        write_payloads(temporary, {**public, **payloads, RECEIPT: raw_receipt})
        bindings = runtime.provenance(repo, temporary, fixture, commit)
        expected = receipt_for(bindings, payloads, public, fixture, commit, run_id, attempt)
        require(probe._same(receipt, expected), "BUNDLE_RECEIPT_JOIN")
        diagnostic_path = "runtime/nested-list-observation.json"
        diagnostic_present = diagnostic_path in public
        observed = False
        if diagnostic_present:
            diagnostic = publication.public_json(public[diagnostic_path].decode("utf-8"))
            runtime.validate_public_diagnostic(diagnostic, temporary)
            require(diagnostic.get("targetFixtureId") == fixture
                    and diagnostic.get("compilerCommitSha") == commit, "DIAGNOSTIC_IDENTITY")
            observed = diagnostic["runtimeObservationVerified"]
            if observed:
                assertions = publication.public_json(
                    public["runtime/output-assertions.json"].decode("utf-8"))
                require(assertions["expectedRunId"].startswith(fixture + "-" + run_id + "-"),
                        "DIAGNOSTIC_RUN_NONCE")
        result = {
            "schemaVersion": "1", "policy": POLICY, "implementationSha": commit,
            "runId": run_id, "runAttempt": attempt, "fixtureId": fixture,
            "bundleReceipt": pin(RECEIPT, raw_receipt),
            "publicTextPublication": expected["publicTextPublication"],
            "originalBytesUnchanged": True, "originalsJoined": len(payloads),
            "diagnosticStatus": "VALIDATED_ORIGINAL" if diagnostic_present else "NOT_PRESENT",
            "runtimeObservationVerified": observed,
            "semanticContractVerified": False, "nativeListTypeProved": False,
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
    parser.add_argument("--root", type=Path, required=True,
                        help="Original QA root for stage; retained three-file bundle for collect.")
    parser.add_argument("--public-root", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--fixture", choices=tuple(FIXTURES), required=True)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--attempt", required=True)
    args = parser.parse_args()
    try:
        result = (stage if args.command == "stage" else collect)(
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
        # No source, credentials, signing URLs or arbitrary platform errors.
        print("NESTED_PUBLIC_ORIGINALS_BLOCKED", file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
