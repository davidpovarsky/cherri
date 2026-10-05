#!/usr/bin/env python3
"""tools/shortcut-runtime/runtime_probe.py

Probes the macOS Shortcuts runtime environment:
1. Environment metadata (macOS version, build, architecture, application & framework versions)
2. Private framework loading (WorkflowKit, ActionKit) via JXA with direct disk output
3. dyld shared-cache extraction for built-in action identifiers
"""

import argparse
import json
import os
import pathlib
import platform
import re
import subprocess
import sys


def get_command_output(cmd, default="unknown"):
    try:
        res = subprocess.run(cmd, capture_output=True, text=True, check=False)
        if res.returncode == 0:
            return res.stdout.strip()
    except Exception:
        pass
    return default


def collect_environment():
    env = {
        "runner": os.environ.get("RUNNER_NAME") or platform.system(),
        "platform": platform.system(),
        "arch": platform.machine(),
        "macOS": "unknown",
        "build": "unknown",
        "shortcutsVersion": "unknown",
        "shortcutsBuild": "unknown",
        "workflowKitVersion": "unknown",
        "actionKitVersion": "unknown",
    }
    if sys.platform == "darwin":
        env["macOS"] = get_command_output(["sw_vers", "-productVersion"])
        env["build"] = get_command_output(["sw_vers", "-buildVersion"])
        env["arch"] = get_command_output(["uname", "-m"])
        env["shortcutsVersion"] = get_command_output(
            ["defaults", "read", "/System/Applications/Shortcuts.app/Contents/Info.plist", "CFBundleShortVersionString"]
        )
        env["shortcutsBuild"] = get_command_output(
            ["defaults", "read", "/System/Applications/Shortcuts.app/Contents/Info.plist", "CFBundleVersion"]
        )
        env["workflowKitVersion"] = get_command_output(
            ["defaults", "read", "/System/Library/PrivateFrameworks/WorkflowKit.framework/Versions/A/Resources/Info.plist", "CFBundleShortVersionString"]
        )
        env["actionKitVersion"] = get_command_output(
            ["defaults", "read", "/System/Library/PrivateFrameworks/ActionKit.framework/Versions/A/Resources/Info.plist", "CFBundleShortVersionString"]
        )
    return env


def probe_frameworks(out_file, script_path=None):
    out_path = pathlib.Path(out_file)
    out_path.parent.mkdir(parents=True, exist_ok=True)

    if script_path is None:
        script_path = pathlib.Path(__file__).parent / "jxa" / "private_framework_probe.js"

    if sys.platform != "darwin":
        # Mock/fallback for non-macOS test environments
        mock_data = {
            "WorkflowKit": {"path": "/System/Library/PrivateFrameworks/WorkflowKit.framework", "exists": False, "loaded": False, "error": "Non-macOS platform"},
            "ActionKit": {"path": "/System/Library/PrivateFrameworks/ActionKit.framework", "exists": False, "loaded": False, "error": "Non-macOS platform"}
        }
        out_path.write_text(json.dumps(mock_data, indent=2))
        return mock_data

    # On macOS, run osascript
    cmd = ["osascript", "-l", "JavaScript", str(script_path), str(out_path.resolve())]
    res = subprocess.run(cmd, capture_output=True, text=True, check=False)

    # First attempt: file written directly by JXA
    if out_path.exists() and out_path.stat().st_size > 0:
        try:
            return json.loads(out_path.read_text(encoding="utf-8"))
        except Exception:
            pass

    # Second attempt: parse stdout
    if res.stdout.strip():
        try:
            data = json.loads(res.stdout.strip())
            out_path.write_text(json.dumps(data, indent=2), encoding="utf-8")
            return data
        except Exception:
            pass

    # Third attempt: parse stderr if console.log was used
    if res.stderr.strip():
        try:
            data = json.loads(res.stderr.strip())
            out_path.write_text(json.dumps(data, indent=2), encoding="utf-8")
            return data
        except Exception:
            pass

    fallback_data = {
        "WorkflowKit": {"path": "/System/Library/PrivateFrameworks/WorkflowKit.framework", "exists": False, "loaded": False, "error": "Probe did not output valid JSON. Stderr: " + res.stderr[:200]},
        "ActionKit": {"path": "/System/Library/PrivateFrameworks/ActionKit.framework", "exists": False, "loaded": False, "error": "Probe did not output valid JSON"}
    }
    out_path.write_text(json.dumps(fallback_data, indent=2), encoding="utf-8")
    return fallback_data


def scan_dyld_identifiers(out_file):
    out_path = pathlib.Path(out_file)
    out_path.parent.mkdir(parents=True, exist_ok=True)

    cache_dirs = [
        pathlib.Path("/System/Volumes/Preboot/Cryptexes/OS/System/Library/dyld"),
        pathlib.Path("/System/Library/dyld")
    ]

    identifiers = set()
    pattern = re.compile(rb"is\.workflow\.actions\.[A-Za-z0-9._-]+")

    for cdir in cache_dirs:
        if not cdir.exists():
            continue
        for f in cdir.glob("dyld_shared_cache_arm64e*"):
            try:
                # Use ripgrep or grep if available, otherwise direct python scan of chunks
                cmd = ["rg", "-a", "-o", "--no-filename", r"is\.workflow\.actions\.[A-Za-z0-9._-]+", str(f)]
                rg_res = subprocess.run(cmd, capture_output=True, text=True, check=False)
                if rg_res.returncode == 0:
                    for line in rg_res.stdout.splitlines():
                        line = line.strip()
                        if line:
                            identifiers.add(line)
                    continue
            except Exception:
                pass

            # Fallback Python chunk scan
            try:
                with open(f, "rb") as fh:
                    while chunk := fh.read(4 * 1024 * 1024):
                        for m in pattern.finditer(chunk):
                            identifiers.add(m.group(0).decode("ascii", errors="ignore"))
            except Exception:
                pass

    # Sort identifiers deterministically
    sorted_ids = sorted(identifiers)
    out_path.write_text("\n".join(sorted_ids) + ("\n" if sorted_ids else ""), encoding="utf-8")
    return sorted_ids


def main():
    parser = argparse.ArgumentParser(description="Probes the macOS Shortcuts runtime environment")
    parser.add_argument("--out-dir", default="out/data", help="Output directory")
    parser.add_argument("--script-path", default=None, help="Path to private_framework_probe.js")
    args = parser.parse_args()

    out_dir = pathlib.Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    env = collect_environment()
    env_file = out_dir / "environment.json"
    env_file.write_text(json.dumps(env, indent=2), encoding="utf-8")

    framework_file = out_dir / "private-framework-load.json"
    frameworks = probe_frameworks(framework_file, script_path=args.script_path)

    dyld_file = out_dir / "builtin-action-identifiers.txt"
    dyld_ids = scan_dyld_identifiers(dyld_file)

    summary = {
        "environment": env,
        "framework_load": frameworks,
        "dyld_identifier_count": len(dyld_ids),
        "dyld_identifier_examples": dyld_ids[:25]
    }
    summary_file = out_dir / "probe-summary.json"
    summary_file.write_text(json.dumps(summary, indent=2), encoding="utf-8")
    print(json.dumps(summary, indent=2))


if __name__ == "__main__":
    main()
