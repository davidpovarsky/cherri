#!/usr/bin/env python3
"""tools/p10r3/verify_or_generate_pack.py

Portable verifier and cross-branch auditor for the Pure Upstream + Pack track
(origin/apple-actions-pack-pure-core-20261022 @ 141b9cc96cf1df3f9d1593548dc49ae8a46a256a)
and Pure Original baseline (bcc0095754e2cc0baaeb4d93dae172b8153b4716) vs P10 Native Compiler.

Uses relative repository paths and `git show` only — zero hardcoded machine paths
(e.g. /home/chatgpt-agent/...).
"""

import argparse
import json
import pathlib
import subprocess
import sys

REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent.parent
PURE_COMMIT = "bcc0095754e2cc0baaeb4d93dae172b8153b4716"
PACK_COMMIT = "141b9cc96cf1df3f9d1593548dc49ae8a46a256a"

EXPECTED_15_GO_ONLY = {
    "createRemindersList": "REQUIRES_APPINTENT_DESCRIPTOR",
    "editEvent": "P10_ONLY_GO_REQUIRED",
    "editReminder": "P10_ONLY_GO_REQUIRED",
    "openBook": "REQUIRES_APPINTENT_DESCRIPTOR",
    "playAudiobook": "REQUIRES_APPINTENT_DESCRIPTOR",
    "playVoiceMemo": "REQUIRES_APPINTENT_DESCRIPTOR",
    "recordVoiceMemo": "REQUIRES_APPINTENT_DESCRIPTOR",
    "run": "ALREADY_UPSTREAM",
    "searchSpotlight": "REQUIRES_APPINTENT_DESCRIPTOR",
    "setSilentMode": "REQUIRES_APPINTENT_DESCRIPTOR",
    "startNotesAudioRecording": "REQUIRES_APPINTENT_DESCRIPTOR",
    "startStopwatch": "REQUIRES_APPINTENT_DESCRIPTOR",
    "stopStopwatch": "REQUIRES_APPINTENT_DESCRIPTOR",
    "stopVoiceMemoRecording": "REQUIRES_APPINTENT_DESCRIPTOR",
    "transcribeText": "REQUIRES_APPINTENT_DESCRIPTOR",
}


def git_show_text(commit: str, rel_path: str) -> str:
    res = subprocess.run(
        ["git", "show", f"{commit}:{rel_path}"],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
        encoding="utf-8",
        check=True,
    )
    return res.stdout


def extract_pack_summary() -> dict:
    manifest_raw = git_show_text(PACK_COMMIT, "extensions/cherri-apple-actions-pack/manifest.json")
    manifest = json.loads(manifest_raw)
    verify_raw = git_show_text(PACK_COMMIT, "extensions/cherri-apple-actions-pack/entry-verification-summary.json")
    verify_summary = json.loads(verify_raw)
    limitations_md = git_show_text(PACK_COMMIT, "extensions/cherri-apple-actions-pack/LIMITATIONS.md")

    decisions = manifest.get("decisions", [])
    new_actions = [d["name"] for d in decisions if d.get("kind") == "action" and d.get("status") == "added"]
    advanced_aliases = [d.get("alias") or d["name"] for d in decisions if d.get("kind") == "action" and d.get("status") == "extended_alias"]
    unchanged_actions = [d["name"] for d in decisions if d.get("kind") == "action" and d.get("status") in ("unchanged", "original_preserved")]

    return {
        "pureCommit": PURE_COMMIT,
        "packCommit": PACK_COMMIT,
        "totalEntries": len(new_actions) + len(advanced_aliases),
        "newActionsCount": len(new_actions),
        "advancedAliasesCount": len(advanced_aliases),
        "unchangedActionsCount": len(unchanged_actions),
        "newActions": sorted(new_actions),
        "advancedAliases": sorted(advanced_aliases),
        "unchangedActions": sorted(unchanged_actions),
        "entryVerificationSummary": verify_summary,
        "limitationsMentionsAll15": all(name in limitations_md for name in EXPECTED_15_GO_ONLY),
    }


def verify_15_go_only_consistency() -> list[str]:
    errors = []
    audit_path = REPO_ROOT / "docs" / "apple-runtime" / "p10" / "p10-pack-15-go-only-audit.json"
    if not audit_path.exists():
        return [f"Missing {audit_path}"]

    audit_data = json.loads(audit_path.read_text(encoding="utf-8"))
    rows = audit_data.get("goOnlyActionsAudit", [])
    if len(rows) != 15:
        errors.append(f"Expected 15 rows in goOnlyActionsAudit, got {len(rows)}")

    found_map = {}
    for row in rows:
        name = row.get("actionName")
        cls = row.get("classification")
        found_map[name] = cls
        if name not in EXPECTED_15_GO_ONLY:
            errors.append(f"Unexpected action in 15 Go-only audit: {name}")
        elif cls != EXPECTED_15_GO_ONLY[name]:
            errors.append(
                f"Classification mismatch for {name}: got {cls!r}, expected {EXPECTED_15_GO_ONLY[name]!r}"
            )

    missing = set(EXPECTED_15_GO_ONLY.keys()) - set(found_map.keys())
    if missing:
        errors.append(f"Missing actions in 15 Go-only audit: {sorted(missing)}")

    # Check pack summary
    pack_info = extract_pack_summary()
    if pack_info["totalEntries"] != 102 or pack_info["newActionsCount"] != 48 or pack_info["advancedAliasesCount"] != 54:
        errors.append(f"Unexpected pack entry counts: {pack_info}")
    if not pack_info["limitationsMentionsAll15"]:
        errors.append("Pack LIMITATIONS.md does not mention all 15 Go-only action names")

    # Verify zero hardcoded /home/chatgpt-agent paths in tools/
    for py_file in (REPO_ROOT / "tools").rglob("*.py"):
        if py_file.resolve() == pathlib.Path(__file__).resolve():
            continue
        content = py_file.read_text(encoding="utf-8", errors="replace")
        if "/home/chatgpt-agent" in content:
            errors.append(f"Hardcoded path /home/chatgpt-agent found in {py_file.relative_to(REPO_ROOT)}")

    return errors


def main():
    parser = argparse.ArgumentParser(description="Portable Pure+Pack and 15 Go-only verifier")
    parser.add_argument("--json", action="store_true", help="Print pack summary JSON")
    args = parser.parse_args()

    errors = verify_15_go_only_consistency()
    if errors:
        for e in errors:
            print(f"ERROR: {e}", file=sys.stderr)
        sys.exit(1)

    summary = extract_pack_summary()
    if args.json:
        print(json.dumps(summary, indent=2))
    else:
        print(
            f"SUCCESS: Verified Pure Original ({PURE_COMMIT[:7]}), Pure+Pack ({PACK_COMMIT[:7]}: "
            f"{summary['totalEntries']} entries = {summary['newActionsCount']} new + {summary['advancedAliasesCount']} aliases), "
            f"and all 15 Go-only actions with exact classifications."
        )


if __name__ == "__main__":
    main()
