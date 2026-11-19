"""Audit bounded continuation pilots against their immutable source and real oracle.

This tool does not dispatch, sign, simulate native execution, or infer a runtime
cause. Downloaded workflow metadata and published artifacts are external inputs.
Synthetic unit controls test audit rejection, never Apple execution.
"""
import argparse
import ast
import copy
from datetime import datetime
import importlib.util
import json
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys
import tempfile

from audit_storage_text_native import digest, source_bytes

FIXTURES = (
    "qa-storage-text-cross-run-v2", "qa-http-json-control-v2",
    "qa-http-json-fetch-v2", "qa-number-comparison-diag-v2",
)
MANIFEST = "docs/apple-runtime/p10r15/native-continuation-probe-manifest.json"
# Reviewed synthetic selectors use their immutable orchestration contract.
# They never derive the expected result from the downloaded artifact.
REVIEWED_SELECTORS = {
    "CherriBindingCapturePOC": "qa-binding-capture",
    "CherriTypedDefaultsPOC": "qa-typed-defaults",
    "qa-native-boundary-diagnostic-v2": "qa-native-boundary-diagnostic",
    "qa-export-unicode-diagnostic-v2": "qa-export-unicode-diagnostic",
    "qa-collect-empty-split-v2": "qa-collect-empty-split",
    "qa-collect-counts-v2": "qa-collect-counts-v2",
    "qa-nil-clear-v2": "qa-nil-clear",
    "qa-file-name-binding-v2": "qa-file-name-binding",
    "qa-file-name-extension-v2": "qa-file-name-extension",
    "qa-f04-yield-list-v2": "qa-f04-yield-list",
    "qa-f04-yield-empty-list-v2": "qa-f04-yield-empty-list",
    "qa-f04-record-empty-v2": "qa-f04-record-empty",
    "qa-number-coercion-control-v2": "qa-number-coercion-control",
    "qa-export-unicode-v2": "qa-export-unicode",
    "qa-export-empty-v2": "qa-export-empty",
    "qa-export-thousand-v2": "qa-export-thousand",
    "qa-export-redacted-v2": "qa-export-redacted",
    "qa-export-unsupported-file-v2": "qa-export-unsupported-file",
    "qa-export-text-file-v2": "qa-export-text-file",
}
# Reviewed export probes are deliberately nested; runtime input cannot choose paths.
REVIEWED_FILES = {
    "CherriBindingCapturePOC": "p15-binding-defaults/CherriBindingCapturePOC.cherri",
    "CherriTypedDefaultsPOC": "p15-binding-defaults/CherriTypedDefaultsPOC.cherri",
    "qa-export-unicode-v2": "p15-export-probes/qa-export-unicode-v2.cherri",
    "qa-export-empty-v2": "p15-export-probes/qa-export-empty-v2.cherri",
    "qa-export-thousand-v2": "p15-export-probes/qa-export-thousand-v2.cherri",
    "qa-export-redacted-v2": "p15-export-probes/qa-export-redacted-v2.cherri",
    "qa-export-unsupported-file-v2": "p15-export-probes/qa-export-unsupported-file-v2.cherri",
    "qa-export-text-file-v2": "p15-export-probes/qa-export-text-file-v2.cherri",
}
FIXTURES += tuple(REVIEWED_SELECTORS)
SHELL = "scripts/ios27_shortcuts_simulator_qa.sh"
STORAGE = "tools/shortcuts-simulator-qa/storage_cross_run.py"
PUBLISHER = "tools/shortcuts-simulator-qa/prepare_public_artifacts.py"
FIELDS = ("DEQ", "DNE", "REQ", "RNE", "BEQ", "BNE",
          "ONE_NE_ZERO", "ZERO_NE_ONE", "ONE_EQ_ONE")


def require(condition, message):
    if not condition:
        raise ValueError(message)


def constant(source, name):
    matches = [n.value for n in ast.parse(source).body
               if isinstance(n, ast.Assign)
               and any(isinstance(t, ast.Name) and t.id == name for t in n.targets)]
    require(len(matches) == 1, "exact source constant unavailable: " + name)
    return ast.literal_eval(matches[0])



