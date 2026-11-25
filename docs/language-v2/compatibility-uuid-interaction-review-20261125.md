# Quantity, compatibility and UUID interaction review — 2026-11-25

This is a read-only source interaction review, not a repeated test execution or
an Apple-runtime result. Reviewed integrated root source:
`ba97d7a882e6fa71521596350e38b1dd38e504b2`, containing compatibility implementation
`9eb369a37c807512489055991c9dc7e9503c25f7` and quantity implementation
`52ffccd5683ba5dfb357745dbfe2f544c29e7bc7`, plus the component UUID audit and its
retained results. The original/current-upstream worktrees remain unchanged.

## Cross-component paths inspected

| Path | Source finding | Limit |
|---|---|---|
| Legacy compound references and generated output identity | `makeOutputName` reuses an operand name only for ordinary assignments. Computed actions receive a distinct output name; `makeVariableInput` assigns from the output. `nextGeneratedUUID` adds an occurrence discriminator without changing the existing output-reference cache. | Root integration and native arithmetic checks remain required. |
| Typed quantity magnitude transport | `lowerQuantity` stores `backend.SemanticValue`; `quantityActionArguments` converts it through the shared backend; `makeQuantityFieldValue` emits a bare reference via `variableValueWithSerialization`. No path stringifies or regenerates an already allocated producer UUID. | Existing quantity focused evidence is reused, not reclassified as new native proof. |
| Named quantities and later writes | Constructor binding retains `SourceIndex`; `callWritesAfter` uses that source argument index. `captureCallSemanticValue` still visits typed quantity magnitudes/units and uses `captureCallReference` for mutable storage before later writes. | No claim of coverage beyond the preserved snapshot tests. |
| Color and invalid constructor handling | The analyzer rejects unsupported constructors before session emission. The lower-only path also validates. Accepted color components are finite literals and consume no generated identity. | No dynamic color schema was inferred. |
| Compilation lifetime | Legacy configures generated IDs at `initParse`; authoritative reset clears the sequence. V2 initializes the canonical registry before configuring the derived sequence, then shares its generator with native expansion. | Source review confirms placement; repeatability results belong to the UUID owner's actual receipts. |
| Imported explicit identity | `Lowerer.GenerateUUID` returns imported provenance before consulting the optional generator. Quantity adapters preserve typed reference identity. | Imported-native tests remain separate from authored runtime tests. |

No new interaction defect was identified in these paths. The review does not
change frozen expectations, bypass native gates, or substitute structural
checks for expected runtime output. Root owns the integrated full suite and
native execution.

## Typed differences remain evidence

The current-upstream device-property probe adds `PropertyUserInfo: 0`; the
frozen/fork shared codec omits it. Explicit integer zero and absence remain
different typed values. Their native impact is **unknown** in this review.
They receive no blanket normalization, cosmetic label, harmlessness assertion,
or equivalence claim. The full raw and identity-renamed diff evidence remains
untouched; only an appropriate independent native check could qualify impact.

## Native Legacy manifest generator qualification

The separate generator uses only real compiler invocation for `.shortcut`
creation and only the existing Cherri signer manifest schema for finalization.
Static controls validated all 37 name transformations with every remaining
source byte unchanged, rejected three malformed/ambiguous name inputs, and
rejected hash mutation and path escape. No final compiler, signer, native
runner or full test suite was executed by this reviewer. Receipt:
`/mnt/development/projects/evidence/cherri-differential-30-20261125/compatibility/native-generator-static-controls.json`.
