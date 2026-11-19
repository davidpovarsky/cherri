# S4 actual WorkflowKit required-encoding triplet

The macOS job 135353904577 in Build & Test run 44462962145 executed the real WorkflowKit REQUIRED encoding gate on exact S4 b97ae752205f1e747b6c98ff7511f3484dd53bc2. The gate step passed at 05:25:06 UTC on 2026-11-19. The later Full Go Test Suite failed on the separately identified menus asset-resolution cases, so the overall Build & Test run remains FAILURE.

The actual osascript processes produced:

| Phase | Required observations passing | Expected process exit | Actual process exit |
| --- | ---: | ---: | ---: |
| Correct set | 23/23 | 0 | 0 |
| One isolated corrupted EXPECT | 22/23 | Nonzero | 1 |
| Restored correct set | 23/23 | 0 | 0 |

The single negative changes plainString.WFStringParameterState.string from accept to reject. Its actual failure is exactly E_REQUIRED_ENCODING_EXPECTATION for that identity; missing engine/discovery/infrastructure errors are not substituted for the expected failure. The canonical set also includes one independently expected rejection.

All three complete reports retain seven input forms across 50 discovered unique state-class entries (the discovery counters separately report 95 parameter-state classes and 20 app-intent classes). These are parameter-state API observations, not signed Shortcut runtime executions.

The audit reopened all phase reports and process receipts, invoked the source-matching strict Python per-form validator, checked every stdout/stderr/report digest and the 26 references across the two producer records, and reconciled all five project input hashes against Git objects at S4. The pinned frontboat/shortcutkit prelude was independently retrieved by its immutable commit and matched its declared SHA256. Raw inputs and source expectations were not modified.

The two original producer records target CI01 and RP47 at S4. This archive preserves their original source/run identity; it does not relabel them to a later compiler or claim that the Go formal intake was executed during this audit. The later aggregate must still apply its source-eligibility and full artifact checks.

All 15 exact public artifact members, the original ZIP, and a bounded audit receipt are included. No private inputs, native shortcuts, binaries, or screenshots are present. Raw bytes are preserved and every indexed Git blob is checked against its file digest before commit.