def reviewed_case(commit, fid):
    """Resolve one source-bound contract; do not execute shell or trust runtime input."""
    require(fid in FIXTURES, "unreviewed fixture")
    if fid not in REVIEWED_SELECTORS:
        cases = [c for c in json.loads(source_bytes(commit, MANIFEST))["cases"] if c["id"] == fid]
        require(len(cases) == 1, "ambiguous source fixture")
        return cases[0]
    shell = source_bytes(commit, SHELL).decode("utf-8")
    selector = REVIEWED_SELECTORS[fid]
    blocks = re.findall(r"(?ms)^  " + re.escape(selector) + r"\)\n(.*?)^    ;;[ \t]*$", shell)
    require(len(blocks) == 1, "reviewed selector missing or ambiguous")
    def assignment(name):
        values = re.findall(r'^    ' + re.escape(name) + r'="([^"\r\n]*)"$', blocks[0], re.M)
        require(len(values) == 1, "source selector assignment missing or ambiguous: " + name)
        return values[0]
    require(assignment("TARGET_FIXTURE_ID") == fid, "source selector fixture mismatch")
    require(assignment("EXPECTED_SHORTCUT_NAME") in (fid, "$TARGET_FIXTURE_ID"),
            "source selector shortcut identity mismatch")
    expected = assignment("EXPECTED_SEMANTIC_OUTPUT")
    require(expected.startswith("CHERRI_QA_") and not any(c in expected for c in ("$", chr(96), "\\", "\n")),
            "expected result must be an immutable source literal")
    marker = 'python3 - "$ROOT_DIR" "$ARTIFACT_DIR" "$SOURCE_SHA"'
    require(shell.count(marker) == 1, "fixture compilation entry point ambiguous")
    start = shell.index("<<'PY'\n", shell.index(marker)) + len("<<'PY'\n")
    program = shell[start:shell.index("\nPY\n", start)]
    specs = constant(program, "fixture_specs")
    matching = [row for row in specs if row[0] == fid]
    filename = REVIEWED_FILES.get(fid, fid + ".cherri")
    if fid in REVIEWED_FILES:
        require(assignment("TARGET_SOURCE_RELATIVE") == filename,
                "source selector nested path mismatch")
    require(matching == [(fid, filename, "v2")], "source fixture compilation identity mismatch")
    path = "tools/shortcuts-simulator-qa/fixtures/" + filename
    return {"id": fid, "path": path, "sourceSha256": digest(source_bytes(commit, path)),
            "expectedSummary": expected, "contractSource": SHELL}


def oracle_program(shell):
    marker = '      RUNTIME_EVAL="$(python3 - '
    require(shell.count(marker) == 1, "exact oracle entry point ambiguous")
    start = shell.index("<<'PY'\n", shell.index(marker)) + len("<<'PY'\n")
    return shell[start:shell.index("\nPY\n", start)]


def replay(program, events, compiler, commit, fid, nonce, expected):
    with tempfile.TemporaryDirectory(prefix="cherri-native-oracle-") as directory:
        root = Path(directory)
        (root / "oracle.py").write_text(program, encoding="utf-8")
        for name, value in (("events.json", events), ("compiler.json", compiler)):
            (root / name).write_text(json.dumps(value), encoding="utf-8")
        trace = root / "trace.jsonl"
        result = subprocess.run([sys.executable, "-X", "utf8", str(root / "oracle.py"),
            str(root / "events.json"), nonce, fid, expected, str(trace),
            str(root / "compiler.json"), commit], capture_output=True, text=True,
            encoding="utf-8", timeout=15, check=True)
        return json.loads(result.stdout), trace.read_bytes() if trace.exists() else b""


def publication_files(artifact, publication, allowed):
    require(publication.get("syntheticOnly") is True
            and publication.get("publicationGatePassed") is True
            and publication.get("pixelRedactionClaimed") is False
            and publication.get("rasterPublicationAllowed") is False,
            "bounded synthetic publication policy not verified")
    entries = publication.get("published")
    require(isinstance(entries, list) and entries, "published inventory missing")
    seen = set()
    for entry in entries:
        name = entry.get("path")
        require(isinstance(name, str) and name in allowed and name not in seen,
                "unreviewed or duplicate publication identity")
        relative = PurePosixPath(name)
        require(not relative.is_absolute() and ".." not in relative.parts
                and "\\" not in name, "invalid publication path")
        path = artifact.joinpath(*relative.parts)
        require(not path.is_symlink() and path.resolve().is_relative_to(artifact.resolve()),
                "publication path escapes root")
        raw = path.read_bytes()
        require(type(entry.get("bytes")) is int and len(raw) == entry["bytes"]
                and digest(raw) == entry.get("sha256"), "published bytes/hash mismatch: " + name)
        seen.add(name)
    require(not any(p.is_symlink() for p in artifact.rglob("*")), "unlisted symlink in artifact")
    actual = {p.relative_to(artifact).as_posix() for p in artifact.rglob("*") if p.is_file()}
    require(actual == seen | {"publication-manifest.json"}, "unlisted artifact file")
    return entries


