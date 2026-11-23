#!/usr/bin/env python3
"""Retain bounded public editor probe files; this collector grants no acceptance."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import zipfile

MAX_FILES = 128
MAX_FILE_BYTES = 8 << 20
MAX_TOTAL_BYTES = 32 << 20
PUBLIC_BUILD_PRODUCTS = {"editor-publication-generated.swift", "position-generated.swift"} | {
    stem + suffix
    for stem in ("editor-publication-probe", "position-probe")
    for suffix in ("", ".exe", ".exp", ".lib")
}
STEPS = {"e01": {"service"}, "e02": {"service"}, "e03": {"service"},
         "e05": {"mobile", "swift"}, "e06": {"lsp", "mobile", "swift"}}

def allowed(relative):
    parts = relative.parts
    if parts == ("editor-contract-evidence.json",):
        return True
    if parts and parts[0] == "canonical-swift-paths":
        parts = parts[1:]
        if parts == ("canonical-swift-paths-evidence.json",):
            return True
    if len(parts) not in {2, 3}:
        return False
    folder = re.fullmatch(r"editor-contract-(e01|e02|e03|e05|e06)-[A-Za-z0-9]+", parts[0])
    if not folder:
        return False
    ident = folder.group(1)
    if len(parts) == 2:
        return parts[1] in {ident + "-" + step + "-" + suffix
                           for step in STEPS[ident]
                           for suffix in ("process.json", "go.jsonl", "stderr.log")}
    if parts[1] != "swift" or ident not in {"e05", "e06"}:
        return False
    stem, generated = (("editor-publication-probe", "editor-publication-generated.swift")
                       if ident == "e05" else ("position-probe", "position-generated.swift"))
    return parts[2] in {"receipt.json", "compile.stdout", "compile.stderr", "execute.stdout",
                        "execute.stderr", generated, stem, stem + ".exe",
                        # Windows Swift emits these exact public binary sidecars.
                        stem + ".exp", stem + ".lib"}

def linked(path):
    return path.is_symlink() or getattr(path, "is_junction", lambda: False)()

def collect(root, output, source, run_id, *, tuples_only=False):
    if not re.fullmatch("[0-9a-f]{40}", source) or not re.fullmatch("[0-9]+", run_id):
        raise ValueError("invalid CI context")
    if linked(root):
        raise ValueError("linked evidence root")
    payloads, inventory, omitted, total, scanned_bytes = [], [], [], 0, 0
    if root.exists():
        if not root.is_dir():
            raise ValueError("evidence root is not a directory")
        for folder, directories, names in os.walk(root, followlinks=False):
            for name in directories:
                if linked(Path(folder) / name):
                    raise ValueError("linked evidence directory")
            for name in names:
                path = Path(folder) / name
                relative = path.relative_to(root)
                if linked(path) or not path.is_file() or not allowed(relative):
                    raise ValueError("foreign or linked public evidence path: " + relative.as_posix())
                size = path.stat().st_size
                scanned_bytes += size
                if len(payloads) + len(omitted) >= MAX_FILES or size > MAX_FILE_BYTES or scanned_bytes > MAX_TOTAL_BYTES:
                    raise ValueError("public editor evidence exceeds fixed capture budget")
                if tuples_only and path.name in PUBLIC_BUILD_PRODUCTS:
                    # Keep originals in place; publish neither source nor executables.
                    omitted.append({"path": relative.as_posix(), "bytes": size})
                    continue
                total += size
                with path.open("rb") as stream:
                    data = stream.read(MAX_FILE_BYTES + 1)
                if len(data) != size:
                    raise ValueError("evidence changed during capture")
                payloads.append((relative.as_posix(), data))
                inventory.append({"path": relative.as_posix(), "bytes": size,
                                  "sha256": hashlib.sha256(data).hexdigest()})
    output.mkdir(parents=True, exist_ok=False)
    archive = output / "public-editor-evidence.zip"
    with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as bundle:
        for name, data in sorted(payloads):
            bundle.writestr(name, data)
    report = {"schema_version": 1, "status": "CAPTURED" if inventory else "NOT_RUN",
              "scope": "Public repository editor probe artifacts only; no private corpus or environment dump",
              "ci_source_sha": source, "ci_run_id": run_id,
              "ui_executed": False, "formal_requirements_proved": [],
              "limits": {"files": MAX_FILES, "file_bytes": MAX_FILE_BYTES, "total_bytes": MAX_TOTAL_BYTES},
              "files": sorted(inventory, key=lambda x: x["path"]), "total_bytes": total,
              "archive_sha256": hashlib.sha256(archive.read_bytes()).hexdigest()}
    if tuples_only:
        report["capture_mode"] = "diagnostic_tuples_only"
        report["omitted_public_build_products"] = sorted(omitted, key=lambda item: item["path"])
    (output / "inventory.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8", newline="\n")
    return report

def main():
    parser = argparse.ArgumentParser()
    for name in ("root", "output"):
        parser.add_argument("--" + name, type=Path, required=True)
    parser.add_argument("--source", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--tuples-only", action="store_true",
                        help="retain original public process/diagnostic tuples without generated source or binaries")
    args = parser.parse_args()
    report = collect(args.root, args.output, args.source, args.run_id, tuples_only=args.tuples_only)
    print(json.dumps({"status": report["status"], "files": len(report["files"]),
                      "total_bytes": report["total_bytes"], "archive_sha256": report["archive_sha256"]}))

if __name__ == "__main__":
    main()
