# RP21 / BRG16: observed local component evidence

The corrected source completed its fixed local component run: **26/26 outer events, 234/234 RP21 events and 213/213 BRG16 events passed**. The source-bound producer report still records both full requirements as `PENDING_EXTERNAL`, with only `unit` and `native-structure` executed. This packet grants no Apple runtime or full179 completion.

This packet retains three distinct attempts. The first failed before any job was created. The second executed the original scenario and exposed a wrong RP21 oracle expectation. The third executed the narrowly corrected oracle and its two new causal controls. Earlier observations are preserved without retroactive replacement.

## Observed runs

| GitHub run | Subject / workflow source | Actual result |
| --- | --- | --- |
| [44972886028](https://github.com/davidpovarsky/cherri/actions/runs/44972886028) | No subject execution / `02a359b21ba0a0f2cb196e8e8fcbd2129c60a86f` | Failure before jobs: 0 jobs, 0 artifacts, 0 check runs. No compiler or scenario execution. |
| [44973222943](https://github.com/davidpovarsky/cherri/actions/runs/44973222943) | `1dcc7b3f97da992200fdf6b627f2c67090ea5298` / `5ef9d581b4ea09832c6b9bfd927a4a06cbbe9057` | Outer: 3 passed, 1 failed, 22 unstarted. RP21: 112 passed, 9 failed, 111 unstarted out of 232. BRG16 independently passed all 213. No timeout. |
| [44975916730](https://github.com/davidpovarsky/cherri/actions/runs/44975916730) | `06b2d2d5ee9c8ad33ee4fd77beb2a9c6af1b6b8b` / `d4c1f7972949040851adbb6d5c47b84be7ba0f56` | Outer 26/26, RP21 234/234 and BRG16 213/213 passed. No failed, skipped, unstarted or unterminated fixed events; no timeout. |

The initial workflow used `runner.temp` at job-level `env`. The retained one-line correction uses `github.workspace`, outside the two checkout directories. GitHub's [context availability documentation](https://docs.github.com/en/actions/reference/workflows-and-actions/contexts) omits `runner` from the contexts permitted at `jobs.<job_id>.env`. This is a source/specification diagnosis; no server annotation proving that cause was observed. The initial run metadata and workflow bytes were retrieved again during packaging and are explicitly labelled as a fresh read, not contemporaneous evidence.

## Causal RP21 correction

All eight executed dynamic RP21 cases showed the same concrete native representation: the initial mutable Number producer has **no `CustomOutputName`**, while its following Set Variable input refers to that UUID with **`OutputName: "Number"`**. The original oracle expected a named producer and rejected the actual graph. The preserved compiler lowering explains the distinction: a mutable binding does not assign the immutable binding's current output name, and its resolved action reference retains the schema title.

The [three-file source correction](https://github.com/davidpovarsky/cherri/commit/06b2d2d5ee9c8ad33ee4fd77beb2a9c6af1b6b8b) changes only the expected mutable producer and independently constructed consumer reference, plus the fixed event count and event inventory. It retains strict action parameters, numeric wire kinds, UUID/order checks, and all bounds/guard predicates. No compiler source or BRG16 oracle changed.

Two new RP21 controls prove both sides of the distinction independently. `mutable_number_producer_name` adds the forbidden producer name and must fail at the exact Number native-field gate. `mutable_number_reference_name` removes the required consumer name and must fail at the exact Set Variable native-field gate. Each first checks the actual healthy pair and retains valid UUID references and order. The fixed RP21 inventory therefore increases from 232 to 234; BRG16 remains 213.

The subsequent CI update changes only the exact subject pin in the driver and workflow, and the RP21 count from 232 to 234. The 109 source witnesses, 26 outer events, BRG16 inventory, process helpers and original budgets remain unchanged: nested Go 3 minutes / outer process 4 minutes, outer Go 9 minutes / supervisor 570 seconds, CI execution step 600 seconds.

## Independent reconciliation

The retained root verification separately parsed original process receipts and duplicate-key-free Go JSON, checked the complete fixed identities, nanosecond UTC bounds and event order, and matched all 109 source witnesses. Full subject and workflow Git trees, index modes/flags and status were independently reconciled with fresh Git API tree responses. The corrected subject contains 6,333 tracked entries and its workflow source 6,352. All 33 metadata-process originals were verified; subject and workflow remained unchanged before and after execution.

The successful outer process took **47.967776 seconds**; RP21 took **31.969016548 seconds** and BRG16 **1.834117793 seconds**. The owned outer process group was inactive after reconciliation. This observation does not claim independent proof of every nested process group.

| Original stream | SHA-256 |
| --- | --- |
| Original failed-run outer Go JSON | `9db078bb8c13297dd6852364f417db19a7791103149ed4b02a9bbec2971120a2` |
| Original failed-run RP21 Go JSON | `b298020a6de4c7c428f936129cc3a40f971bba09415d44c050863d4836448220` |
| Original failed-run BRG16 Go JSON | `f687fb2180bd272a33147372437627fd6dd16e73a8f8fe8bc2ba8d522e52218a` |
| Successful outer Go JSON | `c15411a19e11ef90ae7722a8746a2492100255259b811d7e555b138ca4ac7440` |
| Successful RP21 Go JSON | `5453027348514aef9948fb47f15263c90a874c602951c5b00c1c6df44cf4fc71` |
| Successful BRG16 Go JSON | `b01e3ef52f29b222b5f8a5f2c54f98c39987dffeb8ed5e27d66e90ac3d65b223` |

The actual local producer report is timestamped **2026-11-22T02:25:16Z** and binds subject `06b2d2d5ee9c8ad33ee4fd77beb2a9c6af1b6b8b`. Its four records support two local tiers for each requirement. Its summary has 0 passed cases, 0 failed cases, 2 skipped cases and `closure_complete: false`; both rows explicitly remain `PENDING_EXTERNAL`. The original required tiers are unchanged: RP21 `unit+native+iOS`, BRG16 `unit+native-structure+ios-runtime`. The root's independent successful-run reconciliation was completed later, at 02:35:45 UTC.

## Preserved originals

`originals.zip` is **5,141,459 bytes**, SHA-256 **`38905715741f037b91c3b933fb515763d08aee9dd6b2f54ad5f09c7f54af88e4`**. Its 24 members include two byte-identical GitHub artifact ZIPs containing 221 original files in total, the separate independent root reports and verification scripts, fresh Git/API responses, the failed job log, the source and CI diffs, and the eight original producer/reference observations. `manifest.json` provides every outer member's exact size and digest; each nested artifact has a separate complete member hash inventory.

| Retained GitHub artifact | Original size / SHA-256 |
| --- | --- |
| Failed execution, artifact 14734687066 | 1,974,527 bytes / `7cb889a5d35d1324143acae426393e7a849e1fd32597e828d01e72a9547fc6c1` |
| Successful execution, artifact 14736036441 | 1,994,570 bytes / `8b486bee29926399d24b0947b213d7e35bfe42808f3bf774983b8f4ea05a23b8` |

The `runs/<run-id>/root_verify_originals.py` scripts consume the corresponding artifact extracted into an `originals` directory beside that script. They execute no tests or compiler. They refuse to replace the preserved `root-independent-intake.json`; any fresh audit must use a separate working copy and preserve that original report first. Packaging checked all archive member bytes and all 221 nested originals without another Go, compiler, Apple or model execution.

The remaining Apple runtime obligations and the final combined full179 run are outside this component proof. No other requirement inherits these local results.
