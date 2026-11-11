#!/usr/bin/env python3
"""
tools/p10r7/reconcile_private_and_sanitized_reports.py

Cross-checks, synchronizes (with --sync when local private inputs exist), and fails closed
on any drift or integrity violation across:
  - Private P10R7 audit output (when present locally)
  - docs/apple-runtime/p10r7/p10r7-38-sanitized-matrix.json
  - docs/apple-runtime/p10r7/p10r7-sanitized-diff-matrix.json
  - docs/apple-runtime/p10r7/p10r7-baseline-sanitized.json
  - docs/apple-runtime/p10r7/p10r7-baseline-reconciliation.md
  - docs/apple-runtime/p10r7/p10r7-final-audit.md
  - docs/apple-runtime/p10r6/p10r6-38-sanitized-matrix.json
  - docs/apple-runtime/p10r6/p10r6-38-corpus-summary.md
  - docs/apple-runtime/p10r6/p10r6-final-audit.md
  - docs/apple-runtime/p10r6/p10r6-corpus-privacy-review.md

Flags:
  --sync          Recompute p10r7-sanitized-diff-matrix.json (from private input/ & p10r7-work/),
                  synchronize p10r7-baseline-reconciliation.md specimen rows, and reconcile
                  p10r6-38-sanitized-matrix.json false PASS claims while preserving counts.
  --verify        Verify all committed reports and (if present locally) private <-> sanitized parity.
  --mutate-check  Run adversarial tamper mutations (total drift, false PASS, blocked device spoof,
                  scrambled markdown rows, private path leak) to prove fail-closed enforcement.
"""

from __future__ import annotations

import argparse
import collections
import copy
import json
import pathlib
import plistlib
import re
import sys
from typing import Any

REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent.parent
P10R7_DIR = REPO_ROOT / "docs" / "apple-runtime" / "p10r7"
P10R6_DIR = REPO_ROOT / "docs" / "apple-runtime" / "p10r6"

DEFAULT_AUDIT_ROOT = REPO_ROOT.parent / "cherri-shortcut-audit-20261110"
DEFAULT_PRIVATE_JSON = DEFAULT_AUDIT_ROOT / "p10r7-private" / "p10r7-audit-results.json"
DEFAULT_INPUT_DIR = DEFAULT_AUDIT_ROOT / "input"
DEFAULT_WORK_DIR = DEFAULT_AUDIT_ROOT / "p10r7-work"

FORBIDDEN_SANITIZED_MARKERS = (
    "cherri-shortcut-audit-",
    "C:\\Users\\",
    "C:/Users/",
    "/Users/",
)

SPECIMEN_IDS = [f"{i:03d}" for i in range(1, 39)]


def load_json(path: pathlib.Path) -> Any:
    return json.loads(path.read_text(encoding="utf-8"))


def save_json(path: pathlib.Path, data: Any) -> None:
    path.write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")


def load_plist_or_json(path: pathlib.Path) -> dict[str, Any]:
    raw = path.read_bytes()
    try:
        obj = plistlib.loads(raw)
        if isinstance(obj, dict):
            return obj
    except Exception:
        pass
    obj = json.loads(raw.decode("utf-8"))
    if not isinstance(obj, dict):
        raise ValueError(f"Expected dict in {path}")
    return obj


def header_keys(doc: dict[str, Any]) -> set[str]:
    return {k for k in doc.keys() if k != "WFWorkflowActions"}


def action_counts(doc: dict[str, Any]) -> collections.Counter[str]:
    acts = doc.get("WFWorkflowActions") or []
    return collections.Counter(
        a.get("WFWorkflowActionIdentifier", "") for a in acts if isinstance(a, dict)
    )


