#!/usr/bin/env python3
"""Publish only source-bound formal PoC evidence; quarantine unrelated raw data."""
import argparse
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import tempfile

# Use the same reviewed secret/structured-data handling as strict Simulator QA.
from prepare_public_artifacts import (CANARY, MAGIC, PublicationBlocked, public_json,
                                      safe_path, sha, structured_privacy_failure)
from runtime_poc_device import RECEIPT as DEVICE_RECEIPT, identity

SOURCE_HASHES = {
    "CherriS01F01CoveragePOC": "4a13f90af2bdc326e24f9dbf785f76a1ae41043e176b5a3188ab794694a19aa3",
    "CherriRuntimePOC": "cf5cb580a75833ffd022f4c4b4e8fa0e0e52f101c7f584d2726833000adf050b",
    "CherriDictionaryRuntimePOC": "fc7ce1f3daa3fffa33ab1b1af5e8c86c68fee1719f2f6121dbce6a6611be741c",
    "CherriMenuRuntimePOC": "a021541d5d6a9b87a3c885f419d1e2f286ab3eb7b300e985372095c95ad646c0",
    "CherriLegacyIfPOC": "4364d76ac8f4fdd1bbe049cf0f58d182f056772f79906f27564d66857b84cce7",
}
CI_PREFIX = "artifacts/backend-recovery/ci-artifacts/ios27-runtime-poc/"
FORMAL_TEXT = {
    "coverage-clipboard-result.txt",
    "clipboard-result.txt", "dict-clipboard-result.txt", "legacy-if-clipboard-result.txt",
    "menu-alpha-clipboard-result.txt", "menu-beta-clipboard-result.txt",
    "menu-alpha-selection-proof.json", "menu-beta-selection-proof.json",
    "ui_helper_menu_select_alpha.log", "ui_helper_menu_select_beta.log",
}
FORMAL_FILES = FORMAL_TEXT | {name + suffix for name in SOURCE_HASHES for suffix in (".cherri", ".shortcut")}
SOURCE_RECEIPT = "runtime-poc-source-provenance.json"
SIGNING_RECEIPT = "runtime-poc-signing-provenance.json"
COVERAGE_SUPPORT = {
    "coverage-run-context.json", "coverage-reset-request.txt", "coverage-reset-observed.txt",
    "coverage-reset-proof.json", "coverage-result.json", "coverage-expected.json",
}
SUPPORT = {SOURCE_RECEIPT, SIGNING_RECEIPT, DEVICE_RECEIPT, "runtime-diagnostics.json"} | COVERAGE_SUPPORT
MAX_FILE_BYTES = 10 * 1024 * 1024


def require(ok, code):
    if not ok:
        raise PublicationBlocked(code)


def read_contained(root, name):
    root = Path(root)
    relative = PurePosixPath(name)
    require(not relative.is_absolute() and ".." not in relative.parts and str(relative) == name
            and "\\" not in name, "POC_ARTIFACT_PATH_INVALID")
    require(root.is_dir() and not root.is_symlink(), "POC_RAW_ROOT_INVALID")
    path = root.joinpath(*relative.parts)
    require(all(not root.joinpath(*relative.parts[:i]).is_symlink()
                for i in range(1, len(relative.parts) + 1)), "POC_ARTIFACT_SYMLINK")
    require(path.is_file() and root.resolve() in path.resolve().parents, "POC_ARTIFACT_MISSING")
    require(path.stat().st_size <= MAX_FILE_BYTES, "POC_ARTIFACT_SIZE_LIMIT")
    return path.read_bytes()


def read_text_data(data, name):
    require(not data.startswith(MAGIC), "POC_BINARY_IN_TEXT")
    try:
        text = data.decode("utf-8")
        require("\x00" not in text and CANARY.search(text) is None, "POC_PRIVATE_TEXT")
        if name.endswith(".json"):
            parsed = public_json(text)
            require(structured_privacy_failure(parsed) is None, "POC_PRIVATE_STRUCTURED_DATA")
            return parsed
    except (UnicodeError, ValueError) as exc:
        raise PublicationBlocked("POC_INVALID_PUBLIC_TEXT") from exc
    return text


