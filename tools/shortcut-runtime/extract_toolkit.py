#!/usr/bin/env python3
"""tools/shortcut-runtime/extract_toolkit.py

Extracts and summarizes the Shortcuts ToolKit SQLite registry.
Fixes previous shell/Python scoping bug (toolkit_status NameError).
Safely opens the database in read-only mode and sanitizes absolute paths.
"""

import argparse
import glob
import json
import os
import pathlib
import sqlite3
import subprocess
import sys


def run_toolkit_dumper(shortcutkit_dir, data_dir, logs_dir):
    dumper_script = pathlib.Path(shortcutkit_dir) / "tools" / "dump-toolkit-registry.py"
    if not dumper_script.exists():
        return -1, "dump-toolkit-registry.py not found in shortcutkit directory"

    pathlib.Path(logs_dir).mkdir(parents=True, exist_ok=True)
    stdout_log = pathlib.Path(logs_dir) / "toolkit.stdout.log"
    stderr_log = pathlib.Path(logs_dir) / "toolkit.stderr.log"

    cmd = [sys.executable, str(dumper_script), str(data_dir)]
    res = subprocess.run(cmd, capture_output=True, text=True, check=False)

    stdout_log.write_text(res.stdout, encoding="utf-8")
    stderr_log.write_text(res.stderr, encoding="utf-8")

    return res.returncode, res.stderr.strip()


def inspect_sqlite_database(db_path, dump_exit_code):
    summary = {
        "toolkitDb": os.path.basename(db_path) if db_path else None,
        "failures": []
    }

    if dump_exit_code != 0:
        summary["failures"].append(f"dump-toolkit-registry exit {dump_exit_code}")

    if not db_path or not os.path.exists(db_path):
        summary["failures"].append("ToolKit DB not found")
        return summary

    con = None
    try:
        con = sqlite3.connect(f"file:{pathlib.Path(db_path).resolve().as_posix()}?mode=ro", uri=True)
        tables = {x[0] for x in con.execute("SELECT name FROM sqlite_master WHERE type='table'")}

        for t in ["Tools", "Parameters", "ToolParameterTypes", "ToolOutputTypes", "EnumerationCases"]:
            if t in tables:
                summary[t] = con.execute(f'SELECT count(*) FROM "{t}"').fetchone()[0]
            else:
                summary[t] = 0

        if "Tools" in tables:
            providers = {}
            for p, n in con.execute("SELECT sourceActionProvider, count(*) FROM Tools GROUP BY sourceActionProvider"):
                providers[str(p or "(null)")] = n
            summary["providers"] = providers

            vis = con.execute("SELECT count(*) FROM Tools WHERE visibilityFlags != 0").fetchone()[0]
            summary["visible"] = vis
            summary["hidden"] = summary["Tools"] - vis
            summary["uniqueIdentifiers"] = con.execute("SELECT count(distinct id) FROM Tools").fetchone()[0]
            summary["duplicateIdentifiers"] = [x[0] for x in con.execute("SELECT id FROM Tools GROUP BY id HAVING count(*) > 1")]

        if "ContainerMetadata" in tables:
            total_containers = con.execute("SELECT count(*) FROM ContainerMetadata").fetchone()[0]
            apple_containers = con.execute("SELECT count(*) FROM ContainerMetadata WHERE id LIKE 'com.apple.%'").fetchone()[0]
            summary["containers"] = total_containers
            summary["appleContainers"] = apple_containers
            summary["thirdPartyContainers"] = total_containers - apple_containers

        if "Tools" in tables and "ContainerMetadata" in tables:
            # Check if sourceContainerId column exists in Tools
            cols = {col[1] for col in con.execute("PRAGMA table_info(Tools)")}
            if "sourceContainerId" in cols:
                apple_app_intents = con.execute(
                    "SELECT count(*) FROM Tools t JOIN ContainerMetadata cm ON cm.rowId=t.sourceContainerId "
                    "WHERE t.sourceActionProvider='WFLinkActionProvider' AND cm.id LIKE 'com.apple.%' AND t.visibilityFlags != 0"
                ).fetchone()[0]
                summary["appleAppIntents"] = apple_app_intents
            else:
                summary["appleAppIntents"] = 0
    except Exception as e:
        summary["failures"].append(f"SQLite inspection error: {e}")
    finally:
        if con:
            con.close()

    return summary


def find_default_toolkit_db():
    pattern = os.path.expanduser("~/Library/Shortcuts/ToolKit/Tools-prod.*.sqlite")
    files = sorted(glob.glob(pattern))
    return files[-1] if files else None


def main():
    parser = argparse.ArgumentParser(description="Extracts and summarizes ToolKit registry SQLite DB")
    parser.add_argument("--data-dir", default="out/data", help="Output data directory")
    parser.add_argument("--logs-dir", default="out/logs", help="Logs directory")
    parser.add_argument("--shortcutkit-dir", default="out/shortcutkit", help="ShortcutKit repo directory")
    parser.add_argument("--db-path", help="Explicit path to Tools-prod.*.sqlite")
    parser.add_argument("--skip-dump", action="store_true", help="Skip running dump-toolkit-registry.py")
    args = parser.parse_args()

    data_dir = pathlib.Path(args.data_dir)
    data_dir.mkdir(parents=True, exist_ok=True)

    dump_code = 0
    if not args.skip_dump:
        dump_code, _ = run_toolkit_dumper(args.shortcutkit_dir, args.data_dir, args.logs_dir)

    db_file = args.db_path or find_default_toolkit_db()
    summary = inspect_sqlite_database(db_file, dump_code)

    out_file = data_dir / "toolkit-summary.json"
    out_file.write_text(json.dumps(summary, indent=2, sort_keys=True), encoding="utf-8")
    print(json.dumps(summary, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
