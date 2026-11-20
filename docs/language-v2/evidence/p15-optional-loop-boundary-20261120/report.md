# Optional elements in value-producing loops: bounded safety repair

The clean compiler source is **20cb2e2d34ebdbb6ea2b6e04c2498493efeee87f**. The implementation is **255cf7e526e7a830660ae7e2d1f899160d2de624**, followed by a separate non-circular provenance commit.

The original tests-only source **6c349a12e377e50c10307687d66cc7c9c85d9465** exposed 15 accepted Optional/None collection shapes. Their actual emitted plist bytes are preserved in the original Go event streams. A new positive fixture incorrectly called text with Text?; that additional failure was a test expectation error. The tests-only follow-up **5e2e1611b944b611c108d53d9b24c523e873967d** uses explicit Text interpolation and again reproduces all 15 unchanged negative cases, while 71 named positive and causal-control events pass. The first attempt is not discarded or retroactively reclassified as fully valid.

## Production behavior

The shared analyzer now rejects a statically Optional final result in a value-producing For or Repeat when the current Repeat Results path cannot carry a compiler-owned presence bit per element. It reports E_TARGET_VALUE_SHAPE on the exact final result span before native emission. Existing nested-List rejection, guaranteed-return paths, present scalars and Maps, statement loops and explicitly unverified Unknown/native output boundaries are preserved.

The formerly accepted Optional-yield function-discovery source remains an exact required negative. Its supported counterpart explicitly interpolates the Optional result into Text. The independent complete native graph includes that Text producer, the dispatcher, caller branch, iteration boundary and output edge; the existing 128 dispatcher corruption controls remain active.

## Executed evidence

| Attempt | Actual result |
|---|---|
| Original tests-only6c349a12 | 15 accepted-native boundary failures; 69 named passes; one additional invalid Text-conversion fixture plus parent |
| Corrected tests-only5e2e1611 | Same15 accepted-native boundary failures plus parent; 71 named passes; exit1; 15.890s |
| Dirty precommit component | 279 named passes; no failures/skips; 17.547s; not clean-source acceptance |
| Clean20cb2e2d root selection | 1,761 named passes; no failures/skips; 67.234s |
| Clean20cb2e2d full analysis/lower/types | 109 named passes; no failures/skips; 4.484s |
| Fresh compiler, unchanged official Legacy differential | All47 fixtures:9 VERIFIED_PARITY and38 PROVEN_EQUIVALENT; all17 actions; zero baseline/migration/compile/parity failures; 11.938s |

The root selection executes the complete diagnostic/service/no-artifact cases, present scalar whole graphs and42 graph corruptions, preserved broader Optional/value-flow tests, function-discovery graph/corruption tests, and the provenance registry. Diagnostic corruptions remove the required diagnostic, shift its source span or change its error code.

The fresh Windows compiler SHA256 is **4062de241f4f59a6c633926666d682d34208861cf4e8f58c14d9af362aa5d8d3**. The actual build and parity receipts bind the exact clean source and CHERRI_BIN path; no embedded VCS stamp is asserted. The unchanged parity report SHA256 is **53077e7d0d10ebe13c87f1a1138620d5a6f0dd5d8f729f8e73ba8026bf913338**.

## Preserved limits

This change is a current-transport safety correction. It does not establish that Optional elements are inherently impossible to encode, execute Apple's runtime, prove one item per iteration or zero-iteration runtime cardinality, support JSON null, or close T05/L01/F04/FN05. Present-value native graphs are structural evidence. Unknown/native-last-output boundaries remain unverified.

The immutable acceptance contract, existing Legacy fixtures, comparator policies, source/docs/consumer tuple and formal acceptance registrations are unchanged. Integrated CI and final179 acceptance must execute again at the composed source. The archive retains original byte streams and actual submitted scripts; derived decoded plist witnesses are explicitly distinguished from original files.

## Proof authoring and independent review

The first proof writer stopped on a nonexistent source filename; the corrected inventory records the actual CLI/session/emission adapters. The second stopped at Git's whitespace gate because generated proof documents used CRLF. Only new generated proof-document line endings were corrected; all original Go/process/build/parity bytes inside the archive remain exact. Both stopped authoring attempts and their actual scripts are retained. No semantic test was rerun for these authoring-only issues.

Read-only review by /root/acceptance of immutable5e2e1611→20cb2e2d: no concrete new guard/oracle hole found in the stated current-transport scope. Guaranteed-return bodies retain bypass; final semantic type drives Optional rejection without truthiness; List retains its existing diagnostic; final result span is used. Exact original discovery source remains required zero-byte rejection. Separate present/conversion/opaque controls and seven complete scalar graphs with six faults each retain count/input/body/output edges. Review only; reviewer did not execute this component or infer F04/container/Apple closure.