def verify_sources(repo, root, commit, run_id, attempt):
    identity(commit, run_id, attempt)
    repo, root = Path(repo), Path(root)
    require(root.is_dir() and not root.is_symlink(), "POC_RAW_ROOT_INVALID")
    head = subprocess.check_output(["git", "-C", str(repo), "rev-parse", "HEAD"], text=True).strip()
    require(head == commit, "POC_CHECKOUT_SHA_MISMATCH")
    rows = []
    for name, expected in SOURCE_HASHES.items():
        relative = "tests/runtime_poc/" + name + ".cherri"
        raw = read_contained(repo, relative)
        require(sha(raw) == expected, "POC_UNREVIEWED_SOURCE")
        read_text_data(raw, relative)
        # Hash the committed source too: a matching working file cannot hide HEAD drift.
        committed = subprocess.check_output(["git", "-C", str(repo), "show", commit + ":" + relative])
        require(committed == raw, "POC_COMMITTED_SOURCE_MISMATCH")
        rows.append({"fixtureId": name, "sourcePath": relative, "sourceSha256": expected})
    return {"schemaVersion": "1", "implementationSha": commit, "runId": run_id,
            "runAttempt": attempt, "syntheticSourcesVerified": True, "sources": rows}


def write_new_or_matching(path, value):
    data = (json.dumps(value, indent=2) + "\n").encode("utf-8")
    require(not path.is_symlink(), "POC_RECEIPT_SYMLINK")
    if path.exists():
        require(path.read_bytes() == data, "POC_RECEIPT_ALREADY_EXISTS")
    else:
        path.write_bytes(data)


def check_sources(repo, root, commit, run_id, attempt):
    receipt = verify_sources(repo, root, commit, run_id, attempt)
    write_new_or_matching(Path(root) / SOURCE_RECEIPT, receipt)
    return receipt


def record_signing(repo, root, commit, run_id, attempt, source, unsigned, signed, method, cli_manifest=None):
    verify_sources(repo, root, commit, run_id, attempt)
    root = Path(root)
    name = Path(source).stem
    require(name in SOURCE_HASHES and Path(source).as_posix() == "tests/runtime_poc/" + name + ".cherri",
            "POC_SIGNING_SOURCE_INVALID")
    require(method in ("native", "hubsign"), "POC_SIGNING_METHOD_INVALID")
    def artifact_bytes(value):
        path = Path(value).absolute()
        try:
            relative = path.relative_to(root.absolute()).as_posix()
        except ValueError as exc:
            raise PublicationBlocked("POC_SIGNING_PATH_OUTSIDE_RAW") from exc
        return read_contained(root, relative)
    unsigned_bytes, signed_bytes = artifact_bytes(unsigned), artifact_bytes(signed)
    require(signed_bytes.startswith(b"AEA1") and len(signed_bytes) >= 16, "POC_SIGNED_MAGIC_INVALID")
    cli_provenance = {}
    if method == "hubsign":
        require(cli_manifest is not None, "POC_EXACT_FILE_SIGNING_RECEIPT_MISSING")
        cli_bytes = artifact_bytes(cli_manifest)
        cli = read_text_data(cli_bytes, "signing-provenance.json")
        require(isinstance(cli, dict)
                and cli.get("compilerCommitSha") == commit
                and cli.get("sourceSha256") == SOURCE_HASHES[name]
                and cli.get("unsignedPlistSha256") == sha(unsigned_bytes)
                and cli.get("signedArtifactSha256") == sha(signed_bytes)
                and cli.get("unsignedByteSize") == len(unsigned_bytes)
                and cli.get("signedByteSize") == len(signed_bytes)
                and cli.get("headerMagic") == "AEA1"
                and cli.get("verificationTier") == "AEA1_HEADER_PASS",
                "POC_EXACT_FILE_SIGNING_RECEIPT_MISMATCH")
        cli_provenance = {"exactFileSigningReceiptSha256": sha(cli_bytes),
                          "signingVerificationTier": cli["verificationTier"]}

    path = root / SIGNING_RECEIPT
    require(not path.is_symlink(), "POC_SIGNING_RECEIPT_SYMLINK")
    base = {"schemaVersion": "1", "implementationSha": commit, "runId": run_id,
            "runAttempt": attempt, "fixtures": []}
    if path.exists():
        base = read_text_data(read_contained(root, SIGNING_RECEIPT), SIGNING_RECEIPT)
        require(isinstance(base, dict) and all(base.get(k) == v for k, v in (("implementationSha", commit), ("runId", run_id),
                                                 ("runAttempt", attempt))), "POC_SIGNING_IDENTITY_MISMATCH")
    require(isinstance(base.get("fixtures"), list) and all(isinstance(r, dict) for r in base["fixtures"]), "POC_SIGNING_RECEIPT_INVALID")
    require(not any(r.get("fixtureId") == name for r in base["fixtures"]), "POC_DUPLICATE_SIGNING_RECEIPT")
    base["fixtures"].append({"fixtureId": name, "sourceSha256": SOURCE_HASHES[name],
                             "unsignedSha256": sha(unsigned_bytes), "signedSha256": sha(signed_bytes),
                             "signedArtifact": name + ".shortcut", "signingMethod": method,
                             "signedMagicVerified": True, **cli_provenance})
    path.write_bytes((json.dumps(base, indent=2) + "\n").encode("utf-8"))


