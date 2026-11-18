# Canonical variadic parameter bounds

This follow-up preserves the earlier filter adapter commits and corrects a bounds defect found during integration review.

## Actual reproduction

The canonical URL definition has one variadic parameter. Repeated calls to the parameter expand the evaluated argument slice. With positions `[0, 0, 1]`, the first two values grow that slice to length two. The old remapping condition compared the third position with the growing slice, accepted index one, then read past the single canonical parameter while selecting its wire key.

Actual `CanonicalBackendSession.EmitResolvedCall` execution on source `56f35faec31e5215ee4b32a52d57c69d29c89ebb` reproduced an index-out-of-range panic for `[0, 0, 1]`, `[0, 0, 0, 2]`, and `[-1, 0, 1]`. Four healthy position sequences and three invalid fixed-parameter controls passed in the same run.

## Bounded repair

The remapping condition now compares positions against `len(def.parameters)`. The existing canonical name/key lookup and variadic fallback are unchanged. Appending evaluated variadic values no longer changes which indexes are valid canonical parameter indexes.

The repair is one expression change plus its explanatory comment. Filter conversion, action definitions, metadata encoding, omission behavior, and parameter error messages use their existing paths.

## Verification

The dedicated production tests exercise seven position sequences through the actual session and final plist serialization. They assert all values remain in source order, with their exact count and output UUID. Five deliberately corrupted native outputs must fail the independent oracle. Three invalid positions for a fixed-parameter action must still reject before emitting any action.

The immutable follow-up receipt additionally records the clean source/binary identity, existing filter and canonical controls, an old-bound overlay counterfactual, and unchanged normal V2/Legacy variadic CLI outputs against the retained pre-fix compiler.

This is compiler and native-structure evidence only. No signing, Apple runtime execution, external request, or formal requirement promotion is performed by this change.
