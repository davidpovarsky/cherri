#!/usr/bin/env python3
"""Run one immutable function-array observation; never grant runtime acceptance."""
import argparse
import contextlib
import datetime
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import platform
import re
import stat
import subprocess
import sys
import time

PROBE_SHA = "e0ee87ffa4eb289a8cc9927a86ec0b34a09e9c34"
PROBE_TREE = "7b8854535119d349107d3e0b0a07158831f654ee"
PRELUDE_SHA = "d6fbcaad5104562b04004b39ca098486449399c15c83176c49b2ac28f47a3461"
PRELUDE_COMMIT = "ee52c32937053268e4a035eba4b372fe77ec6411"
PROBE = "tools/shortcut-runtime/observe_function_dictionary_array.py"
PLAN = "tools/shortcut-runtime/fixtures/function-array-hosted-plan.json"
SELF = "tools/shortcut-runtime/hosted_function_array_observation.py"
WORKFLOW = ".github/workflows/macos-shortcuts-deep-extract.yml"
TEST = "tools/shortcut-runtime/tests/test_hosted_function_array_observation.py"
SOURCE_FILES = (WORKFLOW, SELF, PLAN, TEST)
CASE_IDS = ("empty_wrapped_omitted", "empty_wrapped_explicit", "empty_plain",
            "text_wrapped", "text_plain", "attachment_wrapped", "attachment_plain")
CONTROLLER_SECONDS = 150


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def utc():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def write_new(path, value):
    raw = (json.dumps(value, ensure_ascii=False, indent=2, sort_keys=True) + "\n").encode()
    with path.open("xb") as stream:
        stream.write(raw)
    return {"bytes": len(raw), "sha256": sha(raw)}


def regular_file(root, name):
    relative = Path(name)
    if relative.is_absolute() or not relative.parts or any(p in (".", "..") for p in relative.parts):
        raise ValueError("SOURCE_PATH")
    current = root
    for part in relative.parts:
        current = current / part
        if current.is_symlink():
            raise ValueError("SOURCE_SYMLINK")
    if not current.is_file():
        raise ValueError("SOURCE_NOT_REGULAR")
    return current


def validate_plan(plan):
    if (set(plan) != {"schema", "sourceSha", "sourceTree", "preludeSha256", "caseIds",
                      "wholeOwnerSeconds", "perChildSeconds", "sourceFiles"}
            or plan["schema"] != "cherri-function-array-hosted-plan/1"
            or plan["sourceSha"] != PROBE_SHA or plan["sourceTree"] != PROBE_TREE
            or plan["preludeSha256"] != PRELUDE_SHA
            or plan["caseIds"] != list(CASE_IDS)
            or type(plan["wholeOwnerSeconds"]) is not int or plan["wholeOwnerSeconds"] != 90
            or type(plan["perChildSeconds"]) is not int or plan["perChildSeconds"] != 8
            or not isinstance(plan["sourceFiles"], dict) or len(plan["sourceFiles"]) != 24):
        raise ValueError("FIXED_PLAN")
    for name, pin in plan["sourceFiles"].items():
        if (not isinstance(name, str) or not name.startswith("tools/")
                or not isinstance(pin, dict) or set(pin) != {"bytes", "sha256"}
                or type(pin["bytes"]) is not int or not 0 < pin["bytes"] < 1024 * 1024
                or not re.fullmatch(r"[0-9a-f]{64}", pin["sha256"])):
            raise ValueError("FIXED_SOURCE_PIN")
    return plan


def snapshot(root, revision, names, command, expected=None):
    if command(["git", "--no-replace-objects", "-C", str(root), "rev-parse", "HEAD"]).decode().strip() != revision:
        raise ValueError("SOURCE_HEAD")
    if command(["git", "--no-replace-objects", "-C", str(root), "status", "--porcelain"]):
        raise ValueError("SOURCE_DIRTY")
    tree = command(["git", "--no-replace-objects", "-C", str(root), "rev-parse", "HEAD^{tree}"]).decode().strip()
    raw = command(["git", "--no-replace-objects", "-C", str(root), "ls-tree", "-z", revision, "--", *sorted(names)])
    entries = {}
    for entry in raw.split(b"\0"):
        if not entry:
            continue
        fields, name = entry.split(b"\t", 1)
        mode, kind, blob = fields.decode().split()
        name = name.decode()
        if name in entries or mode not in ("100644", "100755") or kind != "blob":
            raise ValueError("SOURCE_GIT_ENTRY")
        entries[name] = (mode, blob)
    if set(entries) != set(names):
        raise ValueError("SOURCE_GIT_INVENTORY")
    pins = {}
    for name in sorted(names):
        path = regular_file(root, name)
        data = path.read_bytes()
        mode, blob = entries[name]
        if hashlib.sha1(b"blob " + str(len(data)).encode() + b"\0" + data).hexdigest() != blob:
            raise ValueError("SOURCE_PHYSICAL_GIT_BYTES")
        if bool(path.stat().st_mode & stat.S_IXUSR) != (mode == "100755"):
            raise ValueError("SOURCE_PHYSICAL_MODE")
        pin = {"bytes": len(data), "sha256": sha(data)}
        if expected is not None and pin != expected[name]:
            raise ValueError("FIXED_SOURCE_BYTES")
        pins[name] = dict(pin, gitBlob=blob, gitMode=mode)
    return {"sha": revision, "tree": tree, "files": pins}


