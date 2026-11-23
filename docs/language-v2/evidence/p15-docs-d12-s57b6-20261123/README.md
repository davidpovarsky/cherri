# D12 documentation compatibility intake on the corrected compiler

## Result and exact source

**PASS for the bounded documentation workflow and independently reconciled originals.** [Docs Snippet Smoke Test run 45123477520](https://github.com/davidpovarsky/cherrilang.org/actions/runs/45123477520), attempt 1, completed successfully on Docs `ce98fa542785fe4298d3acbf38da120b32bd5029` and compiler `57b6d919757e1d0262993c6f11c6dc1cd17d6177`. The original site job ran from 06:20:52 to 06:21:31 UTC; the original snippet/cache/generator job ran from 06:20:52 to 06:22:09 UTC on 23 November 2026.

D12 is a descendant of D11 `365b08c5b4c71c6d22b591babb8d1ab852e5a3b9`. Its entire source diff consists of the compiler SHA literal in `scripts/smoke_test_snippets.py` and the matching literal in `scripts/verify_generated_reference.py`. Both now name the published compiler containing the authored Boolean-helper correction. The exact two-pin diff has SHA-256 `c8127ebfbf9c4439664f47b4a7de1cdaa88579b2fdb4781a383395be91e8fee7`. The documentation body, examples, generator contracts, control inventories and workflow deadlines were unchanged.

One nonforce audit-branch publication and one workflow dispatch were recorded. The audit branch does not trigger an automatic Docs workflow. The sole observer used the original 1,380-second observation bound and a 60-second interval, exited 0, and did not time out. No duplicate run or intake-time compiler, test, build or API execution was performed.

## What the original evidence proves

| Layer | Independently reconciled result |
|---|---|
| Source and command journal | All 33 original process JSONs and 66 stdout/stderr streams match their exact command, working directory, deadline, terminal exit and recorded hash/size. The full compiler and Docs tracked inventories match immutable Git objects. All 11 original source-file witnesses match Git bytes and blob IDs. Before/after source-identity JSON files are byte-for-byte equal. |
| Reference generation | Original generation took 4.931113 s; unchanged `--check` took 0.064087 s. The entire 243,289-byte generated body equals the compiler canonical source and the Docs body after its exact 119-byte frontmatter. It contains 505 native actions and 15 separate compiler macros, 520 unique headings. SHA-256 remains `8f8a953a14fd65c9bfe6024ea065524d76a8f0549bd1d2e45502cda00dd4d0be`. |
| Generator controls | The original Go command took 1.869614 s. The existing pure event validator reconciled one run/pass for each of 4 required parents and all 8 mutation children, with one package pass and zero skips. No Go command was re-executed during intake. |
| Snippets | All 563 original records were matched in source order to the exact six Docs inputs and their original fence/annotation rules: 36 runnable examples plus 3 exact expected-diagnostic examples executed successfully; 524 signatures/fragments remain reference-only. All runnable records report a successful structured check and native unsigned-plist compilation. All diagnostic records report the exact expected errors and compile rejection. |
| Cache controls | The original log contains all 12 source-defined test identities, each once with `ok`, followed by `Ran 12 tests in 25.808s` and `OK`. Eighteen original source witnesses reconcile six acquire/release lease pairs, two same-byte replacement rejections, three owned-file removals and one exact tracked-file restoration. Each lease returns to the same pristine source S. |
| Binary provenance | The cache receipt, snippet receipt and all six acquired-cache witnesses join binary SHA-256 `2a3812c98f1d0ca337c455017a943c6d8490ea787cc6b5cc750d154c3734e69e`. The original before/after `go version -m` streams are equal and identify Go 1.24.13, Linux amd64, exact S and `vcs.modified=false`. The binary itself is not republished or independently recovered from the artifact. |
| Built site and search | The existing positive validator was replayed only over 13 selected original ZIP members, totaling 1,332,557 bytes, against clean exact D12 source. Its result equals the current top-level original receipt: 520 rendered signatures, 50 current search entries including 21 action-reference entries, 87 verified local links, five current routes and zero historical search entries. |
| Site rejection controls | The six source-defined original controls all report rejection, zero failures/skips and exact artifact restoration. Both current top-level receipt JSON objects were independently joined to their original site-job log bodies. Negative controls were not repeated. Historical D9/D10 files nested inside the built site supplied no current credit. |

The eight generator mutations are optional-primary, typed-default, callable identity, wire key, return type, macro identity, missing category and extra section. The six site controls are historical search route, missing project base in a result, missing project base in the client request, absent home authoring entry, absent home image and damaged canonical fingerprint.

The original rendered actions page has SHA-256 `c863ebd0ec9535fd6da10b82d7e3da8c8366f4b07de0ce4855c51e524c5b29fc`. The original search index has SHA-256 `2387c2c2f8da9996a48c3373e05117b8238bec93bd575c5aec3dcd0f237428a0`.

## Proof packet and original retention

`original-proof.zip` contains the unchanged original generator and snippet artifact ZIPs; selected current site bytes and its two top-level receipts; original source/dispatch/terminal metadata; 20 source-bound file snapshots; and the read-only intake programs/receipts. `originals-manifest.json` records every packet member's byte count, SHA-256 and origin. `source-bindings.json` records immutable Git identities. `cache-log-proof.json` contains fixed test/witness projections with exact original log line numbers and digests; `site-intake.json` records the positive replay and original control/log joins.

All three complete original artifact ZIPs and both complete original job logs remain at:

`/mnt/development/projects/evidence/cherri-p10r15-docs-d12-20261123`

| Original artifact | Bytes | SHA-256 |
|---|---:|---|
| docs-generator-evidence, ID 14794308986 | 643,279 | `2de8947a2597f342ba89cf9384a5284998db3b599cbc896daf1e57a9568ea9d9` |
| docs-snippet-evidence, ID 14795155193 | 25,775 | `852e08cdd6b8a9a90e7e9975943b55871111cce8d6517efe6c75065969de95d7` |
| docs-built-site, ID 14795031258 | 16,136,281 | `7347ec71da8cfc4727553bb8a629a15c11e96cc9a4cb9d0a0c64bf44c89f8404` |

The full 16 MB built-site ZIP stays in that Oracle evidence location. It is not added to this packet or republished recursively.

## Acceptance boundary

This packet proves that the existing documentation compatibility producer actually completed against the corrected immutable compiler S. It does not prove the corrected full F01 native runtime contract: the relevant documentation inputs contain no `isTrue(` or `isFalse(` examples, and Apple execution remains a separate requirement. The unsigned snippet outputs are represented by the original producer's hashes/action counts; their raw bytes are not in the snippet artifact.

This packet grants no full179, S4, physical-device, installed-consumer or final release acceptance. D12 was published only to its audit branch. D12 has not been advanced to Docs main or deployed to Pages by this lane, and the original site receipt correctly says `sitePublished=false`. Publication and consumers require their own exact-source receipts after the parent's remaining gates.
