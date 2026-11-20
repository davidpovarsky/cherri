# P15 raw JSON null boundary: real data-loss regression and repair

## Result and scope

Clean production source 3abfeee3fc15b10624839e87d8188e7a1c3d2cc6 rejects authored JSON null even inside native.action/native.plist, and rejects decoded JSON null in native.plist before the serializer can omit a dictionary field, shrink an array, or invent an empty Text value. A literal string containing JSON null remains exact Text, including its false, zero and empty values.

This closes one observed compiler data-loss defect. It does not complete the full T05, L01, FN05 or RP20 contracts, grant Apple execution evidence, or change the immutable 179 requirements. No producer registrations or runtime fixtures/goldens were changed.

## Preserved clean failure

Tests-only source 69804fe0e3335f6c6e466002b046b8201977f2e9 executed the actual analyzer, document service and CompileSourceToPlist boundary. The original clean run exited 1 in 20.938s. All eight null cases failed their unchanged no-data-loss expectation; the positive JSON Text case passed. The raw stream SHA256 is 5f58519be2831edd346f28cd7f80b5551c9ee0720020b7bd99eef2e8a5f04107.

The eight original native plists remain in this directory. Independent Python plistlib decoding recorded:

1. A raw top-level Probe:null field disappeared while Present:false survived.
2. A nested raw map's null became an empty Text item.
3. A raw array's null became a second empty Text item after the authored empty string.
4. A direct native.plist map's Probe:null field disappeared.
5. A valid preceding action did not prevent the later null field from disappearing.
6. Encoded JSON passed to native.plist lost its null field.
7. Encoded nested JSON lost its Probe field, leaving an empty nested dictionary.
8. Encoded JSON array [false,0,"",null] was shortened to three elements.

raw-null-baseline-native-observations.json binds those observations to exact original artifact hashes. The original test stream's P15_RAW_NULL_NATIVE_BASE64 records are the source of the stored native bytes.

## Bounded repair

The analyzer no longer exempts authored null merely because it is inside raw native data. It reports the existing E_NATIVE_JSON_NULL code and source span. Lowering separately rejects TokenNull while preserving the distinct TokenNone path.

Decoded raw native.plist data bypasses authored-literal analysis, so a narrow helper traverses the actual decoded maps and arrays before returning RawPlistValue. It reports a deterministic exact path, such as $["Nested"]["Probe"] or $["Rows"][3]. It does not rewrite data, manufacture plist null, or reinterpret JSON stored as Text. Existing XML/binary/date/data and native-reference regressions remain passing. No shared codec descriptor or wire encoding was changed.

The encoded-null regression was strengthened to require the exact rejected path in addition to the original error-code and zero-native-bytes assertions. No acceptance expectation was weakened.

## Clean repaired verification

At source 3abfeee3fc15b10624839e87d8188e7a1c3d2cc6, all selected 69 root tests executed: 413 named test/subtest passes, zero failed or skipped events, exit 0, 17.391s. The selection includes the new null boundaries, existing T05 presence and diagnostics, nested container guards, exact function/argument effects and snapshots, computed keys, original BRG09 and key roundtrips, shared codec matrix, raw result identities, and XML/binary/date/data passthrough.

Root raw SHA256: 51829eddfb33b8404d1acc836f0ae058e550ab50be1e48e6e75e20282a9acf15.

The complete internal/language/analysis and internal/language/lower packages both passed: 52 named passes, zero failures/skips, exit 0, 5.203s. Package raw SHA256: 180d1237f31bec6585d807e61983a1db32f437c2dc0809c822dd8d4eab859f06.

Both clean runs verified the same HEAD and a clean worktree before and after. Existing finite Go/outer budgets were retained. Exact commands, times, source state and hashes are in each original summary. The earlier successful 39-event precommit replay is retained separately and explicitly records its dirty source; it is not represented as a clean-source run.

## Evidence integrity and remaining work

manifest.json lists the exact size and SHA256 of every copied original artifact. Git evidence attributes preserve the original bytes. The source contracts, runtime fixtures and goldens were not edited. No private inputs were involved.

Structured function optional transport, complete container scenarios and any runtime limitations remain separate work. Neither these local native structures nor their roundtrips establish actual execution by Apple Shortcuts.
