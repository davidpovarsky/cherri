# Supplemental native quantity case Q40

This standalone fixture provides a pure native execution target for the dynamic
quantity repair. The main 39-workflow manifest and generator are unchanged.
It requires no Clock app, timer, network, external input, alert or Show Result.
Its only terminal action is `output`.

## Independent expected result

The expected output was written into `manifest.json` before compilation or native
execution:

    CHERRI_QA_40|2024-02-05:2024-02-06:2024-02-06:2024-02-08

It is exactly 56 UTF-8 bytes, with no trailing newline. Its SHA-256 is
`d509ceed4e1a65059ab6da4006d3ff56c34d0ea83f2a64e2c9c619d4459f4f7c`.

| Position | Quantity magnitude | Expected Gregorian date |
|---|---|---|
| 1 | Runtime arithmetic output `1 + 1` | 2024-02-05 |
| 2 | Mutable `days = 3`, consumed before mutation | 2024-02-06 |
| 3 | Saved numeric snapshot of 3, consumed after mutation | 2024-02-06 |
| 4 | Mutable `days = 5`, consumed after mutation | 2024-02-08 |

All four offsets start from the same explicitly specified local noon,
`2024-02-03 12:00:00`. Native execution assumes the test simulator uses the
Gregorian calendar and retains the same local timezone during the workflow.
Parsing and formatting both use that local timezone; there is no UTC conversion
and no dependency on the current date. The operation adds calendar days, then
formats only `yyyy-MM-dd`. The fixture does not claim to configure or test other
calendar settings.

The original and fork Legacy sources differ only in the declared workflow name.
They explicitly materialize the numeric snapshot with `number(@days)`. V2 uses
its immutable `let snapshot = days` semantics, which emits a Get Variable action
before the later write. The three names are `CHERRI_QA_Q40_O`,
`CHERRI_QA_Q40_L`, and `CHERRI_QA_Q40_V`, allowing simultaneous fresh imports.

## Compile and inspect

Use the existing differential runner with this separate manifest:

    python3 -B tests/differential_30/run.py \
      --manifest tests/differential_30/quantity_runtime/manifest.json \
      --original /path/to/cherri-upstream-98d5191 \
      --original-sha 98d5191ae121a6bcf663ed0e55495f4ee4c7a6e2 \
      --fork /path/to/cherri-fork-4ca75e76 \
      --fork-sha 4ca75e763204f0d4cdcb94442985e6dd776df844 \
      --lanes original,fork_legacy,fork_v2 \
      --out /path/to/new-quantity-runtime-evidence

The actual run reused the clean pinned binaries, made three real CLI
compilations, and wrote untouched compiler bytes. All three outputs have
16 actions and valid producer/reference graphs under the corrected comparator.
All four Adjust Date `WFDuration` fields have `WFQuantityFieldValue` with
`Unit = days` and a bare dictionary `Magnitude`: action output, mutable variable,
action output, mutable variable. Both action-output UUIDs identify exactly one
prior producer. The snapshot action is index 6, before the second mutable write
at index 8 and its quantity consumer at index 9 (zero-based indices).

The original and fork Legacy action trees match after validated native UUID
mapping; workflow metadata remains different. V2 uses Get Variable instead of
Legacy's Number action for the saved snapshot, and its whole-workflow comparison
remains different. Full raw and typed differences are retained. Structural
checks do not establish native semantic equivalence.

`verification.json` pins the actual source, compiler, manifest, comparator and
artifact hashes. External evidence is retained under:

    /mnt/development/projects/evidence/cherri-differential-30-20261125/quantity-runtime/

`compile/results.json` records the CLI commands, exit codes, graphs and unsigned
artifact hashes. `compile/comparisons/` retains both complete comparisons.
`quantity-contract.json` retains the exact native quantity dictionaries and
verified producer/snapshot ordering. `runtime-preparation.json` provides the
three unsigned artifact paths, hashes, unique names and independent expected
output for the native owner to sign and execute.

## Native handoff

Sign each exact unsigned artifact and add the actual signed path and SHA-256 to
the native manifest. Preserve the listed name and expected output. Do not edit
the plist or derive an expected result from observed output. Require fresh
callback identity plus exact raw UTF-8 digest, byte length and result summary.
At the component handoff, all three native statuses are `NOT_RUN`; no signing,
simulator actions, app changes, native execution or full compiler suite occurred
in this task.