def observation(events, nonce, fid):
    require(isinstance(events, list) and all(isinstance(e, dict) for e in events),
            "invalid published callback store")
    matches = [e for e in events if e.get("runId") == nonce and e.get("receiverSelfTest") is False]
    event = matches[0] if len(matches) == 1 else {}
    identity = (len(matches) == 1 and event.get("eventSource") == "shortcuts-x-callback"
                and event.get("fixtureId") == event.get("expectedShortcutName") == fid)
    summary = event.get("resultSummary")
    return event, {
        "matchingCallbacks": len(matches), "identityVerified": identity,
        "status": event.get("status", "NOT_OBSERVED"),
        "runId": nonce, "timestampISO8601": event.get("timestampISO8601", ""),
        "resultSummary": summary if isinstance(summary, str) and len(summary) <= 1000 else None,
        "resultSummarySha256": digest(summary.encode()) if isinstance(summary, str) else None,
    }


def numeric_flags(summary):
    if not isinstance(summary, str):
        return None
    prefix = next((p for p in ("CHERRI_QA_NUMBER_COMPARISON_RESULT:",
                               "CHERRI_QA_NUMBER_COERCION_RESULT:") if summary.startswith(p)), None)
    if prefix is None:
        return None
    values = summary[len(prefix):].split(":")
    if len(values) != len(FIELDS):
        return None
    pairs = [value.split("=") for value in values]
    if any(len(pair) != 2 or pair[0] != field or pair[1] not in ("Y", "N")
           for pair, field in zip(pairs, FIELDS)):
        return None
    return dict(pairs)


def http_type(trace):
    records = [json.loads(line) for line in trace.splitlines() if line.strip()]
    labels = [r.get("label") for r in records if r.get("kind") == "INLINE_DEBUG_ACTION"
              and str(r.get("label", "")).startswith("http-json-raw-type-")]
    known = {"http-json-raw-type-text": "Text", "http-json-raw-type-dictionary": "Dictionary",
             "http-json-raw-type-other": "Other"}
    return known[labels[0]] if len(labels) == 1 and labels[0] in known else "UNOBSERVED_OR_AMBIGUOUS"


