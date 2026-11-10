#!/usr/bin/env python3
"""tools/p10r3/verify_ledger_consistency.py

Verifies mathematical, taxonomic, and fingerprint consistency across all 11 P10 sources:
1. docs/apple-runtime/p10/p10-action-ledger.json (authoritative 2,396 runtime records)
2. docs/apple-runtime/p10/p10-apple-cherri-crosswalk.json
3. docs/apple-runtime/p10/p10-505-catalog-coverage-matrix.json
4. docs/apple-runtime/p10/p10-505-execution-evidence.json
5. docs/apple-runtime/p10/p10r2-validation-results.json
6. docs/apple-runtime/p10/baseline-state.json
7. docs/apple-runtime/p10/p10-coverage-report.md
8. docs/apple-runtime/p10/catalog-supported-by-compiler.md
9. docs/apple-runtime/p10/apple-discovered-not-supported.md
10. docs/apple-runtime/p10/pure-pack-portability.md
11. docs/apple-runtime/p10/task-evidence-matrix.md
"""

import argparse
import collections
import json
import pathlib
import re
import sys

REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent.parent


def check_consistency(
    p10_dir: pathlib.Path = None,
    mutate_mode: str = None,
    overrides: dict = None,
):
    if p10_dir is None:
        p10_dir = REPO_ROOT / "docs" / "apple-runtime" / "p10"
    overrides = overrides or {}
    errors = []

    def load_json(name):
        if name in overrides:
            return overrides[name]
        p = p10_dir / name
        if not p.exists():
            errors.append(f"Missing required JSON file: {p}")
            return None
        return json.loads(p.read_text(encoding="utf-8"))

    def load_text(name):
        if name in overrides:
            return overrides[name]
        p = p10_dir / name
        if not p.exists():
            errors.append(f"Missing required Markdown file: {p}")
            return None
        return p.read_text(encoding="utf-8")

    ledger = load_json("p10-action-ledger.json")
    crosswalk = load_json("p10-apple-cherri-crosswalk.json")
    matrix = load_json("p10-505-catalog-coverage-matrix.json")
    exec_ev = load_json("p10-505-execution-evidence.json")
    val_res = load_json("p10r2-validation-results.json")
    baseline = load_json("baseline-state.json")

    cov_report_md = load_text("p10-coverage-report.md")
    cat_md = load_text("catalog-supported-by-compiler.md")
    gap_md = load_text("apple-discovered-not-supported.md")
    pack_md = load_text("pure-pack-portability.md")
    task_md = load_text("task-evidence-matrix.md")

    if errors:
        return False, errors

    # Apply synthetic mutation if requested (Part C5 — 7 required negative mutations)
    if mutate_mode == "c5_1_ledger_vs_crosswalk":
        crosswalk = json.loads(json.dumps(crosswalk))
        crosswalk["denominators"]["appleCoveredVerifiedIdentifiers"] = 390
    elif mutate_mode == "c5_2_crosswalk_vs_report":
        cov_report_md = cov_report_md.replace("| `COVERED_VERIFIED` | 0 |", "| `COVERED_VERIFIED` | 389 |")
    elif mutate_mode == "c5_3_deferred_double_count":
        crosswalk = json.loads(json.dumps(crosswalk))
        crosswalk["denominators"]["appleRuntimeDiscoveredTotal"] = 2482
        crosswalk["denominators"]["appleEligibleUserFacingTotal"] = 1613
    elif mutate_mode == "c5_4_unique_identifiers_mismatch":
        crosswalk = json.loads(json.dumps(crosswalk))
        crosswalk["reconciliationFormula"]["cherriToAppleMappingEquation"] = (
            "505 Cherri catalog entries map to 381 unique ShortcutIdentifiers in the CLI catalog"
        )
    elif mutate_mode == "c5_5_evidence_tier_inflated":
        matrix = json.loads(json.dumps(matrix))
        matrix["entries"][0]["evidenceTier"] = "NATIVE_SPECIMEN_VERIFIED"
        matrix["entries"][0]["nativeSpecimenParityStatus"] = "NEEDS_EVIDENCE"
    elif mutate_mode == "c5_6_markdown_stale_number":
        gap_md = gap_md.replace("`97` `NEW_NATIVE_ACTION`", "`100` `MISSING_HIGH_PRIORITY`")
    elif mutate_mode == "c5_7_parameter_matrix_vs_report":
        cat_md = cat_md.replace("| `optionalParams` | `505` | `8` | `201` |", "| `optionalParams` | `505` | `8` | `505` |")
    elif mutate_mode == "report_verified":
        cov_report_md = cov_report_md.replace("| `COVERED_VERIFIED` | 0 |", "| `COVERED_VERIFIED` | 389 |")
    elif mutate_mode == "crosswalk_verified":
        crosswalk = json.loads(json.dumps(crosswalk))
        crosswalk["denominators"]["appleCoveredVerifiedIdentifiers"] = 390
    elif mutate_mode == "gap_md_missing_high":
        gap_md = gap_md.replace("`97` `NEW_NATIVE_ACTION`", "`100` `MISSING_HIGH_PRIORITY`")
    elif mutate_mode == "matrix_fake_runtime_tier":
        matrix = json.loads(json.dumps(matrix))
        matrix["entries"][0]["evidenceTier"] = "APPLE_RUNTIME_VERIFIED"
        matrix["entries"][0]["appleRuntimeExecutedStatus"] = "NEEDS_EVIDENCE"

    # 1. Authoritative Ledger (2,396 rows)
    ledger_total = len(ledger)
    status_counts = collections.Counter(r.get("status") for r in ledger)
    elig_counts = collections.Counter(r.get("eligibility") for r in ledger)

    if ledger_total != 2396:
        errors.append(f"Expected 2,396 total ledger rows, got {ledger_total}")

    expected_verified = status_counts.get("COVERED_VERIFIED", 0)
    expected_partial = status_counts.get("COVERED_PARTIAL", 0)
    expected_new_native = status_counts.get("NEW_NATIVE_ACTION", 0)
    expected_missing_intent = status_counts.get("MISSING_APP_INTENT_DESCRIPTOR", 0)
    expected_deferred = status_counts.get("DEFERRED_NO_EVIDENCE", 0)
    expected_hidden = status_counts.get("EXCLUDED_HIDDEN_INTERNAL", 0)
    expected_config = status_counts.get("EXCLUDED_CONFIG_ONLY", 0)
    expected_nonrunnable = status_counts.get("EXCLUDED_NONRUNNABLE", 0)
    expected_third_party = status_counts.get("EXCLUDED_THIRD_PARTY_DEPENDENCY", 0)

    expected_eligible = elig_counts.get("ELIGIBLE_USER_FACING", 0) + elig_counts.get("ELIGIBLE_APP_INTENT", 0)
    expected_excluded = ledger_total - expected_eligible
    expected_covered = expected_verified + expected_partial
    expected_uncovered_eligible = expected_new_native + expected_missing_intent

    if expected_eligible + expected_excluded != ledger_total:
        errors.append("Ledger eligible + excluded != ledger_total")
    if expected_covered + expected_uncovered_eligible != expected_eligible:
        errors.append(
            f"Ledger covered ({expected_covered}) + uncovered eligible ({expected_uncovered_eligible}) != eligible ({expected_eligible})"
        )

    # 2. Crosswalk JSON consistency against Ledger & Matrix
    d = crosswalk.get("denominators", {})
    if d.get("appleRuntimeDiscoveredTotal") != ledger_total:
        errors.append(f"Crosswalk appleRuntimeDiscoveredTotal={d.get('appleRuntimeDiscoveredTotal')} != {ledger_total}")
    if d.get("appleEligibleUserFacingTotal") != expected_eligible:
        errors.append(f"Crosswalk appleEligibleUserFacingTotal={d.get('appleEligibleUserFacingTotal')} != {expected_eligible}")
    if d.get("appleExcludedInternalOrNonRunnableTotal") != expected_excluded:
        errors.append(f"Crosswalk appleExcludedInternalOrNonRunnableTotal={d.get('appleExcludedInternalOrNonRunnableTotal')} != {expected_excluded}")
    if d.get("appleCoveredUniqueIdentifiers") != expected_covered:
        errors.append(f"Crosswalk appleCoveredUniqueIdentifiers={d.get('appleCoveredUniqueIdentifiers')} != {expected_covered}")
    if d.get("appleCoveredVerifiedIdentifiers") != expected_verified:
        errors.append(
            f"Crosswalk appleCoveredVerifiedIdentifiers={d.get('appleCoveredVerifiedIdentifiers')} != ledger COVERED_VERIFIED ({expected_verified})"
        )
    if d.get("appleCoveredPartialIdentifiers") != expected_partial:
        errors.append(
            f"Crosswalk appleCoveredPartialIdentifiers={d.get('appleCoveredPartialIdentifiers')} != ledger COVERED_PARTIAL ({expected_partial})"
        )
    if d.get("appleUncoveredEligibleIdentifiers") != expected_uncovered_eligible:
        errors.append(
            f"Crosswalk appleUncoveredEligibleIdentifiers={d.get('appleUncoveredEligibleIdentifiers')} != {expected_uncovered_eligible}"
        )
    if d.get("appleNewNativeActionIdentifiers") != expected_new_native:
        errors.append(
            f"Crosswalk appleNewNativeActionIdentifiers={d.get('appleNewNativeActionIdentifiers')} != ledger NEW_NATIVE_ACTION ({expected_new_native})"
        )
    if d.get("appleMissingAppIntentDescriptorIdentifiers") != expected_missing_intent:
        errors.append(
            f"Crosswalk appleMissingAppIntentDescriptorIdentifiers={d.get('appleMissingAppIntentDescriptorIdentifiers')} != ledger MISSING_APP_INTENT_DESCRIPTOR ({expected_missing_intent})"
        )

    cw_status = crosswalk.get("ledgerStatusBreakdown", {})
    for k, v in status_counts.items():
        if cw_status.get(k) != v:
            errors.append(f"Crosswalk ledgerStatusBreakdown[{k}]={cw_status.get(k)} != ledger {v}")

    # Ensure crosswalk equations never claim "390 COVERED_VERIFIED" when ledger has 0 COVERED_VERIFIED
    eq_cov = crosswalk.get("reconciliationFormula", {}).get("appleCoverageEquation", "")
    if expected_verified == 0 and "390 COVERED_VERIFIED" in eq_cov:
        errors.append("Crosswalk appleCoverageEquation falsely claims '390 COVERED_VERIFIED' while ledger has 0 COVERED_VERIFIED")

    # Ensure uniqueIdentifiersInCherriCatalog and cherriToAppleMappingEquation are consistent (Part C5 #4)
    m_entries = matrix.get("entries", [])
    expected_unique_idents = len({e.get("shortcutIdentifier") for e in m_entries if e.get("shortcutIdentifier")})
    if d.get("uniqueIdentifiersInCherriCatalog") != expected_unique_idents:
        errors.append(
            f"Crosswalk uniqueIdentifiersInCherriCatalog={d.get('uniqueIdentifiersInCherriCatalog')} != {expected_unique_idents}"
        )
    eq_map = crosswalk.get("reconciliationFormula", {}).get("cherriToAppleMappingEquation", "")
    if f"map to {expected_unique_idents} unique ShortcutIdentifiers" not in eq_map:
        errors.append(
            f"Crosswalk cherriToAppleMappingEquation does not match uniqueIdentifiersInCherriCatalog ({expected_unique_idents}): {eq_map}"
        )
    if "381 unique ShortcutIdentifiers" in eq_map:
        errors.append("Crosswalk cherriToAppleMappingEquation contains stale hardcoded '381 unique ShortcutIdentifiers'")

    # 3. 505 Catalog Coverage Matrix vs Execution Evidence
    ev_entries = exec_ev.get("entries", [])
    if len(m_entries) != 505 or len(ev_entries) != 505:
        errors.append(f"Expected 505 entries in matrix ({len(m_entries)}) and execution evidence ({len(ev_entries)})")
    else:
        ev_by_name = {e["name"]: e for e in ev_entries}
        for e in m_entries:
            nm = e["cherriName"]
            if nm not in ev_by_name:
                errors.append(f"Matrix entry {nm} missing from p10-505-execution-evidence.json")
                continue
            ev_row = ev_by_name[nm]
            if e.get("structuralRoundTripPass") != ev_row.get("structuralRoundTripPass"):
                errors.append(f"Matrix vs execution evidence structuralRoundTripPass mismatch on {nm}")
            if e.get("roundTripVerified") != ev_row.get("roundTripVerified"):
                errors.append(f"Matrix vs execution evidence roundTripVerified mismatch on {nm}")
            if e.get("decompileMode") != ev_row.get("decompileMode"):
                errors.append(f"Matrix vs execution evidence decompileMode mismatch on {nm}")
            if e.get("evidenceTier") in ("APPLE_RUNTIME_VERIFIED", "CONFIRMED_RUNTIME") and e.get("appleRuntimeExecutedStatus") != "APPLE_RUNTIME_EXECUTED":
                errors.append(
                    f"Matrix entry {nm} claims evidenceTier={e.get('evidenceTier')} without appleRuntimeExecutedStatus=APPLE_RUNTIME_EXECUTED"
                )
            if e.get("evidenceTier") == "NATIVE_SPECIMEN_VERIFIED" and e.get("nativeSpecimenParityStatus") != "NATIVE_SPECIMEN_PARITY_PASS":
                errors.append(
                    f"Matrix entry {nm} claims evidenceTier=NATIVE_SPECIMEN_VERIFIED without nativeSpecimenParityStatus=NATIVE_SPECIMEN_PARITY_PASS"
                )
            if e.get("evidenceTier") in ("CONFIRMED_NATIVE", "OBSERVED", "INFERRED"):
                errors.append(
                    f"Matrix entry {nm} uses legacy ambiguous evidenceTier={e.get('evidenceTier')}"
                )

    # 4. Baseline State JSON consistency
    if baseline.get("counts") != crosswalk.get("denominators"):
        errors.append("baseline-state.json counts do not match p10-apple-cherri-crosswalk.json denominators")
    if baseline.get("inputDependencyFingerprint") != matrix.get("inputDependencyFingerprint"):
        errors.append("baseline-state.json inputDependencyFingerprint does not match matrix")
    if crosswalk.get("inputDependencyFingerprint") != matrix.get("inputDependencyFingerprint"):
        errors.append("crosswalk inputDependencyFingerprint does not match matrix")

    # 5. p10-coverage-report.md checks
    m_tot = re.search(r"\*\*Total Raw Discovered Records\*\*\s*\|\s*(\d+)", cov_report_md)
    if not m_tot or int(m_tot.group(1)) != ledger_total:
        errors.append(f"p10-coverage-report.md total records mismatch: found {m_tot.group(1) if m_tot else 'none'}, expected {ledger_total}")

    for st, count in status_counts.items():
        pattern = rf"\|\s*`{re.escape(st)}`\s*\|\s*(\d+)\s*\|"
        m = re.search(pattern, cov_report_md)
        if not m:
            errors.append(f"Status {st} missing from p10-coverage-report.md table")
        elif int(m.group(1)) != count:
            errors.append(f"Status {st} count mismatch in p10-coverage-report.md: {m.group(1)} != ledger {count}")

    m_ver = re.search(r"\|\s*`COVERED_VERIFIED`\s*\|\s*(\d+)\s*\|", cov_report_md)
    if not m_ver or int(m_ver.group(1)) != expected_verified:
        errors.append(
            f"p10-coverage-report.md COVERED_VERIFIED mismatch: report has {m_ver.group(1) if m_ver else 'none'}, ledger has {expected_verified}"
        )

    # 6. apple-discovered-not-supported.md checks
    if f"`{expected_partial}`" not in gap_md or f"`{expected_new_native}` `NEW_NATIVE_ACTION`" not in gap_md:
        errors.append("apple-discovered-not-supported.md missing authoritative COVERED_PARTIAL or NEW_NATIVE_ACTION count")
    if f"`{expected_missing_intent}` `MISSING_APP_INTENT_DESCRIPTOR`" not in gap_md:
        errors.append("apple-discovered-not-supported.md missing authoritative MISSING_APP_INTENT_DESCRIPTOR count")
    for st, count in (
        ("COVERED_VERIFIED", expected_verified),
        ("COVERED_PARTIAL", expected_partial),
        ("NEW_NATIVE_ACTION", expected_new_native),
        ("MISSING_APP_INTENT_DESCRIPTOR", expected_missing_intent),
        ("DEFERRED_NO_EVIDENCE", expected_deferred),
        ("EXCLUDED_HIDDEN_INTERNAL", expected_hidden),
        ("EXCLUDED_CONFIG_ONLY", expected_config),
        ("EXCLUDED_NONRUNNABLE", expected_nonrunnable),
        ("EXCLUDED_THIRD_PARTY_DEPENDENCY", expected_third_party),
    ):
        pat = rf"\|\s*`{re.escape(st)}`\s*\|\s*`?(\d+)`?\s*\|"
        m = re.search(pat, gap_md)
        if not m or int(m.group(1)) != count:
            errors.append(
                f"apple-discovered-not-supported.md table mismatch for {st}: got {m.group(1) if m else 'none'}, expected {count}"
            )
    # Count numbered rows in the NEW_NATIVE_ACTION table at the end of apple-discovered-not-supported.md
    new_native_table_rows = re.findall(r"^\|\s*(\d+)\s*\|\s*`[^`]+`\s*\|", gap_md, flags=re.MULTILINE)
    if len(new_native_table_rows) != expected_new_native:
        errors.append(
            f"apple-discovered-not-supported.md lists {len(new_native_table_rows)} NEW_NATIVE_ACTION rows, expected {expected_new_native}"
        )

    # 7. catalog-supported-by-compiler.md checks
    ms = matrix["summary"]
    if f"`{ms['structuralRoundTripVerifiedCount']} / {ms['totalEntries']}`" not in cat_md:
        errors.append("catalog-supported-by-compiler.md missing structuralRoundTripVerifiedCount")
    cat_table_rows = re.findall(r"^\|\s*(\d+)\s*\|\s*`[^`]+`\s*\|\s*`(?:action|compilerConstruct)`\s*\|", cat_md, flags=re.MULTILINE)
    if len(cat_table_rows) != 505:
        errors.append(f"catalog-supported-by-compiler.md has {len(cat_table_rows)} action rows, expected 505")
    for pd in ms.get("parameterDimensionSummaries", []):
        row_prefix = f"| `{pd['dimension']}` | `{pd['totalCatalog']}` | `{pd['constructsExcluded']}` | `{pd['eligible']}` | `{pd['tested']}` | `{pd['pass']}` | `{pd['fail']}` | `{pd['notApplicable']}` | `{pd['notTested']}` |"
        if row_prefix not in cat_md:
            errors.append(
                f"catalog-supported-by-compiler.md parameter dimension row mismatch for {pd['dimension']}: expected {row_prefix}"
            )

    # 8. pure-pack-portability.md checks
    if "`441` catalog entries" not in pack_md:
        errors.append("pure-pack-portability.md does not report dynamic 441 Pure Original catalog entries")
    if "`352`" in pack_md or "`389` catalog entries" in pack_md:
        errors.append("pure-pack-portability.md contains stale hardcoded 352 or 389 count")

    # 9. task-evidence-matrix.md checks
    if "`441`" not in task_md or "`505`" not in task_md:
        errors.append("task-evidence-matrix.md missing 441 or 505 catalog counts")

    # 10. p10r2-validation-results.json checks
    metrics = val_res.get("catalogMetrics", {})
    if metrics:
        if metrics.get("ledgerTotalRecords") != ledger_total:
            errors.append(f"Validation results ledgerTotalRecords mismatch: {metrics.get('ledgerTotalRecords')} != {ledger_total}")
        if metrics.get("ledgerCoveredVerified") != expected_verified:
            errors.append(f"Validation results ledgerCoveredVerified mismatch: {metrics.get('ledgerCoveredVerified')} != {expected_verified}")
        if metrics.get("ledgerCoveredPartial") != expected_partial:
            errors.append(f"Validation results ledgerCoveredPartial mismatch: {metrics.get('ledgerCoveredPartial')} != {expected_partial}")

    return len(errors) == 0, errors


