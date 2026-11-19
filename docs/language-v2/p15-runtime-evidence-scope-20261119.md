# P15 formal PoC evidence scope — 2026-11-19

The historical Simulator PoC emits seventeen diagnostic rows. Several rows use an obsolete requirement namespace; its first eight rows also associate narrow observations with broader immutable179 scenarios. The raw diagnostic file remains unchanged. The exporter creates a separate formal file and never upgrades a failed observation.

## Exported runtime facets

| Actual producer | Formal requirements | Observed scope |
| --- | --- | --- |
| variables | V01, V03 | Number/Text capture before mutation and one clipboard read before overwrite |
| indexing | L02, L04 | first/last/empty/negative/out-of-range and guarded dynamic indices |
| numbers | N01 | computed 5/2 = 2.5 in the numeric trace |
| functions | FN01, FN02, FN03 | function return, caller continuation, false/zero/empty Text presence defaults, and factorial recursion |
| coverage-s01 | S01 | Exact Unicode output after Hebrew, astral/ZWJ and combining prefixes, with repeated/adjacent references and a literal U+FFFC |
| coverage-f01 | F01 | Exact effect trace and Boolean results for nine cases including nested and mixed AND/OR trees |

The first four rows retain their original scope. The final two require the separate `CherriS01F01CoveragePOC` source and a replayed coverage receipt. Old Unicode/conditional diagnostics cannot supply either new producer. Six records cover ten distinct IDs; no repair or recovery ID inherits their results.

FN03 capture rejection still needs its independent local tiers. Structured defaults, caller input, main return, menu, dictionary, CI, and closure success are not inferred from these rows. Every non-runtime tier of all179 immutable obligations remains mandatory.

## Provenance and truth controls

`runtime_poc_formal_evidence.py` requires the complete seventeen-row diagnostic identity set, the publisher's exact reviewed artifact set, matching current SHA/run/digests, a freshly verified committed-source receipt, and computed observations matching the captured clipboard. Passed records match exact expected values. Failed records remain failed, including absent computed output.

The exporter itself must exist byte-for-byte at the claimed commit. It cannot retrospectively classify frozen evidence from a revision without this producer. Output creation is exclusive, so existing formal evidence is never silently reused or replaced. Raw diagnostics remain separate in `runtime-diagnostics.json`; the formal file is `runtime-evidence.json`.

Coverage rows invoke `runtime_poc_coverage.verify_result` after the run-owned device has been deleted. That observer replays exact clipboard bytes, a fresh per-run reset, source/expected-contract identity, signing identity, run/attempt and device bindings. A trailing newline, changed Unicode byte, changed effect order, wrong Boolean result, missing reset or altered receipt cannot become success. A valid failed observation is exported as failed rather than omitted.

The reviewed coverage source SHA256 is `4a13f90af2bdc326e24f9dbf785f76a1ae41043e176b5a3188ab794694a19aa3`; the expected-byte contract SHA256 is `8baa3279a770ca20e3daa6036f416360a308d6f40eb1dd0874a38afeb7d0b10c`. The contract retains `runtime_status: NOT_RUN`: an expected-output document is not a device receipt.

Both coverage records use the same twenty formal artifact references. Separate assertions bind `coverage-result.json`, context, reset proof, expected contract, source receipt, raw diagnostics, run/attempt and observed clipboard digests. Publication preserves all referenced/support bytes unchanged. Go intake additionally requires the exact S01/F01 producer, source/expected-contract hashes and receipt metadata, with exact observed values; it rejects the old weaker producer IDs and broader requirement membership.

## Executed verification

The original four-row exporter was verified with twelve Python controls and six Go test/subtest events. Those historical results are retained; no old native result is relabelled.

The expanded exporter has sixteen locally executed Python controls, including actual exporter → publication staging, byte preservation, tampered coverage receipt rejection, and independent failed coverage. These controls use explicit synthetic Git/source/signing/device inputs. They exercise the production observer/exporter/publisher without claiming Apple signature validity or native execution.

The candidate Windows runtime suite passed all54 tests in 230.59 seconds, followed by both new exporter/publication staging controls in 14.92 seconds. The actual Python exporter → Go intake and existing fast adversarial evidence tests passed53 named test/subtest events in 12.09 seconds. The first Go replay exposed the stale S01 tier authorization for the old Unicode fixture; that exact authorization was corrected, with its failed log retained separately. The tests preserve all179 accounting, require remaining local tiers, reject old IDs/changed values/missing bindings/borrowed requirements, and retain failed records. Logs are recorded under the isolated worktree's ignored `dist/p15-acceptance-causal/coverage-formal-*` and `coverage-publication-pair*` paths. These runs used the working change atop9a41ecce; final clean integration execution is a separate gate.

Fresh real Simulator evidence and complete final-candidate gates remain required. These local controls do not execute Apple Shortcuts and are never supplied as runtime acceptance receipts.
