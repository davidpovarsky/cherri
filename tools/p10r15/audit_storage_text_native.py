"""Audit a downloaded, bounded Storage Text native artifact using its exact source oracle.

No native run is simulated. Faults below mutate a local copy of the observed
callback to prove that the original strict oracle rejects false success.
"""
import argparse
import copy
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[2]


def digest(data):
    return hashlib.sha256(data).hexdigest()


def source_bytes(commit, path):
    return subprocess.check_output(["git", "show", commit + ":" + path], cwd=ROOT)


def audit(artifact, commit, run_id):
    def read(path):
        return json.loads((artifact / path).read_text(encoding="utf-8"))

    summary = read("summary.json")
    publication = read("publication-manifest.json")
    compiler = read("compiler-manifest.json")
    assertions = read("runtime/output-assertions.json")
    signing = read("runtime/signing-provenance.json")
    callbacks = read("runtime/callback-events.json")
    assert re.fullmatch(r"[0-9a-f]{40}", commit), "immutable compiler commit required"
    assert summary["compilerCommitSha"] == compiler["compilerCommitSha"] == assertions["compilerCommitSha"] == signing["compilerCommitSha"] == commit
    assert str(summary["runId"]) == str(run_id)
    assert summary["requireRuntime"] is True
    assert summary["traceExpectation"] == "REQUIRED_NDJSON"
    assert summary["runtimeContractVerified"] is True
    assert publication["syntheticOnly"] is True and publication["publicationGatePassed"] is True
    assert publication["pixelRedactionClaimed"] is False and publication["rasterPublicationAllowed"] is False
    files = []
    for item in publication["published"]:
        name = item["path"]
        path = (artifact / name).resolve()
        assert path.is_relative_to(artifact.resolve()), "artifact path escapes root"
        raw = path.read_bytes()
        assert len(raw) == item["bytes"] and digest(raw) == item["sha256"], name
        files.append(item)
    assert len({x["path"] for x in files}) == len(files), "duplicate publication identity"
    assert len(files) == 20, "unexpected bounded publication inventory"

    fid = "qa-storage-text-same-run-v2"
    assert summary["targetFixtureId"] == assertions["targetFixtureId"] == fid
    contract_raw = source_bytes(commit, "docs/apple-runtime/p10r15/storage-probe-manifest.json")
    case = next(x for x in json.loads(contract_raw)["cases"] if x["id"] == fid)
    fixture = next(x for x in compiler["fixtures"] if x["fixtureId"] == fid)
    target = signing["targetProvenance"]
    fixture_bytes = source_bytes(commit, case["path"])
    fixture_sha = digest(fixture_bytes)
    assert fixture_sha == case["sourceSha256"] == fixture["sourceSha256"] == assertions["compiledSourceSha256"] == target["sourceSha256"]
    assert fixture["unsignedPlistSha256"] == assertions["unsignedPlistSha256"] == target["unsignedPlistSha256"]
    assert target["compilerCommitSha"] == commit and target["shortcutName"] == fid
    assert fixture["nativeActionCount"] == target["nativeActionCount"] == 32
    assert len(fixture["actionIdentifiers"]) == 32
    for identifier in ("setstoredcontent", "getstoredcontent", "deletestoredcontent"):
        assert fixture["actionIdentifiers"].count("is.workflow.actions." + identifier) == 1
    assert target["signedArtifactSha256"] == signing["signedArtifactSha256"]
    assert target["signedByteSize"] == signing["signedByteSize"] and target["headerMagic"] == "AEA1"

    fresh_run = assertions["expectedRunId"]
    assert re.fullmatch(re.escape(fid + "-" + str(run_id) + "-") + r"[0-9a-f]{32}", fresh_run)
    matches = [x for x in callbacks if x.get("runId") == fresh_run]
    assert len(matches) == 1
    observed = matches[0]
    assert observed["eventSource"] == "shortcuts-x-callback" and observed["receiverSelfTest"] is False
    assert observed["status"] == "SUCCESS" and observed["resultSummary"] == case["expectedSummary"]
    assert assertions["expectedSemanticOutput"] == case["expectedSummary"]
    assert any(x.get("eventSource") == "receiver-self-test" and x.get("runId") != fresh_run for x in callbacks)
    trace = [json.loads(x) for x in observed["tracePayload"].splitlines() if x.strip()]
    published_trace = [json.loads(x) for x in (artifact / "runtime/runtime-trace.jsonl").read_text(encoding="utf-8").splitlines() if x.strip()]
    assert trace == published_trace
    assert [x["stage"] for x in trace] == ["collector_begin", "before", "after", "collector_end"]
    assert all(x.get("valueType") == "UNKNOWN_AT_TRACE_EVENT" for x in trace if x["kind"] == "INLINE_DEBUG_ACTION")

    shell_raw = source_bytes(commit, "scripts/ios27_shortcuts_simulator_qa.sh")
    script = shell_raw.decode("utf-8")
    start = script.index('      RUNTIME_EVAL="$(python3 - ')
    start = script.index("<<'PY'\n", start) + len("<<'PY'\n")
    end = script.index("\nPY\n", start)
    program = script[start:end]

    def verify(events, manifest=compiler):
        with tempfile.TemporaryDirectory(prefix="cherri-storage-audit-") as temporary:
            path = Path(temporary)
            (path / "events.json").write_text(json.dumps(events), encoding="utf-8")
            (path / "manifest.json").write_text(json.dumps(manifest), encoding="utf-8")
            response = subprocess.run([sys.executable, "-c", program, str(path / "events.json"),
                fresh_run, fid, case["expectedSummary"], str(path / "trace.jsonl"),
                str(path / "manifest.json"), commit], capture_output=True, text=True, timeout=15, check=True)
            return json.loads(response.stdout)

    result = verify(callbacks)
    assert result["runGate"] == result["traceGate"] == "PASS", result
    faults = []
    for name, change in [
        ("wrong_type_flag", {"resultSummary": case["expectedSummary"].replace("TYPE_OK", "TYPE_FAIL")}),
        ("wrong_value_flag", {"resultSummary": case["expectedSummary"].replace("VALUE_OK", "VALUE_FAIL")}),
        ("stale_nonce", {"runId": fresh_run[:-1] + ("0" if fresh_run[-1] != "0" else "1")}),
        ("receiver_self_test", {"eventSource": "receiver-self-test", "receiverSelfTest": True}),
        ("native_error", {"status": "ERROR"}),
        ("missing_trace", {"tracePayload": ""}),
    ]:
        mutated = {**observed, **change}
        rejected = verify([mutated])
        gate = "traceGate" if name == "missing_trace" else "runGate"
        assert rejected[gate] == "FAIL_RUNTIME", (name, rejected)
        faults.append({"mutation": name, "gate": gate, "result": rejected[gate]})
    rejected = verify([observed, copy.deepcopy(observed)])
    assert rejected["runGate"] == "FAIL_RUNTIME"
    faults.append({"mutation": "duplicate_correlated_callback", "gate": "runGate", "result": rejected["runGate"]})

    comparison = read("editor/editor-comparison.json")
    ax = read("editor/ax-persistence-diagnostics.json")
    return {
        "schemaVersion": 1,
        "status": "PASS_NATIVE_SAME_RUN_TEXT_ONLY",
        "sourceCommit": commit,
        "runId": run_id,
        "runUrl": "https://github.com/davidpovarsky/cherri/actions/runs/" + str(run_id),
        "fixtureId": fid,
        "sourceSha256": fixture_sha,
        "unsignedPlistSha256": target["unsignedPlistSha256"],
        "signedArtifactSha256": target["signedArtifactSha256"],
        "signedBytes": target["signedByteSize"],
        "signingMethod": signing["signingMethod"],
        "artifactHeaderValidation": target["verificationTier"],
        "nativeAcceptanceEvidence": "Actual Shortcuts import and fresh correlated execution, separate from AEA1 header validation",
        "nativeActionCount": 32,
        "environment": read("environment.json"),
        "observedCallback": {k: observed[k] for k in ("eventSource", "receiverSelfTest", "fixtureId", "expectedShortcutName", "runId", "status", "timestampISO8601", "resultSummary", "resultShape")},
        "trace": trace,
        "oracle": {"sourceScriptSha256": digest(shell_raw), "exactSourceReplay": result, "causalLocalMutationsRejected": faults},
        "publicationManifestSha256": digest((artifact / "publication-manifest.json").read_bytes()),
        "publishedHashesVerified": len(files),
        "files": files,
        "editor": {"status": comparison["status"], "expectedNativeActionCount": comparison["expectedNativeActionCount"], "observedCardCount": comparison["observedCardCount"], "observedCardCountScope": comparison["observedCardCountScope"], "persistedAXNodes": ax["mergedNodeCount"]},
        "scope": case["scope"],
        "notProven": ["Cross-run persistence", "TTL", "General typed cache/archive", "Dictionary/List/File preservation", "Missing-key/delete semantics", "Global/local scope separation", "Per-action native type coverage", "Physical device", "Complete Inspector 1:1 native identity mapping"],
        "privacy": {"syntheticOnly": True, "rawOrRasterPublication": False, "privateCorpusUsed": False},
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--artifact", type=Path, required=True)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--run-id", type=int, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    report = audit(args.artifact, args.commit, args.run_id)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8", newline="\n")
    print(json.dumps({"status": report["status"], "publishedHashesVerified": report["publishedHashesVerified"], "nativeActionCount": report["nativeActionCount"], "causalLocalMutationsRejected": len(report["oracle"]["causalLocalMutationsRejected"]), "editorStatus": report["editor"]["status"]}))


if __name__ == "__main__":
    main()