def main():
    parser = argparse.ArgumentParser(description="Check consistency across all 11 P10 ledgers, crosswalks, and reports")
    parser.add_argument("--mutate-check", action="store_true", help="Simulate tampered artifacts across multiple surfaces to prove fail-closed detection")
    args = parser.parse_args()

    p10_dir = REPO_ROOT / "docs" / "apple-runtime" / "p10"

    if args.mutate_check:
        all_modes = (
            "c5_1_ledger_vs_crosswalk",
            "c5_2_crosswalk_vs_report",
            "c5_3_deferred_double_count",
            "c5_4_unique_identifiers_mismatch",
            "c5_5_evidence_tier_inflated",
            "c5_6_markdown_stale_number",
            "c5_7_parameter_matrix_vs_report",
            "report_verified",
            "crosswalk_verified",
            "gap_md_missing_high",
            "matrix_fake_runtime_tier",
        )
        for mode in all_modes:
            ok, errors = check_consistency(p10_dir=p10_dir, mutate_mode=mode)
            if ok:
                print(f"ERROR: Mutation mode '{mode}' unexpectedly passed consistency check!", file=sys.stderr)
                sys.exit(1)
        print(f"SUCCESS: All {len(all_modes)} mutation modes (including Part C5 1..7) correctly failed closed.")
        sys.exit(0)

    ok, errors = check_consistency(p10_dir=p10_dir)
    if not ok:
        print("ERROR: Inconsistencies detected across P10 ledgers, crosswalk, and Markdown reports:", file=sys.stderr)
        for e in errors:
            print(f"  - {e}", file=sys.stderr)
        sys.exit(1)

    print("SUCCESS: All counts, statuses, and fingerprints across all 11 P10 ledgers and reports are 100% consistent.")
    sys.exit(0)


if __name__ == "__main__":
    main()
