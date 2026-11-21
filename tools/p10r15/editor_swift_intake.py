#!/usr/bin/env python3
"""Read-only intake of actual portable editor Swift controls, never UI proof."""
import argparse
import hashlib
import json
import subprocess
from pathlib import Path
from editor_publication_probe import region, successful, inspect_output
from editor_position_probe import inspect as inspect_position

def digest(data):
    return hashlib.sha256(data).hexdigest()

def read_json(path):
    def pairs(values):
        result = {}
        for key, value in values:
            if key in result:
                raise ValueError("duplicate JSON key")
            result[key] = value
        return result
    return json.loads(path.read_text(encoding="utf-8"), object_pairs_hook=pairs)

def verify(repo, source, directory, kind, executed_repo, executed_directory):
    if len(source) != 40 or any(c not in "0123456789abcdef" for c in source):
        raise ValueError("invalid exact source")
    receipt = read_json(directory / "receipt.json")
    if receipt.get("schema_version") != 1 or receipt.get("uiExecuted") is not False or receipt.get("formalRequirementsProved") != [] or receipt.get("sources_unchanged") is not True or receipt.get("passed") is not True:
        raise ValueError("invalid scope or failed source-bound control")
    if kind == "publication":
        paths = {
            "ios/CherriApp/App/WorkspaceView.swift",
            "ios/CherriApp/Editor/CherriEditorView.swift",
            "ios/CherriApp/Editor/CherriCompletionRequest.swift",
            "ios/CherriApp/Editor/CherriEditorBuildRequest.swift",
            "ios/CherriApp/Compiler/CherriCompiler.swift",
            "tools/p10r15/editor_publication_probe.swift",
        }
        generated_name, binary_stem, execute_budget = "editor-publication-generated.swift", "editor-publication-probe", 45
        if receipt.get("realCompilerExecuted") is not False or receipt.get("scope") != "portable Swift control-flow only, controlled backend adapters":
            raise ValueError("publication scope changed")
    elif kind == "position":
        paths = {
            "tools/p10r15/editor_position_probe.py",
            "tools/p10r15/editor_position_probe.swift",
            "tools/p10r15/editor_publication_probe.py",
            "ios/CherriApp/Compiler/CherriCompiler.swift",
            "ios/CherriApp/Editor/CherriCompletionRequest.swift",
        }
        generated_name, binary_stem, execute_budget = "position-generated.swift", "position-probe", 20
        if receipt.get("scope") != "actual Foundation UTF16 helper; no UI":
            raise ValueError("position scope changed")
    else:
        raise ValueError("foreign portable probe")
    items = receipt.get("sources", [])
    if len(items) != len(paths) or {item.get("path") for item in items} != paths:
        raise ValueError("missing or foreign Swift source")
    files = {}
    for item in items:
        raw = subprocess.check_output(["git", "--no-replace-objects", "-C", str(repo), "cat-file", "blob", source + ":" + item["path"]], timeout=10)
        # Git may check out LF blobs as CRLF on Windows. Only these two exact
        # byte representations can match; arbitrary normalization is forbidden.
        choices = [raw, raw.replace(b"\r\n", b"\n").replace(b"\n", b"\r\n")]
        matches = [b for b in choices if digest(b) == item.get("sha256") and len(b) == item.get("bytes")]
        if not matches:
            raise ValueError("immutable Swift source digest differs: " + item["path"])
        files[item["path"]] = matches[0]
    def text(name):
        return files[name].decode("utf-8", errors="strict")
    item = region(text("ios/CherriApp/Compiler/CherriCompiler.swift"), "struct CherriCompletionItem:", "\nstruct CherriWirePosition:")
    if kind == "publication":
        build = region(text("ios/CherriApp/App/WorkspaceView.swift"), "    private func build(", "\n    @MainActor\n    private func refreshActionCatalog")
        completion = region(text("ios/CherriApp/Editor/CherriEditorView.swift"), "    private func refreshContextualCompletions(", "\n    private func refreshDiagnostic")
        template = text("tools/p10r15/editor_publication_probe.swift")
        if template.count("__ACTUAL_BUILD_METHOD__") != 1 or template.count("__ACTUAL_COMPLETION_METHOD__") != 1:
            raise ValueError("ambiguous probe template")
        generated = template.replace("__ACTUAL_BUILD_METHOD__", build.replace("private func ", "func ", 1)).replace("__ACTUAL_COMPLETION_METHOD__", completion.replace("private func ", "func ", 1))
        generated = ("import Foundation\n" + item + "\n" + generated).encode("utf-8")
    else:
        generated = ("import Foundation\n" + item + "\n" + text("tools/p10r15/editor_position_probe.swift")).encode("utf-8")
    expected_generated = {"path": generated_name, "bytes": len(generated), "sha256": digest(generated)}
    if receipt.get("generated") != expected_generated or (directory / generated_name).read_bytes() != generated:
        raise ValueError("compiled body differs from immutable production source")
    for name, budget in (("compile", 90), ("execution", execute_budget)):
        stage = receipt.get(name)
        if not isinstance(stage, dict) or not successful(stage) or stage.get("timeout_seconds") != budget:
            raise ValueError("failed or changed bounded Swift process")
        artifacts = stage.get("artifacts", [])
        prefix = "compile" if name == "compile" else "execute"
        if len(artifacts) != 2 or {a.get("path") for a in artifacts} != {prefix + ".stdout", prefix + ".stderr"}:
            raise ValueError("partial Swift process output")
        for artifact in artifacts:
            raw = (directory / artifact["path"]).read_bytes()
            if len(raw) != artifact.get("bytes") or digest(raw) != artifact.get("sha256"):
                raise ValueError("changed Swift process output")
    binary_name = binary_stem + (".exe" if receipt["execution"]["command"][0].endswith(".exe") else "")
    binary = directory / binary_name
    executed_binary = executed_directory / binary_name
    command = receipt["compile"]["command"]
    expected = [command[0], "-parse-as-library", str(executed_directory / generated_name), str(executed_repo / "ios/CherriApp/Editor/CherriCompletionRequest.swift")]
    if kind == "publication":
        expected.append(str(executed_repo / "ios/CherriApp/Editor/CherriEditorBuildRequest.swift"))
    expected += ["-o", str(executed_binary)]
    if command != expected or Path(command[0]).name.lower() not in {"swiftc", "swiftc.exe"} or receipt["execution"]["command"] != [str(executed_binary)] or not binary.is_file():
        raise ValueError("detached Swift compiler/executable command")
    if kind == "publication":
        data = binary.read_bytes()
        if receipt.get("binary") != {"bytes": len(data), "sha256": digest(data)}:
            raise ValueError("changed compiled executable")
        controls, summary = inspect_output((directory / "execute.stdout").read_bytes())
        if receipt.get("controls") != controls or receipt.get("summary") != summary or summary["failed"] != 0:
            raise ValueError("incomplete publication observations")
        count = len(controls)
    else:
        rows = inspect_position((directory / "execute.stdout").read_bytes())
        if receipt.get("rows") != rows or rows[-1]["failed"] != 0:
            raise ValueError("incomplete position observations")
        count = rows[-1]["controls"]
    return {"kind": kind, "controls": count, "source": source, "ui_executed": False, "result": "VERIFIED"}

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", type=Path, required=True)
    parser.add_argument("--source", required=True)
    parser.add_argument("--directory", type=Path, required=True)
    parser.add_argument("--executed-repo", type=Path, required=True)
    parser.add_argument("--executed-directory", type=Path, required=True)
    parser.add_argument("--kind", choices=["publication", "position"], required=True)
    args = parser.parse_args()
    print(json.dumps(verify(args.repo.resolve(), args.source, args.directory.resolve(), args.kind, args.executed_repo, args.executed_directory)))
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