def build_sanitized_diff_matrix(
    input_dir: pathlib.Path,
    work_dir: pathlib.Path,
    sanitized_matrix: dict[str, Any],
) -> list[dict[str, Any]]:
    rows_by_id = {r["specimenId"]: r for r in sanitized_matrix.get("matrix", [])}
    out: list[dict[str, Any]] = []
    for sid in SPECIMEN_IDS:
        src_path = input_dir / f"{sid}.plist"
        if not src_path.exists():
            alt = input_dir / f"{sid}.json"
            if alt.exists():
                src_path = alt
        cmp_path = work_dir / f"{sid}_legacy.shortcut"
        if not src_path.exists() or not cmp_path.exists():
            raise FileNotFoundError(f"Missing specimen files for {sid}: {src_path} / {cmp_path}")

        src_doc = load_plist_or_json(src_path)
        cmp_doc = load_plist_or_json(cmp_path)
        src_c = action_counts(src_doc)
        cmp_c = action_counts(cmp_doc)

        added: dict[str, int] = {}
        removed: dict[str, int] = {}
        for ident in sorted(set(src_c) | set(cmp_c)):
            delta = cmp_c[ident] - src_c[ident]
            if delta > 0:
                added[ident] = delta
            elif delta < 0:
                removed[ident] = -delta

        src_hdr = header_keys(src_doc)
        cmp_hdr = header_keys(cmp_doc)
        san_row = rows_by_id[sid]
        src_total = sum(src_c.values())
        cmp_total = sum(cmp_c.values())
        if src_total != san_row["sourceActions"] or cmp_total != san_row["compiledActions"]:
            raise ValueError(
                f"Action count mismatch during diff-matrix sync on {sid}: "
                f"disk=({src_total},{cmp_total}) vs sanitized=({san_row['sourceActions']},{san_row['compiledActions']})"
            )

        out.append(
            {
                "specimenId": sid,
                "sourceActionCount": src_total,
                "compiledActionCount": cmp_total,
                "actionDelta": cmp_total - src_total,
                "strictSequenceMatch": san_row["gates"]["STRICT_ACTION_SEQUENCE_PASS"] == "PASS",
                "addedActionCounts": added,
                "removedActionCounts": removed,
                "missingHeaderKeys": sorted(src_hdr - cmp_hdr),
                "extraHeaderKeys": sorted(cmp_hdr - src_hdr),
            }
        )
    return out


