# P10R15 source-adoption checkpoint — 24 November 2026

The published K17 compiler source has genuine CI failures, and four bounded repair/validator qualifications are now complete. This checkpoint records those originals before the repaired source is adopted. It does not declare a new source commit, successful replacement Build, completed P10 refresh or complete 179 acceptance. The last complete original K476 result remains **111 PASSED / 4 FAILED / 28 NOT_RUN / 36 PENDING_EXTERNAL**, `closure_complete=false`.

The [public metadata projection](evidence/p15-oracle-checkpoint35-20261124/source-adoption.public-projection.json) binds exact original paths, bytes and SHA-256 hashes. It contains public metadata only. Raw job logs, private corpus inputs, ciphertext and credentials are not copied into this document or projection. The [previous continuation](p15-k17-semantic-continuation-20261123.md) and masterplan chapters through 34 retain their historical cutoff and source identities.

## Published source and the two publication records

The frozen source is `5c996a62f349e800e5b1c7f789ee1b3873b4c3eb`, tree `a50f7a49655bf3dfb743c52dbe829e5defdac7c7`, published under `refs/heads/agent/cherri-p10r15-k17-compiler-source-20261123`. The earlier clean N03 source `6172184eac0f53c05268d0df4641772a905ffd09` and published K16 `949b758abf5b6bee073940216dc498a86d84385f` remain distinct historical sources.

The original Windows publication contains nineteen process records and one successful push command. Its later API command timed out, so its genuine result remains `PUSH_ATTEMPTED_REMOTE_OUTCOME_UNVERIFIED`. A separate nine-command read-only reconciliation subsequently proved the actual remote source ref, commit and tree. It performed no import, push or workflow dispatch and left the original files/status unchanged. Admission consumes both independently pinned records; it does not turn the failed original into a successful nineteen-step run. References: `sourceFreeze`, `publicationOriginal` and `publicationReconciliation`.

## Actual S Build: both platform failures remain originals

