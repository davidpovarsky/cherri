#!/usr/bin/env python3
"""Observe the reviewed S01/F01 PoC with exact bytes and a fresh clipboard reset.

This module records file observations made by the Simulator orchestration. It
never executes Shortcuts itself and never substitutes expected bytes for output.
"""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import uuid

from runtime_poc_publication import (
    SOURCE_HASHES, SOURCE_RECEIPT, SIGNING_RECEIPT, DEVICE_RECEIPT,
    PublicationBlocked, read_contained, read_text_data, require, sha, verify_sources,
)

FIXTURE = "CherriS01F01CoveragePOC"
EXPECTED_PATH = "tests/runtime_poc/" + FIXTURE + ".expected.json"
EXPECTED_SHA256 = "8baa3279a770ca20e3daa6036f416360a308d6f40eb1dd0874a38afeb7d0b10c"
CONTEXT = "coverage-run-context.json"
RESET_REQUEST = "coverage-reset-request.txt"
RESET_OBSERVED = "coverage-reset-observed.txt"
RESET_PROOF = "coverage-reset-proof.json"
OBSERVED = "coverage-clipboard-result.txt"
RESULT = "coverage-result.json"
EXPECTED_COPY = "coverage-expected.json"
FIELDS = ("S01_OUTPUT", "F01_TRACE", "F01_RESULTS")


def write_new(root, name, data):
    path = Path(root) / name
    require(Path(root).is_dir() and not Path(root).is_symlink()
            and not path.exists() and not path.is_symlink(), "COVERAGE_OUTPUT_MUST_BE_NEW")
    with path.open("xb") as stream:
        stream.write(data)


def encoded(value):
    return (json.dumps(value, indent=2, ensure_ascii=False) + "\n").encode("utf-8")


def fields(data):
    try:
        lines = data.decode("utf-8").split("\n")
    except UnicodeDecodeError:
        return {}
    if len(lines) != 4 or lines[0] != "CHERRI_S01_F01_COVERAGE_OK":
        return {}
    result = {}
    for line, key in zip(lines[1:], FIELDS):
        if not line.startswith(key + "="):
            return {}
        result[key] = line[len(key) + 1:]
    return result


def contract(repo, root, commit, run_id, attempt):
    source_receipt = verify_sources(repo, root, commit, run_id, attempt)
    raw_receipt = read_contained(root, SOURCE_RECEIPT)
    require(read_text_data(raw_receipt, SOURCE_RECEIPT) == source_receipt,
            "COVERAGE_SOURCE_RECEIPT_MISMATCH")
    data = read_contained(repo, EXPECTED_PATH)
    require(sha(data) == EXPECTED_SHA256, "COVERAGE_EXPECTED_CONTRACT_CHANGED")
    committed = subprocess.check_output(["git", "-C", str(repo), "show", commit + ":" + EXPECTED_PATH])
    require(committed == data, "COVERAGE_EXPECTED_CONTRACT_NOT_COMMITTED")
    committed_helper = subprocess.check_output([
        "git", "-C", str(repo), "show",
        commit + ":tools/shortcuts-simulator-qa/runtime_poc_coverage.py",
    ])
    require(committed_helper == Path(__file__).read_bytes(), "COVERAGE_OBSERVER_NOT_COMMITTED")
    value = read_text_data(data, EXPECTED_PATH)
    expected = value.get("expected_clipboard", "").encode("utf-8")
    require(value.get("schema_version") == 1
            and value.get("source_path") == "tests/runtime_poc/" + FIXTURE + ".cherri"
            and value.get("source_sha256") == SOURCE_HASHES.get(FIXTURE)
            and value.get("requirements") == ["S01", "F01"]
            and value.get("expected_clipboard_sha256") == sha(expected)
            and set(value.get("expected_fields", {})) == set(FIELDS)
            and fields(expected) == value["expected_fields"],
            "COVERAGE_EXPECTED_CONTRACT_INVALID")
    return value, data, sha(raw_receipt)


