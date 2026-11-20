# RP11 per-parameter wire acceptance — 2026-11-20

## Verified result and boundary

The complete immutable RP11 scenario passes local unit and native-differential acceptance at `8c1305b3b9edb2c3329aef84eb4fce9e253312b1`. A real clean-source subprocess executed all83 required named compiler events. The outer producer and evidence-intake suite executed248 named tests, with zero failures or skips, in36.093seconds. Exactly two records were emitted, both for RP11; local validation accepts one requirement and final release validation still rejects missing global CI proof.

No production parameter encoding change was needed for RP11. The exact source binds the actual canonical compiler, the independent captured Apple parameter-state data, the production test helpers and the strict acceptance machinery. Its45 declared source files were independently compared byte-for-byte with their immutable Git blobs.

This does not award linked T03, T09 or R03 requirements, Apple Shortcut runtime, or full179 closure. The static native differential uses captured Apple state serialization; it does not claim a new Apple runtime execution. A combined-source replay remains required after integration.

## Scenario and independent expectations

Eleven cases cover a semantic numeric image width, a bound Number width, an exact decimal-looking Text literal beyond the binary64 exact-integer range, mutable numeric references, Number action outputs, scalar numeric stepper parameters, mutable Text, Text action outputs and contextual Unicode tokenized Text. A plain Text argument to the numeric stepper is rejected before output bytes.

The oracle reads the retained Apple `encoding-table.json` and `parameter-encodings.json` under `docs/apple-runtime/macos-27/`. It independently checks the native state bindings for image width, stepper delay and Text: `WFNumberStringSubstitutableState`, `WFNumberSubstitutableState` and `WFVariableStringParameterState`. It derives wrappers from those captures and substitutes only independently observed source producer identities. It does not ask the encoder under test to generate its expected wrapper.

The contextual case requires the exact surrounding `😀:` and `:é` text and the UTF-16 attachment range `{3, 1}`. Action-output cases require the exact original producer literal and once-only producer order. Mutable references must identify the independently located unique native variable store.

All58 causal mutations must be rejected: wrong action, omitted parameter, blanket float, blanket attachment, foreign reference, changed producer literal, UTF-8 byte indexing in place of UTF-16 indexing, and lost surrounding text.

## Actual process and evidence integrity

The RP11 runner executes only its three declared production test parents, requires all83 exact parent/child events, and binds each tier to its dedicated actual producer. Raw stdout, stderr and process receipt are retained with exact hashes. Validation requires clean unchanged source, bounded monotonic process timing, event times inside the actual interval, exact command, exact discovered event inventory and45 source hashes.

Controls mutate actual artifacts and recompute their superficial digests. Validation rejects altered source hashes, missing or forged tests, skipped or failed children, duplicate/orphan events, wrong package, ambiguous JSON, invalid UTF-8, wrong requirement/producer/tier, incomplete process receipts and misleading timing. Separate controls prove T03/T09/R03 cannot inherit RP11 evidence. The complete179-row audit remains179 rows and awards no unexecuted row.

## Preserved attempts

| Exact source | Observed result | Meaning |
| --- | --- | --- |
| `f8f91e11f8fd50cc8d66d92ecef91b5530c0e681` | Failed initial test-only attempt | The test expected the wrong argument diagnostic identifier and incorrectly assumed a bound Number was emitted as a direct one-action literal. Both original errors are preserved. |
| `b414a347f9c96d8061f5a97a904e3c6881fc116a` |83 named PASS, no failure/skip | The corrected oracle distinguishes a direct literal from a bound Number producer and uses the real argument type diagnostic. |
| `8c1305b3b9edb2c3329aef84eb4fce9e253312b1` |248 outer and83 inner named PASS, no failure/skip | Actual source-bound RP11 producer and corrupted-artifact intake. |

Corrected compiler output SHA-256: `4b04c17592f68213dfd779dd81bb30e6c02b88a122d676c225a50a09f38a74c3`.

Actual outer output SHA-256: `e5b984a4e2cb3e2fab93bcb514d6ebcdcf3dc7bede29d8c2cbfac657b89c553d`.

Actual inner output SHA-256: `cfe2dbfe73f304c6438995c2854355cf8187bd5ebc2507d4164fad19602287d3`.

All original receipts, including failed attempts and unchanged original artifact paths, are retained in [the original-receipts archive](evidence/p15-rp11-parameter-wire-20261120/rp11-original-receipts.zip). [The receipt index](evidence/p15-rp11-parameter-wire-20261120/receipt-index.json) distinguishes derived audit files and records every member digest. Original bytes were checked after archive extraction; no failed receipt was edited or overwritten.
