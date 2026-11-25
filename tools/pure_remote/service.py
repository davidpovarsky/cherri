#!/usr/bin/env python3
"""Versioned external CLI for the unchanged, pinned upstream Cherri compiler."""
from __future__ import annotations
import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import plistlib
import re
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import time

VERSION = "1.0.1"
UPSTREAM = ("bcc0095754e2cc0baaeb4d93dae172b8153b4716",
            "4fe9fbc8c742aaf01d6ec8193d9be708fb1de7b5",
            "https://github.com/electrikmilk/cherri.git")
DOCS = ("4165cc7224b8f4f0500d95129da9fa14eca5458f",
        "f6c3af5143a1bec1a6391c58569fd0aa47673e29",
        "https://github.com/electrikmilk/cherrilang.org.git")
SCHEMA = "cherri-pure-cli-installation/1"


class ContractError(RuntimeError):
    pass


def require(condition, message):
    if not condition:
        raise ContractError(message)


def digest(data):
    return hashlib.sha256(data).hexdigest()


def regular_bytes(path, limit=64 * 1024 * 1024):
    path = Path(path)
    mode = path.lstat().st_mode
    require(stat.S_ISREG(mode) and not path.is_symlink(), "Expected a regular file: " + str(path))
    require(path.stat().st_size <= limit, "File exceeds the bounded input size")
    with path.open("rb") as stream:
        value = stream.read(limit + 1)
    require(len(value) <= limit, "File grew beyond the bounded input size")
    return value


def relative_file(root, name):
    p = PurePosixPath(name)
    require(not p.is_absolute() and p.parts and all(v not in (".", "..") for v in p.parts),
            "Invalid relative installation path")
    result = root.joinpath(*p.parts)
    require(result.resolve().is_relative_to(root.resolve()), "Installation path escapes its root")
    return result


def git(directory, *args):
    process = subprocess.run(["/usr/bin/git", "-C", str(directory), *args],
                             stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=15, check=False)
    require(process.returncode == 0, "Git verification failed: " + " ".join(args[:2]))
    return process.stdout


def verify_checkout(directory, expected):
    require(directory.is_dir() and not directory.is_symlink(), "Missing ordinary checkout")
    require((directory / ".git").is_dir(), "An ordinary .git directory is required")
    alternates = directory / ".git/objects/info/alternates"
    require(not alternates.exists() or not alternates.read_text().strip(), "Checkout depends on external Git objects")
    require(git(directory, "rev-parse", "HEAD").decode().strip() == expected[0], "Checkout commit changed")
    require(git(directory, "rev-parse", "HEAD^{tree}").decode().strip() == expected[1], "Checkout tree changed")
    require(git(directory, "remote", "get-url", "origin").decode().strip() == expected[2], "Checkout origin changed")
    require(not git(directory, "status", "--porcelain=v1", "--untracked-files=all", "--ignored=matching").strip(),
            "Checkout contains changed, untracked, or ignored files")
    count = 0
    for row in git(directory, "ls-tree", "-r", "-z", "HEAD").split(b"\0"):
        if not row:
            continue
        meta, raw_name = row.split(b"\t", 1)
        mode, kind, oid = meta.split()
        require(kind == b"blob" and mode in (b"100644", b"100755"), "Unsupported checkout entry type")
        path = relative_file(directory, raw_name.decode("utf-8"))
        data = regular_bytes(path)
        actual = hashlib.sha1(b"blob " + str(len(data)).encode() + b"\0" + data).hexdigest()
        require(actual == oid.decode(), "Tracked checkout content differs: " + raw_name.decode("utf-8"))
        require(bool(path.stat().st_mode & 0o111) == (mode == b"100755"), "Tracked executable mode differs")
        count += 1
    require(count > 0, "Empty checkout")
    return count


