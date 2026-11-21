# RP14: actual typed mutable and clipboard capture

Test source: `cbf542c3b62bc12e54372101436938b0c046ebf3`, clean before and after execution on 21 November 2026. The immutable RP14 requirement retains unit, native-structure and iOS runtime tiers.

## Verified execution

```text
go test ./tools/language-acceptance -run ^Test(CaptureOrder|TypedCapture|EditorContract(Rejects|Inventory|Strict)) -json -count=1 -timeout=6m
```

The command exited 0 in 80.359 seconds with 137 passing outer events, zero failures and zero skips. The new RP14 producer executed 45 required production events in 5.306 seconds: 11 unit/type events and 34 native graph events. The same run repeated corrected RP12 (65 events) and BRG10 (129 events), and retained the 104 editor inventory/rejection controls. Each actual production receipt binds the exact source, 86 source files, fixed event inventory, measured interval, command and raw outputs. Production and outer time limits are unchanged.

The new source is one actual compiled program with independently declared Number, Bool, Text, List and Map variables. It saves each mutable value, overwrites its source, then consumes the saved producer; Clipboard is likewise captured between two different writes. The unit producer requires the six saved semantic types and rejects Number-to-Text, Bool-to-Number and List/Map type substitutions before emission.

The native oracle checks each initial producer and later assignment, capture position and exact source variable, unique UUIDs, every producer-before-consumer edge, and final uncoerced references to all six saved values. It checks numeric zero, false, empty Text, mixed flat-List element order, and exact typed Dictionary fields including explicit empty List and Map values. Twenty-four per-field mutations change timing, reads, coercion or scope. Nine additional mutations change types, false/zero distinctions, order, empty-container presence, producer multiplicity or required consumers. All are rejected. Ten intake mutations further reject altered source, command, requirement, tier, artifacts and event inventories; unchanged relocated proof is accepted.

## Limits and retained boundaries

RP14 has two actual local records and remains `PENDING_EXTERNAL`. The local manifest report has zero complete PASS and one `NOT_RUN` row for the missing iOS runtime tier. Final-mode validation rejects local-only closure. No related requirement inherits these records, and all 179 immutable rows remain present.

Standalone semantic Bool uses the compiler's established Number 0/1 native carrier; the unit check independently requires Bool. Literal Bool values inside List and Map remain actual Boolean values, distinct from numeric zero in the serialized structure. These facts are not an observation of native runtime type or saved value on Apple. Actual iOS execution remains required.

An initial exploratory source with a nested native List literal was correctly rejected with `E_NESTED_NATIVE_LIST`. That original source and diagnostic are preserved. The supported RP14 fixture uses a mixed flat List and empty nested Dictionary field values. It does not claim native nested-List support, reinterpret that rejection as a pass, or change compiler production code.

## Original evidence

[Summary and digests](evidence/p15-typed-capture-20261121/summary.json) and [original raw archive](evidence/p15-typed-capture-20261121/original-evidence.zip) preserve the clean registrar run, all three production processes, their evidence records, and explicitly separate development-only probes. No private corpus was collected or uploaded.
