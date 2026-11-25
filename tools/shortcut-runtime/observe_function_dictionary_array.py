#!/usr/bin/env python3
"""Observe fixed WorkflowKit array forms with original streams and owned cleanup."""
import argparse
import ast
import datetime
import hashlib
import json
import os
from pathlib import Path
import platform
import signal
import subprocess
import sys
import time

PRELUDE_COMMIT = "ee52c32937053268e4a035eba4b372fe77ec6411"
PRELUDE_SHA256 = "d6fbcaad5104562b04004b39ca098486449399c15c83176c49b2ac28f47a3461"
SCRIPT = "tools/shortcut-runtime/jxa/observe_function_dictionary_array.js"
INPUTS = "tools/shortcut-runtime/fixtures/function-dictionary-array-probe.json"
SELF = "tools/shortcut-runtime/observe_function_dictionary_array.py"
CASE_IDS = ["empty_wrapped_omitted", "empty_wrapped_explicit", "empty_plain", "text_wrapped", "text_plain", "attachment_wrapped", "attachment_plain"]
TOTAL_SECONDS = 90
CHILD_SECONDS = 8


def sha(data):
    return hashlib.sha256(data).hexdigest()


def utc():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def write_new(path, value):
    data = (json.dumps(value, ensure_ascii=False, sort_keys=True, indent=2) + "\n").encode()
    with Path(path).open("xb") as stream:
        stream.write(data)
    return {"path": str(path), "bytes": len(data), "sha256": sha(data)}


def validate_fixed_inputs(inputs):
    if (not isinstance(inputs, dict) or
        set(inputs) != {"schema", "scope", "nativeAcceptanceGranted", "cases"} or
        inputs["schema"] != "cherri-function-array-native-probe-inputs/1" or
        inputs["nativeAcceptanceGranted"] is not False or
        not isinstance(inputs["cases"], list) or
        any(not isinstance(case, dict) or set(case) != {"id", "arrayValue"} for case in inputs["cases"]) or
        [case["id"] for case in inputs["cases"]] != CASE_IDS):
        raise ValueError("FIXED_CASE_INVENTORY")
    return inputs["cases"]


def probe_command(combined, directory, inputs, case_id):
    if case_id not in CASE_IDS:
        raise ValueError("FIXED_CASE_IDENTITY")
    return ["/usr/bin/osascript", "-l", "JavaScript", str(combined), str(directory), str(inputs), case_id]


