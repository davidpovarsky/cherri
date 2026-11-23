# Actual 505 catalog producer refresh on Oracle

This packet records one actual execution of the two existing catalog evidence
producers at clean source **9ce44842761ec4bb639e1f2c80c08d86c022b778**, tree
0b40109c934f743414a50592f114ce8b91fd6c4a. The source checkout and all recorded
generator dependency bytes were unchanged across execution.

## Execution and independent intake

- Platform: Oracle Linux arm64, selected Go 1.24.13; development-volume module,
  build and temporary caches.
- Started: 2026-11-23T00:39:47.477500+00:00.
- Finished: 2026-11-23T00:40:50.907082+00:00.
- Measured producer process time: 63.42959549091756 seconds.
- Process exit: 0; timeout: false.
- Exactly the following two top-level tests ran once and passed. There were no
  failing or skipped events and exactly one package PASS.

    go test -count=1 -json -timeout=15m -parallel=2 \
      -run '^(TestP10_VerifyAndRecord505CatalogExecution|TestP10_ParameterContractMatrix_AllCatalogActions)$' .

The test timeout remained 15 minutes. The external driver had a finite
20-minute process ceiling to include toolchain/compilation time. No deadline,
test identity, assertion, fixed acceptance inventory or compiler source was
changed. No full Go suite, derived-ledger generator or release-gate execution
was included in this packet.

Independent compiler-free intake verified 68 conditions, including every
original file's byte count and SHA-256, the exact source and driver bindings,
the selected toolchain, actual JSON test identities/outcomes, both artifact
schemas and record counts, cross-artifact identities, all six dimension
summaries, and unchanged comparison exclusions. The intake passed. It did
not repeat the tests.

## Measured artifacts

| Artifact | Bytes | SHA-256 |
| --- | ---: | --- |
| p10-505-execution-evidence.json | 717828 | d72e9d5f4a733caa7fa42b29f34f5e3434ed85f6033f987fb676fff5742eadee |
| p10-505-parameter-contract-evidence.json | 1814832 | 74c5ed6ffb65594937aefcf248cef7f1d5ed47388fc57352c148f478e14f8eb5 |

Both files were copied byte-for-byte from the external evidence output into
docs/apple-runtime/p10/. Both contain 505 entries: 497 callable actions and
8 compiler constructs. The existing producer reports 505 passes for Legacy,
V2, identifier, parameter contract, decompile, round-trip and structural
round-trip counters. All 505 recorded structural comparisons use
FULL_WORKFLOW_SEMANTIC_COMPARISON.

| Parameter dimension | PASS / eligible | Not applicable | Failure / not tested / blocked |
| --- | ---: | ---: | --- |
| Required parameters | 347 | 158 | 0 / 0 / 0 |
| Missing required rejected | 347 | 158 | 0 / 0 / 0 |
| Optional parameters | 202 | 303 | 0 / 0 / 0 |
| Enum validation | 98 | 407 | 0 / 0 / 0 |
| Dynamic reference | 397 | 108 | 0 / 0 / 0 |
| Platform gate | 65 | 440 | 0 / 0 / 0 |

Native specimen parity remains 0/505, with all rows NEEDS_EVIDENCE.
Apple runtime execution remains 0/505, with all rows BLOCKED_DEVICE.
This local producer result grants no native or device acceptance.

## Exact differences and observer limitation

There are no changes to top-level totals or dimension summaries. The execution
artifact has one changed record: rawAction.v2EmittedIdent changes from
is.workflow.actions.custom.p10test to is.workflow.actions.nothing.
The original measured output is preserved without correction.

The existing helper inspectEmittedPlistForIdentifier in
p10_catalog_execution_test.go falls back to the last emitted action for
rawAction. Consequently, the unchanged producer's PASS and that field do
not independently establish the custom V2 action's identifier when a trailing
output-normalization action is present. This is a limitation of the existing
observer, not evidence of a compiler regression or proof that the custom
action was absent. No additional compilation was performed to resolve that
narrow observation in this refresh lane.

The parameter artifact has three changed records:

- getType: current missing-required diagnostic now describes Get File of Type
  and its variable return.
- rawAction: current missing-required diagnostic includes the additional
  variadic placeholder.
- streetAddress: dynamic-reference expected-behavior label now names
  addressLine1 instead of addressLine2; its fixture and PASS stay unchanged.

The complete field-level differences are in the independent intake report.

## Originals, provenance and follow-up boundary

originals.zip preserves the original stdout JSONL, empty stderr, execution
intent/process receipts, source witnesses before and after, dependency
witnesses before and after, Go/toolchain observations, both measured artifacts,
the original execution manifest, the exact driver, the independent intake
program and its report. manifest.json binds every archive member and the
archive itself. All packaged bytes were checked against the originals.

Producer driver SHA-256:
ce809d9839a74657d6cbf6fb685a65aef0c86e743642e1eeed31acca03143bb4.

Independent intake program SHA-256:
847fe5b135ee5b03b9f9249771d1c99884f47b4d7297055bb9748ed968afc3f5.

Independent intake report SHA-256:
c02b51e6d8af566fb94ebc85925c7779ff09b4b941d50811f9a350c093451292.

The producer input fingerprint was
08d292f77f286997a19f615821bb76f269e224120ebcce3282556b05cbe268ca
before and after. It describes the actual C0 source, including the old tracked
raw evidence. Newly measured outputs were initially written outside the
checkout, then committed afterward. Future derived metadata must name a clean
ancestor that already contains these new bytes. This packet does not restamp
older raw proof, derived ledgers, release-gate outcomes or the protected-path
R23 failure.

The packet contains repository fixtures and a narrowly selected environment,
not live provider credentials, raw private Apple prerequisite journals or
device payloads. An additional credential-pattern scan found no private-key
header, provider-key, GitHub-token or bearer-token value in the 17 preserved
files. This scan complements the bounded artifact review; it is not a general
secret-detection guarantee.
