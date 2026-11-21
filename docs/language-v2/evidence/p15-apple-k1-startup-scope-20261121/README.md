# K1 Apple failures and scoped startup query recovery — 2026-11-21

Execution mode: [SUBAGENT /root/apple_runtime]. This packet preserves native failures and local harness evidence. It grants no formal requirement acceptance.

## Original native runs

All three completed workflow attempts used exact source `d90b69a92e83b5034728c22e062a3dd96153a2a8` (K1), attempt 1.

| Run | Actual result | Evidence boundary |
| --- | --- | --- |
| [Startup 44938798962](https://github.com/davidpovarsky/cherri/actions/runs/44938798962) | Failure; xcodebuild 65; all three BRG27/D03/E01 UI cases failed | No acceptance records; early main-queue probe execution does not prove continuous responsiveness |
| [File UTF8 44938804420](https://github.com/davidpovarsky/cherri/actions/runs/44938804420) | Failure; part 1 receiver/setup 3 PASS; test02 never started | Destination query returned MISSING; one Simulator open command exited 1; no compiled Shortcut callback |
| [N03 44938970557](https://github.com/davidpovarsky/cherri/actions/runs/44938970557) | Failure; part 1 receiver/setup 3 PASS; test02 never started | Same destination failure; case-sensitive equality remains UNKNOWN |

The four original published evidence ZIPs in `native-originals.zip` are byte-identical to the GitHub artifacts and their API SHA-256 digests. The archive also preserves the original run/jobs/artifacts API JSON for each run. Per-member hashes and exact source/run bindings are in `manifest.json`. No native artifact content was rewritten.

Full original workflow logs and the 46,255,912-byte Simulator app ZIP remain in the isolated host intake directory `C:\Users\DAVID\Code\cherri-p10r15-apple-analysis-20261119\integrated-d90b69a9-20261121`. The app ZIP SHA-256 is `0d420fa8402969b62de1c093c5cee8ce24a09e8f078e730a560ce5fe32cf17ff`, matched to the API. Those originals are not copied into this public evidence packet.

### Startup observations

The actual main-queue probes executed on the main thread in all three tests: BRG27 1,908 ms, D03 2,375 ms, and E01 2,232 ms. These are early observations, not proof that the queue remained responsive later.

BRG27's tap entered at 2.550 s and returned at 56.273 s, beyond the unchanged 28 s readiness budget; the first document initialization occurred at approximately 56.104 s. The sanitized original trace does not separate pre-event XCTest idling from document handling.

D03's editor-wait known-controls query spanned 19.876–27.969 s. The editor-hittability observation ended at 28.227 s and the wait failed. The app reported editor appearance at approximately 26.405 s. The aggregate query trace cannot attribute all 8.093 s to the creation-control query.

E01 retained an unclassified issue during the document-control query. No document phase was observed. No diagnosis is invented from the sanitized issue description.

### Destination and signing observations

Both File and N03 retained a successful xcodebuild destination-query process with MISSING exact owned device matches. Their subsequent `open` commands exited 1. The K1 helper discarded that command's stdout/stderr to DEVNULL; only the destination-query original was quarantined. Therefore the exact OS launch error is unknown. Earlier wording that the open error text was retained was explicitly corrected in Asana.

File's real remote signing succeeded: original source SHA-256 `8e69b72584bedbfc101c8b2e4cfec92e07afc70a47c7f9405abde9bea6a73247`; unsigned 15,447-byte/17-action graph SHA-256 `a7c6f954035101a8a17606833b91507da8aa00a1e3dcb4bf7df35212dc5f62df`; signed AEA1 24,200-byte object SHA-256 `bfa5d77fc10e727653905301ed178f1574a005e03fa7167c1b25d7d22fe70f8f`. Signing and receiver self-tests do not establish Shortcut runtime semantics.

## Scoped startup-query source and verification

Source commit: `44fa207596c756150f54a85b79831729e4c3c128`, based on K1.

After the existing successful single document-creation claim, the editor wait queries only the current editor. Before that claim, creation controls are still resolved before the editor, including the original ambiguity checks. The actual Foundation resolver owns this selection and is covered by query-count/identity/order controls.

A unique current editor, current window/foreground/hittability, successful XCTest wait, one observation consumption, and the original 28 s decision-time deadline remain required. An editor-only observation cannot authorize document creation or a document wait. Receipt schema 1.1 explicitly identifies the control scope; skipped creation queries do not emit false absence timing events. The collector preserves legacy 1.0 evidence and rejects inconsistent scope, stage, timing, creation and metadata combinations.

The clean source was unchanged before and after verification:

- 145 actual Swift Foundation assertions PASS; compile exit 0 in 7.657 s and execution exit 0 in 0.125 s.
- 46 collector tests PASS in 10.488 s; process exit 0 in 10.922 s.
- The original stdout/stderr, process receipts and exact concatenated verifier bytes are in `startup-local-originals.zip`.
- A separate read-only review by /root/docs_consumers inspected the five-file source diff and found no blocker within harness scope. No duplicate replay was used for that review.

These local checks do not execute XCTest or Apple UI. They establish the harness selection, timing and evidence rules. The long BRG27 tap and actual Apple startup failures remain open until a new native run on the integrated source provides evidence.
