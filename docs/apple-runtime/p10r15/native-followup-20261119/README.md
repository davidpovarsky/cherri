# Native follow-up evidence, 19 November 2026

Both original native runs remain failures. Their exact published artifacts and independent audit outputs are retained here so the observations do not depend on the workflow artifact retention period.

| Run | Source | Independent audit | Actual observation |
| --- | --- | --- | --- |
| [44401954515](https://github.com/davidpovarsky/cherri/actions/runs/44401954515) | a04c6feede79358ee642a1ae27f0a0da8f995ad7 | FAIL_RUNTIME_PRESERVED, 20 published hashes verified | Native SUCCESS callback: L=stem, V=stem, I=stem; LT=ok, VT=ok, IT=ok. |
| [44401973737](https://github.com/davidpovarsky/cherri/actions/runs/44401973737) | a04c6feede79358ee642a1ae27f0a0da8f995ad7 | FAIL_RUNTIME_PRESERVED, 21 published hashes verified | Writer passed exact Unicode Text, native type and NDJSON trace. The separate reader produced no matched callback before timeout. |

## File naming

The existing fixture expected the complete name including `.json` from `getName`. On all three binding routes, the observed name was instead the stem, `cherri-export-control`, and the bytes were preserved. These observations do not support a binding-specific alias defect.

The retained independent Apple metadata in `docs/apple-runtime/macos-27/builtin-actions.json` lists only `WFInput` and `GetWebPageTitle` for Get Name. It provides no include-extension switch for that action. Set Name has the separate `WFDontIncludeFileExtension` option. The run has no independent File Extension witness, so it does not establish whether the `.json` extension was retained. The original expected marker and original failing result are unchanged.

## Storage reader dispatch

The writer was independently verified before the reader launch. The reader helper previously waited for 45 seconds but never invoked the UI consent handler that the writer uses. This is a confirmed harness omission; it is not evidence that a particular hidden dialog caused the original native timeout.

The repair reuses the existing single XCTest consent handler after an initial bounded wait, binds it to the reader nonce, refreshes the receiver container after XCTest, and waits once more for the actual callback. It never reissues the READ request and never treats an XCTest success as native storage success. Timed-out native command process groups are stopped. Raw UI logs remain outside the publication allowlist.

The original strict callback/trace/source/order oracle and storage diagnostic schema are unchanged. A new signed native run is required to establish whether this repair permits the two-invocation Text check to complete. It would still not prove typed archives, cache, TTL, deletion, cross-shortcut isolation or hardware behavior.
