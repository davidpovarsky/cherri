# K11 Bool/List Simulator result — original-source intake

## Result and identity

Run [45108412777](https://github.com/davidpovarsky/cherri/actions/runs/45108412777), attempt 1, completed **SUCCESS** on implementation `9b4135e705b08f765b7304e01212d3b9658806fb`, branch `agent/cherri-p10r15-k11-verification-20261123`. Its original `xcode-27` job `137474934460` ran from **2026-11-23 03:59:18 UTC to 04:32:26 UTC** (33 minutes 8 seconds). The original sole observer exited 0 without timing out.

This was the explicitly assigned completion of the unmet original Bool/List publication contract after K10 run `45100952614` was cancelled before runtime. The K10 cancellation originals and cause-undetermined classification remain unchanged. No additional native rerun was performed during this intake.

| Observed layer | Result |
| --- | --- |
| Original XCTest aggregate | 4 passed of 4; 0 failed, skipped or expected failures |
| Exact runtime contract | `CHERRI_QA_BOOL_LIST_RESULT:ok:first=first:last=second` |
| Existing source callback/trace oracle | runGate PASS and traceGate PASS |
| Fresh correlated non-self callback | Exactly 1, status SUCCESS |
| Original NDJSON trace | 4 records; byte-equivalent to canonical replay after CRLF normalization |
| Compiler/signing join | Exact source, unsigned plist, target name, signed digest/size and 32-action inventory agree |
| Published payload integrity | All 25 payload hashes and all 26 native ZIP members verified |
| Public prerequisite layer | Existing exact-source validator PASS and native compiler context bound |
| Private prerequisite originals | 23 complete members, 3 source commands and 21 exact Git-bound files verified in place |

The `AEA1` header is a format/provenance observation; this intake does not claim a separate cryptographic signature verification.

## Exact public contract and limits

The unchanged fixture is `tools/shortcuts-simulator-qa/fixtures/qa-bool-list-v2.cherri`, SHA-256 `8337913a2b3b888bdfbba1abbfc28ddc8ce63b87a7383531a3ac03cab0b49e00`. It exercises the true/false branch checks, obtains the first and last items of a List of Dictionaries, and reads their `name` fields as `first` and `second`.

The first Dictionary also contains a false `flag` and the second contains a nested List, but this fixture does **not read those nested values**. The retained output assertions still report `typedValueCoverage=UNKNOWN_AT_TRACE_EVENT` and `perActionCoverageVerified=false`. Consequently this result does not establish N03, every nested transport/type semantic, or per-action value coverage.

Editor inspection remains `NEEDS_REVIEW` with `PARTIAL_EDITOR_CAPTURE`; overall published readiness remains `PARTIAL_READY`. Physical-device status remains `BLOCKED_DEVICE`. This is bounded K11 Simulator evidence, not physical-device, whole-release, future-source, or generic formal acceptance. `formalAcceptanceGranted` remains false.

## Original evidence and independent intake

`originals.zip` preserves both original GitHub artifact ZIPs and selected original run/jobs/artifact metadata, dispatch request/result, exact run binding, terminal acquisition and watcher result. `originals-manifest.json` binds every member by exact size and SHA-256.

The two GitHub ZIPs are:

- Native publication artifact `14790575145`: 68,101 bytes, SHA-256 `98f3c91cd5f2edeaf8381980b6c7b7d804b99a57cacc5885ffa8e46df529eb71`.
- Sealed prerequisite artifact `14789639590`: 585,986 bytes, SHA-256 `e7df87b6d7446dcdd55a3f794369d8735640fc56fd0f4ff10d35534c36af687f`. It contains only the original `project-prerequisite.zip.p7m` and `summary.json`.

The existing K11 source helpers checked the published inventory, synthetic source provenance, public privacy rules, exact compiler/signing/source joins, fresh callback correlation, strict expected result and original NDJSON. The exact callback oracle was extracted from the frozen orchestrator, and applied to retained original observations. It did not execute a Shortcut or rerun XCTest.

`intake-execution-notes.json` preserves a local caller correction before oracle execution: the existing privacy scanner returns a dictionary with `passed` and `violations` fields, rather than a bare boolean. The caller was corrected to require `passed is true` and an empty violations list. Original artifacts, source validators, test assertions and deadlines were unchanged.

## Private prerequisite verification boundary

The mandatory public validator ran first. Its original receipt and the native compiler-manifest-derived context remain separate from the later private check.

Only the original ciphertext, public summary/protocol/context/validator receipt and reviewed verifier code were transferred to the existing private Windows workspace. Decryption and verification used the already authorized local recipient/key. **No plaintext archive, private path, nonce, key or generic private log was transferred out.** The only returned private-verification result is the fixed booleans/counts/public-digests object in `private-verification.fixed.json`.

The K11 verifier is a data-only extension of the previously calibrated `c74295ff…` verifier. It adds only the exact tuple `45108412777 → (9b4135e705b08f765b7304e01212d3b9658806fb, qa-bool-list-v2)`. All original tuples, validation logic and fixed output fields remain unchanged; the independent static review and extension receipt are included. The prior R7 calibration is mechanical verifier evidence, not a new native result or a source-equivalence exception.

Verifier SHA-256: `1550810ce300a8ed46f76db6156b17d8ea5edbbb3067a6d7f94249fe831a49ee`. Expected protocol SHA-256: `f6f79b7e0e329ff7846db0798c2118e48bd05671ed512dca8e60c59d43ba0fdf`. Native context SHA-256: `ac38de1cf44ce063965d1851915abaafd0c9bbb55c1e1d13c805d0002b594775`.

The private check is prerequisite-only and explicitly claims no native acceptance. The separate actual callback/trace/XCTest originals establish the bounded native contract above.

## Change scope

This evidence commit changes documentation only. It does not modify the frozen K11 source, fixtures, workflow, tests, assertions, budgets, protected UI bytes or release policy.
