# P15 formal PoC evidence scope — 2026-11-19

The historical Simulator PoC emits seventeen diagnostic rows. Several rows use an obsolete requirement namespace; its first eight rows also associate narrow observations with broader immutable179 scenarios. The raw diagnostic file is retained unchanged. The new exporter creates a separate formal file and never upgrades a failed observation.

## Exported runtime facets

| Actual producer | Formal requirements | Observed scope |
| --- | --- | --- |
| variables | V01, V03 | Number/Text capture before mutation and one clipboard read before overwrite |
| indexing | L02, L04 | first/last/empty/negative/out-of-range and guarded dynamic indices |
| numbers | N01 | computed 5/2 = 2.5 in the numeric trace |
| functions | FN01, FN02, FN03 | function return, caller continuation, false/zero/empty Text presence defaults, and factorial recursion |

FN03 capture rejection still needs its independent local tiers. No repair, recovery-gate, Unicode-offset, mixed-Boolean-tree, structured-default, caller-input, main-return, menu, dictionary, CI, or closure success is inferred from these rows. Every non-runtime tier of all179 immutable obligations remains mandatory.

## Provenance and truth controls

`runtime_poc_formal_evidence.py` requires the complete seventeen-row diagnostic identity set, the publisher's exact reviewed artifact set, matching current SHA/run/digests, a freshly verified committed-source receipt, and computed observations matching the captured clipboard. Passed records must match the existing exact expected values. Failed records remain failed, including a failed run with no computed clipboard fields.

The exporter itself must exist byte-for-byte at the claimed commit. It therefore cannot retrospectively classify frozen evidence from a revision that did not contain this producer. The raw bytes are not rewritten; output assertions additionally bind the diagnostic and source-receipt digests. Output creation is exclusive, so an existing formal file is never silently reused or replaced.

The shell and publication wiring are a separate coordinated change. The raw producer must write `runtime-diagnostics.json`; this module writes `runtime-evidence.json` only after its checks succeed.

## Executed verification

Twelve actual Python unit tests passed both in the Linux scratch environment and on the authorized Windows host. They use explicitly synthetic temporary Git/source/artifact controls, including missing/duplicate/stale identities, non-string formal assertions, detached clipboard values, contradictory raw success, failed observations, and attempts to reclassify an older revision.

The actual Python exporter → production Go decoder/validator regression passed six test/subtest events on Windows (`formal-exporter-go-v2.jsonl`). It demonstrates the original raw bool/list schema rejection, valid narrow string-assertion intake, preserved all179 accounting, specific rejection of stale SHA/wrong assertions/changed artifacts/unknown run/unknown requirement, and preserved failed evidence. The full validator correctly rejects closure because the synthetic runtime facets provide no required local-tier evidence. Its policy was not relaxed.

These controls do not execute Apple Shortcuts and are not native runtime receipts. Development logs are under the isolated worktree's ignored `dist/p15-acceptance-causal/`. Fresh real Simulator evidence and complete final-candidate gates remain required.