def invoke_original(probe_root, prelude, output, receipt):
    spec = importlib.util.spec_from_file_location("cherri_fixed_function_array_probe", probe_root / PROBE)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    if (module.TOTAL_SECONDS != 90 or module.CHILD_SECONDS != 8
            or tuple(module.CASE_IDS) != CASE_IDS or module.PRELUDE_SHA256 != PRELUDE_SHA):
        raise ValueError("ORIGINAL_PROTOCOL")
    argv = [str(probe_root / PROBE), "--source-sha", PROBE_SHA,
            "--prelude", str(prelude), "--output", str(output / "observation")]
    receipt["producerInvocation"] = {
        "method": "unchanged main on main thread; no producer subprocess",
        "actualControllerProcess": {"originalArgv": list(sys.orig_argv), "executable": sys.executable,
                                    "pid": os.getpid(), "pgid": os.getpgrp()},
        "effectiveProducerSysArgv": list(argv), "producerModulePath": str(probe_root / PROBE),
        "startedAt": utc(), "wholeOwnerSeconds": 90, "perChildSeconds": 8}
    write_new(output / "producer-intent.original.json", receipt["producerInvocation"])
    prior = sys.argv
    started = time.monotonic()
    try:
        sys.argv = argv
        with (output / "producer.stdout.original").open("x") as stdout, (output / "producer.stderr.original").open("x") as stderr:
            with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
                code = module.main()
        receipt["producerInvocation"]["exitCode"] = code
        return code
    finally:
        sys.argv = prior
        receipt["producerInvocation"].update(finishedAt=utc(), elapsedSeconds=time.monotonic() - started)
        write_new(output / "producer-return.original.json", receipt["producerInvocation"])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--probe", type=Path, required=True)
    parser.add_argument("--prelude", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    controller = Path(__file__).resolve().parents[2]
    source = os.environ.get("GITHUB_SHA", "")
    output = args.output
    if not output.is_absolute() or output.exists() or output.is_symlink():
        raise SystemExit("FRESH_OUTPUT_REQUIRED")
    output.parent.resolve(strict=True)
    output.mkdir(mode=0o700)
    (output / "metadata").mkdir(mode=0o700)
    start = time.monotonic()
    deadline = start + CONTROLLER_SECONDS
    receipt = {"schema": "cherri-function-array-hosted-observation/1", "status": "FAILED",
               "startedAt": utc(), "controllerSha": source, "probeSha": PROBE_SHA,
               "controllerBudgetSeconds": CONTROLLER_SECONDS, "producerWholeSeconds": 90, "producerChildSeconds": 8,
               "runId": os.environ.get("GITHUB_RUN_ID"), "runAttempt": os.environ.get("GITHUB_RUN_ATTEMPT"),
               "workflowRef": os.environ.get("GITHUB_WORKFLOW_REF"), "ref": os.environ.get("GITHUB_REF"),
               "runnerLabel": os.environ.get("CHERRI_OBSERVER_RUNNER"), "platform": platform.platform(),
               "nativeAcceptanceGranted": False, "shortcutRuntime": False, "simulatorInvocation": False,
               "commands": []}
    before = None

    def command(argv):
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise TimeoutError("HOSTED_CONTROLLER_DEADLINE")
        index = len(receipt["commands"])
        row = {"argv": argv, "startedAt": utc()}
        receipt["commands"].append(row)
        try:
            run = subprocess.run(argv, stdin=subprocess.DEVNULL, capture_output=True, timeout=min(5, remaining))
            out, err, code = run.stdout, run.stderr, run.returncode
        except subprocess.TimeoutExpired as exc:
            out, err, code = exc.stdout or b"", exc.stderr or b"", None
        for label, raw in (("stdout", out), ("stderr", err)):
            with (output / "metadata" / (str(index).zfill(3) + "." + label + ".original")).open("xb") as stream:
                stream.write(raw)
            row[label] = {"bytes": len(raw), "sha256": sha(raw)}
        row.update(exitCode=code, finishedAt=utc())
        write_new(output / "metadata" / (str(index).zfill(3) + ".process.original.json"), row)
        if code != 0 or len(out) > 2 * 1024 * 1024 or len(err) > 2 * 1024 * 1024:
            raise ValueError("HOSTED_METADATA_PROCESS")
        return out

    def sources(plan):
        return {"controller": snapshot(controller, source, SOURCE_FILES, command),
                "probe": snapshot(args.probe, PROBE_SHA, plan["sourceFiles"], command, plan["sourceFiles"])}

    try:
        if (platform.system() != "Darwin" or not re.fullmatch(r"[0-9a-f]{40}", source)
                or os.environ.get("GITHUB_REPOSITORY") != "davidpovarsky/cherri"
                or os.environ.get("GITHUB_EVENT_NAME") != "workflow_dispatch"
                or not re.fullmatch(r"[1-9][0-9]*", os.environ.get("GITHUB_RUN_ID", ""))
                or not re.fullmatch(r"[1-9][0-9]*", os.environ.get("GITHUB_RUN_ATTEMPT", ""))
                or os.environ.get("CHERRI_FUNCTION_ARRAY_ONLY") != "true"
                or os.environ.get("CHERRI_LITERAL_ONLY") == "true"):
            raise ValueError("FIXED_HOSTED_ROUTE")
        plan = validate_plan(json.loads(regular_file(controller, PLAN).read_bytes()))
        before = sources(plan)
        if before["probe"]["tree"] != PROBE_TREE:
            raise ValueError("PROBE_TREE")
        write_new(output / "source-before.original.json", before)
        if args.prelude.is_symlink() or not args.prelude.is_file():
            raise ValueError("PINNED_PRELUDE")
        prelude_bytes = args.prelude.read_bytes()
        if sha(prelude_bytes) != PRELUDE_SHA:
            raise ValueError("PINNED_PRELUDE")
        prelude_root = args.prelude.parent.parent
        if (command(["git", "--no-replace-objects", "-C", str(prelude_root), "rev-parse", "HEAD"]).decode().strip() != PRELUDE_COMMIT
                or command(["git", "--no-replace-objects", "-C", str(prelude_root), "status", "--porcelain"])):
            raise ValueError("PINNED_PRELUDE_SOURCE")
        with (output / "prelude.original.js").open("xb") as stream:
            stream.write(prelude_bytes)
        receipt["preludeSha256"] = PRELUDE_SHA
        receipt["preludeCommit"] = PRELUDE_COMMIT
        receipt["preludeRepository"] = "frontboat/shortcutkit"
        receipt["macOS"] = {"version": command(["/usr/bin/sw_vers", "-productVersion"]).decode().strip(),
                            "build": command(["/usr/bin/sw_vers", "-buildVersion"]).decode().strip(),
                            "architecture": command(["/usr/bin/uname", "-m"]).decode().strip()}
        fs = os.statvfs(output)
        receipt["freeBytesBefore"] = fs.f_bavail * fs.f_frsize
        if deadline - time.monotonic() < 100:
            raise TimeoutError("ORIGINAL_OBSERVATION_RESERVE")
        code = invoke_original(args.probe, args.prelude, output, receipt)
        observed = json.loads((output / "observation/result.original.json").read_bytes())
        if (observed.get("sourceSha") != PROBE_SHA or observed.get("caseIds") != list(CASE_IDS)
                or observed.get("wholeOwnerSeconds") != 90 or observed.get("perChildSeconds") != 8
                or observed.get("nativeAcceptanceGranted") is not False):
            raise ValueError("ORIGINAL_RESULT_IDENTITY")
        receipt["producerStatus"] = observed.get("status")
        receipt["status"] = "OBSERVED" if code == 0 and observed.get("status") == "OBSERVED" and observed.get("sourceUnchanged") is True else "PRODUCER_FAILED"
    except BaseException as exc:
        receipt["error"] = {"class": type(exc).__name__, "message": str(exc)}
    finally:
        try:
            if before is not None:
                after = sources(plan)
                write_new(output / "source-after.original.json", after)
                receipt["sourceUnchanged"] = after == before
                if after != before:
                    receipt["status"] = "SOURCE_CHANGED"
        except BaseException as exc:
            receipt["sourceUnchanged"] = False
            receipt["sourceAfterError"] = {"class": type(exc).__name__, "message": str(exc)}
            receipt["status"] = "SOURCE_UNVERIFIED"
        fs = os.statvfs(output)
        receipt.update(finishedAt=utc(), elapsedSeconds=time.monotonic() - start, freeBytesAfter=fs.f_bavail * fs.f_frsize)
        receipt["artifacts"] = [{"path": str(p.relative_to(output)), "bytes": p.stat().st_size, "sha256": sha(p.read_bytes())}
                                for p in sorted(output.rglob("*")) if p.is_file() and not p.is_symlink()]
        write_new(output / "result.original.json", receipt)
    print(json.dumps({"status": receipt["status"], "controllerSha": source, "probeSha": PROBE_SHA,
                      "nativeAcceptanceGranted": False, "output": str(output)}))
    return 0 if receipt["status"] == "OBSERVED" and receipt.get("sourceUnchanged") and receipt["elapsedSeconds"] < CONTROLLER_SECONDS else 1


if __name__ == "__main__":
    raise SystemExit(main())