def phase_replays(artifact, diagnostic, program, compiler, commit, source, events):
    """Replay both callbacks; raw key/launch data remain deliberately quarantined."""
    fid = FIXTURES[0]
    require(diagnostic.get("fixtureId") == fid and diagnostic.get("compilerCommitSha") == commit,
            "storage diagnostic source mismatch")
    require(diagnostic.get("sourceSha256") == source["sourceSha256"]
            and diagnostic.get("unsignedPlistSha256") == source["unsignedPlistSha256"],
            "storage diagnostic artifact mismatch")
    require(diagnostic.get("scenario") == "TWO_EXTERNAL_INVOCATIONS_SAME_SIMULATOR"
            and diagnostic.get("globalFlag") is True, "storage scenario mismatch")
    for key in ("sameSimulatorSha256", "runScopedKeySha256"):
        require(isinstance(diagnostic.get(key), str)
                and re.fullmatch(r"[0-9a-f]{64}", diagnostic[key]), "storage binding hash missing")
    for key in ("cacheImplemented", "typedArchiveProven", "ttlProven",
                "crossShortcutIsolationProven", "physicalDeviceProven"):
        require(diagnostic.get(key) is False, "unproved storage scope promoted: " + key)
    plan = {"compilerCommitSha": commit}
    for name in ("writer", "reader"):
        phase = diagnostic.get(name, {})
        nonce = phase.get("runId")
        require(isinstance(nonce, str) and re.fullmatch(
            re.escape(fid) + r"-[A-Za-z0-9-]{1,90}-[0-9a-f]{32}", nonce),
            "invalid phase nonce")
        plan[name + "Nonce"] = nonce
    require(plan["writerNonce"] != plan["readerNonce"], "reused phase nonce")

    raw_helper = source_bytes(commit, STORAGE)
    with tempfile.TemporaryDirectory(prefix="cherri-storage-phases-") as directory:
        root = Path(directory)
        (root / "runtime").mkdir()
        helper = root / "storage_exact.py"
        helper.write_bytes(raw_helper)
        spec = importlib.util.spec_from_file_location("cherri_exact_storage_audit", helper)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        for name, value in (("compiler-manifest.json", compiler), ("runtime/callback-events.json", events)):
            (root / name).write_text(json.dumps(value), encoding="utf-8")
        phases = {}
        for name, phase in (("writer", "WRITE"), ("reader", "READ")):
            oracle, _ = replay(program, events, compiler, commit, fid, plan[name + "Nonce"], module.marker(phase))
            (root / "runtime" / ("storage-" + name + "-oracle.json")).write_text(
                json.dumps(oracle), encoding="utf-8")
            path = "runtime/storage-writer-trace.jsonl" if name == "writer" else "runtime/runtime-trace.jsonl"
            if (artifact / path).is_file():
                (root / path).write_bytes((artifact / path).read_bytes())
            phases[name] = module.phase_observation(root, plan, phase)
            require(phases[name] == diagnostic[name], "storage phase diagnostic does not match replay: " + name)
        both = phases["writer"]["verified"] and phases["reader"]["verified"]
        if diagnostic.get("crossRunTextVerified") is True:
            require(both and diagnostic.get("readerDispatchedAfterVerifiedWriter") is True
                    and diagnostic.get("dispatchResult") == "CALLBACK_OBSERVED",
                    "writer-only or unverified dispatch cannot prove cross-run storage")
            stamp = lambda value: datetime.fromisoformat(value.replace("Z", "+00:00"))
            require(stamp(phases["writer"]["timestampISO8601"]) <= stamp(phases["reader"]["timestampISO8601"]),
                    "reader callback predates writer")
            require(diagnostic.get("cleanup") == "READ_BRANCH_REQUESTED_DELETE_ONLY",
                    "unsupported deletion claim")
    return {
        "publishedDiagnosticSha256": digest((artifact / "runtime/storage-cross-run-diagnostic.json").read_bytes()),
        "sourceHelperSha256": digest(raw_helper), "independentPhaseReplays": phases,
        "crossRunTextVerified": diagnostic.get("crossRunTextVerified") is True,
        "launchAndKeyBinding": "SOURCE_VALIDATED_PUBLIC_DIAGNOSTIC_RAW_INPUTS_QUARANTINED",
        "rawLaunchAndKeyIndependentlyInspected": False,
        "sameSimulatorSha256": diagnostic["sameSimulatorSha256"],
        "runScopedKeySha256": diagnostic["runScopedKeySha256"],
        "cleanup": diagnostic.get("cleanup"),
    }


def causal_faults(program, events, compiler, commit, fid, nonce, expected, baseline):
    if baseline["runGate"] != "PASS" or baseline["traceGate"] != "PASS":
        return {"status": "NOT_RUN_BASELINE_NOT_PASS", "rejected": []}
    event, _ = observation(events, nonce, fid)
    faults = (
        ("wrong_summary", {"resultSummary": expected + "_WRONG"}, "runGate"),
        ("wrong_nonce", {"runId": nonce + "-stale"}, "runGate"),
        ("self_test", {"receiverSelfTest": True, "eventSource": "receiver-self-test"}, "runGate"),
        ("wrong_fixture", {"fixtureId": "qa-other"}, "runGate"),
        ("native_error", {"status": "ERROR"}, "runGate"),
        ("native_cancelled", {"status": "CANCELLED"}, "runGate"),
        ("missing_trace", {"tracePayload": ""}, "traceGate"),
    )
    rejected = []
    for name, change, gate in faults:
        result, _ = replay(program, [{**event, **change}], compiler, commit, fid, nonce, expected)
        require(result[gate] == "FAIL_RUNTIME", "oracle mutation not rejected: " + name)
        rejected.append({"mutation": name, "gate": gate, "result": result[gate]})
    result, _ = replay(program, [event, copy.deepcopy(event)], compiler, commit, fid, nonce, expected)
    require(result["runGate"] == "FAIL_RUNTIME", "duplicate callback not rejected")
    rejected.append({"mutation": "duplicate_callback", "gate": "runGate", "result": result["runGate"]})
    return {"status": "PASS_LOCAL_ORACLE_MUTATIONS_ONLY", "rejected": rejected}


