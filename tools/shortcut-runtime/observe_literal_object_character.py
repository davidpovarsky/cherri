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


def digest(data):
    return hashlib.sha256(data).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--prelude", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--source-sha", required=True)
    parser.add_argument("--run-id", required=True)
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[2]

    def git(*command):
        return subprocess.run(["git", *command], cwd=root, check=True, capture_output=True).stdout

    if platform.system() != "Darwin":
        raise SystemExit("Actual macOS is required; no simulated evidence is emitted")
    if git("rev-parse", "HEAD").decode().strip() != args.source_sha or git("status", "--porcelain"):
        raise SystemExit("Exact clean source is required")
    prelude = args.prelude.read_bytes()
    if digest(prelude) != PRELUDE_SHA256:
        raise SystemExit("Pinned prelude bytes differ")
    script = (root / SCRIPT).read_bytes()
    if script != git("show", args.source_sha + ":" + SCRIPT):
        raise SystemExit("Observation script differs from source")
    args.output.mkdir(parents=True, exist_ok=True)
    combined = args.output / "combined-original.js"
    combined.write_bytes(prelude + b"\n" + script)
    command = ["/usr/bin/osascript", "-l", "JavaScript", str(combined), str(args.output / "observations-original.json")]
    now = lambda: datetime.datetime.now(datetime.timezone.utc).isoformat()
    receipt = {
        "format_version": 1, "source_sha": args.source_sha, "run_id": args.run_id,
        "source_clean_before": True, "scope": "S01 serialization observation only",
        "shortcut_runtime": False, "mock": False, "command": command,
        "prelude_commit": PIN, "prelude_sha256": digest(prelude),
        "script_sha256": digest(script), "combined_sha256": digest(combined.read_bytes()),
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
