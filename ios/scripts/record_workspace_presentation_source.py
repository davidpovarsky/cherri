#!/usr/bin/env python3
"""Record the declared UIKit component source surface; grant no acceptance tier."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import time
import types


METHODS = (
    "CherriTests/CherriWorkspacePresentationTests/testCompactPaneTransitionsKeepNativeViewsAndDocumentState",
    "CherriTests/CherriWorkspacePresentationTests/testCompactUncompiledDocumentsDoNotInstantiateWebKit",
    "CherriTests/CherriWorkspacePresentationTests/testDocumentationSearchUsesLiteralNativeInputTraits",
)
SOURCE_FILES = (
    ".github/workflows/ios-build.yml",
    "ios/project.yml",
    "ios/CherriApp/App/WorkspaceView.swift",
    "ios/CherriApp/Documentation/DocumentationSearchView.swift",
    "ios/CherriApp/Editor/CherriEditorView.swift",
    "ios/CherriApp/Preview/ShortcutPreviewView.swift",
    "ios/CherriTests/CherriWorkspacePresentationTests.swift",
    "ios/scripts/collect_ios_test_evidence.py",
    "ios/scripts/record_workspace_presentation_source.py",
)
SCHEMA = "cherri-workspace-presentation-source/1"
JOINED_SCHEMA = "cherri-joined-editor-component-source/1"
JOINED_METHODS = METHODS + (
    "CherriUITests/CherriJoinedContractUITests/testRP39ActualCombinedEditorAndDelayedAnalysisUI",
    "CherriUITests/CherriPreviewDocumentUITests/testRP41ActualMappedAndGeneratedPreviewSourceUI",
)
JOINED_SOURCE_FILES = SOURCE_FILES + (
    "ios/CherriApp/Diagnostics/EditorDeliveryObservation.swift",
    "ios/CherriApp/Compiler/CherriCompiler.swift",
    "ios/CherriApp/Compiler/CherriDocumentCompileContext.swift",
    "ios/CherriApp/Editor/CherriEditorBuildRequest.swift",
    "ios/CherriApp/Preview/CherriPreviewEditTransaction.swift",
    "ios/CherriApp/Preview/CherriPreviewRenderState.swift",
    "ios/CherriUITests/CherriJoinedContractUITests.swift",
    "ios/CherriUITests/CherriPreviewDocumentUITests.swift",
    "ios/TestFixtures/joined-editor-contract.json",
    "ios/scripts/joined_editor_evidence.py",
    "ios/scripts/test_record_workspace_presentation_source.py",
)
RP39_SCHEMA = "cherri-rp39-diagnostic-source/1"
RP39_METHODS = (JOINED_METHODS[3],)
RP39_SOURCE_FILES = JOINED_SOURCE_FILES + (
    "ios/scripts/collect_ios_xcresult_summary.py",
    "ios/scripts/verify_ios_xcresult_summary.py",
    "ios/scripts/test_ios_xcresult_summary.py",
    "ios/scripts/seal_rp39_diagnostic.py",
    "ios/scripts/test_rp39_diagnostic_transport.py",
    "ios/scripts/source_git_identity.py",
    "ios/scripts/test_rp39_source_git_failure_retention.py",
    "tools/shortcuts-simulator-qa/simulator_session_recovery.py",
    "tools/shortcuts-simulator-qa/qa_terminal_receipts.py",
    "tools/shortcuts-simulator-qa/project_prerequisite.py",
    "tools/shortcuts-simulator-qa/project_prerequisite_observation.py",
    "tools/shortcuts-simulator-qa/xcode_environment_inventory_recipient.pem",
    "ios/CherriUITests/CherriEditorUITests.swift",
    "ios/CherriTests/CherriEditorInputTests.swift",
    "ios/CherriUITests/EditorStartupDeadline.swift",
)
GIT_IDENTITY_FILE = "ios/scripts/source_git_identity.py"
GIT_IDENTITY_PIN = {"bytes": 4676, "sha256": "ffa29a47f2f8502619253ef77c82df37ae8e79c1b90f8ec44e0fe5fdc713affd"}
FAILURE_SCHEMA = "cherri-rp39-source-failure/1"


def require(condition, code):
    if not condition:
        raise ValueError(code)


def validate_mode(documentation, startup, presentation, joined="false", rp39="false"):
    require(all(value in ("true", "false") for value in (documentation, startup, presentation, joined, rp39)),
            "COMPONENT_MODE_VALUE")
    require(presentation != "true" or (documentation == "false" and startup == "false"),
            "COMPONENT_MODE_CONFLICT")
    require(joined != "true" or (documentation == startup == presentation == "false"),
            "COMPONENT_MODE_CONFLICT")
    require(rp39 != "true" or documentation == startup == presentation == joined == "false",
            "COMPONENT_MODE_CONFLICT")


def context_from_environment(env):
    require(env.get("GITHUB_REPOSITORY") == "davidpovarsky/cherri"
            and env.get("GITHUB_WORKFLOW") == "iOS Build"
            and env.get("GITHUB_EVENT_NAME") == "workflow_dispatch", "COMPONENT_CONTEXT")
    source = env.get("GITHUB_SHA", "")
    require(re.fullmatch(r"[0-9a-f]{40}", source) is not None
            and env.get("GITHUB_WORKFLOW_SHA") == source, "COMPONENT_SOURCE_ID")
    run, attempt = env.get("GITHUB_RUN_ID", ""), env.get("GITHUB_RUN_ATTEMPT", "")
    require(re.fullmatch(r"[1-9][0-9]{0,19}", run) is not None
            and re.fullmatch(r"[1-9][0-9]{0,8}", attempt) is not None, "COMPONENT_RUN_ID")
    return {"source": source, "runId": int(run), "attempt": int(attempt),
            "repository": "davidpovarsky/cherri", "workflow": "iOS Build"}


def select_verified_git(repo, deadline):
    path = repo / GIT_IDENTITY_FILE
    info = path.lstat()
    require(stat.S_ISREG(info.st_mode) and path.resolve(strict=True) == path
            and info.st_size == GIT_IDENTITY_PIN["bytes"], "COMPONENT_SOURCE_CHANGED")
    raw = path.read_bytes()
    require({"bytes": len(raw), "sha256": hashlib.sha256(raw).hexdigest()} == GIT_IDENTITY_PIN
            and time.monotonic() < deadline, "COMPONENT_SOURCE_CHANGED")
    module = types.ModuleType("_component_source_git_identity")
    module.__file__ = str(path)
    exec(compile(raw, str(path), "exec"), module.__dict__)
    return module.select_homebrew(deadline)


def git(repo, args, deadline, *, identity=None):
    executable = "/usr/bin/git" if identity is None else str(identity.check(deadline))
    remaining = deadline - time.monotonic()
    require(remaining > 0, "COMPONENT_SOURCE_DEADLINE")
    argv = [executable, "--no-replace-objects", "-C", str(repo), *args]
    result = subprocess.run(argv,
                            env=dict(os.environ, GIT_OPTIONAL_LOCKS="0"),
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                            timeout=min(10, remaining), check=False)
    if result.returncode != 0:
        error = ValueError("COMPONENT_GIT_READ")
        error.cmd, error.returncode = argv, result.returncode
        error.stdout, error.stderr = result.stdout, result.stderr
        raise error
    if identity is not None:
        identity.check(deadline)
    return result.stdout


def source_snapshot(repo, context, source_files=SOURCE_FILES, *, verified_homebrew_git=False):
    require(source_files in (SOURCE_FILES, JOINED_SOURCE_FILES, RP39_SOURCE_FILES), "COMPONENT_SOURCE_PROFILE")
    deadline = time.monotonic() + 20
    require(type(verified_homebrew_git) is bool and
            (not verified_homebrew_git or source_files == RP39_SOURCE_FILES), "COMPONENT_SOURCE_PROFILE")
    selected = select_verified_git(repo, deadline) if verified_homebrew_git else None
    read_git = git
    if selected is not None:
        def read_git(repo, args, deadline):
            return git(repo, args, deadline, identity=selected)
    require(repo == repo.resolve(strict=True), "COMPONENT_SOURCE_ROOT")
    require(read_git(repo, ["rev-parse", "HEAD"], deadline).decode().strip() == context["source"],
            "COMPONENT_SOURCE_CHANGED")
    tree = read_git(repo, ["rev-parse", context["source"] + "^{tree}"], deadline).decode().strip()
    require(re.fullmatch(r"[0-9a-f]{40}", tree) is not None, "COMPONENT_TREE")
    raw = read_git(repo, ["ls-tree", "-r", "-z", context["source"], "--", *source_files], deadline)
    entries = {}
    for row in raw.split(b"\0"):
        if not row:
            continue
        header, path = row.decode().split("\t", 1)
        mode, kind, blob = header.split()
        require(path not in entries and kind == "blob" and mode in ("100644", "100755"),
                "COMPONENT_TRACKED_FILE")
        entries[path] = (mode, blob)
    require(set(entries) == set(source_files), "COMPONENT_SOURCE_SET")
    files = {}
    for name in source_files:
        path = repo / name
        info = path.lstat()
        require(stat.S_ISREG(info.st_mode) and not path.is_symlink()
                and path.resolve() == path and 0 < info.st_size <= 1024 * 1024,
                "COMPONENT_REGULAR_FILE")
        data = path.read_bytes()
        mode, blob = entries[name]
        actual_blob = hashlib.sha1(b"blob " + str(len(data)).encode() + b"\0" + data).hexdigest()
        expected_mode = 0o755 if mode == "100755" else 0o644
        require(len(data) == info.st_size and actual_blob == blob
                and stat.S_IMODE(info.st_mode) == expected_mode, "COMPONENT_SOURCE_CHANGED")
        files[name] = {"gitBlob": blob, "sha256": hashlib.sha256(data).hexdigest(),
                       "bytes": len(data), "mode": mode}
    require(not read_git(repo, ["status", "--porcelain", "--untracked-files=no", "--", *source_files],
                    deadline).strip(), "COMPONENT_SOURCE_CHANGED")
    require(read_git(repo, ["rev-parse", "HEAD"], deadline).decode().strip() == context["source"],
            "COMPONENT_SOURCE_CHANGED")
    if selected is not None:
        selected.check(deadline, contents=True)
        selected.emit("closed")
    return {"tree": tree, "files": files}


def retain_source_failure(repo, context, error):
    """Retain a failed closing read, without supplying a closing source receipt."""
    output = repo / "ios/.build/rp39-diagnostic-source"
    require(output.is_dir() and not output.is_symlink() and output.resolve() == output,
            "COMPONENT_OUTPUT_ALIAS")
    before = output / "before.original.json"
    require(before.is_file() and not before.is_symlink() and before.stat().st_size <= 65536,
            "COMPONENT_BEFORE_REQUIRED")
    raw = before.read_bytes()
    message = str(error)
    argv = getattr(error, "cmd", None)
    require(argv is None or (isinstance(argv, list) and len(argv) <= 80 and
            all(isinstance(item, str) and len(item.encode()) <= 4096 for item in argv)),
            "COMPONENT_FAILURE_LIMIT")
    streams = {}
    for name in ("stdout", "stderr"):
        value = getattr(error, name, None)
        require(value is None or isinstance(value, bytes) and len(value) <= 32768,
                "COMPONENT_FAILURE_LIMIT")
        streams[name] = None if value is None else {"encoding": "hex", "bytes": len(value), "data": value.hex()}
    value = {"schema": FAILURE_SCHEMA, "context": context, "phase": "after",
             "sourceVerification": "FAILED", "formalAcceptanceGranted": False,
             "beforeSHA256": hashlib.sha256(raw).hexdigest(),
             "exception": {"type": type(error).__name__, "message": message[:4096],
                           "messageTruncated": len(message) > 4096, "argv": argv,
                           "timeoutSeconds": getattr(error, "timeout", None),
                           "returnCode": getattr(error, "returncode", None), **streams}}
    data = (json.dumps(value, sort_keys=True, indent=2) + "\n").encode()
    require(len(data) <= 256 * 1024, "COMPONENT_FAILURE_LIMIT")
    with (output / "after-failure.original.json").open("xb") as handle:
        handle.write(data)


def record(repo, phase, context, *, joined=False, rp39=False, verified_homebrew_git=False):
    require(type(joined) is bool and type(rp39) is bool and not (joined and rp39), "COMPONENT_SOURCE_PROFILE")
    require(phase in ("before", "after"), "COMPONENT_PHASE")
    require(type(verified_homebrew_git) is bool and (not verified_homebrew_git or rp39),
            "COMPONENT_SOURCE_PROFILE")
    try:
        snapshot = source_snapshot(repo, context, RP39_SOURCE_FILES if rp39 else JOINED_SOURCE_FILES if joined else SOURCE_FILES,
                                   verified_homebrew_git=verified_homebrew_git)
    except Exception as error:
        if rp39 and phase == "after":
            try:
                retain_source_failure(repo, context, error)
            except Exception:
                print("RP39_SOURCE_FAILURE_RETENTION_UNAVAILABLE")
        raise
    value = {"schema": RP39_SCHEMA if rp39 else JOINED_SCHEMA if joined else SCHEMA, "context": context, "source": snapshot,
             "methods": list(RP39_METHODS if rp39 else JOINED_METHODS if joined else METHODS),
             "scope": "declared-rp39-diagnostic-source-only" if rp39 else "declared-joined-editor-component-source-only" if joined else "declared-ui-component-source-only",
             "nativeAcceptanceGranted": False}
    output = repo / ("ios/.build/rp39-diagnostic-source" if rp39 else "ios/.build/joined-editor-component-source" if joined else
                     "ios/.build/workspace-presentation-source")
    require(not output.parent.is_symlink() and not output.is_symlink(), "COMPONENT_OUTPUT_ALIAS")
    output.mkdir(parents=True, exist_ok=True)
    require(output.resolve() == output, "COMPONENT_OUTPUT_ALIAS")
    if phase == "after":
        before = output / "before.original.json"
        require(before.is_file() and not before.is_symlink() and before.stat().st_size <= 65536,
                "COMPONENT_BEFORE_REQUIRED")
        raw = before.read_bytes()
        expected = (json.dumps(dict(value, phase="before"), sort_keys=True, indent=2) + "\n").encode()
        require(raw == expected, "COMPONENT_BEFORE_CHANGED")
        value["beforeSHA256"] = hashlib.sha256(raw).hexdigest()
    value["phase"] = phase
    path = output / (phase + ".original.json")
    with path.open("x", encoding="utf-8") as handle:
        json.dump(value, handle, sort_keys=True, indent=2)
        handle.write("\n")
    print(json.dumps({"phase": phase, "source": context["source"],
                      "sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
                      "nativeAcceptanceGranted": False}, sort_keys=True))
    return value


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("phase", choices=("mode", "before", "after"))
    parser.add_argument("--verified-homebrew-git", action="store_true")
    args = parser.parse_args()
    validate_mode(os.environ.get("DOCUMENTATION_COMPONENT_ONLY", "false"),
                  os.environ.get("STARTUP_OBSERVATION_ONLY", "false"),
                  os.environ.get("WORKSPACE_PRESENTATION_ONLY", "false"),
                  os.environ.get("JOINED_EDITOR_COMPONENT_ONLY", "false"),
                  os.environ.get("RP39_DIAGNOSTIC_ONLY", "false"))
    if args.phase == "mode":
        return
    joined = os.environ.get("JOINED_EDITOR_COMPONENT_ONLY", "false") == "true"
    rp39 = os.environ.get("RP39_DIAGNOSTIC_ONLY", "false") == "true"
    require(os.environ.get("WORKSPACE_PRESENTATION_ONLY") == "true" or joined or rp39, "COMPONENT_MODE_REQUIRED")
    record(Path(__file__).resolve().parents[2], args.phase, context_from_environment(os.environ), joined=joined, rp39=rp39,
           verified_homebrew_git=args.verified_homebrew_git)


if __name__ == "__main__":
    main()
