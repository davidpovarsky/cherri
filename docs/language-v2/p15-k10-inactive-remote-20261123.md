# K10 inactive Remote Builder intake — 23 November 2026

This checkpoint retains an unsuccessful complete entrypoint run and a successful signing fault matrix. The live Remote Builder was not activated. K10 cannot satisfy the promotion gate: the original entrypoint result is **38 PASS, 1 FAIL, 0 NOT_RUN**. The observer repair below has focused causal evidence; it does not rewrite that original outcome or substitute for a new producer on the final coherent tuple.

## Exact source and runtime

The inactive label is `p15-r7-d11-k10-20261123`. Its compiler source is `ca316a3dee252456c4bec751594cf249c709b331` (R7), documentation source is `365b08c5b4c71c6d22b591babb8d1ab852e5a3b9` (D11), and Skill source is `4feadab9db4d351628f78593304373631693869a` (K10). The exact ten-field tuple, binary hashes, report digests, actual start/finish times and process receipts are in [the bounded metadata intake](evidence/p15-k10-inactive-remote-20261123/intake.json).

The actual compiler/migrator pair was built from the same clean R7 source for Linux ARM64. The inactive inputs were ordinary owner-safe source copies. All producers ran as the real Remote Builder account with its existing default HOME; Go used the retained 1.24.13 toolchain and development-volume caches.

## Actual producer outcomes

| Original phase | Actual result | Elapsed seconds |
|---|---|---:|
| Inactive staging | PASS; exact paired tuple; not activated | 5.916 |
| Source package integrity | PASS; 23 members | 0.116 |
| Runtime package integrity | PASS; 29 members | 1.473 |
| Unpacked runtime integrity | PASS; same 29-member archive | 0.317 |
| Full installed entrypoints | 39 rows: 38 PASS, 1 FAIL, 0 NOT_RUN | 125.897 |
| Installed signing fault matrix | 58 rows: 5 controls PASS, 53 expected rejections, 0 failures | 245.627 |
| Canonical candidate smoke | NOT_STARTED, held for final coherent tuple | — |

Package integrity is not an extracted runtime self-test. Source/runtime ZIPs, the unpacked package, every original report and raw process stream remain on Oracle outside Git. No private corpus, provider data or key material is included in this metadata intake.

The required `runner_existing_tests` row actually passed in **67.435 seconds**, with exit code zero and its original 600-second limit. It executed the installed runner's original `TestCherriNoSign` and `TestDecomp` commands. No `--skip-go-tests` option was used.

The signing matrix ran once from 02:55:32.513454 UTC to 02:59:38.142751 UTC. It retained all five surfaces, all three timeout cases and the original fault inventory. These were loopback fault injections: **zero canonical public signing commands** have been consumed by this K10 lane.

Across all six completed producer phases, the exact protected live hashes, ownership/modes, explicit Legacy default and registered relative Legacy link remained unchanged. The S/D/K inputs remained clean and at their exact pins. No activation, rollback or reapplication was attempted.

## Reproduced observer defect

The only failing entrypoint row was `runner_compile_unsigned`. The actual command succeeded and produced a public synthetic native graph with wire order `gettext → showresult → nothing`. The producer literal, UUID and consumer attachment matched the exact original fixture. The final parameter-free Nothing represents the current V2 main-void boundary; its empty parameter dictionary is omitted in the actual plist. The old oracle incorrectly required a two-action graph, so the row recorded a genuine observer failure despite the command's successful exit.

The original artifact SHA-256 is `3a94771eac34bd08973964af1b778417ee571f88fefb4bc6821f421b8350f019`. A read-only comparison of those unchanged bytes returned false with the original K10 helper and true with the repaired helper. It invoked no compiler or signer. That comparison explains the defect; it is not a replacement PASS report.

The fix commits are `e780fefa113ab441440b040f0cc3d118c7eeaa85` and the strictly scoped follow-up `c697476374ef3e6659be62ec390594cdb609f8bd`. They change only the Remote Builder observation path and its causal controls:

- V2 requires exactly one terminal Nothing with no parameters and no unreviewed top-level metadata. Only the identifier and optional empty parameters are accepted before the terminal action is excluded from the producer/consumer comparison. The input object is not changed.
- The original exact text, producer UUID, consumer token and attachment checks remain intact. Interior, duplicate, malformed, annotated and unknown terminal actions are rejected. Legacy behavior remains unchanged.
- The entrypoint receipt consumer requires its sorted ID inventory `[gettext, nothing, showresult]` and exactly three actions. The smoke receipt consumer separately requires wire order `[gettext, showresult, nothing]` and exactly three actions. Both retain the exact three-action Legacy graph.

## Focused causal evidence

The original helper failed the native-boundary controls before the fix (three failures and two malformed-input errors). The complete in-memory gate model independently reproduced the stale entrypoint predicate. After that predicate was repaired, a separate smoke-only control reproduced the stale smoke predicate. All original red streams remain retained.

The repaired helper and strict evidence gate then passed **16 test methods** in 0.363362 seconds of supervisor time (0.179 seconds reported by unittest). Those controls include the existing tuple, inventory, provenance and rejection checks plus separate entrypoint/smoke graph corruptions. After review requested an exact terminal-key allowlist, the scoped follow-up passed **six affected boundary methods** once in 0.098535 seconds. Previous receipts were preserved.

These are offline Python controls, with no native runtime acceptance or additional Go suite. The source worktree was clean after both implementation commits.

## Final tuple remains required

The existing promotion protocol requires one complete, source-bound 39/0/0 entrypoint report and the exact 58-row signing matrix, 23/29 package inventories and eight-operation canonical smoke. `remote_release_gates.py` binds these reports to the complete current staged tuple and actual wrapper/package hashes. The collector has no evidence-reuse intake; the old audit fixtures intentionally cannot be used to restamp a fresh report.

Accordingly, final K11 staging must produce its own original package receipts, one full 39-row run, one 58-row matrix and one eight-operation smoke, with exactly two canonical public signing attempts and no retry. The K10 originals stay historical and unchanged. At this checkpoint K11 is not frozen or staged by this lane, and no project-closure or live-promotion claim is made.