def render_baseline_reconciliation_md(
    baseline_rows: list[dict[str, Any]],
    p10r6_matrix: list[dict[str, Any]],
    p10r7_sanitized: dict[str, Any],
) -> str:
    base_by_id = {r["specimenId"]: r for r in baseline_rows}
    r6_by_id = {r["sourceFileId"]: r for r in p10r6_matrix}
    r7_by_id = {r["specimenId"]: r for r in p10r7_sanitized["matrix"]}

    total_src = sum(r["sourceActionCount"] for r in baseline_rows)
    total_r6 = sum(r["p10r6CompiledActionCount"] for r in baseline_rows)
    total_r7 = p10r7_sanitized["totalCompiledActions"]
    r6_seq = sum(1 for r in baseline_rows if r["strictSequenceMatch"])
    r7_seq = p10r7_sanitized["strictActionSequencePassCount"]

    lines = [
        "# Cherri P10R7 — P10R6 Baseline Reconciliation & Metric Integrity",
        "",
        "## 1. Objective",
        "",
        "Establish a single authoritative reconciliation across all 38 real-world specimens (`001`–`038`) connecting:",
        "- Canonical P10R6 sanitized baseline (`docs/apple-runtime/p10r6/p10r6-38-sanitized-matrix.json` & `docs/apple-runtime/p10r7/p10r7-baseline-sanitized.json`)",
        "- Verified P10R7 post-fix audit (`docs/apple-runtime/p10r7/p10r7-38-sanitized-matrix.json` & `p10r7-sanitized-diff-matrix.json`)",
        "",
        "Every number below is computed directly by summing the verified `001`–`038` JSON rows (no manual rounding, placeholder rows, or stale copy-pasted totals).",
        "",
        "---",
        "",
        "## 2. Historical Metric Discrepancy Audit & Correction",
        "",
        "During P10R7 baseline verification, three concrete accounting errors were identified in P10R6 / early P10R7 artifacts and corrected permanently:",
        "",
        "1. **Sum Mismatch in P10R6 Prose (`28,103` / `35,016` vs `27,652` / `34,355`):**",
        "   - Summing the authoritative `001`–`038` JSON matrix rows yields **`27,652` source actions** and **`34,355` compiled actions** (`+6,703` net action bloat in P10R6).",
        "   - `docs/apple-runtime/p10r6/p10r6-38-corpus-summary.md` additionally had a copy-paste cell typo on specimens `018` and `021` (`3,383` instead of `3,381`, accounting for `+4`), while narrative summaries in `p10r6-38-corpus-summary.md`, `p10r6-final-audit.md`, and `p10r6-corpus-privacy-review.md` quoted `28,103` and `35,016`.",
        "2. **Tautological P10R6 Oracle Hardcoding (`38/38` vs `3/38` & `0/38`):**",
        "   - `tools/p10r6/corpus_audit.py` originally hardcoded `ACTION_SEQUENCE_PASS = \"PASS\"` and `FULL_WORKFLOW_SEMANTIC_PASS = \"PASS\"` without invoking a structural plist comparator.",
        f"   - Under the independent strict oracle (`tools/p10r7/independent_fidelity_oracle.py`), only **`{r6_seq}/38`** specimens preserved strict action sequence in the P10R6 baseline (rising to **`{r7_seq}/38`** in P10R7), net corpus action bloat fell from **`+{total_r6 - total_src:,}`** to **`+{total_r7 - total_src:,}`** (`{total_src:,} -> {total_r7:,}`), and **`0/38`** reach `FULL_WORKFLOW_SEMANTIC_PASS`.",
        "   - `tools/p10r6/corpus_audit.py` has been updated to delegate sequence and semantic gates directly to `independent_fidelity_oracle.evaluate_shortcut_fidelity`, and `p10r6-38-sanitized-matrix.json` & P10R6 prose reports have been reconciled.",
        "3. **Initial P10R7 Reconciliation Table Row Scrambling:**",
        "   - An earlier draft of this file repeated specimen counts (`001=338/390`, `002=382/448`, `036=338/390`) that conflicted with `p10r7-baseline-sanitized.json` (`001=162/192`, `002=205/256`). All 38 rows below are machine-generated directly from the authoritative JSON matrices and enforced by `tools/p10r7/reconcile_private_and_sanitized_reports.py --verify`.",
        "",
        "---",
        "",
        "## 3. Reconciled Totals (`001`–`038` Sum of Rows)",
        "",
        "| Metric | P10R6 Baseline (Authoritative Matrix Sum) | P10R7 Final (Strict Oracle) | Change |",
        "| --- | ---: | ---: | ---: |",
        f"| **Total Source Actions** | {total_src:,} | {total_src:,} | 0 |",
        f"| **Total Compiled Actions (Legacy)** | {total_r6:,} | {total_r7:,} | {total_r7 - total_r6:+,} |",
        f"| **Net Action Bloat / Delta** | {total_r6 - total_src:+,} ({((total_r6 - total_src) / total_src * 100):.2f}%) | {total_r7 - total_src:+,} ({((total_r7 - total_src) / total_src * 100):.3f}%) | {total_r7 - total_r6:+,} |",
        f"| **Strict Action Sequence Pass (`STRICT_ACTION_SEQUENCE_PASS`)** | {r6_seq} / 38 | {r7_seq} / 38 | +{r7_seq - r6_seq} |",
        f"| **Full Semantic Parity Pass (`FULL_WORKFLOW_SEMANTIC_PASS`)** | 0 / 38 (hardcoded 38/38 retracted) | {p10r7_sanitized['fullWorkflowSemanticPassCount']} / 38 | Honest oracle |",
        f"| **Physical Apple Device Execution (`APPLE_RUNTIME_EXECUTED`)** | 0 / 38 (`BLOCKED_DEVICE`) | 0 / 38 (`BLOCKED_DEVICE`) | Hardware unavailable |",
        "",
        "---",
        "",
        "## 4. Specimen-by-Specimen Reconciliation (`001`–`038`)",
        "",
        "| ID | Source Actions | P10R6 Compiled | P10R6 Delta | P10R6 Seq | P10R7 Compiled | P10R7 Delta | P10R7 Seq |",
        "| :---: | ---: | ---: | ---: | :---: | ---: | ---: | :---: |",
    ]

    for sid in SPECIMEN_IDS:
        b = base_by_id[sid]
        r6 = r6_by_id[sid]
        r7 = r7_by_id[sid]
        assert b["sourceActionCount"] == r6["sourceActions"] == r7["sourceActions"]
        assert b["p10r6CompiledActionCount"] == r6["compiledActions"]
        r6_seq_str = "PASS" if b["strictSequenceMatch"] else "FAIL"
        r7_seq_str = r7["gates"]["STRICT_ACTION_SEQUENCE_PASS"]
        lines.append(
            f"| {sid} | {b['sourceActionCount']:,} | {b['p10r6CompiledActionCount']:,} | {b['actionDelta']:+d} | {r6_seq_str} | {r7['compiledActions']:,} | {r7['actionDelta']:+d} | {r7_seq_str} |"
        )

    lines.extend(
        [
            f"| **TOTAL** | **{total_src:,}** | **{total_r6:,}** | **{total_r6 - total_src:+,}** | **{r6_seq}/38** | **{total_r7:,}** | **{total_r7 - total_src:+,}** | **{r7_seq}/38** |",
            "",
        ]
    )
    return "\n".join(lines)