def audit(artifact, commit, run_id, fid, metadata):
    artifact = Path(artifact)
    require(fid in FIXTURES and re.fullmatch(r"[0-9a-f]{40}", commit), "unreviewed fixture or mutable commit")
    require(metadata.get("id") == run_id and metadata.get("head_sha") == commit
            and metadata.get("html_url") == "https://github.com/davidpovarsky/cherri/actions/runs/" + str(run_id)
            and metadata.get("path") == ".github/workflows/cherri-ios27-shortcuts-simulator-qa.yml"
            and metadata.get("event") == "workflow_dispatch" and metadata.get("status") == "completed",
            "workflow metadata is not a completed exact-head dispatch")
    read = lambda path: json.loads((artifact / path).read_bytes())
    publisher = source_bytes(commit, PUBLISHER)
    files = publication_files(artifact, read("publication-manifest.json"), constant(publisher, "ALLOWED"))
    summary, compiler = read("summary.json"), read("compiler-manifest.json")
    assertions, signing = read("runtime/output-assertions.json"), read("runtime/signing-provenance.json")
    events = read("runtime/callback-events.json")
    require(all(data.get("compilerCommitSha") == commit for data in (summary, compiler, assertions, signing)),
            "source provenance disagreement")
    require(str(summary.get("runId")) == str(run_id) and summary.get("requireRuntime") is True
            and summary.get("privacyGatePassed") is True and compiler.get("syntheticOnly") is True,
            "run or synthetic privacy contract missing")
    require(summary.get("targetFixtureId") == assertions.get("targetFixtureId") == fid,
            "fixture identity disagreement")
    case = reviewed_case(commit, fid)
    require(not any(row.get("fixtureId") == fid for row in compiler.get("fixtureRejections", [])),
            "compiler-rejected fixture cannot establish native execution")
    entries = [f for f in compiler["fixtures"] if f.get("fixtureId") == fid]
    require(len(entries) == 1, "ambiguous source fixture")
    source = entries[0]
    target = signing.get("targetProvenance", {})
    source_sha = digest(source_bytes(commit, case["path"]))
    require(source_sha == case["sourceSha256"] == source.get("sourceSha256")
            == assertions.get("compiledSourceSha256") == target.get("sourceSha256"),
            "fixture source bytes disagree")
    require(source.get("unsignedPlistSha256") == assertions.get("unsignedPlistSha256")
            == target.get("unsignedPlistSha256")
            and re.fullmatch(r"[0-9a-f]{64}", source.get("unsignedPlistSha256", "")),
            "unsigned artifact provenance disagreement")
    require(source.get("buildProfile") == assertions.get("buildProfile") == summary.get("buildProfile") == "debug"
            and source.get("runtimeTraceExpectation") == assertions.get("traceExpectation")
            == summary.get("traceExpectation") == "REQUIRED_NDJSON"
            and assertions.get("releaseBuildVerified") is False, "normal debug trace contract required")
    require(type(source.get("nativeActionCount")) is int
            and isinstance(source.get("actionIdentifiers"), list)
            and all(isinstance(value, str) and value for value in source["actionIdentifiers"]),
            "native action inventory shape invalid")
    require(target.get("compilerCommitSha") == commit and target.get("shortcutName") == fid
            and target.get("nativeActionCount") == source.get("nativeActionCount")
            == len(source.get("actionIdentifiers", [])) > 0, "signing target shape disagreement")
    require(target.get("signedArtifactSha256") == signing.get("signedArtifactSha256")
            and re.fullmatch(r"[0-9a-f]{64}", target.get("signedArtifactSha256", ""))
            and target.get("signedByteSize") == signing.get("signedByteSize")
            and type(target.get("signedByteSize")) is int and target["signedByteSize"] > 0
            and target.get("headerMagic") == "AEA1", "signing provenance missing")
    nonce = assertions.get("expectedRunId", "")
    storage = fid == FIXTURES[0]
    nonce_pattern = (re.escape(fid + "-") + (r"(?:reader|" + str(run_id) + r")-" if storage
                     else re.escape(str(run_id) + "-")) + r"[0-9a-f]{32}")
    require(isinstance(nonce, str) and re.fullmatch(nonce_pattern, nonce), "fresh run nonce missing")
    expected = assertions.get("expectedSemanticOutput")
    allowed_outputs = {case["expectedSummary"]}
    if storage:
        allowed_outputs.add(case["expectedSummary"].replace(":READ:", ":WRITE:"))
    require(expected in allowed_outputs, "published expected output differs from reviewed contract")
    shell = source_bytes(commit, SHELL)
    program = oracle_program(shell.decode("utf-8"))
    result, trace = replay(program, events, compiler, commit, fid, nonce, expected)
    event, observed = observation(events, nonce, fid)
    strict = result["runGate"] == result["traceGate"] == "PASS"
    if strict:
        published_trace = (artifact / "runtime/runtime-trace.jsonl").read_bytes()
        require(trace.replace(b"\r\n", b"\n") == published_trace.replace(b"\r\n", b"\n"),
                "published trace differs from canonical replay")
    storage_report = phase_replays(artifact, read("runtime/storage-cross-run-diagnostic.json"),
        program, compiler, commit, source, events) if storage else None
    verified = strict and (not storage or (storage_report["crossRunTextVerified"]
                and expected == case["expectedSummary"]))
    require(summary.get("runtimeContractVerified") is verified
            and assertions.get("runtimeContractVerified") is verified,
            "published runtime success disagrees with independent oracle")
    report = {
        "schemaVersion": 1, "status": "PASS_BOUNDED_NATIVE_CONTRACT" if verified else "FAIL_RUNTIME_PRESERVED",
        "sourceCommit": commit, "runId": run_id, "runUrl": metadata["html_url"],
        "workflowConclusion": metadata.get("conclusion"), "fixtureId": fid,
        "sourceSha256": source_sha, "unsignedPlistSha256": source["unsignedPlistSha256"],
        "signedArtifactSha256": target["signedArtifactSha256"],
        "nativeActionCount": source["nativeActionCount"], "observedCallback": observed,
        "canonicalOracleReplay": result, "runtimeContractVerified": verified,
        "oracleSourceSha256": digest(shell), "publicationPolicySourceSha256": digest(publisher),
        "publicationManifestSha256": digest((artifact / "publication-manifest.json").read_bytes()),
        "publishedHashesVerified": len(files), "files": files,
        "causalOracleControls": causal_faults(program, events, compiler, commit, fid, nonce, expected, result),
        "editorStatus": summary.get("editorInspectorStatus", "NOT_OBSERVED"),
        "environment": read("environment.json"),
        "scope": "This exact source, fixture, synthetic inputs and recorded Simulator run only.",
        "notProven": ["Physical device", "General typed cache/TTL", "Native failure cause",
                      "Complete per-action typed coverage", "Complete Inspector identity mapping",
                      "HTTP status/error catching or redirect policy", "Original require fixture repair"],
        "privacy": {"syntheticOnly": True, "rawOrRasterPublication": False, "privateCorpusUsed": False},
    }
    if storage:
        report["storage"] = storage_report
    if fid in (FIXTURES[3], "qa-number-coercion-control-v2"):
        report["observedNumericFlags"] = numeric_flags(observed["resultSummary"])
        report["expectedNumericFlags"] = numeric_flags(case["expectedSummary"])
    if fid in FIXTURES[1:3]:
        report["observedRawTypeWitnessCategory"] = http_type(trace) if strict else "UNVERIFIED"
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--artifact", type=Path, required=True)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--run-id", type=int, required=True)
    parser.add_argument("--fixture", choices=FIXTURES, required=True)
    parser.add_argument("--run-metadata", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        report = audit(args.artifact, args.commit, args.run_id, args.fixture,
                       json.loads(args.run_metadata.read_bytes()))
    except (ValueError, OSError, KeyError, TypeError, subprocess.SubprocessError) as error:
        report = {"schemaVersion": 1, "status": "FAIL_EVIDENCE_VALIDATION",
                  "sourceCommit": args.commit, "runId": args.run_id, "fixtureId": args.fixture,
                  "runtimeContractVerified": False, "errorType": type(error).__name__,
                  "detail": str(error)[:800]}
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n",
                          encoding="utf-8", newline="\n")
    print(json.dumps({k: report[k] for k in ("status", "fixtureId", "runId", "publishedHashesVerified") if k in report}))
    return 3 if report["status"] == "FAIL_EVIDENCE_VALIDATION" else (0 if report["runtimeContractVerified"] else 2)


if __name__ == "__main__":
    raise SystemExit(main())
