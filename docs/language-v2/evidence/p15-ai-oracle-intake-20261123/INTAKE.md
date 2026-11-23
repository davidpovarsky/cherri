# Oracle intake of retained AI tooling evidence

## Result

The AI implementation is already integrated in R7 `ca316a3dee252456c4bec751594cf249c709b331` through `824333e8`. No implementation patch or Python test rerun was needed. This commit restores only the three missing original files from `docs/language-v2/evidence/p15-ai-sealed-intake-20261121/` and adds this receipt.

The incoming branch tips `04be8b60953521dce72597754e41a270d604a968` and `eaa819cd07bceaf3726f69ca44a724979cfd5f84` are not ancestors of R7. Their non-ancestry does not establish missing content: all fifteen tracked Python/data/guide files in the final package are byte-identical at the tested source, incoming proof tip and R7. R7 also includes both Go helper source witnesses from eaa819cd and expands its witness set from 38 to 46. Its later requirement mappings, installation inventory and external control receipt support remain unchanged.

## Independent mechanical verification

Read-only Oracle reconciliation verified the final packet's 347 originals, its nested 203-member local packet and 46-member causal packet against their exact manifests. All fifteen source bindings match actual Git blobs. Six AST test modules, the frozen original Windows inventory and original sequential events agree on 132 parents and 78 passing subcases. Eight historical stage counts also agree with their originals.

The reused execution is the original clean Windows run at `29f35c71d4932ac2ed135f977ac99ec43cddfe3d`: 132 PASS, zero failures/errors/skips, 140.953 seconds within the original 150-second outer budget and unchanged 15-second fake CLI budget. Its interval remains 2026-11-22T01:39:27.778727+00:00 through 2026-11-22T01:41:49.425400+00:00. This is package verification reused by exact source equivalence; it is not a new Oracle run or a joined Go execution.

The restored earlier packet has 89 exact original members and fifteen verified source bindings at `8c85dbdb7c3e37ebe50c3404468bdfe25205d1c2`. Its separate original 121-parent PASS took 35.312 seconds. Original earlier failures and all authored S3 calibration outputs remain intact, including the two malformed Legacy exponent operands; no historical result was relabeled as the later 132-parent run.

## Original file identities

All three files below are copied byte-for-byte from `04be8b60953521dce72597754e41a270d604a968`.

| File under p15-ai-sealed-intake-20261121 | Bytes | SHA-256 |
| --- | ---: | --- |
| README.md | 5,116 | `0326a85b0191ea33a19e80b45ed45f463f786b8ed2cbffe5835dcec74b2eb775` |
| manifest.json | 22,048 | `79dec12bae529c89b98c8783eaa7113850b3297d3b342a466756229524414510` |
| originals.zip | 133,217 | `5b1775e0bd95ba9b5e3db7f6396516cbbb5fff35bb2b1e7a0d9711f9b64d3883` |

The original README and manifest retain their 29 and 580 CRLF pairs. Their line endings are evidence bytes and were not normalized. Default Git whitespace inspection reports these 609 historical CRLF line endings as trailing whitespace; the original diagnostic is retained. The new intake receipt passes whitespace inspection.

The already-integrated final archive remains `bd94fea4a0bb68fa4aae50b9a8019808238a194c9e734064cb2c41985af7bfde`, 871,040 bytes. Its unchanged manifest is `7472ca03f3f92cf6a93b6904b5661000cc2b3868db0d9f2e5f60df7ad75b2b40`. The existing root-review packet also preserves its earlier CRLF/LF wrapper reconciliation.

## Publication and scope

The restored archive contains only the declared earlier control runs, source copies and authored public calibration groups. Inspection of the README, manifest and all 89 members found no live credential, private PEM key, provider token or private evaluation anchor. Original local machine paths and historical process metadata remain part of the retained public evidence.

No live provider request, token-count request, quota change, credential access, new evaluation seed, authenticated anchor, compiler rebuild, test execution or Apple execution occurred in this intake. No model quality or formal acceptance is claimed. A final joined source freeze, exact calibrated compiler, fresh signed instances, independent authenticated Asana observations, authorized measurement and canonical Go intake remain separate requirements.

Full read-only reconciliation is retained on Oracle at `/mnt/development/projects/evidence/cherri-p10r15-ai-integration-20261123/`: `mechanical-intake.json` SHA-256 `29748e3a9218b86614a19bc5b03bd563217fe49755bad70a377d978b4718e16d`; verifier SHA-256 `7d0c92bbeeb6cf462e612b9aff9bd723d07de55d566db612b91960a3c2b45dcf`. Original verification stdout/stderr are retained beside them.