def helper_closure(root):
    directory = root / "tools/shortcuts-simulator-qa"
    pending = [directory / "project_prerequisite_observation.py"]
    found = set()
    while pending:
        path = pending.pop()
        if path in found:
            continue
        if path.is_symlink() or not path.is_file():
            raise ValueError("HELPER_SOURCE_NOT_REGULAR")
        found.add(path)
        for node in ast.walk(ast.parse(path.read_text(encoding="utf-8"))):
            names = []
            if isinstance(node, ast.Import):
                names = [part.name.split(".")[0] for part in node.names]
            elif isinstance(node, ast.ImportFrom) and node.module:
                names = [node.module.split(".")[0]]
            for name in names:
                candidate = directory / (name + ".py")
                if candidate.is_file():
                    pending.append(candidate)
        if len(found) > 32:
            raise ValueError("HELPER_SOURCE_COUNT")
    return sorted(str(path.relative_to(root)) for path in found)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--prelude", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--source-sha", required=True)
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[2]
    if platform.system() != "Darwin":
        raise SystemExit("Actual macOS required; no simulated observation")
    if len(args.source_sha) != 40 or any(c not in "0123456789abcdef" for c in args.source_sha):
        raise SystemExit("Exact source SHA required")
    output = args.output
    if not output.is_absolute() or output.exists() or output.is_symlink():
        raise SystemExit("Fresh absolute output directory required")
    output.mkdir(mode=0o700)
    start = time.monotonic()
    deadline = start + TOTAL_SECONDS
    receipt = {"schema": "cherri-function-array-native-probe/1", "sourceSha": args.source_sha,
               "startedAt": utc(), "platform": platform.platform(), "scope": "Fixed WorkflowKit Dictionary action copy, parameter read and direct constructors; no Shortcut invocation",
               "wholeOwnerSeconds": TOTAL_SECONDS, "perChildSeconds": CHILD_SECONDS,
               "shortcutRuntime": False, "simulatorInvocation": False, "nativeAcceptanceGranted": False,
               "status": "INCOMPLETE", "commands": [], "caseIds": CASE_IDS}
    sources = None

    def git(*arguments):
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise TimeoutError("WHOLE_OBSERVATION_DEADLINE")
        run = subprocess.run(["git", "--no-replace-objects", "-C", str(root), *arguments],
                             stdin=subprocess.DEVNULL, capture_output=True, timeout=min(5, remaining))
        if run.returncode:
            raise ValueError("SOURCE_GIT_READ")
        return run.stdout

    def snapshot(paths):
        if git("rev-parse", "HEAD").decode().strip() != args.source_sha or git("status", "--porcelain"):
            raise ValueError("EXACT_CLEAN_SOURCE_REQUIRED")
        result = {}
        for name in paths:
            path = root / name
            if path.is_symlink() or not path.is_file():
                raise ValueError("SOURCE_NOT_REGULAR")
            raw = path.read_bytes()
            if raw != git("show", args.source_sha + ":" + name):
                raise ValueError("SOURCE_PHYSICAL_BYTES_DIFFER")
            result[name] = {"bytes": len(raw), "sha256": sha(raw)}
        return result

    def interrupted(number, frame):
        raise InterruptedError("OWNED_OBSERVATION_INTERRUPTED")

    old = {number: signal.signal(number, interrupted) for number in (signal.SIGINT, signal.SIGTERM)}
    try:
        source_paths = sorted([SELF, SCRIPT, INPUTS] + helper_closure(root))
        sources = snapshot(source_paths)
        write_new(output / "source-before.original.json", sources)
        receipt["sourceFiles"] = sources
        if args.prelude.is_symlink() or not args.prelude.is_file():
            raise ValueError("PRELUDE_NOT_REGULAR")
        prelude = args.prelude.read_bytes()
        if sha(prelude) != PRELUDE_SHA256:
            raise ValueError("PINNED_PRELUDE_BYTES_DIFFER")
        receipt["preludeCommit"] = PRELUDE_COMMIT
        receipt["preludeSha256"] = sha(prelude)
        combined = output / "combined.original.js"
        with combined.open("xb") as stream:
            stream.write(prelude + b"\n" + (root / SCRIPT).read_bytes())
        receipt["combinedSha256"] = sha(combined.read_bytes())
        inputs = json.loads((root / INPUTS).read_text())
        validate_fixed_inputs(inputs)
        receipt["osascriptSha256"] = sha(Path("/usr/bin/osascript").read_bytes())
        sys.path.insert(0, str(root / "tools/shortcuts-simulator-qa"))
        from project_prerequisite_observation import capture
        for case_id in CASE_IDS:
            if time.monotonic() >= deadline - 10:
                raise TimeoutError("WHOLE_OBSERVATION_RESERVE")
            directory = output / case_id
            directory.mkdir(mode=0o700)
            command = probe_command(combined, directory, root / INPUTS, case_id)
            command_receipt = {"caseId": case_id, "argv": command, "startedAt": utc()}
            receipt["commands"].append(command_receipt)
            write_new(directory / "intent.original.json", command_receipt)
            with (directory / "stdout.original").open("xb") as stdout, (directory / "stderr.original").open("xb") as stderr:
                observed, process = capture(command, CHILD_SECONDS, output=stdout, error_output=stderr,
                    hard_deadline=deadline - 5, execution_deadline=deadline - 10, stream_limit=1024*1024,
                    journal=lambda event, detail: write_new(directory / ("process-" + event + ".original.json"), detail))
            command_receipt.update(observation=observed, process=process, finishedAt=utc())
            write_new(directory / "process.original.json", command_receipt)
            result_file = directory / "observations.original.json"
            if result_file.is_file() and not result_file.is_symlink():
                raw = result_file.read_bytes()
                parsed = json.loads(raw)
                if parsed.get("schema") != "cherri-function-array-native-observation/1" or parsed.get("caseId") != case_id:
                    raise ValueError("CASE_OBSERVATION_IDENTITY")
                command_receipt["retainedObservation"] = {"bytes": len(raw), "sha256": sha(raw), "status": parsed.get("status")}
        receipt["status"] = "OBSERVED" if all(c["observation"]["result"] == "PASS" and c.get("retainedObservation", {}).get("status") == "OBSERVED" for c in receipt["commands"]) else "PARTIAL_OBSERVATION"
    except BaseException as error:
        receipt["error"] = {"class": type(error).__name__, "message": str(error)}
    finally:
        try:
            if sources is not None:
                after = snapshot(sorted(sources))
                write_new(output / "source-after.original.json", after)
                receipt["sourceUnchanged"] = after == sources
                if not receipt["sourceUnchanged"]:
                    receipt["status"] = "SOURCE_CHANGED"
        except BaseException as error:
            receipt["sourceUnchanged"] = False
            receipt["sourceAfterError"] = {"class": type(error).__name__, "message": str(error)}
            receipt["status"] = "SOURCE_UNVERIFIED"
        receipt["finishedAt"] = utc()
        receipt["elapsedSeconds"] = time.monotonic() - start
        receipt["artifacts"] = [{"path": str(p.relative_to(output)), "bytes": p.stat().st_size, "sha256": sha(p.read_bytes())}
                                for p in sorted(output.rglob("*")) if p.is_file() and not p.is_symlink()]
        result_pin = write_new(output / "result.original.json", receipt)
        for number, handler in old.items():
            signal.signal(number, handler)
    print(json.dumps({"result": result_pin, "status": receipt["status"], "shortcutRuntime": False, "nativeAcceptanceGranted": False}))
    return 0 if receipt["status"] == "OBSERVED" and receipt.get("sourceUnchanged") and receipt["elapsedSeconds"] < TOTAL_SECONDS else 1


if __name__ == "__main__":
    raise SystemExit(main())