def reconcile_p10r6_matrix(
    p10r6_matrix: list[dict[str, Any]], baseline_rows: list[dict[str, Any]]
) -> list[dict[str, Any]]:
    """Retain P10R6 historical counts while retracting tautological PASS claims on sequence/semantic gates."""
    base_by_id = {r["specimenId"]: r for r in baseline_rows}
    updated = copy.deepcopy(p10r6_matrix)
    for row in updated:
        sid = row["sourceFileId"]
        base = base_by_id[sid]
        gates = row["gates"]
        gates["ACTION_SEQUENCE_PASS"] = "PASS" if base["strictSequenceMatch"] else "FAIL"
        gates["FULL_WORKFLOW_SEMANTIC_PASS"] = "FAIL"
        if isinstance(row.get("errors"), dict) and "compile_v2" in row["errors"]:
            row["errors"]["compile_v2"] = re.sub(
                r"[A-Za-z]:\\Users\\[^\\]+\\Code\\cherri-shortcut-audit-[0-9]+\\",
                "<PRIVATE_AUDIT_ROOT>\\\\",
                row["errors"]["compile_v2"],
            )
    return updated


def check_no_forbidden_markers(obj: Any, label: str, errors: list[str]) -> None:
    text = json.dumps(obj, ensure_ascii=False)
    for marker in FORBIDDEN_SANITIZED_MARKERS:
        if marker in text:
            errors.append(f"Privacy leak: forbidden marker {marker!r} found in {label}")


