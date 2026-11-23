# K10 iOS 27 Runtime PoC — original success intake

## Actual result and identity

The original workflow succeeded. This packet independently reconciles its
published artifacts, native observations and six formal runtime facets at K10.

| Identity | Original value |
| --- | --- |
| Source | `4feadab9db4d351628f78593304373631693869a` |
| Branch | `agent/cherri-p10r15-k10-verification-20261123` |
| Run / attempt | [45101192445](https://github.com/davidpovarsky/cherri/actions/runs/45101192445) / 1 |
| Workflow / job | `iOS 27 Shortcuts Runtime PoC`, ID `375700807` / job `137452178932` |
| Job interval | 2026-11-23 04:54:30–05:14:59 UTC; 20m29s |
| Runner / host | `xcode-27`; actual macOS 27.0.1 / 26A434 |
| Toolchain | Actual Xcode 27.1 / 27A9275 |
| Simulator runtime | Actual `com.apple.CoreSimulator.SimRuntime.iOS-27-2` |
| Original input | `allow_public_fixture_hubsign=true` |
| Artifact | `14791328268`; 136,823 bytes; 32 ZIP members |

Original artifact SHA-256:
`b2d6218cbe87f343365de51782c2d7e55739eed5064127e1151a208a891d57d2`.
The actual producer emitted `IOS27_SHORTCUTS_RUNTIME_POC=PASS` at
05:14:35.656173 UTC, after the formal exporter and original Go evidence verifier.

## Independent artifact reconciliation

The unchanged publisher, formal exporter and coverage observer were read from a
clean exact-K10 checkout. All 17 local Python helper/dependency files were
compared with immutable Git objects before import. The pure artifact replay
did not execute Shortcuts, compile, run Go tests, dispatch or rerun XCTest.

- All 31 published payload paths, sizes and SHA-256 values match the original
  manifest; the remaining ZIP member is that publication manifest.
- All five reviewed synthetic fixtures match both the source receipt and exact
  committed bytes. All five signed AEA1 artifacts match the original source,
  signed-byte and exact-file signing receipt hashes. The original authorized
  signing method was `hubsign` for all five.
- The exact source/run/attempt-owned Simulator was created and later deleted.
- All 17 historical diagnostic rows report actual pass. Their older requirement
  labels remain diagnostic; they were not promoted into extra formal credit.
- The original six formal records equal the unchanged exporter's output. Every
  referenced artifact and diagnostic/source receipt digest agrees.
- S01/F01 coverage reproduces the original fresh-reset proof and exact 313-byte
  clipboard. Expected and observed SHA-256 are
  `f2e79540f2c95547ac9c83d59f25b63ce4167153d81731a5e00b746f041037f9`.

Intake notes preserve two caller-only filename preflights and their corrections
from actual source inventories. Neither altered source guards, fixtures,
native outcomes or original artifacts.

## Exact formal-runtime scope

| Exported facet | Requirement IDs | Observed bounded contract |
| --- | --- | --- |
| Variables | V01, V03 | Mutable Text/Number capture and clipboard capture before later writes |
| Indexing | L02, L04 | First/last, empty, negative, out-of-range and guarded dynamic List indexing |
| Numbers | N01 | Exact numeric trace, including 5/2 = 2.5 |
| Functions | FN01, FN02, FN03 | Return/continuation, explicit false/zero/empty Text versus omitted defaults, terminating factorial recursion |
| S01 coverage | S01 | Exact Unicode interpolation with Hebrew, astral/ZWJ, combining, adjacent/repeated references and literal U+FFFC |
| F01 coverage | F01 | Exact effect trace and Boolean results for nine nested/mixed AND/OR cases |

The original producer also validated five fixtures: primary runtime, dictionary
approaches, actual Alpha/Beta menu selection, Legacy If and S01/F01 coverage.
Its original Go verifier reports both menu labels actually tapped and both
expected clipboards.

These are six runtime facets for ten IDs, not complete acceptance of those
requirements. FN03's other clauses, N03/nested transport, combined-scenario
producer gaps and every other immutable tier remain separate.
This workflow does not execute the full original `P15_F01_OK` fixture; direct
F01 run `45101081393` is a separate original run, still active at the recorded
05:22:35 UTC local observation. No K10 evidence is relabeled to K11 or K12.

## R22 relationship and platform boundary

`scripts/ios27_runtime_poc.sh` is the same producer invoked by R22. The exact
workflow, producer and release evaluator bytes match retained Linux 56-gate
source `1c07e3fee1368d7379616143168c072c79ffe8ad`. This is a separate genuine
native component at K10, including the original pass marker.

The retained Linux R22 row remains `BLOCKED_PLATFORM`. No aggregate was rerun,
no original row/output hash was rewritten, and no full four-CI or formal
acceptance is claimed. A Simulator supplies no physical-device credit;
that boundary remains `BLOCKED_DEVICE`.

## Observation history and originals

The first 5,100-second observer expired while waiting. The second ended with
HTTP 403 API-rate-limit evidence. Neither event was a workflow failure.
After the reset, the approved same-run observer continued at 60-second cadence
and returned zero on the actual success. Original outcomes, 403 stderr and
continuation receipts are retained. The workflow's 75-minute deadline stayed
unchanged.

`originals.zip` holds the whole original GitHub ZIP and selected original
run/job/artifact, dispatch/binding and observer receipts. Its manifest lists
every member's bytes and digest. The complete 1,577,379-byte workflow log stays
on Oracle, SHA-256
`84f8498ac550c0921311ab8e6af3e471a620372ea9f1f0072f02cf0f5452bbdc`.
`native-original-log-proof.json` contains only selected public producer-result
lines, tied to original line numbers and digests.

This documentation-only intake leaves native results and protected Swift tests
unchanged. `formalAcceptanceGranted` remains false.
