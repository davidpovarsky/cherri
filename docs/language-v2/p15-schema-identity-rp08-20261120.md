# P10R15 RP08: schema default presence and semantic identity

## Verified outcome

The clean implementation 35373c2d25fd171c5370521b7e7e1802273a59f6 passed the actual RP08 production pipeline and focused consumers on Windows/amd64, Go 1.25.6, with GOMAXPROCS=2 and GOFLAGS=-p=2. The worktree was clean before and after all three commands. Every retained command exited zero, with no skipped or failed named test, timeout, or malformed Go JSON event.

| Actual command group | Named passes including parents | Exit | Wall seconds | Raw Go JSON SHA-256 |
| --- | ---: | ---: | ---: | --- |
| production | 9 | 0 | 159.812 | b8f79b70e188841533d344cdab2040cad55f66d591c450e99950a4e91df8002c |
| focused-packages | 55 | 0 | 13.250 | 5113b36e887c66d759d485a94aaae0a7ae0c08f11eb01574ffb19ffd2efb541e |
| catalog-consumers | 8 | 0 | 6.640 | d29485854fea45fc50dc726a05857be1ef9de7e8ca63c5d85bd7f5a26d20d61f |

The production test has eight scenario children plus its parent. Counts include parents; they are not independent requirement counts. Complete commands and named outcomes are in [clean-process.json](evidence/p15-schema-identity-rp08-20261120/clean-process.json). The [immutable source inventory](evidence/p15-schema-identity-rp08-20261120/immutable-source.json) hashes exact Git blobs. Worktree-file hashes in the original process receipt remain unchanged and are not relabeled as Git blob hashes.

## Defects reproduced before the repair

The tests-only production baseline at 89d598ff0c783e79f9f2c720778c6c8634e20400 actually ran the live exporter, generator, compiled registry and consumers. It failed three scenarios: explicit empty Text defaults were omitted in transport, absent and explicit-empty defaults shared identity, and a native Bool false constant shared identity with Text "false". Four other scenario children passed. The command exited 1 after 152.063 seconds, with clean source before and after. Its raw Go JSON SHA-256 is 1775c4d61cbcd62c787037506dae1eb42165905334067ae6120dac2b67fdff60.

Additional clean tests-only baselines 7b3d0a4e05abbc728a8e8d7874dc54e1cb668adb and 6e6441ce274e88aa3b0859f587d2d3f768d650ff demonstrated typed constant, container and delimiter collisions and an invalid custom-action registration that could publish metadata before fingerprinting failed. Deterministic map ordering and documentation-only changes were positive controls.

Dirty implementation drafts are retained separately, without clean-source credit. Causal checks rejected a plain-JSON identity design that collapsed native Data into base64 Text and real numbers into integers; they also detected shared default pointers in registry clones and macro descriptors. Final review reproduced additional collisions in enum records, parameter/action field boundaries, accepted-form sequences and two distinct finite versions rounded to two decimal places. Before/after logs are preserved. An initial mechanical test build failure after changing the default field type remains a build failure, not a semantic test result.

## Production changes

catalogParameter.Default, generated input LegacyParameter.Default, and schema.ParameterSchema.DefaultValue now represent presence with a string pointer. JSON still transports present defaults as strings. A nil pointer means absent; a nonnil pointer to the empty string means an explicitly declared empty Text default. schema.StringDefault supplies present values. The live exporter, generator, generated registry, handwritten macro metadata, documentation renderer and existing consumers preserve that distinction. Registry and compiler-macro descriptor clones copy pointed-to values instead of sharing mutable metadata.

The static-parameter identity encoder uses an explicit recursive type-and-value representation, deterministic map ordering, and unambiguous field encoding. It retains native Data, integer/real, Bool/Text, nil, container and nested type distinctions. Existing hashed string fields and enum/form elements are quoted, and finite native version precision is retained. Unsupported semantic values fail before custom metadata is published, so recovering from a rejected registration cannot leave an invalid action behind with a superficially valid old identity. This is schema metadata identity; runtime native encoding and accepted requirement oracles were not changed.

A temporary generated-source type bootstrap was needed to build the changed exporter. The actual live generator then overwrote that bootstrap from corrected canonical action source. The rebuilt live exporter and generator reproduced identical output twice, and --verify-generated passed. The final generated registry comes from that actual pipeline; it was not left as a handwritten bootstrap. Canonical label and generation-guard fixes from the integration branch were included before regeneration.

## Actual generation and consumer scenarios

TestP15RP08SchemaIdentityProduction in p15_schema_identity_recovery_test.go uses p15SchemaReplica and p15SchemaRun. It changes only a disposable replica of the public action declaration, builds the actual CLI and generator, exports --actions-json, generates and compiles the registry, and runs the real analyzer and capability builder.

The unchanged scenario declares Bool false, Number zero, empty Text and absent Text defaults. It checks presence and type through the exporter, generated source, compiled consumer, cloned registry and capability descriptors. Correct typed arguments are accepted and a wrong typed argument is rejected. Separate actual generations change a default, absent-to-empty presence, constant value, constant type and codec version. The codec mutation must change the declared schema/codec identity tuple; it does not incorrectly demand that codec-only changes alter the schema component. Restoring source restores the original exact catalog/generated bytes and consumer identity.

The public CLI is rebuilt from the generated result. Two independent --capabilities-json executions return identical bytes and agree with the independently compiled consumer on schema, codec and macro identity and all 15 macro descriptors. The probe preserves the declared empty debugTraceEvent.label default and false debugLog.withDates default. Full derived observations remain in the original archive; [a compact identity summary](evidence/p15-schema-identity-rp08-20261120/identity-observations-summary.json) is also provided. These identities belong to the disposable RP08 fixture, not the unmodified canonical application.

Unit controls exercise failed publication, pointer detachment, map-order determinism, documentation exclusion and record delimiters. Direct-schema field boundary controls do not imply that punctuation-containing names are accepted Cherri source.

## Evidence retention and limits

[original-receipts.zip](evidence/p15-schema-identity-rp08-20261120/original-receipts.zip) retains exact original public test logs, process receipts, generated fixture outputs and observations. [manifest.json](evidence/p15-schema-identity-rp08-20261120/manifest.json) records every original member byte length and SHA-256 and labels clean and dirty phases. Reproducible executables are deliberately excluded; no private corpus was copied.

This is real generation and unit/consumer proof for RP08. It is not an Apple runtime result, a complete T01 result, an integrated-source replay, or completion of the 179-row acceptance suite. Formal RP08 registration and a combined-source run are separate integration work. Historical failed and dirty runs retain original source identities and statuses; this report does not transfer results to a later source.

The implementation commit is separate from this evidence/provenance follow-up, keeping firstForkCommit noncircular.
