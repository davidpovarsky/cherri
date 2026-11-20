#!/usr/bin/env python3
"""Package committed Skill files and independently verify their exact archive bytes.

Runtime binaries must identify the compiler commit in the committed compatibility
manifest. No binary is executed by this tool; actual unpacked self-tests are a
separate mandatory acceptance/CI step.
"""
from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import stat
import subprocess
import sys
import tempfile
import zipfile

PREFIX = "cherri-shortcuts/"
RECEIPT = PREFIX + "package-manifest.json"
SHA = re.compile(r"^[0-9a-f]{40}$")


def checked(command, cwd=None):
    result = subprocess.run(command, cwd=cwd, capture_output=True, timeout=120)
    if result.returncode:
        raise ValueError("command failed: " + str(command[:3]) + ": " +
                         result.stderr.decode("utf-8", errors="replace")[-2000:])
    return result.stdout


def digest(data):
    return hashlib.sha256(data).hexdigest()


def canonical(value):
    return (json.dumps(value, indent=2, sort_keys=True) + "\n").encode("utf-8")


def committed_files(repo, commit):
    if not SHA.fullmatch(commit):
        raise ValueError("skill commit must be an immutable 40-character SHA")
    actual = checked(["git", "--no-replace-objects", "rev-parse", commit + "^{commit}"], repo).decode().strip()
    if actual != commit:
        raise ValueError("skill commit resolution mismatch")
    tree = checked(["git", "--no-replace-objects", "ls-tree", "-r", "-z", commit, "--",
                    "skills/cherri-shortcuts", "LICENSE"], repo)
    files = {}
    for entry in tree.split(b"\0"):
        if not entry:
            continue
        metadata, raw_path = entry.split(b"\t", 1)
        mode, kind, oid = metadata.decode("ascii").split()
        path = raw_path.decode("utf-8")
        if kind != "blob" or mode not in ("100644", "100755"):
            raise ValueError("unsupported tracked package entry: " + path)
        if path == "LICENSE":
            name = PREFIX + "LICENSE.cherri"
        else:
            name = PREFIX + path.removeprefix("skills/cherri-shortcuts/")
        if name in (RECEIPT, PREFIX + "bin/cherri", PREFIX + "bin/cherri.commit",
                    PREFIX + "bin/cherri.sha256", PREFIX + "bin/cherri-migrate",
                    PREFIX + "bin/cherri-migrate.commit", PREFIX + "bin/cherri-migrate.sha256"):
            raise ValueError("reserved generated package path: " + name)
        files[name] = (checked(["git", "--no-replace-objects", "cat-file", "blob", oid], repo), int(mode, 8))
    required = [PREFIX + p for p in ("SKILL.md", "compatibility-manifest.json",
                "scripts/setup.sh", "scripts/self-test.sh", "scripts/common.sh",
                "scripts/json-object.awk", "scripts/installation.sh",
                "scripts/build.sh", "scripts/prepare-edit.sh", "LICENSE.cherri")]
    if not all(path in files for path in required):
        raise ValueError("committed Skill package is incomplete")
    manifest = json.loads(files[PREFIX + "compatibility-manifest.json"][0])
    for field in ("compilerCommit", "docsCommit"):
        if not SHA.fullmatch(manifest.get(field, "")):
            raise ValueError("manifest immutable pin missing: " + field)
    for field in ("schemaFingerprint", "codecFingerprint", "macroFingerprint"):
        if not re.fullmatch(r"[0-9a-f]{64}", manifest.get(field, "")):
            raise ValueError("manifest fingerprint missing: " + field)
    return files, manifest


def inspect_binary(path, expected, go, main_package="github.com/electrikmilk/cherri"):
    data = Path(path).read_bytes()
    # Inspect the same owned snapshot whose bytes and digest will be packaged.
    with tempfile.TemporaryDirectory(prefix="cherri-package-identity-") as temporary:
        snapshot = Path(temporary) / "tool"
        snapshot.write_bytes(data)
        output = checked([go, "version", "-m", str(snapshot)]).decode("utf-8", errors="strict")
    settings = {}
    actual_package = None
    for line in output.splitlines():
        fields = line.strip().split(None, 1)
        if len(fields) == 2 and fields[0] == "path":
            if actual_package is not None:
                raise ValueError("duplicate binary main package")
            actual_package = fields[1]
        if len(fields) == 2 and fields[0] == "build" and "=" in fields[1]:
            key, value = fields[1].split("=", 1)
            if key in settings:
                raise ValueError("duplicate binary build setting: " + key)
            settings[key] = value
    if actual_package != main_package:
        raise ValueError("binary main package mismatch: expected " + main_package + ", got " + str(actual_package))
    if settings.get("vcs") != "git" or settings.get("vcs.modified") != "false":
        raise ValueError("runtime binary lacks clean Git build provenance")
    if settings.get("vcs.revision") != expected:
        raise ValueError("compiler revision mismatch: declared " + expected +
                         ", binary " + settings.get("vcs.revision", "<missing>"))
    if not settings.get("GOOS") or not settings.get("GOARCH"):
        raise ValueError("runtime binary platform provenance missing")
    return data, {"compilerCommit": expected, "sha256": digest(data),
                  "goos": settings["GOOS"], "goarch": settings["GOARCH"],
                  "vcsModified": False, "modulePath": actual_package}


