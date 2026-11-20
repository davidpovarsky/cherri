# R4 index, Optional demand and setter integration checkpoint

The independent index/Optional-presence component and setter semantic repair are now integrated. These are local, exact-source compiler checks. The frozen R4 formal179 and full CI failures remain preserved and have not been rewritten or scored from these local selections.

## Verified stages

| Executed scope | Exact integration source | Named passes | Seconds |
| --- | --- | ---: | ---: |
| optional-index-integration / root | `422327ae45c60add7257b591522c13e30759f0c9` | 934 | 57.266 |
| optional-index-integration / packages | `422327ae45c60add7257b591522c13e30759f0c9` | 121 | 7.25 |
| presence-demand-integration / root | `1a595e4fac9a2425c6801bcd8f348b0a196fee32` | 994 | 43.076 |
| presence-demand-integration / packages | `1a595e4fac9a2425c6801bcd8f348b0a196fee32` | 133 | 4.779 |
| setter-integration / root | `c45fb72d6b83762ecef31daf034ac59cef927d17` | 77 | 21.692 |
| setter-integration / migrate | `c45fb72d6b83762ecef31daf034ac59cef927d17` | 19 | 2.442 |
| setter-integration / schema | `c45fb72d6b83762ecef31daf034ac59cef927d17` | 0 | 5.012 |

All listed commands exited zero, with no failed/skipped named events. Schema verification is a live catalog comparison, not a named Go test. The scopes overlap; the counts are not a cumulative number of distinct requirements. Exact commands, full named events, bytes/hashes and clean-worktree receipts are retained in the sibling evidence ZIP.

Before integration, the index component's 23 proof blobs and six source blobs were independently checked, including exact raw 934/93 named event lists. The later presence-demand component's 27 proof blobs, 12 source blobs, five unchanged original fixtures/tests and exact 994/105 event lists were independently checked. R4 contains additional pre-existing package tests, so the actual integrated package counts are 121 and then 133.

## Resulting behavior

The checked in-range static List index can prove an existing definitely-present literal element without native truthiness. Existing sibling-write capture is used to preserve list evaluation order. Dynamic, missing or unproved values do not inherit that fact.

Optional presence metadata is omitted only for authored bindings proved to have no presence consumer. Authored values, explicit clears and ordinary assignments remain. The two original Nil-clear failures and the unchanged 14-action Legacy/V2 nil fixture pass. Demanded, unknown or cloned bindings keep conservative tracking.

The setter repair preserves three-position Legacy Remove mode through lexically shielded migration. A declared newValue containing Remove remains data when mode is omitted; an explicitly specified Remove value is retained in Remove mode. The original iPad fixture08 semantic round-trip and original setter regressions pass.

## Integration provenance

Index tests/source/proof were cherry-picked with the complete component map retained. Presence-demand tests/source/proof map to `3382fa4e4ef9669ad166e49b83df5ba27112311a`, `57df8c4bd33bafeff01ba19bbdaa985afe180894`, and `1a595e4fac9a2425c6801bcd8f348b0a196fee32`.

The setter component `fbbd7027d55425b0d26cb524a4fe5ee11d959401` was merged as `c45fb72d6b83762ecef31daf034ac59cef927d17`. This preserves the actual implementation `542e265468f8b70bd46da3af9912bf463379ec04` and separate provenance `1b92d596ec3539fd0caef2657903a5e6226c3738` as ancestors; their immutable IDs were not rewritten. The component's own clean77/19/3 plus schema proof, original failures and initial oracle correction are retained in [the setter report](p15-setter-mode-roundtrip-20261120.md).

The three immutable 94/52/33 contracts match their original hashes. The codec descriptor source is byte-identical to the preceding54f checkpoint. Original staging remains d7052f45; the previous remote R4 CI branch remains4e578c81. This audit checkpoint does not promote either ref.

## Remaining gates

The unchanged frozen4e report is92 PASS,10 FAIL,61 NOT_RUN,15 PENDING_EXTERNAL and1 AI_EVAL_NOT_RUN. Linux full CI failed; Nix passed; its macOS job remained queued at07:01. The report and failed originals remain in the preceding local179 checkpoint.

The separate require(Dictionary) lookup repair, S01 native/container boundary verification, real packaging/install producers, startup and Optional/Date Apple runs, final coherent compiler/docs/consumer tuple, full CI and final179 remain active work. This source has no new Apple or physical iPad execution credit. The user requested completion only with actual evidence; no task or requirement is marked complete by this checkpoint.

Archive SHA256: `dbe5e65dba6b902455dca8d25c4e96b28954ae9d67bde97a9e945099dcfa39bc`.
