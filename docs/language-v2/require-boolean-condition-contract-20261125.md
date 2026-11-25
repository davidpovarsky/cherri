# Boolean `require` condition — 25 November 2026

This component implements the original condition form from Asana task
1220544599477512 in the shared macro builder. It does not close the separate
signed Apple runtime, physical-device, or final synchronized consumer obligations.

## Source contract

With no effective `type` or `nonEmpty` guard, `require` accepts a source-proved
Boolean. A false value is tested with the existing explicit numeric 0/1 Boolean
representation and emits Alert followed by Stop This Shortcut. Success returns
a Get Variable alias to the same original value. Release retains the complete
semantic guard.

The Legacy adapter obtains the Boolean witness from actual declarations,
bounded aliases and declared action output types. Transformed references,
unknown native content, cycles and repeat-item placeholders are not a Boolean
witness. V2 passes the actual analyzed argument type. The shared builder requires
an actual Boolean literal or an untransformed ActionOutput/Variable attachment
with a complete identity. Optional, AnyContent, Number, text and malformed
references cannot silently acquire Boolean truthiness.

The new condition proof is separate from the existing content-category hint.
Existing type and nonEmpty guard construction is unchanged. In particular,
`require(false, nonEmpty: true)` checks presence and preserves false;
`require(false)` is the false condition path.

## Original executed evidence

Production source: `f7b4f8a6e8723e48c7825b28c8d5164e55951704`.

Qualified source with exact Legacy wrapper oracle:
`84b668fa3e80fede3062c5cbc5fab4bd948978cb`.

The five production files are byte-identical between those commits. Their
difference is confined to two test files. The Legacy Set Variable oracle
explicitly requires its existing outer `WFSerializationType` field; it does
not remove or normalize unknown fields. Four causal faults test missing,
unexpected and incorrect wrapper fields in both dialects.

| Original process | Observed result | Receipt SHA-256 |
| --- | --- | --- |
| First condition RED, source 0882b5a0 | All 24 condition leaves rejected the unsupported API; eight newly authored presence-oracle leaves also failed because they wrongly required an output UUID on an unconsumed final action. Owner remains FAILED. | d6b3fb55d9625ab6090f5769a6a9d4df35fdea62c00caae046eb8a3b1824e6dc |
| Existing presence controls, source 1311705c | 9 named PASS: eight original false/zero leaves and their parent; eight wrong-consumer-edge faults rejected. | 8959d343ad8dd567f1d4766ea978c3bc88cc3f9458b1aaddeef762f38ce2f33f |
| First production qualification, source f7b4f8a6 | 157 named PASS and 10 FAIL. Six Legacy mutable graph leaves exposed an omitted existing wrapper field in the test oracle; their failed parents prevented 18 Legacy fault children. Nine unrelated RequiredVariadicPresence passes were selected by an overbroad prefix and are excluded from this component's planned count. Owner remains FAILED. | d96133a919e713a5260a24c4adcb2c93c9ad1cfb47ff8cf7253263f8eb3fff25 |
| Exact focused replay, source 84b668fa | 51/51 PASS; 0 failures, skips, timeout or capacity abort; 27.57 seconds. Covers the six failed source leaves, all original causal faults, and four new wrapper faults. | 85f02e32556482b1adea8097f60b8d602d2f2be52f7b6b7fc992442a4c8d4307 |
| Independent original intake | 180 distinct planned identities covered across the two production runs: 148 prior planned passes + 51 replay passes − 19 overlapping identities. This is not one GREEN180 execution. | 1ec166e8a6bec688cbaecc93b97c832890ee4e23bd03c5e963ec35ff52442bf0 |

The intake reparses original Go JSONL, checks exactly one run and terminal per
observed identity, all source/stream/product hashes, full immutable Git tree
membership, unchanged production, all 24 original RED source strings, the eight
presence source strings, and all 46 current graph products. All failed originals
remain separate and unchanged.

Evidence root:
`/mnt/development/projects/evidence/cherri-p10r15-require-condition-20261125`.

Independent receipt: `joined-original-intake-01/result.original.json`.

The 180 identities cover original literal/immutable/mutable true/false conditions
in both dialects and profiles; direct/bound/mutable native Boolean producers;
31 strict backend proof cases; 14 Legacy declaration/action/alias proof cases;
seven rejected non-Boolean source cases; 40 causal native-graph mutations;
all 13 original typed/presence require test families; and the original Release
semantic-safety control. Named parents are included in that count.

## Remaining acceptance

These tests compile real native plist graphs and check original source types,
producer identity, single source evaluation, one mutable snapshot, failure
ordering, exact messages and the final consumer edge. They do not execute Apple
Shortcuts.

Historical signed runtime observations about require/Number output remain
unchanged, including the failed zero assertion and the separate unsuccessful
numeric-cast diagnostic. They are not repaired by adopting a new Boolean API or
by a graph oracle. Fresh signed true/false condition, failure-dialog/cancel,
nested-control, producer and original typed/presence runtime checks on the final
source remain necessary. Final canonical metadata, documentation, Skill and
installed-consumer synchronization also remains part of the release chain.