def expected_package(repo, commit, kind, compiler=None, go="go", migrator=None):
    files, manifest = committed_files(repo, commit)
    runtime = None
    if kind == "runtime":
        if not compiler:
            raise ValueError("runtime package requires a trusted compiler file")
        data, runtime = inspect_binary(Path(compiler).resolve(), manifest["compilerCommit"], go)
        if not migrator:
            raise ValueError("runtime package requires a trusted migration tool file")
        migration_data, migration_identity = inspect_binary(
            Path(migrator).resolve(), manifest["compilerCommit"], go,
            "github.com/electrikmilk/cherri/cmd/cherri-migrate")
        if any(runtime[key] != migration_identity[key] for key in ("goos", "goarch")):
            raise ValueError("compiler and migration tool platforms differ")
        runtime["migration"] = migration_identity
        for name, tool_data in (("cherri", data), ("cherri-migrate", migration_data)):
            files[PREFIX + "bin/" + name] = (tool_data, stat.S_IFREG | 0o755)
            files[PREFIX + "bin/" + name + ".commit"] = (
                (manifest["compilerCommit"] + "\n").encode(), stat.S_IFREG | 0o644)
            files[PREFIX + "bin/" + name + ".sha256"] = (
                (digest(tool_data) + "\n").encode(), stat.S_IFREG | 0o644)
    elif kind != "source" or compiler or migrator:
        raise ValueError("source package cannot contain runtime tools")
    receipt = {"schemaVersion": 1, "kind": kind, "skillCommit": commit,
               "compilerCommit": manifest["compilerCommit"],
               "docsCommit": manifest["docsCommit"], "runtime": runtime,
               "files": [{"path": name, "sha256": digest(data), "bytes": len(data),
                          "mode": oct(mode)} for name, (data, mode) in sorted(files.items())],
               "scope": "Exact committed package bytes and clean compiler/migration source and role provenance; "
                        "unpacked execution, installation and signing require separate tests."}
    files[RECEIPT] = (canonical(receipt), stat.S_IFREG | 0o644)
    return files, receipt


def create_archive(archive, files):
    archive = Path(archive)
    archive.parent.mkdir(parents=True, exist_ok=True)
    # Exclusive creation avoids overwriting an existing delivery artifact.
    with zipfile.ZipFile(archive, "x", compression=zipfile.ZIP_DEFLATED) as z:
        for name, (data, mode) in sorted(files.items()):
            info = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
            info.create_system = 3
            info.external_attr = mode << 16
            info.compress_type = zipfile.ZIP_DEFLATED
            z.writestr(info, data)


def verify_archive(archive, files, unpack=None):
    with zipfile.ZipFile(archive) as z:
        entries = z.infolist()
        names = [entry.filename for entry in entries]
        if len(set(names)) != len(names):
            raise ValueError("duplicate archive path")
        for entry in entries:
            path = PurePosixPath(entry.filename)
            if (not entry.filename.startswith(PREFIX) or path.is_absolute() or
                    ".." in path.parts or "\\" in entry.filename or
                    entry.filename != path.as_posix() or entry.is_dir()):
                raise ValueError("unsafe archive path: " + entry.filename)
            mode = entry.external_attr >> 16
            if not stat.S_ISREG(mode) or entry.flag_bits & 1:
                raise ValueError("unsupported archive entry: " + entry.filename)
        if set(names) != set(files):
            raise ValueError("archive file inventory differs from committed package")
        for entry in entries:
            data, mode = files[entry.filename]
            if entry.file_size != len(data) or entry.external_attr >> 16 != mode:
                raise ValueError("archive size/mode mismatch: " + entry.filename)
            if z.read(entry) != data:
                raise ValueError("archive bytes mismatch: " + entry.filename)
        if unpack:
            target = Path(unpack)
            if target.exists() and (not target.is_dir() or any(target.iterdir())):
                raise ValueError("unpack destination must be new or empty")
            target.mkdir(parents=True, exist_ok=True)
            for name, (data, mode) in sorted(files.items()):
                path = target.joinpath(*PurePosixPath(name).parts)
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(data)
                path.chmod(stat.S_IMODE(mode))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("operation", choices=("create", "verify"))
    parser.add_argument("--repo", type=Path, required=True)
    parser.add_argument("--skill-commit", required=True)
    parser.add_argument("--kind", choices=("source", "runtime"), required=True)
    parser.add_argument("--compiler", type=Path)
    parser.add_argument("--migrator", type=Path)
    parser.add_argument("--go", default="go")
    parser.add_argument("--archive", type=Path, required=True)
    parser.add_argument("--unpack", type=Path)
    parser.add_argument("--report", type=Path)
    args = parser.parse_args()
    try:
        files, receipt = expected_package(args.repo, args.skill_commit, args.kind,
                                          args.compiler, args.go, args.migrator)
        if args.operation == "create":
            create_archive(args.archive, files)
        verify_archive(args.archive, files, args.unpack)
        result = {"status": "PASS_PACKAGE_INTEGRITY", "skillCommit": args.skill_commit,
                  "compilerCommit": receipt["compilerCommit"], "docsCommit": receipt["docsCommit"],
                  "kind": args.kind, "archiveSha256": digest(args.archive.read_bytes()),
                  "verifiedFiles": len(files), "runtime": receipt["runtime"],
                  "unpacked": args.unpack is not None,
                  "scope": receipt["scope"]}
        if args.report:
            args.report.parent.mkdir(parents=True, exist_ok=True)
            args.report.write_bytes(canonical(result))
        print(json.dumps(result, sort_keys=True))
        return 0
    except (ValueError, OSError, subprocess.SubprocessError, zipfile.BadZipFile,
            KeyError, UnicodeError) as error:
        print("[CHERRI_PACKAGE_INVALID] " + str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