def verify_all(
    p10r7_sanitized: dict[str, Any],
    p10r7_diffs: list[dict[str, Any]],
    p10r7_baseline: list[dict[str, Any]],
    p10r6_matrix: list[dict[str, Any]],
    baseline_md_text: str,
    private_audit_rows: list[dict[str, Any]] | None = None,
) -> list[str]:
    errors: list[str] = []

    check_no_forbidden_markers(p10r7_sanitized, "p10r7-38-sanitized-matrix.json", errors)
    check_no_forbidden_markers(p10r7_diffs, "p10r7-sanitized-diff-matrix.json", errors)
    check_no_forbidden_markers(p10r7_baseline, "p10r7-baseline-sanitized.json", errors)
    check_no_forbidden_markers(p10r6_matrix, "p10r6-38-sanitized-matrix.json", errors)

    r7_rows = p10r7_sanitized.get("matrix", [])
    if len(r7_rows) != 38 or [r["specimenId"] for r in r7_rows] != SPECIMEN_IDS:
        errors.append("p10r7-38-sanitized-matrix.json must contain exact specimens 001..038")
        return errors

    if len(p10r7_diffs) != 38 or [r["specimenId"] for r in p10r7_diffs] != SPECIMEN_IDS:
        errors.append("p10r7-sanitized-diff-matrix.json must contain exact specimens 001..038")
        return errors

    if len(p10r7_baseline) != 38 or [r["specimenId"] for r in p10r7_baseline] != SPECIMEN_IDS:
        errors.append("p10r7-baseline-sanitized.json must contain exact specimens 001..038")
        return errors

    if len(p10r6_matrix) != 38 or [r["sourceFileId"] for r in p10r6_matrix] != SPECIMEN_IDS:
        errors.append("p10r6-38-sanitized-matrix.json must contain exact specimens 001..038")
        return errors

    # Verify sums & fail-closed oracle invariants across P10R7 sanitized matrix & diff matrix
    sum_src = sum(r["sourceActions"] for r in r7_rows)
    sum_cmp = sum(r["compiledActions"] for r in r7_rows)
    sum_delta = sum(r["actionDelta"] for r in r7_rows)

    if p10r7_sanitized.get("totalSourceActions") != sum_src:
        errors.append(f"P10R7 totalSourceActions drift: header={p10r7_sanitized.get('totalSourceActions')} vs row_sum={sum_src}")
    if p10r7_sanitized.get("totalCompiledActions") != sum_cmp:
        errors.append(f"P10R7 totalCompiledActions drift: header={p10r7_sanitized.get('totalCompiledActions')} vs row_sum={sum_cmp}")
    if p10r7_sanitized.get("totalActionDelta") != sum_delta or sum_delta != (sum_cmp - sum_src):
        errors.append(f"P10R7 totalActionDelta drift: header={p10r7_sanitized.get('totalActionDelta')} vs row_sum={sum_delta}")

    diff_by_id = {r["specimenId"]: r for r in p10r7_diffs}
    base_by_id = {r["specimenId"]: r for r in p10r7_baseline}
    r6_by_id = {r["sourceFileId"]: r for r in p10r6_matrix}

    gate_counts = collections.Counter()
    for row in r7_rows:
        sid = row["specimenId"]
        diff_row = diff_by_id[sid]
        base_row = base_by_id[sid]
        r6_row = r6_by_id[sid]
        gates = row["gates"]

        if row["actionDelta"] != row["compiledActions"] - row["sourceActions"]:
            errors.append(f"P10R7 {sid}: actionDelta != compiledActions - sourceActions")

        if row["sourceSha256"] != base_row["sourceSha256"] or row["sourceSha256"] != r6_row["sha256"]:
            errors.append(f"Specimen {sid}: source SHA-256 mismatch across P10R7/baseline/P10R6")

        if row["sourceActions"] != base_row["sourceActionCount"] or row["sourceActions"] != r6_row["sourceActions"]:
            errors.append(f"Specimen {sid}: sourceActionCount mismatch across P10R7/baseline/P10R6")

        if diff_row["sourceActionCount"] != row["sourceActions"] or diff_row["compiledActionCount"] != row["compiledActions"] or diff_row["actionDelta"] != row["actionDelta"]:
            errors.append(f"Specimen {sid}: p10r7-sanitized-diff-matrix counts mismatch p10r7-38-sanitized-matrix")

        net_from_families = sum(diff_row["addedActionCounts"].values()) - sum(diff_row["removedActionCounts"].values())
        if net_from_families != diff_row["actionDelta"]:
            errors.append(f"Specimen {sid}: diff-matrix added-removed ({net_from_families}) != actionDelta ({diff_row['actionDelta']})")

        if diff_row["strictSequenceMatch"] != (gates.get("STRICT_ACTION_SEQUENCE_PASS") == "PASS"):
            errors.append(f"Specimen {sid}: diff-matrix strictSequenceMatch mismatches gate STRICT_ACTION_SEQUENCE_PASS")

        # Anti-tamper oracle invariants (M31-M33 equivalent guards)
        if row["actionDelta"] != 0 and gates.get("STRICT_ACTION_SEQUENCE_PASS") == "PASS":
            errors.append(f"Specimen {sid}: STRICT_ACTION_SEQUENCE_PASS cannot be PASS when actionDelta={row['actionDelta']}")
        if row["actionDelta"] != 0 and gates.get("FULL_WORKFLOW_SEMANTIC_PASS") == "PASS":
            errors.append(f"Specimen {sid}: FULL_WORKFLOW_SEMANTIC_PASS cannot be PASS when actionDelta={row['actionDelta']}")
        if gates.get("STRICT_ACTION_SEQUENCE_PASS") != "PASS" and gates.get("FULL_WORKFLOW_SEMANTIC_PASS") == "PASS":
            errors.append(f"Specimen {sid}: FULL_WORKFLOW_SEMANTIC_PASS cannot pass when STRICT_ACTION_SEQUENCE_PASS fails")
        if gates.get("ACTION_PARAMETER_WIRE_PASS") != "PASS" and gates.get("FULL_WORKFLOW_SEMANTIC_PASS") == "PASS":
            errors.append(f"Specimen {sid}: FULL_WORKFLOW_SEMANTIC_PASS cannot pass when ACTION_PARAMETER_WIRE_PASS fails")
        if gates.get("APPLE_RUNTIME_EXECUTED") != "BLOCKED_DEVICE":
            errors.append(f"Specimen {sid}: APPLE_RUNTIME_EXECUTED must remain BLOCKED_DEVICE without signed hardware run")

        for g, val in gates.items():
            if val == "PASS":
                gate_counts[g] += 1

    # Summary gate counts match row tallies
    if p10r7_sanitized.get("strictActionSequencePassCount") != gate_counts["STRICT_ACTION_SEQUENCE_PASS"]:
        errors.append("P10R7 strictActionSequencePassCount mismatches row gate tally")
    if p10r7_sanitized.get("fullWorkflowSemanticPassCount") != gate_counts["FULL_WORKFLOW_SEMANTIC_PASS"]:
        errors.append("P10R7 fullWorkflowSemanticPassCount mismatches row gate tally")
    for g, cnt in p10r7_sanitized.get("gateSummary", {}).items():
        if gate_counts[g] != cnt:
            errors.append(f"P10R7 gateSummary[{g}]={cnt} mismatches row tally {gate_counts[g]}")

    # Check P10R6 matrix counts & fail-closed gate honesty
    if sum(r["sourceActions"] for r in p10r6_matrix) != 27652:
        errors.append("P10R6 sourceActions sum must equal 27652")
    if sum(r["compiledActions"] for r in p10r6_matrix) != 34355:
        errors.append("P10R6 compiledActions sum must equal 34355")
    for r6 in p10r6_matrix:
        sid = r6["sourceFileId"]
        base = base_by_id[sid]
        if r6["compiledActions"] != base["p10r6CompiledActionCount"]:
            errors.append(f"P10R6 {sid}: compiledActions mismatch baseline")
        if r6["actionDelta"] != 0 and r6["gates"].get("ACTION_SEQUENCE_PASS") == "PASS":
            errors.append(f"P10R6 {sid}: hardcoded false PASS on ACTION_SEQUENCE_PASS remains when actionDelta={r6['actionDelta']}")
        if r6["gates"].get("FULL_WORKFLOW_SEMANTIC_PASS") == "PASS":
            errors.append(f"P10R6 {sid}: hardcoded false PASS on FULL_WORKFLOW_SEMANTIC_PASS remains")

    # Verify reconciliation Markdown matches JSON rows
    expected_md = render_baseline_reconciliation_md(p10r7_baseline, p10r6_matrix, p10r7_sanitized)
    if baseline_md_text.replace("\r\n", "\n").strip() != expected_md.replace("\r\n", "\n").strip():
        errors.append("p10r7-baseline-reconciliation.md does not match deterministic JSON reconciliation")

    # Verify private <-> sanitized 1:1 parity if local private results are supplied
    if private_audit_rows is not None:
        if len(private_audit_rows) != 38:
            errors.append(f"Private audit row count {len(private_audit_rows)} != 38")
        else:
            priv_by_id = {r["specimenId"]: r for r in private_audit_rows}
            for row in r7_rows:
                sid = row["specimenId"]
                priv = priv_by_id.get(sid)
                if not priv:
                    errors.append(f"Missing {sid} in private audit JSON")
                    continue
                if priv["sourcePlistSha256"] != row["sourceSha256"]:
                    errors.append(f"Private vs sanitized sourceSha256 mismatch on {sid}")
                if priv["compiledShortcutSha256"] != row["compiledSha256"]:
                    errors.append(f"Private vs sanitized compiledSha256 mismatch on {sid}")
                if priv["sourceActions"] != row["sourceActions"] or priv["compiledActions"] != row["compiledActions"] or priv["actionDelta"] != row["actionDelta"]:
                    errors.append(f"Private vs sanitized counts mismatch on {sid}")
                if priv["gates"] != row["gates"]:
                    errors.append(f"Private vs sanitized gates mismatch on {sid}")

    return errors


