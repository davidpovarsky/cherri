#!/usr/bin/env python3
"""Retain a source-bound real-macOS serialization observation, without runtime credit."""
import argparse
import datetime
import hashlib
import json
from pathlib import Path
import platform
import subprocess
import time

PIN = "ee52c32937053268e4a035eba4b372fe77ec6411"
PRELUDE_SHA256 = "d6fbcaad5104562b04004b39ca098486449399c15c83176c49b2ac28f47a3461"
SCRIPT = "tools/shortcut-runtime/jxa/observe_literal_object_character.js"
JSON_SCRIPT = "tools/shortcut-runtime/jxa/observe_json_container_null.js"
JSON_BODY = "tools/shortcut-runtime/fixtures/json-null-frozen-k-wfjsonvalues.json"
S01_CONTROLS = "tools/shortcut-runtime/fixtures/s01-omitted-range-native-hop-controls.json"
JSON_INPUT_PINS = {
    JSON_BODY: "3b91ec031a1cd3411d22eb9d1001802ab042149274516ee5568051373d9e785f",
    S01_CONTROLS: "cde2ee7a20f7acbb7053ab7b6204774b74ff8a7e6bec298fb9781975ac84ee35",
    SCRIPT: "5e6a502f8c292d1e6db5d7df7cc49f48bbadb426679e4260ccce7d419f082454",
}

DATE_WIRE = "tools/shortcut-runtime/fixtures/date-field-compiled-controls.json"
DATE_INPUT_PINS = {
    DATE_WIRE: "6978f99988123cb91be759648ca5fc476ae83a02ea33e8e3481c6b4a4275edf6",
    "tools/shortcut-runtime/jxa/probe_single_action.js": "872975cda77ab0dda41de0e1aa97214b1a73dd9f9d7bda12a2c7d486b50db415",
}



def digest(data):
    return hashlib.sha256(data).hexdigest()


def checked_input_files(root, source_sha, kind, git):
    inputs = {}
    pins = JSON_INPUT_PINS if kind == "json-container-null" else DATE_INPUT_PINS if kind == "date-field-parameters" else {}
    if pins:
        for relative, expected in pins.items():
            data = (root / relative).read_bytes()
            if digest(data) != expected or data != git("show", source_sha + ":" + relative):
                raise SystemExit("Pinned authored observation input differs from source")
            inputs[relative] = data
    return inputs


def prepare_output(path):
    if path.is_symlink() or (path.exists() and (not path.is_dir() or any(path.iterdir()))):
        raise SystemExit("A fresh empty observation directory is required")
    path.mkdir(parents=True, exist_ok=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--prelude", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--source-sha", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--observation-kind", choices=("literal-object-character", "json-container-null", "date-field-parameters"),
                        default="literal-object-character")
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[2]

    def git(*command):
        return subprocess.run(["git", "--no-replace-objects", *command], cwd=root, check=True, capture_output=True).stdout

    if platform.system() != "Darwin":
        raise SystemExit("Actual macOS is required; no simulated evidence is emitted")
    if git("rev-parse", "HEAD").decode().strip() != args.source_sha or git("status", "--porcelain"):
        raise SystemExit("Exact clean source is required")
    prelude = args.prelude.read_bytes()
    if digest(prelude) != PRELUDE_SHA256:
        raise SystemExit("Pinned prelude bytes differ")
    script_path = JSON_SCRIPT if args.observation_kind in ("json-container-null", "date-field-parameters") else SCRIPT
    script = (root / script_path).read_bytes()
    if script != git("show", args.source_sha + ":" + script_path):
        raise SystemExit("Observation script differs from source")
    input_files = checked_input_files(root, args.source_sha, args.observation_kind, git)
    prepare_output(args.output)
    combined = args.output / "combined-original.js"
    combined.write_bytes(prelude + b"\n" + script)
    command = ["/usr/bin/osascript", "-l", "JavaScript", str(combined), str(args.output / "observations-original.json")]
    if args.observation_kind == "json-container-null":
        command.extend([str(root / JSON_BODY), str(root / S01_CONTROLS)])
    elif args.observation_kind == "date-field-parameters":
        command.extend(["date-field-parameters", str(root / DATE_WIRE)])
    for relative, data in input_files.items():
        (args.output / ("input-original-" + Path(relative).name)).write_bytes(data)
    now = lambda: datetime.datetime.now(datetime.timezone.utc).isoformat()
    receipt = {
        "format_version": 1, "source_sha": args.source_sha, "run_id": args.run_id,
        "source_clean_before": True,
        "scope": {"literal-object-character": "S01 serialization observation only",
                  "json-container-null": "JSON/null/container and S01 serialization observations only",
                  "date-field-parameters": "Date parameter-state serialization observations only"}[args.observation_kind],
        "observation_kind": args.observation_kind,
        "shortcut_runtime": False, "mock": False, "command": command,
        "http_executed": False, "formal_acceptance_granted": False,
        "prelude_commit": PIN, "prelude_sha256": digest(prelude),
        "script_sha256": digest(script), "combined_sha256": digest(combined.read_bytes()),
        "input_files": [{"path": path, "sha256": digest(data), "bytes": len(data)}
                        for path, data in input_files.items()],
        "started_at_utc": now(), "platform": platform.platform(),
    }
    started = time.monotonic()
    with (args.output / "stdout-original.log").open("wb") as stdout, (args.output / "stderr-original.log").open("wb") as stderr:
        try:
            result = subprocess.run(command, stdout=stdout, stderr=stderr, timeout=120)
            receipt.update(exit_code=result.returncode, timed_out=False)
        except subprocess.TimeoutExpired:
            receipt.update(exit_code=None, timed_out=True)
    receipt.update(finished_at_utc=now(), elapsed_seconds=time.monotonic() - started)
    receipt["source_clean_after"] = not git("status", "--porcelain") and git("rev-parse", "HEAD").decode().strip() == args.source_sha
    receipt["artifacts"] = [{"path": path.name, "bytes": path.stat().st_size, "sha256": digest(path.read_bytes())}
                            for path in sorted(args.output.iterdir()) if path.is_file()]
    (args.output / "process-original.json").write_text(json.dumps(receipt, indent=2) + "\n", encoding="utf-8")
    raise SystemExit(0 if receipt["exit_code"] == 0 and receipt["source_clean_after"] and not receipt["timed_out"] else 1)


if __name__ == "__main__":
    main()
