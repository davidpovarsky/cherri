# K10 typed-defaults Simulator result — original-source intake

## Result and immutable identity

Run [45101024573](https://github.com/davidpovarsky/cherri/actions/runs/45101024573), attempt 1, completed **SUCCESS** on implementation `4feadab9db4d351628f78593304373631693869a`, branch `agent/cherri-p10r15-k10-verification-20261123`. The original `xcode-27` job `137451663249` ran from **2026-11-23 03:56:45 UTC to 04:34:23 UTC** (37 minutes 38 seconds).

The original observation bound expired while this same workflow remained active. Its receipt is retained, together with the separately authorized sequential observer continuation. The second observer exited 0 at 04:34:39 UTC without timing out. The run ID and attempt remained unchanged; no workflow or test deadline was extended and no native rerun was performed.

## Exact observed defaults contract

The existing QA adapter is `tools/shortcuts-simulator-qa/fixtures/p15-binding-defaults/CherriTypedDefaultsPOC.cherri`, SHA-256 `6b4f62c80d2e63ca53ee68c769aae13dc84b3eca766046481d6cd3b607b9efbe`. Its marked original body is **byte-identical** to `tests/runtime_poc/CherriTypedDefaultsPOC.cherri`, SHA-256 `94165361845193c63fda7740a6cac826b68dea2d829b870c7e87f2f37f8f8063`.

The adapter observes the original body's clipboard result and returns it through its existing bounded Base64 callback plus NDJSON trace. The fresh correlated callback matched the exact immutable source expectation. Its decoded result is:

```text
CHERRI_TYPED_DEFAULTS_OK
DEFAULTS_EXPLICIT=0:0::0:0
DEFAULTS_OMITTED=1:9:def:1:1
DEFAULTS_MIXED=0:9:def:0:1
```

| Original call case | Bool representation | Number | Text | List count | Map key count |
| --- | ---: | ---: | --- | ---: | ---: |
| Explicit false, zero, empty Text, empty List and empty Map | 0 | 0 | empty | 0 | 0 |
| All arguments omitted | 1 | 9 | `def` | 1 | 1 |
| Explicit false and empty List; remaining arguments omitted | 0 | 9 | `def` | 0 | 1 |

This is the original three-case behavioral matrix. It distinguishes explicit empty values from omission in those calls; it does not establish every possible default combination or arbitrary nested transport semantics.

## Original observations and independent checks

| Layer | Observed result |
| --- | --- |
| Original XCTest aggregate | 4 passed of 4; 0 failed, skipped or expected failures |
| Existing exact-source callback/trace oracle | runGate PASS and traceGate PASS |
| Fresh correlated non-self callback | Exactly 1, status SUCCESS |
| Original NDJSON | 4 records; byte-equivalent to canonical replay after CRLF normalization |
| Compiler/signing provenance | Source/unsigned plist/target name/signed digest and size join; 92-action inventory agrees |
| Published native payload | All 25 payload hashes and all 26 ZIP members verified |
| Public prerequisite layer | Existing exact-source validator PASS, bound to the native compiler context |
| Private prerequisite originals | 23 complete members, 3 source commands and 21 exact Git-bound files verified in place |

The signed file's `AEA1` header is a format/provenance observation, not an independent cryptographic signature verification.

The public assertions still report `typedValueCoverage=UNKNOWN_AT_TRACE_EVENT` and `perActionCoverageVerified=false`. The concrete output matrix above is available even though generic strict typed-value trace coverage is not established. Editor inspection remains `NEEDS_REVIEW` / `PARTIAL_EDITOR_CAPTURE`, and published readiness remains `PARTIAL_READY`. Physical-device status remains `BLOCKED_DEVICE`.

This proof is exact **K10** evidence. It does not restamp the result as K11, K12, physical-device or whole-release acceptance; `formalAcceptanceGranted` remains false.

## Selected original packet

`originals.zip` contains both original GitHub artifact ZIPs and selected original run/jobs/artifact metadata, dispatch request/result, immutable run binding, acquisition receipt, original observation timeout and later successful observer receipts. `originals-manifest.json` records the exact byte count and SHA-256 for every member.

- Native artifact `14789549696`: **73,408 bytes**, SHA-256 `5e20cd76711b39933d95ae688bd850ed5837e49f856372bce8fbdc1999933075`.
- Sealed prerequisite artifact `14790426753`: **586,000 bytes**, SHA-256 `ed7ea560a189de6cc5007f257030069996c747ab6c4b6c7b1bbcd0473bd41e00`. Its only members are the original `project-prerequisite.zip.p7m` and `summary.json`.

The independent callback replay reads retained original observations and the unchanged oracle extracted from the frozen K10 script. It does not execute a Shortcut or rerun XCTest. The public prerequisite validator, synthetic source check, publication inventory and privacy checks are also the existing exact-source implementations.

## Private prerequisite boundary

After the mandatory public validator passed, only the original ciphertext and public summary/protocol/context/validator receipt were transferred into the existing private Windows workspace. The unchanged, previously calibrated `c74295ff…` verifier and existing authorized recipient/key were used there.

The decrypted archive and private details stayed in that workspace. Only the fixed booleans/counts/public-digests object in `private-verification.fixed.json` was returned; no plaintext archive, private path, nonce, key or generic private log was transferred. The included independent static review is a verifier review, not a separate native run.

Verifier SHA-256: `c74295fffc41212ec85b2c2b2b9485094bd0234e85e0396e36e61bf81fea3401`. Expected protocol SHA-256: `72c29ee11d588b7c9f9a2c9ca50a09afaa4e21ecfc35d6b4c2b7562081a9ae82`. Native context SHA-256: `417d0351b5ebf29f49d25bceaa3216344a48f699a87fa3a45e809f2e7a161ed9`.

The private verifier's scope is prerequisite-only, and it explicitly claims no native acceptance. The separate original XCTest/callback/trace evidence supplies the bounded runtime result.

## Change scope

This commit adds documentation only. Source, fixture bytes, tests, assertions, deadlines, workflows, protected UI and release policy remain unchanged.