def verify_installation(root, expected_manifest):
    root = Path(root).resolve(strict=True)
    require(re.fullmatch(r"[0-9a-f]{64}", expected_manifest) is not None, "Invalid manifest pin")
    raw = regular_bytes(root / "installation.json", 256 * 1024)
    require(digest(raw) == expected_manifest, "Installation manifest changed")
    manifest = json.loads(raw)
    require(manifest.get("schema") == SCHEMA and manifest.get("wrapperVersion") == VERSION,
            "Unsupported external CLI installation")
    require(manifest.get("engine") == {"commit": UPSTREAM[0], "tree": UPSTREAM[1], "origin": UPSTREAM[2]},
            "Unexpected Pure upstream identity")
    require(manifest.get("docs") == {"commit": DOCS[0], "tree": DOCS[1], "origin": DOCS[2]},
            "Unexpected original documentation identity")
    files = manifest.get("files")
    require(isinstance(files, dict) and set(files) == {"bin/cherri", "lib/service.py", "lib/pack_adapter.py"},
            "Unexpected executable inventory")
    for relative, expected in files.items():
        require(isinstance(expected, str) and re.fullmatch(r"[0-9a-f]{64}", expected) is not None,
                "Invalid executable digest")
        require(digest(regular_bytes(relative_file(root, relative))) == expected,
                "Installation executable changed: " + relative)
    require(os.access(root / "bin/cherri", os.X_OK), "Pure compiler is not executable")
    source_count = verify_checkout(root / "src", UPSTREAM)
    docs_count = verify_checkout(root / "docs", DOCS)
    return {"wrapperVersion": VERSION, "source": UPSTREAM[0], "tree": UPSTREAM[1], "docs": DOCS[0],
            "binarySHA256": files["bin/cherri"], "sourceFiles": source_count, "docsFiles": docs_count,
            "packLoaded": False, "historicalBinaryRestored": False}


def project_snapshot(source, jobs):
    original = Path(source).absolute()
    require(not original.is_symlink(), "Source must not be a symlink")
    source = original.resolve(strict=True)
    jobs = jobs.resolve(strict=True)
    require(source.is_relative_to(jobs) and source != jobs, "Source must be inside the Pure jobs directory")
    require(source.suffix == ".cherri", "Source must be a .cherri file")
    regular_bytes(source)
    project = source.parent
    candidates = [source] if project == jobs else list(project.rglob("*"))
    result = {}
    size = 0
    for path in candidates:
        require(not path.is_symlink(), "Job projects must not contain symlinks")
        if path.is_dir():
            continue
        data = regular_bytes(path)
        relative = path.relative_to(project).as_posix()
        require(".git" not in path.relative_to(project).parts, "A job must not contain a Git checkout")
        size += len(data)
        require(size <= 64 * 1024 * 1024 and len(result) < 2000, "Job exceeds the bounded project size")
        if path.suffix == ".cherri":
            text = data.decode("utf-8")
            # These are filesystem boundaries, not a replacement Cherri parser.
            for line in text.splitlines():
                name = re.match(r"^\s*#define\s+name\s+(.*)$", line)
                if name:
                    value = name.group(1)
                    require(value.strip() and "/" not in value and "\\" not in value and "\0" not in value,
                            "Workflow names must remain single output filenames")
                include = re.match(r"""^\s*#include\s+['"]([^'"]+)['"]""", line)
                if include:
                    target = PurePosixPath(include.group(1))
                    require(not target.is_absolute() and ".." not in target.parts and "\\" not in str(target),
                            "Job includes must stay in the staged project or upstream actions")
        result[relative] = data
    return source, project, result


def _owned_group_present(pgid):
    try:
        os.killpg(pgid, 0)
        return True
    except ProcessLookupError:
        return False


def _finish_owned_group(process, pgid, grace=5):
    """Drain the new session, including children surviving the direct parent."""
    result = {"pgid": pgid, "signals": [], "groupGone": False}
    for sig in (signal.SIGTERM, signal.SIGKILL):
        process.poll()
        if not _owned_group_present(pgid):
            result["groupGone"] = True
            return result
        try:
            os.killpg(pgid, sig)
            result["signals"].append(int(sig))
        except ProcessLookupError:
            result["groupGone"] = True
            return result
        deadline = time.monotonic() + grace
        while time.monotonic() < deadline:
            process.poll()
            if not _owned_group_present(pgid):
                result["groupGone"] = True
                return result
            time.sleep(.02)
    process.poll()
    result["groupGone"] = not _owned_group_present(pgid)
    return result


