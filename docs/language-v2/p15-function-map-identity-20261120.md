# P15 function Map argument identity repair — 2026-11-20

## Result and scope

The actual compiler now preserves an authored Map's semantic identity when it is passed to a user function. The clean final replay at `e06ad57372d2f448cc939fa6eb80d5f47dcf68ee` passed all 23 selected top-level tests: 175 named passes, zero failures, zero skips, exit 0, 15.875 seconds. The working tree and HEAD were unchanged before and after that replay.

This is local compiler and native-structure proof. It does not claim an Apple run, whole FN05/L01/BRG09 acceptance, JSON null support, nested native List support, or completion of any of the immutable 179 requirements. No acceptance registry, source fixture, runtime golden, or requirement contract was changed in this component.

## Reproduced defect

The original function argument path lowered literal Maps into untyped Go maps. Shared raw-parameter serialization could then interpret ordinary `WFSerializationType` and `WFColorRepresentationType` keys as native descriptors, interpolate authored literal braces, or omit the explicit row collection of an empty Dictionary. The same confusion affected the aggregate snapshot created before a later argument wrote a referenced variable.

A tests-only commit, `21ef2c0a7b61672fef43fc348d42547e92c42a91`, retained the failing oracle before production changes. Its clean replay exited 1 in 4.328 seconds. Five literal Map cases failed: empty, native-looking attachment key, native-looking color key, literal braces, and nested values. The later-write snapshot case also failed. The explicit `native.plist` and unsupported-authored-null controls passed. Raw output SHA-256: `2d0678919f7040e8d6bb1f69157f26771cea12f3f2e39fc53f25a5d0834da98d`.

An earlier exploratory replay against the parent with an untracked test is retained separately as exploratory evidence. It is not a clean-source proof.

## Production correction

Commit `e52f50c1f7fce6b37377b8f5d7daed45177f5b0e` changes only the user-function argument entry point and adds a 37-line helper. Literal Map arguments retain their AST-derived semantic value. If later arguments may write mutable references, the compiler materializes the typed Dictionary before those arguments and passes its ActionOutput reference. Scalar, native-reference, and List argument paths retain their existing handling.

The existing explicit `native.action(..., parameters: native.plist(...))` route already preserved opaque templates and remains unchanged. A plain native.action parameter map has separate semantic convenience encoding; it was not used as an opaque-template oracle.

## Preserved broader failure and narrow characterization update

The first clean broader replay at `e52f50c1f7fce6b37377b8f5d7daed45177f5b0e` recorded 150 named passes and exit 1. Its only failing test parents were `TestP15T01FunctionPresenceEnvelope` and `TestP15T01DefaultsGuardedByPresence`. Their old characterization explicitly expected the historical empty Map wrapper with `Value: {}`. The corrected compiler emitted the typed empty Dictionary representation with `WFDictionaryFieldValueItems: []`.

Commit `e06ad57372d2f448cc939fa6eb80d5f47dcf68ee` updates only that exact Map characterization and adds a negative control that rejects the historical omitted-row shape. Existing List encoding expectations, presence markers, default guards, runtime fixtures, and runtime goldens remain unchanged. The failed broader replay is retained byte for byte; raw SHA-256: `8af2b4220a20bdd6558362af57f370f18f678c3f422e03e9db71d1cbc1db0a09`.

## Final verification

The final clean replay includes the six new Map argument/template/null/negative-control parents, all six existing FN01–FN03 parents, both T01 parents, eight C01 source-order/default/capture parents, and the V01 declaration snapshot parent. All 23 selected parents were discovered in the actual Go JSON stream. The new producer-order test requires each argument effect exactly once, the first Map producer before the later argument producer, and exact native references in both envelope positions. The broader C01 replay also verifies default effects and argument snapshots.

The hand-authored typed Dictionary oracle covers nested Dictionary/array structure, empty arrays and Dictionaries, false/zero/empty text, literal keys, native references, and precise wrapper shape. Nine corruption controls reject native-token confusion, dropped fields/empty collections, reordered rows, false-to-zero substitution, missing presence, extra wrappers, and compatibility-array flattening.

Raw final output SHA-256: `776ed2e47fd3e09de176790200ed8a5ba9e14c9d1679b1fab9f48c6630d8c47c`. The original raw stream, stderr, command, selected parents, timings, source SHA and cleanliness receipt are retained under [evidence/p15-function-map-identity-20261120](evidence/p15-function-map-identity-20261120). `artifact-manifest.json` binds all 18 copied original files by exact bytes and SHA-256. Precommit runs are retained as such; they do not replace the clean red and green proofs.

## Remaining scope

The current authored-null diagnostic remains an explicit unsupported boundary. Whole structured return, Optional, nested native List, loop collection, and Apple runtime contracts still require their own complete executed evidence. This bounded repair does not grant credit to linked requirement IDs.
