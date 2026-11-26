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


def require(condition, code):
    if not condition:
        raise ValueError(code)


def validate_mode(documentation, startup, presentation):
    require(all(value in ("true", "false") for value in (documentation, startup, presentation)),
            "COMPONENT_MODE_VALUE")
    require(presentation != "true" or (documentation == "false" and startup == "false"),
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


def git(repo, args, deadline):
    remaining = deadline - time.monotonic()
    require(remaining > 0, "COMPONENT_SOURCE_DEADLINE")
    result = subprocess.run(["/usr/bin/git", "--no-replace-objects", "-C", str(repo), *args],
                            env=dict(os.environ, GIT_OPTIONAL_LOCKS="0"),
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                            timeout=min(10, remaining), check=False)
    require(result.returncode == 0, "COMPONENT_GIT_READ")
    return result.stdout


def source_snapshot(repo, context):
    deadline = time.monotonic() + 20
    require(repo == repo.resolve(strict=True), "COMPONENT_SOURCE_ROOT")
    require(git(repo, ["rev-parse", "HEAD"], deadline).decode().strip() == context["source"],
            "COMPONENT_SOURCE_CHANGED")
    tree = git(repo, ["rev-parse", context["source"] + "^{tree}"], deadline).decode().strip()
    require(re.fullmatch(r"[0-9a-f]{40}", tree) is not None, "COMPONENT_TREE")
    raw = git(repo, ["ls-tree", "-r", "-z", context["source"], "--", *SOURCE_FILES], deadline)
    entries = {}
    for row in raw.split(b"\0"):
        if not row:
            continue
        header, path = row.decode().split("\t", 1)
        mode, kind, blob = header.split()
        require(path not in entries and kind == "blob" and mode in ("100644", "100755"),
                "COMPONENT_TRACKED_FILE")
        entries[path] = (mode, blob)
    require(set(entries) == set(SOURCE_FILES), "COMPONENT_SOURCE_SET")
    files = {}
    for name in SOURCE_FILES:
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
    require(not git(repo, ["status", "--porcelain", "--untracked-files=no", "--", *SOURCE_FILES],
                    deadline).strip(), "COMPONENT_SOURCE_CHANGED")
    require(git(repo, ["rev-parse", "HEAD"], deadline).decode().strip() == context["source"],
            "COMPONENT_SOURCE_CHANGED")
    return {"tree": tree, "files": files}


def record(repo, phase, context):
    require(phase in ("before", "after"), "COMPONENT_PHASE")
    snapshot = source_snapshot(repo, context)
    value = {"schema": SCHEMA, "context": context, "source": snapshot,
             "methods": list(METHODS), "scope": "declared-ui-component-source-only",
             "nativeAcceptanceGranted": False}
    output = repo / "ios/.build/workspace-presentation-source"
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
    args = parser.parse_args()
    validate_mode(os.environ.get("DOCUMENTATION_COMPONENT_ONLY", "false"),
                  os.environ.get("STARTUP_OBSERVATION_ONLY", "false"),
                  os.environ.get("WORKSPACE_PRESENTATION_ONLY", "false"))
    if args.phase == "mode":
        return
    require(os.environ.get("WORKSPACE_PRESENTATION_ONLY") == "true", "COMPONENT_MODE_REQUIRED")
    record(Path(__file__).resolve().parents[2], args.phase, context_from_environment(os.environ))


if __name__ == "__main__":
    main()
