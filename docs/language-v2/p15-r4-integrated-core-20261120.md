# P10R15 R4 integrated core: original replay evidence

## Candidate and scope

The actual composed compiler candidate is fd691b96fb661697a1cbaddab92476d457636ba9, published at audit/cherri-p10r15-r4-source-20261120 and confirmed by authenticated git ls-remote. This documentation-only seal follows the executed source. The original integration branch remains d7052f4568db26dba169bdefc72623e48c04968a.

This checkpoint integrates the observed Date parameter repair, typed Optional If/Menu joins and output/signing transaction repair. It retains the separate K05 fixture dependency correction and its still-failing real consumer results. It is not final acceptance.

## Actual root replays

| Source | Selection | PASS | FAIL | SKIP | Seconds |
| --- | --- | ---: | ---: | ---: | ---: |
| a50a090f | Date shared-core and provenance | 38 | 0 | 0 | 60.391 |
| a50a090f | Original K05 missing dependency | 0 | 2 | 0 | 26.078 |
| 72136c3b | K05 after copying its production JSON parser | 4 | 9 | 1 | 56.375 |
| 544bc5b9 | Combined Optional, Date, S01 and affected root paths | 2677 | 0 | 0 | 66.281 |
| 544bc5b9 | Six language/core packages | 184 | 0 | 0 | 6.375 |
| fd691b96 | Public output, signing failure and CLI compatibility | 127 | 0 | 0 | 37.437 |
| fd691b96 | RP43 strict intake and actual Windows host controls | 97 | 0 | 0 | 7.860 |

Counts include named parent and child outcomes. Every raw stream was independently parsed and checked against its retained receipt, source, stdout/stderr hashes, clean before/after assertion, and exactly one terminal event after each run event. These selections are not the complete Go suite. A separate full diagnostic executes in an isolated frozen checkout and is not claimed here.

The K05 candidate files retain the original green-all-k05 filename, but the actual attempt **FAILED**. Positive edit paths now reach the strict compatibility check and reject the stale consumer tuple. The skip is explicit opt-in live signing; no live signer request was executed by that attempt. No fingerprint comparison, manifest pin, source/output preservation assertion or copied-native policy was relaxed.

## Actual fd691b96 Legacy and capability results

A fresh clean Windows Go1.25.6 build ran the unchanged official runner in10.797s:47 fixtures (9 VERIFIED_PARITY and38 PROVEN_EQUIVALENT), zero baseline/migration/V2-compile/difference failures,17/17 action cases. The original globals.cherri, standard.cherri and menus.cherri fixtures are PROVEN_EQUIVALENT.

Report SHA256:6cc4ae29b54772b3e1d1fe4aeba602f705be77949fc3e416cd7cfd926cf25d31.
Binary SHA256:800974b3ffcc829759e249c0eee2cee4b2ddfe2345d7125a673dd88aca4435b8.
The binary has no embedded VCS stamp despite the recorded -buildvcs=true command. Its source is established by the actual clean build receipt. Canonical consumer binaries are separately built in an ordinary clone with real VCS metadata.

Actual --capabilities-json stdout SHA256:2da9dd298048944a7cb39b1c72e872541bbfea6bb578be1bb8b129a9b188d031.

| Capability | Observed value |
| --- | --- |
| Language/catalog/protocol | 2.0 / 3.0 / 1.0 |
| Actions | 505 |
| Schema | dc6548076b8320b15e062439f0d8e7b032e1fd62d032b19b06ab6c79a54890a5 |
| Codec | 71e2bc18eca1d32bdf2563d84f5c4bfce66086fd9bef7426481f8f5b8392d6de |
| Macro | acb4dd5035ee20786c68a01d323f565e880d2a0b8eedb151145181a4960d455c |

## Component integrity

Optional implementation11a4df6c8595ff7bc0df83db11ca6caa6bc10fd9 was sealed atb1fc485735e455c18d5b5a9cb45bcab504c7d423. Root independently compared all332 manifest-listed committed proof blobs,323 external original files,88 immutable source blobs and21 unchanged boundaries. Its component2599/184 outcomes remain attributed to11a4df6c. The root combined replay is separately attributed to544bc5b9.

Date implementatione259c829fa0345702e0989a25b5bbac1296d1fa6 has separate provenance9c7684b7a86458654a0e37de6bc9f5f8d5e530e1. Root preserved the full S01 descriptor and appended the observed Date-reference policy. The initial CRLF diff-check failure on two human-proof files is retained with their original bytes. Only their line endings were normalized during integration; no tests ran before that initial gate stopped.

RP43 implementationb351ba06c0728197376955629dfd4701c7a6a153 and strict producer468392aae763fe22ef7f6dcde4dcdc6e7f489e91 were integrated with exact history. Actual Linux [run44705394599](https://github.com/davidpovarsky/cherri/actions/runs/44705394599), attempt1, source53daeae947f729e434168a8a09661e735386c661, succeeded with253 outer and78 inner named outcomes. Root independently verified fresh run/job/artifact API identities, all29 sealed archive members, eight original artifact members,47 source blobs, exact78 required events and both RP43-only records. This Linux result is not reattributed tofd691b96. Windows host controls correctly produce no Linux integration/security acceptance records.

Initial RP43 receipt-filename and human-version invocation errors are preserved. They stopped before intended tests. The successful exact-source build was retained and the documented capabilities invocation then used.

## Original evidence

The companion manifest binds every archived original by byte count and SHA256, plus the separately retained generated executable. The ZIP was reopened and compared byte-for-byte against every original. Source bindings record actual immutable Git blob IDs and SHA256 values for composed changed source and selected runners/contracts. All94+52+33 contract hashes remain unchanged.

- [Original manifest](evidence/p15-r4-integrated-core-20261120/manifest.json)
- [Independent replay audit](evidence/p15-r4-integrated-core-20261120/independent-replay-audit.json)
- [Source bindings](evidence/p15-r4-integrated-core-20261120/source-bindings.json)
- [Original evidence archive](evidence/p15-r4-integrated-core-20261120/original-evidence.zip)

## Gates still open

The coherent compiler/docs/Skill tuple is being regenerated and must pass actual installed/extracted/offline/rollback consumers. Final integrated Linux/macOS/Nix CI, fresh179 acceptance, iOS app/UI and native semantic/runtime tests remain required. Physical-device, authorized AI evaluation and remote-consumer scopes require their own evidence. No pending tier or failed result is promoted by this seal.