def execute(argv, cwd, output, timeout=90):
    started = time.monotonic()
    output = Path(output)
    process = None
    pgid = None
    failure = None
    previous_handlers = {}
    cancellation = {"signal": None}
    report = {"argv": [str(v) for v in argv], "exitCode": None, "timedOut": False,
              "cancelled": False, "cancelSignal": None, "status": "FAILED"}

    def request_cancel(signum, _frame):
        # No exception in Popen's assignment window: the owned child always
        # becomes available to the finally cleanup before cancellation acts.
        if cancellation["signal"] is None:
            cancellation["signal"] = signum

    try:
        for sig in (signal.SIGINT, signal.SIGTERM):
            previous_handlers[sig] = signal.getsignal(sig)
            signal.signal(sig, request_cancel)
        with (output / "stdout.original.bin").open("xb") as stdout, (output / "stderr.original.bin").open("xb") as stderr:
            if cancellation["signal"] is None:
                process = subprocess.Popen(argv, cwd=cwd, stdin=subprocess.DEVNULL,
                                           stdout=stdout, stderr=stderr, start_new_session=True)
                actual_pgid, actual_sid = os.getpgid(process.pid), os.getsid(process.pid)
                require(actual_pgid == process.pid and actual_sid == process.pid,
                        "Child did not acquire its private process session")
                pgid = process.pid
                report.update({"pid": process.pid, "pgid": pgid, "sid": actual_sid})
                while cancellation["signal"] is None:
                    code = process.poll()
                    if code is not None:
                        report["exitCode"] = code
                        break
                    if time.monotonic() - started >= timeout:
                        report["timedOut"] = True
                        break
                    time.sleep(.02)
    except KeyboardInterrupt:
        cancellation["signal"] = signal.SIGINT
    except BaseException as error:
        failure = error
        report["error"] = {"type": type(error).__name__, "message": str(error)}
    finally:
        try:
            if process is not None:
                if pgid is not None:
                    report["cleanup"] = _finish_owned_group(process, pgid)
                    require(report["cleanup"]["groupGone"], "Owned compiler process group did not terminate")
                else:
                    # A failed session check never authorizes a group signal.
                    process.kill()
                    process.wait(timeout=5)
                    report["cleanup"] = {"groupSignalAuthorized": False, "directChildReaped": True}
                report["exitCode"] = process.wait(timeout=5)
            else:
                report["cleanup"] = {"childStarted": False, "groupGone": True}
        except BaseException as error:
            report["cleanupError"] = {"type": type(error).__name__, "message": str(error)}
            if failure is None:
                failure = error
        finally:
            for sig, handler in previous_handlers.items():
                signal.signal(sig, handler)
            report["cancelSignal"] = cancellation["signal"]
            report["cancelled"] = cancellation["signal"] is not None
            report["elapsedSeconds"] = time.monotonic() - started
            if failure is None:
                report["status"] = ("CANCELLED" if report["cancelled"] else
                                    "TIMED_OUT" if report["timedOut"] else "COMPLETED")
            with (output / "process.original.json").open("x") as stream:
                json.dump(report, stream, indent=2)
                stream.write("\n")
    if failure is not None:
        raise failure
    return report


def unsigned_actions(data):
    require(not data.startswith(b"AEA1"), "Expected an unsigned plist")
    try:
        value = plistlib.loads(data)
    except (ValueError, TypeError, plistlib.InvalidFileException) as error:
        raise ContractError("Compiler produced an invalid unsigned plist") from error
    require(isinstance(value, dict) and isinstance(value.get("WFWorkflowActions"), list),
            "Compiler did not produce a Shortcut action graph")
    actions = value["WFWorkflowActions"]
    require(all(isinstance(a, dict) and isinstance(a.get("WFWorkflowActionIdentifier"), str)
                and isinstance(a.get("WFWorkflowActionParameters", {}), dict) for a in actions),
            "Malformed Shortcut action")
    return len(actions)


