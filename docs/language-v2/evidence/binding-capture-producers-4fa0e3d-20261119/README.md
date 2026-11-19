# Independent V01/V03 local capture evidence

Exact clean source: 4fa0e3d07fbe18a1b910979b390765f840f6d152.

This change strengthens the existing binding fixture oracle and executes independent local producers for V01 and V03. It changes no compiler production code, runtime fixture, immutable requirement, tier, threshold or expected iOS value.

## Actual causal baseline and fixed result

The new controls ran against the original capture oracle at parent 33f0699c2f7ec58b96a3da75ecd3d2c05027e0f2. Both unchanged controls passed, but all twelve changed native graphs escaped the original oracle. The baseline has 17 named events: 2 passed and 15 failed, including the two kind parents and overall parent. Its raw 19,772-byte log has SHA256 6d7901551a774835fe344b46dfae92d4fbfbba1e5380e0dc00e3ffe9a6e6ef15. The exact test bytes are preserved as .go.txt and match the baseline driver's recorded hash.

The fixed oracle requires the exact four-field result template, prior uniquely identified capture producers in the correct slots, no value-changing transformation, a straight-line path and the final clipboard write consuming that exact result. It rejects missing/wrong terminal consumers, changed labels, swapped observations, transformations and extra attachments. The actual fixed root gate passed 52 named tests with zero failures/skips in 27.203 seconds.

## Exact clean producer replay

The clean 4fa0e3d source ran the two actual independent production selections and their manifest fault controls in 16.422 seconds. All 16 named outer/mutation tests passed; no required test was missing or skipped. The 24,538-byte raw log has SHA256 9fdff5897309812dbfd94d1d6d68d9fcb49a460f7ba381b389041830d623b885. The receipt was reconstructed by concatenating complete go test JSON Output fragments per test and records clean source plus four local tier records.

Each requirement executes its own unit, serialized native graph and CLI/mobile compilation controls. Each still reports PENDING_EXTERNAL for ios-runtime. Local validation reports zero completed requirements, zero failures and two pending cases; final validation rejects the missing iOS tier. Forged native/iOS producer attribution, stale source, truncated commands, missing local tiers and contradictory exit status are rejected.

The existing native callback failures from S3 are not superseded by these local tests. The clipboard callback route and export content handling remain separate native investigations. No iOS values, default transport semantics, native Unicode export or overall release completion are claimed here.

## Integrity

The three immutable contracts and unchanged original binding fixture were hashed before and after the clean replay. The canonical evidence map still has 179 mappings. All raw logs, summaries and drivers retain their exact bytes; local .gitattributes preserves native line endings. Manifest members are verified from committed Git blobs, not only from working-tree files. No private corpus inputs, identities, binaries or large workflow archives are included.
