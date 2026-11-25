# Statistics native input repair (2026-11-25)

## Observed defect

The independently specified differential fixture 15 contains the numeric list
[3, 5, 7], computes Sum and Average, and returns the exact text
CHERRI_QA_15|15:5. This expectation follows from 3 + 5 + 7 = 15 and
15 / 3 = 5; it was fixed before any native execution.

Both current upstream Cherri and the fork's V2 compiler produced shortcuts whose
native execution returned "CHERRI_QA_15|\n:" instead. Shortcuts reported a successful
execution callback, but the independent output comparison correctly marked both
runs OUTPUT_MISMATCH. Import identity was verified for each run. The native
callback receipts, exact output, expected output, signed artifact hashes, and
hashes of the original complete reports are retained in native-before.json.

The defect is inherited upstream behavior, not a V2-only regression. Frozen
upstream bcc0095754e2cc0baaeb4d93dae172b8153b4716 and current upstream
98d5191ae121a6bcf663ed0e55495f4ee4c7a6e2 both emit the same incorrect input key.
The current upstream and fork native failures are preserved; the fixture
expectation and original compiler outputs have not been changed.

## Native contract

Apple's checked-in capture at ../parameter-encodings.json defines the input
parameter of is.workflow.actions.statistics as Input, with parameter class
WFVariablePickerParameter and state class WFVariableParameterState. Its
operation key is WFStatisticsOperation.

A fresh read-only probe of macOS 27.0 build 26A428 loaded WorkflowKit and ActionKit,
located WFCalculateStatisticsAction, and confirmed exactly these two parameter
keys. It did not execute a shortcut or interact with simulator UI.

- inspect-statistics-02.js is the exact probe source.
- native-statistics-metadata-02.json is the exact native observation.
- observation-receipt.json records the host version, invocation, and source and
  metadata hashes.
- Metadata SHA-256: 3c04a499cdde75cd5d9e91299b9cb00f60b6d2ecc9ba86b9c11d69a3f7dee5ce.

The canonical action declaration now binds statistic(..., input: ...) to
Input. The registry was regenerated with go run ./tools/language-schema.
Both compiler modes use this definition. Number producer encoding and input
attachment shape are unchanged.

## Scope and regression checks

The before/after fixture 15 comparisons retain every typed difference.
After bijective UUID renaming, the only action-tree differences are removal of
WFInput and addition of Input for the Sum and Average actions. V2 retains
7 actions and legacy retains 11. Both old and new producer/reference graphs
are valid. These documents are deliberately reported as wire-different.

Focused checks cover all eight statistics operations in both compiler modes,
semantic and hybrid round trips, typed Number-to-List-to-Statistics references,
numeric values/order, and corrupt operation, key, producer, and reference
negative controls. A separate test reads the captured Apple metadata rather
than deriving the expected key from Cherri's own catalog.

The existing upstream oracle continues to assert that the pinned upstream
compiler emits WFInput; it now records the intentional fork correction to
Input explicitly and checks that other legacy fields remain equal. It does
not treat an inherited compiler defect as runtime correctness.

The isolated mutation gate restores WFInput in the actual canonical action
declaration. The corrected test rejects that mutation with
"statistic action missing Input wire key". The unmutated baseline passes.

Historical P10 reports describing WFInput as correct were based on upstream
compiler parity. They are historical evidence and are superseded by this native
observation and the differential execution failure. The active macOS parameter
disposition has been corrected; the historical raw reports remain intact.

## Native repaired execution

The fixed V2 and legacy artifacts have distinct names
CHERRI_QA_D30_VF_15 and CHERRI_QA_D30_LF_15. Their source programs differ from
the committed corpus only in the workflow display name. The expected output
remains CHERRI_QA_15|15:5.

Both corrected shortcuts passed actual iOS Simulator execution through Apple's
Shortcuts app. Each exact callback result was CHERRI_QA_15|15:5, with verified
import identity and receiverSelfTest=false. The root-owned native batch ended
with 2 cases, 2 PASS, no fatal error, and XCTest exit 0.

The unchanged compiler artifacts were signed and imported under their canonical
names. A prior harness attempt appended .signed to the exported filename;
Shortcuts used that suffix as the library name, so the requested run name was
not found. That import/runtime error is preserved separately under
runtime/differential-statistics-repair and is not counted as a statistics result.
The successful retry used the exact same signed bytes with canonical filenames.

- native-fixed.json contains both actual callback and import receipts.
- native-fixed-summary.json records the completed native batch result.
- native-fixed-manifest.json records the unsigned/signed artifacts and expected
  outputs used by the native harness.
- The original failing receipts remain in native-before.json.
