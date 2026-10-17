#!/usr/bin/env python3
"""tools/p10r3/verify_ledger_consistency.py

Verifies mathematical and taxonomic consistency across:
- docs/apple-runtime/p10/p10-action-ledger.json (authoritative source of truth)
- docs/apple-runtime/p10/p10-coverage-report.md
- docs/apple-runtime/p10/p10r2-validation-results.json
"""

import argparse
import collections
import json
import pathlib
import re
import sys

REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent.parent


def check_consistency(ledger_path, report_path, results_path, mutate_count=False):
    errors = []

    # 1. Authoritative Ledger
    if not ledger_path.exists():
        return False, [f"Ledger file not found: {ledger_path}"]
    with open(ledger_path, "r", encoding="utf-8") as f:
        ledger = json.load(f)

    ledger_total = len(ledger)
    status_counts = collections.Counter(r.get("status") for r in ledger)
    
    if ledger_total != 2396:
        errors.append(f"Expected 2,396 total ledger rows, got {ledger_total}")

    expected_verified = status_counts.get("COVERED_VERIFIED", 0)
    expected_partial = status_counts.get("COVERED_PARTIAL", 0)

    # 2. Coverage Report Markdown
    if not report_path.exists():
        errors.append(f"Coverage report file not found: {report_path}")
    else:
        text = report_path.read_text(encoding="utf-8")
        if mutate_count:
            # Simulate a report where COVERED_VERIFIED was tampered to 389
            text = text.replace("| `COVERED_VERIFIED` | 0 |", "| `COVERED_VERIFIED` | 389 |")

        # Parse total raw records
        m_tot = re.search(r"\*\*Total Raw Discovered Records\*\*\s*\|\s*(\d+)", text)
        if not m_tot or int(m_tot.group(1)) != ledger_total:
            errors.append(f"Report total records mismatch: found {m_tot.group(1) if m_tot else 'none'}, expected {ledger_total}")

        # Parse COVERED_VERIFIED and COVERED_PARTIAL
        m_ver = re.search(r"\|\s*`COVERED_VERIFIED`\s*\|\s*(\d+)\s*\|", text)
        m_part = re.search(r"\|\s*`COVERED_PARTIAL`\s*\|\s*(\d+)\s*\|", text)

        if not m_ver or int(m_ver.group(1)) != expected_verified:
            errors.append(f"Report COVERED_VERIFIED mismatch: report has {m_ver.group(1) if m_ver else 'none'}, ledger has {expected_verified}")

        if not m_part or int(m_part.group(1)) != expected_partial:
            errors.append(f"Report COVERED_PARTIAL mismatch: report has {m_part.group(1) if m_part else 'none'}, ledger has {expected_partial}")

        # Verify all individual status counts match
        for st, count in status_counts.items():
            pattern = rf"\|\s*`{re.escape(st)}`\s*\|\s*(\d+)\s*\|"
            m = re.search(pattern, text)
            if not m:
                errors.append(f"Status {st} missing from report Markdown table")
            elif int(m.group(1)) != count:
                errors.append(f"Status {st} count mismatch in report: {m.group(1)} != ledger {count}")

    # 3. Validation Results JSON
    if results_path.exists():
        with open(results_path, "r", encoding="utf-8") as f:
            res = json.load(f)
        metrics = res.get("catalogMetrics", {})
        if metrics:
            if metrics.get("ledgerTotalRecords") != ledger_total:
                errors.append(f"Validation results ledgerTotalRecords mismatch: {metrics.get('ledgerTotalRecords')} != {ledger_total}")
            if metrics.get("ledgerCoveredVerified") != expected_verified:
                errors.append(f"Validation results ledgerCoveredVerified mismatch: {metrics.get('ledgerCoveredVerified')} != {expected_verified}")
            if metrics.get("ledgerCoveredPartial") != expected_partial:
                errors.append(f"Validation results ledgerCoveredPartial mismatch: {metrics.get('ledgerCoveredPartial')} != {expected_partial}")

    return len(errors) == 0, errors


def main():
    parser = argparse.ArgumentParser(description="Check consistency between ledger, report, and results")
    parser.add_argument("--mutate-check", action="store_true", help="Simulate a tampered report to prove fail-closed detection")
    args = parser.parse_args()

    ledger_path = REPO_ROOT / "docs" / "apple-runtime" / "p10" / "p10-action-ledger.json"
    report_path = REPO_ROOT / "docs" / "apple-runtime" / "p10" / "p10-coverage-report.md"
    results_path = REPO_ROOT / "docs" / "apple-runtime" / "p10" / "p10r2-validation-results.json"

    if args.mutate_check:
        ok, errors = check_consistency(ledger_path, report_path, results_path, mutate_count=True)
        if not ok:
            print(f"SUCCESS: Tampered report correctly failed consistency check: {errors}")
            sys.exit(0)
        else:
            print("ERROR: Tampered report unexpectedly passed consistency check!", file=sys.stderr)
            sys.exit(1)

    ok, errors = check_consistency(ledger_path, report_path, results_path, mutate_count=False)
    if not ok:
        print("ERROR: Inconsistencies detected across ledger, report, and results:", file=sys.stderr)
        for e in errors:
            print(f"  - {e}", file=sys.stderr)
        sys.exit(1)

    print("SUCCESS: All counts across p10-action-ledger.json, p10-coverage-report.md, and validation results are 100% consistent.")
    sys.exit(0)


if __name__ == "__main__":
    main()
