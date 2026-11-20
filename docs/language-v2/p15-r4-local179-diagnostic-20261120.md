# R4 local179 diagnostic and follow-up repairs — 20 November 2026

## Original formal diagnostic result

[Run 44680943178](https://github.com/davidpovarsky/cherri/actions/runs/44680943178) executed the real local acceptance runner at clean source 4e578c810a4e30ad7b3060f35f913c9534ef70e5. It failed. All 179 immutable requirement IDs are present: 92 PASSED, 10 FAILED, 61 NOT_RUN, 15 PENDING_EXTERNAL and 1 AI_EVAL_NOT_RUN. The report contains 205 evidence records bound to that exact implementation source. Closure remains false.

The original artifact 14605356003 is retained as its downloaded ZIP, including all 59 original files. Its upload and download SHA256 is 31e2e9930eff9b81d8b7bb691c846c6b62828e1754ad4a5a790333b11852f64f (412,413 bytes). The untouched acceptance report SHA256 is 572c8be26e5d06850e46ee312363461333d074fa83abf9a733d1432c465e820c. Independent checks matched every extracted member to the original ZIP and manifest, all 205 implementation SHAs, exact 94+52+33 contract membership, original contract digests, report and runner-log hashes, and the actual clean-source process receipt.

The workflow deliberately retained the frozen R3 consumer pin cdc8be2c000698dddbe73185f69d8fd94273e1d6 and documentation pin 2bc5f6992cd252f287530b039ee393b1b50ee39e. That is visible diagnostic state, not an assertion that the old consumer tuple is compatible with the new compiler. Only the approved public signing fixture was opted in; no private corpus was requested.

Compared with the frozen R3 report, U01, RP11, RP16, RP33 and BRG19 moved from NOT_RUN to PASSED; RP02 and BRG04 moved from FAILED to PASSED. Ten previously passing rows failed at this intermediate source. Their original failures are not replaced by the subsequent component checks below.

| Failed rows | Observed issue and bounded follow-up |
|---|---|
| V02 | Optional transport rejects the static literal List/index initializer used by the unchanged binding-flow tests. The later forwarding integration still has this provenance gap. Its repair remains separate and open. |
| IM03, RP28, BRG20 | Exact preserved stdfunc replacement fields fail under the intermediate FString lowering. The developed S01 pure-literal fix passes the unchanged native-structure and corruption tests; that component is not yet integrated here. |
| RP08 | The disposable codec mutation test still selected v2.0 after the declared production codec advanced. The one-line fixture correction and actual mutation/restoration replay are recorded below. |
| K02, K03, K04, K05, RP37 | Actual compatibility validation reports the frozen macro/codec identities against the newer compiler. K05/RP37 stop before the importer is reached; these receipts do not demonstrate an importer regression. A new exact compiler/docs/consumer tuple and the complete installed tool pair remain required. |

## Completed Linux job; parent workflow still nonterminal

Linux job 136103015473 in [Build & Test 44680670000](https://github.com/davidpovarsky/cherri/actions/runs/44680670000/job/136103015473) completed FAILED at the same exact 4e source. Its receipt confirms all 30 packages executed once: 29 ordinary packages completed with exit 1 in 467.708 seconds under the original 15-minute limit, and the acceptance package completed with exit 1 in 1,157.171 seconds under its existing 35-minute limit. No test-timeout panic appears. The original 2,714,821-byte job log has SHA256 76b23785589403d010ee96a4e99b2fcabf7750cfa3c415c352fa37d9173eb3cb. The complete suite receipt is retained in original artifact 14605128801 (ZIP SHA256 5dabe64a4a938a44ccc72229cc1601fe6d0c60208abafc424d251d491b4802cf). Additional JSON-request, iPad-kit, nil-diagnostic/parity and inferred-type failures are under investigation. Nix passed; the macOS job was still queued at the captured observation, so no whole-workflow pass is claimed.

## Source-bound follow-ups

The Optional forwarding implementation/proof chain was integrated through 7c6043d891ca977ec6c4f85e89358f87081c5bb9. The resolved codec descriptor preserves the R4 S01 line and adopts the reviewed Optional v2.2 argument carrier. Its combined fingerprint is b69ab1d5da0ed295ec6f8a5b7883df88fcb295b8c2b2bda2665427ed042ab961. The complete component-to-integration map is retained.

The expanded clean replay at 7c6043d8 retained 1,117 named PASS and 33 FAIL events, zero skip, in 136.640 seconds. Its raw stdout is 1,115,543 bytes, SHA256 44ce5177014bc272f9f5c25725ee461e1618be7ecd817e95eb7d6a1fbbc88243. All failures belong to the newly added V02, stdfunc and RP08 selections. The six package selections passed 111 named tests in 7.422 seconds, zero named failure/skip; raw SHA256 e25dfc8ae0a88eba3e83290bd70ac607288753d72f22bd46b0a7a20ef8571893. The modules package has no test files and contributes no named pass.

RP08 was corrected at e408e9b9adc56da7cd32942727004c39b5a4dd10. Only the test's declared version selector changed from v2.0 to v2.2. Its disposable suffix mutation, typed defaults, Bool/Text distinctions, generated schema equality, public capability identities and exact restoration assertions are unchanged. The real compiler and generator replay passed all 9 named tests in 90.187 seconds, zero failure/skip. Original stdout: 203,902 bytes, SHA256 4e4849a5a6a9633ee68d5ff5925fbff5505add36e05751207a93a5a59b339273. Independent review also checked that the mutated codec changes only its identity, while restored original inputs restore both identities and exact catalog/generated hashes. Go splits the large observation line across Output events; the full observation object was reconstructed by concatenating the exact parent's original fragments. The initial prefix-only extraction and the reconstruction receipt are retained separately; the original JSONL was never altered.

The unchanged stdfunc native-structure and source-generation checks passed 33 named tests on clean S01 component d2e6b673a9eace821494c4a639c266fb773e0d66 in 11.578 seconds. Original stdout: 28,639 bytes, SHA256 3a9efc64e5c4e60ed34114441f4d25741e4a5f96baec8ea2e62b2b5e447c3476. This is component evidence, not a replay of the full 4e formal run. The remaining S01 ordinary-container oracle awaits actual native omitted-range observations.

The public Optional native diagnostic and proof were integrated as 737de846a513647fb3cdce93a16a80ad76170cfd and ebc15fa6f1585074199bfb77d45c6b21ca478892 from e84d02744eca729306898775992b545a305e6979 and c513b2d6d6cac9a94b58e3f7fc834f130afc1d80. At clean ebc15fa6, all TestP15OptionalForwarding and TestP15OptionalNativeDiagnostic families passed: 199 named PASS, zero failure/skip, 21.594 seconds. This includes the independent complete 399-action graph and causal corruptions. Original stdout: 186,658 bytes, SHA256 b627afe35c3594a2be3a2b0b690581f3497b18b47427e5c3012597ab3bd1758e. No formal requirement membership was added; native execution remains NOT_RUN.

## Retained originals and remaining gates

[Index](evidence/p15-r4-local179-diagnostic-20261120/index.json) and [original evidence archive](evidence/p15-r4-local179-diagnostic-20261120/original-evidence.zip) bind the original failed formal artifact, original process/log metadata, exact local failure and pass receipts, production RP08 observations, and both integration maps. Archived bytes, event counts, pass-name lists and staged Git blobs were independently verified; no line-ending normalization was applied to originals.

This proof does not promote staging or claim a full suite pass. V02, remaining container/value boundaries, installed companion/rollback work, final compiler/docs/consumer pins, whole CI, Apple app/runtime, physical-device obligations, external AI evaluation and the documented historical preservation gap remain subject to their own actual evidence. The frozen formal report remains FAILED with closure false.