def signed_identity(root, commit, run_id, attempt, allow_deleted=False):
    signing = read_text_data(read_contained(root, SIGNING_RECEIPT), SIGNING_RECEIPT)
    require(signing.get("implementationSha") == commit and signing.get("runId") == run_id
            and signing.get("runAttempt") == attempt, "COVERAGE_SIGNING_IDENTITY")
    rows = [row for row in signing.get("fixtures", []) if row.get("fixtureId") == FIXTURE]
    require(len(rows) == 1, "COVERAGE_SIGNING_RECEIPT_MISSING")
    row = rows[0]
    signed = read_contained(root, FIXTURE + ".shortcut")
    source = read_contained(root, FIXTURE + ".cherri")
    require(row.get("sourceSha256") == SOURCE_HASHES.get(FIXTURE)
            and sha(source) == SOURCE_HASHES.get(FIXTURE)
            and row.get("signedSha256") == sha(signed)
            and row.get("signedMagicVerified") is True
            and signed.startswith(b"AEA1") and len(signed) >= 16,
            "COVERAGE_SIGNED_ARTIFACT_MISMATCH")
    device = read_text_data(read_contained(root, DEVICE_RECEIPT), DEVICE_RECEIPT)
    require(device.get("implementationSha") == commit and device.get("runId") == run_id
            and device.get("runAttempt") == attempt and device.get("createdByRun") is True
            and device.get("cleanupStatus") == ("DELETED" if allow_deleted else "PENDING")
            and re.fullmatch(r"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}", device.get("udid", "")) is not None,
            "COVERAGE_OWNED_RUNNING_DEVICE_REQUIRED")
    return sha(signed), device["udid"]


def prepare(repo, root, commit, run_id, attempt):
    value, expected_raw, source_receipt_sha = contract(repo, root, commit, run_id, attempt)
    signed_sha, udid = signed_identity(root, commit, run_id, attempt)
    nonce = uuid.uuid4().hex
    reset = ("BEFORE_S01_F01_" + run_id + "_" + attempt + "_" + nonce).encode("ascii")
    context = {"schema_version": 1, "implementation_sha": commit, "run_id": run_id,
               "run_attempt": attempt, "fixture_id": FIXTURE,
               "source_sha256": SOURCE_HASHES[FIXTURE],
               "expected_contract_sha256": sha(expected_raw),
               "source_receipt_sha256": source_receipt_sha,
               "signed_artifact_sha256": signed_sha, "device_udid": udid,
               "nonce": nonce, "reset_request_sha256": sha(reset)}
    write_new(root, RESET_REQUEST, reset)
    write_new(root, EXPECTED_COPY, expected_raw)
    write_new(root, CONTEXT, encoded(context))
    return context


def current_context(repo, root, commit, run_id, attempt, allow_deleted=False):
    value, expected_raw, source_receipt_sha = contract(repo, root, commit, run_id, attempt)
    signed_sha, udid = signed_identity(root, commit, run_id, attempt, allow_deleted=allow_deleted)
    context_raw = read_contained(root, CONTEXT)
    context = read_text_data(context_raw, CONTEXT)
    require(context.get("implementation_sha") == commit and context.get("run_id") == run_id
            and context.get("run_attempt") == attempt and context.get("fixture_id") == FIXTURE
            and context.get("source_sha256") == SOURCE_HASHES[FIXTURE]
            and context.get("expected_contract_sha256") == sha(expected_raw)
            and context.get("source_receipt_sha256") == source_receipt_sha
            and context.get("signed_artifact_sha256") == signed_sha
            and context.get("device_udid") == udid
            and re.fullmatch(r"[0-9a-f]{32}", context.get("nonce", "")) is not None,
            "COVERAGE_CONTEXT_IDENTITY")
    reset = read_contained(root, RESET_REQUEST)
    expected_reset = ("BEFORE_S01_F01_" + run_id + "_" + attempt + "_" + context["nonce"]).encode("ascii")
    require(reset == expected_reset and context.get("reset_request_sha256") == sha(reset),
            "COVERAGE_RESET_REQUEST_MISMATCH")
    require(read_contained(root, EXPECTED_COPY) == expected_raw, "COVERAGE_EXPECTED_COPY_CHANGED")
    return context, context_raw, value, reset