def run_mutate_check(
    p10r7_sanitized: dict[str, Any],
    p10r7_diffs: list[dict[str, Any]],
    p10r7_baseline: list[dict[str, Any]],
    p10r6_matrix: list[dict[str, Any]],
    baseline_md_text: str,
) -> bool:
    cases = []

    m1 = copy.deepcopy(p10r7_sanitized)
    m1["totalCompiledActions"] += 1
    cases.append(("summary_total_drift", m1, p10r7_diffs, p10r7_baseline, p10r6_matrix, baseline_md_text))

    m2 = copy.deepcopy(p10r7_sanitized)
    m2["matrix"][1]["gates"]["STRICT_ACTION_SEQUENCE_PASS"] = "PASS"
    cases.append(("false_sequence_pass_on_nonzero_delta", m2, p10r7_diffs, p10r7_baseline, p10r6_matrix, baseline_md_text))

    m3 = copy.deepcopy(p10r7_sanitized)
    m3["matrix"][0]["gates"]["APPLE_RUNTIME_EXECUTED"] = "PASS"
    cases.append(("spoofed_device_execution_pass", m3, p10r7_diffs, p10r7_baseline, p10r6_matrix, baseline_md_text))

    m4 = copy.deepcopy(p10r6_matrix)
    m4[0]["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"] = "PASS"
    cases.append(("reintroduced_p10r6_tautological_pass", p10r7_sanitized, p10r7_diffs, p10r7_baseline, m4, baseline_md_text))

    m5 = baseline_md_text.replace("| 001 | 162 |", "| 001 | 338 |")
    cases.append(("scrambled_markdown_specimen_row", p10r7_sanitized, p10r7_diffs, p10r7_baseline, p10r6_matrix, m5))

    m6 = copy.deepcopy(p10r7_sanitized)
    m6["matrix"][0]["sourceSha256"] = "C:\\Users\\leaked\\private.plist"
    cases.append(("privacy_path_leak_in_sanitized_matrix", m6, p10r7_diffs, p10r7_baseline, p10r6_matrix, baseline_md_text))

    all_detected = True
    for name, s7, d7, b7, r6, md in cases:
        errs = verify_all(s7, d7, b7, r6, md)
        if not errs:
            print(f"[FAIL-OPEN] Mutation {name} was NOT caught!")
            all_detected = False
        else:
            print(f"[DETECTED] {name} -> {errs[0]}")
    return all_detected