def compile_source(root, manifest_sha, operation, requested):
    root = Path(root).resolve(strict=True)
    before = verify_installation(root, manifest_sha)
    jobs, outputs = root / "jobs", root / "outputs"
    require(jobs.is_dir() and not jobs.is_symlink() and outputs.is_dir() and not outputs.is_symlink(),
            "Missing private job/output directories")
    source, project, files = project_snapshot(requested, jobs)
    stage = Path(tempfile.mkdtemp(prefix="pure-request-", dir=jobs))
    output = outputs / stage.name
    output.mkdir(mode=0o700)
    for name, data in files.items():
        target = relative_file(stage, name)
        target.parent.mkdir(parents=True, exist_ok=True)
        with target.open("xb") as stream:
            stream.write(data)
    staged_source = stage / source.relative_to(project)
    argv = [str(root / "bin/cherri"), str(staged_source), "--no-ansi"]
    if operation == "compile":
        argv.append("--skip-sign")
    else:
        require(operation == "sign", "Unsupported compile operation")
        argv.extend(["--hubsign", "--output=" + str(output / "signed.shortcut")])
    report = {"schema": "cherri-pure-cli-request/1", "wrapperVersion": VERSION, "operation": operation,
              "source": str(source), "sourceSHA256": digest(files[source.relative_to(project).as_posix()]),
              "engine": before, "requestDirectory": str(stage), "outputDirectory": str(output),
              "nativeRuntimeCredit": False, "status": "FAILED"}
    try:
        report["process"] = execute(argv, stage, output)
        require(not report["process"]["cancelled"] and not report["process"]["timedOut"]
                and report["process"]["exitCode"] == 0,
                "Upstream compiler failed; original output is retained in " + str(output))
        for name, data in files.items():
            require(regular_bytes(project / name) == data, "Authored project changed during compilation")
        require(verify_installation(root, manifest_sha) == before, "Pure installation changed during compilation")
        if operation == "compile":
            generated = list(stage.glob("*_unsigned.shortcut"))
            require(len(generated) == 1, "Expected exactly one fresh unsigned Shortcut")
            data = regular_bytes(generated[0], 25 * 1024 * 1024)
            report["actions"] = unsigned_actions(data)
            artifact = output / "unsigned.shortcut"
            with artifact.open("xb") as stream:
                stream.write(data)
            report["status"] = "COMPILED_UNSIGNED_PLIST"
        else:
            artifact = output / "signed.shortcut"
            data = regular_bytes(artifact, 25 * 1024 * 1024)
            require(len(data) > 4 and data.startswith(b"AEA1"), "Upstream did not produce signed-container bytes")
            report["status"] = "UPSTREAM_AEA1_FORMAT_ONLY"
            report["cryptographicSignatureVerified"] = False
        report["artifact"] = {"path": str(artifact), "bytes": len(data), "sha256": digest(data)}
        return report
    except (OSError, ValueError, ContractError) as error:
        report["error"] = str(error)
        raise
    finally:
        report["stdoutSHA256"] = digest((output / "stdout.original.bin").read_bytes()) if (output / "stdout.original.bin").exists() else None
        report["stderrSHA256"] = digest((output / "stderr.original.bin").read_bytes()) if (output / "stderr.original.bin").exists() else None
        (output / "result.original.json").write_text(json.dumps(report, indent=2) + "\n")


def show_docs(root, keyword):
    base = root / "docs"
    result = {"source": DOCS[0], "officialURL": "https://cherrilang.org/language/", "matches": []}
    if keyword:
        for path in sorted(base.rglob("*.md")):
            for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
                if keyword.casefold() in line.casefold():
                    result["matches"].append({"path": path.relative_to(base).as_posix(), "line": number, "text": line})
                    if len(result["matches"]) == 20:
                        return result
    return result


def main(argv=None):
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--manifest-sha256", required=True)
    parser.add_argument("operation", choices=["status", "verify", "docs", "compile", "sign"])
    parser.add_argument("argument", nargs="?")
    args = parser.parse_args(argv)
    try:
        if args.operation in ("compile", "sign"):
            require(args.argument is not None, "A job source is required")
            result = compile_source(args.root, args.manifest_sha256, args.operation, args.argument)
        else:
            result = verify_installation(args.root, args.manifest_sha256)
            if args.operation == "docs":
                result = show_docs(args.root.resolve(), args.argument)
            else:
                require(args.argument is None, "Unexpected argument")
                result["status"] = "PURE_ORIGINAL_SOURCE_AND_FRESH_BINARY_VERIFIED"
        print(json.dumps(result, ensure_ascii=False))
        return 0
    except (OSError, ValueError, ContractError, subprocess.TimeoutExpired) as error:
        print("PURE_CLI_ERROR: " + str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
