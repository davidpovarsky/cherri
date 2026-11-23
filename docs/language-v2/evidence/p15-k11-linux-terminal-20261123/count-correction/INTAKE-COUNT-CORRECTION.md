# Correction to the initial Linux intake counts

The initial derived intake undercounted exactly two passing parent tests. Some original GitHub log lines place a UTF-8 BOM before their timestamp. The initial observer stripped the timestamp first, so those two lines retained a timestamp and did not match the anchored PASS pattern.

Corrected Linux totals: **16,863 PASS, 11 SKIP, 0 FAIL** named events: **1,669 parent PASS**, 11 parent SKIP, and 15,194 subcase PASS. Ordinary: 10,837 PASS / 8 SKIP. Acceptance: 6,026 PASS / 3 SKIP.

The two omitted original lines are **21089 — TestP15RP08SchemaIdentityProduction** and **25570 — TestCatalogAccountingProductionEvidence**. Their exact original lines, hashes, section boundaries and corrected totals are in bom-count-correction.json.

The corrected observer removes a boundary BOM before removing the timestamp, and again at the message boundary. It retains the original anchored named-result pattern and exact GO_SUITE_EXECUTE section bounds; nested serialized child events and later workflow steps are not counted.

Initial intake.json, INTAKE.md, intake-audit.original.py, manifest.json and every original execution/download artifact remain unchanged. Read intake.corrected.json for the updated derived counts. No producer, test, download or workflow was rerun. Job failure remains --verify-commit fingerprint drift before --run-all/R23. Execution durations, all 30 package identities, skip reasons and encoding evidence qualification are unchanged.
