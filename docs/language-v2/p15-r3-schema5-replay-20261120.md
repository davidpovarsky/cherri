# Combined R3 schema production replay — 2026-11-20

The dedicated clean detached checkout at `94188983b5f89e56b5935f96c72dc4ef394cfb51` completed the actual five-producer scenario in **722.000 seconds**, exit0. The original Go stream contains36 named passing events: the parent and35 receipt-corruption controls; there are no failures or skips. The checkout remained at the same SHA and clean before and after execution.

The locally verified partial report contains five distinct passed requirements and11 records:

| Requirement | Actually executed tiers |
|---|---|
| RP06 — Current declaration propagation | generation, integration |
| RP07 — Facet conflict detection | generation, negative-integration |
| RP08 — Schema fingerprint and typed facts | generation, unit |
| BRG22 — Live generation failure handling | generation, integration |
| BRG23 — Real declaration propagation | repository, integration, generation |

All11 records identify94188983 and the exact selected production tests. The actual scenarios rebuild and alter owned detached source copies, execute generation and consumer propagation, reject malformed live declarations and restore the changed inputs. The full179-row audit assertions in the parent reject inherited passes for every unexecuted ID.

Each of the five IDs has seven actual negative receipt cases: zero test selection, missing assertion, unrelated producer, stale source, invented runtime tier, absent generation tier, and wrong requirement. The partial report remains `closure_complete=false`; it is not a full179 report and contains no Apple or consumer deployment evidence.

Raw JSONL SHA-256: `6ba927ee431dae3d16a9811d5da4bee039b2f8609e6f2e0238302dc4ad4f4545` (48,979 bytes). [Retained evidence](evidence/p15-r3-schema5-replay-20261120/) contains the original start/process receipts, JSONL and empty stderr, plus a clearly identified derived receipt reconstructed from consecutive Go JSON output events for the parent test. It does not rewrite the original stream.

The earlier8257b5b8 attempt is retained as incomplete under the confirmed600-second external timeout. This completed attempt used unchanged scenarios, a1500-second Go limit,1560-second process limit and1800-second host limit. No timeout was converted into success.

Later integration/proof commits and the final compiler/docs/Skill candidate must execute their own complete acceptance/CI gates. The latest complete formal local179 report remains the independently retained R2 snapshot until that new run finishes.
