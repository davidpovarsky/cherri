"""Public Optional diagnostic pins and the existing independent Go graph gate.

The Go oracle validates the actual unsigned CLI artifact before signing. This
module binds its receipt to those same bytes; it grants no native or formal credit.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess

FIXTURE_ID = "qa-optional-forwarding-diagnostic-v2"
SOURCE_PATH = "tools/shortcuts-simulator-qa/fixtures/" + FIXTURE_ID + ".cherri"
SOURCE_SHA256 = "e6f195aadcd2d46f25c6fe1de5165798c18a40f4c1942785b9e39171b1cc2139"
CONTRACT_SHA256 = "da1335a22389cbe97e72a706192cb6b93ece8eff380080fa0541dcb5cbc50cef"
PREFIX = "CHERRI_OPTIONAL_FORWARDING_DIAGNOSTIC_V1:"
EXPECTED_FIELDS = (
    ("NONE", "NONE"), ("EMPTY_TEXT", "TEXT[]"), ("FALSE", "BOOL[false]"),
    ("ZERO", "NUMBER[0]"), ("EMPTY_LIST", "LIST[0]"), ("EMPTY_MAP", "MAP[0]"),
    ("LIST", "LIST[2|A|B]"), ("MAP", "MAP[1|VALUE|Dictionary]"),
    ("SNAPSHOT", "NONE"), ("WRITTEN", "TEXT[]"), ("DEFAULT", "TEXT[DEFAULT]"),
)
EXPECTED_OUTPUT = PREFIX + ":".join(key + "=" + value for key, value in EXPECTED_FIELDS)
GRAPH_MARKER = "CHERRI_OPTIONAL_UNSIGNED_GRAPH_V1="
MAX_UNSIGNED_BYTES = 4 * 1024 * 1024


def _unique(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate Optional diagnostic receipt key")
        result[key] = value
    return result


def _contained_file(root, path):
    root, path = Path(root).absolute(), Path(path).absolute()
    try:
        path.relative_to(root)
        path.resolve(strict=True).relative_to(root.resolve(strict=True))
    except (ValueError, OSError) as exc:
        raise ValueError("Optional diagnostic file is outside its owned root") from exc
    current = path
    while True:
        if current.is_symlink() or (hasattr(current, "is_junction") and current.is_junction()):
            raise ValueError("Optional diagnostic file contains a linked path")
        if current == root:
            break
        current = current.parent
    if not path.is_file():
        raise ValueError("Optional diagnostic input is not a regular file")
    return path


def verify_source(repo, fixture_id, source_bytes):
    if fixture_id != FIXTURE_ID:
        return None
    source = _contained_file(repo, Path(repo) / SOURCE_PATH)
    contract_path = _contained_file(repo, source.with_suffix(".expected.json"))
    if hashlib.sha256(source_bytes).hexdigest() != SOURCE_SHA256 or source.read_bytes() != source_bytes:
        raise ValueError("Optional reviewed public source changed")
    data = contract_path.read_bytes()
    if hashlib.sha256(data).hexdigest() != CONTRACT_SHA256:
        raise ValueError("Optional independently reviewed output contract changed")
    contract = json.loads(data)
    if (contract["schema_version"] != 1 or contract["fixture_id"] != FIXTURE_ID
            or contract["source_path"] != SOURCE_PATH or contract["source_sha256"] != SOURCE_SHA256
            or contract["diagnostic_prefix"] != PREFIX
            or contract["field_order"] != [key for key, _ in EXPECTED_FIELDS]
            or contract["expected_fields"] != dict(EXPECTED_FIELDS)
            or contract["expected_summary"] != EXPECTED_OUTPUT
            or contract["expected_summary_sha256"] != hashlib.sha256(EXPECTED_OUTPUT.encode()).hexdigest()
            or contract["trace_run_id"] != FIXTURE_ID or contract["trace_label"] != "optional-forwarding"
            or contract["runtime_status"] != "NOT_RUN" or contract["formal_requirements_proved"] != []):
        raise ValueError("Optional source, output, trace or scope identity changed")
    return {"status": "PASS_REVIEWED_PUBLIC_SOURCE_ONLY", "sourceSha256": SOURCE_SHA256,
            "expectedContractSha256": CONTRACT_SHA256, "observedFields": len(EXPECTED_FIELDS),
            "appleRuntime": "NOT_RUN", "formalAcceptanceGranted": False}


def verify_prepared_unsigned(repo, artifact_dir, fixture_id, artifact_path, expected_sha256):
    if fixture_id != FIXTURE_ID:
        return None
    source = _contained_file(repo, Path(repo) / SOURCE_PATH)
    verify_source(repo, fixture_id, source.read_bytes())
    builds = Path(artifact_dir) / "builds"
    path = _contained_file(builds, artifact_path)
    if path.name != FIXTURE_ID + ".shortcut" or not 0 < path.stat().st_size <= MAX_UNSIGNED_BYTES:
        raise ValueError("Optional unsigned artifact name or size is invalid")
    if (not isinstance(expected_sha256, str) or not re.fullmatch(r"[0-9a-f]{64}", expected_sha256)
            or hashlib.sha256(path.read_bytes()).hexdigest() != expected_sha256):
        raise ValueError("Optional unsigned artifact digest changed before its graph gate")
    environment = os.environ.copy()
    environment["CHERRI_OPTIONAL_DIAGNOSTIC_UNSIGNED"] = str(path)
    environment["CHERRI_OPTIONAL_DIAGNOSTIC_SHA256"] = expected_sha256
    command = ["go", "test", "-count=1", "-timeout=3m", "-run", "^TestP15OptionalNativeDiagnostic", "-v", "."]
    try:
        process = subprocess.run(command, cwd=repo, env=environment, capture_output=True, timeout=210)
    except subprocess.TimeoutExpired as exc:
        (builds / (FIXTURE_ID + ".graph-guard.stdout.txt")).write_bytes(exc.stdout or b"")
        (builds / (FIXTURE_ID + ".graph-guard.stderr.txt")).write_bytes(exc.stderr or b"")
        raise ValueError("Optional prepared artifact graph gate timed out") from exc
    # These raw process logs stay in the existing quarantined builds directory.
    # The fixed, hash-bound proof below is retained in the compiler manifest.
    (builds / (FIXTURE_ID + ".graph-guard.stdout.txt")).write_bytes(process.stdout)
    (builds / (FIXTURE_ID + ".graph-guard.stderr.txt")).write_bytes(process.stderr)
    if process.returncode != 0:
        raise ValueError("Optional prepared artifact failed the independent Go graph gate")
    observed = []
    for line in process.stdout.decode("utf-8").splitlines():
        if GRAPH_MARKER in line:
            observed.append(json.loads(line.split(GRAPH_MARKER, 1)[1], object_pairs_hook=_unique))
    expected = {"schemaVersion": "1.0", "fixtureId": FIXTURE_ID,
                "status": "PASS_NATIVE_STRUCTURE_ONLY", "sourceSha256": SOURCE_SHA256,
                "expectedContractSha256": CONTRACT_SHA256, "unsignedPlistSha256": expected_sha256,
                "nativeActionCount": 400, "appleRuntime": "NOT_RUN", "formalAcceptanceGranted": False}
    same = (len(observed) == 1 and isinstance(observed[0], dict) and set(observed[0]) == set(expected)
            and all(type(observed[0][key]) is type(value) and observed[0][key] == value
                    for key, value in expected.items()))
    path = _contained_file(builds, path)
    if not same or hashlib.sha256(path.read_bytes()).hexdigest() != expected_sha256:
        raise ValueError("Optional graph receipt is absent, duplicated or does not bind the unchanged artifact")
    return {**expected, "testStdoutSha256": hashlib.sha256(process.stdout).hexdigest(),
            "testStderrSha256": hashlib.sha256(process.stderr).hexdigest()}