def stage(repo, root, output, commit, run_id, attempt):
    identity(commit, run_id, attempt)
    root, output = Path(root), Path(output).absolute()
    require(not output.exists() and not output.is_symlink() and root.resolve() != output.resolve()
            and root.resolve() not in output.resolve().parents, "POC_OUTPUT_MUST_BE_NEW_OUTSIDE_RAW")
    source_receipt = verify_sources(repo, root, commit, run_id, attempt)
    require(read_text_data(read_contained(root, SOURCE_RECEIPT), SOURCE_RECEIPT) == source_receipt,
            "POC_SOURCE_RECEIPT_MISMATCH")
    device = read_text_data(read_contained(root, DEVICE_RECEIPT), DEVICE_RECEIPT)
    require(isinstance(device, dict) and device.get("createdByRun") is True and device.get("implementationSha") == commit
            and device.get("runId") == run_id and device.get("runAttempt") == attempt
            and device.get("cleanupStatus") == "DELETED", "POC_OWNED_SIMULATOR_PROOF_MISSING")
    signing = read_text_data(read_contained(root, SIGNING_RECEIPT), SIGNING_RECEIPT)
    require(isinstance(signing, dict) and signing.get("implementationSha") == commit and signing.get("runId") == run_id
            and signing.get("runAttempt") == attempt, "POC_SIGNING_IDENTITY_MISMATCH")
    signed_rows = signing.get("fixtures", [])
    require(isinstance(signed_rows, list) and all(isinstance(r, dict) for r in signed_rows)
            and len(signed_rows) == len(SOURCE_HASHES)
            and {r.get("fixtureId") for r in signed_rows} == set(SOURCE_HASHES),
            "POC_SIGNING_PROVENANCE_INCOMPLETE")
    signed_by_name = {r["signedArtifact"]: r for r in signed_rows}
    raw_evidence = read_contained(root, "runtime-evidence.json")
    records = read_text_data(raw_evidence, "runtime-evidence.json")
    raw_diagnostics = read_contained(root, "runtime-diagnostics.json")
    read_text_data(raw_diagnostics, "runtime-diagnostics.json")
    source_receipt_bytes = read_contained(root, SOURCE_RECEIPT)
    require(isinstance(records, list) and records and all(isinstance(r, dict) for r in records), "POC_FORMAL_EVIDENCE_MISSING")
    refs = {}
    for record in records:
        assertions = record.get("assertions", {})
        require(isinstance(assertions, dict)
                and assertions.get("diagnostics_sha256") == sha(raw_diagnostics)
                and assertions.get("source_receipt_sha256") == sha(source_receipt_bytes)
                and assertions.get("run_id") == run_id and assertions.get("run_attempt") == attempt,
                "POC_FORMAL_DIAGNOSTIC_BINDING_MISMATCH")
        if record.get("test_id") in ("ios27-runtime-poc:coverage-s01", "ios27-runtime-poc:coverage-f01"):
            require(assertions.get("coverage_result_sha256") == sha(read_contained(root, "coverage-result.json")),
                    "POC_FORMAL_COVERAGE_BINDING_MISMATCH")
        require(record.get("implementation_sha") == commit and record.get("tier") == "ios-runtime"
                and str(record.get("test_id", "")).startswith("ios27-runtime-poc:"),
                "POC_FORMAL_EVIDENCE_IDENTITY")
        require(isinstance(record.get("artifacts"), list) and record["artifacts"], "POC_FORMAL_REFS_MISSING")
        for artifact in record["artifacts"]:
            require(isinstance(artifact, dict), "POC_FORMAL_REF_INVALID")
            path = artifact.get("path", "")
            require(isinstance(path, str) and path.startswith(CI_PREFIX), "POC_FORMAL_REF_OUTSIDE_PREFIX")
            name = path[len(CI_PREFIX):]
            require(name in FORMAL_FILES and artifact.get("run_id") == run_id, "POC_UNREVIEWED_FORMAL_REF")
            data = read_contained(root, name)
            require(sha(data) == artifact.get("sha256"), "POC_FORMAL_REF_DIGEST_MISMATCH")
            if name.endswith(".cherri"):
                require(sha(data) == SOURCE_HASHES[Path(name).stem], "POC_UNREVIEWED_SOURCE_COPY")
                read_text_data(data, name)
            elif name.endswith(".shortcut"):
                row = signed_by_name.get(name, {})
                require(row.get("fixtureId") == Path(name).stem
                        and row.get("sourceSha256") == SOURCE_HASHES[Path(name).stem]
                        and row.get("signedSha256") == sha(data) and row.get("signedMagicVerified") is True
                        and re.fullmatch(r"[0-9a-f]{64}", row.get("unsignedSha256", "")) is not None
                        and data.startswith(b"AEA1") and len(data) >= 16
                        and row.get("signingMethod") in ("native", "hubsign"), "POC_SIGNED_PROVENANCE_MISMATCH")
                if row["signingMethod"] == "hubsign":
                    require(re.fullmatch(r"[0-9a-f]{64}", row.get("exactFileSigningReceiptSha256", "")) is not None
                            and row.get("signingVerificationTier") == "AEA1_HEADER_PASS",
                            "POC_EXACT_FILE_SIGNING_RECEIPT_MISSING")
            else:
                read_text_data(data, name)
            require(name not in refs or refs[name] == data, "POC_CONFLICTING_FORMAL_REF")
            refs[name] = data
    require({n + suffix for n in SOURCE_HASHES for suffix in (".cherri", ".shortcut")} <= set(refs),
            "POC_SOURCE_OR_SIGNED_REF_MISSING")
    # Every referenced byte remains unchanged. No evidence/result/assertion rewriting.
    approved = {**refs, "runtime-evidence.json": raw_evidence}
    for name in SUPPORT:
        approved[name] = read_contained(root, name)
        read_text_data(approved[name], name)
    quarantine = []
    for path in sorted(root.rglob("*")):
        rel = path.relative_to(root)
        if path.is_symlink() or path.is_file():
            if rel.as_posix() not in approved:
                quarantine.append({"path": safe_path(rel), "reason": "NOT_FORMAL_SOURCE_BOUND_ARTIFACT"})
    manifest = {"schemaVersion": "1", "publicationGatePassed": True, "syntheticOnly": True,
                "implementationSha": commit, "runId": run_id, "runAttempt": attempt,
                "rasterPublicationAllowed": False, "systemLogPublicationAllowed": False,
                "formalEvidenceBytesUnchanged": True,
                "policy": "EXACT_FORMAL_POC_REFS_WITH_REVIEWED_SOURCE_AND_SIGNING_PROVENANCE",
                "published": [{"path": n, "sha256": sha(d), "bytes": len(d)}
                              for n, d in sorted(approved.items())],
                "quarantined": quarantine}
    output.parent.mkdir(parents=True, exist_ok=True)
    temporary = Path(tempfile.mkdtemp(prefix=".cherri-poc-public-", dir=output.parent))
    try:
        for name, data in approved.items():
            (temporary / name).write_bytes(data)
        (temporary / "publication-manifest.json").write_bytes((json.dumps(manifest, indent=2) + "\n").encode())
        temporary.rename(output)
    finally:
        if temporary.exists():
            shutil.rmtree(temporary)
    return manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("check-sources", "record-signing", "stage"))
    parser.add_argument("--repo", type=Path, required=True)
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--commit", default=os.environ.get("GITHUB_SHA", ""))
    parser.add_argument("--run-id", default=os.environ.get("GITHUB_RUN_ID", ""))
    parser.add_argument("--attempt", default=os.environ.get("GITHUB_RUN_ATTEMPT", "1"))
    parser.add_argument("--source")
    parser.add_argument("--unsigned")
    parser.add_argument("--signed")
    parser.add_argument("--method", choices=("native", "hubsign"))
    parser.add_argument("--cli-manifest")
    args = parser.parse_args()
    try:
        common = (args.repo, args.root, args.commit, args.run_id, args.attempt)
        if args.command == "check-sources":
            check_sources(*common)
        elif args.command == "record-signing":
            record_signing(*common, args.source, args.unsigned, args.signed, args.method, args.cli_manifest)
        else:
            require(args.output is not None, "POC_PUBLIC_OUTPUT_REQUIRED")
            report = stage(args.repo, args.root, args.output, args.commit, args.run_id, args.attempt)
            print(json.dumps({"publicationGatePassed": True, "publishedFiles": len(report["published"]),
                              "rasterPublicationAllowed": False, "systemLogPublicationAllowed": False}))
    except (PublicationBlocked, OSError, ValueError, TypeError, KeyError, subprocess.SubprocessError):
        print("POC_PUBLICATION_BLOCKED", file=__import__("sys").stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