def record_reset(repo, root, commit, run_id, attempt):
    context, raw, _, reset = current_context(repo, root, commit, run_id, attempt)
    observed = read_contained(root, RESET_OBSERVED)
    require(observed == reset, "COVERAGE_FRESH_RESET_NOT_OBSERVED")
    proof = {"implementation_sha": commit, "run_id": run_id, "run_attempt": attempt,
             "context_sha256": sha(raw), "reset_request_sha256": sha(reset),
             "reset_observed_sha256": sha(observed), "fresh_reset_observed": True}
    write_new(root, RESET_PROOF, encoded(proof))
    return proof


def observe(repo, root, commit, run_id, attempt, write_result=False, allow_deleted=False):
    context, context_raw, value, reset = current_context(repo, root, commit, run_id, attempt, allow_deleted=allow_deleted)
    reset_raw = read_contained(root, RESET_PROOF)
    proof = read_text_data(reset_raw, RESET_PROOF)
    observed_reset = read_contained(root, RESET_OBSERVED)
    require(observed_reset == reset and proof == {
        "implementation_sha": commit, "run_id": run_id, "run_attempt": attempt,
        "context_sha256": sha(context_raw), "reset_request_sha256": sha(reset),
        "reset_observed_sha256": sha(observed_reset), "fresh_reset_observed": True,
    }, "COVERAGE_RESET_PROOF_MISMATCH")
    observed = read_contained(root, OBSERVED)
    read_text_data(observed, OBSERVED)
    exact = observed == value["expected_clipboard"].encode("utf-8")
    result = {**context, "context_sha256": sha(context_raw),
              "reset_proof_sha256": sha(reset_raw),
              "reset_observed_sha256": sha(observed_reset), "fresh_reset_observed": True,
              "observed_clipboard_sha256": sha(observed),
              "expected_clipboard_sha256": value["expected_clipboard_sha256"],
              "observed_byte_count": len(observed), "expected_byte_count": len(value["expected_clipboard"].encode("utf-8")),
              "exact_output_match": exact, "observed_fields": fields(observed),
              "result": "passed" if exact else "failed", "exit_code": 0 if exact else 1,
              "scope": "SOURCE_BOUND_SIMULATOR_CLIPBOARD_OBSERVATION"}
    if write_result:
        write_new(root, RESULT, encoded(result))
    return result



def verify_result(repo, root, commit, run_id, attempt):
    """Replay stored observations after owned-device cleanup, preserving failures."""
    replayed = observe(repo, root, commit, run_id, attempt, allow_deleted=True)
    raw = read_contained(root, RESULT)
    recorded = read_text_data(raw, RESULT)
    require(recorded == replayed, "COVERAGE_RESULT_REPLAY_MISMATCH")
    return recorded


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("prepare", "record-reset", "matches", "record-result"))
    parser.add_argument("--repo", type=Path, required=True)
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--commit", default=os.environ.get("GITHUB_SHA", ""))
    parser.add_argument("--run-id", default=os.environ.get("GITHUB_RUN_ID", ""))
    parser.add_argument("--attempt", default=os.environ.get("GITHUB_RUN_ATTEMPT", "1"))
    args = parser.parse_args(argv)
    common = (args.repo, args.root, args.commit, args.run_id, args.attempt)
    try:
        if args.command == "prepare":
            prepare(*common)
        elif args.command == "record-reset":
            record_reset(*common)
        else:
            result = observe(*common, write_result=args.command == "record-result")
            if args.command == "record-result":
                print(json.dumps({"coverage_result": result["result"],
                                  "observed_byte_count": result["observed_byte_count"],
                                  "exact_output_match": result["exact_output_match"]}))
            return result["exit_code"]
    except (PublicationBlocked, OSError, ValueError, TypeError, KeyError, subprocess.SubprocessError):
        print("POC_COVERAGE_OBSERVATION_BLOCKED", file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

