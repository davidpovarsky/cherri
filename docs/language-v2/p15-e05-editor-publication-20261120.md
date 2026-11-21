# E05 editor publication: exact request ownership

Tested source: 47630bedef7dc986af78ff293cac4109703f0847. Base: f03209a6b58b748add37aa09961cd21a8333e455.

## Actual execution

The portable probe compiled and executed the verbatim production WorkspaceView build method and CherriEditorView completion method. Only method visibility changed for extraction. Backend, signing and file-publication adapters were explicitly controlled; SwiftUI, the real compiler bridge, native Shortcuts and real signing were not executed.

The first clean14-control baseline had9PASS/5FAIL. The expanded24-control baseline had11PASS/13FAIL. The independent26-control baseline at36a419df had11PASS/15FAIL. The clean repair executed all26 controls with0 failures, plus7 actual Python intake/process test methods. All original raw outputs, generated Swift, compile/runtime receipts and binaries are preserved, including every failed baseline.

## Repair

A shared exact UTF-16 source/mode/name snapshot now owns live-task identity and guards publication. Each invocation owns a monotonically increasing generation, including same-source and A-to-B-to-A replacement. Cancellation is checked before dispatch and again after each await. Only the current generation clears busy state. The diagnostic result is committed after its analysis await and its renewed guard. A successful unsigned build clears the previous signed share URL; a stale result cannot publish a signed file after catalog refresh. The completion method is unchanged and its existing cancellation, source and caret guards pass.

The new helper is compiled by the real Swift compiler in the portable probe. The SwiftUI task modifier itself is source-reviewed but has no UI execution claim. Current success/error/signed-adapter positives prevent an implementation that suppresses every result from passing.

## Boundaries

This proves bounded local production control flow with controlled adapters. It does not establish a complete UI session, C bridge execution, Apple runtime, actual signer behavior, deployment or final179 acceptance. E05's required UI tier remains missing. All three immutable179 contracts are unchanged.

## Evidence

60 exact original files; 53 present Git witnesses and 7 explicit absence witnesses. Every actual probe input source digest matches its immutable Git blob. Original archive: 372849 bytes, SHA256 f1c1763d6f3facd4e31de86b5471aa379cf6a59207f3514db16fea4385bb315f.

See evidence/p15-e05-editor-publication-20261120/provenance.json for exact source, process, event, byte and archive bindings. No historical failed event is relabeled.
