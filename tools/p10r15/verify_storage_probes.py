"""Bind same-run Stored Content probes to committed source and actual CLI output.

This producer never signs, runs a simulator, simulates storage, or claims
persistence. Native execution is a separate evidence stage.
"""
from pathlib import Path
import argparse
import hashlib
import json
import os
import plistlib
import re
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[2]
QA = ROOT / "tools/shortcuts-simulator-qa"


def sha256(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def command(argv, timeout=180):
    return subprocess.run([str(a) for a in argv], cwd=ROOT, capture_output=True,
                          text=True, encoding="utf-8", errors="strict", timeout=timeout)


def checked(argv, label, timeout=180):
    result = command(argv, timeout)
    if result.returncode:
        raise RuntimeError(label + " exited " + str(result.returncode) +
                           ": " + (result.stdout + result.stderr)[-1800:])
    return result


def verify_build_identity(build_info, source_sha, controlled_rebuild, git_file):
    revision = re.search(r"vcs\.revision=(\S+)", build_info)
    modified = re.search(r"vcs\.modified=(\S+)", build_info)
    if revision:
        if revision.group(1) != source_sha or not modified or modified.group(1) != "false":
            raise RuntimeError("compiler VCS revision or dirty flag differs from the clean source")
        return {"mode": "COMPILER_VCS_METADATA", "vcsRevision": source_sha, "vcsModified": False}
    if not controlled_rebuild or not git_file:
        raise RuntimeError("compiler VCS identity missing; a Git worktree requires explicit --build")
    return {"mode": "CONTROLLED_BUILD_FROM_CLEAN_COMMIT",
            "vcsMetadata": "ABSENT_GIT_WORKTREE_FILE",
            "command": ["go", "build", "-buildvcs=true", "-o", "<reviewed ignored binary>", "."]}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("compiler", type=Path)
    parser.add_argument("--build", action="store_true", help="rebuild the ignored local compiler from the verified clean source")
    parser.add_argument("--output", type=Path,
                        default=ROOT / "artifacts/p15-storage-probes/validation.json")
    args = parser.parse_args()
    compiler, output = args.compiler.resolve(), args.output.resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    started = time.monotonic()
    source_sha = checked(["git", "rev-parse", "HEAD"], "source commit").stdout.strip()
    if checked(["git", "status", "--porcelain"], "source cleanliness").stdout.strip():
        raise RuntimeError("committed clean source is required; artifacts must stay ignored")
    manifest_path = ROOT / "docs/apple-runtime/p10r15/storage-probe-manifest.json"
    manifest = json.loads(manifest_path.read_bytes())
    assert manifest["sameRunOnly"] and not manifest["cacheImplemented"]
    assert manifest["crossRunPersistence"] == manifest["typedArchive"] == "NOT_PROVEN"
    cases = manifest["cases"]
    assert len(cases) == 3 and len({c["id"] for c in cases}) == 3
    paths = [
        "tools/p10r15/verify_storage_probes.py",
        "docs/apple-runtime/p10r15/storage-probe-manifest.json",
        "docs/apple-runtime/p10r15/storage-feasibility-probes.md",
        "actions/storage.cherri",
        "docs/apple-runtime/macos-27/builtin-actions.json",
        "p15_storage_probe_test.go",
        "tools/shortcuts-simulator-qa/test_storage_probe_contract.py",
        "tools/shortcuts-simulator-qa/validate_macro_fixtures.py",
        "scripts/ios27_shortcuts_simulator_qa.sh",
        ".github/workflows/cherri-ios27-shortcuts-simulator-qa.yml",
        ".github/workflows/openminis-skill.yml",
        "tools/shortcuts-simulator-qa/XcodeProject/UITests/CherriSimulatorQAUITests.swift",
        *[c["path"] for c in cases],
    ]
    bound = {}
    for relative in paths:
        data = (ROOT / relative).read_bytes()
        original = subprocess.check_output(["git", "show", source_sha + ":" + relative], cwd=ROOT)
        assert data == original, "source bytes differ from HEAD: " + relative
        bound[relative] = hashlib.sha256(data).hexdigest()
    for case in cases:
        assert bound[case["path"]] == case["sourceSha256"], "manifest source hash drift"
    if args.build:
        relative_binary = compiler.relative_to(ROOT)
        checked(["git", "check-ignore", "--", relative_binary], "owned ignored build output")
        checked(["go", "build", "-buildvcs=true", "-o", compiler, "."], "controlled clean-source build")
    build_info = checked(["go", "version", "-m", compiler], "compiler build identity").stdout
    build_identity = verify_build_identity(build_info, source_sha, args.build, (ROOT / ".git").is_file())
    build_identity["goVersion"] = checked(["go", "version"], "toolchain identity").stdout.strip()

    go_result = checked(["go", "test", "-count=1", "-timeout=5m", "-json",
                         "-run", "^TestP15Storage", "."], "storage Go contracts", 330)
    go_log = output.parent / "storage-go-tests.jsonl"
    go_log.write_bytes(go_result.stdout.encode("utf-8"))
    go_events = [json.loads(line) for line in go_result.stdout.splitlines() if line.strip()]
    go_passes = [e["Test"] for e in go_events if e.get("Action") == "pass" and e.get("Test")]
    assert len(go_passes) == 3 and not any(e.get("Action") in ("fail", "skip") for e in go_events)

    sys.path.insert(0, str(QA))
    test_modules = ["test_storage_probe_contract", "test_workflow_input_contract",
                    "test_runtime_callback_contract", "test_collect_native_contract",
                    "test_public_artifacts"]
    test_environment = {**os.environ, "PYTHONPATH": str(QA)}
    py_log = output.parent / "storage-python-tests.log"
    # Stream the bounded test transcript so a timeout retains its causal location.
    with py_log.open("wb") as transcript:
        py_result = subprocess.run([sys.executable, "-X", "utf8", "-m", "unittest", "-v", *test_modules],
                                   cwd=ROOT, env=test_environment, stdout=transcript,
                                   stderr=subprocess.STDOUT, timeout=330)
    py_transcript = py_log.read_text(encoding="utf-8")
    if py_result.returncode:
        raise RuntimeError("storage Python contracts failed; inspect " + str(py_log))
    match = re.search(r"Ran (\d+) tests in ([0-9.]+)s", py_transcript)
    assert match and re.search(r"\nOK\s*$", py_transcript), "unittest completion receipt missing"

    phase0 = checked([sys.executable, "-X", "utf8", QA / "validate_macro_fixtures.py",
                      ROOT, compiler], "actual Phase 0 CLI and publication gates", 180)
    (output.parent / "storage-phase0.log").write_bytes(phase0.stdout.encode("utf-8"))
    phase0_path = ROOT / "artifacts/p15-macro-harness-validation/validation.json"
    phase0_report = json.loads(phase0_path.read_bytes())
    assert phase0_report["compilerCommitSha"] == source_sha
    assert phase0_report["workingTreeDirty"] is False
    selected = []
    for case in cases:
        entry = phase0_report["selectedFixtures"][case["id"]]
        assert entry["buildProfile"] == "debug" and entry["runtimeTraceExpectation"] == "REQUIRED_NDJSON"
        assert not entry["releaseDiagnosticActionsAbsent"] and not entry["releaseBodyShapeVerified"]
        assert entry["sourceSha256"] == case["sourceSha256"]
        original = ROOT / "artifacts/p15-macro-harness-validation/positive/builds" / (case["id"] + ".shortcut")
        imported = output.parent / (case["id"] + "-import.cherri")
        rebuilt = output.parent / (case["id"] + "-rebuilt.shortcut")
        checked([compiler, "import", original, "--mode=v2", "--lossless", "-o", imported],
                case["id"] + " lossless import")
        checked([compiler, "compile", imported, "-o", rebuilt, "--skip-sign", "--no-ansi"],
                case["id"] + " rebuild")
        original_actions = plistlib.loads(original.read_bytes())["WFWorkflowActions"]
        rebuilt_actions = plistlib.loads(rebuilt.read_bytes())["WFWorkflowActions"]
        assert original_actions == rebuilt_actions, "native storage action roundtrip changed"
        selected.append({**entry, "exactNativeActionRoundtrip": True,
                         "expectedSummary": case["expectedSummary"]})
    assert not checked(["git", "status", "--porcelain"], "final source cleanliness").stdout.strip()
    report = {
        "schemaVersion": 1, "status": "PASS_COMMITTED_SOURCE_AND_CLI_ONLY",
        "sourceCommit": source_sha, "workingTreeDirty": False,
        "compilerBinarySha256": sha256(compiler), "compilerBuildIdentity": build_identity,
        "sourceCheckoutBinding": {"status": "MATCHES_COMMIT_BYTES", "files": len(bound), "sha256": bound},
        "goTests": {"passed": go_passes, "causalGraphMutations": 18,
                    "primitiveFrontendProfileConfigurations": 4, "logSha256": sha256(go_log)},
        "pythonTests": {"modules": test_modules, "passed": int(match.group(1)),
                        "seconds": float(match.group(2)), "logSha256": sha256(py_log)},
        "actualCLIFixturesCompiled": phase0_report["actualCLIFixturesCompiled"],
        "storageExactNativeActionRoundtrips": len(selected),
        "storageFixtures": selected,
        "phase0ReceiptSha256": sha256(phase0_path),
        "causalCompileMutations": phase0_report["causalCompileMutations"],
        "causalPublicationMutations": phase0_report["causalPublicationMutations"],
        "publication": phase0_report["publication"],
        "sameRunOnly": True, "cacheImplemented": False,
        "crossRunPersistence": "NOT_PROVEN", "typedArchive": "NOT_PROVEN",
        "nativeExecuted": 0, "signing": "NOT_RUN",
        "nativePrerequisites": [
            "Same-run exact Text/type and global/local behavior must execute on the target iPad runtime.",
            "Delete and missing-value behavior is diagnostic; ERROR remains FAIL and no catchability is inferred.",
            "Cross-run persistence requires the same installed shortcut and simulator/device state across completed runs.",
            "Typed Dict/List/Date/File archival, TTL, clock drift, corruption and concurrency remain unimplemented/unproven."
        ],
        "retainedDevelopmentFailures": [
            "First Go test build used typed ShortcutAction where the graph oracle required canonical plist maps; fixed by the existing plist marshal/unmarshal codec.",
            "First Legacy primitive fixture omitted its existing actions/storage include; explicit include restored the intended Legacy contract.",
            "The first clean-source producer rejected missing compiler VCS metadata. Installed Go1.25.6 recognizes a .git directory, while this checkout uses a .git worktree file; explicit controlled rebuild records clean Git source and binary hash without inventing embedded VCS fields.",
            "One read/status Sentinel exec was not executed: automatic approval review failed with thread-store metadata recovery deadline; narrower reads and later normally reviewed execution succeeded."
        ],
        "elapsedSeconds": round(time.monotonic() - started, 3),
    }
    output.write_bytes((json.dumps(report, indent=2, ensure_ascii=True) + "\n").encode("utf-8"))
    print(json.dumps({k: report[k] for k in ["status", "sourceCommit", "actualCLIFixturesCompiled",
          "storageExactNativeActionRoundtrips", "sameRunOnly", "cacheImplemented",
          "crossRunPersistence", "typedArchive", "nativeExecuted", "elapsedSeconds"]}, indent=2))


if __name__ == "__main__":
    main()