def main() -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")

    parser = argparse.ArgumentParser(description="Reconcile private & sanitized P10R6/P10R7 corpus audit reports")
    parser.add_argument("--sync", action="store_true", help="Synchronize sanitized diff matrix, baseline reconciliation MD, and P10R6 matrix from live artifacts")
    parser.add_argument("--verify", action="store_true", help="Verify cross-report consistency and invariants (fail-closed)")
    parser.add_argument("--mutate-check", action="store_true", help="Verify tamper detection on adversarial report mutations")
    parser.add_argument("--private-json", type=pathlib.Path, default=DEFAULT_PRIVATE_JSON)
    parser.add_argument("--input-dir", type=pathlib.Path, default=DEFAULT_INPUT_DIR)
    parser.add_argument("--work-dir", type=pathlib.Path, default=DEFAULT_WORK_DIR)
    args = parser.parse_args()

    san_path = P10R7_DIR / "p10r7-38-sanitized-matrix.json"
    diff_path = P10R7_DIR / "p10r7-sanitized-diff-matrix.json"
    base_path = P10R7_DIR / "p10r7-baseline-sanitized.json"
    md_path = P10R7_DIR / "p10r7-baseline-reconciliation.md"
    r6_path = P10R6_DIR / "p10r6-38-sanitized-matrix.json"

    p10r7_sanitized = load_json(san_path)
    p10r7_baseline = load_json(base_path)
    p10r6_matrix = load_json(r6_path)

    if args.sync:
        if not args.input_dir.exists() or not args.work_dir.exists():
            print(f"ERROR: --sync requires local audit directories ({args.input_dir}, {args.work_dir})", file=sys.stderr)
            return 1
        p10r7_diffs = build_sanitized_diff_matrix(args.input_dir, args.work_dir, p10r7_sanitized)
        save_json(diff_path, p10r7_diffs)
        p10r6_matrix = reconcile_p10r6_matrix(p10r6_matrix, p10r7_baseline)
        save_json(r6_path, p10r6_matrix)
        md_text = render_baseline_reconciliation_md(p10r7_baseline, p10r6_matrix, p10r7_sanitized)
        md_path.write_text(md_text, encoding="utf-8")
        print(f"Synchronized: {diff_path.name}, {r6_path.name}, {md_path.name}")
    else:
        p10r7_diffs = load_json(diff_path)
        md_text = md_path.read_text(encoding="utf-8")

    private_rows = load_json(args.private_json) if args.private_json.exists() else None
    if args.mutate_check:
        if not run_mutate_check(p10r7_sanitized, p10r7_diffs, p10r7_baseline, p10r6_matrix, md_text):
            return 1

    if args.verify or args.sync or not args.mutate_check:
        errors = verify_all(p10r7_sanitized, p10r7_diffs, p10r7_baseline, p10r6_matrix, md_text, private_rows)
        if errors:
            for e in errors:
                print(f"ERROR: {e}", file=sys.stderr)
            return 1
        scope = "private+sanitized" if private_rows is not None else "sanitized-only (private corpus absent locally)"
        print(
            f"VERIFIED ({scope}): 38/38 specimens consistent | "
            f"P10R6={sum(r['sourceActions'] for r in p10r6_matrix)}->{sum(r['compiledActions'] for r in p10r6_matrix)} | "
            f"P10R7={p10r7_sanitized['totalSourceActions']}->{p10r7_sanitized['totalCompiledActions']} "
            f"(Seq {p10r7_sanitized['strictActionSequencePassCount']}/38, Sem {p10r7_sanitized['fullWorkflowSemanticPassCount']}/38)"
        )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