[Automatic Build 45286107677](https://github.com/davidpovarsky/cherri/actions/runs/45286107677), attempt 1, was triggered by the published source. Linux job `138082721848` and macOS job `138082721363` failed; Nix job `138082721971` succeeded. Both full Go suites failed before the later P10 steps, which were skipped. The status observer's successful terminal observation is not workflow success.

The two original full-suite receipt ZIPs are each 1,093 bytes: Linux artifact `14876335206` and macOS artifact `14875739834`. The acquisition retained their run/source/attempt identities. Both original job logs were recovered from the already acquired authenticated cache, with no second log download, and independently joined to the receipt metadata. The Linux log is 10,792,351 bytes; the macOS log is 10,751,182 bytes. The diagnosis preserves both failures and all original streams. References: `automaticBuildObservation`, `buildFailureDiagnosis` and `buildFailureDiagnosisManifest`.

The failures shared by both platforms include:

- `TestP10_ConditionalGetValueStringComparison_EmitsStringCoercionAndStringTarget`: the old condition-count oracle expected eight while exact-Text lowering added a real inner comparison guard.
- Both F04 guaranteed-return loop cases: lowering rejected a List/Optional result shape even where the loop body cannot normally complete with a yielded result.
- RP18/BRG13 native graph checks: the old cursor did not consume the exact-Text comparison region.
- Optional-return completeness `implicit_none/branch_true`: the old producer decoder missed the new comparison's actual result identity.
- StorageProbe: the old allowlist rejected the intended `text.replace` comparison helper and did not model its connected graph.

The acceptance short-circuit producer failed on both platforms. macOS additionally failed the supplemental-obligation AI harness cleanup absence check. The retained logs show actual test/producer failures; they do not show an ordinary 15-minute or acceptance 35-minute Go timeout or a cancellation. No timeout was enlarged and no unchanged Build was rerun. K16's earlier stale P10 fingerprint failure is a separate historical outcome and is not used as the cause of this S Build.

## F04: normal completion before result-cell demand

The narrow production correction makes the loop lowerer consult its existing normal-completion predicate before applying the existing List/Optional cell-demand decision. The analyzer already distinguishes a body that returns on every reachable path from one that can finish with a loop result. The correction brings lowering into that same decision; it does not introduce another representation or weaken the shape boundary for bodies that can complete normally.

The proposed three-file overlay preserves Repeat/For count and iterable semantics, return scope, yielded outputs and the zero-iteration continuation. It passed **246 named tests across 20 parents**, in two actual Go commands: the root affected selection and lowerer selection. The independent intake reconciled the complete original streams and exact name inventories. All 725 source pins and 623 raw source blobs remained unchanged on clean `5c996a6`; no source adoption occurred during qualification. References: `f04Qualification` and `f04IndependentIntake`.

This repairs the observed guaranteed-return rejection. General nested List/Optional loop-result sequences, function/native/aggregate interfaces and other unimplemented shape behavior remain open under F04/T05/L01. Their existing local unit/native-structure minimum is unchanged; this checkpoint adds no new Apple tier and grants no complete F04 acceptance.

## N03 and Storage: consume the real exact comparison graph

The six-file overlay changes test/oracle code while leaving the source's production exact-Text lowering intact. Its shared comparison helper reuses the existing exact primitive cursor and binds the complete ten-action comparison region, actual operand producers/descriptors, canonical identities, result UUID and immediate Boolean guard. It returns the validated graph boundary to consumers, so they can distinguish authored conditions from comparison-internal conditions without ignoring extra or disconnected actions.

P10 retains its eight authored conditions and all dynamic, Number and missing-value routes. RP18/BRG13 consume the whole native graph and retain all 166 old native causal children. Optional-return checks retain their carrier/output decoder controls and add three targeted cases. Storage retains its authored 2/4/2 condition counts, storage operations, keys, scopes, Unicode and readback semantics. It recognizes only the connected 1/2/0 exact-comparison regions in the three fixtures. Operand and `getitemtype` descriptors bind the ordered real source producers; extra aggrandizements remain invalid.

The actual qualification passed **410 named tests / 18 parents**, one Go command and **1,709 JSON events**, with no failed, skipped, duplicated, foreign or missing named result. Storage's **55 inline faults** comprise the original 36 controls and 19 additions; these are internal causal controls, not another 55 requirements. The valid copied specimen is accepted before each mutation, avoiding a vacuous rejection caused by numeric JSON type changes. Original N03 complete-cursor tests were included because the helper's literal-left-operand signature changed.

Independent intake verified all eleven process receipts, twenty-two streams, exact event/name inventory and absent process groups. The same 725 source pins and 623 raw source blobs were unchanged; the new helper remained absent from W17 before and after. No baseline/full suite/native/provider/acceptance producer ran in this qualification. References: `n03StorageQualification` and `n03StorageIndependentIntake`.

## AI cleanup: a Linux-only process-absence qualification

The cleanup correction passed the two actual Python unittest methods `test_hung_adapter_times_out` and `test_timeout_stops_a_spawned_worker`, together with six POSIX synthetic controls. It preserves the tests' original 0.05-second hung-adapter and 0.6-second descendant limits and the production five-second cleanup deadline. The focused command exited zero and its owner process group was absent afterward. Reference: `aiCleanupQualification`.

This result is explicitly `PASS_FOCUSED_LINUX_ONLY`. It called no provider, ran no Go command and adopted no source. The actual macOS failure remains open pending genuine source-bound macOS execution. These process controls do not supply AI01/RP50 measurements, replenish a consumed cohort, permit reseeding or authorize provider retries. R23v1 remains active and R22 retains its platform boundary.

## Canonical publication admission: preserve separate original states

The existing reconciled-Windows canonical extension passed **80 named tests / three parents**: the three parent results, 24 legacy children and 53 new synthetic children. One parent consumed the actual independently pinned original publication and reconciliation records. The complete 88-file original input inventory and the legacy strict branch were preserved. The independent intake verified the original 324 JSON events and complete process streams. References: `canonicalReconciliationQualification` and `canonicalReconciliationIndependentIntake`.

A new explicit Oracle atomic publication route is being prepared separately. It has no qualifying result or source-adoption credit at this checkpoint. The earlier 80-name result qualifies only its original candidate and cannot be restamped onto a later adapter. Any later route qualification must bind its own exact source pair, command inventory, original outcome and rejection controls. A subsequent pin addendum may record that actual evidence without changing the old result.

## Next frozen source and the preserved production sequence

The planned integration is thirteen source/test files from the bounded repair groups plus four documentation files. The selected future branch is `agent/cherri-p10r15-k17-ci-repair-source-20261124`. Its actual commit/tree, refreshed witness and publication result are intentionally unbound here. The original component overlays are evidence for their exact proposed bytes; source integration still needs its own complete before/after identity and tracked-file inventory. New test-helper files require a real regenerated source witness, not a guessed next pin count.

After the source is actually frozen, continue the already defined P10 chain in order:

1. Produce durable raw 505 on the selected source, then independently intake and adopt raw C0.
2. Generate and adopt the seven derived outputs C1; perform the canonical refresh on C1 and adopt its three ledgers C2.
3. Verify clean C2 while retaining the canonical receipt's actual C1 source. Run the actual 56 gates, independently intake/adopt their originals and verify-commit.

The independently prepared ordinary full-history source clone and two ARM64 compiler/migrator builds remain separate. Their source topology, full inventory, actual VCS metadata, original 600-second build limits and 2 GiB output/cache floor stay intact. Full Go preserves dynamic all-package inventory and its original ordinary 15-minute / acceptance 35-minute children. A new outer owner is not an executed suite. Full179, all five new-source S4 receipts, native/runtime observations and final delivery contexts likewise need their actual source-bound producers and intakes.

Documentation candidate evidence is separate from main/Pages promotion; installed compatibility and Skill fingerprints must come from actual accepted source/binary/package producers. A prepared candidate, observer, consumer or packaging owner does not supply missing production results. No main/Pages promotion, package creation, full producer or final-source declaration occurs in this documentation preparation.

## Capacity: fully allocated device extent, not a free-space claim

The read-only capacity proof reports `/dev/sdb` as a 107,374,182,400-byte (**100 GiB**) device. Its directly mounted ext4 filesystem has 26,214,400 blocks of 4,096 bytes, exactly the same size. Reserved blocks are zero; the device minus filesystem extent and additional whole-block capacity are zero. There is no already allocated gap that a filesystem-only growth operation could recover. Reference: `allocatedCapacityProof`.

This is not a new free-space measurement or proof that the filesystem has zero free space. It does not identify a cloud volume, establish cloud-resize capability or perform a resize/reserve/service change. The last N03/Storage focused qualification observed 1,655,173,120 bytes free before and 1,602,834,432 after. Those original observations satisfied its selected **1 GiB focused** admission. They do not establish a later producer admission, and no **2 GiB production** floor is reduced. Future source staging and producers require fresh checks under their own original gates.

## Remaining scope and truthful acceptance

| Scope | Required continuation |
| --- | --- |
| N03; local RP39/RP41 | Run the existing local producers on the actual updated source. RP39/RP41 also need dedicated complete rendered UI producers and actual execution. |
| T05, L01, F04 | Preserve bounded observations and close genuine lossless null/presence, nested-shape and general loop-result interface gaps. Component repairs do not close the full obligations. |
| RP40 | Acquire actual same-source dedicated hosted-app and UI originals through the existing separate producers/intakes. |
| D03; CI01/RP47; CI02/RP48 | Complete installed/published consumer evidence, actual macOS negative-encoding evidence and computed native runtime results. |
| CI03, CI04, END01 | Supply independently pinned workflow/publication/final-delivery contexts and a finalized same-source ledger. |
| RP20, RP26, BRG15, BRG25–BRG28 | Implement and execute complete independent joined scenarios. Related bridge or build probes remain narrower evidence. |
| RP49, BRG31 | Complete final combined artifact/repository/packaging/security/native validation beyond the CI-only route. |
| B01, BRG01 | Keep missing October contemporaneous originals explicit. Later snapshots cannot reconstruct them and do not prove unauthorized deletion. |
| AI01/RP50, R23v1 and R22 | Preserve genuine measurements/failures, policy/baseline, actual platform scope and consumed cohort/no-reseed/no-retry decisions. |

The historical complete K476 run and its 36 report/audit contradictions remain unchanged. Focused name counts, inline controls, original-artifact validation and prepared owners are not additive formal coverage. No fresh 179 result or final END01 delivery is claimed. The next source adoption and its later production receipts must be evaluated on their own actual identities.
