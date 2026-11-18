# Canonical filter transport - clean production receipt

## Outcome

Source commit `09ca92e144158c147a799c1bc8517f78c5f7fd61` repairs the three reproduced Linux filter regressions through the existing canonical argument adapter. The three original test files are byte-for-byte unchanged. The exact clean compiler embeds that commit and `vcs.modified=false`.

The production change is a 19-line parameter-aware adapter and two existing conversion call sites. Only an explicit Dictionary/List supplied to the canonical `WFContentItemFilter` parameter uses the established semantic-to-native parameter converter. Runtime Dictionary encoding, Legacy parsing, raw native emission, the predicate validator and action registry retain their existing paths.

| Executed check | Observed result |
|---|---|
| Original Linux regressions on source `57a72ed` | All three reproduce the typed-Dictionary panic |
| Clean final source, 16 required top-level tests | PASS; 121 named test PASS events, zero failures |
| Original Linux tests on clean final source | All three PASS with unchanged test hashes |
| Malformed filter specifications | All eight reject through the existing specific validators |
| Independent native table oracle | All eight deliberate corruptions reject |
| Native import round trips | All 16 exact trees pass: XML/binary x hybrid/lossless x Legacy/V2 x two graphs |
| Old-adapter overlay with final causal test | Expected failure: compound/shorthand/empty maps fail; predicate/empty arrays pass |
| Actual compiled CLI | Three positive specimens pass; one malformed filter rejects and creates no output file |

The observed zero/false values, literal braces/backslashes, predicate order, compound mode, source UUID reference, unknown serialized metadata, and absence of extra runtime collection actions remain explicit assertions.

## Identity and retained evidence

- Compiler SHA-256: `6330e647fd4bec4827b5b048645bbb5e6186ef91049cadd266b3ca26f3fb9609`.
- Published receipt SHA-256: `47e08a50a91f77b1509895abe54f6964109a0bbffc47234f648819f8953a74f7`.
- Clean proof job: `job_ecbbb54a90fc`; completed successfully in 134.96 seconds.
- Public CI failure: run `44402799497`, job `135150026633`; the independently checked raw log SHA-256 is `fa551dacdb36fbbd7a266d77b2b2fa9ecae05cf71e7895e48048ecab9145cb86`.
- [receipt.json](receipt.json) records actual commands, outcomes, compiler identity and unchanged test hashes.
- [manifest.json](manifest.json) binds the published files and preserves the original capture hashes where path labels were sanitized.
- [native-roundtrip-specimens.json](native-roundtrip-specimens.json) contains the 16 emitted production specimen receipts.
- [test-fixture-calibration.json](test-fixture-calibration.json) records two failures in the new compatibility fixture before source freeze. The Legacy fixture needed its established assignment spacing and exact real-valued JSON number expectations. The retained pre-fix compiler independently confirms that representation; numeric equality was not relaxed.

The old-adapter check uses Go's overlay mechanism. Neither clean source checkout was modified for that negative control. Public synthetic source files and action graphs are included; binaries and raw local captures remain in the owned evidence directory.

## Limits

No Apple runtime filter query, user-data access, signing or native dispatch was performed by this stage. This receipt proves compilation, exact serialized native structure, existing import round trips, and the reproduced regression repair. It does not promote any formal acceptance requirement or claim Files, Reminders or Calendar runtime results.

Re-run the named tests from the source commit using the commands in the receipt. The final combined Linux/macOS/Nix and private-corpus boundary remains owned by the integration stage.
